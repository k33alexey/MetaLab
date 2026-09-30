package metadata

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The totals of a balance register hold balances, not turnovers.
//
// A row is the balance of one combination of dimensions at the beginning of a
// month - every movement before that month summed - for the months inside the
// period of calculated totals; and one more row per combination, the present
// totals, holds the balance after every movement there is. A balance at a date
// is then read from the nearest stored point plus the movements between it
// and the date, and the cost of a read stops growing with the length of the
// history. That is the prototype's model, and its help describes it through
// the methods that manage it: an upper bound inside a month means balances are
// kept for the start of the next month, later balances come from the present
// totals, and with those switched off from the latest kept balance plus the
// movements after it.
//
// The register used to keep the turnover of each month instead, and a balance
// was the sum of every month from the start of accounting. Measured on 20 000
// combinations, a read filtered by one dimension went from 0.9 ms on one month
// of history to 17 ms on thirty-six, by sequential scan, and no index could
// help: a condition "period before the date" selects the whole history.
//
// A turnover register keeps its monthly turnovers: for it that is the model.

const (
	// accumulationBalanceTotalsFormat marks the totals of a balance register
	// as balances. A base whose register has no mark and a non-empty totals
	// table was written by the code that kept turnovers there, and reading
	// those rows as balances would give wrong numbers without a single error.
	accumulationBalanceTotalsFormat int16 = 2
)

// accumulationPresentTotalsPeriod is the period the present totals are stored
// under. A movement's period is within years 1..3999, so no month of the
// period of calculated totals can ever be this one, and it sorts after all of
// them - which keeps the order rows are taken in the same for every writer.
var accumulationPresentTotalsPeriod = time.Date(5000, 1, 1, 0, 0, 0, 0, time.UTC)

// ErrAccumulationTotalsOutdated is a register whose totals are still in the
// old format. The totals are rebuilt by "Сохранить данные"; until then the
// register refuses to read balances and to write movements rather than answer
// with a wrong balance.
var ErrAccumulationTotalsOutdated = errors.New("accumulation register totals are stored in the old format and must be rebuilt")

// ErrAccumulationTotalsDisabled is a register whose totals are switched off:
// the virtual tables of balances and turnovers are then not available, which
// is what the prototype does too.
var ErrAccumulationTotalsDisabled = errors.New("accumulation register totals are switched off")

// balanceTotalsState is what the base says about the totals of one register,
// with the defaults applied for everything it has not said.
type balanceTotalsState struct {
	// minPeriod and maxPeriod are month starts, and both are inclusive: the
	// balances are stored at the beginning of every month from minPeriod to
	// maxPeriod. No maxPeriod means no monthly balances at all; no minPeriod
	// means from the first month there are movements in.
	minPeriod *time.Time
	maxPeriod *time.Time
	present   bool
	use       bool
	format    int16
}

func readBalanceTotalsState(ctx context.Context, query sequenceQuery, metadataID uuid.UUID) (balanceTotalsState, error) {
	if query == nil || metadataID.IsZero() {
		return balanceTotalsState{}, fmt.Errorf("invalid totals mode request")
	}
	state := balanceTotalsState{present: true, use: true}
	var format *int16
	var present, use *bool
	err := query.QueryRow(ctx, `
SELECT totals_format, min_period, max_period, present_totals, use_totals
FROM ml_core.register_totals_modes WHERE metadata_id = $1`, metadataID).Scan(&format, &state.minPeriod, &state.maxPeriod, &present, &use)
	switch {
	case err == pgx.ErrNoRows:
		return state, nil
	case err != nil:
		return balanceTotalsState{}, fmt.Errorf("read totals mode: %w", err)
	}
	if format != nil {
		state.format = *format
	}
	if present != nil {
		state.present = *present
	}
	if use != nil {
		state.use = *use
	}
	for _, bound := range []*time.Time{state.minPeriod, state.maxPeriod} {
		if bound != nil {
			*bound = bound.UTC()
		}
	}
	return state, nil
}

// balanceTargets is every period of totals a movement of this period changes:
// the beginning of each stored month after it, and the present totals.
func (state balanceTotalsState) balanceTargets(period time.Time) []time.Time {
	if !state.use {
		return nil
	}
	var targets []time.Time
	if state.maxPeriod != nil {
		first := accumulationMonth(period).AddDate(0, 1, 0)
		if state.minPeriod != nil && first.Before(*state.minPeriod) {
			first = *state.minPeriod
		}
		for month := first; !month.After(*state.maxPeriod); month = month.AddDate(0, 1, 0) {
			targets = append(targets, month)
		}
	}
	if state.present {
		targets = append(targets, accumulationPresentTotalsPeriod)
	}
	return targets
}

func accumulationMonth(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// requireBalanceTotalsFormat refuses a register whose totals are in the old
// format. An empty totals table is in no format at all, and a writer marks it
// as the current one - that is how a register created after this change, or
// one that has never had a movement, gets its mark without a rebuild.
func requireBalanceTotalsFormat(ctx context.Context, query dataQueryer, executor sqlExecutor, definition AccumulationRegisterDefinition, state balanceTotalsState) error {
	if definition.Kind != AccumulationRegisterBalance || state.format == accumulationBalanceTotalsFormat {
		return nil
	}
	table, _ := PhysicalAccumulationRegisterTotalsTable(definition.ID)
	var occupied bool
	if err := query.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM "+qualifiedCatalogTable(table)+")").Scan(&occupied); err != nil {
		return fmt.Errorf("inspect accumulation register %s totals: %w", definition.Name, err)
	}
	if occupied {
		return fmt.Errorf("accumulation register %s: %w", definition.Name, ErrAccumulationTotalsOutdated)
	}
	if executor == nil {
		return nil
	}
	return markBalanceTotalsFormat(ctx, executor, definition.ID)
}

func markBalanceTotalsFormat(ctx context.Context, executor sqlExecutor, metadataID uuid.UUID) error {
	if _, err := executor.Exec(ctx, `
INSERT INTO ml_core.register_totals_modes(metadata_id, totals_format) VALUES ($1, $2)
ON CONFLICT (metadata_id) DO UPDATE SET totals_format = EXCLUDED.totals_format`, metadataID, accumulationBalanceTotalsFormat); err != nil {
		return fmt.Errorf("mark totals format: %w", err)
	}
	return nil
}

// balanceTotalsIndexKeys is the index a balance is read by: the period and
// then the dimensions. A read names one period - a month start, or the present
// totals - so the period leads and every dimension filter narrows inside it.
//
// A dimension goes in only while the entry is sure to fit: a btree refuses an
// entry larger than a third of a page, and it refuses it at the write, which
// would turn an ordinary posting into an error. A string of unbounded length,
// a composite value and binary data can be of any size, so they stay out of
// the index, and a filter on them is checked on the rows the index found.
func (catalog *Catalog) balanceTotalsIndexKeys(definition AccumulationRegisterDefinition) ([]string, error) {
	const budget = 2000
	keys := []string{"total_period"}
	used := 8
	for _, dimension := range definition.Dimensions {
		storage, err := catalog.attributeStorage(dimension.Types)
		if err != nil {
			return nil, err
		}
		size, ok := btreeEntryBytes(storage)
		if !ok || used+size > budget {
			continue
		}
		column, err := PhysicalAttributeColumn(dimension.ID)
		if err != nil {
			return nil, err
		}
		keys, used = append(keys, column), used+size
	}
	return keys, nil
}

// balanceTotalsDimensionIndexes are the indexes a developer asks for by marking
// a dimension "Индексировать": the period and that dimension alone.
//
// The index of the balances leads with the period and then the dimensions in
// their order, and an index serves only a prefix of its fields - a filter on
// the third dimension without the first two walks every entry of the period.
// Marking the dimension is how the developer answers that, the same way as in
// the prototype, whose list of platform indexes has this one for a dimension
// with the property set, from the second on. The first needs none: it stands
// right after the period in the index of the balances already.
//
// A dimension whose value has no bound on its size is left without one, for
// the reason the index of the balances leaves it out - see
// balanceTotalsIndexKeys.
func (catalog *Catalog) balanceTotalsDimensionIndexes(definition AccumulationRegisterDefinition) ([]schemadiff.Index, error) {
	var indexes []schemadiff.Index
	for position, dimension := range definition.Dimensions {
		if position == 0 || !dimension.Indexing.indexes() {
			continue
		}
		storage, err := catalog.attributeStorage(dimension.Types)
		if err != nil {
			return nil, err
		}
		if _, ok := btreeEntryBytes(storage); !ok {
			continue
		}
		column, err := PhysicalAttributeColumn(dimension.ID)
		if err != nil {
			return nil, err
		}
		indexes = append(indexes, schemadiff.Index{Name: physicalObjectName("itd", dimension.ID), Method: "btree", Keys: []string{"total_period", column}})
	}
	return indexes, nil
}

// btreeEntryBytes is the largest a value of this storage can take in an index
// entry, or false when it has no bound.
func btreeEntryBytes(storage attributeStorage) (int, bool) {
	if storage.composite {
		return 0, false
	}
	sqlType := storage.sqlType
	bounded := func(prefix string) (int, bool) {
		if !strings.HasPrefix(sqlType, prefix) || !strings.HasSuffix(sqlType, ")") {
			return 0, false
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(sqlType, prefix), ")")
		if comma := strings.IndexByte(inner, ','); comma >= 0 {
			inner = inner[:comma]
		}
		length, err := strconv.Atoi(inner)
		if err != nil || length <= 0 {
			return 0, false
		}
		return length, true
	}
	switch {
	case sqlType == "uuid":
		return 16, true
	case sqlType == "boolean":
		return 1, true
	case sqlType == "timestamp with time zone":
		return 8, true
	}
	if length, ok := bounded("character varying("); ok {
		return 4*length + 4, true
	}
	if length, ok := bounded("character("); ok {
		return 4*length + 4, true
	}
	if precision, ok := bounded("numeric("); ok {
		return precision/2 + 8, true
	}
	return 0, false
}

// balanceColumns are the quoted dimension columns, and for each resource its
// quoted column and the signed amount of a movement.
type balanceColumns struct {
	dimensions []string
	resources  []string
	signed     []string
}

func newBalanceColumns(definition AccumulationRegisterDefinition) balanceColumns {
	var columns balanceColumns
	for _, dimension := range definition.Dimensions {
		column, _ := PhysicalAttributeColumn(dimension.ID)
		columns.dimensions = append(columns.dimensions, pgx.Identifier{column}.Sanitize())
	}
	for _, resource := range definition.Resources {
		column, _ := PhysicalAttributeColumn(resource.ID)
		quoted := pgx.Identifier{column}.Sanitize()
		amount := "COALESCE(" + quoted + ", 0)"
		columns.resources = append(columns.resources, quoted)
		columns.signed = append(columns.signed, "CASE WHEN movement_kind = 1 THEN "+amount+" ELSE -"+amount+" END")
	}
	return columns
}

// placeholder appends a value and says how a statement refers to it.
func placeholder(arguments *[]any, value any) string {
	*arguments = append(*arguments, value)
	return "$" + strconv.Itoa(len(*arguments))
}

// balanceParts are the statements a balance at a date is summed from. Each
// part yields the dimensions and then, for each resource, what resource
// renders from the amount expression of that part.
//
// **The choice of the point and the reading are one statement.** The period of
// calculated totals is read by the statement itself, so the rows it reads and
// the period it read them for come from one state of the base. Deciding in Go
// and reading afterwards would leave a window in which the period moves: the
// rows of a month that just left the period would be gone, and a missing row
// reads as a zero balance.
//
// inclusive says whether the movements of the date itself are in: a balance
// at a date is, the opening balance of a period that starts at it is not.
func balanceParts(definition AccumulationRegisterDefinition, at time.Time, inclusive bool, now time.Time, arguments *[]any, restriction string, resource func(index int, amount string) []string) (string, []string) {
	movementTable, _ := PhysicalAccumulationRegisterTable(definition.ID)
	totalsTable, _ := PhysicalAccumulationRegisterTotalsTable(definition.ID)
	movements, totals := qualifiedCatalogTable(movementTable), qualifiedCatalogTable(totalsTable)
	columns := newBalanceColumns(definition)
	date := placeholder(arguments, at)
	before, after := " <= ", " > "
	if !inclusive {
		before, after = " < ", " >= "
	}
	selectFrom := func(amounts []string) string {
		items := append([]string{}, columns.dimensions...)
		for index, amount := range amounts {
			items = append(items, resource(index, amount)...)
		}
		return "SELECT " + strings.Join(items, ", ")
	}
	negated := make([]string, len(columns.signed))
	for index, amount := range columns.signed {
		negated[index] = "-(" + amount + ")"
	}
	if restriction != "" {
		// A row restriction cannot narrow a number that has already been
		// added up, so the totals are left out and the balance is summed from
		// the movements the restriction lets through.
		return "", []string{selectFrom(columns.signed) + " FROM " + movements + " WHERE active = true AND period" + before + date + restriction}
	}
	id := placeholder(arguments, definition.ID.String())
	month := placeholder(arguments, accumulationMonth(at))
	moment := placeholder(arguments, now.UTC())
	present := placeholder(arguments, accumulationPresentTotalsPeriod)
	// The point is the month the balance is taken from, or the present totals,
	// or nothing - and then the movements alone. Nearest wins after the upper
	// bound, because either side is exact and only the count of movements to
	// add differs.
	with := "WITH totals_mode AS (" +
		"SELECT COALESCE(stored.use_totals, true) AS use_totals, COALESCE(stored.present_totals, true) AS present, " +
		"stored.min_period AS lower_bound, stored.max_period AS upper_bound " +
		"FROM (VALUES (1)) AS single(value) LEFT JOIN ml_core.register_totals_modes AS stored ON stored.metadata_id = " + id + "::uuid), " +
		"totals_point AS (SELECT kind, CASE WHEN kind = 'month' THEN LEAST(" + month + "::timestamptz, upper_bound) END AS point FROM (" +
		"SELECT upper_bound, CASE " +
		"WHEN NOT use_totals THEN 'movements' " +
		"WHEN upper_bound IS NOT NULL AND " + month + "::timestamptz <= upper_bound AND (lower_bound IS NULL OR " + month + "::timestamptz >= lower_bound) THEN 'month' " +
		"WHEN upper_bound IS NOT NULL AND " + month + "::timestamptz > upper_bound AND present AND (" + date + "::timestamptz - upper_bound) > (" + moment + "::timestamptz - " + date + "::timestamptz) THEN 'present' " +
		"WHEN upper_bound IS NOT NULL AND " + month + "::timestamptz > upper_bound THEN 'month' " +
		"WHEN present AND (lower_bound IS NULL OR " + month + "::timestamptz >= lower_bound) THEN 'present' " +
		"ELSE 'movements' END AS kind FROM totals_mode) AS decided) "
	parts := []string{
		selectFrom(columns.resources) + " FROM " + totals + " WHERE total_period = (SELECT point FROM totals_point)",
		selectFrom(columns.signed) + " FROM " + movements + " WHERE active = true AND period >= (SELECT point FROM totals_point) AND period" + before + date,
		selectFrom(columns.resources) + " FROM " + totals + " WHERE total_period = " + present + " AND (SELECT kind FROM totals_point) = 'present'",
		selectFrom(negated) + " FROM " + movements + " WHERE active = true AND (SELECT kind FROM totals_point) = 'present' AND period" + after + date,
		selectFrom(columns.signed) + " FROM " + movements + " WHERE active = true AND (SELECT kind FROM totals_point) = 'movements' AND period" + before + date,
	}
	return with, parts
}

// rebuildBalanceMonths replaces the stored balances at the beginning of every
// month from first to last, both month starts and both included.
//
// Each month's balance is the running sum of the monthly net movements before
// it. The running sum is taken per combination and carried over the months a
// combination has no movement in: a row of the running sum stands for every
// month start after its month up to and including the next month the
// combination moved in.
func rebuildBalanceMonths(ctx context.Context, executor sqlExecutor, definition AccumulationRegisterDefinition, first, last time.Time) error {
	if last.Before(first) {
		return nil
	}
	movementTable, _ := PhysicalAccumulationRegisterTable(definition.ID)
	totalsTable, _ := PhysicalAccumulationRegisterTotalsTable(definition.ID)
	movements, totals := qualifiedCatalogTable(movementTable), qualifiedCatalogTable(totalsTable)
	if _, err := executor.Exec(ctx, "DELETE FROM "+totals+" WHERE total_period >= $1 AND total_period <= $2", first, last); err != nil {
		return fmt.Errorf("clear accumulation register %s balances: %w", definition.Name, err)
	}
	columns := newBalanceColumns(definition)
	monthExpression := "date_trunc('month', period AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"
	insertColumns := []string{"total_period", "totals_split", "dimension_key"}
	monthly := []string{monthExpression + " AS month", "dimension_key"}
	running := []string{"month", "dimension_key"}
	outer := []string{"points.point", "0", "running.dimension_key"}
	groups := []string{"1", "dimension_key"}
	for _, dimension := range columns.dimensions {
		monthly, running, outer, groups = append(monthly, dimension), append(running, dimension), append(outer, "running."+dimension), append(groups, dimension)
	}
	for _, dimension := range definition.Dimensions {
		column, _ := PhysicalAttributeColumn(dimension.ID)
		insertColumns = append(insertColumns, column)
	}
	nonZero := make([]string, 0, len(columns.resources))
	for index, resource := range definition.Resources {
		column, _ := PhysicalAttributeColumn(resource.ID)
		alias := pgx.Identifier{fmt.Sprintf("r%d", index)}.Sanitize()
		insertColumns = append(insertColumns, column)
		monthly = append(monthly, "SUM("+columns.signed[index]+") AS "+alias)
		running = append(running, "SUM("+alias+") OVER history AS "+alias)
		outer = append(outer, "running."+alias)
		nonZero = append(nonZero, "running."+alias+" <> 0")
	}
	running = append(running, "LEAD(month) OVER history AS next_month")
	// The series is built in UTC wall time and converted back: a month added
	// to a moment moves with the session's time zone, and in a zone with
	// daylight saving that is an hour off the month start.
	statement := "INSERT INTO " + totals + " (" + strings.Join(quoteCatalogColumns(insertColumns), ", ") + ") SELECT " + strings.Join(outer, ", ") +
		" FROM (SELECT " + strings.Join(running, ", ") + " FROM (SELECT " + strings.Join(monthly, ", ") + " FROM " + movements +
		" WHERE active = true AND period < $2 GROUP BY " + strings.Join(groups, ", ") + ") AS monthly" +
		" WINDOW history AS (PARTITION BY dimension_key ORDER BY month)) AS running" +
		" JOIN (SELECT series AT TIME ZONE 'UTC' AS point FROM generate_series($1::timestamptz AT TIME ZONE 'UTC', $2::timestamptz AT TIME ZONE 'UTC', interval '1 month') AS series) AS points" +
		" ON points.point > running.month AND (running.next_month IS NULL OR points.point <= running.next_month)"
	if len(nonZero) != 0 {
		statement += " WHERE " + strings.Join(nonZero, " OR ")
	}
	if _, err := executor.Exec(ctx, statement, first, last); err != nil {
		return fmt.Errorf("rebuild accumulation register %s balances: %w", definition.Name, err)
	}
	return nil
}

// rebuildBalancePresent replaces the present totals: the balance after every
// movement of the register.
func rebuildBalancePresent(ctx context.Context, executor sqlExecutor, definition AccumulationRegisterDefinition, present bool) error {
	movementTable, _ := PhysicalAccumulationRegisterTable(definition.ID)
	totalsTable, _ := PhysicalAccumulationRegisterTotalsTable(definition.ID)
	movements, totals := qualifiedCatalogTable(movementTable), qualifiedCatalogTable(totalsTable)
	if _, err := executor.Exec(ctx, "DELETE FROM "+totals+" WHERE total_period = $1", accumulationPresentTotalsPeriod); err != nil {
		return fmt.Errorf("clear accumulation register %s present totals: %w", definition.Name, err)
	}
	if !present {
		return nil
	}
	columns := newBalanceColumns(definition)
	insertColumns := []string{"total_period", "totals_split", "dimension_key"}
	selects := []string{"$1::timestamptz", "0", "dimension_key"}
	groups := []string{"dimension_key"}
	for index, dimension := range definition.Dimensions {
		column, _ := PhysicalAttributeColumn(dimension.ID)
		insertColumns, selects, groups = append(insertColumns, column), append(selects, columns.dimensions[index]), append(groups, columns.dimensions[index])
	}
	nonZero := make([]string, 0, len(columns.signed))
	for index, resource := range definition.Resources {
		column, _ := PhysicalAttributeColumn(resource.ID)
		insertColumns = append(insertColumns, column)
		selects = append(selects, "SUM("+columns.signed[index]+")")
		nonZero = append(nonZero, "SUM("+columns.signed[index]+") <> 0")
	}
	statement := "INSERT INTO " + totals + " (" + strings.Join(quoteCatalogColumns(insertColumns), ", ") + ") SELECT " + strings.Join(selects, ", ") +
		" FROM " + movements + " WHERE active = true GROUP BY " + strings.Join(groups, ", ")
	if len(nonZero) != 0 {
		statement += " HAVING " + strings.Join(nonZero, " OR ")
	}
	if _, err := executor.Exec(ctx, statement, accumulationPresentTotalsPeriod); err != nil {
		return fmt.Errorf("rebuild accumulation register %s present totals: %w", definition.Name, err)
	}
	return nil
}

// balanceMonthsLowerBound is the first month the stored balances begin at: the
// lower bound when there is one, otherwise the month of the earliest movement.
// No movement and no lower bound means nothing to store.
func balanceMonthsLowerBound(ctx context.Context, query sequenceQuery, definition AccumulationRegisterDefinition, state balanceTotalsState) (*time.Time, error) {
	if state.minPeriod != nil {
		first := *state.minPeriod
		return &first, nil
	}
	movementTable, _ := PhysicalAccumulationRegisterTable(definition.ID)
	var earliest *time.Time
	if err := query.QueryRow(ctx, "SELECT min(period) FROM "+qualifiedCatalogTable(movementTable)+" WHERE active = true").Scan(&earliest); err != nil {
		return nil, fmt.Errorf("inspect accumulation register %s movements: %w", definition.Name, err)
	}
	if earliest == nil {
		return nil, nil
	}
	first := accumulationMonth(*earliest)
	return &first, nil
}

type balanceTotalsTransaction interface {
	sqlExecutor
	sequenceQuery
}

// rebuildBalanceTotals is the full recalculation of a balance register: the
// present totals and every stored month, in the current format.
func rebuildBalanceTotals(ctx context.Context, transaction balanceTotalsTransaction, definition AccumulationRegisterDefinition, state balanceTotalsState) error {
	totalsTable, _ := PhysicalAccumulationRegisterTotalsTable(definition.ID)
	if _, err := transaction.Exec(ctx, "DELETE FROM "+qualifiedCatalogTable(totalsTable)); err != nil {
		return fmt.Errorf("clear accumulation register %s totals: %w", definition.Name, err)
	}
	if err := rebuildBalancePresent(ctx, transaction, definition, state.present); err != nil {
		return err
	}
	if state.maxPeriod != nil {
		first, err := balanceMonthsLowerBound(ctx, transaction, definition, state)
		if err != nil {
			return err
		}
		if first != nil {
			if err := rebuildBalanceMonths(ctx, transaction, definition, *first, *state.maxPeriod); err != nil {
				return err
			}
		}
	}
	return markBalanceTotalsFormat(ctx, transaction, definition.ID)
}

// UpgradeAccumulationTotals rebuilds, in the transaction of "Сохранить
// данные", every balance register whose totals are not yet in the current
// format. It is the only way a base written by the old code gets its balances
// back: until it runs such a register refuses to be read or written.
//
// A register with an empty totals table only gets its mark - there is
// nothing to rebuild, and a table created by this very migration is one.
func UpgradeAccumulationTotals(ctx context.Context, transaction pgx.Tx, catalog *Catalog) error {
	if transaction == nil || catalog == nil {
		return fmt.Errorf("upgrading accumulation totals requires a transaction and a catalog")
	}
	for _, definition := range catalog.AccumulationRegisters {
		if definition.Kind != AccumulationRegisterBalance {
			continue
		}
		state, err := readBalanceTotalsState(ctx, transaction, definition.ID)
		if err != nil {
			return err
		}
		if state.format == accumulationBalanceTotalsFormat {
			continue
		}
		if _, err := transaction.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", objectLockKey("accumulation-register", definition.ID, definition.ID)); err != nil {
			return fmt.Errorf("lock accumulation register %s: %w", definition.Name, err)
		}
		if err := rebuildBalanceTotals(ctx, transaction, definition, state); err != nil {
			return err
		}
	}
	return nil
}

// AccumulationTotalsSettings is how the totals of one register are kept in
// this base.
type AccumulationTotalsSettings struct {
	// MinPeriod is the beginning of the first month balances are stored at,
	// zero when there is no lower bound.
	MinPeriod time.Time
	// MaxPeriod is the last moment of the month balances are calculated
	// through - the date the balances are stored for is the next month start.
	// Zero when no monthly balances are stored.
	MaxPeriod     time.Time
	PresentTotals bool
	UseTotals     bool
}

// TotalsSettings reports the totals settings of a register.
func (repository *AccumulationRegisterRepository) TotalsSettings(ctx context.Context, name string) (AccumulationTotalsSettings, error) {
	definition, ok := repository.catalog.AccumulationRegisterDefinition(name)
	if !ok {
		return AccumulationTotalsSettings{}, fmt.Errorf("unknown accumulation register %q", name)
	}
	query, err := queryData(ctx, repository.pool)
	if err != nil {
		return AccumulationTotalsSettings{}, err
	}
	state, err := readBalanceTotalsState(ctx, query, definition.ID)
	if err != nil {
		return AccumulationTotalsSettings{}, recordDataError(ctx, repository.pool, err)
	}
	settings := AccumulationTotalsSettings{PresentTotals: state.present, UseTotals: state.use}
	if definition.Kind != AccumulationRegisterBalance {
		settings.PresentTotals = false
	}
	if state.minPeriod != nil {
		settings.MinPeriod = *state.minPeriod
	}
	if state.maxPeriod != nil {
		settings.MaxPeriod = state.maxPeriod.Add(-time.Second)
	}
	return settings, nil
}

// TotalsBound is one bound of the period of calculated totals as the caller
// gives it: Set says the bound is being changed at all, and a zero Date
// removes it.
type TotalsBound struct {
	Set  bool
	Date time.Time
}

// SetTotalsPeriods moves the bounds of the period of calculated totals.
//
// A minimum is the date balances are stored from, rounded down to its month;
// a maximum is the date they are calculated through, so a maximum of 31
// January keeps the balances of 1 February. The
// months that enter the period are calculated, the rows of the months that
// leave it are deleted, and the months that stay are not touched.
func (repository *AccumulationRegisterRepository) SetTotalsPeriods(ctx context.Context, name string, minimum, maximum TotalsBound) error {
	definition, err := repository.balanceTotalsDefinition(ctx, name)
	if err != nil {
		return err
	}
	bound := func(value TotalsBound, next bool) (*time.Time, error) {
		if value.Date.IsZero() {
			return nil, nil
		}
		if err := validateDocumentDate(value.Date.UTC()); err != nil {
			return nil, err
		}
		month := accumulationMonth(value.Date)
		if next {
			month = month.AddDate(0, 1, 0)
		}
		return &month, nil
	}
	return repository.manageTotals(ctx, definition, func(transactionContext context.Context, transaction pgx.Tx, state balanceTotalsState) error {
		next := state
		if minimum.Set {
			value, err := bound(minimum, false)
			if err != nil {
				return err
			}
			next.minPeriod = value
		}
		if maximum.Set {
			value, err := bound(maximum, true)
			if err != nil {
				return err
			}
			next.maxPeriod = value
		}
		if next.minPeriod != nil && next.maxPeriod != nil && next.minPeriod.After(*next.maxPeriod) {
			return fmt.Errorf("accumulation register %s: the lower bound of calculated totals is after the upper one", definition.Name)
		}
		if _, err := transaction.Exec(transactionContext, `
INSERT INTO ml_core.register_totals_modes(metadata_id, min_period, max_period) VALUES ($1, $2, $3)
ON CONFLICT (metadata_id) DO UPDATE SET min_period = EXCLUDED.min_period, max_period = EXCLUDED.max_period`,
			definition.ID, next.minPeriod, next.maxPeriod); err != nil {
			return fmt.Errorf("set totals period: %w", err)
		}
		return reshapeBalanceMonths(transactionContext, transaction, definition, state, next)
	})
}

// reshapeBalanceMonths brings the stored months from the old period to the
// new one. Only the difference is calculated: a month that was stored and
// stays stored is already right, and the monthly job that moves the upper
// bound by one month must cost one month, not the whole history.
func reshapeBalanceMonths(ctx context.Context, transaction pgx.Tx, definition AccumulationRegisterDefinition, old, next balanceTotalsState) error {
	totalsTable, _ := PhysicalAccumulationRegisterTotalsTable(definition.ID)
	totals := qualifiedCatalogTable(totalsTable)
	if !next.use {
		// Switched-off totals are rebuilt in full when switched on again, so
		// there is nothing worth calculating now.
		return nil
	}
	if next.maxPeriod == nil {
		_, err := transaction.Exec(ctx, "DELETE FROM "+totals+" WHERE total_period <> $1", accumulationPresentTotalsPeriod)
		return err
	}
	newFirst, err := balanceMonthsLowerBound(ctx, transaction, definition, next)
	if err != nil {
		return err
	}
	if newFirst == nil {
		// No movement and no lower bound: nothing to store.
		_, err := transaction.Exec(ctx, "DELETE FROM "+totals+" WHERE total_period <> $1", accumulationPresentTotalsPeriod)
		return err
	}
	newLast := *next.maxPeriod
	if _, err := transaction.Exec(ctx, "DELETE FROM "+totals+" WHERE total_period <> $1 AND (total_period < $2 OR total_period > $3)",
		accumulationPresentTotalsPeriod, *newFirst, newLast); err != nil {
		return fmt.Errorf("trim accumulation register %s balances: %w", definition.Name, err)
	}
	if old.maxPeriod == nil {
		return rebuildBalanceMonths(ctx, transaction, definition, *newFirst, newLast)
	}
	oldFirst, err := balanceMonthsLowerBound(ctx, transaction, definition, old)
	if err != nil {
		return err
	}
	if oldFirst == nil || oldFirst.After(newLast) || old.maxPeriod.Before(*newFirst) {
		return rebuildBalanceMonths(ctx, transaction, definition, *newFirst, newLast)
	}
	if newFirst.Before(*oldFirst) {
		if err := rebuildBalanceMonths(ctx, transaction, definition, *newFirst, oldFirst.AddDate(0, -1, 0)); err != nil {
			return err
		}
	}
	if newLast.After(*old.maxPeriod) {
		if err := rebuildBalanceMonths(ctx, transaction, definition, old.maxPeriod.AddDate(0, 1, 0), newLast); err != nil {
			return err
		}
	}
	return nil
}

// SetPresentTotalsUsing switches the present totals. Switched off they are not
// written, which takes away the one row every posting of a combination
// touches; the balance after the last stored month is then the stored
// balance plus the movements after it.
func (repository *AccumulationRegisterRepository) SetPresentTotalsUsing(ctx context.Context, name string, present bool) error {
	definition, err := repository.balanceTotalsDefinition(ctx, name)
	if err != nil {
		return err
	}
	return repository.manageTotals(ctx, definition, func(transactionContext context.Context, transaction pgx.Tx, state balanceTotalsState) error {
		if _, err := transaction.Exec(transactionContext, `
INSERT INTO ml_core.register_totals_modes(metadata_id, present_totals) VALUES ($1, $2)
ON CONFLICT (metadata_id) DO UPDATE SET present_totals = EXCLUDED.present_totals`, definition.ID, present); err != nil {
			return fmt.Errorf("set present totals: %w", err)
		}
		if !state.use {
			return nil
		}
		return rebuildBalancePresent(transactionContext, transaction, definition, present)
	})
}

// SetTotalsUsing switches the totals of a register as a whole. Switched off a
// write keeps no totals and the virtual tables are not available; switching
// on rebuilds them, because every movement written in between is missing
// from them.
func (repository *AccumulationRegisterRepository) SetTotalsUsing(ctx context.Context, name string, use bool) error {
	definition, ok := repository.catalog.AccumulationRegisterDefinition(name)
	if !ok {
		return fmt.Errorf("unknown accumulation register %q", name)
	}
	if err := requireObject(ctx, definition.ID, PermissionTotalsControl); err != nil {
		return err
	}
	return repository.manageTotals(ctx, definition, func(transactionContext context.Context, transaction pgx.Tx, state balanceTotalsState) error {
		if _, err := transaction.Exec(transactionContext, `
INSERT INTO ml_core.register_totals_modes(metadata_id, use_totals) VALUES ($1, $2)
ON CONFLICT (metadata_id) DO UPDATE SET use_totals = EXCLUDED.use_totals`, definition.ID, use); err != nil {
			return fmt.Errorf("set totals use: %w", err)
		}
		if !use || state.use {
			return nil
		}
		state.use = true
		return rebuildRegisterTotals(transactionContext, transaction, definition, state)
	})
}

// RecalcPresentTotals rebuilds only the present totals.
func (repository *AccumulationRegisterRepository) RecalcPresentTotals(ctx context.Context, name string) error {
	definition, err := repository.balanceTotalsDefinition(ctx, name)
	if err != nil {
		return err
	}
	return repository.manageTotals(ctx, definition, func(transactionContext context.Context, transaction pgx.Tx, state balanceTotalsState) error {
		if !state.use {
			return fmt.Errorf("accumulation register %s: %w", definition.Name, ErrAccumulationTotalsDisabled)
		}
		return rebuildBalancePresent(transactionContext, transaction, definition, state.present)
	})
}

// RecalcTotalsForPeriod rebuilds the totals of the months between two dates,
// either of which may be zero for "from the start" and "to the end".
func (repository *AccumulationRegisterRepository) RecalcTotalsForPeriod(ctx context.Context, name string, begin, end time.Time) error {
	definition, ok := repository.catalog.AccumulationRegisterDefinition(name)
	if !ok {
		return fmt.Errorf("unknown accumulation register %q", name)
	}
	if definition.Kind != AccumulationRegisterBalance {
		// A turnover register's totals are one row per month of movements,
		// and rebuilding them whole costs the same as rebuilding a range.
		return repository.RebuildTotals(ctx, name)
	}
	if err := requireObject(ctx, definition.ID, PermissionTotalsControl); err != nil {
		return err
	}
	return repository.manageTotals(ctx, definition, func(transactionContext context.Context, transaction pgx.Tx, state balanceTotalsState) error {
		if !state.use {
			return fmt.Errorf("accumulation register %s: %w", definition.Name, ErrAccumulationTotalsDisabled)
		}
		if state.maxPeriod == nil {
			return nil
		}
		first, err := balanceMonthsLowerBound(transactionContext, transaction, definition, state)
		if err != nil || first == nil {
			return err
		}
		last := *state.maxPeriod
		if !begin.IsZero() && accumulationMonth(begin).After(*first) {
			*first = accumulationMonth(begin)
		}
		if !end.IsZero() && accumulationMonth(end).Before(last) {
			last = accumulationMonth(end)
		}
		return rebuildBalanceMonths(transactionContext, transaction, definition, *first, last)
	})
}

func (repository *AccumulationRegisterRepository) balanceTotalsDefinition(ctx context.Context, name string) (AccumulationRegisterDefinition, error) {
	definition, ok := repository.catalog.AccumulationRegisterDefinition(name)
	if !ok {
		return AccumulationRegisterDefinition{}, fmt.Errorf("unknown accumulation register %q", name)
	}
	if definition.Kind != AccumulationRegisterBalance {
		// The prototype offers these settings for balance registers only.
		return AccumulationRegisterDefinition{}, fmt.Errorf("accumulation register %s does not store balances", definition.Name)
	}
	if err := requireObject(ctx, definition.ID, PermissionTotalsControl); err != nil {
		return AccumulationRegisterDefinition{}, err
	}
	return definition, nil
}

// manageTotals runs a change of the totals settings under the exclusive lock
// of the register: writers hold it shared, so while a setting changes and the
// rows it implies are rebuilt no movement is written into rows that are about
// to be replaced.
func (repository *AccumulationRegisterRepository) manageTotals(ctx context.Context, definition AccumulationRegisterDefinition, change func(context.Context, pgx.Tx, balanceTotalsState) error) error {
	return runDataTransaction(ctx, repository.pool, nil, func(transactionContext context.Context, transaction pgx.Tx) error {
		if _, err := transaction.Exec(transactionContext, "SELECT pg_advisory_xact_lock($1)", objectLockKey("accumulation-register", definition.ID, definition.ID)); err != nil {
			return fmt.Errorf("lock accumulation register %s: %w", definition.Name, err)
		}
		state, err := readBalanceTotalsState(transactionContext, transaction, definition.ID)
		if err != nil {
			return err
		}
		// A register still in the old format is brought to the current one
		// first: every setting below assumes the rows are balances, and a
		// management action is exactly the moment the lock to rebuild them is
		// already held.
		if definition.Kind == AccumulationRegisterBalance && state.format != accumulationBalanceTotalsFormat {
			if err := rebuildBalanceTotals(transactionContext, transaction, definition, state); err != nil {
				return err
			}
			state.format = accumulationBalanceTotalsFormat
		}
		return change(transactionContext, transaction, state)
	})
}

// rebuildRegisterTotals is the full rebuild of either kind of register.
func rebuildRegisterTotals(ctx context.Context, transaction pgx.Tx, definition AccumulationRegisterDefinition, state balanceTotalsState) error {
	if definition.Kind == AccumulationRegisterBalance {
		return rebuildBalanceTotals(ctx, transaction, definition, state)
	}
	return rebuildTurnoverTotals(ctx, transaction, definition)
}
