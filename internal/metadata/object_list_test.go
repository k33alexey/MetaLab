package metadata

import (
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
	statement, arguments, err := buildDynamicListSQL("t_demo", request, []string{"ИНН"}, func(field string) (string, bool) {
		return catalogListColumn(definition, field)
	}, func(field string) (listColumn, bool) {
		return catalogListField(definition, field)
	})
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
	if _, err := normalizeDynamicListRequest(DynamicListRequest{Limit: 25}, ListSettings{}); err == nil {
		t.Fatal("unsupported page size was accepted")
	}
	if _, _, err := buildDynamicListSQL("t_demo", DynamicListRequest{Limit: 20, Search: "value"}, nil, func(string) (string, bool) { return "", false }, func(string) (listColumn, bool) { return listColumn{}, false }); err == nil {
		t.Fatal("search without configured fields was accepted")
	}
	if _, _, err := buildDynamicListSQL("t_demo", DynamicListRequest{Limit: 20, Filters: []ListFilter{{Field: "x;drop", Value: "1"}}}, nil, func(string) (string, bool) { return "", false }, func(string) (listColumn, bool) { return listColumn{}, false }); err == nil {
		t.Fatal("unknown filter field was accepted")
	}
	if _, _, err := buildDynamicListSQL("t_demo", DynamicListRequest{Limit: 20, SortField: "x;drop"}, nil, func(string) (string, bool) { return "", false }, func(string) (listColumn, bool) { return listColumn{}, false }); err == nil {
		t.Fatal("unknown sort field was accepted")
	}
	definition := CatalogDefinition{Code: CatalogCode{Type: StringType}}
	if _, _, err := buildDynamicListSQL("t_demo", DynamicListRequest{Limit: 20, Search: "x", SearchField: "Code"}, []string{"Description"}, func(field string) (string, bool) {
		return catalogListColumn(definition, field)
	}, func(field string) (listColumn, bool) {
		return catalogListField(definition, field)
	}); err == nil {
		t.Fatal("advanced search outside input-by-string fields was accepted")
	}
}

func TestDynamicListSearchesNumericFieldByExactValue(t *testing.T) {
	t.Parallel()
	definition := CatalogDefinition{Code: CatalogCode{Type: NumberType}}
	statement, arguments, err := buildDynamicListSQL("t_demo", DynamicListRequest{Limit: 20, Search: "12,5", SearchField: "Code"}, []string{"Description", "Code"}, func(field string) (string, bool) {
		return catalogListColumn(definition, field)
	}, func(field string) (listColumn, bool) {
		return catalogListField(definition, field)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(statement, "code = $2") || arguments[1] != "12.5" {
		t.Fatalf("statement=%s arguments=%v", statement, arguments)
	}
}

func TestListSettingsRequireSearchableFields(t *testing.T) {
	t.Parallel()
	attributes := []Attribute{
		{ID: uuid.MustNew(), Name: "ИНН", Types: []Type{{Kind: StringType}}},
		{ID: uuid.MustNew(), Name: "Сумма", Types: []Type{{Kind: NumberType}}},
		{ID: uuid.MustNew(), Name: "Активен", Types: []Type{{Kind: BooleanType}}},
	}
	if issues := validateListSettings(ListSettings{PageSize: 20, SearchFields: []string{"Description", "ИНН"}}, attributes, map[string]TypeKind{"description": StringType}); len(issues) != 0 {
		t.Fatalf("valid settings issues=%v", issues)
	}
	if issues := validateListSettings(ListSettings{PageSize: 25, SearchFields: []string{"Активен", "НетПоля"}}, attributes, nil); len(issues) != 3 {
		t.Fatalf("invalid settings issues=%v", issues)
	}
}
