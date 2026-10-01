package metadata

import (
	"context"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A subscription is attached, when the program starts, to what its source
// reaches - the one document named, every document, the documents of a
// defined type - and its procedure is what runs: here it refuses the write. The
// defect it catches is a source that reads right and is never connected, so the
// handler of a whole kind or of a defined type does not run at all.
func TestASubscriptionRunsForWhatItsSourceReaches(t *testing.T) {
	t.Parallel()
	sale, purchase, definedID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	module := commonModuleFixture()
	subscription := func(name string, source Type) EventSubscriptionDefinition {
		return EventSubscriptionDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: name, Title: LocalizedText{"ru": name},
			Source: []Type{source}, Event: "before-write", Module: module.ID, Procedure: "Запретить"}
	}
	catalog := &Catalog{
		Documents: []DocumentDefinition{
			{ID: sale, Name: "Продажа", Number: DocumentNumber{Type: StringType, Length: 9, Periodicity: NumberPeriodNone}},
			{ID: purchase, Name: "Покупка", Number: DocumentNumber{Type: StringType, Length: 9, Periodicity: NumberPeriodNone}},
		},
		DefinedTypes:  []DefinedTypeObject{{ID: definedID, Name: "ОбъектПродажи", Types: []Type{{Kind: DocumentObjectType, Reference: &sale}}}},
		CommonModules: []CommonModuleDefinition{module},
		EventSubscriptions: []EventSubscriptionDefinition{
			subscription("ОднаПродажа", Type{Kind: DocumentObjectType, Reference: &sale}),
			subscription("ВсеДокументы", Type{Kind: DocumentObjectType}),
			subscription("ЧерезОпределяемый", Type{Kind: DefinedType, Reference: &definedID}),
		},
		documentByName: map[string]int{"продажа": 0, "покупка": 1}, documentByID: map[uuid.UUID]int{sale: 0, purchase: 1},
		definedTypeByName: map[string]int{"объектпродажи": 0}, definedTypeByID: map[uuid.UUID]int{definedID: 0},
		commonModuleByName: map[string]int{"общегоназначения": 0}, commonModuleByID: map[uuid.UUID]int{module.ID: 0},
	}
	runtime := &Runtime{
		catalog: catalog, events: make(map[uuid.UUID]CatalogEventHandler),
		documentEvents: make(map[uuid.UUID]DocumentEventHandler), informationRegisterEvents: make(map[uuid.UUID]InformationRegisterEventHandler),
		accumulationRegisterEvents: make(map[uuid.UUID]AccumulationRegisterEventHandler),
	}
	path, err := project.ModulePath(module.Module)
	if err != nil {
		t.Fatal(err)
	}
	program, diagnostics := compiler.CompileModules([]compiler.ModuleSource{{
		Name: module.Name, Filename: path,
		Source: "Процедура Запретить(Источник, Отказ, РежимЗаписи, РежимПроведения) Экспорт\n    Отказ = Истина;\nКонецПроцедуры",
	}})
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	if err := runtime.ConfigureProgramEvents(program); err != nil {
		t.Fatal(err)
	}
	subscriptionsOf := func(id uuid.UUID) []string {
		handlers, _ := runtime.documentEventHandler(id).(documentEventHandlers)
		var names []string
		for _, handler := range handlers {
			if subscribed, ok := handler.(*documentEventSubscriptionHandler); ok {
				names = append(names, subscribed.subscription.Name)
			}
		}
		return names
	}
	if got := subscriptionsOf(sale); len(got) != 3 {
		t.Fatalf("the sale is listened to by %v, want all three", got)
	}
	if got := subscriptionsOf(purchase); len(got) != 1 || got[0] != "ВсеДокументы" {
		t.Fatalf("the purchase is listened to by %v, want only the one for every document", got)
	}
	handlers, _ := runtime.documentEventHandler(purchase).(documentEventHandlers)
	record := &DocumentRecord{Reference: DocumentReference{DocumentID: purchase, ObjectID: uuid.MustNew()},
		Attributes: map[uuid.UUID]Value{}, TableParts: map[uuid.UUID][]DocumentRow{}}
	cancelled, err := handlers.HandleDocumentEvent(context.Background(), DocumentEventBefore, record)
	if err != nil || !cancelled {
		t.Fatalf("the procedure of the subscription for every document did not run: cancelled=%v error=%v", cancelled, err)
	}
}
