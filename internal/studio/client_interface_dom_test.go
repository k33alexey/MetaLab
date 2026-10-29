package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
)

// clientInterfaceWalk opens «Открыть интерфейс клиентского приложения» of
// the menu of the root on the interface of erp and goes through the window:
// moves the panel of open windows up, adds a panel to the group of the top
// and chooses the panel of history for it, adds a group to the right and
// deletes it, gives the panel of sections «Текст под картинкой», goes from a
// panel of the tree to the list and saves.
const clientInterfaceWalk = `(async () => {
  const studio = window.__studio, steps = {};
  steps.errors = []; window.addEventListener('unhandledrejection', event => steps.errors.push(String(event.reason?.stack || event.reason))); window.addEventListener('error', event => steps.errors.push(String(event.message)));
  window.confirm = () => true;
  const settle = () => new Promise(resolve => setTimeout(resolve, 120)).then(() => studio.settle());
  await settle();
  const root = document.querySelector('#tree summary');
  root.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10})); await settle();
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent.includes('Открыть интерфейс клиентского приложения')).click(); await settle();
  const host = document.querySelector('#client-interface');
  const rows = () => [...host.querySelectorAll('.client-interface-row')].map(row => row.textContent + (row.classList.contains('current') ? ' *' : ''));
  const listed = () => [...host.querySelectorAll('.client-interface-list .object-window-list-row')].map(row => row.textContent + (row.querySelector('.client-interface-placed') ? ' ■' : '') + (row.classList.contains('current') ? ' *' : ''));
  const buttons = () => [...host.querySelectorAll('.client-interface-tree .object-window-panel button')].map(item => item.title + (item.disabled ? ' -' : ''));
  const button = title => [...host.querySelectorAll('.client-interface-tree .object-window-panel button')].find(item => item.title === title);
  const row = (text, at = 0) => [...host.querySelectorAll('.client-interface-row')].filter(item => item.textContent === text)[at];
  const pick = async (text, at) => { row(text, at).dispatchEvent(new MouseEvent('mousedown', {bubbles: true})); await settle(); };
  const dialog = () => document.querySelector('dialog[open]');
  const press = text => [...dialog().querySelectorAll('button')].find(item => item.textContent === text).click();
  const choices = () => [...dialog().querySelectorAll('.object-window-list-row')].map(item => item.textContent + (item.querySelector('.client-interface-placed') ? ' ■' : '') + (item.classList.contains('current') ? ' *' : ''));
  const choose = async text => { [...dialog().querySelectorAll('.object-window-list-row')].find(item => item.textContent === text).click(); await settle(); };
  const palette = () => [document.querySelector('#inspector-title').textContent, ...[...document.querySelectorAll('#properties .object-palette-row')].map(line =>
    line.querySelector('.object-palette-caption').textContent + ': ' + (line.querySelector('select')?.selectedOptions[0].textContent ?? line.querySelector('input')?.value ?? '') + (line.querySelector('input:disabled, select:disabled') ? ' -' : ''))];
  steps.title = document.querySelector('#file-path').textContent;
  steps.opened = rows();
  steps.list = listed();
  steps.atSide = buttons();
  await pick('Панель открытых');
  steps.atPanel = buttons();
  row('Панель открытых').dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10})); await settle();
  steps.menu = [...document.querySelectorAll('.context-menu .context-menu-item')].map(item => item.textContent + (item.disabled || item.classList.contains('disabled') ? ' -' : ''));
  document.dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape', bubbles: true})); await settle();
  closeContextMenu?.();
  button('Переместить вверх').click(); await settle();
  steps.up = rows();
  steps.atFirst = buttons();
  await pick('Группа');
  button('Добавить (Ins)').click(); await settle();
  steps.type = [dialog().querySelector('h3').textContent, ...choices()];
  press('OK'); await settle();
  steps.added = rows();
  steps.addedPalette = palette();
  const dots = document.querySelector('#properties .field-choice');
  if (!dots) return steps;
  dots.click(); await settle();
  steps.choice = [dialog().querySelector('h3').textContent, ...choices()];
  await choose('Панель разделов');
  steps.placedOK = [...dialog().querySelectorAll('button')].find(item => item.textContent === 'OK').disabled;
  await choose('Панель истории');
  press('OK'); await settle();
  steps.chosen = rows();
  steps.chosenList = listed();
  await pick('Право');
  button('Добавить (Ins)').click(); await settle();
  await choose('Группа'); press('OK'); await settle();
  steps.group = rows();
  button('Удалить текущий (Del)').click(); await settle();
  steps.deleted = rows();
  await pick('Низ');
  button('Добавить (Ins)').click(); await settle();
  await choose('Группа'); press('OK'); await settle();
  button('Удалить текущий (Del)').click(); await settle();
  steps.middle = rows();
  [...host.querySelectorAll('.client-interface-list .object-window-list-row')].find(item => item.textContent.startsWith('Панель разделов')).click(); await settle();
  steps.definition = palette();
  const shown = document.querySelector('#properties select');
  shown.value = 'picture-on-top-and-text'; shown.dispatchEvent(new Event('change', {bubbles: true})); await settle();
  steps.represented = palette();
  [...host.querySelectorAll('.client-interface-list .object-window-list-row')].find(item => item.textContent.startsWith('Панель открытых')).click(); await settle();
  steps.other = palette();
  await pick('Панель истории');
  host.querySelector('.client-interface-rows').dispatchEvent(new KeyboardEvent('keydown', {key: 'F12', bubbles: true})); await settle();
  steps.goneTo = listed();
  steps.dirty = !document.querySelector('#save').disabled;
  document.querySelector('#save').click(); await settle();
  steps.saved = document.querySelector('#save').disabled;
  steps.status = document.querySelector('#status').textContent;
  return steps;
})()`

// erpClientInterface is the interface of erp in the words of the model: at
// the top a group of the panel the platform's list does not have and the
// panel of open windows, at the left a group of the panel of sections, the
// five panels defined.
const erpClientInterface = `top:
  - id: 1b4042b7-6a75-4b3a-a7cd-e5306844686c
    items:
      - items: [{id: 1c6905f0-bef8-4e56-958c-25c457268b87, panel: 8e10648b-f52d-4ec2-b4dd-87de33778d95}]
      - items: [{id: 074990e2-40ee-43fa-9a09-eeab8933d830, panel: cbab57f2-a0f3-4f0a-89ea-4cb19570ab75}]
left:
  - id: 5e17de8d-d23a-4fad-ba89-943c4cc9deb6
    items:
      - items: [{id: de5a40ef-33e7-45fb-a690-73fc69670243, panel: b553047f-c9aa-4157-978d-448ecad24248}]
panels:
  - {id: b553047f-c9aa-4157-978d-448ecad24248}
  - {id: 13322b22-3960-4d68-93a6-fe2dd7f28ca3}
  - {id: c933ac92-92cd-459d-81cc-e0c8a83ced99}
  - {id: cbab57f2-a0f3-4f0a-89ea-4cb19570ab75}
  - {id: b2735bd3-d822-4430-ba59-c9e869693b24}
`

// The window «Интерфейс клиентского приложения» is the prototype's
// (konfiguraciya.md, «Окна», «Досъёмка 27.10.2026», «Предсъёмка
// 28.10.2026», «Досъёмка 29.10.2026, вечер»; snapshots konfiguraciya/23,
// series-2.595/53-57, 71, 72, dosyomka-2646/45-62): on the interface of erp
// the sides with their groups and panels - the panel the list has not is a
// row of no caption - and the five panels with the marks of the placed;
// the panel and its availability on a side and on a panel; the menu of a
// row; up within the group, unavailable then at its top; «Добавить» on a
// group - «Тип элемента», the panel at the end of the group, «Стандартная
// панель», its palette «Не выбрана»; «Выбор стандартной панели» with OK
// unavailable on a panel placed; a group at the end of the right, deleted;
// a row deleted in the middle making the next one current; «Описание
// панели» of the panel of sections with «Отображение» and of another
// without; «Перейти»; «Сохранить» writes the interface. Defects
// caught: the nameless wrappers shown; a panel added elsewhere; a placed
// panel chosen twice; the arrows going out of the group or round; the mark
// not following the choice; «Отображение» on every panel; the
// representation or the panels lost in the file.
func TestTheClientInterfaceWindowIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	if err := os.WriteFile(filepath.Join(root, project.ClientInterfaceFile), []byte(erpClientInterface), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, clientInterfaceWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Title, Status                                                         string
		Opened, List, AtSide, AtPanel, Menu, Up, AtFirst, Type, Added, Middle []string
		AddedPalette, Choice, Chosen, ChosenList, Group, Deleted, Definition  []string
		Represented, Other, GoneTo                                            []string
		PlacedOK, Dirty, Saved                                                bool
		Errors                                                                []string
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
	expect("the errors of the page", steps.Errors, []string{})
	expect("the title", steps.Title, "Интерфейс клиентского приложения")
	expect("opened", steps.Opened, []string{"Верх *", "Группа", "", "Панель открытых", "Низ", "Лево", "Группа", "Панель разделов", "Право"})
	expect("the list", steps.List, []string{"Панель разделов ■", "Панель избранного", "Панель истории", "Панель открытых ■", "Панель функций текущего раздела"})
	expect("the panel at a side", steps.AtSide, []string{"Добавить (Ins)", "Изменить текущий элемент (F2)", "Удалить текущий (Del) -", "Переместить вверх -", "Переместить вниз -"})
	expect("the panel at a panel", steps.AtPanel, []string{"Добавить (Ins) -", "Изменить текущий элемент (F2)", "Удалить текущий (Del)", "Переместить вверх", "Переместить вниз -"})
	expect("the menu of a row", steps.Menu, []string{"ДобавитьIns -", "ИзменитьF2", "УдалитьDel", "Переместить вверхCmd+Shift+Up", "Переместить внизCmd+Shift+Down -", "ПерейтиF12", "СвойстваOption+Enter"})
	expect("up", steps.Up, []string{"Верх", "Группа", "Панель открытых *", "", "Низ", "Лево", "Группа", "Панель разделов", "Право"})
	expect("at the top of the group", steps.AtFirst, []string{"Добавить (Ins) -", "Изменить текущий элемент (F2)", "Удалить текущий (Del)", "Переместить вверх -", "Переместить вниз"})
	expect("«Тип элемента»", steps.Type, []string{"Тип элемента", "Стандартная панель *", "Группа"})
	expect("added to the group", steps.Added, []string{"Верх", "Группа", "Панель открытых", "", "Стандартная панель *", "Низ", "Лево", "Группа", "Панель разделов", "Право"})
	expect("its palette", steps.AddedPalette, []string{"Свойства: Стандартная панель", "Стандартная панель: Не выбрана"})
	expect("«Выбор стандартной панели»", steps.Choice, []string{"Выбор стандартной панели", "Панель разделов ■ *", "Панель избранного", "Панель истории", "Панель открытых ■", "Панель функций текущего раздела"})
	expect("OK on a panel placed", steps.PlacedOK, true)
	expect("chosen", steps.Chosen, []string{"Верх", "Группа", "Панель открытых", "", "Панель истории *", "Низ", "Лево", "Группа", "Панель разделов", "Право"})
	expect("the list after the choice", steps.ChosenList, []string{"Панель разделов ■", "Панель избранного", "Панель истории ■", "Панель открытых ■", "Панель функций текущего раздела"})
	expect("a group at the right", steps.Group, []string{"Верх", "Группа", "Панель открытых", "", "Панель истории", "Низ", "Лево", "Группа", "Панель разделов", "Право", "Группа *"})
	expect("deleted", steps.Deleted, []string{"Верх", "Группа", "Панель открытых", "", "Панель истории", "Низ", "Лево", "Группа", "Панель разделов", "Право *"})
	expect("deleted in the middle", steps.Middle, []string{"Верх", "Группа", "Панель открытых", "", "Панель истории", "Низ", "Лево *", "Группа", "Панель разделов", "Право"})
	expect("«Описание панели» of the panel of sections", steps.Definition, []string{"Свойства: Описание панели", "Имя: ПанельРазделов -", "Отображение: Картинка и текст"})
	expect("represented", steps.Represented, []string{"Свойства: Описание панели", "Имя: ПанельРазделов -", "Отображение: Текст под картинкой"})
	expect("«Описание панели» of another", steps.Other, []string{"Свойства: Описание панели", "Имя: ПанельОткрытых -"})
	expect("«Перейти»", steps.GoneTo, []string{"Панель разделов ■", "Панель избранного", "Панель истории ■ *", "Панель открытых ■", "Панель функций текущего раздела"})
	expect("changed", steps.Dirty, true)
	expect("saved", steps.Saved, true)
	file, err := os.Open(filepath.Join(root, project.ClientInterfaceFile))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	read, err := metadata.DecodeRootDescription(project.ClientInterfaceFile, file)
	if err != nil {
		t.Fatalf("the interface written is not read: %v", err)
	}
	value := read.(metadata.ClientInterface)
	group := value.Top[0].Items
	switch {
	case len(group) != 3 || group[0].Items[0].Panel.String() != "cbab57f2-a0f3-4f0a-89ea-4cb19570ab75" ||
		group[1].Items[0].Panel.String() != "8e10648b-f52d-4ec2-b4dd-87de33778d95" || group[2].Items[0].Panel.String() != "c933ac92-92cd-459d-81cc-e0c8a83ced99":
		t.Errorf("the group of the top was written otherwise: %+v", group)
	case len(value.Right) != 0 || len(value.Left) != 1:
		t.Errorf("the sides were written otherwise: %+v %+v", value.Left, value.Right)
	case len(value.Panels) != 5 || value.Panels[0].PanelRepresentation != metadata.SectionsPanelPictureOnTopAndText:
		t.Errorf("the panels defined were written otherwise: %+v", value.Panels)
	}
}
