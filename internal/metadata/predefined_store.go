package metadata

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// PredefinedSyncResult describes changes made from metadata without overwriting user fields.
type PredefinedSyncResult struct {
	Created  int
	Assigned int
	Detached int
}

// SynchronizePredefined creates missing predefined items and synchronizes their
// stable metadata identity. Existing user-editable values are preserved.
func (repository *CatalogRepository) SynchronizePredefined(ctx context.Context) (PredefinedSyncResult, error) {
	var result PredefinedSyncResult
	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return result, fmt.Errorf("begin predefined catalog synchronization: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	for _, definition := range repository.catalog.Catalogs {
		current, err := repository.predefinedIdentities(ctx, transaction, definition)
		if err != nil {
			return result, err
		}
		table, _ := PhysicalCatalogTable(definition.ID)
		qualified := qualifiedCatalogTable(table)
		if _, err := transaction.Exec(ctx, "UPDATE "+qualified+" SET predefined_name = NULL WHERE predefined_name IS NOT NULL"); err != nil {
			return result, fmt.Errorf("clear catalog %s predefined identities: %w", definition.Name, err)
		}
		desired := make(map[uuid.UUID]bool, len(definition.Predefined))
		for _, item := range definition.Predefined {
			desired[item.ID] = true
			var exists bool
			if err := transaction.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM "+qualified+" WHERE ref = $1)", item.ID.String()).Scan(&exists); err != nil {
				return result, fmt.Errorf("inspect predefined catalog item %s.%s: %w", definition.Name, item.Name, err)
			}
			if exists {
				increment := 0
				if current[item.ID] != item.Name {
					increment = 1
					result.Assigned++
				}
				if _, err := transaction.Exec(ctx, "UPDATE "+qualified+" SET predefined_name = $2, version = version + $3 WHERE ref = $1", item.ID.String(), item.Name, increment); err != nil {
					return result, fmt.Errorf("assign predefined catalog item %s.%s: %w", definition.Name, item.Name, err)
				}
				continue
			}
			record, err := repository.newPredefinedRecord(ctx, transaction, definition, item)
			if err != nil {
				return result, err
			}
			if _, err := repository.writeRecord(ctx, transaction, definition, record); err != nil {
				return result, err
			}
			result.Created++
		}
		for id := range current {
			if desired[id] {
				continue
			}
			if _, err := transaction.Exec(ctx, "UPDATE "+qualified+" SET version = version + 1 WHERE ref = $1", id.String()); err != nil {
				return result, fmt.Errorf("detach removed predefined catalog item %s: %w", definition.Name, err)
			}
			result.Detached++
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit predefined catalog synchronization: %w", err)
	}
	return result, nil
}

func (repository *CatalogRepository) predefinedIdentities(ctx context.Context, transaction pgx.Tx, definition CatalogDefinition) (map[uuid.UUID]string, error) {
	table, _ := PhysicalCatalogTable(definition.ID)
	rows, err := transaction.Query(ctx, "SELECT ref::text, predefined_name FROM "+qualifiedCatalogTable(table)+" WHERE predefined_name IS NOT NULL")
	if err != nil {
		return nil, fmt.Errorf("read catalog %s predefined identities: %w", definition.Name, err)
	}
	defer rows.Close()
	result := make(map[uuid.UUID]string)
	for rows.Next() {
		var idText, name string
		if err := rows.Scan(&idText, &name); err != nil {
			return nil, err
		}
		id, err := uuid.Parse(idText)
		if err != nil {
			return nil, fmt.Errorf("parse catalog %s predefined reference: %w", definition.Name, err)
		}
		result[id] = name
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (repository *CatalogRepository) newPredefinedRecord(ctx context.Context, transaction pgx.Tx, definition CatalogDefinition, item PredefinedCatalogItem) (*CatalogRecord, error) {
	record := &CatalogRecord{
		Reference: CatalogReference{CatalogID: definition.ID, ObjectID: item.ID}, Code: item.Code,
		Description: item.Description, PredefinedName: item.Name, Attributes: make(map[uuid.UUID]Value),
		TableParts: make(map[uuid.UUID][]CatalogRow),
	}
	for name, value := range item.Attributes {
		attribute, ok := findCatalogAttribute(definition.Attributes, name)
		if !ok {
			return nil, fmt.Errorf("predefined catalog item %s.%s has unknown attribute %s", definition.Name, item.Name, name)
		}
		record.Attributes[attribute.ID] = value
	}
	for _, part := range definition.TableParts {
		record.TableParts[part.ID] = []CatalogRow{}
	}
	if record.Code == "" && definition.Code.Auto {
		table, _ := PhysicalCatalogTable(definition.ID)
		value, err := nextObjectSequence(ctx, transaction, definition.ID, 0, table, "code", definition.Code.Type == StringType, false)
		if err != nil {
			return nil, fmt.Errorf("predefined catalog item %s.%s code: %w", definition.Name, item.Name, err)
		}
		record.Code, err = formatAutomaticIdentifier(value, definition.Code.Type, definition.Code.Length)
		if err != nil {
			return nil, fmt.Errorf("predefined catalog item %s.%s code: %w", definition.Name, item.Name, err)
		}
	}
	if err := repository.normalizeRecord(definition, record); err != nil {
		return nil, fmt.Errorf("predefined catalog item %s.%s: %w", definition.Name, item.Name, err)
	}
	return record, nil
}
