package studio

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/querylang"
)

func TestQueryDesignerSchemaAndBuild(t *testing.T) {
	t.Parallel()
	workspace, _, _ := createManagedFormSource(t)
	schema, err := workspace.QueryDesignerSchema()
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Sources) != 1 || schema.Sources[0].Path != "Справочник.Контрагенты" || !queryDesignerHasField(schema.Sources[0], "ИНН") {
		t.Fatalf("query schema = %+v", schema)
	}
	schema.Sources[0].Fields[0].Name = "Изменено"
	again, err := workspace.QueryDesignerSchema()
	if err != nil || again.Sources[0].Fields[0].Name == "Изменено" {
		t.Fatalf("cached query schema exposed mutable state: %+v, error=%v", again, err)
	}
	result, err := workspace.BuildDesignedQuery(QueryDesign{
		Distinct: true, Top: 50,
		Sources: []QueryDesignSource{{Path: "Справочник.Контрагенты", Alias: "Контрагенты"}},
		Fields:  []QueryDesignField{{Source: "Контрагенты", Field: "Код"}, {Source: "Контрагенты", Field: "ИНН", Alias: "ИННКонтрагента", Order: "asc"}},
		Where:   "НЕ Контрагенты.ПометкаУдаления",
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := querylang.Parse(result.Query)
	if err != nil || !parsed.Distinct || parsed.Top != 50 || len(parsed.Fields) != 2 || len(parsed.Order) != 1 || parsed.Where == nil {
		t.Fatalf("query=%s\nparsed=%+v error=%v", result.Query, parsed, err)
	}
}

func TestBuildDesignedQueryValidatesJoinsAndFields(t *testing.T) {
	t.Parallel()
	schema := QueryDesignerSchema{Sources: []QueryDesignerSource{
		{Path: "Справочник.Товары", Fields: []QueryDesignerField{{Name: "Ссылка"}, {Name: "Владелец"}}},
		{Path: "Справочник.Группы", Fields: []QueryDesignerField{{Name: "Ссылка"}, {Name: "Наименование"}}},
	}}
	design := QueryDesign{
		Sources: []QueryDesignSource{
			{Path: "Справочник.Товары", Alias: "Товары"},
			{Path: "Справочник.Группы", Alias: "Группы", Join: "left", LeftAlias: "Товары", LeftField: "Владелец", RightField: "Ссылка"},
		},
		Fields: []QueryDesignField{{Source: "Товары", Field: "Ссылка", Group: true}, {Source: "Группы", Field: "Наименование", Group: true, Order: "desc"}},
	}
	query, err := buildDesignedQuery(schema, design)
	if err != nil || !strings.Contains(query, "ЛЕВОЕ СОЕДИНЕНИЕ") || !strings.Contains(query, "СГРУППИРОВАТЬ ПО") || !strings.Contains(query, "УБЫВ") {
		t.Fatalf("query=%s error=%v", query, err)
	}
	design.Sources[1].Join = "right"
	right, err := buildDesignedQuery(schema, design)
	if err != nil || strings.Contains(right, "ПРАВОЕ") || !strings.Contains(right, "ИЗ\n    Справочник.Группы КАК Группы\nЛЕВОЕ СОЕДИНЕНИЕ Справочник.Товары КАК Товары") {
		t.Fatalf("normalized right join=%s error=%v", right, err)
	}
	design.Sources[1].Join = "left"
	design.Fields[0].Field = "Несуществующее"
	if _, err := buildDesignedQuery(schema, design); err == nil || !strings.Contains(err.Error(), "unknown query field") {
		t.Fatalf("invalid field error = %v", err)
	}
}

func TestBuildDesignedQueryRejectsNumericAggregateForString(t *testing.T) {
	t.Parallel()
	schema := QueryDesignerSchema{Sources: []QueryDesignerSource{{
		Path:   "Справочник.Контрагенты",
		Fields: []QueryDesignerField{{Name: "Наименование", Type: "string"}},
	}}}
	_, err := buildDesignedQuery(schema, QueryDesign{
		Sources: []QueryDesignSource{{Path: "Справочник.Контрагенты", Alias: "Контрагенты"}},
		Fields:  []QueryDesignField{{Source: "Контрагенты", Field: "Наименование", Aggregate: "sum"}},
	})
	if err == nil || !strings.Contains(err.Error(), "numeric field") {
		t.Fatalf("buildDesignedQuery() error = %v, want numeric-field validation", err)
	}
}

func TestQueryDesignerHTTPAPI(t *testing.T) {
	t.Parallel()
	workspace, _, _ := createManagedFormSource(t)
	handler := NewHandler(workspace)
	for _, target := range []string{"/", "/ui/query-designer.js", "/ui/query-designer.css"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status=%d", target, response.Code)
		}
		if target == "/" && (!strings.Contains(response.Body.String(), `id="query-dialog"`) || !strings.Contains(response.Body.String(), `/ui/query-designer.js`)) {
			t.Fatal("Studio shell does not connect the query designer")
		}
	}
	schema := httptest.NewRecorder()
	handler.ServeHTTP(schema, httptest.NewRequest(http.MethodGet, "/api/query/schema", nil))
	if schema.Code != http.StatusOK || !strings.Contains(schema.Body.String(), "Справочник.Контрагенты") {
		t.Fatalf("schema status=%d body=%s", schema.Code, schema.Body.String())
	}
	design := QueryDesign{Sources: []QueryDesignSource{{Path: "Справочник.Контрагенты", Alias: "Контрагенты"}}, Fields: []QueryDesignField{{Source: "Контрагенты", Field: "Ссылка"}}}
	payload, err := json.Marshal(design)
	if err != nil {
		t.Fatal(err)
	}
	denied := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/query/build", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(denied, request)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status = %d", denied.Code)
	}
	built := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/query/build", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-ML-CSRF", "1")
	handler.ServeHTTP(built, request)
	if built.Code != http.StatusOK || !strings.Contains(built.Body.String(), "ВЫБРАТЬ") {
		t.Fatalf("build status=%d body=%s", built.Code, built.Body.String())
	}
}

func queryDesignerHasField(source QueryDesignerSource, expected string) bool {
	for _, field := range source.Fields {
		if field.Name == expected {
			return true
		}
	}
	return false
}
