package metadata

import (
	"strings"
	"testing"
)

// The setting is carried by the four kinds that keep predefined data, and by no
// others. The three values are the prototype's; anything else is a misspelling
// that would otherwise be read as "auto" and silently change what happens to
// real rows on the next update.
func TestPredefinedDataUpdateIsCarriedByTheKindsThatKeepPredefinedData(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]struct {
		kind    Kind
		id      string
		content string
		accepts bool
	}{
		"справочник, не обновлять": {CatalogKind, catalogID, `format: 1
id: ` + catalogID + `
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
predefined_data_update: dont-auto-update
`, true},
		"справочник, обновлять": {CatalogKind, catalogID, `format: 1
id: ` + catalogID + `
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
predefined_data_update: auto-update
`, true},
		"справочник, авто": {CatalogKind, catalogID, `format: 1
id: ` + catalogID + `
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
predefined_data_update: auto
`, true},
		"план видов характеристик": {ChartOfCharacteristicTypesKind, characteristicsID, `format: 1
id: ` + characteristicsID + `
name: Свойства
title: {ru: Свойства}
code: {type: string, length: 9, auto: true}
description_length: 100
value_type: [{kind: string, length: 100}]
predefined_data_update: dont-auto-update
`, true},
		"план видов расчёта": {ChartOfCalculationTypesKind, calcTypesStandardID, `format: 1
id: ` + calcTypesStandardID + `
name: Начисления
title: {ru: Начисления}
code: {type: string, length: 9, auto: true}
description_length: 100
predefined_data_update: dont-auto-update
`, true},
		"план счетов": {ChartOfAccountsKind, accountsID, `format: 1
id: ` + accountsID + `
name: Основной
title: {ru: Основной}
code: {type: string, length: 5, auto: false}
description_length: 120
predefined_data_update: dont-auto-update
`, true},
		"значения только три": {CatalogKind, catalogID, `format: 1
id: ` + catalogID + `
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
predefined_data_update: sometimes
`, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, body.kind, body.id, body.content)
			_, err := Load(root)
			switch {
			case body.accepts && err != nil:
				t.Fatalf("refused: %v", err)
			case !body.accepts && err == nil:
				t.Fatal("a value the prototype has no name for was accepted")
			case !body.accepts && !strings.Contains(err.Error(), "predefined_data_update must be"):
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// An exchange plan keeps no predefined items, and the help names only four
// managers as the ones with this setting. The key is refused where it is read,
// because the definition has no field for it - which is the strictest and
// cheapest refusal available.
func TestAKindWithoutPredefinedDataHasNoSuchSetting(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, ExchangePlanKind, exchangePlanID, `format: 1
id: `+exchangePlanID+`
name: Филиалы
title: {ru: Филиалы}
code: {type: string, length: 36, auto: false}
description_length: 150
predefined_data_update: dont-auto-update
`)
	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), "predefined_data_update") {
		t.Fatalf("err = %v", err)
	}
}

// Auto is kept as its own value rather than rewritten on the way in: it is what
// 112 of the 118 objects of the demonstration configuration say, and turning it
// into auto-update would be answering a question the configuration left open.
// What it resolves to is a separate question, asked in one place.
func TestAutoResolvesToUpdatingAutomatically(t *testing.T) {
	t.Parallel()
	for mode, want := range map[PredefinedDataUpdate]bool{
		"":                            true,
		PredefinedDataUpdateAuto:      true,
		PredefinedDataUpdateAutomatic: true,
		PredefinedDataUpdateManual:    false,
	} {
		if got := UpdatesPredefinedDataAutomatically(mode); got != want {
			t.Errorf("UpdatesPredefinedDataAutomatically(%q) = %v, want %v", mode, got, want)
		}
	}
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
predefined_data_update: auto
`)
	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	definition, ok := loaded.CatalogDefinition("номенклатура")
	if !ok || definition.PredefinedDataUpdate != PredefinedDataUpdateAuto {
		t.Fatalf("auto was not kept as itself: %q", definition.PredefinedDataUpdate)
	}
}
