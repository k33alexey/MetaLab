package studio

import (
	"bytes"
	"context"
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

// openStyleProject opens Studio in the browser over the demo project with
// four style items - a colour of its own, a font taken from the
// platform's, a border of no value and a single one of 2 - and the style Основной giving an
// activity colour of numbers, an accent web colour, the font struck out
// and the border taken from the platform's.
func openStyleProject(t *testing.T) (context.Context, *devtools, string, string) {
	t.Helper()
	root := copyDemoProjectForStudioTest(t)
	red, title, frame := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	for _, item := range []metadata.StyleItemDefinition{
		{ID: red, Name: "ЦветОшибки", Title: metadata.LocalizedText{"ru": "Цвет ошибки"}, Type: metadata.ColorStyleItem,
			Value: metadata.StyleItemValue{Color: &metadata.ColorValue{Source: metadata.AbsoluteColor, RGB: "#FF0000"}}},
		{ID: title, Name: "ШрифтЗаголовка", Type: metadata.FontStyleItem,
			Value: metadata.StyleItemValue{Font: &metadata.FontValue{Source: metadata.StyleFont, From: &metadata.StyleItemReference{Standard: "TextFont"}}}},
		{ID: frame, Name: "РамкаПоля", Type: metadata.BorderStyleItem},
		{ID: uuid.MustNew(), Name: "РамкаТонкая", Type: metadata.BorderStyleItem,
			Value: metadata.StyleItemValue{Border: &metadata.BorderValue{Source: metadata.AbsoluteBorder, Line: metadata.SingleBorderLine, Width: 2}}},
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
	return ctx, browser, page, root
}

// The window of a style is the prototype's (docs/block-00/stil.md, «Окно»,
// «Палитра», «Новый стиль»; snapshots stil/01, 02, 05, 06, series-2.584/
// 004, 008, 014, 049, 055): «Добавить» opens Стиль1 - the name, synonym,
// comment, subsystems, the preview, and at the right the platform's five
// colours, then the project's items in the order of the tree (by name, where the
// project keeps no order of its own), each «стиль:
// <элемент>» but a border, which shows nothing; «…» of a colour and a font
// unavailable until the windows choosing them, of a border available. The palette is one tab with «Стиль: Открыть»,
// which opens the same values with OK and «Отмена» and changes nothing. A
// style giving values shows a colour by its numbers, a web colour by its
// name, a font struck out and a border taken from a style item by the
// item's caption, and draws the preview by them.
// Defects caught: a caption or an item not the prototype's, or out of its
// order; a value shown in other words than the prototype's; the sample of
// an item not followed to its own value; the preview not drawn by the
// style; «Стиль: Открыть» not opening or changing the style.
func TestTheWindowOfAStyleIsThePrototypes(t *testing.T) {
	ctx, browser, page, _ := openStyleProject(t)
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
		{"РамкаПоля", "", "", false},
		{"РамкаТонкая", "", "", false},
		{"Цвет ошибки", "стиль: Цвет ошибки", "rgb(255, 0, 0)", true},
		{"ШрифтЗаголовка", "стиль: ШрифтЗаголовка", "", true},
	}})
	expect("the palette", []any{steps.PaletteTitle, steps.Palette}, []any{"Свойства: Стиль1", []string{"Имя", "Синоним", "Комментарий", "Стиль"}})
	expect("«Стиль: Открыть»", steps.Dialog, struct {
		Title   string
		Values  int
		Buttons []string
	}{"Стиль Стиль1", 9, []string{"OK", "Отмена"}})
	expect("closed by «Отмена», unchanged", []bool{steps.DialogClosed, steps.Unsaved}, []bool{true, false})
	expect("a style giving values", steps.Main.Values, [][]any{
		{"Цвет активности", "250, 195, 113", "rgb(250, 195, 113)", true},
		{"Цвет акцентирования", "Мятный крем (MintCream)", "rgb(245, 255, 250)", true},
		{"Цвет навигации", "стиль: Цвет навигации", none, true},
		{"Цвет дополнительной навигации", "стиль: Цвет дополнительной навигации", none, true},
		{"Цвет важного", "стиль: Цвет важного", none, true},
		{"РамкаПоля", "стиль: Рамка элементов управления", "", false},
		{"РамкаТонкая", "", "", false},
		{"Цвет ошибки", "стиль: Цвет ошибки", "rgb(255, 0, 0)", true},
		{"ШрифтЗаголовка", "стиль: Шрифт текста, зачеркнутый", "", true},
	})
	expect("the preview drawn by it", steps.Paint, struct{ Heading, Field string }{"rgb(245, 255, 250)", "rgb(250, 195, 113)"})
}

// styleBorderWalk opens Основной, chooses the border of РамкаПоля by «…»
// and reads «Выбор рамки» as the inventory reads it: marked «Из стиля»,
// unmarked, «Двойная», thickness 6 refused and 5 taken; OK; then
// «Стиль: Открыть» gives the border back to the style's by «Из стиля» and
// is left by «Отмена», then by OK.
const styleBorderWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 80)).then(() => studio.settle());
  const dialog = () => [...document.querySelectorAll('dialog.border-choice-dialog')].pop();
  const button = (within, text) => [...within.querySelectorAll('button')].find(item => item.textContent === text);
  const read = () => {
    const window = dialog(), lists = window.querySelectorAll('.object-window-list');
    const current = list => list.querySelector('.current')?.textContent || '';
    const thickness = windowField('dialog.border-choice-dialog', 'Толщина').querySelector('input');
    return {fromStyle: windowField('dialog.border-choice-dialog', 'Из стиля').querySelector('input').checked,
      styles: [...lists[0].children].map(item => item.textContent), style: current(lists[0]),
      types: [...lists[1].children].map(item => item.textContent), type: current(lists[1]),
      thickness: thickness.value + (thickness.disabled ? ' (недоступна)' : ''),
      sample: window.querySelector('.border-choice-sample .border-choice-none')?.textContent || (window.querySelector('.border-choice-box') ? 'рамка' : '')};
  };
  const row = (host, caption) => [...document.querySelectorAll(host + ' .style-value-caption')].find(item => item.textContent === caption).nextElementSibling;
  const pick = async text => { [...dialog().querySelectorAll('.object-window-list-row')].find(item => item.textContent === text).click(); await settle(); };
  await openObject('Основной');
  button(row('#style-editor', 'РамкаПоля'), '…').click(); await settle();
  steps.opened = read();
  windowField('dialog.border-choice-dialog', 'Из стиля').querySelector('input').click(); await settle();
  steps.unmarked = read();
  await pick('Двойная');
  steps.double = read();
  const thickness = () => windowField('dialog.border-choice-dialog', 'Толщина').querySelector('input');
  thickness().value = '6'; thickness().dispatchEvent(new Event('change')); await settle();
  const question = [...document.querySelectorAll('dialog')].pop();
  steps.question = question.querySelector('h3').textContent;
  button(question, 'Да').click(); await settle();
  steps.after6 = read().thickness;
  thickness().value = '5'; thickness().dispatchEvent(new Event('change')); await settle();
  steps.five = read();
  await pick('Нет рамки');
  steps.none = read();
  await pick('Двойная');
  button(dialog(), 'OK').click(); await settle();
  steps.chosen = {text: row('#style-editor', 'РамкаПоля').querySelector('.style-value-field').textContent, unsaved: !document.querySelector('#save').disabled};
  document.querySelector('#save').click(); await settle();
  [...document.querySelectorAll('#properties .object-palette-row a, #properties .object-palette-row button')].find(item => item.textContent === 'Открыть').click(); await settle();
  button(row('dialog.style-values-window', 'РамкаПоля'), '…').click(); await settle();
  steps.reopened = read();
  windowField('dialog.border-choice-dialog', 'Из стиля').querySelector('input').click(); await settle();
  await pick('РамкаТонкая');
  steps.ownBorder = read();
  button(dialog(), 'OK').click(); await settle();
  steps.inValues = row('dialog.style-values-window', 'РамкаПоля').querySelector('.style-value-field').textContent;
  button(document.querySelector('dialog.style-values-window'), 'Отмена').click(); await settle();
  steps.cancelled = {text: row('#style-editor', 'РамкаПоля').querySelector('.style-value-field').textContent, unsaved: !document.querySelector('#save').disabled};
  [...document.querySelectorAll('#properties .object-palette-row a, #properties .object-palette-row button')].find(item => item.textContent === 'Открыть').click(); await settle();
  button(row('dialog.style-values-window', 'РамкаПоля'), '…').click(); await settle();
  windowField('dialog.border-choice-dialog', 'Из стиля').querySelector('input').click(); await settle();
  button(dialog(), 'OK').click(); await settle();
  button(document.querySelector('dialog.style-values-window'), 'OK').click(); await settle();
  steps.taken = {text: row('#style-editor', 'РамкаПоля').querySelector('.style-value-field').textContent, unsaved: !document.querySelector('#save').disabled};
  return steps;
})()`

// «Выбор рамки» is the prototype's (docs/block-00/stil.md, «Досъёмка
// 27.10.2026»; snapshots series-2.584/033-049): on a border taken from the
// platform's it stands marked «Из стиля» with the platform's border and
// then the project's, the type «Нет рамки», the thickness 1 unavailable,
// «Нет просмотра»; unmarked, the type and thickness stay «Нет рамки», 1
// unavailable; «Двойная» makes the thickness available and draws a frame;
// 6 is asked «Отменить ввод?», «Да» brings back 1; 5 is taken, «Нет рамки»
// shows 1 again, unavailable; a border of the project «Из стиля» shows its
// own type and thickness, unavailable; OK gives the
// style an absolute double border of 5, written by «Сохранить». «Стиль:
// Открыть» opens the same window over its copy: «Отмена» leaves the style
// as it was, OK gives it the values chosen.
// Defects caught: a type or a caption not the prototype's, or out of its
// order; the thickness editable where the prototype keeps it; a thickness
// beyond 1-5 taken; the border chosen not written, or written by «Отмена».
func TestTheBorderOfAStyleIsChosenByThePrototypesWindow(t *testing.T) {
	ctx, browser, page, root := openStyleProject(t)
	result, err := browser.evaluate(ctx, page, styleBorderWalk)
	if err != nil {
		t.Fatal(err)
	}
	type window struct {
		FromStyle                      bool
		Styles, Types                  []string
		Style, Type, Thickness, Sample string
	}
	type shown struct {
		Text    string
		Unsaved bool
	}
	var steps struct {
		Opened, Unmarked, Double, Five, None, Reopened, OwnBorder window
		Question, After6, InValues                                string
		Chosen, Cancelled, Taken                                  shown
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
	styles := []string{"Рамка элементов управления", "РамкаПоля", "РамкаТонкая"}
	types := []string{"Нет рамки", "Одинарная", "Двойная", "Выпуклая", "Вдавленная", "Подчеркивание", "Двойное подчеркивание", "Черта сверху"}
	expect("opened on the platform's border", steps.Opened, window{FromStyle: true, Styles: styles, Types: types, Style: "Рамка элементов управления",
		Type: "Нет рамки", Thickness: "1 (недоступна)", Sample: "Нет просмотра"})
	expect("unmarked", steps.Unmarked, window{Styles: styles, Types: types, Type: "Нет рамки", Thickness: "1 (недоступна)", Sample: "Нет просмотра"})
	expect("double", steps.Double, window{Styles: styles, Types: types, Type: "Двойная", Thickness: "1", Sample: "рамка"})
	expect("6 asked", []string{steps.Question, steps.After6}, []string{"В поле введены некорректные данные. Отменить ввод?", "1"})
	expect("5 taken", steps.Five, window{Styles: styles, Types: types, Type: "Двойная", Thickness: "5", Sample: "рамка"})
	expect("«Нет рамки» after 5", steps.None, window{Styles: styles, Types: types, Type: "Нет рамки", Thickness: "1 (недоступна)", Sample: "Нет просмотра"})
	expect("chosen", steps.Chosen, shown{Text: "", Unsaved: true})
	entries, _ := os.ReadDir(filepath.Join(root, "metadata", "styles"))
	if len(entries) != 1 {
		t.Fatalf("the styles: %v", entries)
	}
	written, err := os.ReadFile(filepath.Join(root, "metadata", "styles", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(written, []byte("line: double")) || !bytes.Contains(written, []byte("width: 5")) {
		t.Errorf("the border chosen is not written:\n%s", written)
	}
	expect("reopened in «Стиль: Открыть»", steps.Reopened, window{Styles: styles, Types: types, Type: "Двойная", Thickness: "5", Sample: "рамка"})
	expect("a border of the project «Из стиля»", steps.OwnBorder, window{FromStyle: true, Styles: styles, Types: types, Style: "РамкаТонкая",
		Type: "Одинарная", Thickness: "2 (недоступна)", Sample: "рамка"})
	expect("chosen in the values", steps.InValues, "стиль: РамкаТонкая")
	expect("«Отмена» of the values", steps.Cancelled, shown{Text: "", Unsaved: false})
	expect("OK of the values", steps.Taken, shown{Text: "стиль: Рамка элементов управления", Unsaved: true})
}
