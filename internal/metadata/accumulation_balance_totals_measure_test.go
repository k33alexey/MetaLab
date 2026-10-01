package metadata

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// TestMeasureBalanceTotalsAtScale is a measurement, not a check, and runs only
// when ML_MEASURE_TOTALS names the number of movements. It answers the two
// costs the model of the prototype moves around: a full rebuild of the totals
// over millions of movements, and a backdated posting that has to touch every
// stored month after it - with the period of calculated totals twelve months
// long, that is twelve rows of totals plus the present ones per combination.
//
// The movements are written straight into the table: posting five million of
// them through the repository measures the repository, not the totals.
func TestMeasureBalanceTotalsAtScale(t *testing.T) {
	count, _ := strconv.Atoi(os.Getenv("ML_MEASURE_TOTALS"))
	if count <= 0 {
		t.Skip("ML_MEASURE_TOTALS is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	fixture := newBalanceTotalsFixture(ctx, t, false, 0)
	warehouse, _ := PhysicalAttributeColumn(fixture.warehouseID)
	product, _ := PhysicalAttributeColumn(fixture.productID)
	quantity, _ := PhysicalAttributeColumn(fixture.quantityID)
	movementTable, _ := PhysicalAccumulationRegisterTable(fixture.definition.ID)
	movements := qualifiedCatalogTable(movementTable)

	const warehouses, products = 50, 200
	combinations := make([][]any, 0, warehouses*products)
	for w := 1; w <= warehouses; w++ {
		for p := 1; p <= products; p++ {
			name, item := "W"+strconv.Itoa(w), "P"+strconv.Itoa(p)
			key := accumulationDimensionKey(fixture.definition, map[uuid.UUID]Value{
				fixture.warehouseID: {Kind: StringType, Data: name},
				fixture.productID:   {Kind: StringType, Data: item},
			})
			combinations = append(combinations, []any{len(combinations), key, name, item})
		}
	}
	started := time.Now()
	if _, err := fixture.pool.Exec(ctx, "CREATE UNLOGGED TABLE measure_combinations (n integer PRIMARY KEY, dimension_key character(64), warehouse text, product text)"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = fixture.pool.Exec(context.Background(), "DROP TABLE IF EXISTS measure_combinations") })
	if _, err := fixture.pool.CopyFrom(ctx, pgx.Identifier{"measure_combinations"}, []string{"n", "dimension_key", "warehouse", "product"}, pgx.CopyFromRows(combinations)); err != nil {
		t.Fatal(err)
	}
	// Ten lines per recorder, three years of movements, two receipts to one
	// expense so balances stay mostly positive, as a warehouse's do.
	insert := `INSERT INTO ` + movements + ` (record_id, dimension_key, period, recorder_type, recorder_ref, line_no, active, movement_kind, ` +
		pgx.Identifier{warehouse}.Sanitize() + `, ` + pgx.Identifier{product}.Sanitize() + `, ` + pgx.Identifier{quantity}.Sanitize() + `)
SELECT gen_random_uuid(), c.dimension_key,
       timestamptz '2023-01-01 00:00:00+00' + (s % 1095) * interval '1 day' + (s % 86400) * interval '1 second',
       $1::uuid, md5((s / 10)::text)::uuid, (s % 10) + 1, true, CASE WHEN s % 3 = 0 THEN 2 ELSE 1 END,
       c.warehouse, c.product, (s % 97) + 1
FROM generate_series(0, $2::bigint - 1) s JOIN measure_combinations c ON c.n = (s * 7919) % $3`
	if _, err := fixture.pool.Exec(ctx, insert, fixture.documents.catalog.Documents[0].ID.String(), count, len(combinations)); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, "ANALYZE "+movements); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d movements over %d combinations written in %s", count, len(combinations), time.Since(started).Round(time.Millisecond))

	minimum, maximum := utcDate(2025, 1, 1, 0), utcDate(2025, 12, 31, 0)
	started = time.Now()
	if err := fixture.registers.SetTotalsPeriods(ctx, "Остатки", TotalsBound{Set: true, Date: minimum}, TotalsBound{Set: true, Date: maximum}); err != nil {
		t.Fatal(err)
	}
	t.Logf("period of calculated totals set to 12 months in %s", time.Since(started).Round(time.Millisecond))
	started = time.Now()
	if err := fixture.registers.RebuildTotals(ctx, "Остатки"); err != nil {
		t.Fatal(err)
	}
	t.Logf("full rebuild in %s", time.Since(started).Round(time.Millisecond))
	var rows int64
	if err := fixture.pool.QueryRow(ctx, "SELECT count(*) FROM "+qualifiedCatalogTable(fixture.totalsTable)).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d rows of totals", rows)

	// A fast rebuild is worth nothing if it is wrong: one product across all
	// warehouses, inside the stored months and after them, against the sum of
	// the movements strictly before the date.
	for _, at := range []time.Time{utcDate(2025, 6, 15, 12), utcDate(2026, 6, 1, 0)} {
		var want int64
		if err := fixture.pool.QueryRow(ctx, "SELECT COALESCE(sum(CASE WHEN movement_kind = 1 THEN "+pgx.Identifier{quantity}.Sanitize()+" ELSE -"+pgx.Identifier{quantity}.Sanitize()+" END), 0)::bigint FROM "+movements+
			" WHERE active AND period < $1 AND "+pgx.Identifier{product}.Sanitize()+" = 'P7'", at).Scan(&want); err != nil {
			t.Fatal(err)
		}
		started = time.Now()
		balances, err := fixture.registers.Balances(ctx, "Остатки", at, map[uuid.UUID]Value{fixture.productID: {Kind: StringType, Data: "P7"}})
		if err != nil {
			t.Fatal(err)
		}
		elapsed := time.Since(started)
		var got int64
		for _, row := range balances {
			got += wholeAmount(t, row.Turnover[fixture.quantityID])
		}
		if got != want {
			t.Fatalf("balance of P7 at %s: totals give %d, movements %d", at.Format(time.RFC3339), got, want)
		}
		t.Logf("balance of one product over %d warehouses at %s read in %s", len(balances), at.Format(time.DateOnly), elapsed.Round(time.Microsecond))
	}

	for _, probe := range []struct {
		name string
		at   time.Time
	}{
		{"backdated to the first stored month", utcDate(2025, 1, 15, 0)},
		{"after the stored months", utcDate(2026, 3, 15, 0)},
	} {
		const postings = 20
		var slowest time.Duration
		started = time.Now()
		for posting := 0; posting < postings; posting++ {
			lines := make([]balanceMovement, 0, 10)
			for line := 0; line < 10; line++ {
				n := posting*10 + line
				lines = append(lines, balanceMovement{probe.at, "W" + strconv.Itoa(1+n%warehouses), "P" + strconv.Itoa(1+n%products), true, 5})
			}
			one := time.Now()
			if err := fixture.post(ctx, probe.name[:4]+strconv.Itoa(posting), lines...); err != nil {
				t.Fatal(err)
			}
			slowest = max(slowest, time.Since(one))
		}
		t.Logf("%s: %d postings of 10 lines, %s each on average, slowest %s", probe.name, postings,
			(time.Since(started) / postings).Round(time.Microsecond), slowest.Round(time.Microsecond))
	}
}
