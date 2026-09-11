package metadata

import (
	"fmt"
	"strings"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const maxListSearchFields = 16

type listColumn struct {
	name string
	kind TypeKind
}

func validateListSettings(settings ListSettings, attributes []Attribute, systemFields map[string]TypeKind) []string {
	var issues []string
	if settings.PageSize != 0 && !validDynamicListPageSize(settings.PageSize) {
		issues = append(issues, "list.page_size must be 20, 50 or 100")
	}
	if len(settings.SearchFields) > maxListSearchFields {
		issues = append(issues, fmt.Sprintf("list.search_fields must not contain more than %d items", maxListSearchFields))
	}
	available := make(map[string]bool, len(attributes)+len(systemFields))
	for name, kind := range systemFields {
		available[strings.ToLower(name)] = searchableListType(kind)
	}
	for _, attribute := range attributes {
		available[strings.ToLower(attribute.Name)] = len(attribute.Types) == 1 && searchableListType(attribute.Types[0].Kind)
	}
	seen := make(map[string]bool, len(settings.SearchFields))
	for index, name := range settings.SearchFields {
		folded := strings.ToLower(strings.TrimSpace(name))
		if !validIdentifier(name) {
			issues = append(issues, fmt.Sprintf("list.search_fields[%d] must be a valid field name", index))
		} else if seen[folded] {
			issues = append(issues, fmt.Sprintf("list.search_fields[%d] must be unique", index))
		} else if searchable, exists := available[folded]; !exists {
			issues = append(issues, fmt.Sprintf("list.search_fields[%d] references an unknown field", index))
		} else if !searchable {
			issues = append(issues, fmt.Sprintf("list.search_fields[%d] must reference a string field", index))
		}
		seen[folded] = true
	}
	return issues
}

func validDynamicListPageSize(value int) bool { return value == 20 || value == 50 || value == 100 }

func defaultDynamicListPageSize(settings ListSettings) int {
	if settings.PageSize == 0 {
		return 20
	}
	return settings.PageSize
}

func searchableListType(kind TypeKind) bool { return kind == StringType || kind == NumberType }

func effectiveListSearchFields(settings ListSettings, defaults []string) []string {
	if len(settings.SearchFields) != 0 {
		return settings.SearchFields
	}
	return defaults
}

func appendListSearchIndexes(table *schemadiff.Table, ownerID uuid.UUID, settings ListSettings, defaults []string, attributes []Attribute, systemColumns map[string]listColumn) {
	for _, field := range effectiveListSearchFields(settings, defaults) {
		folded := strings.ToLower(field)
		resolved := systemColumns[folded]
		column := resolved.name
		kind := resolved.kind
		indexName := physicalObjectName("is"+searchIndexSuffix(folded), ownerID)
		if column == "" {
			for _, attribute := range attributes {
				if strings.EqualFold(attribute.Name, field) {
					column, _ = PhysicalAttributeColumn(attribute.ID)
					if len(attribute.Types) == 1 {
						kind = attribute.Types[0].Kind
					}
					indexName = physicalObjectName("is", attribute.ID)
					break
				}
			}
		}
		if column == "" {
			continue
		}
		key := column
		if kind == StringType {
			key = "lower(" + column + "::text) text_pattern_ops"
		}
		table.Indexes = append(table.Indexes, schemadiff.Index{Name: indexName, Method: "btree", Keys: []string{key}})
	}
}

func searchIndexSuffix(value string) string {
	value = strings.ToLower(value)
	if len(value) > 8 {
		value = value[:8]
	}
	return value
}
