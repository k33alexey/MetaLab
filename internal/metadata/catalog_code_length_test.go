package metadata

import (
	"context"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/querylang"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A code of length 0 switches the code off, as the prototype does - checked by
// the owner on 01.10.2026: 720 catalogs of the configurations being moved have
// no code, and every one of them was refused on import. Off means off
// everywhere: no field on the object, no column, nothing to query, search,
// show on a form or grant a right on; and a configuration whose input by
// string still names the code is refused, as the designer refuses to save it.

func codelessCatalog(extra string) string {
	return `format: 1
id: ` + catalogID + `
name: Товары
title: {ru: Товары}
code: {type: string, length: 0}
description_length: 100
` + extra
}

func TestACodeOfLengthZeroIsSwitchedOff(t *testing.T) {
	t.Parallel()
	decode := func(body string) error {
		_, err := DecodeCatalog("object.yaml", strings.NewReader(body), metadataConfiguration())
		return err
	}
	if err := decode(codelessCatalog("")); err != nil {
		t.Fatalf("a catalog without a code was refused: %v", err)
	}
	// The prototype writes autonumbering and the check for repeats on every
	// catalog, a codeless one too; they are carried and do nothing.
	if err := decode(strings.Replace(codelessCatalog(""), "length: 0}", "length: 0, auto: true, unique: true}", 1)); err != nil {
		t.Fatalf("a codeless catalog with the settings the prototype writes was refused: %v", err)
	}
	if err := decode(codelessCatalog("input_by_string:\n  - {standard: Наименование}\n")); err != nil {
		t.Fatalf("input by the description was refused: %v", err)
	}
	for name, testCase := range map[string]struct{ body, message string }{
		"ввод по строке по коду": {codelessCatalog("input_by_string:\n  - {standard: Код}\n"), "switched off by a length of 0"},
		"код предопределённого":  {codelessCatalog("predefined:\n  - {id: 6f0a0000-0000-4000-8000-000000000001, name: Основной, code: \"001\"}\n"), "code is switched off"},
	} {
		if err := decode(testCase.body); err == nil || !strings.Contains(err.Error(), testCase.message) {
			t.Fatalf("%s: error = %v, expected it to say %q", name, err, testCase.message)
		}
	}
	// A predefined item of a codeless catalog needs no code.
	if err := decode(codelessCatalog("predefined:\n  - {id: 6f0a0000-0000-4000-8000-000000000001, name: Основной}\n")); err != nil {
		t.Fatalf("a predefined item without a code was refused: %v", err)
	}

	// The chart of characteristic types has the same, and a kind the owner did
	// not check keeps its code.
	if _, err := DecodeChartOfCharacteristicTypes("object.yaml", strings.NewReader(`format: 1
id: `+characteristicsID+`
name: Виды
title: {ru: Виды}
code: {type: string, length: 0}
description_length: 100
value_type: [{kind: string, length: 100}]
`), metadataConfiguration()); err != nil {
		t.Fatalf("a chart of characteristic types without a code was refused: %v", err)
	}
	if _, err := DecodeExchangePlan("object.yaml", strings.NewReader(`format: 1
id: `+exchangePlanID+`
name: Филиалы
title: {ru: Филиалы}
code: {type: string, length: 0}
description_length: 100
`), metadataConfiguration()); err == nil || !strings.Contains(err.Error(), "code.length must be 1..50") {
		t.Fatalf("an exchange plan without a code: %v", err)
	}
}

func TestACodelessCatalogHasNoCodeAnywhere(t *testing.T) {
	t.Parallel()
	goodsID := uuid.MustNew()
	definition := CatalogDefinition{ID: goodsID, Name: "Товары", Code: CatalogCode{Type: StringType}, DescriptionLength: 100}
	catalog := &Catalog{
		Catalogs:      []CatalogDefinition{definition},
		catalogByName: map[string]int{"товары": 0}, catalogByID: map[uuid.UUID]int{goodsID: 0},
	}

	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	table, _ := PhysicalCatalogTable(goodsID)
	for _, candidate := range schema.Tables {
		if candidate.Name != table {
			continue
		}
		for _, column := range candidate.Columns {
			if column.Name == "code" {
				t.Fatal("the table got a code column")
			}
		}
		for _, index := range candidate.Indexes {
			if strings.Contains(strings.Join(index.Keys, ","), "code") {
				t.Fatalf("the table got an index by the code: %+v", index)
			}
		}
		for _, constraint := range candidate.Constraints {
			if strings.Contains(constraint.Definition, "code") {
				t.Fatalf("the table got a constraint on the code: %+v", constraint)
			}
		}
	}

	// A query naming the code is refused, as the prototype's «Поле не найдено».
	runtime := &Runtime{catalog: catalog}
	parsed, err := querylang.Parse(`ВЫБРАТЬ Т.Код ИЗ Справочник.Товары КАК Т`)
	if err != nil {
		t.Fatal(err)
	}
	compiler, err := runtime.newQueryCompiler(context.Background(), parsed, map[string]bytecode.Value{}, nil)
	if err == nil {
		_, err = compiler.compile(parsed)
	}
	if err == nil {
		t.Fatal("a query read the code of a catalog that has none")
	}

	form, err := catalog.CatalogForm("Товары", ObjectForm, "ru")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range form.Fields {
		if field.Name == "Code" {
			t.Fatal("the generated form shows a code")
		}
	}
	if len(form.List.SearchFields) != 1 || form.List.SearchFields[0] != "Description" {
		t.Fatalf("the list is searched by %v", form.List.SearchFields)
	}
	if _, ok := catalogListField(definition, "Code"); ok {
		t.Fatal("the list sorts or filters by a code that is not there")
	}

	// The object has no Код at all - the prototype's debugger shows none - and
	// the description is still there.
	object := &catalogObject{definition: definition, runtime: runtime,
		record: &CatalogRecord{Reference: CatalogReference{CatalogID: goodsID, ObjectID: uuid.MustNew()}, Description: "Первый"}}
	if _, err := runtime.GetObjectProperty(context.Background(), object, "Код"); err == nil {
		t.Fatal("the object of a codeless catalog has a code")
	}
	if err := runtime.SetObjectProperty(context.Background(), object, "Код", bytecode.String("1")); err == nil {
		t.Fatal("a code was set on the object of a codeless catalog")
	}
	if value, err := runtime.GetObjectProperty(context.Background(), object, "Наименование"); err != nil {
		t.Fatalf("the description is gone with the code: %v", err)
	} else if text, _ := value.AsString(); text != "Первый" {
		t.Fatalf("description = %q", text)
	}
}

// A description of length 0 is switched off the same way - 22 catalogs of the
// configurations being moved have none - and a catalog may have neither code
// nor description: ИТС 1590 lists the index the platform builds for exactly
// that case.
func TestADescriptionOfLengthZeroIsSwitchedOff(t *testing.T) {
	t.Parallel()
	nameless := strings.Replace(codelessCatalog(""), "description_length: 100", "description_length: 0", 1)
	if _, err := DecodeCatalog("object.yaml", strings.NewReader(nameless), metadataConfiguration()); err != nil {
		t.Fatalf("a catalog with neither code nor description was refused: %v", err)
	}
	if _, err := DecodeCatalog("object.yaml", strings.NewReader(nameless+"input_by_string:\n  - {standard: Наименование}\n"), metadataConfiguration()); err == nil ||
		!strings.Contains(err.Error(), "Наименование is switched off by a length of 0") {
		t.Fatalf("input by a switched off description: %v", err)
	}

	goodsID := uuid.MustNew()
	definition := CatalogDefinition{ID: goodsID, Name: "Товары", Code: CatalogCode{Type: StringType, Length: 9}}
	catalog := &Catalog{Catalogs: []CatalogDefinition{definition},
		catalogByName: map[string]int{"товары": 0}, catalogByID: map[uuid.UUID]int{goodsID: 0}}
	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	table, _ := PhysicalCatalogTable(goodsID)
	for _, candidate := range schema.Tables {
		if candidate.Name != table {
			continue
		}
		for _, column := range candidate.Columns {
			if column.Name == "description" {
				t.Fatal("the table got a description column")
			}
		}
	}
	runtime := &Runtime{catalog: catalog}
	parsed, err := querylang.Parse(`ВЫБРАТЬ Т.Наименование ИЗ Справочник.Товары КАК Т`)
	if err != nil {
		t.Fatal(err)
	}
	compiler, err := runtime.newQueryCompiler(context.Background(), parsed, map[string]bytecode.Value{}, nil)
	if err == nil {
		_, err = compiler.compile(parsed)
	}
	if err == nil {
		t.Fatal("a query read the description of a catalog that has none")
	}
	form, err := catalog.CatalogForm("Товары", ObjectForm, "ru")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range form.Fields {
		if field.Name == "Description" {
			t.Fatal("the generated form shows a description")
		}
	}
	if len(form.List.SearchFields) != 1 || form.List.SearchFields[0] != "Code" {
		t.Fatalf("the list is searched by %v", form.List.SearchFields)
	}
	object := &catalogObject{definition: definition, runtime: runtime,
		record: &CatalogRecord{Reference: CatalogReference{CatalogID: goodsID, ObjectID: uuid.MustNew()}, Code: "001"}}
	if _, err := runtime.GetObjectProperty(context.Background(), object, "Наименование"); err == nil {
		t.Fatal("the object of a nameless catalog has a description")
	}
}
