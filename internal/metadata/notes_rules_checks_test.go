package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// notedBy runs one rule over a catalog built by hand and returns the places it
// noted, so a test sees exactly what the rule says and nothing else.
func notedBy(catalog *Catalog, rule func(*Catalog, func(where, written string))) []string {
	var places []string
	rule(catalog, func(where, written string) { places = append(places, where) })
	return places
}

// A link by type that leads nowhere is collected as a remnant where it
// stands - in an attribute of the object and in an attribute of its table
// part - and one that resolves is not. The place is named in the words of the
// project's files.
//
// Defect caught: a resolved link taken for a remnant; the holders inside
// lists not walked at all; the place named table-parts instead of table_parts.
func TestUnresolvedLinkByTypeIsNamedWhereItStands(t *testing.T) {
	t.Parallel()
	link := func(path FieldPath) FieldChoice { return FieldChoice{LinkByType: &TypeLink{Source: path}} }
	catalog := &Catalog{Documents: []DocumentDefinition{{
		Name: "Заказ",
		Attributes: []Attribute{
			{Name: "Вид", Choice: link(FieldPath{Unresolved: "Нет.Поля"})},
			{Name: "Склад", Choice: link(FieldPath{Standard: "ref"})},
		},
		TableParts: []TablePart{{Name: "Товары", Attributes: []Attribute{{Name: "Упаковка", Choice: link(FieldPath{Unresolved: "Нет.Другого"})}}}},
	}}}
	catalog.collectRemnants()
	var places []string
	for _, item := range catalog.unresolved {
		places = append(places, item.Where)
	}
	if len(places) != 2 {
		t.Fatalf("collected %v, want the two links that lead nowhere", places)
	}
	if !strings.Contains(places[0], "attributes Вид") || !strings.Contains(places[1], "table_parts Товары attributes Упаковка") {
		t.Fatalf("places named as %v", places)
	}
}

// A fixed length means nothing on a numeric number and is noted there; on a
// string number it is what the number is, and is not noted.
//
// Defect caught: the fixed length of a string number noted as carried and
// not followed.
func TestFixedLengthIsNotedOnANumericNumberOnly(t *testing.T) {
	t.Parallel()
	catalog := &Catalog{Documents: []DocumentDefinition{
		{Name: "Числовой", Number: DocumentNumber{Type: NumberType, Length: 9, FixedLength: true}},
		{Name: "Строковый", Number: DocumentNumber{Type: StringType, Length: 9, FixedLength: true}},
	}}
	places := notedBy(catalog, noteFixedLengthOfNumber)
	if len(places) != 1 || !strings.Contains(places[0], "Числовой") {
		t.Fatalf("noted %v, want the numeric number alone", places)
	}
}

// A register is noted for having no fields when it has none of the three
// kinds, and not when it has one dimension and one resource, or one resource
// and one attribute.
//
// Defect caught: the count of fields taken with a minus anywhere, which is zero
// for one dimension and one resource, or for one resource and one attribute.
func TestARegisterWithoutFieldsIsNoted(t *testing.T) {
	t.Parallel()
	field := Attribute{ID: uuid.MustNew(), Name: "Поле"}
	catalog := &Catalog{InformationRegisters: []InformationRegisterDefinition{
		{Name: "Пустой"},
		{Name: "Полный", Dimensions: []RegisterDimension{{Attribute: field}}, Resources: []Attribute{field}},
		{Name: "БезИзмерений", Resources: []Attribute{field}, Attributes: []Attribute{field}},
	}}
	places := notedBy(catalog, noteRegisterWithoutFields)
	if len(places) != 1 || !strings.Contains(places[0], "Пустой") {
		t.Fatalf("noted %v, want the register with no fields alone", places)
	}
}

// A table of a source must name its table in the source when it is a table -
// the type written out or left empty - and not when it is an expression.
//
// Defect caught: a table with its type written out not noted, or an
// expression noted for having no table name.
func TestATableOfASourceWithoutItsNameIsNoted(t *testing.T) {
	t.Parallel()
	catalog := &Catalog{ExternalDataSources: []ExternalDataSourceDefinition{{Name: "Склад", Tables: []ExternalTable{
		{Name: "БезТипа"},
		{Name: "Таблица", TableType: ExternalTableFromTable},
		{Name: "Выражение", TableType: ExternalTableFromExpression, ExpressionInDataSource: "SELECT 1"},
	}}}}
	var tables []string
	for _, place := range notedBy(catalog, noteEmptyNameInSource) {
		if strings.HasSuffix(place, "name_in_data_source") && !strings.Contains(place, " fields ") {
			tables = append(tables, place)
		}
	}
	if len(tables) != 2 || !strings.Contains(tables[0], "БезТипа") || !strings.Contains(tables[1], "Таблица") {
		t.Fatalf("tables noted %v, want the two tables and not the expression", tables)
	}
}

// A string of a source longer than a string the configuration can hold is
// noted; one of exactly 1024 fits and is not.
//
// Defect caught: a string of the longest length noted as too long.
func TestALongStringOfASourceIsNotedPastTheLimit(t *testing.T) {
	t.Parallel()
	field := func(name string, length int) ExternalField {
		return ExternalField{Attribute: Attribute{Name: name, Types: []Type{{Kind: StringType, Length: length}}}, NameInDataSource: name}
	}
	catalog := &Catalog{ExternalDataSources: []ExternalDataSourceDefinition{{Name: "Склад", Tables: []ExternalTable{{
		Name: "Товары", NameInDataSource: "goods", Fields: []ExternalField{field("Ровно", maxStringLength), field("Длиннее", maxStringLength+1)},
	}}}}}
	places := notedBy(catalog, noteLongExternalString)
	if len(places) != 1 || !strings.Contains(places[0], "Длиннее") {
		t.Fatalf("noted %v, want the longer field alone", places)
	}
}

// A kind is named the way its folder is: lowercase words joined by hyphens,
// with no hyphen in front.
//
// Defect caught: -charts-of-accounts, or the words run together.
func TestKindNamesAreKebabCase(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]string{"ChartsOfAccounts": "charts-of-accounts", "Catalogs": "catalogs"} {
		if got := kebab(name); got != want {
			t.Fatalf("kebab(%q) = %q, want %q", name, got, want)
		}
	}
}
