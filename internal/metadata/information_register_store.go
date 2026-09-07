package metadata

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	maxInformationRegisterRecords    = 1_000_000
	maxInformationRegisterLineNumber = int64(1<<31 - 1)
)

// InformationRegisterRecord is one independent or recorder-owned register row.
type InformationRegisterRecord struct {
	RecordID   uuid.UUID
	Period     time.Time
	Recorder   DocumentReference
	LineNumber int
	Active     bool
	Dimensions map[uuid.UUID]Value
	Resources  map[uuid.UUID]Value
	Attributes map[uuid.UUID]Value
}

// InformationRegisterFilter contains equality conditions supported by record sets.
type InformationRegisterFilter struct {
	Period     *time.Time
	Recorder   *DocumentReference
	Dimensions map[uuid.UUID]Value
}

// InformationRegisterRecordSet is the mutable BSL-compatible unit of register I/O.
type InformationRegisterRecordSet struct {
	RegisterID uuid.UUID
	Filter     InformationRegisterFilter
	Records    []*InformationRegisterRecord
}

// Add appends a blank record with a stable internal identity.
func (set *InformationRegisterRecordSet) Add() (*InformationRegisterRecord, error) {
	if set == nil {
		return nil, fmt.Errorf("information register record set is required")
	}
	if len(set.Records) >= maxInformationRegisterRecords {
		return nil, fmt.Errorf("information register record set exceeds %d records", maxInformationRegisterRecords)
	}
	id, err := uuid.New()
	if err != nil {
		return nil, err
	}
	record := &InformationRegisterRecord{
		RecordID: id, Active: true, Dimensions: map[uuid.UUID]Value{}, Resources: map[uuid.UUID]Value{}, Attributes: map[uuid.UUID]Value{},
	}
	set.Records = append(set.Records, record)
	return record, nil
}

type InformationRegisterRepository struct {
	pool    *pgxpool.Pool
	catalog *Catalog
}

func NewInformationRegisterRepository(pool *pgxpool.Pool, catalog *Catalog) (*InformationRegisterRepository, error) {
	if pool == nil || catalog == nil {
		return nil, fmt.Errorf("information register repository requires PostgreSQL and metadata catalog")
	}
	return &InformationRegisterRepository{pool: pool, catalog: catalog}, nil
}

func (repository *InformationRegisterRepository) NewRecordSet(name string) (*InformationRegisterRecordSet, error) {
	definition, ok := repository.catalog.InformationRegisterDefinition(name)
	if !ok {
		return nil, fmt.Errorf("unknown information register %q", name)
	}
	return &InformationRegisterRecordSet{
		RegisterID: definition.ID,
		Filter:     InformationRegisterFilter{Dimensions: map[uuid.UUID]Value{}},
		Records:    []*InformationRegisterRecord{},
	}, nil
}

// Read replaces the in-memory rows with records matching the record-set filter.
func (repository *InformationRegisterRepository) Read(ctx context.Context, set *InformationRegisterRecordSet) error {
	definition, filter, err := repository.normalizeSetIdentity(set)
	if err != nil {
		return err
	}
	records, err := repository.read(ctx, definition, filter, "", nil)
	if err != nil {
		return err
	}
	set.Filter = filter
	set.Records = records
	return nil
}

// Write appends rows or atomically replaces every row matching the record-set filter.
func (repository *InformationRegisterRepository) Write(ctx context.Context, set *InformationRegisterRecordSet, replace bool) error {
	return repository.WriteWithHandler(ctx, set, replace, nil)
}

// WriteWithHandler writes a record set and dispatches its lifecycle events in the same transaction.
func (repository *InformationRegisterRepository) WriteWithHandler(ctx context.Context, set *InformationRegisterRecordSet, replace bool, handler InformationRegisterEventHandler) error {
	if set == nil {
		return fmt.Errorf("information register record set is required")
	}
	original := cloneInformationRegisterRecordSet(set)
	working := cloneInformationRegisterRecordSet(set)
	definition, filter, err := repository.normalizeSetIdentity(working)
	if err != nil {
		return err
	}
	working.Filter = filter
	if len(working.Records) > maxInformationRegisterRecords {
		return fmt.Errorf("information register %s record set exceeds %d records", definition.Name, maxInformationRegisterRecords)
	}
	if err := repository.normalizeRecordSetRecords(definition, filter, working); err != nil {
		return err
	}
	if size, ok := informationRegisterSetMemory(working, maxInformationRegisterRuntimeMemory); !ok || size > maxInformationRegisterRuntimeMemory {
		return fmt.Errorf("information register %s record set exceeds the %d-byte memory limit", definition.Name, maxInformationRegisterRuntimeMemory)
	}
	err = runDataTransaction(ctx, repository.pool, func() { *set = *cloneInformationRegisterRecordSet(original) }, func(transactionContext context.Context, transaction pgx.Tx) error {
		if err := dispatchInformationRegisterEvent(transactionContext, handler, InformationRegisterEventBeforeWrite, working, replace); err != nil {
			return err
		}
		afterDefinition, afterFilter, err := repository.normalizeSetIdentity(working)
		if err != nil {
			return err
		}
		if afterDefinition.ID != definition.ID || !informationRegisterFiltersEqual(filter, afterFilter) {
			return fmt.Errorf("information register before-write event changed immutable record-set identity or filter")
		}
		working.Filter = filter
		if err := repository.normalizeRecordSetRecords(definition, filter, working); err != nil {
			return err
		}
		if size, ok := informationRegisterSetMemory(working, maxInformationRegisterRuntimeMemory); !ok || size > maxInformationRegisterRuntimeMemory {
			return fmt.Errorf("information register %s record set exceeds the %d-byte memory limit", definition.Name, maxInformationRegisterRuntimeMemory)
		}
		if err := lockInformationRegisterWrite(transactionContext, transaction, definition, filter); err != nil {
			return err
		}
		if err := repository.validateRecorders(transactionContext, transaction, definition, working.Records); err != nil {
			return err
		}
		if replace {
			where, arguments, err := repository.filterSQL(definition, filter, 1)
			if err != nil {
				return err
			}
			table, _ := PhysicalInformationRegisterTable(definition.ID)
			if _, err := transaction.Exec(transactionContext, "DELETE FROM "+qualifiedCatalogTable(table)+where, arguments...); err != nil {
				return informationRegisterWriteError(definition.Name, err)
			}
		}
		if err := repository.insertRecords(transactionContext, transaction, definition, working.Records); err != nil {
			return err
		}
		for _, event := range []InformationRegisterEvent{InformationRegisterEventOnWrite, InformationRegisterEventAfterWrite} {
			if err := dispatchInformationRegisterEvent(transactionContext, handler, event, cloneInformationRegisterRecordSet(working), replace); err != nil {
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

func (repository *InformationRegisterRepository) validateRecorders(ctx context.Context, transaction pgx.Tx, definition InformationRegisterDefinition, records []*InformationRegisterRecord) error {
	if definition.WriteMode != InformationRegisterRecorder {
		return nil
	}
	type recorderIdentity struct{ document, object uuid.UUID }
	unique := map[recorderIdentity]bool{}
	for _, record := range records {
		unique[recorderIdentity{document: record.Recorder.DocumentID, object: record.Recorder.ObjectID}] = true
	}
	ordered := make([]recorderIdentity, 0, len(unique))
	for identity := range unique {
		ordered = append(ordered, identity)
	}
	slices.SortFunc(ordered, func(left, right recorderIdentity) int {
		if compared := strings.Compare(left.document.String(), right.document.String()); compared != 0 {
			return compared
		}
		return strings.Compare(left.object.String(), right.object.String())
	})
	for _, identity := range ordered {
		if err := lockObjectForWrite(ctx, transaction, DocumentType, identity.document, identity.object); err != nil {
			return err
		}
		table, _ := PhysicalDocumentTable(identity.document)
		var exists bool
		if err := transaction.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM "+qualifiedCatalogTable(table)+" WHERE ref = $1)", identity.object.String()).Scan(&exists); err != nil {
			return fmt.Errorf("validate information register %s recorder: %w", definition.Name, err)
		}
		if !exists {
			return fmt.Errorf("information register %s recorder does not exist", definition.Name)
		}
	}
	return nil
}

func (repository *InformationRegisterRepository) normalizeRecordSetRecords(definition InformationRegisterDefinition, filter InformationRegisterFilter, set *InformationRegisterRecordSet) error {
	keys := make(map[string]bool, len(set.Records))
	nextLine := int64(1)
	if definition.WriteMode == InformationRegisterRecorder {
		for _, record := range set.Records {
			if record == nil {
				continue
			}
			candidate := int64(record.LineNumber)
			if candidate >= nextLine && candidate <= maxInformationRegisterLineNumber {
				nextLine = candidate + 1
			}
		}
	}
	for index, record := range set.Records {
		if record == nil {
			return fmt.Errorf("information register %s record %d is nil", definition.Name, index+1)
		}
		if definition.WriteMode == InformationRegisterRecorder && record.LineNumber == 0 {
			if nextLine > maxInformationRegisterLineNumber {
				return fmt.Errorf("information register %s line number is exhausted", definition.Name)
			}
			record.LineNumber = int(nextLine)
			nextLine++
		}
		if err := repository.normalizeRecord(definition, record, index+1); err != nil {
			return err
		}
		if !informationRegisterRecordMatchesFilter(*record, filter) {
			return fmt.Errorf("information register %s record %d does not match the record-set filter", definition.Name, index+1)
		}
		key := informationRegisterRecordKey(definition, *record)
		if keys[key] {
			return fmt.Errorf("information register %s record set contains a duplicate key", definition.Name)
		}
		keys[key] = true
	}
	return nil
}

func informationRegisterFiltersEqual(left, right InformationRegisterFilter) bool {
	if (left.Period == nil) != (right.Period == nil) || (left.Recorder == nil) != (right.Recorder == nil) || len(left.Dimensions) != len(right.Dimensions) {
		return false
	}
	if left.Period != nil && !left.Period.Equal(*right.Period) || left.Recorder != nil && *left.Recorder != *right.Recorder {
		return false
	}
	for id, value := range left.Dimensions {
		if right.Dimensions[id] != value {
			return false
		}
	}
	return true
}

// SliceLast returns the latest active record for every dimension combination at or before period.
func (repository *InformationRegisterRepository) SliceLast(ctx context.Context, name string, period time.Time, dimensions map[uuid.UUID]Value) ([]*InformationRegisterRecord, error) {
	return repository.slice(ctx, name, period, dimensions, false)
}

// SliceFirst returns the earliest active record for every dimension combination at or after period.
func (repository *InformationRegisterRepository) SliceFirst(ctx context.Context, name string, period time.Time, dimensions map[uuid.UUID]Value) ([]*InformationRegisterRecord, error) {
	return repository.slice(ctx, name, period, dimensions, true)
}

func (repository *InformationRegisterRepository) slice(ctx context.Context, name string, period time.Time, dimensions map[uuid.UUID]Value, first bool) ([]*InformationRegisterRecord, error) {
	definition, ok := repository.catalog.InformationRegisterDefinition(name)
	if !ok {
		return nil, fmt.Errorf("unknown information register %q", name)
	}
	if definition.Periodicity == InformationRegisterPeriodNone {
		return nil, fmt.Errorf("information register %s is not periodic", definition.Name)
	}
	period, err := normalizeInformationRegisterPeriod(definition.Periodicity, period)
	if err != nil {
		return nil, fmt.Errorf("information register %s period: %w", definition.Name, err)
	}
	filter := InformationRegisterFilter{Dimensions: mapsCloneValues(dimensions)}
	_, filter, err = repository.normalizeSetIdentity(&InformationRegisterRecordSet{RegisterID: definition.ID, Filter: filter})
	if err != nil {
		return nil, err
	}
	operator, direction := " <= ", " DESC"
	if first {
		operator, direction = " >= ", " ASC"
	}
	periodColumn := pgx.Identifier{"period"}.Sanitize()
	extra := periodColumn + operator + "$1"
	return repository.read(ctx, definition, filter, extra, []any{period}, direction)
}

func (repository *InformationRegisterRepository) normalizeSetIdentity(set *InformationRegisterRecordSet) (InformationRegisterDefinition, InformationRegisterFilter, error) {
	if set == nil || set.RegisterID.IsZero() {
		return InformationRegisterDefinition{}, InformationRegisterFilter{}, fmt.Errorf("information register record set identity is invalid")
	}
	definition, ok := repository.catalog.InformationRegisterByID(set.RegisterID)
	if !ok {
		return InformationRegisterDefinition{}, InformationRegisterFilter{}, fmt.Errorf("unknown information register %s", set.RegisterID)
	}
	filter := cloneInformationRegisterFilter(set.Filter)
	known := make(map[uuid.UUID]Attribute, len(definition.Dimensions))
	for _, dimension := range definition.Dimensions {
		known[dimension.ID] = dimension
		value, present := filter.Dimensions[dimension.ID]
		if !present {
			continue
		}
		normalized, err := repository.catalog.normalizeTypes("information register "+definition.Name+" dimension "+dimension.Name, dimension.Types, value)
		if err != nil {
			return InformationRegisterDefinition{}, InformationRegisterFilter{}, err
		}
		filter.Dimensions[dimension.ID] = normalized
	}
	for id := range filter.Dimensions {
		if _, ok := known[id]; !ok {
			return InformationRegisterDefinition{}, InformationRegisterFilter{}, fmt.Errorf("information register %s filter contains unknown dimension %s", definition.Name, id)
		}
	}
	if filter.Period != nil {
		if definition.Periodicity == InformationRegisterPeriodNone {
			return InformationRegisterDefinition{}, InformationRegisterFilter{}, fmt.Errorf("information register %s is not periodic", definition.Name)
		}
		normalized, err := normalizeInformationRegisterPeriod(definition.Periodicity, *filter.Period)
		if err != nil {
			return InformationRegisterDefinition{}, InformationRegisterFilter{}, err
		}
		filter.Period = &normalized
	}
	if filter.Recorder != nil {
		if definition.WriteMode != InformationRegisterRecorder || !allowedInformationRegisterRecorder(definition, *filter.Recorder) {
			return InformationRegisterDefinition{}, InformationRegisterFilter{}, fmt.Errorf("information register %s filter has an invalid recorder", definition.Name)
		}
		copy := *filter.Recorder
		filter.Recorder = &copy
	}
	return definition, filter, nil
}

func (repository *InformationRegisterRepository) normalizeRecord(definition InformationRegisterDefinition, record *InformationRegisterRecord, line int) error {
	if record.RecordID.IsZero() {
		id, err := uuid.New()
		if err != nil {
			return err
		}
		record.RecordID = id
	}
	var err error
	if definition.Periodicity == InformationRegisterPeriodNone {
		record.Period = time.Time{}
	} else if record.Period, err = normalizeInformationRegisterPeriod(definition.Periodicity, record.Period); err != nil {
		return fmt.Errorf("information register %s record %d period: %w", definition.Name, line, err)
	}
	if definition.WriteMode == InformationRegisterRecorder {
		if !allowedInformationRegisterRecorder(definition, record.Recorder) {
			return fmt.Errorf("information register %s record %d has an invalid recorder", definition.Name, line)
		}
		if record.LineNumber < 1 || int64(record.LineNumber) > maxInformationRegisterLineNumber {
			return fmt.Errorf("information register %s record %d line number must be 1..%d", definition.Name, line, maxInformationRegisterLineNumber)
		}
	} else {
		record.Recorder, record.LineNumber, record.Active = DocumentReference{}, 0, true
	}
	record.Dimensions, err = repository.catalog.normalizeAttributes(definition.Name, definition.Dimensions, record.Dimensions)
	if err != nil {
		return fmt.Errorf("information register %s record %d dimensions: %w", definition.Name, line, err)
	}
	record.Resources, err = repository.catalog.normalizeAttributes(definition.Name, definition.Resources, record.Resources)
	if err != nil {
		return fmt.Errorf("information register %s record %d resources: %w", definition.Name, line, err)
	}
	record.Attributes, err = repository.catalog.normalizeAttributes(definition.Name, definition.Attributes, record.Attributes)
	if err != nil {
		return fmt.Errorf("information register %s record %d attributes: %w", definition.Name, line, err)
	}
	return nil
}

func allowedInformationRegisterRecorder(definition InformationRegisterDefinition, recorder DocumentReference) bool {
	if recorder.ObjectID.IsZero() || recorder.DocumentID.IsZero() {
		return false
	}
	return slices.Contains(definition.Recorders, recorder.DocumentID)
}

func normalizeInformationRegisterPeriod(periodicity InformationRegisterPeriodicity, value time.Time) (time.Time, error) {
	value = value.UTC()
	if err := validateDocumentDate(value); err != nil {
		return time.Time{}, err
	}
	year, month, day := value.Date()
	switch periodicity {
	case InformationRegisterPeriodSecond:
		return value.Truncate(time.Second), nil
	case InformationRegisterPeriodDay:
		return time.Date(year, month, day, 0, 0, 0, 0, time.UTC), nil
	case InformationRegisterPeriodMonth:
		return time.Date(year, month, 1, 0, 0, 0, 0, time.UTC), nil
	case InformationRegisterPeriodQuarter:
		quarterMonth := time.Month((int(month)-1)/3*3 + 1)
		return time.Date(year, quarterMonth, 1, 0, 0, 0, 0, time.UTC), nil
	case InformationRegisterPeriodYear:
		return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC), nil
	case InformationRegisterPeriodRecorderPosition:
		return value.Truncate(100 * time.Microsecond), nil
	default:
		return time.Time{}, fmt.Errorf("register is not periodic")
	}
}

func (repository *InformationRegisterRepository) insertRecords(ctx context.Context, transaction pgx.Tx, definition InformationRegisterDefinition, records []*InformationRegisterRecord) error {
	if len(records) == 0 {
		return nil
	}
	table, _ := PhysicalInformationRegisterTable(definition.ID)
	columns := []string{"record_id", "record_key"}
	if definition.Periodicity != InformationRegisterPeriodNone {
		columns = append(columns, "period")
	}
	if definition.WriteMode == InformationRegisterRecorder {
		columns = append(columns, "recorder_type", "recorder_ref", "line_no", "active")
	}
	for _, field := range informationRegisterFields(definition) {
		column, _ := PhysicalAttributeColumn(field.ID)
		columns = append(columns, column)
	}
	source := pgx.CopyFromSlice(len(records), func(index int) ([]any, error) {
		record := records[index]
		arguments := []any{record.RecordID.String(), informationRegisterRecordKey(definition, *record)}
		if definition.Periodicity != InformationRegisterPeriodNone {
			arguments = append(arguments, record.Period)
		}
		if definition.WriteMode == InformationRegisterRecorder {
			arguments = append(arguments, record.Recorder.DocumentID.String(), record.Recorder.ObjectID.String(), record.LineNumber, record.Active)
		}
		groups := []struct {
			definitions []Attribute
			values      map[uuid.UUID]Value
		}{{definition.Dimensions, record.Dimensions}, {definition.Resources, record.Resources}, {definition.Attributes, record.Attributes}}
		for _, group := range groups {
			for _, field := range group.definitions {
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
		return informationRegisterWriteError(definition.Name, err)
	}
	if written != int64(len(records)) {
		return fmt.Errorf("write information register %s: copied %d of %d records", definition.Name, written, len(records))
	}
	return nil
}
func (repository *InformationRegisterRepository) read(ctx context.Context, definition InformationRegisterDefinition, filter InformationRegisterFilter, extra string, extraArguments []any, direction ...string) ([]*InformationRegisterRecord, error) {
	table, _ := PhysicalInformationRegisterTable(definition.ID)
	where, arguments, err := repository.filterSQL(definition, filter, len(extraArguments)+1)
	if err != nil {
		return nil, err
	}
	conditions := make([]string, 0, 3)
	if extra != "" {
		conditions = append(conditions, extra)
	}
	if where != "" {
		conditions = append(conditions, strings.TrimPrefix(where, " WHERE "))
	}
	if len(direction) != 0 && definition.WriteMode == InformationRegisterRecorder {
		conditions = append(conditions, "active = true")
	}
	where = ""
	if len(conditions) != 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	arguments = append(extraArguments, arguments...)
	order := " ORDER BY record_id"
	if definition.WriteMode == InformationRegisterRecorder {
		order = " ORDER BY recorder_type, recorder_ref, line_no, record_id"
	}
	if len(direction) != 0 {
		dimensionColumns := make([]string, len(definition.Dimensions))
		for index, dimension := range definition.Dimensions {
			column, _ := PhysicalAttributeColumn(dimension.ID)
			dimensionColumns[index] = pgx.Identifier{column}.Sanitize()
		}
		if len(dimensionColumns) == 0 {
			order = " ORDER BY period" + direction[0] + informationRegisterRecorderOrder(definition, direction[0]) + ", record_id LIMIT 1"
		} else {
			joined := strings.Join(dimensionColumns, ", ")
			order = " ORDER BY " + joined + ", period" + direction[0] + informationRegisterRecorderOrder(definition, direction[0]) + ", record_id LIMIT " + fmt.Sprint(maxInformationRegisterRecords+1)
		}
	} else {
		order += " LIMIT " + fmt.Sprint(maxInformationRegisterRecords+1)
	}
	projection := fmt.Sprintf("pg_column_size(item), CASE WHEN pg_column_size(item) <= %d THEN to_jsonb(item) END", maxInformationRegisterRuntimeMemory)
	selectPrefix := "SELECT " + projection + " FROM "
	if len(direction) != 0 && len(definition.Dimensions) != 0 {
		dimensionColumns := make([]string, len(definition.Dimensions))
		for index, dimension := range definition.Dimensions {
			column, _ := PhysicalAttributeColumn(dimension.ID)
			dimensionColumns[index] = pgx.Identifier{column}.Sanitize()
		}
		selectPrefix = "SELECT DISTINCT ON (" + strings.Join(dimensionColumns, ", ") + ") " + projection + " FROM "
	}
	query, err := queryData(ctx, repository.pool)
	if err != nil {
		return nil, err
	}
	rows, err := query.Query(ctx, selectPrefix+qualifiedCatalogTable(table)+" AS item"+where+order, arguments...)
	if err != nil {
		return nil, recordDataError(ctx, repository.pool, fmt.Errorf("read information register %s: %w", definition.Name, err))
	}
	defer rows.Close()
	records := make([]*InformationRegisterRecord, 0)
	memory := uint64(512)
	for rows.Next() {
		if len(records) >= maxInformationRegisterRecords {
			return nil, fmt.Errorf("information register %s read exceeds %d records", definition.Name, maxInformationRegisterRecords)
		}
		var storedBytes int64
		var encoded []byte
		if err := rows.Scan(&storedBytes, &encoded); err != nil {
			return nil, err
		}
		if storedBytes < 0 || uint64(storedBytes) > maxInformationRegisterRuntimeMemory {
			return nil, fmt.Errorf("information register %s row exceeds the %d-byte memory limit", definition.Name, maxInformationRegisterRuntimeMemory)
		}
		record, err := repository.decodeRecord(definition, encoded)
		if err != nil {
			return nil, err
		}
		addition := informationRegisterRecordMemory(record)
		if addition > maxInformationRegisterRuntimeMemory-memory {
			return nil, fmt.Errorf("information register %s read exceeds the %d-byte memory limit", definition.Name, maxInformationRegisterRuntimeMemory)
		}
		memory += addition
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, recordDataError(ctx, repository.pool, err)
	}
	return records, nil
}

func informationRegisterRecorderOrder(definition InformationRegisterDefinition, direction string) string {
	if definition.WriteMode != InformationRegisterRecorder {
		return ""
	}
	return ", recorder_type" + direction + ", recorder_ref" + direction + ", line_no" + direction
}

func (repository *InformationRegisterRepository) filterSQL(definition InformationRegisterDefinition, filter InformationRegisterFilter, firstArgument int) (string, []any, error) {
	conditions, arguments := []string{}, []any{}
	add := func(column string, value any) {
		conditions = append(conditions, pgx.Identifier{column}.Sanitize()+fmt.Sprintf(" = $%d", firstArgument+len(arguments)))
		arguments = append(arguments, value)
	}
	if filter.Period != nil {
		add("period", *filter.Period)
	}
	if filter.Recorder != nil {
		add("recorder_type", filter.Recorder.DocumentID.String())
		add("recorder_ref", filter.Recorder.ObjectID.String())
	}
	for _, dimension := range definition.Dimensions {
		value, present := filter.Dimensions[dimension.ID]
		if !present {
			continue
		}
		storage, _ := repository.catalog.attributeStorage(dimension.Types)
		encoded, err := databaseAttributeValue(storage, value)
		if err != nil {
			return "", nil, err
		}
		column, _ := PhysicalAttributeColumn(dimension.ID)
		add(column, encoded)
	}
	if len(conditions) == 0 {
		return "", arguments, nil
	}
	return " WHERE " + strings.Join(conditions, " AND "), arguments, nil
}

func (repository *InformationRegisterRepository) decodeRecord(definition InformationRegisterDefinition, encoded []byte) (*InformationRegisterRecord, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, fmt.Errorf("decode information register %s row: %w", definition.Name, err)
	}
	recordID, err := decodeJSONUUID(fields["record_id"])
	if err != nil {
		return nil, err
	}
	record := &InformationRegisterRecord{
		RecordID: recordID, Active: true, Dimensions: map[uuid.UUID]Value{}, Resources: map[uuid.UUID]Value{}, Attributes: map[uuid.UUID]Value{},
	}
	if definition.Periodicity != InformationRegisterPeriodNone {
		var text string
		if err := json.Unmarshal(fields["period"], &text); err != nil {
			return nil, err
		}
		record.Period, err = time.Parse(time.RFC3339Nano, text)
		if err != nil {
			return nil, err
		}
		record.Period, err = normalizeInformationRegisterPeriod(definition.Periodicity, record.Period)
		if err != nil {
			return nil, err
		}
	}
	if definition.WriteMode == InformationRegisterRecorder {
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
	}
	groups := []struct {
		definitions []Attribute
		values      map[uuid.UUID]Value
	}{{definition.Dimensions, record.Dimensions}, {definition.Resources, record.Resources}, {definition.Attributes, record.Attributes}}
	for _, group := range groups {
		for _, field := range group.definitions {
			column, _ := PhysicalAttributeColumn(field.ID)
			raw := fields[column]
			if len(raw) == 0 || string(raw) == "null" {
				continue
			}
			storage, _ := repository.catalog.attributeStorage(field.Types)
			value, err := decodeDatabaseAttribute(storage, raw)
			if err != nil {
				return nil, fmt.Errorf("decode information register %s field %s: %w", definition.Name, field.Name, err)
			}
			value, err = repository.catalog.normalizeTypes("information register "+definition.Name+" field "+field.Name, field.Types, value)
			if err != nil {
				return nil, err
			}
			group.values[field.ID] = value
		}
	}
	return record, nil
}

func informationRegisterRecordMatchesFilter(record InformationRegisterRecord, filter InformationRegisterFilter) bool {
	if filter.Period != nil && !record.Period.Equal(*filter.Period) {
		return false
	}
	if filter.Recorder != nil && record.Recorder != *filter.Recorder {
		return false
	}
	for id, expected := range filter.Dimensions {
		actual, ok := record.Dimensions[id]
		if !ok || actual != expected {
			return false
		}
	}
	return true
}

func informationRegisterRecordKey(definition InformationRegisterDefinition, record InformationRegisterRecord) string {
	hash := sha256.New()
	writeHashPart(hash, definition.ID.String())
	if definition.Periodicity != InformationRegisterPeriodNone {
		writeHashPart(hash, record.Period.Format(time.RFC3339Nano))
	}
	if definition.Periodicity == InformationRegisterPeriodRecorderPosition {
		writeHashPart(hash, record.Recorder.DocumentID.String())
		writeHashPart(hash, record.Recorder.ObjectID.String())
	}
	for _, dimension := range definition.Dimensions {
		writeHashPart(hash, dimension.ID.String())
		value, ok := record.Dimensions[dimension.ID]
		if !ok {
			writeHashPart(hash, "<undefined>")
			continue
		}
		writeHashPart(hash, string(value.Kind))
		writeHashPart(hash, value.Data)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

type hashWriter interface{ Write([]byte) (int, error) }

func writeHashPart(writer hashWriter, value string) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = writer.Write(size[:])
	_, _ = writer.Write([]byte(value))
}

func lockInformationRegisterWrite(ctx context.Context, transaction pgx.Tx, definition InformationRegisterDefinition, filter InformationRegisterFilter) error {
	if transaction == nil {
		return fmt.Errorf("invalid information register lock")
	}
	tableKey := objectLockKey("information-register", definition.ID, definition.ID)
	exact := len(filter.Dimensions) == len(definition.Dimensions)
	if definition.Periodicity != InformationRegisterPeriodNone {
		exact = exact && filter.Period != nil
	}
	if definition.WriteMode == InformationRegisterRecorder {
		exact = filter.Recorder != nil
	}
	if !exact {
		if _, err := transaction.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", tableKey); err != nil {
			return fmt.Errorf("lock information register %s: %w", definition.Name, err)
		}
		return nil
	}
	if _, err := transaction.Exec(ctx, "SELECT pg_advisory_xact_lock_shared($1)", tableKey); err != nil {
		return fmt.Errorf("lock information register %s: %w", definition.Name, err)
	}
	key := informationRegisterFilterLockKey(definition, filter)
	if _, err := transaction.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", key); err != nil {
		return fmt.Errorf("lock information register %s key: %w", definition.Name, err)
	}
	return nil
}

func informationRegisterFilterLockKey(definition InformationRegisterDefinition, filter InformationRegisterFilter) int64 {
	hash := sha256.New()
	writeHashPart(hash, "information-register-key")
	writeHashPart(hash, definition.ID.String())
	if filter.Recorder != nil {
		writeHashPart(hash, filter.Recorder.DocumentID.String())
		writeHashPart(hash, filter.Recorder.ObjectID.String())
		return int64(binary.BigEndian.Uint64(hash.Sum(nil)[:8]))
	}
	if filter.Period != nil {
		writeHashPart(hash, filter.Period.Format(time.RFC3339Nano))
	}
	for _, dimension := range definition.Dimensions {
		value, ok := filter.Dimensions[dimension.ID]
		if ok {
			writeHashPart(hash, string(value.Kind))
			writeHashPart(hash, value.Data)
		}
	}
	return int64(binary.BigEndian.Uint64(hash.Sum(nil)[:8]))
}

func informationRegisterWriteError(name string, err error) error {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" {
		return fmt.Errorf("information register %s contains a duplicate key: %w", name, err)
	}
	return fmt.Errorf("write information register %s: %w", name, err)
}

func cloneInformationRegisterRecordSet(source *InformationRegisterRecordSet) *InformationRegisterRecordSet {
	if source == nil {
		return nil
	}
	result := *source
	result.Filter = cloneInformationRegisterFilter(source.Filter)
	result.Records = make([]*InformationRegisterRecord, len(source.Records))
	for index, record := range source.Records {
		result.Records[index] = cloneInformationRegisterRecord(record)
	}
	return &result
}

func cloneInformationRegisterFilter(source InformationRegisterFilter) InformationRegisterFilter {
	result := source
	result.Dimensions = mapsCloneValues(source.Dimensions)
	if result.Dimensions == nil {
		result.Dimensions = map[uuid.UUID]Value{}
	}
	if source.Period != nil {
		value := *source.Period
		result.Period = &value
	}
	if source.Recorder != nil {
		value := *source.Recorder
		result.Recorder = &value
	}
	return result
}

func cloneInformationRegisterRecord(source *InformationRegisterRecord) *InformationRegisterRecord {
	if source == nil {
		return nil
	}
	result := *source
	result.Dimensions = mapsCloneValues(source.Dimensions)
	result.Resources = mapsCloneValues(source.Resources)
	result.Attributes = mapsCloneValues(source.Attributes)
	return &result
}

func mapsCloneValues(source map[uuid.UUID]Value) map[uuid.UUID]Value {
	if source == nil {
		return map[uuid.UUID]Value{}
	}
	result := make(map[uuid.UUID]Value, len(source))
	for id, value := range source {
		result[id] = value
	}
	return result
}
