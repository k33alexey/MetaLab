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
	ErrCatalogRecordNotFound = errors.New("catalog record not found")
	ErrCatalogWriteConflict  = errors.New("catalog record was changed or removed")
)

type CatalogReference struct {
	CatalogID uuid.UUID
	ObjectID  uuid.UUID
}

func (reference CatalogReference) IsEmpty() bool {
	return reference.CatalogID.IsZero() || reference.ObjectID.IsZero()
}

type CatalogRow struct {
	Values map[uuid.UUID]Value
}

type CatalogRecord struct {
	Reference   CatalogReference
	Version     int64
	Code        string
	Description string
	Attributes  map[uuid.UUID]Value
	TableParts  map[uuid.UUID][]CatalogRow
}

type CatalogRepository struct {
	pool    *pgxpool.Pool
	catalog *Catalog
}

func NewCatalogRepository(pool *pgxpool.Pool, catalog *Catalog) (*CatalogRepository, error) {
	if pool == nil || catalog == nil {
		return nil, fmt.Errorf("catalog repository requires PostgreSQL and metadata catalog")
	}
	return &CatalogRepository{pool: pool, catalog: catalog}, nil
}

func (repository *CatalogRepository) New(ctx context.Context, name string, handler CatalogEventHandler) (*CatalogRecord, error) {
	definition, ok := repository.catalog.CatalogDefinition(name)
	if !ok {
		return nil, fmt.Errorf("unknown catalog %q", name)
	}
	id, err := uuid.New()
	if err != nil {
		return nil, err
	}
	record := &CatalogRecord{
		Reference:  CatalogReference{CatalogID: definition.ID, ObjectID: id},
		Attributes: make(map[uuid.UUID]Value), TableParts: make(map[uuid.UUID][]CatalogRow),
	}
	reference := record.Reference
	for _, part := range definition.TableParts {
		record.TableParts[part.ID] = []CatalogRow{}
	}
	if err := dispatchCatalogEvent(ctx, handler, CatalogEventFill, record); err != nil {
		return nil, err
	}
	if record.Reference != reference || record.Version != 0 {
		return nil, fmt.Errorf("catalog fill event changed immutable record identity")
	}
	return record, nil
}

func (repository *CatalogRepository) Save(ctx context.Context, record *CatalogRecord, handler CatalogEventHandler) error {
	if record == nil {
		return fmt.Errorf("catalog record is required")
	}
	working := cloneCatalogRecord(record)
	definition, ok := repository.catalog.CatalogByID(working.Reference.CatalogID)
	if !ok {
		return fmt.Errorf("unknown catalog %s", working.Reference.CatalogID)
	}
	reference, expectedVersion := working.Reference, working.Version
	if err := dispatchCatalogEvent(ctx, handler, CatalogEventFillCheck, working); err != nil {
		return err
	}
	if working.Reference != reference || working.Version != expectedVersion {
		return fmt.Errorf("catalog fill-check event changed immutable record identity")
	}
	if err := dispatchCatalogEvent(ctx, handler, CatalogEventBefore, working); err != nil {
		return err
	}
	if working.Reference != reference || working.Version != expectedVersion {
		return fmt.Errorf("catalog before-write event changed immutable record identity")
	}
	if err := repository.normalizeRecord(definition, working); err != nil {
		return err
	}
	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin catalog write: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	version, err := repository.writeRecord(ctx, transaction, definition, working)
	if err != nil {
		return err
	}
	working.Version = version
	if err := repository.writeTableParts(ctx, transaction, definition, working); err != nil {
		return err
	}
	for _, event := range []CatalogEvent{CatalogEventOnWrite, CatalogEventAfter} {
		if err := dispatchCatalogEvent(ctx, handler, event, cloneCatalogRecord(working)); err != nil {
			return err
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit catalog write: %w", err)
	}
	*record = *working
	return nil
}

func (repository *CatalogRepository) Get(ctx context.Context, reference CatalogReference) (*CatalogRecord, error) {
	definition, ok := repository.catalog.CatalogByID(reference.CatalogID)
	if !ok || reference.ObjectID.IsZero() {
		return nil, ErrCatalogRecordNotFound
	}
	table, _ := PhysicalCatalogTable(definition.ID)
	statement := "SELECT to_jsonb(item) FROM " + qualifiedCatalogTable(table) + " AS item WHERE ref = $1"
	var encoded []byte
	if err := repository.pool.QueryRow(ctx, statement, reference.ObjectID.String()).Scan(&encoded); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCatalogRecordNotFound
	} else if err != nil {
		return nil, fmt.Errorf("read catalog %s: %w", definition.Name, err)
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

func (repository *CatalogRepository) FindByCode(ctx context.Context, name, code string) (CatalogReference, bool, error) {
	definition, ok := repository.catalog.CatalogDefinition(name)
	if !ok {
		return CatalogReference{}, false, fmt.Errorf("unknown catalog %q", name)
	}
	code, err := normalizeCatalogCode(definition.Code, code)
	if err != nil {
		return CatalogReference{}, false, err
	}
	table, _ := PhysicalCatalogTable(definition.ID)
	statement := "SELECT ref::text FROM " + qualifiedCatalogTable(table) + " WHERE code = $1 ORDER BY ref LIMIT 1"
	var idText string
	if err := repository.pool.QueryRow(ctx, statement, code).Scan(&idText); errors.Is(err, pgx.ErrNoRows) {
		return CatalogReference{}, false, nil
	} else if err != nil {
		return CatalogReference{}, false, fmt.Errorf("find catalog %s by code: %w", definition.Name, err)
	}
	id, err := uuid.Parse(idText)
	if err != nil {
		return CatalogReference{}, false, fmt.Errorf("parse catalog reference: %w", err)
	}
	return CatalogReference{CatalogID: definition.ID, ObjectID: id}, true, nil
}

func (repository *CatalogRepository) normalizeRecord(definition CatalogDefinition, record *CatalogRecord) error {
	if record.Reference.CatalogID != definition.ID || record.Reference.ObjectID.IsZero() || record.Version < 0 {
		return fmt.Errorf("invalid catalog %s reference or version", definition.Name)
	}
	code, err := normalizeCatalogCode(definition.Code, record.Code)
	if err != nil {
		return fmt.Errorf("catalog %s code: %w", definition.Name, err)
	}
	record.Code = code
	if !utf8.ValidString(record.Description) || utf8.RuneCountInString(record.Description) > definition.DescriptionLength {
		return fmt.Errorf("catalog %s description exceeds %d characters", definition.Name, definition.DescriptionLength)
	}
	attributes, err := repository.normalizeAttributes(definition.Name, definition.Attributes, record.Attributes)
	if err != nil {
		return err
	}
	record.Attributes = attributes
	if record.TableParts == nil {
		record.TableParts = make(map[uuid.UUID][]CatalogRow, len(definition.TableParts))
	}
	knownParts := make(map[uuid.UUID]bool, len(definition.TableParts))
	for _, part := range definition.TableParts {
		knownParts[part.ID] = true
		rows := record.TableParts[part.ID]
		if len(rows) > 1_000_000 {
			return fmt.Errorf("catalog %s table part %s exceeds 1000000 rows", definition.Name, part.Name)
		}
		normalizedRows := make([]CatalogRow, len(rows))
		for index, row := range rows {
			values, err := repository.normalizeAttributes(definition.Name+"."+part.Name, part.Attributes, row.Values)
			if err != nil {
				return fmt.Errorf("row %d: %w", index+1, err)
			}
			normalizedRows[index] = CatalogRow{Values: values}
		}
		record.TableParts[part.ID] = normalizedRows
	}
	for id := range record.TableParts {
		if !knownParts[id] {
			return fmt.Errorf("catalog %s contains unknown table part %s", definition.Name, id)
		}
	}
	return nil
}

func (repository *CatalogRepository) normalizeAttributes(owner string, definitions []Attribute, values map[uuid.UUID]Value) (map[uuid.UUID]Value, error) {
	known := make(map[uuid.UUID]Attribute, len(definitions))
	result := make(map[uuid.UUID]Value, len(values))
	for _, attribute := range definitions {
		known[attribute.ID] = attribute
		value, present := values[attribute.ID]
		if !present {
			if attribute.Required {
				return nil, fmt.Errorf("%s attribute %s is required", owner, attribute.Name)
			}
			continue
		}
		normalized, err := repository.catalog.normalizeTypes("attribute "+owner+"."+attribute.Name, attribute.Types, value)
		if err != nil {
			return nil, err
		}
		result[attribute.ID] = normalized
	}
	for id := range values {
		if _, ok := known[id]; !ok {
			return nil, fmt.Errorf("%s contains unknown attribute %s", owner, id)
		}
	}
	return result, nil
}

func (repository *CatalogRepository) writeRecord(ctx context.Context, transaction pgx.Tx, definition CatalogDefinition, record *CatalogRecord) (int64, error) {
	table, _ := PhysicalCatalogTable(definition.ID)
	columns := []string{"ref", "code", "description"}
	arguments := []any{record.Reference.ObjectID.String(), record.Code, record.Description}
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
			return 0, catalogWriteError(definition.Name, err)
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
		return 0, ErrCatalogWriteConflict
	} else if err != nil {
		return 0, catalogWriteError(definition.Name, err)
	}
	return version, nil
}

func (repository *CatalogRepository) writeTableParts(ctx context.Context, transaction pgx.Tx, definition CatalogDefinition, record *CatalogRecord) error {
	for _, part := range definition.TableParts {
		table, _ := PhysicalCatalogTable(part.ID)
		if _, err := transaction.Exec(ctx, "DELETE FROM "+qualifiedCatalogTable(table)+" WHERE owner_ref = $1", record.Reference.ObjectID.String()); err != nil {
			return fmt.Errorf("clear catalog table part %s: %w", part.Name, err)
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
			return fmt.Errorf("write catalog table part %s: %w", part.Name, err)
		}
		if written != int64(len(partRows)) {
			return fmt.Errorf("write catalog table part %s: copied %d of %d rows", part.Name, written, len(partRows))
		}
	}
	return nil
}

func (repository *CatalogRepository) decodeRecord(definition CatalogDefinition, encoded []byte) (*CatalogRecord, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, fmt.Errorf("decode catalog %s row: %w", definition.Name, err)
	}
	objectID, err := decodeJSONUUID(fields["ref"])
	if err != nil {
		return nil, err
	}
	record := &CatalogRecord{Reference: CatalogReference{CatalogID: definition.ID, ObjectID: objectID}, Attributes: make(map[uuid.UUID]Value), TableParts: make(map[uuid.UUID][]CatalogRow)}
	if err := json.Unmarshal(fields["version"], &record.Version); err != nil || record.Version < 1 {
		return nil, fmt.Errorf("decode catalog %s version", definition.Name)
	}
	if record.Code, err = decodeCatalogCode(definition.Code, fields["code"]); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(fields["description"], &record.Description); err != nil {
		return nil, fmt.Errorf("decode catalog %s description: %w", definition.Name, err)
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
			return nil, fmt.Errorf("decode catalog %s attribute %s: %w", definition.Name, attribute.Name, err)
		}
		record.Attributes[attribute.ID] = value
	}
	return record, nil
}

func (repository *CatalogRepository) readTableParts(ctx context.Context, definition CatalogDefinition, record *CatalogRecord) error {
	for _, part := range definition.TableParts {
		table, _ := PhysicalCatalogTable(part.ID)
		rows, err := repository.pool.Query(ctx, "SELECT to_jsonb(item) FROM "+qualifiedCatalogTable(table)+" AS item WHERE owner_ref = $1 ORDER BY line_no", record.Reference.ObjectID.String())
		if err != nil {
			return fmt.Errorf("read catalog table part %s: %w", part.Name, err)
		}
		decodedRows := make([]CatalogRow, 0)
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
			row := CatalogRow{Values: make(map[uuid.UUID]Value)}
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
					return fmt.Errorf("decode catalog table part %s attribute %s: %w", part.Name, attribute.Name, err)
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

func normalizeCatalogCode(code CatalogCode, value string) (string, error) {
	if code.Type == StringType {
		if !utf8.ValidString(value) || value == "" || utf8.RuneCountInString(value) > code.Length {
			return "", fmt.Errorf("string code must contain 1..%d characters", code.Length)
		}
		return value, nil
	}
	number, err := bslnumber.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid numeric code")
	}
	canonical := number.String()
	precision, scale := decimalSize(canonical)
	if strings.HasPrefix(canonical, "-") || scale != 0 || precision > code.Length {
		return "", fmt.Errorf("numeric code must be a non-negative integer with at most %d digits", code.Length)
	}
	return canonical, nil
}

func databaseAttributeValue(storage attributeStorage, value Value) (any, error) {
	if storage.composite {
		encoded, err := json.Marshal(value)
		return string(encoded), err
	}
	switch storage.valueType {
	case StringType:
		return value.Data, nil
	case NumberType:
		return value.Data, nil
	case BooleanType:
		return value.Data == "true", nil
	case DateType:
		return time.Parse(time.RFC3339Nano, value.Data)
	case UUIDType, EnumerationType, CatalogType:
		return value.Data, nil
	default:
		return nil, fmt.Errorf("unsupported database value type %s", storage.valueType)
	}
}

func decodeDatabaseAttribute(storage attributeStorage, raw json.RawMessage) (Value, error) {
	if storage.composite {
		var value Value
		if err := json.Unmarshal(raw, &value); err != nil {
			return Value{}, err
		}
		return value, nil
	}
	value := Value{Kind: storage.valueType}
	switch storage.valueType {
	case StringType, DateType, UUIDType, EnumerationType, CatalogType:
		if err := json.Unmarshal(raw, &value.Data); err != nil {
			return Value{}, err
		}
	case NumberType:
		value.Data = string(raw)
	case BooleanType:
		value.Data = string(raw)
	default:
		return Value{}, fmt.Errorf("unsupported database value type %s", storage.valueType)
	}
	return value, nil
}

func decodeCatalogCode(code CatalogCode, raw json.RawMessage) (string, error) {
	if code.Type == NumberType {
		return normalizeCatalogCode(code, string(raw))
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	return normalizeCatalogCode(code, value)
}

func decodeJSONUUID(raw json.RawMessage) (uuid.UUID, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return uuid.UUID{}, fmt.Errorf("decode UUID: %w", err)
	}
	return uuid.Parse(value)
}

func catalogWriteError(name string, err error) error {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" {
		return fmt.Errorf("catalog %s contains a duplicate code or reference: %w", name, err)
	}
	return fmt.Errorf("write catalog %s: %w", name, err)
}

func qualifiedCatalogTable(table string) string {
	return pgx.Identifier{schemadiff.ApplicationSchema, table}.Sanitize()
}

func quoteCatalogColumns(columns []string) []string {
	result := make([]string, len(columns))
	for index, column := range columns {
		result[index] = pgx.Identifier{column}.Sanitize()
	}
	return result
}

func cloneCatalogRecord(source *CatalogRecord) *CatalogRecord {
	if source == nil {
		return nil
	}
	result := *source
	result.Attributes = make(map[uuid.UUID]Value, len(source.Attributes))
	for id, value := range source.Attributes {
		result.Attributes[id] = value
	}
	result.TableParts = make(map[uuid.UUID][]CatalogRow, len(source.TableParts))
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
