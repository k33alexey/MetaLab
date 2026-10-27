package studio

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// styleWindowWalk makes a style by «Добавить» and reads its window and
// palette, opens «Стиль: Открыть» and leaves it by «Отмена»; then it opens
// Основной, which gives some items values of its own, and reads its values
// and its preview.
const styleWindowWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const host = document.querySelector('#style-editor');
  const settle = () => new Promise(resolve => setTimeout(resolve, 80)).then(() => studio.settle());
  const values = within => [...within.querySelectorAll('.style-value-caption')].map(caption => {
    const field = caption.nextElementSibling, sample = field.querySelector('.style-value-sample');
    return [caption.textContent, field.querySelector('.style-value-field > span:last-child').textContent,
      sample ? getComputedStyle(sample).backgroundColor : '', field.querySelector('button').disabled];
  });
  const read = () => ({
    title: host.querySelector('.object-window-title')?.textContent || '',
    captions: [...host.querySelectorAll('.catalog-field > span:first-child')].map(item => item.textContent),
    preview: [...host.querySelectorAll('.style-window-preview > *')].map(item => item.textContent),
    values: values(host),
  });
  await openObject('Стили');
  document.querySelector('#tree-add').click(); await settle();
  steps.created = read();
  steps.paletteTitle = document.querySelector('#inspector-title').textContent;
  steps.palette = [...document.querySelectorAll('#properties .object-palette-row .object-palette-caption')].map(item => item.textContent);
  [...document.querySelectorAll('#properties .object-palette-row button, #properties .object-palette-row a')].find(item => item.textContent === 'Открыть').click();
  await settle();
  const dialog = document.querySelector('dialog.style-values-window');
  steps.dialog = {title: dialog.querySelector('h3').textContent, values: values(dialog).length,
    buttons: [...dialog.querySelectorAll('.localized-title-actions button')].map(item => item.textContent)};
  [...dialog.querySelectorAll('button')].find(item => item.textContent === 'Отмена').click(); await settle();
  steps.dialogClosed = !document.querySelector('dialog.style-values-window');
  steps.unsaved = !document.querySelector('#save').disabled;
  await openObject('Основной');
  steps.main = read();
  const paint = selector => { const item = host.querySelector(selector); return [getComputedStyle(item).color, getComputedStyle(item).borderTopColor]; };
  steps.paint = {heading: paint('.style-preview-heading')[0], field: paint('.style-preview-field')[1]};
  return steps;
})()`

// The window of a style is the prototype's (docs/block-00/stil.md, «Окно»,
// «Палитра», «Новый стиль»; snapshots stil/01, 02, 05, 06, series-2.584/
// 004, 008, 014, 049, 055): «Добавить» opens Стиль1 - the name, synonym,
// comment, subsystems, the preview, and at the right the platform's five
// colours, then the project's items in the order of the tree (by name, where the
// project keeps no order of its own), each «стиль:
// <элемент>» but a border, which shows nothing; «…» unavailable until the
// windows choosing a value. The palette is one tab with «Стиль: Открыть»,
// which opens the same values with OK and «Отмена» and changes nothing. A
// style giving values shows a colour by its numbers, a web colour by its
// name, a font struck out and a border taken from a style item by the
// item's caption, and draws the preview by them.
// Defects caught: a caption or an item not the prototype's, or out of its
// order; a value shown in other words than the prototype's; the sample of
// an item not followed to its own value; the preview not drawn by the
// style; «Стиль: Открыть» not opening or changing the style.
func TestTheWindowOfAStyleIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	red, title, frame := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	for _, item := range []metadata.StyleItemDefinition{
		{ID: red, Name: "ЦветОшибки", Title: metadata.LocalizedText{"ru": "Цвет ошибки"}, Type: metadata.ColorStyleItem,
			Value: metadata.StyleItemValue{Color: &metadata.ColorValue{Source: metadata.AbsoluteColor, RGB: "#FF0000"}}},
		{ID: title, Name: "ШрифтЗаголовка", Type: metadata.FontStyleItem,
			Value: metadata.StyleItemValue{Font: &metadata.FontValue{Source: metadata.StyleFont, From: &metadata.StyleItemReference{Standard: "TextFont"}}}},
		{ID: frame, Name: "РамкаПоля", Type: metadata.BorderStyleItem},
	} {
		item.Format = metadata.CurrentFormat
		var encoded bytes.Buffer
		if err := metadata.Encode(&encoded, item); err != nil {
			t.Fatal(err)
		}
		folder := filepath.Join(root, "metadata", "style-items")
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, item.ID.String()+".yaml"), encoded.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		created, err := workspace.CreateStyle("Основной")
		if err != nil {
			t.Fatal(err)
		}
		struck := true
		style := created.Style
		style.Items = []metadata.StyleItemSetting{
			{Standard: "ActivityColor", Value: metadata.StyleItemValue{Color: &metadata.ColorValue{Source: metadata.AbsoluteColor, RGB: "#FAC371"}}},
			{Standard: "ЦветАкцента", Value: metadata.StyleItemValue{Color: &metadata.ColorValue{Source: metadata.WebColor, Name: "MintCream"}}},
			{Item: &title, Value: metadata.StyleItemValue{Font: &metadata.FontValue{Source: metadata.StyleFont, From: &metadata.StyleItemReference{Standard: "TextFont"}, Strikeout: &struck}}},
			{Item: &frame, Value: metadata.StyleItemValue{Border: &metadata.BorderValue{Source: metadata.StyleBorder, From: &metadata.StyleItemReference{Standard: "ControlBorder"}}}},
		}
		if _, err := workspace.SaveStyleWindow(created.Path, StyleWindowValue{StyleDefinition: style}, created.Revision); err != nil {
			t.Fatal(err)
		}
	})
	result, err := browser.evaluate(ctx, page, styleWindowWalk)
	if err != nil {
		t.Fatal(err)
	}
	type window struct {
		Title             string
		Captions, Preview []string
		Values            [][]any
	}
	var steps struct {
		Created, Main window
		PaletteTitle  string
		Palette       []string
		Dialog        struct {
			Title   string
			Values  int
			Buttons []string
		}
		DialogClosed, Unsaved bool
		Paint                 struct{ Heading, Field string }
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	expect := func(what string, got, want any) {
		t.Helper()
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("%s: %s, want %s", what, gotJSON, wantJSON)
		}
	}
	none := "rgba(0, 0, 0, 0)"
	captions := []string{"Имя:", "Синоним:", "Комментарий:", "Подсистемы:"}
	preview := []string{"Предварительный просмотр", "Панель разделов", "Панель функций текущего раздела", "Заголовок группы", "Кнопка по умолчанию", "Флажок", "Колонка1Колонка2", "…×"}
	expect("the new style", steps.Created, window{Title: "Стиль Стиль1", Captions: captions, Preview: preview, Values: [][]any{
		{"Цвет активности", "стиль: Цвет активности", none, true},
		{"Цвет акцентирования", "стиль: Цвет акцентирования", none, true},
		{"Цвет навигации", "стиль: Цвет навигации", none, true},
		{"Цвет дополнительной навигации", "стиль: Цвет дополнительной навигации", none, true},
		{"Цвет важного", "стиль: Цвет важного", none, true},
		{"РамкаПоля", "", "", true},
		{"Цвет ошибки", "стиль: Цвет ошибки", "rgb(255, 0, 0)", true},
		{"ШрифтЗаголовка", "стиль: ШрифтЗаголовка", "", true},
	}})
	expect("the palette", []any{steps.PaletteTitle, steps.Palette}, []any{"Свойства: Стиль1", []string{"Имя", "Синоним", "Комментарий", "Стиль"}})
	expect("«Стиль: Открыть»", steps.Dialog, struct {
		Title   string
		Values  int
		Buttons []string
	}{"Стиль Стиль1", 8, []string{"OK", "Отмена"}})
	expect("closed by «Отмена», unchanged", []bool{steps.DialogClosed, steps.Unsaved}, []bool{true, false})
	expect("a style giving values", steps.Main.Values, [][]any{
		{"Цвет активности", "250, 195, 113", "rgb(250, 195, 113)", true},
		{"Цвет акцентирования", "Мятный крем (MintCream)", "rgb(245, 255, 250)", true},
		{"Цвет навигации", "стиль: Цвет навигации", none, true},
		{"Цвет дополнительной навигации", "стиль: Цвет дополнительной навигации", none, true},
		{"Цвет важного", "стиль: Цвет важного", none, true},
		{"РамкаПоля", "стиль: Рамка элементов управления", "", true},
		{"Цвет ошибки", "стиль: Цвет ошибки", "rgb(255, 0, 0)", true},
		{"ШрифтЗаголовка", "стиль: Шрифт текста, зачеркнутый", "", true},
	})
	expect("the preview drawn by it", steps.Paint, struct{ Heading, Field string }{"rgb(245, 255, 250)", "rgb(250, 195, 113)"})
}
