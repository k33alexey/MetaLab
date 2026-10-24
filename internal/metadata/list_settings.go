package metadata

import (
	"strings"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

type listColumn struct {
	name string
	kind TypeKind
}

// DynamicListPageSize is how many rows a list shows at once. The prototype
// has no such property of an object: the page and the search of a list are
// the form's and its dynamic list's (owner, 24.10.2026, BLOCKS 2б «размер
// страницы и поля поиска у ссылочных видов — убрать»). Until the form of a
// list is executed (block 7) ML App lists by this constant.
const DynamicListPageSize = 100

// validDynamicListPageSize is the bound of a page a caller of the list asks
// for: the constant above or one of the two smaller pages the API took
// before it.
func validDynamicListPageSize(value int) bool {
	return value == 20 || value == 50 || value == DynamicListPageSize
}

func searchableListType(kind TypeKind) bool { return kind == StringType || kind == NumberType }

// appendListSearchIndexes indexes the fields a list of the object is searched
// by - the fields of its presentation, the code and the description or the
// number (fields). They are the ones a list searched by before the search
// fields left the object, so the schema of a project that named none is what
// it was.
func appendListSearchIndexes(table *schemadiff.Table, ownerID uuid.UUID, fields []string, systemColumns map[string]listColumn) {
	for _, field := range fields {
		folded := strings.ToLower(field)
		resolved := systemColumns[folded]
		column := resolved.name
		kind := resolved.kind
		indexName := physicalObjectName("is"+searchIndexSuffix(folded), ownerID)
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
