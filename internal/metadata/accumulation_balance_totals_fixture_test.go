package metadata

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// balanceTotalsFixture is a balance register with two dimensions and the
// movements every test wrote into it, kept on the Go side as well: the answer
// a balance must give is computed from them, not from the base.
type balanceTotalsFixture struct {
	pool        *pgxpool.Pool
	registers   *AccumulationRegisterRepository
	documents   *DocumentRepository
	definition  AccumulationRegisterDefinition
	warehouseID uuid.UUID
	productID   uuid.UUID
	quantityID  uuid.UUID
	totalsTable string

	mu        sync.Mutex
	movements map[string][]balanceMovement
	records   map[string]*DocumentRecord
}

type balanceMovement struct {
	period    time.Time
	warehouse string
	product   string
	receipt   bool
	amount    int64
}

func newBalanceTotalsFixture(ctx context.Context, t *testing.T, splitting bool, maxConnections int) *balanceTotalsFixture {
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
	documentID, registerID, warehouseID, productID, quantityID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_, _ = pool.Exec(cleanup, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE")
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.register_totals_modes WHERE metadata_id = $1", registerID)
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
		pool.Close()
	})
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE"); err != nil {
		t.Fatal(err)
	}
	if err := EnsureRegisterTotalsStorage(ctx, pool); err != nil {
		t.Fatal(err)
	}
	catalog := &Catalog{
		Documents: []DocumentDefinition{{
			ID: documentID, Name: "Приход", Number: DocumentNumber{Type: StringType, Length: 20, Unique: true, Periodicity: NumberPeriodYear},
			Movements: []uuid.UUID{registerID},
		}},
		AccumulationRegisters: []AccumulationRegisterDefinition{{
			ID: registerID, Name: "Остатки", Kind: AccumulationRegisterBalance, AllowTotalsSplitting: splitting,
			Dimensions: []RegisterDimension{
				{Attribute: Attribute{ID: warehouseID, Name: "Склад", Types: []Type{{Kind: StringType, Length: 20}}}},
				{Attribute: Attribute{ID: productID, Name: "Товар", Types: []Type{{Kind: StringType, Length: 50}}}},
			},
			Resources: []Attribute{{ID: quantityID, Name: "Количество", Types: []Type{{Kind: NumberType, Precision: 15, Scale: 3}}}},
		}},
		documentByName: map[string]int{"приход": 0}, documentByID: map[uuid.UUID]int{documentID: 0},
		accumulationRegisterByName: map[string]int{"остатки": 0}, accumulationRegisterByID: map[uuid.UUID]int{registerID: 0},
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
	return &balanceTotalsFixture{
		pool: pool, registers: registers, documents: documents, definition: catalog.AccumulationRegisters[0],
		warehouseID: warehouseID, productID: productID, quantityID: quantityID, totalsTable: totalsTable,
		movements: map[string][]balanceMovement{}, records: map[string]*DocumentRecord{},
	}
}

// post writes the movements of one document, replacing what it had. The
// document is created on first use.
func (fixture *balanceTotalsFixture) post(ctx context.Context, number string, movements ...balanceMovement) error {
	fixture.mu.Lock()
	document := fixture.records[number]
	fixture.mu.Unlock()
	if document == nil {
		created, err := fixture.documents.New(ctx, "Приход", nil)
		if err != nil {
			return err
		}
		created.Number, created.Date = number, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		if err := fixture.documents.Save(ctx, created, nil); err != nil {
			return err
		}
		document = created
		fixture.mu.Lock()
		fixture.records[number] = document
		fixture.mu.Unlock()
	}
	set, err := fixture.registers.NewRecordSet("Остатки")
	if err != nil {
		return err
	}
	set.Filter.Recorder = &document.Reference
	for _, movement := range movements {
		row, err := set.Add()
		if err != nil {
			return err
		}
		row.Period, row.Recorder, row.Active = movement.period, document.Reference, true
		row.MovementKind = AccumulationMovementExpense
		if movement.receipt {
			row.MovementKind = AccumulationMovementReceipt
		}
		row.Dimensions[fixture.warehouseID] = Value{Kind: StringType, Data: movement.warehouse}
		row.Dimensions[fixture.productID] = Value{Kind: StringType, Data: movement.product}
		row.Resources[fixture.quantityID] = Value{Kind: NumberType, Data: fmt.Sprint(movement.amount)}
	}
	if err := fixture.registers.Write(ctx, set, true); err != nil {
		return err
	}
	fixture.mu.Lock()
	fixture.movements[number] = append([]balanceMovement(nil), movements...)
	fixture.mu.Unlock()
	return nil
}

// expected is the balance of every combination from the movements alone:
// those up to and including the date, or strictly before it.
func (fixture *balanceTotalsFixture) expected(at time.Time, inclusive bool) map[string]int64 {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	result := map[string]int64{}
	for _, movements := range fixture.movements {
		for _, movement := range movements {
			if movement.period.After(at) || (!inclusive && movement.period.Equal(at)) {
				continue
			}
			amount := movement.amount
			if !movement.receipt {
				amount = -amount
			}
			result[movement.warehouse+"/"+movement.product] += amount
		}
	}
	for key, amount := range result {
		if amount == 0 {
			delete(result, key)
		}
	}
	return result
}

func (fixture *balanceTotalsFixture) combination(row AccumulationRegisterTotalsRow) string {
	return row.Dimensions[fixture.warehouseID].Data + "/" + row.Dimensions[fixture.productID].Data
}

func wholeAmount(t *testing.T, value Value) int64 {
	t.Helper()
	amount, ok := new(big.Rat).SetString(value.Data)
	if !ok || !amount.IsInt() {
		t.Fatalf("amount %q is not a whole number", value.Data)
	}
	return amount.Num().Int64()
}

// requireBalances compares the balances at a date, whole and filtered, with
// the movements.
func (fixture *balanceTotalsFixture) requireBalances(ctx context.Context, t *testing.T, label string, at time.Time) {
	t.Helper()
	want := fixture.expected(at, true)
	rows, err := fixture.registers.Balances(ctx, "Остатки", at, nil)
	if err != nil {
		t.Fatalf("%s: balances at %s: %v", label, at.Format(time.RFC3339), err)
	}
	got := map[string]int64{}
	for _, row := range rows {
		got[fixture.combination(row)] = wholeAmount(t, row.Turnover[fixture.quantityID])
	}
	requireSameBalances(t, fmt.Sprintf("%s: balances at %s", label, at.Format(time.RFC3339)), want, got)
	// One filter on the second dimension alone and one on both: the index
	// leads with the first, and a filter it cannot use must answer the same.
	for _, filter := range []struct {
		warehouse, product string
	}{{"", "P2"}, {"W1", "P1"}} {
		dimensions := map[uuid.UUID]Value{fixture.productID: {Kind: StringType, Data: filter.product}}
		if filter.warehouse != "" {
			dimensions[fixture.warehouseID] = Value{Kind: StringType, Data: filter.warehouse}
		}
		rows, err := fixture.registers.Balances(ctx, "Остатки", at, dimensions)
		if err != nil {
			t.Fatalf("%s: filtered balances: %v", label, err)
		}
		got := map[string]int64{}
		for _, row := range rows {
			got[fixture.combination(row)] = wholeAmount(t, row.Turnover[fixture.quantityID])
		}
		narrowed := map[string]int64{}
		for key, amount := range want {
			parts := strings.SplitN(key, "/", 2)
			if parts[1] == filter.product && (filter.warehouse == "" || parts[0] == filter.warehouse) {
				narrowed[key] = amount
			}
		}
		requireSameBalances(t, fmt.Sprintf("%s: balances at %s filtered %+v", label, at.Format(time.RFC3339), filter), narrowed, got)
	}
	// The opening balance of a period that starts at the date is the balance
	// strictly before it, and the closing one is the balance at its end.
	end := at.Add(36 * time.Hour)
	if end.Year() > 3999 {
		end = at
	}
	combined, err := fixture.registers.BalancesAndTurnovers(ctx, "Остатки", at, end, nil)
	if err != nil {
		t.Fatalf("%s: balances and turnovers: %v", label, err)
	}
	opening, closing := map[string]int64{}, map[string]int64{}
	for _, row := range combined {
		if amount := wholeAmount(t, row.Opening[fixture.quantityID]); amount != 0 {
			opening[fixture.combination(row)] = amount
		}
		if amount := wholeAmount(t, row.Closing[fixture.quantityID]); amount != 0 {
			closing[fixture.combination(row)] = amount
		}
	}
	requireSameBalances(t, fmt.Sprintf("%s: opening at %s", label, at.Format(time.RFC3339)), fixture.expected(at, false), opening)
	requireSameBalances(t, fmt.Sprintf("%s: closing at %s", label, end.Format(time.RFC3339)), fixture.expected(end, true), closing)
}

func requireSameBalances(t *testing.T, label string, want, got map[string]int64) {
	t.Helper()
	keys := map[string]bool{}
	for key := range want {
		keys[key] = true
	}
	for key := range got {
		keys[key] = true
	}
	var differences []string
	for key := range keys {
		if want[key] != got[key] {
			differences = append(differences, fmt.Sprintf("%s want %d got %d", key, want[key], got[key]))
		}
	}
	if len(differences) != 0 {
		sort.Strings(differences)
		t.Fatalf("%s:\n  %s", label, strings.Join(differences, "\n  "))
	}
}

// storedTotals is what the table of totals holds, summed across splits: the
// period each row is for and its amount per combination.
func (fixture *balanceTotalsFixture) storedTotals(ctx context.Context, t *testing.T) map[time.Time]map[string]int64 {
	t.Helper()
	warehouse, _ := PhysicalAttributeColumn(fixture.warehouseID)
	product, _ := PhysicalAttributeColumn(fixture.productID)
	quantity, _ := PhysicalAttributeColumn(fixture.quantityID)
	rows, err := fixture.pool.Query(ctx, "SELECT total_period, "+pgx.Identifier{warehouse}.Sanitize()+", "+pgx.Identifier{product}.Sanitize()+
		", SUM("+pgx.Identifier{quantity}.Sanitize()+")::bigint FROM "+qualifiedCatalogTable(fixture.totalsTable)+" GROUP BY 1, 2, 3")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	result := map[time.Time]map[string]int64{}
	for rows.Next() {
		var period time.Time
		var warehouseValue, productValue string
		var amount int64
		if err := rows.Scan(&period, &warehouseValue, &productValue, &amount); err != nil {
			t.Fatal(err)
		}
		period = period.UTC()
		if result[period] == nil {
			result[period] = map[string]int64{}
		}
		if amount != 0 {
			result[period][warehouseValue+"/"+productValue] = amount
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

// requireStoredShape checks the rows themselves, not the answers: a row for
// every stored month holding the balance before it, the present totals holding
// the balance after everything, and nothing else. The answers alone would not
// see a model that is wrong in a way reading happens to cancel out.
func (fixture *balanceTotalsFixture) requireStoredShape(ctx context.Context, t *testing.T, label string, first, last *time.Time, present bool) {
	t.Helper()
	stored := fixture.storedTotals(ctx, t)
	expected := map[time.Time]map[string]int64{}
	if present {
		expected[accumulationPresentTotalsPeriod] = fixture.expected(time.Date(3999, 12, 31, 0, 0, 0, 0, time.UTC), true)
	}
	if first != nil && last != nil {
		for month := *first; !month.After(*last); month = month.AddDate(0, 1, 0) {
			expected[month] = fixture.expected(month, false)
		}
	}
	for period, amounts := range stored {
		if len(amounts) == 0 {
			t.Fatalf("%s: rows of totals at %s add up to zero and should have been removed", label, period.Format(time.RFC3339))
		}
		if _, ok := expected[period]; !ok {
			t.Fatalf("%s: a row of totals at %s is outside the stored period", label, period.Format(time.RFC3339))
		}
	}
	for period, amounts := range expected {
		requireSameBalances(t, fmt.Sprintf("%s: stored totals at %s", label, period.Format(time.RFC3339)), amounts, stored[period])
	}
}
