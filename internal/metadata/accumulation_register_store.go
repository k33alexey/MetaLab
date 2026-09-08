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
	accumulationTotalsSplitCount   = 16
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

// AccumulationRegisterAggregate is one row returned by a basic virtual table.
type AccumulationRegisterAggregate struct {
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
		if err := repository.insertMovements(transactionContext, transaction, definition, working.Records); err != nil {
			return err
		}
		if err := repository.applyTotalsChanges(transactionContext, transaction, definition, old, working.Records); err != nil {
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
	if !allowedAccumulationRegisterRecorder(definition, *filter.Recorder) {
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
	if !allowedAccumulationRegisterRecorder(definition, record.Recorder) {
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
	record.Dimensions, err = repository.catalog.normalizeAttributes(definition.Name, definition.Dimensions, record.Dimensions)
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

func allowedAccumulationRegisterRecorder(definition AccumulationRegisterDefinition, recorder DocumentReference) bool {
	return !recorder.DocumentID.IsZero() && !recorder.ObjectID.IsZero() && slices.Contains(definition.Recorders, recorder.DocumentID)
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
	columns := []string{"record_id", "dimension_key", "period", "recorder_type", "recorder_ref", "line_no", "active", "totals_split"}
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
			record.Recorder.DocumentID.String(), record.Recorder.ObjectID.String(), record.LineNumber, record.Active, accumulationTotalsSplit(record.Recorder)}
		if definition.Kind == AccumulationRegisterBalance {
			arguments = append(arguments, int16(record.MovementKind))
		}
		for _, group := range []struct {
			fields []Attribute
			values map[uuid.UUID]Value
		}{
			{definition.Dimensions, record.Dimensions}, {definition.Resources, record.Resources}, {definition.Attributes, record.Attributes},
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
		{definition.Dimensions, record.Dimensions}, {definition.Resources, record.Resources}, {definition.Attributes, record.Attributes},
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

type accumulationTotalDelta struct {
	period       time.Time
	split        int16
	dimensionKey string
	dimensions   map[uuid.UUID]Value
	resources    map[uuid.UUID]*big.Rat
}

func (repository *AccumulationRegisterRepository) applyTotalsChanges(ctx context.Context, transaction pgx.Tx, definition AccumulationRegisterDefinition, removed, added []*AccumulationRegisterRecord) error {
	deltas := map[string]*accumulationTotalDelta{}
	accumulate := func(record *AccumulationRegisterRecord, replacementSign int64) error {
		if record == nil || !record.Active {
			return nil
		}
		period := time.Date(record.Period.UTC().Year(), record.Period.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
		split, dimensionKey := accumulationTotalsSplit(record.Recorder), accumulationDimensionKey(definition, record.Dimensions)
		key := period.Format(time.RFC3339Nano) + ":" + fmt.Sprint(split) + ":" + dimensionKey
		delta := deltas[key]
		if delta == nil {
			delta = &accumulationTotalDelta{period: period, split: split, dimensionKey: dimensionKey, dimensions: mapsCloneValues(record.Dimensions), resources: map[uuid.UUID]*big.Rat{}}
			deltas[key] = delta
		}
		direction := replacementSign
		if definition.Kind == AccumulationRegisterBalance && record.MovementKind == AccumulationMovementExpense {
			direction = -direction
		}
		for _, resource := range definition.Resources {
			value := record.Resources[resource.ID]
			amount, ok := new(big.Rat).SetString(value.Data)
			if !ok {
				return fmt.Errorf("accumulation register %s resource %s is not numeric", definition.Name, resource.Name)
			}
			if direction < 0 {
				amount.Neg(amount)
			}
			if delta.resources[resource.ID] == nil {
				delta.resources[resource.ID] = new(big.Rat)
			}
			delta.resources[resource.ID].Add(delta.resources[resource.ID], amount)
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
	for start := 0; start < len(keys); start += 256 {
		end := start + 256
		if end > len(keys) {
			end = len(keys)
		}
		batch := &pgx.Batch{}
		for _, key := range keys[start:end] {
			delta := deltas[key]
			statement, arguments, err := repository.totalUpsert(definition, delta)
			if err != nil {
				return err
			}
			batch.Queue(statement, arguments...)
			batch.Queue(accumulationZeroTotalDelete(definition), arguments[0], arguments[1], arguments[2])
		}
		results := transaction.SendBatch(ctx, batch)
		for range keys[start:end] {
			if _, err := results.Exec(); err != nil {
				_ = results.Close()
				return fmt.Errorf("update accumulation register %s totals: %w", definition.Name, err)
			}
			if _, err := results.Exec(); err != nil {
				_ = results.Close()
				return fmt.Errorf("clean accumulation register %s totals: %w", definition.Name, err)
			}
		}
		if err := results.Close(); err != nil {
			return fmt.Errorf("update accumulation register %s totals: %w", definition.Name, err)
		}
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

func (repository *AccumulationRegisterRepository) totalUpsert(definition AccumulationRegisterDefinition, delta *accumulationTotalDelta) (string, []any, error) {
	table, _ := PhysicalAccumulationRegisterTotalsTable(definition.ID)
	columns := []string{"total_period", "totals_split", "dimension_key"}
	arguments := []any{delta.period, delta.split, delta.dimensionKey}
	for _, dimension := range definition.Dimensions {
		column, _ := PhysicalAttributeColumn(dimension.ID)
		columns = append(columns, column)
		value, present := delta.dimensions[dimension.ID]
		if !present {
			arguments = append(arguments, nil)
			continue
		}
		storage, _ := repository.catalog.attributeStorage(dimension.Types)
		encoded, err := databaseAttributeValue(storage, value)
		if err != nil {
			return "", nil, err
		}
		arguments = append(arguments, encoded)
	}
	updates := make([]string, 0, len(definition.Resources))
	for _, resource := range definition.Resources {
		column, _ := PhysicalAttributeColumn(resource.ID)
		columns = append(columns, column)
		numberType, err := repository.catalog.accumulationResourceType(resource)
		if err != nil {
			return "", nil, err
		}
		scale := numberType.Scale
		arguments = append(arguments, delta.resources[resource.ID].FloatString(scale))
		quoted := pgx.Identifier{column}.Sanitize()
		updates = append(updates, quoted+" = "+pgx.Identifier{table}.Sanitize()+"."+quoted+" + EXCLUDED."+quoted)
	}
	quotedColumns := quoteCatalogColumns(columns)
	placeholders := make([]string, len(arguments))
	for index := range placeholders {
		placeholders[index] = fmt.Sprintf("$%d", index+1)
	}
	statement := "INSERT INTO " + qualifiedCatalogTable(table) + " (" + strings.Join(quotedColumns, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ") " +
		"ON CONFLICT (total_period, totals_split, dimension_key) DO UPDATE SET " + strings.Join(updates, ", ")
	return statement, arguments, nil
}

// RebuildTotals restores all monthly totals from primary movement rows.
func (repository *AccumulationRegisterRepository) RebuildTotals(ctx context.Context, name string) error {
	definition, ok := repository.catalog.AccumulationRegisterDefinition(name)
	if !ok {
		return fmt.Errorf("unknown accumulation register %q", name)
	}
	return runDataTransaction(ctx, repository.pool, nil, func(transactionContext context.Context, transaction pgx.Tx) error {
		tableKey := objectLockKey("accumulation-register", definition.ID, definition.ID)
		if _, err := transaction.Exec(transactionContext, "SELECT pg_advisory_xact_lock($1)", tableKey); err != nil {
			return err
		}
		movementTable, _ := PhysicalAccumulationRegisterTable(definition.ID)
		totalsTable, _ := PhysicalAccumulationRegisterTotalsTable(definition.ID)
		if _, err := transaction.Exec(transactionContext, "DELETE FROM "+qualifiedCatalogTable(totalsTable)); err != nil {
			return fmt.Errorf("clear accumulation register %s totals: %w", definition.Name, err)
		}
		columns := []string{"total_period", "totals_split", "dimension_key"}
		monthExpression := "date_trunc('month', period AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"
		selects := []string{monthExpression, "totals_split", "dimension_key"}
		groups := []string{monthExpression, "totals_split", "dimension_key"}
		for _, dimension := range definition.Dimensions {
			column, _ := PhysicalAttributeColumn(dimension.ID)
			quoted := pgx.Identifier{column}.Sanitize()
			columns, selects, groups = append(columns, column), append(selects, quoted), append(groups, quoted)
		}
		for _, resource := range definition.Resources {
			column, _ := PhysicalAttributeColumn(resource.ID)
			quoted := pgx.Identifier{column}.Sanitize()
			columns = append(columns, column)
			expression := "SUM(" + quoted + ")"
			if definition.Kind == AccumulationRegisterBalance {
				expression = "SUM(CASE WHEN movement_kind = 1 THEN " + quoted + " ELSE -" + quoted + " END)"
			}
			selects = append(selects, expression)
		}
		statement := "INSERT INTO " + qualifiedCatalogTable(totalsTable) + " (" + strings.Join(quoteCatalogColumns(columns), ", ") + ") SELECT " +
			strings.Join(selects, ", ") + " FROM " + qualifiedCatalogTable(movementTable) + " WHERE active = true GROUP BY " + strings.Join(groups, ", ")
		if _, err := transaction.Exec(transactionContext, statement); err != nil {
			return fmt.Errorf("rebuild accumulation register %s totals: %w", definition.Name, err)
		}
		zeroConditions := make([]string, 0, len(definition.Resources))
		for _, resource := range definition.Resources {
			column, _ := PhysicalAttributeColumn(resource.ID)
			zeroConditions = append(zeroConditions, pgx.Identifier{column}.Sanitize()+" = 0")
		}
		if _, err := transaction.Exec(transactionContext, "DELETE FROM "+qualifiedCatalogTable(totalsTable)+" WHERE "+strings.Join(zeroConditions, " AND ")); err != nil {
			return fmt.Errorf("clean accumulation register %s totals: %w", definition.Name, err)
		}
		return nil
	})
}

func (repository *AccumulationRegisterRepository) Balances(ctx context.Context, name string, period time.Time, dimensions map[uuid.UUID]Value) ([]AccumulationRegisterAggregate, error) {
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

func (repository *AccumulationRegisterRepository) Turnovers(ctx context.Context, name string, begin, end time.Time, dimensions map[uuid.UUID]Value) ([]AccumulationRegisterAggregate, error) {
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
	return repository.queryAggregates(ctx, definition, &begin, &end, dimensions)
}

func (repository *AccumulationRegisterRepository) BalancesAndTurnovers(ctx context.Context, name string, begin, end time.Time, dimensions map[uuid.UUID]Value) ([]AccumulationRegisterAggregate, error) {
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

func (repository *AccumulationRegisterRepository) queryBalancesAndTurnovers(ctx context.Context, definition AccumulationRegisterDefinition, begin, end time.Time, dimensions map[uuid.UUID]Value) ([]AccumulationRegisterAggregate, error) {
	filter, err := repository.normalizeDimensions(definition, dimensions)
	if err != nil {
		return nil, err
	}
	month := time.Date(begin.UTC().Year(), begin.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	movementTable, _ := PhysicalAccumulationRegisterTable(definition.ID)
	totalsTable, _ := PhysicalAccumulationRegisterTotalsTable(definition.ID)
	dimensionColumns := make([]string, len(definition.Dimensions))
	totalSelects, beforeSelects, turnoverSelects, outerSelects, groups := []string{}, []string{}, []string{}, []string{}, []string{}
	for index, dimension := range definition.Dimensions {
		column, _ := PhysicalAttributeColumn(dimension.ID)
		quoted := pgx.Identifier{column}.Sanitize()
		dimensionColumns[index] = column
		totalSelects, beforeSelects, turnoverSelects, outerSelects, groups = append(totalSelects, quoted), append(beforeSelects, quoted), append(turnoverSelects, quoted), append(outerSelects, quoted), append(groups, quoted)
	}
	for index, resource := range definition.Resources {
		column, _ := PhysicalAttributeColumn(resource.ID)
		quoted := pgx.Identifier{column}.Sanitize()
		amount := "COALESCE(" + quoted + ", 0)"
		signed := "CASE WHEN movement_kind = 1 THEN " + amount + " ELSE -" + amount + " END"
		openingAlias := pgx.Identifier{fmt.Sprintf("r%d_opening", index)}.Sanitize()
		receiptAlias := pgx.Identifier{fmt.Sprintf("r%d_receipt", index)}.Sanitize()
		expenseAlias := pgx.Identifier{fmt.Sprintf("r%d_expense", index)}.Sanitize()
		netAlias := pgx.Identifier{fmt.Sprintf("r%d_net", index)}.Sanitize()
		totalSelects = append(totalSelects, quoted+" AS "+openingAlias, "0::numeric AS "+receiptAlias, "0::numeric AS "+expenseAlias, "0::numeric AS "+netAlias)
		beforeSelects = append(beforeSelects, signed+" AS "+openingAlias, "0::numeric AS "+receiptAlias, "0::numeric AS "+expenseAlias, "0::numeric AS "+netAlias)
		turnoverSelects = append(turnoverSelects, "0::numeric AS "+openingAlias, "CASE WHEN movement_kind = 1 THEN "+amount+" ELSE 0 END AS "+receiptAlias, "CASE WHEN movement_kind = 2 THEN "+amount+" ELSE 0 END AS "+expenseAlias, signed+" AS "+netAlias)
		outerSelects = append(outerSelects, "SUM("+openingAlias+") AS "+openingAlias, "SUM("+receiptAlias+") AS "+receiptAlias, "SUM("+expenseAlias+") AS "+expenseAlias, "SUM("+netAlias+") AS "+netAlias)
	}
	union := "SELECT " + strings.Join(totalSelects, ", ") + " FROM " + qualifiedCatalogTable(totalsTable) + " WHERE total_period < $1 UNION ALL SELECT " +
		strings.Join(beforeSelects, ", ") + " FROM " + qualifiedCatalogTable(movementTable) + " WHERE active = true AND period >= $1 AND period < $2 UNION ALL SELECT " +
		strings.Join(turnoverSelects, ", ") + " FROM " + qualifiedCatalogTable(movementTable) + " WHERE active = true AND period >= $2 AND period <= $3"
	conditions, arguments := []string{}, []any{month, begin, end}
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
		arguments = append(arguments, encoded)
		conditions = append(conditions, pgx.Identifier{column}.Sanitize()+fmt.Sprintf(" = $%d", len(arguments)))
	}
	inner := "SELECT " + strings.Join(outerSelects, ", ") + " FROM (" + union + ") AS source"
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
	rows, err := query.Query(ctx, "SELECT to_jsonb(item) FROM ("+inner+") AS item", arguments...)
	if err != nil {
		return nil, recordDataError(ctx, repository.pool, fmt.Errorf("read accumulation register %s balances and turnovers: %w", definition.Name, err))
	}
	defer rows.Close()
	result := make([]AccumulationRegisterAggregate, 0)
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

func (repository *AccumulationRegisterRepository) decodeBalancesAndTurnovers(definition AccumulationRegisterDefinition, dimensionColumns []string, encoded []byte) (AccumulationRegisterAggregate, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return AccumulationRegisterAggregate{}, err
	}
	result := AccumulationRegisterAggregate{Dimensions: map[uuid.UUID]Value{}, Opening: map[uuid.UUID]Value{}, Receipt: map[uuid.UUID]Value{}, Expense: map[uuid.UUID]Value{}, Turnover: map[uuid.UUID]Value{}, Closing: map[uuid.UUID]Value{}}
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

func accumulationAggregateZero(resources []Attribute, aggregate AccumulationRegisterAggregate) bool {
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

func (repository *AccumulationRegisterRepository) balanceAt(ctx context.Context, definition AccumulationRegisterDefinition, period time.Time, dimensions map[uuid.UUID]Value) ([]AccumulationRegisterAggregate, error) {
	month := time.Date(period.UTC().Year(), period.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	filter, err := repository.normalizeDimensions(definition, dimensions)
	if err != nil {
		return nil, err
	}
	movementTable, _ := PhysicalAccumulationRegisterTable(definition.ID)
	totalsTable, _ := PhysicalAccumulationRegisterTotalsTable(definition.ID)
	dimensionColumns := make([]string, len(definition.Dimensions))
	totalSelects, movementSelects, outerSelects, groups := []string{}, []string{}, []string{}, []string{}
	for index, dimension := range definition.Dimensions {
		column, _ := PhysicalAttributeColumn(dimension.ID)
		quoted := pgx.Identifier{column}.Sanitize()
		dimensionColumns[index] = column
		totalSelects, movementSelects, outerSelects, groups = append(totalSelects, quoted), append(movementSelects, quoted), append(outerSelects, quoted), append(groups, quoted)
	}
	for index, resource := range definition.Resources {
		column, _ := PhysicalAttributeColumn(resource.ID)
		quoted := pgx.Identifier{column}.Sanitize()
		alias := pgx.Identifier{fmt.Sprintf("r%d_net", index)}.Sanitize()
		totalSelects = append(totalSelects, quoted+" AS "+alias)
		movementSelects = append(movementSelects, "CASE WHEN movement_kind = 1 THEN COALESCE("+quoted+", 0) ELSE -COALESCE("+quoted+", 0) END AS "+alias)
		outerSelects = append(outerSelects, "SUM("+alias+") AS "+alias)
	}
	union := "SELECT " + strings.Join(totalSelects, ", ") + " FROM " + qualifiedCatalogTable(totalsTable) + " WHERE total_period < $1 UNION ALL SELECT " +
		strings.Join(movementSelects, ", ") + " FROM " + qualifiedCatalogTable(movementTable) + " WHERE active = true AND period >= $1 AND period <= $2"
	conditions, arguments := []string{}, []any{month, period}
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
		arguments = append(arguments, encoded)
		conditions = append(conditions, pgx.Identifier{column}.Sanitize()+fmt.Sprintf(" = $%d", len(arguments)))
	}
	inner := "SELECT " + strings.Join(outerSelects, ", ") + " FROM (" + union + ") AS source"
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
	rows, err := query.Query(ctx, "SELECT to_jsonb(item) FROM ("+inner+") AS item", arguments...)
	if err != nil {
		return nil, recordDataError(ctx, repository.pool, fmt.Errorf("read accumulation register %s balances: %w", definition.Name, err))
	}
	defer rows.Close()
	result := make([]AccumulationRegisterAggregate, 0)
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

func (repository *AccumulationRegisterRepository) queryTotalsBefore(ctx context.Context, definition AccumulationRegisterDefinition, period time.Time, dimensions map[uuid.UUID]Value) ([]AccumulationRegisterAggregate, error) {
	filter, err := repository.normalizeDimensions(definition, dimensions)
	if err != nil {
		return nil, err
	}
	table, _ := PhysicalAccumulationRegisterTotalsTable(definition.ID)
	dimensionColumns := make([]string, len(definition.Dimensions))
	selects := make([]string, 0, len(definition.Dimensions)+len(definition.Resources))
	groups := make([]string, 0, len(definition.Dimensions))
	for index, dimension := range definition.Dimensions {
		column, _ := PhysicalAttributeColumn(dimension.ID)
		quoted := pgx.Identifier{column}.Sanitize()
		dimensionColumns[index], selects, groups = column, append(selects, quoted), append(groups, quoted)
	}
	for index, resource := range definition.Resources {
		column, _ := PhysicalAttributeColumn(resource.ID)
		quoted := pgx.Identifier{column}.Sanitize()
		selects = append(selects, "SUM("+quoted+") AS "+pgx.Identifier{fmt.Sprintf("r%d_net", index)}.Sanitize())
	}
	conditions, arguments := []string{"total_period < $1"}, []any{period}
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
		arguments = append(arguments, encoded)
		conditions = append(conditions, pgx.Identifier{column}.Sanitize()+fmt.Sprintf(" = $%d", len(arguments)))
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
		return nil, recordDataError(ctx, repository.pool, fmt.Errorf("read accumulation register %s totals: %w", definition.Name, err))
	}
	defer rows.Close()
	result := make([]AccumulationRegisterAggregate, 0)
	memory := uint64(512)
	for rows.Next() {
		if len(result) >= maxAccumulationRegisterRecords {
			return nil, fmt.Errorf("accumulation register %s totals exceed row limit", definition.Name)
		}
		var encoded []byte
		if err := rows.Scan(&encoded); err != nil {
			return nil, err
		}
		if uint64(len(encoded))+256 > maxAccumulationRegisterMemory-memory {
			return nil, fmt.Errorf("accumulation register %s totals exceed memory limit", definition.Name)
		}
		memory += uint64(len(encoded)) + 256
		item, err := repository.decodeAggregate(definition, dimensionColumns, encoded)
		if err != nil {
			return nil, err
		}
		if definition.Kind == AccumulationRegisterBalance && (!accumulationResourcesZero(definition.Resources, item.Receipt) || !accumulationResourcesZero(definition.Resources, item.Expense)) || definition.Kind == AccumulationRegisterTurnover && !accumulationResourcesZero(definition.Resources, item.Turnover) {
			result = append(result, item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, recordDataError(ctx, repository.pool, err)
	}
	return result, nil
}

func mergeAccumulationBalanceParts(definition AccumulationRegisterDefinition, parts ...[]AccumulationRegisterAggregate) ([]AccumulationRegisterAggregate, error) {
	combined := map[string]*AccumulationRegisterAggregate{}
	for _, source := range parts {
		for _, row := range source {
			key := accumulationDimensionKey(definition, row.Dimensions)
			target := combined[key]
			if target == nil {
				target = &AccumulationRegisterAggregate{Dimensions: mapsCloneValues(row.Dimensions), Turnover: map[uuid.UUID]Value{}}
				combined[key] = target
			}
			for _, resource := range definition.Resources {
				leftText := target.Turnover[resource.ID].Data
				if leftText == "" {
					leftText = "0"
				}
				rightText := row.Turnover[resource.ID].Data
				if rightText == "" {
					rightText = "0"
				}
				left, _ := bslnumber.Parse(leftText)
				right, _ := bslnumber.Parse(rightText)
				sum, err := left.Add(right)
				if err != nil {
					return nil, err
				}
				target.Turnover[resource.ID] = Value{Kind: NumberType, Data: sum.String()}
			}
		}
	}
	keys := make([]string, 0, len(combined))
	for key := range combined {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	result := make([]AccumulationRegisterAggregate, 0, len(keys))
	for _, key := range keys {
		result = append(result, *combined[key])
	}
	return result, nil
}

func (repository *AccumulationRegisterRepository) queryAggregates(ctx context.Context, definition AccumulationRegisterDefinition, begin, end *time.Time, dimensions map[uuid.UUID]Value) ([]AccumulationRegisterAggregate, error) {
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
	result := make([]AccumulationRegisterAggregate, 0)
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
		known[dimension.ID] = dimension
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

func (repository *AccumulationRegisterRepository) decodeAggregate(definition AccumulationRegisterDefinition, dimensionColumns []string, encoded []byte) (AccumulationRegisterAggregate, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return AccumulationRegisterAggregate{}, err
	}
	result := AccumulationRegisterAggregate{Dimensions: map[uuid.UUID]Value{}, Receipt: map[uuid.UUID]Value{}, Expense: map[uuid.UUID]Value{}, Turnover: map[uuid.UUID]Value{}}
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

func mergeAccumulationAggregates(definition AccumulationRegisterDefinition, opening, turnover []AccumulationRegisterAggregate) ([]AccumulationRegisterAggregate, error) {
	combined := map[string]*AccumulationRegisterAggregate{}
	add := func(source AccumulationRegisterAggregate, isOpening bool) error {
		key := accumulationDimensionKey(definition, source.Dimensions)
		target := combined[key]
		if target == nil {
			target = &AccumulationRegisterAggregate{Dimensions: mapsCloneValues(source.Dimensions), Opening: map[uuid.UUID]Value{}, Receipt: map[uuid.UUID]Value{}, Expense: map[uuid.UUID]Value{}, Turnover: map[uuid.UUID]Value{}, Closing: map[uuid.UUID]Value{}}
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
	result := make([]AccumulationRegisterAggregate, 0, len(keys))
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

func accumulationTotalsSplit(recorder DocumentReference) int16 {
	hash := sha256.New()
	writeHashPart(hash, "accumulation-totals-split")
	writeHashPart(hash, recorder.DocumentID.String())
	writeHashPart(hash, recorder.ObjectID.String())
	return int16(binary.BigEndian.Uint64(hash.Sum(nil)[:8]) % accumulationTotalsSplitCount)
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
