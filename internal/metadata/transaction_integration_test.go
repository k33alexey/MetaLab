package metadata

import (
	"context"
	"errors"
	"os"
	"strings"
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

func TestTransactionsAndDataLocksIntegration(t *testing.T) {
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
	projectID, catalogID := uuid.MustNew(), uuid.MustNew()
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
	catalog := &Catalog{
		Catalogs: []CatalogDefinition{{
			ID: catalogID, Name: "Товары", Code: CatalogCode{Type: StringType, Length: 20, Unique: true}, DescriptionLength: 100,
		}},
		catalogByName: map[string]int{"товары": 0}, catalogByID: map[uuid.UUID]int{catalogID: 0},
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
		ProjectID: projectID, PackageSHA256: repeatCatalogHex('1', 64), GitCommit: repeatCatalogHex('2', 40), Desired: desired,
		ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	repository, err := NewCatalogRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewRuntimeWithCatalogs(nil, repository, catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.SetDataLockWaitTimeout(150 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	directContext, finishDirect, err := runtime.BeginExecution(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.BeginTransaction(directContext); err != nil {
		t.Fatal(err)
	}
	direct, err := repository.New(directContext, "Товары", nil)
	if err != nil {
		t.Fatal(err)
	}
	direct.Code = "DIRECT-ROLLBACK"
	if err := repository.Save(directContext, direct, nil); err != nil || direct.Version != 1 {
		t.Fatalf("direct transaction save version=%d error=%v", direct.Version, err)
	}
	if _, err := repository.Get(directContext, direct.Reference); err != nil {
		t.Fatalf("uncommitted record is not visible: %v", err)
	}
	if err := runtime.RollbackTransaction(directContext); err != nil {
		t.Fatal(err)
	}
	if direct.Version != 0 {
		t.Fatalf("rollback did not restore in-memory version: %d", direct.Version)
	}
	if err := finishDirect(nil); err != nil {
		t.Fatal(err)
	}
	if _, found, err := repository.FindByCode(ctx, "Товары", "DIRECT-ROLLBACK"); err != nil || found {
		t.Fatalf("direct rollback record found=%v error=%v", found, err)
	}

	outer, err := repository.New(ctx, "Товары", nil)
	if err != nil {
		t.Fatal(err)
	}
	outer.Code = "EVENT-OUTER"
	var nested *CatalogRecord
	rejected := errors.New("reject complete operation")
	handler := CatalogEventHandlerFunc(func(eventContext context.Context, event CatalogEvent, _ *CatalogRecord) (bool, error) {
		if event != CatalogEventOnWrite {
			return false, nil
		}
		var nestedErr error
		nested, nestedErr = repository.New(eventContext, "Товары", nil)
		if nestedErr != nil {
			return false, nestedErr
		}
		nested.Code = "EVENT-NESTED"
		if nestedErr = repository.Save(eventContext, nested, nil); nestedErr != nil {
			return false, nestedErr
		}
		return false, rejected
	})
	if err := repository.Save(ctx, outer, handler); !errors.Is(err, rejected) {
		t.Fatalf("event transaction error=%v", err)
	}
	if nested == nil || nested.Version != 0 {
		t.Fatalf("nested record was not restored after rollback: %+v", nested)
	}
	for _, code := range []string{"EVENT-OUTER", "EVENT-NESTED"} {
		if _, found, err := repository.FindByCode(ctx, "Товары", code); err != nil || found {
			t.Fatalf("event rollback %s found=%v error=%v", code, found, err)
		}
	}

	program, diagnostics := compiler.CompileSource("transactions.bsl", `&НаСервере
Функция ЗаписатьИЗафиксировать()
    НачатьТранзакцию();
    Элемент = Справочники.Товары.СоздатьЭлемент();
    Элемент.Код = "COMMIT";
    Элемент.Наименование = "Зафиксирован";
    Элемент.Записать();
    Если Справочники.Товары.НайтиПоКоду("COMMIT").Пустая() Тогда
        ВызватьИсключение "Запись не видна внутри транзакции";
    КонецЕсли;
    ЗафиксироватьТранзакцию();
    Возврат Не ТранзакцияАктивна();
КонецФункции

&НаСервере
Функция ЗаписатьИОтменить()
    НачатьТранзакцию();
    Элемент = Справочники.Товары.СоздатьЭлемент();
    Элемент.Код = "ROLLBACK";
    Элемент.Записать();
    ОтменитьТранзакцию();
    Возврат Не ТранзакцияАктивна();
КонецФункции

&НаСервере
Функция ВложенныйОткат()
    НачатьТранзакцию();
    НачатьТранзакцию();
    Элемент = Справочники.Товары.СоздатьЭлемент();
    Элемент.Код = "NESTED";
    Элемент.Записать();
    ЗафиксироватьТранзакцию();
    ОтменитьТранзакцию();
    Возврат Истина;
КонецФункции

&НаСервере
Функция ОшибкаВложеннойТранзакции()
    НачатьТранзакцию();
    НачатьТранзакцию();
    Элемент = Справочники.Товары.СоздатьЭлемент();
    Элемент.Код = "DOOMED";
    Элемент.Записать();
    ОтменитьТранзакцию();
    Попытка
        ЗафиксироватьТранзакцию();
    Исключение
        Возврат Не ТранзакцияАктивна();
    КонецПопытки;
    Возврат Ложь;
КонецФункции

&НаСервере
Функция Незавершенная()
    НачатьТранзакцию();
    Элемент = Справочники.Товары.СоздатьЭлемент();
    Элемент.Код = "UNFINISHED";
    Элемент.Записать();
    Возврат Истина;
КонецФункции

&НаСервере
Функция ПроверитьБлокировку(Идентификатор)
    НачатьТранзакцию();
    Блокировка = Новый БлокировкаДанных;
    Элемент = Блокировка.Добавить("Справочник.Товары");
    Элемент.УстановитьЗначение("Ссылка", Справочники.Товары.ПолучитьСсылку(Идентификатор));
    Элемент.Режим = РежимБлокировкиДанных.Исключительный;
    Блокировка.Заблокировать();
    ОтменитьТранзакцию();
    Возврат Истина;
КонецФункции`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	vmContext := machine.NewContextWithMetadata(runtime)
	for _, routine := range []string{"ЗаписатьИЗафиксировать", "ЗаписатьИОтменить", "ВложенныйОткат", "ОшибкаВложеннойТранзакции"} {
		result, err := vmContext.CallContext(ctx, routine)
		active, _ := result.AsBoolean()
		if err != nil || !active {
			t.Fatalf("%s result=%v error=%v", routine, result, err)
		}
	}
	if _, found, err := repository.FindByCode(ctx, "Товары", "COMMIT"); err != nil || !found {
		t.Fatalf("committed record found=%v error=%v", found, err)
	}
	for _, code := range []string{"ROLLBACK", "NESTED", "DOOMED"} {
		if _, found, err := repository.FindByCode(ctx, "Товары", code); err != nil || found {
			t.Fatalf("rolled back %s found=%v error=%v", code, found, err)
		}
	}
	if _, err := vmContext.CallContext(ctx, "Незавершенная"); err == nil || !strings.Contains(err.Error(), ErrTransactionNotCompleted.Error()) {
		t.Fatalf("unfinished transaction error=%v", err)
	}
	if _, found, err := repository.FindByCode(ctx, "Товары", "UNFINISHED"); err != nil || found {
		t.Fatalf("unfinished record found=%v error=%v", found, err)
	}

	locked, err := repository.New(ctx, "Товары", nil)
	if err != nil {
		t.Fatal(err)
	}
	locked.Code = "LOCKED"
	if err := repository.Save(ctx, locked, nil); err != nil {
		t.Fatal(err)
	}
	result, err := vmContext.CallContext(ctx, "ПроверитьБлокировку", bytecode.String(locked.Reference.ObjectID.String()))
	if accepted, _ := result.AsBoolean(); err != nil || !accepted {
		t.Fatalf("BSL data lock result=%v error=%v", result, err)
	}

	firstContext, finishFirst, err := runtime.BeginExecution(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.BeginTransaction(firstContext); err != nil {
		t.Fatal(err)
	}
	firstLock := testCatalogDataLock(t, runtime, firstContext, CatalogReference{CatalogID: catalogID, ObjectID: locked.Reference.ObjectID}, dataLockExclusive)
	if _, _, err := runtime.callDataLockMethod(firstContext, firstLock, "Lock", nil); err != nil {
		t.Fatal(err)
	}
	secondContext, finishSecond, err := runtime.BeginExecution(ctx)
	if err != nil {
		t.Fatal(err)
	}
	conflicting := cloneCatalogRecord(locked)
	conflicting.Description = "blocked"
	if err := repository.Save(secondContext, conflicting, nil); !errors.Is(err, ErrDataLockTimeout) {
		t.Fatalf("automatic lock conflict error=%v", err)
	}
	if err := finishSecond(nil); err != nil {
		t.Fatal(err)
	}
	if err := runtime.RollbackTransaction(firstContext); err != nil {
		t.Fatal(err)
	}
	if err := finishFirst(nil); err != nil {
		t.Fatal(err)
	}

	testSharedDataLocks(t, runtime, ctx, catalogID, locked.Reference.ObjectID)
	testDeadlockDetection(t, runtime, repository, ctx, catalogID, locked.Reference.ObjectID)
}

func testDeadlockDetection(t *testing.T, runtime *Runtime, repository *CatalogRepository, parent context.Context, catalogID, firstID uuid.UUID) {
	t.Helper()
	secondRecord, err := repository.New(parent, "Товары", nil)
	if err != nil {
		t.Fatal(err)
	}
	secondRecord.Code = "DEADLOCK"
	if err := repository.Save(parent, secondRecord, nil); err != nil {
		t.Fatal(err)
	}
	if err := runtime.SetDataLockWaitTimeout(3 * time.Second); err != nil {
		t.Fatal(err)
	}
	firstContext, finishFirst, err := runtime.BeginExecution(parent)
	if err != nil {
		t.Fatal(err)
	}
	secondContext, finishSecond, err := runtime.BeginExecution(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = finishFirst(errors.New("test cleanup")); _ = finishSecond(errors.New("test cleanup")) }()
	if err := runtime.BeginTransaction(firstContext); err != nil {
		t.Fatal(err)
	}
	if err := runtime.BeginTransaction(secondContext); err != nil {
		t.Fatal(err)
	}
	firstHeld := testCatalogDataLock(t, runtime, firstContext, CatalogReference{CatalogID: catalogID, ObjectID: firstID}, dataLockExclusive)
	secondHeld := testCatalogDataLock(t, runtime, secondContext, secondRecord.Reference, dataLockExclusive)
	if _, _, err := runtime.callDataLockMethod(firstContext, firstHeld, "Lock", nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runtime.callDataLockMethod(secondContext, secondHeld, "Lock", nil); err != nil {
		t.Fatal(err)
	}
	firstCross := testCatalogDataLock(t, runtime, firstContext, secondRecord.Reference, dataLockExclusive)
	secondCross := testCatalogDataLock(t, runtime, secondContext, CatalogReference{CatalogID: catalogID, ObjectID: firstID}, dataLockExclusive)
	firstLockContext, cancelFirstLock := context.WithCancel(firstContext)
	secondLockContext, cancelSecondLock := context.WithCancel(secondContext)
	defer cancelFirstLock()
	defer cancelSecondLock()
	type outcome struct {
		owner int
		err   error
	}
	results := make(chan outcome, 2)
	go func() {
		_, _, err := runtime.callDataLockMethod(firstLockContext, firstCross, "Lock", nil)
		results <- outcome{owner: 1, err: err}
	}()
	go func() {
		_, _, err := runtime.callDataLockMethod(secondLockContext, secondCross, "Lock", nil)
		results <- outcome{owner: 2, err: err}
	}()
	rollbackOwner := func(owner int) error {
		if owner == 1 {
			return runtime.RollbackTransaction(firstContext)
		}
		return runtime.RollbackTransaction(secondContext)
	}
	outcomes := make([]outcome, 0, 2)
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for len(outcomes) < 2 {
		select {
		case result := <-results:
			outcomes = append(outcomes, result)
			// PostgreSQL may wake the surviving statement before the victim
			// goroutine gets CPU time to publish its error. Do not infer the
			// victim from channel receive order.
			if errors.Is(result.err, ErrDeadlockDetected) {
				if err := rollbackOwner(result.owner); err != nil {
					cancelFirstLock()
					cancelSecondLock()
					t.Fatal(err)
				}
			}
		case <-timer.C:
			cancelFirstLock()
			cancelSecondLock()
			for len(outcomes) < 2 {
				select {
				case result := <-results:
					outcomes = append(outcomes, result)
				case <-time.After(5 * time.Second):
					t.Fatal("PostgreSQL advisory-lock calls did not stop after context cancellation")
				}
			}
			t.Fatal("PostgreSQL did not detect advisory-lock deadlock")
		}
	}
	var victim, survivor *outcome
	for index := range outcomes {
		switch {
		case errors.Is(outcomes[index].err, ErrDeadlockDetected):
			victim = &outcomes[index]
		case outcomes[index].err == nil:
			survivor = &outcomes[index]
		}
	}
	if victim == nil || survivor == nil || victim.owner == survivor.owner {
		t.Fatalf("unexpected deadlock outcomes: %+v", outcomes)
	}
	if err := rollbackOwner(survivor.owner); err != nil {
		t.Fatal(err)
	}
}

func testCatalogDataLock(t *testing.T, runtime *Runtime, ctx context.Context, reference CatalogReference, mode string) bytecode.RuntimeObject {
	t.Helper()
	value, handled, err := runtime.ConstructRuntimeObject(ctx, "DataLock", nil)
	if err != nil || !handled {
		t.Fatalf("construct data lock handled=%v error=%v", handled, err)
	}
	lock, _ := value.AsRuntimeObject()
	elementValue, handled, err := runtime.callDataLockMethod(ctx, lock, "Add", []bytecode.Value{bytecode.String("Catalog.Товары")})
	if err != nil || !handled {
		t.Fatalf("add data lock handled=%v error=%v", handled, err)
	}
	element, _ := elementValue.AsRuntimeObject()
	wrapped, err := runtime.wrapCatalogReference(runtime.catalog.Catalogs[0], reference)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := runtime.callDataLockMethod(ctx, element, "SetValue", []bytecode.Value{bytecode.String("Ref"), wrapped}); err != nil {
		t.Fatal(err)
	}
	if handled, err := runtime.setDataLockProperty(element, "Mode", bytecode.String(mode)); err != nil || !handled {
		t.Fatalf("set data lock mode handled=%v error=%v", handled, err)
	}
	return lock
}

func testSharedDataLocks(t *testing.T, runtime *Runtime, parent context.Context, catalogID, objectID uuid.UUID) {
	t.Helper()
	first, finishFirst, err := runtime.BeginExecution(parent)
	if err != nil {
		t.Fatal(err)
	}
	second, finishSecond, err := runtime.BeginExecution(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = finishFirst(errors.New("test cleanup")); _ = finishSecond(errors.New("test cleanup")) }()
	if err := runtime.BeginTransaction(first); err != nil {
		t.Fatal(err)
	}
	if err := runtime.BeginTransaction(second); err != nil {
		t.Fatal(err)
	}
	firstLock := testCatalogDataLock(t, runtime, first, CatalogReference{CatalogID: catalogID, ObjectID: objectID}, dataLockShared)
	secondLock := testCatalogDataLock(t, runtime, second, CatalogReference{CatalogID: catalogID, ObjectID: objectID}, dataLockShared)
	if _, _, err := runtime.callDataLockMethod(first, firstLock, "Lock", nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runtime.callDataLockMethod(second, secondLock, "Lock", nil); err != nil {
		t.Fatalf("two shared locks conflict: %v", err)
	}
	if err := runtime.RollbackTransaction(second); err != nil {
		t.Fatal(err)
	}
	if err := runtime.RollbackTransaction(first); err != nil {
		t.Fatal(err)
	}
}
