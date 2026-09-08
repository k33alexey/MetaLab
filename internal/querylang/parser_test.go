package querylang

import (
	"strings"
	"testing"
)

func TestParseBasicRussianQuery(t *testing.T) {
	query, err := Parse(`
// basic selection
ВЫБРАТЬ ПЕРВЫЕ 20
    Товары.Ссылка,
    Товары.Наименование КАК Имя
ИЗ Справочник.Товары КАК Товары
ГДЕ НЕ Товары.ПометкаУдаления
    И (Товары.Код = &Код ИЛИ Товары.Наименование ПОДОБНО &Поиск)
УПОРЯДОЧИТЬ ПО Имя, Товары.Код УБЫВ;
`)
	if err != nil {
		t.Fatal(err)
	}
	if query.Top != 20 || len(query.Fields) != 2 || len(query.Source.Path) != 2 || query.Source.Alias != "Товары" || len(query.Order) != 2 {
		t.Fatalf("unexpected query: %+v", query)
	}
	if query.Fields[1].Alias != "Имя" || !query.Order[1].Descending || query.Where == nil {
		t.Fatalf("unexpected fields/order/where: %+v", query)
	}
}

func TestParseEnglishQueryAndFilters(t *testing.T) {
	query, err := Parse(`SELECT DISTINCT Item.Code AS Code FROM Catalog.Items Item
WHERE Item.Code IN (&First, &Second) AND Item.Description IS NOT NULL
ORDER BY Code DESC`)
	if err != nil {
		t.Fatal(err)
	}
	if !query.Distinct || len(query.Fields) != 1 || len(query.Order) != 1 || !query.Order[0].Descending {
		t.Fatalf("unexpected query: %+v", query)
	}
}

func TestParseComparisonVariants(t *testing.T) {
	for _, source := range []string{
		`ВЫБРАТЬ Код ИЗ Справочник.Товары ГДЕ Код НЕ В ("A", "B")`,
		`ВЫБРАТЬ Код ИЗ Справочник.Товары ГДЕ Наименование НЕ ПОДОБНО &Шаблон`,
		`SELECT Code FROM Catalog.Items WHERE Description IS NOT NULL`,
		`ВЫБРАТЬ Код ИЗ Справочник.Товары ГДЕ НЕ (Код = "A" ИЛИ Код <> "B")`,
	} {
		if _, err := Parse(source); err != nil {
			t.Fatalf("Parse(%q): %v", source, err)
		}
	}
}

func TestParseRejectsUnsafeOrUnsupportedSyntax(t *testing.T) {
	for _, source := range []string{
		`ВЫБРАТЬ Ссылка ИЗ Справочник.Товары; DROP TABLE x`,
		`ВЫБРАТЬ Ссылка ИЗ Справочник.Товары ГДЕ Код = &`,
		`ВЫБРАТЬ ПЕРВЫЕ 100001 Ссылка ИЗ Справочник.Товары`,
		`ВЫБРАТЬ Ссылка ИЗ Справочник.Товары ГДЕ Код ЕСТЬ 1`,
	} {
		if _, err := Parse(source); err == nil || !strings.Contains(err.Error(), "query line") {
			t.Fatalf("Parse(%q) error=%v", source, err)
		}
	}
}

func TestParseRejectsExcessiveExpressionNesting(t *testing.T) {
	source := `ВЫБРАТЬ Код ИЗ Справочник.Товары ГДЕ ` + strings.Repeat("НЕ ", MaxNesting+1) + `Истина`
	if _, err := Parse(source); err == nil || !strings.Contains(err.Error(), "глубина") {
		t.Fatalf("Parse() error=%v", err)
	}
}
