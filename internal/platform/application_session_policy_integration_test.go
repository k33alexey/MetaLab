package platform

import (
	"bytes"
	"context"
	"encoding/json"
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

// A restriction of access is a condition in the query language, and running
// it is block 10's. Until then a right a role restricts admits no record:
// the list of a restricted catalog comes back empty and none of its objects
// opens, while a catalog the role does not restrict reads whole - the
// restriction never shows more than it would, and never more than its object.
func TestRestrictedReadAdmitsNoRecordIntegration(t *testing.T) {
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
	// ТекущийПользователь объявлен самой конфигурацией, как во всех трёх
	// проверочных; значение ему даёт её модуль сеанса, а не платформа.
	tasksID, performerID := uuid.MustNew(), uuid.MustNew()
	tasks := metadata.CatalogDefinition{
		Format: 1, ID: tasksID, Name: "Задачи", Title: metadata.LocalizedText{"ru": "Задачи"},
		Code: metadata.CatalogCode{Type: metadata.StringType, Length: 9, Auto: true, Unique: true}, DescriptionLength: 150,
		Attributes: []metadata.Attribute{
			{ID: performerID, Name: "Исполнитель", Title: metadata.LocalizedText{"ru": "Исполнитель"}, Types: []metadata.Type{{Kind: metadata.StringType, Length: 50}}},
		},
	}
	currentUser := metadata.SessionParameter{
		Format: 1, ID: uuid.MustNew(), Name: "ТекущийПользователь", Title: metadata.LocalizedText{"ru": "Текущий пользователь"},
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
		if role, ok := value.(metadata.RoleDefinition); ok {
			if err := metadata.WriteRole(root, role); err != nil {
				t.Fatal(err)
			}
			return
		}
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
	write("metadata/catalogs/"+tasks.Name+"/object.yaml", tasks)
	write("metadata/session-parameters/"+currentUser.ID.String()+".yaml", currentUser)
	if err := os.WriteFile(filepath.Join(root, project.SessionModuleFile), []byte(`Процедура УстановкаПараметровСеанса(ИменаПараметровСеанса)
    Склады = Новый Массив;
    Склады.Добавить("Основной");
    Склады.Добавить("Розничный");
    ПараметрыСеанса.ДоступныеСклады = Склады;
    ПараметрыСеанса.ТекущийПользователь = "Иванов";
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
			permission.Restrictions = []metadata.RightRestriction{{Right: metadata.PermissionRead,
				Condition: "ГДЕ Склад В (&ДоступныеСклады)"}}
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
	orders := []string{}
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
		orders = append(orders, record.Reference.ObjectID.String())
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
	if len(page.Rows) != 0 {
		t.Fatalf("a restriction not run yet admitted records: %+v", page.Rows)
	}
	references := []string{}
	for _, performer := range []string{"Иванов", "Петров"} {
		record, err := repository.New(ctx, "Задачи", nil)
		if err != nil {
			t.Fatal(err)
		}
		record.Description = performer
		record.Attributes[performerID] = metadata.Value{Kind: metadata.StringType, Data: performer}
		if err := repository.Save(ctx, record, nil); err != nil {
			t.Fatal(err)
		}
		references = append(references, record.Reference.ObjectID.String())
	}
	page, err = runtime.LoadApplicationList(ctx, login.Token, registered.ID, metadata.CatalogKind, "Задачи", metadata.DynamicListRequest{Limit: 20})
	if err != nil {
		t.Fatalf("unrestricted task list read: %v", err)
	}
	if len(page.Rows) != 2 {
		t.Fatalf("the restriction of another catalog narrowed this one: %+v", page.Rows)
	}
	if _, err := runtime.GetApplicationObject(ctx, login.Token, registered.ID, metadata.CatalogKind, "Задачи", references[0]); err != nil {
		t.Fatalf("an unrestricted object did not open: %v", err)
	}
	// Открытие объекта идёт своим путём, не через список, и ограничение
	// действует и в нём.
	if _, err := runtime.GetApplicationObject(ctx, login.Token, registered.ID, metadata.CatalogKind, "Заказы", orders[0]); err == nil {
		t.Fatal("an object of a restricted catalog opened")
	}

	// A base saved by an earlier version of ML, while it still kept the thin
	// client, does not open, and the refusal names the role and the right
	// (ACCESS-RIGHTS, «Роли, сохранённые прежней версией ML»): the saved
	// state is given the right as that version wrote it. Defect caught: the
	// reason thrown away on the way to ML App («База недоступна», 2.444).
	saved, found, err := publication.CurrentDatabaseState(ctx, pool)
	if err != nil || !found {
		t.Fatalf("saved state: found=%v error=%v", found, err)
	}
	saved.Roles[0].Objects[0].Operations = append(saved.Roles[0].Objects[0].Operations, "thin-client")
	encoded, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE ml_core.database_state SET runtime = $1 WHERE singleton", encoded); err != nil {
		t.Fatal(err)
	}
	var notRepaired *metadata.RoleNotRepaired
	if _, err := runtime.LoadApplicationObjects(ctx, login.Token, registered.ID, nil); !errors.As(err, &notRepaired) ||
		notRepaired.Role != "Кладовщик" || notRepaired.Right != "thin-client" || notRepaired.RightName != "ТонкийКлиент" || notRepaired.Denies {
		t.Fatalf("a base with a role of an earlier version opened as %v, want the role Кладовщик and the right thin-client named", err)
	}
}
