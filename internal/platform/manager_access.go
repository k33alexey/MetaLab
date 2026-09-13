package platform

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/k33alexey/MetaLab/internal/auth"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

type managerTokenKey struct{}
type databaseOwnerKey struct{}

func (runtime *Runtime) AuthorizeStudioLease(ctx context.Context, lease StudioLease) error {
	database, err := runtime.systemDatabase()
	if err != nil {
		return err
	}
	digest := auth.SessionTokenDigest(lease.Token)
	session, err := database.StudioSessions.Authenticate(ctx, lease.Session.ID, digest[:])
	if err != nil {
		return err
	}
	if session.DatabaseID != lease.Session.DatabaseID || session.ProjectID != lease.Session.ProjectID {
		return systemdb.ErrStudioSessionNotFound
	}
	return nil
}

func (runtime *Runtime) databaseCreationContext(ctx context.Context) (context.Context, error) {
	if ctx.Value(managerTokenKey{}) == nil {
		return ctx, nil
	} // Trusted local CLI/service callers have no interactive identity.
	actor, err := runtime.RequireManager(ctx)
	if err != nil {
		return nil, err
	}
	return context.WithValue(ctx, databaseOwnerKey{}, actor.UserID), nil
}

// WithManagerToken attaches only an opaque token, never caller-supplied rights.
// Every authorization reloads the account and grants from ML System.
func WithManagerToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, managerTokenKey{}, token)
}

func (runtime *Runtime) systemDatabase() (*systemdb.Database, error) {
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	if runtime.database == nil {
		return nil, fmt.Errorf("ML System PostgreSQL is not configured")
	}
	return runtime.database, nil
}

func (runtime *Runtime) LoginManager(ctx context.Context, login, password, remoteAddress, userAgent string) (PortalLogin, error) {
	database, err := runtime.systemDatabase()
	if err != nil {
		return PortalLogin{}, err
	}
	user, err := database.Users.Authenticate(ctx, login, password)
	if err != nil {
		return PortalLogin{}, err
	}
	token, digest, err := auth.NewSessionToken()
	if err != nil {
		return PortalLogin{}, err
	}
	session, err := database.Sessions.CreateManager(ctx, user.ID, digest[:], remoteAddress, userAgent)
	if errors.Is(err, pgx.ErrNoRows) {
		return PortalLogin{}, systemdb.ErrManagerAccessDenied
	}
	if err != nil {
		return PortalLogin{}, err
	}
	_, _ = database.Audit.Write(ctx, systemdb.AuditEvent{Level: "info", Code: "manager.login", UserID: &user.ID, SessionID: &session.ID, Message: "Manager session started"})
	return PortalLogin{Token: token, Session: session}, nil
}

func (runtime *Runtime) AuthenticateManager(ctx context.Context, token string) (systemdb.PortalSession, error) {
	if token == "" {
		return systemdb.PortalSession{}, systemdb.ErrSessionNotFound
	}
	database, err := runtime.systemDatabase()
	if err != nil {
		return systemdb.PortalSession{}, err
	}
	digest := auth.SessionTokenDigest(token)
	return database.Sessions.AuthenticateManager(ctx, digest[:])
}

func (runtime *Runtime) LogoutManager(ctx context.Context, token string) error {
	database, err := runtime.systemDatabase()
	if err != nil {
		return err
	}
	digest := auth.SessionTokenDigest(token)
	return database.Sessions.RevokeManager(ctx, digest[:])
}

func (runtime *Runtime) ChangeManagerPassword(ctx context.Context, token, currentPassword, newPassword string) error {
	session, err := runtime.AuthenticateManager(ctx, token)
	if err != nil {
		return err
	}
	database, err := runtime.systemDatabase()
	if err != nil {
		return err
	}
	return database.Users.ChangePasswordKeepingSession(ctx, session.Login, currentPassword, newPassword, session.ID)
}

// RequireManager never grants administrative authority from a Portal token.
func (runtime *Runtime) RequireManager(ctx context.Context) (systemdb.PortalSession, error) {
	token, _ := ctx.Value(managerTokenKey{}).(string)
	session, err := runtime.AuthenticateManager(ctx, token)
	if err != nil {
		return systemdb.PortalSession{}, err
	}
	if session.MustChangePassword {
		return systemdb.PortalSession{}, systemdb.ErrPasswordChangeRequired
	}
	return session, nil
}

// AuthorizeManager scopes administration, Studio and global platform actions.
func (runtime *Runtime) AuthorizeManager(ctx context.Context, databaseID uuid.UUID, permission string) error {
	session, err := runtime.RequireManager(ctx)
	if err != nil {
		return err
	}
	if permission == "platform" {
		if session.PlatformAdmin {
			return nil
		}
		return systemdb.ErrManagerAccessDenied
	}
	if permission == "developer" {
		if session.MetadataAdmin {
			return nil
		}
		return systemdb.ErrManagerAccessDenied
	}
	if permission != "studio" && permission != "admin" {
		return systemdb.ErrManagerAccessDenied
	}
	if permission == "admin" && session.PlatformAdmin {
		return nil
	}
	database, err := runtime.systemDatabase()
	if err != nil {
		return err
	}
	access, err := database.DatabaseAccess.Get(ctx, session.UserID, databaseID)
	if err != nil {
		return err
	}
	if permission == "studio" && session.MetadataAdmin && access.StudioAccess {
		return nil
	}
	if permission == "admin" && access.DatabaseAdmin {
		return nil
	}
	return systemdb.ErrDatabaseAccessDenied
}

// ManagerDatabase annotates the user's registry view without adding Portal access.
type ManagerDatabase struct {
	systemdb.RegisteredDatabase
	Permissions systemdb.DatabasePermissions `json:"permissions"`
	Owner       *systemdb.DatabaseOwnerInfo  `json:"owner,omitempty"`
}

func (runtime *Runtime) ListManagerDatabases(ctx context.Context) ([]ManagerDatabase, error) {
	session, err := runtime.RequireManager(ctx)
	if err != nil {
		return nil, err
	}
	database, err := runtime.systemDatabase()
	if err != nil {
		return nil, err
	}
	access, err := database.DatabaseAccess.List(ctx, session.UserID)
	if err != nil {
		return nil, err
	}
	grants := make(map[uuid.UUID]systemdb.DatabasePermissions, len(access))
	for _, item := range access {
		grants[item.DatabaseID] = systemdb.DatabasePermissions{App: item.AppAccess, Studio: session.MetadataAdmin && item.StudioAccess, Admin: item.DatabaseAdmin}
	}
	result := []ManagerDatabase{}
	var items []systemdb.RegisteredDatabase
	if session.PlatformAdmin {
		items, err = database.Databases.List(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		for _, item := range access {
			grant := grants[item.DatabaseID]
			if !grant.Studio && !grant.Admin {
				continue
			}
			registered, err := database.Databases.Get(ctx, item.DatabaseID)
			if errors.Is(err, systemdb.ErrDatabaseNotFound) {
				continue
			}
			if err != nil {
				return nil, err
			}
			items = append(items, registered)
		}
	}
	visibleIDs := make([]uuid.UUID, len(items))
	for index, item := range items {
		visibleIDs[index] = item.ID
	}
	owners, err := database.DatabaseAccess.OwnersForDatabases(ctx, visibleIDs)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		grant := grants[item.ID]
		grant.Admin = grant.Admin || session.PlatformAdmin
		entry := ManagerDatabase{RegisteredDatabase: item, Permissions: grant}
		if owner, ok := owners[item.ID]; ok {
			entry.Owner = &owner
		}
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := strings.ToLower(result[i].Name), strings.ToLower(result[j].Name)
		if left == right {
			return result[i].ID.String() < result[j].ID.String()
		}
		return left < right
	})
	return result, nil
}

func (runtime *Runtime) SetManagerDatabasePermissions(ctx context.Context, userID, databaseID uuid.UUID, permissions systemdb.DatabasePermissions) error {
	session, err := runtime.RequireManager(ctx)
	if err != nil {
		return err
	}
	database, err := runtime.systemDatabase()
	if err != nil {
		return err
	}
	return database.DatabaseAccess.SetPermissions(ctx, session.UserID, userID, databaseID, permissions)
}

func (runtime *Runtime) ListManagerDatabaseAccess(ctx context.Context, databaseID uuid.UUID) ([]systemdb.DatabaseAccess, error) {
	session, err := runtime.RequireManager(ctx)
	if err != nil {
		return nil, err
	}
	database, err := runtime.systemDatabase()
	if err != nil {
		return nil, err
	}
	return database.DatabaseAccess.ListForDatabase(ctx, session.UserID, databaseID)
}

func (runtime *Runtime) AssignManagerDatabasePermissions(ctx context.Context, databaseID uuid.UUID, login string, permissions systemdb.DatabasePermissions) error {
	if err := runtime.AuthorizeManager(ctx, databaseID, "admin"); err != nil {
		return err
	}
	database, err := runtime.systemDatabase()
	if err != nil {
		return err
	}
	userID, err := database.Users.FindID(ctx, login)
	if err != nil {
		return err
	}
	return runtime.SetManagerDatabasePermissions(ctx, userID, databaseID, permissions)
}

func (runtime *Runtime) CreateManagerUser(ctx context.Context, creation systemdb.UserCreation) (systemdb.User, error) {
	if err := runtime.AuthorizeManager(ctx, uuid.UUID{}, "platform"); err != nil {
		return systemdb.User{}, err
	}
	database, err := runtime.systemDatabase()
	if err != nil {
		return systemdb.User{}, err
	}
	creation.ID = uuid.MustNew()
	creation.MustChangePassword = true
	return database.Users.Create(ctx, creation)
}

func (runtime *Runtime) UpdateManagerUser(ctx context.Context, userID uuid.UUID, update systemdb.UserAccessUpdate) error {
	session, err := runtime.RequireManager(ctx)
	if err != nil {
		return err
	}
	database, err := runtime.systemDatabase()
	if err != nil {
		return err
	}
	return database.Users.UpdateAccess(ctx, session.UserID, userID, update)
}

func (runtime *Runtime) ListManagerUsers(ctx context.Context) ([]systemdb.User, error) {
	if err := runtime.AuthorizeManager(ctx, uuid.UUID{}, "platform"); err != nil {
		return nil, err
	}
	database, err := runtime.systemDatabase()
	if err != nil {
		return nil, err
	}
	return database.Users.List(ctx)
}

func (runtime *Runtime) AuthorizeManagerSession(ctx context.Context, sessionID uuid.UUID) error {
	if _, err := runtime.RequireManager(ctx); err != nil {
		return err
	}
	database, err := runtime.systemDatabase()
	if err != nil {
		return err
	}
	id, err := database.Sessions.SessionDatabaseID(ctx, sessionID)
	if err != nil {
		return err
	}
	return runtime.AuthorizeManager(ctx, id, "admin")
}

func (runtime *Runtime) ListManagerStudioSessions(ctx context.Context) ([]systemdb.StudioSession, error) {
	visible, err := runtime.ListManagerDatabases(ctx)
	if err != nil {
		return nil, err
	}
	allowed := make(map[uuid.UUID]bool, len(visible))
	for _, item := range visible {
		allowed[item.ID] = true
	}
	database, err := runtime.systemDatabase()
	if err != nil {
		return nil, err
	}
	items, err := database.StudioSessions.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	result := []systemdb.StudioSession{}
	for _, item := range items {
		if allowed[item.DatabaseID] {
			result = append(result, item)
		}
	}
	return result, nil
}

func (runtime *Runtime) ListManagerSessions(ctx context.Context, databaseID *uuid.UUID) ([]systemdb.DatabaseSession, error) {
	visible, err := runtime.ListManagerDatabases(ctx)
	if err != nil {
		return nil, err
	}
	allowed := make(map[uuid.UUID]bool, len(visible))
	for _, item := range visible {
		allowed[item.ID] = item.Permissions.Admin
	}
	if databaseID != nil && !allowed[*databaseID] {
		return nil, systemdb.ErrDatabaseAccessDenied
	}
	database, err := runtime.systemDatabase()
	if err != nil {
		return nil, err
	}
	items, err := database.Sessions.ListDatabaseSessions(ctx, databaseID)
	if err != nil {
		return nil, err
	}
	result := []systemdb.DatabaseSession{}
	for _, item := range items {
		if allowed[item.DatabaseID] {
			result = append(result, item)
		}
	}
	return result, nil
}
