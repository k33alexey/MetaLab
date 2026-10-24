package metadata

import (
	"context"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestDynamicListBuildsBoundedParameterizedQuery(t *testing.T) {
	t.Parallel()
	attributeID := uuid.MustNew()
	definition := CatalogDefinition{Attributes: []Attribute{{ID: attributeID, Name: "ИНН", Types: []Type{{Kind: StringType}}}}}
	request := DynamicListRequest{
		Limit: 50, Search: `Ива%_`, SortField: "ИНН", Descending: true,
		Filters: []ListFilter{{Field: "ИНН", Value: `1' OR TRUE`}},
	}
	statement, arguments, err := buildDynamicListSQL(context.Background(), "t_demo", request, []string{"ИНН"}, func(field string) (string, bool) {
		return catalogListColumn(definition, field)
	}, func(field string) (listColumn, bool) {
		return catalogListField(definition, field)
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	column, _ := PhysicalAttributeColumn(attributeID)
	if strings.Contains(statement, request.Filters[0].Value) || strings.Contains(statement, request.Search) || !strings.Contains(statement, "ORDER BY "+column+" DESC NULLS LAST, ref DESC LIMIT $4") {
		t.Fatalf("statement=%s arguments=%v", statement, arguments)
	}
	if arguments[1] != request.Filters[0].Value || arguments[2] != `Ива\%\_%` || arguments[3] != 51 {
		t.Fatalf("arguments=%v", arguments)
	}
}

func TestDynamicListRejectsInvalidRequests(t *testing.T) {
	t.Parallel()
	if _, err := normalizeDynamicListRequest(DynamicListRequest{Limit: 25}); err == nil {
		t.Fatal("unsupported page size was accepted")
	}
	if _, _, err := buildDynamicListSQL(context.Background(), "t_demo", DynamicListRequest{Limit: 20, Search: "value"}, nil, func(string) (string, bool) { return "", false }, func(string) (listColumn, bool) { return listColumn{}, false }, false); err == nil {
		t.Fatal("search without configured fields was accepted")
	}
	if _, _, err := buildDynamicListSQL(context.Background(), "t_demo", DynamicListRequest{Limit: 20, Filters: []ListFilter{{Field: "x;drop", Value: "1"}}}, nil, func(string) (string, bool) { return "", false }, func(string) (listColumn, bool) { return listColumn{}, false }, false); err == nil {
		t.Fatal("unknown filter field was accepted")
	}
	if _, _, err := buildDynamicListSQL(context.Background(), "t_demo", DynamicListRequest{Limit: 20, SortField: "x;drop"}, nil, func(string) (string, bool) { return "", false }, func(string) (listColumn, bool) { return listColumn{}, false }, false); err == nil {
		t.Fatal("unknown sort field was accepted")
	}
	definition := CatalogDefinition{Code: CatalogCode{Type: StringType}}
	if _, _, err := buildDynamicListSQL(context.Background(), "t_demo", DynamicListRequest{Limit: 20, Search: "x", SearchField: "Code"}, []string{"Description"}, func(field string) (string, bool) {
		return catalogListColumn(definition, field)
	}, func(field string) (listColumn, bool) {
		return catalogListField(definition, field)
	}, false); err == nil {
		t.Fatal("advanced search outside input-by-string fields was accepted")
	}
}

func TestDynamicListSearchesNumericFieldByExactValue(t *testing.T) {
	t.Parallel()
	definition := CatalogDefinition{Code: CatalogCode{Type: NumberType, Length: 9}}
	statement, arguments, err := buildDynamicListSQL(context.Background(), "t_demo", DynamicListRequest{Limit: 20, Search: "12,5", SearchField: "Code"}, []string{"Description", "Code"}, func(field string) (string, bool) {
		return catalogListColumn(definition, field)
	}, func(field string) (listColumn, bool) {
		return catalogListField(definition, field)
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(statement, "code = $2") || arguments[1] != "12.5" {
		t.Fatalf("statement=%s arguments=%v", statement, arguments)
	}
}

// A list asked for no page is read by the constant page of a list, 100 rows
// (owner, 24.10.2026): the object no longer says its page. Defect caught: the
// former default of 20 left in place.
func TestAListAskedForNoPageReadsAHundredRows(t *testing.T) {
	t.Parallel()
	request, err := normalizeDynamicListRequest(DynamicListRequest{})
	if err != nil || request.Limit != 100 {
		t.Fatalf("limit=%d error=%v", request.Limit, err)
	}
}

// The page and the search fields of a list are not a property of an object in
// the prototype - its form and dynamic list give them - so no kind reads them
// (owner, 24.10.2026, BLOCKS 2б «размер страницы и поля поиска у ссылочных
// видов — убрать»). A description naming them is refused by the key rather
// than read with it dropped. The nine kinds are written out here, the ones
// that carried the key. Defect caught: a kind still reading the settings.
func TestNoKindReadsTheSettingsOfAList(t *testing.T) {
	t.Parallel()
	for _, kind := range []Kind{CatalogKind, DocumentKind, DocumentJournalKind, ChartOfAccountsKind, ChartOfCalculationTypesKind,
		ChartOfCharacteristicTypesKind, BusinessProcessKind, TaskKind, ExchangePlanKind} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			body := "format: 1\nid: d5000000-0000-4000-8000-000000000001\nname: Объект\ntitle: {ru: Объект}\n" +
				"list: {page_size: 50, search_fields: [Description]}\n"
			_, described, err := DecodeSource("metadata/"+string(kind)+"/Объект/object.yaml", strings.NewReader(body), nil, metadataConfiguration())
			if !described {
				t.Fatalf("the description of %s was not read as one", kind)
			}
			if err == nil || !strings.Contains(err.Error(), "field list not found") {
				t.Fatalf("the settings of a list of %s were not refused by the key: %v", kind, err)
			}
		})
	}
}
