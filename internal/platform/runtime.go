// Package platform coordinates ML Manager with ML System and protected secrets.
package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/appdb"
	"github.com/k33alexey/MetaLab/internal/auth"
	"github.com/k33alexey/MetaLab/internal/pgbackup"
	"github.com/k33alexey/MetaLab/internal/pgcopy"
	"github.com/k33alexey/MetaLab/internal/postgresadmin"
	"github.com/k33alexey/MetaLab/internal/postgresconn"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Secrets is the minimal protected-storage boundary used by Manager.
type Secrets interface {
	Set(string, string) error
	Get(string) (string, error)
	Delete(string) error
}

// ProvisionRequest contains ephemeral administrator credentials and target names.
type ProvisionRequest struct {
	Host                  string `json:"host"`
	Port                  uint16 `json:"port"`
	AdministratorDatabase string `json:"administratorDatabase"`
	AdministratorUser     string `json:"administratorUser"`
	AdministratorPassword string `json:"administratorPassword"`
	SSLMode               string `json:"sslMode"`
	SystemDatabase        string `json:"systemDatabase"`
	TechnicalUser         string `json:"technicalUser"`
}

// RegisterDatabaseRequest contains one ephemeral application-database connection.
type RegisterDatabaseRequest struct {
	Name     string                `json:"name"`
	Host     string                `json:"host"`
	Port     uint16                `json:"port"`
	Database string                `json:"database"`
	User     string                `json:"user"`
	Password string                `json:"password"`
	SSLMode  string                `json:"sslMode"`
	Mode     systemdb.DatabaseMode `json:"mode"`
}

// CreateDebugDatabaseRequest provisions an isolated debug database, optionally with source data.
type CreateDebugDatabaseRequest struct {
	Name                  string `json:"name"`
	CopyData              bool   `json:"copyData"`
	Host                  string `json:"host"`
	Port                  uint16 `json:"port"`
	AdministratorDatabase string `json:"administratorDatabase"`
	AdministratorUser     string `json:"administratorUser"`
	AdministratorPassword string `json:"administratorPassword"`
	SSLMode               string `json:"sslMode"`
	TargetDatabase        string `json:"targetDatabase"`
	TechnicalUser         string `json:"technicalUser"`
}

// DatabaseCopier is the official PostgreSQL dump/restore boundary.
type DatabaseCopier interface {
	Copy(context.Context, postgresconn.Descriptor, string, int, postgresconn.Descriptor, string, int) error
}

// DatabaseBackupTool is the official PostgreSQL archive boundary.
type DatabaseBackupTool interface {
	Create(context.Context, postgresconn.Descriptor, string, string, string) (pgbackup.Archive, error)
	Restore(context.Context, postgresconn.Descriptor, string, string, string) error
}

// State is safe to expose to ML Manager UI.
type State struct {
	Configured bool                     `json:"configured"`
	Connected  bool                     `json:"connected"`
	Connection *postgresconn.Descriptor `json:"connection,omitempty"`
	Error      string                   `json:"error,omitempty"`
}

// PortalLogin is returned once after successful authentication.
type PortalLogin struct {
	Token   string                 `json:"-"`
	Session systemdb.PortalSession `json:"session"`
}

// PortalDatabase is the safe user-facing projection of a running database.
type PortalDatabase struct {
	ID               uuid.UUID                     `json:"id"`
	Name             string                        `json:"name"`
	Mode             systemdb.DatabaseMode         `json:"mode"`
	AllowNewSessions bool                          `json:"allowNewSessions"`
	HealthStatus     systemdb.DatabaseHealthStatus `json:"healthStatus"`
}

// Runtime owns the live ML System connection used by local Manager operations.
type Runtime struct {
	mu            sync.RWMutex
	configuration appconfig.Config
	secrets       Secrets
	database      *systemdb.Database
	connectionErr error
	copier        DatabaseCopier
	copierErr     error
	backupTool    DatabaseBackupTool
	backupToolErr error
}

// New opens an already configured ML System. A connection problem remains visible in State.
func New(ctx context.Context, configuration appconfig.Config, secrets Secrets) *Runtime {
	runtime := &Runtime{configuration: configuration, secrets: secrets}
	runtime.copier, runtime.copierErr = pgcopy.New()
	runtime.backupTool, runtime.backupToolErr = pgbackup.New()
	if secrets == nil {
		runtime.connectionErr = fmt.Errorf("OS credential store is unavailable")
		return runtime
	}
	if configuration.SystemDatabase != nil {
		runtime.database, runtime.connectionErr = openSystemDatabase(ctx, *configuration.SystemDatabase, secrets)
	} else if databaseURL := os.Getenv("ML_SYSTEM_DATABASE_URL"); databaseURL != "" {
		runtime.database, runtime.connectionErr = systemdb.Open(ctx, databaseURL)
	}
	return runtime
}

// Close releases the current ML System connection.
func (runtime *Runtime) Close() {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.database != nil {
		runtime.database.Close()
		runtime.database = nil
	}
}

// State returns non-secret PostgreSQL setup state.
func (runtime *Runtime) State() State {
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	state := State{Configured: runtime.configuration.SystemDatabase != nil, Connected: runtime.database != nil}
	if runtime.configuration.SystemDatabase != nil {
		copy := *runtime.configuration.SystemDatabase
		state.Connection = &copy
	}
	if runtime.connectionErr != nil {
		state.Error = runtime.connectionErr.Error()
	}
	return state
}

// CheckPostgreSQL validates a proposed administrative connection without saving it.
func (runtime *Runtime) CheckPostgreSQL(ctx context.Context, request ProvisionRequest) (postgresadmin.Check, error) {
	descriptor, err := request.administratorDescriptor()
	if err != nil {
		return postgresadmin.Check{}, err
	}
	return postgresadmin.Validate(ctx, descriptor, request.AdministratorPassword)
}

// ProvisionPostgreSQL creates, migrates and securely saves a fresh ML System database.
func (runtime *Runtime) ProvisionPostgreSQL(ctx context.Context, request ProvisionRequest) (postgresadmin.Check, error) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.configuration.SystemDatabase != nil || runtime.database != nil {
		return postgresadmin.Check{}, fmt.Errorf("ML System PostgreSQL is already configured")
	}
	if runtime.secrets == nil {
		return postgresadmin.Check{}, fmt.Errorf("OS credential store is unavailable")
	}
	administrator, err := request.administratorDescriptor()
	if err != nil {
		return postgresadmin.Check{}, err
	}
	check, err := postgresadmin.Validate(ctx, administrator, request.AdministratorPassword)
	if err != nil {
		return postgresadmin.Check{}, err
	}
	provisioned, err := postgresadmin.Provision(
		ctx, administrator, request.AdministratorPassword, request.SystemDatabase, request.TechnicalUser,
	)
	if err != nil {
		return postgresadmin.Check{}, err
	}
	rollback := func(cause error) (postgresadmin.Check, error) {
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		rollbackErr := postgresadmin.RollbackProvisioned(rollbackCtx, administrator, request.AdministratorPassword, provisioned)
		if rollbackErr != nil {
			return postgresadmin.Check{}, errors.Join(cause, rollbackErr)
		}
		return postgresadmin.Check{}, cause
	}
	poolConfiguration, err := provisioned.Connection.PoolConfig(provisioned.Password)
	if err != nil {
		return rollback(err)
	}
	database, err := systemdb.OpenConfig(ctx, poolConfiguration)
	if err != nil {
		return rollback(err)
	}
	if err := runtime.secrets.Set(provisioned.Connection.SecretKey, provisioned.Password); err != nil {
		database.Close()
		return rollback(err)
	}
	configuration := runtime.configuration
	configuration.SystemDatabase = &provisioned.Connection
	path := configuration.SourcePath
	if path == "" {
		path, err = appconfig.DefaultPath()
	}
	if err == nil {
		err = appconfig.Save(path, configuration)
	}
	if err != nil {
		database.Close()
		_ = runtime.secrets.Delete(provisioned.Connection.SecretKey)
		return rollback(err)
	}
	configuration.SourcePath = path
	runtime.configuration = configuration
	runtime.database = database
	runtime.connectionErr = nil
	return check, nil
}

// InitialSetupRequired implements the Manager administrator wizard boundary.
func (runtime *Runtime) InitialSetupRequired(ctx context.Context) (bool, error) {
	runtime.mu.RLock()
	database, connectionErr := runtime.database, runtime.connectionErr
	runtime.mu.RUnlock()
	if database == nil {
		if connectionErr != nil {
			return false, connectionErr
		}
		return false, fmt.Errorf("ML System PostgreSQL is not configured")
	}
	return database.Administrators.InitialSetupRequired(ctx)
}

// CreateInitial creates the first platform administrator after PostgreSQL setup.
func (runtime *Runtime) CreateInitial(ctx context.Context, login, password string) (systemdb.Administrator, []string, error) {
	runtime.mu.RLock()
	database := runtime.database
	runtime.mu.RUnlock()
	if database == nil {
		return systemdb.Administrator{}, nil, fmt.Errorf("ML System PostgreSQL is not configured")
	}
	return database.Administrators.CreateInitial(ctx, login, password)
}

// ListDatabases returns the safe application-database registry.
func (runtime *Runtime) ListDatabases(ctx context.Context) ([]systemdb.RegisteredDatabase, error) {
	runtime.mu.RLock()
	database, connectionErr := runtime.database, runtime.connectionErr
	runtime.mu.RUnlock()
	if database == nil {
		if connectionErr != nil {
			return nil, connectionErr
		}
		return nil, fmt.Errorf("ML System PostgreSQL is not configured")
	}
	return database.Databases.List(ctx)
}

// RegisterDatabase verifies a physical PostgreSQL database and stores its password separately.
func (runtime *Runtime) RegisterDatabase(ctx context.Context, request RegisterDatabaseRequest) (systemdb.RegisteredDatabase, error) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.database == nil {
		if runtime.connectionErr != nil {
			return systemdb.RegisteredDatabase{}, runtime.connectionErr
		}
		return systemdb.RegisteredDatabase{}, fmt.Errorf("ML System PostgreSQL is not configured")
	}
	if err := systemdb.ValidateDatabaseName(request.Name); err != nil {
		return systemdb.RegisteredDatabase{}, err
	}
	if request.Mode == "" {
		request.Mode = systemdb.DatabasePrimary
	}
	if request.Mode != systemdb.DatabasePrimary && request.Mode != systemdb.DatabaseDebug {
		return systemdb.RegisteredDatabase{}, fmt.Errorf("invalid database mode %q", request.Mode)
	}
	descriptor := postgresconn.Descriptor{
		Host: request.Host, Port: request.Port, Database: request.Database,
		User: request.User, SSLMode: request.SSLMode, SecretKey: "placeholder",
	}
	if err := validateApplicationConnection(descriptor, request.Password); err != nil {
		return systemdb.RegisteredDatabase{}, err
	}
	if _, err := postgresadmin.Validate(ctx, descriptor, request.Password); err != nil {
		return systemdb.RegisteredDatabase{}, err
	}
	physicalID, err := appdb.EnsureIdentity(ctx, descriptor, request.Password)
	if err != nil {
		return systemdb.RegisteredDatabase{}, err
	}
	return runtime.storeDatabase(ctx, request.Name, request.Mode, nil, descriptor, request.Password, physicalID)
}

// CreateDebugDatabase creates a separate clean or data-bearing debug database from a primary source.
func (runtime *Runtime) CreateDebugDatabase(ctx context.Context, sourceID uuid.UUID, request CreateDebugDatabaseRequest) (systemdb.RegisteredDatabase, error) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.database == nil {
		return systemdb.RegisteredDatabase{}, fmt.Errorf("ML System PostgreSQL is not configured")
	}
	if err := systemdb.ValidateDatabaseName(request.Name); err != nil {
		return systemdb.RegisteredDatabase{}, err
	}
	source, err := runtime.database.Databases.Get(ctx, sourceID)
	if err != nil {
		return systemdb.RegisteredDatabase{}, err
	}
	if source.Mode != systemdb.DatabasePrimary {
		return systemdb.RegisteredDatabase{}, fmt.Errorf("debug database can only be created from a primary database")
	}
	if request.CopyData && runtime.copier == nil {
		if runtime.copierErr != nil {
			return systemdb.RegisteredDatabase{}, runtime.copierErr
		}
		return systemdb.RegisteredDatabase{}, fmt.Errorf("pg_dump and pg_restore are unavailable")
	}
	administratorRequest := ProvisionRequest{
		Host: request.Host, Port: request.Port,
		AdministratorDatabase: request.AdministratorDatabase, AdministratorUser: request.AdministratorUser,
		AdministratorPassword: request.AdministratorPassword, SSLMode: request.SSLMode,
		SystemDatabase: request.TargetDatabase, TechnicalUser: request.TechnicalUser,
	}
	administrator, err := administratorRequest.administratorDescriptor()
	if err != nil {
		return systemdb.RegisteredDatabase{}, err
	}
	targetCheck, err := postgresadmin.Validate(ctx, administrator, request.AdministratorPassword)
	if err != nil {
		return systemdb.RegisteredDatabase{}, err
	}
	provisioned, err := postgresadmin.Provision(
		ctx, administrator, request.AdministratorPassword, request.TargetDatabase, request.TechnicalUser,
	)
	if err != nil {
		return systemdb.RegisteredDatabase{}, err
	}
	rollback := func(cause error) (systemdb.RegisteredDatabase, error) {
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		rollbackErr := postgresadmin.RollbackProvisioned(rollbackCtx, administrator, request.AdministratorPassword, provisioned)
		return systemdb.RegisteredDatabase{}, errors.Join(cause, rollbackErr)
	}
	var physicalID uuid.UUID
	if request.CopyData {
		sourcePassword, secretErr := runtime.secrets.Get(source.Connection.SecretKey)
		if secretErr != nil {
			return rollback(secretErr)
		}
		sourceCheck, checkErr := postgresadmin.Validate(ctx, source.Connection, sourcePassword)
		if checkErr != nil {
			return rollback(checkErr)
		}
		if copyErr := runtime.copier.Copy(
			ctx, source.Connection, sourcePassword, sourceCheck.VersionNumber/10000,
			provisioned.Connection, provisioned.Password, targetCheck.VersionNumber/10000,
		); copyErr != nil {
			return rollback(copyErr)
		}
		physicalID, err = appdb.ResetIdentity(ctx, provisioned.Connection, provisioned.Password)
	} else {
		physicalID, err = appdb.EnsureIdentity(ctx, provisioned.Connection, provisioned.Password)
	}
	if err != nil {
		return rollback(err)
	}
	registered, err := runtime.storeDatabase(
		ctx, request.Name, systemdb.DatabaseDebug, &sourceID,
		provisioned.Connection, provisioned.Password, physicalID,
	)
	if err != nil {
		return rollback(err)
	}
	return registered, nil
}

// UnregisterDatabase removes a stopped registry entry and its protected password.
func (runtime *Runtime) UnregisterDatabase(ctx context.Context, id uuid.UUID) error {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.database == nil {
		return fmt.Errorf("ML System PostgreSQL is not configured")
	}
	registered, err := runtime.database.Databases.Get(ctx, id)
	if err != nil {
		return err
	}
	if registered.State != systemdb.DatabaseStopped {
		return systemdb.ErrDatabaseCannotUnregister
	}
	password, err := runtime.secrets.Get(registered.Connection.SecretKey)
	if err != nil {
		return err
	}
	if err := runtime.secrets.Delete(registered.Connection.SecretKey); err != nil {
		return err
	}
	if _, err := runtime.database.Databases.Unregister(ctx, id); err != nil {
		if restoreErr := runtime.secrets.Set(registered.Connection.SecretKey, password); restoreErr != nil {
			return errors.Join(err, fmt.Errorf("restore protected database password: %w", restoreErr))
		}
		return err
	}
	return nil
}

// LoginPortal authenticates an internal account and creates a revocable session.
func (runtime *Runtime) LoginPortal(ctx context.Context, login, password, remoteAddress, userAgent string) (PortalLogin, error) {
	runtime.mu.RLock()
	database := runtime.database
	runtime.mu.RUnlock()
	if database == nil {
		return PortalLogin{}, fmt.Errorf("ML System PostgreSQL is not configured")
	}
	administrator, err := database.Administrators.Authenticate(ctx, login, password)
	if err != nil {
		_, _ = database.Audit.Write(ctx, systemdb.AuditEvent{
			Level: "warning", Code: "portal.login_failed", Message: "Portal authentication failed",
		})
		return PortalLogin{}, err
	}
	token, digest, err := auth.NewSessionToken()
	if err != nil {
		return PortalLogin{}, err
	}
	session, err := database.Sessions.CreatePortal(ctx, administrator.ID, digest[:], remoteAddress, userAgent)
	if err != nil {
		return PortalLogin{}, err
	}
	_, _ = database.Audit.Write(ctx, systemdb.AuditEvent{
		Level: "info", Code: "portal.login", UserID: &administrator.ID, SessionID: &session.ID,
		Message: "Portal session started",
	})
	return PortalLogin{Token: token, Session: session}, nil
}

// AuthenticatePortal resolves and refreshes an opaque portal token.
func (runtime *Runtime) AuthenticatePortal(ctx context.Context, token string) (systemdb.PortalSession, error) {
	runtime.mu.RLock()
	database := runtime.database
	runtime.mu.RUnlock()
	if database == nil || token == "" {
		return systemdb.PortalSession{}, systemdb.ErrSessionNotFound
	}
	digest := auth.SessionTokenDigest(token)
	return database.Sessions.AuthenticatePortal(ctx, digest[:])
}

// LogoutPortal revokes a portal token and all derived database sessions.
func (runtime *Runtime) LogoutPortal(ctx context.Context, token string) error {
	runtime.mu.RLock()
	database := runtime.database
	runtime.mu.RUnlock()
	if database == nil || token == "" {
		return systemdb.ErrSessionNotFound
	}
	digest := auth.SessionTokenDigest(token)
	return database.Sessions.RevokePortal(ctx, digest[:])
}

// ChangePortalPassword completes a mandatory or voluntary password change in the current session.
func (runtime *Runtime) ChangePortalPassword(ctx context.Context, token, currentPassword, newPassword string) error {
	session, err := runtime.AuthenticatePortal(ctx, token)
	if err != nil {
		return err
	}
	runtime.mu.RLock()
	database := runtime.database
	runtime.mu.RUnlock()
	if err := database.Administrators.ChangePasswordKeepingSession(ctx, session.Login, currentPassword, newPassword, session.ID); err != nil {
		return err
	}
	_, _ = database.Audit.Write(ctx, systemdb.AuditEvent{
		Level: "info", Code: "portal.password_changed", UserID: &session.UserID,
		SessionID: &session.ID, Message: "User password changed",
	})
	return nil
}

// LoadPortal authenticates once and returns the current user with running databases.
func (runtime *Runtime) LoadPortal(ctx context.Context, token string) (systemdb.PortalSession, []PortalDatabase, error) {
	session, err := runtime.AuthenticatePortal(ctx, token)
	if err != nil {
		return systemdb.PortalSession{}, nil, err
	}
	items, err := runtime.ListDatabases(ctx)
	if err != nil {
		return systemdb.PortalSession{}, nil, err
	}
	visible := make([]PortalDatabase, 0, len(items))
	if session.MustChangePassword {
		return session, visible, nil
	}
	for _, item := range items {
		if item.State == systemdb.DatabaseRunning {
			visible = append(visible, PortalDatabase{
				ID: item.ID, Name: item.Name, Mode: item.Mode, AllowNewSessions: item.AllowNewSessions,
				HealthStatus: item.HealthStatus,
			})
		}
	}
	return session, visible, nil
}

// OpenPortalDatabase starts or refreshes the caller's session in a running database.
func (runtime *Runtime) OpenPortalDatabase(ctx context.Context, token string, databaseID uuid.UUID) (systemdb.DatabaseSession, error) {
	session, err := runtime.AuthenticatePortal(ctx, token)
	if err != nil {
		return systemdb.DatabaseSession{}, err
	}
	if session.MustChangePassword {
		return systemdb.DatabaseSession{}, systemdb.ErrPasswordChangeRequired
	}
	runtime.mu.RLock()
	database := runtime.database
	runtime.mu.RUnlock()
	registered, err := database.Databases.Get(ctx, databaseID)
	if err != nil {
		return systemdb.DatabaseSession{}, err
	}
	if registered.State != systemdb.DatabaseRunning {
		return systemdb.DatabaseSession{}, systemdb.ErrDatabaseNotRunning
	}
	if !registered.AllowNewSessions {
		if existing, resumeErr := database.Sessions.ResumeDatabase(ctx, session.ID, databaseID); resumeErr == nil {
			return existing, nil
		}
		return systemdb.DatabaseSession{}, systemdb.ErrNewSessionsForbidden
	}
	if _, err := runtime.CheckDatabaseHealth(ctx, databaseID); err != nil {
		return systemdb.DatabaseSession{}, err
	}
	opened, err := database.Sessions.OpenDatabase(ctx, session.ID, databaseID)
	if err == nil {
		_, _ = database.Audit.Write(ctx, systemdb.AuditEvent{
			Level: "info", Code: "database.session_opened", DatabaseID: &databaseID,
			UserID: &session.UserID, SessionID: &opened.ID, Message: "Database session opened",
		})
	}
	return opened, err
}

// ResumePortalDatabase refreshes only an already active database session.
func (runtime *Runtime) ResumePortalDatabase(ctx context.Context, token string, databaseID uuid.UUID) (systemdb.DatabaseSession, error) {
	session, err := runtime.AuthenticatePortal(ctx, token)
	if err != nil {
		return systemdb.DatabaseSession{}, err
	}
	if session.MustChangePassword {
		return systemdb.DatabaseSession{}, systemdb.ErrPasswordChangeRequired
	}
	runtime.mu.RLock()
	database := runtime.database
	runtime.mu.RUnlock()
	registered, err := database.Databases.Get(ctx, databaseID)
	if err != nil {
		return systemdb.DatabaseSession{}, err
	}
	if registered.State != systemdb.DatabaseRunning {
		return systemdb.DatabaseSession{}, systemdb.ErrDatabaseNotRunning
	}
	return database.Sessions.ResumeDatabase(ctx, session.ID, databaseID)
}

// AcknowledgeSessionMessage removes a message after the browser displays it.
func (runtime *Runtime) AcknowledgeSessionMessage(ctx context.Context, token string, sessionID uuid.UUID) error {
	session, err := runtime.AuthenticatePortal(ctx, token)
	if err != nil {
		return err
	}
	runtime.mu.RLock()
	database := runtime.database
	runtime.mu.RUnlock()
	return database.Sessions.AcknowledgeMessage(ctx, session.ID, sessionID)
}

// StartDatabase verifies the connection and exposes a registered database in Portal.
func (runtime *Runtime) StartDatabase(ctx context.Context, id uuid.UUID) (systemdb.RegisteredDatabase, error) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.database == nil {
		return systemdb.RegisteredDatabase{}, fmt.Errorf("ML System PostgreSQL is not configured")
	}
	item, err := runtime.database.Databases.Get(ctx, id)
	if err != nil {
		return systemdb.RegisteredDatabase{}, err
	}
	if item.State == systemdb.DatabaseError {
		item, err = runtime.database.Databases.Transition(ctx, id, item.State, item.StateRevision, systemdb.DatabaseStopped, "")
		if err != nil {
			return systemdb.RegisteredDatabase{}, err
		}
	}
	starting, err := runtime.database.Databases.Transition(ctx, id, item.State, item.StateRevision, systemdb.DatabaseStarting, "")
	if err != nil {
		return systemdb.RegisteredDatabase{}, err
	}
	password, err := runtime.secrets.Get(item.Connection.SecretKey)
	if err == nil {
		var physicalID uuid.UUID
		physicalID, err = appdb.EnsureIdentity(ctx, item.Connection, password)
		if err == nil && physicalID != item.PhysicalID {
			err = fmt.Errorf("physical PostgreSQL database identity changed")
		}
	}
	if err != nil {
		message := safeOperationalError(err, password)
		_, _ = runtime.database.Databases.RecordHealth(ctx, id, false, message)
		failed, transitionErr := runtime.database.Databases.Transition(ctx, id, systemdb.DatabaseStarting, starting.StateRevision, systemdb.DatabaseError, message)
		if transitionErr != nil {
			return systemdb.RegisteredDatabase{}, errors.Join(err, transitionErr)
		}
		return failed, err
	}
	if _, err := runtime.database.Databases.RecordHealth(ctx, id, true, ""); err != nil {
		return systemdb.RegisteredDatabase{}, err
	}
	running, err := runtime.database.Databases.Transition(ctx, id, systemdb.DatabaseStarting, starting.StateRevision, systemdb.DatabaseRunning, "")
	if err == nil {
		_, _ = runtime.database.Audit.Write(ctx, systemdb.AuditEvent{Level: "info", Code: "database.started", DatabaseID: &id, Message: "Database started"})
	}
	return running, err
}

// StopDatabase blocks new work, terminates sessions and marks a database stopped.
func (runtime *Runtime) StopDatabase(ctx context.Context, id uuid.UUID) (systemdb.RegisteredDatabase, error) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.database == nil {
		return systemdb.RegisteredDatabase{}, fmt.Errorf("ML System PostgreSQL is not configured")
	}
	item, err := runtime.database.Databases.Get(ctx, id)
	if err != nil {
		return systemdb.RegisteredDatabase{}, err
	}
	if item.State == systemdb.DatabaseError {
		return runtime.database.Databases.Transition(ctx, id, item.State, item.StateRevision, systemdb.DatabaseStopped, "")
	}
	stopping, err := runtime.database.Databases.Transition(ctx, id, item.State, item.StateRevision, systemdb.DatabaseStopping, "")
	if err != nil {
		return systemdb.RegisteredDatabase{}, err
	}
	if _, err := runtime.database.Sessions.TerminateDatabaseSessions(ctx, id); err != nil {
		failed, transitionErr := runtime.database.Databases.Transition(ctx, id, systemdb.DatabaseStopping, stopping.StateRevision, systemdb.DatabaseError, err.Error())
		if transitionErr != nil {
			return systemdb.RegisteredDatabase{}, errors.Join(err, transitionErr)
		}
		return failed, err
	}
	stopped, err := runtime.database.Databases.Transition(ctx, id, systemdb.DatabaseStopping, stopping.StateRevision, systemdb.DatabaseStopped, "")
	if err == nil {
		_, _ = runtime.database.Audit.Write(ctx, systemdb.AuditEvent{Level: "info", Code: "database.stopped", DatabaseID: &id, Message: "Database stopped"})
	}
	return stopped, err
}

// SetDatabaseSessionAccess enables or forbids only new logins.
func (runtime *Runtime) SetDatabaseSessionAccess(ctx context.Context, id uuid.UUID, allowed bool) (systemdb.RegisteredDatabase, error) {
	runtime.mu.RLock()
	database := runtime.database
	runtime.mu.RUnlock()
	if database == nil {
		return systemdb.RegisteredDatabase{}, fmt.Errorf("ML System PostgreSQL is not configured")
	}
	item, err := database.Databases.SetSessionAccess(ctx, id, allowed)
	if err == nil {
		code := "database.logins_forbidden"
		if allowed {
			code = "database.logins_allowed"
		}
		_, _ = database.Audit.Write(ctx, systemdb.AuditEvent{Level: "info", Code: code, DatabaseID: &id, Message: "Database login policy changed"})
	}
	return item, err
}

// CheckDatabaseHealth verifies connectivity and the stable physical identity.
func (runtime *Runtime) CheckDatabaseHealth(ctx context.Context, id uuid.UUID) (systemdb.RegisteredDatabase, error) {
	runtime.mu.RLock()
	database := runtime.database
	runtime.mu.RUnlock()
	if database == nil {
		return systemdb.RegisteredDatabase{}, fmt.Errorf("ML System PostgreSQL is not configured")
	}
	item, err := database.Databases.Get(ctx, id)
	if err != nil {
		return systemdb.RegisteredDatabase{}, err
	}
	password, err := runtime.secrets.Get(item.Connection.SecretKey)
	if err == nil {
		var physicalID uuid.UUID
		physicalID, err = appdb.EnsureIdentity(ctx, item.Connection, password)
		if err == nil && physicalID != item.PhysicalID {
			err = fmt.Errorf("physical PostgreSQL database identity changed")
		}
	}
	if err != nil {
		updated, recordErr := database.Databases.RecordHealth(ctx, id, false, safeOperationalError(err, password))
		return updated, errors.Join(err, recordErr)
	}
	return database.Databases.RecordHealth(ctx, id, true, "")
}

func safeOperationalError(err error, secrets ...string) string {
	message := err.Error()
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[REDACTED]")
		}
	}
	return message
}

// ListDatabaseSessions returns active application sessions for administration.
func (runtime *Runtime) ListDatabaseSessions(ctx context.Context, databaseID *uuid.UUID) ([]systemdb.DatabaseSession, error) {
	runtime.mu.RLock()
	database := runtime.database
	runtime.mu.RUnlock()
	if database == nil {
		return nil, fmt.Errorf("ML System PostgreSQL is not configured")
	}
	return database.Sessions.ListDatabaseSessions(ctx, databaseID)
}

// SendSessionMessage queues an administrative message for one session.
func (runtime *Runtime) SendSessionMessage(ctx context.Context, id uuid.UUID, message string) error {
	runtime.mu.RLock()
	database := runtime.database
	runtime.mu.RUnlock()
	if database == nil {
		return fmt.Errorf("ML System PostgreSQL is not configured")
	}
	if err := database.Sessions.SendMessage(ctx, id, message); err != nil {
		return err
	}
	_, _ = database.Audit.Write(ctx, systemdb.AuditEvent{
		Level: "info", Code: "database.session_message", SessionID: &id,
		Message: "Administrative message sent to database session",
	})
	return nil
}

// TerminateDatabaseSession forcibly ends one application session.
func (runtime *Runtime) TerminateDatabaseSession(ctx context.Context, id uuid.UUID) error {
	runtime.mu.RLock()
	database := runtime.database
	runtime.mu.RUnlock()
	if database == nil {
		return fmt.Errorf("ML System PostgreSQL is not configured")
	}
	if err := database.Sessions.TerminateDatabaseSession(ctx, id); err != nil {
		return err
	}
	_, _ = database.Audit.Write(ctx, systemdb.AuditEvent{
		Level: "warning", Code: "database.session_terminated", SessionID: &id,
		Message: "Database session terminated by administrator",
	})
	return nil
}

// ListAuditEvents exposes a bounded technical journal to ML Manager.
func (runtime *Runtime) ListAuditEvents(ctx context.Context, databaseID *uuid.UUID, limit int) ([]systemdb.AuditEvent, error) {
	runtime.mu.RLock()
	database := runtime.database
	runtime.mu.RUnlock()
	if database == nil {
		return nil, fmt.Errorf("ML System PostgreSQL is not configured")
	}
	return database.Audit.List(ctx, databaseID, limit)
}

// CreateDatabaseBackup creates a consistent local archive without stopping the database.
func (runtime *Runtime) CreateDatabaseBackup(ctx context.Context, databaseID uuid.UUID) (systemdb.Backup, error) {
	runtime.mu.RLock()
	database, tool, toolErr := runtime.database, runtime.backupTool, runtime.backupToolErr
	configuration := runtime.configuration
	runtime.mu.RUnlock()
	if database == nil {
		return systemdb.Backup{}, fmt.Errorf("ML System PostgreSQL is not configured")
	}
	if tool == nil {
		if toolErr != nil {
			return systemdb.Backup{}, toolErr
		}
		return systemdb.Backup{}, fmt.Errorf("PostgreSQL backup tools are unavailable")
	}
	registered, err := database.Databases.Get(ctx, databaseID)
	if err != nil {
		return systemdb.Backup{}, err
	}
	password, err := runtime.secrets.Get(registered.Connection.SecretKey)
	if err != nil {
		return systemdb.Backup{}, err
	}
	directory, err := configuration.BackupDirectory()
	if err != nil {
		return systemdb.Backup{}, err
	}
	id, err := uuid.New()
	if err != nil {
		return systemdb.Backup{}, err
	}
	fileName := registered.ID.String() + "-" + time.Now().UTC().Format("20060102T150405Z") + "-" + id.String() + ".mlbackup"
	archive, err := tool.Create(ctx, registered.Connection, password, directory, fileName)
	if err != nil {
		return systemdb.Backup{}, err
	}
	backup, err := database.Backups.Add(ctx, systemdb.Backup{
		ID: id, DatabaseID: databaseID, FileName: archive.FileName,
		SizeBytes: archive.SizeBytes, SHA256: archive.SHA256,
	})
	if err != nil {
		if cleanupErr := os.Remove(filepath.Join(directory, archive.FileName)); cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
		return systemdb.Backup{}, err
	}
	_, _ = database.Audit.Write(ctx, systemdb.AuditEvent{
		Level: "info", Code: "database.backup_created", DatabaseID: &databaseID,
		Message: "Local database backup created", Details: map[string]any{"backupId": backup.ID.String(), "sizeBytes": backup.SizeBytes},
	})
	return backup, nil
}

// ListDatabaseBackups returns the verified archive catalog for one database.
func (runtime *Runtime) ListDatabaseBackups(ctx context.Context, databaseID uuid.UUID) ([]systemdb.Backup, error) {
	runtime.mu.RLock()
	database := runtime.database
	runtime.mu.RUnlock()
	if database == nil {
		return nil, fmt.Errorf("ML System PostgreSQL is not configured")
	}
	if _, err := database.Databases.Get(ctx, databaseID); err != nil {
		return nil, err
	}
	return database.Backups.List(ctx, databaseID)
}

// DeleteDatabaseBackup removes a local archive and its catalog entry.
func (runtime *Runtime) DeleteDatabaseBackup(ctx context.Context, databaseID, backupID uuid.UUID) error {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.database == nil {
		return fmt.Errorf("ML System PostgreSQL is not configured")
	}
	backup, err := runtime.database.Backups.Get(ctx, backupID)
	if err != nil {
		return err
	}
	if backup.DatabaseID != databaseID {
		return systemdb.ErrBackupNotFound
	}
	directory, err := runtime.configuration.BackupDirectory()
	if err != nil {
		return err
	}
	path := filepath.Join(directory, backup.FileName)
	quarantine := path + ".deleting-" + backup.ID.String()
	fileMissing := false
	if err := os.Rename(path, quarantine); errors.Is(err, os.ErrNotExist) {
		fileMissing = true
	} else if err != nil {
		return fmt.Errorf("prepare backup deletion: %w", err)
	}
	if _, err := runtime.database.Backups.Delete(ctx, backupID); err != nil {
		if !fileMissing {
			if restoreErr := os.Rename(quarantine, path); restoreErr != nil {
				err = errors.Join(err, restoreErr)
			}
		}
		return err
	}
	if !fileMissing {
		if err := os.Remove(quarantine); err != nil {
			return fmt.Errorf("remove backup archive: %w", err)
		}
	}
	_, _ = runtime.database.Audit.Write(ctx, systemdb.AuditEvent{
		Level: "warning", Code: "database.backup_deleted", DatabaseID: &databaseID,
		Message: "Local database backup deleted", Details: map[string]any{"backupId": backupID.String()},
	})
	return nil
}

// RestoreDatabaseBackup atomically restores a verified archive into its stopped source database.
func (runtime *Runtime) RestoreDatabaseBackup(ctx context.Context, databaseID, backupID uuid.UUID) error {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.database == nil {
		return fmt.Errorf("ML System PostgreSQL is not configured")
	}
	if runtime.backupTool == nil {
		return fmt.Errorf("PostgreSQL backup tools are unavailable")
	}
	registered, err := runtime.database.Databases.Get(ctx, databaseID)
	if err != nil {
		return err
	}
	if registered.State != systemdb.DatabaseStopped {
		return fmt.Errorf("database must be stopped before restore")
	}
	backup, err := runtime.database.Backups.Get(ctx, backupID)
	if err != nil {
		return err
	}
	if backup.DatabaseID != databaseID {
		return systemdb.ErrBackupNotFound
	}
	password, err := runtime.secrets.Get(registered.Connection.SecretKey)
	if err != nil {
		return err
	}
	directory, err := runtime.configuration.BackupDirectory()
	if err != nil {
		return err
	}
	if err := runtime.backupTool.Restore(
		ctx, registered.Connection, password, filepath.Join(directory, backup.FileName), backup.SHA256,
	); err != nil {
		return err
	}
	physicalID, err := appdb.EnsureIdentity(ctx, registered.Connection, password)
	if err != nil {
		return err
	}
	if physicalID != registered.PhysicalID {
		return fmt.Errorf("restored backup has a different physical database identity")
	}
	_, _ = runtime.database.Audit.Write(ctx, systemdb.AuditEvent{
		Level: "warning", Code: "database.backup_restored", DatabaseID: &databaseID,
		Message: "Local database backup restored", Details: map[string]any{"backupId": backup.ID.String()},
	})
	return nil
}

func (request ProvisionRequest) administratorDescriptor() (postgresconn.Descriptor, error) {
	descriptor := postgresconn.Descriptor{
		Host: request.Host, Port: request.Port, Database: request.AdministratorDatabase,
		User: request.AdministratorUser, SSLMode: request.SSLMode, SecretKey: "postgres.bootstrap.password",
	}
	if err := descriptor.Validate(); err != nil {
		return postgresconn.Descriptor{}, err
	}
	if request.AdministratorPassword == "" && !strings.HasPrefix(descriptor.Host, "/") {
		return postgresconn.Descriptor{}, fmt.Errorf("PostgreSQL administrator password is required")
	}
	if !descriptor.IsLocal() && descriptor.SSLMode == "disable" {
		return postgresconn.Descriptor{}, fmt.Errorf("TLS cannot be disabled for a remote PostgreSQL server")
	}
	return descriptor, nil
}

func validateApplicationConnection(descriptor postgresconn.Descriptor, password string) error {
	if err := descriptor.Validate(); err != nil {
		return err
	}
	if password == "" {
		return fmt.Errorf("PostgreSQL password is required")
	}
	if !descriptor.IsLocal() && descriptor.SSLMode == "disable" {
		return fmt.Errorf("TLS cannot be disabled for a remote PostgreSQL server")
	}
	return nil
}

func (runtime *Runtime) storeDatabase(
	ctx context.Context,
	name string,
	mode systemdb.DatabaseMode,
	sourceID *uuid.UUID,
	descriptor postgresconn.Descriptor,
	password string,
	physicalID uuid.UUID,
) (systemdb.RegisteredDatabase, error) {
	id, err := uuid.New()
	if err != nil {
		return systemdb.RegisteredDatabase{}, err
	}
	descriptor.SecretKey = "database." + id.String() + ".password"
	if err := runtime.secrets.Set(descriptor.SecretKey, password); err != nil {
		return systemdb.RegisteredDatabase{}, err
	}
	registered, err := runtime.database.Databases.Register(ctx, systemdb.DatabaseRegistration{
		ID: id, Name: name, PhysicalID: physicalID, Connection: descriptor, Mode: mode, SourceDatabaseID: sourceID,
	})
	if err != nil {
		if cleanupErr := runtime.secrets.Delete(descriptor.SecretKey); cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
		return systemdb.RegisteredDatabase{}, err
	}
	return registered, nil
}

func openSystemDatabase(ctx context.Context, descriptor postgresconn.Descriptor, secrets Secrets) (*systemdb.Database, error) {
	password, err := secrets.Get(descriptor.SecretKey)
	if err != nil {
		return nil, err
	}
	configuration, err := descriptor.PoolConfig(password)
	if err != nil {
		return nil, err
	}
	return systemdb.OpenConfig(ctx, configuration)
}
