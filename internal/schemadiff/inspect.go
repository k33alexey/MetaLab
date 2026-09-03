package schemadiff

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Inspect reads one application schema through PostgreSQL catalogs without changing it.
func Inspect(ctx context.Context, pool *pgxpool.Pool, schemaName string) (Schema, error) {
	if pool == nil || !sqlIdentifier.MatchString(schemaName) {
		return Schema{}, fmt.Errorf("invalid PostgreSQL schema inspection request")
	}
	transaction, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return Schema{}, fmt.Errorf("begin PostgreSQL schema inspection: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	result, err := inspectCatalog(ctx, transaction, schemaName)
	if err != nil {
		return Schema{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return Schema{}, fmt.Errorf("finish PostgreSQL schema inspection: %w", err)
	}
	return result, nil
}

type catalogQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func inspectCatalog(ctx context.Context, query catalogQuerier, schemaName string) (Schema, error) {
	result := Schema{Name: schemaName, Tables: make([]Table, 0)}
	if err := query.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname = $1)", schemaName).Scan(&result.Exists); err != nil {
		return Schema{}, fmt.Errorf("inspect PostgreSQL schema: %w", err)
	}
	if !result.Exists {
		return result, nil
	}
	rows, err := query.Query(ctx, `
SELECT relation.relname
FROM pg_class AS relation
JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
WHERE namespace.nspname = $1 AND relation.relkind IN ('r', 'p')
ORDER BY relation.relname`, schemaName)
	if err != nil {
		return Schema{}, fmt.Errorf("list PostgreSQL tables: %w", err)
	}
	for rows.Next() {
		var table Table
		if err := rows.Scan(&table.Name); err != nil {
			rows.Close()
			return Schema{}, fmt.Errorf("scan PostgreSQL table: %w", err)
		}
		result.Tables = append(result.Tables, table)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Schema{}, fmt.Errorf("iterate PostgreSQL tables: %w", err)
	}
	rows.Close()
	for index := range result.Tables {
		table := &result.Tables[index]
		if table.Columns, err = inspectColumns(ctx, query, schemaName, table.Name); err != nil {
			return Schema{}, err
		}
		if table.Indexes, err = inspectIndexes(ctx, query, schemaName, table.Name); err != nil {
			return Schema{}, err
		}
		if table.Constraints, err = inspectConstraints(ctx, query, schemaName, table.Name); err != nil {
			return Schema{}, err
		}
	}
	if err := result.normalizeActual(); err != nil {
		return Schema{}, fmt.Errorf("normalize inspected PostgreSQL schema: %w", err)
	}
	return result, nil
}

func inspectColumns(ctx context.Context, query catalogQuerier, schemaName, tableName string) ([]Column, error) {
	rows, err := query.Query(ctx, `
SELECT attribute.attname, pg_catalog.format_type(attribute.atttypid, attribute.atttypmod),
       NOT attribute.attnotnull, COALESCE(pg_get_expr(default_value.adbin, default_value.adrelid), '')
FROM pg_attribute AS attribute
JOIN pg_class AS relation ON relation.oid = attribute.attrelid
JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
LEFT JOIN pg_attrdef AS default_value ON default_value.adrelid = relation.oid AND default_value.adnum = attribute.attnum
WHERE namespace.nspname = $1 AND relation.relname = $2
  AND attribute.attnum > 0 AND NOT attribute.attisdropped
ORDER BY attribute.attname`, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("list PostgreSQL columns for %s: %w", tableName, err)
	}
	defer rows.Close()
	items := make([]Column, 0)
	for rows.Next() {
		var item Column
		if err := rows.Scan(&item.Name, &item.Type, &item.Nullable, &item.Default); err != nil {
			return nil, fmt.Errorf("scan PostgreSQL column for %s: %w", tableName, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate PostgreSQL columns for %s: %w", tableName, err)
	}
	return items, nil
}

func inspectIndexes(ctx context.Context, query catalogQuerier, schemaName, tableName string) ([]Index, error) {
	rows, err := query.Query(ctx, `
SELECT index_relation.relname, indexed.indisunique, access_method.amname,
       ARRAY(SELECT pg_get_indexdef(indexed.indexrelid, position, TRUE)
             FROM generate_series(1, indexed.indnkeyatts) AS position ORDER BY position),
       ARRAY(SELECT pg_get_indexdef(indexed.indexrelid, position, TRUE)
             FROM generate_series(indexed.indnkeyatts + 1, indexed.indnatts) AS position ORDER BY position),
       COALESCE(pg_get_expr(indexed.indpred, indexed.indrelid), '')
FROM pg_index AS indexed
JOIN pg_class AS relation ON relation.oid = indexed.indrelid
JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
JOIN pg_class AS index_relation ON index_relation.oid = indexed.indexrelid
JOIN pg_am AS access_method ON access_method.oid = index_relation.relam
WHERE namespace.nspname = $1 AND relation.relname = $2 AND indexed.indisvalid
  AND NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conindid = indexed.indexrelid)
ORDER BY index_relation.relname`, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("list PostgreSQL indexes for %s: %w", tableName, err)
	}
	defer rows.Close()
	items := make([]Index, 0)
	for rows.Next() {
		var item Index
		if err := rows.Scan(&item.Name, &item.Unique, &item.Method, &item.Keys, &item.Include, &item.Predicate); err != nil {
			return nil, fmt.Errorf("scan PostgreSQL index for %s: %w", tableName, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate PostgreSQL indexes for %s: %w", tableName, err)
	}
	return items, nil
}

func inspectConstraints(ctx context.Context, query catalogQuerier, schemaName, tableName string) ([]Constraint, error) {
	rows, err := query.Query(ctx, `
SELECT constraint_name.conname,
       CASE constraint_name.contype
         WHEN 'p' THEN 'primary_key' WHEN 'u' THEN 'unique' WHEN 'f' THEN 'foreign_key'
         WHEN 'c' THEN 'check' WHEN 'x' THEN 'exclusion'
       END,
       pg_get_constraintdef(constraint_name.oid, TRUE)
FROM pg_constraint AS constraint_name
JOIN pg_class AS relation ON relation.oid = constraint_name.conrelid
JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
WHERE namespace.nspname = $1 AND relation.relname = $2
  AND constraint_name.contype IN ('p', 'u', 'f', 'c', 'x')
ORDER BY constraint_name.conname`, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("list PostgreSQL constraints for %s: %w", tableName, err)
	}
	defer rows.Close()
	items := make([]Constraint, 0)
	for rows.Next() {
		var item Constraint
		if err := rows.Scan(&item.Name, &item.Type, &item.Definition); err != nil {
			return nil, fmt.Errorf("scan PostgreSQL constraint for %s: %w", tableName, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate PostgreSQL constraints for %s: %w", tableName, err)
	}
	return items, nil
}
