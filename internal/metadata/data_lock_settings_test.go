package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

const (
	lockCatalogID   = "10c40000-0000-4000-8000-000000000001"
	lockRegisterID  = "10c40000-0000-4000-8000-000000000002"
	lockAttributeID = "10c40000-0000-4000-8000-000000000010"
	lockDimensionID = "10c40000-0000-4000-8000-000000000011"
	lockResourceID  = "10c40000-0000-4000-8000-000000000012"
)

func lockCatalogBody(body string) string {
	return `format: 1
id: ` + lockCatalogID + `
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
hierarchy: {enabled: true, kind: folders-and-items}
attributes:
  - id: ` + lockAttributeID + `
    name: Артикул
    title: {ru: Артикул}
    types: [{kind: string, length: 20}]
` + body
}

// The mode has three values, not two. The enumeration a metadata object uses
// is the one with «автоматический и управляемый» in it - an object read under
// one regime and written under the other - and we carried two of the three on
// a constant, so a configuration using the third was refused at the door.
func TestDataLockModeHasThreeValues(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		value   string
		accepts bool
	}{
		"управляемый":                  {"managed", true},
		"автоматический":               {"automatic", true},
		"автоматический и управляемый": {"automatic-and-managed", true},
		"такого режима нет":            {"ручной", false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, CatalogKind, lockCatalogID, lockCatalogBody("data_lock: "+want.value+"\n"))
			catalog, err := Load(root)
			switch {
			case want.accepts && err != nil:
				t.Fatalf("%s: refused: %v", name, err)
			case !want.accepts:
				if err == nil || !strings.Contains(err.Error(), "must be automatic, managed or automatic-and-managed") {
					t.Fatalf("%s: %v", name, err)
				}
				return
			}
			definition, _ := catalog.CatalogDefinition("Номенклатура")
			if string(definition.DataLock) != want.value {
				t.Fatalf("the mode was lost: %q", definition.DataLock)
			}
		})
	}
}

// The mode is on almost everything that stores data; the fields only on the
// eight kinds that have an object of their own. A register has records and no
// object, and the prototype gives it no list of fields - so neither do we.
func TestOnlyObjectsAreLockedByFields(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, InformationRegisterKind, lockRegisterID, `format: 1
id: `+lockRegisterID+`
name: Цены
title: {ru: Цены}
write_mode: independent
periodicity: day
data_lock: automatic
dimensions:
  - {id: `+lockDimensionID+`, name: Товар, title: {ru: Товар}, types: [{kind: string, length: 20}]}
resources:
  - {id: `+lockResourceID+`, name: Цена, title: {ru: Цена}, types: [{kind: number, precision: 15, scale: 2}]}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	register, _ := catalog.InformationRegisterDefinition("Цены")
	if register.DataLock != project.AutomaticDataLock {
		t.Fatalf("a register lost its lock mode: %q", register.DataLock)
	}

	// The register has no such property at all, so naming one is a file that
	// describes something the platform would not read.
	second := metadataProject(t)
	writeMetadata(t, second, InformationRegisterKind, lockRegisterID, `format: 1
id: `+lockRegisterID+`
name: Цены
title: {ru: Цены}
write_mode: independent
periodicity: day
data_lock_fields:
  - {attribute: Товар}
dimensions:
  - {id: `+lockDimensionID+`, name: Товар, title: {ru: Товар}, types: [{kind: string, length: 20}]}
resources:
  - {id: `+lockResourceID+`, name: Цена, title: {ru: Цена}, types: [{kind: number, precision: 15, scale: 2}]}
`)
	if _, err := Load(second); err == nil || !strings.Contains(err.Error(), "data_lock_fields") {
		t.Fatalf("a register accepted a list of fields it has not got: %v", err)
	}
}

// What may be in the list is wider than what input by string allows, and on
// purpose: the help states a rule for the search and only advice for the lock.
// A standard field the search would never look at - the owner, the parent - is
// written as a lock field by the demonstration configuration.
func TestAnObjectIsLockedByItsOwnFields(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		fields  string
		accepts bool
		refusal string
	}{
		"свой реквизит": {"  - {attribute: Артикул}\n", true, ""},
		"стандартное поле, по которому не ищут вводом по строке": {"  - {standard: Родитель}\n", true, ""},
		"несколько полей сразу":                                  {"  - {standard: Код}\n  - {attribute: Артикул}\n", true, ""},
		"поля нет у объекта":                                     {"  - {attribute: Цена}\n", false, "is not an attribute of this object"},
		"стандартного поля нет у вида": {"  - {standard: Номер}\n", false,
			"is not a standard field of this kind of object"},
		"одно поле дважды": {"  - {attribute: Артикул}\n  - {attribute: артикул}\n", false,
			"is already among the fields this object is locked by"},
		"ни то ни другое": {"  - {}\n", false, "names neither a standard field nor an attribute"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, CatalogKind, lockCatalogID, lockCatalogBody("data_lock_fields:\n"+want.fields))
			catalog, err := Load(root)
			switch {
			case want.accepts && err != nil:
				t.Fatalf("%s: refused: %v", name, err)
			case !want.accepts:
				if err == nil || !strings.Contains(err.Error(), want.refusal) {
					t.Fatalf("%s: %v", name, err)
				}
				return
			}
			definition, _ := catalog.CatalogDefinition("Номенклатура")
			if len(definition.DataLockFields) == 0 {
				t.Fatal("the fields were lost")
			}
			definition.DataLockFields[0] = ObjectField{}
			again, _ := catalog.CatalogDefinition("Номенклатура")
			if again.DataLockFields[0] == (ObjectField{}) {
				t.Fatal("the list of fields was handed out by reference")
			}
		})
	}
}

// Both settings reach the eight kinds through two shared shapes, so this
// guards the wiring rather than each kind's own code.
func TestEveryObjectKindCarriesBothLockSettings(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]struct {
		kind    Kind
		content string
		field   string
	}{
		"справочник": {CatalogKind, `code: {type: string, length: 9, auto: true}
description_length: 150
`, "Код"},
		"план видов характеристик": {ChartOfCharacteristicTypesKind, `code: {type: string, length: 9, auto: true}
description_length: 100
value_type: [{kind: string, length: 100}]
`, "Код"},
		"план счетов": {ChartOfAccountsKind, `code: {type: string, length: 5, auto: false}
description_length: 120
`, "Код"},
		"план видов расчёта": {ChartOfCalculationTypesKind, `code: {type: string, length: 9, auto: true}
description_length: 100
`, "Наименование"},
		"план обмена": {ExchangePlanKind, `code: {type: string, length: 36, auto: false}
description_length: 150
`, "Код"},
		"документ": {DocumentKind, `number: {type: string, length: 11, auto: true, periodicity: year}
`, "Дата"},
		"бизнес-процесс": {BusinessProcessKind, `number: {type: string, length: 11, auto: true, periodicity: year}
`, "ВедущаяЗадача"},
		"задача": {TaskKind, `number: {type: string, length: 11, auto: true, periodicity: none}
description_length: 150
addressing_attributes: []
`, "БизнесПроцесс"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, body.kind, lockCatalogID, `format: 1
id: `+lockCatalogID+`
name: Объект
title: {ru: Объект}
`+body.content+`data_lock: automatic-and-managed
data_lock_fields:
  - {standard: `+body.field+`}
`)
			if _, err := Load(root); err != nil {
				t.Fatalf("a kind that is locked by fields refused them: %v", err)
			}
		})
	}
}
