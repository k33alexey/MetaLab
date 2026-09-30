package metadata

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	bslnumber "github.com/k33alexey/MetaLab/internal/bsl/number"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	maxAccumulationRegisterRecords = 1_000_000
	maxAccumulationRegisterMemory  = 64 << 20
	// accumulationTotalRowSpread is how wide the random step is when writers
	// make rows of one combination at once - see accumulationTotalRowStep.
	// Wide enough that a handful of writers rarely land on one number, small
	// enough that the numbering stays a short list.
	accumulationTotalRowSpread = 64
	// maxAccumulationTotalRowNumber is the highest number a row of totals can
	// carry, and it is the ceiling of the smallint column that holds it. The
	// number is not allowed past it: a number PostgreSQL refuses would fail a
	// write for arithmetic, and a write of movements must not fail for
	// arithmetic. At the ceiling the new row lands on the highest existing one
	// instead and the change is added to it - see totalRowTakeOrMake.
	maxAccumulationTotalRowNumber = 32767
)

// AccumulationMovementKind is the direction of a balance-register movement.
type AccumulationMovementKind int16

const (
	AccumulationMovementReceipt AccumulationMovementKind = 1
	AccumulationMovementExpense AccumulationMovementKind = 2
)

// AccumulationRegisterRecord is one recorder-owned movement.
type AccumulationRegisterRecord struct {
	RecordID     uuid.UUID
	Period       time.Time
	Recorder     DocumentReference
	LineNumber   int
	Active       bool
	MovementKind AccumulationMovementKind
	Dimensions   map[uuid.UUID]Value
	Resources    map[uuid.UUID]Value
	Attributes   map[uuid.UUID]Value
}

// AccumulationRegisterFilter identifies the recorder owned by a record set.
type AccumulationRegisterFilter struct {
	Recorder *DocumentReference
}

// AccumulationRegisterRecordSet is the mutable unit of movement I/O.
type AccumulationRegisterRecordSet struct {
	RegisterID    uuid.UUID
	Filter        AccumulationRegisterFilter
	Records       []*AccumulationRegisterRecord
	LockForUpdate bool
}

func (set *AccumulationRegisterRecordSet) Add() (*AccumulationRegisterRecord, error) {
	if set == nil {
		return nil, fmt.Errorf("accumulation register record set is required")
	}
	if len(set.Records) >= maxAccumulationRegisterRecords {
		return nil, fmt.Errorf("accumulation register record set exceeds %d records", maxAccumulationRegisterRecords)
	}
	id, err := uuid.New()
	if err != nil {
		return nil, err
	}
	record := &AccumulationRegisterRecord{
		RecordID: id, Active: true, MovementKind: AccumulationMovementReceipt,
		Dimensions: map[uuid.UUID]Value{}, Resources: map[uuid.UUID]Value{}, Attributes: map[uuid.UUID]Value{},
	}
	set.Records = append(set.Records, record)
	return record, nil
}

// AccumulationRegisterTotalsRow is one row returned by a basic virtual table.
//
// It was called an aggregate until the register got real ones. Агрегат is the
// prototype's word for a precomputed cut declared in the configuration - see
// accumulation_aggregates.go - and leaving it on a row of a virtual table
// would have made every later reader ask which of the two was meant.
type AccumulationRegisterTotalsRow struct {
	Dimensions map[uuid.UUID]Value
	Opening    map[uuid.UUID]Value
	Receipt    map[uuid.UUID]Value
	Expense    map[uuid.UUID]Value
	Turnover   map[uuid.UUID]Value
	Closing    map[uuid.UUID]Value
}

type AccumulationRegisterRepository struct {
	pool    *pgxpool.Pool
	catalog *Catalog
}

func NewAccumulationRegisterRepository(pool *pgxpool.Pool, catalog *Catalog) (*AccumulationRegisterRepository, error) {
	if pool == nil || catalog == nil {
		return nil, fmt.Errorf("accumulation register repository requires PostgreSQL and metadata catalog")
	}
	return &AccumulationRegisterRepository{pool: pool, catalog: catalog}, nil
}

func (repository *AccumulationRegisterRepository) NewRecordSet(name string) (*AccumulationRegisterRecordSet, error) {
	definition, ok := repository.catalog.AccumulationRegisterDefinition(name)
	if !ok {
		return nil, fmt.Errorf("unknown accumulation register %q", name)
	}
	return &AccumulationRegisterRecordSet{RegisterID: definition.ID, Records: []*AccumulationRegisterRecord{}}, nil
}

func (repository *AccumulationRegisterRepository) Read(ctx context.Context, set *AccumulationRegisterRecordSet) error {
	definition, filter, err := repository.normalizeSetIdentity(set, true)
	if err != nil {
		return err
	}
	if err := requireUnrestrictedRegisterRead(ctx, definition.ID, definition.Name); err != nil {
		return err
	}
	query, err := queryData(ctx, repository.pool)
	if err != nil {
		return err
	}
	records, err := repository.readMovements(ctx, query, definition, *filter.Recorder)
	if err != nil {
		return recordDataError(ctx, repository.pool, err)
	}
	set.Filter, set.Records = filter, records
	return nil
}

func (repository *AccumulationRegisterRepository) Write(ctx context.Context, set *AccumulationRegisterRecordSet, replace bool) error {
	return repository.WriteWithHandler(ctx, set, replace, nil)
}

func (repository *AccumulationRegisterRepository) WriteWithHandler(ctx context.Context, set *AccumulationRegisterRecordSet, replace bool, handler AccumulationRegisterEventHandler) error {
	if set == nil {
		return fmt.Errorf("accumulation register record set is required")
	}
	original, working := cloneAccumulationRegisterRecordSet(set), cloneAccumulationRegisterRecordSet(set)
	definition, filter, err := repository.normalizeSetIdentity(working, true)
	if err != nil {
		return err
	}
	working.Filter = filter
	if err := repository.normalizeRecords(definition, working); err != nil {
		return err
	}
	if size, ok := accumulationRegisterSetMemory(working, maxAccumulationRegisterMemory); !ok || size > maxAccumulationRegisterMemory {
		return fmt.Errorf("accumulation register %s record set exceeds the %d-byte memory limit", definition.Name, maxAccumulationRegisterMemory)
	}
	err = runDataTransaction(ctx, repository.pool, func() { *set = *cloneAccumulationRegisterRecordSet(original) }, func(transactionContext context.Context, transaction pgx.Tx) error {
		if err := dispatchAccumulationRegisterEvent(transactionContext, handler, AccumulationRegisterEventBeforeWrite, working, replace); err != nil {
			return err
		}
		afterDefinition, afterFilter, err := repository.normalizeSetIdentity(working, true)
		if err != nil {
			return err
		}
		if afterDefinition.ID != definition.ID || *afterFilter.Recorder != *filter.Recorder || working.LockForUpdate != original.LockForUpdate {
			return fmt.Errorf("accumulation register before-write event changed immutable record-set identity, filter or lock mode")
		}
		working.Filter = filter
		if err := repository.normalizeRecords(definition, working); err != nil {
			return err
		}
		if size, ok := accumulationRegisterSetMemory(working, maxAccumulationRegisterMemory); !ok || size > maxAccumulationRegisterMemory {
			return fmt.Errorf("accumulation register %s record set exceeds the %d-byte memory limit", definition.Name, maxAccumulationRegisterMemory)
		}
		if err := lockAccumulationRegisterWriteGate(transactionContext, transaction, definition, *filter.Recorder, working.LockForUpdate); err != nil {
			return err
		}
		if err := repository.validateRecorder(transactionContext, transaction, definition, *filter.Recorder); err != nil {
			return err
		}
		var old []*AccumulationRegisterRecord
		if replace {
			old, err = repository.readMovements(transactionContext, transaction, definition, *filter.Recorder)
			if err != nil {
				return err
			}
			table, _ := PhysicalAccumulationRegisterTable(definition.ID)
			if _, err := transaction.Exec(transactionContext, "DELETE FROM "+qualifiedCatalogTable(table)+" WHERE recorder_type = $1 AND recorder_ref = $2", filter.Recorder.DocumentID.String(), filter.Recorder.ObjectID.String()); err != nil {
				return accumulationRegisterWriteError(definition.Name, err)
			}
		}
		if !working.LockForUpdate {
			if err := lockAccumulationRegisterDimensions(transactionContext, transaction, definition, append(slices.Clone(old), working.Records...)); err != nil {
				return err
			}
		}
		// The mode is data of this base, read inside the write's own
		// transaction - see register_totals_mode.go. It decides where the
		// totals of this write land and nothing else: rows written under the
		// other mode are summed beside these ones on reading.
		splitting, err := effectiveTotalsSplitting(transactionContext, transaction, definition.ID, definition.AllowTotalsSplitting)
		if err != nil {
			return err
		}
		// The rest of the totals settings are read the same way and under
		// the same shared lock, which a change of them waits out - see
		// manageTotals. A register whose totals are still turnovers refuses
		// the write: adding a balance to a row that holds a turnover would
		// make it neither.
		totalsState, err := readBalanceTotalsState(transactionContext, transaction, definition.ID)
		if err != nil {
			return err
		}
		if err := requireBalanceTotalsFormat(transactionContext, transaction, transaction, definition, totalsState); err != nil {
			return err
		}
		if err := repository.insertMovements(transactionContext, transaction, definition, working.Records); err != nil {
			return err
		}
		if err := repository.applyTotalsChanges(transactionContext, transaction, definition, old, working.Records, splitting, totalsState); err != nil {
			return err
		}
		for _, event := range []AccumulationRegisterEvent{AccumulationRegisterEventOnWrite, AccumulationRegisterEventAfterWrite} {
			if err := dispatchAccumulationRegisterEvent(transactionContext, handler, event, cloneAccumulationRegisterRecordSet(working), replace); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	*set = *working
	return nil
}

func (repository *AccumulationRegisterRepository) normalizeSetIdentity(set *AccumulationRegisterRecordSet, requireRecorder bool) (AccumulationRegisterDefinition, AccumulationRegisterFilter, error) {
	if set == nil || set.RegisterID.IsZero() {
		return AccumulationRegisterDefinition{}, AccumulationRegisterFilter{}, fmt.Errorf("accumulation register record set identity is invalid")
	}
	definition, ok := repository.catalog.AccumulationRegisterByID(set.RegisterID)
	if !ok {
		return AccumulationRegisterDefinition{}, AccumulationRegisterFilter{}, fmt.Errorf("unknown accumulation register %s", set.RegisterID)
	}
	filter := cloneAccumulationRegisterFilter(set.Filter)
	if filter.Recorder == nil {
		if requireRecorder {
			return AccumulationRegisterDefinition{}, AccumulationRegisterFilter{}, fmt.Errorf("accumulation register %s recorder filter is required", definition.Name)
		}
		return definition, filter, nil
	}
	if !allowedAccumulationRegisterRecorder(repository.catalog, definition, *filter.Recorder) {
		return AccumulationRegisterDefinition{}, AccumulationRegisterFilter{}, fmt.Errorf("accumulation register %s recorder filter is invalid", definition.Name)
	}
	return definition, filter, nil
}

func (repository *AccumulationRegisterRepository) normalizeRecords(definition AccumulationRegisterDefinition, set *AccumulationRegisterRecordSet) error {
	if len(set.Records) > maxAccumulationRegisterRecords {
		return fmt.Errorf("accumulation register %s record set exceeds %d records", definition.Name, maxAccumulationRegisterRecords)
	}
	usedLines := make(map[int]bool, len(set.Records))
	nextLine := 1
	for _, record := range set.Records {
		if record != nil && record.LineNumber >= nextLine && int64(record.LineNumber) <= maxInformationRegisterLineNumber {
			nextLine = record.LineNumber + 1
		}
	}
	for index, record := range set.Records {
		if record == nil {
			return fmt.Errorf("accumulation register %s record %d is nil", definition.Name, index+1)
		}
		if record.LineNumber == 0 {
			if int64(nextLine) > maxInformationRegisterLineNumber {
				return fmt.Errorf("accumulation register %s line number is exhausted", definition.Name)
			}
			record.LineNumber, nextLine = nextLine, nextLine+1
		}
		if usedLines[record.LineNumber] {
			return fmt.Errorf("accumulation register %s record set contains a duplicate line number", definition.Name)
		}
		usedLines[record.LineNumber] = true
		if err := repository.normalizeRecord(definition, record, index+1); err != nil {
			return err
		}
		if set.Filter.Recorder == nil || record.Recorder != *set.Filter.Recorder {
			return fmt.Errorf("accumulation register %s record %d does not match the recorder filter", definition.Name, index+1)
		}
	}
	return nil
}

func (repository *AccumulationRegisterRepository) normalizeRecord(definition AccumulationRegisterDefinition, record *AccumulationRegisterRecord, line int) error {
	if record.RecordID.IsZero() {
		id, err := uuid.New()
		if err != nil {
			return err
		}
		record.RecordID = id
	}
	var err error
	record.Period, err = normalizeAccumulationPeriod(record.Period)
	if err != nil {
		return fmt.Errorf("accumulation register %s record %d period: %w", definition.Name, line, err)
	}
	if !allowedAccumulationRegisterRecorder(repository.catalog, definition, record.Recorder) {
		return fmt.Errorf("accumulation register %s record %d has an invalid recorder", definition.Name, line)
	}
	if record.LineNumber < 1 || int64(record.LineNumber) > maxInformationRegisterLineNumber {
		return fmt.Errorf("accumulation register %s record %d line number must be 1..%d", definition.Name, line, maxInformationRegisterLineNumber)
	}
	if definition.Kind == AccumulationRegisterBalance {
		if record.MovementKind != AccumulationMovementReceipt && record.MovementKind != AccumulationMovementExpense {
			return fmt.Errorf("accumulation register %s record %d movement kind is invalid", definition.Name, line)
		}
	} else {
		record.MovementKind = 0
	}
	record.Dimensions, err = repository.catalog.normalizeAttributes(definition.Name, RegisterDimensionAttributes(definition.Dimensions), record.Dimensions)
	if err != nil {
		return fmt.Errorf("accumulation register %s record %d dimensions: %w", definition.Name, line, err)
	}
	if record.Resources == nil {
		record.Resources = map[uuid.UUID]Value{}
	}
	for _, resource := range definition.Resources {
		if _, ok := record.Resources[resource.ID]; !ok {
			record.Resources[resource.ID] = Value{Kind: NumberType, Data: "0"}
		}
	}
	record.Resources, err = repository.catalog.normalizeAttributes(definition.Name, definition.Resources, record.Resources)
	if err != nil {
		return fmt.Errorf("accumulation register %s record %d resources: %w", definition.Name, line, err)
	}
	record.Attributes, err = repository.catalog.normalizeAttributes(definition.Name, definition.Attributes, record.Attributes)
	if err != nil {
		return fmt.Errorf("accumulation register %s record %d attributes: %w", definition.Name, line, err)
	}
	return nil
}

func normalizeAccumulationPeriod(value time.Time) (time.Time, error) {
	value = value.UTC().Truncate(100 * time.Microsecond)
	if err := validateDocumentDate(value); err != nil {
		return time.Time{}, err
	}
	return value, nil
}

func allowedAccumulationRegisterRecorder(catalog *Catalog, definition AccumulationRegisterDefinition, recorder DocumentReference) bool {
	return !recorder.DocumentID.IsZero() && !recorder.ObjectID.IsZero() && catalog.writesInto(recorder.DocumentID, definition.ID)
}

func (repository *AccumulationRegisterRepository) validateRecorder(ctx context.Context, transaction pgx.Tx, definition AccumulationRegisterDefinition, recorder DocumentReference) error {
	if err := lockObjectForWrite(ctx, transaction, DocumentType, recorder.DocumentID, recorder.ObjectID); err != nil {
		return err
	}
	table, _ := PhysicalDocumentTable(recorder.DocumentID)
	var exists bool
	if err := transaction.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM "+qualifiedCatalogTable(table)+" WHERE ref = $1)", recorder.ObjectID.String()).Scan(&exists); err != nil {
		return fmt.Errorf("validate accumulation register %s recorder: %w", definition.Name, err)
	}
	if !exists {
		return fmt.Errorf("accumulation register %s recorder does not exist", definition.Name)
	}
	return nil
}

func (repository *AccumulationRegisterRepository) insertMovements(ctx context.Context, transaction pgx.Tx, definition AccumulationRegisterDefinition, records []*AccumulationRegisterRecord) error {
	if len(records) == 0 {
		return nil
	}
	table, _ := PhysicalAccumulationRegisterTable(definition.ID)
	// The movement does not carry the row of totals it contributed to, and
	// cannot: with rows multiplying on contention the row is decided by who
	// held what at that moment, not by anything about the movement. The
	// column was written and never read even before that - the rebuild stopped
	// grouping by it when it started folding the rows together.
	columns := []string{"record_id", "dimension_key", "period", "recorder_type", "recorder_ref", "line_no", "active"}
	if definition.Kind == AccumulationRegisterBalance {
		columns = append(columns, "movement_kind")
	}
	for _, field := range accumulationRegisterFields(definition) {
		column, _ := PhysicalAttributeColumn(field.ID)
		columns = append(columns, column)
	}
	source := pgx.CopyFromSlice(len(records), func(index int) ([]any, error) {
		record := records[index]
		arguments := []any{record.RecordID.String(), accumulationDimensionKey(definition, record.Dimensions), record.Period,
			record.Recorder.DocumentID.String(), record.Recorder.ObjectID.String(), record.LineNumber, record.Active}
		if definition.Kind == AccumulationRegisterBalance {
			arguments = append(arguments, int16(record.MovementKind))
		}
		for _, group := range []struct {
			fields []Attribute
			values map[uuid.UUID]Value
		}{
			{RegisterDimensionAttributes(definition.Dimensions), record.Dimensions}, {definition.Resources, record.Resources}, {definition.Attributes, record.Attributes},
		} {
			for _, field := range group.fields {
				value, present := group.values[field.ID]
				if !present {
					arguments = append(arguments, nil)
					continue
				}
				storage, _ := repository.catalog.attributeStorage(field.Types)
				encoded, err := databaseAttributeValue(storage, value)
				if err != nil {
					return nil, err
				}
				arguments = append(arguments, encoded)
			}
		}
		return arguments, nil
	})
	written, err := transaction.CopyFrom(ctx, pgx.Identifier{schemadiff.ApplicationSchema, table}, columns, source)
	if err != nil {
		return accumulationRegisterWriteError(definition.Name, err)
	}
	if written != int64(len(records)) {
		return fmt.Errorf("write accumulation register %s: copied %d of %d records", definition.Name, written, len(records))
	}
	return nil
}

func (repository *AccumulationRegisterRepository) readMovements(ctx context.Context, query dataQueryer, definition AccumulationRegisterDefinition, recorder DocumentReference) ([]*AccumulationRegisterRecord, error) {
	table, _ := PhysicalAccumulationRegisterTable(definition.ID)
	statement := fmt.Sprintf("SELECT pg_column_size(item), CASE WHEN pg_column_size(item) <= %d THEN to_jsonb(item) END FROM %s AS item WHERE recorder_type = $1 AND recorder_ref = $2 ORDER BY line_no, record_id LIMIT %d", maxAccumulationRegisterMemory, qualifiedCatalogTable(table), maxAccumulationRegisterRecords+1)
	rows, err := query.Query(ctx, statement, recorder.DocumentID.String(), recorder.ObjectID.String())
	if err != nil {
		return nil, fmt.Errorf("read accumulation register %s: %w", definition.Name, err)
	}
	defer rows.Close()
	records, memory := make([]*AccumulationRegisterRecord, 0), uint64(512)
	for rows.Next() {
		if len(records) >= maxAccumulationRegisterRecords {
			return nil, fmt.Errorf("accumulation register %s read exceeds %d records", definition.Name, maxAccumulationRegisterRecords)
		}
		var storedBytes int64
		var encoded []byte
		if err := rows.Scan(&storedBytes, &encoded); err != nil {
			return nil, err
		}
		if storedBytes < 0 || uint64(storedBytes) > maxAccumulationRegisterMemory {
			return nil, fmt.Errorf("accumulation register %s row exceeds memory limit", definition.Name)
		}
		record, err := repository.decodeMovement(definition, encoded)
		if err != nil {
			return nil, err
		}
		addition := accumulationRegisterRecordMemory(record)
		if addition > maxAccumulationRegisterMemory-memory {
			return nil, fmt.Errorf("accumulation register %s read exceeds memory limit", definition.Name)
		}
		memory += addition
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (repository *AccumulationRegisterRepository) decodeMovement(definition AccumulationRegisterDefinition, encoded []byte) (*AccumulationRegisterRecord, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, fmt.Errorf("decode accumulation register %s row: %w", definition.Name, err)
	}
	recordID, err := decodeJSONUUID(fields["record_id"])
	if err != nil {
		return nil, err
	}
	record := &AccumulationRegisterRecord{RecordID: recordID, Dimensions: map[uuid.UUID]Value{}, Resources: map[uuid.UUID]Value{}, Attributes: map[uuid.UUID]Value{}}
	var period string
	if err := json.Unmarshal(fields["period"], &period); err != nil {
		return nil, err
	}
	record.Period, err = time.Parse(time.RFC3339Nano, period)
	if err != nil {
		return nil, err
	}
	record.Recorder.DocumentID, err = decodeJSONUUID(fields["recorder_type"])
	if err != nil {
		return nil, err
	}
	record.Recorder.ObjectID, err = decodeJSONUUID(fields["recorder_ref"])
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(fields["line_no"], &record.LineNumber); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(fields["active"], &record.Active); err != nil {
		return nil, err
	}
	if definition.Kind == AccumulationRegisterBalance {
		if err := json.Unmarshal(fields["movement_kind"], &record.MovementKind); err != nil {
			return nil, err
		}
	}
	for _, group := range []struct {
		fields []Attribute
		values map[uuid.UUID]Value
	}{
		{RegisterDimensionAttributes(definition.Dimensions), record.Dimensions}, {definition.Resources, record.Resources}, {definition.Attributes, record.Attributes},
	} {
		for _, field := range group.fields {
			column, _ := PhysicalAttributeColumn(field.ID)
			raw := fields[column]
			if len(raw) == 0 || string(raw) == "null" {
				continue
			}
			storage, _ := repository.catalog.attributeStorage(field.Types)
			value, err := decodeDatabaseAttribute(storage, raw)
			if err != nil {
				return nil, fmt.Errorf("decode accumulation register %s field %s: %w", definition.Name, field.Name, err)
			}
			value, err = repository.catalog.normalizeTypes("accumulation register "+definition.Name+" field "+field.Name, field.Types, value)
			if err != nil {
				return nil, err
			}
			group.values[field.ID] = value
		}
	}
	return record, nil
}

// accumulationTotalDelta is what one combination of month and dimensions is
// changed by. Which row of totals carries the change is not part of it: the
// row is chosen when the change is written, by who is holding what.
type accumulationTotalDelta struct {
	period       time.Time
	dimensionKey string
	dimensions   map[uuid.UUID]Value
	resources    map[uuid.UUID]*big.Rat
}

// applyTotalsChanges turns the movements a write removed and added into the
// changes of the rows of totals they belong to.
//
// A turnover register has one row per month of a movement. A balance register
// has a row for the beginning of every stored month after the movement and
// one for the present totals, so a movement dated into the past changes more
// rows than one dated today - see balanceTargets.
func (repository *AccumulationRegisterRepository) applyTotalsChanges(ctx context.Context, transaction pgx.Tx, definition AccumulationRegisterDefinition, removed, added []*AccumulationRegisterRecord, splitting bool, state balanceTotalsState) error {
	if !state.use {
		// Switched-off totals are not kept by a write at all; they are
		// rebuilt when switched back on.
		return nil
	}
	deltas := map[string]*accumulationTotalDelta{}
	accumulate := func(record *AccumulationRegisterRecord, replacementSign int64) error {
		if record == nil || !record.Active {
			return nil
		}
		periods := []time.Time{accumulationMonth(record.Period)}
		if definition.Kind == AccumulationRegisterBalance {
			periods = state.balanceTargets(record.Period)
		}
		dimensionKey := accumulationDimensionKey(definition, record.Dimensions)
		direction := replacementSign
		if definition.Kind == AccumulationRegisterBalance && record.MovementKind == AccumulationMovementExpense {
			direction = -direction
		}
		amounts := make(map[uuid.UUID]*big.Rat, len(definition.Resources))
		for _, resource := range definition.Resources {
			value := record.Resources[resource.ID]
			amount, ok := new(big.Rat).SetString(value.Data)
			if !ok {
				return fmt.Errorf("accumulation register %s resource %s is not numeric", definition.Name, resource.Name)
			}
			if direction < 0 {
				amount.Neg(amount)
			}
			amounts[resource.ID] = amount
		}
		for _, period := range periods {
			// The key sorts by period first, and the sorted keys are the
			// order rows are taken in: every writer takes the rows of the
			// same combinations in the same order, the present totals last.
			key := period.Format(time.RFC3339Nano) + ":" + dimensionKey
			delta := deltas[key]
			if delta == nil {
				delta = &accumulationTotalDelta{period: period, dimensionKey: dimensionKey, dimensions: mapsCloneValues(record.Dimensions), resources: map[uuid.UUID]*big.Rat{}}
				deltas[key] = delta
			}
			for _, resource := range definition.Resources {
				if delta.resources[resource.ID] == nil {
					delta.resources[resource.ID] = new(big.Rat)
				}
				delta.resources[resource.ID].Add(delta.resources[resource.ID], amounts[resource.ID])
			}
		}
		return nil
	}
	for _, record := range removed {
		if err := accumulate(record, -1); err != nil {
			return err
		}
	}
	for _, record := range added {
		if err := accumulate(record, 1); err != nil {
			return err
		}
	}
	if len(deltas) == 0 {
		return nil
	}
	keys := make([]string, 0, len(deltas))
	for key := range deltas {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return repository.applyTotalDeltas(ctx, transaction, definition, deltas, keys, splitting)
}

// applyTotalDeltas puts each change into a row of totals, making a new row
// when it cannot have an existing one.
//
// Разделение итогов, as the prototype describes it: sessions writing
// movements at the same time put their changes of the totals into separate
// rows instead of updating the same ones, the rows multiply only while
// transactions actually run in parallel, and a combination ends up with as
// many rows as the most transactions that ever wrote it at once.
//
// So a row is not addressed, it is taken. A write asks for any row of its
// combination that nobody is holding; if it gets one it adds to it, and if it
// gets none - because there are none, or because every one of them is held -
// it makes another. The count then follows the concurrency that actually
// happened instead of a constant, which is the whole difference from what was
// here before: sixteen rows per combination chosen by hashing the recorder,
// written even by a database with one person in it.
//
// **Which row gets the change does not matter, and that is what makes this
// work at all.** Reading totals sums the rows of a combination, so a change
// may land anywhere among them - including a subtraction that leaves one row
// negative while the sum stays right. The prototype says the same of its own
// mechanism: it changes how much can run in parallel and nothing of what the
// application sees.
//
// With splitting off there is one row per combination and a write waits for
// it rather than making a second - that is what not splitting means.
func (repository *AccumulationRegisterRepository) applyTotalDeltas(
	ctx context.Context, transaction pgx.Tx, definition AccumulationRegisterDefinition,
	deltas map[string]*accumulationTotalDelta, keys []string, splitting bool,
) error {
	type applied struct {
		delta *accumulationTotalDelta
		split int16
	}
	var touched []applied
	// **Every combination is taken or made by one statement, in the order of
	// the keys, which is the same for every writer.** The order is what keeps
	// writers from waiting for each other in a circle: while a writer is at a
	// combination it holds rows of the combinations before it and of none
	// after, so whoever it waits for has gone further along the same order and
	// never waits back.
	//
	// The order has to hold for everything that can wait, and two kinds of
	// statement can: taking a row, which with splitting off waits for a held
	// one, and making a row, whose unique check waits for any writer that is
	// inserting, updating or deleting a row with the same number. The second
	// kind was missed twice. Taking all combinations first and making the
	// missing ones afterwards let a writer hold a row of a later combination
	// while it waited to make an earlier one - and another writer, making a
	// row of that later combination under a number a third one had just
	// committed, waited for it back. A backdated movement of a balance
	// register changes a row for every stored month after it, which is what
	// made the circle likely enough to show.
	resolved, leftover, err := repository.takeOrMakeTotalRows(ctx, transaction, definition, deltas, keys, splitting)
	if err != nil {
		return err
	}
	for _, row := range resolved {
		touched = append(touched, applied{deltas[row.key], row.split})
	}
	if len(leftover) != 0 {
		// The statement takes, makes or merges, so one of the three always
		// answers; silence here would be a change that went nowhere.
		return fmt.Errorf("update accumulation register %s totals: no row of totals could be taken", definition.Name)
	}
	return repository.dropEmptyTotals(ctx, transaction, definition, func(yield func(*accumulationTotalDelta, int16)) {
		for _, row := range touched {
			yield(row.delta, row.split)
		}
	})
}

// takeOrMakeTotalRows gives each change a row in one statement per
// combination, all of them in one batch and in the order of the keys.
func (repository *AccumulationRegisterRepository) takeOrMakeTotalRows(
	ctx context.Context, transaction pgx.Tx, definition AccumulationRegisterDefinition,
	deltas map[string]*accumulationTotalDelta, keys []string, race bool,
) ([]totalRow, []string, error) {
	statement := repository.totalRowTakeOrMake(definition, race)
	var resolved []totalRow
	var leftover []string
	for start := 0; start < len(keys); start += 256 {
		end := min(start+256, len(keys))
		batch := &pgx.Batch{}
		for _, key := range keys[start:end] {
			arguments, err := repository.totalRowInsertArguments(definition, deltas[key])
			if err != nil {
				return nil, nil, err
			}
			batch.Queue(statement, append(arguments, accumulationTotalRowStep())...)
		}
		results := transaction.SendBatch(ctx, batch)
		for _, key := range keys[start:end] {
			var split int16
			switch scanErr := results.QueryRow().Scan(&split); {
			case scanErr == pgx.ErrNoRows:
				leftover = append(leftover, key)
			case scanErr != nil:
				_ = results.Close()
				return nil, nil, fmt.Errorf("write accumulation register %s totals: %w", definition.Name, scanErr)
			default:
				resolved = append(resolved, totalRow{key, split})
			}
		}
		if err := results.Close(); err != nil {
			return nil, nil, fmt.Errorf("write accumulation register %s totals: %w", definition.Name, err)
		}
	}
	return resolved, leftover, nil
}

// totalRowTakeOrMake adds the change to a free row of the combination, and
// when there is none makes a new one - in one statement, so that a combination
// is settled completely before the next one is touched. The arguments are the
// period, the key, the dimensions, the amounts and the step past the highest
// number.
//
// Racing means SKIP LOCKED: a row another writer is holding is not a row this
// one can have, and waiting for it is what разделение итогов exists to avoid.
// Without racing the statement waits for the lowest row instead, which is how
// a register that does not split its totals writes them.
//
// A new row is numbered 0 when the combination has none, and otherwise a
// random step past the highest number: rows are made only when every existing
// one is held, so the makers are writers meeting each other, and they see the
// same highest number because none sees another's uncommitted row. The step
// keeps them apart. When two still land on one number, the second adds its
// change to the first one's row instead of giving up - it waits for that
// writer to finish, but it never leaves the statement without a row, and that
// is what keeps the order of the keys whole. Which row gets a change does not
// matter: reading sums the rows of a combination. The number stops at the
// ceiling of its column, where it lands on the highest row on purpose: see
// maxAccumulationTotalRowNumber.
func (repository *AccumulationRegisterRepository) totalRowTakeOrMake(definition AccumulationRegisterDefinition, race bool) string {
	table, _ := PhysicalAccumulationRegisterTotalsTable(definition.ID)
	qualified := qualifiedCatalogTable(table)
	firstResource := 3 + len(definition.Dimensions)
	updates := make([]string, 0, len(definition.Resources))
	for index, resource := range definition.Resources {
		column, _ := PhysicalAttributeColumn(resource.ID)
		quoted := pgx.Identifier{column}.Sanitize()
		updates = append(updates, fmt.Sprintf("%s = %s + $%d", quoted, quoted, firstResource+index))
	}
	locking := "FOR UPDATE"
	if race {
		locking = "FOR UPDATE SKIP LOCKED"
	}
	columns := []string{"total_period", "totals_split", "dimension_key"}
	values := []string{"$1::timestamptz", "", "$2"}
	position := 2
	for _, dimension := range definition.Dimensions {
		column, _ := PhysicalAttributeColumn(dimension.ID)
		position++
		columns = append(columns, column)
		values = append(values, fmt.Sprintf("$%d", position))
	}
	for _, resource := range definition.Resources {
		column, _ := PhysicalAttributeColumn(resource.ID)
		position++
		columns = append(columns, column)
		values = append(values, fmt.Sprintf("$%d", position))
	}
	// Counted in integer and narrowed last: the highest number plus the step
	// overflows smallint at the ceiling before LEAST could stop it.
	values[1] = fmt.Sprintf("LEAST(COALESCE((SELECT MAX(totals_split) FROM %s WHERE total_period = $1 AND dimension_key = $2)::integer + $%d::integer, 0), %d)::smallint",
		qualified, position+1, maxAccumulationTotalRowNumber)
	merges := make([]string, 0, len(definition.Resources))
	for _, resource := range definition.Resources {
		column, _ := PhysicalAttributeColumn(resource.ID)
		quoted := pgx.Identifier{column}.Sanitize()
		merges = append(merges, quoted+" = "+qualified+"."+quoted+" + EXCLUDED."+quoted)
	}
	conflict := "DO NOTHING"
	if len(merges) != 0 {
		conflict = "DO UPDATE SET " + strings.Join(merges, ", ")
	}
	take := "UPDATE " + qualified + " SET " + strings.Join(updates, ", ") +
		" WHERE total_period = $1 AND dimension_key = $2 AND totals_split = (" +
		"SELECT totals_split FROM " + qualified + " WHERE total_period = $1 AND dimension_key = $2 " +
		"ORDER BY totals_split " + locking + " LIMIT 1) RETURNING totals_split"
	create := "INSERT INTO " + qualified + " (" + strings.Join(quoteCatalogColumns(columns), ", ") + ") SELECT " + strings.Join(values, ", ") +
		" WHERE NOT EXISTS (SELECT 1 FROM taken)" +
		" ON CONFLICT (total_period, dimension_key, totals_split) " + conflict + " RETURNING totals_split"
	return "WITH taken AS (" + take + "), made AS (" + create + ") SELECT totals_split FROM taken UNION ALL SELECT totals_split FROM made"
}

type totalRow struct {
	key   string
	split int16
}

// accumulationTotalRowStep is how far past the highest existing number a new
// row of totals is placed beside rows that are all held.
//
// The writers that make such rows are writers meeting each other, and they see
// the same highest number, because none sees another's uncommitted row. A
// fixed step would put all of them on one number, and all but the first would
// wait for it - which is exactly the waiting splitting exists to avoid. So the
// step is drawn at random, and the few that still land together share the row.
//
// Gaps in the numbering cost nothing. The number stopped being an address when
// rows stopped being chosen by a hash: it only tells one row of a combination
// from another, reading sums them all, and a rebuild folds them back into one.
func accumulationTotalRowStep() int16 {
	return int16(1 + rand.IntN(accumulationTotalRowSpread))
}

// dropEmptyTotals removes the rows this write left at zero. Only the rows it
// touched, because those are the ones it holds: deleting a row held by another
// writer would wait for it, which is the waiting the mechanism exists to avoid.
func (repository *AccumulationRegisterRepository) dropEmptyTotals(
	ctx context.Context, transaction pgx.Tx, definition AccumulationRegisterDefinition,
	rows func(func(*accumulationTotalDelta, int16)),
) error {
	statement := accumulationZeroTotalDelete(definition)
	batch := &pgx.Batch{}
	queued := 0
	rows(func(delta *accumulationTotalDelta, split int16) {
		batch.Queue(statement, delta.period, split, delta.dimensionKey)
		queued++
	})
	if queued == 0 {
		return nil
	}
	results := transaction.SendBatch(ctx, batch)
	for index := 0; index < queued; index++ {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("clean accumulation register %s totals: %w", definition.Name, err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("clean accumulation register %s totals: %w", definition.Name, err)
	}
	return nil
}

func accumulationZeroTotalDelete(definition AccumulationRegisterDefinition) string {
	table, _ := PhysicalAccumulationRegisterTotalsTable(definition.ID)
	conditions := []string{"total_period = $1", "totals_split = $2", "dimension_key = $3"}
	for _, resource := range definition.Resources {
		column, _ := PhysicalAttributeColumn(resource.ID)
		conditions = append(conditions, pgx.Identifier{column}.Sanitize()+" = 0")
	}
	return "DELETE FROM " + qualifiedCatalogTable(table) + " WHERE " + strings.Join(conditions, " AND ")
}

// totalRowInsertArguments is what the insert takes: the same, with the
// dimensions in between, because a new row has to be given them.
func (repository *AccumulationRegisterRepository) totalRowInsertArguments(definition AccumulationRegisterDefinition, delta *accumulationTotalDelta) ([]any, error) {
	arguments := []any{delta.period, delta.dimensionKey}
	for _, dimension := range definition.Dimensions {
		value, present := delta.dimensions[dimension.ID]
		if !present {
			arguments = append(arguments, nil)
			continue
		}
		storage, _ := repository.catalog.attributeStorage(dimension.Types)
		encoded, err := databaseAttributeValue(storage, value)
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, encoded)
	}
	return repository.appendTotalResources(definition, delta, arguments)
}

func (repository *AccumulationRegisterRepository) appendTotalResources(definition AccumulationRegisterDefinition, delta *accumulationTotalDelta, arguments []any) ([]any, error) {
	for _, resource := range definition.Resources {
		numberType, err := repository.catalog.accumulationResourceType(resource)
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, delta.resources[resource.ID].FloatString(numberType.Scale))
	}
	return arguments, nil
}

// RebuildTotals restores all totals from primary movement rows - ПересчитатьИтоги.
//
// It is guarded by its own right rather than by write access to the register:
// rebuilding reads every movement there has ever been and replaces the numbers
// every balance query answers from. Someone allowed to record movements is not
// by that fact someone allowed to recompute the whole register's totals.
func (repository *AccumulationRegisterRepository) RebuildTotals(ctx context.Context, name string) error {
	definition, ok := repository.catalog.AccumulationRegisterDefinition(name)
	if !ok {
		return fmt.Errorf("unknown accumulation register %q", name)
	}
	if err := requireObject(ctx, definition.ID, PermissionTotalsControl); err != nil {
		return err
	}
	return repository.manageTotals(ctx, definition, func(transactionContext context.Context, transaction pgx.Tx, state balanceTotalsState) error {
		return rebuildRegisterTotals(transactionContext, transaction, definition, state)
	})
}

// rebuildTurnoverTotals restores the monthly turnovers of a turnover register.
func rebuildTurnoverTotals(ctx context.Context, transaction pgx.Tx, definition AccumulationRegisterDefinition) error {
	movementTable, _ := PhysicalAccumulationRegisterTable(definition.ID)
	totalsTable, _ := PhysicalAccumulationRegisterTotalsTable(definition.ID)
	if _, err := transaction.Exec(ctx, "DELETE FROM "+qualifiedCatalogTable(totalsTable)); err != nil {
		return fmt.Errorf("clear accumulation register %s totals: %w", definition.Name, err)
	}
	columns := []string{"total_period", "totals_split", "dimension_key"}
	monthExpression := "date_trunc('month', period AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"
	// A rebuild collapses the split rows into one, whatever mode the
	// register is in - the prototype folds them on recalculation too.
	// Grouping by the split stored on the movements would
	// reproduce the old spread instead, and a register switched out of
	// splitting would keep its multiplied rows for ever.
	//
	// Later writes spread themselves again if the mode says so, and that
	// costs nothing: a change to a movement written before the rebuild
	// subtracts its delta in the row its recorder belongs to, which is
	// then a row of its own, and reading sums the two.
	selects := []string{monthExpression, "0", "dimension_key"}
	groups := []string{monthExpression, "dimension_key"}
	for _, dimension := range definition.Dimensions {
		column, _ := PhysicalAttributeColumn(dimension.ID)
		quoted := pgx.Identifier{column}.Sanitize()
		columns, selects, groups = append(columns, column), append(selects, quoted), append(groups, quoted)
	}
	for _, resource := range definition.Resources {
		column, _ := PhysicalAttributeColumn(resource.ID)
		quoted := pgx.Identifier{column}.Sanitize()
		columns = append(columns, column)
		selects = append(selects, "SUM("+quoted+")")
	}
	statement := "INSERT INTO " + qualifiedCatalogTable(totalsTable) + " (" + strings.Join(quoteCatalogColumns(columns), ", ") + ") SELECT " +
		strings.Join(selects, ", ") + " FROM " + qualifiedCatalogTable(movementTable) + " WHERE active = true GROUP BY " + strings.Join(groups, ", ")
	if _, err := transaction.Exec(ctx, statement); err != nil {
		return fmt.Errorf("rebuild accumulation register %s totals: %w", definition.Name, err)
	}
	zeroConditions := make([]string, 0, len(definition.Resources))
	for _, resource := range definition.Resources {
		column, _ := PhysicalAttributeColumn(resource.ID)
		zeroConditions = append(zeroConditions, pgx.Identifier{column}.Sanitize()+" = 0")
	}
	if len(zeroConditions) != 0 {
		if _, err := transaction.Exec(ctx, "DELETE FROM "+qualifiedCatalogTable(totalsTable)+" WHERE "+strings.Join(zeroConditions, " AND ")); err != nil {
			return fmt.Errorf("clean accumulation register %s totals: %w", definition.Name, err)
		}
	}
	return nil
}

func (repository *AccumulationRegisterRepository) Balances(ctx context.Context, name string, period time.Time, dimensions map[uuid.UUID]Value) ([]AccumulationRegisterTotalsRow, error) {
	definition, ok := repository.catalog.AccumulationRegisterDefinition(name)
	if !ok {
		return nil, fmt.Errorf("unknown accumulation register %q", name)
	}
	if definition.Kind != AccumulationRegisterBalance {
		return nil, fmt.Errorf("accumulation register %s does not store balances", definition.Name)
	}
	period, err := normalizeAccumulationPeriod(period)
	if err != nil {
		return nil, err
	}
	return repository.balanceAt(ctx, definition, period, dimensions)
}

func (repository *AccumulationRegisterRepository) Turnovers(ctx context.Context, name string, begin, end time.Time, dimensions map[uuid.UUID]Value) ([]AccumulationRegisterTotalsRow, error) {
	definition, ok := repository.catalog.AccumulationRegisterDefinition(name)
	if !ok {
		return nil, fmt.Errorf("unknown accumulation register %q", name)
	}
	begin, err := normalizeAccumulationPeriod(begin)
	if err != nil {
		return nil, err
	}
	end, err = normalizeAccumulationPeriod(end)
	if err != nil {
		return nil, err
	}
	if end.Before(begin) {
		return nil, fmt.Errorf("accumulation register turnover end precedes begin")
	}
	query, err := queryData(ctx, repository.pool)
	if err != nil {
		return nil, err
	}
	if err := repository.requireReadableTotals(ctx, query, definition); err != nil {
		return nil, err
	}
	return repository.queryAggregates(ctx, definition, &begin, &end, dimensions)
}

func (repository *AccumulationRegisterRepository) BalancesAndTurnovers(ctx context.Context, name string, begin, end time.Time, dimensions map[uuid.UUID]Value) ([]AccumulationRegisterTotalsRow, error) {
	definition, ok := repository.catalog.AccumulationRegisterDefinition(name)
	if !ok {
		return nil, fmt.Errorf("unknown accumulation register %q", name)
	}
	if definition.Kind != AccumulationRegisterBalance {
		return nil, fmt.Errorf("accumulation register %s does not store balances", definition.Name)
	}
	begin, err := normalizeAccumulationPeriod(begin)
	if err != nil {
		return nil, err
	}
	end, err = normalizeAccumulationPeriod(end)
	if err != nil {
		return nil, err
	}
	if end.Before(begin) {
		return nil, fmt.Errorf("accumulation register turnover end precedes begin")
	}
	return repository.queryBalancesAndTurnovers(ctx, definition, begin, end, dimensions)
}

func (repository *AccumulationRegisterRepository) queryBalancesAndTurnovers(ctx context.Context, definition AccumulationRegisterDefinition, begin, end time.Time, dimensions map[uuid.UUID]Value) ([]AccumulationRegisterTotalsRow, error) {
	filter, err := repository.normalizeDimensions(definition, dimensions)
	if err != nil {
		return nil, err
	}
	movementTable, _ := PhysicalAccumulationRegisterTable(definition.ID)
	dimensionColumns := make([]string, len(definition.Dimensions))
	turnoverSelects, outerSelects, groups := []string{}, []string{}, []string{}
	for index, dimension := range definition.Dimensions {
		column, _ := PhysicalAttributeColumn(dimension.ID)
		quoted := pgx.Identifier{column}.Sanitize()
		dimensionColumns[index] = column
		turnoverSelects, outerSelects, groups = append(turnoverSelects, quoted), append(outerSelects, quoted), append(groups, quoted)
	}
	aliases := func(index int) (string, string, string, string) {
		return pgx.Identifier{fmt.Sprintf("r%d_opening", index)}.Sanitize(), pgx.Identifier{fmt.Sprintf("r%d_receipt", index)}.Sanitize(),
			pgx.Identifier{fmt.Sprintf("r%d_expense", index)}.Sanitize(), pgx.Identifier{fmt.Sprintf("r%d_net", index)}.Sanitize()
	}
	for index, resource := range definition.Resources {
		column, _ := PhysicalAttributeColumn(resource.ID)
		quoted := pgx.Identifier{column}.Sanitize()
		amount := "COALESCE(" + quoted + ", 0)"
		signed := "CASE WHEN movement_kind = 1 THEN " + amount + " ELSE -" + amount + " END"
		openingAlias, receiptAlias, expenseAlias, netAlias := aliases(index)
		turnoverSelects = append(turnoverSelects, "0::numeric AS "+openingAlias, "CASE WHEN movement_kind = 1 THEN "+amount+" ELSE 0 END AS "+receiptAlias, "CASE WHEN movement_kind = 2 THEN "+amount+" ELSE 0 END AS "+expenseAlias, signed+" AS "+netAlias)
		outerSelects = append(outerSelects, "SUM("+openingAlias+") AS "+openingAlias, "SUM("+receiptAlias+") AS "+receiptAlias, "SUM("+expenseAlias+") AS "+expenseAlias, "SUM("+netAlias+") AS "+netAlias)
	}
	arguments := []any{}
	// The opening balance is the balance before the first moment of the
	// period, read the same way as a balance at a date; the turnovers of the
	// period come from its movements.
	restriction, err := repository.restrictedMovements(ctx, definition, &arguments)
	if err != nil {
		return nil, err
	}
	with, parts := balanceParts(definition, begin, false, time.Now(), &arguments, restriction, func(index int, amount string) []string {
		openingAlias, receiptAlias, expenseAlias, netAlias := aliases(index)
		return []string{amount + " AS " + openingAlias, "0::numeric AS " + receiptAlias, "0::numeric AS " + expenseAlias, "0::numeric AS " + netAlias}
	})
	from, to := placeholder(&arguments, begin), placeholder(&arguments, end)
	parts = append(parts, "SELECT "+strings.Join(turnoverSelects, ", ")+" FROM "+qualifiedCatalogTable(movementTable)+" WHERE active = true AND period >= "+from+" AND period <= "+to+restriction)
	conditions, err := repository.balanceFilterConditions(definition, filter, &arguments)
	if err != nil {
		return nil, err
	}
	inner := with + "SELECT " + strings.Join(outerSelects, ", ") + " FROM (" + strings.Join(parts, " UNION ALL ") + ") AS source"
	if len(conditions) != 0 {
		inner += " WHERE " + strings.Join(conditions, " AND ")
	}
	if len(groups) != 0 {
		inner += " GROUP BY " + strings.Join(groups, ", ")
	}
	inner += " ORDER BY "
	if len(groups) == 0 {
		inner += "1"
	} else {
		inner += strings.Join(groups, ", ")
	}
	inner += " LIMIT " + fmt.Sprint(maxAccumulationRegisterRecords+1)
	query, err := queryData(ctx, repository.pool)
	if err != nil {
		return nil, err
	}
	if err := repository.requireReadableTotals(ctx, query, definition); err != nil {
		return nil, err
	}
	rows, err := query.Query(ctx, "SELECT to_jsonb(item) FROM ("+inner+") AS item", arguments...)
	if err != nil {
		return nil, recordDataError(ctx, repository.pool, fmt.Errorf("read accumulation register %s balances and turnovers: %w", definition.Name, err))
	}
	defer rows.Close()
	result := make([]AccumulationRegisterTotalsRow, 0)
	memory := uint64(512)
	for rows.Next() {
		if len(result) >= maxAccumulationRegisterRecords {
			return nil, fmt.Errorf("accumulation register %s balances and turnovers exceed row limit", definition.Name)
		}
		var encoded []byte
		if err := rows.Scan(&encoded); err != nil {
			return nil, err
		}
		if uint64(len(encoded))+256 > maxAccumulationRegisterMemory-memory {
			return nil, fmt.Errorf("accumulation register %s balances and turnovers exceed memory limit", definition.Name)
		}
		memory += uint64(len(encoded)) + 256
		item, err := repository.decodeBalancesAndTurnovers(definition, dimensionColumns, encoded)
		if err != nil {
			return nil, err
		}
		if !accumulationAggregateZero(definition.Resources, item) {
			result = append(result, item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, recordDataError(ctx, repository.pool, err)
	}
	return result, nil
}

func (repository *AccumulationRegisterRepository) decodeBalancesAndTurnovers(definition AccumulationRegisterDefinition, dimensionColumns []string, encoded []byte) (AccumulationRegisterTotalsRow, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return AccumulationRegisterTotalsRow{}, err
	}
	result := AccumulationRegisterTotalsRow{Dimensions: map[uuid.UUID]Value{}, Opening: map[uuid.UUID]Value{}, Receipt: map[uuid.UUID]Value{}, Expense: map[uuid.UUID]Value{}, Turnover: map[uuid.UUID]Value{}, Closing: map[uuid.UUID]Value{}}
	for index, dimension := range definition.Dimensions {
		raw := fields[dimensionColumns[index]]
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		storage, _ := repository.catalog.attributeStorage(dimension.Types)
		value, err := decodeDatabaseAttribute(storage, raw)
		if err != nil {
			return result, err
		}
		value, err = repository.catalog.normalizeTypes("accumulation register dimension "+dimension.Name, dimension.Types, value)
		if err != nil {
			return result, err
		}
		result.Dimensions[dimension.ID] = value
	}
	for index, resource := range definition.Resources {
		opening, err := accumulationAggregateNumber(fields[fmt.Sprintf("r%d_opening", index)])
		if err != nil {
			return result, err
		}
		receipt, err := accumulationAggregateNumber(fields[fmt.Sprintf("r%d_receipt", index)])
		if err != nil {
			return result, err
		}
		expense, err := accumulationAggregateNumber(fields[fmt.Sprintf("r%d_expense", index)])
		if err != nil {
			return result, err
		}
		turnover, err := accumulationAggregateNumber(fields[fmt.Sprintf("r%d_net", index)])
		if err != nil {
			return result, err
		}
		left, _ := bslnumber.Parse(opening.Data)
		right, _ := bslnumber.Parse(turnover.Data)
		closing, err := left.Add(right)
		if err != nil {
			return result, err
		}
		result.Opening[resource.ID], result.Receipt[resource.ID], result.Expense[resource.ID], result.Turnover[resource.ID], result.Closing[resource.ID] = opening, receipt, expense, turnover, Value{Kind: NumberType, Data: closing.String()}
	}
	return result, nil
}

func accumulationAggregateZero(resources []Attribute, aggregate AccumulationRegisterTotalsRow) bool {
	for _, resource := range resources {
		for _, values := range []map[uuid.UUID]Value{aggregate.Opening, aggregate.Receipt, aggregate.Expense, aggregate.Turnover, aggregate.Closing} {
			value := values[resource.ID]
			if value.Data == "" {
				continue
			}
			number, err := bslnumber.Parse(value.Data)
			if err != nil || !number.IsZero() {
				return false
			}
		}
	}
	return true
}

// maxRestrictedBalanceMovements bounds the work a restricted aggregate may do.
// Reading pre-summed totals is cheap and predictable; summing movements live is
// neither, so a restricted caller gets a clear refusal instead of a query that
// may never return - the same bargain the paged lists and the query row limit
// already make. The number is provisional: it was chosen without measuring real
// registers and should be revisited against actual volumes.
const maxRestrictedBalanceMovements = 200_000

// restrictedMovements returns the predicate that narrows movement rows for this
// caller, or "" when nothing is restricted. It also refuses up front when the
// live summation the restriction forces would be unbounded.
func (repository *AccumulationRegisterRepository) restrictedMovements(ctx context.Context, definition AccumulationRegisterDefinition, arguments *[]any) (string, error) {
	// The measuring query is rendered into its own argument list: it does not
	// reference the caller's period bounds, and binding parameters a statement
	// never mentions leaves their type undetermined.
	countArguments := []any{}
	countRestriction, err := readRowPredicate(ctx, repository.catalog, definition.ID, accumulationRegisterPolicyColumn(definition), &countArguments)
	if err != nil || countRestriction == "" {
		return "", err
	}
	table, _ := PhysicalAccumulationRegisterTable(definition.ID)
	query, err := queryData(ctx, repository.pool)
	if err != nil {
		return "", err
	}
	var movements int64
	statement := "SELECT count(*) FROM (SELECT 1 FROM " + qualifiedCatalogTable(table) + " WHERE active = true" + countRestriction +
		fmt.Sprintf(" LIMIT %d) AS bounded", maxRestrictedBalanceMovements+1)
	if err := query.QueryRow(ctx, statement, countArguments...).Scan(&movements); err != nil {
		return "", recordDataError(ctx, repository.pool, fmt.Errorf("measure accumulation register %s movements: %w", definition.Name, err))
	}
	if movements > maxRestrictedBalanceMovements {
		return "", fmt.Errorf("accumulation register %s has a row-level restriction, and computing it from more than %d movements is refused", definition.Name, maxRestrictedBalanceMovements)
	}
	return readRowPredicate(ctx, repository.catalog, definition.ID, accumulationRegisterPolicyColumn(definition), arguments)
}

func (repository *AccumulationRegisterRepository) balanceAt(ctx context.Context, definition AccumulationRegisterDefinition, period time.Time, dimensions map[uuid.UUID]Value) ([]AccumulationRegisterTotalsRow, error) {
	statement, arguments, dimensionColumns, err := repository.balanceStatement(ctx, definition, period, time.Now(), dimensions)
	if err != nil {
		return nil, err
	}
	query, err := queryData(ctx, repository.pool)
	if err != nil {
		return nil, err
	}
	if err := repository.requireReadableTotals(ctx, query, definition); err != nil {
		return nil, err
	}
	rows, err := query.Query(ctx, statement, arguments...)
	if err != nil {
		return nil, recordDataError(ctx, repository.pool, fmt.Errorf("read accumulation register %s balances: %w", definition.Name, err))
	}
	defer rows.Close()
	result := make([]AccumulationRegisterTotalsRow, 0)
	memory := uint64(512)
	for rows.Next() {
		if len(result) >= maxAccumulationRegisterRecords {
			return nil, fmt.Errorf("accumulation register %s balances exceed row limit", definition.Name)
		}
		var encoded []byte
		if err := rows.Scan(&encoded); err != nil {
			return nil, err
		}
		if uint64(len(encoded))+256 > maxAccumulationRegisterMemory-memory {
			return nil, fmt.Errorf("accumulation register %s balances exceed memory limit", definition.Name)
		}
		memory += uint64(len(encoded)) + 256
		item, err := repository.decodeAggregate(definition, dimensionColumns, encoded)
		if err != nil {
			return nil, err
		}
		if !accumulationResourcesZero(definition.Resources, item.Turnover) {
			result = append(result, item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, recordDataError(ctx, repository.pool, err)
	}
	return result, nil
}

// balanceStatement is the statement a balance at a date is read by, apart from
// its execution so that its plan can be asked about.
func (repository *AccumulationRegisterRepository) balanceStatement(ctx context.Context, definition AccumulationRegisterDefinition, period, now time.Time, dimensions map[uuid.UUID]Value) (string, []any, []string, error) {
	filter, err := repository.normalizeDimensions(definition, dimensions)
	if err != nil {
		return "", nil, nil, err
	}
	dimensionColumns := make([]string, len(definition.Dimensions))
	outerSelects, groups := []string{}, []string{}
	for index, dimension := range definition.Dimensions {
		column, _ := PhysicalAttributeColumn(dimension.ID)
		quoted := pgx.Identifier{column}.Sanitize()
		dimensionColumns[index] = column
		outerSelects, groups = append(outerSelects, quoted), append(groups, quoted)
	}
	for index := range definition.Resources {
		alias := pgx.Identifier{fmt.Sprintf("r%d_net", index)}.Sanitize()
		outerSelects = append(outerSelects, "SUM("+alias+") AS "+alias)
	}
	arguments := []any{}
	restriction, err := repository.restrictedMovements(ctx, definition, &arguments)
	if err != nil {
		return "", nil, nil, err
	}
	with, parts := balanceParts(definition, period, true, now, &arguments, restriction, func(index int, amount string) []string {
		return []string{amount + " AS " + pgx.Identifier{fmt.Sprintf("r%d_net", index)}.Sanitize()}
	})
	conditions, err := repository.balanceFilterConditions(definition, filter, &arguments)
	if err != nil {
		return "", nil, nil, err
	}
	inner := with + "SELECT " + strings.Join(outerSelects, ", ") + " FROM (" + strings.Join(parts, " UNION ALL ") + ") AS source"
	if len(conditions) != 0 {
		inner += " WHERE " + strings.Join(conditions, " AND ")
	}
	if len(groups) != 0 {
		inner += " GROUP BY " + strings.Join(groups, ", ")
	}
	inner += " ORDER BY "
	if len(groups) == 0 {
		inner += "1"
	} else {
		inner += strings.Join(groups, ", ")
	}
	inner += " LIMIT " + fmt.Sprint(maxAccumulationRegisterRecords+1)
	return "SELECT to_jsonb(item) FROM (" + inner + ") AS item", arguments, dimensionColumns, nil
}

// balanceFilterConditions are the equality filters on dimensions, applied to
// every part of the balance at once.
func (repository *AccumulationRegisterRepository) balanceFilterConditions(definition AccumulationRegisterDefinition, filter map[uuid.UUID]Value, arguments *[]any) ([]string, error) {
	var conditions []string
	for _, dimension := range definition.Dimensions {
		value, present := filter[dimension.ID]
		if !present {
			continue
		}
		storage, _ := repository.catalog.attributeStorage(dimension.Types)
		encoded, err := databaseAttributeValue(storage, value)
		if err != nil {
			return nil, err
		}
		column, _ := PhysicalAttributeColumn(dimension.ID)
		conditions = append(conditions, pgx.Identifier{column}.Sanitize()+" = "+placeholder(arguments, encoded))
	}
	return conditions, nil
}

// requireReadableTotals refuses a read the totals cannot answer: totals
// switched off, or still in the old format. It is checked apart from the
// statement, and that is safe - the format only ever changes once, to the
// current one, and a switch of the totals seen late by this check is seen on
// time by the statement, which then sums the movements and answers right.
func (repository *AccumulationRegisterRepository) requireReadableTotals(ctx context.Context, query dataQueryer, definition AccumulationRegisterDefinition) error {
	state, err := readBalanceTotalsState(ctx, query, definition.ID)
	if err != nil {
		return recordDataError(ctx, repository.pool, err)
	}
	if !state.use {
		return fmt.Errorf("accumulation register %s: %w", definition.Name, ErrAccumulationTotalsDisabled)
	}
	return requireBalanceTotalsFormat(ctx, query, nil, definition, state)
}

func accumulationResourcesZero(resources []Attribute, values map[uuid.UUID]Value) bool {
	for _, resource := range resources {
		value := values[resource.ID]
		number, err := bslnumber.Parse(value.Data)
		if err != nil || !number.IsZero() {
			return false
		}
	}
	return true
}

func (repository *AccumulationRegisterRepository) queryAggregates(ctx context.Context, definition AccumulationRegisterDefinition, begin, end *time.Time, dimensions map[uuid.UUID]Value) ([]AccumulationRegisterTotalsRow, error) {
	filter, err := repository.normalizeDimensions(definition, dimensions)
	if err != nil {
		return nil, err
	}
	table, _ := PhysicalAccumulationRegisterTable(definition.ID)
	dimensionColumns := make([]string, len(definition.Dimensions))
	selects := make([]string, 0, len(definition.Dimensions)+len(definition.Resources)*3)
	groups := make([]string, 0, len(definition.Dimensions))
	for index, dimension := range definition.Dimensions {
		column, _ := PhysicalAttributeColumn(dimension.ID)
		quoted := pgx.Identifier{column}.Sanitize()
		dimensionColumns[index], selects, groups = column, append(selects, quoted), append(groups, quoted)
	}
	for index, resource := range definition.Resources {
		column, _ := PhysicalAttributeColumn(resource.ID)
		quoted := pgx.Identifier{column}.Sanitize()
		if definition.Kind == AccumulationRegisterBalance {
			selects = append(selects,
				"SUM(CASE WHEN movement_kind = 1 THEN "+quoted+" ELSE -"+quoted+" END) AS "+pgx.Identifier{fmt.Sprintf("r%d_net", index)}.Sanitize(),
				"SUM(CASE WHEN movement_kind = 1 THEN "+quoted+" ELSE 0 END) AS "+pgx.Identifier{fmt.Sprintf("r%d_receipt", index)}.Sanitize(),
				"SUM(CASE WHEN movement_kind = 2 THEN "+quoted+" ELSE 0 END) AS "+pgx.Identifier{fmt.Sprintf("r%d_expense", index)}.Sanitize())
		} else {
			selects = append(selects, "SUM("+quoted+") AS "+pgx.Identifier{fmt.Sprintf("r%d_net", index)}.Sanitize())
		}
	}
	conditions := []string{"active = true"}
	arguments := []any{}
	add := func(condition string, value any) {
		arguments = append(arguments, value)
		conditions = append(conditions, condition+fmt.Sprintf(" $%d", len(arguments)))
	}
	if begin != nil {
		add("period >=", *begin)
	}
	if end != nil {
		add("period <=", *end)
	}
	for _, dimension := range definition.Dimensions {
		value, present := filter[dimension.ID]
		if !present {
			continue
		}
		storage, _ := repository.catalog.attributeStorage(dimension.Types)
		encoded, err := databaseAttributeValue(storage, value)
		if err != nil {
			return nil, err
		}
		column, _ := PhysicalAttributeColumn(dimension.ID)
		add(pgx.Identifier{column}.Sanitize()+" =", encoded)
	}
	inner := "SELECT " + strings.Join(selects, ", ") + " FROM " + qualifiedCatalogTable(table) + " WHERE " + strings.Join(conditions, " AND ")
	if len(groups) != 0 {
		inner += " GROUP BY " + strings.Join(groups, ", ")
	}
	inner += " ORDER BY "
	if len(groups) == 0 {
		inner += "1"
	} else {
		inner += strings.Join(groups, ", ")
	}
	inner += " LIMIT " + fmt.Sprint(maxAccumulationRegisterRecords+1)
	query, err := queryData(ctx, repository.pool)
	if err != nil {
		return nil, err
	}
	rows, err := query.Query(ctx, "SELECT to_jsonb(item) FROM ("+inner+") AS item", arguments...)
	if err != nil {
		return nil, recordDataError(ctx, repository.pool, fmt.Errorf("read accumulation register %s virtual table: %w", definition.Name, err))
	}
	defer rows.Close()
	result := make([]AccumulationRegisterTotalsRow, 0)
	memory := uint64(512)
	for rows.Next() {
		if len(result) >= maxAccumulationRegisterRecords {
			return nil, fmt.Errorf("accumulation register %s virtual table exceeds row limit", definition.Name)
		}
		var encoded []byte
		if err := rows.Scan(&encoded); err != nil {
			return nil, err
		}
		if uint64(len(encoded)) > maxAccumulationRegisterMemory-memory {
			return nil, fmt.Errorf("accumulation register %s virtual table exceeds memory limit", definition.Name)
		}
		memory += uint64(len(encoded)) + 256
		item, err := repository.decodeAggregate(definition, dimensionColumns, encoded)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, recordDataError(ctx, repository.pool, err)
	}
	return result, nil
}

func (repository *AccumulationRegisterRepository) normalizeDimensions(definition AccumulationRegisterDefinition, dimensions map[uuid.UUID]Value) (map[uuid.UUID]Value, error) {
	result := mapsCloneValues(dimensions)
	known := map[uuid.UUID]Attribute{}
	for _, dimension := range definition.Dimensions {
		known[dimension.ID] = dimension.Attribute
		value, present := result[dimension.ID]
		if !present {
			continue
		}
		normalized, err := repository.catalog.normalizeTypes("accumulation register "+definition.Name+" dimension "+dimension.Name, dimension.Types, value)
		if err != nil {
			return nil, err
		}
		result[dimension.ID] = normalized
	}
	for id := range result {
		if _, ok := known[id]; !ok {
			return nil, fmt.Errorf("accumulation register %s filter contains unknown dimension %s", definition.Name, id)
		}
	}
	return result, nil
}

func (repository *AccumulationRegisterRepository) decodeAggregate(definition AccumulationRegisterDefinition, dimensionColumns []string, encoded []byte) (AccumulationRegisterTotalsRow, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return AccumulationRegisterTotalsRow{}, err
	}
	result := AccumulationRegisterTotalsRow{Dimensions: map[uuid.UUID]Value{}, Receipt: map[uuid.UUID]Value{}, Expense: map[uuid.UUID]Value{}, Turnover: map[uuid.UUID]Value{}}
	for index, dimension := range definition.Dimensions {
		raw := fields[dimensionColumns[index]]
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		storage, _ := repository.catalog.attributeStorage(dimension.Types)
		value, err := decodeDatabaseAttribute(storage, raw)
		if err != nil {
			return result, err
		}
		value, err = repository.catalog.normalizeTypes("accumulation register dimension "+dimension.Name, dimension.Types, value)
		if err != nil {
			return result, err
		}
		result.Dimensions[dimension.ID] = value
	}
	for index, resource := range definition.Resources {
		net, err := accumulationAggregateNumber(fields[fmt.Sprintf("r%d_net", index)])
		if err != nil {
			return result, fmt.Errorf("accumulation register %s resource %s: %w", definition.Name, resource.Name, err)
		}
		result.Turnover[resource.ID] = net
		if definition.Kind == AccumulationRegisterBalance {
			receipt, err := accumulationAggregateNumber(fields[fmt.Sprintf("r%d_receipt", index)])
			if err != nil {
				return result, err
			}
			expense, err := accumulationAggregateNumber(fields[fmt.Sprintf("r%d_expense", index)])
			if err != nil {
				return result, err
			}
			result.Receipt[resource.ID], result.Expense[resource.ID] = receipt, expense
		}
	}
	return result, nil
}

func accumulationAggregateNumber(raw json.RawMessage) (Value, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return Value{Kind: NumberType, Data: "0"}, nil
	}
	value, err := bslnumber.Parse(string(raw))
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: NumberType, Data: value.String()}, nil
}

func mergeAccumulationAggregates(definition AccumulationRegisterDefinition, opening, turnover []AccumulationRegisterTotalsRow) ([]AccumulationRegisterTotalsRow, error) {
	combined := map[string]*AccumulationRegisterTotalsRow{}
	add := func(source AccumulationRegisterTotalsRow, isOpening bool) error {
		key := accumulationDimensionKey(definition, source.Dimensions)
		target := combined[key]
		if target == nil {
			target = &AccumulationRegisterTotalsRow{Dimensions: mapsCloneValues(source.Dimensions), Opening: map[uuid.UUID]Value{}, Receipt: map[uuid.UUID]Value{}, Expense: map[uuid.UUID]Value{}, Turnover: map[uuid.UUID]Value{}, Closing: map[uuid.UUID]Value{}}
			combined[key] = target
		}
		for _, resource := range definition.Resources {
			if isOpening {
				target.Opening[resource.ID] = source.Turnover[resource.ID]
				continue
			}
			target.Receipt[resource.ID], target.Expense[resource.ID], target.Turnover[resource.ID] = source.Receipt[resource.ID], source.Expense[resource.ID], source.Turnover[resource.ID]
		}
		return nil
	}
	for _, item := range opening {
		if err := add(item, true); err != nil {
			return nil, err
		}
	}
	for _, item := range turnover {
		if err := add(item, false); err != nil {
			return nil, err
		}
	}
	keys := make([]string, 0, len(combined))
	for key := range combined {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	result := make([]AccumulationRegisterTotalsRow, 0, len(keys))
	for _, key := range keys {
		item := combined[key]
		for _, resource := range definition.Resources {
			openingValue := item.Opening[resource.ID]
			if openingValue.Data == "" {
				openingValue = Value{Kind: NumberType, Data: "0"}
				item.Opening[resource.ID] = openingValue
			}
			turnoverValue := item.Turnover[resource.ID]
			if turnoverValue.Data == "" {
				turnoverValue = Value{Kind: NumberType, Data: "0"}
				item.Turnover[resource.ID] = turnoverValue
			}
			left, _ := bslnumber.Parse(openingValue.Data)
			right, _ := bslnumber.Parse(turnoverValue.Data)
			closing, err := left.Add(right)
			if err != nil {
				return nil, err
			}
			item.Closing[resource.ID] = Value{Kind: NumberType, Data: closing.String()}
			if item.Receipt[resource.ID].Data == "" {
				item.Receipt[resource.ID] = Value{Kind: NumberType, Data: "0"}
			}
			if item.Expense[resource.ID].Data == "" {
				item.Expense[resource.ID] = Value{Kind: NumberType, Data: "0"}
			}
		}
		result = append(result, *item)
	}
	return result, nil
}

func accumulationDimensionKey(definition AccumulationRegisterDefinition, dimensions map[uuid.UUID]Value) string {
	hash := sha256.New()
	writeHashPart(hash, definition.ID.String())
	for _, dimension := range definition.Dimensions {
		writeHashPart(hash, dimension.ID.String())
		value, ok := dimensions[dimension.ID]
		if !ok {
			writeHashPart(hash, "<undefined>")
			continue
		}
		writeHashPart(hash, string(value.Kind))
		writeHashPart(hash, value.Data)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func lockAccumulationRegisterWriteGate(ctx context.Context, transaction pgx.Tx, definition AccumulationRegisterDefinition, recorder DocumentReference, lockForUpdate bool) error {
	if transaction == nil {
		return fmt.Errorf("invalid accumulation register lock")
	}
	tableKey := objectLockKey("accumulation-register", definition.ID, definition.ID)
	lockFunction := "pg_advisory_xact_lock_shared"
	if lockForUpdate {
		lockFunction = "pg_advisory_xact_lock"
	}
	if _, err := transaction.Exec(ctx, "SELECT "+lockFunction+"($1)", tableKey); err != nil {
		return fmt.Errorf("lock accumulation register %s: %w", definition.Name, err)
	}
	key := objectLockKey("accumulation-register-recorder", definition.ID, recorder.ObjectID)
	if _, err := transaction.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", key); err != nil {
		return fmt.Errorf("lock accumulation register %s recorder: %w", definition.Name, err)
	}
	return nil
}

func lockAccumulationRegisterDimensions(ctx context.Context, transaction pgx.Tx, definition AccumulationRegisterDefinition, records []*AccumulationRegisterRecord) error {
	keys := make([]int64, 0, len(records))
	seen := map[int64]bool{}
	for _, record := range records {
		if record == nil || !record.Active {
			continue
		}
		key := accumulationDimensionLockKey(definition, record.Dimensions)
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	for _, key := range keys {
		if _, err := transaction.Exec(ctx, "SELECT pg_advisory_xact_lock_shared($1)", key); err != nil {
			return fmt.Errorf("lock accumulation register %s dimensions: %w", definition.Name, err)
		}
	}
	return nil
}

func accumulationDimensionLockKey(definition AccumulationRegisterDefinition, dimensions map[uuid.UUID]Value) int64 {
	hash := sha256.New()
	writeHashPart(hash, "accumulation-register-dimensions")
	writeHashPart(hash, definition.ID.String())
	writeHashPart(hash, accumulationDimensionKey(definition, dimensions))
	return int64(binary.BigEndian.Uint64(hash.Sum(nil)[:8]))
}

func accumulationRegisterWriteError(name string, err error) error {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" {
		return fmt.Errorf("accumulation register %s contains duplicate recorder lines: %w", name, err)
	}
	return fmt.Errorf("write accumulation register %s: %w", name, err)
}

func cloneAccumulationRegisterRecordSet(source *AccumulationRegisterRecordSet) *AccumulationRegisterRecordSet {
	if source == nil {
		return nil
	}
	result := *source
	result.Filter = cloneAccumulationRegisterFilter(source.Filter)
	result.Records = make([]*AccumulationRegisterRecord, len(source.Records))
	for index, record := range source.Records {
		result.Records[index] = cloneAccumulationRegisterRecord(record)
	}
	return &result
}

func cloneAccumulationRegisterFilter(source AccumulationRegisterFilter) AccumulationRegisterFilter {
	result := source
	if source.Recorder != nil {
		recorder := *source.Recorder
		result.Recorder = &recorder
	}
	return result
}

func cloneAccumulationRegisterRecord(source *AccumulationRegisterRecord) *AccumulationRegisterRecord {
	if source == nil {
		return nil
	}
	result := *source
	result.Dimensions = mapsCloneValues(source.Dimensions)
	result.Resources = mapsCloneValues(source.Resources)
	result.Attributes = mapsCloneValues(source.Attributes)
	return &result
}

func accumulationRegisterSetMemory(set *AccumulationRegisterRecordSet, limit uint64) (uint64, bool) {
	if set == nil {
		return 0, true
	}
	size := uint64(512)
	for _, record := range set.Records {
		if record == nil || size > limit {
			return limit, false
		}
		size += accumulationRegisterRecordMemory(record)
		if size > limit {
			return limit, false
		}
	}
	return size, true
}

func accumulationRegisterRecordMemory(record *AccumulationRegisterRecord) uint64 {
	size := uint64(512)
	for _, group := range []map[uuid.UUID]Value{record.Dimensions, record.Resources, record.Attributes} {
		for _, value := range group {
			size += uint64(len(value.Data) + 96)
		}
	}
	return size
}
