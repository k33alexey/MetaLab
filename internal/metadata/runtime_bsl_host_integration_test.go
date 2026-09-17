package metadata

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// TestWireBSLEventsMatchesRuntimeSnapshotModuleNamingIntegration exercises the
// exact ML App production path: a RuntimeSnapshot compiled as a whole project
// via CompileModules, wired into a Runtime via WireBSLEvents - not the
// synthetic single-module CompileDocumentObjectModule helper that
// TestDocumentPostingIsAtomicAndReplacesMovementsIntegration uses. This is
// the seam where the RuntimeSnapshot module-naming scheme
// (moduleNameDescriptors, "МодульОбъектаДокумента."+name) and the event
// bridge module-naming scheme (DocumentObjectModuleName) must agree - a
// mismatch here previously made Провести silently create zero movements.
func TestWireBSLEventsMatchesRuntimeSnapshotModuleNamingIntegration(t *testing.T) {
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

	documentID, registerID, objectModuleID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	productAttributeID, quantityAttributeID := uuid.MustNew(), uuid.MustNew()
	productDimensionID, quantityResourceID := uuid.MustNew(), uuid.MustNew()
	document := DocumentDefinition{
		ID: documentID, Name: "Поступление", Posting: true, ObjectModule: &objectModuleID,
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

	// Name comes from moduleNameDescriptors - the actual production naming
	// scheme LoadProjectModules uses - not from DocumentObjectModuleName
	// itself, so this test genuinely proves the two schemes agree instead of
	// trivially matching a function against itself.
	descriptor := moduleNameDescriptors(catalog)[objectModuleID.String()]
	snapshot, err := RuntimeSnapshot{Format: CurrentFormat}.WithModules([]RuntimeModule{{
		Name: descriptor.name, Filename: "receipt-object.bsl",
		PredefinedVariables: descriptor.predefined,
		Source: `&НаСервере
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
КонецПроцедуры`,
	}})
	if err != nil {
		t.Fatal(err)
	}
	program, diagnostics, err := snapshot.CompileModules()
	if err != nil {
		t.Fatalf("compile: %v %v", err, diagnostics)
	}
	if err := WireBSLEvents(runtime, program, catalog); err != nil {
		t.Fatal(err)
	}

	value, err := runtime.CreateDocumentObject(ctx, document.Name)
	if err != nil {
		t.Fatal(err)
	}
	objectValue, _ := value.AsRuntimeObject()
	date := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
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
}
