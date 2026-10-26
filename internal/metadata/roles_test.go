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
			Fields: []FieldPermission{{Field: "description", Operations: []PermissionOperation{PermissionView}}}}}}
}

func TestDecodeRoleStrictAndBounded(t *testing.T) {
	t.Parallel()
	role := roleFixture()
	description, grid, err := EncodeRole(role)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeRole("role.yaml", bytes.NewReader(description), bytes.NewReader(grid), metadataConfiguration())
	if err != nil || decoded.ID != role.ID {
		t.Fatalf("decode role = %+v, %v", decoded, err)
	}
	for _, suffix := range []string{"unknown: true\n", "---\nformat: 1\n"} {
		if _, err := DecodeRole("role.yaml", strings.NewReader(string(description)+suffix), bytes.NewReader(grid), metadataConfiguration()); err == nil {
			t.Fatalf("accepted invalid suffix %q", suffix)
		}
	}
	tests := map[string]func(*RoleDefinition){
		"format":            func(r *RoleDefinition) { r.Format++ },
		"zero identity":     func(r *RoleDefinition) { r.ID = uuid.UUID{} },
		"name":              func(r *RoleDefinition) { r.Name = "Invalid Name" },
		"language":          func(r *RoleDefinition) { r.Title = LocalizedText{"d=e": "Leser"} },
		"zero object":       func(r *RoleDefinition) { r.Objects[0].Object = uuid.UUID{} },
		"duplicate object":  func(r *RoleDefinition) { r.Objects = append(r.Objects, r.Objects[0]) },
		"unknown operation": func(r *RoleDefinition) { r.Objects[0].Operations = []PermissionOperation{"admin"} },
		"duplicate operation": func(r *RoleDefinition) {
			r.Objects[0].Operations = []PermissionOperation{PermissionRead, PermissionRead}
		},
		"unknown field right": func(r *RoleDefinition) { r.Objects[0].Fields[0].Operations = []PermissionOperation{"admin"} },
		"unknown denial":      func(r *RoleDefinition) { r.Objects[0].Denied = []PermissionOperation{"admin"} },
		"granted and denied": func(r *RoleDefinition) {
			r.Objects[0].Denied = []PermissionOperation{PermissionRead}
		},
		"field granted and denied": func(r *RoleDefinition) {
			r.Objects[0].Fields[0].Denied = []PermissionOperation{PermissionView}
		},
		"duplicate denial": func(r *RoleDefinition) {
			r.Objects[0].Denied = []PermissionOperation{PermissionDelete, PermissionDelete}
		},
		"two dots":                func(r *RoleDefinition) { r.Objects[0].Fields[0].Field = "extdimensiontypes.turnoversonly.x" },
		"empty field operations":  func(r *RoleDefinition) { r.Objects[0].Fields[0].Operations = nil },
		"duplicate field":         func(r *RoleDefinition) { r.Objects[0].Fields = append(r.Objects[0].Fields, r.Objects[0].Fields[0]) },
		"empty field":             func(r *RoleDefinition) { r.Objects[0].Fields[0].Field = "" },
		"zero field UUID":         func(r *RoleDefinition) { r.Objects[0].Fields[0].Field = "00000000-0000-0000-0000-000000000000" },
		"noncanonical field UUID": func(r *RoleDefinition) { r.Objects[0].Fields[0].Field = "ABCDEFAB-0000-4000-8000-000000000001" },
		"field alias":             func(r *RoleDefinition) { r.Objects[0].Fields[0].Field = "Наименование" },
		"zero command":            func(r *RoleDefinition) { r.Commands = []CommandPermission{{Form: uuid.MustNew()}} },
		"zero form":               func(r *RoleDefinition) { r.Commands = []CommandPermission{{Command: uuid.MustNew()}} },
		// A template is a name with the names of its parameters in brackets
		// (validateRestrictionTemplates), one of a name in a role whatever
		// its case.
		"template parameter": func(r *RoleDefinition) {
			r.RestrictionTemplates = []RestrictionTemplate{{Name: "Шаблон(Поле 1)"}}
		},
		"template bracket": func(r *RoleDefinition) {
			r.RestrictionTemplates = []RestrictionTemplate{{Name: "Шаблон(Поле"}}
		},
		"template name": func(r *RoleDefinition) { r.RestrictionTemplates = []RestrictionTemplate{{Name: "1Шаблон"}} },
		"template twice": func(r *RoleDefinition) {
			r.RestrictionTemplates = []RestrictionTemplate{{Name: "Шаблон"}, {Name: "шаблон(Поле)"}}
		},
		"duplicate command": func(r *RoleDefinition) {
			c := CommandPermission{Form: uuid.MustNew(), Command: uuid.MustNew()}
			r.Commands = []CommandPermission{c, c}
		},
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
	templated := cloneRole(role)
	templated.RestrictionTemplates = []RestrictionTemplate{{Name: "Шаблон"}, {Name: "ПоПолю( Поле , Пользователь )", Condition: "ГДЕ #Поле = &Пользователь"}}
	if err := ValidateRole("templated-role.yaml", templated, metadataConfiguration()); err != nil {
		t.Fatalf("templates of names and parameters were refused: %v", err)
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
		role.Objects[0].Fields = append(role.Objects[0].Fields, FieldPermission{Field: id.String(), Operations: []PermissionOperation{PermissionView}})
	}
	// A denial on the object and on a part: the three states travel through
	// every copy of a role, not only the grants.
	role.Objects[0].Denied = []PermissionOperation{PermissionDelete}
	role.Objects[0].Fields[1].Denied = []PermissionOperation{PermissionEdit}
	form := ManagedForm{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Форма", Title: LocalizedText{"ru": "Форма"}, Kind: ObjectForm,
		Commands: []ManagedFormCommand{{ID: uuid.MustNew(), Name: "Обновить", Title: LocalizedText{"ru": "Обновить"}, Action: FormCommandRefresh}}}
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
	role.Objects[0].Fields[0].Operations[0] = PermissionEdit
	lookedUp, ok := catalog.Role("ЧИТАТЕЛЬ")
	if !ok || lookedUp.Objects[0].Fields[0].Operations[0] != PermissionView {
		t.Fatal("constructor did not isolate role fields")
	}
	lookedUp.Title["ru"] = "Изменено"
	lookedUp.Objects[0].Operations[0] = PermissionDelete
	lookedUp.Objects[0].Fields[0].Operations[0] = PermissionUpdate
	lookedUp.Objects[0].Denied[0] = PermissionInsert
	lookedUp.Objects[0].Fields[1].Denied[0] = PermissionView
	again, ok := catalog.RoleByID(role.ID)
	if !ok || again.Title["ru"] != "Читатель" || again.Objects[0].Operations[0] != PermissionRead || again.Objects[0].Fields[0].Operations[0] != PermissionView ||
		again.Objects[0].Denied[0] != PermissionDelete || again.Objects[0].Fields[1].Denied[0] != PermissionEdit {
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
		"unknown object":         func(c *Catalog, _ *ManagedForm) { c.Roles[0].Objects[0].Object = uuid.MustNew() },
		"wrong field owner":      func(c *Catalog, _ *ManagedForm) { c.Roles[0].Objects[0].Fields[0].Field = c.Roles[0].ID.String() },
		"deleted attribute":      func(c *Catalog, _ *ManagedForm) { c.Catalogs[0].Attributes = nil },
		"unknown standard field": func(c *Catalog, _ *ManagedForm) { c.Roles[0].Objects[0].Fields[0].Field = "password" },
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
		"global UUID collision": func(c *Catalog, _ *ManagedForm) { c.Roles[0].ID = c.Catalogs[0].Attributes[0].ID },
		// A role gives no right on a command of a form, not even on one the
		// form declares (ACCESS-RIGHTS, «Команды формы»).
		"a command of a form": func(c *Catalog, f *ManagedForm) {
			c.Roles[0].Commands = []CommandPermission{{Form: f.ID, Command: f.Commands[0].ID}}
		},
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
	// What the prototype's roles keep is not refused: posting on a catalog,
	// editing the reference.
	for name, mutate := range map[string]func(*Catalog){
		"a right the object does not use": func(c *Catalog) { c.Roles[0].Objects[0].Operations = []PermissionOperation{PermissionPosting} },
		"editing a standard field the runtime never writes": func(c *Catalog) {
			c.Roles[0].Objects[0].Fields[0] = FieldPermission{Field: "ref", Operations: []PermissionOperation{PermissionEdit}}
		},
	} {
		catalog, _, form := roleCatalogFixture(t)
		mutate(catalog)
		if _, err := NewRuntimeSnapshot(catalog, []ManagedForm{form}); err != nil {
			t.Fatalf("%s was refused: %v", name, err)
		}
	}
	catalog, _, form := roleCatalogFixture(t)
	catalog.Catalogs[0].Name = "НовоеИмя"
	catalog.Catalogs[0].Attributes[0].Name = "Стоимость"
	if _, err := NewRuntimeSnapshot(catalog, []ManagedForm{form}); err != nil {
		t.Fatalf("rename should preserve role grants: %v", err)
	}
}

func TestLoadRolesAndCommonFormSources(t *testing.T) {
	t.Parallel()
	for _, broken := range []string{"", "role filename", "form folder", "symlink form", "unknown YAML field"} {
		t.Run(broken, func(t *testing.T) {
			root := metadataProject(t)
			catalog, role, form := roleCatalogFixture(t)
			write := func(relative string, value any) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(filepath.Join(root, relative)), 0o700); err != nil {
					t.Fatal(err)
				}
				var buffer bytes.Buffer
				if role, ok := value.(RoleDefinition); ok {
					// Written by hand rather than by WriteRole: the case of
					// the wrong file name puts the role where its identifier
					// does not say.
					description, grid, err := EncodeRole(role)
					if err != nil {
						t.Fatal(err)
					}
					buffer.Write(description)
					if grid != nil {
						writeRoleGrid(t, root, strings.TrimSuffix(filepath.Base(relative), ".yaml"), string(grid))
					}
				} else if err := Encode(&buffer, value); err != nil {
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
			write(formPath, form)
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

// A role carries any of the prototype's rights on any object, as the
// prototype's roles do - posting on a document that is never posted - and
// refuses only a part the object does not have under the prototype's
// composition: a field of another object, a standard field the kind has not,
// a key of this platform's own that names no part at all.
func TestRoleRightsAreCarriedAndPartsFollowThePrototype(t *testing.T) {
	t.Parallel()
	catalog, role, _ := roleCatalogFixture(t)
	constant, enumeration, document, unpostable, independent, recorder, balance, turnover := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	attribute := func() Attribute {
		return Attribute{ID: uuid.MustNew(), Name: "Количество", Title: LocalizedText{"ru": "Количество"}, Types: []Type{{Kind: NumberType, Precision: 15, Scale: 3}}}
	}
	catalog.Constants = []Constant{{Format: CurrentFormat, ID: constant, Name: "Константа", Title: role.Title, Types: []Type{{Kind: BooleanType}}}}
	catalog.Enumerations = []Enumeration{{Format: CurrentFormat, ID: enumeration, Name: "Перечисление", Title: role.Title, Values: []EnumerationValue{{ID: uuid.MustNew(), Name: "Первый", Title: role.Title}}}}
	catalog.Documents = []DocumentDefinition{
		{Format: CurrentFormat, ID: document, Name: "Документ", Title: role.Title, Posting: DocumentPosting{Allowed: true}, Number: DocumentNumber{Type: StringType, Length: 9, Periodicity: NumberPeriodNone},
			Movements: []uuid.UUID{recorder, balance, turnover}},
		{Format: CurrentFormat, ID: unpostable, Name: "Непроводимый", Title: role.Title, Number: DocumentNumber{Type: StringType, Length: 9, Periodicity: NumberPeriodNone}},
	}
	catalog.InformationRegisters = []InformationRegisterDefinition{
		{Format: CurrentFormat, ID: independent, Name: "Независимый", Title: role.Title, WriteMode: InformationRegisterIndependent, Periodicity: InformationRegisterPeriodNone, Resources: []Attribute{attribute()}},
		{Format: CurrentFormat, ID: recorder, Name: "Подчиненный", Title: role.Title, WriteMode: InformationRegisterRecorder, Periodicity: InformationRegisterPeriodMonth, Resources: []Attribute{attribute()}},
	}
	catalog.AccumulationRegisters = []AccumulationRegisterDefinition{
		{Format: CurrentFormat, ID: balance, Name: "Остатки", Title: role.Title, Kind: AccumulationRegisterBalance, Resources: []Attribute{attribute()}},
		{Format: CurrentFormat, ID: turnover, Name: "Обороты", Title: role.Title, Kind: AccumulationRegisterTurnover, Resources: []Attribute{attribute()}},
	}
	cases := []struct {
		name           string
		id             uuid.UUID
		operation      PermissionOperation
		field          string
		fieldOperation PermissionOperation
		valid          bool
	}{
		{"constant write", constant, PermissionUpdate, "", "", true},
		{"constant insert is carried", constant, PermissionInsert, "", "", true},
		{"constant has no parts", constant, PermissionRead, "value", PermissionView, false},
		{"enumeration read", enumeration, PermissionRead, "ref", PermissionView, true},
		{"enumeration update is carried", enumeration, PermissionUpdate, "", "", true},
		{"catalog update", catalog.Catalogs[0].ID, PermissionUpdate, "description", PermissionEdit, true},
		{"catalog owner", catalog.Catalogs[0].ID, PermissionRead, "owner", PermissionView, true},
		{"document insert", document, PermissionInsert, "number", PermissionEdit, true},
		{"document posting", document, PermissionPosting, "posted", PermissionView, true},
		{"posted edited is carried", document, PermissionUpdate, "posted", PermissionEdit, true},
		{"posting a document never posted is carried", unpostable, PermissionPosting, "", "", true},
		{"interactive right", document, PermissionInteractivePostingRegular, "", "", true},
		{"foreign attribute", document, PermissionRead, catalog.Catalogs[0].Attributes[0].ID.String(), PermissionView, false},
		{"independent register", independent, PermissionUpdate, catalog.InformationRegisters[0].Resources[0].ID.String(), PermissionEdit, true},
		{"every information register has a period", independent, PermissionRead, "period", PermissionView, true},
		{"and a recorder", independent, PermissionRead, "recorder", PermissionView, true},
		{"record id is no part", independent, PermissionRead, "recordid", PermissionView, false},
		{"totals of an information register are carried", recorder, PermissionTotalsControl, "", "", true},
		{"register delete is carried", recorder, PermissionDelete, "", "", true},
		{"record type", balance, PermissionUpdate, "recordtype", PermissionEdit, true},
		{"our old key for it is no part", balance, PermissionUpdate, "movementkind", PermissionEdit, false},
		{"turnovers have no record type", turnover, PermissionRead, "recordtype", PermissionView, false},
		{"accumulation resource", balance, PermissionRead, catalog.AccumulationRegisters[0].Resources[0].ID.String(), PermissionView, true},
		{"accumulation line", turnover, PermissionRead, "linenumber", PermissionView, true},
		{"not a right", document, "admin", "", "", false},
		{"not a right on a part", document, PermissionRead, "number", "admin", false},
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
	snapshot.Roles[0].Commands = []CommandPermission{{Form: form.ID, Command: form.Commands[0].ID}}
	if _, err := snapshot.Catalog(); err == nil {
		t.Fatal("runtime catalog accepted a right on a command of a form")
	}
	if err := snapshot.Validate(); err == nil {
		t.Fatal("runtime validation accepted a right on a command of a form")
	}
}

func TestRoleAutoGrantDefaults(t *testing.T) {
	t.Parallel()
	objectID := uuid.MustNew()

	// No existing entry for the object at all: creates one with Read.
	role := RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "R", Title: LocalizedText{"ru": "R"}}
	if !role.GrantObjectReadByDefault(CatalogKind, objectID) {
		t.Fatal("expected the first grant to change the role")
	}
	if len(role.Objects) != 1 || role.Objects[0].Object != objectID || !slices.Contains(role.Objects[0].Operations, PermissionRead) {
		t.Fatalf("role after first grant: %+v", role)
	}

	// Already has Read: idempotent, reports no change, no duplicate operation.
	if role.GrantObjectReadByDefault(CatalogKind, objectID) {
		t.Fatal("expected the second grant to be a no-op")
	}
	if len(role.Objects[0].Operations) != 1 {
		t.Fatalf("operations duplicated: %+v", role.Objects[0].Operations)
	}

	// Has the object with a different operation only: Read is added alongside it.
	other := RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "R2", Title: LocalizedText{"ru": "R2"},
		Objects: []ObjectPermission{{Object: objectID, Operations: []PermissionOperation{PermissionUpdate}}}}
	if !other.GrantObjectReadByDefault(CatalogKind, objectID) {
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

// A comment of a role is text, and the prototype puts no ceiling on it: a long
// one is carried. What is still refused is a comment that is not text at all.
func TestRoleCommentIsCarriedWhateverItsLength(t *testing.T) {
	t.Parallel()
	configuration := project.Project{Format: 1, ID: uuid.MustNew(), Name: "P", Title: project.LocalizedText{"ru": "P"}, DefaultLanguage: "ru", Languages: []project.Language{{ID: uuid.MustNew(), Name: "Русский", Title: project.LocalizedText{"ru": "Русский"}, Code: "ru"}}}
	role := RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "R", Title: LocalizedText{"ru": "R"}, Comment: strings.Repeat("a", 20_000)}
	if err := ValidateRole("role", role, configuration); err != nil {
		t.Fatalf("a long comment was refused: %v", err)
	}
	role.Comment = "\xff"
	if err := ValidateRole("role", role, configuration); err == nil {
		t.Fatal("a comment that is not UTF-8 was accepted")
	}
}

// A standard field key is lowercase Latin from a to z, both ends included:
// the structural check must not refuse a key the object may well have, and
// must refuse one that no object can have.
func TestPermissionFieldKeyIsLowercaseLatin(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"a", "z", "zone", "predefineddataname"} {
		if !validPermissionField(key) {
			t.Fatalf("key %q refused", key)
		}
	}
	for _, key := range []string{"", "Code", "код", "line_number", "{", "`", uuid.UUID{}.String(), strings.ToUpper(uuid.MustNew().String())} {
		if validPermissionField(key) {
			t.Fatalf("key %q accepted", key)
		}
	}
}

// The role editor shows standard fields by their keys and the attributes by
// their names, and lists the configuration first and then the objects kind by
// kind in the order of the configuration tree, by name without regard to
// case, so the same project always shows the same list.
func TestPermissionSchemaShowsStandardFieldsInOrder(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	goods, services, setting, order := uuid.MustNew().String(), uuid.MustNew().String(), uuid.MustNew().String(), uuid.MustNew().String()
	price := uuid.MustNew().String()
	writeMetadata(t, root, CatalogKind, goods, "format: 1\nid: "+goods+"\nname: товары\ntitle: {ru: Товары}\n"+
		"code: {type: string, length: 9}\ndescription_length: 150\n"+
		"attributes:\n  - {id: "+price+", name: Цена, title: {ru: Цена}, types: [{kind: number, precision: 10, scale: 2}]}\n")
	writeMetadata(t, root, CatalogKind, services, "format: 1\nid: "+services+"\nname: Услуги\ntitle: {ru: Услуги}\n"+
		"code: {type: string, length: 9}\ndescription_length: 150\n")
	writeMetadata(t, root, ConstantKind, setting, "format: 1\nid: "+setting+"\nname: Режим\ntitle: {ru: Режим}\ntypes: [{kind: boolean}]\n")
	writeMetadata(t, root, DocumentKind, order, "format: 1\nid: "+order+"\nname: Заказ\ntitle: {ru: Заказ}\n"+
		"number: {type: string, length: 9, periodicity: none}\n")
	user := uuid.MustNew().String()
	writeMetadata(t, root, SessionParameterKind, user, "format: 1\nid: "+user+"\nname: Пользователь\ntitle: {ru: Пользователь}\n"+
		"types: [{kind: string, length: 10}]\n")
	schema, err := LoadPermissionSchema(root)
	if err != nil {
		t.Fatal(err)
	}
	var listed []string
	for _, object := range schema.Objects {
		listed = append(listed, string(object.Kind)+"/"+object.Name)
	}
	want := []string{"/MetadataTest", "session-parameters/Пользователь", "constants/Режим", "catalogs/товары",
		"catalogs/Услуги", "documents/Заказ"}
	if !slices.Equal(listed, want) {
		t.Fatalf("objects listed as %v, want %v", listed, want)
	}
	// Each object carries the rights the configurator offers on its kind, the
	// configuration its own; written out here from the exports, as the
	// editor must show them.
	for index, rights := range map[int][]PermissionOperation{
		0: {PermissionAdministration, PermissionDataAdministration, PermissionExclusiveMode},
		1: {PermissionGet, PermissionSet},
		2: {PermissionRead, PermissionUpdate, PermissionView, PermissionEdit},
		3: {PermissionInputByString, PermissionInteractiveDeletePredefinedData},
		5: {PermissionPosting, PermissionInteractiveChangeOfPosted},
	} {
		object := schema.Objects[index]
		for _, right := range rights {
			if !slices.Contains(object.Rights, right) {
				t.Errorf("%s %s is not offered %s: %v", object.Kind, object.Name, right, object.Rights)
			}
		}
		if (index == 0) != object.Root {
			t.Errorf("%s %s is root: %v", object.Kind, object.Name, object.Root)
		}
	}
	// The rights of the history of data ML does not keep (ACCESS-RIGHTS,
	// «Права истории данных»).
	if slices.Contains(schema.Objects[2].Rights, PermissionInputByString) || slices.Contains(schema.Objects[0].Rights, PermissionRead) ||
		slices.Contains(schema.Objects[3].Rights, "read-data-history") {
		t.Errorf("a right is offered where the configurator offers none: %v, %v", schema.Objects[2].Rights, schema.Objects[0].Rights)
	}
	var keys []string
	for _, field := range schema.Objects[3].Fields {
		if field.Key == price {
			if field.Name != "Цена" {
				t.Fatalf("an attribute is shown as %q, not by its name", field.Name)
			}
			continue
		}
		keys = append(keys, field.Key+"="+field.Name+"/"+field.Title["ru"])
	}
	// The prototype's composition in its own order, keyed by the English
	// name in lower case and shown in words.
	if !slices.Equal(keys, []string{"ref=Ссылка/Ссылка", "deletionmark=ПометкаУдаления/Пометка удаления", "code=Код/Код",
		"description=Наименование/Наименование", "predefined=Предопределенный/Предопределенный",
		"predefineddataname=ИмяПредопределенныхДанных/Имя предопределенных данных", "parent=Родитель/Родитель",
		"isfolder=ЭтоГруппа/Это группа", "owner=Владелец/Владелец"}) {
		t.Fatalf("standard fields of a catalog shown as %v", keys)
	}
}

// The role editor checks one role against the project as it is. A role that
// names an object the project does not have must be refused there, before it
// is saved, and not only when the whole project is published.
func TestProjectRoleIsCheckedAgainstTheProject(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	role := roleFixture()
	err := ValidateProjectRole(root, role)
	if err == nil || !strings.Contains(err.Error(), "references unknown or unsupported object "+role.Objects[0].Object.String()) {
		t.Fatalf("a role naming an object the project lacks: %v", err)
	}
}

// The role editor checks one role without the others: a role another
// developer left naming an object that is gone does not keep the role being
// edited from being saved (ValidateProjectRole), and is repaired in its own
// window. Defect caught: the role checked together with every role of the
// project, the broken one refusing it.
func TestAProjectRoleIsCheckedWithoutTheOtherRoles(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	broken := roleFixture()
	if err := WriteRole(root, broken); err != nil {
		t.Fatal(err)
	}
	edited := roleFixture()
	edited.Name, edited.Objects = "Администратор", []ObjectPermission{{Object: catalog.Project.ID,
		Operations: []PermissionOperation{PermissionAdministration}}}
	if err := ValidateProjectRole(root, edited); err != nil {
		t.Fatalf("a role was refused for another role naming an object that is gone: %v", err)
	}
}

// A standard field key has no length ceiling of its own: a key the object
// does not have is refused by the object, whatever its length, and with words
// that say so.
func TestLongFieldKeyIsRefusedByTheObject(t *testing.T) {
	t.Parallel()
	catalog, role, _ := roleCatalogFixture(t)
	value := *catalog
	r := cloneRole(role)
	r.Commands = nil
	key := strings.Repeat("x", 40)
	r.Objects[0].Fields = []FieldPermission{{Field: key, Operations: []PermissionOperation{PermissionView}}}
	value.Roles = []RoleDefinition{r}
	_, err := NewRuntimeSnapshot(&value, nil)
	if err == nil || !strings.Contains(err.Error(), "field "+key+" does not belong to object") {
		t.Fatalf("a long key the object lacks: %v", err)
	}
}

// writeRoleGrid puts the grid of rights beside a role's description, where the
// loader reads it.
func writeRoleGrid(t *testing.T, root, roleID, grid string) {
	t.Helper()
	directory := filepath.Join(root, "metadata", "roles", roleID)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, RoleRightsFile), []byte(grid), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The standard fields of an accounting register are numbered - the first
// subconto, its kind - and acc grants rights on them in its roles.
//
// Defect caught: a key of letters only, which refused every role of acc that
// names a subconto.
func TestStandardKeysMayNumberTheirFields(t *testing.T) {
	t.Parallel()
	for key, valid := range map[string]bool{"extdimension1": true, "extdimensiontype3": true,
		"extdimensiontypes.turnoversonly": true, "1extdimension": false, "ext-dimension": false, "": false} {
		if validPermissionField(key) != valid {
			t.Errorf("key %q: valid %v, want %v", key, !valid, valid)
		}
	}
}
