package platform

import (
	"errors"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestApplicationPermissionsBindProjectAndDenyUnassigned(t *testing.T) {
	t.Parallel()
	manifest := project.Project{Format: 1, ID: uuid.MustNew(), Name: "RolesTest", Title: "Roles test", DefaultLanguage: "ru", Languages: []project.Language{{Name: "Русский", Title: "Русский", Code: "ru"}}}
	role := metadata.RoleDefinition{Format: 1, ID: uuid.MustNew(), Name: "Читатель", Title: metadata.LocalizedText{"ru": "Читатель"}}
	catalog, err := metadata.NewCatalogSnapshotWithRoles(manifest, nil, nil, nil, nil, nil, nil, nil, []metadata.RoleDefinition{role})
	if err != nil {
		t.Fatal(err)
	}
	for _, assignment := range []systemdb.ApplicationRoleAssignment{{}, {ProjectID: &manifest.ID, RoleIDs: []uuid.UUID{role.ID}, Revision: 1}} {
		policy, err := permissionsForAssignment(catalog, assignment)
		if err != nil || policy.ProjectID() != manifest.ID || policy.AllowsObject(uuid.MustNew(), metadata.PermissionRead) {
			t.Fatalf("valid empty policy: %v", err)
		}
	}
	foreign := uuid.MustNew()
	for _, assignment := range []systemdb.ApplicationRoleAssignment{
		{RoleIDs: []uuid.UUID{role.ID}}, {Revision: 1}, {ProjectID: &foreign, RoleIDs: []uuid.UUID{role.ID}, Revision: 1}, {ProjectID: &manifest.ID, RoleIDs: []uuid.UUID{foreign}, Revision: 1},
	} {
		if policy, err := permissionsForAssignment(catalog, assignment); !errors.Is(err, metadata.ErrPermissionDenied) || policy != nil {
			t.Fatalf("foreign/invalid assignment accepted: %v", err)
		}
	}
	user := uuid.MustNew()
	values := applicationSessionValues(user)
	current, ok := values[metadata.CurrentUserParameter]
	if !ok || len(current) != 1 || current[0].Data != user.String() {
		t.Fatalf("the platform must resolve the current user itself: %+v", values)
	}
	if len(values) != 1 {
		t.Fatalf("only names the platform can resolve without BSL belong here: %+v", values)
	}

	if _, err := permissionsForAssignment(nil, systemdb.ApplicationRoleAssignment{}); !errors.Is(err, metadata.ErrPermissionDenied) {
		t.Fatal(err)
	}
	if systemdb.MaxApplicationRoles != metadata.MaxAssignedRoles {
		t.Fatal("role selection bounds disagree")
	}
	snapshot, err := metadata.NewRuntimeSnapshot(catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	view := managerApplicationRoleView(snapshot, systemdb.ApplicationRoleAssignment{RoleIDs: []uuid.UUID{}, Revision: 0})
	if view.ProjectID != manifest.ID || len(view.Available) != 1 || view.Available[0].Name != role.Name {
		t.Fatalf("role choices: %+v", view)
	}
	view.Available[0].Title["ru"] = "changed"
	if snapshot.Roles[0].Title["ru"] != "Читатель" {
		t.Fatal("role view aliases publication")
	}
}

// ML App offers the standard commands of an object - "открыть список",
// "создать" - from this list, so it has to be the caller's real rights and not
// a fixed set: offering "создать" to someone who may only read is an invitation
// to an error message.
func TestAllowedOperationsReportOnlyWhatTheRoleGrants(t *testing.T) {
	t.Parallel()
	manifest := project.Project{Format: 1, ID: uuid.MustNew(), Name: "OperationsTest", Title: "Operations test", DefaultLanguage: "ru",
		Languages: []project.Language{{Name: "Русский", Title: "Русский", Code: "ru"}}}
	goods := metadata.CatalogDefinition{Format: 1, ID: uuid.MustNew(), Name: "Товары", Title: metadata.LocalizedText{"ru": "Товары"},
		Code: metadata.CatalogCode{Type: metadata.StringType, Length: 9}, DescriptionLength: 150}
	partners := metadata.CatalogDefinition{Format: 1, ID: uuid.MustNew(), Name: "Контрагенты", Title: metadata.LocalizedText{"ru": "Контрагенты"},
		Code: metadata.CatalogCode{Type: metadata.StringType, Length: 9}, DescriptionLength: 150}
	role := metadata.RoleDefinition{Format: 1, ID: uuid.MustNew(), Name: "Продавец", Title: metadata.LocalizedText{"ru": "Продавец"},
		Objects: []metadata.ObjectPermission{
			{Object: goods.ID, Operations: []metadata.PermissionOperation{metadata.PermissionRead, metadata.PermissionCreate, metadata.PermissionUpdate}},
			{Object: partners.ID, Operations: []metadata.PermissionOperation{metadata.PermissionRead}},
		}}
	catalog, err := metadata.NewCatalogSnapshotWithRoles(manifest, nil, nil, nil,
		[]metadata.CatalogDefinition{goods, partners}, nil, nil, nil, []metadata.RoleDefinition{role})
	if err != nil {
		t.Fatal(err)
	}
	permissions, err := metadata.CompilePermissions(catalog, []uuid.UUID{role.ID})
	if err != nil {
		t.Fatal(err)
	}
	if operations := allowedOperations(permissions, goods.ID); !slices.Equal(operations, []metadata.PermissionOperation{
		metadata.PermissionRead, metadata.PermissionCreate, metadata.PermissionUpdate}) {
		t.Fatalf("granted operations = %v", operations)
	}
	if operations := allowedOperations(permissions, partners.ID); !slices.Equal(operations, []metadata.PermissionOperation{metadata.PermissionRead}) {
		t.Fatalf("read-only operations = %v", operations)
	}
	if operations := allowedOperations(permissions, uuid.MustNew()); len(operations) != 0 {
		t.Fatalf("an object no role mentions = %v", operations)
	}
}
