package metadata

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestDocumentRepositoryLifecycleIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	projectID := uuid.MustNew()
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, _ = pool.Exec(cleanup, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE")
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
		pool.Close()
	})
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE"); err != nil {
		t.Fatal(err)
	}

	catalogID, documentID := uuid.MustNew(), uuid.MustNew()
	partnerID, linesID, productID, quantityID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{
		Catalogs: []CatalogDefinition{{
			ID: catalogID, Name: "Контрагенты", Code: CatalogCode{Type: StringType, Length: 9, Unique: true}, DescriptionLength: 250,
		}},
		Documents: []DocumentDefinition{{
			ID: documentID, Name: "Продажа", Posting: true,
			Number:     DocumentNumber{Type: StringType, Length: 11, Unique: true, Periodicity: NumberPeriodYear},
			Attributes: []Attribute{{ID: partnerID, Name: "Контрагент", Required: true, Types: []Type{{Kind: CatalogType, Reference: &catalogID}}}},
			TableParts: []TablePart{{ID: linesID, Name: "Товары", Attributes: []Attribute{
				{ID: productID, Name: "Товар", Required: true, Types: []Type{{Kind: StringType, Length: 100}}},
				{ID: quantityID, Name: "Количество", Required: true, Types: []Type{{Kind: NumberType, Precision: 15, Scale: 3}}},
			}}},
		}},
		catalogByName:  map[string]int{"контрагенты": 0},
		catalogByID:    map[uuid.UUID]int{catalogID: 0},
		documentByName: map[string]int{"продажа": 0},
		documentByID:   map[uuid.UUID]int{documentID: 0},
	}
	desired, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := schemadiff.Prepare(ctx, pool, desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schemadiff.Execute(ctx, pool, schemadiff.MigrationRequest{
		ProjectID: projectID, PackageSHA256: repeatCatalogHex('c', 64), GitCommit: repeatCatalogHex('d', 40), Desired: desired,
		ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	actual, err := schemadiff.Inspect(ctx, pool, schemadiff.ApplicationSchema)
	if err != nil {
		t.Fatal(err)
	}
	if plan, compareErr := schemadiff.Compare(desired, actual); compareErr != nil || len(plan.Changes) != 0 {
		t.Fatalf("post-migration plan=%+v error=%v", plan, compareErr)
	}

	catalogRepository, err := NewCatalogRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	partner, err := catalogRepository.New(ctx, "Контрагенты", nil)
	if err != nil {
		t.Fatal(err)
	}
	partner.Code, partner.Description = "C001", "Покупатель"
	if err := catalogRepository.Save(ctx, partner, nil); err != nil {
		t.Fatal(err)
	}

	repository, err := NewDocumentRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	fixedDate := time.Date(2026, 9, 6, 12, 30, 45, 123_400_000, time.UTC)
	repository.now = func() time.Time { return fixedDate }
	var events []DocumentEvent
	handler := DocumentEventHandlerFunc(func(_ context.Context, event DocumentEvent, record *DocumentRecord) (bool, error) {
		events = append(events, event)
		if event == DocumentEventFill {
			record.Number = "SALE-1"
			record.Attributes[partnerID] = Value{Kind: CatalogType, Data: partner.Reference.ObjectID.String()}
		}
		return false, nil
	})
	record, err := repository.New(ctx, "продажа", handler)
	if err != nil {
		t.Fatal(err)
	}
	record.TableParts[linesID] = []DocumentRow{{Values: map[uuid.UUID]Value{
		productID: {Kind: StringType, Data: "Ноутбук"}, quantityID: {Kind: NumberType, Data: "2.000"},
	}}}
	if err := repository.Save(ctx, record, handler); err != nil {
		t.Fatal(err)
	}
	wantEvents := []DocumentEvent{DocumentEventFill, DocumentEventFillCheck, DocumentEventBefore, DocumentEventOnWrite, DocumentEventAfter}
	if !slices.Equal(events, wantEvents) || record.Version != 1 || !record.Date.Equal(fixedDate) {
		t.Fatalf("events=%v version=%d date=%s", events, record.Version, record.Date)
	}
	loaded, err := repository.Get(ctx, record.Reference)
	if err != nil || loaded.Number != "SALE-1" || loaded.Posted || loaded.Attributes[partnerID].Data != partner.Reference.ObjectID.String() ||
		len(loaded.TableParts[linesID]) != 1 || loaded.TableParts[linesID][0].Values[quantityID].Data != "2" {
		t.Fatalf("loaded=%+v error=%v", loaded, err)
	}
	found, ok, err := repository.FindByNumber(ctx, "Продажа", "SALE-1", &fixedDate)
	if err != nil || !ok || found != record.Reference {
		t.Fatalf("found=%+v ok=%v error=%v", found, ok, err)
	}
	if _, _, err := repository.FindByNumber(ctx, "Продажа", "SALE-1", nil); err == nil {
		t.Fatal("periodic FindByNumber accepted a missing date")
	}

	stale := cloneDocumentRecord(loaded)
	loaded.Number = "SALE-2"
	if err := repository.Save(ctx, loaded, nil); err != nil || loaded.Version != 2 {
		t.Fatalf("update version=%d error=%v", loaded.Version, err)
	}
	if err := repository.Save(ctx, stale, nil); !errors.Is(err, ErrDocumentWriteConflict) {
		t.Fatalf("stale write error=%v", err)
	}

	duplicate, err := repository.New(ctx, "Продажа", nil)
	if err != nil {
		t.Fatal(err)
	}
	duplicate.Number, duplicate.Date = "SALE-2", fixedDate
	duplicate.Attributes[partnerID] = Value{Kind: CatalogType, Data: partner.Reference.ObjectID.String()}
	if err := repository.Save(ctx, duplicate, nil); err == nil {
		t.Fatal("duplicate periodic number was accepted")
	}
	duplicate.Reference.ObjectID = uuid.MustNew()
	duplicate.Date = fixedDate.AddDate(1, 0, 0)
	if err := repository.Save(ctx, duplicate, nil); err != nil {
		t.Fatalf("same number in another period: %v", err)
	}

	runtime, err := NewRuntimeWithObjects(nil, catalogRepository, repository, catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	program, diagnostics := compiler.CompileSource("document.bsl", `&НаСервере
Функция Проверить()
    Документ = Документы.Продажа.СоздатьДокумент();
    Документ.Номер = "SALE-3";
    Документ.Дата = '20260907120000';
    Документ.Контрагент = Справочники.Контрагенты.НайтиПоКоду("C001");
    Строка = Документ.Товары.Добавить();
    Строка.Товар = "Монитор";
    Строка.Количество = 3;
    Документ.Записать();
    Ссылка = Документы.Продажа.НайтиПоНомеру("SALE-3", '20260907120000');
    Загружен = Ссылка.ПолучитьОбъект();
    Возврат Загружен.Номер + ":" + Загружен.Контрагент.ПолучитьОбъект().Наименование + ":" + Загружен.Товары[0].Товар;
КонецФункции`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	result, err := machine.NewContextWithMetadata(runtime).CallContext(ctx, "Проверить")
	if err != nil || result.String() != "SALE-3:Покупатель:Монитор" {
		t.Fatalf("BSL document result=%v error=%v", result, err)
	}
}
