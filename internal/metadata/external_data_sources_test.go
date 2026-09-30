package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	warehouseSource      = "f8000000-0000-4000-8000-000000000001"
	otherSource          = "f8000000-0000-4000-8000-000000000002"
	goodsTable           = "f8000000-0000-4000-8000-000000000011"
	stockTable           = "f8000000-0000-4000-8000-000000000012"
	foreignTable         = "f8000000-0000-4000-8000-000000000013"
	goodsCodeField       = "f8000000-0000-4000-8000-000000000021"
	goodsNameField       = "f8000000-0000-4000-8000-000000000022"
	goodsParentField     = "f8000000-0000-4000-8000-000000000023"
	goodsVersionField    = "f8000000-0000-4000-8000-000000000024"
	stockGoodsField      = "f8000000-0000-4000-8000-000000000031"
	stockWarehouseField  = "f8000000-0000-4000-8000-000000000032"
	stockQuantityField   = "f8000000-0000-4000-8000-000000000033"
	foreignCodeField     = "f8000000-0000-4000-8000-000000000041"
	basedOnCatalogSource = "f8000000-0000-4000-8000-000000000051"
)

// writeExternalSource writes one source's description.
func writeExternalSource(t *testing.T, root, id, name, body string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "metadata", string(ExternalDataSourceKind), name, "object.yaml"), `format: 1
id: `+id+`
name: `+name+`
title: {ru: `+name+`}
`+body)
}

// writeExternalTable writes one table of a source, and the forms its
// description names by default: a default form names a form the table's folder
// keeps, and a fixture that named forms without keeping them would be refused
// for that before it said what it was written to say.
func writeExternalTable(t *testing.T, root, source, id, name, body string) {
	t.Helper()
	directory := filepath.Join(root, "metadata", string(ExternalDataSourceKind), source, ExternalDataSourceTablesDirectory, name)
	writeFile(t, filepath.Join(directory, "object.yaml"), `format: 1
id: `+id+`
name: `+name+`
title: {ru: `+name+`}
`+body)
	for _, line := range strings.Split(body, "\n") {
		forms, found := strings.CutPrefix(line, "forms: {")
		if !found {
			continue
		}
		for _, slot := range strings.Split(strings.TrimSuffix(forms, "}"), ",") {
			_, form, _ := strings.Cut(slot, ":")
			writeTableForm(t, directory, strings.TrimSpace(form))
		}
	}
}

// writeTableForm writes one form into a table's folder, with an identifier
// derived from where it lies so that no two fixtures share one.
func writeTableForm(t *testing.T, tableDirectory, form string) {
	t.Helper()
	id := uuid.Derive(uuid.UUID{}, filepath.Join(tableDirectory, form)).String()
	writeFile(t, filepath.Join(tableDirectory, "forms", form, "form.yaml"),
		"format: 1\nid: "+id+"\nname: "+form+"\ntitle: {ru: "+form+"}\nkind: list\n")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// goodsBody is an object table with every group of properties filled in: a
// hierarchy, a version, locks, input by string and the presentations.
const goodsBody = `name_in_data_source: dbo.Goods
data_type: object
key_fields: [Код]
presentation_field: Наименование
hierarchy:
  parent_field: Родитель
  unfilled_parent_value: {kind: string, data: ""}
data_version_field: Версия
data_lock: managed
data_lock_fields: [Код]
read_only: true
isolation_level: read-committed
edit_type: in-dialog
quick_choice: true
input_by_string: [Наименование, Код]
search_string_mode: any-part
choice_data_get_mode: background
create_on_input: dont-use
choice_history_on_input: auto
object_presentation: {ru: Товар}
list_presentation: {ru: Товары склада}
explanation: {ru: Номенклатура складской системы}
use_standard_commands: true
forms: {object: ФормаТовара, list: ФормаСписка, choice: ФормаВыбора}
fields:
  - id: ` + goodsCodeField + `
    name: Код
    title: {ru: Код}
    types: [{kind: string, length: 20}]
    name_in_data_source: code
  - id: ` + goodsNameField + `
    name: Наименование
    title: {ru: Наименование}
    types: [{kind: string, length: 150}]
    name_in_data_source: name
    allow_null: true
    presentation: {tooltip: {ru: Как товар называется на складе}}
    filling: {value: {kind: string, data: Без названия}}
  - id: ` + goodsParentField + `
    name: Родитель
    title: {ru: Родитель}
    types: [{kind: external-data-source-table, reference: ` + goodsTable + `}]
    name_in_data_source: parent_code
  - id: ` + goodsVersionField + `
    name: Версия
    title: {ru: Версия}
    types: [{kind: number, precision: 10}]
    name_in_data_source: row_version
    read_only: true
`

// stockBody is a table of records keyed by two fields, one of them a reference
// to the object table beside it.
const stockBody = `table_type: expression
expression_in_data_source: SELECT goods, warehouse, quantity FROM dbo.Stock WHERE quantity > &1
data_type: non-object
key_fields: [Товар, Склад]
record_presentation: {ru: Остаток}
forms: {record: ФормаЗаписи, list: ФормаСписка}
fields:
  - id: ` + stockGoodsField + `
    name: Товар
    title: {ru: Товар}
    types: [{kind: external-data-source-table, reference: ` + goodsTable + `}]
    name_in_data_source: goods
  - id: ` + stockWarehouseField + `
    name: Склад
    title: {ru: Склад}
    types: [{kind: string, length: 10}]
    name_in_data_source: warehouse
  - id: ` + stockQuantityField + `
    name: Количество
    title: {ru: Количество}
    types: [{kind: number, precision: 15, scale: 3}]
    name_in_data_source: quantity
`

func writeWarehouse(t *testing.T, root string) {
	t.Helper()
	writeExternalSource(t, root, warehouseSource, "Склад", "comment: Складская система\ndata_lock: automatic-and-managed\n")
	writeExternalTable(t, root, "Склад", goodsTable, "Товары", goodsBody)
	writeExternalTable(t, root, "Склад", stockTable, "Остатки", stockBody)
}

// refusedExternalTable writes the warehouse with the goods table replaced by
// the given body and returns the error the load must end with.
func refusedExternalTable(t *testing.T, body, what string) string {
	t.Helper()
	root := metadataProject(t)
	writeExternalSource(t, root, warehouseSource, "Склад", "")
	writeExternalTable(t, root, "Склад", goodsTable, "Товары", body)
	return loadRefused(t, root, what)
}

// A source carries its tables and a table carries every group of properties
// the help lists for it, fields included. Catches a kind still read by
// identity alone, a table folder nobody reads, and a property read into the
// wrong field or not at all.
func TestExternalDataSourceCarriesItsTablesAndTheirFields(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeWarehouse(t, root)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	source, ok := catalog.ExternalDataSource("склад")
	if !ok || source.Comment != "Складская система" || source.DataLock != "automatic-and-managed" {
		t.Fatalf("the source is not found by name or lost its properties: %+v", source)
	}
	if byID, ok := catalog.ExternalDataSourceByID(source.ID); !ok || byID.Name != "Склад" {
		t.Fatal("the source is not found by identifier")
	}
	if len(source.Tables) != 2 {
		t.Fatalf("a table was lost: %d tables", len(source.Tables))
	}
	goods, owner, ok := catalog.ExternalTableByID(mustUUID(t, goodsTable))
	if !ok || owner != "Склад" || !goods.ObjectTable() {
		t.Fatalf("the object table is not found by identifier: %+v", goods)
	}
	if goods.NameInDataSource != "dbo.Goods" || goods.PresentationField != "Наименование" || goods.DataVersionField != "Версия" ||
		!goods.ReadOnly || goods.IsolationLevel != IsolationReadCommitted || goods.DataLock != "managed" ||
		goods.EditType != EditInDialog || !goods.QuickChoice || goods.SearchStringMode != SearchAnyPart ||
		goods.ChoiceDataGetMode != ChoiceDataBackground || goods.CreateOnInput != "dont-use" || !goods.UseStandardCommands {
		t.Fatalf("a property of the table was lost: %+v", goods)
	}
	if strings.Join(goods.InputByString, ",") != "Наименование,Код" || strings.Join(goods.KeyFields, ",") != "Код" {
		t.Fatalf("a list of fields lost its order: %v %v", goods.InputByString, goods.KeyFields)
	}
	if goods.Hierarchy == nil || goods.Hierarchy.ParentField != "Родитель" || goods.Hierarchy.UnfilledParentValue == nil {
		t.Fatalf("the hierarchy was lost: %+v", goods.Hierarchy)
	}
	if goods.ObjectPresentation["ru"] != "Товар" || goods.Explanation["ru"] != "Номенклатура складской системы" || goods.Forms.Choice != "ФормаВыбора" {
		t.Fatalf("a presentation or a form was lost: %+v", goods)
	}
	name, ok := goods.Field("наименование")
	if !ok || name.NameInDataSource != "name" || !name.AllowNull || name.Presentation.ToolTip["ru"] == "" || name.Filling.Value == nil {
		t.Fatalf("a property of the field was lost: %+v", name)
	}
	if version, _ := goods.Field("Версия"); !version.ReadOnly {
		t.Fatal("a read-only field came back writable")
	}
	stock, _, _ := catalog.ExternalTableByID(mustUUID(t, stockTable))
	if stock.TableType != ExternalTableFromExpression || !strings.Contains(stock.ExpressionInDataSource, "&1") ||
		stock.ObjectTable() || stock.Forms.Record != "ФормаЗаписи" || stock.RecordPresentation["ru"] != "Остаток" {
		t.Fatalf("the table of records lost its properties: %+v", stock)
	}
	if len(catalog.OutlinedObjectsOf(ExternalDataSourceKind)) != 0 {
		t.Fatal("a source is still read as an object known by identity alone")
	}
}

// A copy handed out is a copy, down to the fields. Catches a clone that copies
// the source and shares its tables, or a table and shares its fields.
func TestExternalDataSourceIsHandedOutAsACopy(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeWarehouse(t, root)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	given, _ := catalog.ExternalDataSource("Склад")
	given.Tables[0].Name = "Испорчено"
	given.Tables[0].Fields[0].Name = "Испорчено"
	given.Tables[0].KeyFields = append(given.Tables[0].KeyFields[:0], "Испорчено")
	again, _ := catalog.ExternalDataSource("Склад")
	for _, table := range again.Tables {
		if table.Name == "Испорчено" || table.Fields[0].Name == "Испорчено" || (len(table.KeyFields) > 0 && table.KeyFields[0] == "Испорчено") {
			t.Fatalf("changing a copy changed the catalog: %+v", table)
		}
	}
}

// Every list a table keeps names its own fields. Catches a name that is not a
// field - a key nobody can build, a lock nobody can take, a search that never
// matches - and a field named twice.
func TestExternalTableListsNameItsOwnFields(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ from, to, says string }{
		"a key that is not a field":        {"key_fields: [Код]", "key_fields: [Артикул]", `key_fields[0] names "Артикул"`},
		"a lock by a field that is absent": {"data_lock_fields: [Код]", "data_lock_fields: [Артикул]", `data_lock_fields[0] names "Артикул"`},
		"a lock by one field twice":        {"data_lock_fields: [Код]", "data_lock_fields: [Код, код]", "names \"код\" twice"},
		"a search by a field that is absent": {"input_by_string: [Наименование, Код]", "input_by_string: [Артикул]",
			`input_by_string[0] names "Артикул"`},
		"a search by a reference": {"input_by_string: [Наименование, Код]", "input_by_string: [Родитель]",
			"searched by only if it is of one type"},
		"a presentation field that is absent": {"presentation_field: Наименование", "presentation_field: Артикул", `presentation_field names "Артикул"`},
		"a version field that is absent":      {"data_version_field: Версия", "data_version_field: Артикул", `data_version_field names "Артикул"`},
		"a parent field that is absent":       {"parent_field: Родитель", "parent_field: Артикул", `hierarchy.parent_field names "Артикул"`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if !strings.Contains(goodsBody, test.from) {
				t.Fatalf("the fixture has no %q", test.from)
			}
			if message := refusedExternalTable(t, strings.Replace(goodsBody, test.from, test.to, 1), name); !strings.Contains(message, test.says) {
				t.Fatalf("the error does not say what is wrong: %v", message)
			}
		})
	}
}

// An object table is keyed by one field and a table of records has no row to
// refer to. Catches the configurator's rule of the key not being held, and what
// only a row as a thing may have - a presentation, a hierarchy, a form of an
// object, characteristics - being accepted on a table of records.
func TestExternalTableDataTypeDecidesWhatTheTableMayHave(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ body, says string }{
		"an object table with two key fields": {strings.Replace(goodsBody, "key_fields: [Код]", "key_fields: [Код, Наименование]", 1),
			"key_fields must name exactly one field"},
		"an object table with no key": {strings.Replace(goodsBody, "key_fields: [Код]\n", "", 1), "key_fields must name exactly one field"},
		"an object table with a form of a record": {strings.Replace(goodsBody, "forms: {object: ФормаТовара,", "forms: {record: ФормаЗаписи, object: ФормаТовара,", 1),
			"forms.record belongs to a table of records"},
		"records with a presentation field": {strings.Replace(goodsBody, "data_type: object", "data_type: non-object", 1),
			"presentation_field belongs to a table of object data"},
		"records with a hierarchy": {strings.Replace(strings.Replace(goodsBody, "data_type: object", "data_type: non-object", 1), "presentation_field: Наименование\n", "", 1),
			"hierarchy belongs to a table of object data"},
		"no data type at all": {strings.Replace(goodsBody, "data_type: object\n", "", 1), "data_type must be object or non-object"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if message := refusedExternalTable(t, test.body, name); !strings.Contains(message, test.says) {
				t.Fatalf("the error does not say what is wrong: %v", message)
			}
		})
	}
}

// A table is made of a table of the source or of an expression, and says which
// by exactly one of two properties. Catches a table read from two places, and
// one read from none.
func TestExternalTableIsMadeOfATableOrAnExpression(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ body, says string }{
		"a table with no name": {strings.Replace(goodsBody, "name_in_data_source: dbo.Goods\n", "", 1), "name_in_data_source must name the table"},
		"a table with an expression too": {strings.Replace(goodsBody, "name_in_data_source: dbo.Goods\n", "name_in_data_source: dbo.Goods\nexpression_in_data_source: SELECT 1\n", 1),
			"expression_in_data_source belongs to a table made of an expression"},
		"an expression with a name too": {strings.Replace(goodsBody, "name_in_data_source: dbo.Goods\n", "table_type: expression\nname_in_data_source: dbo.Goods\nexpression_in_data_source: SELECT 1\n", 1),
			"name_in_data_source belongs to a table made of a table"},
		"an empty expression": {strings.Replace(goodsBody, "name_in_data_source: dbo.Goods\n", "table_type: expression\nexpression_in_data_source: \"  \"\n", 1),
			"expression_in_data_source must hold the expression"},
		"a name on two lines": {strings.Replace(goodsBody, "name_in_data_source: dbo.Goods", `name_in_data_source: "dbo.\nGoods"`, 1), "name_in_data_source must be one line"},
		"an unknown kind":     {strings.Replace(goodsBody, "name_in_data_source: dbo.Goods\n", "table_type: view\nname_in_data_source: dbo.Goods\n", 1), "table_type must be table or expression"},
		"an unknown isolation": {strings.Replace(goodsBody, "isolation_level: read-committed", "isolation_level: snapshot", 1),
			"isolation_level must be"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if message := refusedExternalTable(t, test.body, name); !strings.Contains(message, test.says) {
				t.Fatalf("the error does not say what is wrong: %v", message)
			}
		})
	}
}

// A field of a source is a field without the eight properties an attribute of
// our own objects has and it has not, and with a column it names. Catches each
// of the eight accepted and kept as a setting nobody reads, and a field that
// names no column.
func TestExternalFieldRefusesWhatOnlyAnAttributeHas(t *testing.T) {
	t.Parallel()
	const anchor = "    name_in_data_source: code\n"
	for property, line := range map[string]string{
		"indexing":                 "    indexing: index\n",
		"full_text_search":         "    full_text_search: use\n",
		"data_history":             "    data_history: use\n",
		"use":                      "    use: for-item\n",
		"presentation.min_value":   "    presentation: {min_value: {kind: string, data: a}}\n",
		"presentation.max_value":   "    presentation: {max_value: {kind: string, data: z}}\n",
		"choice.folders_and_items": "    choice: {folders_and_items: items}\n",
		"choice.link_by_type":      "    choice: {link_by_type: {source: {attribute: " + goodsNameField + "}}}\n",
	} {
		t.Run(property, func(t *testing.T) {
			t.Parallel()
			message := refusedExternalTable(t, strings.Replace(goodsBody, anchor, anchor+line, 1), property)
			if !strings.Contains(message, "fields[0]."+property+" belongs to an attribute") {
				t.Fatalf("the error does not name %s: %v", property, message)
			}
		})
	}
	t.Run("no column", func(t *testing.T) {
		t.Parallel()
		if message := refusedExternalTable(t, strings.Replace(goodsBody, anchor, "", 1), "a field with no column"); !strings.Contains(message, "fields[0].name_in_data_source must name the column") {
			t.Fatalf("the error does not say what is missing: %v", message)
		}
	})
	t.Run("two fields of one name", func(t *testing.T) {
		t.Parallel()
		body := strings.Replace(goodsBody, "    name: Версия\n", "    name: код\n", 1)
		if message := refusedExternalTable(t, body, "two fields of one name"); !strings.Contains(message, "fields[3].name is used twice") {
			t.Fatalf("the error does not say what is wrong: %v", message)
		}
	})
}

// A field refers only to an object table of its own source. Catches a
// reference to a table of records, which has no reference; to a table of
// another source, which is a key of another database; and to nothing.
func TestExternalFieldRefersToAnObjectTableOfItsOwnSource(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		target, says string
		foreign      bool
	}{
		"a table of records":        {stockTable, "holds records and has no reference", false},
		"a table of another source": {foreignTable, "refers to a table of another source", true},
		"nothing":                   {"f8000000-0000-4000-8000-0000000000ff", "refers to unknown table", false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeExternalSource(t, root, warehouseSource, "Склад", "")
			writeExternalTable(t, root, "Склад", goodsTable, "Товары", strings.Replace(goodsBody,
				"reference: "+goodsTable+"}]\n    name_in_data_source: parent_code", "reference: "+test.target+"}]\n    name_in_data_source: parent_code", 1))
			writeExternalTable(t, root, "Склад", stockTable, "Остатки", stockBody)
			if test.foreign {
				writeExternalSource(t, root, otherSource, "Бухгалтерия", "")
				writeExternalTable(t, root, "Бухгалтерия", foreignTable, "Счета", `name_in_data_source: accounts
data_type: object
key_fields: [Код]
fields:
  - id: `+foreignCodeField+`
    name: Код
    title: {ru: Код}
    types: [{kind: string, length: 10}]
    name_in_data_source: code
`)
			}
			if message := loadRefused(t, root, "a reference to "+name); !strings.Contains(message, test.says) {
				t.Fatalf("the error does not say what is wrong: %v", message)
			}
		})
	}
}

// The reference to a table of a source is not a type of the configuration's own
// objects yet. Catches it leaking into the type system through the one
// function every type goes through, where it would be accepted and stored as
// nothing.
func TestAReferenceToAnExternalTableIsNotYetATypeOfACatalogAttribute(t *testing.T) {
	t.Parallel()
	issues := validateTypes("types", []Type{{Kind: ExternalTableType, Reference: ptr(mustUUID(t, goodsTable))}}, uuid.UUID{})
	if len(issues) == 0 {
		t.Fatal("an attribute of the configuration's own object accepted a reference to a table of a source")
	}
}

// The values a table holds are values of the fields they stand for. Catches an
// unfilled parent value of a type the key does not have, and a filling value
// its field cannot hold.
func TestExternalTableValuesAreValuesOfTheirFields(t *testing.T) {
	t.Parallel()
	if message := refusedExternalTable(t, strings.Replace(goodsBody, `unfilled_parent_value: {kind: string, data: ""}`, `unfilled_parent_value: {kind: number, data: "0"}`, 1),
		"an unfilled parent of another type"); !strings.Contains(message, "hierarchy.unfilled_parent_value") {
		t.Fatalf("the error does not name the property: %v", message)
	}
	if message := refusedExternalTable(t, strings.Replace(goodsBody, "filling: {value: {kind: string, data: Без названия}}", `filling: {value: {kind: boolean, data: "true"}}`, 1),
		"a filling value of another type"); !strings.Contains(message, "fields[1].filling.value is of a type the field cannot hold") {
		t.Fatalf("the error does not name the field: %v", message)
	}
	// Absent is Null, the prototype's first choice, and is not refused.
	root := metadataProject(t)
	writeExternalSource(t, root, warehouseSource, "Склад", "")
	writeExternalTable(t, root, "Склад", goodsTable, "Товары", strings.Replace(goodsBody, "  unfilled_parent_value: {kind: string, data: \"\"}\n", "", 1))
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a hierarchy whose unfilled parent is Null was refused: %v", err)
	}
	if goods, _, _ := catalog.ExternalTableByID(mustUUID(t, goodsTable)); goods.Hierarchy.UnfilledParentValue != nil {
		t.Fatal("an absent unfilled parent value came back as a value")
	}
}

// A table's folder is an object folder: it keeps the modules of the roles the
// table has, the forms its defaults name, and nothing else. Catches a module of
// a role the table has not - an object module on a table of records, which
// nothing would ever run - a default form the folder does not keep, a stray
// file, and a table in a folder of another name.
func TestExternalTableFolderIsAnObjectFolder(t *testing.T) {
	t.Parallel()
	const goods, stock = "tables/Товары/", "tables/Остатки/"
	root := metadataProject(t)
	writeWarehouse(t, root)
	for _, path := range []string{goods + "МодульОбъекта.bsl", goods + "МодульМенеджера.bsl", stock + "МодульНабораЗаписей.bsl",
		goods + "forms/ФормаТовара/МодульФормы.bsl"} {
		writeFile(t, filepath.Join(root, "metadata", string(ExternalDataSourceKind), "Склад", filepath.FromSlash(path)), "\n")
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a table folder with the modules of its roles was refused: %v", err)
	}
	if names := catalog.ObjectFormNames(ExternalDataSourceTableKind, "Склад.Товары"); len(names) != 3 {
		t.Fatalf("the forms of the table were not indexed: %v", names)
	}
	for name, test := range map[string]struct{ path, says string }{
		"an object module on records":    {stock + "МодульОбъекта.bsl", "МодульОбъекта.bsl"},
		"a record set module on objects": {goods + "МодульНабораЗаписей.bsl", "МодульНабораЗаписей.bsl"},
		"a module of no role":            {goods + "МодульСервиса.bsl", "МодульСервиса.bsl"},
		"a template nobody declared":     {goods + "templates/Макет/content.yaml", "Макет"},
		"a stray file of a source":       {"заметки.txt", `keeps "заметки.txt"`},
		"a table that is not a folder":   {"tables/Товары.yaml", "a table is a folder"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeWarehouse(t, root)
			writeFile(t, filepath.Join(root, "metadata", string(ExternalDataSourceKind), "Склад", filepath.FromSlash(test.path)), "\n")
			if message := loadRefused(t, root, name); !strings.Contains(message, test.says) {
				t.Fatalf("the error does not name the file: %v", message)
			}
		})
	}
	t.Run("a default form the folder does not keep", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeWarehouse(t, root)
		if err := os.RemoveAll(filepath.Join(root, "metadata", string(ExternalDataSourceKind), "Склад", "tables", "Товары", "forms", "ФормаВыбора")); err != nil {
			t.Fatal(err)
		}
		if message := loadRefused(t, root, "a default form that is not there"); !strings.Contains(message, "ФормаВыбора") {
			t.Fatalf("the error does not name the form: %v", message)
		}
	})
	t.Run("a table in a folder of another name", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeWarehouse(t, root)
		tables := filepath.Join(root, "metadata", string(ExternalDataSourceKind), "Склад", ExternalDataSourceTablesDirectory)
		if err := os.Rename(filepath.Join(tables, "Товары"), filepath.Join(tables, "Номенклатура")); err != nil {
			t.Fatal(err)
		}
		if message := loadRefused(t, root, "a table in a folder of another name"); !strings.Contains(message, "lies in a folder called Номенклатура") {
			t.Fatalf("the error does not say what is wrong: %v", message)
		}
	})
}

// A table's commands and templates are declared and kept the way a catalog's
// are. Catches a command declared with no module behind it, and a table's
// command missing from the checks every command of the configuration goes
// through.
func TestExternalTableCommandsAndTemplatesAreDeclaredAndKept(t *testing.T) {
	t.Parallel()
	const command = "commands:\n  - id: f8000000-0000-4000-8000-000000000071\n    name: Печать\n    title: {ru: Печать}\n"
	root := metadataProject(t)
	writeExternalSource(t, root, warehouseSource, "Склад", "")
	writeExternalTable(t, root, "Склад", goodsTable, "Товары", goodsBody+command)
	if message := loadRefused(t, root, "a command with no module"); !strings.Contains(message, "Печать") {
		t.Fatalf("the error does not name the command: %v", message)
	}
	writeFile(t, filepath.Join(root, "metadata", string(ExternalDataSourceKind), "Склад", "tables", "Товары", "commands", "Печать", "МодульКоманды.bsl"), "\n")
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a command with its module was refused: %v", err)
	}
	goods, _, _ := catalog.ExternalTableByID(mustUUID(t, goodsTable))
	if len(goods.Commands) != 1 || goods.Commands[0].Name != "Печать" {
		t.Fatalf("the command came back as %+v", goods.Commands)
	}
	// The description of a command is read the way a catalog's command is:
	// two commands of one name are one command offered twice.
	twice := command + "  - id: f8000000-0000-4000-8000-000000000072\n    name: печать\n    title: {ru: Печать}\n"
	if message := refusedExternalTable(t, goodsBody+twice, "two commands of one name"); !strings.Contains(message, "commands[1]") {
		t.Fatalf("the error does not name the command: %v", message)
	}
	// A declared template is kept in its folder and accepted there.
	root = metadataProject(t)
	writeExternalSource(t, root, warehouseSource, "Склад", "")
	writeExternalTable(t, root, "Склад", goodsTable, "Товары", goodsBody+
		"templates:\n  - id: f8000000-0000-4000-8000-000000000073\n    name: Макет\n    title: {ru: Макет}\n    kind: spreadsheet\n")
	writeFile(t, filepath.Join(root, "metadata", string(ExternalDataSourceKind), "Склад", "tables", "Товары", "templates", "Макет", "content.yaml"), "rows: []\n")
	if withTemplate, err := Load(root); err != nil {
		t.Fatalf("a declared template in its folder was refused: %v", err)
	} else if table, _, _ := withTemplate.ExternalTableByID(mustUUID(t, goodsTable)); len(table.Templates) != 1 {
		t.Fatalf("the template came back as %+v", table.Templates)
	}
	owners := map[string]bool{}
	for _, owned := range catalog.everyObjectCommands() {
		owners[owned.owner] = true
	}
	if !owners["external data source table Склад.Товары"] {
		t.Fatalf("the table's commands are not among the commands of the configuration: %v", owners)
	}
}

// The modules of a table are compiled under names that carry the source, and
// its forms' modules too. Catches two tables of one name in two sources being
// compiled under one name, and a table's module falling back to a name built
// from its path.
func TestExternalTableModulesAreNamedWithTheirSource(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeWarehouse(t, root)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	names := moduleNameDescriptors(catalog)
	for path, want := range map[string]string{
		"metadata/external-data-sources/Склад/tables/Товары/МодульОбъекта.bsl":                 "МодульОбъектаТаблицыВнешнегоИсточника.Склад.Товары",
		"metadata/external-data-sources/Склад/tables/Товары/МодульМенеджера.bsl":               "МодульМенеджераТаблицыВнешнегоИсточника.Склад.Товары",
		"metadata/external-data-sources/Склад/tables/Остатки/МодульНабораЗаписей.bsl":          "МодульНабораЗаписейТаблицыВнешнегоИсточника.Склад.Остатки",
		"metadata/external-data-sources/Склад/tables/Товары/forms/ФормаТовара/МодульФормы.bsl": "МодульФормыТаблицыВнешнегоИсточника.Склад.Товары.ФормаТовара",
	} {
		if got := names[path].name; got != want {
			t.Errorf("%s is compiled as %q, want %q", path, got, want)
		}
	}
	if _, ok := names["metadata/external-data-sources/Склад/tables/Остатки/МодульОбъекта.bsl"]; ok {
		t.Error("a table of records was given an object module name")
	}
}

// Identifiers are unique across the whole configuration and table names within
// their source. Catches a table that shares an identifier with an object of
// another kind, and two tables of one name in two sources being refused.
func TestExternalTableIdentityIsCheckedLikeEveryOther(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeWarehouse(t, root)
	writeExternalSource(t, root, otherSource, "Бухгалтерия", "")
	writeExternalTable(t, root, "Бухгалтерия", foreignTable, "Товары", strings.NewReplacer(
		goodsCodeField, "f8000000-0000-4000-8000-000000000061",
		goodsNameField, "f8000000-0000-4000-8000-000000000062",
		goodsParentField, "f8000000-0000-4000-8000-000000000063",
		goodsVersionField, "f8000000-0000-4000-8000-000000000064",
		"reference: "+goodsTable, "reference: "+foreignTable).Replace(goodsBody))
	if _, err := Load(root); err != nil {
		t.Fatalf("two sources with a table of one name were refused: %v", err)
	}
	writeWSReference(t, root, stockTable, "Склад", "", wsdl11)
	if message := loadRefused(t, root, "a WS reference sharing a table's identifier"); !strings.Contains(message, "use "+stockTable) {
		t.Fatalf("the error does not name the identifier: %v", message)
	}
}

// A table of object data is entered on the basis of other objects and may be
// one's basis. Catches a table left out of the list of carriers, so that its
// list is never resolved and a basis nobody declared is accepted.
func TestExternalTableIsEnteredOnTheBasisOfObjectsOfTheConfiguration(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeWarehouse(t, root)
	writeExternalTable(t, root, "Склад", goodsTable, "Товары", goodsBody+"based_on: ["+basedOnCatalogSource+"]\n")
	if message := loadRefused(t, root, "a basis nobody declared"); !strings.Contains(message, "is entered on the basis of "+basedOnCatalogSource) {
		t.Fatalf("the error does not name the basis: %v", message)
	}
}

func ptr[T any](value T) *T { return &value }

const (
	propertyKindsTable  = "f8000000-0000-4000-8000-000000000081"
	propertyKindsKey    = "f8000000-0000-4000-8000-000000000082"
	propertyValuesTable = "f8000000-0000-4000-8000-000000000083"
	propertyValuesOwner = "f8000000-0000-4000-8000-000000000084"
	propertyValuesKind  = "f8000000-0000-4000-8000-000000000085"
	propertyValuesValue = "f8000000-0000-4000-8000-000000000086"
)

// writePropertyTables writes the two tables of the source a characteristic of
// the goods table is read from: the kinds of properties, an object table, and
// their values, a table of records keyed by the goods row and the kind.
func writePropertyTables(t *testing.T, root, ownerType string) {
	t.Helper()
	writeExternalTable(t, root, "Склад", propertyKindsTable, "ВидыСвойств", `name_in_data_source: dbo.PropertyKinds
data_type: object
key_fields: [Код]
fields:
  - id: `+propertyKindsKey+`
    name: Код
    title: {ru: Код}
    types: [{kind: string, length: 10}]
    name_in_data_source: code
`)
	writeExternalTable(t, root, "Склад", propertyValuesTable, "ЗначенияСвойств", `name_in_data_source: dbo.PropertyValues
data_type: non-object
key_fields: [Объект, Свойство]
fields:
  - id: `+propertyValuesOwner+`
    name: Объект
    title: {ru: Объект}
    types: [`+ownerType+`]
    name_in_data_source: object
  - id: `+propertyValuesKind+`
    name: Свойство
    title: {ru: Свойство}
    types: [{kind: external-data-source-table, reference: `+propertyKindsTable+`}]
    name_in_data_source: kind
  - id: `+propertyValuesValue+`
    name: Значение
    title: {ru: Значение}
    types: [{kind: string, length: 100}]
    name_in_data_source: value
`)
}

// goodsCharacteristics describes where the goods table's characteristics are
// read from, with the key of the kinds table given by the caller.
func goodsCharacteristics(kindsTable, key string) string {
	return `characteristics:
  - types:
      table: {kind: external-data-source-tables, object: ` + kindsTable + `}
      key: ` + key + `
    values:
      table: {kind: external-data-source-tables, object: ` + propertyValuesTable + `}
      object: {attribute: ` + propertyValuesOwner + `}
      type: {attribute: ` + propertyValuesKind + `}
      value: {attribute: ` + propertyValuesValue + `}
`
}

// The characteristics of a table of a source are resolved against the tables
// they name, and those are tables of the same source. Catches a description
// that reads a table which is not there, a field of the values that cannot hold
// a reference to the table it keeps values of, and a standard field a table of
// a source does not have - each of which leaves the table silently without
// any additional property.
func TestExternalTableCharacteristicsAreResolved(t *testing.T) {
	t.Parallel()
	const goodsRef = "{kind: external-data-source-table, reference: " + goodsTable + "}"
	root := metadataProject(t)
	writeExternalSource(t, root, warehouseSource, "Склад", "")
	writeExternalTable(t, root, "Склад", goodsTable, "Товары", goodsBody+goodsCharacteristics(propertyKindsTable, "{standard: ref}"))
	writePropertyTables(t, root, goodsRef)
	if _, err := Load(root); err != nil {
		t.Fatalf("characteristics read from tables of the source were refused: %v", err)
	}
	for name, test := range map[string]struct{ kinds, key, owner, says string }{
		"a table of kinds that is not there": {"f8000000-0000-4000-8000-0000000000ee", "{standard: ref}", goodsRef, "which is not in the configuration"},
		"values that cannot hold the goods":  {propertyKindsTable, "{standard: ref}", "{kind: string, length: 10}", "values.object cannot hold a reference to external data source table Склад.Товары"},
		"a code a table of a source has not": {propertyKindsTable, "{standard: code}", goodsRef, "names the standard field code"},
		"a key the kinds table has not":      {propertyKindsTable, "{attribute: " + propertyValuesValue + "}", goodsRef, "names attribute " + propertyValuesValue},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeExternalSource(t, root, warehouseSource, "Склад", "")
			writeExternalTable(t, root, "Склад", goodsTable, "Товары", goodsBody+goodsCharacteristics(test.kinds, test.key))
			writePropertyTables(t, root, test.owner)
			if message := loadRefused(t, root, name); !strings.Contains(message, test.says) {
				t.Fatalf("the error does not say what is wrong: %v", message)
			}
		})
	}
	// A table of records has no reference: it can keep values, but its rows
	// cannot be the kinds a key refers to.
	t.Run("a reference a table of records has not", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeExternalSource(t, root, warehouseSource, "Склад", "")
		writeExternalTable(t, root, "Склад", goodsTable, "Товары", goodsBody+goodsCharacteristics(propertyValuesTable, "{standard: ref}"))
		writePropertyTables(t, root, goodsRef)
		if message := loadRefused(t, root, "a reference to a row of records"); !strings.Contains(message, "names the standard field ref") {
			t.Fatalf("the error does not say what is wrong: %v", message)
		}
	})
}
