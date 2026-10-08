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
