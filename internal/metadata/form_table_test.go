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
		BehaviorOnHorizontalCompression: FormHorizontalCompressionAuto}
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
		"картинка поля":           {"kind: table, file_drag_mode: as-file, picture_size: stretch", "items[0] has what only a picture field has"},
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
