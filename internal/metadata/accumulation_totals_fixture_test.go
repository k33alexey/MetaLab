package metadata

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// accumulationTotalsFixture is one turnover register with one dimension and one
// resource, and a document that writes into it. Every test of the rows of
// totals needs the same thing, and they need it in a real database: what is
// being tested is who waits for whom and which index a statement uses, and
// neither exists outside PostgreSQL.
type accumulationTotalsFixture struct {
	pool        *pgxpool.Pool
	catalog     *Catalog
	definition  AccumulationRegisterDefinition
	documents   *DocumentRepository
	registers   *AccumulationRegisterRepository
	registerID  uuid.UUID
	productID   uuid.UUID
	amountID    uuid.UUID
	totalsTable string
	month       time.Time
}

// newAccumulationTotalsFixture prepares the register. maxConnections raises the
// pool ceiling for the tests that want many writers at once: the default pool
// is the size of the machine, and writers queueing on a connection are writers
// that never meet in the database, which is the opposite of what those tests
// are for.
func newAccumulationTotalsFixture(ctx context.Context, t *testing.T, splitting bool, maxConnections int) *accumulationTotalsFixture {
	t.Helper()
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	if maxConnections > 0 {
		separator := "?"
		if strings.Contains(databaseURL, "?") {
			separator = "&"
		}
		databaseURL += fmt.Sprintf("%spool_max_conns=%d", separator, maxConnections)
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	projectID := uuid.MustNew()
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
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
			ID: registerID, Name: "Продажи", Kind: AccumulationRegisterTurnover, AllowTotalsSplitting: splitting,
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
	totalsTable, _ := PhysicalAccumulationRegisterTotalsTable(registerID)
	return &accumulationTotalsFixture{
		pool: pool, catalog: catalog, definition: catalog.AccumulationRegisters[0],
		documents: documents, registers: registers,
		registerID: registerID, productID: productID, amountID: amountID,
		totalsTable: totalsTable, month: time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC),
	}
}

// write posts one document with one movement, which is the smallest thing that
// touches the rows of totals the way the application does.
func (fixture *accumulationTotalsFixture) write(ctx context.Context, number, product, amount string) error {
	document, err := fixture.newDocument(ctx, number)
	if err != nil {
		return err
	}
	return fixture.post(ctx, document, product, amount)
}

// newDocument and post are the two halves of write, apart because the tests of
// contention need the documents ready before anybody starts.
//
// Saving a document takes its own locks - the number is allocated under one -
// so writers that create their documents at the same moment queue there and
// reach the rows of totals one after another, which is the opposite of what a
// test of collisions needs. Prepared documents let the writers meet where they
// are meant to meet.
func (fixture *accumulationTotalsFixture) newDocument(ctx context.Context, number string) (*DocumentRecord, error) {
	document, err := fixture.documents.New(ctx, "Продажа", nil)
	if err != nil {
		return nil, err
	}
	document.Number, document.Date = number, fixture.month
	if err := fixture.documents.Save(ctx, document, nil); err != nil {
		return nil, err
	}
	return document, nil
}

func (fixture *accumulationTotalsFixture) post(ctx context.Context, document *DocumentRecord, product, amount string) error {
	set, err := fixture.registers.NewRecordSet("Продажи")
	if err != nil {
		return err
	}
	set.Filter.Recorder = &document.Reference
	row, err := set.Add()
	if err != nil {
		return err
	}
	row.Period, row.Recorder, row.Active = fixture.month, document.Reference, true
	row.Dimensions[fixture.productID] = Value{Kind: StringType, Data: product}
	row.Resources[fixture.amountID] = Value{Kind: NumberType, Data: amount}
	return fixture.registers.Write(ctx, set, true)
}

// holdLowestRow keeps the lowest-numbered row of totals locked until the test
// releases it, the way another writer's open transaction would.
func (fixture *accumulationTotalsFixture) holdLowestRow(ctx context.Context, t *testing.T) (pgx.Tx, int16) {
	t.Helper()
	holder, err := fixture.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Rollback(context.Background()) })
	var held int16
	if err := holder.QueryRow(ctx, "SELECT totals_split FROM "+fixture.qualifiedTotals()+
		" ORDER BY totals_split LIMIT 1 FOR UPDATE").Scan(&held); err != nil {
		t.Fatal(err)
	}
	return holder, held
}

func (fixture *accumulationTotalsFixture) qualifiedTotals() string {
	return qualifiedCatalogTable(fixture.totalsTable)
}

func (fixture *accumulationTotalsFixture) rowCount(ctx context.Context, t *testing.T) int {
	t.Helper()
	var count int
	if err := fixture.pool.QueryRow(ctx, "SELECT count(*) FROM "+fixture.qualifiedTotals()).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

// splitNumbers is the numbering one combination ended up with, highest last.
func (fixture *accumulationTotalsFixture) splitNumbers(ctx context.Context, t *testing.T, product string) []int16 {
	t.Helper()
	column, _ := PhysicalAttributeColumn(fixture.productID)
	rows, err := fixture.pool.Query(ctx, "SELECT totals_split FROM "+fixture.qualifiedTotals()+
		" WHERE "+pgx.Identifier{column}.Sanitize()+" = $1 ORDER BY totals_split", product)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var numbers []int16
	for rows.Next() {
		var number int16
		if err := rows.Scan(&number); err != nil {
			t.Fatal(err)
		}
		numbers = append(numbers, number)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return numbers
}

func (fixture *accumulationTotalsFixture) turnover(ctx context.Context, t *testing.T, product string) string {
	t.Helper()
	result, err := fixture.registers.Turnovers(ctx, "Продажи", fixture.month, fixture.month,
		map[uuid.UUID]Value{fixture.productID: {Kind: StringType, Data: product}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("turnover rows for %q = %d", product, len(result))
	}
	return result[0].Turnover[fixture.amountID].Data
}
