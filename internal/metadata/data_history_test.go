package metadata

import (
	"strings"
	"testing"
)

const (
	historyCatalogID   = "d1500000-0000-4000-8000-000000000001"
	historyRegisterID  = "d1500000-0000-4000-8000-000000000002"
	historyCommonID    = "d1500000-0000-4000-8000-000000000003"
	historyConstantID  = "d1500000-0000-4000-8000-000000000004"
	historyAttributeID = "d1500000-0000-4000-8000-000000000010"
	historyDimensionID = "d1500000-0000-4000-8000-000000000011"
	historyResourceID  = "d1500000-0000-4000-8000-000000000012"
)

func historyCatalogBody(body string) string {
	return `format: 1
id: ` + historyCatalogID + `
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true}
description_length: 150
attributes:
  - id: ` + historyAttributeID + `
    name: ИНН
    title: {ru: ИНН}
    types: [{kind: string, length: 12}]
` + body
}

// Participation has two values and no third, exactly as the field's flag has.
func TestAnObjectSaysWhetherItsVersionsAreKept(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		value   string
		accepts bool
	}{
		"участвует":    {"use", true},
		"не участвует": {"dont-use", true},
		"третьего нет": {"auto", false},
		"и четвёртого": {"иногда", false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, CatalogKind, historyCatalogID, historyCatalogBody("data_history: "+want.value+"\n"))
			catalog, err := Load(root)
			switch {
			case want.accepts && err != nil:
				t.Fatalf("%s: refused: %v", name, err)
			case !want.accepts:
				if err == nil || !strings.Contains(err.Error(), "data_history must be use or dont-use") {
					t.Fatalf("%s: %v", name, err)
				}
				return
			}
			definition, _ := catalog.CatalogDefinition("Контрагенты")
			if string(definition.DataHistory) != want.value {
				t.Fatalf("the flag was lost: %q", definition.DataHistory)
			}
		})
	}
}

// The two flags that travel with participation. They are carried whatever
// participation says, including with the history off: the demonstration
// configuration has the history off on all 629 of its objects, so a refusal
// there would fire only on configurations nobody has seen, while the flag
// itself reads as what it is - ready for the day the history is switched on.
func TestTheFlagsBesideDataHistoryAreCarriedEitherWay(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"история ведётся": `data_history: use
update_data_history_immediately_after_write: true
execute_after_data_history_version_write_processing: true
`,
		"история не ведётся, а флаги записаны": `data_history: dont-use
update_data_history_immediately_after_write: true
execute_after_data_history_version_write_processing: true
`,
		"история не объявлена вовсе, а флаги записаны": `update_data_history_immediately_after_write: true
execute_after_data_history_version_write_processing: true
`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, CatalogKind, historyCatalogID, historyCatalogBody(body))
			catalog, err := Load(root)
			if err != nil {
				t.Fatalf("%s: refused: %v", name, err)
			}
			definition, _ := catalog.CatalogDefinition("Контрагенты")
			switch {
			case !definition.UpdateDataHistoryImmediatelyAfterWrite:
				t.Fatalf("%s: when the version is built was lost: %+v", name, definition.DataHistorySettings)
			case !definition.ExecuteAfterDataHistoryVersionWriteProcessing:
				t.Fatalf("%s: whether the module sees the version was lost: %+v", name, definition.DataHistorySettings)
			}
		})
	}
}

// Ten kinds carry all three properties, and among the four registers only the
// information register is one of them - the one place where data history and
// the full-text flag part company. A constant is another: it has carried
// participation since block 1 and neither of the two flags.
//
// A common attribute carries the field's half, and only that half: the two
// flags belong to an object. The requirements said all three were available
// on an attribute; the syntax assistant gives a field one.
func TestDataHistoryReachesTheKindsThatCarryIt(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, InformationRegisterKind, historyRegisterID, `format: 1
id: `+historyRegisterID+`
name: Цены
title: {ru: Цены}
write_mode: independent
periodicity: day
data_history: use
update_data_history_immediately_after_write: true
dimensions:
  - {id: `+historyDimensionID+`, name: Товар, title: {ru: Товар}, types: [{kind: string, length: 20}]}
resources:
  - {id: `+historyResourceID+`, name: Цена, title: {ru: Цена}, types: [{kind: number, precision: 15, scale: 2}]}
`)
	writeMetadata(t, root, ConstantKind, historyConstantID, `format: 1
id: `+historyConstantID+`
name: Режим
title: {ru: Режим}
types: [{kind: boolean}]
data_history: use
execute_after_data_history_version_write_processing: true
`)
	writeMetadata(t, root, CommonAttributeKind, historyCommonID, `format: 1
id: `+historyCommonID+`
name: Организация
title: {ru: Организация}
types: [{kind: string, length: 50}]
data_history: use
objects: [`+historyRegisterID+`]
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	register, _ := catalog.InformationRegisterDefinition("Цены")
	switch {
	case register.DataHistory != DataHistoryUse:
		t.Fatalf("a register lost its flag: %q", register.DataHistory)
	case !register.UpdateDataHistoryImmediatelyAfterWrite:
		t.Fatalf("a register lost when the version is built: %+v", register.DataHistorySettings)
	}
	constant, ok := catalog.Constant("Режим")
	switch {
	case !ok || constant.DataHistory != DataHistoryUse:
		t.Fatalf("a constant lost its flag: %+v found=%v", constant, ok)
	case !constant.ExecuteAfterDataHistoryVersionWriteProcessing:
		t.Fatalf("a constant lost whether the module sees the version: %+v", constant.DataHistorySettings)
	}
	common, ok := catalog.CommonAttribute("Организация")
	if !ok || common.DataHistory != DataHistoryUse {
		t.Fatalf("a common attribute lost its flag: %+v found=%v", common, ok)
	}
	// A common attribute becomes an ordinary attribute of the object it
	// targets, and the flag has to survive that: kept on the definition alone
	// it would describe a field nothing sees.
	for _, attribute := range register.Attributes {
		if attribute.Name == "Организация" && attribute.DataHistory != UsageUse {
			t.Fatalf("the flag did not reach the propagated field: %+v", attribute)
		}
	}
}

// A common attribute has the field's flag and not the object's, so the two
// object flags must not be accepted on one: accepting them would describe a
// mechanism the prototype does not give it.
func TestACommonAttributeHasNoObjectFlagsOfDataHistory(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, InformationRegisterKind, historyRegisterID, `format: 1
id: `+historyRegisterID+`
name: Цены
title: {ru: Цены}
write_mode: independent
periodicity: day
dimensions:
  - {id: `+historyDimensionID+`, name: Товар, title: {ru: Товар}, types: [{kind: string, length: 20}]}
resources:
  - {id: `+historyResourceID+`, name: Цена, title: {ru: Цена}, types: [{kind: number, precision: 15, scale: 2}]}
`)
	writeMetadata(t, root, CommonAttributeKind, historyCommonID, `format: 1
id: `+historyCommonID+`
name: Организация
title: {ru: Организация}
types: [{kind: string, length: 50}]
data_history: use
update_data_history_immediately_after_write: true
objects: [`+historyRegisterID+`]
`)
	if _, err := Load(root); err == nil {
		t.Fatal("a common attribute took an object's flag")
	}
}
