package metadata

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The tests in this file were written from a mutation run over the model: each
// one names a place where the code could be broken - a limit moved by one, a
// size miscounted, a condition turned around - and every test the package had
// still passed. A limit is tested on both sides of it, because the broken
// version differs from the right one only at the edge.

// registerWithFields reads a register of the kind with that many dimensions,
// resources and attributes, all distinct, and returns the error.
func registerWithFields(kind string, dimensions, resources, attributes int) error {
	var source strings.Builder
	source.WriteString("format: 1\nid: " + uuid.MustNew().String() + "\nname: Регистр\ntitle: {ru: Регистр}\n")
	field := func(prefix string, index int, types string) string {
		return fmt.Sprintf("  - {id: %s, name: %s%d, title: {ru: %s%d}, types: [%s]}\n", uuid.MustNew(), prefix, index, prefix, index, types)
	}
	group := func(name, prefix string, count int, types string) {
		if count == 0 {
			return
		}
		source.WriteString(name + ":\n")
		for index := 0; index < count; index++ {
			source.WriteString(field(prefix, index, types))
		}
	}
	text := "{kind: string, length: 10}"
	number := "{kind: number, precision: 15, scale: 2}"
	switch kind {
	case "information":
		source.WriteString("write_mode: independent\nperiodicity: none\n")
	case "accumulation":
		source.WriteString("kind: turnover\n")
	}
	group("dimensions", "Измерение", dimensions, text)
	group("resources", "Ресурс", resources, number)
	group("attributes", "Реквизит", attributes, text)
	var err error
	switch kind {
	case "information":
		_, err = DecodeInformationRegister("register.yaml", strings.NewReader(source.String()), demoConfiguration())
	case "accumulation":
		_, err = DecodeAccumulationRegister("register.yaml", strings.NewReader(source.String()), demoConfiguration())
	}
	return err
}

func requireMessage(t *testing.T, label string, err error, message string, present bool) {
	t.Helper()
	got := err != nil && strings.Contains(err.Error(), message)
	if got != present {
		t.Fatalf("%s: error %v, expected %q present=%v", label, err, message, present)
	}
}

// Thirty-two dimensions are allowed and thirty-three are not; fifteen hundred
// fields in all are allowed and one more is not. The defect on each side: a
// register the prototype accepts refused on import, or a register past the
// limit taken in and built into a table the database would refuse.
func TestRegisterFieldLimitsHoldOnBothSides(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"information", "accumulation"} {
		requireMessage(t, kind+", 32 dimensions", registerWithFields(kind, 32, 1, 0), "more than 32 items", false)
		requireMessage(t, kind+", 33 dimensions", registerWithFields(kind, 33, 1, 0), "more than 32 items", true)
		requireMessage(t, kind+", 1500 fields", registerWithFields(kind, 1, 1, 1498), "more than 1500 items in total", false)
		requireMessage(t, kind+", 1501 fields", registerWithFields(kind, 1, 1, 1499), "more than 1500 items in total", true)
	}
	// An information register needs one field of any sort, and one is enough.
	requireMessage(t, "information, no fields", registerWithFields("information", 0, 0, 0), "must contain at least one item", true)
	requireMessage(t, "information, one attribute", registerWithFields("information", 0, 0, 1), "must contain at least one item", false)
	// Fields of different groups count together: a count that subtracted one
	// group from another would call a register of a resource and an attribute
	// empty.
	requireMessage(t, "information, a resource and an attribute", registerWithFields("information", 0, 1, 1), "must contain at least one item", false)
}

// Sixty-four additional indexes of an object and thirty-two fields in one
// index are the platform's ceilings, and the check sits exactly on them.
func TestAdditionalIndexLimitsHoldOnBothSides(t *testing.T) {
	t.Parallel()
	fields := make([]string, 40)
	for index := range fields {
		fields[index] = fmt.Sprintf("Поле%d", index)
	}
	tables := recordIndexTables(nil, fields)
	indexes := func(count, width int) []AdditionalIndex {
		result := make([]AdditionalIndex, count)
		for index := range result {
			result[index] = AdditionalIndex{Name: fmt.Sprintf("Индекс%d", index), IndexedFields: []string{fields[index%len(fields)]}}
		}
		if width > 0 {
			result[0].IndexedFields = fields[:width]
		}
		return result
	}
	joined := func(issues []string) error {
		if len(issues) == 0 {
			return nil
		}
		return fmt.Errorf("%s", strings.Join(issues, "; "))
	}
	requireMessage(t, "64 indexes", joined(validateAdditionalIndexes(indexes(64, 0), InformationRegisterKind, tables)), "must not contain more than 64 items", false)
	requireMessage(t, "65 indexes", joined(validateAdditionalIndexes(indexes(65, 0), InformationRegisterKind, tables)), "must not contain more than 64 items", true)
	requireMessage(t, "32 fields", joined(validateAdditionalIndexes(indexes(1, 32), InformationRegisterKind, tables)), "an index takes at most 32", false)
	requireMessage(t, "33 fields", joined(validateAdditionalIndexes(indexes(1, 33), InformationRegisterKind, tables)), "an index takes at most 32", true)
	// The ceiling is on the fields an index names, key and carried alike.
	carried := indexes(1, 20)
	carried[0].AdditionalFields = fields[20:33]
	requireMessage(t, "20 key fields and 13 carried", joined(validateAdditionalIndexes(carried, InformationRegisterKind, tables)), "an index takes at most 32", true)
}

// Thirty-two recalculations of one calculation register are allowed and one
// more is not.
func TestRecalculationLimitHoldsOnBothSides(t *testing.T) {
	t.Parallel()
	register := func(count int) error {
		value := CalculationRegisterDefinition{Recalculations: make([]Recalculation, count)}
		issues := validateRecalculations(value, demoConfiguration())
		if len(issues) == 0 {
			return nil
		}
		return fmt.Errorf("%s", strings.Join(issues, "; "))
	}
	requireMessage(t, "32 recalculations", register(32), "must not contain more than 32 items", false)
	requireMessage(t, "33 recalculations", register(33), "must not contain more than 32 items", true)
}

// A dimension linked to a schedule needs a register that has one: the link
// names a dimension of the schedule, and with no schedule it names nothing.
func TestScheduleLinkNeedsASchedule(t *testing.T) {
	t.Parallel()
	source := func(withSchedule bool) string {
		text := "format: 1\nid: " + uuid.MustNew().String() + "\nname: Начисления\ntitle: {ru: Начисления}\n" +
			"chart_of_calculation_types: " + uuid.MustNew().String() + "\nperiodicity: month\n"
		if withSchedule {
			text += "schedule: " + uuid.MustNew().String() + "\nschedule_value: " + uuid.MustNew().String() + "\nschedule_date: " + uuid.MustNew().String() + "\n"
		}
		return text + "dimensions:\n  - {id: " + uuid.MustNew().String() + ", name: Сотрудник, title: {ru: Сотрудник}, types: [{kind: string, length: 10}], schedule_link: " + uuid.MustNew().String() + "}\n" +
			"resources:\n  - {id: " + uuid.MustNew().String() + ", name: Результат, title: {ru: Результат}, types: [{kind: number, precision: 15, scale: 2}]}\n"
	}
	_, err := DecodeCalculationRegister("register.yaml", strings.NewReader(source(false)), demoConfiguration())
	requireMessage(t, "no schedule", err, "schedule_link needs a schedule", true)
	_, err = DecodeCalculationRegister("register.yaml", strings.NewReader(source(true)), demoConfiguration())
	requireMessage(t, "with a schedule", err, "schedule_link needs a schedule", false)
}

// Separation needs both session parameters, and each one missing alone is
// refused - a check that looked at only one of them passed the other gap.
func TestDataSeparationNeedsBothSessionParameters(t *testing.T) {
	t.Parallel()
	decode := func(value, use bool) error {
		attribute := commonAttributeFixture(uuid.MustNew())
		attribute.DataSeparation = SeparationSeparate
		if value {
			id := uuid.MustNew()
			attribute.DataSeparationValue = &id
		}
		if use {
			id := uuid.MustNew()
			attribute.DataSeparationUse = &id
		}
		var encoded bytes.Buffer
		if err := Encode(&encoded, attribute); err != nil {
			t.Fatal(err)
		}
		_, err := DecodeCommonAttribute("common-attribute.yaml", bytes.NewReader(encoded.Bytes()), metadataConfiguration())
		return err
	}
	const message = "data_separation needs data_separation_value and data_separation_use"
	requireMessage(t, "value only", decode(true, false), message, true)
	requireMessage(t, "use only", decode(false, true), message, true)
	requireMessage(t, "both", decode(true, true), message, false)
}

// How large a value can be in an index entry decides whether a dimension goes
// into the index of the balances, and a miscount either side is a defect: too
// small lets a dimension in whose entry the database refuses at posting time,
// too large keeps out one the index could have served.
func TestIndexEntrySizeIsCountedByStorage(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		storage attributeStorage
		size    int
		bounded bool
	}{
		{attributeStorage{sqlType: "character varying(10)"}, 44, true},
		{attributeStorage{sqlType: "character(10)"}, 44, true},
		{attributeStorage{sqlType: "numeric(15,2)"}, 15, true},
		{attributeStorage{sqlType: "numeric(10)"}, 13, true},
		{attributeStorage{sqlType: "uuid"}, 16, true},
		{attributeStorage{sqlType: "boolean"}, 1, true},
		{attributeStorage{sqlType: "timestamp with time zone"}, 8, true},
		{attributeStorage{sqlType: "text"}, 0, false},
		{attributeStorage{sqlType: "bytea"}, 0, false},
		{attributeStorage{sqlType: "character varying(0)"}, 0, false},
		{attributeStorage{sqlType: "jsonb", composite: true}, 0, false},
	} {
		size, bounded := btreeEntryBytes(testCase.storage)
		if size != testCase.size || bounded != testCase.bounded {
			t.Fatalf("%+v: size %d bounded %v, want %d %v", testCase.storage, size, bounded, testCase.size, testCase.bounded)
		}
	}
}

// The index of the balances takes dimensions while their entries fit in the
// budget - the period's eight bytes and the dimensions', two thousand in all -
// and a dimension that would pass it is left out while the ones after it may
// still fit. The edge is exact: a string of 497 characters is 1992 bytes and
// brings the entry to the budget itself, one more character passes it.
func TestBalanceIndexTakesDimensionsWithinItsBudget(t *testing.T) {
	t.Parallel()
	keys := func(lengths ...int) []string {
		definition := AccumulationRegisterDefinition{ID: uuid.MustNew(), Kind: AccumulationRegisterBalance}
		for index, length := range lengths {
			definition.Dimensions = append(definition.Dimensions, RegisterDimension{Attribute: Attribute{
				ID: uuid.MustNew(), Name: fmt.Sprintf("Измерение%d", index), Types: []Type{{Kind: StringType, Length: length}},
			}})
		}
		result, err := (&Catalog{}).balanceTotalsIndexKeys(definition)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	if got := keys(497); len(got) != 2 {
		t.Fatalf("a dimension that brings the entry exactly to the budget was left out: %v", got)
	}
	if got := keys(498); len(got) != 1 {
		t.Fatalf("a dimension past the budget went into the index: %v", got)
	}
	// 200 characters are 804 bytes: two of them take 1616 with the period,
	// a third would pass the budget, a boolean-sized one after it still fits.
	if got := keys(200, 200, 200, 1); len(got) != 4 {
		t.Fatalf("keys = %v, want the period, the first two and the last", got)
	}
}

// A balance register's movements carry the direction of each one - receipt or
// expense - and a turnover register's do not: its movements have no direction.
func TestOnlyABalanceRegisterRecordsTheDirectionOfAMovement(t *testing.T) {
	t.Parallel()
	for kind, want := range map[AccumulationRegisterKindValue]bool{AccumulationRegisterBalance: true, AccumulationRegisterTurnover: false} {
		definition := AccumulationRegisterDefinition{ID: uuid.MustNew(), Name: "Регистр", Kind: kind,
			Resources: []Attribute{{ID: uuid.MustNew(), Name: "Сумма", Types: []Type{{Kind: NumberType, Precision: 15, Scale: 2}}}}}
		movements, _, err := (&Catalog{}).accumulationRegisterTables(definition)
		if err != nil {
			t.Fatal(err)
		}
		column, check := false, false
		for _, item := range movements.Columns {
			column = column || item.Name == "movement_kind"
		}
		for _, item := range movements.Constraints {
			check = check || strings.Contains(item.Definition, "movement_kind")
		}
		if column != want || check != want {
			t.Fatalf("%s: movement_kind column %v, check %v, want %v", kind, column, check, want)
		}
	}
}

// A resource whose type cannot be resolved is an error, not a type of zero
// precision that every amount of the register would then be rounded to.
func TestAResourceTypeThatCannotBeResolvedIsAnError(t *testing.T) {
	t.Parallel()
	resource := Attribute{ID: uuid.MustNew(), Name: "Сумма", Types: []Type{{Kind: CatalogType, Reference: func() *uuid.UUID { id := uuid.MustNew(); return &id }()}}}
	if _, err := (&Catalog{}).accumulationResourceType(resource); err == nil {
		t.Fatal("a resource of an unresolvable type was given a type")
	}
}
