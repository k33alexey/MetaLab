package metadata

import (
	"testing"
)

const (
	reportID        = "4e900000-0000-4000-8000-000000000001"
	dataProcessorID = "4e900000-0000-4000-8000-000000000002"
	reportGoods     = "4e900000-0000-4000-8000-000000000003"
	reportAttribute = "4e900000-0000-4000-8000-000000000010"
	reportPart      = "4e900000-0000-4000-8000-000000000011"
	reportPartField = "4e900000-0000-4000-8000-000000000012"
	reportSchema    = "4e900000-0000-4000-8000-000000000013"
)

// A report and a data processor keep no data of their own: their attributes
// and table parts exist only while the object runs. So they are described and
// checked like any other object, and they get no table at all.
func TestReportsAndDataProcessorsKeepNoData(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	catalogNamed(t, root, reportGoods, "Номенклатура")
	writeMetadata(t, root, ReportKind, reportID, `format: 1
id: `+reportID+`
name: ОстаткиТоваров
title: {ru: Остатки товаров}
main_schema: `+reportSchema+`
attributes:
  - id: `+reportAttribute+`
    name: Товар
    title: {ru: Товар}
    types: [{kind: catalog, reference: `+reportGoods+`}]
table_parts:
  - id: `+reportPart+`
    name: Отборы
    title: {ru: Отборы}
    attributes:
      - {id: `+reportPartField+`, name: Значение, title: {ru: Значение}, types: [{kind: string, length: 100}]}
`)
	writeMetadata(t, root, DataProcessorKind, dataProcessorID, `format: 1
id: `+dataProcessorID+`
name: ЗагрузкаЦен
title: {ru: Загрузка цен}
attributes:
  - {id: 4e900000-0000-4000-8000-000000000020, name: Файл, title: {ru: Файл}, types: [{kind: string, length: 260}]}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	report, ok := catalog.Report("ОстаткиТоваров")
	if !ok {
		t.Fatal("the report did not load")
	}
	if report.MainSchema == nil {
		t.Fatal("the report lost the schema it is built by")
	}
	if len(report.Attributes) != 1 || len(report.TableParts) != 1 || len(report.TableParts[0].Attributes) != 1 {
		t.Fatalf("the report lost what it is made of: %+v", report)
	}
	if _, ok := catalog.DataProcessor("ЗагрузкаЦен"); !ok {
		t.Fatal("the data processor did not load")
	}

	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{reportID, dataProcessorID, reportPart} {
		table, err := PhysicalCatalogTable(mustUUID(t, id))
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range schema.Tables {
			if item.Name == table {
				t.Fatalf("an object that keeps no data was given a table: %s", table)
			}
		}
	}
}

// What is checked in a running object is what is checked everywhere: names do
// not collide, types resolve, identifiers are unique across the project.
func TestReportsAreCheckedLikeAnyOtherObject(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"табличная часть названа как реквизит": `attributes:
  - {id: ` + reportAttribute + `, name: Отборы, title: {ru: Отборы}, types: [{kind: string, length: 10}]}
table_parts:
  - {id: ` + reportPart + `, name: Отборы, title: {ru: Отборы}, attributes: [{id: ` + reportPartField + `, name: Значение, title: {ru: Значение}, types: [{kind: string, length: 10}]}]}`,
		"тип указывает в никуда": `attributes:
  - {id: ` + reportAttribute + `, name: Товар, title: {ru: Товар}, types: [{kind: catalog, reference: 4e900000-0000-4000-8000-0000000000ff}]}`,
		"два реквизита с одним идентификатором": `attributes:
  - {id: ` + reportAttribute + `, name: Первый, title: {ru: Первый}, types: [{kind: string, length: 10}]}
  - {id: ` + reportAttribute + `, name: Второй, title: {ru: Второй}, types: [{kind: string, length: 10}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			catalogNamed(t, root, reportGoods, "Номенклатура")
			writeMetadata(t, root, ReportKind, reportID, `format: 1
id: `+reportID+`
name: ОстаткиТоваров
title: {ru: Остатки товаров}
`+body+`
`)
			if _, err := Load(root); err == nil {
				t.Fatal("a report that does not hold together was accepted")
			}
		})
	}
}

// A running object has no standard attributes, so none of its names is taken,
// and an attribute of the object itself may be left without a type: the
// prototype takes it as arbitrary. Both are what the configurations being
// moved do - «Ссылка» and «НомерСтроки» on a data processor seven times, an
// attribute with no type 51 times. Each case loads as a report and as a data
// processor: the two are checked by one code, and a defect in either kind's
// call would stay unseen if only one were tried.
func TestRunningObjectsTakeAnyNameAndNoType(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"реквизит назван Ссылка": `attributes:
  - {id: ` + reportAttribute + `, name: Ссылка, title: {ru: Ссылка}, types: [{kind: string, length: 10}]}`,
		"реквизит назван НомерСтроки": `attributes:
  - {id: ` + reportAttribute + `, name: НомерСтроки, title: {ru: Номер}, types: [{kind: string, length: 5}]}`,
		"табличная часть названа Ссылка": `table_parts:
  - {id: ` + reportPart + `, name: Ссылка, title: {ru: Ссылка}, attributes: [{id: ` + reportPartField + `, name: Значение, title: {ru: Значение}, types: [{kind: string, length: 10}]}]}`,
		"реквизит без типа": `attributes:
  - {id: ` + reportAttribute + `, name: Контекст, title: {ru: Контекст}}`,
	} {
		for _, kind := range []Kind{ReportKind, DataProcessorKind} {
			t.Run(name+"/"+string(kind), func(t *testing.T) {
				t.Parallel()
				root := metadataProject(t)
				writeMetadata(t, root, kind, reportID, `format: 1
id: `+reportID+`
name: Помощник
title: {ru: Помощник}
`+body+`
`)
				catalog, err := Load(root)
				if err != nil {
					t.Fatalf("a running object the prototype saves was rejected: %v", err)
				}
				var attributes []Attribute
				if kind == ReportKind {
					item, _ := catalog.Report("Помощник")
					attributes = item.Attributes
				} else {
					item, _ := catalog.DataProcessor("Помощник")
					attributes = item.Attributes
				}
				if name == "реквизит без типа" && (len(attributes) != 1 || len(attributes[0].Types) != 0) {
					t.Fatalf("an attribute with no type did not stay without one: %+v", attributes)
				}
			})
		}
	}
}

// What a running object is let off is the object's own attribute, and only
// that. A table part attribute is a field like any other and must say what it
// holds; a reference in a type must lead somewhere wherever it stands. Every
// case is tried on both kinds, so a check that one of them skips turns red.
func TestRunningObjectsStillHoldTogether(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"реквизит табличной части без типа": `table_parts:
  - {id: ` + reportPart + `, name: Отборы, title: {ru: Отборы}, attributes: [{id: ` + reportPartField + `, name: Значение, title: {ru: Значение}}]}`,
		"тип реквизита указывает в никуда": `attributes:
  - {id: ` + reportAttribute + `, name: Товар, title: {ru: Товар}, types: [{kind: catalog, reference: 4e900000-0000-4000-8000-0000000000ff}]}`,
		"тип реквизита табличной части указывает в никуда": `table_parts:
  - {id: ` + reportPart + `, name: Отборы, title: {ru: Отборы}, attributes: [{id: ` + reportPartField + `, name: Товар, title: {ru: Товар}, types: [{kind: catalog, reference: 4e900000-0000-4000-8000-0000000000ff}]}]}`,
	} {
		for _, kind := range []Kind{ReportKind, DataProcessorKind} {
			t.Run(name+"/"+string(kind), func(t *testing.T) {
				t.Parallel()
				root := metadataProject(t)
				catalogNamed(t, root, reportGoods, "Номенклатура")
				writeMetadata(t, root, kind, reportID, `format: 1
id: `+reportID+`
name: Помощник
title: {ru: Помощник}
`+body+`
`)
				if _, err := Load(root); err == nil {
					t.Fatal("a running object that does not hold together was accepted")
				}
			})
		}
	}
}
