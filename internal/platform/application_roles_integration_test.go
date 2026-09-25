package platform

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/postgresadmin"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/publication"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestManagerApplicationRolesUseActivePublicationIntegration(t *testing.T) {
	url := os.Getenv("ML_TEST_ADMIN_DATABASE_URL")
	if url == "" {
		t.Skip("ML_TEST_ADMIN_DATABASE_URL is not set")
	}
	administrator, password := platformDescriptorFromURL(t, url)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	configuration := appconfig.Default()
	configuration.SourcePath = filepath.Join(t.TempDir(), "config.yaml")
	runtime := New(ctx, configuration, &memorySecrets{values: map[string]string{}})
	provisioned := []postgresadmin.Provisioned{}
	t.Cleanup(func() {
		runtime.Close()
		cleanup, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		for index := len(provisioned) - 1; index >= 0; index-- {
			if err := postgresadmin.RollbackProvisioned(cleanup, administrator, password, provisioned[index]); err != nil {
				t.Error(err)
			}
		}
	})
	for _, kind := range []string{"sys", "app"} {
		value, err := postgresadmin.Provision(ctx, administrator, password, "ml_roles_"+kind+"_"+suffix, "ml_roles_"+kind+"_u_"+suffix)
		if err != nil {
			t.Fatal(err)
		}
		provisioned = append(provisioned, value)
	}
	var err error
	runtime.database, err = systemdb.OpenConfig(ctx, mustPoolConfig(t, provisioned[0].Connection, provisioned[0].Password))
	if err != nil {
		t.Fatal(err)
	}
	admin, _, err := runtime.CreateInitial(ctx, "administrator", "administrator password")
	if err != nil {
		t.Fatal(err)
	}
	login, err := runtime.LoginManager(ctx, admin.Login, "administrator password", "local", "roles test")
	if err != nil {
		t.Fatal(err)
	}
	adminContext := WithManagerToken(ctx, login.Token)
	member, err := runtime.database.Users.Create(ctx, systemdb.UserCreation{ID: uuid.MustNew(), Login: "member", Password: "member test password"})
	if err != nil {
		t.Fatal(err)
	}
	app := provisioned[1]
	registered, err := runtime.RegisterDatabase(ctx, RegisterDatabaseRequest{Name: "Roles test", Host: app.Connection.Host, Port: app.Connection.Port, Database: app.Connection.Database, User: app.Connection.User, Password: app.Password, SSLMode: app.Connection.SSLMode, Mode: systemdb.DatabaseDebug})
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.SetManagerDatabasePermissions(adminContext, member.ID, registered.ID, systemdb.DatabasePermissions{App: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.StartDatabase(ctx, registered.ID); err != nil {
		t.Fatal(err)
	}
	portal, err := runtime.LoginPortal(ctx, member.Login, "member test password", "browser", "roles test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.GetManagerApplicationRoles(WithManagerToken(ctx, portal.Token), registered.ID, member.ID); !errors.Is(err, systemdb.ErrSessionNotFound) {
		t.Fatalf("Portal token admitted to role editor: %v", err)
	}
	if _, err := runtime.GetManagerApplicationRoles(adminContext, registered.ID, member.ID); err == nil {
		t.Fatal("unpublished database offered roles")
	}
	pool, err := pgxpool.NewWithConfig(ctx, mustPoolConfig(t, app.Connection, app.Password))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	manifest := project.Project{Format: 1, ID: uuid.MustNew(), Name: "RoleDemo", Title: project.LocalizedText{"ru": "Role demo"}, DefaultLanguage: "ru", Languages: []project.Language{{ID: uuid.MustNew(), Name: "Русский", Title: "Русский", Code: "ru"}}}
	root := filepath.Join(t.TempDir(), "project")
	if err := project.Initialize(root, manifest); err != nil {
		t.Fatal(err)
	}
	constant := metadata.Constant{Format: 1, ID: uuid.MustNew(), Name: "Режим", Title: metadata.LocalizedText{"ru": "Режим"}, Types: []metadata.Type{{Kind: metadata.BooleanType}}}
	reader := metadata.RoleDefinition{Format: 1, ID: uuid.MustNew(), Name: "Читатель", Title: metadata.LocalizedText{"ru": "Читатель"}, Objects: []metadata.ObjectPermission{{Object: constant.ID, Operations: []metadata.PermissionOperation{metadata.PermissionRead}, Fields: []metadata.FieldPermission{{Field: "value", Operations: []metadata.PermissionOperation{metadata.PermissionRead}}}}}}
	write := func(relative string, value any) {
		t.Helper()
		var content bytes.Buffer
		if err := metadata.Encode(&content, value); err != nil {
			t.Fatal(err)
		}
		absolute := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, content.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("metadata/constants/"+constant.ID.String()+".yaml", constant)
	write("metadata/roles/"+reader.ID.String()+".yaml", reader)
	runTestGit(t, root, "init", "-b", "main")
	runTestGit(t, root, "config", "user.name", "MetaLab Test")
	runTestGit(t, root, "config", "user.email", "metalab-test@example.invalid")
	runTestGit(t, root, "add", "--all")
	runTestGit(t, root, "commit", "-m", "Initial project")
	if _, _, err := publication.SaveData(ctx, pool, publication.SaveDataRequest{Root: root, Mode: publication.ActivationDebug, Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	catalog, err := metadata.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	view, err := runtime.GetManagerApplicationRoles(adminContext, registered.ID, member.ID)
	if err != nil || view.ProjectID != manifest.ID || len(view.Available) != 1 || view.Available[0].ID != reader.ID || view.Assignment.Revision != 0 {
		t.Fatalf("published role choices: %+v %v", view, err)
	}
	// An unpublished role in the working directory must never become assignable.
	unpublished := reader
	unpublished.ID = uuid.MustNew()
	unpublished.Name = "НеОпубликована"
	write("metadata/roles/"+unpublished.ID.String()+".yaml", unpublished)
	for _, update := range []ApplicationRoleUpdate{{ProjectID: uuid.MustNew(), RoleIDs: []uuid.UUID{reader.ID}}, {ProjectID: manifest.ID, RoleIDs: []uuid.UUID{unpublished.ID}}, {ProjectID: manifest.ID, RoleIDs: []uuid.UUID{reader.ID, reader.ID}}} {
		if _, err := runtime.SetManagerApplicationRoles(adminContext, registered.ID, member.ID, update); err == nil {
			t.Fatal("invalid/unpublished role was assigned")
		}
	}
	assignment, err := runtime.SetManagerApplicationRoles(adminContext, registered.ID, member.ID, ApplicationRoleUpdate{ProjectID: manifest.ID, RoleIDs: []uuid.UUID{reader.ID}})
	if err != nil || assignment.Revision != 1 {
		t.Fatalf("assign published role: %+v %v", assignment, err)
	}
	stored, err := runtime.database.DatabaseAccess.ApplicationRoles(ctx, member.ID, registered.ID)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := permissionsForAssignment(catalog, stored)
	if err != nil || !policy.AllowsFields(constant.ID, metadata.PermissionRead, "value") || policy.AllowsFields(constant.ID, metadata.PermissionUpdate, "value") {
		t.Fatalf("resolved stored permissions: %v", err)
	}
	if _, err := runtime.SetManagerApplicationRoles(adminContext, registered.ID, member.ID, ApplicationRoleUpdate{ProjectID: manifest.ID, ExpectedRevision: 0}); !errors.Is(err, systemdb.ErrApplicationRolesChanged) {
		t.Fatalf("stale role editor: %v", err)
	}
	view, err = runtime.GetManagerApplicationRoles(adminContext, registered.ID, member.ID)
	if err != nil || len(view.Available) != 1 || view.Assignment.Revision != 1 || len(view.Assignment.RoleIDs) != 1 {
		t.Fatalf("role editor did not read active stored selection: %+v %v", view, err)
	}
}

func runTestGit(t *testing.T, root string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
}
