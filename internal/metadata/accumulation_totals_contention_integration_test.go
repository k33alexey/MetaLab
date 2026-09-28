package metadata

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The point of the iteration: rows of totals multiply when writers collide and
// only then. «Записи будут "размножаться" только при параллельно выполняемых
// транзакциях, их количество по каждой комбинации измерений будет зависеть от
// максимального количества одновременно выполняемых транзакций.»
//
// The old mechanism hashed the recorder into one of sixteen rows and wrote
// them whether or not anybody else was there, so a database with one person in
// it carried sixteen times the totals it needed.
func TestTotalsMultiplyOnlyUnderContention(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
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
	if err := EnsureRegisterTotalsStorage(ctx, pool); err != nil {
		t.Fatal(err)
	}

	documentID, registerID, productID, amountID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{
		Documents: []DocumentDefinition{{
			ID: documentID, Name: "Продажа", Number: DocumentNumber{Type: StringType, Length: 20, Unique: true, Periodicity: NumberPeriodYear},
			Movements: []uuid.UUID{registerID},
		}},
		AccumulationRegisters: []AccumulationRegisterDefinition{{
			ID: registerID, Name: "Продажи", Kind: AccumulationRegisterTurnover, AllowTotalsSplitting: true,
			Dimensions: []Attribute{{ID: productID, Name: "Товар", Types: []Type{{Kind: StringType, Length: 100}}}},
			Resources:  []Attribute{{ID: amountID, Name: "Сумма", Types: []Type{{Kind: NumberType, Precision: 15, Scale: 2}}}},
		}},
		documentByName: map[string]int{"продажа": 0}, documentByID: map[uuid.UUID]int{documentID: 0},
		accumulationRegisterByName: map[string]int{"продажи": 0}, accumulationRegisterByID: map[uuid.UUID]int{registerID: 0},
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
		ProjectID: projectID, PackageSHA256: repeatCatalogHex('e', 64), GitCommit: repeatCatalogHex('f', 40),
		Desired: desired, ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true,
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
	month := time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC)
	write := func(number, amount string) error {
		document, createErr := documents.New(ctx, "Продажа", nil)
		if createErr != nil {
			return createErr
		}
		document.Number, document.Date = number, month
		if saveErr := documents.Save(ctx, document, nil); saveErr != nil {
			return saveErr
		}
		set, setErr := registers.NewRecordSet("Продажи")
		if setErr != nil {
			return setErr
		}
		set.Filter.Recorder = &document.Reference
		row, addErr := set.Add()
		if addErr != nil {
			return addErr
		}
		row.Period, row.Recorder, row.Active = month, document.Reference, true
		row.Dimensions[productID] = Value{Kind: StringType, Data: "A"}
		row.Resources[amountID] = Value{Kind: NumberType, Data: amount}
		return registers.Write(ctx, set, true)
	}
	totalsTable, _ := PhysicalAccumulationRegisterTotalsTable(registerID)
	rowCount := func() int {
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+qualifiedCatalogTable(totalsTable)).Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}
	sum := func() string {
		result, queryErr := registers.Turnovers(ctx, "Продажи", month, month, map[uuid.UUID]Value{productID: {Kind: StringType, Data: "A"}})
		if queryErr != nil {
			t.Fatal(queryErr)
		}
		if len(result) != 1 {
			t.Fatalf("turnover rows = %d", len(result))
		}
		return result[0].Turnover[amountID].Data
	}

	// Twenty writes one after another meet nobody, so they share one row.
	for index := 0; index < 20; index++ {
		if err := write("Q-"+string(rune('a'+index)), "1"); err != nil {
			t.Fatal(err)
		}
	}
	if rows := rowCount(); rows != 1 {
		t.Fatalf("twenty writes in a row made %d rows of totals, and met nobody", rows)
	}
	if sum() != "20" {
		t.Fatalf("turnover after twenty writes = %q", sum())
	}

	// Eight at once do meet each other, and the totals give way instead of
	// queueing. How many rows appear depends on the collisions that really
	// happened, so the test says what must hold whatever they were: more than
	// one writer got through without waiting for the first, no row went
	// missing, and the sum is exact.
	const writers = 8
	var group sync.WaitGroup
	errs := make(chan error, writers)
	start := make(chan struct{})
	for index := 0; index < writers; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start
			errs <- write("P-"+string(rune('a'+index)), "3")
		}(index)
	}
	close(start)
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("a concurrent write failed: %v", err)
		}
	}
	if sum() != "44" {
		t.Fatalf("turnover after twenty and eight writes = %q, rows = %d", sum(), rowCount())
	}
	concurrent := rowCount()
	t.Logf("%d одновременных писателей оставили %d записей итогов", writers, concurrent)
	if concurrent < 1 || concurrent > writers+1 {
		t.Fatalf("rows of totals = %d, writers = %d", concurrent, writers)
	}

	// A rebuild folds whatever the concurrency left behind, and the number
	// does not move.
	if err := registers.RebuildTotals(ctx, "Продажи"); err != nil {
		t.Fatal(err)
	}
	if rows := rowCount(); rows != 1 {
		t.Fatalf("the rebuild left %d rows", rows)
	}
	if sum() != "44" {
		t.Fatalf("turnover after the rebuild = %q", sum())
	}
}
