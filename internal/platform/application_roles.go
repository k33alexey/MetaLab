package platform

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

type ApplicationRoleOption struct {
	ID    uuid.UUID              `json:"id"`
	Name  string                 `json:"name"`
	Title metadata.LocalizedText `json:"title"`
}

type ManagerApplicationRoles struct {
	ProjectID  uuid.UUID                          `json:"projectId"`
	Available  []ApplicationRoleOption            `json:"available"`
	Assignment systemdb.ApplicationRoleAssignment `json:"assignment"`
}

type ApplicationRoleUpdate struct {
	ProjectID        uuid.UUID   `json:"projectId"`
	RoleIDs          []uuid.UUID `json:"roleIds"`
	ExpectedRevision int64       `json:"expectedRevision"`
}

func (runtime *Runtime) GetManagerApplicationRoles(ctx context.Context, databaseID, userID uuid.UUID) (ManagerApplicationRoles, error) {
	if err := runtime.AuthorizeManager(ctx, databaseID, "admin"); err != nil {
		return ManagerApplicationRoles{}, err
	}
	actor, err := runtime.RequireManager(ctx)
	if err != nil {
		return ManagerApplicationRoles{}, err
	}
	database, err := runtime.systemDatabase()
	if err != nil {
		return ManagerApplicationRoles{}, err
	}
	assignment, err := database.DatabaseAccess.GetApplicationRoles(ctx, actor.UserID, userID, databaseID)
	if err != nil {
		return ManagerApplicationRoles{}, err
	}
	snapshot, pool, err := runtime.loadPublishedMetadata(ctx, databaseID)
	if err != nil {
		return ManagerApplicationRoles{}, err
	}
	defer pool.Close()
	return managerApplicationRoleView(snapshot, assignment), nil
}

func managerApplicationRoleView(snapshot metadata.RuntimeSnapshot, assignment systemdb.ApplicationRoleAssignment) ManagerApplicationRoles {
	result := ManagerApplicationRoles{ProjectID: snapshot.Project.ID, Assignment: assignment, Available: []ApplicationRoleOption{}}
	for _, role := range snapshot.Roles {
		title := metadata.LocalizedText{}
		for language, text := range role.Title {
			title[language] = text
		}
		result.Available = append(result.Available, ApplicationRoleOption{ID: role.ID, Name: role.Name, Title: title})
	}
	sort.Slice(result.Available, func(i, j int) bool {
		return strings.ToLower(result.Available[i].Name) < strings.ToLower(result.Available[j].Name)
	})
	return result
}

// SetManagerApplicationRoles accepts identifiers, not role definitions or
// system privilege flags. Role identities come only from the active publication.
func (runtime *Runtime) SetManagerApplicationRoles(ctx context.Context, databaseID, userID uuid.UUID, update ApplicationRoleUpdate) (systemdb.ApplicationRoleAssignment, error) {
	if err := runtime.AuthorizeManager(ctx, databaseID, "admin"); err != nil {
		return systemdb.ApplicationRoleAssignment{}, err
	}
	actor, err := runtime.RequireManager(ctx)
	if err != nil {
		return systemdb.ApplicationRoleAssignment{}, err
	}
	snapshot, pool, err := runtime.loadPublishedMetadata(ctx, databaseID)
	if err != nil {
		return systemdb.ApplicationRoleAssignment{}, err
	}
	defer pool.Close()
	catalog, err := snapshot.Catalog()
	if err != nil {
		return systemdb.ApplicationRoleAssignment{}, err
	}
	if update.ProjectID != catalog.Project.ID {
		return systemdb.ApplicationRoleAssignment{}, systemdb.ErrInvalidApplicationRoles
	}
	if _, err := metadata.CompilePermissions(catalog, update.RoleIDs); err != nil {
		return systemdb.ApplicationRoleAssignment{}, err
	}
	database, err := runtime.systemDatabase()
	if err != nil {
		return systemdb.ApplicationRoleAssignment{}, err
	}
	return database.DatabaseAccess.SetApplicationRoles(ctx, actor.UserID, userID, databaseID, catalog.Project.ID, update.RoleIDs, update.ExpectedRevision)
}

// applicationPermissions resolves the policy of one already authenticated ML App
// user on one database and is re-read at every execution boundary rather than
// cached alongside the session: revoking a role must take effect on the user's
// next request, not at their next login. A user with no selection gets a policy
// that denies everything, because the absence of a grant is never itself a grant.
func (runtime *Runtime) applicationPermissions(ctx context.Context, databaseID, userID uuid.UUID, catalog *metadata.Catalog) (*metadata.Permissions, error) {
	database, err := runtime.systemDatabase()
	if err != nil {
		return nil, err
	}
	assignment, err := database.DatabaseAccess.ApplicationRoles(ctx, userID, databaseID)
	if err != nil {
		return nil, err
	}
	return permissionsForAssignment(catalog, assignment)
}

// permissionsForAssignment binds an ML System selection to exactly one project.
// No selection, including in a role-free project, means no application grants.
// Callers must authenticate and reload the selection for each execution boundary.
func permissionsForAssignment(catalog *metadata.Catalog, assignment systemdb.ApplicationRoleAssignment) (*metadata.Permissions, error) {
	if catalog == nil || assignment.ProjectID == nil && (len(assignment.RoleIDs) != 0 || assignment.Revision != 0) || assignment.ProjectID != nil && *assignment.ProjectID != catalog.Project.ID {
		return nil, metadata.ErrPermissionDenied
	}
	policy, err := metadata.CompilePermissions(catalog, assignment.RoleIDs)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", metadata.ErrPermissionDenied, err)
	}
	return policy, nil
}
