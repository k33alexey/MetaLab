package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// templateConstructorWalk opens «Добавить» of the common templates and reads
// the constructor as the inventory reads it, chooses binary data and makes
// the template; reads its palette; opens the constructor again, makes a
// second Макет and reads the refusal, and cancels the constructor.
const templateConstructorWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 80)).then(() => studio.settle());
  const dialog = () => document.querySelector('dialog[open]');
  const read = () => ({
    title: dialog()?.querySelector('h3')?.textContent || '',
    captions: [...dialog().querySelectorAll('.catalog-field > span:first-child')].map(item => item.textContent),
    name: dialog().querySelector('.catalog-field input').value,
    synonym: dialog().querySelectorAll('.catalog-field input')[1].value,
    frame: dialog().querySelector('fieldset legend').textContent,
    kinds: [...dialog().querySelectorAll('fieldset label')].map(label => label.textContent + (label.querySelector('input').disabled ? ' (недоступен)' : '')),
    checked: dialog().querySelector('fieldset input:checked').parentElement.textContent,
    load: dialog().querySelector('.catalog-field:last-of-type button').disabled,
    buttons: [...dialog().querySelectorAll('.localized-title-actions button')].map(item => item.textContent),
  });
  await openObject('Общие макеты');
  document.querySelector('#tree-add').click(); await settle();
  steps.opened = read();
  [...dialog().querySelectorAll('fieldset label')].find(label => label.textContent === 'Двоичные данные').querySelector('input').click(); await settle();
  steps.binaryLoad = dialog().querySelector('.catalog-field:last-of-type button').disabled;
  [...dialog().querySelectorAll('.localized-title-actions button')].find(item => item.textContent === 'Готово').click(); await settle();
  steps.made = {dialog: !!dialog(), tree: !!treeRow('Макет'), title: document.querySelector('#inspector-title').textContent,
    tabs: [...document.querySelectorAll('#properties .studio-tab')].map(tab => tab.textContent),
    rows: [...document.querySelectorAll('#properties .object-palette-row .object-palette-caption')].map(item => item.textContent),
    kind: windowField('#properties', 'Тип макета').querySelector('select').value,
    kindDisabled: windowField('#properties', 'Тип макета').querySelector('select').disabled,
    open: windowField('#properties', 'Макет').querySelector('button').disabled,
    synonym: windowField('#properties', 'Синоним').querySelector('input').value};
  await openObject('Общие макеты');
  document.querySelector('#tree-add').click(); await settle();
  [...dialog().querySelectorAll('.localized-title-actions button')].find(item => item.textContent === 'Готово').click(); await settle();
  steps.refusal = dialog()?.querySelector('h3')?.textContent || '';
  [...dialog().querySelectorAll('button')].find(item => item.textContent === 'ОК').click(); await settle();
  steps.again = dialog()?.querySelector('h3')?.textContent || '';
  [...dialog().querySelectorAll('.localized-title-actions button')].find(item => item.textContent === 'Отмена').click(); await settle();
  steps.cancelled = {dialog: !!dialog(), templates: [...document.querySelectorAll('#tree .studio-tree-row')].filter(row => row.textContent.trim() === 'Макет').length};
  return steps;
})()`

// The constructor of a template is the prototype's (docs/block-00/
// obshchij-maket.md, «Команды»; snapshot obshchij-maket/03-konstruktor):
// «Конструктор макета», the name «Макет» and its synonym, the comment,
// «Выберите тип макета:» with the ten kinds, «Табличный документ» marked and
// «Active document» unavailable, «Загрузить из файла:» available for binary
// data alone of the two here; «Готово», «Отмена», «Справка». The template
// made shows its palette (snapshot 07): «Тип макета» unavailable with its
// kind, «Макет: Открыть» unavailable until the editors of content. A second
// Макет is refused with the prototype's words and the constructor opens
// again; «Отмена» makes nothing. Defects caught: a caption, a kind, its
// order or its availability not the prototype's; the kind made not the one
// chosen; the kind editable after; a refused name losing the constructor.
func TestTheConstructorOfATemplateIsThePrototypes(t *testing.T) {
	ctx, browser, page := openStudioInBrowser(t, copyDemoProjectForStudioTest(t), func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, templateConstructorWalk)
	if err != nil {
		t.Fatal(err)
	}
	type constructor struct {
		Title, Name, Synonym, Frame, Checked string
		Captions, Kinds, Buttons             []string
		Load                                 bool
	}
	var steps struct {
		Opened     constructor
		BinaryLoad bool
		Made       struct {
			Dialog, Tree, KindDisabled, Open bool
			Title, Kind, Synonym             string
			Tabs, Rows                       []string
		}
		Refusal, Again string
		Cancelled      struct {
			Dialog    bool
			Templates int
		}
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
	expect("the constructor", steps.Opened, constructor{Title: "Конструктор макета", Name: "Макет", Synonym: "Макет", Frame: "Выберите тип макета:",
		Checked: "Табличный документ", Captions: []string{"Имя:", "Синоним:", "Комментарий:", "Загрузить из файла:"},
		Kinds: []string{"Табличный документ", "Текстовый документ", "Двоичные данные", "Active document (недоступен)", "HTML документ",
			"Географическая схема", "Графическая схема", "Схема компоновки данных", "Макет оформления компоновки данных", "Внешняя компонента"},
		Buttons: []string{"Готово", "Отмена", "Справка"}, Load: true})
	expect("binary data loads a file", steps.BinaryLoad, false)
	expect("the template made", steps.Made, struct {
		Dialog, Tree, KindDisabled, Open bool
		Title, Kind, Synonym             string
		Tabs, Rows                       []string
	}{false, true, true, true, "Свойства: Макет", "binary", "Макет", []string{"Основные"}, []string{"Имя", "Синоним", "Комментарий", "Тип макета", "Макет"}})
	expect("a second Макет", []string{steps.Refusal, steps.Again}, []string{"Имя не уникально!", "Конструктор макета"})
	expect("cancelled", steps.Cancelled, struct {
		Dialog    bool
		Templates int
	}{false, 1})
}

// objectTemplateWalk opens the page «Макеты» of the window of Товары, adds a
// text template Печать by the constructor and reads the page and the
// palette; saves; adds печать again and reads the refusal; then adds one by
// «Добавить» on the branch «Макеты» of the tree and cancels it.
const objectTemplateWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 80)).then(() => studio.settle());
  const host = document.querySelector('#catalog-editor');
  const dialog = () => document.querySelector('dialog[open]');
  const press = caption => [...dialog().querySelectorAll('button')].find(item => item.textContent === caption).click();
  const add = () => host.querySelector('.object-window-panel button[title=Добавить]');
  await openObject('Товары');
  await windowPage('Макеты');
  steps.addAvailable = !add().disabled;
  add().click(); await settle();
  steps.constructor = dialog()?.querySelector('h3')?.textContent || '';
  const name = dialog().querySelector('.catalog-field input');
  name.value = 'Печать'; name.dispatchEvent(new Event('input', {bubbles: true}));
  [...dialog().querySelectorAll('fieldset label')].find(label => label.textContent === 'Текстовый документ').querySelector('input').click();
  press('Готово'); await settle();
  steps.added = {rows: [...host.querySelectorAll('.object-window-body .studio-tree-row')].map(item => item.textContent.trim()),
    chosen: host.querySelector('.object-window-body .studio-tree-row.selected')?.textContent.trim() || '',
    palette: document.querySelector('#inspector-title').textContent,
    paletteRows: [...document.querySelectorAll('#properties .object-palette-row .object-palette-caption')].map(item => item.textContent),
    kind: windowField('#properties', 'Тип макета').querySelector('select').value,
    kindDisabled: windowField('#properties', 'Тип макета').querySelector('select').disabled,
    synonym: windowField('#properties', 'Синоним').querySelector('input').value,
    unsaved: !document.querySelector('#save').disabled};
  document.querySelector('#save').click(); await settle();
  steps.saved = !document.querySelector('#save').disabled;
  add().click(); await settle();
  const again = dialog().querySelector('.catalog-field input');
  again.value = 'печать'; again.dispatchEvent(new Event('input', {bubbles: true}));
  press('Готово'); await settle();
  steps.refusal = dialog()?.querySelector('h3')?.textContent || '';
  press('ОК'); await settle();
  press('Отмена'); await settle();
  treeRow('Макеты').click(); await settle();
  document.querySelector('#tree-add').click(); await settle();
  steps.fromTree = dialog()?.querySelector('h3')?.textContent || '';
  press('Отмена'); await settle();
  steps.left = {dialog: !!dialog(), unsaved: !document.querySelector('#save').disabled};
  return steps;
})()`

// «Добавить» of the templates of a catalog - on its page «Макеты» and on its
// branch of the tree - is the same constructor as a common template's
// (docs/block-00/maket.md): the template made stands in the page chosen,
// its palette the common template's - the kind unavailable; «Сохранить»
// writes it; a name taken among the catalog's templates is refused with the
// prototype's words. Defects caught: «Добавить» unavailable; the template
// not added, not chosen, or of another kind; its palette not the
// template's; a second template of one name added; the branch of the tree
// opening nothing.
func TestTheConstructorOfATemplateOfACatalog(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, objectTemplateWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		AddAvailable bool
		Constructor  string
		Added        struct {
			Rows, PaletteRows     []string
			Chosen, Palette, Kind string
			Synonym               string
			KindDisabled, Unsaved bool
		}
		Saved             bool
		Refusal, FromTree string
		Left              struct{ Dialog, Unsaved bool }
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
	expect("«Добавить» on the page", []any{steps.AddAvailable, steps.Constructor}, []any{true, "Конструктор макета"})
	expect("its palette", steps.Added.PaletteRows, []string{"Имя", "Синоним", "Комментарий", "Тип макета", "Макет"})
	expect("the template added", []any{steps.Added.Rows, steps.Added.Chosen, steps.Added.Palette, steps.Added.Kind, steps.Added.KindDisabled, steps.Added.Synonym, steps.Added.Unsaved},
		[]any{[]string{"Макеты", "Печать"}, "Печать", "Свойства: Печать", "text", true, "Печать", true})
	expect("saved", steps.Saved, false)
	expect("a second печать", steps.Refusal, "Имя не уникально!")
	expect("the branch of the tree", steps.FromTree, "Конструктор макета")
	expect("cancelled", steps.Left, struct{ Dialog, Unsaved bool }{false, false})
	content, err := os.ReadFile(filepath.Join(root, "metadata", "catalogs", "Товары", "object.yaml"))
	if err != nil || !strings.Contains(string(content), "name: Печать") || !strings.Contains(string(content), "kind: text") {
		t.Errorf("the catalog on disk does not declare Печать: %v", err)
	}
}
