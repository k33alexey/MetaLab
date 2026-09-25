package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	templateCatalog = "ce000000-0000-4000-8000-000000000001"
	templateID      = "ce000000-0000-4000-8000-000000000100"
	templateSecond  = "ce000000-0000-4000-8000-000000000101"
)

// A template is a subordinate entity of every kind of object at once, the same
// way a command is. The kinds are checked together with the commands because
// they hang off the same objects and are lost the same way.
func TestEveryObjectKindCarriesItsOwnTemplates(t *testing.T) {
	t.Parallel()
	root := subordinateProject(t)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string][]ObjectTemplate{}
	definition, _ := catalog.CatalogDefinition("Контрагенты")
	kinds["справочник"] = definition.Templates
	document, _ := catalog.DocumentDefinition("РасходТовара")
	kinds["документ"] = document.Templates
	journal, _ := catalog.DocumentJournal("ОбщийЖурнал")
	kinds["журнал документов"] = journal.Templates
	information, _ := catalog.InformationRegisterDefinition("Курсы")
	kinds["регистр сведений"] = information.Templates
	accumulation, _ := catalog.AccumulationRegisterDefinition("ОстаткиТоваров")
	kinds["регистр накопления"] = accumulation.Templates
	characteristics, _ := catalog.ChartOfCharacteristicTypes("ВидыСвойств")
	kinds["план видов характеристик"] = characteristics.Templates
	accounts, _ := catalog.ChartOfAccounts("Основной")
	kinds["план счетов"] = accounts.Templates
	calculationTypes, _ := catalog.ChartOfCalculationTypes("Начисления")
	kinds["план видов расчёта"] = calculationTypes.Templates
	process, _ := catalog.BusinessProcess("Задание")
	kinds["бизнес-процесс"] = process.Templates
	task, _ := catalog.Task("Исполнение")
	kinds["задача"] = task.Templates
	plan, _ := catalog.ExchangePlan("Филиалы")
	kinds["план обмена"] = plan.Templates
	entries, _ := catalog.AccountingRegister("Хозрасчетный")
	kinds["регистр бухгалтерии"] = entries.Templates
	calculations, _ := catalog.CalculationRegister("ОсновныеНачисления")
	kinds["регистр расчёта"] = calculations.Templates
	report, _ := catalog.Report("ОстаткиНаСкладе")
	kinds["отчёт"] = report.Templates
	processor, _ := catalog.DataProcessor("ЗагрузкаЦен")
	kinds["обработка"] = processor.Templates

	if len(kinds) != len(objectFolderKindsForTest()) {
		t.Fatalf("the test covers %d kinds of object, the project has %d that keep their own templates",
			len(kinds), len(objectFolderKindsForTest()))
	}
	for kind, templates := range kinds {
		if len(templates) != 1 {
			t.Fatalf("%s lost its templates: %+v", kind, templates)
		}
		if templates[0].Name != "ПечатнаяФорма" || templates[0].Kind != SpreadsheetTemplate {
			t.Fatalf("%s carried its template but not what it is: %+v", kind, templates[0])
		}
	}
}

// All ten kinds of template are carried, not only the ones the reference
// configuration happens to use. A kind the model does not know is a template
// lost without a word, and the kind decides where its content is looked for.
func TestTemplateOfEveryKindIsStoredWhereItsKindSays(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	declared, ids := "", map[TemplateKind]string{}
	for index, kind := range []TemplateKind{SpreadsheetTemplate, TextTemplate, BinaryTemplate, HTMLTemplate,
		CompositionSchema, CompositionAppearance, GeographicalSchema, GraphicalSchema, ActiveDocument, AddInTemplate} {
		id := templateIdentifier(index)
		ids[kind] = id
		declared += "  - {id: " + id + ", name: Макет" + string('A'+rune(index)) +
			", title: {ru: Макет}, kind: " + string(kind) + "}\n"
	}
	writeMetadata(t, root, CatalogKind, templateCatalog, `format: 1
id: `+templateCatalog+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true}
description_length: 150
templates:
`+declared)
	for kind, id := range ids {
		switch kind {
		case HTMLTemplate:
			// One document per language, because it is read by a person.
			writeTemplateContent(t, root, CatalogKind, "Контрагенты", id, "ru.html", "<p>Привет</p>")
			writeTemplateContent(t, root, CatalogKind, "Контрагенты", id, "uk.html", "<p>Привіт</p>")
		default:
			writeTemplateContent(t, root, CatalogKind, "Контрагенты", id, kind.contentFile(), "содержимое")
		}
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	definition, ok := catalog.CatalogDefinition("Контрагенты")
	if !ok {
		t.Fatal("the catalog did not load")
	}
	if len(definition.Templates) != len(ids) {
		t.Fatalf("%d templates of %d survived: %+v", len(definition.Templates), len(ids), definition.Templates)
	}
	seen := map[TemplateKind]bool{}
	for _, template := range definition.Templates {
		seen[template.Kind] = true
	}
	for kind := range ids {
		if !seen[kind] {
			t.Fatalf("a template of kind %s was lost", kind)
		}
	}

	// The catalog hands out copies here too.
	definition.Templates[0].Name = "Подменено"
	again, _ := catalog.CatalogDefinition("Контрагенты")
	if again.Templates[0].Name == "Подменено" {
		t.Fatal("the templates of an object were handed out by reference")
	}
}

// A template without content is normal: no editor writes it yet, and the
// reference configuration carries templates that have none. What is not normal
// is content nobody declared, or content of the wrong shape for the kind -
// both mean the platform would look for the template elsewhere than it lies.
func TestTemplateContentIsCheckedAgainstItsKind(t *testing.T) {
	t.Parallel()
	const body = `format: 1
id: ` + templateCatalog + `
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true}
description_length: 150
templates:
  - {id: ` + templateID + `, name: ПечатнаяФорма, title: {ru: Печатная форма}, kind: spreadsheet}
  - {id: ` + templateSecond + `, name: Письмо, title: {ru: Письмо}, kind: html}
`
	// A declared template with no content at all loads.
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, templateCatalog, body)
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}

	for name, broken := range map[string]struct {
		template, file, want string
	}{
		"табличный документ с текстом": {templateID, "content.txt", "cannot hold"},
		"HTML без языка":               {templateSecond, "content.yaml", "cannot hold"},
		"язык не язык":                 {templateSecond, "ЯЗЫК.html", "cannot hold"},
		"содержимое без макета": {"ce000000-0000-4000-8000-0000000001ff", "content.yaml",
			"which it does not declare"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, CatalogKind, templateCatalog, body)
			writeTemplateContent(t, root, CatalogKind, "Контрагенты", broken.template, broken.file, "x")
			_, err := Load(root)
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}

	// A file lying directly among the templates belongs to no template.
	root = metadataProject(t)
	writeMetadata(t, root, CatalogKind, templateCatalog, body)
	directory := filepath.Join(root, "metadata", string(CatalogKind), templateCatalog, "templates")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "content.yaml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("a file lying beside the templates was accepted as one")
	}
}

// What a template is checked for is what makes it a template: a name of its
// own and a kind the platform knows.
func TestBrokenTemplatesAreRefused(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ body, want string }{
		"два имени в одном объекте": {`templates:
  - {id: ` + templateID + `, name: Макет, title: {ru: Макет}, kind: spreadsheet}
  - {id: ` + templateSecond + `, name: макет, title: {ru: Макет ещё}, kind: text}`,
			"templates[1].name must be unique"},
		"один идентификатор на два макета": {`templates:
  - {id: ` + templateID + `, name: Первый, title: {ru: Первый}, kind: spreadsheet}
  - {id: ` + templateID + `, name: Второй, title: {ru: Второй}, kind: text}`,
			"templates[1].id must be unique"},
		"имя не идентификатор": {`templates:
  - {id: ` + templateID + `, name: "Печатная форма", title: {ru: Макет}, kind: spreadsheet}`,
			"templates[0].name must be a valid identifier"},
		"вид макета не назван": {`templates:
  - {id: ` + templateID + `, name: Макет, title: {ru: Макет}}`,
			"templates[0].kind is not a kind of template"},
		"вида макета не существует": {`templates:
  - {id: ` + templateID + `, name: Макет, title: {ru: Макет}, kind: слайды}`,
			"templates[0].kind is not a kind of template"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeCatalog("object.yaml", strings.NewReader(`format: 1
id: `+templateCatalog+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true}
description_length: 150
`+broken.body+`
`), metadataManifest())
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

func templateIdentifier(index int) string {
	const digits = "0123456789abcdef"
	return "ce000000-0000-4000-8000-0000000002" + string([]byte{digits[index/16], digits[index%16]})
}
