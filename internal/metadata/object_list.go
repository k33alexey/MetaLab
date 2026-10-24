package metadata

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

const maxBasicListPageSize = 100

const (
	maxDynamicListFilters = 16
	maxDynamicSearchRunes = 256
)

type DynamicListRequest struct {
	Cursor      *uuid.UUID
	Limit       int
	Search      string
	SearchField string
	SortField   string
	Descending  bool
	Filters     []ListFilter
}

type ListFilter struct {
	Field string
	Value string
}

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
	arguments := []any{basicListCursor(cursor), limit + 1}
	restriction := readRowPredicate(ctx, definition.ID)
	statement := "SELECT to_jsonb(item) FROM " + qualifiedCatalogTable(table) + " AS item WHERE ref > $1" + restriction + " ORDER BY ref LIMIT $2"
	return repository.listPageArguments(ctx, definition, statement, arguments, limit)
}

// ListDynamic returns a filtered keyset page using only metadata-owned fields.
func (repository *CatalogRepository) ListDynamic(ctx context.Context, name string, request DynamicListRequest) (CatalogListPage, error) {
	definition, ok := repository.catalog.CatalogDefinition(name)
	if !ok {
		return CatalogListPage{}, fmt.Errorf("unknown catalog %q", name)
	}
	request, err := normalizeDynamicListRequest(request)
	if err != nil {
		return CatalogListPage{}, err
	}
	table, _ := PhysicalCatalogTable(definition.ID)
	statement, arguments, err := buildDynamicListSQL(ctx, table, request, []string{"Description", "Code"}, func(field string) (string, bool) {
		return catalogListColumn(definition, field)
	}, func(field string) (listColumn, bool) {
		return catalogListField(definition, field)
	}, readRestricted(ctx, definition.ID))
	if err != nil {
		return CatalogListPage{}, err
	}
	return repository.listPageArguments(ctx, definition, statement, arguments, request.Limit)
}

func (repository *CatalogRepository) listPageArguments(ctx context.Context, definition CatalogDefinition, statement string, arguments []any, limit int) (CatalogListPage, error) {
	query, err := queryData(ctx, repository.pool)
	if err != nil {
		return CatalogListPage{}, err
	}
	rows, err := query.Query(ctx, statement, arguments...)
	if err != nil {
		return CatalogListPage{}, recordDataError(ctx, repository.pool, fmt.Errorf("list catalog %s: %w", definition.Name, err))
	}
	defer rows.Close()
	records, next, err := readKeysetPage(rows, limit, func(encoded []byte) (*CatalogRecord, error) {
		return repository.decodeRecord(definition, encoded)
	}, func(record *CatalogRecord) uuid.UUID { return record.Reference.ObjectID })
	if err != nil {
		return CatalogListPage{}, recordDataError(ctx, repository.pool, err)
	}
	return CatalogListPage{Records: records, NextCursor: next}, nil
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
	arguments := []any{basicListCursor(cursor), limit + 1}
	restriction := readRowPredicate(ctx, definition.ID)
	query, err := queryData(ctx, repository.pool)
	if err != nil {
		return DocumentListPage{}, err
	}
	rows, err := query.Query(ctx, "SELECT to_jsonb(item) FROM "+qualifiedCatalogTable(table)+" AS item WHERE ref > $1"+restriction+" ORDER BY ref LIMIT $2", arguments...)
	if err != nil {
		return DocumentListPage{}, recordDataError(ctx, repository.pool, fmt.Errorf("list document %s: %w", definition.Name, err))
	}
	defer rows.Close()
	records, next, err := readKeysetPage(rows, limit, func(encoded []byte) (*DocumentRecord, error) {
		return repository.decodeRecord(definition, encoded)
	}, func(record *DocumentRecord) uuid.UUID { return record.Reference.ObjectID })
	if err != nil {
		return DocumentListPage{}, recordDataError(ctx, repository.pool, err)
	}
	return DocumentListPage{Records: records, NextCursor: next}, nil
}

// ListDynamic returns a filtered keyset page using only metadata-owned fields.
func (repository *DocumentRepository) ListDynamic(ctx context.Context, name string, request DynamicListRequest) (DocumentListPage, error) {
	definition, ok := repository.catalog.DocumentDefinition(name)
	if !ok {
		return DocumentListPage{}, fmt.Errorf("unknown document %q", name)
	}
	request, err := normalizeDynamicListRequest(request)
	if err != nil {
		return DocumentListPage{}, err
	}
	table, _ := PhysicalDocumentTable(definition.ID)
	statement, arguments, err := buildDynamicListSQL(ctx, table, request, []string{"Number"}, func(field string) (string, bool) {
		return documentListColumn(definition, field)
	}, func(field string) (listColumn, bool) {
		return documentListField(definition, field)
	}, readRestricted(ctx, definition.ID))
	if err != nil {
		return DocumentListPage{}, err
	}
	query, err := queryData(ctx, repository.pool)
	if err != nil {
		return DocumentListPage{}, err
	}
	rows, err := query.Query(ctx, statement, arguments...)
	if err != nil {
		return DocumentListPage{}, recordDataError(ctx, repository.pool, fmt.Errorf("list document %s: %w", definition.Name, err))
	}
	defer rows.Close()
	records, next, err := readKeysetPage(rows, request.Limit, func(encoded []byte) (*DocumentRecord, error) {
		return repository.decodeRecord(definition, encoded)
	}, func(record *DocumentRecord) uuid.UUID { return record.Reference.ObjectID })
	if err != nil {
		return DocumentListPage{}, recordDataError(ctx, repository.pool, err)
	}
	return DocumentListPage{Records: records, NextCursor: next}, nil
}

// keysetRows are the rows of a query a keyset page reads.
type keysetRows interface {
	Next() bool
	Scan(...any) error
	Err() error
}

// readKeysetPage reads a keyset page of records the statement asked limit+1
// rows for, each row one record as JSON: the first limit records, and the
// reference of the last of them as the cursor of the next page when the
// statement found one more. The rows are closed by the caller.
func readKeysetPage[R any](rows keysetRows, limit int, decode func([]byte) (R, error), reference func(R) uuid.UUID) ([]R, *uuid.UUID, error) {
	records := make([]R, 0, limit)
	for rows.Next() {
		var encoded []byte
		if err := rows.Scan(&encoded); err != nil {
			return nil, nil, err
		}
		record, err := decode(encoded)
		if err != nil {
			return nil, nil, err
		}
		if len(records) == limit {
			next := reference(records[len(records)-1])
			return records, &next, nil
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return records, nil, nil
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

func normalizeDynamicListRequest(request DynamicListRequest) (DynamicListRequest, error) {
	if request.Limit == 0 {
		request.Limit = DynamicListPageSize
	}
	if request.Cursor != nil && request.Cursor.IsZero() {
		return DynamicListRequest{}, fmt.Errorf("list cursor must be a non-zero UUID")
	}
	if !validDynamicListPageSize(request.Limit) {
		return DynamicListRequest{}, fmt.Errorf("dynamic list page size must be 20, 50 or 100")
	}
	request.Search = strings.TrimSpace(request.Search)
	if utf8.RuneCountInString(request.Search) > maxDynamicSearchRunes {
		return DynamicListRequest{}, fmt.Errorf("dynamic list search must not exceed %d characters", maxDynamicSearchRunes)
	}
	if len(request.Filters) > maxDynamicListFilters {
		return DynamicListRequest{}, fmt.Errorf("dynamic list must not contain more than %d filters", maxDynamicListFilters)
	}
	return request, nil
}

func buildDynamicListSQL(ctx context.Context, table string, request DynamicListRequest, searchFields []string, resolve func(string) (string, bool), resolveSearch func(string) (listColumn, bool), restricted bool) (string, []any, error) {
	arguments := []any{basicListCursor(request.Cursor)}
	sortColumn := "ref"
	if request.SortField != "" {
		var ok bool
		sortColumn, ok = resolve(request.SortField)
		if !ok {
			return "", nil, fmt.Errorf("invalid dynamic list sort field %q", request.SortField)
		}
	}
	comparison, direction := ">", "ASC"
	if request.Descending {
		comparison, direction = "<", "DESC"
	}
	conditions := []string{"ref > $1"}
	if request.Cursor != nil {
		if sortColumn == "ref" {
			conditions[0] = "ref " + comparison + " $1"
		} else {
			cursorValue := "(SELECT " + sortColumn + " FROM " + qualifiedCatalogTable(table) + " WHERE ref = $1)"
			conditions[0] = "((" + cursorValue + " IS NOT NULL AND (" + sortColumn + " " + comparison + " " + cursorValue + " OR " + sortColumn + " IS NULL OR (" + sortColumn + " = " + cursorValue + " AND ref " + comparison + " $1))) OR (" + cursorValue + " IS NULL AND " + sortColumn + " IS NULL AND ref " + comparison + " $1))"
		}
	}
	seenFilters := make(map[string]bool, len(request.Filters))
	for _, filter := range request.Filters {
		column, ok := resolve(filter.Field)
		folded := strings.ToLower(strings.TrimSpace(filter.Field))
		if !ok || seenFilters[folded] {
			return "", nil, fmt.Errorf("invalid or duplicate dynamic list filter %q", filter.Field)
		}
		seenFilters[folded] = true
		arguments = append(arguments, filter.Value)
		conditions = append(conditions, fmt.Sprintf("%s = $%d", column, len(arguments)))
	}
	// Row-level access restrictions narrow the same WHERE clause as ordinary
	// filters, so a restricted row is never fetched, counted or paged over.
	// Their conditions are not run yet, so a restricted list admits none.
	if restricted {
		conditions = append(conditions, "FALSE")
	}
	if request.Search != "" && len(searchFields) == 0 {
		return "", nil, fmt.Errorf("dynamic list search fields are not configured")
	}
	if request.SearchField != "" {
		field, ok := resolveSearch(request.SearchField)
		if !ok || !searchableListType(field.kind) || !containsListField(searchFields, request.SearchField) {
			return "", nil, fmt.Errorf("invalid dynamic list search field %q", request.SearchField)
		}
		searchFields = []string{request.SearchField}
	}
	if request.Search != "" {
		alternatives := make([]string, 0, len(searchFields))
		stringPlaceholder := 0
		for _, field := range searchFields {
			resolved, ok := resolveSearch(field)
			if !ok || !searchableListType(resolved.kind) {
				return "", nil, fmt.Errorf("invalid dynamic list search field %q", field)
			}
			if resolved.kind == NumberType {
				if numeric, ok := numericSearchValue(request.Search); ok {
					arguments = append(arguments, numeric)
					alternatives = append(alternatives, fmt.Sprintf("%s = $%d", resolved.name, len(arguments)))
				}
				continue
			}
			if stringPlaceholder == 0 {
				arguments = append(arguments, escapeLikePrefix(request.Search)+"%")
				stringPlaceholder = len(arguments)
			}
			alternatives = append(alternatives, fmt.Sprintf("lower(%s::text) LIKE lower($%d) ESCAPE '\\'", resolved.name, stringPlaceholder))
		}
		if len(alternatives) == 0 {
			conditions = append(conditions, "FALSE")
		} else {
			conditions = append(conditions, "("+strings.Join(alternatives, " OR ")+")")
		}
	}
	arguments = append(arguments, request.Limit+1)
	order := "ref " + direction
	if sortColumn != "ref" {
		order = sortColumn + " " + direction + " NULLS LAST, ref " + direction
	}
	statement := "SELECT to_jsonb(item) FROM " + qualifiedCatalogTable(table) + " AS item WHERE " + strings.Join(conditions, " AND ") + " ORDER BY " + order + fmt.Sprintf(" LIMIT $%d", len(arguments))
	return statement, arguments, nil
}

func containsListField(fields []string, expected string) bool {
	for _, field := range fields {
		if strings.EqualFold(strings.TrimSpace(field), strings.TrimSpace(expected)) {
			return true
		}
	}
	return false
}

func catalogListColumn(definition CatalogDefinition, field string) (string, bool) {
	resolved, ok := catalogListField(definition, field)
	return resolved.name, ok
}

func catalogListField(definition CatalogDefinition, field string) (listColumn, bool) {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "ref":
		return listColumn{name: "ref", kind: UUIDType}, true
	case "code":
		return listColumn{name: "code", kind: definition.Code.Type}, catalogHasCode(definition)
	case "description":
		return listColumn{name: "description", kind: StringType}, catalogHasDescription(definition)
	case "deletionmark":
		return listColumn{name: "deletion_mark", kind: BooleanType}, true
	}
	return attributeListField(definition.Attributes, field)
}

func documentListColumn(definition DocumentDefinition, field string) (string, bool) {
	resolved, ok := documentListField(definition, field)
	return resolved.name, ok
}

func documentListField(definition DocumentDefinition, field string) (listColumn, bool) {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "ref":
		return listColumn{name: "ref", kind: UUIDType}, true
	case "number":
		return listColumn{name: "number", kind: definition.Number.Type}, documentHasNumber(definition)
	case "date":
		return listColumn{name: "date", kind: DateType}, true
	case "posted":
		return listColumn{name: "posted", kind: BooleanType}, true
	case "deletionmark":
		return listColumn{name: "deletion_mark", kind: BooleanType}, true
	}
	return attributeListField(definition.Attributes, field)
}

func attributeListField(attributes []Attribute, field string) (listColumn, bool) {
	for _, attribute := range attributes {
		if strings.EqualFold(attribute.Name, strings.TrimSpace(field)) {
			column, _ := PhysicalAttributeColumn(attribute.ID)
			kind := TypeKind("")
			if single, ok := SingleType(attribute.Types); ok {
				kind = single.Kind
			}
			return listColumn{name: column, kind: kind}, true
		}
	}
	return listColumn{}, false
}

func escapeLikePrefix(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	return strings.ReplaceAll(value, "_", "\\_")
}

var numericSearchPattern = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:[.,][0-9]*)?|[.,][0-9]+)$`)

func numericSearchValue(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if !numericSearchPattern.MatchString(value) {
		return "", false
	}
	return strings.Replace(value, ",", ".", 1), true
}
