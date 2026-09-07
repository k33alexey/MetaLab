package metadata

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestObjectIntegrityLifecycleIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	projectID := uuid.MustNew()
	catalogID, predefinedID, documentID, constantID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	directID, compositeID := uuid.MustNew(), uuid.MustNew()
	targetID := uuid.MustNew()
	defaultTarget := Value{Kind: CatalogType, Data: targetID.String()}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, _ = pool.Exec(cleanup, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE")
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.object_sequences WHERE metadata_id = ANY($1::uuid[])", []string{catalogID.String(), documentID.String()})
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.object_deletions WHERE metadata_id = ANY($1::uuid[])", []string{catalogID.String(), documentID.String()})
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.constant_values WHERE constant_id = $1", constantID.String())
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
		pool.Close()
	})
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE"); err != nil {
		t.Fatal(err)
	}

	catalog := &Catalog{
		Constants: []Constant{{
			ID: constantID, Name: "ОсновнойТовар", Types: []Type{{Kind: CatalogType, Reference: &catalogID}}, Default: &defaultTarget,
		}},
		Catalogs: []CatalogDefinition{{
			ID: catalogID, Name: "Товары", Code: CatalogCode{Type: StringType, Length: 4, Auto: true, Unique: true}, DescriptionLength: 100,
			Predefined: []PredefinedCatalogItem{{ID: predefinedID, Name: "Базовый", Description: "Базовый товар"}},
		}},
		Documents: []DocumentDefinition{{
			ID: documentID, Name: "Заказ", Number: DocumentNumber{Type: StringType, Length: 4, Auto: true, Unique: true, Periodicity: NumberPeriodYear},
			Attributes: []Attribute{
				{ID: directID, Name: "Товар", Types: []Type{{Kind: CatalogType, Reference: &catalogID}}},
				{ID: compositeID, Name: "Выбор", Types: []Type{{Kind: CatalogType, Reference: &catalogID}, {Kind: StringType, Length: 100}}},
			},
		}},
		constantByName: map[string]int{"основнойтовар": 0}, constantByID: map[uuid.UUID]int{constantID: 0},
		catalogByName: map[string]int{"товары": 0}, catalogByID: map[uuid.UUID]int{catalogID: 0},
		documentByName: map[string]int{"заказ": 0}, documentByID: map[uuid.UUID]int{documentID: 0},
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
		ProjectID: projectID, PackageSHA256: repeatCatalogHex('e', 64), GitCommit: repeatCatalogHex('f', 40), Desired: desired,
		ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := EnsureObjectIntegrityStorage(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := EnsureConstantStorage(ctx, pool); err != nil {
		t.Fatal(err)
	}

	catalogs, err := NewCatalogRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	syncResult, err := catalogs.SynchronizePredefined(ctx)
	if err != nil || syncResult.Created != 1 {
		t.Fatalf("predefined sync=%+v error=%v", syncResult, err)
	}
	predefined, err := catalogs.Get(ctx, CatalogReference{CatalogID: catalogID, ObjectID: predefinedID})
	if err != nil || predefined.Code != "0001" || predefined.PredefinedName != "Базовый" {
		t.Fatalf("predefined=%+v error=%v", predefined, err)
	}
	predefined.Code, predefined.Description = "EDIT", "Изменено пользователем"
	if err := catalogs.Save(ctx, predefined, nil); err != nil {
		t.Fatal(err)
	}
	if second, err := catalogs.SynchronizePredefined(ctx); err != nil || second.Created != 0 || second.Assigned != 0 {
		t.Fatalf("second predefined sync=%+v error=%v", second, err)
	}
	preserved, err := catalogs.Get(ctx, predefined.Reference)
	if err != nil || preserved.Code != "EDIT" || preserved.Description != "Изменено пользователем" {
		t.Fatalf("preserved predefined=%+v error=%v", preserved, err)
	}

	const parallelObjects = 24
	codes := make(chan string, parallelObjects)
	errorsChannel := make(chan error, parallelObjects)
	var group sync.WaitGroup
	for index := range parallelObjects {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			record, err := catalogs.New(ctx, "Товары", nil)
			if err == nil {
				record.Description = fmt.Sprintf("Товар %d", index)
				err = catalogs.Save(ctx, record, nil)
			}
			if err != nil {
				errorsChannel <- err
				return
			}
			codes <- record.Code
		}(index)
	}
	group.Wait()
	close(errorsChannel)
	close(codes)
	for err := range errorsChannel {
		t.Fatal(err)
	}
	uniqueCodes := make(map[string]bool, parallelObjects)
	for code := range codes {
		if uniqueCodes[code] || len(code) != 4 {
			t.Fatalf("invalid or duplicate automatic code %q", code)
		}
		uniqueCodes[code] = true
	}
	if len(uniqueCodes) != parallelObjects {
		t.Fatalf("automatic codes=%d", len(uniqueCodes))
	}
	manual, err := catalogs.New(ctx, "Товары", nil)
	if err != nil {
		t.Fatal(err)
	}
	manual.Code, manual.Description = "0900", "Ручной код"
	if err := catalogs.Save(ctx, manual, nil); err != nil {
		t.Fatal(err)
	}
	afterManual, err := catalogs.New(ctx, "Товары", nil)
	if err != nil {
		t.Fatal(err)
	}
	afterManual.Description = "После ручного кода"
	if err := catalogs.Save(ctx, afterManual, nil); err != nil || afterManual.Code != "0901" {
		t.Fatalf("code after manual=%q error=%v", afterManual.Code, err)
	}

	documents, err := NewDocumentRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	firstDate := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	first := saveAutomaticDocument(t, ctx, documents, firstDate, nil)
	second := saveAutomaticDocument(t, ctx, documents, firstDate.AddDate(0, 1, 0), nil)
	nextYear := saveAutomaticDocument(t, ctx, documents, firstDate.AddDate(1, 0, 0), nil)
	if first.Number != "0001" || second.Number != "0002" || nextYear.Number != "0001" {
		t.Fatalf("periodic numbers=%q,%q,%q", first.Number, second.Number, nextYear.Number)
	}
	const parallelDocuments = 12
	numbers := make(chan string, parallelDocuments)
	documentErrors := make(chan error, parallelDocuments)
	for index := range parallelDocuments {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			record, err := documents.New(ctx, "Заказ", nil)
			if err == nil {
				record.Date = firstDate.Add(time.Duration(index) * time.Minute)
				err = documents.Save(ctx, record, nil)
			}
			if err != nil {
				documentErrors <- err
				return
			}
			numbers <- record.Number
		}(index)
	}
	group.Wait()
	close(documentErrors)
	close(numbers)
	for err := range documentErrors {
		t.Fatal(err)
	}
	uniqueNumbers := map[string]bool{first.Number: true, second.Number: true}
	for number := range numbers {
		if uniqueNumbers[number] || len(number) != 4 {
			t.Fatalf("invalid or duplicate automatic document number %q", number)
		}
		uniqueNumbers[number] = true
	}
	if len(uniqueNumbers) != parallelDocuments+2 {
		t.Fatalf("automatic document numbers=%d", len(uniqueNumbers))
	}

	target, err := catalogs.New(ctx, "Товары", nil)
	if err != nil {
		t.Fatal(err)
	}
	target.Reference.ObjectID = targetID
	target.Description = "Удаляемый товар"
	if err := catalogs.Save(ctx, target, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := catalogs.Delete(ctx, target, nil, nil); !errors.Is(err, ErrDeletionMarkRequired) {
		t.Fatalf("unmarked delete error=%v", err)
	}
	target.DeletionMark = true
	if err := catalogs.Save(ctx, target, nil); err != nil {
		t.Fatal(err)
	}
	direct := saveAutomaticDocument(t, ctx, documents, firstDate, map[uuid.UUID]Value{
		directID: {Kind: CatalogType, Data: target.Reference.ObjectID.String()},
	})
	if uses, err := catalogs.FindReferences(ctx, target.Reference, 10); err != nil || !hasReferenceUse(uses, "document", "Товар") || !hasReferenceUse(uses, "constant-default", "default") {
		t.Fatalf("direct references=%+v error=%v", uses, err)
	}
	if _, err := catalogs.Delete(ctx, target, nil, nil); !errors.Is(err, ErrObjectReferenced) {
		t.Fatalf("referenced delete error=%v", err)
	}
	direct.DeletionMark = true
	if err := documents.Save(ctx, direct, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := documents.Delete(ctx, direct, nil, nil); err != nil {
		t.Fatal(err)
	}
	composite := saveAutomaticDocument(t, ctx, documents, firstDate, map[uuid.UUID]Value{
		compositeID: {Kind: CatalogType, Data: target.Reference.ObjectID.String()},
	})
	if uses, err := catalogs.FindReferences(ctx, target.Reference, 10); err != nil || !hasReferenceUse(uses, "document", "Выбор") || !hasReferenceUse(uses, "constant-default", "default") {
		t.Fatalf("composite references=%+v error=%v", uses, err)
	}
	composite.DeletionMark = true
	if err := documents.Save(ctx, composite, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := documents.Delete(ctx, composite, nil, nil); err != nil {
		t.Fatal(err)
	}
	if uses, err := catalogs.FindReferences(ctx, target.Reference, 10); err != nil || len(uses) != 1 || uses[0].OwnerKind != "constant-default" {
		t.Fatalf("default constant references=%+v error=%v", uses, err)
	}
	constants, err := NewConstantRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := constants.Set(ctx, "ОсновнойТовар", Value{Kind: CatalogType, Data: predefinedID.String()}, nil); err != nil {
		t.Fatal(err)
	}
	actor := uuid.MustNew()
	deleted, err := catalogs.Delete(ctx, target, nil, &actor)
	if err != nil || deleted.ObjectID != target.Reference.ObjectID || deleted.ActorID == nil || *deleted.ActorID != actor {
		t.Fatalf("deletion=%+v error=%v", deleted, err)
	}
	if _, err := catalogs.Get(ctx, target.Reference); !errors.Is(err, ErrCatalogRecordNotFound) {
		t.Fatalf("deleted catalog read error=%v", err)
	}
	history, err := ListObjectDeletions(ctx, pool, 1000)
	if err != nil {
		t.Fatal(err)
	}
	foundDeletion := false
	for _, item := range history {
		if item.ID == deleted.ID && item.ObjectID == target.Reference.ObjectID && item.ActorID != nil && *item.ActorID == actor {
			foundDeletion = true
		}
	}
	if !foundDeletion {
		t.Fatalf("deletion %s is missing from history", deleted.ID)
	}
	preserved.DeletionMark = true
	if err := catalogs.Save(ctx, preserved, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := catalogs.Delete(ctx, preserved, nil, nil); !errors.Is(err, ErrPredefinedDeleteDenied) {
		t.Fatalf("predefined delete error=%v", err)
	}
	runtime, err := NewRuntimeWithCatalogs(nil, catalogs, catalog, &actor)
	if err != nil {
		t.Fatal(err)
	}
	program, diagnostics := compiler.CompileSource("delete.bsl", `&НаСервере
Функция Проверить()
    Если Справочники.Товары.Базовый.ПолучитьОбъект().ИмяПредопределенныхДанных <> "Базовый" Тогда
        Возврат Ложь;
    КонецЕсли;
    Элемент = Справочники.Товары.СоздатьЭлемент();
    Элемент.Наименование = "Удаление из BSL";
    Элемент.Записать();
    Ссылка = Элемент.ПолучитьСсылку();
    Элемент.УстановитьПометкуУдаления(Истина);
    Элемент.Удалить();
    Попытка
        Ссылка.ПолучитьОбъект();
        Возврат Ложь;
    Исключение
        Возврат Истина;
    КонецПопытки;
КонецФункции`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	result, err := machine.NewContextWithMetadata(runtime).CallContext(ctx, "Проверить")
	deletedFromBSL, ok := result.AsBoolean()
	if err != nil || !ok || !deletedFromBSL {
		t.Fatalf("BSL deletion result=%v error=%v", result, err)
	}
}

func saveAutomaticDocument(t *testing.T, ctx context.Context, repository *DocumentRepository, date time.Time, attributes map[uuid.UUID]Value) *DocumentRecord {
	t.Helper()
	record, err := repository.New(ctx, "Заказ", nil)
	if err != nil {
		t.Fatal(err)
	}
	record.Date = date
	for id, value := range attributes {
		record.Attributes[id] = value
	}
	if err := repository.Save(ctx, record, nil); err != nil {
		t.Fatal(err)
	}
	return record
}

func hasReferenceUse(uses []ReferenceUse, kind, field string) bool {
	for _, use := range uses {
		if use.OwnerKind == kind && use.Field == field {
			return true
		}
	}
	return false
}
