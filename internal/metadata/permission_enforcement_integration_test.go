package metadata

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// TestApplicationRoleEnforcementIntegration proves that a context carrying a
// compiled Permissions policy is actually enforced by the metadata Runtime for
// objects, fields, writes and queries, while a context without one (Studio,
// CLI, plain tests) keeps running unrestricted.
func TestApplicationRoleEnforcementIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := integrationDatabase(t, ctx, 0)
	projectID := uuid.MustNew()

	catalogID, priceID := uuid.MustNew(), uuid.MustNew()
	readerRoleID, editorRoleID := uuid.MustNew(), uuid.MustNew()
	catalogDefinition := CatalogDefinition{
		ID: catalogID, Name: "Товары", Code: CatalogCode{Type: StringType, Length: 9, Unique: true}, DescriptionLength: 100,
		Attributes: []Attribute{{ID: priceID, Name: "Цена", Types: []Type{{Kind: NumberType, Precision: 15, Scale: 2}}}},
	}
	readerRole := RoleDefinition{
		ID: readerRoleID, Name: "ЧитательБезЦены", Title: LocalizedText{"ru": "Читатель без цены"},
		Objects: []ObjectPermission{{
			Object: catalogID, Operations: []PermissionOperation{PermissionRead},
			Fields: []FieldPermission{{Field: "description", Operations: []PermissionOperation{PermissionView}}},
		}},
	}
	editorRole := RoleDefinition{
		ID: editorRoleID, Name: "Редактор", Title: LocalizedText{"ru": "Редактор"},
		Objects: []ObjectPermission{{
			Object: catalogID, Operations: []PermissionOperation{PermissionRead, PermissionInsert, PermissionUpdate, PermissionDelete},
			Fields: []FieldPermission{
				{Field: "description", Operations: []PermissionOperation{PermissionView, PermissionEdit}},
				{Field: priceID.String(), Operations: []PermissionOperation{PermissionView, PermissionEdit}},
			},
		}},
	}
	catalog := &Catalog{
		Project:       project.Project{ID: projectID},
		Catalogs:      []CatalogDefinition{catalogDefinition},
		Roles:         []RoleDefinition{readerRole, editorRole},
		catalogByName: map[string]int{"товары": 0}, catalogByID: map[uuid.UUID]int{catalogID: 0},
		roleByName: map[string]int{"читательбезцены": 0, "редактор": 1}, roleByID: map[uuid.UUID]int{readerRoleID: 0, editorRoleID: 1},
	}
	migrateIntegrationSchema(t, ctx, pool, catalog, projectID)

	repository, err := NewCatalogRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := repository.New(ctx, "Товары", nil)
	if err != nil {
		t.Fatal(err)
	}
	seed.Code, seed.Description = "K001", "Монитор"
	seed.Attributes[priceID] = Value{Kind: NumberType, Data: "100"}
	if err := repository.Save(ctx, seed, nil); err != nil {
		t.Fatal(err)
	}
	reference := seed.Reference

	runtime, err := NewRuntimeWithCatalogs(nil, repository, catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	readerPolicy, err := CompilePermissions(catalog, []uuid.UUID{readerRoleID})
	if err != nil {
		t.Fatal(err)
	}
	editorPolicy, err := CompilePermissions(catalog, []uuid.UUID{editorRoleID})
	if err != nil {
		t.Fatal(err)
	}
	refValue := bytecode.String(reference.ObjectID.String())

	// Unrestricted context: exactly today's behavior, nothing new.
	if _, err := runtime.GetCatalogObject(ctx, "Товары", refValue); err != nil {
		t.Fatalf("unrestricted read: %v", err)
	}

	// Restricted reader: object read allowed, price field denied, writes denied.
	readerCtx := WithPermissions(ctx, readerPolicy)
	readerValue, err := runtime.GetCatalogObject(readerCtx, "Товары", refValue)
	if err != nil {
		t.Fatalf("reader role should read the object: %v", err)
	}
	readerObject, _ := readerValue.AsRuntimeObject()
	if _, err := runtime.GetObjectProperty(readerCtx, readerObject, "Наименование"); err != nil {
		t.Fatalf("reader role should read Description: %v", err)
	}
	if _, err := runtime.GetObjectProperty(readerCtx, readerObject, "Цена"); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("reader role read Price: %v", err)
	}
	if err := runtime.SetObjectProperty(readerCtx, readerObject, "Наименование", bytecode.String("Изменено")); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("reader role wrote Description: %v", err)
	}
	if _, err := runtime.CallObjectMethod(readerCtx, readerObject, "Записать", nil); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("reader role wrote the object: %v", err)
	}
	if _, err := runtime.CallObjectMethod(readerCtx, readerObject, "Удалить", nil); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("reader role deleted the object: %v", err)
	}
	if _, err := runtime.executeQuery(readerCtx, "ВЫБРАТЬ Ссылка ИЗ Справочник.Товары", nil); err != nil {
		t.Fatalf("reader role should query Товары: %v", err)
	}

	// No role at all denies everything, including Read.
	noRolePolicy, err := CompilePermissions(catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	noRoleCtx := WithPermissions(ctx, noRolePolicy)
	if _, err := runtime.GetCatalogObject(noRoleCtx, "Товары", refValue); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("empty role selection read the object: %v", err)
	}
	if _, err := runtime.executeQuery(noRoleCtx, "ВЫБРАТЬ Ссылка ИЗ Справочник.Товары", nil); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("empty role selection queried Товары: %v", err)
	}

	// Editor: full round trip still works with the policy attached.
	editorCtx := WithPermissions(ctx, editorPolicy)
	editorValue, err := runtime.GetCatalogObject(editorCtx, "Товары", refValue)
	if err != nil {
		t.Fatalf("editor role should read the object: %v", err)
	}
	editorObject, _ := editorValue.AsRuntimeObject()
	if err := runtime.SetObjectProperty(editorCtx, editorObject, "Цена", bytecode.Number(150)); err != nil {
		t.Fatalf("editor role should update Price: %v", err)
	}
	if _, err := runtime.CallObjectMethod(editorCtx, editorObject, "Записать", nil); err != nil {
		t.Fatalf("editor role should write the object: %v", err)
	}
	reloaded, err := repository.Get(ctx, reference)
	if err != nil || reloaded.Attributes[priceID].Data != "150" {
		t.Fatalf("editor write did not persist: reloaded=%+v error=%v", reloaded, err)
	}

	// The test-role wiring (transaction.go) must compile the same policy from
	// a role name and reject an unknown one instead of running unrestricted.
	if _, _, err := runtime.BeginTestExecutionWithOptions(ctx, "НесуществующаяРоль"); err == nil {
		t.Fatal("unknown role was silently accepted")
	}
	testCtx, finish, err := runtime.BeginTestExecutionWithOptions(ctx, readerRole.Name)
	if err != nil {
		t.Fatal(err)
	}
	testFailure := errors.New("stop the test transaction")
	defer func() { _ = finish(testFailure) }()
	testValue, err := runtime.GetCatalogObject(testCtx, "Товары", refValue)
	if err != nil {
		t.Fatalf("test role should read the object: %v", err)
	}
	testObject, _ := testValue.AsRuntimeObject()
	if _, err := runtime.CallObjectMethod(testCtx, testObject, "Записать", nil); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("test role wrote the object: %v", err)
	}
}
