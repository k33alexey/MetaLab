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
// <элемент>» but a border, which shows nothing, a platform's colour with
// the sample of the platform's own numbers (stil.md, «Второй заход»); «…»
// of a font unavailable until the window choosing it, of a colour and a
// border available. The palette is one tab with «Стиль: Открыть»,
// which opens the same values with OK and «Отмена» and changes nothing. A
// style giving values shows a colour by its numbers, a web colour by its
// name, a font struck out and a border taken from a style item by the
// item's caption, and draws the preview by them.
// Defects caught: a caption or an item not the prototype's, or out of its
// order; a value shown in other words than the prototype's; the sample of
// an item not followed to its own value, of a platform's colour not the
// platform's; the preview not drawn by the
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
	captions := []string{"Имя:", "Синоним:", "Комментарий:", "Подсистемы:"}
	preview := []string{"Предварительный просмотр", "Панель разделов", "Панель функций текущего раздела", "Заголовок группы", "Кнопка по умолчанию", "Флажок", "Колонка1Колонка2", "…×"}
	expect("the new style", steps.Created, window{Title: "Стиль Стиль1", Captions: captions, Preview: preview, Values: [][]any{
		{"Цвет активности", "стиль: Цвет активности", "rgb(250, 204, 31)", false},
		{"Цвет акцентирования", "стиль: Цвет акцентирования", "rgb(0, 150, 70)", false},
		{"Цвет навигации", "стиль: Цвет навигации", "rgb(251, 237, 158)", false},
		{"Цвет дополнительной навигации", "стиль: Цвет дополнительной навигации", "rgb(255, 255, 217)", false},
		{"Цвет важного", "стиль: Цвет важного", "rgb(255, 225, 0)", false},
		{"РамкаПоля", "", "", false},
		{"РамкаТонкая", "", "", false},
		{"Цвет ошибки", "стиль: Цвет ошибки", "rgb(255, 0, 0)", false},
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
		{"Цвет активности", "250, 195, 113", "rgb(250, 195, 113)", false},
		{"Цвет акцентирования", "Мятный крем (MintCream)", "rgb(245, 255, 250)", false},
		{"Цвет навигации", "стиль: Цвет навигации", "rgb(251, 237, 158)", false},
		{"Цвет дополнительной навигации", "стиль: Цвет дополнительной навигации", "rgb(255, 255, 217)", false},
		{"Цвет важного", "стиль: Цвет важного", "rgb(255, 225, 0)", false},
		{"РамкаПоля", "стиль: Рамка элементов управления", "", false},
		{"РамкаТонкая", "", "", false},
		{"Цвет ошибки", "стиль: Цвет ошибки", "rgb(255, 0, 0)", false},
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
  steps.caption = question.querySelector('.dialog-caption')?.textContent || '';
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
		Question, Caption, After6, InValues                       string
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
	// The window of the question is titled as ML Studio, where the
	// prototype's is «Конфигуратор» (ML-STUDIO.md, the owner, 24.10.2026).
	expect("the title of the question", steps.Caption, "ML Studio")
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

// styleColorWalk opens Основной and chooses colours by «Выбор цвета» as the
// inventory reads it: «…» of an item the style gives nothing, the tab
// «Web» and a row of it, 256 refused, a sample, OK; again on the colour of
// numbers, a row of «Из стиля», OK; the item's own row, OK; «#» refused
// and taken, the palette and its bar, «Отмена»; then the simple window as
// the palette of a style item opens it.
const styleColorWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 80)).then(() => studio.settle());
  const dialog = () => [...document.querySelectorAll('dialog.color-choice-dialog')].pop();
  const button = (within, text) => [...within.querySelectorAll('button')].find(item => item.textContent === text);
  const number = caption => [...dialog().querySelectorAll('.catalog-field')].find(field => field.querySelector('span').textContent === caption).querySelector('input');
  const read = () => {
    const window = dialog(), rows = [...window.querySelectorAll('.color-choice-row')];
    return {tab: window.querySelector('.studio-tab[aria-selected=true]')?.textContent || '',
      rows: rows.length, first: rows[0]?.textContent || '', last: rows.at(-1)?.textContent || '',
      firstSample: rows[0] ? getComputedStyle(rows[0].querySelector('.color-choice-sample')).backgroundColor : '',
      current: window.querySelector('.color-choice-row.current')?.textContent || '',
      numbers: ['Красный', 'Зеленый', 'Синий'].map(caption => number(caption).value).join(', '),
      hex: number('#').value, chosen: getComputedStyle(window.querySelector('.color-choice-chosen')).backgroundColor,
      swatch: window.querySelector('.color-choice-swatch.current')?.title || ''};
  };
  const field = caption => [...document.querySelectorAll('#style-editor .style-value-caption')].find(item => item.textContent === caption).nextElementSibling;
  const shown = caption => { const row = field(caption); return [row.querySelector('.style-value-field').textContent, getComputedStyle(row.querySelector('.style-value-sample')).backgroundColor]; };
  const open = async caption => { button(field(caption), '…').click(); await settle(); };
  const pick = async text => { [...dialog().querySelectorAll('.color-choice-row')].find(item => item.textContent === text).click(); await settle(); };
  const tab = async text => { [...dialog().querySelectorAll('.studio-tab')].find(item => item.textContent === text).click(); await settle(); };
  const type = async (caption, value) => { const input = number(caption); input.value = value; input.dispatchEvent(new Event('change')); await settle(); };
  await openObject('Основной');
  await open('Цвет навигации');
  steps.opened = read();
  steps.buttons = [...dialog().querySelectorAll('.localized-title-actions button')].map(item => item.textContent);
  await tab('Web');
  steps.web = read();
  await pick('Мятный крем (MintCream)');
  steps.mint = read();
  await type('Красный', '256');
  const question = [...document.querySelectorAll('dialog')].pop();
  steps.question = question.querySelector('h3').textContent;
  button(question, 'Да').click(); await settle();
  steps.after256 = read().numbers;
  [...dialog().querySelectorAll('.color-choice-swatch')][16].click(); await settle();
  steps.swatch = read();
  button(dialog(), 'OK').click(); await settle();
  steps.chosen = {shown: shown('Цвет навигации'), unsaved: !document.querySelector('#save').disabled};
  await open('Цвет навигации');
  steps.reopened = read();
  await pick('Цвет важного');
  button(dialog(), 'OK').click(); await settle();
  steps.fromStyle = shown('Цвет навигации');
  await open('Цвет навигации');
  steps.onStyle = read();
  await pick('Цвет навигации');
  button(dialog(), 'OK').click(); await settle();
  steps.itself = shown('Цвет навигации');
  document.querySelector('#save').click(); await settle();
  await open('Цвет ошибки');
  await type('#', 'zz');
  const refusal = [...document.querySelectorAll('dialog')].pop();
  steps.hexQuestion = refusal.querySelector('h3').textContent;
  button(refusal, 'Да').click(); await settle();
  steps.hexBack = read().hex;
  await type('#', '00ff00');
  steps.hex = read();
  const at = (element, x, y) => { const box = element.getBoundingClientRect();
    element.dispatchEvent(new PointerEvent('pointerdown', {clientX: box.left + x * box.width, clientY: box.top + y * box.height, bubbles: true, pointerId: 1}));
    element.dispatchEvent(new PointerEvent('pointerup', {bubbles: true, pointerId: 1})); };
  at(dialog().querySelector('.color-choice-palette'), 0, 0); await settle();
  steps.paletteCorner = read().numbers;
  at(dialog().querySelector('.color-choice-palette'), 1 / 3, 0); await settle();
  steps.paletteThird = read().numbers;
  at(dialog().querySelector('.color-choice-palette'), 0.5, 1); await settle();
  steps.paletteBottom = read().numbers;
  at(dialog().querySelector('.color-choice-value'), 0, 1); await settle();
  steps.valueBottom = read().numbers;
  button(dialog(), 'Отмена').click(); await settle();
  steps.cancelled = shown('Цвет ошибки');
  const path = [...document.querySelectorAll('*')].find(item => item.children.length === 0 && /^metadata\/styles\/.*\.yaml$/.test(item.textContent)).textContent;
  const source = {...await (await fetch('/api/style?path=' + encodeURIComponent(path))).json(), style: {items: []}};
  const simple = chooseColor({source: 'absolute', rgb: '#FF0000'}, styleColors(source), styleWebColors(source), {simple: true});
  await settle();
  const shownSimple = dialog();
  steps.simple = {tabs: shownSimple.querySelectorAll('.studio-tab').length, rows: shownSimple.querySelectorAll('.color-choice-row').length,
    current: shownSimple.querySelector('.color-choice-row.current')?.textContent || '',
    fields: [...shownSimple.querySelectorAll('.catalog-field > span:first-child')].map(item => item.textContent),
    examples: [...shownSimple.querySelectorAll('.color-choice-examples > div')].map(item => item.textContent),
    palette: !!shownSimple.querySelector('.color-choice-palette'), buttons: [...shownSimple.querySelectorAll('.localized-title-actions button')].map(item => item.textContent)};
  await pick('Фон формы');
  button(dialog(), 'OK').click();
  steps.simpleChosen = await simple;
  return steps;
})()`

// «Выбор цвета» is the prototype's (stil.md, «Окно», «Второй заход
// 27.10.2026»; element-stilya.md, «Палитра»; obshchee.md, «Значения цвета,
// шрифта и рамки»; snapshots stil/07, series-2.584/024, series-2.595/05):
// «…» of an item the style gives nothing opens on «Из стиля» at the item's
// own row - the platform's 28 colours with the prototype's numbers, then
// the project's - with its numbers and code; «Web» lists 146 colours
// from «Белый (White)» to «Темно-бордовый (Maroon)»; a row sets the
// numbers; 256 is asked «Отменить ввод?»; a sample takes the colour itself
// and marks the first row drawing it; OK writes the numbers, which stay
// numbers on reopening with «Особый текст» marked; a row of «Из стиля»
// writes «стиль: <элемент>»; the item's own row leaves the style giving it
// nothing; a code of not six hex digits is asked; the palette's corner is
// red, a third across green, its bottom white, the bottom of the bar
// black; «Отмена» changes
// nothing. The simple window has no tabs, palette or help: one list of
// 175 rows, «Красный:», «Зеленый:», «Синий:», «Пример фона», «Пример
// текста», and answers the row chosen.
// Defects caught: a number, a caption or a row out of the prototype's
// order; a row answered as numbers or numbers as a row; a value beyond
// 0-255 or a bad code taken; the item's own row written as a reference to
// itself; «Отмена» applied; the simple window drawn as the full one.
func TestTheColourOfAStyleIsChosenByThePrototypesWindow(t *testing.T) {
	ctx, browser, page, root := openStyleProject(t)
	result, err := browser.evaluate(ctx, page, styleColorWalk)
	if err != nil {
		t.Fatal(err)
	}
	type window struct {
		Tab, First, Last, FirstSample, Current, Numbers, Hex, Chosen, Swatch string
		Rows                                                                 int
	}
	var steps struct {
		Opened, Web, Mint, Swatch, Reopened, OnStyle, Hex       window
		Buttons                                                 []string
		Question, After256, HexQuestion, HexBack                string
		PaletteCorner, PaletteThird, PaletteBottom, ValueBottom string
		Chosen                                                  struct {
			Shown   []string
			Unsaved bool
		}
		FromStyle, Itself, Cancelled []string
		Simple                       struct {
			Tabs, Rows                int
			Current                   string
			Fields, Examples, Buttons []string
			Palette                   bool
		}
		SimpleChosen metadata.ColorValue
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
	// The style lists the platform's 28 colours, then Цвет ошибки; Фон
	// формы is FCFAEB, Цвет навигации FBED9E (stil.md, «Второй заход»).
	expect("opened on the item's own row", steps.Opened, window{Tab: "Из стиля", Rows: 29, First: "Фон формы", Last: "Цвет ошибки",
		FirstSample: "rgb(252, 250, 235)", Current: "Цвет навигации", Numbers: "251, 237, 158", Hex: "FBED9E", Chosen: "rgb(251, 237, 158)"})
	expect("the buttons", steps.Buttons, []string{"OK", "Отмена", "Справка"})
	expect("the tab «Web»", steps.Web, window{Tab: "Web", Rows: 146, First: "Белый (White)", Last: "Темно-бордовый (Maroon)",
		FirstSample: "rgb(255, 255, 255)", Numbers: "251, 237, 158", Hex: "FBED9E", Chosen: "rgb(251, 237, 158)"})
	expect("a web colour", steps.Mint, window{Tab: "Web", Rows: 146, First: "Белый (White)", Last: "Темно-бордовый (Maroon)",
		FirstSample: "rgb(255, 255, 255)", Current: "Мятный крем (MintCream)", Numbers: "245, 255, 250", Hex: "F5FFFA", Chosen: "rgb(245, 255, 250)"})
	expect("256 asked", []string{steps.Question, steps.After256}, []string{"В поле введены некорректные данные. Отменить ввод?", "245, 255, 250"})
	expect("a sample", steps.Swatch, window{Tab: "Web", Rows: 146, First: "Белый (White)", Last: "Темно-бордовый (Maroon)",
		FirstSample: "rgb(255, 255, 255)", Current: "Красный (Red)", Numbers: "255, 0, 0", Hex: "FF0000", Chosen: "rgb(255, 0, 0)", Swatch: "255, 0, 0"})
	expect("written as numbers", steps.Chosen, struct {
		Shown   []string
		Unsaved bool
	}{[]string{"255, 0, 0", "rgb(255, 0, 0)"}, true})
	expect("reopened on the numbers", steps.Reopened, window{Tab: "Из стиля", Rows: 29, First: "Фон формы", Last: "Цвет ошибки",
		FirstSample: "rgb(252, 250, 235)", Current: "Особый текст", Numbers: "255, 0, 0", Hex: "FF0000", Chosen: "rgb(255, 0, 0)", Swatch: "255, 0, 0"})
	expect("a row of «Из стиля»", steps.FromStyle, []string{"стиль: Цвет важного", "rgb(255, 225, 0)"})
	expect("reopened on the row", steps.OnStyle.Current, "Цвет важного")
	expect("the item's own row", steps.Itself, []string{"стиль: Цвет навигации", "rgb(251, 237, 158)"})
	expect("a code asked", []string{steps.HexQuestion, steps.HexBack}, []string{"В поле введены некорректные данные. Отменить ввод?", "FF0000"})
	expect("a code taken", []string{steps.Hex.Numbers, steps.Hex.Current}, []string{"0, 255, 0", ""})
	expect("the palette", []string{steps.PaletteCorner, steps.PaletteThird, steps.PaletteBottom, steps.ValueBottom},
		[]string{"255, 0, 0", "0, 255, 0", "255, 255, 255", "0, 0, 0"})
	expect("«Отмена»", steps.Cancelled, []string{"стиль: Цвет ошибки", "rgb(255, 0, 0)"})
	expect("the simple window", steps.Simple, struct {
		Tabs, Rows                int
		Current                   string
		Fields, Examples, Buttons []string
		Palette                   bool
	}{0, 175, "Особый текст", []string{"Красный:", "Зеленый:", "Синий:"}, []string{"Пример фона", "Пример текста"}, []string{"OK", "Отмена"}, false})
	expect("the simple window's answer", steps.SimpleChosen, metadata.ColorValue{Source: metadata.StyleColor, From: &metadata.StyleItemReference{Standard: "FormBackColor"}})
	entries, _ := os.ReadDir(filepath.Join(root, "metadata", "styles"))
	if len(entries) != 1 {
		t.Fatalf("the styles: %v", entries)
	}
	written, err := os.ReadFile(filepath.Join(root, "metadata", "styles", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(written, []byte("NavigationColor")) || !bytes.Contains(written, []byte("ActivityColor")) {
		t.Errorf("the item's own row is written as a value of the style:\n%s", written)
	}
}
