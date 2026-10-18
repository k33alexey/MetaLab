package metadata

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestPermissionsDenyByDefault(t *testing.T) {
	t.Parallel()
	catalog, role, _ := roleCatalogFixture(t)
	empty, err := CompilePermissions(catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, policy := range []*Permissions{nil, {}, empty} {
		for _, operation := range []PermissionOperation{PermissionRead, PermissionInsert, PermissionUpdate, PermissionDelete, PermissionPosting, PermissionUndoPosting, "admin", ""} {
			if policy.AllowsObject(role.Objects[0].Object, operation) || policy.AllowsFields(role.Objects[0].Object, operation) {
				t.Fatalf("implicit object grant: %s", operation)
			}
			if !errors.Is(policy.RequireFields(role.Objects[0].Object, operation, "description"), ErrPermissionDenied) {
				t.Fatal("missing policy allowed a field")
			}
		}
	}
	if empty.ProjectID() != catalog.Project.ID {
		t.Fatal("missing project binding")
	}
	withoutRoles, err := NewCatalogSnapshot(metadataConfiguration(), nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := CompilePermissions(withoutRoles, nil)
	if err != nil || policy.AllowsObject(role.Objects[0].Object, PermissionRead) {
		t.Fatalf("role-free project granted access: %v", err)
	}
}

func TestPermissionsUnionAndIsolation(t *testing.T) {
	catalog, reader, _ := roleCatalogFixture(t)
	object := reader.Objects[0].Object
	writer := RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Создатель", Title: LocalizedText{"ru": "Создатель"},
		Objects: []ObjectPermission{{Object: object, Operations: []PermissionOperation{PermissionInsert}, Fields: []FieldPermission{{Field: "description", Operations: []PermissionOperation{PermissionEdit}}}}}}
	catalog.Roles = append(catalog.Roles, writer)
	if err := catalog.indexAndValidate(""); err != nil {
		t.Fatal(err)
	}
	ids := []uuid.UUID{reader.ID, writer.ID}
	policy, err := CompilePermissions(catalog, ids)
	if err != nil {
		t.Fatal(err)
	}
	if !policy.AllowsFields(object, PermissionRead, "description") || !policy.AllowsFields(object, PermissionInsert, "description") {
		t.Fatal("explicit grants did not combine")
	}
	if policy.AllowsFields(object, PermissionUpdate, "description") || policy.AllowsFields(object, PermissionInsert, "ref") || policy.AllowsFields(object, PermissionRead, "Description") {
		t.Fatal("operation or field identity was widened")
	}
	ids[0] = uuid.MustNew()
	catalog.Roles[0].Objects[0].Operations = nil
	catalog.Roles[0].Objects[0].Fields[0].Operations = nil
	if !policy.AllowsFields(object, PermissionRead, "description") {
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

func TestPermissionsObjectAndFieldAreIndependent(t *testing.T) {
	t.Parallel()
	catalog, role, _ := roleCatalogFixture(t)
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
	if fieldOnly.AllowsFields(object, PermissionRead, column) || fieldOnly.AllowsObject(object, PermissionRead) {
		t.Fatal("field access implicitly granted object read")
	}
	catalog.Roles[0].Objects[0] = ObjectPermission{Object: object, Operations: []PermissionOperation{PermissionRead}, Fields: []FieldPermission{{Field: column, Operations: []PermissionOperation{PermissionView}}}}
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
	for _, ids := range [][]uuid.UUID{{uuid.UUID{}}, {uuid.MustNew()}, {role.ID, uuid.MustNew()}, {role.ID, role.ID}} {
		if policy, err := CompilePermissions(catalog, ids); !errors.Is(err, ErrInvalidRoleSelection) || policy != nil {
			t.Fatalf("invalid selection yielded a usable policy: %v", err)
		}
	}
	if _, err := CompilePermissions(nil, nil); !errors.Is(err, ErrInvalidRoleSelection) {
		t.Fatalf("nil catalog: %v", err)
	}
}

// The prototype bounds neither how many roles a user holds nor how many
// rights they add up to, and erp has 1211 roles: a user given all of them, or
// a few wide ones, must get every right of every role and not a refusal to
// enter. 1300 roles, eleven of them each viewing every one of ten thousand
// attributes - more than both ceilings that used to stand here.
func TestPermissionsTakeEveryRoleTheCatalogHas(t *testing.T) {
	catalog, role, _ := roleCatalogFixture(t)
	const roles, wide, piece = 1300, 11, 10_000
	object := role.Objects[0].Object
	attributes := make([]Attribute, piece)
	for index := range attributes {
		attributes[index] = Attribute{ID: uuid.MustNew(), Name: fmt.Sprintf("Реквизит%d", index), Title: LocalizedText{"ru": "Реквизит"},
			Types: []Type{{Kind: StringType, Length: 10}}}
	}
	catalog.Catalogs[0].Attributes = attributes
	widest := ObjectPermission{Object: object, Operations: []PermissionOperation{PermissionRead}}
	for _, attribute := range attributes {
		widest.Fields = append(widest.Fields, FieldPermission{Field: attribute.ID.String(), Operations: []PermissionOperation{PermissionView}})
	}
	ids := []uuid.UUID{}
	catalog.Roles = nil
	for index := 0; index < roles; index++ {
		next := cloneRole(role)
		next.ID, next.Name = uuid.MustNew(), fmt.Sprintf("Роль%d", index)
		next.Objects = []ObjectPermission{{Object: object, Operations: []PermissionOperation{PermissionRead},
			Fields: []FieldPermission{{Field: "description", Operations: []PermissionOperation{PermissionView}}}}}
		if index < wide {
			next.Objects = []ObjectPermission{widest}
		}
		catalog.Roles = append(catalog.Roles, next)
		ids = append(ids, next.ID)
	}
	if err := catalog.indexAndValidate(""); err != nil {
		t.Fatal(err)
	}
	policy, err := CompilePermissions(catalog, ids)
	if err != nil {
		t.Fatalf("%d roles with %d rights on fields among them were refused: %v", roles, wide*piece, err)
	}
	if !policy.AllowsFields(object, PermissionRead, attributes[piece-1].ID.String()) || !policy.AllowsFields(object, PermissionRead, "description") {
		t.Fatal("a right of one of the roles was lost")
	}
}

// Каждое право обязано иметь свой бит: право без бита не выдаётся никогда, и
// именно так «Просмотр» однажды оказался невыдаваемым — модель его знала, а
// компиляция прав молча теряла.
func TestEveryObjectOperationHasItsOwnBit(t *testing.T) {
	t.Parallel()
	seen := map[permissionBits]PermissionOperation{}
	for _, operation := range ObjectOperations() {
		bit := operationBit(operation)
		if bit == 0 {
			t.Fatalf("operation %q has no bit", operation)
		}
		if other, exists := seen[bit]; exists {
			t.Fatalf("operations %q and %q share a bit", operation, other)
		}
		seen[bit] = operation
	}
	if operationBit("не существует") != 0 {
		t.Fatal("an unknown operation must not map to a bit")
	}
}

// Fields are granted for reading and editing only. Every other right is about
// the object as a whole: asked with no fields it is the object's right, asked
// with fields it is refused, however the fields themselves are granted.
func TestOnlyReadAndEditReachFields(t *testing.T) {
	t.Parallel()
	catalog, role, _ := roleCatalogFixture(t)
	object := role.Objects[0].Object
	catalog.Roles[0].Objects[0].Operations = []PermissionOperation{PermissionRead, PermissionDelete}
	policy, err := CompilePermissions(catalog, []uuid.UUID{role.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !policy.AllowsFields(object, PermissionDelete) {
		t.Fatal("a granted object right was refused when asked with no fields")
	}
	if policy.AllowsFields(object, PermissionDelete, "description") {
		t.Fatal("deletion was allowed field by field")
	}
	if !policy.AllowsFields(object, PermissionRead, "description") {
		t.Fatal("a granted field was not readable")
	}
}

// A restriction's condition is not run yet, so a role restricting a right gets
// nothing from it: reading admits no row, adding, changing and deleting are
// not granted. A restriction with no condition restricts nothing, a right
// restricted in another role does not narrow this one, and roles add up: one
// reading without a restriction opens every row.
//
// Defect caught: a restriction that is not run read as no restriction at all,
// so a role of the prototype restricted to its own rows would read every row;
// and the other way, a restriction of one role closing the object to a role
// that does not restrict it.
func TestRestrictedRightsGiveNothingUntilConditionsRun(t *testing.T) {
	t.Parallel()
	catalog, reader, _ := roleCatalogFixture(t)
	object := reader.Objects[0].Object
	restricted := RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Ограниченный", Title: LocalizedText{"ru": "Ограниченный"},
		Objects: []ObjectPermission{{Object: object,
			Operations: []PermissionOperation{PermissionRead, PermissionInsert, PermissionUpdate, PermissionDelete},
			Restrictions: []RightRestriction{
				{Right: PermissionRead, Fields: []string{"ref"}},
				{Right: PermissionRead, Condition: "ГДЕ Ссылка = &Текущий"},
				{Right: PermissionInsert, Condition: "ГДЕ ЛОЖЬ"},
				{Right: PermissionUpdate, Condition: " \n"},
				{Right: PermissionDelete, Condition: "#Шаблон()"}}}}}
	catalog.Roles = append(catalog.Roles, restricted)
	if err := catalog.indexAndValidate(""); err != nil {
		t.Fatal(err)
	}
	alone, err := CompilePermissions(catalog, []uuid.UUID{restricted.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !alone.AllowsObject(object, PermissionRead) || alone.ReadsEveryRow(object) {
		t.Fatal("restricted reading was not granted as reading of no row")
	}
	if alone.AllowsObject(object, PermissionInsert) || alone.AllowsObject(object, PermissionDelete) {
		t.Fatal("a restricted write was granted")
	}
	if !alone.AllowsObject(object, PermissionUpdate) {
		t.Fatal("a restriction without a condition took the right away")
	}
	both, err := CompilePermissions(catalog, []uuid.UUID{restricted.ID, reader.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !both.ReadsEveryRow(object) {
		t.Fatal("a role reading without a restriction did not open every row")
	}
	open, err := CompilePermissions(catalog, []uuid.UUID{reader.ID})
	if err != nil || !open.ReadsEveryRow(object) {
		t.Fatalf("another role's restriction narrowed this one: %v", err)
	}
	var none *Permissions
	if none.ReadsEveryRow(object) {
		t.Fatal("no permissions read every row")
	}
}
