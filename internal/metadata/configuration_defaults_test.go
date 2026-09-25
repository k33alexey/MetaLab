package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	defaultsStyleItem      = "f1000000-0000-4000-8000-000000000001"
	defaultsStyle          = "f1000000-0000-4000-8000-000000000002"
	defaultsRole           = "f1000000-0000-4000-8000-000000000003"
	defaultsSecondRole     = "f1000000-0000-4000-8000-000000000004"
	defaultsReportForm     = "f1000000-0000-4000-8000-000000000010"
	defaultsSettingsForm   = "f1000000-0000-4000-8000-000000000011"
	defaultsVariantForm    = "f1000000-0000-4000-8000-000000000012"
	defaultsConstantsForm  = "f1000000-0000-4000-8000-000000000013"
	defaultsSearchForm     = "f1000000-0000-4000-8000-000000000014"
	defaultsAppearance     = "f1000000-0000-4000-8000-000000000020"
	defaultsSpreadsheet    = "f1000000-0000-4000-8000-000000000021"
	defaultsCommonStorage  = "f1000000-0000-4000-8000-000000000030"
	defaultsReportSettings = "f1000000-0000-4000-8000-000000000031"
	defaultsReportVariants = "f1000000-0000-4000-8000-000000000032"
	defaultsListSettings   = "f1000000-0000-4000-8000-000000000033"
	defaultsFormData       = "f1000000-0000-4000-8000-000000000034"
	defaultsStranger       = "f1000000-0000-4000-8000-0000000000ff"
)

// defaultsProject writes a project holding one of everything the root's
// defaults can name: a style, two roles, the five common forms, an appearance
// template beside a spreadsheet one, and the five settings storages.
func defaultsProject(t *testing.T) string {
	t.Helper()
	root := metadataProject(t)
	writeStyleItem(t, root, defaultsStyleItem, "ЦветПоля", ColorStyleItem,
		"  color: {source: absolute, rgb: '#1C55AE'}\n")
	writeMetadata(t, root, StyleKind, defaultsStyle, `format: 1
id: `+defaultsStyle+`
name: ОсновнойСтиль
title: {ru: Основной стиль}
items:
  - {item: `+defaultsStyleItem+`, value: {color: {source: absolute, rgb: '#FFFFFF'}}}
`)
	for id, name := range map[string]string{defaultsRole: "ПолныеПрава", defaultsSecondRole: "АдминистраторСистемы"} {
		writeMetadata(t, root, RoleKind, id, `format: 1
id: `+id+`
name: `+name+`
title: {ru: `+name+`}
`)
	}
	for id, name := range map[string]string{
		defaultsReportForm: "ФормаОтчета", defaultsSettingsForm: "ФормаНастроекОтчета",
		defaultsVariantForm: "ФормаВариантаОтчета", defaultsConstantsForm: "ФормаКонстант",
		defaultsSearchForm: "ФормаПоиска",
	} {
		writeCommonForm(t, root, name, `format: 1
id: `+id+`
name: `+name+`
title: {ru: `+name+`}
kind: common
`)
	}
	writeCommonTemplate(t, root, defaultsAppearance, "ОформлениеОтчетов", CompositionAppearance)
	writeCommonTemplate(t, root, defaultsSpreadsheet, "ПечатнаяФорма", SpreadsheetTemplate)
	for id, name := range map[string]string{
		defaultsCommonStorage: "ХранилищеОбщихНастроек", defaultsReportSettings: "ХранилищеНастроекОтчетов",
		defaultsReportVariants: "ХранилищеВариантов", defaultsListSettings: "ХранилищеНастроекСписков",
		defaultsFormData: "ХранилищеДанныхФорм",
	} {
		writeMetadata(t, root, SettingsStorageKind, id, `format: 1
id: `+id+`
name: `+name+`
title: {ru: `+name+`}
`)
	}
	return root
}

// mustParse is the identifier of a fixture, and a fixture that does not parse
// is a broken test rather than a failed assertion.
func mustParse(t *testing.T, value string) *uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	return &id
}

// saveDefaults writes the root with the defaults a test needs, through the same
// path Studio writes it: what the test proves has to survive being stored.
func saveDefaults(t *testing.T, root string, fill func(configuration *project.Project)) {
	t.Helper()
	configuration, err := project.ValidateLayout(root)
	if err != nil {
		t.Fatal(err)
	}
	fill(&configuration)
	if err := project.SaveConfiguration(root, configuration); err != nil {
		t.Fatal(err)
	}
}

// allDefaults fills every default of the root with the object written for it.
func allDefaults(t *testing.T) func(configuration *project.Project) {
	t.Helper()
	return func(configuration *project.Project) {
		configuration.DefaultStyle = mustParse(t, defaultsStyle)
		configuration.DefaultInterface = "ОсновнойИнтерфейс"
		configuration.DefaultRoles = []uuid.UUID{*mustParse(t, defaultsRole), *mustParse(t, defaultsSecondRole)}
		configuration.DefaultReportForm = mustParse(t, defaultsReportForm)
		configuration.DefaultReportSettingsForm = mustParse(t, defaultsSettingsForm)
		configuration.DefaultReportVariantForm = mustParse(t, defaultsVariantForm)
		configuration.DefaultConstantsForm = mustParse(t, defaultsConstantsForm)
		configuration.DefaultSearchForm = mustParse(t, defaultsSearchForm)
		configuration.DefaultReportAppearanceTemplate = mustParse(t, defaultsAppearance)
		configuration.CommonSettingsStorage = mustParse(t, defaultsCommonStorage)
		configuration.ReportsUserSettingsStorage = mustParse(t, defaultsReportSettings)
		configuration.ReportsVariantsStorage = mustParse(t, defaultsReportVariants)
		configuration.DynamicListsUserSettingsStorage = mustParse(t, defaultsListSettings)
		configuration.FormDataSettingsStorage = mustParse(t, defaultsFormData)
	}
}

// Every default of the root survives being written and read back, and every
// reference in it resolves to the object it names. A default that is stored but
// forgotten is the same loss as one that was never stored.
func TestConfigurationDefaultsAreCarriedAndResolved(t *testing.T) {
	t.Parallel()
	root := defaultsProject(t)
	saveDefaults(t, root, allDefaults(t))

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	configuration := catalog.Project
	switch {
	case configuration.DefaultStyle == nil || configuration.DefaultStyle.String() != defaultsStyle:
		t.Fatalf("the default style was lost: %+v", configuration.DefaultStyle)
	case configuration.DefaultInterface != "ОсновнойИнтерфейс":
		t.Fatalf("the default interface was lost: %q", configuration.DefaultInterface)
	case len(configuration.DefaultRoles) != 2 || configuration.DefaultRoles[0].String() != defaultsRole:
		t.Fatalf("the default roles were lost or reordered: %+v", configuration.DefaultRoles)
	case configuration.DefaultReportForm == nil || configuration.DefaultReportSettingsForm == nil ||
		configuration.DefaultReportVariantForm == nil || configuration.DefaultConstantsForm == nil ||
		configuration.DefaultSearchForm == nil:
		t.Fatalf("one of the five default forms was lost: %+v", configuration)
	case configuration.DefaultReportAppearanceTemplate == nil:
		t.Fatal("the report appearance template was lost")
	case configuration.CommonSettingsStorage == nil || configuration.ReportsUserSettingsStorage == nil ||
		configuration.ReportsVariantsStorage == nil || configuration.DynamicListsUserSettingsStorage == nil ||
		configuration.FormDataSettingsStorage == nil:
		t.Fatalf("one of the five storages was lost: %+v", configuration)
	}
}

// A default naming an object that is not in the configuration is refused where
// it is written, not where it is opened: an empty window is a far worse answer
// than a project that will not load.
func TestConfigurationDefaultPointingAtNothingIsRefused(t *testing.T) {
	t.Parallel()
	stranger := func(fill func(configuration *project.Project)) func(configuration *project.Project) {
		return fill
	}
	tests := map[string]struct {
		fill    func(configuration *project.Project)
		message string
	}{
		"стиль": {stranger(func(configuration *project.Project) {
			configuration.DefaultStyle = mustParse(t, defaultsStranger)
		}), "drawn with style"},
		"роль": {stranger(func(configuration *project.Project) {
			configuration.DefaultRoles = []uuid.UUID{*mustParse(t, defaultsStranger)}
		}), "runs under role"},
		"форма отчёта": {stranger(func(configuration *project.Project) {
			configuration.DefaultReportForm = mustParse(t, defaultsStranger)
		}), "opens reports"},
		"форма настроек отчёта": {stranger(func(configuration *project.Project) {
			configuration.DefaultReportSettingsForm = mustParse(t, defaultsStranger)
		}), "opens report settings"},
		"форма варианта отчёта": {stranger(func(configuration *project.Project) {
			configuration.DefaultReportVariantForm = mustParse(t, defaultsStranger)
		}), "opens report variants"},
		"форма констант": {stranger(func(configuration *project.Project) {
			configuration.DefaultConstantsForm = mustParse(t, defaultsStranger)
		}), "opens constants"},
		"форма поиска": {stranger(func(configuration *project.Project) {
			configuration.DefaultSearchForm = mustParse(t, defaultsStranger)
		}), "opens full-text search"},
		"макет оформления": {stranger(func(configuration *project.Project) {
			configuration.DefaultReportAppearanceTemplate = mustParse(t, defaultsStranger)
		}), "paints its reports"},
		"хранилище общих настроек": {stranger(func(configuration *project.Project) {
			configuration.CommonSettingsStorage = mustParse(t, defaultsStranger)
		}), "keeps common settings"},
		"хранилище настроек отчётов": {stranger(func(configuration *project.Project) {
			configuration.ReportsUserSettingsStorage = mustParse(t, defaultsStranger)
		}), "keeps user report settings"},
		"хранилище вариантов отчётов": {stranger(func(configuration *project.Project) {
			configuration.ReportsVariantsStorage = mustParse(t, defaultsStranger)
		}), "keeps report variants"},
		"хранилище настроек списков": {stranger(func(configuration *project.Project) {
			configuration.DynamicListsUserSettingsStorage = mustParse(t, defaultsStranger)
		}), "keeps user dynamic list settings"},
		"хранилище данных форм": {stranger(func(configuration *project.Project) {
			configuration.FormDataSettingsStorage = mustParse(t, defaultsStranger)
		}), "keeps form data"},
		"макет оформления чужого вида": {stranger(func(configuration *project.Project) {
			configuration.DefaultReportAppearanceTemplate = mustParse(t, defaultsSpreadsheet)
		}), "which is a spreadsheet"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := defaultsProject(t)
			saveDefaults(t, root, test.fill)
			_, err := Load(root)
			if err == nil {
				t.Fatal("a default pointing at nothing was accepted")
			}
			if !strings.Contains(err.Error(), test.message) {
				t.Fatalf("the error does not say what is wrong: %v", err)
			}
		})
	}
}

// The default language is the one default resolved with the languages
// themselves: a code, not an identifier, because every synonym in the
// configuration is checked against the languages long before a catalog exists.
func TestConfigurationDefaultLanguageIsCheckedAmongTheLanguages(t *testing.T) {
	t.Parallel()
	root := defaultsProject(t)
	configuration, err := project.ValidateLayout(root)
	if err != nil {
		t.Fatal(err)
	}
	configuration.DefaultLanguage = "de"
	if err := project.SaveConfiguration(root, configuration); err == nil {
		t.Fatal("a default language nobody configured was accepted")
	}
}

// The role editor validates one edited role against a catalog that holds no
// other roles. The root's default roles must not be judged there: they would
// all look missing, and editing any role in a configuration that names default
// roles would become impossible.
func TestConfigurationDefaultRolesSurviveRoleEditing(t *testing.T) {
	t.Parallel()
	root := defaultsProject(t)
	saveDefaults(t, root, allDefaults(t))
	edited := RoleDefinition{Format: CurrentFormat, ID: *mustParse(t, defaultsRole),
		Name: "ПолныеПрава", Title: LocalizedText{"ru": "Полные права"}}
	if err := ValidateProjectRole(root, edited); err != nil {
		t.Fatalf("a role cannot be edited while the root names default roles: %v", err)
	}
}
