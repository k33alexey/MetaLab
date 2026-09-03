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

func TestCreateDebugDatabaseCopiesOrStartsCleanIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_ADMIN_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_ADMIN_DATABASE_URL is not set")
	}
	administrator, administratorPassword := platformDescriptorFromURL(t, databaseURL)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	configuration := appconfig.Default()
	configuration.SourcePath = t.TempDir() + "/config.yaml"
	secrets := &memorySecrets{values: make(map[string]string)}
	runtime := New(ctx, configuration, secrets)
	system, err := postgresadmin.Provision(
		ctx, administrator, administratorPassword,
		"ml_debug_system_"+suffix, "ml_debug_system_role_"+suffix,
	)
	if err != nil {
		t.Fatal(err)
	}
	runtime.database, err = systemdb.OpenConfig(ctx, mustPoolConfig(t, system.Connection, system.Password))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	provisioned := []postgresadmin.Provisioned{system}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		for index := len(provisioned) - 1; index >= 0; index-- {
			_ = postgresadmin.RollbackProvisioned(cleanupCtx, administrator, administratorPassword, provisioned[index])
		}
	})
	source, err := postgresadmin.Provision(
		ctx, administrator, administratorPassword,
		"ml_debug_source_"+suffix, "ml_debug_source_role_"+suffix,
	)
	if err != nil {
		t.Fatal(err)
	}
	provisioned = append(provisioned, source)
	registeredSource, err := runtime.RegisterDatabase(ctx, RegisterDatabaseRequest{
		Name: "Основная " + suffix, Host: source.Connection.Host, Port: source.Connection.Port,
		Database: source.Connection.Database, User: source.Connection.User, Password: source.Password,
		SSLMode: source.Connection.SSLMode, Mode: systemdb.DatabasePrimary,
	})
	if err != nil {
		t.Fatal(err)
	}
	sourcePool, err := pgxpool.NewWithConfig(ctx, mustPoolConfig(t, source.Connection, source.Password))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sourcePool.Exec(ctx, "CREATE TABLE copied_probe(value TEXT NOT NULL); INSERT INTO copied_probe VALUES ('from primary')"); err != nil {
		sourcePool.Close()
		t.Fatal(err)
	}
	sourcePool.Close()

	copyRequest := debugRequest(administrator, administratorPassword, suffix, "copy")
	copied, err := runtime.CreateDebugDatabase(ctx, registeredSource.ID, copyRequest)
	if err != nil {
		t.Fatal(err)
	}
	copyPassword, err := secrets.Get(copied.Connection.SecretKey)
	if err != nil {
		t.Fatal(err)
	}
	provisioned = append(provisioned, postgresadmin.Provisioned{Connection: copied.Connection, Password: copyPassword})
	if copied.Mode != systemdb.DatabaseDebug || copied.SourceDatabaseID == nil || *copied.SourceDatabaseID != registeredSource.ID {
		t.Fatalf("copied debug registry entry = %+v", copied)
	}
	if copied.PhysicalID == registeredSource.PhysicalID {
		t.Fatal("debug copy reused the primary physical database identity")
	}
	copyPool, err := pgxpool.NewWithConfig(ctx, mustPoolConfig(t, copied.Connection, copyPassword))
	if err != nil {
		t.Fatal(err)
	}
	var copiedValue string
	if err := copyPool.QueryRow(ctx, "SELECT value FROM copied_probe").Scan(&copiedValue); err != nil {
		copyPool.Close()
		t.Fatal(err)
	}
	copyPool.Close()
	if copiedValue != "from primary" {
		t.Fatalf("copied value = %q", copiedValue)
	}

	cleanRequest := debugRequest(administrator, administratorPassword, suffix, "clean")
	cleanRequest.CopyData = false
	clean, err := runtime.CreateDebugDatabase(ctx, registeredSource.ID, cleanRequest)
	if err != nil {
		t.Fatal(err)
	}
	cleanPassword, err := secrets.Get(clean.Connection.SecretKey)
	if err != nil {
		t.Fatal(err)
	}
	provisioned = append(provisioned, postgresadmin.Provisioned{Connection: clean.Connection, Password: cleanPassword})
	cleanPool, err := pgxpool.NewWithConfig(ctx, mustPoolConfig(t, clean.Connection, cleanPassword))
	if err != nil {
		t.Fatal(err)
	}
	var copiedTableExists bool
	if err := cleanPool.QueryRow(ctx, "SELECT to_regclass('copied_probe') IS NOT NULL").Scan(&copiedTableExists); err != nil {
		cleanPool.Close()
		t.Fatal(err)
	}
	cleanPool.Close()
	if copiedTableExists {
		t.Fatal("clean debug database contains source business data")
	}
	runtime.copier = failingDatabaseCopier{}
	failedRequest := debugRequest(administrator, administratorPassword, suffix, "failed")
	if _, err := runtime.CreateDebugDatabase(ctx, registeredSource.ID, failedRequest); err == nil || !strings.Contains(err.Error(), "forced copy failure") {
		t.Fatalf("failed debug copy error = %v", err)
	}
	adminPool, err := pgxpool.NewWithConfig(ctx, mustPoolConfig(t, administrator, administratorPassword))
	if err != nil {
		t.Fatal(err)
	}
	var failedDatabaseExists, failedRoleExists bool
	if err := adminPool.QueryRow(ctx, `
SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_database WHERE datname = $1),
       EXISTS(SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = $2)`,
		failedRequest.TargetDatabase, failedRequest.TechnicalUser,
	).Scan(&failedDatabaseExists, &failedRoleExists); err != nil {
		adminPool.Close()
		t.Fatal(err)
	}
	adminPool.Close()
	if failedDatabaseExists || failedRoleExists {
		t.Fatalf("failed copy left database=%v role=%v", failedDatabaseExists, failedRoleExists)
	}
	if err := runtime.UnregisterDatabase(ctx, registeredSource.ID); !errors.Is(err, systemdb.ErrDatabaseHasDebugCopies) {
		t.Fatalf("unregister source with debug copies error = %v", err)
	}
	if err := runtime.UnregisterDatabase(ctx, copied.ID); err != nil {
		t.Fatal(err)
	}
	if err := runtime.UnregisterDatabase(ctx, clean.ID); err != nil {
		t.Fatal(err)
	}
	if err := runtime.UnregisterDatabase(ctx, registeredSource.ID); err != nil {
		t.Fatal(err)
	}
}

type failingDatabaseCopier struct{}

func (failingDatabaseCopier) Copy(context.Context, postgresconn.Descriptor, string, int, postgresconn.Descriptor, string, int) error {
	return errors.New("forced copy failure")
}

func debugRequest(administrator postgresconn.Descriptor, password, suffix, variant string) CreateDebugDatabaseRequest {
	return CreateDebugDatabaseRequest{
		Name: "Отладка " + variant + " " + suffix, CopyData: true,
		Host: administrator.Host, Port: administrator.Port,
		AdministratorDatabase: administrator.Database, AdministratorUser: administrator.User,
		AdministratorPassword: password, SSLMode: administrator.SSLMode,
		TargetDatabase: "ml_debug_" + variant + "_" + suffix,
		TechnicalUser:  "ml_debug_" + variant + "_role_" + suffix,
	}
}

func mustPoolConfig(t *testing.T, descriptor postgresconn.Descriptor, password string) *pgxpool.Config {
	t.Helper()
	configuration, err := descriptor.PoolConfig(password)
	if err != nil {
		t.Fatal(err)
	}
	return configuration
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
