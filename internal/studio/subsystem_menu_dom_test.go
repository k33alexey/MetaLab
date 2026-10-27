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
    .map(item => item.matches('.context-menu-separator') ? '|' : item.childNodes[0].textContent + (item.disabled ? ' (недоступна)' : ''));
  await openObject('Подсистемы');
  await menu('Продажи');
  steps.menu = items(document.querySelector('.context-menu')).slice(0, 9);
  [...document.querySelectorAll('.context-menu-item')].find(item => item.childNodes[0].textContent === 'Добавить').click(); await settle();
  steps.submenu = items(document.querySelector('.context-submenu'));
  [...document.querySelectorAll('.context-submenu .context-menu-item')].find(item => item.textContent === 'Подчиненная подсистема').click(); await settle();
  steps.created = document.querySelector('#subsystem-editor .object-window-title')?.textContent || '';
  await menu('Склад');
  [...document.querySelectorAll('.context-menu-item')].find(item => item.childNodes[0].textContent === 'Переместить подсистему').click(); await settle();
  const dialog = document.querySelector('dialog.subsystem-move-dialog');
  steps.window = {title: dialog.querySelector('h3').textContent, rows: [...dialog.querySelectorAll('.studio-tree-row')].map(item => item.textContent),
    chosen: dialog.querySelector('.studio-tree-row.selected')?.textContent || ''};
  dialog.querySelector('.studio-tree-row').click(); await settle();
  [...dialog.querySelectorAll('button')].find(item => item.textContent === 'OK').click(); await settle();
  return steps;
})()`

// The menu of a subsystem is the prototype's (docs/block-00/podsistema.md,
// «Команды»; snapshots podsistema/15, 16): «Открыть командный интерфейс»
// and «Объекты подсистемы» unavailable, «Добавить ▸» of «Подчиненная
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
	expect("the menu", steps.Menu, []string{"Открыть командный интерфейс (недоступна)", "Объекты подсистемы (недоступна)", "|", "Добавить", "|",
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
