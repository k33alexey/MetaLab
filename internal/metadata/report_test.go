package metadata

import (
	"strings"
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
templates:
  - {id: `+reportSchema+`, name: ОсновнаяСхема, title: {ru: Основная схема}, kind: composition-schema}
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
	writeTemplateContent(t, root, ReportKind, "ОстаткиТоваров", "ОсновнаяСхема", "content.yaml", "format: 1\n")
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

// The main schema of a report is one of its own templates holding a
// composition schema: the designer offers the report's schemas to choose from,
// and the configurations being moved never name anything else. Each case is a
// report that would be built by something it cannot be built by - a schema
// that is not there, a printed form, another report's schema - and each must
// be refused; the last case is the report that holds together, so a check
// that refuses everything turns red too.
func TestMainSchemaIsTheReportsOwnCompositionSchema(t *testing.T) {
	t.Parallel()
	const (
		printForm    = "4e900000-0000-4000-8000-000000000014"
		otherReport  = "4e900000-0000-4000-8000-000000000015"
		otherSchema  = "4e900000-0000-4000-8000-000000000016"
		ownTemplates = `templates:
  - {id: ` + reportSchema + `, name: Схема, title: {ru: Схема}, kind: composition-schema}
  - {id: ` + printForm + `, name: Печать, title: {ru: Печать}, kind: spreadsheet}`
	)
	// A schema that names no template of the report is a reference to what
	// is gone: carried and noted (A3). A template of the report holding
	// something else is refused - that is no reference to nothing.
	for name, test := range map[string]struct {
		schema string
		valid  bool
		noted  bool
	}{
		"схемы нет среди макетов":      {"4e900000-0000-4000-8000-0000000000ff", true, true},
		"макет не схема компоновки":    {printForm, false, false},
		"схема другого отчёта":         {otherSchema, true, true},
		"собственная схема компоновки": {reportSchema, true, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, ReportKind, reportID, `format: 1
id: `+reportID+`
name: Продажи
title: {ru: Продажи}
main_schema: `+test.schema+`
`+ownTemplates+`
`)
			writeTemplateContent(t, root, ReportKind, "Продажи", "Схема", "content.yaml", "format: 1\n")
			writeTemplateContent(t, root, ReportKind, "Продажи", "Печать", "content.yaml", "format: 1\n")
			writeMetadata(t, root, ReportKind, otherReport, `format: 1
id: `+otherReport+`
name: Остатки
title: {ru: Остатки}
templates:
  - {id: `+otherSchema+`, name: Схема, title: {ru: Схема}, kind: composition-schema}
`)
			writeTemplateContent(t, root, ReportKind, "Остатки", "Схема", "content.yaml", "format: 1\n")
			catalog, err := Load(root)
			if test.valid && err != nil {
				t.Fatalf("the report was rejected: %v", err)
			}
			if !test.valid {
				if err == nil || !strings.Contains(err.Error(), "not a composition schema") {
					t.Fatalf("refused for another reason or not at all: %v", err)
				}
				return
			}
			if noted := hasUnresolvedNote(catalog, "report Продажи main schema", test.schema); noted != test.noted {
				t.Fatalf("main schema noted = %v, want %v: %+v", noted, test.noted, catalog.Notes())
			}
		})
	}
}
