package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// formWindowWhole sets every property of the window of a form away from its
// default.
const formWindowWhole = formAttrHead + `window_opening_mode: lock-whole-interface
no_auto_title: true
hide_title: true
hide_close_button: true
command_bar_location: none
enter_key_behavior: default-button
disabled: true
conversations: dont-show
no_auto_url: true
no_auto_fill_check: true
auto_save_data_in_settings: true
save_data_in_settings: true
not_customizable: true
no_save_window_settings: true
`

// The window of a form keeps every property it was given, and comes back the
// same written out as YAML and carried through the Studio as JSON.
//
// Defect caught: a property of the window read into the wrong field or into
// none - the description refuses unknown properties, so one left out of the
// model refuses the form, and one mapped wrong loses its value without a
// word; and a property lost on the way through the Studio, which writes the
// form back. Every field of the window is checked to be set, so a property
// added to the model and not to this test fails it.
func TestTheWindowOfAFormKeepsEveryProperty(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formWindowWhole), configuration)
	if err != nil {
		t.Fatal(err)
	}
	want := FormWindow{
		WindowOpeningMode: FormWindowLockWholeInterface, NoAutoTitle: true, HideTitle: true, HideCloseButton: true,
		CommandBarLocation: FormCommandBarNone, EnterKeyBehavior: FormEnterDefaultButton, Disabled: true,
		Conversations: FormConversationsDontShow, NoAutoURL: true, NoAutoFillCheck: true,
		AutoSaveDataInSettings: true, SaveDataInSettings: true, NotCustomizable: true, NoSaveWindowSettings: true,
	}
	if form.FormWindow != want {
		t.Fatalf("window = %+v, want %+v", form.FormWindow, want)
	}
	value := reflect.ValueOf(want)
	for index := range value.NumField() {
		if value.Field(index).IsZero() {
			t.Errorf("%s is not set by the test", value.Type().Field(index).Name)
		}
	}
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil {
		t.Fatalf("a form written back is refused: %v\n%s", err, written)
	}
	if again.FormWindow != want {
		t.Fatalf("written back: %+v", again.FormWindow)
	}
	carried, err := json.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	var received ManagedForm
	if err := json.Unmarshal(carried, &received); err != nil {
		t.Fatal(err)
	}
	if err := ValidateManagedForm("studio", received, configuration); err != nil {
		t.Fatalf("a form carried through the Studio is refused: %v", err)
	}
	if received.FormWindow != want {
		t.Fatalf("carried through the Studio: %+v", received.FormWindow)
	}
}

// A form that says nothing of its window has the defaults of the prototype,
// and a written default is accepted as it is.
//
// Defect caught: a default kept the other way round - a form that does not
// write ShowTitle shown without its title, the 5000 forms that do not write
// WindowOpeningMode opened as if they locked their owner.
func TestTheWindowOfAFormDefaultsAsThePrototype(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrHead), configuration)
	if err != nil {
		t.Fatal(err)
	}
	if form.FormWindow != (FormWindow{}) {
		t.Fatalf("window = %+v", form.FormWindow)
	}
	source := formAttrHead + "window_opening_mode: independent\ncommand_bar_location: auto\nenter_key_behavior: control-navigation\nconversations: auto\n"
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration); err != nil {
		t.Fatalf("a written default is refused: %v", err)
	}
}

// A value of the window that is none of the help's is refused by name.
//
// Defect caught: a value the form cannot run accepted, so that the form opens
// with whatever the runtime falls back to; and a refusal that does not say
// which property.
func TestTheWindowOfAFormRefusesWhatIsNoneOfItsValues(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for property, want := range map[string]string{
		"window_opening_mode: modal":    "window_opening_mode must be independent, lock-owner-window or lock-whole-interface",
		"command_bar_location: left":    "command_bar_location must be auto, top, bottom or none",
		"enter_key_behavior: tab":       "enter_key_behavior must be control-navigation or default-button",
		"conversations: always":         "conversations must be auto, show or dont-show",
		"show_title: false":             "show_title",
		"customizable: false":           "customizable",
		"auto_save_data_in_settings: 1": "line 6: cannot unmarshal",
	} {
		t.Run(property, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrHead+property+"\n"), configuration)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("err = %v, want %q", err, want)
			}
		})
	}
}

// formLayoutWhole sets every property of the layout of a form away from its
// default.
const formLayoutWhole = formAttrHead + `vertical_scroll: use-if-necessary
width: 400
height: 150
children_group: horizontal-if-possible
children_width: left-narrowest
horizontal_align: center
vertical_align: bottom
items_and_titles_align: titles-left-data-auto
vertical_spacing: one-and-half
horizontal_spacing: half
scale_variant: compact
collapse_by_importance: dont-use
scale: 90
`

// The layout of a form keeps every property it was given, and comes back the
// same written out as YAML and carried through the Studio as JSON - the value
// of the alignment the help does not name included.
//
// Defect caught: a property of the layout read into the wrong field or into
// none, or lost on the way through the Studio; TitlesLeftDataAuto refused, so
// that the form of sb that writes it is not moved. Every field is checked to
// be set, so a property added to the model and not to this test fails it.
func TestTheLayoutOfAFormKeepsEveryProperty(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formLayoutWhole), configuration)
	if err != nil {
		t.Fatal(err)
	}
	want := FormLayout{
		VerticalScroll: FormScrollUseIfNecessary, Width: 400, Height: 150, ChildrenGroup: ChildrenHorizontalIfPossible,
		ChildrenWidth: ChildrenWidthLeftNarrowest, HorizontalAlign: ItemHorizontalCenter, VerticalAlign: ItemVerticalBottom,
		ItemsAndTitlesAlign: ItemsAndTitlesTitlesLeftDataAuto, VerticalSpacing: ItemSpacingOneAndHalf,
		HorizontalSpacing: ItemSpacingHalf, ScaleVariant: FormScaleCompact, CollapseByImportance: CollapseByImportanceDontUse, Scale: 90,
	}
	if form.FormLayout != want {
		t.Fatalf("layout = %+v, want %+v", form.FormLayout, want)
	}
	value := reflect.ValueOf(want)
	for index := range value.NumField() {
		if value.Field(index).IsZero() {
			t.Errorf("%s is not set by the test", value.Type().Field(index).Name)
		}
	}
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil {
		t.Fatalf("a form written back is refused: %v\n%s", err, written)
	}
	if again.FormLayout != want {
		t.Fatalf("written back: %+v", again.FormLayout)
	}
	carried, err := json.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	var received ManagedForm
	if err := json.Unmarshal(carried, &received); err != nil {
		t.Fatal(err)
	}
	if err := ValidateManagedForm("studio", received, configuration); err != nil {
		t.Fatalf("a form carried through the Studio is refused: %v", err)
	}
	if received.FormLayout != want {
		t.Fatalf("carried through the Studio: %+v", received.FormLayout)
	}
}

// A form that says nothing of its layout has the defaults of the prototype,
// a written default is accepted, and a size of no limit stands.
//
// Defect caught: a default kept the other way round, a written Auto refused,
// and a limit on the size of a form that the prototype does not have.
func TestTheLayoutOfAFormDefaultsAsThePrototype(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrHead), configuration)
	if err != nil {
		t.Fatal(err)
	}
	if form.FormLayout != (FormLayout{}) {
		t.Fatalf("layout = %+v", form.FormLayout)
	}
	source := formAttrHead + "vertical_scroll: auto\nwidth: 100000\nheight: 0\nchildren_group: vertical\nchildren_width: auto\n" +
		"horizontal_align: auto\nvertical_align: auto\nitems_and_titles_align: auto\nvertical_spacing: auto\nhorizontal_spacing: auto\n" +
		"scale_variant: auto\ncollapse_by_importance: auto\n"
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration); err != nil {
		t.Fatalf("a written default is refused: %v", err)
	}
}

// A value of the layout that is none of the help's, and a negative size, are
// refused by name.
//
// Defect caught: a value the form cannot lay out accepted; a refusal that does
// not say which property; the two spacings checked as one.
func TestTheLayoutOfAFormRefusesWhatIsNoneOfItsValues(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for property, want := range map[string]string{
		"vertical_scroll: always":           "vertical_scroll must be auto, use, use-without-stretch or use-if-necessary",
		"width: -1":                         "width must not be negative",
		"height: -1":                        "height must not be negative",
		"children_group: grid":              "children_group must be vertical, horizontal, always-horizontal or horizontal-if-possible",
		"children_width: right-wide":        "children_width must be auto, equal, left-narrowest, left-narrow, left-wide or left-widest",
		"horizontal_align: justify":         "horizontal_align must be auto, left, center or right",
		"vertical_align: middle":            "vertical_align must be auto, top, center or bottom",
		"items_and_titles_align: titles-up": "items_and_titles_align must be auto, none, items-left-titles-left",
		"vertical_spacing: triple":          "vertical_spacing must be auto, none, half, single, one-and-half or double",
		"horizontal_spacing: triple":        "horizontal_spacing must be auto, none, half, single, one-and-half or double",
		"scale_variant: large":              "scale_variant must be auto, normal or compact",
		"collapse_by_importance: always":    "collapse_by_importance must be auto, use or dont-use",
		"scale: -1":                         "scale must not be negative",
	} {
		t.Run(property, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrHead+property+"\n"), configuration)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("err = %v, want %q", err, want)
			}
		})
	}
}

// formExtensionOf is a form whose main attribute is of the given type, with
// the given properties of its extension.
func formExtensionOf(mainType, properties string) string {
	return formAttrHead + properties + "attributes:\n  - {id: " + formAttrObject + ", name: Объект, main: true, types: [" + mainType + "]}\n" +
		"  - {id: " + formAttrList + ", name: Результат, types: [{kind: spreadsheet-document}]}\n"
}

// What a form has by its main attribute is kept for each kind - a catalog, a
// document, a report, a dynamic list - and comes back the same through YAML
// and the Studio; a number written for the attribute of the result, as the
// prototype writes one, is carried as it is.
//
// Defect caught: a property of an extension read into the wrong field or into
// none, or lost on the way through the Studio; the prototype's code of an
// attribute refused, so that the three reports of erp and sb that write one
// are not moved.
func TestAFormKeepsWhatItHasByItsMainAttribute(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	storage, err := uuid.Parse(formAttrRole)
	if err != nil {
		t.Fatal(err)
	}
	for name, test := range map[string]struct {
		source string
		want   FormExtension
	}{
		"справочник": {formExtensionOf("{kind: catalog-object, reference: "+formAttrCatalog+"}", "folders_and_items: folders\n"),
			FormExtension{FoldersAndItems: FormFolders}},
		"план видов характеристик": {formExtensionOf("{kind: chart-of-characteristic-types-object, reference: "+formAttrCatalog+"}", "folders_and_items: items\n"),
			FormExtension{FoldersAndItems: FormItems}},
		"документ": {formExtensionOf("{kind: document-object, reference: "+formAttrCatalog+"}", "auto_time: current-or-last\nposting_mode: regular\nno_repost_on_write: true\n"),
			FormExtension{AutoTime: FormAutoTimeCurrentOrLast, PostingMode: FormPostingRegular, NoRepostOnWrite: true}},
		"отчёт": {formExtensionOf("{kind: report-object, reference: "+formAttrCatalog+"}", "report_form_type: variant\nauto_show_state: show-on-composition\n"+
			"result_view_mode: compact\nview_mode_on_set_result: dont-apply\nreport_result: результат\ndetails_data: \"4\"\n"+
			"variant_appearance: результат\ncustom_settings_folder: \"3:02023637-7868-4a5f-8576-835a76e0c9ba\"\n"),
			FormExtension{ReportFormType: ReportFormVariant, AutoShowState: ReportShowStateOnComposition, ResultViewMode: ReportResultViewCompact,
				ViewModeOnSetResult: ReportViewModeOnSetDontApply, ReportResult: "результат", DetailsData: "4",
				VariantAppearance: "результат", CustomSettingsFolder: "3:02023637-7868-4a5f-8576-835a76e0c9ba"}},
		"любой отчёт": {formExtensionOf("{kind: report-object}", "report_form_type: variant\nauto_show_state: show-on-composition\n"+
			"result_view_mode: compact\nview_mode_on_set_result: dont-apply\nreport_result: результат\ndetails_data: \"4\"\n"+
			"variant_appearance: результат\ncustom_settings_folder: \"3:02023637-7868-4a5f-8576-835a76e0c9ba\"\n"),
			FormExtension{ReportFormType: ReportFormVariant, AutoShowState: ReportShowStateOnComposition, ResultViewMode: ReportResultViewCompact,
				ViewModeOnSetResult: ReportViewModeOnSetDontApply, ReportResult: "результат", DetailsData: "4",
				VariantAppearance: "результат", CustomSettingsFolder: "3:02023637-7868-4a5f-8576-835a76e0c9ba"}},
		"динамический список": {formExtensionOf("{kind: dynamic-list}", "group_list: \"2:02023637-7868-4a5f-8576-835a76e0c9ba\"\n"),
			FormExtension{GroupList: "2:02023637-7868-4a5f-8576-835a76e0c9ba"}},
		"компоновщик настроек": {formExtensionOf("{kind: settings-composer}", "custom_settings_folder: \"3:02023637-7868-4a5f-8576-835a76e0c9ba\"\n"),
			FormExtension{CustomSettingsFolder: "3:02023637-7868-4a5f-8576-835a76e0c9ba"}},
		"хранилище настроек у обработки": {formExtensionOf("{kind: data-processor-object, reference: "+formAttrCatalog+"}", "settings_storage: "+formAttrRole+"\n"),
			FormExtension{SettingsStorage: &storage}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			form, err := DecodeManagedForm("form.yaml", strings.NewReader(test.source), configuration)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(form.FormExtension, test.want) {
				t.Fatalf("extension = %+v, want %+v", form.FormExtension, test.want)
			}
			written, err := yaml.Marshal(form)
			if err != nil {
				t.Fatal(err)
			}
			again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
			if err != nil || !reflect.DeepEqual(again.FormExtension, test.want) {
				t.Fatalf("written back: %+v, %v\n%s", again.FormExtension, err, written)
			}
			carried, err := json.Marshal(form)
			if err != nil {
				t.Fatal(err)
			}
			var received ManagedForm
			if err := json.Unmarshal(carried, &received); err != nil {
				t.Fatal(err)
			}
			if err := ValidateManagedForm("studio", received, configuration); err != nil || !reflect.DeepEqual(received.FormExtension, test.want) {
				t.Fatalf("carried through the Studio: %+v, %v", received.FormExtension, err)
			}
		})
	}
	// Every property of the extension is given by one of the cases above.
	var all FormExtension
	for _, set := range []FormExtension{{FoldersAndItems: FormFolders}, {AutoTime: FormAutoTimeFirst, PostingMode: FormPostingAuto, NoRepostOnWrite: true},
		{ReportFormType: ReportFormMain, AutoShowState: ReportShowState, ResultViewMode: ReportResultViewAuto, ViewModeOnSetResult: ReportViewModeOnSetAuto,
			ReportResult: "a", DetailsData: "a", VariantAppearance: "a", CustomSettingsFolder: "a"}, {GroupList: "a"}, {SettingsStorage: &storage}} {
		merged := reflect.ValueOf(&all).Elem()
		value := reflect.ValueOf(set)
		for index := range value.NumField() {
			if !value.Field(index).IsZero() {
				merged.Field(index).Set(value.Field(index))
			}
		}
	}
	value := reflect.ValueOf(all)
	for index := range value.NumField() {
		if value.Field(index).IsZero() {
			t.Errorf("%s is given by no case", value.Type().Field(index).Name)
		}
	}
}

// A property of an extension stands only with a main attribute of its kind,
// a value is one of the help's, and the attribute of the result is one the
// form has.
//
// Defect caught: a property of a document accepted on a form of a catalog, or
// one of a report on a form with no main attribute, so that the form carries
// what nothing will ever run; a value the form cannot run; a result kept in an
// attribute the form does not have.
func TestAFormRefusesWhatItCannotHaveByItsMainAttribute(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	catalog := "{kind: catalog-object, reference: " + formAttrCatalog + "}"
	report := "{kind: report-object, reference: " + formAttrCatalog + "}"
	for name, test := range map[string]struct{ main, properties, want string }{
		"группы не у справочника":       {report, "folders_and_items: items\n", "folders_and_items belongs to a form whose main attribute is catalog-object or chart-of-characteristic-types-object"},
		"время не у документа":          {catalog, "auto_time: last\n", "auto_time belongs to a form whose main attribute is document-object"},
		"проведение не у документа":     {catalog, "posting_mode: auto\n", "posting_mode belongs to a form whose main attribute is document-object"},
		"перепроведение не у документа": {catalog, "no_repost_on_write: true\n", "no_repost_on_write belongs to a form whose main attribute is document-object"},
		"тип отчёта не у отчёта":        {catalog, "report_form_type: main\n", "report_form_type belongs to a form whose main attribute is report-object"},
		"состояние не у отчёта":         {catalog, "auto_show_state: auto\n", "auto_show_state belongs"},
		"режим результата не у отчёта":  {catalog, "result_view_mode: auto\n", "result_view_mode belongs"},
		"применение не у отчёта":        {catalog, "view_mode_on_set_result: auto\n", "view_mode_on_set_result belongs"},
		"результат не у отчёта":         {catalog, "report_result: Результат\n", "report_result belongs"},
		"расшифровка не у отчёта":       {catalog, "details_data: Результат\n", "details_data belongs"},
		"вариант не у отчёта":           {catalog, "variant_appearance: Поле\n", "variant_appearance belongs"},
		"настройки не у отчёта":         {catalog, "custom_settings_folder: Группа\n", "custom_settings_folder belongs to a form whose main attribute is report-object or settings-composer"},
		"настройки у списка":            {"{kind: dynamic-list}", "custom_settings_folder: Группа\n", "custom_settings_folder belongs to a form whose main attribute is report-object or settings-composer"},
		"список групп не у списка":      {catalog, "group_list: Дерево\n", "group_list belongs to a form whose main attribute is dynamic-list"},
		"группы и элементы":             {catalog, "folders_and_items: all\n", "folders_and_items must be folders-and-items, folders or items"},
		"время":                         {"{kind: document-object, reference: " + formAttrCatalog + "}", "auto_time: now\n", "auto_time must be dont-use, first, last, current-or-first or current-or-last"},
		"тип формы отчёта":              {report, "report_form_type: print\n", "report_form_type must be main, settings or variant"},
		"результат без реквизита":       {report, "report_result: Отчёт\n", "report_result names no attribute of the form"},
		"расшифровка без реквизита":     {report, "details_data: Расшифровка\n", "details_data names no attribute of the form"},
		"вариант без реквизита":         {report, "variant_appearance: Поле\n", "variant_appearance names no attribute of the form"},
		"пробелы в варианте":            {report, "variant_appearance: \" Поле\"\n", "variant_appearance must be written without surrounding spaces"},
		"хранилище без ссылки":          {catalog, "settings_storage: 00000000-0000-0000-0000-000000000000\n", "settings_storage must be a non-zero UUID"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(formExtensionOf(test.main, test.properties)), configuration)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
	// With no main attribute a form has no extension.
	_, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrHead+"auto_time: last\n"), configuration)
	if err == nil || !strings.Contains(err.Error(), "auto_time belongs") {
		t.Fatalf("err = %v", err)
	}
}

// The settings storage a form of an object or a common form names is resolved
// with the project: one that is gone is a reference to nothing, named with
// the form.
//
// Defect caught: a form keeping its settings in a storage the project does not
// have, loading clean.
func TestTheSettingsStorageOfAFormIsResolved(t *testing.T) {
	t.Parallel()
	root := formReferencesProject(t)
	path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	withStorage := strings.Replace(string(content), "kind: object\n", "kind: object\nsettings_storage: "+cmpSettingsStorage+"\n", 1)
	writeFile(t, path, withStorage)
	if _, err := Load(root); err != nil {
		t.Fatalf("a form naming a storage of the project: %v", err)
	}
	writeFile(t, path, strings.Replace(withStorage, cmpSettingsStorage, refGone, 1))
	found := unresolvedOf(t, root)
	if !containsWhere(found, "catalog Номенклатура form ФормаЭлемента settings storage") {
		t.Fatalf("unresolved = %+v", found)
	}
	// A common form is resolved the same way.
	root = formReferencesProject(t)
	writeCommonForm(t, root, "НастройкиОтчетов", "format: 1\nid: c0de0000-0000-4000-8000-000000000160\nname: НастройкиОтчетов\n"+
		"title: {ru: Н}\nkind: common\nsettings_storage: "+refGone+"\n")
	if found := unresolvedOf(t, root); !containsWhere(found, "common form НастройкиОтчетов settings storage") {
		t.Fatalf("unresolved = %+v", found)
	}
}

// The standard commands a form takes out of itself are kept by name, in
// order, through YAML and the Studio; a name that is no name, and one named
// twice, are refused.
//
// Defect caught: the prototype's CommandSet of a form (3056 forms of the
// exports) lost on the way, so that a form shows the commands it hid; a
// misspelt or repeated name taken without a word.
func TestAFormKeepsTheStandardCommandsItTakesOut(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrHead+"excluded_commands: [SaveValues, RestoreValues, Retry]\n"), configuration)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"SaveValues", "RestoreValues", "Retry"}
	if !reflect.DeepEqual(form.ExcludedCommands, want) {
		t.Fatalf("excluded commands = %v", form.ExcludedCommands)
	}
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil || !reflect.DeepEqual(again.ExcludedCommands, want) {
		t.Fatalf("written back: %v, %v", again.ExcludedCommands, err)
	}
	carried, err := json.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	var received ManagedForm
	if err := json.Unmarshal(carried, &received); err != nil {
		t.Fatal(err)
	}
	if err := ValidateManagedForm("studio", received, configuration); err != nil || !reflect.DeepEqual(received.ExcludedCommands, want) {
		t.Fatalf("carried through the Studio: %v, %v", received.ExcludedCommands, err)
	}
	for list, refusal := range map[string]string{
		"[\"Сохранить значения\"]":        "excluded_commands[0] must be the name of a command",
		"[SaveValues, Retry, SaveValues]": "excluded_commands[2] names SaveValues twice",
	} {
		_, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrHead+"excluded_commands: "+list+"\n"), configuration)
		if err == nil || !strings.Contains(err.Error(), refusal) {
			t.Errorf("%s: err = %v, want %q", list, err, refusal)
		}
	}
}
