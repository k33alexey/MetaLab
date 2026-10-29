package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// subsystemOverviewWalk opens «Все подсистемы» from the menu of the branch
// «Подсистемы», reads the tree and the panes before a click, clicks Склад,
// reads its content and command interface, takes Товары out of its content,
// adds Контрагенты by «Редактировать состав…», takes a command's box off,
// saves; then moves Склад to the root by «Переместить подсистему».
const subsystemOverviewWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 150)).then(() => studio.settle());
  const host = document.querySelector('#subsystem-overview');
  [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === 'Подсистемы').dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10})); await settle();
  steps.branchMenu = [...document.querySelector('.context-menu').children].filter(item => item.matches('.context-menu-item')).map(item => item.childNodes[0].textContent);
  [...document.querySelectorAll('.context-menu-item')].find(item => item.childNodes[0].textContent === 'Все подсистемы').click(); await settle();
  const rows = selector => [...host.querySelectorAll(selector + ' .studio-tree-row')].map(item => item.textContent + (item.classList.contains('selected') ? ' *' : ''));
  const buttons = within => [...within.querySelectorAll('.object-window-panel button')].map(item => item.title + (item.disabled ? ' (недоступна)' : ''));
  const panels = host.querySelectorAll('.subsystem-overview-left > div:not(.object-window-tree):not(.subsystem-overview-caption)');
  steps.title = document.querySelector('#file-path').textContent; steps.status = document.querySelector('#status').textContent;
  steps.captions = [...host.querySelectorAll('.subsystem-overview-caption')].map(item => item.textContent);
  steps.tree = rows('.subsystem-overview-tree');
  steps.panel = buttons(panels[0]);
  steps.before = {composition: rows('.subsystem-overview-composition'), commands: host.querySelectorAll('.subsystem-overview-commands .studio-tree-row').length};
  [...host.querySelectorAll('.subsystem-overview-tree .studio-tree-row')].find(item => item.textContent === 'Склад').click(); await settle();
  steps.composition = rows('.subsystem-overview-composition');
  steps.compositionPanel = buttons(panels[1]);
  steps.commands = [...host.querySelectorAll('.subsystem-overview-commands .studio-tree-leaf')].map(item => item.textContent);
  host.querySelector('.subsystem-overview-composition summary')?.click(); await settle();
  [...host.querySelectorAll('.subsystem-overview-composition .studio-tree-row')].find(item => item.textContent === 'Товары').click(); await settle();
  steps.chosenPanel = buttons(panels[1]);
  panels[1].querySelectorAll('button')[1].click(); await settle();
  steps.removed = rows('.subsystem-overview-composition');
  steps.dirty = !document.querySelector('#save').disabled;
  panels[1].querySelectorAll('button')[0].click(); await settle();
  const dialog = document.querySelector('dialog.object-choice-dialog');
  steps.choice = {title: dialog.querySelector('h3').textContent, open: [...dialog.querySelectorAll('details[open] > summary .studio-tree-row, details[open] > .studio-tree-row')].map(item => item.textContent).slice(0, 3)};
  [...dialog.querySelectorAll('.studio-tree-row')].find(item => item.textContent === 'Контрагенты').querySelector('input').click();
  [...dialog.querySelectorAll('button')].find(item => item.textContent === 'ОК').click(); await settle();
  steps.added = rows('.subsystem-overview-composition');
  const box = [...host.querySelectorAll('.subsystem-overview-commands .studio-tree-leaf')].find(item => item.textContent === 'Контрагенты')?.querySelector('input');
  steps.box = box ? box.checked : null;
  box?.click(); await settle();
  document.querySelector('#save').click(); await settle();
  steps.saved = document.querySelector('#save').disabled;
  steps.after = rows('.subsystem-overview-tree');
  [...host.querySelectorAll('.subsystem-overview-tree .studio-tree-row')].find(item => item.textContent === 'Продажи').click(); await settle();
  const removeButton = [...panels[0].querySelectorAll('button')].find(item => item.title === 'Удалить текущий элемент');
  steps.deleteOn = !removeButton.disabled;
  [...host.querySelectorAll('.subsystem-overview-tree .studio-tree-row')].find(item => item.textContent === 'Склад').click(); await settle();
  [...panels[0].querySelectorAll('button')].find(item => item.title === 'Переместить подсистему').click(); await settle();
  const move = document.querySelector('dialog.subsystem-move-dialog');
  move.querySelector('.studio-tree-row').click(); await settle();
  [...move.querySelectorAll('button')].find(item => item.textContent === 'OK').click(); await settle(); await settle();
  steps.moved = rows('.subsystem-overview-tree');
  return steps;
})()`

// «Все подсистемы» is the prototype's (docs/block-00/podsistema.md,
// «Командный интерфейс, „Объекты подсистемы“, „Все подсистемы“»,
// «Предсъёмка 28.10.2026»; snapshots series-2.603/51, 52,
// predsyomka-1/01-30): the command of the branch; at the left the tree of
// the subsystems in the command interface - Сервис, not in it, stays out -
// with its panel and the first row current, «Состав» and the command
// interface empty until a click; «Состав» of the row clicked with its two
// buttons, the deletion unavailable until an object is chosen and taking it
// away with no question; «Редактировать состав…» - «Выбор объекта» open
// down its first branch - adding what is marked; the command interface of
// the content the window holds; «Сохранить» writing the subsystems changed.
// Defects caught: a subsystem out of the command interface in the tree; the
// panes filled before a click; a deletion from the content not written, or
// written to another subsystem; the command interface read for the content
// saved rather than the one shown; up and down acting before the order of
// the objects of the tree is written (the item «порядок объектов»); a
// subsystem not moved by «Переместить подсистему», or the row clicked lost.
func TestAllSubsystemsIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	directory := filepath.Join(root, "metadata", "subsystems")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	const service = "30000000-0000-4000-8000-000000000303"
	salesBody := "format: 1\nid: " + treeSalesSubsystem + "\nname: Продажи\ntitle: {ru: Продажи}\ninclude_in_command_interface: true\nmembers:\n  - 10000000-0000-4000-8000-000000000102\n"
	parties := filepath.Join(root, "metadata", "catalogs", "Контрагенты", "object.yaml")
	content, err := os.ReadFile(parties)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parties, append(content, "use_standard_commands: true\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	for id, body := range map[string]string{
		treeSalesSubsystem: salesBody,
		treeWarehouseSubsystem: "format: 1\nid: " + treeWarehouseSubsystem + "\nname: Склад\ntitle: {ru: Склад}\nparent: " + treeSalesSubsystem +
			"\ninclude_in_command_interface: true\nmembers:\n  - 10000000-0000-4000-8000-000000000101\n",
		service: "format: 1\nid: " + service + "\nname: Сервис\ntitle: {ru: Сервис}\n",
	} {
		if err := os.WriteFile(filepath.Join(directory, id+".yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, subsystemOverviewWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		BranchMenu, Captions, Tree, Panel, Composition, CompositionPanel, Commands, ChosenPanel, Removed, Added, After []string
		Title                                                                                                          string
		Before                                                                                                         struct {
			Composition []string
			Commands    int
		}
		Choice struct {
			Title string
			Open  []string
		}
		Dirty, Saved, DeleteOn bool
		Moved                  []string
		Box                    *bool
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
	expect("the menu of the branch", steps.BranchMenu[:1], []string{"Все подсистемы"})
	expect("the title", steps.Title, "Все подсистемы")
	expect("the captions", steps.Captions, []string{"Подсистемы", "Состав", "Командный интерфейс"})
	expect("the tree", steps.Tree, []string{"Продажи *", "Склад"})
	expect("the panel", steps.Panel, []string{"Добавить новый элемент", "Изменить (или открыть) текущий элемент", "Удалить текущий элемент",
		"Переместить текущий элемент вверх (недоступна)", "Переместить текущий элемент вниз (недоступна)", "Переместить подсистему"})
	expect("before a click", steps.Before, struct {
		Composition []string
		Commands    int
	}{[]string{}, 0})
	expect("«Состав» of Склад", steps.Composition, []string{"Справочники", "Товары"})
	expect("the panel of «Состав»", steps.CompositionPanel, []string{"Редактировать состав объектов метаданных, входящих в подсистему",
		"Удалить объект метаданных из состава подсистемы (недоступна)"})
	expect("an object chosen", steps.ChosenPanel, []string{"Редактировать состав объектов метаданных, входящих в подсистему",
		"Удалить объект метаданных из состава подсистемы"})
	expect("taken away", steps.Removed, []string{})
	expect("changed", steps.Dirty, true)
	expect("«Выбор объекта»", steps.Choice.Title, "Выбор объекта")
	expect("«Выбор объекта» open down its first branch", len(steps.Choice.Open) >= 2, true)
	expect("added", steps.Added, []string{"Справочники", "Контрагенты"})
	expect("the command of Контрагенты, visible by default", steps.Box != nil && *steps.Box, true)
	expect("saved", steps.Saved, true)
	expect("delete on Продажи", steps.DeleteOn, true)
	// «Переместить подсистему» - the window of the menu, the root chosen:
	// Склад stands in the root, current still (owner, 29.10.2026: the
	// prototype's button did not act, its meaning is the menu's).
	expect("moved to the root", steps.Moved, []string{"Продажи", "Склад *"})
	description, _ := os.ReadFile(filepath.Join(directory, treeWarehouseSubsystem+".yaml"))
	if !strings.Contains(string(description), "members:\n  - 10000000-0000-4000-8000-000000000102\n") || strings.Contains(string(description), "000000000101") {
		t.Errorf("the content of Склад is not written:\n%s", description)
	}
	if strings.Contains(string(description), "\nparent:") {
		t.Errorf("Склад is not moved to the root:\n%s", description)
	}
	if !strings.Contains(string(description), "\ncommand_interface:") {
		t.Errorf("the command interface of Склад is not written:\n%s", description)
	}
	sales, _ := os.ReadFile(filepath.Join(directory, treeSalesSubsystem+".yaml"))
	if string(sales) != salesBody {
		t.Errorf("Продажи is written though the window did not change it:\n%s", sales)
	}
}
