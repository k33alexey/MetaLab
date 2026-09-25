package metadata

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func roleFixture() RoleDefinition {
	return RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Читатель", Title: LocalizedText{"ru": "Читатель"},
		Objects: []ObjectPermission{{Object: uuid.MustNew(), Operations: []PermissionOperation{PermissionRead},
			Fields: []FieldPermission{{Field: "description", Operations: []PermissionOperation{PermissionRead}}}}}}
}

func TestDecodeRoleStrictAndBounded(t *testing.T) {
	t.Parallel()
	role := roleFixture()
	var encoded bytes.Buffer
	if err := Encode(&encoded, role); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeRole("role.yaml", bytes.NewReader(encoded.Bytes()), metadataConfiguration())
	if err != nil || decoded.ID != role.ID {
		t.Fatalf("decode role = %+v, %v", decoded, err)
	}
	for _, suffix := range []string{"unknown: true\n", "---\nformat: 1\n"} {
		if _, err := DecodeRole("role.yaml", strings.NewReader(encoded.String()+suffix), metadataConfiguration()); err == nil {
			t.Fatalf("accepted invalid suffix %q", suffix)
		}
	}
	tests := map[string]func(*RoleDefinition){
		"format":            func(r *RoleDefinition) { r.Format++ },
		"zero identity":     func(r *RoleDefinition) { r.ID = uuid.UUID{} },
		"name":              func(r *RoleDefinition) { r.Name = "Invalid Name" },
		"language":          func(r *RoleDefinition) { r.Title = LocalizedText{"de": "Leser"} },
		"zero object":       func(r *RoleDefinition) { r.Objects[0].Object = uuid.UUID{} },
		"duplicate object":  func(r *RoleDefinition) { r.Objects = append(r.Objects, r.Objects[0]) },
		"unknown operation": func(r *RoleDefinition) { r.Objects[0].Operations = []PermissionOperation{"admin"} },
		"duplicate operation": func(r *RoleDefinition) {
			r.Objects[0].Operations = []PermissionOperation{PermissionRead, PermissionRead}
		},
		"field operation":         func(r *RoleDefinition) { r.Objects[0].Fields[0].Operations = []PermissionOperation{PermissionPost} },
		"empty field operations":  func(r *RoleDefinition) { r.Objects[0].Fields[0].Operations = nil },
		"duplicate field":         func(r *RoleDefinition) { r.Objects[0].Fields = append(r.Objects[0].Fields, r.Objects[0].Fields[0]) },
		"empty field":             func(r *RoleDefinition) { r.Objects[0].Fields[0].Field = "" },
		"zero field UUID":         func(r *RoleDefinition) { r.Objects[0].Fields[0].Field = "00000000-0000-0000-0000-000000000000" },
		"noncanonical field UUID": func(r *RoleDefinition) { r.Objects[0].Fields[0].Field = "ABCDEFAB-0000-4000-8000-000000000001" },
		"field alias":             func(r *RoleDefinition) { r.Objects[0].Fields[0].Field = "Наименование" },
		"zero command":            func(r *RoleDefinition) { r.Commands = []CommandPermission{{Form: uuid.MustNew()}} },
		"zero form":               func(r *RoleDefinition) { r.Commands = []CommandPermission{{Command: uuid.MustNew()}} },
		"duplicate command": func(r *RoleDefinition) {
			c := CommandPermission{Form: uuid.MustNew(), Command: uuid.MustNew()}
			r.Commands = []CommandPermission{c, c}
		},
		"too many commands": func(r *RoleDefinition) { r.Commands = make([]CommandPermission, MaxRolePermissions+1) },
		"too many fields":   func(r *RoleDefinition) { r.Objects[0].Fields = make([]FieldPermission, MaxRolePermissions) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			value := cloneRole(role)
			mutate(&value)
			if err := ValidateRole("role.yaml", value, metadataConfiguration()); err == nil {
				t.Fatal("invalid role accepted")
			} else if name == "format" && !errors.Is(err, ErrUnsupportedFormat) {
				t.Fatalf("format error = %v", err)
			}
		})
	}
	empty := cloneRole(role)
	empty.Objects = nil
	if err := ValidateRole("empty-role.yaml", empty, metadataConfiguration()); err != nil {
		t.Fatalf("empty role must be valid and grant nothing: %v", err)
	}
}

func roleCatalogFixture(t *testing.T) (*Catalog, RoleDefinition, ManagedForm) {
	t.Helper()
	role := roleFixture()
	definition := CatalogDefinition{Format: CurrentFormat, ID: role.Objects[0].Object, Name: "Товары", Title: LocalizedText{"ru": "Товары"}, Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 150,
		Attributes: []Attribute{{ID: uuid.MustNew(), Name: "Цена", Title: LocalizedText{"ru": "Цена"}, Types: []Type{{Kind: NumberType, Precision: 12, Scale: 2}}}},
		TableParts: []TablePart{{ID: uuid.MustNew(), Name: "Детали", Title: LocalizedText{"ru": "Детали"}, Attributes: []Attribute{{ID: uuid.MustNew(), Name: "Текст", Title: LocalizedText{"ru": "Текст"}, Types: []Type{{Kind: StringType, Length: 100}}}}}}}
	for _, id := range []uuid.UUID{definition.Attributes[0].ID, definition.TableParts[0].ID, definition.TableParts[0].Attributes[0].ID} {
		role.Objects[0].Fields = append(role.Objects[0].Fields, FieldPermission{Field: id.String(), Operations: []PermissionOperation{PermissionRead}})
	}
	form := ManagedForm{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Форма", Title: LocalizedText{"ru": "Форма"}, Kind: ObjectForm,
		Commands: []ManagedFormCommand{{ID: uuid.MustNew(), Name: "Обновить", Title: LocalizedText{"ru": "Обновить"}, Action: FormCommandRefresh}}}
	role.Commands = []CommandPermission{{Form: form.ID, Command: form.Commands[0].ID}}
	catalog, err := NewCatalogSnapshotWithRoles(metadataConfiguration(), nil, nil, nil, []CatalogDefinition{definition}, nil, nil, nil, []RoleDefinition{role})
	if err != nil {
		t.Fatal(err)
	}
	return catalog, role, form
}

func TestRoleSnapshotIsolationAndReferences(t *testing.T) {
	t.Parallel()
	catalog, role, form := roleCatalogFixture(t)
	snapshot, err := NewRuntimeSnapshot(catalog, []ManagedForm{form})
	if err != nil {
		t.Fatal(err)
	}
	role.Objects[0].Fields[0].Operations[0] = PermissionUpdate
	lookedUp, ok := catalog.Role("ЧИТАТЕЛЬ")
	if !ok || lookedUp.Objects[0].Fields[0].Operations[0] != PermissionRead {
		t.Fatal("constructor did not isolate role fields")
	}
	lookedUp.Title["ru"] = "Изменено"
	lookedUp.Objects[0].Operations[0] = PermissionDelete
	lookedUp.Objects[0].Fields[0].Operations[0] = PermissionUpdate
	lookedUp.Commands[0].Command = uuid.MustNew()
	again, ok := catalog.RoleByID(role.ID)
	if !ok || again.Title["ru"] != "Читатель" || again.Objects[0].Operations[0] != PermissionRead || again.Objects[0].Fields[0].Operations[0] != PermissionRead || again.Commands[0].Command != form.Commands[0].ID {
		t.Fatal("lookup exposed mutable role storage")
	}
	if _, ok := catalog.Role("нет"); ok {
		t.Fatal("unknown role found")
	}
	if _, ok := catalog.RoleByID(uuid.MustNew()); ok {
		t.Fatal("unknown role UUID found")
	}
	catalog.Roles[0].Objects[0].Operations[0] = PermissionDelete
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var decoded RuntimeSnapshot
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatal(err)
	}
	loaded, err := decoded.Catalog()
	if err != nil || loaded.Roles[0].Objects[0].Operations[0] != PermissionRead {
		t.Fatalf("roundtrip lost or aliased role: %v", err)
	}
	loaded.Roles[0].Title["ru"] = "Изменено"
	if decoded.Roles[0].Title["ru"] != "Читатель" {
		t.Fatal("reconstructed catalog shares role storage")
	}
}

func TestRoleReferencesRejectInvalidRuntimeMetadata(t *testing.T) {
	t.Parallel()
	tests := map[string]func(*Catalog, *ManagedForm){
		"unknown object":    func(c *Catalog, _ *ManagedForm) { c.Roles[0].Objects[0].Object = uuid.MustNew() },
		"wrong field owner": func(c *Catalog, _ *ManagedForm) { c.Roles[0].Objects[0].Fields[0].Field = c.Roles[0].ID.String() },
		"deleted attribute": func(c *Catalog, _ *ManagedForm) { c.Catalogs[0].Attributes = nil },
		"unsupported operation": func(c *Catalog, _ *ManagedForm) {
			c.Roles[0].Objects[0].Operations = []PermissionOperation{PermissionPost}
		},
		"unknown standard field": func(c *Catalog, _ *ManagedForm) { c.Roles[0].Objects[0].Fields[0].Field = "password" },
		"read only field": func(c *Catalog, _ *ManagedForm) {
			c.Roles[0].Objects[0].Fields[0] = FieldPermission{Field: "ref", Operations: []PermissionOperation{PermissionUpdate}}
		},
		"duplicate role UUID": func(c *Catalog, _ *ManagedForm) {
			r := cloneRole(c.Roles[0])
			r.Name = "Другой"
			c.Roles = append(c.Roles, r)
		},
		"duplicate role name": func(c *Catalog, _ *ManagedForm) {
			r := cloneRole(c.Roles[0])
			r.ID = uuid.MustNew()
			r.Name = "ЧИТАТЕЛЬ"
			c.Roles = append(c.Roles, r)
		},
		"global UUID collision":  func(c *Catalog, _ *ManagedForm) { c.Roles[0].ID = c.Catalogs[0].Attributes[0].ID },
		"unknown form":           func(c *Catalog, _ *ManagedForm) { c.Roles[0].Commands[0].Form = uuid.MustNew() },
		"wrong command owner":    func(c *Catalog, _ *ManagedForm) { c.Roles[0].Commands[0].Command = uuid.MustNew() },
		"deleted command":        func(_ *Catalog, f *ManagedForm) { f.Commands = nil },
		"invalid JSON operation": func(c *Catalog, _ *ManagedForm) { c.Roles[0].Objects[0].Operations = []PermissionOperation{"admin"} },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			catalog, _, form := roleCatalogFixture(t)
			mutate(catalog, &form)
			if _, err := NewRuntimeSnapshot(catalog, []ManagedForm{form}); err == nil {
				t.Fatal("invalid role references accepted")
			}
		})
	}
	catalog, _, form := roleCatalogFixture(t)
	catalog.Catalogs[0].Name = "НовоеИмя"
	catalog.Catalogs[0].Attributes[0].Name = "Стоимость"
	if _, err := NewRuntimeSnapshot(catalog, []ManagedForm{form}); err != nil {
		t.Fatalf("rename should preserve role grants: %v", err)
	}
}

func TestLoadRolesAndCommandSources(t *testing.T) {
	t.Parallel()
	for _, broken := range []string{"", "role filename", "form folder", "missing form", "symlink form", "missing command", "unknown YAML field"} {
		t.Run(broken, func(t *testing.T) {
			root := metadataProject(t)
			catalog, role, form := roleCatalogFixture(t)
			write := func(relative string, value any) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(filepath.Join(root, relative)), 0o700); err != nil {
					t.Fatal(err)
				}
				var buffer bytes.Buffer
				if err := Encode(&buffer, value); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, relative), buffer.Bytes(), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			write("metadata/catalogs/"+catalog.Catalogs[0].Name+"/object.yaml", catalog.Catalogs[0])
			// A common form keeps a folder named after it, holding form.yaml.
			formPath := "metadata/common-forms/" + form.Name + "/" + project.FormMetadataFile
			if broken == "form folder" {
				formPath = "metadata/common-forms/ДругоеИмя/" + project.FormMetadataFile
			}
			if broken == "missing command" {
				form.Commands = nil
			}
			if broken != "missing form" {
				write(formPath, form)
			}
			if broken == "symlink form" {
				target := filepath.Join(t.TempDir(), "form.yaml")
				if err := os.Rename(filepath.Join(root, formPath), target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, filepath.Join(root, formPath)); err != nil {
					t.Fatal(err)
				}
			}
			rolePath := "metadata/roles/" + role.ID.String() + ".yaml"
			if broken == "role filename" {
				role.ID = uuid.MustNew()
			}
			write(rolePath, role)
			if broken == "unknown YAML field" {
				file, err := os.OpenFile(filepath.Join(root, rolePath), os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				_, err = file.WriteString("full_access: true\n")
				closeErr := file.Close()
				if err != nil || closeErr != nil {
					t.Fatalf("write: %v, close: %v", err, closeErr)
				}
			}
			loaded, err := Load(root)
			if broken == "" {
				if err != nil || len(loaded.Roles) != 1 {
					t.Fatalf("load role: %v", err)
				}
			} else if err == nil {
				t.Fatalf("accepted %s", broken)
			}
		})
	}
}

func TestRoleTargetsMatchObjectCapabilities(t *testing.T) {
	t.Parallel()
	catalog, role, _ := roleCatalogFixture(t)
	constant, enumeration, document, unpostable, independent, recorder, balance, turnover := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	attribute := func() Attribute {
		return Attribute{ID: uuid.MustNew(), Name: "Количество", Title: LocalizedText{"ru": "Количество"}, Types: []Type{{Kind: NumberType, Precision: 15, Scale: 3}}}
	}
	catalog.Constants = []Constant{{Format: CurrentFormat, ID: constant, Name: "Константа", Title: role.Title, Types: []Type{{Kind: BooleanType}}}}
	catalog.Enumerations = []Enumeration{{Format: CurrentFormat, ID: enumeration, Name: "Перечисление", Title: role.Title, Values: []EnumerationValue{{ID: uuid.MustNew(), Name: "Первый", Title: role.Title}}}}
	catalog.Documents = []DocumentDefinition{
		{Format: CurrentFormat, ID: document, Name: "Документ", Title: role.Title, Posting: true, Number: DocumentNumber{Type: StringType, Length: 9, Periodicity: NumberPeriodNone}},
		{Format: CurrentFormat, ID: unpostable, Name: "Непроводимый", Title: role.Title, Number: DocumentNumber{Type: StringType, Length: 9, Periodicity: NumberPeriodNone}},
	}
	catalog.InformationRegisters = []InformationRegisterDefinition{
		{Format: CurrentFormat, ID: independent, Name: "Независимый", Title: role.Title, WriteMode: InformationRegisterIndependent, Periodicity: InformationRegisterPeriodNone, Resources: []Attribute{attribute()}},
		{Format: CurrentFormat, ID: recorder, Name: "Подчиненный", Title: role.Title, WriteMode: InformationRegisterRecorder, Periodicity: InformationRegisterPeriodMonth, Recorders: []uuid.UUID{document}, Resources: []Attribute{attribute()}},
	}
	catalog.AccumulationRegisters = []AccumulationRegisterDefinition{
		{Format: CurrentFormat, ID: balance, Name: "Остатки", Title: role.Title, Kind: AccumulationRegisterBalance, Recorders: []uuid.UUID{document}, Resources: []Attribute{attribute()}},
		{Format: CurrentFormat, ID: turnover, Name: "Обороты", Title: role.Title, Kind: AccumulationRegisterTurnover, Recorders: []uuid.UUID{document}, Resources: []Attribute{attribute()}},
	}
	cases := []struct {
		name           string
		id             uuid.UUID
		operation      PermissionOperation
		field          string
		fieldOperation PermissionOperation
		valid          bool
	}{
		{"constant write", constant, PermissionUpdate, "value", PermissionUpdate, true},
		{"constant create", constant, PermissionCreate, "", "", false},
		{"enumeration read", enumeration, PermissionRead, "ref", PermissionRead, true},
		{"enumeration update", enumeration, PermissionUpdate, "", "", false},
		{"catalog update", catalog.Catalogs[0].ID, PermissionUpdate, "description", PermissionUpdate, true},
		{"document create", document, PermissionCreate, "number", PermissionUpdate, true},
		{"document delete", document, PermissionDelete, "", "", true},
		{"document post", document, PermissionPost, "posted", PermissionRead, true},
		{"document undo posting", document, PermissionUndoPosting, "", "", true},
		{"direct posted update", document, PermissionUpdate, "posted", PermissionUpdate, false},
		{"posting disabled", unpostable, PermissionPost, "", "", false},
		{"foreign attribute", document, PermissionRead, catalog.Catalogs[0].Attributes[0].ID.String(), PermissionRead, false},
		{"independent register", independent, PermissionUpdate, catalog.InformationRegisters[0].Resources[0].ID.String(), PermissionUpdate, true},
		{"nonperiodic register", independent, PermissionRead, "period", PermissionRead, false},
		{"register without recorder", independent, PermissionRead, "recorder", PermissionRead, false},
		{"register record id", independent, PermissionRead, "recordid", PermissionRead, true},
		{"register immutable id", independent, PermissionUpdate, "recordid", PermissionUpdate, false},
		{"recorder register", recorder, PermissionUpdate, "period", PermissionUpdate, true},
		{"register recorder", recorder, PermissionRead, "recorder", PermissionRead, true},
		{"register active", recorder, PermissionUpdate, "active", PermissionUpdate, true},
		{"register separate delete", recorder, PermissionDelete, "", "", false},
		{"accumulation movement", balance, PermissionUpdate, "movementkind", PermissionUpdate, true},
		{"accumulation resource", balance, PermissionRead, catalog.AccumulationRegisters[0].Resources[0].ID.String(), PermissionRead, true},
		{"turnover without movement", turnover, PermissionRead, "movementkind", PermissionRead, false},
		{"accumulation line", turnover, PermissionRead, "linenumber", PermissionRead, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			value := *catalog
			r := cloneRole(role)
			r.Commands = nil
			r.Objects = []ObjectPermission{{Object: test.id, Operations: []PermissionOperation{test.operation}}}
			if test.field != "" {
				r.Objects[0].Fields = []FieldPermission{{Field: test.field, Operations: []PermissionOperation{test.fieldOperation}}}
			}
			value.Roles = []RoleDefinition{r}
			_, err := NewRuntimeSnapshot(&value, nil)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v, error=%v", test.valid, err)
			}
		})
	}
}

func TestRuntimeRolesStableOrderAndValidation(t *testing.T) {
	t.Parallel()
	catalog, role, form := roleCatalogFixture(t)
	first, _ := uuid.Parse("10000000-0000-4000-8000-000000000001")
	second, _ := uuid.Parse("20000000-0000-4000-8000-000000000001")
	role.ID = second
	other := cloneRole(role)
	other.ID, other.Name = first, "Другой"
	catalog.Roles = []RoleDefinition{role, other}
	snapshot, err := NewRuntimeSnapshot(catalog, []ManagedForm{form})
	if err != nil || snapshot.Roles[0].ID != first || snapshot.Roles[1].ID != second {
		t.Fatalf("roles are not sorted: %v", err)
	}
	if err := snapshot.Validate(); err != nil {
		t.Fatal(err)
	}
	snapshot.Roles[0], snapshot.Roles[1] = snapshot.Roles[1], snapshot.Roles[0]
	if err := snapshot.Validate(); err == nil {
		t.Fatal("noncanonical role order accepted")
	}
	snapshot.Roles[0], snapshot.Roles[1] = snapshot.Roles[1], snapshot.Roles[0]
	snapshot.Roles[0].Commands[0].Command = uuid.MustNew()
	if _, err := snapshot.Catalog(); err == nil {
		t.Fatal("runtime catalog accepted unknown command")
	}
	if err := snapshot.Validate(); err == nil {
		t.Fatal("runtime validation accepted unknown command")
	}
}

func TestRoleAutoGrantDefaults(t *testing.T) {
	t.Parallel()
	objectID := uuid.MustNew()

	// No existing entry for the object at all: creates one with Read.
	role := RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "R", Title: LocalizedText{"ru": "R"}}
	if !role.GrantObjectReadByDefault(objectID) {
		t.Fatal("expected the first grant to change the role")
	}
	if len(role.Objects) != 1 || role.Objects[0].Object != objectID || !slices.Contains(role.Objects[0].Operations, PermissionRead) {
		t.Fatalf("role after first grant: %+v", role)
	}

	// Already has Read: idempotent, reports no change, no duplicate operation.
	if role.GrantObjectReadByDefault(objectID) {
		t.Fatal("expected the second grant to be a no-op")
	}
	if len(role.Objects[0].Operations) != 1 {
		t.Fatalf("operations duplicated: %+v", role.Objects[0].Operations)
	}

	// Has the object with a different operation only: Read is added alongside it.
	other := RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "R2", Title: LocalizedText{"ru": "R2"},
		Objects: []ObjectPermission{{Object: objectID, Operations: []PermissionOperation{PermissionUpdate}}}}
	if !other.GrantObjectReadByDefault(objectID) {
		t.Fatal("expected Read to be added alongside an existing different operation")
	}
	if !slices.Contains(other.Objects[0].Operations, PermissionUpdate) || !slices.Contains(other.Objects[0].Operations, PermissionRead) {
		t.Fatalf("operations after grant: %+v", other.Objects[0].Operations)
	}

	// GrantFieldReadByDefault never grants a field on an object the role has
	// no ObjectPermission entry for at all.
	fieldless := RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "R3", Title: LocalizedText{"ru": "R3"}}
	if fieldless.GrantFieldReadByDefault(objectID, "attribute") {
		t.Fatal("expected no field grant without an existing object entry")
	}
	if len(fieldless.Objects) != 0 {
		t.Fatalf("unexpected object entry created: %+v", fieldless.Objects)
	}

	// With an existing object entry, a new field is added with Read.
	if !role.GrantFieldReadByDefault(objectID, "attribute") {
		t.Fatal("expected the first field grant to change the role")
	}
	if len(role.Objects[0].Fields) != 1 || role.Objects[0].Fields[0].Field != "attribute" {
		t.Fatalf("role fields after grant: %+v", role.Objects[0].Fields)
	}

	// Idempotent: field already has Read.
	if role.GrantFieldReadByDefault(objectID, "attribute") {
		t.Fatal("expected the second field grant to be a no-op")
	}
	if len(role.Objects[0].Fields[0].Operations) != 1 {
		t.Fatalf("field operations duplicated: %+v", role.Objects[0].Fields[0].Operations)
	}
}

func TestRoleCommentLimit(t *testing.T) {
	t.Parallel()
	configuration := project.Project{Format: 1, ID: uuid.MustNew(), Name: "P", Title: project.LocalizedText{"ru": "P"}, DefaultLanguage: "ru", Languages: []project.Language{{ID: uuid.MustNew(), Name: "Русский", Title: "Русский", Code: "ru"}}}
	role := RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "R", Title: LocalizedText{"ru": "R"}, Comment: strings.Repeat("a", MaxRoleComment)}
	if err := ValidateRole("role", role, configuration); err != nil {
		t.Fatalf("comment at the limit rejected: %v", err)
	}
	role.Comment += "a"
	if err := ValidateRole("role", role, configuration); err == nil {
		t.Fatal("comment over the limit accepted")
	}
}
