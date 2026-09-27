package metadata

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Two of the six settings govern machinery that already runs, so they are tested
// against it rather than against the model.
//
// Deleting on unposting only is the one that matters most and reads the least
// obviously. Under it, nothing is wiped when posting begins: the records written
// last time are still there while the handler runs, so a handler that writes
// nothing leaves the previous records standing. Under "auto" - which is what ML
// did before this point, and what silence still means - the same posting clears
// them first and the balance goes to nothing.
func TestPostingHonoursHowItsRecordsAreDeletedIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	for name, deletion := range map[string]struct {
		mode RegisterRecordsDeletion
		// balanceAfterEmptyRepost is what is left after a repost whose handler
		// writes nothing at all.
		balanceAfterEmptyRepost string
	}{
		"авто стирает при начале проведения": {RegisterRecordsDeleteAuto, ""},
		"молчание значит авто":               {"", ""},
		"при отмене проведения — не стирает": {RegisterRecordsDeleteOnUnpost, "5"},
		"не стирать — не стирает":            {RegisterRecordsDeleteOff, "5"},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			pool, err := pgxpool.New(ctx, databaseURL)
			if err != nil {
				t.Fatal(err)
			}
			projectID := uuid.MustNew()
			documentID, registerID := uuid.MustNew(), uuid.MustNew()
			productAttributeID, quantityAttributeID := uuid.MustNew(), uuid.MustNew()
			productDimensionID, quantityResourceID := uuid.MustNew(), uuid.MustNew()
			t.Cleanup(func() {
				cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
				defer stop()
				_, _ = pool.Exec(cleanup, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE")
				_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.object_sequences WHERE metadata_id = ANY($1::uuid[])",
					[]string{documentID.String(), registerID.String()})
				_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
				pool.Close()
			})
			if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE"); err != nil {
				t.Fatal(err)
			}

			document := DocumentDefinition{
				ID: documentID, Name: "Поступление",
				Posting:   DocumentPosting{Allowed: true, RecordsDeletion: deletion.mode},
				Movements: []uuid.UUID{registerID},
				Number:    DocumentNumber{Type: StringType, Length: 20, Unique: true, Periodicity: NumberPeriodYear},
				Attributes: []Attribute{
					{ID: productAttributeID, Name: "Товар", FillChecking: ShowFillingError, Types: []Type{{Kind: StringType, Length: 100}}},
					{ID: quantityAttributeID, Name: "Количество", FillChecking: ShowFillingError, Types: []Type{{Kind: NumberType, Precision: 15, Scale: 3}}},
				},
			}
			register := AccumulationRegisterDefinition{
				ID: registerID, Name: "ОстаткиТоваров", Kind: AccumulationRegisterBalance,
				Dimensions: []Attribute{{ID: productDimensionID, Name: "Товар", FillChecking: ShowFillingError, Types: []Type{{Kind: StringType, Length: 100}}}},
				Resources:  []Attribute{{ID: quantityResourceID, Name: "Количество", FillChecking: ShowFillingError, Types: []Type{{Kind: NumberType, Precision: 15, Scale: 3}}}},
			}
			catalog := &Catalog{
				Documents: []DocumentDefinition{document}, AccumulationRegisters: []AccumulationRegisterDefinition{register},
				documentByName: map[string]int{"поступление": 0}, documentByID: map[uuid.UUID]int{documentID: 0},
				accumulationRegisterByName: map[string]int{"остаткитоваров": 0},
				accumulationRegisterByID:   map[uuid.UUID]int{registerID: 0},
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
				ProjectID: projectID, PackageSHA256: repeatCatalogHex('3', 64), GitCommit: repeatCatalogHex('4', 40), Desired: desired,
				ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true,
			}); err != nil {
				t.Fatal(err)
			}
			if err := EnsureObjectIntegrityStorage(ctx, pool); err != nil {
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
			// The gate lives on the runtime's posting path, so the posting goes
			// through the runtime. The handler is plain Go rather than BSL: what
			// is under test is when the previous records are wiped, not the
			// language.
			runtime, err := NewRuntimeWithAllRegisters(nil, nil, documents, nil, registers, catalog, nil)
			if err != nil {
				t.Fatal(err)
			}
			date := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
			writesFive := true
			handler := DocumentEventHandlerFunc(func(eventContext context.Context, event DocumentEvent, posted *DocumentRecord) (bool, error) {
				if event != DocumentEventPosting || !writesFive {
					return false, nil
				}
				return false, writeAccumulationMovement(eventContext, registers, register, posted, date,
					productDimensionID, quantityResourceID, "5")
			})
			if err := runtime.SetDocumentEventHandler("Поступление", handler); err != nil {
				t.Fatal(err)
			}
			value, err := runtime.CreateDocumentObject(ctx, "Поступление")
			if err != nil {
				t.Fatal(err)
			}
			objectValue, _ := value.AsRuntimeObject()
			dateValue, err := bytecode.Date(date)
			if err != nil {
				t.Fatal(err)
			}
			for name, assigned := range map[string]bytecode.Value{
				"Номер": bytecode.String("IN-1"), "Дата": dateValue,
				"Товар": bytecode.String("A"), "Количество": bytecode.Number(5),
			} {
				if err := runtime.SetObjectProperty(ctx, objectValue, name, assigned); err != nil {
					t.Fatal(err)
				}
			}
			// First posting writes five. Deletion is not exercised yet: there is
			// nothing there to delete.
			if _, err := runtime.CallObjectMethod(ctx, objectValue, "Провести", nil); err != nil {
				t.Fatal(err)
			}
			reference := objectValue.(*documentObject).record.Reference
			assertDocumentPostingState(t, ctx, documents, registers, reference,
				productDimensionID, quantityResourceID, date, true, "5")

			// Second posting writes nothing. What survives depends entirely on
			// whether the runtime wiped the previous records first.
			writesFive = false
			if _, err := runtime.CallObjectMethod(ctx, objectValue, "Провести", nil); err != nil {
				t.Fatal(err)
			}
			assertDocumentPostingState(t, ctx, documents, registers, reference,
				productDimensionID, quantityResourceID, date, true, deletion.balanceAfterEmptyRepost)
		})
	}
}

// A document that denies real-time posting is never posted that way. Refusing is
// the whole point: posting in the regular mode instead would look like success
// to a caller that asked to compete for the present.
func TestRealTimePostingIsRefusedWhereItIsDeniedIntegration(t *testing.T) {
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
	deniedID, allowedID := uuid.MustNew(), uuid.MustNew()
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, _ = pool.Exec(cleanup, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE")
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.object_sequences WHERE metadata_id = ANY($1::uuid[])",
			[]string{deniedID.String(), allowedID.String()})
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
		pool.Close()
	})
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE"); err != nil {
		t.Fatal(err)
	}
	number := DocumentNumber{Type: StringType, Length: 20, Auto: true, Unique: true, Periodicity: NumberPeriodYear}
	catalog := &Catalog{
		Documents: []DocumentDefinition{
			{ID: deniedID, Name: "Запрещено", Number: number,
				Posting: DocumentPosting{Allowed: true, RealTime: RealTimePostingDeny}},
			{ID: allowedID, Name: "Разрешено", Number: number,
				Posting: DocumentPosting{Allowed: true}},
		},
		documentByName: map[string]int{"запрещено": 0, "разрешено": 1},
		documentByID:   map[uuid.UUID]int{deniedID: 0, allowedID: 1},
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
		ProjectID: projectID, PackageSHA256: repeatCatalogHex('5', 64), GitCommit: repeatCatalogHex('6', 40), Desired: desired,
		ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := EnsureObjectIntegrityStorage(ctx, pool); err != nil {
		t.Fatal(err)
	}
	documents, err := NewDocumentRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]struct {
		document string
		refused  bool
	}{
		"запрещающий отказывает": {"запрещено", true},
		"разрешающий проводит":   {"разрешено", false},
	} {
		t.Run(name, func(t *testing.T) {
			record, err := documents.New(ctx, want.document, nil)
			if err != nil {
				t.Fatal(err)
			}
			record.Date = time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
			err = documents.Write(ctx, record, DocumentPost, DocumentPostingRealTime, nil, nil)
			switch {
			case want.refused && err == nil:
				t.Fatal("a document that denies real-time posting was posted in real time")
			case want.refused && !strings.Contains(err.Error(), "does not allow real-time posting"):
				t.Fatalf("err = %v", err)
			case !want.refused && err != nil:
				t.Fatalf("a document that allows real-time posting was refused: %v", err)
			}
		})
	}
	// Regular posting stays open to both: the setting is about the moment, not
	// about posting.
	record, err := documents.New(ctx, "запрещено", nil)
	if err != nil {
		t.Fatal(err)
	}
	record.Date = time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	if err := documents.Write(ctx, record, DocumentPost, DocumentPostingRegular, nil, nil); err != nil {
		t.Fatalf("regular posting was refused on a document that only denies real time: %v", err)
	}
}

// writeAccumulationMovement stands in for what a posting handler does, so that
// the test is about when the previous records are wiped and not about BSL.
func writeAccumulationMovement(ctx context.Context, registers *AccumulationRegisterRepository,
	definition AccumulationRegisterDefinition, record *DocumentRecord, period time.Time,
	dimensionID, resourceID uuid.UUID, quantity string) error {
	recorder := record.Reference
	set := &AccumulationRegisterRecordSet{
		RegisterID: definition.ID,
		Filter:     AccumulationRegisterFilter{Recorder: &recorder},
		Records: []*AccumulationRegisterRecord{{
			Period: period, Recorder: recorder, Active: true, MovementKind: AccumulationMovementReceipt,
			Dimensions: map[uuid.UUID]Value{dimensionID: {Kind: StringType, Data: "A"}},
			Resources:  map[uuid.UUID]Value{resourceID: {Kind: NumberType, Data: quantity}},
		}},
	}
	return registers.Write(ctx, set, true)
}
