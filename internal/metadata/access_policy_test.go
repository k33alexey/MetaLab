package metadata

import (
	"bytes"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// policyRoleFixture grants read on one object and restricts those reads to the
// rows whose warehouse attribute matches the session parameter.
func policyRoleFixture() (RoleDefinition, uuid.UUID) {
	warehouse := uuid.MustNew()
	role := RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Кладовщик", Title: LocalizedText{"ru": "Кладовщик"},
		PolicyTemplates: []PolicyTemplate{{Name: "ПоСкладу", Rule: PolicyRule{Field: warehouse.String(), Operator: PolicyIn, Parameter: "ДоступныеСклады"}}},
		Objects: []ObjectPermission{{Object: uuid.MustNew(), Operations: []PermissionOperation{PermissionRead},
			Policies: []AccessPolicy{{Operations: []PermissionOperation{PermissionRead}, Template: "ПоСкладу"}}}}}
	return role, warehouse
}

// publishablePolicyRole returns a role whose restriction survives catalog
// validation: the rule compares a real attribute against literal values.
func publishablePolicyRole() (RoleDefinition, CatalogDefinition) {
	warehouse := Attribute{ID: uuid.MustNew(), Name: "Склад", Title: LocalizedText{"ru": "Склад"}, Types: []Type{{Kind: StringType, Length: 50}}}
	definition := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Товары", Title: LocalizedText{"ru": "Товары"},
		Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 150, Attributes: []Attribute{warehouse}}
	role := RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Кладовщик", Title: LocalizedText{"ru": "Кладовщик"},
		PolicyTemplates: []PolicyTemplate{{Name: "ПоСкладу", Rule: PolicyRule{Field: warehouse.ID.String(), Operator: PolicyIn,
			Values: []Value{{Kind: StringType, Data: "Основной"}}}}},
		Objects: []ObjectPermission{{Object: definition.ID, Operations: []PermissionOperation{PermissionRead},
			Policies: []AccessPolicy{{Operations: []PermissionOperation{PermissionRead}, Template: "ПоСкладу"}}}}}
	return role, definition
}

// Publication validation is the whole point of declarative rules: a restriction
// that cannot be resolved must never reach a working database.
func TestCatalogRejectsUnresolvablePolicies(t *testing.T) {
	t.Parallel()
	tests := map[string]func(*RoleDefinition, CatalogDefinition){
		"field of another object": func(r *RoleDefinition, _ CatalogDefinition) {
			r.PolicyTemplates[0].Rule.Field = uuid.MustNew().String()
		},
		"operation the object does not support": func(r *RoleDefinition, _ CatalogDefinition) {
			r.Objects[0].Operations = append(r.Objects[0].Operations, PermissionPost)
			r.Objects[0].Policies[0].Operations = []PermissionOperation{PermissionPost}
		},
		"unknown session parameter": func(r *RoleDefinition, _ CatalogDefinition) {
			r.PolicyTemplates[0].Rule = PolicyRule{Field: r.PolicyTemplates[0].Rule.Field, Operator: PolicyEqual, Parameter: "НетТакого"}
		},
	}
	for name, mutate := range tests {
		role, definition := publishablePolicyRole()
		mutate(&role, definition)
		if _, err := NewCatalogSnapshotWithRoles(metadataManifest(), nil, nil, nil, []CatalogDefinition{definition}, nil, nil, nil, []RoleDefinition{role}); err == nil {
			t.Fatalf("catalog accepted a policy with %s", name)
		}
	}
}

// The same rule must be accepted once the session parameter it names exists.
func TestCatalogAcceptsPolicyBoundToSessionParameter(t *testing.T) {
	t.Parallel()
	role, definition := publishablePolicyRole()
	role.PolicyTemplates[0].Rule = PolicyRule{Field: role.PolicyTemplates[0].Rule.Field, Operator: PolicyEqual, Parameter: "ТекущийСклад"}
	parameter := SessionParameter{Format: CurrentFormat, ID: uuid.MustNew(), Name: "ТекущийСклад", Title: LocalizedText{"ru": "Текущий склад"},
		Types: []Type{{Kind: StringType, Length: 50}}}
	if _, err := NewCatalogSnapshotWithSessionParameters(metadataManifest(), nil, nil, nil, []CatalogDefinition{definition}, nil, nil, nil,
		[]RoleDefinition{role}, nil, []SessionParameter{parameter}); err != nil {
		t.Fatal(err)
	}
}

// A template exists to be reused for a different field on each object; without
// parameters it is one fixed rule and reuse is impossible.
func TestParameterizedPolicyTemplates(t *testing.T) {
	t.Parallel()
	owner, warehouse := uuid.MustNew(), uuid.MustNew()
	role := RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Кладовщик", Title: LocalizedText{"ru": "Кладовщик"},
		PolicyTemplates: []PolicyTemplate{{Name: "Своё", Parameters: []string{"Поле"},
			Rule: PolicyRule{Field: "$Поле", Operator: PolicyEqual, Parameter: CurrentUserParameter}}},
		Objects: []ObjectPermission{{Object: uuid.MustNew(), Operations: []PermissionOperation{PermissionRead},
			Policies: []AccessPolicy{
				{Operations: []PermissionOperation{PermissionRead}, Template: "Своё", Arguments: []string{owner.String()}},
				{Operations: []PermissionOperation{PermissionUpdate}, Template: "Своё", Arguments: []string{warehouse.String()}},
			}}}}
	role.Objects[0].Operations = append(role.Objects[0].Operations, PermissionUpdate)
	if err := ValidateRole("role.yaml", role, metadataManifest()); err != nil {
		t.Fatal(err)
	}
	// The same template resolves to a different field for each use.
	read, err := resolveRolePolicies(role, role.Objects[0], PermissionRead)
	if err != nil || len(read) != 1 || read[0].Field != owner.String() {
		t.Fatalf("read: %+v error=%v", read, err)
	}
	update, err := resolveRolePolicies(role, role.Objects[0], PermissionUpdate)
	if err != nil || len(update) != 1 || update[0].Field != warehouse.String() {
		t.Fatalf("update: %+v error=%v", update, err)
	}

	broken := map[string]func(*RoleDefinition){
		"too few arguments":      func(r *RoleDefinition) { r.Objects[0].Policies[0].Arguments = nil },
		"too many arguments":     func(r *RoleDefinition) { r.Objects[0].Policies[0].Arguments = []string{"code", "description"} },
		"undeclared placeholder": func(r *RoleDefinition) { r.PolicyTemplates[0].Rule.Field = "$Другое" },
		"placeholder inline": func(r *RoleDefinition) {
			r.Objects[0].Policies[0] = AccessPolicy{Operations: []PermissionOperation{PermissionRead}, Rule: &PolicyRule{Field: "$Поле", Operator: PolicyEqual, Parameter: CurrentUserParameter}}
		},
		"arguments on inline": func(r *RoleDefinition) {
			r.Objects[0].Policies[0] = AccessPolicy{Operations: []PermissionOperation{PermissionRead}, Rule: &PolicyRule{Field: "code", Operator: PolicyEqual, Parameter: CurrentUserParameter}, Arguments: []string{"code"}}
		},
		"duplicate parameter": func(r *RoleDefinition) { r.PolicyTemplates[0].Parameters = []string{"Поле", "Поле"} },
	}
	for name, mutate := range broken {
		invalid := cloneRole(role)
		mutate(&invalid)
		if err := ValidateRole("role.yaml", invalid, metadataManifest()); err == nil {
			t.Fatalf("accepted a role with %s", name)
		}
	}
}

// Membership in a set read from another table is the shape real restrictions
// use; comparing against a literal or a parameter cannot express it.
func TestPolicySubqueryValidation(t *testing.T) {
	t.Parallel()
	field, source := uuid.MustNew().String(), uuid.MustNew()
	valid := PolicyRule{Field: field, Operator: PolicyIn, Subquery: &PolicySubquery{Object: source, Field: "code",
		Where: []PolicyRule{{Field: "description", Operator: PolicyEqual, Parameter: CurrentUserParameter}}}}
	role := func(rule PolicyRule) RoleDefinition {
		return RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Кладовщик", Title: LocalizedText{"ru": "Кладовщик"},
			Objects: []ObjectPermission{{Object: uuid.MustNew(), Operations: []PermissionOperation{PermissionRead},
				Policies: []AccessPolicy{{Operations: []PermissionOperation{PermissionRead}, Rule: &rule}}}}}
	}
	if err := ValidateRole("role.yaml", role(valid), metadataManifest()); err != nil {
		t.Fatal(err)
	}
	broken := map[string]PolicyRule{
		"wrong operator": {Field: field, Operator: PolicyEqual, Subquery: &PolicySubquery{Object: source, Field: "code"}},
		"zero object":    {Field: field, Operator: PolicyIn, Subquery: &PolicySubquery{Field: "code"}},
		"bad field":      {Field: field, Operator: PolicyIn, Subquery: &PolicySubquery{Object: source, Field: "Код"}},
		"with values":    {Field: field, Operator: PolicyIn, Values: []Value{{Data: "x"}}, Subquery: &PolicySubquery{Object: source, Field: "code"}},
		"nested": {Field: field, Operator: PolicyIn, Subquery: &PolicySubquery{Object: source, Field: "code",
			Where: []PolicyRule{{Field: "code", Operator: PolicyIn, Subquery: &PolicySubquery{Object: source, Field: "code"}}}}},
	}
	for name, rule := range broken {
		if err := ValidateRole("role.yaml", role(rule), metadataManifest()); err == nil {
			t.Fatalf("accepted a subquery with %s", name)
		}
	}
}

func TestDecodeRoleWithAccessPolicies(t *testing.T) {
	t.Parallel()
	role, _ := policyRoleFixture()
	var encoded bytes.Buffer
	if err := Encode(&encoded, role); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeRole("role.yaml", bytes.NewReader(encoded.Bytes()), metadataManifest())
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.PolicyTemplates) != 1 || decoded.PolicyTemplates[0].Name != "ПоСкладу" {
		t.Fatalf("templates = %+v", decoded.PolicyTemplates)
	}
	if rule := decoded.PolicyTemplates[0].Rule; rule.Operator != PolicyIn || rule.Parameter != "ДоступныеСклады" || len(rule.Values) != 0 {
		t.Fatalf("rule = %+v", rule)
	}
	if policies := decoded.Objects[0].Policies; len(policies) != 1 || policies[0].Template != "ПоСкладу" || policies[0].Rule != nil {
		t.Fatalf("policies = %+v", policies)
	}
}

func TestValidateRoleRejectsBrokenPolicies(t *testing.T) {
	t.Parallel()
	field := uuid.MustNew().String()
	inline := func(rule PolicyRule) RoleDefinition {
		role, _ := policyRoleFixture()
		role.PolicyTemplates = nil
		role.Objects[0].Policies = []AccessPolicy{{Operations: []PermissionOperation{PermissionRead}, Rule: &rule}}
		return role
	}
	tests := map[string]func(*RoleDefinition){
		"template does not exist": func(r *RoleDefinition) { r.Objects[0].Policies[0].Template = "Отсутствует" },
		"rule and template both": func(r *RoleDefinition) {
			r.Objects[0].Policies[0].Rule = &PolicyRule{Field: field, Operator: PolicyEqual, Parameter: "ТекущийПользователь"}
		},
		"duplicate template name": func(r *RoleDefinition) { r.PolicyTemplates = append(r.PolicyTemplates, r.PolicyTemplates[0]) },
		"template name invalid":   func(r *RoleDefinition) { r.PolicyTemplates[0].Name = "По складу" },
		"no operations":           func(r *RoleDefinition) { r.Objects[0].Policies[0].Operations = nil },
		"field level operation":   func(r *RoleDefinition) { r.PolicyTemplates[0].Rule.Field = "Код" },
	}
	for name, mutate := range tests {
		role, _ := policyRoleFixture()
		mutate(&role)
		if err := ValidateRole("role.yaml", role, metadataManifest()); err == nil {
			t.Fatalf("accepted role with %s", name)
		}
	}

	rules := map[string]PolicyRule{
		"no operand":              {Field: field, Operator: PolicyEqual},
		"values and parameter":    {Field: field, Operator: PolicyEqual, Parameter: "П", Values: []Value{{Kind: StringType, Data: "x"}}},
		"unknown operator":        {Field: field, Operator: PolicyOperator("like"), Values: []Value{{Kind: StringType, Data: "x"}}},
		"eq with two values":      {Field: field, Operator: PolicyEqual, Values: []Value{{Kind: StringType, Data: "a"}, {Kind: StringType, Data: "b"}}},
		"parameter is not a name": {Field: field, Operator: PolicyEqual, Parameter: "не имя"},
	}
	for name, rule := range rules {
		if err := ValidateRole("role.yaml", inline(rule), metadataManifest()); err == nil {
			t.Fatalf("accepted rule with %s", name)
		}
	}
}

func TestValidateRoleAcceptsListAndLiteralRules(t *testing.T) {
	t.Parallel()
	field := uuid.MustNew().String()
	rules := map[string]PolicyRule{
		"single literal":     {Field: field, Operator: PolicyEqual, Values: []Value{{Kind: StringType, Data: "Основной"}}},
		"negated literal":    {Field: field, Operator: PolicyNotEqual, Values: []Value{{Kind: StringType, Data: "Основной"}}},
		"literal list":       {Field: field, Operator: PolicyIn, Values: []Value{{Kind: StringType, Data: "Основной"}, {Kind: StringType, Data: "Розничный"}}},
		"negated list":       {Field: field, Operator: PolicyNotIn, Values: []Value{{Kind: StringType, Data: "Архив"}}},
		"session parameter":  {Field: field, Operator: PolicyEqual, Parameter: "ТекущийПользователь"},
		"standard field key": {Field: "description", Operator: PolicyEqual, Parameter: "ТекущийПользователь"},
	}
	for name, rule := range rules {
		role, _ := policyRoleFixture()
		role.PolicyTemplates = nil
		role.Objects[0].Policies = []AccessPolicy{{Operations: []PermissionOperation{PermissionRead}, Rule: &rule}}
		if err := ValidateRole("role.yaml", role, metadataManifest()); err != nil {
			t.Fatalf("rejected %s: %v", name, err)
		}
	}
}

// A literal list is what makes the operator useful before session parameters can
// hold collections, so it must survive a YAML round trip intact.
func TestPolicyLiteralListRoundTrip(t *testing.T) {
	t.Parallel()
	role, _ := policyRoleFixture()
	role.PolicyTemplates[0].Rule = PolicyRule{Field: uuid.MustNew().String(), Operator: PolicyIn,
		Values: []Value{{Kind: StringType, Data: "Основной"}, {Kind: StringType, Data: "Розничный"}}}
	var encoded bytes.Buffer
	if err := Encode(&encoded, role); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeRole("role.yaml", bytes.NewReader(encoded.Bytes()), metadataManifest())
	if err != nil {
		t.Fatal(err)
	}
	values := decoded.PolicyTemplates[0].Rule.Values
	if len(values) != 2 || values[0].Data != "Основной" || values[1].Data != "Розничный" {
		t.Fatalf("values = %+v", values)
	}
	if strings.Contains(encoded.String(), "parameter") {
		t.Fatalf("empty parameter was encoded:\n%s", encoded.String())
	}
}

// Roles handed out by the catalog must be deep copies: a caller mutating a
// policy must not reach the shared snapshot every request reads.
func TestCatalogRoleClonesPolicies(t *testing.T) {
	t.Parallel()
	role, definition := publishablePolicyRole()
	catalog, err := NewCatalogSnapshotWithRoles(metadataManifest(), nil, nil, nil, []CatalogDefinition{definition}, nil, nil, nil, []RoleDefinition{role})
	if err != nil {
		t.Fatal(err)
	}
	first, ok := catalog.RoleByID(role.ID)
	if !ok {
		t.Fatal("role is missing from the catalog")
	}
	first.PolicyTemplates[0].Name = "Подменено"
	first.PolicyTemplates[0].Rule.Values = append(first.PolicyTemplates[0].Rule.Values, Value{Kind: StringType, Data: "x"})
	first.Objects[0].Policies[0].Operations[0] = PermissionDelete
	second, _ := catalog.RoleByID(role.ID)
	if second.PolicyTemplates[0].Name != "ПоСкладу" || len(second.PolicyTemplates[0].Rule.Values) != 1 {
		t.Fatalf("template leaked: %+v", second.PolicyTemplates[0])
	}
	if second.Objects[0].Policies[0].Operations[0] != PermissionRead {
		t.Fatalf("policy operations leaked: %+v", second.Objects[0].Policies[0])
	}
}
