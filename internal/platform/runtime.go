// Package platform coordinates ML Manager with ML System and protected secrets.
package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/appdb"
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

// State is safe to expose to ML Manager UI.
type State struct {
	Configured bool                     `json:"configured"`
	Connected  bool                     `json:"connected"`
	Connection *postgresconn.Descriptor `json:"connection,omitempty"`
	Error      string                   `json:"error,omitempty"`
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
}

// New opens an already configured ML System. A connection problem remains visible in State.
func New(ctx context.Context, configuration appconfig.Config, secrets Secrets) *Runtime {
	runtime := &Runtime{configuration: configuration, secrets: secrets}
	runtime.copier, runtime.copierErr = pgcopy.New()
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
