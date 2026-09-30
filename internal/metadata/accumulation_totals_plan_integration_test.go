package metadata

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// explainPlan is the plan PostgreSQL would use for the statement, as text. The
// statement is not executed: EXPLAIN without ANALYZE plans and stops, which is
// what lets an INSERT and an UPDATE be asked about without writing anything.
func explainPlan(ctx context.Context, t *testing.T, pool *pgxpool.Pool, statement string, arguments ...any) string {
	t.Helper()
	rows, err := pool.Query(ctx, "EXPLAIN "+statement, arguments...)
	if err != nil {
		t.Fatalf("explain: %v\nstatement: %s", err, statement)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return strings.Join(lines, "\n")
}

// requireCombinationLookup fails unless the plan finds the combination through
// an index. Both halves matter and for different reasons: the condition has to
// carry dimension_key, because a condition on the period alone reads every
// combination of the month to find one; and nothing may fall back to a
// sequential scan, because that reads the month whether the condition names
// the combination or not.
func requireCombinationLookup(t *testing.T, what, plan string) {
	t.Helper()
	found := false
	for _, line := range strings.Split(plan, "\n") {
		if !strings.Contains(line, "Index Cond") {
			continue
		}
		if strings.Contains(line, "dimension_key") && strings.Contains(line, "total_period") {
			found = true
		}
	}
	if !found {
		t.Fatalf("%s does not look up the combination by index - a write would read the whole month to find one row:\n%s", what, plan)
	}
	if strings.Contains(plan, "Seq Scan") {
		t.Fatalf("%s reads the rows of totals sequentially:\n%s", what, plan)
	}
}

// TestTotalsStatementsFindTheCombinationByIndex is the test for the defect that
// the previous iteration left behind and no test could see.
//
// The rows of totals were once addressed by hashing the recorder, so every
// statement knew the number in advance and the primary key was ordered
// (period, number, combination) to match. Once rows stopped being addressed and
// started being taken, every statement began asking the opposite question -
// «дай любую строку этой комбинации» - and with the number still in the middle
// of the key the index could only narrow by period. One write then read every
// row of the month to find one combination, and no assertion about totals would
// ever notice: the numbers it produced were right.
//
// This is why the plan is asserted and not the result. The table below is large
// enough and analysed, because a planner with no statistics or a handful of
// rows will read them sequentially and be right to.
func TestTotalsStatementsFindTheCombinationByIndex(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	fixture := newAccumulationTotalsFixture(ctx, t, true, 0)
	productColumn, _ := PhysicalAttributeColumn(fixture.productID)
	amountColumn, _ := PhysicalAttributeColumn(fixture.amountID)
	month := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	const combinations = 20000
	if _, err := fixture.pool.Exec(ctx, "INSERT INTO "+fixture.qualifiedTotals()+
		" (total_period, totals_split, dimension_key, "+pgx.Identifier{productColumn}.Sanitize()+", "+pgx.Identifier{amountColumn}.Sanitize()+")"+
		" SELECT $1, 0, lpad(series::text, 64, '0'), 'P' || series, 1 FROM generate_series(1, $2) AS series",
		month, combinations); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, "ANALYZE "+fixture.qualifiedTotals()); err != nil {
		t.Fatal(err)
	}
	delta := &accumulationTotalDelta{
		period:       month,
		dimensionKey: fmt.Sprintf("%064d", combinations/2),
		dimensions:   map[uuid.UUID]Value{fixture.productID: {Kind: StringType, Data: fmt.Sprintf("P%d", combinations/2)}},
		resources:    map[uuid.UUID]*big.Rat{fixture.amountID: big.NewRat(1, 1)},
	}

	arguments, err := fixture.registers.totalRowInsertArguments(fixture.definition, delta)
	if err != nil {
		t.Fatal(err)
	}
	arguments = append(arguments, accumulationTotalRowStep())
	// One statement takes or makes the row of a combination, racing with
	// splitting on and waiting with it off. Each half has to find the
	// combination by index: the taking and the highest number the making
	// counts from.
	for _, race := range []bool{true, false} {
		statement := fixture.registers.totalRowTakeOrMake(fixture.definition, race)
		requireCombinationLookup(t, fmt.Sprintf("taking or making a row of totals (race=%v)", race), explainPlan(ctx, t, fixture.pool, statement, arguments...))
	}
}

// TestTotalsWriteFailsRatherThanLosingAChange is the failure path of a register
// that does not split its totals: there is one row per combination and a writer
// waits for it.
//
// The thing that must not happen is the quiet one. «Взять любую свободную
// строку» is one SKIP LOCKED away from «взять ни одну и пойти дальше», and a
// write that goes on after taking no row reports success while its amount is
// nowhere: the movements are in the base, the totals are short by exactly that
// document, and the discrepancy surfaces months later in a balance nobody can
// explain. So with the only row held and no time to wait, the write must come
// back as an error and leave the totals exactly as they were.
func TestTotalsWriteFailsRatherThanLosingAChange(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	fixture := newAccumulationTotalsFixture(ctx, t, false, 8)
	if err := fixture.write(ctx, "N-1", "A", "5"); err != nil {
		t.Fatal(err)
	}
	holder, err := fixture.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	var held int16
	if err := holder.QueryRow(ctx, "SELECT totals_split FROM "+fixture.qualifiedTotals()+
		" ORDER BY totals_split LIMIT 1 FOR UPDATE").Scan(&held); err != nil {
		t.Fatal(err)
	}

	short, stop := context.WithTimeout(ctx, 3*time.Second)
	defer stop()
	if err := fixture.write(short, "N-2", "A", "7"); err == nil {
		t.Fatal("the write succeeded while the only row of totals was held by somebody else: its amount went nowhere")
	} else {
		t.Logf("write with the only row held: %v", err)
	}
	if turnover := fixture.turnover(ctx, t, "A"); turnover != "5" {
		t.Fatalf("turnover after the refused write = %q, and nothing of it should have been applied", turnover)
	}
	if rows := fixture.rowCount(ctx, t); rows != 1 {
		t.Fatalf("a register that does not split its totals kept %d rows", rows)
	}

	if err := holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := fixture.write(ctx, "N-3", "A", "7"); err != nil {
		t.Fatalf("the write after the row was released failed: %v", err)
	}
	if turnover := fixture.turnover(ctx, t, "A"); turnover != "12" {
		t.Fatalf("turnover after the released row was written = %q", turnover)
	}
}

// TestTotalsNumberingAtTheCeilingStillCompletesTheWrite is the boundary of the
// type that holds the number.
//
// A new row is numbered past the highest there is, so on a combination that
// keeps multiplying the numbering only ever climbs. At the ceiling of the
// smallint column the arithmetic would overflow, and an overflow here is a
// write of movements failing because of a number that means nothing to anyone -
// «smallint out of range» on a perfectly ordinary posting. It must instead
// collide with the highest row on purpose and fall back to waiting for it,
// which is what the mechanism does when it cannot have a row of its own.
func TestTotalsNumberingAtTheCeilingStillCompletesTheWrite(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	fixture := newAccumulationTotalsFixture(ctx, t, true, 8)
	if err := fixture.write(ctx, "C-1", "A", "5"); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, "UPDATE "+fixture.qualifiedTotals()+" SET totals_split = $1",
		int16(maxAccumulationTotalRowNumber)); err != nil {
		t.Fatal(err)
	}
	holder, err := fixture.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	var held int16
	if err := holder.QueryRow(ctx, "SELECT totals_split FROM "+fixture.qualifiedTotals()+
		" ORDER BY totals_split LIMIT 1 FOR UPDATE").Scan(&held); err != nil {
		t.Fatal(err)
	}
	if held != maxAccumulationTotalRowNumber {
		t.Fatalf("held row number = %d", held)
	}

	done := make(chan error, 1)
	go func() { done <- fixture.write(ctx, "C-2", "A", "7") }()
	time.Sleep(time.Second)
	select {
	case err := <-done:
		t.Fatalf("the write finished while the only row was held instead of waiting for it: %v", err)
	default:
	}
	if err := holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("the write at the ceiling of the number failed: %v", err)
	}
	if turnover := fixture.turnover(ctx, t, "A"); turnover != "12" {
		t.Fatalf("turnover at the ceiling of the number = %q", turnover)
	}
	numbers := fixture.splitNumbers(ctx, t, "A")
	if len(numbers) != 1 || numbers[0] != maxAccumulationTotalRowNumber {
		t.Fatalf("rows of totals at the ceiling = %v", numbers)
	}
}
