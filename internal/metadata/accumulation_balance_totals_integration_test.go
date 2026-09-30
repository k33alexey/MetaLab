package metadata

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

func utcDate(year int, month time.Month, day, hour int) time.Time {
	return time.Date(year, month, day, hour, 0, 0, 0, time.UTC)
}

func monthPointer(year int, month time.Month) *time.Time {
	value := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	return &value
}

// balanceProbeDates are the dates every state of the totals is asked at: before
// all movements, inside each month, exactly at a month start - where a
// movement also stands, so "before the month" and "from the month" are told
// apart - after the last movement and far in the future.
var balanceProbeDates = []time.Time{
	utcDate(2025, 9, 1, 0),
	utcDate(2025, 10, 20, 12),
	utcDate(2025, 11, 1, 0),
	utcDate(2025, 11, 15, 0),
	utcDate(2025, 12, 31, 23),
	utcDate(2026, 1, 1, 0),
	utcDate(2026, 1, 10, 0),
	utcDate(2026, 2, 1, 0),
	utcDate(2026, 2, 14, 8),
	utcDate(2026, 3, 20, 0),
	utcDate(2026, 6, 1, 0),
	utcDate(3999, 12, 31, 0),
}

func (fixture *balanceTotalsFixture) requireEveryProbe(ctx context.Context, t *testing.T, label string) {
	t.Helper()
	for _, at := range balanceProbeDates {
		fixture.requireBalances(ctx, t, label, at)
	}
}

// TestBalanceTotalsAnswerLikeTheMovements is the test of the model itself.
//
// Totals are only an acceleration: every balance must be the one the movements
// alone give, whatever the period of calculated totals, whether the present
// totals are kept, and after every way of changing them. The defect it catches
// is the one the model can have and no single scenario would show: a month
// counted twice or not at all at a boundary, a backdated movement that misses
// the stored months after it, a period change that leaves stale months behind
// or does not calculate the new ones.
//
// Each step checks the answers at every probe date and the rows themselves.
func TestBalanceTotalsAnswerLikeTheMovements(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	fixture := newBalanceTotalsFixture(ctx, t, true, 0)
	mustPost := func(number string, movements ...balanceMovement) {
		t.Helper()
		if err := fixture.post(ctx, number, movements...); err != nil {
			t.Fatalf("post %s: %v", number, err)
		}
	}
	mustPost("R-1",
		balanceMovement{utcDate(2025, 10, 5, 9), "W1", "P1", true, 100},
		balanceMovement{utcDate(2025, 10, 5, 9), "W1", "P2", true, 40},
		balanceMovement{utcDate(2025, 10, 5, 9), "W2", "P2", true, 25})
	mustPost("R-2",
		balanceMovement{utcDate(2025, 11, 1, 0), "W1", "P1", false, 30},
		balanceMovement{utcDate(2025, 11, 20, 0), "W2", "P2", false, 25})
	mustPost("R-3",
		balanceMovement{utcDate(2026, 1, 1, 0), "W1", "P1", true, 7},
		balanceMovement{utcDate(2026, 1, 1, 0), "W3", "P2", true, 11})
	mustPost("R-4",
		balanceMovement{utcDate(2026, 2, 14, 8), "W1", "P2", false, 40},
		balanceMovement{utcDate(2026, 3, 3, 0), "W3", "P1", true, 5})

	// No period set: only the present totals exist.
	fixture.requireEveryProbe(ctx, t, "present totals only")
	fixture.requireStoredShape(ctx, t, "present totals only", nil, nil, true)

	// Totals calculated through January: balances stored on 01.02 and back to
	// the first month of movements.
	if err := fixture.registers.SetTotalsPeriods(ctx, "Остатки", TotalsBound{}, TotalsBound{Set: true, Date: utcDate(2026, 1, 31, 0)}); err != nil {
		t.Fatal(err)
	}
	settings, err := fixture.registers.TotalsSettings(ctx, "Остатки")
	if err != nil || !settings.MinPeriod.IsZero() || !settings.MaxPeriod.Equal(utcDate(2026, 1, 31, 23).Add(59*time.Minute+59*time.Second)) || !settings.PresentTotals || !settings.UseTotals {
		t.Fatalf("settings after setting the upper bound = %+v, %v", settings, err)
	}
	fixture.requireEveryProbe(ctx, t, "through January")
	fixture.requireStoredShape(ctx, t, "through January", monthPointer(2025, 10), monthPointer(2026, 2), true)

	// Backdated and current postings change the stored months after them.
	mustPost("R-5", balanceMovement{utcDate(2025, 11, 10, 0), "W2", "P1", true, 9})
	mustPost("R-6", balanceMovement{utcDate(2026, 3, 5, 0), "W1", "P1", false, 2})
	fixture.requireEveryProbe(ctx, t, "after backdated posting")
	fixture.requireStoredShape(ctx, t, "after backdated posting", monthPointer(2025, 10), monthPointer(2026, 2), true)

	// A lower bound: before it the balance is the movements'.
	if err := fixture.registers.SetTotalsPeriods(ctx, "Остатки", TotalsBound{Set: true, Date: utcDate(2025, 12, 10, 0)}, TotalsBound{}); err != nil {
		t.Fatal(err)
	}
	fixture.requireEveryProbe(ctx, t, "with lower bound")
	fixture.requireStoredShape(ctx, t, "with lower bound", monthPointer(2025, 12), monthPointer(2026, 2), true)

	// The monthly job moves the upper bound forward, and it can move back.
	if err := fixture.registers.SetTotalsPeriods(ctx, "Остатки", TotalsBound{}, TotalsBound{Set: true, Date: utcDate(2026, 2, 20, 0)}); err != nil {
		t.Fatal(err)
	}
	fixture.requireEveryProbe(ctx, t, "upper bound forward")
	fixture.requireStoredShape(ctx, t, "upper bound forward", monthPointer(2025, 12), monthPointer(2026, 3), true)
	if err := fixture.registers.SetTotalsPeriods(ctx, "Остатки", TotalsBound{Set: true, Date: utcDate(2025, 11, 3, 0)}, TotalsBound{Set: true, Date: utcDate(2025, 12, 31, 0)}); err != nil {
		t.Fatal(err)
	}
	fixture.requireEveryProbe(ctx, t, "both bounds moved")
	fixture.requireStoredShape(ctx, t, "both bounds moved", monthPointer(2025, 11), monthPointer(2026, 1), true)

	// Without present totals the latest stored month carries the balance.
	if err := fixture.registers.SetPresentTotalsUsing(ctx, "Остатки", false); err != nil {
		t.Fatal(err)
	}
	mustPost("R-7", balanceMovement{utcDate(2026, 3, 25, 0), "W2", "P2", true, 13})
	mustPost("R-2", balanceMovement{utcDate(2025, 11, 1, 0), "W1", "P1", false, 31}) // reposted: the old movements leave
	fixture.requireEveryProbe(ctx, t, "no present totals")
	fixture.requireStoredShape(ctx, t, "no present totals", monthPointer(2025, 11), monthPointer(2026, 1), false)
	if err := fixture.registers.SetPresentTotalsUsing(ctx, "Остатки", true); err != nil {
		t.Fatal(err)
	}
	fixture.requireEveryProbe(ctx, t, "present totals back")
	fixture.requireStoredShape(ctx, t, "present totals back", monthPointer(2025, 11), monthPointer(2026, 1), true)

	// Totals switched off: a write keeps none and a read is refused; switched
	// on, the totals come back including what was written in between.
	if err := fixture.registers.SetTotalsUsing(ctx, "Остатки", false); err != nil {
		t.Fatal(err)
	}
	mustPost("R-8", balanceMovement{utcDate(2025, 12, 5, 0), "W3", "P1", true, 17})
	if _, err := fixture.registers.Balances(ctx, "Остатки", utcDate(2026, 1, 1, 0), nil); !errors.Is(err, ErrAccumulationTotalsDisabled) {
		t.Fatalf("a read with the totals switched off answered: %v", err)
	}
	if err := fixture.registers.SetTotalsUsing(ctx, "Остатки", true); err != nil {
		t.Fatal(err)
	}
	fixture.requireEveryProbe(ctx, t, "totals switched back on")
	fixture.requireStoredShape(ctx, t, "totals switched back on", monthPointer(2025, 11), monthPointer(2026, 1), true)

	// Every kind of recalculation leaves the same totals.
	if err := fixture.registers.RecalcTotalsForPeriod(ctx, "Остатки", utcDate(2025, 12, 1, 0), time.Time{}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.registers.RecalcPresentTotals(ctx, "Остатки"); err != nil {
		t.Fatal(err)
	}
	fixture.requireStoredShape(ctx, t, "partial recalculation", monthPointer(2025, 11), monthPointer(2026, 1), true)
	if err := fixture.registers.RebuildTotals(ctx, "Остатки"); err != nil {
		t.Fatal(err)
	}
	fixture.requireEveryProbe(ctx, t, "full recalculation")
	fixture.requireStoredShape(ctx, t, "full recalculation", monthPointer(2025, 11), monthPointer(2026, 1), true)

	// Removing both bounds leaves the present totals alone.
	if err := fixture.registers.SetTotalsPeriods(ctx, "Остатки", TotalsBound{Set: true}, TotalsBound{Set: true}); err != nil {
		t.Fatal(err)
	}
	fixture.requireEveryProbe(ctx, t, "bounds removed")
	fixture.requireStoredShape(ctx, t, "bounds removed", nil, nil, true)
}

// A lower bound after the upper one is no period at all.
func TestBalanceTotalsRefuseAnInvertedPeriod(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	fixture := newBalanceTotalsFixture(ctx, t, false, 0)
	err := fixture.registers.SetTotalsPeriods(ctx, "Остатки", TotalsBound{Set: true, Date: utcDate(2026, 3, 1, 0)}, TotalsBound{Set: true, Date: utcDate(2026, 1, 31, 0)})
	if err == nil || !strings.Contains(err.Error(), "after the upper one") {
		t.Fatalf("inverted period: %v", err)
	}
	settings, err := fixture.registers.TotalsSettings(ctx, "Остатки")
	if err != nil || !settings.MinPeriod.IsZero() || !settings.MaxPeriod.IsZero() {
		t.Fatalf("a refused period was stored: %+v %v", settings, err)
	}
}

// TestOldTotalsAreRefusedUntilSavingRebuildsThem is the upgrade of a base the
// old code wrote. Its balance register keeps monthly turnovers in the same
// table and columns the new code reads balances from, so nothing in the schema
// tells them apart; read as balances they give wrong numbers and no error.
//
// Three things must hold: such a register refuses both reading and writing
// with an error that names it; the upgrade that "Сохранить данные" runs
// rebuilds it into balances; and a register with an empty table is in no
// format and just gets the mark, because there is nothing to misread.
func TestOldTotalsAreRefusedUntilSavingRebuildsThem(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	fixture := newBalanceTotalsFixture(ctx, t, false, 0)
	if err := fixture.post(ctx, "O-1",
		balanceMovement{utcDate(2025, 10, 5, 0), "W1", "P1", true, 100},
		balanceMovement{utcDate(2025, 12, 5, 0), "W1", "P1", false, 30}); err != nil {
		t.Fatalf("the first write on an empty table must mark it and succeed: %v", err)
	}
	// What the old code left: one row per month of movements holding the
	// month's turnover, and no mark.
	warehouse, _ := PhysicalAttributeColumn(fixture.warehouseID)
	product, _ := PhysicalAttributeColumn(fixture.productID)
	quantity, _ := PhysicalAttributeColumn(fixture.quantityID)
	totals := qualifiedCatalogTable(fixture.totalsTable)
	key := accumulationDimensionKey(fixture.definition, map[uuid.UUID]Value{
		fixture.warehouseID: {Kind: StringType, Data: "W1"}, fixture.productID: {Kind: StringType, Data: "P1"}})
	for _, statement := range []string{
		"DELETE FROM " + totals,
		"INSERT INTO " + totals + " (total_period, totals_split, dimension_key, " + pgx.Identifier{warehouse}.Sanitize() + ", " + pgx.Identifier{product}.Sanitize() + ", " + pgx.Identifier{quantity}.Sanitize() +
			") VALUES ('2025-10-01T00:00:00Z', 0, '" + key + "', 'W1', 'P1', 100), ('2025-12-01T00:00:00Z', 0, '" + key + "', 'W1', 'P1', -30)",
		"UPDATE ml_core.register_totals_modes SET totals_format = NULL WHERE metadata_id = '" + fixture.definition.ID.String() + "'",
	} {
		if _, err := fixture.pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := fixture.registers.Balances(ctx, "Остатки", utcDate(2026, 1, 1, 0), nil); !errors.Is(err, ErrAccumulationTotalsOutdated) || !strings.Contains(err.Error(), "Остатки") {
		t.Fatalf("a read of old totals answered: %v", err)
	}
	if err := fixture.post(ctx, "O-2", balanceMovement{utcDate(2026, 1, 5, 0), "W1", "P1", true, 1}); !errors.Is(err, ErrAccumulationTotalsOutdated) {
		t.Fatalf("a write onto old totals went through: %v", err)
	}

	transaction, err := fixture.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()
	if err := UpgradeAccumulationTotals(ctx, transaction, fixture.registers.catalog); err != nil {
		t.Fatal(err)
	}
	if err := transaction.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	fixture.requireEveryProbe(ctx, t, "after upgrade")
	fixture.requireStoredShape(ctx, t, "after upgrade", nil, nil, true)
	if err := fixture.post(ctx, "O-2", balanceMovement{utcDate(2026, 1, 5, 0), "W1", "P1", true, 1}); err != nil {
		t.Fatalf("a write after the upgrade: %v", err)
	}
	fixture.requireEveryProbe(ctx, t, "write after upgrade")
}

// TestBalanceReadFindsItsPointByIndex is the plan of a balance read, asserted
// rather than its result - the defect it is for produced right numbers. With
// thirty-six months of history the totals must be reached through the index by
// period and dimensions, for a date inside the stored months, for one after
// them read from the present totals, and with a filter on the second dimension
// only, which the index does not lead with.
func TestBalanceReadFindsItsPointByIndex(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	fixture := newBalanceTotalsFixture(ctx, t, false, 0)
	warehouse, _ := PhysicalAttributeColumn(fixture.warehouseID)
	product, _ := PhysicalAttributeColumn(fixture.productID)
	quantity, _ := PhysicalAttributeColumn(fixture.quantityID)
	totals := qualifiedCatalogTable(fixture.totalsTable)
	columns := " (total_period, totals_split, dimension_key, " + pgx.Identifier{warehouse}.Sanitize() + ", " + pgx.Identifier{product}.Sanitize() + ", " + pgx.Identifier{quantity}.Sanitize() + ")"
	rows := " SELECT $1, 0, lpad(md5(ws::text || '/' || ps::text), 64, '0'), 'W' || ws, 'P' || ps, 1 FROM generate_series(1, 10) ws, generate_series(1, 2000) ps"
	for month := 0; month < 36; month++ {
		if _, err := fixture.pool.Exec(ctx, "INSERT INTO "+totals+columns+rows, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, month, 0)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := fixture.pool.Exec(ctx, "INSERT INTO "+totals+columns+rows, accumulationPresentTotalsPeriod); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, `
INSERT INTO ml_core.register_totals_modes(metadata_id, totals_format, max_period) VALUES ($1, $2, '2025-12-01T00:00:00Z')
ON CONFLICT (metadata_id) DO UPDATE SET totals_format = EXCLUDED.totals_format, max_period = EXCLUDED.max_period`, fixture.definition.ID, accumulationBalanceTotalsFormat); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, "ANALYZE "+totals); err != nil {
		t.Fatal(err)
	}
	now := utcDate(2026, 9, 30, 0)
	for _, probe := range []struct {
		name string
		at   time.Time
	}{
		{"inside the stored months", utcDate(2025, 6, 15, 0)},
		{"after them, from the present totals", utcDate(2026, 9, 20, 0)},
	} {
		for _, filter := range []map[uuid.UUID]Value{
			{fixture.productID: {Kind: StringType, Data: "P1000"}},
			{fixture.warehouseID: {Kind: StringType, Data: "W5"}, fixture.productID: {Kind: StringType, Data: "P1000"}},
		} {
			statement, arguments, _, err := fixture.registers.balanceStatement(ctx, fixture.definition, probe.at, now, filter)
			if err != nil {
				t.Fatal(err)
			}
			plan := explainPlan(ctx, t, fixture.pool, statement, arguments...)
			for _, line := range strings.Split(plan, "\n") {
				if strings.Contains(line, "Seq Scan on "+fixture.totalsTable) {
					t.Fatalf("%s, %d filters: the totals are read sequentially:\n%s", probe.name, len(filter), plan)
				}
			}
			if !strings.Contains(plan, physicalObjectName("it", fixture.definition.ID)) {
				t.Fatalf("%s, %d filters: the totals index is not used:\n%s", probe.name, len(filter), plan)
			}
		}
	}
}

// TestBalanceReadNeverMixesTwoPeriods is the reason the point is chosen by the
// statement that reads. While the period of calculated totals moves back and
// forth, a read that decided on a month and then read it could find the month
// gone - and a missing row reads as a zero balance. Readers at dates inside
// and outside the period must see the right balance every time.
func TestBalanceReadNeverMixesTwoPeriods(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	fixture := newBalanceTotalsFixture(ctx, t, false, 16)
	for index, month := range []time.Month{time.October, time.November, time.December} {
		if err := fixture.post(ctx, fmt.Sprintf("M-%d", index),
			balanceMovement{utcDate(2025, month, 3, 0), "W1", "P1", true, int64(10 * (index + 1))},
			balanceMovement{utcDate(2025, month, 3, 0), "W2", "P2", true, int64(index + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	probes := []time.Time{utcDate(2025, 11, 20, 0), utcDate(2025, 12, 10, 0), utcDate(2026, 2, 1, 0)}
	want := make([]map[string]int64, len(probes))
	for index, at := range probes {
		want[index] = fixture.expected(at, true)
	}
	stop := make(chan struct{})
	var wait sync.WaitGroup
	failures := make(chan error, 64)
	wait.Add(1)
	go func() {
		defer wait.Done()
		bounds := []time.Time{utcDate(2025, 10, 31, 0), utcDate(2025, 12, 31, 0)}
		for round := 0; ; round++ {
			select {
			case <-stop:
				return
			default:
			}
			if err := fixture.registers.SetTotalsPeriods(ctx, "Остатки", TotalsBound{Set: true, Date: bounds[round%2]}, TotalsBound{Set: true, Date: bounds[round%2]}); err != nil {
				failures <- err
				return
			}
			if err := fixture.registers.SetPresentTotalsUsing(ctx, "Остатки", round%3 != 0); err != nil {
				failures <- err
				return
			}
		}
	}()
	var readers sync.WaitGroup
	for reader := 0; reader < 6; reader++ {
		readers.Add(1)
		go func(reader int) {
			defer readers.Done()
			for attempt := 0; attempt < 60; attempt++ {
				index := (reader + attempt) % len(probes)
				rows, err := fixture.registers.Balances(ctx, "Остатки", probes[index], nil)
				if err != nil {
					failures <- err
					return
				}
				got := map[string]int64{}
				for _, row := range rows {
					amount, _ := new(big.Rat).SetString(row.Turnover[fixture.quantityID].Data)
					got[fixture.combination(row)] = amount.Num().Int64()
				}
				for key := range want[index] {
					if got[key] != want[index][key] {
						failures <- fmt.Errorf("balance at %s of %s = %d while the period moved, want %d", probes[index].Format(time.RFC3339), key, got[key], want[index][key])
						return
					}
				}
			}
		}(reader)
	}
	// The readers are the measure of the test: once they are done, or one of
	// them has failed and returned, the mover stops.
	readers.Wait()
	close(stop)
	wait.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
}

// TestConcurrentBackdatedPostingsNeitherDeadlockNorLoseAChange is the writers'
// side of the model. A backdated movement now changes a row for each stored
// month after it and the present totals, so one posting holds many rows, and
// two postings of the same combinations in opposite order are the classic
// deadlock. Every writer must finish, and the totals must equal the movements.
func TestConcurrentBackdatedPostingsNeitherDeadlockNorLoseAChange(t *testing.T) {
	for _, splitting := range []bool{false, true} {
		t.Run(fmt.Sprintf("splitting=%v", splitting), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			fixture := newBalanceTotalsFixture(ctx, t, splitting, 24)
			if err := fixture.post(ctx, "S-0", balanceMovement{utcDate(2025, 7, 1, 0), "W1", "P1", true, 1000}); err != nil {
				t.Fatal(err)
			}
			if err := fixture.registers.SetTotalsPeriods(ctx, "Остатки", TotalsBound{}, TotalsBound{Set: true, Date: utcDate(2026, 6, 30, 0)}); err != nil {
				t.Fatal(err)
			}
			var wait sync.WaitGroup
			errorsSeen := make(chan error, 256)
			for writer := 0; writer < 12; writer++ {
				wait.Add(1)
				go func(writer int) {
					defer wait.Done()
					random := rand.New(rand.NewPCG(uint64(writer), 7))
					for round := 0; round < 6; round++ {
						first, second := balanceMovement{utcDate(2025, time.Month(8+random.IntN(4)), 1+random.IntN(27), 0), "W1", "P1", random.IntN(2) == 0, int64(1 + random.IntN(9))},
							balanceMovement{utcDate(2025, time.Month(8+random.IntN(4)), 1+random.IntN(27), 0), "W2", "P2", true, int64(1 + random.IntN(9))}
						movements := []balanceMovement{first, second}
						if writer%2 == 1 {
							movements = []balanceMovement{second, first}
						}
						if err := fixture.post(ctx, fmt.Sprintf("S-%d-%d", writer, round), movements...); err != nil {
							errorsSeen <- err
							return
						}
					}
				}(writer)
			}
			wait.Wait()
			close(errorsSeen)
			for err := range errorsSeen {
				t.Fatalf("a concurrent posting failed: %v", err)
			}
			fixture.requireStoredShape(ctx, t, "after concurrent postings", monthPointer(2025, 7), monthPointer(2026, 7), true)
			for _, at := range []time.Time{utcDate(2025, 9, 15, 0), utcDate(2026, 1, 1, 0), utcDate(2026, 9, 1, 0)} {
				fixture.requireBalances(ctx, t, "after concurrent postings", at)
			}
		})
	}
}
