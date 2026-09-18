package platform

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

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

// applicationSessionValues resolves the session parameters the platform owns.
// They are deliberately few: a read path must be able to evaluate a row
// restriction without compiling and running a session module, so only values
// the platform already knows about the authenticated caller qualify.
//
// The current user is carried as the ML platform user identifier. An applied
// solution restricting rows by owner therefore has to store that identifier in
// its own data - the platform does not assume a "Users" catalog exists.
func applicationSessionValues(userID uuid.UUID) map[string][]metadata.Value {
	return map[string][]metadata.Value{
		metadata.CurrentUserParameter: {{Kind: metadata.UUIDType, Data: userID.String()}},
	}
}

// applicationSessionResolver answers for the session parameters the PROJECT
// computes - "the warehouses this user may see" and the like. Only the session
// module can produce them, so the resolver builds a BSL runtime; it does so
// lazily and once, because the overwhelming majority of reads never reach a
// restriction that names such a parameter, and compiling the project's modules
// for every list page would be a real cost paid for nothing.
func applicationSessionResolver(snapshot metadata.RuntimeSnapshot, pool *pgxpool.Pool, catalog *metadata.Catalog, actor uuid.UUID) metadata.SessionValueResolver {
	var once sync.Once
	var sessionRuntime *metadata.Runtime
	var failure error
	return func(ctx context.Context, name string) ([]metadata.Value, bool, error) {
		once.Do(func() { sessionRuntime, failure = newSessionParameterRuntime(snapshot, pool, catalog, actor) })
		if failure != nil {
			return nil, false, failure
		}
		return sessionRuntime.SessionParameterValues(ctx, name)
	}
}

func newSessionParameterRuntime(snapshot metadata.RuntimeSnapshot, pool *pgxpool.Pool, catalog *metadata.Catalog, actor uuid.UUID) (*metadata.Runtime, error) {
	sessionRuntime, err := metadata.NewApplicationRuntime(pool, catalog, &actor)
	if err != nil {
		return nil, err
	}
	if len(snapshot.Modules) == 0 {
		return sessionRuntime, nil
	}
	program, diagnostics, err := snapshot.CompileModules()
	if err != nil {
		return nil, fmt.Errorf("compile BSL: %w (%v)", err, diagnostics)
	}
	if err := metadata.WireBSLEvents(sessionRuntime, program, catalog); err != nil {
		return nil, err
	}
	return sessionRuntime, nil
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
