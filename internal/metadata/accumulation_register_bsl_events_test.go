package metadata

import (
	"context"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestAccumulationRegisterBSLEvents(t *testing.T) {
	t.Parallel()
	registerID, resourceID := uuid.MustNew(), uuid.MustNew()
	definition := AccumulationRegisterDefinition{ID: registerID, Name: "Продажи", Kind: AccumulationRegisterTurnover, Resources: []Attribute{{ID: resourceID, Name: "Сумма", Types: []Type{{Kind: NumberType, Precision: 15, Scale: 2}}}}}
	catalog := &Catalog{AccumulationRegisters: []AccumulationRegisterDefinition{definition}, accumulationRegisterByName: map[string]int{"продажи": 0}, accumulationRegisterByID: map[uuid.UUID]int{registerID: 0}}
	repository := &AccumulationRegisterRepository{catalog: catalog}
	runtime, err := NewRuntimeWithAllRegisters(nil, nil, nil, nil, repository, catalog, nil)
	if err == nil {
		t.Fatal("repository without PostgreSQL was accepted")
	}
	// Event execution itself does not touch PostgreSQL, so use a runtime value with the same validated catalog.
	runtime = &Runtime{catalog: catalog, accumulationRegisterRepository: repository, accumulationRegisterEvents: map[uuid.UUID]AccumulationRegisterEventHandler{}}
	program, diagnostics := CompileAccumulationRegisterRecordSetModule(definition, "record-set.bsl", `&НаСервере
Процедура ПередЗаписью(Отказ, Замещение)
    ЭтотОбъект[0].Сумма = 25;
КонецПроцедуры

&НаСервере
Процедура ПриЗаписи(Отказ, Замещение)
    Отказ = Истина;
КонецПроцедуры`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewAccumulationRegisterBSLEvents(runtime, machine.NewContextWithMetadata(runtime), definition)
	if err != nil {
		t.Fatal(err)
	}
	set := &AccumulationRegisterRecordSet{RegisterID: registerID, Records: []*AccumulationRegisterRecord{{Resources: map[uuid.UUID]Value{resourceID: {Kind: NumberType, Data: "1"}}, Dimensions: map[uuid.UUID]Value{}, Attributes: map[uuid.UUID]Value{}}}}
	cancel, err := handler.HandleAccumulationRegisterEvent(context.Background(), AccumulationRegisterEventBeforeWrite, set, true)
	if err != nil || cancel || set.Records[0].Resources[resourceID].Data != "25" {
		t.Fatalf("before write set=%+v cancel=%v error=%v", set, cancel, err)
	}
	cancel, err = handler.HandleAccumulationRegisterEvent(context.Background(), AccumulationRegisterEventOnWrite, set, true)
	if err != nil || !cancel {
		t.Fatalf("on write cancel=%v error=%v", cancel, err)
	}
}
