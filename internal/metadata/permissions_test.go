package metadata

import (
	"errors"
	"sync"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestPermissionsDenyByDefault(t *testing.T) {
	t.Parallel()
	catalog, role, form := roleCatalogFixture(t)
	empty, err := CompilePermissions(catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, policy := range []*Permissions{nil, {}, empty} {
		for _, operation := range []PermissionOperation{PermissionRead, PermissionCreate, PermissionUpdate, PermissionDelete, PermissionPost, PermissionUndoPosting, "admin", ""} {
			if policy.AllowsObject(role.Objects[0].Object, operation) || policy.AllowsFields(role.Objects[0].Object, operation) {
				t.Fatalf("implicit object grant: %s", operation)
			}
			if !errors.Is(policy.RequireFields(role.Objects[0].Object, operation, "description"), ErrPermissionDenied) {
				t.Fatal("missing policy allowed a field")
			}
		}
		if !errors.Is(policy.RequireCommand(form.ID, form.Commands[0].ID), ErrPermissionDenied) {
			t.Fatal("implicit command grant")
		}
	}
	if empty.ProjectID() != catalog.Project.ID {
		t.Fatal("missing project binding")
	}
	withoutRoles, err := NewCatalogSnapshot(metadataManifest(), nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := CompilePermissions(withoutRoles, nil)
	if err != nil || policy.AllowsObject(role.Objects[0].Object, PermissionRead) {
		t.Fatalf("role-free project granted access: %v", err)
	}
}

func TestPermissionsUnionAndIsolation(t *testing.T) {
	catalog, reader, form := roleCatalogFixture(t)
	object := reader.Objects[0].Object
	writer := RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Создатель", Title: LocalizedText{"ru": "Создатель"},
		Objects: []ObjectPermission{{Object: object, Operations: []PermissionOperation{PermissionCreate}, Fields: []FieldPermission{{Field: "description", Operations: []PermissionOperation{PermissionUpdate}}}}}}
	catalog.Roles = append(catalog.Roles, writer)
	if err := catalog.indexAndValidate(""); err != nil {
		t.Fatal(err)
	}
	ids := []uuid.UUID{reader.ID, writer.ID}
	policy, err := CompilePermissions(catalog, ids)
	if err != nil {
		t.Fatal(err)
	}
	if !policy.AllowsFields(object, PermissionRead, "description") || !policy.AllowsFields(object, PermissionCreate, "description") {
		t.Fatal("explicit grants did not combine")
	}
	if policy.AllowsFields(object, PermissionUpdate, "description") || policy.AllowsFields(object, PermissionCreate, "ref") || policy.AllowsFields(object, PermissionRead, "Description") {
		t.Fatal("operation or field identity was widened")
	}
	if !policy.AllowsCommand(form.ID, form.Commands[0].ID) || policy.AllowsCommand(uuid.MustNew(), form.Commands[0].ID) {
		t.Fatal("command was not scoped to its form")
	}
	ids[0] = uuid.MustNew()
	catalog.Roles[0].Objects[0].Operations = nil
	catalog.Roles[0].Objects[0].Fields[0].Operations = nil
	catalog.Roles[0].Commands = nil
	if !policy.AllowsFields(object, PermissionRead, "description") || !policy.AllowsCommand(form.ID, form.Commands[0].ID) {
		t.Fatal("compiled permissions retained mutable source data")
	}
	var readers sync.WaitGroup
	for range 8 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for range 1000 {
				if policy.RequireFields(object, PermissionRead, "description") != nil || policy.RequireObject(object, PermissionDelete) == nil {
					t.Error("concurrent permission lookup changed")
					return
				}
			}
		}()
	}
	readers.Wait()
	if allocations := testing.AllocsPerRun(100, func() { _ = policy.AllowsFields(object, PermissionRead, "description") }); allocations != 0 {
		t.Fatalf("permission lookup allocates: %g", allocations)
	}
}

func TestPermissionsObjectFieldAndCommandAreIndependent(t *testing.T) {
	t.Parallel()
	catalog, role, form := roleCatalogFixture(t)
	object := role.Objects[0].Object
	part := catalog.Catalogs[0].TableParts[0]
	column := part.Attributes[0].ID.String()
	catalog.Roles[0].Objects[0].Operations = nil
	if err := catalog.indexAndValidate(""); err != nil {
		t.Fatal(err)
	}
	fieldOnly, err := CompilePermissions(catalog, []uuid.UUID{role.ID})
	if err != nil {
		t.Fatal(err)
	}
	if fieldOnly.AllowsFields(object, PermissionRead, column) || !fieldOnly.AllowsCommand(form.ID, form.Commands[0].ID) {
		t.Fatal("field/command access implicitly granted object read")
	}
	catalog.Roles[0].Objects[0] = ObjectPermission{Object: object, Operations: []PermissionOperation{PermissionRead}, Fields: []FieldPermission{{Field: column, Operations: []PermissionOperation{PermissionRead}}}}
	if err := catalog.indexAndValidate(""); err != nil {
		t.Fatal(err)
	}
	columnOnly, err := CompilePermissions(catalog, []uuid.UUID{role.ID})
	if err != nil {
		t.Fatal(err)
	}
	if columnOnly.AllowsFields(object, PermissionRead, column) || columnOnly.AllowsFields(object, PermissionRead, part.ID.String()) {
		t.Fatal("column access bypassed table part grant")
	}
	if !columnOnly.AllowsObject(object, PermissionRead) || columnOnly.AllowsFields(object, PermissionRead, "ref") {
		t.Fatal("object operation implicitly granted standard fields")
	}
}

func TestPermissionsRejectInvalidSelection(t *testing.T) {
	t.Parallel()
	catalog, role, _ := roleCatalogFixture(t)
	for _, ids := range [][]uuid.UUID{{uuid.UUID{}}, {uuid.MustNew()}, {role.ID, uuid.MustNew()}, {role.ID, role.ID}, make([]uuid.UUID, MaxAssignedRoles+1)} {
		if policy, err := CompilePermissions(catalog, ids); !errors.Is(err, ErrInvalidRoleSelection) || policy != nil {
			t.Fatalf("invalid selection yielded a usable policy: %v", err)
		}
	}
	if _, err := CompilePermissions(nil, nil); !errors.Is(err, ErrInvalidRoleSelection) {
		t.Fatalf("nil catalog: %v", err)
	}
}

func TestPermissionsBoundCombinedRoleSize(t *testing.T) {
	catalog, role, form := roleCatalogFixture(t)
	ids := []uuid.UUID{}
	catalog.Roles = nil
	for index := 0; index <= MaxEffectivePermissions/MaxRolePermissions; index++ {
		next := cloneRole(role)
		next.ID, next.Name = uuid.MustNew(), "Роль"+string(rune('А'+index))
		next.Objects = nil
		next.Commands = make([]CommandPermission, MaxRolePermissions)
		for position := range next.Commands {
			next.Commands[position] = CommandPermission{Form: form.ID, Command: uuid.MustNew()}
		}
		catalog.Roles = append(catalog.Roles, next)
		ids = append(ids, next.ID)
	}
	// Catalog construction validates structure; runtime-snapshot validation is
	// separately responsible for resolving every command against form sources.
	if err := catalog.indexAndValidate(""); err != nil {
		t.Fatal(err)
	}
	if policy, err := CompilePermissions(catalog, ids); !errors.Is(err, ErrInvalidRoleSelection) || policy != nil {
		t.Fatalf("unbounded combined roles: %v", err)
	}
}
