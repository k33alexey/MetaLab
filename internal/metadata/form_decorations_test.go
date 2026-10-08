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
)

// A label and a picture keep what they have through YAML and the Studio: a
// label its size and limits, font, colours, border, where its text stands,
// the height of its title, its link and a formatted title; a picture what it
// shows, how it fits and scales it, the text with no picture, zooming,
// dragging and its link.
//
// Defect caught: the width of a label (2047 times), its limit turned off
// (5738), its text centred up and down (1092), its colour (3308) or link
// (1452) refused, so that 13149 labels of the exports are not moved; a
// formatted title (1166) read as plain text, so that its marks of bold and
// links show as written; a formatted title with no text losing the mark; the
// picture a decoration shows (3841), how it fits (559), its scale (6) or its
// dragged files (3670) lost.
func TestADecorationKeepsWhatItHas(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Надпись, kind: label-decoration, title: {ru: \"<b>Итого</b>\"}, title_formatted: true," +
		" width: 40, height: 2, no_auto_max_width: true, max_width: 60, no_auto_max_height: true, max_height: 3, horizontal_stretch: true," +
		" vertical_stretch: false, group_horizontal_align: right, group_vertical_align: center, horizontal_align: center, vertical_align: top," +
		" font: {source: auto}, text_color: {source: web, name: Gray}, back_color: {source: auto}, border_color: {source: auto}," +
		" border: {source: absolute, line: single, width: 1}, title_height: 2, hyperlink: true, skip_on_input: false, shortcut: F7}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Пустая, kind: label-decoration, title_formatted: true}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990003, name: Картинка, kind: picture-decoration, picture: {standard: Change, load_transparent: true}," +
		" picture_size: proportionally, image_scale: 40, nonselected_picture_text: {ru: Нет}, zoomable: true, file_drag_mode: as-file," +
		" enable_drag: true, enable_start_drag: true, hyperlink: true, border: {source: absolute, line: underline}, width: 2, height: 1}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check := func(source string, items []ManagedFormElement) {
		t.Helper()
		label, empty, picture := items[0], items[1], items[2]
		switch {
		case !label.TitleFormatted || label.Title["ru"] != "<b>Итого</b>" || !empty.TitleFormatted || len(empty.Title) != 0:
			t.Fatalf("%s: formatted titles: %+v %+v", source, label, empty)
		case label.Width != 40 || label.Height != 2 || !label.NoAutoMaxWidth || label.MaxWidth != 60 || !label.NoAutoMaxHeight || label.MaxHeight != 3 ||
			label.HorizontalStretch == nil || !*label.HorizontalStretch || label.VerticalStretch == nil || *label.VerticalStretch ||
			label.GroupHorizontalAlign != ItemHorizontalRight || label.GroupVerticalAlign != ItemVerticalCenter ||
			label.HorizontalAlign != ItemHorizontalCenter || label.VerticalAlign != ItemVerticalTop:
			t.Fatalf("%s: the size of a label: %+v", source, label.FieldLayout)
		case label.Font == nil || label.TextColor == nil || label.BackColor == nil || label.BorderColor == nil || label.Border == nil || label.TitleHeight != 2:
			t.Fatalf("%s: the look of a label: %+v", source, label.FieldLook)
		case !label.Hyperlink || label.SkipOnInput == nil || *label.SkipOnInput || label.Shortcut != "F7":
			t.Fatalf("%s: the link of a label: %+v", source, label)
		case picture.Picture == nil || picture.Picture.Standard != "Change" || !picture.Picture.LoadTransparent || picture.PictureSize != FormPictureProportionally ||
			picture.ImageScale != 40 || picture.NonselectedPictureText["ru"] != "Нет" || !picture.Zoomable || picture.FileDragMode != FormFileDragAsFile:
			t.Fatalf("%s: a picture: %+v", source, picture.FieldPicture)
		case picture.EnableDrag == nil || picture.EnableStartDrag == nil || !picture.Hyperlink || picture.Border == nil || picture.Border.Line != UnderlineBorderLine:
			t.Fatalf("%s: dragging and link of a picture: %+v", source, picture)
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
		t.Fatal("a decoration changed on the way")
	}
}

// What a decoration does not have is refused on it, and what only a
// decoration has on anything else, naming the property.
//
// Defect caught: a formatted title on a field or a group, where nothing reads
// the marks; the scale of a picture on a label or a picture field; the
// background, the alignment of text or the height of the title on a picture,
// which only a label has; the title font, the activation or the edit mode of
// a field on a decoration; the picture of values of a picture field on a
// picture decoration; a negative scale.
func TestADecorationRefusesWhatIsWrong(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for name, test := range map[string]struct{ element, want string }{
		"форматированное поле":   {"kind: input-field, title_formatted: true", "items[0].title_formatted is allowed only for decorations"},
		"форматированная группа": {"kind: usual-group, title_formatted: true", "items[0].title_formatted is allowed only for decorations"},
		"масштаб надписи":        {"kind: label-decoration, image_scale: 50", "items[0].image_scale is allowed only for pictures"},
		"масштаб поля":           {"kind: picture-field, image_scale: 50", "items[0].image_scale is allowed only for pictures"},
		"масштаб отрицательный":  {"kind: picture-decoration, image_scale: -5", "items[0].image_scale must not be negative"},
		"фон картинки":           {"kind: picture-decoration, back_color: {source: auto}", "items[0] has the look of a field"},
		"высота заголовка":       {"kind: picture-decoration, title_height: 2", "items[0] has the look of a field"},
		"текст картинки":         {"kind: picture-decoration, vertical_align: top", "items[0] has the size and alignment of a field"},
		"шрифт заголовка":        {"kind: label-decoration, title_font: {source: auto}", "items[0] has the look of a field"},
		"активизация":            {"kind: label-decoration, default_item: true", "items[0] has what only a field has"},
		"картинка значений":      {"kind: picture-decoration, values_picture: {standard: Change}", "items[0].values_picture is allowed only for picture fields"},
		"картинка надписи":       {"kind: label-decoration, picture: {standard: Change}", "items[0].picture is allowed only for pages, popups, buttons and pictures"},
		"увеличение надписи":     {"kind: label-decoration, zoomable: true", "items[0].zoomable is allowed only for picture fields and pictures"},
		"перетаскивание надписи": {"kind: label-decoration, enable_drag: true", "items[0].enable_drag is allowed only for spreadsheet document, calendar and planner fields, tables and pictures"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			source := formElementsForm("  - {id: c0de0000-0000-4000-8000-000000990001, name: Элемент, " + test.element + "}\n")
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// The picture a decoration shows is resolved as every picture of an element
// is: a common picture with the project, a file of its own in the folder of
// the element.
//
// Defect caught: the picture of a decoration left out of the pictures of an
// element, so that a common picture that is gone - the 4 pictures sb writes
// by a code of a deleted picture - loads clean, and a file of its own (182
// in the exports) is refused as a file no picture draws.
func TestThePictureOfADecorationIsResolved(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		picture string
		files   []string
		refused string
		gone    bool
	}{
		"общая картинка":     {picture: "{common: " + cmpCommonPicture + "}"},
		"удалённая картинка": {picture: "{common: " + refGone + "}", gone: true},
		"свой файл":          {picture: "{file: Picture.png}", files: []string{"Логотип/Picture.png"}},
		"своего файла нет":   {picture: "{file: Picture.png}", refused: "element Логотип picture is shown with picture file Picture.png, which its folder does not hold"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			element := "items:\n  - {id: c0de0000-0000-4000-8000-000000990002, name: Логотип, kind: picture-decoration, picture: " + test.picture + "}\n"
			writeFile(t, path, strings.Replace(string(content), "attributes:\n", element+"attributes:\n", 1))
			for _, file := range test.files {
				writeFile(t, filepath.Join(filepath.Dir(path), project.FormItemsDirectory, file), "image")
			}
			switch {
			case test.gone:
				if found := unresolvedOf(t, root); !containsWhere(found, "catalog Номенклатура form ФормаЭлемента element Логотип picture") {
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

// Every element keeps its extended tooltip through YAML and the Studio - its
// identifier, its name as written, its formatted text and what it has as a
// label - and the tooltip is an element the element holds.
//
// Defect caught: the 294504 extended tooltips of the exports lost, with the
// names code reaches them by (84 thousand in English, kept apart from the
// name of their element); a tooltip of a button, a page, a table or an
// addition lost while that of a field is kept; its formatted text read as
// plain, its width (113) or limit (920) lost; what it does while the main
// server is out of reach lost; the tooltip left out of what its element
// holds, so that no name or check of the form reaches it.
func TestAnElementKeepsItsExtendedTooltip(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	tooltip := func(id, name, rest string) string {
		return "extended_tooltip: {id: c0de0000-0000-4000-8000-0000009901" + id + ", name: " + name + ", kind: label-decoration" + rest + "}"
	}
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, " +
		tooltip("01", "ПолеРасширеннаяПодсказка", ", title: {ru: \"<b>Сумма</b> с НДС\"}, title_formatted: true, no_auto_max_width: true, max_width: 40,"+
			" width: 43, text_color: {source: web, name: Gray}, on_main_server_unavailable: make-disable") + "}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Кнопка, kind: button, " + tooltip("02", "КнопкаExtendedTooltip", "") + "}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990003, name: Страницы, kind: pages, children: [{id: c0de0000-0000-4000-8000-000000990004," +
		" name: Страница, kind: page, " + tooltip("03", "СтраницаРасширеннаяПодсказка", "") + "}]}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990005, name: Список, kind: table, " + tooltip("04", "СписокРасширеннаяПодсказка", "") +
		", search_string_addition: {id: c0de0000-0000-4000-8000-000000990006, name: СписокСтрокаПоиска, kind: search-string-addition, " +
		tooltip("05", "СписокСтрокаПоискаРасширеннаяПодсказка", "") + "}}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check := func(source string, items []ManagedFormElement) {
		t.Helper()
		field := items[0].ExtendedTooltip
		switch {
		case field == nil || field.Name != "ПолеРасширеннаяПодсказка" || field.Kind != FormElementLabelDecoration || !field.TitleFormatted ||
			field.Title["ru"] != "<b>Сумма</b> с НДС" || !field.NoAutoMaxWidth || field.MaxWidth != 40 || field.Width != 43 || field.TextColor == nil ||
			field.OnMainServerUnavailable != FormServerUnavailableMakeDisable:
			t.Fatalf("%s: the tooltip of a field: %+v", source, field)
		case items[1].ExtendedTooltip == nil || items[1].ExtendedTooltip.Name != "КнопкаExtendedTooltip":
			t.Fatalf("%s: the tooltip of a button: %+v", source, items[1].ExtendedTooltip)
		case items[2].Children[0].ExtendedTooltip == nil || items[3].ExtendedTooltip == nil ||
			items[3].SearchStringAddition.ExtendedTooltip == nil:
			t.Fatalf("%s: the tooltip of a page, a table or an addition lost", source)
		}
		if nested := items[0].Nested(); len(nested) != 1 || nested[0].Name != "ПолеРасширеннаяПодсказка" {
			t.Fatalf("%s: what a field holds: %+v", source, nested)
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
		t.Fatal("a tooltip changed on the way")
	}
}

// What is wrong with an extended tooltip is refused, naming the place, and
// the tooltip is checked as every element is.
//
// Defect caught: a tooltip that is a field or a picture; a tooltip of a
// tooltip, or a context menu of one; a tooltip named as another element, or
// with an identifier taken; what only a picture has on a tooltip; what the
// help gives a decoration for the main server let through on a field, or
// spelled as the prototype writes it; an addition a table holds naming as
// its source an element the form does not have.
func TestAnExtendedTooltipRefusesWhatIsWrong(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	field := func(tooltip string) string {
		return "  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, extended_tooltip: {id: c0de0000-0000-4000-8000-000000990002, " + tooltip + "}}\n"
	}
	for name, test := range map[string]struct{ items, want string }{
		"поле":     {field("name: Подсказка, kind: input-field"), "items[0].extended_tooltip.kind must be label-decoration"},
		"картинка": {field("name: Подсказка, kind: picture-decoration"), "items[0].extended_tooltip.kind must be label-decoration"},
		"подсказка подсказки": {field("name: Подсказка, kind: label-decoration, extended_tooltip: {id: c0de0000-0000-4000-8000-000000990003, name: Ещё, kind: label-decoration}"),
			"items[0].extended_tooltip.extended_tooltip: an extended tooltip has none of its own"},
		"меню подсказки": {field("name: Подсказка, kind: label-decoration, context_menu: {id: c0de0000-0000-4000-8000-000000990003, name: Меню}"),
			"items[0].extended_tooltip.context_menu: an extended tooltip has none"},
		"имя поля":         {field("name: Поле, kind: label-decoration"), "items[0].extended_tooltip.name must be unique within the form"},
		"идентификатор":    {field("name: Подсказка, kind: label-decoration") + "  - {id: c0de0000-0000-4000-8000-000000990002, name: Другое, kind: input-field}\n", ".id must be unique"},
		"увеличение":       {field("name: Подсказка, kind: label-decoration, zoomable: true"), "items[0].extended_tooltip.zoomable is allowed only for picture fields and pictures"},
		"сервер у поля":    {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, on_main_server_unavailable: auto}\n", "items[0].on_main_server_unavailable is allowed only for decorations"},
		"сервер прототипа": {field("name: Подсказка, kind: label-decoration, on_main_server_unavailable: MakeDisable"), "items[0].extended_tooltip.on_main_server_unavailable must be auto, dont-change-behavior or make-disable"},
		"источник ничей": {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Список, kind: table, search_string_addition: {id: c0de0000-0000-4000-8000-000000990002," +
			" name: Поиск, kind: search-string-addition, addition_source: СписокРасширеннаяПодсказка}}\n", "items[0].search_string_addition.addition_source names no element of the form"},
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

// Extended tooltips may share a name, as the prototype names them itself
// and saves the name repeated (3 forms of the exports: «ExtendedTooltip»
// three times, a pair of «Группа2РасширеннаяПодсказка»); the shared name is
// noted.
//
// Defect caught: the forms of the exports whose tooltips repeat a name
// refused; a repeated name carried without a note; a tooltip allowed to share
// the name of an element that is no tooltip.
func TestExtendedTooltipsShareANameWithANote(t *testing.T) {
	t.Parallel()
	shared := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Первое, kind: input-field, extended_tooltip:" +
		" {id: c0de0000-0000-4000-8000-000000990002, name: ExtendedTooltip, kind: label-decoration}}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990003, name: Второе, kind: check-box-field, extended_tooltip:" +
		" {id: c0de0000-0000-4000-8000-000000990004, name: extendedtooltip, kind: label-decoration}}\n"
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(shared)), managedFormConfiguration()); err != nil {
		t.Fatalf("the tooltips of the exports: %v", err)
	}
	clash := shared + "  - {id: c0de0000-0000-4000-8000-000000990005, name: ExtendedTooltip, kind: label-decoration}\n"
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(clash)), managedFormConfiguration()); err == nil ||
		!strings.Contains(err.Error(), "name must be unique within the form") {
		t.Fatalf("a label sharing the name of tooltips: %v", err)
	}
	root := formReferencesProject(t)
	path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, strings.Replace(string(content), "attributes:\n", "items:\n"+shared+"attributes:\n", 1))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	var found []Note
	for _, note := range catalog.Notes() {
		if note.Kind == NoteRepeatedElementName {
			found = append(found, note)
		}
	}
	if len(found) != 1 || !strings.HasSuffix(found[0].Where, "element ExtendedTooltip") || found[0].Written != "2 times" {
		t.Fatalf("notes = %+v", found)
	}
}
