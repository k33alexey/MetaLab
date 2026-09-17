package platform

import (
	"bytes"
	"context"
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
	productAttributeID, quantityAttributeID := uuid.MustNew(), uuid.MustNew()
	productDimensionID, quantityResourceID := uuid.MustNew(), uuid.MustNew()
	document := metadata.DocumentDefinition{
		Format: 1, ID: documentID, Name: "Поступление", Title: metadata.LocalizedText{"ru": "Поступление"}, Posting: true,
		Number: metadata.DocumentNumber{Type: metadata.StringType, Length: 20, Unique: true, Periodicity: metadata.NumberPeriodYear},
		Attributes: []metadata.Attribute{
			{ID: productAttributeID, Name: "Товар", Title: metadata.LocalizedText{"ru": "Товар"}, Required: true, Types: []metadata.Type{{Kind: metadata.StringType, Length: 100}}},
			{ID: quantityAttributeID, Name: "Количество", Title: metadata.LocalizedText{"ru": "Количество"}, Required: true, Types: []metadata.Type{{Kind: metadata.NumberType, Precision: 15, Scale: 3}}},
		},
		ObjectModule: &moduleID,
	}
	register := metadata.AccumulationRegisterDefinition{
		Format: 1, ID: registerID, Name: "ОстаткиТоваров", Title: metadata.LocalizedText{"ru": "Остатки товаров"},
		Kind: metadata.AccumulationRegisterBalance, Recorders: []uuid.UUID{documentID},
		Dimensions: []metadata.Attribute{{ID: productDimensionID, Name: "Товар", Title: metadata.LocalizedText{"ru": "Товар"}, Required: true, Types: []metadata.Type{{Kind: metadata.StringType, Length: 100}}}},
		Resources:  []metadata.Attribute{{ID: quantityResourceID, Name: "Количество", Title: metadata.LocalizedText{"ru": "Количество"}, Required: true, Types: []metadata.Type{{Kind: metadata.NumberType, Precision: 15, Scale: 3}}}},
	}
	manifest := project.Project{Format: 1, ID: uuid.MustNew(), Name: "WriteDemo", Title: "Write demo", DefaultLanguage: "ru", Languages: []project.Language{{Name: "Русский", Title: "Русский", Code: "ru"}}}
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
	write("metadata/documents/"+documentID.String()+"/object.yaml", document)
	write("metadata/accumulation-registers/"+registerID.String()+"/object.yaml", register)
	if err := os.WriteFile(filepath.Join(root, "metadata/documents", documentID.String(), moduleID.String()+".bsl"), []byte(`&НаСервере
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

	reloaded, err := runtime.GetApplicationObject(ctx, portalLogin.Token, registered.ID, metadata.DocumentKind, "Поступление", created.Reference)
	if err != nil || reloaded.Reference != created.Reference || reloaded.Fields["Товар"].Data != "A" {
		t.Fatalf("reload saved document: %+v error=%v", reloaded, err)
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
}

func mustCatalog(t *testing.T, root string) *metadata.Catalog {
	t.Helper()
	catalog, err := metadata.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}
