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
	"github.com/k33alexey/MetaLab/internal/postgresadmin"
	"github.com/k33alexey/MetaLab/internal/postgresconn"
	"github.com/k33alexey/MetaLab/internal/systemdb"
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
}

// New opens an already configured ML System. A connection problem remains visible in State.
func New(ctx context.Context, configuration appconfig.Config, secrets Secrets) *Runtime {
	runtime := &Runtime{configuration: configuration, secrets: secrets}
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
