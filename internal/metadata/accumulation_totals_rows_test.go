package metadata

import (
	"strings"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Writers that make rows of one combination at once see the same highest
// number, and a step that does not differ between them puts them all on one
// number: all but the first then wait for it, which is the waiting splitting
// exists to avoid. So the step is drawn at random, which is what this checks.
// It also checks the two ends of the range, each a defect of its own: a step
// of zero puts the new row on the highest one already there, and a step below
// zero breaks the CHECK the table carries.
func TestAccumulationTotalRowStepDisagreesBetweenWriters(t *testing.T) {
	t.Parallel()
	seen := map[int16]bool{}
	for call := 0; call < 1500; call++ {
		step := accumulationTotalRowStep()
		if step < 1 || step > accumulationTotalRowSpread {
			t.Fatalf("step = %d, outside 1..%d", step, accumulationTotalRowSpread)
		}
		seen[step] = true
	}
	if len(seen) < 2 {
		t.Fatalf("every step was the same value: writers making rows at once would all land on one number")
	}
}

// accumulationValidationFixture is a register and a document that writes into
// it, with no database behind them: everything below is refused before any
// statement is sent.
type accumulationValidationFixture struct {
	repository *AccumulationRegisterRepository
	definition AccumulationRegisterDefinition
	recorder   DocumentReference
	stranger   DocumentReference
	product    uuid.UUID
	amount     uuid.UUID
}

func newAccumulationValidationFixture() accumulationValidationFixture {
	documentID, strangerID, registerID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	productID, amountID := uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{
		Documents: []DocumentDefinition{
			{ID: documentID, Name: "Продажа", Movements: []uuid.UUID{registerID}},
			{ID: strangerID, Name: "Заказ"},
		},
		AccumulationRegisters: []AccumulationRegisterDefinition{{
			ID: registerID, Name: "Продажи", Kind: AccumulationRegisterTurnover, AllowTotalsSplitting: true,
			Dimensions: []RegisterDimension{{Attribute: Attribute{ID: productID, Name: "Товар", Types: []Type{{Kind: StringType, Length: 4}}}}},
			Resources:  []Attribute{{ID: amountID, Name: "Сумма", Types: []Type{{Kind: NumberType, Precision: 15, Scale: 2}}}},
		}},
		documentByName:             map[string]int{"продажа": 0, "заказ": 1},
		documentByID:               map[uuid.UUID]int{documentID: 0, strangerID: 1},
		accumulationRegisterByName: map[string]int{"продажи": 0},
		accumulationRegisterByID:   map[uuid.UUID]int{registerID: 0},
	}
	definition := catalog.AccumulationRegisters[0]
	return accumulationValidationFixture{
		repository: &AccumulationRegisterRepository{catalog: catalog},
		definition: definition,
		recorder:   DocumentReference{DocumentID: documentID, ObjectID: uuid.MustNew()},
		stranger:   DocumentReference{DocumentID: strangerID, ObjectID: uuid.MustNew()},
		product:    productID,
		amount:     amountID,
	}
}

// TestAccumulationRegisterRefusesInvalidRecords walks the wrong input a record
// set can carry. Every case here would otherwise reach the movement table, and
// what is written into movements is what every balance is later computed from -
// a value the storage accepts and the metadata does not is a wrong number in
// every report from that day on, with nothing left to say where it came from.
func TestAccumulationRegisterRefusesInvalidRecords(t *testing.T) {
	t.Parallel()
	fixture := newAccumulationValidationFixture()
	period := time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC)
	good := func() *AccumulationRegisterRecord {
		return &AccumulationRegisterRecord{
			Period: period, Recorder: fixture.recorder, LineNumber: 1, Active: true,
			Dimensions: map[uuid.UUID]Value{fixture.product: {Kind: StringType, Data: "A"}},
			Resources:  map[uuid.UUID]Value{fixture.amount: {Kind: NumberType, Data: "10"}},
		}
	}
	cases := []struct {
		name    string
		records []*AccumulationRegisterRecord
		message string
	}{
		{"нет записи", []*AccumulationRegisterRecord{nil}, "is nil"},
		{"пустой период", func() []*AccumulationRegisterRecord {
			record := good()
			record.Period = time.Time{}
			return []*AccumulationRegisterRecord{record}
		}(), "period"},
		{"год за пределами", func() []*AccumulationRegisterRecord {
			record := good()
			record.Period = time.Date(4100, 1, 1, 0, 0, 0, 0, time.UTC)
			return []*AccumulationRegisterRecord{record}
		}(), "period"},
		{"регистратор не пишет в регистр", func() []*AccumulationRegisterRecord {
			record := good()
			record.Recorder = fixture.stranger
			return []*AccumulationRegisterRecord{record}
		}(), "recorder"},
		{"номер строки за пределами", func() []*AccumulationRegisterRecord {
			record := good()
			record.LineNumber = -1
			return []*AccumulationRegisterRecord{record}
		}(), "line number"},
		{"номер строки повторяется", func() []*AccumulationRegisterRecord {
			first, second := good(), good()
			second.LineNumber = first.LineNumber
			return []*AccumulationRegisterRecord{first, second}
		}(), "duplicate line number"},
		{"ресурс не число", func() []*AccumulationRegisterRecord {
			record := good()
			record.Resources[fixture.amount] = Value{Kind: StringType, Data: "десять"}
			return []*AccumulationRegisterRecord{record}
		}(), "resources"},
		{"пустая строка вместо числа", func() []*AccumulationRegisterRecord {
			record := good()
			record.Resources[fixture.amount] = Value{Kind: NumberType, Data: ""}
			return []*AccumulationRegisterRecord{record}
		}(), "resources"},
		{"измерение длиннее объявленного", func() []*AccumulationRegisterRecord {
			record := good()
			record.Dimensions[fixture.product] = Value{Kind: StringType, Data: "слишком длинное"}
			return []*AccumulationRegisterRecord{record}
		}(), "dimensions"},
		{"незнакомое измерение", func() []*AccumulationRegisterRecord {
			record := good()
			record.Dimensions[uuid.MustNew()] = Value{Kind: StringType, Data: "A"}
			return []*AccumulationRegisterRecord{record}
		}(), "unknown attribute"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			set := &AccumulationRegisterRecordSet{
				RegisterID: fixture.definition.ID,
				Filter:     AccumulationRegisterFilter{Recorder: &fixture.recorder},
				Records:    testCase.records,
			}
			err := fixture.repository.normalizeRecords(fixture.definition, set)
			if err == nil {
				t.Fatalf("the record set was accepted")
			}
			if !strings.Contains(err.Error(), testCase.message) {
				t.Fatalf("error = %q, expected it to name %q", err, testCase.message)
			}
		})
	}
}

// A record that names a recorder other than the one the set is filtered by
// belongs to somebody else's set, and writing it would put a movement under a
// document that does not know about it - a movement no reposting of either
// document would ever clear.
func TestAccumulationRegisterRefusesARecordOfAnotherRecorder(t *testing.T) {
	t.Parallel()
	fixture := newAccumulationValidationFixture()
	other := DocumentReference{DocumentID: fixture.recorder.DocumentID, ObjectID: uuid.MustNew()}
	set := &AccumulationRegisterRecordSet{
		RegisterID: fixture.definition.ID,
		Filter:     AccumulationRegisterFilter{Recorder: &fixture.recorder},
		Records: []*AccumulationRegisterRecord{{
			Period: time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC), Recorder: other, LineNumber: 1, Active: true,
			Resources: map[uuid.UUID]Value{fixture.amount: {Kind: NumberType, Data: "1"}},
		}},
	}
	err := fixture.repository.normalizeRecords(fixture.definition, set)
	if err == nil || !strings.Contains(err.Error(), "does not match the recorder filter") {
		t.Fatalf("error = %v", err)
	}
}
