package metadata

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The setting permits, it does not switch. Silence from the base means the
// mechanism is engaged - the help says of the property that it «будет
// задействован», and the dialog offers to turn off an already set mode - while
// a register whose configuration does not permit splitting has no mode at all.
func TestTheBaseDecidesWhetherTotalsAreSplit(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		allowed   bool
		stored    *bool
		splitting bool
	}{
		"разрешено, база молчит — включено":                 {true, nil, true},
		"разрешено, база выключила":                         {true, boolPointer(false), false},
		"разрешено, база включила":                          {true, boolPointer(true), true},
		"не разрешено — выключено, что бы ни лежало в базе": {false, boolPointer(true), false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			query := storedTotalsMode{value: want.stored}
			splitting, err := effectiveTotalsSplitting(context.Background(), query, uuid.MustNew(), want.allowed)
			if err != nil || splitting != want.splitting {
				t.Fatalf("%s: splitting=%v error=%v", name, splitting, err)
			}
		})
	}
}

// A register that does not allow splitting cannot be switched, the way the
// prototype does not offer it in the list.
func TestSwitchingIsRefusedWhereTheConfigurationDoesNotAllowIt(t *testing.T) {
	t.Parallel()
	err := SetTotalsSplitting(context.Background(), refusingExecutor{}, uuid.MustNew(), false, true)
	if err == nil || !strings.Contains(err.Error(), "does not allow splitting its totals") {
		t.Fatalf("error = %v", err)
	}
}

// With the mode off every movement goes to one row of totals per combination
// of dimensions, which is what a register that does not split them means. With
// it on the recorder decides the row.
func TestTheModeDecidesWhichRowOfTotalsAWriteTouches(t *testing.T) {
	t.Parallel()
	recorder := DocumentReference{DocumentID: uuid.MustNew(), ObjectID: uuid.MustNew()}
	if split := accumulationTotalsSplit(recorder, false); split != 0 {
		t.Fatalf("a register that does not split its totals used row %d", split)
	}
	split := accumulationTotalsSplit(recorder, true)
	if split < 0 || split >= accumulationTotalsSplitCount {
		t.Fatalf("the row is outside the table's check constraint: %d", split)
	}
	// The same recorder always lands in the same row: the movements of one
	// document must not spread themselves over the totals.
	if again := accumulationTotalsSplit(recorder, true); again != split {
		t.Fatalf("one recorder used two rows: %d and %d", split, again)
	}
}

func boolPointer(value bool) *bool { return &value }

// storedTotalsMode stands in for the base's answer without needing a base.
type storedTotalsMode struct{ value *bool }

func (mode storedTotalsMode) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return storedTotalsRow{value: mode.value}
}

type storedTotalsRow struct{ value *bool }

func (row storedTotalsRow) Scan(destination ...any) error {
	if row.value == nil {
		return pgx.ErrNoRows
	}
	if len(destination) != 1 {
		return fmt.Errorf("totals mode row takes one destination")
	}
	target, ok := destination[0].(*bool)
	if !ok {
		return fmt.Errorf("totals mode row is a boolean")
	}
	*target = *row.value
	return nil
}

// refusingExecutor fails if it is ever asked to write: the refusal under test
// must happen before any statement reaches the base.
type refusingExecutor struct{}

func (refusingExecutor) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, fmt.Errorf("a refused switch must not reach the database")
}
