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

func TestCatalogRepositoryLifecycleIntegration(t *testing.T) {
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

	catalogID, emailID, parentID, phonesID, phoneID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{
		Catalogs: []CatalogDefinition{{
			ID: catalogID, Name: "Контрагенты", Code: CatalogCode{Type: StringType, Length: 9, Unique: true}, DescriptionLength: 250,
			Attributes: []Attribute{
				{ID: emailID, Name: "Email", Required: true, Types: []Type{{Kind: StringType, Length: 100}}},
				{ID: parentID, Name: "Родитель", Indexed: true, Types: []Type{{Kind: CatalogType, Reference: &catalogID}}},
			},
			TableParts: []TablePart{{ID: phonesID, Name: "Телефоны", Attributes: []Attribute{{ID: phoneID, Name: "Номер", Required: true, Types: []Type{{Kind: StringType, Length: 32}}}}}},
		}},
		catalogByName: map[string]int{"контрагенты": 0}, catalogByID: map[uuid.UUID]int{catalogID: 0},
	}
	desired, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := schemadiff.Prepare(ctx, pool, desired)
	if err != nil {
		t.Fatal(err)
	}
	_, err = schemadiff.Execute(ctx, pool, schemadiff.MigrationRequest{
		ProjectID: projectID, PackageSHA256: repeatCatalogHex('a', 64), GitCommit: repeatCatalogHex('b', 40), Desired: desired,
		ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	actual, err := schemadiff.Inspect(ctx, pool, schemadiff.ApplicationSchema)
	if err != nil {
		t.Fatal(err)
	}
	if plan, compareErr := schemadiff.Compare(desired, actual); compareErr != nil || len(plan.Changes) != 0 {
		t.Fatalf("post-migration plan=%+v error=%v", plan, compareErr)
	}

	repository, err := NewCatalogRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	var events []CatalogEvent
	handler := CatalogEventHandlerFunc(func(_ context.Context, event CatalogEvent, record *CatalogRecord) (bool, error) {
		events = append(events, event)
		if event == CatalogEventFill {
			record.Code, record.Description = "K001", "Первый"
			record.Attributes[emailID] = Value{Kind: StringType, Data: "first@example.test"}
		}
		return false, nil
	})
	record, err := repository.New(ctx, "контрагенты", handler)
	if err != nil {
		t.Fatal(err)
	}
	record.TableParts[phonesID] = []CatalogRow{{Values: map[uuid.UUID]Value{phoneID: {Kind: StringType, Data: "+380001"}}}}
	if err := repository.Save(ctx, record, handler); err != nil {
		t.Fatal(err)
	}
	wantEvents := []CatalogEvent{CatalogEventFill, CatalogEventFillCheck, CatalogEventBefore, CatalogEventOnWrite, CatalogEventAfter}
	if !slices.Equal(events, wantEvents) || record.Version != 1 {
		t.Fatalf("events=%v version=%d", events, record.Version)
	}
	loaded, err := repository.Get(ctx, record.Reference)
	if err != nil || loaded.Description != "Первый" || loaded.Attributes[emailID].Data != "first@example.test" || len(loaded.TableParts[phonesID]) != 1 {
		t.Fatalf("loaded=%+v error=%v", loaded, err)
	}
	found, ok, err := repository.FindByCode(ctx, "Контрагенты", "K001")
	if err != nil || !ok || found != record.Reference {
		t.Fatalf("found=%+v ok=%v error=%v", found, ok, err)
	}

	stale := cloneCatalogRecord(loaded)
	loaded.Description = "Второй"
	if err := repository.Save(ctx, loaded, nil); err != nil || loaded.Version != 2 {
		t.Fatalf("update version=%d error=%v", loaded.Version, err)
	}
	stale.Description = "Устаревший"
	if err := repository.Save(ctx, stale, nil); !errors.Is(err, ErrCatalogWriteConflict) {
		t.Fatalf("stale write error=%v", err)
	}

	rejected, err := repository.New(ctx, "Контрагенты", nil)
	if err != nil {
		t.Fatal(err)
	}
	rejected.Code, rejected.Description = "K002", "Откат"
	rejected.Attributes[emailID] = Value{Kind: StringType, Data: "rollback@example.test"}
	rejecting := CatalogEventHandlerFunc(func(_ context.Context, event CatalogEvent, _ *CatalogRecord) (bool, error) {
		if event == CatalogEventOnWrite {
			return false, errors.New("reject write")
		}
		return false, nil
	})
	if err := repository.Save(ctx, rejected, rejecting); err == nil {
		t.Fatal("event error did not roll back catalog write")
	}
	if _, err := repository.Get(ctx, rejected.Reference); !errors.Is(err, ErrCatalogRecordNotFound) {
		t.Fatalf("rolled back record error=%v", err)
	}

	runtime, err := NewRuntimeWithCatalogs(nil, repository, catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	program, diagnostics := compiler.CompileSource("catalog.bsl", `&НаСервере
Функция Проверить()
    Элемент = Справочники.Контрагенты.СоздатьЭлемент();
    Элемент.Код = "K003";
	Элемент.Наименование = "BSL";
	Элемент.Email = "bsl@example.test";
	Элемент.Родитель = Справочники.Контрагенты.НайтиПоКоду("K001");
	Строка = Элемент.Телефоны.Добавить();
    Строка.Номер = "+380003";
    Элемент.Записать();
	Ссылка = Справочники.Контрагенты.НайтиПоКоду("K003");
	Загружен = Ссылка.ПолучитьОбъект();
	Родитель = Загружен.Родитель.ПолучитьОбъект();
	Возврат Загружен.Email + ":" + Загружен.Телефоны[0].Номер + ":" + Родитель.Наименование;
КонецФункции`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	result, err := machine.NewContextWithMetadata(runtime).CallContext(ctx, "Проверить")
	if err != nil || result.String() != "bsl@example.test:+380003:Второй" {
		t.Fatalf("BSL catalog result=%v error=%v", result, err)
	}
}

func repeatCatalogHex(character byte, count int) string {
	result := make([]byte, count)
	for index := range result {
		result[index] = character
	}
	return string(result)
}
