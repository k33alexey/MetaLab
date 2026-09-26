package platform

import (
	"bytes"
	"context"
	"errors"
	"os"
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

// The point of the whole chain: a restriction compares a field with a session
// parameter, and the value of that parameter is computed by the solution's own
// session module. Nothing in the platform knows which warehouses a user may
// see; without the module running on the read path the restriction would have
// no value to compare against and the read would refuse.
func TestSessionParameterRestrictsListReadsIntegration(t *testing.T) {
	url := os.Getenv("ML_TEST_ADMIN_DATABASE_URL")
	if url == "" {
		t.Skip("ML_TEST_ADMIN_DATABASE_URL is not set")
	}
	administrator, password := platformDescriptorFromURL(t, url)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	settings := appconfig.Default()
	settings.SourcePath = filepath.Join(t.TempDir(), "config.yaml")
	runtime := New(ctx, settings, &memorySecrets{values: map[string]string{}})
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
		value, err := postgresadmin.Provision(ctx, administrator, password, "ml_sp_"+kind+"_"+suffix, "ml_sp_"+kind+"_u_"+suffix)
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
	admin, _, err := runtime.database.Administrators.CreateInitial(ctx, "admin-"+suffix, "session parameter password")
	if err != nil {
		t.Fatal(err)
	}
	app := provisioned[1]
	registered, err := runtime.RegisterDatabase(ctx, RegisterDatabaseRequest{
		Name: "Session parameter test", Host: app.Connection.Host, Port: app.Connection.Port, Database: app.Connection.Database,
		User: app.Connection.User, Password: app.Password, SSLMode: app.Connection.SSLMode, Mode: systemdb.DatabaseDebug,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.StartDatabase(ctx, registered.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.database.DatabaseAccess.GrantApp(ctx, admin.ID, admin.ID, registered.ID); err != nil {
		t.Fatal(err)
	}
	login, err := runtime.LoginPortal(ctx, "admin-"+suffix, "session parameter password", "127.0.0.1", "session parameter test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.OpenPortalDatabase(ctx, login.Token, registered.ID); err != nil {
		t.Fatal(err)
	}

	catalogID, warehouseAttributeID := uuid.MustNew(), uuid.MustNew()
	definition := metadata.CatalogDefinition{
		Format: 1, ID: catalogID, Name: "Заказы", Title: metadata.LocalizedText{"ru": "Заказы"},
		Code: metadata.CatalogCode{Type: metadata.StringType, Length: 9, Auto: true, Unique: true}, DescriptionLength: 150,
		Attributes: []metadata.Attribute{{ID: warehouseAttributeID, Name: "Склад", Title: metadata.LocalizedText{"ru": "Склад"},
			Types: []metadata.Type{{Kind: metadata.StringType, Length: 50}}}},
	}
	parameter := metadata.SessionParameter{
		Format: 1, ID: uuid.MustNew(), Name: "ДоступныеСклады", Title: metadata.LocalizedText{"ru": "Доступные склады"},
		Types: []metadata.Type{{Kind: metadata.StringType, Length: 50}},
	}
	configuration := project.Project{Format: 1, ID: uuid.MustNew(), Name: "SessionDemo", Title: project.LocalizedText{"ru": "Session demo"}, DefaultLanguage: "ru",
		Languages: []project.Language{{ID: uuid.MustNew(), Name: "Русский", Title: project.LocalizedText{"ru": "Русский"}, Code: "ru"}}}
	root := filepath.Join(t.TempDir(), "project")
	if err := project.Initialize(root, configuration); err != nil {
		t.Fatal(err)
	}
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
	write("metadata/catalogs/"+definition.Name+"/object.yaml", definition)
	write("metadata/session-parameters/"+parameter.ID.String()+".yaml", parameter)
	if err := os.WriteFile(filepath.Join(root, project.SessionModuleFile), []byte(`Процедура УстановкаПараметровСеанса(ИменаПараметровСеанса)
    Склады = Новый Массив;
    Склады.Добавить("Основной");
    Склады.Добавить("Розничный");
    ПараметрыСеанса.ДоступныеСклады = Склады;
КонецПроцедуры`), 0o600); err != nil {
		t.Fatal(err)
	}
	// Полные права на объект, суженные ограничением по списку из параметра сеанса.
	schema, err := metadata.LoadPermissionSchema(root)
	if err != nil {
		t.Fatal(err)
	}
	role := metadata.RoleDefinition{Format: 1, ID: uuid.MustNew(), Name: "Кладовщик", Title: metadata.LocalizedText{"ru": "Кладовщик"}}
	for _, object := range schema.Objects {
		permission := metadata.ObjectPermission{Object: object.ID, Operations: object.Operations}
		for _, field := range object.Fields {
			permission.Fields = append(permission.Fields, metadata.FieldPermission{Field: field.Key, Operations: field.Operations})
		}
		if object.ID == catalogID {
			permission.Policies = []metadata.AccessPolicy{{Operations: []metadata.PermissionOperation{metadata.PermissionRead},
				Rule: &metadata.PolicyRule{Field: warehouseAttributeID.String(), Operator: metadata.PolicyIn, Parameter: "ДоступныеСклады"}}}
		}
		role.Objects = append(role.Objects, permission)
	}
	write("metadata/roles/"+role.ID.String()+".yaml", role)
	runTestGit(t, root, "init", "-b", "main")
	runTestGit(t, root, "config", "user.name", "MetaLab Test")
	runTestGit(t, root, "config", "user.email", "metalab-test@example.invalid")
	runTestGit(t, root, "add", "--all")
	runTestGit(t, root, "commit", "-m", "Initial project")
	pool, err := pgxpool.NewWithConfig(ctx, mustPoolConfig(t, app.Connection, app.Password))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, _, err := publication.SaveData(ctx, pool, publication.SaveDataRequest{Root: root, Mode: publication.ActivationDebug, Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.database.DatabaseAccess.SetApplicationRoles(ctx, admin.ID, admin.ID, registered.ID, configuration.ID, []uuid.UUID{role.ID}, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.OpenPortalDatabase(ctx, login.Token, registered.ID); err != nil {
		t.Fatal(err)
	}

	// Строки пишутся напрямую в базу: запись под тем же ограничением - отдельный
	// разговор, здесь проверяется чтение.
	repository, err := metadata.NewCatalogRepository(pool, mustCatalog(t, root))
	if err != nil {
		t.Fatal(err)
	}
	for _, warehouse := range []string{"Основной", "Розничный", "Закрытый"} {
		record, err := repository.New(ctx, "Заказы", nil)
		if err != nil {
			t.Fatal(err)
		}
		record.Description = "Заказ " + warehouse
		record.Attributes[warehouseAttributeID] = metadata.Value{Kind: metadata.StringType, Data: warehouse}
		if err := repository.Save(ctx, record, nil); err != nil {
			t.Fatal(err)
		}
	}

	// Просмотр — отдельное право: та же роль без него видит объект из кода, но
	// ни в навигации, ни в списке приложения его не получает.
	withoutView := role
	withoutView.Objects = append([]metadata.ObjectPermission(nil), role.Objects...)
	for index := range withoutView.Objects {
		operations := make([]metadata.PermissionOperation, 0, len(withoutView.Objects[index].Operations))
		for _, operation := range withoutView.Objects[index].Operations {
			if operation != metadata.PermissionView {
				operations = append(operations, operation)
			}
		}
		withoutView.Objects[index].Operations = operations
	}
	write("metadata/roles/"+role.ID.String()+".yaml", withoutView)
	runTestGit(t, root, "add", "--all")
	runTestGit(t, root, "commit", "-m", "Role without the view right")
	if _, _, err := publication.SaveData(ctx, pool, publication.SaveDataRequest{Root: root, Mode: publication.ActivationDebug, Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	if listed, err := runtime.LoadApplicationObjects(ctx, login.Token, registered.ID, nil); err != nil || len(listed.Objects) != 0 {
		t.Fatalf("navigation without the view right = %+v error=%v", listed.Objects, err)
	}
	if _, err := runtime.LoadApplicationList(ctx, login.Token, registered.ID, metadata.CatalogKind, "Заказы", metadata.DynamicListRequest{Limit: 20}); !errors.Is(err, metadata.ErrPermissionDenied) {
		t.Fatalf("list without the view right = %v, want permission denied", err)
	}
	// Возвращаем право и убеждаемся, что дело именно в нём.
	write("metadata/roles/"+role.ID.String()+".yaml", role)
	runTestGit(t, root, "add", "--all")
	runTestGit(t, root, "commit", "-m", "Role with the view right")
	if _, _, err := publication.SaveData(ctx, pool, publication.SaveDataRequest{Root: root, Mode: publication.ActivationDebug, Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	page, err := runtime.LoadApplicationList(ctx, login.Token, registered.ID, metadata.CatalogKind, "Заказы", metadata.DynamicListRequest{Limit: 20})
	if err != nil {
		t.Fatalf("restricted list read: %v", err)
	}
	seen := map[string]bool{}
	for _, row := range page.Rows {
		seen[row.Values["Склад"]] = true
	}
	if len(page.Rows) != 2 || !seen["Основной"] || !seen["Розничный"] || seen["Закрытый"] {
		t.Fatalf("session module did not shape the restriction: %+v", page.Rows)
	}
}
