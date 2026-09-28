package metadata

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
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
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	fixture := newAccumulationTotalsFixture(ctx, t, true, 0)

	// Twenty writes one after another meet nobody, so they share one row.
	for index := 0; index < 20; index++ {
		if err := fixture.write(ctx, "Q-"+string(rune('a'+index)), "A", "1"); err != nil {
			t.Fatal(err)
		}
	}
	if rows := fixture.rowCount(ctx, t); rows != 1 {
		t.Fatalf("twenty writes in a row made %d rows of totals, and met nobody", rows)
	}
	if turnover := fixture.turnover(ctx, t, "A"); turnover != "20" {
		t.Fatalf("turnover after twenty writes = %q", turnover)
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
			errs <- fixture.write(ctx, "P-"+string(rune('a'+index)), "A", "3")
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
	if turnover := fixture.turnover(ctx, t, "A"); turnover != "44" {
		t.Fatalf("turnover after twenty and eight writes = %q, rows = %d", turnover, fixture.rowCount(ctx, t))
	}
	concurrent := fixture.rowCount(ctx, t)
	t.Logf("%d одновременных писателей оставили %d записей итогов", writers, concurrent)
	if concurrent < 1 || concurrent > writers+1 {
		t.Fatalf("rows of totals = %d, writers = %d", concurrent, writers)
	}

	// A rebuild folds whatever the concurrency left behind, and the number
	// does not move.
	if err := fixture.registers.RebuildTotals(ctx, "Продажи"); err != nil {
		t.Fatal(err)
	}
	if rows := fixture.rowCount(ctx, t); rows != 1 {
		t.Fatalf("the rebuild left %d rows", rows)
	}
	if turnover := fixture.turnover(ctx, t, "A"); turnover != "44" {
		t.Fatalf("turnover after the rebuild = %q", turnover)
	}
}

// TestTotalsNumberARowWhenTheExistingOneIsHeld drives the path where a writer
// cannot take a row and has to number one, which is where colliding happens.
//
// A writer that cannot take a row numbers one past the highest it can see, and
// it cannot see the uncommitted rows of the others - so writers that start
// together all aim at the same number, one of them gets it and the rest have to
// choose again. That second choice is the whole mechanism of colliding, and
// while the step it added was a fixed function of the attempt the rest aimed at
// the same number again, and again: one writer got through per round, and a
// writer that lost more rounds than the attempts allow stopped racing and
// waited for a row that nobody was going to release. The step is drawn at
// random now, so the writers that lost disagree with each other.
//
// Two things are arranged for that. The documents are posted after all of them
// exist, because saving a document takes locks of its own and writers that
// queue there never meet at the totals - that is what the first version of this
// test did, and its numbering came out 0, 1, 2, 3, 4, 5, 6, 7: not one
// collision, and the code it was written for never ran. And the one existing
// row is held by somebody else, so the first writers have nothing to take and
// have to number.
//
// What this test does NOT prove: that the steps of two colliding writers
// differ. Writers reach the totals spread out in time - the ones that come
// later find the rows the earlier ones have already committed and released, and
// take them instead of numbering, which is the mechanism working as it should -
// so how many collisions a run gets is not up to the test. The step itself is
// held to disagree by TestAccumulationTotalRowOffsetDisagreesBetweenWriters,
// which needs no database. What this test holds is what must be true whatever
// the collisions were: every write finishes, the sum is exact, rows appear only
// where a writer had nothing to take, and the numbering does not run away.
func TestTotalsNumberARowWhenTheExistingOneIsHeld(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	const writers = 8
	fixture := newAccumulationTotalsFixture(ctx, t, true, writers+8)
	if err := fixture.write(ctx, "S-0", "A", "5"); err != nil {
		t.Fatal(err)
	}
	holder, held := fixture.holdLowestRow(ctx, t)
	if held != 0 {
		t.Fatalf("held row number = %d", held)
	}
	documents := make([]*DocumentRecord, writers)
	for index := range documents {
		document, err := fixture.newDocument(ctx, fmt.Sprintf("S-%d", index+1))
		if err != nil {
			t.Fatal(err)
		}
		documents[index] = document
	}

	// The concurrent part has a deadline of its own, and a short one, because
	// the failure it guards against is a writer that waits for a row nobody
	// will release. Without it a regression hangs until the whole test times
	// out and reads as a slow test rather than as the defect it is.
	posting, stopPosting := context.WithTimeout(ctx, 45*time.Second)
	defer stopPosting()
	var group sync.WaitGroup
	errs := make(chan error, writers)
	start := make(chan struct{})
	for index := range documents {
		group.Add(1)
		go func(document *DocumentRecord) {
			defer group.Done()
			<-start
			errs <- fixture.post(posting, document, "A", "2")
		}(documents[index])
	}
	close(start)
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("a writer that had to number a row of its own failed: %v", err)
		}
	}

	numbers := fixture.splitNumbers(ctx, t, "A")
	t.Logf("%d писателей при занятой единственной записи оставили номера %v", writers, numbers)
	// At least two rows: the held one, and one numbered by a writer that could
	// not have it - a single row would mean somebody wrote into a row another
	// transaction was holding. At most one per writer: more would mean rows
	// appearing without a writer to need them, which is the old mechanism.
	if len(numbers) < 2 || len(numbers) > writers+1 {
		t.Fatalf("rows of totals = %d with %d writers and one row held: %v", len(numbers), writers, numbers)
	}
	if highest := numbers[len(numbers)-1]; highest > int16(writers*accumulationTotalRowSpread) {
		t.Fatalf("highest number = %d after %d rows: the numbering climbs faster than rows appear", highest, len(numbers))
	}
	expected := fmt.Sprint(5 + writers*2)
	if turnover := fixture.turnover(ctx, t, "A"); turnover != expected {
		t.Fatalf("turnover = %q, expected %q", turnover, expected)
	}

	// With the row released there is something to take again, so the next write
	// takes it and numbers nothing.
	if err := holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := fixture.write(ctx, "S-last", "A", "1"); err != nil {
		t.Fatal(err)
	}
	if rows := fixture.rowCount(ctx, t); rows != len(numbers) {
		t.Fatalf("a write that had a free row to take made a new one: rows = %d, were %d", rows, len(numbers))
	}
	if turnover := fixture.turnover(ctx, t, "A"); turnover != fmt.Sprint(6+writers*2) {
		t.Fatalf("turnover after the released row was written = %q", turnover)
	}

	// And a rebuild folds the lot back into one row without moving the number.
	if err := fixture.registers.RebuildTotals(ctx, "Продажи"); err != nil {
		t.Fatal(err)
	}
	if rows := fixture.rowCount(ctx, t); rows != 1 {
		t.Fatalf("the rebuild left %d rows", rows)
	}
	if turnover := fixture.turnover(ctx, t, "A"); turnover != fmt.Sprint(6+writers*2) {
		t.Fatalf("turnover after the rebuild = %q", turnover)
	}
}
