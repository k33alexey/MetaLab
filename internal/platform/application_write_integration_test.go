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

// TestApplicationObjectWritePathIntegration exercises the exact path a real
// ML App click takes: a portal session created via LoginPortal/OpenPortalDatabase,
// then GetApplicationObject/SaveApplicationObject/PostApplicationDocument/
// UndoApplicationDocumentPosting/SetApplicationDeletionMark - the same
// platform.Runtime methods internal/portal's HTTP routes call - against a
// real PostgreSQL database with BSL actually compiled from what "Сохранить
// данные" persisted, not a hand-assembled *metadata.Runtime like
// document_posting_integration_test.go uses.
func TestApplicationObjectWritePathIntegration(t *testing.T) {
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
		value, err := postgresadmin.Provision(ctx, administrator, password, "ml_write_"+kind+"_"+suffix, "ml_write_"+kind+"_u_"+suffix)
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
	admin, _, err := runtime.database.Administrators.CreateInitial(ctx, "admin-"+suffix, "write integration password")
	if err != nil {
		t.Fatal(err)
	}
	app := provisioned[1]
	registered, err := runtime.RegisterDatabase(ctx, RegisterDatabaseRequest{
		Name: "Write test", Host: app.Connection.Host, Port: app.Connection.Port, Database: app.Connection.Database,
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
	portalLogin, err := runtime.LoginPortal(ctx, "admin-"+suffix, "write integration password", "127.0.0.1", "write integration test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.OpenPortalDatabase(ctx, portalLogin.Token, registered.ID); err != nil {
		t.Fatal(err)
	}

	documentID, moduleID, registerID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	productAttributeID, quantityAttributeID, warehouseAttributeID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	productDimensionID, quantityResourceID := uuid.MustNew(), uuid.MustNew()
	document := metadata.DocumentDefinition{
		Format: 1, ID: documentID, Name: "Поступление", Title: metadata.LocalizedText{"ru": "Поступление"}, Posting: true,
		Number: metadata.DocumentNumber{Type: metadata.StringType, Length: 20, Auto: true, Unique: true, Periodicity: metadata.NumberPeriodYear},
		Attributes: []metadata.Attribute{
			{ID: productAttributeID, Name: "Товар", Title: metadata.LocalizedText{"ru": "Товар"}, Required: true, Types: []metadata.Type{{Kind: metadata.StringType, Length: 100}}},
			{ID: quantityAttributeID, Name: "Количество", Title: metadata.LocalizedText{"ru": "Количество"}, Required: true, Types: []metadata.Type{{Kind: metadata.NumberType, Precision: 15, Scale: 3}}},
			{ID: warehouseAttributeID, Name: "Склад", Title: metadata.LocalizedText{"ru": "Склад"}, Types: []metadata.Type{{Kind: metadata.StringType, Length: 50}}},
		},
		ObjectModule: &moduleID,
	}
	register := metadata.AccumulationRegisterDefinition{
		Format: 1, ID: registerID, Name: "ОстаткиТоваров", Title: metadata.LocalizedText{"ru": "Остатки товаров"},
		Kind: metadata.AccumulationRegisterBalance, Recorders: []uuid.UUID{documentID},
		Dimensions: []metadata.Attribute{{ID: productDimensionID, Name: "Товар", Title: metadata.LocalizedText{"ru": "Товар"}, Required: true, Types: []metadata.Type{{Kind: metadata.StringType, Length: 100}}}},
		Resources:  []metadata.Attribute{{ID: quantityResourceID, Name: "Количество", Title: metadata.LocalizedText{"ru": "Количество"}, Required: true, Types: []metadata.Type{{Kind: metadata.NumberType, Precision: 15, Scale: 3}}}},
	}
	manifest := project.Project{Format: 1, ID: uuid.MustNew(), Name: "WriteDemo", Title: "Write demo", DefaultLanguage: "ru", Languages: []project.Language{{ID: uuid.MustNew(), Name: "Русский", Title: "Русский", Code: "ru"}}}
	root := filepath.Join(t.TempDir(), "project")
	if err := project.Initialize(root, manifest); err != nil {
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
	// Сессионный параметр и модуль сеанса: значение не хранится нигде, его
	// вычисляет решение, и здесь проверяется, что оно доходит до прикладного
	// BSL на настоящем пути записи, а не только в модульном тесте.
	warehouseParameter := metadata.SessionParameter{
		Format: 1, ID: uuid.MustNew(), Name: "ДоступныйСклад", Title: metadata.LocalizedText{"ru": "Доступный склад"},
		Types: []metadata.Type{{Kind: metadata.StringType, Length: 50}},
	}
	write("metadata/session-parameters/"+warehouseParameter.ID.String()+".yaml", warehouseParameter)
	if err := os.WriteFile(filepath.Join(root, project.SessionModuleFile), []byte(`Процедура УстановкаПараметровСеанса(ИменаПараметровСеанса)
    ПараметрыСеанса.ДоступныйСклад = "Основной";
КонецПроцедуры`), 0o600); err != nil {
		t.Fatal(err)
	}
	write("metadata/documents/"+documentID.String()+"/object.yaml", document)
	write("metadata/accumulation-registers/"+registerID.String()+"/object.yaml", register)
	if err := os.WriteFile(filepath.Join(root, "metadata/documents", documentID.String(), moduleID.String()+".bsl"), []byte(`&НаСервере
Процедура ПередЗаписью(Отказ, РежимЗаписи, РежимПроведения)
    ЭтотОбъект.Склад = ПараметрыСеанса.ДоступныйСклад;
КонецПроцедуры

&НаСервере
Процедура ОбработкаПроведения(Отказ, РежимПроведения)
    Набор = РегистрыНакопления.ОстаткиТоваров.СоздатьНаборЗаписей();
    Набор.Отбор.Регистратор.Установить(ЭтотОбъект.Ссылка);
    Строка = Набор.Добавить();
    Строка.Период = ЭтотОбъект.Дата;
    Строка.Регистратор = ЭтотОбъект.Ссылка;
    Строка.ВидДвижения = ВидДвиженияНакопления.Приход;
    Строка.Товар = ЭтотОбъект.Товар;
    Строка.Количество = ЭтотОбъект.Количество;
    Набор.Записать();
КонецПроцедуры`), 0o600); err != nil {
		t.Fatal(err)
	}
	// ML App grants nothing without an assigned role - not even to the platform
	// administrator - so an authorized user has to be modelled explicitly.
	schema, err := metadata.LoadPermissionSchema(root)
	if err != nil {
		t.Fatal(err)
	}
	role := metadata.RoleDefinition{Format: 1, ID: uuid.MustNew(), Name: "Полный", Title: metadata.LocalizedText{"ru": "Полный"}}
	for _, object := range schema.Objects {
		permission := metadata.ObjectPermission{Object: object.ID, Operations: object.Operations}
		for _, field := range object.Fields {
			permission.Fields = append(permission.Fields, metadata.FieldPermission{Field: field.Key, Operations: field.Operations})
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
	if _, err := runtime.database.DatabaseAccess.SetApplicationRoles(ctx, admin.ID, admin.ID, registered.ID, manifest.ID, []uuid.UUID{role.ID}, 0); err != nil {
		t.Fatal(err)
	}
	// Changing an assignment terminates that user's application sessions on
	// purpose, so a revoked role cannot outlive the request that revoked it.
	if _, err := runtime.OpenPortalDatabase(ctx, portalLogin.Token, registered.ID); err != nil {
		t.Fatal(err)
	}

	date := metadata.Value{Kind: metadata.DateType, Data: time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC).Format("2006-01-02T15:04:05.999999999Z07:00")}
	created, err := runtime.SaveApplicationObject(ctx, portalLogin.Token, registered.ID, metadata.DocumentKind, "Поступление", ApplicationObjectWrite{
		Fields: map[string]metadata.Value{
			"Number": {Kind: metadata.StringType, Data: "IN-1"}, "Date": date,
			"Товар": {Kind: metadata.StringType, Data: "A"}, "Количество": {Kind: metadata.NumberType, Data: "5"},
		},
	})
	if err != nil || created.Reference == "" {
		t.Fatalf("save new document: %+v error=%v", created, err)
	}
	if created.Fields["Товар"].Data != "A" || created.Fields["Количество"].Data != "5" {
		t.Fatalf("saved fields not round-tripped: %+v", created.Fields)
	}
	// Значение пришло из модуля сеанса: клиент его не передавал, в базе его
	// нет, и без вызова обработчика поле осталось бы пустым.
	if created.Fields["Склад"].Data != "Основной" {
		t.Fatalf("session module did not supply the session parameter on the write path: %+v", created.Fields)
	}

	reloaded, err := runtime.GetApplicationObject(ctx, portalLogin.Token, registered.ID, metadata.DocumentKind, "Поступление", created.Reference)
	if err != nil || reloaded.Reference != created.Reference || reloaded.Fields["Товар"].Data != "A" {
		t.Fatalf("reload saved document: %+v error=%v", reloaded, err)
	}

	// Leaving Number out entirely (as the client does when its input is
	// blank) must let DocumentRepository.Write auto-number it, exactly like
	// an explicit empty string does for a catalog's auto Code - not fail
	// validation the way sending Number="" would.
	autoNumbered, err := runtime.SaveApplicationObject(ctx, portalLogin.Token, registered.ID, metadata.DocumentKind, "Поступление", ApplicationObjectWrite{
		Fields: map[string]metadata.Value{"Date": date, "Товар": {Kind: metadata.StringType, Data: "B"}, "Количество": {Kind: metadata.NumberType, Data: "3"}},
	})
	if err != nil || autoNumbered.Fields["Number"].Data == "" {
		t.Fatalf("auto-numbered document: %+v error=%v", autoNumbered, err)
	}

	posted, err := runtime.PostApplicationDocument(ctx, portalLogin.Token, registered.ID, "Поступление", created.Reference)
	if err != nil || !posted.Posted {
		t.Fatalf("post document: %+v error=%v", posted, err)
	}
	registers, err := metadata.NewAccumulationRegisterRepository(pool, mustCatalog(t, root))
	if err != nil {
		t.Fatal(err)
	}
	period, err := time.Parse(time.RFC3339Nano, date.Data)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := registers.Balances(ctx, "ОстаткиТоваров", period.Add(time.Hour), map[uuid.UUID]metadata.Value{productDimensionID: {Kind: metadata.StringType, Data: "A"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Turnover[quantityResourceID].Data != "5" {
		t.Fatalf("posted balance: %+v", rows)
	}

	undone, err := runtime.UndoApplicationDocumentPosting(ctx, portalLogin.Token, registered.ID, "Поступление", created.Reference)
	if err != nil || undone.Posted {
		t.Fatalf("undo posting: %+v error=%v", undone, err)
	}

	marked, err := runtime.SetApplicationDeletionMark(ctx, portalLogin.Token, registered.ID, metadata.DocumentKind, "Поступление", created.Reference, true)
	if err != nil || !marked.DeletionMark {
		t.Fatalf("set deletion mark: %+v error=%v", marked, err)
	}

	// Revoking the role must close every path immediately, on the same user,
	// same database and same document that worked a moment ago - otherwise the
	// grants above prove nothing about enforcement.
	if _, err := runtime.database.DatabaseAccess.SetApplicationRoles(ctx, admin.ID, admin.ID, registered.ID, manifest.ID, nil, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.OpenPortalDatabase(ctx, portalLogin.Token, registered.ID); err != nil {
		t.Fatal(err)
	}
	loaded, err := runtime.LoadApplicationObjects(ctx, portalLogin.Token, registered.ID, []string{"ru"})
	if err != nil || len(loaded.Objects) != 0 {
		t.Fatalf("objects still listed without a role: %+v error=%v", loaded.Objects, err)
	}
	denied := map[string]func() error{
		"form": func() error {
			_, err := runtime.LoadApplicationForm(ctx, portalLogin.Token, registered.ID, metadata.DocumentKind, "Поступление", metadata.ObjectForm, []string{"ru"})
			return err
		},
		"list": func() error {
			_, err := runtime.LoadApplicationList(ctx, portalLogin.Token, registered.ID, metadata.DocumentKind, "Поступление", metadata.DynamicListRequest{Limit: 10})
			return err
		},
		"read object": func() error {
			_, err := runtime.GetApplicationObject(ctx, portalLogin.Token, registered.ID, metadata.DocumentKind, "Поступление", created.Reference)
			return err
		},
		"save object": func() error {
			_, err := runtime.SaveApplicationObject(ctx, portalLogin.Token, registered.ID, metadata.DocumentKind, "Поступление", ApplicationObjectWrite{
				Reference: created.Reference, Fields: map[string]metadata.Value{"Товар": {Kind: metadata.StringType, Data: "B"}}})
			return err
		},
		"post document": func() error {
			_, err := runtime.PostApplicationDocument(ctx, portalLogin.Token, registered.ID, "Поступление", created.Reference)
			return err
		},
		"deletion mark": func() error {
			_, err := runtime.SetApplicationDeletionMark(ctx, portalLogin.Token, registered.ID, metadata.DocumentKind, "Поступление", created.Reference, false)
			return err
		},
	}
	for name, call := range denied {
		if err := call(); !errors.Is(err, metadata.ErrPermissionDenied) {
			t.Fatalf("%s was allowed without a role: %v", name, err)
		}
	}
	// The document itself must be untouched by the refused write attempts.
	if _, err := runtime.database.DatabaseAccess.SetApplicationRoles(ctx, admin.ID, admin.ID, registered.ID, manifest.ID, []uuid.UUID{role.ID}, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.OpenPortalDatabase(ctx, portalLogin.Token, registered.ID); err != nil {
		t.Fatal(err)
	}
	restored, err := runtime.GetApplicationObject(ctx, portalLogin.Token, registered.ID, metadata.DocumentKind, "Поступление", created.Reference)
	if err != nil || restored.Fields["Товар"].Data != "A" || !restored.DeletionMark {
		t.Fatalf("refused writes changed the document: %+v error=%v", restored, err)
	}
}

func mustCatalog(t *testing.T, root string) *metadata.Catalog {
	t.Helper()
	catalog, err := metadata.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}
