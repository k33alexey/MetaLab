package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	bslnumber "github.com/k33alexey/MetaLab/internal/bsl/number"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

var (
	ErrDocumentRecordNotFound = errors.New("document record not found")
	ErrDocumentWriteConflict  = errors.New("document record was changed or removed")
)

type DocumentReference struct {
	DocumentID uuid.UUID
	ObjectID   uuid.UUID
}

func (reference DocumentReference) IsEmpty() bool {
	return reference.DocumentID.IsZero() || reference.ObjectID.IsZero()
}

type DocumentRow = ObjectRow

type DocumentRecord struct {
	Reference    DocumentReference
	Version      int64
	Number       string
	Date         time.Time
	Posted       bool
	DeletionMark bool
	Attributes   map[uuid.UUID]Value
	TableParts   map[uuid.UUID][]DocumentRow
}

type DocumentRepository struct {
	pool    *pgxpool.Pool
	catalog *Catalog
	now     func() time.Time
}

// DocumentPostingAction writes or removes recorder-owned movements while the
// document transaction is still active.
type DocumentPostingAction func(context.Context, *DocumentRecord, DocumentWriteMode, DocumentPostingMode) error

func NewDocumentRepository(pool *pgxpool.Pool, catalog *Catalog) (*DocumentRepository, error) {
	if pool == nil || catalog == nil {
		return nil, fmt.Errorf("document repository requires PostgreSQL and metadata catalog")
	}
	return &DocumentRepository{pool: pool, catalog: catalog, now: time.Now}, nil
}

func (repository *DocumentRepository) New(ctx context.Context, name string, handler DocumentEventHandler) (*DocumentRecord, error) {
	definition, ok := repository.catalog.DocumentDefinition(name)
	if !ok {
		return nil, fmt.Errorf("unknown document %q", name)
	}
	id, err := uuid.New()
	if err != nil {
		return nil, err
	}
	record := &DocumentRecord{
		Reference: DocumentReference{DocumentID: definition.ID, ObjectID: id},
		Date:      normalizeDocumentDate(repository.now()), Attributes: make(map[uuid.UUID]Value),
		TableParts: make(map[uuid.UUID][]DocumentRow),
	}
	for _, part := range definition.TableParts {
		record.TableParts[part.ID] = []DocumentRow{}
	}
	reference := record.Reference
	if err := dispatchDocumentEvent(ctx, handler, DocumentEventFill, record); err != nil {
		return nil, err
	}
	if record.Reference != reference || record.Version != 0 || record.Posted {
		return nil, fmt.Errorf("document fill event changed immutable record state")
	}
	return record, nil
}

func (repository *DocumentRepository) Save(ctx context.Context, record *DocumentRecord, handler DocumentEventHandler) error {
	return repository.Write(ctx, record, DocumentWrite, DocumentPostingRegular, handler, nil)
}

// Write saves, posts, reposts or unposts a document as one PostgreSQL transaction.
func (repository *DocumentRepository) Write(ctx context.Context, record *DocumentRecord, writeMode DocumentWriteMode, postingMode DocumentPostingMode, handler DocumentEventHandler, postingAction DocumentPostingAction) error {
	if record == nil {
		return fmt.Errorf("document record is required")
	}
	original, working := cloneDocumentRecord(record), cloneDocumentRecord(record)
	definition, ok := repository.catalog.DocumentByID(working.Reference.DocumentID)
	if !ok {
		return fmt.Errorf("unknown document %s", working.Reference.DocumentID)
	}
	if err := validateDocumentWriteMode(definition, writeMode, postingMode); err != nil {
		return err
	}
	if writeMode == DocumentPost && working.DeletionMark {
		return fmt.Errorf("document %s marked for deletion cannot be posted", definition.Name)
	}
	if writeMode == DocumentUndoPosting && working.Version == 0 {
		return fmt.Errorf("new document %s cannot be unposted", definition.Name)
	}
	switch writeMode {
	case DocumentPost:
		working.Posted = true
	case DocumentUndoPosting:
		working.Posted = false
	}
	operationContext := withDocumentOperation(ctx, writeMode, postingMode)
	reference, expectedVersion, posted := working.Reference, working.Version, working.Posted
	prepare := func() error {
		if err := dispatchDocumentEvent(operationContext, handler, DocumentEventFillCheck, working); err != nil {
			return err
		}
		if working.Reference != reference || working.Version != expectedVersion || working.Posted != posted {
			return fmt.Errorf("document fill-check event changed immutable record state")
		}
		if err := dispatchDocumentEvent(operationContext, handler, DocumentEventBefore, working); err != nil {
			return err
		}
		if working.Reference != reference || working.Version != expectedVersion || working.Posted != posted {
			return fmt.Errorf("document before-write event changed immutable record state")
		}
		return nil
	}
	if repository.pool == nil {
		if err := prepare(); err != nil {
			return err
		}
	}
	err := runDataTransaction(ctx, repository.pool, func() { *record = *cloneDocumentRecord(original) }, func(transactionContext context.Context, transaction pgx.Tx) error {
		ctx = transactionContext
		operationContext = withDocumentOperation(transactionContext, writeMode, postingMode)
		if err := prepare(); err != nil {
			return err
		}
		if working.Version == 0 && working.Number == "" && definition.Number.Auto {
			working.Date = normalizeDocumentDate(working.Date)
			if err := validateDocumentDate(working.Date); err != nil {
				return fmt.Errorf("document %s date: %w", definition.Name, err)
			}
			table, _ := PhysicalDocumentTable(definition.ID)
			period := documentNumberPeriod(definition.Number.Periodicity, working.Date)
			value, err := nextObjectSequence(ctx, transaction, definition.ID, period, table, "number", definition.Number.Type == StringType, definition.Number.Periodicity != NumberPeriodNone)
			if err != nil {
				return fmt.Errorf("document %s number: %w", definition.Name, err)
			}
			working.Number, err = formatAutomaticIdentifier(value, definition.Number.Type, definition.Number.Length)
			if err != nil {
				return fmt.Errorf("document %s number: %w", definition.Name, err)
			}
		}
		if err := repository.normalizeRecord(definition, working); err != nil {
			return err
		}
		if err := lockObjectForWrite(ctx, transaction, DocumentType, definition.ID, working.Reference.ObjectID); err != nil {
			return err
		}
		version, err := repository.writeRecord(ctx, transaction, definition, working)
		if err != nil {
			return err
		}
		working.Version = version
		if err := repository.writeTableParts(ctx, transaction, definition, working); err != nil {
			return err
		}
		if err := dispatchDocumentEvent(operationContext, handler, DocumentEventOnWrite, cloneDocumentRecord(working)); err != nil {
			return err
		}
		if writeMode != DocumentWrite && postingAction != nil {
			if err := postingAction(operationContext, cloneDocumentRecord(working), writeMode, postingMode); err != nil {
				return err
			}
		}
		if err := dispatchDocumentEvent(operationContext, handler, DocumentEventAfter, cloneDocumentRecord(working)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	*record = *working
	return nil
}

func validateDocumentWriteMode(definition DocumentDefinition, writeMode DocumentWriteMode, postingMode DocumentPostingMode) error {
	if writeMode != DocumentWrite && writeMode != DocumentPost && writeMode != DocumentUndoPosting {
		return fmt.Errorf("document %s write mode %q is invalid", definition.Name, writeMode)
	}
	if postingMode != DocumentPostingRegular && postingMode != DocumentPostingRealTime {
		return fmt.Errorf("document %s posting mode %q is invalid", definition.Name, postingMode)
	}
	if writeMode != DocumentWrite && !definition.Posting {
		return fmt.Errorf("document %s does not allow posting", definition.Name)
	}
	return nil
}

func (repository *DocumentRepository) Get(ctx context.Context, reference DocumentReference) (*DocumentRecord, error) {
	definition, ok := repository.catalog.DocumentByID(reference.DocumentID)
	if !ok || reference.ObjectID.IsZero() {
		return nil, ErrDocumentRecordNotFound
	}
	table, _ := PhysicalDocumentTable(definition.ID)
	statement := "SELECT to_jsonb(item) FROM " + qualifiedCatalogTable(table) + " AS item WHERE ref = $1"
	var encoded []byte
	query, err := queryData(ctx, repository.pool)
	if err != nil {
		return nil, err
	}
	if err := query.QueryRow(ctx, statement, reference.ObjectID.String()).Scan(&encoded); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDocumentRecordNotFound
	} else if err != nil {
		return nil, recordDataError(ctx, repository.pool, fmt.Errorf("read document %s: %w", definition.Name, err))
	}
	record, err := repository.decodeRecord(definition, encoded)
	if err != nil {
		return nil, err
	}
	if err := repository.readTableParts(ctx, definition, record); err != nil {
		return nil, err
	}
	return record, nil
}

func (repository *DocumentRepository) FindByNumber(ctx context.Context, name, number string, date *time.Time) (DocumentReference, bool, error) {
	definition, ok := repository.catalog.DocumentDefinition(name)
	if !ok {
		return DocumentReference{}, false, fmt.Errorf("unknown document %q", name)
	}
	number, err := normalizeDocumentNumber(definition.Number, number)
	if err != nil {
		return DocumentReference{}, false, err
	}
	if definition.Number.Periodicity != NumberPeriodNone && date == nil {
		return DocumentReference{}, false, fmt.Errorf("document %s requires a date to find a periodic number", definition.Name)
	}
	period := 0
	if date != nil {
		normalized := normalizeDocumentDate(*date)
		if err := validateDocumentDate(normalized); err != nil {
			return DocumentReference{}, false, err
		}
		period = documentNumberPeriod(definition.Number.Periodicity, normalized)
	}
	table, _ := PhysicalDocumentTable(definition.ID)
	statement := "SELECT ref::text FROM " + qualifiedCatalogTable(table) + " WHERE number_period = $1 AND number = $2 ORDER BY date DESC, ref LIMIT 1"
	var idText string
	query, err := queryData(ctx, repository.pool)
	if err != nil {
		return DocumentReference{}, false, err
	}
	if err := query.QueryRow(ctx, statement, period, number).Scan(&idText); errors.Is(err, pgx.ErrNoRows) {
		return DocumentReference{}, false, nil
	} else if err != nil {
		return DocumentReference{}, false, recordDataError(ctx, repository.pool, fmt.Errorf("find document %s by number: %w", definition.Name, err))
	}
	id, err := uuid.Parse(idText)
	if err != nil {
		return DocumentReference{}, false, fmt.Errorf("parse document reference: %w", err)
	}
	return DocumentReference{DocumentID: definition.ID, ObjectID: id}, true, nil
}

func (repository *DocumentRepository) normalizeRecord(definition DocumentDefinition, record *DocumentRecord) error {
	if record.Reference.DocumentID != definition.ID || record.Reference.ObjectID.IsZero() || record.Version < 0 {
		return fmt.Errorf("invalid document %s reference or version", definition.Name)
	}
	number, err := normalizeDocumentNumber(definition.Number, record.Number)
	if err != nil {
		return fmt.Errorf("document %s number: %w", definition.Name, err)
	}
	record.Number = number
	record.Date = normalizeDocumentDate(record.Date)
	if err := validateDocumentDate(record.Date); err != nil {
		return fmt.Errorf("document %s date: %w", definition.Name, err)
	}
	if record.Posted && !definition.Posting {
		return fmt.Errorf("document %s does not allow posting", definition.Name)
	}
	attributes, err := repository.catalog.normalizeAttributes(definition.Name, definition.Attributes, record.Attributes)
	if err != nil {
		return err
	}
	record.Attributes = attributes
	if record.TableParts == nil {
		record.TableParts = make(map[uuid.UUID][]DocumentRow, len(definition.TableParts))
	}
	knownParts := make(map[uuid.UUID]bool, len(definition.TableParts))
	for _, part := range definition.TableParts {
		knownParts[part.ID] = true
		rows := record.TableParts[part.ID]
		if len(rows) > 1_000_000 {
			return fmt.Errorf("document %s table part %s exceeds 1000000 rows", definition.Name, part.Name)
		}
		normalizedRows := make([]DocumentRow, len(rows))
		for index, row := range rows {
			values, err := repository.catalog.normalizeAttributes(definition.Name+"."+part.Name, part.Attributes, row.Values)
			if err != nil {
				return fmt.Errorf("row %d: %w", index+1, err)
			}
			normalizedRows[index] = DocumentRow{Values: values}
		}
		record.TableParts[part.ID] = normalizedRows
	}
	for id := range record.TableParts {
		if !knownParts[id] {
			return fmt.Errorf("document %s contains unknown table part %s", definition.Name, id)
		}
	}
	return nil
}

func (repository *DocumentRepository) writeRecord(ctx context.Context, transaction pgx.Tx, definition DocumentDefinition, record *DocumentRecord) (int64, error) {
	table, _ := PhysicalDocumentTable(definition.ID)
	columns := []string{"ref", "number", "number_period", "date", "posted", "deletion_mark"}
	arguments := []any{
		record.Reference.ObjectID.String(), record.Number, documentNumberPeriod(definition.Number.Periodicity, record.Date), record.Date, record.Posted, record.DeletionMark,
	}
	for _, attribute := range definition.Attributes {
		column, _ := PhysicalAttributeColumn(attribute.ID)
		columns = append(columns, column)
		value, present := record.Attributes[attribute.ID]
		if !present {
			arguments = append(arguments, nil)
			continue
		}
		storage, _ := repository.catalog.attributeStorage(attribute.Types)
		encoded, err := databaseAttributeValue(storage, value)
		if err != nil {
			return 0, err
		}
		arguments = append(arguments, encoded)
	}
	quoted := quoteCatalogColumns(columns)
	if record.Version == 0 {
		placeholders := make([]string, len(arguments))
		for index := range placeholders {
			placeholders[index] = fmt.Sprintf("$%d", index+1)
		}
		statement := "INSERT INTO " + qualifiedCatalogTable(table) + " (" + strings.Join(quoted, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ") RETURNING version"
		var version int64
		if err := transaction.QueryRow(ctx, statement, arguments...).Scan(&version); err != nil {
			return 0, documentWriteError(definition.Name, err)
		}
		return version, nil
	}
	assignments := make([]string, 0, len(arguments))
	for index, column := range quoted[1:] {
		assignments = append(assignments, fmt.Sprintf("%s = $%d", column, index+1))
	}
	updateArguments := append([]any(nil), arguments[1:]...)
	updateArguments = append(updateArguments, record.Reference.ObjectID.String(), record.Version)
	statement := "UPDATE " + qualifiedCatalogTable(table) + " SET version = version + 1, " + strings.Join(assignments, ", ") +
		fmt.Sprintf(" WHERE ref = $%d AND version = $%d RETURNING version", len(updateArguments)-1, len(updateArguments))
	var version int64
	if err := transaction.QueryRow(ctx, statement, updateArguments...).Scan(&version); errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrDocumentWriteConflict
	} else if err != nil {
		return 0, documentWriteError(definition.Name, err)
	}
	return version, nil
}

func (repository *DocumentRepository) writeTableParts(ctx context.Context, transaction pgx.Tx, definition DocumentDefinition, record *DocumentRecord) error {
	for _, part := range definition.TableParts {
		table, _ := PhysicalDocumentTable(part.ID)
		if _, err := transaction.Exec(ctx, "DELETE FROM "+qualifiedCatalogTable(table)+" WHERE owner_ref = $1", record.Reference.ObjectID.String()); err != nil {
			return fmt.Errorf("clear document table part %s: %w", part.Name, err)
		}
		columns := []string{"owner_ref", "line_no"}
		for _, attribute := range part.Attributes {
			column, _ := PhysicalAttributeColumn(attribute.ID)
			columns = append(columns, column)
		}
		partRows := record.TableParts[part.ID]
		if len(partRows) == 0 {
			continue
		}
		source := pgx.CopyFromSlice(len(partRows), func(rowIndex int) ([]any, error) {
			row := partRows[rowIndex]
			arguments := make([]any, 0, len(columns))
			arguments = append(arguments, record.Reference.ObjectID.String(), rowIndex+1)
			for _, attribute := range part.Attributes {
				value, present := row.Values[attribute.ID]
				if !present {
					arguments = append(arguments, nil)
					continue
				}
				storage, _ := repository.catalog.attributeStorage(attribute.Types)
				encoded, err := databaseAttributeValue(storage, value)
				if err != nil {
					return nil, err
				}
				arguments = append(arguments, encoded)
			}
			return arguments, nil
		})
		written, err := transaction.CopyFrom(ctx, pgx.Identifier{schemadiff.ApplicationSchema, table}, columns, source)
		if err != nil {
			return fmt.Errorf("write document table part %s: %w", part.Name, err)
		}
		if written != int64(len(partRows)) {
			return fmt.Errorf("write document table part %s: copied %d of %d rows", part.Name, written, len(partRows))
		}
	}
	return nil
}

func (repository *DocumentRepository) decodeRecord(definition DocumentDefinition, encoded []byte) (*DocumentRecord, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, fmt.Errorf("decode document %s row: %w", definition.Name, err)
	}
	objectID, err := decodeJSONUUID(fields["ref"])
	if err != nil {
		return nil, err
	}
	record := &DocumentRecord{
		Reference:  DocumentReference{DocumentID: definition.ID, ObjectID: objectID},
		Attributes: make(map[uuid.UUID]Value), TableParts: make(map[uuid.UUID][]DocumentRow),
	}
	if err := json.Unmarshal(fields["version"], &record.Version); err != nil || record.Version < 1 {
		return nil, fmt.Errorf("decode document %s version", definition.Name)
	}
	if record.Number, err = decodeDocumentNumber(definition.Number, fields["number"]); err != nil {
		return nil, err
	}
	var dateText string
	if err := json.Unmarshal(fields["date"], &dateText); err != nil {
		return nil, fmt.Errorf("decode document %s date: %w", definition.Name, err)
	}
	record.Date, err = time.Parse(time.RFC3339Nano, dateText)
	if err != nil {
		return nil, fmt.Errorf("decode document %s date: %w", definition.Name, err)
	}
	record.Date = normalizeDocumentDate(record.Date)
	if err := json.Unmarshal(fields["posted"], &record.Posted); err != nil {
		return nil, fmt.Errorf("decode document %s posted state: %w", definition.Name, err)
	}
	if err := json.Unmarshal(fields["deletion_mark"], &record.DeletionMark); err != nil {
		return nil, fmt.Errorf("decode document %s deletion mark: %w", definition.Name, err)
	}
	for _, attribute := range definition.Attributes {
		column, _ := PhysicalAttributeColumn(attribute.ID)
		raw := fields[column]
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		storage, _ := repository.catalog.attributeStorage(attribute.Types)
		value, err := decodeDatabaseAttribute(storage, raw)
		if err != nil {
			return nil, fmt.Errorf("decode document %s attribute %s: %w", definition.Name, attribute.Name, err)
		}
		value, err = repository.catalog.normalizeTypes("attribute "+definition.Name+"."+attribute.Name, attribute.Types, value)
		if err != nil {
			return nil, fmt.Errorf("decode document %s attribute %s: %w", definition.Name, attribute.Name, err)
		}
		record.Attributes[attribute.ID] = value
	}
	return record, nil
}

func (repository *DocumentRepository) readTableParts(ctx context.Context, definition DocumentDefinition, record *DocumentRecord) error {
	query, err := queryData(ctx, repository.pool)
	if err != nil {
		return err
	}
	for _, part := range definition.TableParts {
		table, _ := PhysicalDocumentTable(part.ID)
		rows, err := query.Query(ctx, "SELECT to_jsonb(item) FROM "+qualifiedCatalogTable(table)+" AS item WHERE owner_ref = $1 ORDER BY line_no", record.Reference.ObjectID.String())
		if err != nil {
			return recordDataError(ctx, repository.pool, fmt.Errorf("read document table part %s: %w", part.Name, err))
		}
		decodedRows := make([]DocumentRow, 0)
		for rows.Next() {
			var encoded []byte
			if err := rows.Scan(&encoded); err != nil {
				rows.Close()
				return err
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &fields); err != nil {
				rows.Close()
				return err
			}
			row := DocumentRow{Values: make(map[uuid.UUID]Value)}
			for _, attribute := range part.Attributes {
				column, _ := PhysicalAttributeColumn(attribute.ID)
				raw := fields[column]
				if len(raw) == 0 || string(raw) == "null" {
					continue
				}
				storage, _ := repository.catalog.attributeStorage(attribute.Types)
				value, err := decodeDatabaseAttribute(storage, raw)
				if err != nil {
					rows.Close()
					return fmt.Errorf("decode document table part %s attribute %s: %w", part.Name, attribute.Name, err)
				}
				value, err = repository.catalog.normalizeTypes("attribute "+definition.Name+"."+part.Name+"."+attribute.Name, attribute.Types, value)
				if err != nil {
					rows.Close()
					return fmt.Errorf("decode document table part %s attribute %s: %w", part.Name, attribute.Name, err)
				}
				row.Values[attribute.ID] = value
			}
			decodedRows = append(decodedRows, row)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		record.TableParts[part.ID] = decodedRows
	}
	return nil
}

func normalizeDocumentNumber(number DocumentNumber, value string) (string, error) {
	if number.Type == StringType {
		if !utf8.ValidString(value) || value == "" || utf8.RuneCountInString(value) > number.Length {
			return "", fmt.Errorf("string number must contain 1..%d characters", number.Length)
		}
		return value, nil
	}
	parsed, err := bslnumber.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid numeric number")
	}
	canonical := parsed.String()
	precision, scale := decimalSize(canonical)
	if strings.HasPrefix(canonical, "-") || scale != 0 || precision > number.Length {
		return "", fmt.Errorf("numeric number must be a non-negative integer with at most %d digits", number.Length)
	}
	return canonical, nil
}

func documentWriteError(name string, err error) error {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" {
		return fmt.Errorf("document %s contains a duplicate number or reference: %w", name, err)
	}
	return fmt.Errorf("write document %s: %w", name, err)
}

func decodeDocumentNumber(number DocumentNumber, raw json.RawMessage) (string, error) {
	if number.Type == NumberType {
		return normalizeDocumentNumber(number, string(raw))
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	return normalizeDocumentNumber(number, value)
}

func normalizeDocumentDate(value time.Time) time.Time {
	return value.UTC().Truncate(100 * time.Microsecond)
}

func validateDocumentDate(value time.Time) error {
	if value.IsZero() || value.Year() < 1 || value.Year() > 3999 {
		return fmt.Errorf("date must be within years 1..3999")
	}
	return nil
}

func documentNumberPeriod(periodicity NumberPeriodicity, date time.Time) int {
	year, month, day := date.UTC().Date()
	switch periodicity {
	case NumberPeriodYear:
		return year
	case NumberPeriodQuarter:
		return year*10 + (int(month)-1)/3 + 1
	case NumberPeriodMonth:
		return year*100 + int(month)
	case NumberPeriodDay:
		return year*10_000 + int(month)*100 + day
	default:
		return 0
	}
}

func cloneDocumentRecord(source *DocumentRecord) *DocumentRecord {
	if source == nil {
		return nil
	}
	result := *source
	result.Attributes = make(map[uuid.UUID]Value, len(source.Attributes))
	for id, value := range source.Attributes {
		result.Attributes[id] = value
	}
	result.TableParts = make(map[uuid.UUID][]DocumentRow, len(source.TableParts))
	for id, rows := range source.TableParts {
		cloned := slices.Clone(rows)
		for index := range cloned {
			cloned[index].Values = make(map[uuid.UUID]Value, len(rows[index].Values))
			for field, value := range rows[index].Values {
				cloned[index].Values[field] = value
			}
		}
		result.TableParts[id] = cloned
	}
	return &result
}
