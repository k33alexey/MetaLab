package platform

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/postgresadmin"
	"github.com/k33alexey/MetaLab/internal/postgresconn"
	"github.com/k33alexey/MetaLab/internal/secretstore"
	"github.com/k33alexey/MetaLab/internal/systemdb"
)

func TestProvisionPersistsAndReopensMLSystemIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_ADMIN_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_ADMIN_DATABASE_URL is not set")
	}
	administrator, administratorPassword := platformDescriptorFromURL(t, databaseURL)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	request := ProvisionRequest{
		Host: administrator.Host, Port: administrator.Port,
		AdministratorDatabase: administrator.Database, AdministratorUser: administrator.User,
		AdministratorPassword: administratorPassword, SSLMode: administrator.SSLMode,
		SystemDatabase: "ml_platform_db_" + suffix, TechnicalUser: "ml_platform_role_" + suffix,
	}
	configuration := appconfig.Default()
	configuration.SourcePath = t.TempDir() + "/config.yaml"
	secrets := &memorySecrets{values: make(map[string]string)}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	runtime := New(ctx, configuration, secrets)
	check, err := runtime.ProvisionPostgreSQL(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if !check.CanProvision() || !runtime.State().Connected {
		t.Fatalf("check=%+v state=%+v", check, runtime.State())
	}
	provisionedConnection := *runtime.State().Connection
	cleaned := false
	t.Cleanup(func() {
		if cleaned {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		_ = postgresadmin.RollbackProvisioned(cleanupCtx, administrator, administratorPassword, postgresadmin.Provisioned{Connection: provisionedConnection})
	})
	if _, _, err := runtime.CreateInitial(ctx, "admin", "platform integration password"); err != nil {
		t.Fatal(err)
	}
	runtime.Close()
	loaded, _, err := appconfig.Load(configuration.SourcePath)
	if err != nil {
		t.Fatal(err)
	}
	reopened := New(ctx, loaded, secrets)
	required, err := reopened.InitialSetupRequired(ctx)
	if err != nil || required {
		t.Fatalf("required=%v error=%v state=%+v", required, err, reopened.State())
	}
	reopened.Close()
	connection := loaded.SystemDatabase
	if connection == nil {
		t.Fatalf("saved configuration = %+v", loaded)
	}
	if err := postgresadmin.RollbackProvisioned(ctx, administrator, administratorPassword, postgresadmin.Provisioned{Connection: *connection}); err != nil {
		t.Fatal(err)
	}
	cleaned = true
}

func TestProvisionRollsBackWhenProtectedStoreFailsIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_ADMIN_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_ADMIN_DATABASE_URL is not set")
	}
	administrator, administratorPassword := platformDescriptorFromURL(t, databaseURL)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	request := ProvisionRequest{
		Host: administrator.Host, Port: administrator.Port,
		AdministratorDatabase: administrator.Database, AdministratorUser: administrator.User,
		AdministratorPassword: administratorPassword, SSLMode: administrator.SSLMode,
		SystemDatabase: "ml_rollback_db_" + suffix, TechnicalUser: "ml_rollback_role_" + suffix,
	}
	configuration := appconfig.Default()
	configuration.SourcePath = t.TempDir() + "/config.yaml"
	runtime := New(context.Background(), configuration, failingSecrets{})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := runtime.ProvisionPostgreSQL(ctx, request); err == nil || !strings.Contains(err.Error(), "protected store rejected secret") {
		t.Fatalf("ProvisionPostgreSQL() error = %v", err)
	}
	poolConfiguration, err := administrator.PoolConfig(administratorPassword)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfiguration)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var databaseExists, roleExists bool
	if err := pool.QueryRow(ctx, `
SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_database WHERE datname = $1),
       EXISTS(SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = $2)`,
		request.SystemDatabase, request.TechnicalUser).Scan(&databaseExists, &roleExists); err != nil {
		t.Fatal(err)
	}
	if databaseExists || roleExists {
		t.Fatalf("rollback left database=%v role=%v", databaseExists, roleExists)
	}
}

func TestApplicationDatabaseRegistryRejectsSamePhysicalDatabaseIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_ADMIN_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_ADMIN_DATABASE_URL is not set")
	}
	administrator, administratorPassword := platformDescriptorFromURL(t, databaseURL)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	systemRequest := ProvisionRequest{
		Host: administrator.Host, Port: administrator.Port,
		AdministratorDatabase: administrator.Database, AdministratorUser: administrator.User,
		AdministratorPassword: administratorPassword, SSLMode: administrator.SSLMode,
		SystemDatabase: "ml_registry_system_" + suffix, TechnicalUser: "ml_registry_system_role_" + suffix,
	}
	configuration := appconfig.Default()
	configuration.SourcePath = t.TempDir() + "/config.yaml"
	secrets := &memorySecrets{values: make(map[string]string)}
	runtime := New(ctx, configuration, secrets)
	if _, err := runtime.ProvisionPostgreSQL(ctx, systemRequest); err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	systemConnection := *runtime.State().Connection
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		_ = postgresadmin.RollbackProvisioned(cleanupCtx, administrator, administratorPassword, postgresadmin.Provisioned{Connection: systemConnection})
	})
	application, err := postgresadmin.Provision(
		ctx, administrator, administratorPassword, "ml_registry_app_"+suffix, "ml_registry_app_role_"+suffix,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		_ = postgresadmin.RollbackProvisioned(cleanupCtx, administrator, administratorPassword, application)
	})
	request := RegisterDatabaseRequest{
		Name: "Продажи " + suffix, Host: application.Connection.Host, Port: application.Connection.Port,
		Database: application.Connection.Database, User: application.Connection.User,
		Password: application.Password, SSLMode: application.Connection.SSLMode,
	}
	registered, err := runtime.RegisterDatabase(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if registered.State != systemdb.DatabaseStopped || registered.PhysicalID.IsZero() || registered.Connection.SecretKey == "" {
		t.Fatalf("registered = %+v", registered)
	}
	request.Name = "Та же база через вторую запись " + suffix
	if _, err := runtime.RegisterDatabase(ctx, request); !errors.Is(err, systemdb.ErrPhysicalDatabaseExists) {
		t.Fatalf("duplicate physical database error = %v", err)
	}
	items, err := runtime.ListDatabases(ctx)
	if err != nil || len(items) != 1 || items[0].ID != registered.ID {
		t.Fatalf("items=%+v error=%v", items, err)
	}
	if err := runtime.UnregisterDatabase(ctx, registered.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := secrets.Get(registered.Connection.SecretKey); !errors.Is(err, secretstore.ErrNotFound) {
		t.Fatalf("protected password after unregister error = %v", err)
	}
}

type memorySecrets struct {
	mu     sync.Mutex
	values map[string]string
}

type failingSecrets struct{}

func (failingSecrets) Set(string, string) error   { return errors.New("protected store rejected secret") }
func (failingSecrets) Get(string) (string, error) { return "", secretstore.ErrNotFound }
func (failingSecrets) Delete(string) error        { return nil }

func (store *memorySecrets) Set(key, value string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.values[key] = value
	return nil
}
func (store *memorySecrets) Get(key string) (string, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	value, exists := store.values[key]
	if !exists {
		return "", secretstore.ErrNotFound
	}
	return value, nil
}
func (store *memorySecrets) Delete(key string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.values[key]; !exists {
		return errors.New("secret does not exist")
	}
	delete(store.values, key)
	return nil
}

func platformDescriptorFromURL(t *testing.T, databaseURL string) (postgresconn.Descriptor, string) {
	t.Helper()
	configuration, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	sslMode := parsed.Query().Get("sslmode")
	if sslMode == "" {
		sslMode = "require"
	}
	return postgresconn.Descriptor{
		Host: configuration.ConnConfig.Host, Port: configuration.ConnConfig.Port,
		Database: configuration.ConnConfig.Database, User: configuration.ConnConfig.User,
		SSLMode: sslMode, SecretKey: "test.admin.password",
	}, configuration.ConnConfig.Password
}
