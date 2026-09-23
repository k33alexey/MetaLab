package metadata

import (
	"bytes"
	"context"
	"errors"
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

	if predicate, _ := (rowRestriction{}).predicate(context.Background(), &[]any{}); predicate != "" {
		t.Fatalf("unrestricted read must add no condition, got %q", predicate)
	}
	if predicate, _ := (rowRestriction{restricted: true}).predicate(context.Background(), &[]any{}); predicate != "FALSE" {
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
		predicate, err := rowRestriction{restricted: true, alternatives: []PolicyRule{test.rule}, column: column}.predicate(context.Background(), &arguments)
		if err != nil || predicate != test.want || len(arguments) != test.arguments {
			t.Fatalf("%s: predicate=%q arguments=%v error=%v", name, predicate, arguments, err)
		}
	}

	arguments := []any{}
	predicate, err := rowRestriction{restricted: true, column: column,
		alternatives: []PolicyRule{rule(PolicyEqual, "A"), rule(PolicyEqual, "B")}}.predicate(context.Background(), &arguments)
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
		_, err := rowRestriction{restricted: true, alternatives: []PolicyRule{test.rule}, column: test.column}.predicate(context.Background(), &arguments)
		if err == nil || !strings.Contains(err.Error(), test.message) {
			t.Fatalf("%s: error=%v", name, err)
		}
	}
}

// A rule comparing against a session parameter is evaluated from values the
// hosting layer resolved, without any BSL runtime on the read path.
func TestRowRestrictionResolvesSessionParameters(t *testing.T) {
	t.Parallel()
	column := func(string) (listColumn, bool) { return listColumn{name: "owner"}, true }
	render := func(values map[string][]Value, rule PolicyRule) (string, []any, error) {
		restriction := rowRestriction{restricted: true, alternatives: []PolicyRule{rule}, column: column,
			parameter: sessionValue}
		arguments := []any{}
		predicate, err := restriction.predicate(WithSessionValues(context.Background(), values), &arguments)
		return predicate, arguments, err
	}
	user := Value{Kind: ObjectUUIDType, Data: uuid.MustNew().String()}

	predicate, arguments, err := render(map[string][]Value{CurrentUserParameter: {user}},
		PolicyRule{Field: "owner", Operator: PolicyEqual, Parameter: CurrentUserParameter})
	if err != nil || predicate != "(owner = $1)" || len(arguments) != 1 || arguments[0] != user.Data {
		t.Fatalf("single value: %q %v error=%v", predicate, arguments, err)
	}

	// A collection is the ordinary case, not the exception.
	warehouses := []Value{{Kind: StringType, Data: "Основной"}, {Kind: StringType, Data: "Розничный"}}
	predicate, arguments, err = render(map[string][]Value{"ДоступныеСклады": warehouses},
		PolicyRule{Field: "owner", Operator: PolicyIn, Parameter: "ДоступныеСклады"})
	if err != nil || predicate != "(owner IN ($1,$2))" || len(arguments) != 2 {
		t.Fatalf("collection: %q %v error=%v", predicate, arguments, err)
	}

	// An empty collection is a real answer - no accessible warehouses means no
	// rows - and must not be read as "no restriction".
	predicate, _, err = render(map[string][]Value{"ДоступныеСклады": {}},
		PolicyRule{Field: "owner", Operator: PolicyIn, Parameter: "ДоступныеСклады"})
	if err != nil || predicate != "(FALSE)" {
		t.Fatalf("empty collection must admit no rows: %q error=%v", predicate, err)
	}
	// Its negation is the mirror image: excluded from nothing means everything.
	predicate, _, err = render(map[string][]Value{"ДоступныеСклады": {}},
		PolicyRule{Field: "owner", Operator: PolicyNotIn, Parameter: "ДоступныеСклады"})
	if err != nil || predicate != "(TRUE)" {
		t.Fatalf("empty negated collection: %q error=%v", predicate, err)
	}

	// A parameter this session has no value for refuses the read rather than
	// quietly matching everything.
	if _, _, err := render(map[string][]Value{}, PolicyRule{Field: "owner", Operator: PolicyEqual, Parameter: "НетТакого"}); err == nil {
		t.Fatal("unresolved session parameter was ignored")
	}
}

// The platform owns this name, so a project must not be able to declare it too.
func TestSessionParameterNameIsReserved(t *testing.T) {
	t.Parallel()
	if !ReservedSessionParameter(CurrentUserParameter) || !ReservedSessionParameter("currentuser") {
		t.Fatal("reserved names are not recognised")
	}
	if ReservedSessionParameter("ТекущийСклад") {
		t.Fatal("an ordinary name was treated as reserved")
	}
	parameter := SessionParameter{Format: CurrentFormat, ID: uuid.MustNew(), Name: CurrentUserParameter,
		Title: LocalizedText{"ru": "Текущий пользователь"}, Types: []Type{{Kind: ObjectUUIDType}}}
	var encoded bytes.Buffer
	if err := Encode(&encoded, parameter); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSessionParameter("p.yaml", bytes.NewReader(encoded.Bytes()), metadataManifest()); err == nil {
		t.Fatal("a project was allowed to declare the platform's own session parameter")
	}
}

// Totals are summed over every movement, so a per-row restriction cannot be
// applied to them after the fact. Refusing beats answering with rows the policy
// meant to hide.
func TestRegisterAggregateReadRefusesUnappliedRestriction(t *testing.T) {
	t.Parallel()
	role := readPolicyRole("Кладовщик", "code", "A")
	catalog, definition, _ := policyCatalog(t, role)
	policy, err := CompilePermissions(catalog, []uuid.UUID{role.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := requireUnrestrictedRegisterRead(context.Background(), definition.ID, "ОстаткиТоваров"); err != nil {
		t.Fatalf("a caller with no policy must not be refused: %v", err)
	}
	restricted := WithPermissions(context.Background(), policy)
	if err := requireUnrestrictedRegisterRead(restricted, definition.ID, "ОстаткиТоваров"); err == nil {
		t.Fatal("an aggregate read answered despite a restriction it cannot apply")
	}
	// An object the policy does not restrict at all still reads normally.
	if err := requireUnrestrictedRegisterRead(restricted, uuid.MustNew(), "Другой"); err == nil {
		t.Fatal("an ungranted object must be refused as well")
	}
}

// A context without a policy reads unrestricted - Studio and the CLI rely on it.
func TestListRowRestrictionFollowsContext(t *testing.T) {
	t.Parallel()
	object := uuid.MustNew()
	if restriction := listRowRestriction(context.Background(), nil, object, nil); restriction.restricted {
		t.Fatal("a context without a policy must read unrestricted")
	}
	role := readPolicyRole("Кладовщик", "code", "A")
	catalog, definition, _ := policyCatalog(t, role)
	policy, err := CompilePermissions(catalog, []uuid.UUID{role.ID})
	if err != nil {
		t.Fatal(err)
	}
	restriction := listRowRestriction(WithPermissions(context.Background(), policy), catalog, definition.ID, nil)
	if !restriction.restricted || len(restriction.alternatives) != 1 {
		t.Fatalf("restriction=%+v", restriction)
	}
}

// Values the PROJECT computes reach a restriction through a resolver, because
// producing them means running the session module - work no read should do
// unless a restriction actually asks for it.
func TestRowRestrictionAsksTheProjectResolverForItsOwnParameters(t *testing.T) {
	t.Parallel()
	column := func(string) (listColumn, bool) { return listColumn{name: "warehouse"}, true }
	render := func(ctx context.Context, rule PolicyRule) (string, []any, error) {
		restriction := rowRestriction{restricted: true, alternatives: []PolicyRule{rule}, column: column, parameter: sessionValue}
		arguments := []any{}
		predicate, err := restriction.predicate(ctx, &arguments)
		return predicate, arguments, err
	}
	rule := PolicyRule{Field: "warehouse", Operator: PolicyIn, Parameter: "ДоступныеСклады"}

	asked := 0
	resolver := func(_ context.Context, name string) ([]Value, bool, error) {
		asked++
		if name != "ДоступныеСклады" {
			return nil, false, nil
		}
		return []Value{{Kind: StringType, Data: "Основной"}, {Kind: StringType, Data: "Розничный"}}, true, nil
	}
	ctx := WithSessionResolver(WithSessionValues(context.Background(), map[string][]Value{
		CurrentUserParameter: {{Kind: ObjectUUIDType, Data: uuid.MustNew().String()}},
	}), resolver)
	predicate, arguments, err := render(ctx, rule)
	if err != nil || predicate != "(warehouse IN ($1,$2))" || len(arguments) != 2 {
		t.Fatalf("resolved list: %q %v error=%v", predicate, arguments, err)
	}

	// A platform-owned name is answered by the platform: application code must
	// not be able to decide who the current user is.
	if _, _, err := render(ctx, PolicyRule{Field: "warehouse", Operator: PolicyEqual, Parameter: CurrentUserParameter}); err != nil {
		t.Fatal(err)
	}
	if asked != 1 {
		t.Fatalf("resolver consulted %d times, want once - the platform name must not reach it", asked)
	}

	// A resolver that fails fails the read, and says why: the alternative is
	// answering as if the restriction were satisfied.
	failing := WithSessionResolver(context.Background(), func(context.Context, string) ([]Value, bool, error) {
		return nil, false, errors.New("склады недоступны")
	})
	if _, _, err := render(failing, rule); err == nil || !strings.Contains(err.Error(), "склады недоступны") {
		t.Fatalf("failing resolver: %v", err)
	}

	// A resolver with no answer is not an empty answer: the read refuses.
	absent := WithSessionResolver(context.Background(), func(context.Context, string) ([]Value, bool, error) { return nil, false, nil })
	if _, _, err := render(absent, rule); err == nil || !strings.Contains(err.Error(), "has no value for") {
		t.Fatalf("resolver without an answer: %v", err)
	}
}
