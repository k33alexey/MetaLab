package metadata

import (
	"strings"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The defect this catches is the one the previous iteration shipped: a step
// that is a function of the attempt number alone.
//
// Two writers that collide see the same highest number, because neither sees
// the other's uncommitted row. Given the same step they add the same amount,
// collide again, and go on colliding every round until the attempts run out -
// and the write that runs out fails a write the base had every reason to
// accept. So the step has to differ between writers, which means drawn at
// random, which is what this checks. It also checks the two ends of the range,
// because both ends are a defect of their own: a step of zero puts the new row
// on the number the highest row already holds, so the insert conflicts for
// ever, and a step below zero breaks the CHECK the table carries.
func TestAccumulationTotalRowOffsetDisagreesBetweenWriters(t *testing.T) {
	t.Parallel()
	if first := accumulationTotalRowOffset(0); first != 1 {
		t.Fatalf("offset of the first try = %d, and a register nobody competes over must number its rows 0, 1, 2", first)
	}
	seen := map[int16]bool{}
	for call := 0; call < 500; call++ {
		for _, attempt := range []int{1, 2, 7} {
			offset := accumulationTotalRowOffset(attempt)
			if offset < 1 || offset > accumulationTotalRowSpread {
				t.Fatalf("offset of attempt %d = %d, outside 1..%d", attempt, offset, accumulationTotalRowSpread)
			}
			seen[offset] = true
		}
	}
	if len(seen) < 2 {
		t.Fatalf("every retry offset was the same value: two writers that collide once would collide every round after")
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
			Dimensions: []Attribute{{ID: productID, Name: "Товар", Types: []Type{{Kind: StringType, Length: 4}}}},
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
