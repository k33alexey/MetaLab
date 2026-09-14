package metadata

import (
	"context"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestCatalogEventSubscriptionHandlerFiresOnlyForItsOwnEventAndCancels(t *testing.T) {
	t.Parallel()
	definition := CatalogDefinition{ID: uuid.MustNew(), Name: "Товары", Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 250}
	catalog := &Catalog{Catalogs: []CatalogDefinition{definition}}
	catalog.catalogByName = map[string]int{"товары": 0}
	catalog.catalogByID = map[uuid.UUID]int{definition.ID: 0}
	runtime := &Runtime{catalog: catalog, events: make(map[uuid.UUID]CatalogEventHandler)}
	subscription := EventSubscriptionDefinition{
		ID: uuid.MustNew(), Name: "ЗапретПустогоНаименования", Objects: []uuid.UUID{definition.ID},
		Event: "before-write", Module: uuid.MustNew(), Procedure: "ПередЗаписьюТовара",
	}
	program, diagnostics := compiler.CompileModules([]compiler.ModuleSource{{
		Name: "ОбщегоНазначения", Filename: "common.bsl", Source: `&НаСервере
Процедура ПередЗаписьюТовара(Источник, Отказ) Экспорт
    Если Источник.Наименование = "" Тогда
        Отказ = Истина;
    КонецЕсли;
КонецПроцедуры`,
	}})
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	handler := &catalogEventSubscriptionHandler{
		runtime: runtime, context: machine.NewContextWithMetadata(runtime),
		definition: definition, subscription: subscription, module: "ОбщегоНазначения",
	}
	record := &CatalogRecord{
		Reference:  CatalogReference{CatalogID: definition.ID, ObjectID: uuid.MustNew()},
		Attributes: map[uuid.UUID]Value{}, TableParts: map[uuid.UUID][]CatalogRow{},
	}
	cancelled, err := handler.HandleCatalogEvent(context.Background(), CatalogEventOnWrite, record)
	if err != nil || cancelled {
		t.Fatalf("mismatched event must be a no-op: cancelled=%v error=%v", cancelled, err)
	}
	cancelled, err = handler.HandleCatalogEvent(context.Background(), CatalogEventBefore, record)
	if err != nil || !cancelled {
		t.Fatalf("empty name must cancel: cancelled=%v error=%v", cancelled, err)
	}
	record.Description = "Молоко"
	cancelled, err = handler.HandleCatalogEvent(context.Background(), CatalogEventBefore, record)
	if err != nil || cancelled {
		t.Fatalf("filled name must not cancel: cancelled=%v error=%v", cancelled, err)
	}
}

func TestCatalogEventHandlersCombineObjectModuleAndSubscriptions(t *testing.T) {
	t.Parallel()
	definition := CatalogDefinition{ID: uuid.MustNew(), Name: "Товары", Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 250}
	catalog := &Catalog{Catalogs: []CatalogDefinition{definition}}
	catalog.catalogByName = map[string]int{"товары": 0}
	catalog.catalogByID = map[uuid.UUID]int{definition.ID: 0}
	runtime := &Runtime{catalog: catalog, events: make(map[uuid.UUID]CatalogEventHandler)}
	objectProgram, diagnostics := CompileCatalogObjectModule(definition, "object.bsl", `&НаСервере
Процедура ПередЗаписью(Отказ)
КонецПроцедуры`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	objectMachine, err := vm.New(objectProgram)
	if err != nil {
		t.Fatal(err)
	}
	objectHandler, err := NewCatalogBSLEvents(runtime, objectMachine.NewContextWithMetadata(runtime), definition)
	if err != nil {
		t.Fatal(err)
	}
	subscriptionProgram, diagnostics := compiler.CompileModules([]compiler.ModuleSource{{
		Name: "ОбщегоНазначения", Filename: "common.bsl", Source: `&НаСервере
Процедура ЗапретитьЗапись(Источник, Отказ) Экспорт
    Отказ = Истина;
КонецПроцедуры`,
	}})
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	subscriptionMachine, err := vm.New(subscriptionProgram)
	if err != nil {
		t.Fatal(err)
	}
	subscriptionHandler := &catalogEventSubscriptionHandler{
		runtime: runtime, context: subscriptionMachine.NewContextWithMetadata(runtime),
		definition: definition, module: "ОбщегоНазначения",
		subscription: EventSubscriptionDefinition{
			ID: uuid.MustNew(), Name: "ЗапретЗаписи", Objects: []uuid.UUID{definition.ID}, Event: "before-write", Procedure: "ЗапретитьЗапись",
		},
	}
	handlers := catalogEventHandlers{objectHandler, subscriptionHandler}
	record := &CatalogRecord{
		Reference:  CatalogReference{CatalogID: definition.ID, ObjectID: uuid.MustNew()},
		Attributes: map[uuid.UUID]Value{}, TableParts: map[uuid.UUID][]CatalogRow{},
	}
	cancelled, err := handlers.HandleCatalogEvent(context.Background(), CatalogEventBefore, record)
	if err != nil || !cancelled {
		t.Fatalf("object module allows the write but the subscription must still cancel it: cancelled=%v error=%v", cancelled, err)
	}
}

func TestDocumentEventSubscriptionHandlerFiresOnlyForItsOwnEventAndCancels(t *testing.T) {
	t.Parallel()
	definition := DocumentDefinition{ID: uuid.MustNew(), Name: "Продажа", Number: DocumentNumber{Type: StringType, Length: 11, Periodicity: NumberPeriodYear}}
	catalog := &Catalog{Documents: []DocumentDefinition{definition}}
	catalog.documentByName = map[string]int{"продажа": 0}
	catalog.documentByID = map[uuid.UUID]int{definition.ID: 0}
	runtime := &Runtime{catalog: catalog, events: make(map[uuid.UUID]CatalogEventHandler), documentEvents: make(map[uuid.UUID]DocumentEventHandler)}
	subscription := EventSubscriptionDefinition{
		ID: uuid.MustNew(), Name: "ЗапретПустогоНомера", Objects: []uuid.UUID{definition.ID},
		Event: "before-write", Module: uuid.MustNew(), Procedure: "ПередЗаписьюПродажи",
	}
	program, diagnostics := compiler.CompileModules([]compiler.ModuleSource{{
		Name: "ОбщегоНазначения", Filename: "common.bsl", Source: `&НаСервере
Процедура ПередЗаписьюПродажи(Источник, Отказ, РежимЗаписи, РежимПроведения) Экспорт
    Если Источник.Номер = "" Тогда
        Отказ = Истина;
    КонецЕсли;
КонецПроцедуры`,
	}})
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	handler := &documentEventSubscriptionHandler{
		runtime: runtime, context: machine.NewContextWithMetadata(runtime),
		definition: definition, subscription: subscription, module: "ОбщегоНазначения",
	}
	record := &DocumentRecord{
		Reference:  DocumentReference{DocumentID: definition.ID, ObjectID: uuid.MustNew()},
		Attributes: map[uuid.UUID]Value{}, TableParts: map[uuid.UUID][]DocumentRow{},
	}
	cancelled, err := handler.HandleDocumentEvent(context.Background(), DocumentEventAfter, record)
	if err != nil || cancelled {
		t.Fatalf("mismatched event must be a no-op: cancelled=%v error=%v", cancelled, err)
	}
	cancelled, err = handler.HandleDocumentEvent(context.Background(), DocumentEventBefore, record)
	if err != nil || !cancelled {
		t.Fatalf("empty number must cancel: cancelled=%v error=%v", cancelled, err)
	}
	record.Number = "SALE-1"
	cancelled, err = handler.HandleDocumentEvent(context.Background(), DocumentEventBefore, record)
	if err != nil || cancelled {
		t.Fatalf("filled number must not cancel: cancelled=%v error=%v", cancelled, err)
	}
}

func TestInformationRegisterEventSubscriptionHandlerEchoesReplaceAndCancels(t *testing.T) {
	t.Parallel()
	resourceID := uuid.MustNew()
	definition := InformationRegisterDefinition{
		ID: uuid.MustNew(), Name: "Цены", WriteMode: InformationRegisterIndependent, Periodicity: InformationRegisterPeriodNone,
		Resources: []Attribute{{ID: resourceID, Name: "Цена", Types: []Type{{Kind: NumberType, Precision: 15, Scale: 2}}}},
	}
	catalog := &Catalog{
		InformationRegisters:      []InformationRegisterDefinition{definition},
		informationRegisterByName: map[string]int{"цены": 0}, informationRegisterByID: map[uuid.UUID]int{definition.ID: 0},
	}
	runtime := &Runtime{
		catalog: catalog, events: map[uuid.UUID]CatalogEventHandler{}, documentEvents: map[uuid.UUID]DocumentEventHandler{},
		informationRegisterEvents: map[uuid.UUID]InformationRegisterEventHandler{},
	}
	subscription := EventSubscriptionDefinition{
		ID: uuid.MustNew(), Name: "ЗапретДобавления", Objects: []uuid.UUID{definition.ID},
		Event: "before-write", Module: uuid.MustNew(), Procedure: "ПередЗаписьюЦен",
	}
	program, diagnostics := compiler.CompileModules([]compiler.ModuleSource{{
		Name: "ОбщегоНазначения", Filename: "common.bsl", Source: `&НаСервере
Процедура ПередЗаписьюЦен(Источник, Отказ, Замещение) Экспорт
    Если Не Замещение Тогда
        Отказ = Истина;
    КонецЕсли;
КонецПроцедуры`,
	}})
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	handler := &informationRegisterEventSubscriptionHandler{
		runtime: runtime, context: machine.NewContextWithMetadata(runtime),
		definition: definition, subscription: subscription, module: "ОбщегоНазначения",
	}
	set := &InformationRegisterRecordSet{
		RegisterID: definition.ID, Filter: InformationRegisterFilter{Dimensions: map[uuid.UUID]Value{}},
		Records: []*InformationRegisterRecord{{RecordID: uuid.MustNew(), Active: true, Dimensions: map[uuid.UUID]Value{}, Resources: map[uuid.UUID]Value{}, Attributes: map[uuid.UUID]Value{}}},
	}
	cancelled, err := handler.HandleInformationRegisterEvent(context.Background(), InformationRegisterEventBeforeWrite, set, true)
	if err != nil || cancelled {
		t.Fatalf("replace must not cancel: cancelled=%v error=%v", cancelled, err)
	}
	cancelled, err = handler.HandleInformationRegisterEvent(context.Background(), InformationRegisterEventBeforeWrite, set, false)
	if err != nil || !cancelled {
		t.Fatalf("append must cancel: cancelled=%v error=%v", cancelled, err)
	}
	cancelled, err = handler.HandleInformationRegisterEvent(context.Background(), InformationRegisterEventAfterWrite, set, true)
	if err != nil || cancelled {
		t.Fatalf("mismatched event must be a no-op: cancelled=%v error=%v", cancelled, err)
	}
}
