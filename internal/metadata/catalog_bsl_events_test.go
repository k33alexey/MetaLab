package metadata

import (
	"context"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestCatalogBSLEventsProvideThisObjectAndCancellation(t *testing.T) {
	t.Parallel()
	definition := CatalogDefinition{
		ID: uuid.MustNew(), Name: "Товары", Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 250,
	}
	catalog := &Catalog{Catalogs: []CatalogDefinition{definition}}
	catalog.catalogByName = map[string]int{"товары": 0}
	catalog.catalogByID = map[uuid.UUID]int{definition.ID: 0}
	runtime := &Runtime{catalog: catalog, events: make(map[uuid.UUID]CatalogEventHandler)}
	program, diagnostics := CompileCatalogObjectModule(definition, "object.bsl", `&НаСервере
Процедура ОбработкаЗаполнения(ДанныеЗаполнения, СтандартнаяОбработка)
    ЭтотОбъект.Код = "AUTO";
    ThisObject.Наименование = "Заполнен";
КонецПроцедуры

&НаСервере
Процедура ПередЗаписью(Отказ)
    Если ЭтотОбъект.Наименование = "" Тогда
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
	cancelled, err := handler.HandleCatalogEvent(context.Background(), CatalogEventFill, record)
	if err != nil || cancelled || record.Code != "AUTO" || record.Description != "Заполнен" {
		t.Fatalf("fill record=%+v cancelled=%v error=%v", record, cancelled, err)
	}
	record.Description = ""
	cancelled, err = handler.HandleCatalogEvent(context.Background(), CatalogEventBefore, record)
	if err != nil || !cancelled {
		t.Fatalf("before cancelled=%v error=%v", cancelled, err)
	}
}

func TestCatalogReferenceTypeCannotBeMixed(t *testing.T) {
	t.Parallel()
	products := CatalogDefinition{ID: uuid.MustNew(), Name: "Товары"}
	partners := CatalogDefinition{ID: uuid.MustNew(), Name: "Контрагенты"}
	catalog := &Catalog{
		Catalogs:      []CatalogDefinition{products, partners},
		catalogByName: map[string]int{"товары": 0, "контрагенты": 1},
		catalogByID:   map[uuid.UUID]int{products.ID: 0, partners.ID: 1},
	}
	runtime := &Runtime{catalog: catalog, events: make(map[uuid.UUID]CatalogEventHandler)}
	value, err := runtime.wrapCatalogReference(partners, CatalogReference{CatalogID: partners.ID, ObjectID: uuid.MustNew()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.catalogValueFromBSL([]Type{{Kind: CatalogType, Reference: &products.ID}}, value, "attribute Product"); err == nil {
		t.Fatal("catalogValueFromBSL accepted a reference of another catalog")
	}
}
