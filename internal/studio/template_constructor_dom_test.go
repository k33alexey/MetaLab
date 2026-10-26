package studio

import (
	"encoding/json"
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
