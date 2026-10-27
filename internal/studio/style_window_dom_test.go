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
// of every value available. The palette is one tab with «Стиль: Открыть»,
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
		{"ШрифтЗаголовка", "стиль: ШрифтЗаголовка", "", false},
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
		{"ШрифтЗаголовка", "стиль: Шрифт текста, зачеркнутый", "", false},
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

// styleFontWalk opens Основной and chooses fonts by «Выбор шрифта» as the
// inventory reads it: on a font of the style struck out, another font of
// the style, the face, the size and the switches unmarked from the style
// and set, the scale refused and taken, OK; again, «Системный шрифт» and
// without it, OK; the item's own row, OK; «Отмена»; then the simple window
// as the palette of a style item opens it.
const styleFontWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 80)).then(() => studio.settle());
  const dialog = () => [...document.querySelectorAll('dialog.font-choice-dialog')].pop();
  const button = (within, text) => [...within.querySelectorAll('button')].find(item => item.textContent === text);
  const mark = selector => dialog().querySelector(selector + ' input[type=checkbox]');
  const marks = {style: '.font-choice-left > .catalog-check:nth-of-type(1)', system: '.font-choice-left > .catalog-check:nth-of-type(2)',
    face: '.font-choice-face > .catalog-check', size: '.font-choice-size > .catalog-check', flags: '.font-choice-flags > .catalog-check'};
  const state = input => input ? (input.checked ? 'x' : '-') + (input.disabled ? ' (недоступен)' : '') : '';
  const field = input => input.value + (input.disabled ? ' (недоступно)' : '');
  const read = () => {
    const window = dialog(), lists = [...window.querySelectorAll('.font-choice-list')];
    const current = list => list.querySelector('.current')?.textContent || '';
    return {style: state(mark(marks.style)), system: state(mark(marks.system)), styles: current(lists[0]), systems: current(lists[1]),
      faceFrom: state(mark(marks.face)), face: field(window.querySelector('.font-choice-face > input')),
      sizeFrom: state(mark(marks.size)), size: field(window.querySelector('.font-choice-size > input')),
      flagsFrom: state(mark(marks.flags)),
      flags: [...window.querySelectorAll('.font-choice-flag-grid .catalog-check')].map(item => item.querySelector('span').textContent + ' ' + state(item.querySelector('input'))),
      scale: window.querySelector('.font-choice-scale input').value, sample: window.querySelector('.font-choice-sample').textContent};
  };
  const row = caption => [...document.querySelectorAll('#style-editor .style-value-caption')].find(item => item.textContent === caption).nextElementSibling;
  const shown = caption => row(caption).querySelector('.style-value-field').textContent;
  const open = async caption => { button(row(caption), '…').click(); await settle(); };
  const pick = async (list, text) => { [...dialog().querySelectorAll('.font-choice-list')][list].querySelectorAll('.object-window-list-row').forEach(item => { if (item.textContent === text) item.click(); }); await settle(); };
  const click = async selector => { mark(selector).click(); await settle(); };
  const type = async (input, value) => { input.value = value; input.dispatchEvent(new Event('change')); await settle(); };
  const faces = await (await fetch('/api/fonts')).json();
  steps.faces = faces.length;
  const styleRead = async () => {
    const path = [...document.querySelectorAll('*')].find(item => item.children.length === 0 && /^metadata\/styles\/.*\.yaml$/.test(item.textContent)).textContent;
    return (await fetch('/api/style?path=' + encodeURIComponent(path))).json();
  };
  const fontsGiven = async () => (await styleRead()).style.items.filter(setting => setting.value.font).map(setting => setting.value.font);
  await openObject('Основной');
  await open('ШрифтЗаголовка');
  steps.opened = read();
  steps.lists = [...dialog().querySelectorAll('.font-choice-list')].map(list => [...list.children].map(item => item.textContent));
  steps.buttons = [...dialog().querySelectorAll('.localized-title-actions button')].map(item => item.textContent);
  await pick(0, 'Обычный шрифт текста');
  steps.another = read();
  await click(marks.face);
  steps.faceUnmarked = read().face;
  await pick(2, faces[0]);
  await click(marks.size);
  steps.sizeUnmarked = read().size;
  await pick(3, '12');
  await click(marks.flags);
  steps.flagsUnmarked = read().flags;
  dialog().querySelectorAll('.font-choice-flag-grid .catalog-check input')[0].click(); await settle();
  await type(dialog().querySelector('.font-choice-scale input'), '1000');
  const question = [...document.querySelectorAll('dialog')].pop();
  steps.question = question.querySelector('h3').textContent;
  button(question, 'Да').click(); await settle();
  steps.after1000 = read().scale;
  await type(dialog().querySelector('.font-choice-scale input'), '150');
  steps.changed = read();
  button(dialog(), 'OK').click(); await settle();
  steps.chosen = shown('ШрифтЗаголовка');
  document.querySelector('#save').click(); await settle();
  steps.saved = await fontsGiven();
  await open('ШрифтЗаголовка');
  steps.reopened = read();
  await click(marks.system);
  steps.systemMarked = read();
  await click(marks.system);
  steps.own = read();
  button(dialog(), 'OK').click(); await settle();
  steps.ownShown = shown('ШрифтЗаголовка');
  await open('ШрифтЗаголовка');
  await click(marks.style);
  await type(dialog().querySelector('.font-choice-scale input'), '100');
  await pick(0, 'ШрифтЗаголовка');
  button(dialog(), 'OK').click(); await settle();
  steps.itself = shown('ШрифтЗаголовка');
  document.querySelector('#save').click(); await settle();
  steps.itselfSaved = await fontsGiven();
  await open('ШрифтЗаголовка');
  await click(marks.style);
  steps.ownFromStyle = read();
  await pick(1, 'OEM моноширинный шрифт');
  button(dialog(), 'Отмена').click(); await settle();
  steps.cancelled = shown('ШрифтЗаголовка');
  const source = await styleRead();
  const fonts = styleFonts(source).filter(font => font.value.from.standard);
  const simple = chooseFont({source: 'style', from: {standard: 'TextFont'}}, fonts, [], faces, {simple: true});
  await settle();
  const simpleWindow = dialog(), simpleLists = [...simpleWindow.querySelectorAll('.font-choice-list')];
  steps.simple = {lists: simpleLists.map(list => [...list.children].slice(0, 7).map(item => item.textContent)),
    rows: simpleLists[0].children.length, marks: simpleWindow.querySelectorAll('.font-choice-left > .catalog-check, .font-choice-face, .font-choice-flags > .catalog-check').length,
    flags: simpleWindow.querySelectorAll('.font-choice-flag-grid input:not([disabled])').length,
    current: simpleLists.map(list => list.querySelector('.current')?.textContent || ''),
    buttons: [...simpleWindow.querySelectorAll('.localized-title-actions button')].map(item => item.textContent)};
  await pick(1, '12');
  simpleWindow.querySelectorAll('.font-choice-flag-grid input')[0].click(); await settle();
  await pick(0, '<Обычный шрифт текста>');
  steps.simpleStyle = [...dialog().querySelectorAll('.font-choice-flag-grid input')].map(input => input.checked).join(), steps.simpleSize = dialog().querySelectorAll('.font-choice-list')[1].querySelector('.current').textContent;
  dialog().querySelectorAll('.font-choice-flag-grid input')[1].click(); await settle();
  button(dialog(), 'OK').click();
  steps.simpleChosen = await simple;
  const face = chooseFont({source: 'style', from: {standard: 'TextFont'}, size: 12}, fonts, [], faces, {simple: true});
  await settle();
  await pick(0, faces[0]);
  button(dialog(), 'OK').click();
  steps.simpleFace = await face;
  return steps;
})()`

// «Выбор шрифта» is the prototype's (stil.md, «Окно», «Второй заход
// 27.10.2026»; element-stilya.md, «Палитра»; obshchee.md, «Значения цвета,
// шрифта и рамки»; snapshots stil/08, series-2.595/02, 58): opened on a
// font of the style struck out it marks «Шрифт стиля» at «Шрифт текста»,
// the face and the size from the style, the switches its own; the lists
// are the platform's five fonts, then the project's, the five fonts of the
// system, the faces of the system and the sizes 8-72. Another font of the
// style marks the three «Из стиля» again and drops the switch; unmarked,
// the face is empty, the size 13, the switches unmarked; 1000 of the scale
// is asked «Отменить ввод?»; OK writes «стиль: Обычный шрифт текста,
// <гарнитура>, 12, полужирный, 150%» and «Сохранить» the values. Reopened,
// it shows them; «Системный шрифт» takes «Шрифт стиля» away and keeps the
// marks; without either, the font is Arial, 13, every «Из стиля» unmarked
// and unavailable, written «Arial, 13». The item's own row leaves the
// style giving it nothing; «Отмена» changes nothing. The simple window
// lists the five fonts in angle brackets, then the faces; the sizes with
// «<>» first; no «Из стиля», no «Системный шрифт», no help; a font of the
// style drops the switches and keeps the size; a face after it sets 8.
// Defects caught: a list, a caption or an order not the prototype's; a
// mark not taken, not given back or available where the prototype keeps
// it; a font of the style keeping what was changed; a scale beyond 999
// taken; a font written in words not the platform's or not written; the
// item's own row written as a reference to itself; «Отмена» applied; the
// simple window drawn as the full one or not dropping and setting what the
// prototype's does.
func TestTheFontOfAStyleIsChosenByThePrototypesWindow(t *testing.T) {
	ctx, browser, page, _ := openStyleProject(t)
	result, err := browser.evaluate(ctx, page, styleFontWalk)
	if err != nil {
		t.Fatal(err)
	}
	type window struct {
		Style, System, Styles, Systems, FaceFrom, Face, SizeFrom, Size, FlagsFrom string
		Flags                                                                     []string
		Scale, Sample                                                             string
	}
	var steps struct {
		Faces                                                               int
		Opened, Another, Changed, Reopened, SystemMarked, Own, OwnFromStyle window
		Saved, ItselfSaved                                                  []metadata.FontValue
		Lists                                                               [][]string
		Buttons                                                             []string
		FaceUnmarked, SizeUnmarked                                          string
		FlagsUnmarked                                                       []string
		Question, After1000, Chosen, OwnShown, Itself, Cancelled            string
		Simple                                                              struct {
			Lists       [][]string
			Rows, Marks int
			Flags       int
			Current     []string
			Buttons     []string
		}
		SimpleStyle, SimpleSize  string
		SimpleChosen, SimpleFace metadata.FontValue
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	if steps.Faces == 0 {
		t.Fatal("the server read no font of the system")
	}
	var faces []string
	if err := json.Unmarshal(mustJSON(t, systemFaces()), &faces); err != nil {
		t.Fatal(err)
	}
	expect := func(what string, got, want any) {
		t.Helper()
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("%s: %s, want %s", what, gotJSON, wantJSON)
		}
	}
	flags := func(states ...string) []string {
		names := []string{"Полужирный", "Наклонный", "Подчеркнутый", "Зачеркнутый"}
		for index := range states {
			states[index] = names[index] + " " + states[index]
		}
		return states
	}
	off := "- (недоступен)"
	expect("opened on a font of the style struck out", steps.Opened, window{Style: "x", System: "-", Styles: "Шрифт текста", FaceFrom: "x", Face: " (недоступно)",
		SizeFrom: "x", Size: " (недоступно)", FlagsFrom: "-", Flags: flags("-", "-", "-", "x"), Scale: "100", Sample: "Шрифт текста"})
	sizes := []string{"8", "9", "10", "11", "12", "14", "16", "18", "20", "22", "24", "26", "28", "36", "48", "72"}
	expect("the lists", steps.Lists, [][]string{
		{"Шрифт текста", "Мелкий шрифт текста", "Обычный шрифт текста", "Крупный шрифт текста", "Очень крупный шрифт текста", "ШрифтЗаголовка"},
		{"Шрифт диалогов и меню", "OEM моноширинный шрифт", "ANSI моноширинный шрифт", "ANSI пропорциональный шрифт", "Системный шрифт"},
		faces, sizes})
	expect("the buttons", steps.Buttons, []string{"OK", "Отмена", "Справка"})
	expect("another font of the style", steps.Another, window{Style: "x", System: "-", Styles: "Обычный шрифт текста", FaceFrom: "x", Face: " (недоступно)",
		SizeFrom: "x", Size: " (недоступно)", FlagsFrom: "x", Flags: flags(off, off, off, off), Scale: "100", Sample: "Обычный шрифт текста"})
	expect("unmarked", []any{steps.FaceUnmarked, steps.SizeUnmarked, steps.FlagsUnmarked}, []any{"", "13", flags("-", "-", "-", "-")})
	expect("1000 asked", []string{steps.Question, steps.After1000}, []string{"В поле введены некорректные данные. Отменить ввод?", "100"})
	changed := window{Style: "x", System: "-", Styles: "Обычный шрифт текста", FaceFrom: "-", Face: faces[0], SizeFrom: "-", Size: "12",
		FlagsFrom: "-", Flags: flags("x", "-", "-", "-"), Scale: "150", Sample: faces[0]}
	expect("changed", steps.Changed, changed)
	expect("written", steps.Chosen, "стиль: Обычный шрифт текста, "+faces[0]+", 12, полужирный, 150%")
	expect("reopened", steps.Reopened, changed)
	systemMarked := changed
	systemMarked.Style, systemMarked.System, systemMarked.Styles, systemMarked.Systems = "-", "x", "", "Шрифт диалогов и меню"
	expect("«Системный шрифт»", steps.SystemMarked, systemMarked)
	expect("a font of its own", steps.Own, window{Style: "-", System: "-", FaceFrom: off, Face: "Arial", SizeFrom: off, Size: "13", FlagsFrom: off,
		Flags: flags("-", "-", "-", "-"), Scale: "150", Sample: "Arial"})
	expect("written as its own", steps.OwnShown, "Arial, 13, 150%")
	expect("the item's own row", steps.Itself, "стиль: ШрифтЗаголовка")
	expect("«Отмена»", steps.Cancelled, "стиль: ШрифтЗаголовка")
	simpleFaces := append([]string{"<Шрифт текста>", "<Мелкий шрифт текста>", "<Обычный шрифт текста>", "<Крупный шрифт текста>", "<Очень крупный шрифт текста>"}, faces...)
	expect("the simple window", steps.Simple, struct {
		Lists       [][]string
		Rows, Marks int
		Flags       int
		Current     []string
		Buttons     []string
	}{[][]string{simpleFaces[:7], append([]string{"<>"}, sizes[:6]...)}, len(simpleFaces), 0, 4, []string{"<Шрифт текста>", "<>"}, []string{"OK", "Отмена"}})
	expect("a font of the style in the simple window", []string{steps.SimpleStyle, steps.SimpleSize}, []string{"false,false,false,false", "12"})
	yes := true
	expect("the simple window's answer", steps.SimpleChosen, metadata.FontValue{Source: metadata.StyleFont, From: &metadata.StyleItemReference{Standard: "NormalTextFont"},
		Size: 12, Italic: &yes})
	expect("a face after a font of the style", steps.SimpleFace, metadata.FontValue{Source: metadata.AbsoluteFont, Face: faces[0], Size: 8})
	no := false
	expect("saved", steps.Saved, []metadata.FontValue{{Source: metadata.StyleFont, From: &metadata.StyleItemReference{Standard: "NormalTextFont"},
		Face: faces[0], Size: 12, Scale: 150, Bold: &yes, Italic: &no, Underline: &no, Strikeout: &no}})
	expect("the item's own row saved", steps.ItselfSaved, []metadata.FontValue{})
	expect("a font of the style unmarked", steps.OwnFromStyle, window{Style: "-", System: "-", FaceFrom: off, Face: "Arial", SizeFrom: off, Size: "13", FlagsFrom: off,
		Flags: flags("-", "-", "-", "-"), Scale: "100", Sample: "Arial"})
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
