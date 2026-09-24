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
		"реквизит занял стандартное имя": `attributes:
  - {id: ` + reportAttribute + `, name: Ссылка, title: {ru: Ссылка}, types: [{kind: string, length: 10}]}`,
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
