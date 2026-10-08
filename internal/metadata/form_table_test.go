package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/k33alexey/MetaLab/internal/project"
)

// A table keeps what it has of its own, and what it shares with a field,
// through YAML and the Studio: how it is drawn and selects, its header,
// footer and lines, how rows are put in, changed and moved, where a list
// and a tree open, the picture of its rows, where its command bar and its
// additions stand, its height in rows, and of a field its size, title,
// colours, scroll bars, dragging, output, the commands taken out of it, the
// mark of an empty table and what its current row does on a phone.
//
// Defect caught: a table drawn as a tree (1528 times) or selecting single
// rows (660) refused; its header hidden (1726), its lines hidden, rows that
// may not be added or moved (1545, 1531), the search string put nowhere
// (2499) or into the command bar (678) lost; a footer of 0 rows read as not
// said; the files dragged onto a table (8502), its scroll bars, its height
// governed in rows of the table, or the commands taken out of it (2963)
// refused as properties of other fields.
func TestATableKeepsWhatItHasOfItsOwn(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Список, kind: table, data_path: Список, representation: tree," +
		" selection_mode: single-row, row_selection_mode: row, no_header: true, footer: true, header_height: 2, footer_height: 0," +
		" no_horizontal_lines: true, no_vertical_lines: true, use_alternation_row_color: true, auto_insert_new_row: true," +
		" no_change_row_set: true, no_change_row_order: true, auto_add_incomplete: false, row_input_mode: after-current-row," +
		" choice_mode: true, multiple_choice: true, initial_list_view: end, initial_tree_view: expand-top-level," +
		" command_bar_location: none, search_string_location: none, view_status_location: top, search_control_location: command-bar," +
		" search_on_input: dont-use, height_in_table_rows: 5, no_auto_max_rows_count: true, max_rows_count: 8," +
		" rows_picture: {standard: Change}, row_picture_data_path: Список.ВидПиктограммы, refresh_request: pull-from-top," +
		" behavior_on_horizontal_compression: move-items-by-importance," +
		" width: 60, height: 7, no_auto_max_width: true, max_width: 80, vertical_stretch: false, group_horizontal_align: right," +
		" title_location: top, skip_on_input: true, default_item: true, shortcut: Cmd+1, title_height: 2," +
		" border_color: {source: style, from: {standard: BorderColor}}, title_font: {source: auto}, text_color: {source: auto}," +
		" horizontal_scroll_bar: dont-use, vertical_scroll_bar: use-always, enable_drag: true, enable_start_drag: true, output: disable," +
		" excluded_commands: [Add, Copy], file_drag_mode: as-file, height_control_variant: use-height-in-table-rows," +
		" auto_mark_incomplete: true, current_row_use: selection-presentation-and-choice, display_importance: very-high, read_only: true}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Товары, kind: table, header_height: 3}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check := func(source string, items []ManagedFormElement) {
		t.Helper()
		table := items[0]
		switch {
		case table.Representation != "tree" || table.SelectionMode != "single-row" || table.RowSelectionMode != FormRowSelectionRow:
			t.Fatalf("%s: how it is drawn: %+v", source, table)
		case !table.NoHeader || !table.Footer || table.HeaderHeight == nil || *table.HeaderHeight != 2 || table.FooterHeight == nil || *table.FooterHeight != 0:
			t.Fatalf("%s: header and footer: %+v", source, table.TableProperties)
		case items[1].FooterHeight != nil || items[1].HeaderHeight == nil || *items[1].HeaderHeight != 3:
			t.Fatalf("%s: a footer not said: %+v", source, items[1].TableProperties)
		case !table.NoHorizontalLines || !table.NoVerticalLines || !table.UseAlternationRowColor:
			t.Fatalf("%s: lines: %+v", source, table.TableProperties)
		case !table.AutoInsertNewRow || !table.NoChangeRowSet || !table.NoChangeRowOrder || table.AutoAddIncomplete == nil || *table.AutoAddIncomplete ||
			table.RowInputMode != FormRowInputAfterCurrentRow:
			t.Fatalf("%s: rows: %+v", source, table.TableProperties)
		case !table.ChoiceMode || !table.MultipleChoice || table.InitialListView != FormInitialListEnd || table.InitialTreeView != FormInitialTreeExpandTopLevel:
			t.Fatalf("%s: choice and view: %+v", source, table.TableProperties)
		case table.CommandBarLocation != FormTableLocationNone || table.SearchStringLocation != FormTableLocationNone ||
			table.ViewStatusLocation != FormTableLocationTop || table.SearchControlLocation != FormTableLocationCommandBar ||
			table.SearchOnInput != FormSearchOnInputDontUse:
			t.Fatalf("%s: locations: %+v", source, table.TableProperties)
		case table.HeightInTableRows != 5 || !table.NoAutoMaxRowsCount || table.MaxRowsCount != 8 || table.RowsPicture == nil ||
			table.RowsPicture.Standard != "Change" || table.RowPictureDataPath != "Список.ВидПиктограммы":
			t.Fatalf("%s: height and pictures: %+v", source, table.TableProperties)
		case table.RefreshRequest != FormRefreshRequestPullFromTop || table.BehaviorOnHorizontalCompression != FormHorizontalCompressionMoveItemsByImportance:
			t.Fatalf("%s: the mobile client: %+v", source, table.TableProperties)
		case table.Width != 60 || table.Height != 7 || !table.NoAutoMaxWidth || table.MaxWidth != 80 || table.VerticalStretch == nil ||
			*table.VerticalStretch || table.GroupHorizontalAlign != ItemHorizontalRight:
			t.Fatalf("%s: size: %+v", source, table.FieldLayout)
		case table.TitleLocation != FormTitleTop || table.SkipOnInput == nil || !*table.SkipOnInput || !table.DefaultItem || table.Shortcut != "Cmd+1":
			t.Fatalf("%s: title and input: %+v", source, table.FieldBehavior)
		case table.TitleHeight != 2 || table.BorderColor == nil || table.TitleFont == nil || table.TextColor == nil:
			t.Fatalf("%s: look: %+v", source, table.FieldLook)
		case table.HorizontalScrollBar != FormScrollBarDontUse || table.VerticalScrollBar != FormScrollBarUseAlways || table.EnableDrag == nil ||
			table.EnableStartDrag == nil || table.Output != FormUseOutputDisable || !slices.Equal(table.ExcludedCommands, []string{"Add", "Copy"}):
			t.Fatalf("%s: what it shares with a document: %+v", source, table.FieldDocument)
		case table.FileDragMode != FormFileDragAsFile || table.HeightControlVariant != FormHeightControlUseHeightInTableRows ||
			table.AutoMarkIncomplete == nil || !*table.AutoMarkIncomplete || table.CurrentRowUse != FormCurrentRowSelectionPresentationAndChoice:
			t.Fatalf("%s: what it shares with other fields: %+v", source, table)
		case table.DisplayImportance != FormDisplayImportanceVeryHigh || !table.ReadOnly:
			t.Fatalf("%s: what every element has: %+v", source, table)
		}
	}
	check("read", form.Items)
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check("written back", again.Items)
	carried, err := json.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	var received ManagedForm
	if err := json.Unmarshal(carried, &received); err != nil {
		t.Fatal(err)
	}
	if err := ValidateManagedForm("studio", received, configuration); err != nil {
		t.Fatal(err)
	}
	check("carried through the Studio", received.Items)
	if !reflect.DeepEqual(received.Items, form.Items) || !reflect.DeepEqual(again.Items, form.Items) {
		t.Fatal("a table changed on the way")
	}
}

// Each property a table has of its own, set alone on every kind of element,
// stands only on a table. The properties are taken from the group itself,
// so that one added to it is checked here too.
//
// Defect caught: the header, the lines or the search string of a table let
// through on a field or a group, where nothing draws them; a property of a
// table refused on the table.
func TestEachPropertyOfATableStandsOnlyOnATable(t *testing.T) {
	t.Parallel()
	no, zero := false, 0
	full := TableProperties{RowSelectionMode: FormRowSelectionCell, NoHeader: true, Footer: true, HeaderHeight: &zero, FooterHeight: &zero,
		NoHorizontalLines: true, NoVerticalLines: true, UseAlternationRowColor: true, AutoInsertNewRow: true, NoChangeRowSet: true,
		NoChangeRowOrder: true, AutoAddIncomplete: &no, RowInputMode: FormRowInputEndOfList, ChoiceMode: true, MultipleChoice: true,
		InitialListView: FormInitialListBeginning, InitialTreeView: FormInitialTreeNoExpand, CommandBarLocation: FormTableLocationBottom,
		SearchStringLocation: FormTableLocationFormCaption, ViewStatusLocation: FormTableLocationAuto, SearchControlLocation: FormTableLocationNone,
		SearchOnInput: FormSearchOnInputUse, HeightInTableRows: 1, NoAutoMaxRowsCount: true, MaxRowsCount: 1,
		RowsPicture: &PictureReference{Standard: "Change"}, RowPictureDataPath: "Список.Картинка", RefreshRequest: FormRefreshRequestNone,
		BehaviorOnHorizontalCompression: FormHorizontalCompressionAuto, DynamicList: &TableDynamicList{Period: FormStandardPeriod{Variant: "custom"}},
		ViewMode: FormSettingsViewQuickAccess, NoNamedItemDetailedRepresentation: true, Autofill: true}
	value := reflect.ValueOf(full)
	for index := range value.NumField() {
		name := value.Type().Field(index).Name
		if value.Field(index).IsZero() {
			t.Fatalf("%s is not set by the test", name)
		}
		for kind := range formElementClasses {
			var alone TableProperties
			reflect.ValueOf(&alone).Elem().Field(index).Set(value.Field(index))
			issues := validateTableProperties("items[0]", alone, kind)
			refused := len(issues) == 1 && issues[0] == "items[0] has what only a table has"
			if want := kind != FormElementTable; want != refused || !want && len(issues) != 0 {
				t.Errorf("%s alone on %s: %v", name, kind, issues)
			}
		}
	}
}

// A table refuses what is wrong in what it has, naming the property, and a
// property of a table refuses a value the help gives another element.
//
// Defect caught: the prototype's spelling ("Tree", "SingleRow") taken as
// written; the search control put at the top, which the help does not let
// it; a negative height; the picture of rows named by a path with spaces;
// the height of a table governed in rows of the table on an input field;
// the current row of a group given a value of a table and the other way
// round; what only a picture field has on a table beside its dragging.
func TestATableRefusesWhatIsWrong(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for name, test := range map[string]struct{ element, want string }{
		"вид прототипа":           {"kind: table, representation: Tree", "items[0].representation must be list, hierarchical-list or tree"},
		"выделение прототипа":     {"kind: table, selection_mode: SingleRow", "items[0].selection_mode must be single-row or multi-row"},
		"выделение календаря":     {"kind: table, selection_mode: single", "items[0].selection_mode must be single-row or multi-row"},
		"выделение строки":        {"kind: table, row_selection_mode: line", "items[0].row_selection_mode must be cell or row"},
		"управление поиском":      {"kind: table, search_control_location: top", "items[0].search_control_location must be auto, none or command-bar"},
		"состояние в заголовке":   {"kind: table, view_status_location: form-caption", "items[0].view_status_location must be auto, none, top or bottom"},
		"панель в командной":      {"kind: table, command_bar_location: command-bar", "items[0].command_bar_location must be auto, none, top or bottom"},
		"строка поиска":           {"kind: table, search_string_location: Top", "items[0].search_string_location must be auto, none, top, bottom, command-bar, form-caption or pull-from-top"},
		"ввод строк":              {"kind: table, row_input_mode: EndOfList", "items[0].row_input_mode must be end-of-list, end-of-window, after-current-row or before-current-row"},
		"начало списка":           {"kind: table, initial_list_view: top", "items[0].initial_list_view must be auto, beginning or end"},
		"дерево":                  {"kind: table, initial_tree_view: expand", "items[0].initial_tree_view must be no-expand, expand-top-level or expand-all-levels"},
		"поиск при вводе":         {"kind: table, search_on_input: yes", "items[0].search_on_input must be auto, use or dont-use"},
		"обновление":              {"kind: table, refresh_request: auto", "items[0].refresh_request must be none, pull-from-bottom, pull-from-top or pull-from-top-or-bottom"},
		"сжатие":                  {"kind: table, behavior_on_horizontal_compression: hide", "items[0].behavior_on_horizontal_compression must be auto, hide-items-by-importance or move-items-by-importance"},
		"высота в строках":        {"kind: table, height_in_table_rows: -1", "items[0].height_in_table_rows must not be negative"},
		"предел строк":            {"kind: table, max_rows_count: -1", "items[0].max_rows_count must not be negative"},
		"высота шапки":            {"kind: table, header_height: -1", "items[0].header_height must not be negative"},
		"высота подвала":          {"kind: table, footer_height: -2", "items[0].footer_height must not be negative"},
		"путь картинки":           {"kind: table, row_picture_data_path: Список. Картинка", "items[0].row_picture_data_path must be names separated by dots"},
		"картинка двух":           {"kind: table, rows_picture: {standard: Change, file: RowsPicture.png}", "items[0].rows_picture"},
		"строки таблицы у поля":   {"kind: input-field, height_control_variant: use-height-in-table-rows", "items[0].height_control_variant must be auto, use-content-height or use-height-in-form-rows"},
		"строка таблицы у группы": {"kind: usual-group, current_row_use: choice", "items[0].current_row_use must be auto, use or dont-use"},
		"строка группы у таблицы": {"kind: table, current_row_use: use", "items[0].current_row_use must be auto, choice, selection-presentation or selection-presentation-and-choice"},
		"картинка поля":           {"kind: table, file_drag_mode: as-file, picture_size: stretch", "items[0].picture_size is allowed only for picture fields and pictures"},
		"текст поля":              {"kind: table, height_control_variant: auto, mask: '99'", "items[0] has the text input of an input field"},
		"шапка у поля":            {"kind: input-field, no_header: true", "items[0] has what only a table has"},
		"выравнивание колонки":    {"kind: table, horizontal_align: left", "items[0] has the size and alignment of a field"},
		"рамка":                   {"kind: table, border: {source: auto}", "items[0] has the look of a field"},
		"режим редактирования":    {"kind: table, edit_mode: directly", "items[0] has what only a field has"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			source := formElementsForm("  - {id: c0de0000-0000-4000-8000-000000990001, name: Список, " + test.element + "}\n")
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// The picture of the rows of a table is resolved as every picture of an
// element is: a common picture with the project, a file of its own in the
// folder of the element.
//
// Defect caught: the picture of rows left out of the pictures of an element,
// so that a common picture that is gone loads clean, a file of its own the
// form does not keep loads clean, and the folder keeping it is refused as
// holding a file no picture draws (25 tables draw their rows from a file of
// their own).
func TestThePictureOfRowsIsResolved(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		picture string
		files   []string
		refused string
		gone    bool
	}{
		"общая картинка":     {picture: "{common: " + cmpCommonPicture + "}"},
		"удалённая картинка": {picture: "{common: " + refGone + "}", gone: true},
		"свой файл":          {picture: "{file: RowsPicture.png}", files: []string{"Список/RowsPicture.png"}},
		"своего файла нет":   {picture: "{file: RowsPicture.png}", refused: "element Список rows_picture is shown with picture file RowsPicture.png, which its folder does not hold"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			element := "items:\n  - {id: c0de0000-0000-4000-8000-000000990002, name: Список, kind: table, rows_picture: " + test.picture + "}\n"
			writeFile(t, path, strings.Replace(string(content), "attributes:\n", element+"attributes:\n", 1))
			for _, file := range test.files {
				writeFile(t, filepath.Join(filepath.Dir(path), project.FormItemsDirectory, file), "image")
			}
			switch {
			case test.gone:
				if found := unresolvedOf(t, root); !containsWhere(found, "catalog Номенклатура form ФормаЭлемента element Список rows_picture") {
					t.Fatalf("unresolved = %+v", found)
				}
			case test.refused != "":
				if _, err := Load(root); err == nil || !strings.Contains(err.Error(), test.refused) {
					t.Fatalf("err = %v, want %q", err, test.refused)
				}
			default:
				if _, err := Load(root); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// tableOfAList writes a form whose table shows the dynamic list Список with
// what is given, beside a usual group and an input field to name.
func tableOfAList(list string) string {
	return formElementsForm("  - {id: c0de0000-0000-4000-8000-000000990001, name: Список, kind: table, data_path: Список, dynamic_list: {"+list+"}}\n"+
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: ГруппаНастроек, kind: usual-group}\n"+
		"  - {id: c0de0000-0000-4000-8000-000000990003, name: Период, kind: input-field}\n"+
		"  - {id: c0de0000-0000-4000-8000-000000990004, name: Товары, kind: table, data_path: Товары}\n") +
		"attributes:\n  - {id: c0de0000-0000-4000-8000-000000990011, name: Список, types: [{kind: dynamic-list}]}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990012, name: Товары, types: [{kind: value-table}]}\n"
}

// A table of a dynamic list keeps what it has besides through YAML and the
// Studio - its refresh, period, choice of folders and items, root, the row
// it restores, its refresh on a change of data, the link to its current row
// and the group of its user settings - and a table of the settings of a
// composition its view mode, the representation of its named items and its
// autofill.
//
// Defect caught: a list refreshed every minute (71 of them) read as not
// refreshed; the period of a list, a root shown (3143) or a group of the
// user settings (2206) lost; a list whose current row cannot be linked (8)
// read as linkable; a filter of quick access (72) shown whole.
func TestATableOfADynamicListKeepsWhatItHas(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	source := tableOfAList("auto_refresh: true, auto_refresh_period: 59, period: {variant: custom, start_date: \"2024-01-01T00:00:00\", end_date: \"2024-12-31T23:59:59\"}," +
		" choice_folders_and_items: folders-and-items, restore_current_row: true, show_root: true, allow_root_choice: true," +
		" top_level_parent: {kind: catalog, data: c0de0000-0000-4000-8000-000000990021, object: c0de0000-0000-4000-8000-000000990022}," +
		" update_on_data_change: dont-update, user_settings_group: ГруппаНастроек")
	source = strings.Replace(source, "name: Товары, kind: table, data_path: Товары}", "name: Товары, kind: table, data_path: Товары, view_mode: quick-access,"+
		" no_named_item_detailed_representation: true, autofill: true}", 1)
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check := func(source string, items []ManagedFormElement) {
		t.Helper()
		list, settings := items[0].DynamicList, items[3]
		switch {
		case list == nil:
			t.Fatalf("%s: the list is gone", source)
		case !list.AutoRefresh || list.AutoRefreshPeriod != 59 || list.Period.Variant != "custom" || list.Period.StartDate != "2024-01-01T00:00:00" ||
			list.Period.EndDate != "2024-12-31T23:59:59":
			t.Fatalf("%s: refresh and period: %+v", source, list)
		case list.ChoiceFoldersAndItems != FormFoldersAndItemsBoth || !list.RestoreCurrentRow || !list.ShowRoot || !list.AllowRootChoice ||
			list.UpdateOnDataChange != FormUpdateOnDataChangeDontUpdate || list.AllowGettingCurrentRowURL || list.UserSettingsGroup != "ГруппаНастроек":
			t.Fatalf("%s: the list: %+v", source, list)
		case list.TopLevelParent == nil || list.TopLevelParent.Kind != CatalogType || list.TopLevelParent.Data != "c0de0000-0000-4000-8000-000000990021":
			t.Fatalf("%s: the root: %+v", source, list.TopLevelParent)
		case settings.DynamicList != nil || settings.ViewMode != FormSettingsViewQuickAccess || !settings.NoNamedItemDetailedRepresentation || !settings.Autofill:
			t.Fatalf("%s: a table of settings: %+v", source, settings.TableProperties)
		}
	}
	check("read", form.Items)
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check("written back", again.Items)
	carried, err := json.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	var received ManagedForm
	if err := json.Unmarshal(carried, &received); err != nil {
		t.Fatal(err)
	}
	if err := ValidateManagedForm("studio", received, configuration); err != nil {
		t.Fatal(err)
	}
	check("carried through the Studio", received.Items)
	if !reflect.DeepEqual(received.Items, form.Items) || !reflect.DeepEqual(again.Items, form.Items) {
		t.Fatal("a table changed on the way")
	}
}

// A table of a dynamic list refuses what is wrong in what it has, and the
// record of a dynamic list stands only on a table showing one.
//
// Defect caught: the prototype's spelling ("Custom", "Items", "DontUpdate")
// taken as written; a period without its variant read as custom; a date the
// prototype does not write - without its time, with fractions of a second,
// which Go's own parsing lets through, or a month that is none; the root of a list an item of a document; the
// group of the user settings naming an element the form does not have, or
// written with spaces; what a dynamic list has on a table of a value table,
// or of an attribute the form does not have.
func TestATableOfADynamicListRefusesWhatIsWrong(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for name, test := range map[string]struct{ list, want string }{
		"вариант прототипа":  {"period: {variant: Custom}", "items[0].dynamic_list.period.variant must be custom, today"},
		"вариант не назван":  {"period: {start_date: \"2024-01-01T00:00:00\"}", "items[0].dynamic_list.period.variant must be named"},
		"дата без времени":   {"period: {variant: custom, start_date: \"2024-01-01\"}", "items[0].dynamic_list.period.start_date must be a date written as 2006-01-02T15:04:05"},
		"дробные секунды":    {"period: {variant: custom, start_date: \"2024-01-01T00:00:00.5\"}", "items[0].dynamic_list.period.start_date must be a date written as 2006-01-02T15:04:05"},
		"дата не дата":       {"period: {variant: custom, end_date: \"2024-13-01T00:00:00\"}", "items[0].dynamic_list.period.end_date must be a date written as 2006-01-02T15:04:05"},
		"период обновления":  {"period: {variant: custom}, auto_refresh_period: -1", "items[0].dynamic_list.auto_refresh_period must not be negative"},
		"группы и элементы":  {"period: {variant: custom}, choice_folders_and_items: Items", "items[0].dynamic_list.choice_folders_and_items must be folders, items or folders-and-items"},
		"обновление":         {"period: {variant: custom}, update_on_data_change: DontUpdate", "items[0].dynamic_list.update_on_data_change must be auto or dont-update"},
		"корень документ":    {"period: {variant: custom}, top_level_parent: {kind: document, data: x, object: c0de0000-0000-4000-8000-000000990022}", "items[0].dynamic_list.top_level_parent must be an item of a catalog, a chart of characteristic types or a chart of accounts"},
		"корень без объекта": {"period: {variant: custom}, top_level_parent: {kind: catalog, data: c0de0000-0000-4000-8000-000000990021}", "items[0].dynamic_list.top_level_parent must name its object and its item"},
		"корень без вида":    {"period: {variant: custom}, top_level_parent: {data: x}", "items[0].dynamic_list.top_level_parent must be an item of a catalog"},
		"группа с пробелом":  {"period: {variant: custom}, user_settings_group: Группа Настроек", "items[0].dynamic_list.user_settings_group must be the name or the code of an element of the form"},
		"группы нет в форме": {"period: {variant: custom}, user_settings_group: ГруппаОтборов", "items[0].dynamic_list.user_settings_group names no element of the form"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			source := tableOfAList(test.list)
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
	for name, test := range map[string]struct{ table, want string }{
		"таблица значений": {"name: Товары, kind: table, data_path: Товары, dynamic_list: {period: {variant: custom}}", "items[0].dynamic_list belongs to a table showing an attribute of the form that is a dynamic list"},
		"нет реквизита":    {"name: Остатки, kind: table, data_path: Остатки, dynamic_list: {period: {variant: custom}}", "items[0].dynamic_list belongs to a table showing an attribute of the form that is a dynamic list"},
		"режим просмотра":  {"name: Товары, kind: table, data_path: Товары, view_mode: QuickAccess", "items[0].view_mode must be all or quick-access"},
		"не таблица":       {"name: Поле, kind: input-field, data_path: Список, dynamic_list: {period: {variant: custom}}", "items[0] has what only a table has"},
		"код группы":       {"name: Список, kind: table, data_path: Список, dynamic_list: {period: {variant: custom}, user_settings_group: \"1:02023637-7868-4a5f-8576-835a76e0c9ba\"}", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			source := formElementsForm("  - {id: c0de0000-0000-4000-8000-000000990001, "+test.table+"}\n") +
				"attributes:\n  - {id: c0de0000-0000-4000-8000-000000990011, name: Список, types: [{kind: dynamic-list}]}\n" +
				"  - {id: c0de0000-0000-4000-8000-000000990012, name: Товары, types: [{kind: value-table}]}\n"
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration)
			if test.want == "" && err != nil || test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// What a table of a form names is resolved and noted at load: the root of a
// dynamic list with the project, the group of its user settings by what it
// names, and the autofill the help does not know.
//
// Defect caught: the root of a list naming a catalog the project no longer
// has, loading clean; the group of the user settings written as a code (70
// in the exports) or naming an input field (2) accepted silently; the
// autofill of 267 tables carried without a note; a note on a list whose
// group is a group; an addition a table holds naming another element (3 in
// sb) carried silently, or one naming nothing noted.
func TestWhatATableNamesIsResolvedAndNoted(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		table   string
		note    NoteKind
		written string
		gone    bool
	}{
		"группа":           {table: "dynamic_list: {period: {variant: custom}, user_settings_group: Группа, top_level_parent: {kind: catalog, data: c0de0000-0000-4000-8000-000000990021, object: " + cmpGoods + "}}"},
		"удалённый корень": {table: "dynamic_list: {period: {variant: custom}, top_level_parent: {kind: catalog, data: c0de0000-0000-4000-8000-000000990021, object: " + refGone + "}}", gone: true},
		"корень остатком":  {table: "dynamic_list: {period: {variant: custom}, top_level_parent: {kind: unresolved-reference, data: \"1a2b.3c4d\"}}", gone: true},
		"код группы":       {table: "dynamic_list: {period: {variant: custom}, user_settings_group: \"1:02023637-7868-4a5f-8576-835a76e0c9ba\"}", note: NoteFormReferenceAsWritten, written: "1:02023637-7868-4a5f-8576-835a76e0c9ba"},
		"группа-поле":      {table: "dynamic_list: {period: {variant: custom}, user_settings_group: Поле}", note: NoteUserSettingsGroupNotGroup, written: "Поле (input-field)"},
		"автозаполнение":   {table: "autofill: true", note: NotePropertyOutsideHelp, written: "true"},
		"дополнение чужого": {table: "search_control_addition: {id: c0de0000-0000-4000-8000-000000990005, name: ТаблицаУправлениеПоиском," +
			" kind: search-control-addition, addition_source: ПолеРасширеннаяПодсказка}", note: NoteHeldAdditionOfAnother, written: "ПолеРасширеннаяПодсказка"},
		"дополнение своё": {table: "search_control_addition: {id: c0de0000-0000-4000-8000-000000990005, name: ТаблицаУправлениеПоиском, kind: search-control-addition}"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			element := "items:\n  - {id: c0de0000-0000-4000-8000-000000990002, name: Таблица, kind: table, data_path: Список, " + test.table + "}\n" +
				"  - {id: c0de0000-0000-4000-8000-000000990003, name: Группа, kind: usual-group}\n" +
				"  - {id: c0de0000-0000-4000-8000-000000990004, name: Поле, kind: input-field, extended_tooltip:" +
				" {id: c0de0000-0000-4000-8000-000000990006, name: ПолеРасширеннаяПодсказка, kind: label-decoration}}\n"
			writeFile(t, path, strings.Replace(string(content), "attributes:\n", element+"attributes:\n", 1))
			where := "catalog Номенклатура form ФормаЭлемента element Таблица"
			if test.gone {
				if unresolved := unresolvedOf(t, root); !containsWhere(unresolved, where+" top_level_parent") {
					t.Fatalf("unresolved = %+v", unresolved)
				}
				return
			}
			catalog, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			var found []Note
			for _, note := range catalog.Notes() {
				if strings.HasPrefix(note.Where, where) {
					found = append(found, note)
				}
			}
			if test.note == "" && len(found) != 0 || test.note != "" && (len(found) != 1 || found[0].Kind != test.note || found[0].Written != test.written) {
				t.Fatalf("notes = %+v, want %s %q", found, test.note, test.written)
			}
		})
	}
}

// A table keeps the search string, the view status and the search control it
// holds, and an addition standing apart keeps the table it is of, through
// YAML and the Studio; the additions a table holds are elements of the form,
// reached as every element it holds is.
//
// Defect caught: the 27015 additions the tables of the exports hold lost,
// with their names code reaches them by and their context menus; the width
// or the place of a search string, the alignment of the view status (42) or
// a hidden search control lost; an addition standing in a command bar or a
// group (351) losing its table; the additions of a table left out of what
// the table holds, so that no check, picture or name of the form reaches
// them.
func TestATableKeepsItsAdditions(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Список, kind: table," +
		" search_string_addition: {id: c0de0000-0000-4000-8000-000000990002, name: СписокСтрокаПоиска, kind: search-string-addition," +
		" width: 30, no_auto_max_width: true, max_width: 40, horizontal_stretch: false, group_horizontal_align: right, display_importance: very-low," +
		" context_menu: {id: c0de0000-0000-4000-8000-000000990003, name: СписокСтрокаПоискаКонтекстноеМеню}}," +
		" view_status_addition: {id: c0de0000-0000-4000-8000-000000990004, name: СписокСостояниеПросмотра, kind: view-status-addition," +
		" horizontal_align: left, title: {ru: Состояние}, disabled: true}," +
		" search_control_addition: {id: c0de0000-0000-4000-8000-000000990005, name: СписокУправлениеПоиском, kind: search-control-addition," +
		" hidden: true, addition_source: СписокРасширеннаяПодсказка}," +
		" extended_tooltip: {id: c0de0000-0000-4000-8000-000000990010, name: СписокРасширеннаяПодсказка, kind: label-decoration}}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990006, name: Панель, kind: command-bar, children: [{id: c0de0000-0000-4000-8000-000000990007," +
		" name: ПоискВПанели, kind: search-string-addition, addition_source: Список}]}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990008, name: Группа, kind: usual-group, children: [{id: c0de0000-0000-4000-8000-000000990009," +
		" name: СостояниеВГруппе, kind: view-status-addition, addition_source: список}]}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check := func(source string, items []ManagedFormElement) {
		t.Helper()
		table := items[0]
		search, status, control := table.SearchStringAddition, table.ViewStatusAddition, table.SearchControlAddition
		switch {
		case search == nil || status == nil || control == nil:
			t.Fatalf("%s: additions lost: %+v", source, table.TableAdditions)
		case search.Name != "СписокСтрокаПоиска" || search.Width != 30 || !search.NoAutoMaxWidth || search.MaxWidth != 40 ||
			search.HorizontalStretch == nil || *search.HorizontalStretch || search.GroupHorizontalAlign != ItemHorizontalRight ||
			search.DisplayImportance != FormDisplayImportanceVeryLow || search.ContextMenu == nil || search.AdditionSource != "":
			t.Fatalf("%s: the search string: %+v", source, search)
		case status.HorizontalAlign != ItemHorizontalLeft || status.Title["ru"] != "Состояние" || !status.Disabled:
			t.Fatalf("%s: the view status: %+v", source, status)
		case !control.Hidden || control.AdditionSource != "СписокРасширеннаяПодсказка":
			t.Fatalf("%s: the search control: %+v", source, control)
		case items[1].Children[0].AdditionSource != "Список" || items[2].Children[0].AdditionSource != "список":
			t.Fatalf("%s: additions standing apart: %+v %+v", source, items[1].Children[0], items[2].Children[0])
		}
		nested := map[string]bool{}
		for _, element := range table.Nested() {
			nested[element.Name] = true
		}
		if !nested["СписокСтрокаПоиска"] || !nested["СписокСостояниеПросмотра"] || !nested["СписокУправлениеПоиском"] {
			t.Fatalf("%s: what the table holds: %v", source, nested)
		}
	}
	check("read", form.Items)
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check("written back", again.Items)
	carried, err := json.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	var received ManagedForm
	if err := json.Unmarshal(carried, &received); err != nil {
		t.Fatal(err)
	}
	if err := ValidateManagedForm("studio", received, configuration); err != nil {
		t.Fatal(err)
	}
	check("carried through the Studio", received.Items)
	if !reflect.DeepEqual(received.Items, form.Items) || !reflect.DeepEqual(again.Items, form.Items) {
		t.Fatal("the additions changed on the way")
	}
}

// What is wrong with an addition is refused, naming the place.
//
// Defect caught: a search control held where the search string stands, or
// an addition held by a field; an addition standing apart naming a field or
// an element the form does not have; a source written on a field; the
// additions a table holds checked as nothing - an identifier or a name
// repeated, a property of a field let through.
func TestAnAdditionRefusesWhatIsWrong(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	held := func(slot, element string) string {
		return "  - {id: c0de0000-0000-4000-8000-000000990001, name: Список, kind: table, " + slot + ": {id: c0de0000-0000-4000-8000-000000990002, " + element + "}}\n"
	}
	field := "  - {id: c0de0000-0000-4000-8000-000000990003, name: Поле, kind: input-field}\n"
	for name, test := range map[string]struct{ items, want string }{
		"чужой вид":             {held("search_string_addition", "name: Поиск, kind: search-control-addition"), "items[0].search_string_addition.kind must be search-string-addition"},
		"не дополнение":         {held("view_status_addition", "name: Поиск, kind: input-field"), "items[0].view_status_addition.kind must be view-status-addition"},
		"у поля":                {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, search_control_addition: {id: c0de0000-0000-4000-8000-000000990002, name: Поиск, kind: search-control-addition}}\n", "items[0].search_control_addition is allowed only for tables"},
		"источник поле":         {field + "  - {id: c0de0000-0000-4000-8000-000000990001, name: Поиск, kind: search-string-addition, addition_source: Поле}\n", "items[1].addition_source names no table of the form"},
		"источника нет":         {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Поиск, kind: view-status-addition, addition_source: Список}\n", "items[0].addition_source names no table of the form"},
		"источник с пробелом":   {held("search_string_addition", "name: Поиск, kind: search-string-addition, addition_source: Список Товаров"), "items[0].search_string_addition.addition_source must be the name of an element of the form"},
		"источник у поля":       {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, addition_source: Список}\n", "items[0].addition_source is allowed only for additions of a table"},
		"повтор идентификатора": {held("search_string_addition", "name: Поиск, kind: search-string-addition") + "  - {id: c0de0000-0000-4000-8000-000000990002, name: Поле, kind: input-field}\n", ".id must be unique"},
		"повтор имени":          {held("search_string_addition", "name: Поле, kind: search-string-addition") + field, "name must be unique within the form"},
		"высота дополнения":     {held("search_control_addition", "name: Поиск, kind: search-control-addition, height: 2"), "items[0].search_control_addition has the size and alignment of a field"},
		"выравнивание строки":   {held("search_string_addition", "name: Поиск, kind: search-string-addition, horizontal_align: left"), "items[0].search_string_addition has the size and alignment of a field"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(test.items)), configuration)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// An addition standing apart that names no table is carried and noted, as
// the prototype saves it; an addition a table holds names none and is not
// noted, being of its table.
//
// Defect caught: the form of lombard1 whose command bar holds a search
// string and a search control of no table refused, so that the form is not
// moved; such an addition carried without a note; a note on every addition
// a table holds, burying the real one under 27 thousand.
func TestAnAdditionOfNoTableIsCarriedWithANote(t *testing.T) {
	t.Parallel()
	root := formReferencesProject(t)
	path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	items := "items:\n  - {id: c0de0000-0000-4000-8000-000000990001, name: Список, kind: table, search_string_addition:" +
		" {id: c0de0000-0000-4000-8000-000000990002, name: СписокСтрокаПоиска, kind: search-string-addition}}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990003, name: Панель, kind: command-bar, children: [" +
		"{id: c0de0000-0000-4000-8000-000000990004, name: Дополнение1, kind: search-string-addition}," +
		" {id: c0de0000-0000-4000-8000-000000990005, name: Дополнение2, kind: search-control-addition, addition_source: Список}]}\n"
	writeFile(t, path, strings.Replace(string(content), "attributes:\n", items+"attributes:\n", 1))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	var found []Note
	for _, note := range catalog.Notes() {
		if note.Kind == NoteAdditionOfNoTable {
			found = append(found, note)
		}
	}
	if len(found) != 1 || found[0].Where != "catalog Номенклатура form ФормаЭлемента element Дополнение1" || found[0].Written != "search-string-addition" {
		t.Fatalf("notes = %+v", found)
	}
}
