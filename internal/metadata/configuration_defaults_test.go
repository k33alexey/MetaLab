package metadata

import (
	"os"
	"path/filepath"
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

const (
	defaultsDictionaryTemplate = "f1000000-0000-4000-8000-000000000060"
	defaultsDictionaryConstant = "f1000000-0000-4000-8000-000000000061"
)

// A dictionary of the full-text search is kept either in a common template or
// in a constant, and both are resolved by the kind the root wrote beside the
// identifier: a dictionary that is not there is not a search that works a
// little worse, it is a search that will not start.
func TestFullTextSearchDictionariesAreResolvedByTheirKind(t *testing.T) {
	t.Parallel()
	root := defaultsProject(t)
	writeCommonTemplate(t, root, defaultsDictionaryTemplate, "СловарьСинонимов", BinaryTemplate)
	writeMetadata(t, root, ConstantKind, defaultsDictionaryConstant, `format: 1
id: `+defaultsDictionaryConstant+`
name: СловарьМорфологии
title: {ru: Словарь морфологии}
types: [{kind: string, length: 100}]
`)
	saveDefaults(t, root, func(configuration *project.Project) {
		configuration.AdditionalFullTextSearchDictionaries = []project.DictionaryReference{
			{Kind: project.TemplateDictionary, Object: *mustParse(t, defaultsDictionaryTemplate)},
			{Kind: project.ConstantDictionary, Object: *mustParse(t, defaultsDictionaryConstant)},
		}
	})
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Project.AdditionalFullTextSearchDictionaries) != 2 {
		t.Fatalf("a dictionary was lost: %+v", catalog.Project.AdditionalFullTextSearchDictionaries)
	}

	for name, dictionary := range map[string]project.DictionaryReference{
		"макета нет":            {Kind: project.TemplateDictionary, Object: *mustParse(t, defaultsStranger)},
		"константы нет":         {Kind: project.ConstantDictionary, Object: *mustParse(t, defaultsStranger)},
		"вид перепутан":         {Kind: project.ConstantDictionary, Object: *mustParse(t, defaultsDictionaryTemplate)},
		"вид перепутан ещё раз": {Kind: project.TemplateDictionary, Object: *mustParse(t, defaultsDictionaryConstant)},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			broken := defaultsProject(t)
			writeCommonTemplate(t, broken, defaultsDictionaryTemplate, "СловарьСинонимов", BinaryTemplate)
			writeMetadata(t, broken, ConstantKind, defaultsDictionaryConstant, `format: 1
id: `+defaultsDictionaryConstant+`
name: СловарьМорфологии
title: {ru: Словарь морфологии}
types: [{kind: string, length: 100}]
`)
			saveDefaults(t, broken, func(configuration *project.Project) {
				configuration.AdditionalFullTextSearchDictionaries = []project.DictionaryReference{dictionary}
			})
			if _, err := Load(broken); err == nil {
				t.Fatal("a dictionary pointing at nothing was accepted")
			} else if !strings.Contains(err.Error(), "as a dictionary") {
				t.Fatalf("the error does not say what is wrong: %v", err)
			}
		})
	}
}

// The rest of the root's references resolve the same way the first fifteen do,
// including the four that name a form nothing will ever open: a mechanism that
// does not run is no reason to lose track of which form was meant.
func TestRemainingConfigurationReferencesAreResolved(t *testing.T) {
	t.Parallel()
	for name, fill := range map[string]func(configuration *project.Project){
		"хранилище внешних данных ссылок": func(configuration *project.Project) {
			configuration.URLExternalDataStorage = mustParse(t, defaultsStranger)
		},
		"форма настроек динамического списка": func(configuration *project.Project) {
			configuration.DefaultDynamicListSettingsForm = mustParse(t, defaultsStranger)
		},
		"вспомогательная форма констант": func(configuration *project.Project) {
			configuration.AuxiliaryConstantsForm = mustParse(t, defaultsStranger)
		},
		"форма изменений истории данных": func(configuration *project.Project) {
			configuration.DataHistoryChangesForm = mustParse(t, defaultsStranger)
		},
		"форма версии истории данных": func(configuration *project.Project) {
			configuration.DataHistoryVersionForm = mustParse(t, defaultsStranger)
		},
		"форма различий версий": func(configuration *project.Project) {
			configuration.DataHistoryVersionDifferenceForm = mustParse(t, defaultsStranger)
		},
		"форма выбора пользователей системы взаимодействия": func(configuration *project.Project) {
			configuration.CollaborationSystemUsersChoiceForm = mustParse(t, defaultsStranger)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := defaultsProject(t)
			saveDefaults(t, root, fill)
			if _, err := Load(root); err == nil {
				t.Fatal("a reference pointing at nothing was accepted")
			}
		})
	}

	// All seven at once, each naming what is actually there.
	root := defaultsProject(t)
	saveDefaults(t, root, func(configuration *project.Project) {
		configuration.URLExternalDataStorage = mustParse(t, defaultsFormData)
		configuration.DefaultDynamicListSettingsForm = mustParse(t, defaultsReportForm)
		configuration.AuxiliaryConstantsForm = mustParse(t, defaultsConstantsForm)
		configuration.DataHistoryChangesForm = mustParse(t, defaultsSettingsForm)
		configuration.DataHistoryVersionForm = mustParse(t, defaultsVariantForm)
		configuration.DataHistoryVersionDifferenceForm = mustParse(t, defaultsSearchForm)
		configuration.CollaborationSystemUsersChoiceForm = mustParse(t, defaultsReportForm)
	})
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Project.URLExternalDataStorage == nil || catalog.Project.CollaborationSystemUsersChoiceForm == nil {
		t.Fatalf("a reference was lost: %+v", catalog.Project)
	}
}

// The logo, the splash and the picture of the main section are the root's own
// files, so nothing declares them: the folder is the declaration. The rules
// are those of every picture - one file per density, or the files a
// description beside them names.
func TestRootPicturesAreCheckedLikeEveryPicture(t *testing.T) {
	t.Parallel()
	write := func(t *testing.T, root, picture, file string) {
		t.Helper()
		writeText(t, root, picture, file, "image")
	}

	// A configuration with no pictures at all is ordinary.
	if _, err := Load(defaultsProject(t)); err != nil {
		t.Fatalf("a configuration without a logo was refused: %v", err)
	}

	good := defaultsProject(t)
	write(t, good, project.LogoDirectory, "100.png")
	write(t, good, project.LogoDirectory, "200.png")
	write(t, good, project.SplashDirectory, "100.svg")
	if _, err := Load(good); err != nil {
		t.Fatalf("a logo at two densities was refused: %v", err)
	}

	// A picture of the root without the base density is saved by the
	// prototype as any picture is. Defect caught: it was refused.
	noBase := defaultsProject(t)
	write(t, noBase, project.LogoDirectory, "200.png")
	if _, err := Load(noBase); err != nil {
		t.Fatalf("a logo without the base density was refused: %v", err)
	}

	// The picture of the main section is the third picture of the root, and
	// in the configurations being moved it is drawn from variants (demo-base
	// keeps it as a set of them). Defect caught: the model had no place for
	// it at all, so it was lost at import.
	variants := defaultsProject(t)
	for _, file := range []string{"100.png", "picture.png"} {
		write(t, variants, project.MainSectionPictureDirectory, file)
	}
	writeText(t, variants, project.MainSectionPictureDirectory, project.RootPictureDescriptionFile, `load_transparent: true
transparent_pixel: {x: 1, y: 2}
variants:
  - {file: 100.png, density: 100, glyph_width: 16, glyph_height: 16}
  - {file: Picture.png, density: 100, interface: "8.2"}
`)
	if _, err := Load(variants); err != nil {
		t.Fatalf("the picture of the main section with variants was refused: %v", err)
	}
	images, err := ReadRootPictureImages(variants, project.MainSectionPictureDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if !images.LoadTransparent || images.TransparentPixel == nil || len(images.Variants) != 2 || images.Variants[1].Interface != Version82PictureInterface {
		t.Fatalf("the description of the picture was lost: %+v", images)
	}
	if _, err := ReadRootPictureImages(variants, "Обои"); err == nil {
		t.Fatal("a folder that is not a picture of the root was read as one")
	}

	for name, broken := range map[string]func(t *testing.T, root string){
		// The description is checked like a common picture's.
		"описание не того вида": func(t *testing.T, root string) {
			write(t, root, project.MainSectionPictureDirectory, "Picture.png")
			writeText(t, root, project.MainSectionPictureDirectory, project.RootPictureDescriptionFile, "variants: [{file: Picture.png, density: 110}]\n")
		},
		"неизвестное свойство описания": func(t *testing.T, root string) {
			write(t, root, project.MainSectionPictureDirectory, "100.png")
			writeText(t, root, project.MainSectionPictureDirectory, project.RootPictureDescriptionFile, "variant: []\n")
		},
		"вариант без файла": func(t *testing.T, root string) {
			write(t, root, project.MainSectionPictureDirectory, "100.png")
			writeText(t, root, project.MainSectionPictureDirectory, project.RootPictureDescriptionFile, "variants: [{file: 200.png, density: 200}]\n")
		},
		"имя не плотность": func(t *testing.T, root string) {
			write(t, root, project.SplashDirectory, "logo.png")
		},
		"формат, который нечем нарисовать": func(t *testing.T, root string) {
			write(t, root, project.LogoDirectory, "100.psd")
		},
		"одна плотность дважды": func(t *testing.T, root string) {
			write(t, root, project.LogoDirectory, "100.png")
			write(t, root, project.LogoDirectory, "100.svg")
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := defaultsProject(t)
			broken(t, root)
			if _, err := Load(root); err == nil {
				t.Fatal("a broken picture of the root was accepted")
			}
		})
	}
}

// The standalone application is made of objects of this configuration, and the
// roles narrowing it are its roles. ML builds no mobile application, and the
// references are resolved anyway: an object named here and deleted there is a
// mistake in the configuration whoever reads the list afterwards.
func TestStandaloneConfigurationIsResolved(t *testing.T) {
	t.Parallel()
	root := defaultsProject(t)
	saveDefaults(t, root, func(configuration *project.Project) {
		configuration.StandaloneConfigurationRestrictionRoles = []uuid.UUID{*mustParse(t, defaultsRole)}
		configuration.UsedMobileFunctionalities = []string{"Геолокация"}
	})
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Project.StandaloneConfigurationRestrictionRoles) != 1 ||
		len(catalog.Project.UsedMobileFunctionalities) != 1 {
		t.Fatalf("what the root says about the mobile application was lost: %+v", catalog.Project)
	}

	for name, fill := range map[string]func(configuration *project.Project){
		"роль ограничения": func(configuration *project.Project) {
			configuration.StandaloneConfigurationRestrictionRoles = []uuid.UUID{*mustParse(t, defaultsStranger)}
		},
		"объект состава": func(configuration *project.Project) {
			configuration.StandaloneConfigurationContent = []project.ObjectReference{
				{Kind: "catalogs", Object: *mustParse(t, defaultsStranger)},
			}
		},
		"вид, которого нет": func(configuration *project.Project) {
			configuration.StandaloneConfigurationContent = []project.ObjectReference{
				{Kind: "телефоны", Object: *mustParse(t, defaultsRole)},
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			broken := defaultsProject(t)
			saveDefaults(t, broken, fill)
			if _, err := Load(broken); err == nil {
				t.Fatal("a standalone application made of nothing was accepted")
			}
		})
	}
}

// writeText writes one file into a folder of the project root.
func writeText(t *testing.T, root, folder, file, content string) {
	t.Helper()
	directory := filepath.Join(root, folder)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
