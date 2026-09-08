package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/querylang"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	maxTemporaryTables       = 256
	maxTemporaryManagerBytes = 64 << 20
)

type queryTemporaryTable struct {
	name         string
	columns      []queryColumn
	encodedRows  []byte
	rowCount     int
	indexColumns []int
}

type temporaryTableManagerObject struct {
	mu      sync.RWMutex
	runtime *Runtime
	closed  bool
	version uint64
	tables  map[string]queryTemporaryTable
}

func (*temporaryTableManagerObject) RuntimeTypeName() string { return "TempTablesManager" }

func (object *temporaryTableManagerObject) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*temporaryTableManagerObject)
	return ok && candidate == object
}

func (object *temporaryTableManagerObject) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	object.mu.RLock()
	defer object.mu.RUnlock()
	size := uint64(256)
	if size > limit {
		return limit, false
	}
	for _, table := range object.tables {
		addition := uint64(len(table.name) + len(table.encodedRows) + len(table.columns)*192 + len(table.indexColumns)*8 + 64)
		for _, column := range table.columns {
			addition += uint64(len(column.name) + len(column.sql) + len(column.storage.sqlType))
		}
		if addition > limit-min(size, limit) {
			return limit, false
		}
		size += addition
	}
	return size, size <= limit
}

func (runtime *Runtime) constructTemporaryTableManager(arguments []bytecode.Value) (bytecode.Value, error) {
	if len(arguments) != 0 {
		return bytecode.Undefined(), fmt.Errorf("TempTablesManager expects no arguments")
	}
	return bytecode.Object(&temporaryTableManagerObject{runtime: runtime, tables: make(map[string]queryTemporaryTable)})
}

func (object *temporaryTableManagerObject) snapshot() (map[string]queryTemporaryTable, uint64, error) {
	object.mu.RLock()
	defer object.mu.RUnlock()
	if object.closed {
		return nil, 0, fmt.Errorf("temporary table manager is closed")
	}
	return cloneTemporaryTables(object.tables), object.version, nil
}

func (object *temporaryTableManagerObject) apply(expected uint64, tables map[string]queryTemporaryTable) (uint64, error) {
	object.mu.Lock()
	defer object.mu.Unlock()
	if object.closed {
		return 0, fmt.Errorf("temporary table manager is closed")
	}
	if object.version != expected {
		return 0, fmt.Errorf("temporary table manager was changed concurrently")
	}
	if len(tables) > maxTemporaryTables {
		return 0, fmt.Errorf("temporary table manager cannot contain more than %d tables", maxTemporaryTables)
	}
	if memory := temporaryTablesMemory(tables); memory > maxTemporaryManagerBytes {
		return 0, fmt.Errorf("temporary tables exceed %d MiB", maxTemporaryManagerBytes>>20)
	}
	object.tables = tables
	object.version++
	return object.version, nil
}

func (object *temporaryTableManagerObject) restore(expected uint64, tables map[string]queryTemporaryTable, version uint64) {
	object.mu.Lock()
	defer object.mu.Unlock()
	if object.closed || object.version != expected {
		return
	}
	object.tables = cloneTemporaryTables(tables)
	object.version = version
}

func (object *temporaryTableManagerObject) close() {
	object.mu.Lock()
	object.closed = true
	object.tables = nil
	object.version++
	object.mu.Unlock()
}

func cloneTemporaryTables(source map[string]queryTemporaryTable) map[string]queryTemporaryTable {
	result := make(map[string]queryTemporaryTable, len(source))
	for name, table := range source {
		result[name] = table
	}
	return result
}

func temporaryTablesMemory(tables map[string]queryTemporaryTable) uint64 {
	result := uint64(0)
	for name, table := range tables {
		result += uint64(len(name) + len(table.name) + len(table.encodedRows) + len(table.columns)*192 + len(table.indexColumns)*8 + 64)
		for _, column := range table.columns {
			result += uint64(len(column.name) + len(column.storage.sqlType))
		}
	}
	return result
}

func temporaryTableKey(name string) string { return strings.ToLower(name) }

func queryUsesTemporarySource(query querylang.Query) bool {
	if len(query.Source.Path) == 1 {
		return true
	}
	for _, join := range query.Joins {
		if len(join.Source.Path) == 1 {
			return true
		}
	}
	return false
}

func (runtime *Runtime) materializeTemporarySources(ctx context.Context, transaction pgx.Tx, query querylang.Query, tables map[string]queryTemporaryTable) (map[string]queryMaterializedTemporaryTable, func() error, error) {
	result := make(map[string]queryMaterializedTemporaryTable)
	created := make([]string, 0, len(query.Joins)+1)
	cleanup := func() error {
		for index := len(created) - 1; index >= 0; index-- {
			if _, err := transaction.Exec(ctx, "DROP TABLE IF EXISTS "+pgx.Identifier{"pg_temp", created[index]}.Sanitize()); err != nil {
				return fmt.Errorf("drop materialized temporary table: %w", err)
			}
		}
		return nil
	}
	sources := make([]querylang.Source, 0, len(query.Joins)+1)
	sources = append(sources, query.Source)
	for _, join := range query.Joins {
		sources = append(sources, join.Source)
	}
	for _, source := range sources {
		if len(source.Path) != 1 {
			continue
		}
		key := temporaryTableKey(source.Path[0])
		if _, ok := result[key]; ok {
			continue
		}
		table, ok := tables[key]
		if !ok {
			_ = cleanup()
			return nil, func() error { return nil }, querySemanticError(source.Position, "неизвестная временная таблица "+source.Path[0])
		}
		physical := "ml_temp_" + strings.ReplaceAll(uuid.MustNew().String(), "-", "")
		columns, definitions := temporarySQLColumns(table)
		if len(columns) == 0 {
			_ = cleanup()
			return nil, func() error { return nil }, fmt.Errorf("temporary table %s has no columns", table.name)
		}
		identifier := pgx.Identifier{physical}.Sanitize()
		if _, err := transaction.Exec(ctx, "CREATE TEMP TABLE "+identifier+" ("+strings.Join(definitions, ", ")+") ON COMMIT DROP"); err != nil {
			_ = cleanup()
			return nil, func() error { return nil }, fmt.Errorf("materialize temporary table %s: %w", table.name, err)
		}
		created = append(created, physical)
		if table.rowCount != 0 {
			statement := "INSERT INTO " + identifier + " (" + strings.Join(columns, ", ") + ") SELECT " + strings.Join(columns, ", ") + " FROM jsonb_to_recordset($1::jsonb) AS rows(" + strings.Join(definitions, ", ") + ")"
			if _, err := transaction.Exec(ctx, statement, string(table.encodedRows)); err != nil {
				_ = cleanup()
				return nil, func() error { return nil }, fmt.Errorf("load temporary table %s: %w", table.name, err)
			}
		}
		if len(table.indexColumns) != 0 {
			indexed := make([]string, len(table.indexColumns))
			for index, column := range table.indexColumns {
				indexed[index] = columns[column]
			}
			if _, err := transaction.Exec(ctx, "CREATE INDEX ON "+identifier+" ("+strings.Join(indexed, ", ")+")"); err != nil {
				_ = cleanup()
				return nil, func() error { return nil }, fmt.Errorf("index temporary table %s: %w", table.name, err)
			}
		}
		if table.rowCount != 0 {
			if _, err := transaction.Exec(ctx, "ANALYZE "+identifier); err != nil {
				_ = cleanup()
				return nil, func() error { return nil }, fmt.Errorf("analyze temporary table %s: %w", table.name, err)
			}
		}
		result[key] = queryMaterializedTemporaryTable{fromSQL: pgx.Identifier{"pg_temp", physical}.Sanitize(), table: table}
	}
	return result, cleanup, nil
}

func temporarySQLColumns(table queryTemporaryTable) ([]string, []string) {
	columns := make([]string, len(table.columns))
	definitions := make([]string, len(table.columns))
	for index, column := range table.columns {
		columns[index] = pgx.Identifier{fmt.Sprintf("c%d", index)}.Sanitize()
		definitions[index] = columns[index] + " " + temporarySQLType(column)
	}
	return columns, definitions
}

func temporarySQLType(column queryColumn) string {
	if column.kind == queryRecorderColumn || column.storage.composite {
		return "jsonb"
	}
	if column.kind == queryMovementKindColumn {
		return "smallint"
	}
	if column.storage.sqlType != "" {
		return column.storage.sqlType
	}
	switch column.storage.valueType {
	case NumberType:
		return "numeric"
	case BooleanType:
		return "boolean"
	case DateType:
		return "timestamptz"
	case UUIDType, EnumerationType, CatalogType, DocumentType:
		return "uuid"
	default:
		return "text"
	}
}

func buildTemporaryTable(name string, indexExpressions []querylang.Expression, result *queryResultObject, raw [][]json.RawMessage) (queryTemporaryTable, error) {
	if len(result.columns) == 0 || len(result.descriptors) != len(result.columns) {
		return queryTemporaryTable{}, fmt.Errorf("temporary table must contain typed columns")
	}
	columns := make([]queryColumn, len(result.columns))
	byName := make(map[string]int, len(result.columns))
	for index := range result.columns {
		column := result.descriptors[index]
		column.name = result.columns[index]
		column.sql = ""
		columns[index] = column
		byName[strings.ToLower(column.name)] = index
	}
	indexes := make([]int, 0, len(indexExpressions))
	seen := make(map[int]struct{}, len(indexExpressions))
	for _, expression := range indexExpressions {
		field, ok := expression.(querylang.Field)
		if !ok || len(field.Path) != 1 {
			return queryTemporaryTable{}, querySemanticError(expression.ExpressionPosition(), "ИНДЕКСИРОВАТЬ ПО допускает только поля результата")
		}
		index, ok := byName[strings.ToLower(field.Path[0])]
		if !ok {
			return queryTemporaryTable{}, querySemanticError(field.Position, "неизвестное поле индекса "+field.Path[0])
		}
		if _, duplicate := seen[index]; duplicate {
			return queryTemporaryTable{}, querySemanticError(field.Position, "поле индекса указано повторно")
		}
		if columns[index].kind == queryRecorderColumn || columns[index].storage.composite {
			return queryTemporaryTable{}, querySemanticError(field.Position, "индекс составного поля не поддерживается")
		}
		seen[index] = struct{}{}
		indexes = append(indexes, index)
	}
	rows := make([]map[string]json.RawMessage, len(raw))
	for rowIndex, source := range raw {
		if len(source) != len(columns) {
			return queryTemporaryTable{}, fmt.Errorf("temporary table row has invalid column count")
		}
		row := make(map[string]json.RawMessage, len(source))
		for columnIndex, value := range source {
			row[fmt.Sprintf("c%d", columnIndex)] = value
		}
		rows[rowIndex] = row
	}
	encoded, err := json.Marshal(rows)
	if err != nil {
		return queryTemporaryTable{}, fmt.Errorf("encode temporary table: %w", err)
	}
	return queryTemporaryTable{name: name, columns: columns, encodedRows: encoded, rowCount: len(rows), indexColumns: indexes}, nil
}
