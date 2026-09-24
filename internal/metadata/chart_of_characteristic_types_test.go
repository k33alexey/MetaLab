package metadata

import (
	"strings"
	"testing"
)

// A chart of characteristic types repeats a catalog and adds what makes it one:
// the types a characteristic's value may take, and the catalog holding values
// that fit no type. Both have to survive the round trip through the project, or
// a configuration that relies on user-invented properties arrives hollow.
func TestLoadChartOfCharacteristicTypes(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: ЗначенияСвойств
title: {ru: Значения свойств}
code: {type: string, length: 9}
description_length: 150
`)
	writeMetadata(t, root, ChartOfCharacteristicTypesKind, characteristicsID, `format: 1
id: `+characteristicsID+`
name: ДополнительныеРеквизиты
title: {ru: Дополнительные реквизиты}
code: {type: string, length: 9, auto: true, unique: true}
description_length: 150
value_type:
  - {kind: string, length: 100}
  - {kind: number, precision: 15, scale: 2, non_negative: true}
  - {kind: catalog, reference: `+catalogID+`}
additional_values: `+catalogID+`
attributes:
  - id: `+characteristicAttrID+`
    name: Обязательный
    title: {ru: Обязательный}
    types: [{kind: boolean}]
table_parts:
  - id: `+characteristicPartID+`
    name: Подсказки
    title: {ru: Подсказки}
    attributes:
      - id: 70000000-0000-4000-8000-000000000004
        name: Текст
        title: {ru: Текст}
        types: [{kind: string, length: 200}]
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	chart, ok := catalog.ChartOfCharacteristicTypes("ДополнительныеРеквизиты")
	if !ok {
		t.Fatal("the chart did not load")
	}
	if len(chart.ValueType) != 3 {
		t.Fatalf("value type = %+v, want three allowed types", chart.ValueType)
	}
	if chart.AdditionalValues == nil || chart.AdditionalValues.String() != catalogID {
		t.Fatalf("additional values = %v", chart.AdditionalValues)
	}
	if len(chart.Attributes) != 1 || len(chart.TableParts) != 1 {
		t.Fatalf("attributes = %d, table parts = %d", len(chart.Attributes), len(chart.TableParts))
	}
	if _, ok := catalog.ChartOfCharacteristicTypesByID(chart.ID); !ok {
		t.Fatal("the chart is not reachable by its identifier")
	}
}

// An attribute may be a reference to one kind of characteristic, exactly as it
// may be a reference to a catalog element - and the stored column is then a
// reference, with the foreign key that makes it one.
func TestCharacteristicReferenceIsStoredAsAReference(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, ChartOfCharacteristicTypesKind, characteristicsID, `format: 1
id: `+characteristicsID+`
name: ВидыСубконто
title: {ru: Виды субконто}
code: {type: string, length: 9}
description_length: 150
value_type: [{kind: string, length: 50}]
`)
	writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: Настройки
title: {ru: Настройки}
code: {type: string, length: 9}
description_length: 150
attributes:
  - id: `+attributeID+`
    name: ВидСубконто
    title: {ru: Вид субконто}
    types: [{kind: chart-of-characteristic-types, reference: `+characteristicsID+`}]
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	chartTable, err := PhysicalCatalogTable(catalog.ChartsOfCharacteristicTypes[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	var chartFound, referenceFound bool
	for _, table := range schema.Tables {
		if table.Name == chartTable {
			chartFound = true
			var valueType bool
			for _, column := range table.Columns {
				if column.Name == "value_type" {
					valueType = true
				}
			}
			if !valueType {
				t.Fatal("an element of a chart must carry its own value type")
			}
		}
		for _, constraint := range table.Constraints {
			if constraint.Type == "foreign_key" && strings.Contains(constraint.Definition, chartTable) {
				referenceFound = true
			}
		}
	}
	if !chartFound {
		t.Fatal("the chart has no table of its own")
	}
	if !referenceFound {
		t.Fatal("an attribute referencing a characteristic kind got no foreign key")
	}
}

// The value type is the whole point of the object, and the catalog of
// additional values must exist: a chart that allows nothing, or points at a
// catalog that is not there, loses values rather than storing them.
func TestChartOfCharacteristicTypesRefusesAnEmptyOrDanglingDescription(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, ChartOfCharacteristicTypesKind, characteristicsID, `format: 1
id: `+characteristicsID+`
name: БезТипа
title: {ru: Без типа}
code: {type: string, length: 9}
description_length: 150
value_type: []
`)
	if _, err := Load(root); err == nil {
		t.Fatal("a chart allowing no type at all was accepted")
	}

	other := metadataProject(t)
	writeMetadata(t, other, ChartOfCharacteristicTypesKind, characteristicsID, `format: 1
id: `+characteristicsID+`
name: СВисячейСсылкой
title: {ru: С висячей ссылкой}
code: {type: string, length: 9}
description_length: 150
value_type: [{kind: string, length: 50}]
additional_values: `+catalogID+`
`)
	if _, err := Load(other); err == nil {
		t.Fatal("a chart pointing at a missing catalog of additional values was accepted")
	}
}

// The value type of an element is a standard attribute, so an attribute of that
// name would collide with what the platform already put there.
func TestChartOfCharacteristicTypesKeepsTheValueTypeName(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, ChartOfCharacteristicTypesKind, characteristicsID, `format: 1
id: `+characteristicsID+`
name: Свойства
title: {ru: Свойства}
code: {type: string, length: 9}
description_length: 150
value_type: [{kind: string, length: 50}]
attributes:
  - id: `+characteristicAttrID+`
    name: ТипЗначения
    title: {ru: Тип значения}
    types: [{kind: string, length: 10}]
`)
	if _, err := Load(root); err == nil {
		t.Fatal("an attribute named after the standard value type was accepted")
	}
}
