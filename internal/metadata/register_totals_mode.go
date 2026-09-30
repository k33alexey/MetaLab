package metadata

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Режим разделения итогов — the running database's answer, not the
// configuration's.
//
// The configuration property is called РазрешитьРазделениеИтогов, and the word
// is the whole point: it permits the mechanism, it does not switch it on. The
// prototype switches it in the working database - «Управление итогами» has a
// tab of its own that lists the registers whose configuration allows the
// mode, and there the administrator turns
// the mode on or off - and the manager carries the pair of methods
// УстановитьРежимРазделенияИтогов and ПолучитьРежимРазделенияИтогов.
//
// That is the third mechanism in a row built this way: the schedule of a
// scheduled job, the mode of aggregates, and now this. The configuration
// declares what is possible; the switch is data.
//
// **Therefore the schema must not depend on the mode.** A setting an
// administrator flips on a live base cannot be allowed to rewrite the totals
// table and its primary key. The splitting column stays in the schema whatever
// the mode says, and the mode only decides where a write puts its row.
//
// Nothing is lost by that, and the prototype says why: the mode changes how
// much can run in parallel and nothing of what the application computes.
// Reading totals sums the rows
// across splits, so switching the mode changes no number - the rows written
// before the switch are read beside the ones written after, and a rebuild of
// the totals collapses them when someone asks for one.
//
// **The default when the configuration allows it is on.** The help says of the
// property that the mechanism «будет задействован», and the dialog says the
// administrator may turn off «установленный режим» - it is on until turned
// off. A register whose configuration does not allow splitting has no mode at
// all: the switch is refused there, exactly as the prototype refuses to list
// it.

// EnsureRegisterTotalsStorage creates the store of the totals modes.
func EnsureRegisterTotalsStorage(ctx context.Context, executor sqlExecutor) error {
	if executor == nil {
		return fmt.Errorf("register totals storage executor is required")
	}
	// Every mode of a register lives in one row, and a row may exist for any
	// one of them alone: an administrator who only sets the period of the
	// totals has said nothing about splitting. So every column but the key
	// means "not said" when empty, and the defaults are applied on reading,
	// where the configuration is known - see balanceTotalsState.
	//
	// The ALTERs are for a base made before the columns existed. They are
	// idempotent, and they run in the transaction of "Сохранить данные", so a
	// base either has all of them or none.
	_, err := executor.Exec(ctx, `
CREATE SCHEMA IF NOT EXISTS ml_core;
CREATE TABLE IF NOT EXISTS ml_core.register_totals_modes (
    metadata_id uuid PRIMARY KEY,
    splitting boolean
);
ALTER TABLE ml_core.register_totals_modes ALTER COLUMN splitting DROP NOT NULL;
ALTER TABLE ml_core.register_totals_modes ADD COLUMN IF NOT EXISTS totals_format smallint;
ALTER TABLE ml_core.register_totals_modes ADD COLUMN IF NOT EXISTS min_period timestamp with time zone;
ALTER TABLE ml_core.register_totals_modes ADD COLUMN IF NOT EXISTS max_period timestamp with time zone;
ALTER TABLE ml_core.register_totals_modes ADD COLUMN IF NOT EXISTS present_totals boolean;
ALTER TABLE ml_core.register_totals_modes ADD COLUMN IF NOT EXISTS use_totals boolean`)
	if err != nil {
		return fmt.Errorf("ensure register totals storage: %w", err)
	}
	return nil
}

// totalsSplittingMode is what the base says for this register, if it has said
// anything. The caller decides what silence means, because that depends on
// whether the configuration allows splitting at all.
func totalsSplittingMode(ctx context.Context, query sequenceQuery, metadataID uuid.UUID) (bool, bool, error) {
	if query == nil || metadataID.IsZero() {
		return false, false, fmt.Errorf("invalid totals mode request")
	}
	var splitting *bool
	err := query.QueryRow(ctx,
		"SELECT splitting FROM ml_core.register_totals_modes WHERE metadata_id = $1", metadataID).Scan(&splitting)
	switch {
	case err == pgx.ErrNoRows:
		return false, false, nil
	case err != nil:
		return false, false, fmt.Errorf("read totals mode: %w", err)
	case splitting == nil:
		// A row kept for another mode says nothing about this one.
		return false, false, nil
	}
	return *splitting, true, nil
}

// effectiveTotalsSplitting is the answer a write needs: is this register
// splitting its totals right now.
func effectiveTotalsSplitting(ctx context.Context, query sequenceQuery, metadataID uuid.UUID, allowed bool) (bool, error) {
	if !allowed {
		// Not allowed by the configuration means off, and no row can say
		// otherwise: SetTotalsSplitting refuses to write one.
		return false, nil
	}
	splitting, stored, err := totalsSplittingMode(ctx, query, metadataID)
	if err != nil {
		return false, err
	}
	if !stored {
		return true, nil
	}
	return splitting, nil
}

// SetTotalsSplitting is УстановитьРежимРазделенияИтогов: the administrator's
// switch, stored in the base.
//
// It refuses a register whose configuration does not allow splitting, the way
// the prototype refuses to offer one. Turning the mode off writes nothing to
// the existing totals: the rows already spread across splits are summed on
// reading like any others, and a rebuild folds them together when one is
// asked for. That is what makes this switch cheap enough to be a switch.
func SetTotalsSplitting(ctx context.Context, executor sqlExecutor, metadataID uuid.UUID, allowed, splitting bool) error {
	if executor == nil || metadataID.IsZero() {
		return fmt.Errorf("invalid totals mode request")
	}
	if !allowed {
		return fmt.Errorf("this register does not allow splitting its totals")
	}
	_, err := executor.Exec(ctx, `
INSERT INTO ml_core.register_totals_modes(metadata_id, splitting) VALUES ($1, $2)
ON CONFLICT (metadata_id) DO UPDATE SET splitting = EXCLUDED.splitting`, metadataID, splitting)
	if err != nil {
		return fmt.Errorf("set totals mode: %w", err)
	}
	return nil
}
