package platform

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/postgresadmin"
	"github.com/k33alexey/MetaLab/internal/postgresconn"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestManagerIdentityAndDatabaseScopeIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_ADMIN_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_ADMIN_DATABASE_URL is not set")
	}
	administrator, password := platformDescriptorFromURL(t, databaseURL)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	configuration := appconfig.Default()
	configuration.SourcePath = t.TempDir() + "/config.yaml"
	runtime := New(ctx, configuration, &memorySecrets{values: make(map[string]string)})
	_, err := runtime.ProvisionPostgreSQL(ctx, ProvisionRequest{Host: administrator.Host, Port: administrator.Port, AdministratorDatabase: administrator.Database, AdministratorUser: administrator.User, AdministratorPassword: password, SSLMode: administrator.SSLMode, SystemDatabase: "ml_manager_" + suffix, TechnicalUser: "ml_mgr_role_" + suffix})
	if err != nil {
		runtime.Close()
		t.Fatal(err)
	}
	connection := *runtime.State().Connection
	t.Cleanup(func() {
		runtime.Close()
		cleanup, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		if err := postgresadmin.RollbackProvisioned(cleanup, administrator, password, postgresadmin.Provisioned{Connection: connection}); err != nil {
			t.Error(err)
		}
	})
	admin, _, err := runtime.CreateInitial(ctx, "administrator", "administrator password")
	if err != nil {
		t.Fatal(err)
	}
	adminLogin, err := runtime.LoginManager(ctx, admin.Login, "administrator password", "local", "test")
	if err != nil {
		t.Fatal(err)
	}
	adminContext := WithManagerToken(ctx, adminLogin.Token)
	developer, err := runtime.CreateManagerUser(adminContext, systemdb.UserCreation{Login: "developer", Password: "initial developer password", MetadataAdministrator: true})
	if err != nil {
		t.Fatal(err)
	}
	developerLogin, err := runtime.LoginManager(ctx, developer.Login, "initial developer password", "local", "test")
	if err != nil {
		t.Fatal(err)
	}
	developerContext := WithManagerToken(ctx, developerLogin.Token)
	if _, err := runtime.ListManagerDatabases(developerContext); !errors.Is(err, systemdb.ErrPasswordChangeRequired) {
		t.Fatalf("temporary password bypass: %v", err)
	}
	if err := runtime.ChangeManagerPassword(ctx, developerLogin.Token, "initial developer password", "private developer password"); err != nil {
		t.Fatal(err)
	}
	portal, err := runtime.LoginPortal(ctx, developer.Login, "private developer password", "browser", "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ListManagerDatabases(WithManagerToken(ctx, portal.Token)); !errors.Is(err, systemdb.ErrSessionNotFound) {
		t.Fatalf("Portal authorized Manager: %v", err)
	}
	if _, err := runtime.ListManagerDatabases(ctx); !errors.Is(err, systemdb.ErrSessionNotFound) {
		t.Fatalf("missing Manager token: %v", err)
	}
	first, second := uuid.MustNew(), uuid.MustNew()
	for index, id := range []uuid.UUID{first, second} {
		ownerID := admin.ID
		if index == 0 {
			ownerID = developer.ID
		}
		_, err := runtime.database.Databases.Register(ctx, systemdb.DatabaseRegistration{ID: id, Name: []string{"Developer database", "Foreign database"}[index], PhysicalID: uuid.MustNew(), Mode: systemdb.DatabaseDebug, OwnerUserID: &ownerID, Connection: postgresconn.Descriptor{Host: "localhost", Port: 5432, Database: "manager_registry_test", User: "metalab", SSLMode: "disable", SecretKey: "test." + id.String()}})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := runtime.SetManagerDatabasePermissions(adminContext, developer.ID, first, systemdb.DatabasePermissions{Studio: true}); err != nil {
		t.Fatal(err)
	}
	if items, err := runtime.ListManagerDatabases(developerContext); err != nil || len(items) != 1 || items[0].ID != first || items[0].Permissions.Admin || items[0].Permissions.App {
		t.Fatalf("developer scope: %+v %v", items, err)
	} else if items[0].Owner == nil || items[0].Owner.UserID != developer.ID || items[0].Owner.Login != developer.Login {
		t.Fatalf("visible database owner: %+v", items[0].Owner)
	}
	if items, err := runtime.ListManagerDatabases(adminContext); err != nil || len(items) != 2 || !items[0].Permissions.Admin {
		t.Fatalf("administrator scope: %+v %v", items, err)
	} else if items[1].Owner == nil || items[1].Owner.UserID != admin.ID || items[1].Owner.Login != admin.Login {
		t.Fatalf("foreign database owner: %+v", items[1].Owner)
	}
	if items, err := runtime.database.DatabaseAccess.ListApp(ctx, developer.ID); err != nil || len(items) != 0 {
		t.Fatalf("Studio leaked into Portal: %+v %v", items, err)
	}
	for _, check := range []struct {
		id         uuid.UUID
		permission string
	}{{first, "admin"}, {second, "studio"}, {second, "admin"}, {uuid.UUID{}, "platform"}} {
		if err := runtime.AuthorizeManager(developerContext, check.id, check.permission); err == nil {
			t.Fatalf("developer authorized %s", check.permission)
		}
	}
	if _, err := runtime.CreateManagerUser(developerContext, systemdb.UserCreation{Login: "unauthorized", Password: "test password"}); !errors.Is(err, systemdb.ErrManagerAccessDenied) {
		t.Fatalf("developer created user: %v", err)
	}
	if err := runtime.SetManagerDatabasePermissions(developerContext, developer.ID, second, systemdb.DatabasePermissions{Studio: true}); !errors.Is(err, systemdb.ErrDatabaseOwnerOnly) {
		t.Fatalf("foreign grant: %v", err)
	}
	projectID := uuid.MustNew()
	lease, err := runtime.AcquireStudioSession(developerContext, first, projectID, "fake OS name", "host", 100)
	if err != nil {
		t.Fatal(err)
	}
	if lease.Session.OwnerName != developer.Login {
		t.Fatalf("owner=%s", lease.Session.OwnerName)
	}
	if err := runtime.AuthorizeStudioLease(ctx, lease); err != nil {
		t.Fatal(err)
	}
	wrong := lease
	wrong.Session.DatabaseID = second
	if err := runtime.AuthorizeStudioLease(ctx, wrong); !errors.Is(err, systemdb.ErrStudioSessionNotFound) {
		t.Fatalf("lease database swap: %v", err)
	}
	if err := runtime.SetManagerDatabasePermissions(adminContext, developer.ID, first, systemdb.DatabasePermissions{App: true}); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AuthorizeStudioLease(ctx, lease); !errors.Is(err, systemdb.ErrStudioSessionNotFound) {
		t.Fatalf("revoked Studio request: %v", err)
	}
	if items, err := runtime.ListManagerDatabases(developerContext); err != nil || len(items) != 0 {
		t.Fatalf("App-only in Manager: %+v %v", items, err)
	}
	if err := runtime.LogoutManager(ctx, developerLogin.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.AuthenticatePortal(ctx, portal.Token); err != nil {
		t.Fatalf("Manager logout killed Portal: %v", err)
	}
}
