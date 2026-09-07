package metadata

import (
	"context"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestInformationRegisterBSLEventsProvideRecordSetAndCancellation(t *testing.T) {
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
	program, diagnostics := CompileInformationRegisterRecordSetModule(definition, "record-set.bsl", `&НаСервере
Процедура ПередЗаписью(Отказ, Замещение)
    ЭтотОбъект[0].Цена = 125.50;
    Если Не Замещение Тогда
        Отказ = Истина;
    КонецЕсли;
КонецПроцедуры

&НаСервере
Процедура ПриЗаписи(Отказ, Замещение)
    Если ЭтотОбъект.Количество() <> 1 Тогда
        Отказ = Истина;
    КонецЕсли;
КонецПроцедуры

&НаСервере
Процедура ПослеЗаписи(Замещение)
КонецПроцедуры`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewInformationRegisterBSLEvents(runtime, machine.NewContextWithMetadata(runtime), definition)
	if err != nil {
		t.Fatal(err)
	}
	set := &InformationRegisterRecordSet{
		RegisterID: definition.ID, Filter: InformationRegisterFilter{Dimensions: map[uuid.UUID]Value{}},
		Records: []*InformationRegisterRecord{{RecordID: uuid.MustNew(), Active: true, Dimensions: map[uuid.UUID]Value{}, Resources: map[uuid.UUID]Value{}, Attributes: map[uuid.UUID]Value{}}},
	}
	cancelled, err := handler.HandleInformationRegisterEvent(context.Background(), InformationRegisterEventBeforeWrite, set, true)
	if err != nil || cancelled || set.Records[0].Resources[resourceID].Data != "125.5" {
		t.Fatalf("before set=%+v cancelled=%v error=%v", set, cancelled, err)
	}
	cancelled, err = handler.HandleInformationRegisterEvent(context.Background(), InformationRegisterEventBeforeWrite, set, false)
	if err != nil || !cancelled {
		t.Fatalf("append cancellation=%v error=%v", cancelled, err)
	}
	for _, event := range []InformationRegisterEvent{InformationRegisterEventOnWrite, InformationRegisterEventAfterWrite} {
		if cancelled, err = handler.HandleInformationRegisterEvent(context.Background(), event, set, true); err != nil || cancelled {
			t.Fatalf("event=%s cancellation=%v error=%v", event, cancelled, err)
		}
	}
}
