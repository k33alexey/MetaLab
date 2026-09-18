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
	parameter    func(string) ([]Value, bool)
	catalog      *Catalog
}

// listRowRestriction reads the caller's policy from ctx. A context without one
// (Studio, CLI, role-free tests) reads unrestricted, the same convention
// requireObject follows - the hosting layer decides whether a caller is subject
// to application permissions at all.
func listRowRestriction(ctx context.Context, catalog *Catalog, objectID uuid.UUID, column func(string) (listColumn, bool)) rowRestriction {
	permissions, ok := PermissionsFromContext(ctx)
	if !ok {
		return rowRestriction{}
	}
	alternatives, restricted := permissions.RowFilter(objectID, PermissionRead)
	return rowRestriction{restricted: restricted, alternatives: alternatives, column: column, catalog: catalog,
		parameter: func(name string) ([]Value, bool) { return sessionValue(ctx, name) }}
}

// readRowPredicate renders the caller's row policy as an extra AND-condition for
// a read that already selects by key, returning "" when nothing is restricted.
// A hidden row then simply does not come back, which is exactly what "not found"
// has to look like: answering with a distinct access error would confirm the
// object exists to someone not allowed to know it.
//
// It returns the fragment rather than a finished statement because these reads
// end in ORDER BY/LIMIT, and the condition has to go before that, not after.
func readRowPredicate(ctx context.Context, catalog *Catalog, objectID uuid.UUID, column func(string) (listColumn, bool), arguments *[]any) (string, error) {
	predicate, err := listRowRestriction(ctx, catalog, objectID, column).predicate(arguments)
	if err != nil || predicate == "" {
		return "", err
	}
	return " AND " + predicate, nil
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
	column, ok := restriction.column(rule.Field)
	if !ok {
		return "", fmt.Errorf("access policy field %q is not available in this list", rule.Field)
	}
	if rule.Subquery != nil {
		return restriction.renderSubquery(column, rule, arguments)
	}
	operands := rule.Values
	if rule.Parameter != "" {
		resolved, ok := []Value(nil), false
		if restriction.parameter != nil {
			resolved, ok = restriction.parameter(rule.Parameter)
		}
		if !ok {
			return "", fmt.Errorf("access policy references session parameter %q, which this session has no value for", rule.Parameter)
		}
		if len(resolved) == 0 {
			// A parameter resolving to an empty set is a real answer, not a
			// missing one: a user with no accessible warehouses sees no rows.
			// Matching everything here would invert the restriction.
			if rule.Operator == PolicyIn || rule.Operator == PolicyEqual {
				return "FALSE", nil
			}
			return "TRUE", nil
		}
		operands = resolved
	}
	if len(operands) == 0 {
		// A literal rule with nothing to compare against is malformed, not a
		// legitimately empty set - validation should have refused it earlier.
		return "", fmt.Errorf("access policy on %q has no value to compare against", rule.Field)
	}
	placeholders := make([]string, 0, len(operands))
	for _, value := range operands {
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

// policySubqueryAlias names the subquery's own table explicitly. Without it an
// inner column that does not exist would silently resolve against the OUTER
// query instead of failing - PostgreSQL's scoping rules make that legal, and it
// would turn a restriction into a condition that is always true.
const policySubqueryAlias = "policy_source"

func (restriction rowRestriction) renderSubquery(column listColumn, rule PolicyRule, arguments *[]any) (string, error) {
	if restriction.catalog == nil {
		return "", fmt.Errorf("access policy subquery cannot be resolved without metadata")
	}
	table, resolve, ok := restriction.catalog.policySubqueryTarget(rule.Subquery.Object)
	if !ok {
		return "", fmt.Errorf("access policy subquery reads an object that is not a catalog or document")
	}
	inner := rowRestriction{restricted: true, catalog: restriction.catalog, parameter: restriction.parameter,
		column: queryPolicyColumn(policySubqueryAlias, resolve)}
	source, ok := inner.column(rule.Subquery.Field)
	if !ok {
		return "", fmt.Errorf("access policy subquery field %q does not belong to the object it reads", rule.Subquery.Field)
	}
	conditions := make([]string, 0, len(rule.Subquery.Where))
	for _, condition := range rule.Subquery.Where {
		clause, err := inner.renderRule(condition, arguments)
		if err != nil {
			return "", err
		}
		conditions = append(conditions, clause)
	}
	statement := "SELECT " + source.name + " FROM " + table + " AS " + policySubqueryAlias
	if len(conditions) != 0 {
		// Conditions narrow one set, so they combine with AND - unlike the
		// alternatives of different roles, which describe separate allowed sets.
		statement += " WHERE " + strings.Join(conditions, " AND ")
	}
	operator := "IN"
	if rule.Operator == PolicyNotIn {
		operator = "NOT IN"
	}
	return column.name + " " + operator + " (" + statement + ")", nil
}

// queryPolicyColumn qualifies a policy column with the source alias the query
// engine gave that table, so a restriction survives joins and self-joins.
func queryPolicyColumn(sqlAlias string, base func(string) (listColumn, bool)) func(string) (listColumn, bool) {
	return func(field string) (listColumn, bool) {
		column, ok := base(field)
		if !ok {
			return listColumn{}, false
		}
		column.name = querySourceColumnSQL(sqlAlias, column.name)
		return column, true
	}
}

func catalogPolicyColumn(definition CatalogDefinition) func(string) (listColumn, bool) {
	return policyListColumn(func(field string) (listColumn, bool) {
		return catalogListField(definition, field)
	}, definition.Attributes)
}

func documentPolicyColumn(definition DocumentDefinition) func(string) (listColumn, bool) {
	return policyListColumn(func(field string) (listColumn, bool) {
		return documentListField(definition, field)
	}, definition.Attributes)
}

// requireUnrestrictedRegisterRead refuses a register read that cannot honour an
// active row restriction, instead of answering with rows the policy meant to
// hide.
//
// Balances and turnovers are the reason this exists: they are served from a
// PRE-AGGREGATED totals table, and those totals were summed over every movement
// regardless of who reads them. A per-row restriction cannot be applied to a
// number that has already been added up - filtering the movements alone would
// contradict the totals, and using the totals as they are would leak exactly
// what the restriction forbids. Until that is resolved (061.11), refusing is the
// only answer that is neither wrong nor silent. Queries through the BSL engine
// are unaffected: they read movements and do honour restrictions.
func requireUnrestrictedRegisterRead(ctx context.Context, objectID uuid.UUID, register string) error {
	permissions, ok := PermissionsFromContext(ctx)
	if !ok {
		return nil
	}
	if _, restricted := permissions.RowFilter(objectID, PermissionRead); !restricted {
		return nil
	}
	return fmt.Errorf("register %s has a row-level access restriction, which this read cannot apply; use a query instead", register)
}

// registerPolicyColumn maps a policy field key to a register column. Registers
// have their own vocabulary - period, line number, activity, movement kind -
// rather than the code/description of a catalog.
//
// "Регистратор" is deliberately absent: it is stored as a type/reference pair,
// and a rule compares one column, so a restriction naming it cannot be rendered.
// Refusing it by name gives a clear message instead of a confusing one about an
// unknown field - and refusing is the only safe answer, since silently dropping
// the condition would widen access.
func registerPolicyColumn(periodic, subordinate bool, movementKind bool, attributes []Attribute) func(string) (listColumn, bool) {
	standard := map[string]listColumn{
		"recordid": {name: "record_id", kind: UUIDType},
		"active":   {name: "active", kind: BooleanType},
	}
	if periodic {
		standard["period"] = listColumn{name: "period", kind: DateType}
	}
	if subordinate {
		standard["linenumber"] = listColumn{name: "line_no", kind: NumberType}
	}
	if movementKind {
		standard["movementkind"] = listColumn{name: "movement_kind", kind: NumberType}
	}
	return policyListColumn(func(field string) (listColumn, bool) {
		column, ok := standard[strings.ToLower(strings.TrimSpace(field))]
		return column, ok
	}, attributes)
}

func informationRegisterPolicyColumn(definition InformationRegisterDefinition) func(string) (listColumn, bool) {
	return registerPolicyColumn(definition.Periodicity != InformationRegisterPeriodNone,
		definition.WriteMode == InformationRegisterRecorder, false, informationRegisterFields(definition))
}

func accumulationRegisterPolicyColumn(definition AccumulationRegisterDefinition) func(string) (listColumn, bool) {
	return registerPolicyColumn(true, true, definition.Kind == AccumulationRegisterBalance, accumulationRegisterFields(definition))
}

// policySubqueryTarget resolves the object a subquery reads: its physical table
// and how to name its fields. Only catalogs and documents qualify - the same
// coverage the restricted side has, so a policy cannot reach through a subquery
// into something the restriction itself could not address.
func (catalog *Catalog) policySubqueryTarget(objectID uuid.UUID) (string, func(string) (listColumn, bool), bool) {
	if index, ok := catalog.catalogByID[objectID]; ok {
		definition := catalog.Catalogs[index]
		table, err := PhysicalCatalogTable(definition.ID)
		if err != nil {
			return "", nil, false
		}
		return qualifiedCatalogTable(table), catalogPolicyColumn(definition), true
	}
	if index, ok := catalog.documentByID[objectID]; ok {
		definition := catalog.Documents[index]
		table, err := PhysicalDocumentTable(definition.ID)
		if err != nil {
			return "", nil, false
		}
		return qualifiedCatalogTable(table), documentPolicyColumn(definition), true
	}
	// A register is the usual home of "which rows is this user allowed" tables,
	// so reading one from a subquery is the main reason subqueries exist here.
	if index, ok := catalog.informationRegisterByID[objectID]; ok {
		definition := catalog.InformationRegisters[index]
		table, err := PhysicalInformationRegisterTable(definition.ID)
		if err != nil {
			return "", nil, false
		}
		return qualifiedCatalogTable(table), informationRegisterPolicyColumn(definition), true
	}
	if index, ok := catalog.accumulationRegisterByID[objectID]; ok {
		definition := catalog.AccumulationRegisters[index]
		table, err := PhysicalAccumulationRegisterTable(definition.ID)
		if err != nil {
			return "", nil, false
		}
		return qualifiedCatalogTable(table), accumulationRegisterPolicyColumn(definition), true
	}
	return "", nil, false
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
