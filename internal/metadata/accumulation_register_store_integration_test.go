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

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestAccumulationRegisterRepositoryIntegration(t *testing.T) {
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

	documentID, registerID, productID, quantityID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	turnoverID, turnoverProductID, amountID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{
		Documents: []DocumentDefinition{{ID: documentID, Name: "Приходная", Number: DocumentNumber{Type: StringType, Length: 20, Unique: true, Periodicity: NumberPeriodYear}}},
		AccumulationRegisters: []AccumulationRegisterDefinition{
			{
				ID: registerID, Name: "ОстаткиТоваров", Kind: AccumulationRegisterBalance, Recorders: []uuid.UUID{documentID},
				Dimensions: []Attribute{{ID: productID, Name: "Товар", Required: true, Types: []Type{{Kind: StringType, Length: 100}}}},
				Resources:  []Attribute{{ID: quantityID, Name: "Количество", Required: true, Types: []Type{{Kind: NumberType, Precision: 15, Scale: 3}}}},
			},
			{
				ID: turnoverID, Name: "Продажи", Kind: AccumulationRegisterTurnover, Recorders: []uuid.UUID{documentID},
				Dimensions: []Attribute{{ID: turnoverProductID, Name: "Товар", Required: true, Types: []Type{{Kind: StringType, Length: 100}}}},
				Resources:  []Attribute{{ID: amountID, Name: "Сумма", Required: true, Types: []Type{{Kind: NumberType, Precision: 15, Scale: 2}}}},
			},
		},
		documentByName: map[string]int{"приходная": 0}, documentByID: map[uuid.UUID]int{documentID: 0},
		accumulationRegisterByName: map[string]int{"остаткитоваров": 0, "продажи": 1}, accumulationRegisterByID: map[uuid.UUID]int{registerID: 0, turnoverID: 1},
	}
	desired, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := schemadiff.Prepare(ctx, pool, desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schemadiff.Execute(ctx, pool, schemadiff.MigrationRequest{ProjectID: projectID, PackageSHA256: repeatCatalogHex('a', 64), GitCommit: repeatCatalogHex('b', 40), Desired: desired, ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true}); err != nil {
		t.Fatal(err)
	}

	documents, err := NewDocumentRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	repository, err := NewAccumulationRegisterRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	newDocument := func(number string, date time.Time) *DocumentRecord {
		record, createErr := documents.New(ctx, "Приходная", nil)
		if createErr != nil {
			t.Fatal(createErr)
		}
		record.Number, record.Date = number, date
		if saveErr := documents.Save(ctx, record, nil); saveErr != nil {
			t.Fatal(saveErr)
		}
		return record
	}
	write := func(document *DocumentRecord, values ...struct {
		kind   AccumulationMovementKind
		amount string
		active bool
	}) *AccumulationRegisterRecordSet {
		set, createErr := repository.NewRecordSet("ОстаткиТоваров")
		if createErr != nil {
			t.Fatal(createErr)
		}
		set.Filter.Recorder = &document.Reference
		for _, value := range values {
			row, addErr := set.Add()
			if addErr != nil {
				t.Fatal(addErr)
			}
			row.Period, row.Recorder, row.MovementKind, row.Active = document.Date, document.Reference, value.kind, value.active
			row.Dimensions[productID] = Value{Kind: StringType, Data: "A"}
			row.Resources[quantityID] = Value{Kind: NumberType, Data: value.amount}
		}
		if writeErr := repository.Write(ctx, set, true); writeErr != nil {
			t.Fatal(writeErr)
		}
		return set
	}

	january := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	firstDocument := newDocument("IN-1", january)
	var events []AccumulationRegisterEvent
	firstSet, _ := repository.NewRecordSet("ОстаткиТоваров")
	firstSet.Filter.Recorder = &firstDocument.Reference
	for _, value := range []struct {
		kind   AccumulationMovementKind
		amount string
		active bool
	}{{AccumulationMovementReceipt, "10", true}, {AccumulationMovementExpense, "3", true}, {AccumulationMovementReceipt, "100", false}} {
		row, _ := firstSet.Add()
		row.Period, row.Recorder, row.MovementKind, row.Active = january, firstDocument.Reference, value.kind, value.active
		row.Dimensions[productID] = Value{Kind: StringType, Data: "A"}
		row.Resources[quantityID] = Value{Kind: NumberType, Data: value.amount}
	}
	handler := AccumulationRegisterEventHandlerFunc(func(_ context.Context, event AccumulationRegisterEvent, _ *AccumulationRegisterRecordSet, replace bool) (bool, error) {
		events = append(events, event)
		if !replace {
			t.Error("unexpected append mode")
		}
		return false, nil
	})
	if err := repository.WriteWithHandler(ctx, firstSet, true, handler); err != nil {
		t.Fatal(err)
	}
	if want := []AccumulationRegisterEvent{AccumulationRegisterEventBeforeWrite, AccumulationRegisterEventOnWrite, AccumulationRegisterEventAfterWrite}; !slices.Equal(events, want) {
		t.Fatalf("events=%v", events)
	}
	if firstSet.Records[0].LineNumber != 1 || firstSet.Records[2].LineNumber != 3 {
		t.Fatalf("line numbers=%+v", firstSet.Records)
	}
	loaded, _ := repository.NewRecordSet("ОстаткиТоваров")
	loaded.Filter.Recorder = &firstDocument.Reference
	if err := repository.Read(ctx, loaded); err != nil || len(loaded.Records) != 3 {
		t.Fatalf("loaded=%+v error=%v", loaded, err)
	}

	filter := map[uuid.UUID]Value{productID: {Kind: StringType, Data: "A"}}
	balances, err := repository.Balances(ctx, "ОстаткиТоваров", january.Add(time.Hour), filter)
	if err != nil || len(balances) != 1 || balances[0].Turnover[quantityID].Data != "7" {
		t.Fatalf("balances=%+v error=%v", balances, err)
	}
	turnovers, err := repository.Turnovers(ctx, "ОстаткиТоваров", january.Add(-time.Hour), january.Add(time.Hour), filter)
	if err != nil || turnovers[0].Turnover[quantityID].Data != "7" || turnovers[0].Receipt[quantityID].Data != "10" || turnovers[0].Expense[quantityID].Data != "3" {
		t.Fatalf("turnovers=%+v error=%v", turnovers, err)
	}
	turnoverSet, _ := repository.NewRecordSet("Продажи")
	turnoverSet.Filter.Recorder = &firstDocument.Reference
	turnoverRow, _ := turnoverSet.Add()
	turnoverRow.Period, turnoverRow.Recorder, turnoverRow.MovementKind = january, firstDocument.Reference, AccumulationMovementExpense
	turnoverRow.Dimensions[turnoverProductID] = Value{Kind: StringType, Data: "A"}
	turnoverRow.Resources[amountID] = Value{Kind: NumberType, Data: "100"}
	if err := repository.Write(ctx, turnoverSet, true); err != nil || turnoverSet.Records[0].MovementKind != 0 {
		t.Fatalf("turnover write=%+v error=%v", turnoverSet, err)
	}
	sales, err := repository.Turnovers(ctx, "Продажи", january.Add(-time.Hour), january.Add(time.Hour), map[uuid.UUID]Value{turnoverProductID: {Kind: StringType, Data: "A"}})
	if err != nil || len(sales) != 1 || sales[0].Turnover[amountID].Data != "100" {
		t.Fatalf("sales=%+v error=%v", sales, err)
	}
	if _, err := repository.Balances(ctx, "Продажи", january, nil); err == nil {
		t.Fatal("turnover register exposed balances")
	}
	zeroDocument := newDocument("ZERO-1", january.Add(2*time.Hour))
	zeroSet, _ := repository.NewRecordSet("ОстаткиТоваров")
	zeroSet.Filter.Recorder = &zeroDocument.Reference
	for _, movement := range []AccumulationMovementKind{AccumulationMovementReceipt, AccumulationMovementExpense} {
		row, _ := zeroSet.Add()
		row.Period, row.Recorder, row.MovementKind = zeroDocument.Date, zeroDocument.Reference, movement
		row.Dimensions[productID] = Value{Kind: StringType, Data: "ZERO"}
		row.Resources[quantityID] = Value{Kind: NumberType, Data: "5"}
	}
	if err := repository.Write(ctx, zeroSet, true); err != nil {
		t.Fatal(err)
	}
	zeroKey := accumulationDimensionKey(catalog.AccumulationRegisters[0], zeroSet.Records[0].Dimensions)
	zeroTotalsTable, _ := PhysicalAccumulationRegisterTotalsTable(registerID)
	var zeroRows int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+qualifiedCatalogTable(zeroTotalsTable)+" WHERE dimension_key = $1", zeroKey).Scan(&zeroRows); err != nil || zeroRows != 0 {
		t.Fatalf("zero total rows=%d error=%v", zeroRows, err)
	}

	replacement := write(firstDocument, struct {
		kind   AccumulationMovementKind
		amount string
		active bool
	}{AccumulationMovementReceipt, "20", true})
	if len(replacement.Records) != 1 {
		t.Fatal("replacement failed")
	}
	secondDocument := newDocument("OUT-1", january.Add(24*time.Hour))
	write(secondDocument, struct {
		kind   AccumulationMovementKind
		amount string
		active bool
	}{AccumulationMovementExpense, "5", true})
	february := time.Date(2026, 2, 5, 10, 0, 0, 0, time.UTC)
	thirdDocument := newDocument("IN-2", february)
	write(thirdDocument, struct {
		kind   AccumulationMovementKind
		amount string
		active bool
	}{AccumulationMovementReceipt, "2", true})
	balances, err = repository.Balances(ctx, "ОстаткиТоваров", february.Add(time.Hour), filter)
	if err != nil || balances[0].Turnover[quantityID].Data != "17" {
		t.Fatalf("replaced balances=%+v error=%v", balances, err)
	}
	combined, err := repository.BalancesAndTurnovers(ctx, "ОстаткиТоваров", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), february.Add(time.Hour), filter)
	if err != nil || combined[0].Opening[quantityID].Data != "15" || combined[0].Turnover[quantityID].Data != "2" || combined[0].Closing[quantityID].Data != "17" {
		t.Fatalf("combined=%+v error=%v", combined, err)
	}

	totalsTable, _ := PhysicalAccumulationRegisterTotalsTable(registerID)
	var total string
	resourceColumn, _ := PhysicalAttributeColumn(quantityID)
	if err := pool.QueryRow(ctx, "SELECT COALESCE(SUM("+pgx.Identifier{resourceColumn}.Sanitize()+"), 0)::text FROM "+qualifiedCatalogTable(totalsTable)).Scan(&total); err != nil || total != "17.000" {
		t.Fatalf("total=%s error=%v", total, err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM "+qualifiedCatalogTable(totalsTable)); err != nil {
		t.Fatal(err)
	}
	if err := repository.RebuildTotals(ctx, "ОстаткиТоваров"); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT COALESCE(SUM("+pgx.Identifier{resourceColumn}.Sanitize()+"), 0)::text FROM "+qualifiedCatalogTable(totalsTable)).Scan(&total); err != nil || total != "17.000" {
		t.Fatalf("rebuilt total=%s error=%v", total, err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+qualifiedCatalogTable(totalsTable)+" WHERE dimension_key = $1", zeroKey).Scan(&zeroRows); err != nil || zeroRows != 0 {
		t.Fatalf("rebuilt zero total rows=%d error=%v", zeroRows, err)
	}

	rejected := errors.New("reject")
	replacement.Records[0].Resources[quantityID] = Value{Kind: NumberType, Data: "999"}
	if err := repository.WriteWithHandler(ctx, replacement, true, AccumulationRegisterEventHandlerFunc(func(_ context.Context, event AccumulationRegisterEvent, _ *AccumulationRegisterRecordSet, _ bool) (bool, error) {
		if event == AccumulationRegisterEventOnWrite {
			return false, rejected
		}
		return false, nil
	})); !errors.Is(err, rejected) {
		t.Fatalf("rejection error=%v", err)
	}
	balances, _ = repository.Balances(ctx, "ОстаткиТоваров", february.Add(time.Hour), filter)
	if balances[0].Turnover[quantityID].Data != "17" {
		t.Fatalf("event rollback balance=%+v", balances)
	}

	runtime, err := NewRuntimeWithAllRegisters(nil, nil, documents, nil, repository, catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	fourthDocument := newDocument("IN-3", february.Add(24*time.Hour))
	reference, err := runtime.wrapDocumentReference(catalog.Documents[0], fourthDocument.Reference)
	if err != nil {
		t.Fatal(err)
	}
	program, diagnostics := compiler.CompileSource("accumulation.bsl", `&НаСервере
Функция Проверить(Регистратор)
    Набор = РегистрыНакопления.ОстаткиТоваров.СоздатьНаборЗаписей();
    Набор.Отбор.Регистратор.Установить(Регистратор);
    Строка = Набор.Добавить();
    Строка.Период = '20260206100000';
    Строка.Регистратор = Регистратор;
    Строка.ВидДвижения = ВидДвиженияНакопления.Приход;
    Строка.Товар = "A";
    Строка.Количество = 4;
    Набор.Записать();
    Остатки = РегистрыНакопления.ОстаткиТоваров.Остатки('20260207000000', Новый Структура("Товар", "A"));
    Возврат Остатки[0].КоличествоОстаток;
КонецФункции`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	result, err := machine.NewContextWithMetadata(runtime).CallContext(ctx, "Проверить", reference)
	if err != nil || result.String() != "21" {
		t.Fatalf("BSL result=%v error=%v", result, err)
	}
	if err := runtime.SetDataLockWaitTimeout(150 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	lockContext, finishLock, err := runtime.BeginExecution(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.BeginTransaction(lockContext); err != nil {
		t.Fatal(err)
	}
	lock := &dataLock{runtime: runtime}
	lockElement, err := runtime.newDataLockElement("РегистрНакопления.ОстаткиТоваров")
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.setDataLockValue(lockElement, "Товар", bytecode.String("A")); err != nil {
		t.Fatal(err)
	}
	lock.entries = []*dataLockElement{lockElement}
	if err := runtime.acquireDataLocks(lockContext, lock); err != nil {
		t.Fatal(err)
	}
	conflictingContext, finishConflicting, err := runtime.BeginExecution(ctx)
	if err != nil {
		t.Fatal(err)
	}
	conflicting, _ := repository.NewRecordSet("ОстаткиТоваров")
	conflicting.Filter.Recorder = &fourthDocument.Reference
	conflictingRow, _ := conflicting.Add()
	conflictingRow.Period, conflictingRow.Recorder = fourthDocument.Date, fourthDocument.Reference
	conflictingRow.Dimensions[productID] = Value{Kind: StringType, Data: "A"}
	conflictingRow.Resources[quantityID] = Value{Kind: NumberType, Data: "5"}
	if err := repository.Write(conflictingContext, conflicting, true); !errors.Is(err, ErrDataLockTimeout) {
		t.Fatalf("accumulation data lock conflict=%v", err)
	}
	if err := finishConflicting(nil); err != nil {
		t.Fatal(err)
	}
	if err := runtime.RollbackTransaction(lockContext); err != nil {
		t.Fatal(err)
	}
	if err := finishLock(nil); err != nil {
		t.Fatal(err)
	}

	uses, err := documents.FindReferences(ctx, fourthDocument.Reference, 10)
	if err != nil || len(uses) != 1 || uses[0].OwnerKind != "accumulation-register" || uses[0].Field != "Recorder" {
		t.Fatalf("references=%+v error=%v", uses, err)
	}
}
