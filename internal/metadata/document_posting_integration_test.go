package metadata

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestDocumentPostingIsAtomicAndReplacesMovementsIntegration(t *testing.T) {
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

	documentID, registerID := uuid.MustNew(), uuid.MustNew()
	productAttributeID, quantityAttributeID := uuid.MustNew(), uuid.MustNew()
	productDimensionID, quantityResourceID := uuid.MustNew(), uuid.MustNew()
	document := DocumentDefinition{
		ID: documentID, Name: "Поступление", Posting: true,
		Number: DocumentNumber{Type: StringType, Length: 20, Unique: true, Periodicity: NumberPeriodYear},
		Attributes: []Attribute{
			{ID: productAttributeID, Name: "Товар", Required: true, Types: []Type{{Kind: StringType, Length: 100}}},
			{ID: quantityAttributeID, Name: "Количество", Required: true, Types: []Type{{Kind: NumberType, Precision: 15, Scale: 3}}},
		},
	}
	register := AccumulationRegisterDefinition{
		ID: registerID, Name: "ОстаткиТоваров", Kind: AccumulationRegisterBalance, Recorders: []uuid.UUID{documentID},
		Dimensions: []Attribute{{ID: productDimensionID, Name: "Товар", Required: true, Types: []Type{{Kind: StringType, Length: 100}}}},
		Resources:  []Attribute{{ID: quantityResourceID, Name: "Количество", Required: true, Types: []Type{{Kind: NumberType, Precision: 15, Scale: 3}}}},
	}
	catalog := &Catalog{
		Documents: []DocumentDefinition{document}, AccumulationRegisters: []AccumulationRegisterDefinition{register},
		documentByName: map[string]int{"поступление": 0}, documentByID: map[uuid.UUID]int{documentID: 0},
		accumulationRegisterByName: map[string]int{"остаткитоваров": 0}, accumulationRegisterByID: map[uuid.UUID]int{registerID: 0},
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

	documents, err := NewDocumentRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	registers, err := NewAccumulationRegisterRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewRuntimeWithAllRegisters(nil, nil, documents, nil, registers, catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	program, diagnostics := CompileDocumentObjectModule(document, "receipt-object.bsl", `&НаСервере
Процедура ПередЗаписью(Отказ, РежимЗаписи, РежимПроведения)
    Если РежимЗаписи = "Post" И РежимПроведения = "DoNotPost" Тогда
        Отказ = Истина;
    КонецЕсли;
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
КонецПроцедуры`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	bslEvents, err := NewDocumentBSLEvents(runtime, machine.NewContextWithMetadata(runtime), document)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.SetDocumentEventHandler(document.Name, bslEvents); err != nil {
		t.Fatal(err)
	}

	value, err := runtime.CreateDocumentObject(ctx, document.Name)
	if err != nil {
		t.Fatal(err)
	}
	objectValue, _ := value.AsRuntimeObject()
	date := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	dateValue, err := bytecode.Date(date)
	if err != nil {
		t.Fatal(err)
	}
	for name, assigned := range map[string]bytecode.Value{
		"Номер": bytecode.String("IN-1"), "Дата": dateValue, "Товар": bytecode.String("A"), "Количество": bytecode.Number(5),
	} {
		if err := runtime.SetObjectProperty(ctx, objectValue, name, assigned); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := runtime.CallObjectMethod(ctx, objectValue, "Записать", []bytecode.Value{
		bytecode.String("post"), bytecode.String("regular"),
	}); err != nil {
		t.Fatal(err)
	}
	object := objectValue.(*documentObject)
	reference := object.record.Reference
	assertDocumentPostingState(t, ctx, documents, registers, reference, productDimensionID, quantityResourceID, date, true, "5")

	if err := runtime.SetObjectProperty(ctx, objectValue, "Количество", bytecode.Number(7)); err != nil {
		t.Fatal(err)
	}
	postingFailure := errors.New("posting failure after movement write")
	if err := runtime.SetDocumentEventHandler(document.Name, DocumentEventHandlerFunc(func(eventContext context.Context, event DocumentEvent, record *DocumentRecord) (bool, error) {
		cancelled, handleErr := bslEvents.HandleDocumentEvent(eventContext, event, record)
		if handleErr == nil && event == DocumentEventPosting {
			return false, postingFailure
		}
		return cancelled, handleErr
	})); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.CallObjectMethod(ctx, objectValue, "Провести", nil); !errors.Is(err, postingFailure) {
		t.Fatalf("failed repost error = %v", err)
	}
	assertDocumentPostingState(t, ctx, documents, registers, reference, productDimensionID, quantityResourceID, date, true, "5")

	if err := runtime.SetDocumentEventHandler(document.Name, bslEvents); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.CallObjectMethod(ctx, objectValue, "Провести", []bytecode.Value{bytecode.String("real-time")}); err != nil {
		t.Fatal(err)
	}
	assertDocumentPostingState(t, ctx, documents, registers, reference, productDimensionID, quantityResourceID, date, true, "7")
	if _, err := runtime.CallObjectMethod(ctx, objectValue, "ОтменитьПроведение", nil); err != nil {
		t.Fatal(err)
	}
	assertDocumentPostingState(t, ctx, documents, registers, reference, productDimensionID, quantityResourceID, date, false, "")
}

func assertDocumentPostingState(t *testing.T, ctx context.Context, documents *DocumentRepository, registers *AccumulationRegisterRepository, reference DocumentReference, dimensionID, resourceID uuid.UUID, period time.Time, posted bool, balance string) {
	t.Helper()
	record, err := documents.Get(ctx, reference)
	if err != nil || record.Posted != posted {
		t.Fatalf("document posted=%v error=%v", record != nil && record.Posted, err)
	}
	rows, err := registers.Balances(ctx, "ОстаткиТоваров", period.Add(time.Hour), map[uuid.UUID]Value{dimensionID: {Kind: StringType, Data: "A"}})
	if err != nil {
		t.Fatal(err)
	}
	if balance == "" {
		if len(rows) != 0 {
			t.Fatalf("unposted document retained balances: %+v", rows)
		}
		return
	}
	if len(rows) != 1 || rows[0].Turnover[resourceID].Data != balance {
		t.Fatalf("balance=%+v, want %s", rows, balance)
	}
}
