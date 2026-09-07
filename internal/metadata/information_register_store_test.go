package metadata

import (
	"strings"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestInformationRegisterPeriodNormalizationAndKeys(t *testing.T) {
	t.Parallel()
	value := time.Date(2026, 8, 17, 12, 34, 56, 789_123_000, time.FixedZone("test", 2*60*60))
	tests := []struct {
		periodicity InformationRegisterPeriodicity
		want        time.Time
	}{
		{InformationRegisterPeriodSecond, time.Date(2026, 8, 17, 10, 34, 56, 0, time.UTC)},
		{InformationRegisterPeriodDay, time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)},
		{InformationRegisterPeriodMonth, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)},
		{InformationRegisterPeriodQuarter, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)},
		{InformationRegisterPeriodYear, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{InformationRegisterPeriodRecorderPosition, time.Date(2026, 8, 17, 10, 34, 56, 789_100_000, time.UTC)},
	}
	for _, test := range tests {
		got, err := normalizeInformationRegisterPeriod(test.periodicity, value)
		if err != nil || !got.Equal(test.want) {
			t.Fatalf("periodicity %s: got=%s want=%s error=%v", test.periodicity, got, test.want, err)
		}
	}

	registerID, dimensionID := uuid.MustNew(), uuid.MustNew()
	definition := InformationRegisterDefinition{
		ID: registerID, WriteMode: InformationRegisterIndependent, Periodicity: InformationRegisterPeriodDay,
		Dimensions: []Attribute{{ID: dimensionID}},
	}
	first := InformationRegisterRecord{Period: tests[1].want, Dimensions: map[uuid.UUID]Value{dimensionID: {Kind: UUIDType, Data: uuid.MustNew().String()}}}
	second := first
	second.RecordID = uuid.MustNew()
	if informationRegisterRecordKey(definition, first) != informationRegisterRecordKey(definition, second) {
		t.Fatal("internal record UUID changed the semantic register key")
	}
	second.Dimensions = mapsCloneValues(first.Dimensions)
	second.Dimensions[dimensionID] = Value{Kind: UUIDType, Data: uuid.MustNew().String()}
	if informationRegisterRecordKey(definition, first) == informationRegisterRecordKey(definition, second) {
		t.Fatal("different dimensions produced the same register key")
	}
}

func TestInformationRegisterRecordSetCloneIsIndependent(t *testing.T) {
	t.Parallel()
	registerID, dimensionID := uuid.MustNew(), uuid.MustNew()
	period := time.Now().UTC()
	set := &InformationRegisterRecordSet{
		RegisterID: registerID,
		Filter:     InformationRegisterFilter{Period: &period, Dimensions: map[uuid.UUID]Value{dimensionID: {Kind: StringType, Data: "A"}}},
		Records:    []*InformationRegisterRecord{{RecordID: uuid.MustNew(), Dimensions: map[uuid.UUID]Value{dimensionID: {Kind: StringType, Data: "A"}}}},
	}
	clone := cloneInformationRegisterRecordSet(set)
	clone.Filter.Dimensions[dimensionID] = Value{Kind: StringType, Data: "B"}
	clone.Records[0].Dimensions[dimensionID] = Value{Kind: StringType, Data: "B"}
	*clone.Filter.Period = period.Add(time.Hour)
	if set.Filter.Dimensions[dimensionID].Data != "A" || set.Records[0].Dimensions[dimensionID].Data != "A" || !set.Filter.Period.Equal(period) {
		t.Fatal("record set clone shares mutable state")
	}
}

func TestRecorderInformationRegisterHasBothSemanticAndRecorderKeys(t *testing.T) {
	t.Parallel()
	registerID, documentID, dimensionID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	firstRecorder := DocumentReference{DocumentID: documentID, ObjectID: uuid.MustNew()}
	secondRecorder := DocumentReference{DocumentID: documentID, ObjectID: uuid.MustNew()}
	period := time.Date(2026, 9, 7, 12, 30, 0, 0, time.UTC)
	value := Value{Kind: UUIDType, Data: uuid.MustNew().String()}
	record := InformationRegisterRecord{
		Period: period, Recorder: firstRecorder, LineNumber: 1,
		Dimensions: map[uuid.UUID]Value{dimensionID: value},
	}
	definition := InformationRegisterDefinition{
		ID: registerID, WriteMode: InformationRegisterRecorder, Periodicity: InformationRegisterPeriodDay,
		Dimensions: []Attribute{{ID: dimensionID}},
	}
	otherLineAndRecorder := record
	otherLineAndRecorder.LineNumber = 2
	otherLineAndRecorder.Recorder = secondRecorder
	if informationRegisterRecordKey(definition, record) != informationRegisterRecordKey(definition, otherLineAndRecorder) {
		t.Fatal("regular periodic semantic key must be based on period and dimensions")
	}
	definition.Periodicity = InformationRegisterPeriodRecorderPosition
	if informationRegisterRecordKey(definition, record) == informationRegisterRecordKey(definition, otherLineAndRecorder) {
		t.Fatal("recorder-position semantic key must include the recorder")
	}

	filter := InformationRegisterFilter{Period: &period, Recorder: &firstRecorder, Dimensions: map[uuid.UUID]Value{dimensionID: value}}
	recorderOnly := InformationRegisterFilter{Recorder: &firstRecorder, Dimensions: map[uuid.UUID]Value{}}
	if informationRegisterFilterLockKey(definition, filter) != informationRegisterFilterLockKey(definition, recorderOnly) {
		t.Fatal("recorder lock key must not change when narrower fields are also set")
	}
}

func TestInformationRegisterChildObjectAccountsForRetainedRecordSet(t *testing.T) {
	t.Parallel()
	registerID, resourceID := uuid.MustNew(), uuid.MustNew()
	set := &InformationRegisterRecordSet{
		RegisterID: registerID, Filter: InformationRegisterFilter{Dimensions: map[uuid.UUID]Value{}},
		Records: []*InformationRegisterRecord{{
			Resources: map[uuid.UUID]Value{resourceID: {Kind: StringType, Data: strings.Repeat("x", 2048)}},
		}},
	}
	owner := &informationRegisterRecordSetObject{set: set}
	for _, object := range []interface {
		RuntimeDynamicMemory(uint64) (uint64, bool)
	}{
		&informationRegisterRecordObject{owner: owner, record: set.Records[0]},
		&informationRegisterFilterObject{owner: owner},
		&informationRegisterFilterItemObject{owner: owner},
	} {
		if _, ok := object.RuntimeDynamicMemory(1024); ok {
			t.Fatal("child runtime object bypassed the retained record-set memory limit")
		}
	}
}

func TestInformationRegisterAssignsLineAfterHighestExistingLine(t *testing.T) {
	t.Parallel()
	registerID, documentID, dimensionID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	recorder := DocumentReference{DocumentID: documentID, ObjectID: uuid.MustNew()}
	definition := InformationRegisterDefinition{
		ID: registerID, Name: "Движения", WriteMode: InformationRegisterRecorder, Periodicity: InformationRegisterPeriodRecorderPosition,
		Recorders: []uuid.UUID{documentID}, Dimensions: []Attribute{{ID: dimensionID, Name: "Ключ", Required: true, Types: []Type{{Kind: UUIDType}}}},
	}
	firstPeriod := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	secondPeriod := firstPeriod.Add(time.Second)
	set := &InformationRegisterRecordSet{
		RegisterID: registerID, Filter: InformationRegisterFilter{Recorder: &recorder, Dimensions: map[uuid.UUID]Value{}},
		Records: []*InformationRegisterRecord{
			{Period: firstPeriod, Recorder: recorder, LineNumber: 3, Dimensions: map[uuid.UUID]Value{dimensionID: {Kind: UUIDType, Data: uuid.MustNew().String()}}, Resources: map[uuid.UUID]Value{}, Attributes: map[uuid.UUID]Value{}},
			{Period: secondPeriod, Recorder: recorder, Dimensions: map[uuid.UUID]Value{dimensionID: {Kind: UUIDType, Data: uuid.MustNew().String()}}, Resources: map[uuid.UUID]Value{}, Attributes: map[uuid.UUID]Value{}},
		},
	}
	repository := &InformationRegisterRepository{catalog: &Catalog{}}
	if err := repository.normalizeRecordSetRecords(definition, set.Filter, set); err != nil {
		t.Fatal(err)
	}
	if set.Records[1].LineNumber != 4 {
		t.Fatalf("automatic line number=%d want=4", set.Records[1].LineNumber)
	}
}
