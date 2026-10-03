package metadata

import (
	"path/filepath"
	"strings"
	"testing"
)

const (
	salesCube         = "f8100000-0000-4000-8000-000000000001"
	goodsDimension    = "f8100000-0000-4000-8000-000000000002"
	quantityResource  = "f8100000-0000-4000-8000-000000000003"
	goodsDimTable     = "f8100000-0000-4000-8000-000000000011"
	goodsDimTableCode = "f8100000-0000-4000-8000-000000000012"
	goodsDimTableName = "f8100000-0000-4000-8000-000000000013"
	periodDimension   = "f8100000-0000-4000-8000-000000000004"
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
  - id: ` + periodDimension + `
    name: Период
    title: {ru: Период}
    types: [{kind: date}]
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
		*table.UnfilledParentValue != "" || table.PresentationField != "Наименование" || len(table.Fields) != 2 || table.Forms.Choice != "ФормаВыбора" {
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
// dimension and a resource of one name - one field of the cube read twice - a
// cube with no name in the source, an unfilled
// parent on a table with no hierarchy, and a presentation field that is not a
// field.
func TestExternalCubeAndDimensionTableAreCheckedOnTheirOwn(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ cube, dimension, says string }{
		"a dimension and a resource of one name": {strings.Replace(salesCubeBody, "    name: Количество", "    name: Товар", 1), goodsDimTableBody, "resources[0].name is used twice"},
		"a cube with no name in the source":      {strings.Replace(salesCubeBody, "name_in_data_source: '[Sales]'\n", "", 1), goodsDimTableBody, "must name the cube in the data source"},
		"an unfilled parent without a hierarchy": {salesCubeBody, strings.Replace(goodsDimTableBody, "hierarchical: true\n", "", 1), "unfilled_parent_value belongs to a hierarchical dimension table"},
		"a presentation field that is absent":    {salesCubeBody, strings.Replace(goodsDimTableBody, "presentation_field: Наименование", "presentation_field: Артикул", 1), `presentation_field names "Артикул"`},
		"an attribute's property on a dimension": {strings.Replace(salesCubeBody, "    types: [{kind: date}]\n", "    types: [{kind: date}]\n    indexing: index\n", 1), goodsDimTableBody, "dimensions[1].indexing belongs to an attribute"},
		// A dimension is found through its dimension table and has no column
		// of its own: the strict reading names the property.
		"a column on a dimension":   {strings.Replace(salesCubeBody, "    types: [{kind: date}]\n", "    types: [{kind: date}]\n    name_in_data_source: '[Date]'\n", 1), goodsDimTableBody, "name_in_data_source not found"},
		"a filling on a resource":   {strings.Replace(salesCubeBody, "    name_in_data_source: '[Measures].[Quantity]'\n", "    name_in_data_source: '[Measures].[Quantity]'\n    filling: {value: {kind: number, data: \"0\"}}\n", 1), goodsDimTableBody, "resources[0].filling belongs to a field somebody enters"},
		"bounds on a resource":      {strings.Replace(salesCubeBody, "    name_in_data_source: '[Measures].[Quantity]'\n", "    name_in_data_source: '[Measures].[Quantity]'\n    presentation: {max_value: \"9\"}\n", 1), goodsDimTableBody, "resources[0].presentation.max_value belongs to"},
		"a resource with no column": {strings.Replace(salesCubeBody, "    name_in_data_source: '[Measures].[Quantity]'\n", "", 1), goodsDimTableBody, "resources[0].name_in_data_source must name the measure"},
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
	names := moduleNameDescriptors(catalog)
	for path, want := range map[string]string{
		"metadata/external-data-sources/Склад/cubes/Продажи/МодульНабораЗаписей.bsl":                                   "МодульНабораЗаписейКубаВнешнегоИсточника.Склад.Продажи",
		"metadata/external-data-sources/Склад/cubes/Продажи/dimension-tables/Товары/МодульОбъекта.bsl":                 "МодульОбъектаТаблицыИзмеренияВнешнегоИсточника.Склад.Продажи.Товары",
		"metadata/external-data-sources/Склад/cubes/Продажи/dimension-tables/Товары/forms/ФормаВыбора/МодульФормы.bsl": "МодульФормыТаблицыИзмеренияВнешнегоИсточника.Склад.Продажи.Товары.ФормаВыбора",
	} {
		if got := names[path].name; got != want {
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

// A cube is independent of every other cube of its source, as the platform
// shows: its configurator offers a dimension of one cube the dimension tables
// of that cube alone. Catches a dimension of one cube referring to a dimension
// table of another, which the rule of the source alone would let through.
func TestExternalCubeDimensionRefersOnlyToItsOwnCube(t *testing.T) {
	t.Parallel()
	const otherCube, otherTable = "f8100000-0000-4000-8000-000000000021", "f8100000-0000-4000-8000-000000000022"
	root := metadataProject(t)
	writeSalesCube(t, root, strings.Replace(salesCubeBody, "reference: "+goodsDimTable, "reference: "+otherTable, 1), goodsDimTableBody)
	other := filepath.Join(root, "metadata", string(ExternalDataSourceKind), "Склад", ExternalDataSourceCubesDirectory, "Закупки")
	writeFile(t, filepath.Join(other, "object.yaml"), "format: 1\nid: "+otherCube+"\nname: Закупки\ntitle: {ru: Закупки}\nname_in_data_source: Purchases\n")
	writeFile(t, filepath.Join(other, ExternalDimensionTablesDirectory, "Поставщики", "object.yaml"),
		"format: 1\nid: "+otherTable+"\nname: Поставщики\ntitle: {ru: Поставщики}\nname_in_data_source: Suppliers\n")
	if message := loadRefused(t, root, "a dimension of another cube's table"); !strings.Contains(message, "refers to a dimension table of another cube") {
		t.Fatalf("the error does not say what is wrong: %v", message)
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
