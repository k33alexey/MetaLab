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

func TestInformationRegisterRepositoryIntegration(t *testing.T) {
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

	documentID := uuid.MustNew()
	independentID, currencyID, rateID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	recorderID, productID, priceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{
		Documents: []DocumentDefinition{{
			ID: documentID, Name: "УстановкаЦен", Number: DocumentNumber{Type: StringType, Length: 11, Unique: true, Periodicity: NumberPeriodYear},
		}},
		InformationRegisters: []InformationRegisterDefinition{
			{
				ID: independentID, Name: "КурсыВалют", WriteMode: InformationRegisterIndependent, Periodicity: InformationRegisterPeriodDay,
				Dimensions: []Attribute{{ID: currencyID, Name: "Валюта", Required: true, Types: []Type{{Kind: UUIDType}}}},
				Resources:  []Attribute{{ID: rateID, Name: "Курс", Required: true, Types: []Type{{Kind: NumberType, Precision: 15, Scale: 4}}}},
			},
			{
				ID: recorderID, Name: "Цены", WriteMode: InformationRegisterRecorder, Periodicity: InformationRegisterPeriodRecorderPosition,
				Recorders:  []uuid.UUID{documentID},
				Dimensions: []Attribute{{ID: productID, Name: "Товар", Required: true, Types: []Type{{Kind: UUIDType}}}},
				Resources:  []Attribute{{ID: priceID, Name: "Цена", Required: true, Types: []Type{{Kind: NumberType, Precision: 15, Scale: 2}}}},
			},
		},
		documentByName: map[string]int{"установкацен": 0}, documentByID: map[uuid.UUID]int{documentID: 0},
		informationRegisterByName: map[string]int{"курсывалют": 0, "цены": 1},
		informationRegisterByID:   map[uuid.UUID]int{independentID: 0, recorderID: 1},
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
	actual, err := schemadiff.Inspect(ctx, pool, schemadiff.ApplicationSchema)
	if err != nil {
		t.Fatal(err)
	}
	if plan, compareErr := schemadiff.Compare(desired, actual); compareErr != nil || len(plan.Changes) != 0 {
		t.Fatalf("post-migration plan=%+v error=%v", plan, compareErr)
	}

	repository, err := NewInformationRegisterRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	currency := uuid.MustNew()
	firstPeriod := time.Date(2026, 9, 1, 13, 15, 0, 0, time.UTC)
	set, err := repository.NewRecordSet("КурсыВалют")
	if err != nil {
		t.Fatal(err)
	}
	set.Filter.Period = &firstPeriod
	set.Filter.Dimensions[currencyID] = Value{Kind: UUIDType, Data: currency.String()}
	record, err := set.Add()
	if err != nil {
		t.Fatal(err)
	}
	record.Period = firstPeriod
	record.Dimensions[currencyID] = Value{Kind: UUIDType, Data: currency.String()}
	record.Resources[rateID] = Value{Kind: NumberType, Data: "40.2500"}
	var events []InformationRegisterEvent
	handler := InformationRegisterEventHandlerFunc(func(_ context.Context, event InformationRegisterEvent, _ *InformationRegisterRecordSet, replace bool) (bool, error) {
		events = append(events, event)
		if !replace {
			t.Error("unexpected append mode")
		}
		return false, nil
	})
	if err := repository.WriteWithHandler(ctx, set, true, handler); err != nil {
		t.Fatal(err)
	}
	if want := []InformationRegisterEvent{InformationRegisterEventBeforeWrite, InformationRegisterEventOnWrite, InformationRegisterEventAfterWrite}; !slices.Equal(events, want) {
		t.Fatalf("events=%v want=%v", events, want)
	}
	day := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if !set.Records[0].Period.Equal(day) || set.Records[0].Resources[rateID].Data != "40.25" {
		t.Fatalf("normalized set=%+v", set)
	}
	loaded, _ := repository.NewRecordSet("КурсыВалют")
	loaded.Filter = cloneInformationRegisterFilter(set.Filter)
	if err := repository.Read(ctx, loaded); err != nil || len(loaded.Records) != 1 || loaded.Records[0].Resources[rateID].Data != "40.25" {
		t.Fatalf("loaded=%+v error=%v", loaded, err)
	}
	if err := repository.Write(ctx, loaded, false); err == nil {
		t.Fatal("duplicate independent key was accepted")
	}

	secondPeriod := firstPeriod.AddDate(0, 0, 1)
	second, _ := repository.NewRecordSet("КурсыВалют")
	second.Filter.Period = &secondPeriod
	second.Filter.Dimensions[currencyID] = Value{Kind: UUIDType, Data: currency.String()}
	secondRecord, _ := second.Add()
	secondRecord.Period = secondPeriod
	secondRecord.Dimensions[currencyID] = Value{Kind: UUIDType, Data: currency.String()}
	secondRecord.Resources[rateID] = Value{Kind: NumberType, Data: "41.5"}
	if err := repository.Write(ctx, second, true); err != nil {
		t.Fatal(err)
	}
	last, err := repository.SliceLast(ctx, "КурсыВалют", secondPeriod.AddDate(0, 0, 1), map[uuid.UUID]Value{currencyID: {Kind: UUIDType, Data: currency.String()}})
	if err != nil || len(last) != 1 || last[0].Resources[rateID].Data != "41.5" {
		t.Fatalf("last slice=%+v error=%v", last, err)
	}
	first, err := repository.SliceFirst(ctx, "КурсыВалют", firstPeriod, map[uuid.UUID]Value{currencyID: {Kind: UUIDType, Data: currency.String()}})
	if err != nil || len(first) != 1 || first[0].Resources[rateID].Data != "40.25" {
		t.Fatalf("first slice=%+v error=%v", first, err)
	}

	documentRepository, err := NewDocumentRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	document, err := documentRepository.New(ctx, "УстановкаЦен", nil)
	if err != nil {
		t.Fatal(err)
	}
	document.Number, document.Date = "PRICE-1", firstPeriod
	if err := documentRepository.Save(ctx, document, nil); err != nil {
		t.Fatal(err)
	}
	prices, _ := repository.NewRecordSet("Цены")
	prices.Filter.Recorder = &document.Reference
	for index := 0; index < 2; index++ {
		price, _ := prices.Add()
		price.Period, price.Recorder = document.Date, document.Reference
		price.Dimensions[productID] = Value{Kind: UUIDType, Data: uuid.MustNew().String()}
		price.Resources[priceID] = Value{Kind: NumberType, Data: "100"}
	}
	if err := repository.Write(ctx, prices, true); err != nil || prices.Records[0].LineNumber != 1 || prices.Records[1].LineNumber != 2 {
		t.Fatalf("recorder write=%+v error=%v", prices, err)
	}
	duplicateSemantic, _ := repository.NewRecordSet("Цены")
	duplicateSemantic.Filter.Recorder = &document.Reference
	duplicateSemanticRecord, _ := duplicateSemantic.Add()
	duplicateSemanticRecord.Period, duplicateSemanticRecord.Recorder, duplicateSemanticRecord.LineNumber = document.Date, document.Reference, 3
	duplicateSemanticRecord.Dimensions[productID] = prices.Records[0].Dimensions[productID]
	duplicateSemanticRecord.Resources[priceID] = Value{Kind: NumberType, Data: "101"}
	if err := repository.Write(ctx, duplicateSemantic, false); err == nil {
		t.Fatal("duplicate period, dimensions and recorder were accepted")
	}
	duplicateLine, _ := repository.NewRecordSet("Цены")
	duplicateLine.Filter.Recorder = &document.Reference
	duplicateLineRecord, _ := duplicateLine.Add()
	duplicateLineRecord.Period, duplicateLineRecord.Recorder, duplicateLineRecord.LineNumber = document.Date.Add(time.Second), document.Reference, 1
	duplicateLineRecord.Dimensions[productID] = Value{Kind: UUIDType, Data: uuid.MustNew().String()}
	duplicateLineRecord.Resources[priceID] = Value{Kind: NumberType, Data: "102"}
	if err := repository.Write(ctx, duplicateLine, false); err == nil {
		t.Fatal("duplicate recorder and line number were accepted")
	}
	missingReference := DocumentReference{DocumentID: documentID, ObjectID: uuid.MustNew()}
	missingRecorder, _ := repository.NewRecordSet("Цены")
	missingRecorder.Filter.Recorder = &missingReference
	missingRecord, _ := missingRecorder.Add()
	missingRecord.Period, missingRecord.Recorder = document.Date, missingReference
	missingRecord.Dimensions[productID] = Value{Kind: UUIDType, Data: uuid.MustNew().String()}
	missingRecord.Resources[priceID] = Value{Kind: NumberType, Data: "103"}
	if err := repository.Write(ctx, missingRecorder, true); err == nil {
		t.Fatal("nonexistent recorder was accepted")
	}
	uses, err := documentRepository.FindReferences(ctx, document.Reference, 10)
	if err != nil || len(uses) != 2 || uses[0].OwnerKind != "information-register" || uses[0].Field != "Recorder" {
		t.Fatalf("recorder references=%+v error=%v", uses, err)
	}
	replacement, _ := repository.NewRecordSet("Цены")
	replacement.Filter.Recorder = &document.Reference
	replacementRecord, _ := replacement.Add()
	replacementRecord.Period, replacementRecord.Recorder = document.Date, document.Reference
	replacementRecord.Dimensions[productID] = Value{Kind: UUIDType, Data: uuid.MustNew().String()}
	replacementRecord.Resources[priceID] = Value{Kind: NumberType, Data: "250"}
	if err := repository.Write(ctx, replacement, true); err != nil {
		t.Fatal(err)
	}
	readReplacement, _ := repository.NewRecordSet("Цены")
	readReplacement.Filter.Recorder = &document.Reference
	if err := repository.Read(ctx, readReplacement); err != nil || len(readReplacement.Records) != 1 || readReplacement.Records[0].Resources[priceID].Data != "250" {
		t.Fatalf("recorder replacement=%+v error=%v", readReplacement, err)
	}

	runtime, err := NewRuntimeWithAllObjects(nil, nil, documentRepository, repository, catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	bslCurrency := uuid.MustNew()
	program, diagnostics := compiler.CompileSource("information_register.bsl", `&НаСервере
Функция Проверить(Валюта)
    Набор = РегистрыСведений.КурсыВалют.СоздатьНаборЗаписей();
    Набор.Отбор.Период.Установить('20260903000000');
    Набор.Отбор.Валюта.Установить(Валюта);
    Строка = Набор.Добавить();
    Строка.Период = '20260903174500';
    Строка.Валюта = Валюта;
    Строка.Курс = 42.75;
    Набор.Записать();

    Прочитано = РегистрыСведений.КурсыВалют.СоздатьНаборЗаписей();
    Прочитано.Отбор.Период.Установить('20260903000000');
    Прочитано.Отбор.Валюта.Установить(Валюта);
    Прочитано.Прочитать();
    Сумма = 0;
    Для Каждого Запись Из Прочитано Цикл
        Сумма = Сумма + Запись.Курс;
    КонецЦикла;
    Срез = РегистрыСведений.КурсыВалют.СрезПоследних('20260904000000', Новый Структура("Валюта", Валюта));
    Возврат Прочитано.Количество() + Сумма + Срез[0].Курс;
КонецФункции

&НаСервере
Функция ПроверитьБлокировку(Валюта)
    НачатьТранзакцию();
    Блокировка = Новый БлокировкаДанных;
    Элемент = Блокировка.Добавить("РегистрСведений.КурсыВалют");
    Элемент.УстановитьЗначение("Период", '20260903000000');
    Элемент.УстановитьЗначение("Валюта", Валюта);
    Элемент.Режим = РежимБлокировкиДанных.Исключительный;
    Блокировка.Заблокировать();
    ОтменитьТранзакцию();
    Возврат Истина;
КонецФункции

&НаСервере
Процедура ИзменитьСрез(Валюта)
    Срез = РегистрыСведений.КурсыВалют.СрезПоследних('20260904000000', Новый Структура("Валюта", Валюта));
    Срез[0].Курс = 0;
КонецПроцедуры`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	vmContext := machine.NewContextWithMetadata(runtime)
	result, err := vmContext.CallContext(ctx, "Проверить", bytecode.String(bslCurrency.String()))
	if err != nil || result.String() != "86.5" {
		t.Fatalf("BSL register result=%v error=%v", result, err)
	}
	if result, err := vmContext.CallContext(ctx, "ПроверитьБлокировку", bytecode.String(bslCurrency.String())); err != nil || result.String() != "True" {
		t.Fatalf("BSL register lock result=%v error=%v", result, err)
	}
	if _, err := vmContext.CallContext(ctx, "ИзменитьСрез", bytecode.String(bslCurrency.String())); err == nil {
		t.Fatal("information register slice allowed mutation")
	}
	if err := runtime.SetDataLockWaitTimeout(150 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	firstLockContext, finishFirstLock, err := runtime.BeginExecution(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.BeginTransaction(firstLockContext); err != nil {
		t.Fatal(err)
	}
	lock := &dataLock{runtime: runtime}
	lockElement, err := runtime.newDataLockElement("РегистрСведений.КурсыВалют")
	if err != nil {
		t.Fatal(err)
	}
	periodValue, _ := bytecode.Date(time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC))
	if err := runtime.setDataLockValue(lockElement, "Период", periodValue); err != nil {
		t.Fatal(err)
	}
	if err := runtime.setDataLockValue(lockElement, "Валюта", bytecode.String(bslCurrency.String())); err != nil {
		t.Fatal(err)
	}
	lock.entries = []*dataLockElement{lockElement}
	if err := runtime.acquireDataLocks(firstLockContext, lock); err != nil {
		t.Fatal(err)
	}
	conflicting, _ := repository.NewRecordSet("КурсыВалют")
	conflictingPeriod := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	conflicting.Filter.Period = &conflictingPeriod
	conflicting.Filter.Dimensions[currencyID] = Value{Kind: UUIDType, Data: bslCurrency.String()}
	conflictingRecord, _ := conflicting.Add()
	conflictingRecord.Period = conflictingPeriod
	conflictingRecord.Dimensions = mapsCloneValues(conflicting.Filter.Dimensions)
	conflictingRecord.Resources[rateID] = Value{Kind: NumberType, Data: "50"}
	secondLockContext, finishSecondLock, err := runtime.BeginExecution(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Write(secondLockContext, conflicting, true); !errors.Is(err, ErrDataLockTimeout) {
		t.Fatalf("register lock conflict error=%v", err)
	}
	if err := finishSecondLock(nil); err != nil {
		t.Fatal(err)
	}
	if err := runtime.RollbackTransaction(firstLockContext); err != nil {
		t.Fatal(err)
	}
	if err := finishFirstLock(nil); err != nil {
		t.Fatal(err)
	}
	broadContext, finishBroad, err := runtime.BeginExecution(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.BeginTransaction(broadContext); err != nil {
		t.Fatal(err)
	}
	broadElement, err := runtime.newDataLockElement("РегистрСведений.КурсыВалют")
	if err != nil {
		t.Fatal(err)
	}
	broadElement.mode = dataLockShared
	broadLock := &dataLock{runtime: runtime, entries: []*dataLockElement{broadElement}}
	if err := runtime.acquireDataLocks(broadContext, broadLock); err != nil {
		t.Fatal(err)
	}
	broadWriterContext, finishBroadWriter, err := runtime.BeginExecution(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Write(broadWriterContext, conflicting, true); !errors.Is(err, ErrDataLockTimeout) {
		t.Fatalf("shared broad register lock did not block a write: %v", err)
	}
	if err := finishBroadWriter(nil); err != nil {
		t.Fatal(err)
	}
	if err := runtime.RollbackTransaction(broadContext); err != nil {
		t.Fatal(err)
	}
	if err := finishBroad(nil); err != nil {
		t.Fatal(err)
	}

	rollbackSet, _ := repository.NewRecordSet("КурсыВалют")
	rollbackPeriod := firstPeriod.AddDate(0, 0, 10)
	rollbackSet.Filter.Period = &rollbackPeriod
	rollbackSet.Filter.Dimensions[currencyID] = Value{Kind: UUIDType, Data: uuid.MustNew().String()}
	rollbackRecord, _ := rollbackSet.Add()
	rollbackRecord.Period = rollbackPeriod
	rollbackRecord.Dimensions = mapsCloneValues(rollbackSet.Filter.Dimensions)
	rollbackRecord.Resources[rateID] = Value{Kind: NumberType, Data: "1"}
	execution, finish, err := runtime.BeginExecution(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.BeginTransaction(execution); err != nil {
		t.Fatal(err)
	}
	if err := repository.Write(execution, rollbackSet, true); err != nil {
		t.Fatal(err)
	}
	if err := runtime.RollbackTransaction(execution); err != nil {
		t.Fatal(err)
	}
	if err := finish(nil); err != nil {
		t.Fatal(err)
	}
	if rollbackSet.Records[0].RecordID.IsZero() {
		t.Fatal("rollback did not restore record-set identity")
	}
	check, _ := repository.NewRecordSet("КурсыВалют")
	check.Filter = cloneInformationRegisterFilter(rollbackSet.Filter)
	if err := repository.Read(ctx, check); err != nil || len(check.Records) != 0 {
		t.Fatalf("rolled-back records=%+v error=%v", check.Records, err)
	}

	rejected := errors.New("reject register write")
	second.Records[0].Resources[rateID] = Value{Kind: NumberType, Data: "999"}
	if err := repository.WriteWithHandler(ctx, second, true, InformationRegisterEventHandlerFunc(func(_ context.Context, event InformationRegisterEvent, _ *InformationRegisterRecordSet, _ bool) (bool, error) {
		if event == InformationRegisterEventOnWrite {
			return false, rejected
		}
		return false, nil
	})); !errors.Is(err, rejected) {
		t.Fatalf("rejected register write error=%v", err)
	}
	preserved, _ := repository.NewRecordSet("КурсыВалют")
	preserved.Filter = cloneInformationRegisterFilter(second.Filter)
	if err := repository.Read(ctx, preserved); err != nil || len(preserved.Records) != 1 || preserved.Records[0].Resources[rateID].Data != "41.5" {
		t.Fatalf("event rollback did not preserve stored register data: records=%+v error=%v", preserved.Records, err)
	}
}
