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
	return repository.listPage(ctx, definition, "SELECT to_jsonb(item) FROM "+qualifiedCatalogTable(table)+" AS item WHERE ref > $1 ORDER BY ref LIMIT $2", cursor, limit)
}

// ListDynamic returns a filtered keyset page using only metadata-owned fields.
func (repository *CatalogRepository) ListDynamic(ctx context.Context, name string, request DynamicListRequest) (CatalogListPage, error) {
	definition, ok := repository.catalog.CatalogDefinition(name)
	if !ok {
		return CatalogListPage{}, fmt.Errorf("unknown catalog %q", name)
	}
	request, err := normalizeDynamicListRequest(request, definition.List)
	if err != nil {
		return CatalogListPage{}, err
	}
	table, _ := PhysicalCatalogTable(definition.ID)
	statement, arguments, err := buildDynamicListSQL(table, request, effectiveListSearchFields(definition.List, []string{"Description", "Code"}), func(field string) (string, bool) {
		return catalogListColumn(definition, field)
	}, func(field string) (listColumn, bool) {
		return catalogListField(definition, field)
	})
	if err != nil {
		return CatalogListPage{}, err
	}
	return repository.listPageArguments(ctx, definition, statement, arguments, request.Limit)
}

func (repository *CatalogRepository) listPage(ctx context.Context, definition CatalogDefinition, statement string, cursor *uuid.UUID, limit int) (CatalogListPage, error) {
	return repository.listPageArguments(ctx, definition, statement, []any{basicListCursor(cursor), limit + 1}, limit)
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

// ListDynamic returns a filtered keyset page using only metadata-owned fields.
func (repository *DocumentRepository) ListDynamic(ctx context.Context, name string, request DynamicListRequest) (DocumentListPage, error) {
	definition, ok := repository.catalog.DocumentDefinition(name)
	if !ok {
		return DocumentListPage{}, fmt.Errorf("unknown document %q", name)
	}
	request, err := normalizeDynamicListRequest(request, definition.List)
	if err != nil {
		return DocumentListPage{}, err
	}
	table, _ := PhysicalDocumentTable(definition.ID)
	statement, arguments, err := buildDynamicListSQL(table, request, effectiveListSearchFields(definition.List, []string{"Number"}), func(field string) (string, bool) {
		return documentListColumn(definition, field)
	}, func(field string) (listColumn, bool) {
		return documentListField(definition, field)
	})
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
	result := DocumentListPage{Records: make([]*DocumentRecord, 0, request.Limit)}
	for rows.Next() {
		var encoded []byte
		if err := rows.Scan(&encoded); err != nil {
			return DocumentListPage{}, err
		}
		record, err := repository.decodeRecord(definition, encoded)
		if err != nil {
			return DocumentListPage{}, err
		}
		if len(result.Records) == request.Limit {
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

func normalizeDynamicListRequest(request DynamicListRequest, settings ListSettings) (DynamicListRequest, error) {
	if request.Limit == 0 {
		request.Limit = defaultDynamicListPageSize(settings)
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

func buildDynamicListSQL(table string, request DynamicListRequest, searchFields []string, resolve func(string) (string, bool), resolveSearch func(string) (listColumn, bool)) (string, []any, error) {
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
		return listColumn{name: "code", kind: definition.Code.Type}, true
	case "description":
		return listColumn{name: "description", kind: StringType}, true
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
		return listColumn{name: "number", kind: definition.Number.Type}, true
	case "date":
		return listColumn{name: "date", kind: DateType}, true
	case "posted":
		return listColumn{name: "posted", kind: BooleanType}, true
	case "deletionmark":
		return listColumn{name: "deletion_mark", kind: BooleanType}, true
	}
	return attributeListField(definition.Attributes, field)
}

func attributeListColumn(attributes []Attribute, field string) (string, bool) {
	resolved, ok := attributeListField(attributes, field)
	return resolved.name, ok
}

func attributeListField(attributes []Attribute, field string) (listColumn, bool) {
	for _, attribute := range attributes {
		if strings.EqualFold(attribute.Name, strings.TrimSpace(field)) {
			column, _ := PhysicalAttributeColumn(attribute.ID)
			kind := TypeKind("")
			if len(attribute.Types) == 1 {
				kind = attribute.Types[0].Kind
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
