package metadata

import (
	"context"
	"fmt"
	"strings"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// rowRestriction is the compiled row-level policy for one list read.
//
// When a predicate cannot be built - an unsupported field, a session parameter
// that nothing can resolve yet - the read fails instead of falling back to
// unrestricted rows. A restriction that cannot be applied must never be the
// reason a user sees more than they should.
type rowRestriction struct {
	restricted   bool
	alternatives []PolicyRule
	column       func(string) (listColumn, bool)
}

// listRowRestriction reads the caller's policy from ctx. A context without one
// (Studio, CLI, role-free tests) reads unrestricted, the same convention
// requireObject follows - the hosting layer decides whether a caller is subject
// to application permissions at all.
func listRowRestriction(ctx context.Context, objectID uuid.UUID, column func(string) (listColumn, bool)) rowRestriction {
	permissions, ok := PermissionsFromContext(ctx)
	if !ok {
		return rowRestriction{}
	}
	alternatives, restricted := permissions.RowFilter(objectID, PermissionRead)
	return rowRestriction{restricted: restricted, alternatives: alternatives, column: column}
}

// predicate renders the restriction as one SQL condition and appends whatever
// arguments it needs. Alternatives are OR-ed; no alternatives admits no rows.
func (restriction rowRestriction) predicate(arguments *[]any) (string, error) {
	if !restriction.restricted {
		return "", nil
	}
	if len(restriction.alternatives) == 0 {
		return "FALSE", nil
	}
	rendered := make([]string, 0, len(restriction.alternatives))
	for _, rule := range restriction.alternatives {
		clause, err := restriction.renderRule(rule, arguments)
		if err != nil {
			return "", err
		}
		rendered = append(rendered, clause)
	}
	return "(" + strings.Join(rendered, " OR ") + ")", nil
}

func (restriction rowRestriction) renderRule(rule PolicyRule, arguments *[]any) (string, error) {
	if rule.Parameter != "" {
		// Platform-provided session parameters are a separate, deliberate step;
		// until then such a rule cannot be evaluated, and refusing the read is
		// the only honest outcome.
		return "", fmt.Errorf("access policy on session parameter %q cannot be applied yet", rule.Parameter)
	}
	column, ok := restriction.column(rule.Field)
	if !ok {
		return "", fmt.Errorf("access policy field %q is not available in this list", rule.Field)
	}
	if len(rule.Values) == 0 {
		return "", fmt.Errorf("access policy on %q has no value to compare against", rule.Field)
	}
	placeholders := make([]string, 0, len(rule.Values))
	for _, value := range rule.Values {
		// Values travel as text and PostgreSQL coerces them per column, the same
		// way ordinary dynamic list filters already pass their values.
		*arguments = append(*arguments, value.Data)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(*arguments)))
	}
	switch rule.Operator {
	case PolicyEqual:
		return column.name + " = " + placeholders[0], nil
	case PolicyNotEqual:
		return column.name + " <> " + placeholders[0], nil
	case PolicyIn:
		return column.name + " IN (" + strings.Join(placeholders, ",") + ")", nil
	case PolicyNotIn:
		return column.name + " NOT IN (" + strings.Join(placeholders, ",") + ")", nil
	}
	return "", fmt.Errorf("unsupported access policy operator %q", rule.Operator)
}

// policyListColumn maps a policy field key - an attribute UUID or a lowercase
// standard field key - to the list column it restricts. Names are never
// security identifiers, so attributes resolve through their UUID only.
func policyListColumn(standard func(string) (listColumn, bool), attributes []Attribute) func(string) (listColumn, bool) {
	return func(field string) (listColumn, bool) {
		if id, err := uuid.Parse(field); err == nil {
			column, err := PhysicalAttributeColumn(id)
			if err != nil {
				return listColumn{}, false
			}
			for _, attribute := range attributes {
				if attribute.ID == id {
					kind := TypeKind("")
					if len(attribute.Types) == 1 {
						kind = attribute.Types[0].Kind
					}
					return listColumn{name: column, kind: kind}, true
				}
			}
			return listColumn{}, false
		}
		return standard(field)
	}
}
