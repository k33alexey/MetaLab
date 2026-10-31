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
	expect("the panel at a panel", steps.AtPanel, []string{"Добавить (Ins)", "Изменить текущий элемент (F2)", "Удалить текущий (Del)", "Переместить вверх", "Переместить вниз -"})
	expect("the menu of a row", steps.Menu, []string{"ДобавитьIns", "ИзменитьF2", "УдалитьDel", "Переместить вверхCmd+Shift+Up", "Переместить внизCmd+Shift+Down -", "ПерейтиF12", "СвойстваOption+Enter"})
	expect("up", steps.Up, []string{"Верх", "Группа", "Панель открытых *", "", "Низ", "Лево", "Группа", "Панель разделов", "Право"})
	expect("at the top of the group", steps.AtFirst, []string{"Добавить (Ins)", "Изменить текущий элемент (F2)", "Удалить текущий (Del)", "Переместить вверх -", "Переместить вниз"})
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

// clientInterfaceDragWalk opens the window on the interface of erp and drags
// panels of the list: the panel of favourites onto the group of the left,
// the panel of history onto the side Право, the panel of functions onto the
// panel of sections, the placed panel of sections onto the side Низ; then
// rows of the tree: the panel of open windows onto Право, the panel none
// chosen onto the panel of favourites, the panel of history onto a group;
// then groups: the group of Лево onto Низ, the group of Верх onto the panel
// of open windows, the group of Право onto a panel of the group of Низ.
const clientInterfaceDragWalk = `(async () => {
  const studio = window.__studio, steps = {};
  steps.errors = []; window.addEventListener('unhandledrejection', event => steps.errors.push(String(event.reason?.stack || event.reason))); window.addEventListener('error', event => steps.errors.push(String(event.message)));
  const settle = () => new Promise(resolve => setTimeout(resolve, 120)).then(() => studio.settle());
  await settle();
  document.querySelector('#tree summary').dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10})); await settle();
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent.includes('Открыть интерфейс клиентского приложения')).click(); await settle();
  const host = document.querySelector('#client-interface');
  const rows = () => [...host.querySelectorAll('.client-interface-row')].map(row => row.textContent + (row.classList.contains('current') ? ' *' : ''));
  const listRow = text => [...host.querySelectorAll('.client-interface-list .object-window-list-row')].find(row => row.textContent === text);
  const listed = () => [...host.querySelectorAll('.client-interface-list .object-window-list-row')].map(row => row.textContent + (row.querySelector('.client-interface-placed') ? ' ■' : '') + (row.draggable ? ' ↕' : ''));
  const row = (text, at = 0) => [...host.querySelectorAll('.client-interface-row')].filter(item => item.textContent === text)[at];
  // A drag from the row source() onto the row target(): answers whether the
  // target took it, and the frame drawn round it while over it.
  const drag = async (source, target) => {
    const data = new DataTransfer();
    source().dispatchEvent(new DragEvent('dragstart', {bubbles: true, cancelable: true, dataTransfer: data}));
    const over = new DragEvent('dragover', {bubbles: true, cancelable: true, dataTransfer: data});
    target().dispatchEvent(over);
    const framed = target().classList.contains('drop-target');
    target().dispatchEvent(new DragEvent('drop', {bubbles: true, cancelable: true, dataTransfer: data}));
    await settle();
    return [over.defaultPrevented, framed];
  };
  steps.list = listed();
  steps.overGroup = await drag(() => listRow('Панель избранного'), () => row('Группа', 1));
  steps.dropped = rows();
  steps.listAfter = listed();
  steps.overSide = await drag(() => listRow('Панель истории'), () => row('Право'));
  steps.onSide = rows();
  steps.overPanel = await drag(() => listRow('Панель функций текущего раздела'), () => row('Панель разделов'));
  steps.onPanel = rows();
  steps.overWithPlaced = await drag(() => listRow('Панель разделов'), () => row('Низ'));
  steps.placed = rows();
  steps.rowOnSide = await drag(() => row('Панель открытых'), () => row('Право'));
  steps.movedOnSide = rows();
  steps.rowOnPanel = await drag(() => row('Стандартная панель'), () => row('Панель избранного'));
  steps.movedOnPanel = rows();
  steps.rowOnGroup = await drag(() => row('Панель истории'), () => row('Группа', 1));
  steps.movedOnGroup = rows();
  steps.groupOnSide = await drag(() => row('Группа', 1), () => row('Низ'));
  steps.groupMovedOnSide = rows();
  steps.groupOnPanel = await drag(() => row('Группа', 0), () => row('Панель открытых'));
  steps.groupMovedOnPanel = rows();
  steps.groupOnPanelOfGroup = await drag(() => row('Группа', 1), () => row('Панель разделов'));
  steps.notTaken = rows();
  steps.groupDragged = row('Группа').draggable;
  steps.panelDragged = row('Панель истории').draggable;
  steps.listEnd = listed();
  steps.dirty = !document.querySelector('#save').disabled;
  return steps;
})()`

// Drags in «Интерфейс клиентского приложения» are the prototype's
// (konfiguraciya.md, «Досъёмка 31.10.2026, перетаскивание»; series-2.595/71,
// dosyomka-2667/18-28): a panel of the list stands where «Добавить» puts
// one at the row it is dropped onto - at the end of a group, at the end of a
// side on the side itself, before a panel in its group - current, the list
// marking it placed; a panel placed already puts «Стандартная панель» there
// and stays; a panel of the tree moves - onto a side to its end, onto a
// panel before it, onto a group to its end - and its old place goes; a
// group of the tree moves onto a side to its end, onto a panel of a side
// before it, with its panels, and onto a panel of a group it does not go,
// the frame drawn all the same (configurator-2026-10-31/vopr-2672/04-13);
// a black frame round the row under the drag.
// Defects caught: the panel put first, into a group on a side, after the
// panel dropped onto; the row current left where it was; the mark not
// following; a placed panel standing twice or refused; a row of the tree
// copied, not moved, or refused by a group; a group not dragged, put
// after the panel, without its panels or into a group; a panel of the
// tree not dragged; no frame.
func TestAPanelDraggedStandsWhereThePrototypePutsIt(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	if err := os.WriteFile(filepath.Join(root, project.ClientInterfaceFile), []byte(erpClientInterface), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, clientInterfaceDragWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		List, Dropped, ListAfter, OnSide, OnPanel, Placed, MovedOnSide, MovedOnPanel, MovedOnGroup, NotTaken, ListEnd []string
		GroupMovedOnSide, GroupMovedOnPanel                                                                           []string
		OverGroup, OverSide, OverPanel, OverWithPlaced, RowOnSide, RowOnPanel, RowOnGroup                             []bool
		GroupOnSide, GroupOnPanel, GroupOnPanelOfGroup                                                                []bool
		GroupDragged, PanelDragged, Dirty                                                                             bool
		Errors                                                                                                        []string
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
	taken := []bool{true, true}
	expect("the errors of the page", steps.Errors, []string{})
	expect("the list", steps.List, []string{"Панель разделов ■ ↕", "Панель избранного ↕", "Панель истории ↕", "Панель открытых ■ ↕", "Панель функций текущего раздела ↕"})
	expect("a group takes the drag", steps.OverGroup, taken)
	expect("dropped onto the group", steps.Dropped, []string{"Верх", "Группа", "", "Панель открытых", "Низ", "Лево", "Группа", "Панель разделов", "Панель избранного *", "Право"})
	expect("the list after the drop", steps.ListAfter, []string{"Панель разделов ■ ↕", "Панель избранного ■ ↕", "Панель истории ↕", "Панель открытых ■ ↕", "Панель функций текущего раздела ↕"})
	expect("a side takes the drag", steps.OverSide, taken)
	expect("dropped onto a side", steps.OnSide, []string{"Верх", "Группа", "", "Панель открытых", "Низ", "Лево", "Группа", "Панель разделов", "Панель избранного", "Право", "Панель истории *"})
	expect("a panel takes the drag", steps.OverPanel, taken)
	expect("dropped onto a panel", steps.OnPanel, []string{"Верх", "Группа", "", "Панель открытых", "Низ", "Лево", "Группа", "Панель функций текущего раздела *", "Панель разделов", "Панель избранного", "Право", "Панель истории"})
	expect("a placed panel taken", steps.OverWithPlaced, taken)
	expect("a placed panel dropped", steps.Placed, []string{"Верх", "Группа", "", "Панель открытых", "Низ", "Стандартная панель *", "Лево", "Группа", "Панель функций текущего раздела", "Панель разделов", "Панель избранного", "Право", "Панель истории"})
	expect("a row taken by a side", steps.RowOnSide, taken)
	expect("a row moved onto a side", steps.MovedOnSide, []string{"Верх", "Группа", "", "Низ", "Стандартная панель", "Лево", "Группа", "Панель функций текущего раздела", "Панель разделов", "Панель избранного", "Право", "Панель истории", "Панель открытых *"})
	expect("a row taken by a panel", steps.RowOnPanel, taken)
	expect("a row moved onto a panel", steps.MovedOnPanel, []string{"Верх", "Группа", "", "Низ", "Лево", "Группа", "Панель функций текущего раздела", "Панель разделов", "Стандартная панель *", "Панель избранного", "Право", "Панель истории", "Панель открытых"})
	expect("a row taken by a group", steps.RowOnGroup, taken)
	expect("a row moved onto a group", steps.MovedOnGroup, []string{"Верх", "Группа", "", "Низ", "Лево", "Группа", "Панель функций текущего раздела", "Панель разделов", "Стандартная панель", "Панель избранного", "Панель истории *", "Право", "Панель открытых"})
	expect("a group taken by a side", steps.GroupOnSide, taken)
	expect("a group moved onto a side", steps.GroupMovedOnSide, []string{"Верх", "Группа", "", "Низ", "Группа *", "Панель функций текущего раздела", "Панель разделов", "Стандартная панель", "Панель избранного", "Панель истории", "Лево", "Право", "Панель открытых"})
	expect("a group taken by a panel of a side", steps.GroupOnPanel, taken)
	expect("a group moved onto a panel of a side", steps.GroupMovedOnPanel, []string{"Верх", "Низ", "Группа", "Панель функций текущего раздела", "Панель разделов", "Стандартная панель", "Панель избранного", "Панель истории", "Лево", "Право", "Группа *", "", "Панель открытых"})
	expect("a panel of a group framed under a group", steps.GroupOnPanelOfGroup, taken)
	expect("nothing moved by a group onto a panel of a group", steps.NotTaken, steps.GroupMovedOnPanel)
	expect("a group dragged", steps.GroupDragged, true)
	expect("a panel of the tree dragged", steps.PanelDragged, true)
	expect("the list at the end", steps.ListEnd, []string{"Панель разделов ■ ↕", "Панель избранного ■ ↕", "Панель истории ■ ↕", "Панель открытых ■ ↕", "Панель функций текущего раздела ■ ↕"})
	expect("changed", steps.Dirty, true)
}

// clientInterfaceDefaultWalk opens the window on a project writing no
// interface, then adds before the current rows: a panel at the panel of
// functions, a group at the panel of sections, a panel into that group and
// a panel at that panel - reading «Тип элемента» there - and saves.
const clientInterfaceDefaultWalk = `(async () => {
  const studio = window.__studio, steps = {};
  steps.errors = []; window.addEventListener('unhandledrejection', event => steps.errors.push(String(event.reason?.stack || event.reason))); window.addEventListener('error', event => steps.errors.push(String(event.message)));
  const settle = () => new Promise(resolve => setTimeout(resolve, 120)).then(() => studio.settle());
  await settle();
  document.querySelector('#tree summary').dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10})); await settle();
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent.includes('Открыть интерфейс клиентского приложения')).click(); await settle();
  const host = document.querySelector('#client-interface');
  const rows = () => [...host.querySelectorAll('.client-interface-row')].map(row => row.textContent + (row.classList.contains('current') ? ' *' : ''));
  const listed = () => [...host.querySelectorAll('.client-interface-list .object-window-list-row')].map(row => row.textContent + (row.querySelector('.client-interface-placed') ? ' ■' : ''));
  const row = (text, at = 0) => [...host.querySelectorAll('.client-interface-row')].filter(item => item.textContent === text)[at];
  const pick = async (text, at) => { row(text, at).dispatchEvent(new MouseEvent('mousedown', {bubbles: true})); await settle(); };
  const dialog = () => document.querySelector('dialog[open]');
  const types = () => [...dialog().querySelectorAll('.object-window-list-row')].map(item => item.textContent);
  const add = async kind => {
    [...host.querySelectorAll('.client-interface-tree .object-window-panel button')].find(item => item.title === 'Добавить (Ins)').click(); await settle();
    const offered = types();
    [...dialog().querySelectorAll('.object-window-list-row')].find(item => item.textContent === kind).click(); await settle();
    [...dialog().querySelectorAll('button')].find(item => item.textContent === 'OK').click(); await settle();
    return offered;
  };
  steps.opened = rows();
  steps.list = listed();
  steps.clean = document.querySelector('#save').disabled;
  await pick('Панель функций текущего раздела');
  steps.atSidePanel = await add('Стандартная панель');
  steps.panelBefore = rows();
  await pick('Панель разделов');
  await add('Группа');
  steps.groupBefore = rows();
  await add('Стандартная панель');
  steps.atGroupPanel = await add('Стандартная панель');
  steps.inGroupBefore = rows();
  document.querySelector('#save').click(); await settle();
  steps.saved = document.querySelector('#save').disabled;
  return steps;
})()`

// A project writing no interface shows the platform's default one - at the
// top the panel of sections and the panel of functions of the current
// section on the side itself - unchanged until edited; «Добавить» at an
// element puts the new one before it, «Тип элемента» offering the panel
// alone at a panel of a group; the file written defines the five panels of
// the platform (konfiguraciya.md, «Досъёмка 29.10.2026, ночь»;
// dosyomka-2649/01-09). Defects caught: an empty window for a project with
// no file, the window changed by opening it, the new element put after the
// current or at the end, a group offered into a group, the definitions of
// the panels not placed left out of the file.
func TestTheDefaultClientInterfaceAndAddingBeforeTheCurrent(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, clientInterfaceDefaultWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Opened, List, AtSidePanel, PanelBefore, GroupBefore, AtGroupPanel, InGroupBefore []string
		Clean, Saved                                                                     bool
		Errors                                                                           []string
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
	expect("opened", steps.Opened, []string{"Верх *", "Панель разделов", "Панель функций текущего раздела", "Низ", "Лево", "Право"})
	expect("the list", steps.List, []string{"Панель разделов ■", "Панель избранного", "Панель истории", "Панель открытых", "Панель функций текущего раздела ■"})
	expect("not changed by opening", steps.Clean, true)
	expect("«Тип элемента» at a panel of a side", steps.AtSidePanel, []string{"Стандартная панель", "Группа"})
	expect("a panel before a panel of the side", steps.PanelBefore, []string{"Верх", "Панель разделов", "Стандартная панель *", "Панель функций текущего раздела", "Низ", "Лево", "Право"})
	expect("a group before a panel of the side", steps.GroupBefore, []string{"Верх", "Группа *", "Панель разделов", "Стандартная панель", "Панель функций текущего раздела", "Низ", "Лево", "Право"})
	expect("«Тип элемента» at a panel of a group", steps.AtGroupPanel, []string{"Стандартная панель"})
	expect("a panel before a panel of the group", steps.InGroupBefore, []string{"Верх", "Группа", "Стандартная панель *", "Стандартная панель", "Панель разделов", "Стандартная панель", "Панель функций текущего раздела", "Низ", "Лево", "Право"})
	expect("saved", steps.Saved, true)
	file, err := os.Open(filepath.Join(root, project.ClientInterfaceFile))
	if err != nil {
		t.Fatalf("the interface was not written: %v", err)
	}
	defer file.Close()
	read, err := metadata.DecodeRootDescription(project.ClientInterfaceFile, file)
	if err != nil {
		t.Fatalf("the interface written is not read: %v", err)
	}
	value := read.(metadata.ClientInterface)
	var defined []string
	for _, panel := range value.Panels {
		defined = append(defined, panel.ID.String())
	}
	expect("the panels defined", defined, []string{"b553047f-c9aa-4157-978d-448ecad24248", "13322b22-3960-4d68-93a6-fe2dd7f28ca3",
		"c933ac92-92cd-459d-81cc-e0c8a83ced99", "cbab57f2-a0f3-4f0a-89ea-4cb19570ab75", "b2735bd3-d822-4430-ba59-c9e869693b24"})
	if len(value.Top) != 4 || value.Top[0].ID == nil || len(value.Top[0].Items) != 2 || value.Top[1].Panel == nil || value.Top[1].ID == nil ||
		value.Top[1].Panel.String() != "b553047f-c9aa-4157-978d-448ecad24248" || value.Top[3].Panel.String() != "b2735bd3-d822-4430-ba59-c9e869693b24" {
		t.Errorf("the top was written as %+v", value.Top)
	}
}
