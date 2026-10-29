package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// subsystemMenuWalk reads the menu of the subsystem Продажи, opens its
// «Добавить ▸» and makes a subordinate subsystem; then moves Склад out of
// Продажи to the root by «Переместить подсистему».
const subsystemMenuWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 80)).then(() => studio.settle());
  const row = text => [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.querySelector(':scope > span:not(.studio-tree-sign)')?.textContent === text);
  const menu = async text => { row(text).dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10})); await settle(); };
  const items = within => [...within.children].filter(item => item.matches('.context-menu-item, .context-menu-separator'))
    .map(item => item.matches('.context-menu-separator') ? '|' : [...item.childNodes].find(child => child.nodeType === 3).textContent + (item.disabled ? ' (недоступна)' : ''));
  await openObject('Подсистемы');
  await menu('Продажи');
  steps.menu = items(document.querySelector('.context-menu')).slice(0, 9);
  [...document.querySelectorAll('.context-menu-item')].find(item => [...item.childNodes].find(child => child.nodeType === 3).textContent === 'Добавить').click(); await settle();
  steps.submenu = items(document.querySelector('.context-submenu'));
  [...document.querySelectorAll('.context-submenu .context-menu-item')].find(item => item.textContent === 'Подчиненная подсистема').click(); await settle();
  steps.created = document.querySelector('#subsystem-editor .object-window-title')?.textContent || '';
  await menu('Склад');
  [...document.querySelectorAll('.context-menu-item')].find(item => [...item.childNodes].find(child => child.nodeType === 3).textContent === 'Переместить подсистему').click(); await settle();
  const dialog = document.querySelector('dialog.subsystem-move-dialog');
  steps.window = {title: dialog.querySelector('h3').textContent, rows: [...dialog.querySelectorAll('.studio-tree-row')].map(item => item.textContent),
    chosen: dialog.querySelector('.studio-tree-row.selected')?.textContent || ''};
  dialog.querySelector('.studio-tree-row').click(); await settle();
  [...dialog.querySelectorAll('button')].find(item => item.textContent === 'OK').click(); await settle();
  return steps;
})()`

// The menu of a subsystem is the prototype's (docs/block-00/podsistema.md,
// «Команды»; snapshots podsistema/15, 16): «Открыть командный интерфейс»,
// «Объекты подсистемы», «Добавить ▸» of «Подчиненная
// подсистема» and «Подсистема», «Переместить подсистему», «Справочная
// информация» unavailable. «Подчиненная подсистема» of Продажи makes
// «Подсистема1» under it; «Переместить подсистему» of Склад opens «Выбор
// объекта» - the project and the subsystems but Склад, its parent chosen -
// and OK on the project puts it at the root.
// Defects caught: a command or a submenu not the prototype's; a child made
// at the root; a subsystem offered as a place for itself; a move not
// written.
func TestTheMenuOfASubsystemIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addTreeSubsystems(t, root)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, subsystemMenuWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Menu, Submenu []string
		Created       string
		Window        struct {
			Title, Chosen string
			Rows          []string
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
	expect("the menu", steps.Menu, []string{"Открыть командный интерфейс", "Объекты подсистемы", "|", "Добавить", "|",
		"Переместить подсистему", "|", "Справочная информация (недоступна)"})
	expect("«Добавить ▸»", steps.Submenu, []string{"Подчиненная подсистема", "Подсистема"})
	expect("a subordinate subsystem", steps.Created, "Подсистема Подсистема1")
	expect("«Выбор объекта»", steps.Window.Title, "Выбор объекта")
	// The project, Продажи and the subsystem made under it a step before;
	// Склад itself is not offered.
	expect("the window of the move", steps.Window.Rows, []string{"ПродажиИСклад", "Продажи", "Подсистема1"})
	expect("its parent chosen", steps.Window.Chosen, "Продажи")
	entries, _ := os.ReadDir(filepath.Join(root, "metadata", "subsystems"))
	parents := map[string]bool{}
	for _, entry := range entries {
		content, _ := os.ReadFile(filepath.Join(root, "metadata", "subsystems", entry.Name()))
		if strings.Contains(string(content), "name: Подсистема1") {
			parents["child under Продажи"] = strings.Contains(string(content), "parent: "+treeSalesSubsystem)
		}
		if strings.Contains(string(content), "name: Склад") {
			parents["Склад at the root"] = !strings.Contains(string(content), "parent:")
		}
	}
	expect("written", parents, map[string]bool{"child under Продажи": true, "Склад at the root": true})
}

// subsystemObjectsWalk filters the tree by «Объекты подсистемы» of Продажи,
// reads the tree and the window of the filter, then does the same from
// Склад.
const subsystemObjectsWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 80)).then(() => studio.settle());
  const name = item => item.querySelector(':scope > span:not(.studio-tree-sign)')?.textContent || item.textContent;
  const row = text => [...document.querySelectorAll('#tree .studio-tree-row')].find(item => name(item) === text);
  const objects = async text => {
    row(text).dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10})); await settle();
    [...document.querySelectorAll('.context-menu-item')].find(item => [...item.childNodes].find(child => child.nodeType === 3).textContent === 'Объекты подсистемы').click(); await settle();
    return {rows: [...document.querySelectorAll('#tree .studio-tree-row')].map(name), pressed: document.querySelector('#tree-subsystems').getAttribute('aria-pressed')};
  };
  await openObject('Подсистемы');
  steps.sales = await objects('Продажи');
  document.querySelector('#tree-subsystems').click(); await settle();
  const dialog = document.querySelector('dialog.role-subsystem-dialog');
  steps.window = [...dialog.querySelectorAll('label')].map(label => label.textContent + (label.querySelector('input').checked ? ' [x]' : label.querySelector('input').indeterminate ? ' [-]' : ' [ ]'));
  [...dialog.querySelectorAll('button')].find(item => item.textContent === 'Отмена').click(); await settle();
  steps.cancelled = document.querySelector('#tree-subsystems').getAttribute('aria-pressed');
  document.querySelector('#tree-subsystems').click(); await settle();
  [...document.querySelectorAll('dialog.role-subsystem-dialog button')].find(item => item.textContent === 'Отключить').click(); await settle();
  steps.off = document.querySelectorAll('#tree .studio-tree-row').length;
  await openObject('Подсистемы');
  steps.warehouse = await objects('Склад');
  return steps;
})()`

// «Объекты подсистемы» filters the tree of the project by the subsystem
// alone (docs/block-00/podsistema.md, «Командный интерфейс, „Объекты
// подсистемы“, „Все подсистемы“»; snapshots series-2.603/49, 50): what it
// holds stays, by branches, the subsystem itself too; what a subsystem
// beneath it holds does not; the button of the filter stands pressed, and
// its window shows that subsystem marked and both flags off. From Склад,
// nested in Продажи, Продажи stands as the path to it.
// Expectations from the subsystems the test gives the example
// (addTreeSubsystems): Продажи holds Контрагенты and ПродажаТоваров, Склад
// holds Товары.
// Defects caught: the command unavailable; the filter taking in the
// subsystems beneath or above; the filter set and not shown on the button
// or in its window.
func TestTheObjectsOfASubsystemFilterTheTree(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addTreeSubsystems(t, root)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, subsystemObjectsWalk)
	if err != nil {
		t.Fatal(err)
	}
	type tree struct {
		Rows    []string
		Pressed string
	}
	var steps struct {
		Sales, Warehouse tree
		Window           []string
		Cancelled        string
		Off              int
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
	// Склад, its catalog Товары and the register ОстаткиТоваров are not in
	// Продажи; «Товары» below is the tabular section of the document.
	expect("the objects of Продажи", steps.Sales, tree{Rows: []string{"ПродажиИСклад", "Общие", "Подсистемы", "Продажи", "Справочники", "Контрагенты",
		"Реквизиты", "ИНН", "Табличные части", "Формы", "Команды", "Макеты", "Документы", "ПродажаТоваров", "Реквизиты", "Контрагент",
		"Табличные части", "Товары", "Товар", "Количество", "Цена", "Формы", "Команды", "Макеты"}, Pressed: "true"})
	expect("the window of the filter", steps.Window, []string{"ПродажиИСклад [-]", "Продажи [x]", "Склад [-]", "<Не входящие в подсистемы> [ ]",
		"Включать объекты подчиненных подсистем [ ]", "Включать объекты родительских подсистем [ ]"})
	expect("«Отмена» keeps the filter", steps.Cancelled, "true")
	if steps.Off < 50 {
		t.Errorf("«Отключить» left %d rows in the tree", steps.Off)
	}
	expect("the objects of Склад", steps.Warehouse, tree{Rows: []string{"ПродажиИСклад", "Общие", "Подсистемы", "Продажи", "Склад", "Справочники", "Товары",
		"Реквизиты", "Артикул", "Табличные части", "Формы", "Команды", "Макеты"}, Pressed: "true"})
}
