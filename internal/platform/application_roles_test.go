package platform

import (
	"errors"
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
