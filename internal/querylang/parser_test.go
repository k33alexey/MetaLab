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

func TestParseJoinsGroupingAggregatesAndHaving(t *testing.T) {
	query, err := Parse(`
ВЫБРАТЬ
    Склады.Наименование КАК Склад,
    КОЛИЧЕСТВО(РАЗЛИЧНЫЕ Товары.Ссылка) КАК Товаров,
    СУММА(Товары.Цена) КАК Сумма
ИЗ Справочник.Склады КАК Склады
ЛЕВОЕ ВНЕШНЕЕ СОЕДИНЕНИЕ Справочник.Товары КАК Товары
ПО Товары.Склад = Склады.Ссылка
ГДЕ НЕ Склады.ПометкаУдаления
СГРУППИРОВАТЬ ПО Склады.Наименование
ИМЕЮЩИЕ СУММА(Товары.Цена) > &Минимум
УПОРЯДОЧИТЬ ПО Сумма УБЫВ`)
	if err != nil {
		t.Fatal(err)
	}
	if len(query.Joins) != 1 || query.Joins[0].Kind != JoinLeft || len(query.Group) != 1 || query.Having == nil || len(query.Fields) != 3 {
		t.Fatalf("unexpected extended query: %+v", query)
	}
	aggregate, ok := query.Fields[1].Expression.(Function)
	if !ok || aggregate.Name != "КОЛИЧЕСТВО" || !aggregate.Distinct || aggregate.Wildcard {
		t.Fatalf("unexpected aggregate: %+v", query.Fields[1].Expression)
	}
}

func TestParsePackageWithTemporaryTables(t *testing.T) {
	statements, err := ParsePackage(`
ВЫБРАТЬ Товары.*, Цена
ПОМЕСТИТЬ ВременныеТовары
ИЗ Справочник.Товары КАК Товары
ИНДЕКСИРОВАТЬ ПО Код;
SELECT T.Code, COUNT(*) AS Amount
FROM ВременныеТовары AS T
GROUP BY T.Code;
DROP ВременныеТовары`)
	if err != nil {
		t.Fatal(err)
	}
	if len(statements) != 3 || statements[0].Query == nil || statements[0].Query.Into == nil || len(statements[0].Query.IndexBy) != 1 || statements[1].Query == nil || statements[2].Drop == nil {
		t.Fatalf("unexpected package: %+v", statements)
	}
	if fields := statements[0].Query.Fields; !fields[0].Wildcard || len(fields[0].WildcardSource) != 1 || fields[0].WildcardSource[0] != "Товары" {
		t.Fatalf("unexpected qualified wildcard: %+v", fields[0])
	}
	if _, err := Parse(`ВЫБРАТЬ Код ИЗ ВременныеТовары; УНИЧТОЖИТЬ ВременныеТовары`); err == nil {
		t.Fatal("Parse accepted a package")
	}
}

func TestParseRightAndFullJoins(t *testing.T) {
	query, err := Parse(`SELECT A.Code FROM Catalog.A A RIGHT JOIN Catalog.B B ON A.Code = B.Code FULL OUTER JOIN Catalog.C C ON B.Code = C.Code`)
	if err != nil {
		t.Fatal(err)
	}
	if len(query.Joins) != 2 || query.Joins[0].Kind != JoinRight || query.Joins[1].Kind != JoinFull {
		t.Fatalf("unexpected joins: %+v", query.Joins)
	}
}

func TestParseRejectsIndexWithoutTemporaryTable(t *testing.T) {
	if _, err := Parse(`ВЫБРАТЬ Код ИЗ Справочник.Товары ИНДЕКСИРОВАТЬ ПО Код`); err == nil || !strings.Contains(err.Error(), "ПОМЕСТИТЬ") {
		t.Fatalf("index without temporary table error=%v", err)
	}
}

func BenchmarkParseExtendedQueryPackage(b *testing.B) {
	source := `ВЫБРАТЬ Л.Код, СУММА(П.Цена) КАК Сумма ПОМЕСТИТЬ Итоги
ИЗ Справочник.Товары КАК Л
ЛЕВОЕ СОЕДИНЕНИЕ Справочник.Товары КАК П ПО Л.Код = П.Код
СГРУППИРОВАТЬ ПО Л.Код
ИМЕЮЩИЕ СУММА(П.Цена) > &Минимум
ИНДЕКСИРОВАТЬ ПО Код;
ВЫБРАТЬ Код, Сумма ИЗ Итоги УПОРЯДОЧИТЬ ПО Сумма УБЫВ`
	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	for range b.N {
		if _, err := ParsePackage(source); err != nil {
			b.Fatal(err)
		}
	}
}
