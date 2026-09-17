package metadata

import (
	"context"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// policyCatalog builds a catalog holding one document restricted by the roles
// the caller describes, so the compiled row filter can be inspected directly.
func policyCatalog(t *testing.T, roles ...RoleDefinition) (*Catalog, CatalogDefinition, uuid.UUID) {
	t.Helper()
	warehouse := Attribute{ID: uuid.MustNew(), Name: "Склад", Title: LocalizedText{"ru": "Склад"}, Types: []Type{{Kind: StringType, Length: 50}}}
	definition := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Товары", Title: LocalizedText{"ru": "Товары"},
		Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 150, Attributes: []Attribute{warehouse}}
	for index := range roles {
		for objectIndex := range roles[index].Objects {
			roles[index].Objects[objectIndex].Object = definition.ID
		}
	}
	catalog, err := NewCatalogSnapshotWithRoles(metadataManifest(), nil, nil, nil, []CatalogDefinition{definition}, nil, nil, nil, roles)
	if err != nil {
		t.Fatal(err)
	}
	return catalog, definition, warehouse.ID
}

func readPolicyRole(name string, field string, values ...string) RoleDefinition {
	rule := PolicyRule{Field: field, Operator: PolicyIn}
	for _, value := range values {
		rule.Values = append(rule.Values, Value{Kind: StringType, Data: value})
	}
	return RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: name, Title: LocalizedText{"ru": name},
		Objects: []ObjectPermission{{Operations: []PermissionOperation{PermissionRead},
			Policies: []AccessPolicy{{Operations: []PermissionOperation{PermissionRead}, Rule: &rule}}}}}
}

// An operation granted by a role that attached no restriction must stay
// unrestricted no matter what other roles restrict: holding more roles can only
// widen access.
func TestRowFilterCombinesRolesByUnion(t *testing.T) {
	t.Parallel()
	restricted := readPolicyRole("Кладовщик", "code", "Основной")
	open := RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Полный", Title: LocalizedText{"ru": "Полный"},
		Objects: []ObjectPermission{{Operations: []PermissionOperation{PermissionRead}}}}

	catalog, definition, _ := policyCatalog(t, restricted)
	policy, err := CompilePermissions(catalog, []uuid.UUID{restricted.ID})
	if err != nil {
		t.Fatal(err)
	}
	alternatives, isRestricted := policy.RowFilter(definition.ID, PermissionRead)
	if !isRestricted || len(alternatives) != 1 || alternatives[0].Field != "code" {
		t.Fatalf("single restricted role: %+v restricted=%v", alternatives, isRestricted)
	}

	second := readPolicyRole("Второй", "description", "Розничный")
	catalog, definition, _ = policyCatalog(t, restricted, second)
	policy, err = CompilePermissions(catalog, []uuid.UUID{restricted.ID, second.ID})
	if err != nil {
		t.Fatal(err)
	}
	if alternatives, isRestricted = policy.RowFilter(definition.ID, PermissionRead); !isRestricted || len(alternatives) != 2 {
		t.Fatalf("two restricted roles must union their alternatives: %+v restricted=%v", alternatives, isRestricted)
	}

	catalog, definition, _ = policyCatalog(t, restricted, open)
	policy, err = CompilePermissions(catalog, []uuid.UUID{restricted.ID, open.ID})
	if err != nil {
		t.Fatal(err)
	}
	if alternatives, isRestricted = policy.RowFilter(definition.ID, PermissionRead); isRestricted || alternatives != nil {
		t.Fatalf("an unrestricted grant must win: %+v restricted=%v", alternatives, isRestricted)
	}
}

// An operation that was never granted must report "restricted, no alternatives"
// so a caller that forgets the grant check still admits no rows.
func TestRowFilterFailsClosedForUngrantedOperations(t *testing.T) {
	t.Parallel()
	role := readPolicyRole("Кладовщик", "code", "A")
	catalog, definition, _ := policyCatalog(t, role)
	policy, err := CompilePermissions(catalog, []uuid.UUID{role.ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []PermissionOperation{PermissionUpdate, PermissionDelete} {
		if alternatives, restricted := policy.RowFilter(definition.ID, operation); !restricted || len(alternatives) != 0 {
			t.Fatalf("%s: %+v restricted=%v", operation, alternatives, restricted)
		}
	}
	var missing *Permissions
	if _, restricted := missing.RowFilter(definition.ID, PermissionRead); !restricted {
		t.Fatal("a nil policy must not read as unrestricted")
	}
}

func TestRowRestrictionPredicate(t *testing.T) {
	t.Parallel()
	column := func(field string) (listColumn, bool) {
		if field == "code" {
			return listColumn{name: "code", kind: StringType}, true
		}
		return listColumn{}, false
	}
	rule := func(operator PolicyOperator, values ...string) PolicyRule {
		result := PolicyRule{Field: "code", Operator: operator}
		for _, value := range values {
			result.Values = append(result.Values, Value{Kind: StringType, Data: value})
		}
		return result
	}

	if predicate, _ := (rowRestriction{}).predicate(&[]any{}); predicate != "" {
		t.Fatalf("unrestricted read must add no condition, got %q", predicate)
	}
	if predicate, _ := (rowRestriction{restricted: true}).predicate(&[]any{}); predicate != "FALSE" {
		t.Fatalf("a restriction with no alternatives must admit no rows, got %q", predicate)
	}

	tests := map[string]struct {
		rule      PolicyRule
		want      string
		arguments int
	}{
		"equal":     {rule(PolicyEqual, "A"), "(code = $1)", 1},
		"not equal": {rule(PolicyNotEqual, "A"), "(code <> $1)", 1},
		"in list":   {rule(PolicyIn, "A", "B"), "(code IN ($1,$2))", 2},
		"not in":    {rule(PolicyNotIn, "A", "B"), "(code NOT IN ($1,$2))", 2},
	}
	for name, test := range tests {
		arguments := []any{}
		predicate, err := rowRestriction{restricted: true, alternatives: []PolicyRule{test.rule}, column: column}.predicate(&arguments)
		if err != nil || predicate != test.want || len(arguments) != test.arguments {
			t.Fatalf("%s: predicate=%q arguments=%v error=%v", name, predicate, arguments, err)
		}
	}

	arguments := []any{}
	predicate, err := rowRestriction{restricted: true, column: column,
		alternatives: []PolicyRule{rule(PolicyEqual, "A"), rule(PolicyEqual, "B")}}.predicate(&arguments)
	if err != nil || predicate != "(code = $1 OR code = $2)" {
		t.Fatalf("alternatives must be OR-ed: %q error=%v", predicate, err)
	}
}

// A restriction that cannot be rendered must fail the read rather than quietly
// return unrestricted rows.
func TestRowRestrictionRefusesWhatItCannotApply(t *testing.T) {
	t.Parallel()
	column := func(string) (listColumn, bool) { return listColumn{}, false }
	known := func(field string) (listColumn, bool) { return listColumn{name: "code"}, field == "code" }
	tests := map[string]struct {
		rule    PolicyRule
		column  func(string) (listColumn, bool)
		message string
	}{
		"session parameter":   {PolicyRule{Field: "code", Operator: PolicyEqual, Parameter: "ТекущийСклад"}, known, "session parameter"},
		"field not in list":   {PolicyRule{Field: "code", Operator: PolicyEqual, Values: []Value{{Data: "A"}}}, column, "not available"},
		"no value to compare": {PolicyRule{Field: "code", Operator: PolicyEqual}, known, "no value"},
		"unknown operator":    {PolicyRule{Field: "code", Operator: PolicyOperator("like"), Values: []Value{{Data: "A"}}}, known, "unsupported"},
	}
	for name, test := range tests {
		arguments := []any{}
		_, err := rowRestriction{restricted: true, alternatives: []PolicyRule{test.rule}, column: test.column}.predicate(&arguments)
		if err == nil || !strings.Contains(err.Error(), test.message) {
			t.Fatalf("%s: error=%v", name, err)
		}
	}
}

// A context without a policy reads unrestricted - Studio and the CLI rely on it.
func TestListRowRestrictionFollowsContext(t *testing.T) {
	t.Parallel()
	object := uuid.MustNew()
	if restriction := listRowRestriction(context.Background(), object, nil); restriction.restricted {
		t.Fatal("a context without a policy must read unrestricted")
	}
	role := readPolicyRole("Кладовщик", "code", "A")
	catalog, definition, _ := policyCatalog(t, role)
	policy, err := CompilePermissions(catalog, []uuid.UUID{role.ID})
	if err != nil {
		t.Fatal(err)
	}
	restriction := listRowRestriction(WithPermissions(context.Background(), policy), definition.ID, nil)
	if !restriction.restricted || len(restriction.alternatives) != 1 {
		t.Fatalf("restriction=%+v", restriction)
	}
}
