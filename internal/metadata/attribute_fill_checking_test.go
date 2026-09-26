package metadata

import (
	"context"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The filling check is a check, not a constraint. The prototype keeps no
// column the database refuses to leave empty, so neither do we: what used to
// be a required attribute now says it is checked, and the column stays
// nullable.
func TestCheckedFieldDoesNotMakeItsColumnRefuseEmptiness(t *testing.T) {
	t.Parallel()
	catalogID, checked, plain := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	definition := CatalogDefinition{
		ID: catalogID, Name: "Контрагенты", Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 150,
		Attributes: []Attribute{
			{ID: checked, Name: "ИНН", Types: []Type{{Kind: StringType, Length: 12}}, FillChecking: ShowFillingError},
			{ID: plain, Name: "Комментарий", Types: []Type{{Kind: StringType, Length: 100}}},
		},
	}
	catalog := &Catalog{Catalogs: []CatalogDefinition{definition}}
	table, _, err := catalog.catalogTables(definition)
	if err != nil {
		t.Fatal(err)
	}
	for _, attribute := range definition.Attributes {
		column, err := PhysicalAttributeColumn(attribute.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, stored := range table.Columns {
			if stored.Name == column && !stored.Nullable {
				t.Fatalf("%s refuses an empty value, and the prototype has no such column: %+v", attribute.Name, stored)
			}
		}
	}
}

// What the check checks travels to the handler: the platform hands
// ОбработкаПроверкиЗаполнения the attributes to check, and that list is now
// the fields that say they are checked.
func TestFillCheckEventIsHandedTheCheckedFields(t *testing.T) {
	t.Parallel()
	definition := CatalogDefinition{
		ID: uuid.MustNew(), Name: "Контрагенты", Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 150,
		Attributes: []Attribute{
			{ID: uuid.MustNew(), Name: "ИНН", Types: []Type{{Kind: StringType, Length: 12}}, FillChecking: ShowFillingError},
			{ID: uuid.MustNew(), Name: "Комментарий", Types: []Type{{Kind: StringType, Length: 100}}, FillChecking: DontCheckFilling},
		},
	}
	catalog := &Catalog{Catalogs: []CatalogDefinition{definition}}
	catalog.catalogByName = map[string]int{"контрагенты": 0}
	catalog.catalogByID = map[uuid.UUID]int{definition.ID: 0}
	runtime := &Runtime{catalog: catalog, events: make(map[uuid.UUID]CatalogEventHandler)}
	program, diagnostics := CompileCatalogObjectModule(definition, "object.bsl", `&НаСервере
Процедура ОбработкаПроверкиЗаполнения(Отказ, ПроверяемыеРеквизиты)
    Если ПроверяемыеРеквизиты.Количество() <> 1 Тогда
        Отказ = Истина;
    ИначеЕсли ПроверяемыеРеквизиты[0] <> "ИНН" Тогда
        Отказ = Истина;
    КонецЕсли;
КонецПроцедуры`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewCatalogBSLEvents(runtime, machine.NewContextWithMetadata(runtime), definition)
	if err != nil {
		t.Fatal(err)
	}
	record := &CatalogRecord{
		Reference:  CatalogReference{CatalogID: definition.ID, ObjectID: uuid.MustNew()},
		Attributes: map[uuid.UUID]Value{}, TableParts: map[uuid.UUID][]CatalogRow{},
	}
	cancelled, err := handler.HandleCatalogEvent(context.Background(), CatalogEventFillCheck, record)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled {
		t.Fatal("the checked fields handed to the handler were not the fields that say they are checked")
	}
}

// A predefined item may leave a checked field empty. The check belongs to the
// user filling a form, and a predefined item is filled by the developer.
func TestPredefinedItemMayLeaveACheckedFieldEmpty(t *testing.T) {
	t.Parallel()
	attribute, predefined := uuid.MustNew(), uuid.MustNew()
	_, err := DecodeCatalog("object.yaml", strings.NewReader(`format: 1
id: `+uuid.MustNew().String()+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true}
description_length: 150
attributes:
  - {id: `+attribute.String()+`, name: ИНН, title: {ru: ИНН}, types: [{kind: string, length: 12}], fill_checking: show-error}
predefined:
  - {id: `+predefined.String()+`, name: Основной, description: Основной}
`), metadataConfiguration())
	if err != nil {
		t.Fatalf("a predefined item was refused for leaving a checked field empty: %v", err)
	}
}

func TestBrokenFillCheckingIsRefused(t *testing.T) {
	t.Parallel()
	_, err := DecodeCatalog("object.yaml", strings.NewReader(`format: 1
id: `+uuid.MustNew().String()+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true}
description_length: 150
attributes:
  - {id: `+uuid.MustNew().String()+`, name: ИНН, title: {ru: ИНН}, types: [{kind: string, length: 12}], fill_checking: refuse}
`), metadataConfiguration())
	if err == nil || !strings.Contains(err.Error(), "fill_checking must be dont-check or show-error") {
		t.Fatalf("an unknown filling check was accepted: %v", err)
	}
}
