package metadata

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

var (
	ErrDeletionMarkRequired   = errors.New("object must be marked for deletion first")
	ErrPredefinedDeleteDenied = errors.New("predefined catalog item cannot be deleted")
	ErrObjectReferenced       = errors.New("object is referenced by other data")
)

type ReferenceUse struct {
	OwnerKind     string
	OwnerName     string
	Field         string
	OwnerObjectID uuid.UUID
	LineNumber    int
}

type ReferenceIntegrityError struct {
	Uses []ReferenceUse
}

func (err *ReferenceIntegrityError) Error() string {
	if len(err.Uses) == 0 {
		return ErrObjectReferenced.Error()
	}
	first := err.Uses[0]
	return fmt.Sprintf("%s: %s %s field %s", ErrObjectReferenced, first.OwnerKind, first.OwnerName, first.Field)
}

func (*ReferenceIntegrityError) Unwrap() error { return ErrObjectReferenced }

type DeletionRecord struct {
	ID           uuid.UUID
	ObjectKind   string
	MetadataID   uuid.UUID
	MetadataName string
	ObjectID     uuid.UUID
	Presentation string
	ActorID      *uuid.UUID
	DeletedAt    time.Time
}

// EnsureObjectIntegrityStorage creates platform-owned counters and deletion history.
func EnsureObjectIntegrityStorage(ctx context.Context, executor sqlExecutor) error {
	if executor == nil {
		return fmt.Errorf("object integrity storage executor is required")
	}
	_, err := executor.Exec(ctx, `
CREATE SCHEMA IF NOT EXISTS ml_core;
CREATE TABLE IF NOT EXISTS ml_core.object_sequences (
    metadata_id uuid NOT NULL,
    period integer NOT NULL,
    last_value numeric(128,0) NOT NULL CHECK (last_value > 0),
    PRIMARY KEY (metadata_id, period)
);
CREATE TABLE IF NOT EXISTS ml_core.object_deletions (
    id uuid PRIMARY KEY,
    object_kind character varying(16) NOT NULL,
    metadata_id uuid NOT NULL,
    metadata_name character varying(128) NOT NULL,
    object_id uuid NOT NULL,
    presentation text NOT NULL,
    actor_id uuid,
    deleted_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS object_deletions_time_idx
    ON ml_core.object_deletions(deleted_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS object_deletions_object_idx
    ON ml_core.object_deletions(metadata_id, object_id)`)
	if err != nil {
		return fmt.Errorf("ensure object integrity storage: %w", err)
	}
	return nil
}

type sequenceQuery interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func nextObjectSequence(ctx context.Context, query sequenceQuery, metadataID uuid.UUID, period int, table, column string, textual, periodic bool) (string, error) {
	if query == nil || metadataID.IsZero() || period < 0 {
		return "", fmt.Errorf("invalid automatic identifier request")
	}
	qualified := pgx.Identifier{schemadiff.ApplicationSchema, table}.Sanitize()
	quotedColumn := pgx.Identifier{column}.Sanitize()
	expression, predicates := quotedColumn, make([]string, 0, 2)
	if textual {
		expression = "(" + quotedColumn + ")::numeric"
		predicates = append(predicates, quotedColumn+" ~ '^[0-9]+$'")
	}
	if periodic {
		predicates = append(predicates, "number_period = $2")
	}
	where := ""
	if len(predicates) > 0 {
		where = " WHERE " + strings.Join(predicates, " AND ")
	}
	statement := `
INSERT INTO ml_core.object_sequences(metadata_id, period, last_value)
SELECT $1::uuid, $2::integer, GREATEST(COALESCE(MAX(` + expression + `), 0) + 1, 1)
FROM ` + qualified + where + `
ON CONFLICT (metadata_id, period) DO UPDATE
SET last_value = GREATEST(ml_core.object_sequences.last_value + 1, EXCLUDED.last_value)
RETURNING last_value::text`
	var result string
	if err := query.QueryRow(ctx, statement, metadataID.String(), period).Scan(&result); err != nil {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "22003" {
			return "", fmt.Errorf("automatic identifier sequence is exhausted")
		}
		return "", fmt.Errorf("allocate automatic identifier: %w", err)
	}
	return result, nil
}

func formatAutomaticIdentifier(value string, kind TypeKind, length int) (string, error) {
	if value == "" || strings.Trim(value, "0123456789") != "" || len(value) > length {
		return "", fmt.Errorf("automatic identifier exceeds %d digits", length)
	}
	if kind == StringType {
		return strings.Repeat("0", length-len(value)) + value, nil
	}
	return value, nil
}

type objectIdentity struct {
	kind       TypeKind
	metadataID uuid.UUID
	objectID   uuid.UUID
}

type referenceSource struct {
	table           string
	ownerKind       string
	ownerName       string
	ownerMetadataID uuid.UUID
	field           string
	column          string
	ownerColumn     string
	lineColumn      bool
	composite       bool
	discriminator   string
	discriminatorID uuid.UUID
}

func (catalog *Catalog) referenceSources(target objectIdentity) ([]referenceSource, error) {
	var result []referenceSource
	appendAttributes := func(kind, name string, metadataID uuid.UUID, table, ownerColumn, prefix string, attributes []Attribute, lineColumn bool) error {
		for _, attribute := range attributes {
			allowed, err := catalog.allowsObjectReference(attribute.Types, target.kind, target.metadataID)
			if err != nil {
				return err
			}
			if !allowed {
				continue
			}
			storage, err := catalog.attributeStorage(attribute.Types)
			if err != nil {
				return err
			}
			column, err := PhysicalAttributeColumn(attribute.ID)
			if err != nil {
				return err
			}
			result = append(result, referenceSource{
				table: table, ownerKind: kind, ownerName: name, ownerMetadataID: metadataID,
				field: prefix + attribute.Name, column: column, ownerColumn: ownerColumn,
				lineColumn: lineColumn, composite: storage.composite,
			})
		}
		return nil
	}
	for _, definition := range catalog.Catalogs {
		table, _ := PhysicalCatalogTable(definition.ID)
		if err := appendAttributes("catalog", definition.Name, definition.ID, table, "ref", "", definition.Attributes, false); err != nil {
			return nil, err
		}
		for _, part := range definition.TableParts {
			table, _ := PhysicalCatalogTable(part.ID)
			if err := appendAttributes("catalog", definition.Name, definition.ID, table, "owner_ref", part.Name+".", part.Attributes, true); err != nil {
				return nil, err
			}
		}
	}
	for _, definition := range catalog.Documents {
		table, _ := PhysicalDocumentTable(definition.ID)
		if err := appendAttributes("document", definition.Name, definition.ID, table, "ref", "", definition.Attributes, false); err != nil {
			return nil, err
		}
		for _, part := range definition.TableParts {
			table, _ := PhysicalDocumentTable(part.ID)
			if err := appendAttributes("document", definition.Name, definition.ID, table, "owner_ref", part.Name+".", part.Attributes, true); err != nil {
				return nil, err
			}
		}
	}
	for _, definition := range catalog.InformationRegisters {
		table, _ := PhysicalInformationRegisterTable(definition.ID)
		if target.kind == DocumentType && slices.Contains(definition.Recorders, target.metadataID) {
			result = append(result, referenceSource{
				table: table, ownerKind: "information-register", ownerName: definition.Name, ownerMetadataID: definition.ID,
				field: "Recorder", column: "recorder_ref", ownerColumn: "record_id",
				discriminator: "recorder_type", discriminatorID: target.metadataID,
			})
		}
		if err := appendAttributes("information-register", definition.Name, definition.ID, table, "record_id", "", informationRegisterFields(definition), false); err != nil {
			return nil, err
		}
	}
	for _, definition := range catalog.AccumulationRegisters {
		table, _ := PhysicalAccumulationRegisterTable(definition.ID)
		if target.kind == DocumentType && slices.Contains(definition.Recorders, target.metadataID) {
			result = append(result, referenceSource{
				table: table, ownerKind: "accumulation-register", ownerName: definition.Name, ownerMetadataID: definition.ID,
				field: "Recorder", column: "recorder_ref", ownerColumn: "record_id",
				discriminator: "recorder_type", discriminatorID: target.metadataID,
			})
		}
		if err := appendAttributes("accumulation-register", definition.Name, definition.ID, table, "record_id", "", accumulationRegisterFields(definition), false); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (catalog *Catalog) staticConstantReferences(target objectIdentity, stored map[uuid.UUID]bool, limit int) []ReferenceUse {
	result := make([]ReferenceUse, 0)
	for _, constant := range catalog.Constants {
		if len(result) >= limit || stored[constant.ID] || constant.Default == nil || constant.Default.Kind != target.kind || constant.Default.Data != target.objectID.String() {
			continue
		}
		allowed, err := catalog.allowsObjectReference(constant.Types, target.kind, target.metadataID)
		if err == nil && allowed {
			result = append(result, ReferenceUse{OwnerKind: "constant-default", OwnerName: constant.Name, Field: "default"})
		}
	}
	return result
}

func lockReferenceTables(ctx context.Context, transaction pgx.Tx, targetTable string, sources []referenceSource) (bool, error) {
	tables := map[string]bool{pgx.Identifier{schemadiff.ApplicationSchema, targetTable}.Sanitize(): true}
	for _, source := range sources {
		tables[pgx.Identifier{schemadiff.ApplicationSchema, source.table}.Sanitize()] = true
	}
	var constantStorage bool
	if err := transaction.QueryRow(ctx, "SELECT to_regclass('ml_core.constant_values') IS NOT NULL").Scan(&constantStorage); err != nil {
		return false, fmt.Errorf("inspect constant storage: %w", err)
	}
	if constantStorage {
		tables[pgx.Identifier{"ml_core", "constant_values"}.Sanitize()] = true
	}
	ordered := make([]string, 0, len(tables))
	for table := range tables {
		ordered = append(ordered, table)
	}
	sort.Strings(ordered)
	if _, err := transaction.Exec(ctx, "LOCK TABLE "+strings.Join(ordered, ", ")+" IN SHARE ROW EXCLUSIVE MODE"); err != nil {
		return false, fmt.Errorf("lock object reference tables: %w", err)
	}
	return constantStorage, nil
}

func (catalog *Catalog) findObjectReferences(ctx context.Context, transaction pgx.Tx, target objectIdentity, sources []referenceSource, constantStorage bool, limit int) ([]ReferenceUse, error) {
	result := make([]ReferenceUse, 0)
	for _, source := range sources {
		if len(result) >= limit {
			break
		}
		qualified := pgx.Identifier{schemadiff.ApplicationSchema, source.table}.Sanitize()
		column := pgx.Identifier{source.column}.Sanitize()
		ownerColumn := pgx.Identifier{source.ownerColumn}.Sanitize()
		predicate := column + " = $1::uuid"
		arguments := []any{target.objectID.String()}
		if source.composite {
			predicate = column + "->>'kind' = $2 AND " + column + "->>'data' = $1"
			arguments = append(arguments, string(target.kind))
		}
		if source.discriminator != "" {
			arguments = append(arguments, source.discriminatorID.String())
			predicate += fmt.Sprintf(" AND %s = $%d::uuid", pgx.Identifier{source.discriminator}.Sanitize(), len(arguments))
		}
		if source.ownerMetadataID == target.metadataID && source.ownerKind == string(target.kind) {
			predicate += " AND " + ownerColumn + " <> $1::uuid"
		}
		remaining := limit - len(result)
		arguments = append(arguments, remaining)
		limitPlaceholder := fmt.Sprintf("$%d", len(arguments))
		line := "0"
		order := ownerColumn
		if source.lineColumn {
			line = "line_no"
			order += ", line_no"
		}
		rows, err := transaction.Query(ctx, "SELECT "+ownerColumn+"::text, "+line+" FROM "+qualified+" WHERE "+predicate+" ORDER BY "+order+" LIMIT "+limitPlaceholder, arguments...)
		if err != nil {
			return nil, fmt.Errorf("find references in %s %s field %s: %w", source.ownerKind, source.ownerName, source.field, err)
		}
		for rows.Next() {
			var ownerText string
			var lineNumber int
			if err := rows.Scan(&ownerText, &lineNumber); err != nil {
				rows.Close()
				return nil, err
			}
			ownerID, err := uuid.Parse(ownerText)
			if err != nil {
				rows.Close()
				return nil, fmt.Errorf("parse reference owner: %w", err)
			}
			result = append(result, ReferenceUse{
				OwnerKind: source.ownerKind, OwnerName: source.ownerName, Field: source.field,
				OwnerObjectID: ownerID, LineNumber: lineNumber,
			})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	storedConstants := make(map[uuid.UUID]bool)
	if constantStorage && len(result) < limit {
		ids := make([]string, 0)
		for _, constant := range catalog.Constants {
			allowed, err := catalog.allowsObjectReference(constant.Types, target.kind, target.metadataID)
			if err != nil {
				return nil, err
			}
			if allowed {
				ids = append(ids, constant.ID.String())
			}
		}
		if len(ids) > 0 {
			rows, err := transaction.Query(ctx, `
SELECT constant_id::text, value->>'kind', value->>'data' FROM ml_core.constant_values
WHERE constant_id = ANY($1::uuid[]) ORDER BY constant_id`, ids)
			if err != nil {
				return nil, fmt.Errorf("find constant references: %w", err)
			}
			for rows.Next() {
				var idText, kind, data string
				if err := rows.Scan(&idText, &kind, &data); err != nil {
					rows.Close()
					return nil, err
				}
				id, err := uuid.Parse(idText)
				if err != nil {
					rows.Close()
					return nil, err
				}
				storedConstants[id] = true
				if len(result) < limit && kind == string(target.kind) && data == target.objectID.String() {
					constant, _ := catalog.ConstantByID(id)
					result = append(result, ReferenceUse{OwnerKind: "constant", OwnerName: constant.Name, Field: "value"})
				}
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return nil, err
			}
			rows.Close()
		}
	}
	if len(result) < limit {
		result = append(result, catalog.staticConstantReferences(target, storedConstants, limit-len(result))...)
	}
	return result, nil
}

func (catalog *Catalog) objectReferences(ctx context.Context, pool *pgxpool.Pool, target objectIdentity, targetTable string, limit int) ([]ReferenceUse, error) {
	if pool == nil || target.metadataID.IsZero() || target.objectID.IsZero() || limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("invalid object reference check")
	}
	sources, err := catalog.referenceSources(target)
	if err != nil {
		return nil, err
	}
	if scope, ok := scopeFromContext(ctx, pool); ok && scope.tx != nil {
		if scope.rollbackOnly {
			return nil, ErrTransactionDoomed
		}
		constantStorage, err := lockReferenceTables(ctx, scope.tx, targetTable, sources)
		if err != nil {
			return nil, recordDataError(ctx, pool, err)
		}
		result, err := catalog.findObjectReferences(ctx, scope.tx, target, sources, constantStorage, limit)
		return result, recordDataError(ctx, pool, err)
	}
	transaction, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadWrite, IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin object reference check: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	constantStorage, err := lockReferenceTables(ctx, transaction, targetTable, sources)
	if err != nil {
		return nil, err
	}
	return catalog.findObjectReferences(ctx, transaction, target, sources, constantStorage, limit)
}

// FindReferences returns data that currently prevents final catalog deletion.
func (repository *CatalogRepository) FindReferences(ctx context.Context, reference CatalogReference, limit int) ([]ReferenceUse, error) {
	definition, ok := repository.catalog.CatalogByID(reference.CatalogID)
	if !ok || reference.ObjectID.IsZero() {
		return nil, ErrCatalogRecordNotFound
	}
	table, _ := PhysicalCatalogTable(definition.ID)
	return repository.catalog.objectReferences(ctx, repository.pool, objectIdentity{
		kind: CatalogType, metadataID: definition.ID, objectID: reference.ObjectID,
	}, table, limit)
}

// FindReferences returns data that currently prevents final document deletion.
func (repository *DocumentRepository) FindReferences(ctx context.Context, reference DocumentReference, limit int) ([]ReferenceUse, error) {
	definition, ok := repository.catalog.DocumentByID(reference.DocumentID)
	if !ok || reference.ObjectID.IsZero() {
		return nil, ErrDocumentRecordNotFound
	}
	table, _ := PhysicalDocumentTable(definition.ID)
	return repository.catalog.objectReferences(ctx, repository.pool, objectIdentity{
		kind: DocumentType, metadataID: definition.ID, objectID: reference.ObjectID,
	}, table, limit)
}

func (catalog *Catalog) deleteObject(ctx context.Context, pool *pgxpool.Pool, target objectIdentity, table string, version int64, metadataName, presentation string, actor *uuid.UUID, before func(context.Context) error) (DeletionRecord, error) {
	if pool == nil || target.metadataID.IsZero() || target.objectID.IsZero() || version < 1 {
		return DeletionRecord{}, fmt.Errorf("invalid object deletion request")
	}
	var actorValue any
	if actor != nil {
		if actor.IsZero() {
			return DeletionRecord{}, fmt.Errorf("deletion actor UUID must not be zero")
		}
		actorValue = actor.String()
	}
	sources, err := catalog.referenceSources(target)
	if err != nil {
		return DeletionRecord{}, err
	}
	eventID, err := uuid.New()
	if err != nil {
		return DeletionRecord{}, err
	}
	record := DeletionRecord{
		ID: eventID, ObjectKind: string(target.kind), MetadataID: target.metadataID,
		MetadataName: metadataName, ObjectID: target.objectID, Presentation: presentation,
	}
	if actor != nil {
		copy := *actor
		record.ActorID = &copy
	}
	err = runDataTransaction(ctx, pool, nil, func(transactionContext context.Context, transaction pgx.Tx) error {
		ctx = transactionContext
		if err := lockObjectForWrite(ctx, transaction, target.kind, target.metadataID, target.objectID); err != nil {
			return err
		}
		if before != nil {
			if err := before(ctx); err != nil {
				return err
			}
		}
		constantStorage, err := lockReferenceTables(ctx, transaction, table, sources)
		if err != nil {
			return err
		}
		uses, err := catalog.findObjectReferences(ctx, transaction, target, sources, constantStorage, 100)
		if err != nil {
			return err
		}
		if len(uses) > 0 {
			return &ReferenceIntegrityError{Uses: uses}
		}
		conditions := "ref = $1 AND version = $2 AND deletion_mark"
		if target.kind == CatalogType {
			conditions += " AND predefined_name IS NULL"
		} else if target.kind == DocumentType {
			conditions += " AND NOT posted"
		}
		command, err := transaction.Exec(ctx, "DELETE FROM "+pgx.Identifier{schemadiff.ApplicationSchema, table}.Sanitize()+" WHERE "+conditions, target.objectID.String(), version)
		if err != nil {
			return fmt.Errorf("delete %s %s: %w", target.kind, metadataName, err)
		}
		if command.RowsAffected() != 1 {
			return fmt.Errorf("object changed before deletion")
		}
		return transaction.QueryRow(ctx, `
INSERT INTO ml_core.object_deletions(id, object_kind, metadata_id, metadata_name, object_id, presentation, actor_id)
VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING deleted_at`,
			record.ID.String(), record.ObjectKind, record.MetadataID.String(), record.MetadataName,
			record.ObjectID.String(), record.Presentation, actorValue).Scan(&record.DeletedAt)
	})
	if err != nil {
		return DeletionRecord{}, fmt.Errorf("record object deletion: %w", err)
	}
	return record, nil
}

// Delete permanently removes a marked catalog record after checking references.
func (repository *CatalogRepository) Delete(ctx context.Context, record *CatalogRecord, handler CatalogEventHandler, actor *uuid.UUID) (DeletionRecord, error) {
	if record == nil {
		return DeletionRecord{}, fmt.Errorf("catalog record is required")
	}
	working := cloneCatalogRecord(record)
	definition, ok := repository.catalog.CatalogByID(working.Reference.CatalogID)
	if !ok || working.Reference.ObjectID.IsZero() || working.Version < 1 {
		return DeletionRecord{}, ErrCatalogRecordNotFound
	}
	if !working.DeletionMark {
		return DeletionRecord{}, ErrDeletionMarkRequired
	}
	if working.PredefinedName != "" {
		return DeletionRecord{}, ErrPredefinedDeleteDenied
	}
	identity, version, predefinedName := working.Reference, working.Version, working.PredefinedName
	table, _ := PhysicalCatalogTable(definition.ID)
	presentation := working.Description
	if presentation == "" {
		presentation = working.Code
	}
	return repository.catalog.deleteObject(ctx, repository.pool, objectIdentity{
		kind: CatalogType, metadataID: definition.ID, objectID: identity.ObjectID,
	}, table, version, definition.Name, presentation, actor, func(eventContext context.Context) error {
		if err := dispatchCatalogEvent(eventContext, handler, CatalogEventBeforeDelete, working); err != nil {
			return err
		}
		if working.Reference != identity || working.Version != version || working.PredefinedName != predefinedName || !working.DeletionMark {
			return fmt.Errorf("catalog before-delete event changed immutable record state")
		}
		return nil
	})
}

// Delete permanently removes a marked, unposted document after checking references.
func (repository *DocumentRepository) Delete(ctx context.Context, record *DocumentRecord, handler DocumentEventHandler, actor *uuid.UUID) (DeletionRecord, error) {
	if record == nil {
		return DeletionRecord{}, fmt.Errorf("document record is required")
	}
	working := cloneDocumentRecord(record)
	definition, ok := repository.catalog.DocumentByID(working.Reference.DocumentID)
	if !ok || working.Reference.ObjectID.IsZero() || working.Version < 1 {
		return DeletionRecord{}, ErrDocumentRecordNotFound
	}
	if !working.DeletionMark {
		return DeletionRecord{}, ErrDeletionMarkRequired
	}
	if working.Posted {
		return DeletionRecord{}, fmt.Errorf("posted document cannot be deleted")
	}
	identity, version, posted := working.Reference, working.Version, working.Posted
	table, _ := PhysicalDocumentTable(definition.ID)
	presentation := working.Number + " " + working.Date.Format(time.RFC3339)
	return repository.catalog.deleteObject(ctx, repository.pool, objectIdentity{
		kind: DocumentType, metadataID: definition.ID, objectID: identity.ObjectID,
	}, table, version, definition.Name, presentation, actor, func(eventContext context.Context) error {
		if err := dispatchDocumentEvent(eventContext, handler, DocumentEventBeforeDelete, working); err != nil {
			return err
		}
		if working.Reference != identity || working.Version != version || working.Posted != posted || !working.DeletionMark {
			return fmt.Errorf("document before-delete event changed immutable record state")
		}
		return nil
	})
}

func ListObjectDeletions(ctx context.Context, pool *pgxpool.Pool, limit int) ([]DeletionRecord, error) {
	if pool == nil || limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("object deletion history limit must be between 1 and 1000")
	}
	query, err := queryData(ctx, pool)
	if err != nil {
		return nil, err
	}
	rows, err := query.Query(ctx, `
SELECT id::text, object_kind, metadata_id::text, metadata_name, object_id::text,
       presentation, actor_id::text, deleted_at
FROM ml_core.object_deletions ORDER BY deleted_at DESC, id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, recordDataError(ctx, pool, fmt.Errorf("list object deletions: %w", err))
	}
	defer rows.Close()
	result := make([]DeletionRecord, 0)
	for rows.Next() {
		var item DeletionRecord
		var idText, metadataText, objectText string
		var actorText *string
		if err := rows.Scan(&idText, &item.ObjectKind, &metadataText, &item.MetadataName, &objectText, &item.Presentation, &actorText, &item.DeletedAt); err != nil {
			return nil, err
		}
		item.ID, err = uuid.Parse(idText)
		if err == nil {
			item.MetadataID, err = uuid.Parse(metadataText)
		}
		if err == nil {
			item.ObjectID, err = uuid.Parse(objectText)
		}
		if err == nil && actorText != nil {
			actor, parseErr := uuid.Parse(*actorText)
			if parseErr != nil {
				err = parseErr
			} else {
				item.ActorID = &actor
			}
		}
		if err != nil {
			return nil, fmt.Errorf("parse object deletion history: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
