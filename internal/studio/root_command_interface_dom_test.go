package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rootCommandInterfaceWalk opens «Открыть командный интерфейс проекта» of
// the menu of the root, reads the window, moves Склад up, takes the box of
// Продажи off, presses OK and «Сохранить».
const rootCommandInterfaceWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;
  const settle = () => new Promise(resolve => setTimeout(resolve, 120)).then(() => studio.settle());
  await settle();
  const root = document.querySelector('#tree summary');
  root.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10})); await settle();
  const opening = [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent.trim() === 'Открыть командный интерфейс проекта');
  opening.click(); await settle();
  const dialog = document.querySelector('dialog.section-commands-dialog');
  const read = () => [...dialog.querySelectorAll('.section-commands .studio-tree-row')].map(row => row.textContent + (row.querySelector('input') ? (row.querySelector('input').checked ? ' ✓' : ' -') : ''));
  steps.title = dialog.querySelector('h3').textContent;
  steps.buttons = [...dialog.querySelectorAll('.object-window-panel button')].map(item => item.title);
  steps.rows = read();
  [...dialog.querySelectorAll('.section-commands .studio-tree-row')].find(row => row.textContent === 'Склад').click(); await settle();
  [...dialog.querySelectorAll('.object-window-panel button')].find(item => item.title === 'Переместить вверх').click(); await settle();
  [...dialog.querySelectorAll('.section-commands .studio-tree-row')].find(row => row.textContent === 'Продажи').querySelector('input').click(); await settle();
  steps.changed = read();
  [...dialog.querySelectorAll('button')].find(item => item.textContent === 'OK').click(); await settle();
  steps.dirty = !document.querySelector('#save').disabled;
  document.querySelector('#save').click(); await settle();
  steps.saved = document.querySelector('#save').disabled;
  return steps;
})()`

// The window «Командный интерфейс» of the root is the prototype's
// (konfiguraciya.md, «Окна», «Досъёмка 27.10.2026»; snapshots
// konfiguraciya/16, 17, series-2.595/36-40, 66-68): opened from the menu of
// the root; up, down, «Установить свойства по умолчанию», «Скрыть невидимые
// по умолчанию» - no move into another group; one group «Разделы» - the
// subsystems of the root in the command interface, Сервис out of it is not,
// the subsystem under another is not - by the alphabet, each visible; up
// makes the order manual; OK and «Сохранить» write the order of the sections
// and the visibility taken off into the root. Defects caught: a section
// missing or extra; a button the root's window has not; the order or the
// visibility not written, or written into a subsystem.
func TestTheCommandInterfaceOfTheRootIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	directory := filepath.Join(root, "metadata", "subsystems")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	const (
		sales     = "30000000-0000-4000-8000-000000000401"
		warehouse = "30000000-0000-4000-8000-000000000402"
		service   = "30000000-0000-4000-8000-000000000403"
		inner     = "30000000-0000-4000-8000-000000000404"
	)
	for id, body := range map[string]string{
		sales:     "format: 1\nid: " + sales + "\nname: Продажи\ntitle: {ru: Продажи}\ninclude_in_command_interface: true\n",
		warehouse: "format: 1\nid: " + warehouse + "\nname: Склад\ntitle: {ru: Склад}\ninclude_in_command_interface: true\n",
		service:   "format: 1\nid: " + service + "\nname: Сервис\ntitle: {ru: Сервис}\n",
		inner:     "format: 1\nid: " + inner + "\nname: Отгрузки\ntitle: {ru: Отгрузки}\nparent: " + sales + "\ninclude_in_command_interface: true\n",
	} {
		if err := os.WriteFile(filepath.Join(directory, id+".yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, rootCommandInterfaceWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Title                  string
		Buttons, Rows, Changed []string
		Dirty, Saved           bool
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
	expect("the title", steps.Title, "Командный интерфейс")
	expect("the buttons", steps.Buttons, []string{"Переместить вверх", "Переместить вниз", "Установить свойства по умолчанию",
		"Скрыть невидимые по умолчанию", "Выбор объекта: Роль"})
	expect("the sections", steps.Rows, []string{"Разделы", "Продажи ✓", "Склад ✓"})
	expect("up and a box off", steps.Changed, []string{"Разделы (ручной порядок)", "Склад ✓", "Продажи -"})
	expect("changed", steps.Dirty, true)
	expect("saved", steps.Saved, true)
	configuration, _ := os.ReadFile(filepath.Join(root, "configuration.yaml"))
	want := "subsystems_order:\n  - " + warehouse + "\n  - " + sales + "\nsubsystems_visibility:\n  - subsystem: " + sales + "\n    common: false\n"
	if !strings.Contains(string(configuration), want) {
		t.Errorf("the root does not keep the order and the visibility of its sections:\n%s", configuration)
	}
	for _, id := range []string{sales, warehouse} {
		if description, _ := os.ReadFile(filepath.Join(directory, id+".yaml")); strings.Contains(string(description), "\ncommand_interface:") {
			t.Errorf("the window of the root wrote into a subsystem:\n%s", description)
		}
	}
}
