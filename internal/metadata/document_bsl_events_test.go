package metadata

import (
	"context"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestDocumentBSLEventsProvideThisObjectAndWriteModes(t *testing.T) {
	t.Parallel()
	definition := DocumentDefinition{
		ID: uuid.MustNew(), Name: "Продажа",
		Number: DocumentNumber{Type: StringType, Length: 11, Periodicity: NumberPeriodYear},
	}
	catalog := &Catalog{Documents: []DocumentDefinition{definition}}
	catalog.documentByName = map[string]int{"продажа": 0}
	catalog.documentByID = map[uuid.UUID]int{definition.ID: 0}
	runtime := &Runtime{catalog: catalog, events: make(map[uuid.UUID]CatalogEventHandler), documentEvents: make(map[uuid.UUID]DocumentEventHandler)}
	program, diagnostics := CompileDocumentObjectModule(definition, "object.bsl", `&НаСервере
Процедура ОбработкаЗаполнения(ДанныеЗаполнения, СтандартнаяОбработка)
    ЭтотОбъект.Номер = "SALE-1";
    ThisObject.Дата = '20260906120000';
КонецПроцедуры

&НаСервере
Процедура ПередЗаписью(Отказ, РежимЗаписи, РежимПроведения)
    Если ЭтотОбъект.Номер = "" Или РежимЗаписи <> "Write" Или РежимПроведения <> "DoNotPost" Тогда
        Отказ = Истина;
    КонецЕсли;
КонецПроцедуры

&НаСервере
Процедура ПередУдалением(Отказ)
    Отказ = Истина;
КонецПроцедуры`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewDocumentBSLEvents(runtime, machine.NewContextWithMetadata(runtime), definition)
	if err != nil {
		t.Fatal(err)
	}
	record := &DocumentRecord{
		Reference: DocumentReference{DocumentID: definition.ID, ObjectID: uuid.MustNew()}, Date: time.Now().UTC(),
		Attributes: map[uuid.UUID]Value{}, TableParts: map[uuid.UUID][]DocumentRow{},
	}
	cancelled, err := handler.HandleDocumentEvent(context.Background(), DocumentEventFill, record)
	if err != nil || cancelled || record.Number != "SALE-1" || record.Date.Year() != 2026 {
		t.Fatalf("fill record=%+v cancelled=%v error=%v", record, cancelled, err)
	}
	record.Number = ""
	cancelled, err = handler.HandleDocumentEvent(context.Background(), DocumentEventBefore, record)
	if err != nil || !cancelled {
		t.Fatalf("before cancelled=%v error=%v", cancelled, err)
	}
	cancelled, err = handler.HandleDocumentEvent(context.Background(), DocumentEventBeforeDelete, record)
	if err != nil || !cancelled {
		t.Fatalf("before-delete cancelled=%v error=%v", cancelled, err)
	}
}

func TestDocumentEventRoutinesReceivePostingModes(t *testing.T) {
	t.Parallel()
	ctx := withDocumentOperation(context.Background(), DocumentPost, DocumentPostingRealTime)
	_, _, before := documentEventRoutine(ctx, DocumentEventBefore, DocumentDefinition{})
	_, _, posting := documentEventRoutine(ctx, DocumentEventPosting, DocumentDefinition{})
	if before[1].String() != "Post" || before[2].String() != "RealTime" || posting[1].String() != "RealTime" {
		t.Fatalf("before=%v posting=%v", before, posting)
	}
}
