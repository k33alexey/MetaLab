package metadata

import (
	"context"
	"fmt"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

const maxBasicListPageSize = 100

type CatalogListPage struct {
	Records    []*CatalogRecord
	NextCursor *uuid.UUID
}

type DocumentListPage struct {
	Records    []*DocumentRecord
	NextCursor *uuid.UUID
}

// List returns one bounded, stable UUID-keyset page without loading table parts.
func (repository *CatalogRepository) List(ctx context.Context, name string, cursor *uuid.UUID, limit int) (CatalogListPage, error) {
	definition, ok := repository.catalog.CatalogDefinition(name)
	if !ok {
		return CatalogListPage{}, fmt.Errorf("unknown catalog %q", name)
	}
	if err := validateBasicListPage(cursor, limit); err != nil {
		return CatalogListPage{}, err
	}
	table, _ := PhysicalCatalogTable(definition.ID)
	return repository.listPage(ctx, definition, "SELECT to_jsonb(item) FROM "+qualifiedCatalogTable(table)+" AS item WHERE ref > $1 ORDER BY ref LIMIT $2", cursor, limit)
}

func (repository *CatalogRepository) listPage(ctx context.Context, definition CatalogDefinition, statement string, cursor *uuid.UUID, limit int) (CatalogListPage, error) {
	query, err := queryData(ctx, repository.pool)
	if err != nil {
		return CatalogListPage{}, err
	}
	rows, err := query.Query(ctx, statement, basicListCursor(cursor), limit+1)
	if err != nil {
		return CatalogListPage{}, recordDataError(ctx, repository.pool, fmt.Errorf("list catalog %s: %w", definition.Name, err))
	}
	defer rows.Close()
	result := CatalogListPage{Records: make([]*CatalogRecord, 0, limit)}
	for rows.Next() {
		var encoded []byte
		if err := rows.Scan(&encoded); err != nil {
			return CatalogListPage{}, err
		}
		record, err := repository.decodeRecord(definition, encoded)
		if err != nil {
			return CatalogListPage{}, err
		}
		if len(result.Records) == limit {
			next := result.Records[len(result.Records)-1].Reference.ObjectID
			result.NextCursor = &next
			break
		}
		result.Records = append(result.Records, record)
	}
	if err := rows.Err(); err != nil {
		return CatalogListPage{}, recordDataError(ctx, repository.pool, err)
	}
	return result, nil
}

// List returns one bounded, stable UUID-keyset page without loading table parts.
func (repository *DocumentRepository) List(ctx context.Context, name string, cursor *uuid.UUID, limit int) (DocumentListPage, error) {
	definition, ok := repository.catalog.DocumentDefinition(name)
	if !ok {
		return DocumentListPage{}, fmt.Errorf("unknown document %q", name)
	}
	if err := validateBasicListPage(cursor, limit); err != nil {
		return DocumentListPage{}, err
	}
	table, _ := PhysicalDocumentTable(definition.ID)
	query, err := queryData(ctx, repository.pool)
	if err != nil {
		return DocumentListPage{}, err
	}
	rows, err := query.Query(ctx, "SELECT to_jsonb(item) FROM "+qualifiedCatalogTable(table)+" AS item WHERE ref > $1 ORDER BY ref LIMIT $2", basicListCursor(cursor), limit+1)
	if err != nil {
		return DocumentListPage{}, recordDataError(ctx, repository.pool, fmt.Errorf("list document %s: %w", definition.Name, err))
	}
	defer rows.Close()
	result := DocumentListPage{Records: make([]*DocumentRecord, 0, limit)}
	for rows.Next() {
		var encoded []byte
		if err := rows.Scan(&encoded); err != nil {
			return DocumentListPage{}, err
		}
		record, err := repository.decodeRecord(definition, encoded)
		if err != nil {
			return DocumentListPage{}, err
		}
		if len(result.Records) == limit {
			next := result.Records[len(result.Records)-1].Reference.ObjectID
			result.NextCursor = &next
			break
		}
		result.Records = append(result.Records, record)
	}
	if err := rows.Err(); err != nil {
		return DocumentListPage{}, recordDataError(ctx, repository.pool, err)
	}
	return result, nil
}

func validateBasicListPage(cursor *uuid.UUID, limit int) error {
	if cursor != nil && cursor.IsZero() {
		return fmt.Errorf("list cursor must be a non-zero UUID")
	}
	if limit < 1 || limit > maxBasicListPageSize {
		return fmt.Errorf("list page size must be 1..%d", maxBasicListPageSize)
	}
	return nil
}

func basicListCursor(cursor *uuid.UUID) string {
	if cursor == nil {
		return "00000000-0000-0000-0000-000000000000"
	}
	return cursor.String()
}
