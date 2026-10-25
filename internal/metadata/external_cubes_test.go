package metadata

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	salesCube         = "f8100000-0000-4000-8000-000000000001"
	goodsDimension    = "f8100000-0000-4000-8000-000000000002"
	quantityResource  = "f8100000-0000-4000-8000-000000000003"
	goodsDimTable     = "f8100000-0000-4000-8000-000000000011"
	goodsDimTableCode = "f8100000-0000-4000-8000-000000000012"
	goodsDimTableName = "f8100000-0000-4000-8000-000000000013"
	analogDimension   = "f8100000-0000-4000-8000-000000000004"
)

// cubeDirectory is the folder of the sales cube of the warehouse source.
func cubeDirectory(root string) string {
	return filepath.Join(root, "metadata", string(ExternalDataSourceKind), "Склад", ExternalDataSourceCubesDirectory, "Продажи")
}

const salesCubeBody = `name_in_data_source: '[Sales]'
list_presentation: {ru: Продажи}
record_presentation: {ru: Продажа}
use_standard_commands: true
forms: {list: ФормаСписка}
dimensions:
  - id: ` + goodsDimension + `
    name: Товар
    title: {ru: Товар}
    types: [{kind: external-data-source-dimension-table, reference: ` + goodsDimTable + `}]
    choice: {folders_and_items: items}
  - id: ` + analogDimension + `
    name: Аналог
    title: {ru: Аналог}
    types: [{kind: external-data-source-dimension-table, reference: ` + goodsDimTable + `}]
    presentation: {min_value: "2020-01-01T00:00:00Z"}
resources:
  - id: ` + quantityResource + `
    name: Количество
    title: {ru: Количество}
    types: [{kind: number, precision: 15, scale: 3}]
    name_in_data_source: '[Measures].[Quantity]'
`

const goodsDimTableBody = `name_in_data_source: '[Goods]'
hierarchy_name_in_data_source: '[Goods].[Groups]'
hierarchical: true
unfilled_parent_value: ""
presentation_field: Наименование
quick_choice: true
object_presentation: {ru: Товар}
forms: {object: ФормаТовара, choice: ФормаВыбора}
fields:
  - id: ` + goodsDimTableCode + `
    name: Код
    title: {ru: Код}
    types: [{kind: string, length: 20}]
    name_in_data_source: '[Goods].[Code]'
  - id: ` + goodsDimTableName + `
    name: Наименование
    title: {ru: Наименование}
    types: [{kind: string, length: 150}]
    name_in_data_source: '[Goods].[Name]'
`

// writeSalesCube writes the warehouse source with one cube and one dimension
// table, and the forms their defaults name.
func writeSalesCube(t *testing.T, root, cubeBody, dimensionBody string) {
	t.Helper()
	writeExternalSource(t, root, warehouseSource, "Склад", "")
	cube := cubeDirectory(root)
	writeFile(t, filepath.Join(cube, "object.yaml"), "format: 1\nid: "+salesCube+"\nname: Продажи\ntitle: {ru: Продажи}\n"+cubeBody)
	if strings.Contains(cubeBody, "ФормаСписка") {
		writeTableForm(t, cube, "ФормаСписка")
	}
	table := filepath.Join(cube, ExternalDimensionTablesDirectory, "Товары")
	writeFile(t, filepath.Join(table, "object.yaml"), "format: 1\nid: "+goodsDimTable+"\nname: Товары\ntitle: {ru: Товары}\n"+dimensionBody)
	for _, form := range []string{"ФормаТовара", "ФормаВыбора"} {
		if strings.Contains(dimensionBody, form) {
			writeTableForm(t, table, form)
		}
	}
}

// A cube carries its dimensions, its resources and its dimension tables, and a
// dimension refers to a member of one of them. Catches a cube folder nobody
// reads, a dimension table left out of its cube, and a property of either read
// into the wrong field or not at all.
func TestExternalCubeCarriesItsDimensionsResourcesAndTables(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeSalesCube(t, root, salesCubeBody, goodsDimTableBody)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	cube, ok := catalog.ExternalCube("склад", "продажи")
	if !ok {
		t.Fatal("the cube is not found by the names of its source and itself")
	}
	if cube.Dimensions[0].Choice.FoldersAndItems == "" || cube.Dimensions[1].Presentation.MinValue == nil || cube.Resources[0].NameInDataSource != "[Measures].[Quantity]" {
		t.Fatalf("a property of a dimension or a resource was lost: %+v %+v", cube.Dimensions, cube.Resources)
	}
	if cube.NameInDataSource != "[Sales]" || len(cube.Dimensions) != 2 || len(cube.Resources) != 1 ||
		cube.ListPresentation["ru"] != "Продажи" || cube.RecordPresentation["ru"] != "Продажа" || !cube.UseStandardCommands || cube.Forms.List != "ФормаСписка" {
		t.Fatalf("a property of the cube was lost: %+v", cube)
	}
	if len(cube.DimensionTables) != 1 {
		t.Fatalf("the dimension table was lost: %+v", cube.DimensionTables)
	}
	table := cube.DimensionTables[0]
	if table.HierarchyNameInDataSource != "[Goods].[Groups]" || !table.Hierarchical || table.UnfilledParentValue == nil ||
		*table.UnfilledParentValue != "" || table.PresentationField != "Наименование" || len(table.Fields) != 2 || table.Forms.Choice != "ФормаВыбора" ||
		!table.QuickChoice {
		t.Fatalf("a property of the dimension table was lost: %+v", table)
	}
	// A copy handed out is a copy, down to the dimension tables.
	cube.DimensionTables[0].Name = "Испорчено"
	cube.Dimensions[0].Name = "Испорчено"
	again, _ := catalog.ExternalCube("Склад", "Продажи")
	if again.DimensionTables[0].Name != "Товары" || again.Dimensions[0].Name != "Товар" {
		t.Fatal("changing a copy changed the catalog")
	}
}

// What a cube and a dimension table are checked for on their own. Catches a
// dimension and a resource of one name - one field of the cube read twice -
// and a presentation field that is not a field, and a part of either sharing
// its identifier with a part of the other. A cube or a resource with no
// name in the source and an unfilled parent without a hierarchy are carried
// and noted (notes_test.go).
func TestExternalCubeAndDimensionTableAreCheckedOnTheirOwn(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ cube, dimension, says string }{
		"a dimension and a resource of one name": {strings.Replace(salesCubeBody, "    name: Количество", "    name: Товар", 1), goodsDimTableBody, "resources[0].name is used twice"},
		"a presentation field that is absent":    {salesCubeBody, strings.Replace(goodsDimTableBody, "presentation_field: Наименование", "presentation_field: Артикул", 1), `presentation_field names "Артикул"`},
		"an attribute's property on a dimension": {strings.Replace(salesCubeBody, "    title: {ru: Аналог}\n", "    title: {ru: Аналог}\n    indexing: index\n", 1), goodsDimTableBody, "dimensions[1].indexing belongs to an attribute"},
		// A dimension is found through its dimension table and has no column
		// of its own: the strict reading names the property.
		"a column on a dimension": {strings.Replace(salesCubeBody, "    title: {ru: Аналог}\n", "    title: {ru: Аналог}\n    name_in_data_source: '[Analog]'\n", 1), goodsDimTableBody, "name_in_data_source not found"},
		"a filling on a resource": {strings.Replace(salesCubeBody, "    name_in_data_source: '[Measures].[Quantity]'\n", "    name_in_data_source: '[Measures].[Quantity]'\n    filling: {value: {kind: number, data: \"0\"}}\n", 1), goodsDimTableBody, "resources[0].filling belongs to a field somebody enters"},
		// The parts of a cube and of a dimension table are in the one walk
		// of what an object holds, so their identifiers are unique across
		// the configuration, not only within the object.
		"a field of a dimension table with a dimension's identifier": {salesCubeBody, strings.Replace(goodsDimTableBody, "id: "+goodsDimTableCode, "id: "+goodsDimension, 1),
			"cube Склад.Продажи dimension Товар and dimension table Склад.Продажи.Товары field Код use"},
		"bounds on a resource": {strings.Replace(salesCubeBody, "    name_in_data_source: '[Measures].[Quantity]'\n", "    name_in_data_source: '[Measures].[Quantity]'\n    presentation: {max_value: \"9\"}\n", 1), goodsDimTableBody, "resources[0].presentation.max_value belongs to"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeSalesCube(t, root, test.cube, test.dimension)
			if message := loadRefused(t, root, name); !strings.Contains(message, test.says) {
				t.Fatalf("the error does not say what is wrong: %v", message)
			}
		})
	}
}

// A dimension refers to a member of a dimension table of its own source.
// Catches a reference to a dimension table that is not there, and a reference
// to a table of another source's cube, which is a key of another database.
func TestExternalCubeDimensionRefersToADimensionTableOfItsSource(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeSalesCube(t, root, strings.Replace(salesCubeBody, "reference: "+goodsDimTable, "reference: f8100000-0000-4000-8000-0000000000ff", 1), goodsDimTableBody)
	if message := loadRefused(t, root, "a reference to nothing"); !strings.Contains(message, "refers to unknown dimension table") {
		t.Fatalf("the error does not say what is wrong: %v", message)
	}
}

// A cube's folder and a dimension table's are object folders, each keeping the
// modules of its roles. Catches a cube given an object module - a cube is read
// as records - a dimension table given a record set module, a default form the
// folder does not keep, and the dimension tables' folder refused as a stray
// folder of the cube.
func TestExternalCubeFoldersAreObjectFolders(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeSalesCube(t, root, salesCubeBody, goodsDimTableBody)
	cube := cubeDirectory(root)
	table := filepath.Join(cube, ExternalDimensionTablesDirectory, "Товары")
	for _, path := range []string{filepath.Join(cube, "МодульНабораЗаписей.bsl"), filepath.Join(cube, "МодульМенеджера.bsl"),
		filepath.Join(table, "МодульОбъекта.bsl"), filepath.Join(table, "МодульМенеджера.bsl")} {
		writeFile(t, path, "\n")
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("the folders with the modules of their roles were refused: %v", err)
	}
	namer := catalog.ModuleNamer()
	for path, want := range map[string]string{
		"metadata/external-data-sources/Склад/cubes/Продажи/МодульНабораЗаписей.bsl":                                   "МодульНабораЗаписейКубаВнешнегоИсточника.Склад.Продажи",
		"metadata/external-data-sources/Склад/cubes/Продажи/dimension-tables/Товары/МодульОбъекта.bsl":                 "МодульОбъектаТаблицыИзмеренияВнешнегоИсточника.Склад.Продажи.Товары",
		"metadata/external-data-sources/Склад/cubes/Продажи/dimension-tables/Товары/forms/ФормаВыбора/МодульФормы.bsl": "МодульФормыТаблицыИзмеренияВнешнегоИсточника.Склад.Продажи.Товары.ФормаВыбора",
	} {
		if got := namer.Identify(path).Name; got != want {
			t.Errorf("%s is compiled as %q, want %q", path, got, want)
		}
	}
	// Paths below are relative to the cube's folder of each project.
	for name, test := range map[string]struct{ path, says string }{
		"an object module on a cube":               {"МодульОбъекта.bsl", "МодульОбъекта.bsl"},
		"a record set module on a dimension table": {"dimension-tables/Товары/МодульНабораЗаписей.bsl", "МодульНабораЗаписей.bsl"},
		"a stray folder in a cube":                 {"tables/x.yaml", `folder "tables"`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeSalesCube(t, root, salesCubeBody, goodsDimTableBody)
			writeFile(t, filepath.Join(cubeDirectory(root), filepath.FromSlash(test.path)), "\n")
			if message := loadRefused(t, root, name); !strings.Contains(message, test.says) {
				t.Fatalf("the error does not name the file: %v", message)
			}
		})
	}
	t.Run("a default form the cube does not keep", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeSalesCube(t, root, strings.Replace(salesCubeBody, "forms: {list: ФормаСписка}", "forms: {list: ФормаСписка, record: ФормаЗаписи}", 1), goodsDimTableBody)
		if message := loadRefused(t, root, "a missing record form"); !strings.Contains(message, "ФормаЗаписи") {
			t.Fatalf("the error does not name the form: %v", message)
		}
	})
}

// A cube carries characteristics: the syntax assistant does not give it any, an
// export writes the property on a cube all the same. They are checked in shape
// and not resolved - a record of a cube has no reference to hang them off.
// Catches the property refused as unknown, and a malformed description let
// through because it is not resolved.
func TestExternalCubeCarriesCharacteristicsInShape(t *testing.T) {
	t.Parallel()
	const characteristics = `characteristics:
  - types:
      table: {kind: external-data-source-tables, object: ` + goodsTable + `}
      key: {standard: ref}
    values:
      table: {kind: external-data-source-tables, object: ` + stockTable + `}
      object: {attribute: ` + stockGoodsField + `}
      type: {attribute: ` + stockWarehouseField + `}
      value: {attribute: ` + stockQuantityField + `}
`
	root := metadataProject(t)
	writeSalesCube(t, root, salesCubeBody+characteristics, goodsDimTableBody)
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a cube with characteristics was refused: %v", err)
	}
	cube, _ := catalog.ExternalCube("Склад", "Продажи")
	if len(cube.Characteristics) != 1 {
		t.Fatalf("the characteristics were lost: %+v", cube.Characteristics)
	}
	cube.Characteristics[0].Types.Key.Standard = "испорчено"
	if again, _ := catalog.ExternalCube("Склад", "Продажи"); again.Characteristics[0].Types.Key.Standard != RefStandardField {
		t.Fatal("changing a copy's characteristics changed the catalog")
	}
	// A characteristic with its key left unnamed is what the prototype saves
	// with the tables chosen and the fields not: carried, offering nothing.
	root = metadataProject(t)
	writeSalesCube(t, root, salesCubeBody+strings.Replace(characteristics, "      key: {standard: ref}\n", "      key: {}\n", 1), goodsDimTableBody)
	unfilled, err := Load(root)
	if err != nil {
		t.Fatalf("an unfilled characteristic was refused: %v", err)
	}
	if cube, _ := unfilled.ExternalCube("Склад", "Продажи"); len(cube.Characteristics) != 1 || cube.Characteristics[0].Filled() {
		t.Fatalf("the unfilled characteristic was lost or reads as filled: %+v", cube.Characteristics)
	}
}

// A dimension of one cube may refer to a dimension table of another cube of
// its source: the editor of its type offers the dimension tables of every
// cube, and «Проверка конфигурации» takes such a dimension (opis of
// 24.10.2026, vneshnij-istochnik.md, snapshots series-2.531/20, 23-25); it is
// one database, so nothing is noted. A field of a dimension table may too -
// its own table, another cube's, another source's, the last noted (opis of
// 25.10.2026, snapshots series-2.535/03-05). A resource refers to no table,
// its own cube's or another's (snapshots 01, 02); the rest of what a
// resource is TestAResourceOfACubeIsANumberOrAString's.
// Catches the dimension or the field refused, as they were until 2.539 and
// 2.545, noted as a reference to another source within one; a resource
// referring to its own cube's table taken, as it was until 2.545.
func TestAnExternalCubeDimensionMayReferToAnotherCube(t *testing.T) {
	t.Parallel()
	const otherCube, otherTable = "f8100000-0000-4000-8000-000000000021", "f8100000-0000-4000-8000-000000000022"
	const foreignCube, foreignDimTable = "f8100000-0000-4000-8000-000000000031", "f8100000-0000-4000-8000-000000000032"
	purchases := func(root string) {
		other := filepath.Join(root, "metadata", string(ExternalDataSourceKind), "Склад", ExternalDataSourceCubesDirectory, "Закупки")
		writeFile(t, filepath.Join(other, "object.yaml"), "format: 1\nid: "+otherCube+"\nname: Закупки\ntitle: {ru: Закупки}\nname_in_data_source: Purchases\n")
		writeFile(t, filepath.Join(other, ExternalDimensionTablesDirectory, "Поставщики", "object.yaml"),
			"format: 1\nid: "+otherTable+"\nname: Поставщики\ntitle: {ru: Поставщики}\nname_in_data_source: Suppliers\n")
		writeExternalSource(t, root, otherSource, "Бухгалтерия", "")
		foreign := filepath.Join(root, "metadata", string(ExternalDataSourceKind), "Бухгалтерия", ExternalDataSourceCubesDirectory, "Обороты")
		writeFile(t, filepath.Join(foreign, "object.yaml"), "format: 1\nid: "+foreignCube+"\nname: Обороты\ntitle: {ru: Обороты}\nname_in_data_source: Turnovers\n")
		writeFile(t, filepath.Join(foreign, ExternalDimensionTablesDirectory, "Счета", "object.yaml"),
			"format: 1\nid: "+foreignDimTable+"\nname: Счета\ntitle: {ru: Счета}\nname_in_data_source: Accounts\n")
	}
	root := metadataProject(t)
	writeSalesCube(t, root, strings.Replace(salesCubeBody, "reference: "+goodsDimTable, "reference: "+otherTable, 1), goodsDimTableBody)
	purchases(root)
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a dimension of another cube's table was refused: %v", err)
	}
	if slices.ContainsFunc(catalog.Notes(), func(note Note) bool { return note.Kind == NoteForeignSourceReference }) {
		t.Errorf("another cube of the same source is noted as another source: %+v", catalog.Notes())
	}
	field := func(name, table string) string {
		return "  - {id: " + uuid.MustNew().String() + ", name: " + name + ", title: {ru: " + name + "}, " +
			"types: [{kind: external-data-source-dimension-table, reference: " + table + "}], name_in_data_source: " + name + "}\n"
	}
	root = metadataProject(t)
	writeSalesCube(t, root, salesCubeBody, goodsDimTableBody+field("Аналог", goodsDimTable)+field("Поставщик", otherTable)+field("Счет", foreignDimTable))
	purchases(root)
	catalog, err = Load(root)
	if err != nil {
		t.Fatalf("a field of a dimension table of its own, another cube's and another source's table was refused: %v", err)
	}
	var noted []string
	for _, note := range catalog.Notes() {
		if note.Kind == NoteForeignSourceReference {
			noted = append(noted, note.Where)
		}
	}
	if want := []string{"external-data-sources Склад cubes Продажи dimension-tables Товары fields Счет"}; !slices.Equal(noted, want) {
		t.Errorf("noted %v, want %v", noted, want)
	}
	resource := func(types string) string {
		return strings.Replace(salesCubeBody, "    types: [{kind: number, precision: 15, scale: 3}]\n", "    types: "+types+"\n", 1)
	}
	objectTable := uuid.MustNew().String()
	for name, written := range map[string]string{
		"another cube's": "[{kind: external-data-source-dimension-table, reference: " + otherTable + "}]",
		"its own cube's": "[{kind: external-data-source-dimension-table, reference: " + goodsDimTable + "}]",
		"an object":      "[{kind: external-data-source-table, reference: " + objectTable + "}]",
	} {
		root = metadataProject(t)
		writeSalesCube(t, root, resource(written), goodsDimTableBody)
		purchases(root)
		writeExternalTable(t, root, "Склад", objectTable, "Клиенты", "name_in_data_source: clients\ndata_type: object\nkey_fields: [Код]\nfields:\n"+
			"  - {id: "+uuid.MustNew().String()+", name: Код, title: {ru: Код}, types: [{kind: string, length: 10}], name_in_data_source: code}\n")
		if message := loadRefused(t, root, "a resource of "+name+" table"); !strings.Contains(message, "resource Количество is not a number or a string") {
			t.Errorf("a resource of %s table: %v", name, message)
		}
	}
}

// A resource of a cube is a number or a string: «Проверка конфигурации»
// refuses one whose defined type holds a date, a boolean, an identifier, a
// value storage, binary data, «Число + Дата», a reference to an object table
// or to a catalog - «Недопустимый тип» - and takes a defined type of a string,
// of a number and of «Число + Строка» (opis of 25.10.2026,
// vneshnij-istochnik.md, «Досъёмка 25.10.2026 (вопросы В1–В2 серии 2.543)»,
// snapshots series-2.543/04-06). The check judges the type expanded, so a
// date written out is refused as well, and a number beside a string written
// out is taken. The types are written here from the opis.
//
// Catches a defined type of a date or of a catalog taken in a resource, as it
// was until 2.548; the composite of a number and a string refused; the type
// written out judged apart from the one a defined type gives.
func TestAResourceOfACubeIsANumberOrAString(t *testing.T) {
	t.Parallel()
	const catalogID, objectTable = "f8100000-0000-4000-8000-000000000041", "f8100000-0000-4000-8000-000000000042"
	resource := func(types string) string {
		return strings.Replace(salesCubeBody, "    types: [{kind: number, precision: 15, scale: 3}]\n", "    types: "+types+"\n", 1)
	}
	project := func(cubeBody string) string {
		root := metadataProject(t)
		writeMetadata(t, root, CatalogKind, catalogID, "format: 1\nid: "+catalogID+"\nname: Валюты\ntitle: {ru: Валюты}\n"+
			"code: {type: string, length: 3}\ndescription_length: 50\n")
		writeSalesCube(t, root, cubeBody, goodsDimTableBody)
		writeExternalTable(t, root, "Склад", objectTable, "Клиенты", "name_in_data_source: clients\ndata_type: object\nkey_fields: [Код]\nfields:\n"+
			"  - {id: "+uuid.MustNew().String()+", name: Код, title: {ru: Код}, types: [{kind: string, length: 10}], name_in_data_source: code}\n")
		return root
	}
	const refusal = "resource Количество is not a number or a string"
	defined := uuid.MustNew().String()
	for written, refused := range map[string]bool{
		"[{kind: date, date_parts: date}]": true,
		"[{kind: boolean}]":                true,
		"[{kind: uuid}]":                   true,
		"[{kind: value-storage}]":          true,
		"[{kind: binary-data}]":            true,
		"[{kind: number, precision: 10}, {kind: date, date_parts: date}]":      true,
		"[{kind: external-data-source-table, reference: " + objectTable + "}]": true,
		"[{kind: catalog, reference: " + catalogID + "}]":                      true,
		"[{kind: string, length: 10}]":                                         false,
		"[{kind: number, precision: 10}]":                                      false,
		"[{kind: number, precision: 10}, {kind: string, length: 10}]":          false,
	} {
		root := project(resource("[{kind: defined-type, reference: " + defined + "}]"))
		writeMetadata(t, root, DefinedTypeKind, defined, "format: 1\nid: "+defined+"\nname: Мера\ntitle: {ru: Мера}\ntypes: "+written+"\n")
		_, err := Load(root)
		if refused != (err != nil) || refused && !strings.Contains(err.Error(), refusal) {
			t.Errorf("a resource of a defined type of %s: %v", written, err)
		}
	}
	for written, refused := range map[string]bool{
		"[{kind: date, date_parts: date}]":                            true,
		"[{kind: number, precision: 10}, {kind: string, length: 10}]": false,
	} {
		_, err := Load(project(resource(written)))
		if refused != (err != nil) || refused && !strings.Contains(err.Error(), refusal) {
			t.Errorf("a resource of %s written out: %v", written, err)
		}
	}
}

// A field of a dimension table takes binary data with its qualifiers, as a
// field of a table does: the window of either offers it (opis of 25.10.2026,
// vneshnij-istochnik.md, snapshots series-2.535/03-05, series-2.543/01).
// A dimension of the cube does not: its window offers no simple type.
//
// Catches the field of a dimension table checked as a stored field, which
// refuses binary data, as it was until 2.549, or the schema of the
// application refusing it.
func TestAFieldOfADimensionTableTakesBinaryData(t *testing.T) {
	t.Parallel()
	binary := goodsDimTableBody + "  - {id: " + uuid.MustNew().String() + ", name: Файл, title: {ru: Файл}, " +
		"types: [{kind: binary-data, length: 16, fixed_length: true}], name_in_data_source: '[Goods].[File]'}\n"
	if _, err := DecodeExternalDimensionTable("table.yaml", strings.NewReader("format: 1\nid: "+goodsDimTable+"\nname: Товары\ntitle: {ru: Товары}\n"+binary),
		metadataConfiguration()); err != nil {
		t.Fatalf("a field of a dimension table of binary data: %v", err)
	}
	root := metadataProject(t)
	writeSalesCube(t, root, salesCubeBody, binary)
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a dimension table with a field of binary data was refused: %v", err)
	}
	if _, err := catalog.ApplicationSchema(); err != nil {
		t.Fatalf("the schema refused a dimension table with a field of binary data: %v", err)
	}
}

// A hierarchical dimension table keeps the level the designer saved with it -
// 1 in the files the designer writes (mdclasses, 8.3.25 and 8.3.27) - and is
// read at level 0, as the help says such a table is. The defects caught: the
// saved level refused, which turns away what the designer saves; the level
// lost on the way, which changes the file; and the application reading the
// saved number, not 0. A table without a hierarchy is read at its own level,
// so a rule that answers 0 for everything is caught as well. The value of an
// unfilled parent has no ceiling in the help, and a long one loads.
func TestExternalDimensionTableKeepsItsLevelAndIsReadAtTheHierarchys(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		body             string
		saved, effective int
	}{
		"иерархическая с уровнем 1":                {goodsDimTableBody + "level_number: 1\n", 1, 0},
		"иерархическая без уровня":                 {goodsDimTableBody, 0, 0},
		"неиерархическая уровня 2":                 {strings.Replace(strings.Replace(goodsDimTableBody, "hierarchical: true\n", "", 1), "unfilled_parent_value: \"\"\n", "", 1) + "level_number: 2\n", 2, 2},
		"длинное значение незаполненного родителя": {strings.Replace(goodsDimTableBody, `unfilled_parent_value: ""`, "unfilled_parent_value: "+strings.Repeat("x", 2000), 1), 0, 0},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeSalesCube(t, root, salesCubeBody, test.body)
			catalog, err := Load(root)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			cube, _ := catalog.ExternalCube("Склад", "Продажи")
			table := cube.DimensionTables[0]
			if table.LevelNumber != test.saved {
				t.Fatalf("the saved level came back as %d, want %d", table.LevelNumber, test.saved)
			}
			if got := table.EffectiveLevelNumber(); got != test.effective {
				t.Fatalf("the application reads level %d, want %d", got, test.effective)
			}
		})
	}
}

// TestAnEmptyNameInTheSourceIsRefused catches any of the five places taken with
// no name in the source - a table made of a table, its field, a cube, its
// resource, a dimension table and its field: the designer's check of the
// configuration refuses each (docs/block-00/kub.md, vneshnij-istochnik.md).
// A table made of an expression names nothing and is taken.
func TestAnEmptyNameInTheSourceIsRefused(t *testing.T) {
	t.Parallel()
	const says = " in the source: the designer refuses an empty name in the data source"
	for name, test := range map[string]struct{ body, says string }{
		"таблица без вида": {strings.Replace(goodsBody, "name_in_data_source: dbo.Goods\n", "", 1),
			"name_in_data_source must name the table" + says},
		"таблица вида «Таблица»": {strings.Replace(goodsBody, "name_in_data_source: dbo.Goods\n", "table_type: table\n", 1),
			"name_in_data_source must name the table" + says},
		"поле таблицы": {strings.Replace(goodsBody, "    name_in_data_source: code\n", "", 1),
			"fields[0].name_in_data_source must name the field" + says},
	} {
		if message := refusedExternalTable(t, test.body, name); !strings.Contains(message, test.says) {
			t.Errorf("%s: want %q, got %v", name, test.says, message)
		}
	}
	for name, test := range map[string]struct{ cube, table, says string }{
		"куб": {strings.Replace(noteCubeBody(), "name_in_data_source: '[Sales]'\n", "", 1), goodsDimTableBody,
			"name_in_data_source must name the cube" + says},
		"ресурс": {strings.Replace(noteCubeBody(), "    name_in_data_source: '[Measures].[Quantity]'\n", "", 1), goodsDimTableBody,
			"resources[0].name_in_data_source must name the measure" + says},
		"таблица измерения": {noteCubeBody(), strings.Replace(goodsDimTableBody, "name_in_data_source: '[Goods]'\n", "", 1),
			"name_in_data_source must name the dimension table" + says},
		"поле таблицы измерения": {noteCubeBody(), strings.Replace(goodsDimTableBody, "    name_in_data_source: '[Goods].[Code]'\n", "", 1),
			"fields[0].name_in_data_source must name the field" + says},
	} {
		root := metadataProject(t)
		writeSalesCube(t, root, test.cube, test.table)
		if _, err := Load(root); err == nil || !strings.Contains(err.Error(), test.says) {
			t.Errorf("%s: want %q, got %v", name, test.says, err)
		}
	}
	root := metadataProject(t)
	writeSalesCube(t, root, noteCubeBody(), goodsDimTableBody)
	writeExternalTable(t, root, "Склад", goodsTable, "Товары", goodsBody)
	writeExternalTable(t, root, "Склад", stockTable, "Остатки", stockBody)
	if _, err := Load(root); err != nil {
		t.Fatalf("a cube named in full and a table made of an expression refused: %v", err)
	}
}

// TestADimensionOfACubeIsOneReferenceToItsDimensionTable catches a dimension
// of a cube typed otherwise than by one reference to a dimension table of its
// cube: the editor gives a defined type or such a reference and nothing else,
// and «Проверка конфигурации» refuses no type and a defined type of a string
// (docs/block-00/kub.md). Written out or by a defined type holding it alone,
// the reference is taken.
func TestADimensionOfACubeIsOneReferenceToItsDimensionTable(t *testing.T) {
	t.Parallel()
	const reference = "    types: [{kind: external-data-source-dimension-table, reference: " + goodsDimTable + "}]\n    choice:"
	const refused = "dimension Товар must be typed by one reference to a dimension table, written out or by a defined type holding it alone"
	const otherCube, otherTable = "f8100000-0000-4000-8000-000000000021", "f8100000-0000-4000-8000-000000000022"
	gln, goods, both, others := uuid.MustNew().String(), uuid.MustNew().String(), uuid.MustNew().String(), uuid.MustNew().String()
	defined := func(t *testing.T, root string) {
		for id, types := range map[string]string{
			gln:    "[{kind: string, length: 13}]",
			goods:  "[{kind: external-data-source-dimension-table, reference: " + goodsDimTable + "}]",
			both:   "[{kind: external-data-source-dimension-table, reference: " + goodsDimTable + "}, {kind: string, length: 13}]",
			others: "[{kind: external-data-source-dimension-table, reference: " + otherTable + "}]",
		} {
			writeMetadata(t, root, DefinedTypeKind, id, "format: 1\nid: "+id+"\nname: Т"+strings.ReplaceAll(id, "-", "")[:8]+"\ntitle: {ru: Т}\ntypes: "+types+"\n")
		}
	}
	for name, test := range map[string]struct{ types, want string }{
		"без типа":               {"    types: []\n    choice:", "dimensions[0].types must contain at least one type"},
		"простой":                {"    types: [{kind: date}]\n    choice:", refused},
		"составной":              {"    types: [{kind: external-data-source-dimension-table, reference: " + goodsDimTable + "}, {kind: date}]\n    choice:", refused},
		"определяемый-строка":    {"    types: [{kind: defined-type, reference: " + gln + "}]\n    choice:", refused},
		"определяемый составной": {"    types: [{kind: defined-type, reference: " + both + "}]\n    choice:", refused},
		"определяемый — таблица другого куба": {"    types: [{kind: defined-type, reference: " + others + "}]\n    choice:", ""},
		"определяемый-ссылка":                 {"    types: [{kind: defined-type, reference: " + goods + "}]\n    choice:", ""},
		"ссылка":                              {reference, ""},
	} {
		root := metadataProject(t)
		defined(t, root)
		writeSalesCube(t, root, strings.Replace(noteCubeBody(), reference, test.types, 1), goodsDimTableBody)
		other := filepath.Join(root, "metadata", string(ExternalDataSourceKind), "Склад", ExternalDataSourceCubesDirectory, "Закупки")
		writeFile(t, filepath.Join(other, "object.yaml"), "format: 1\nid: "+otherCube+"\nname: Закупки\ntitle: {ru: Закупки}\nname_in_data_source: Purchases\n")
		writeFile(t, filepath.Join(other, ExternalDimensionTablesDirectory, "Поставщики", "object.yaml"),
			"format: 1\nid: "+otherTable+"\nname: Поставщики\ntitle: {ru: Поставщики}\nname_in_data_source: Suppliers\n")
		_, err := Load(root)
		if test.want == "" && err != nil {
			t.Errorf("%s refused: %v", name, err)
		}
		if test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
			t.Errorf("%s: want %q, got %v", name, test.want, err)
		}
	}
}
