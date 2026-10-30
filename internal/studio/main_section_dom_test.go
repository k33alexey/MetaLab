package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
)

// mainSectionWalk opens «Открыть командный интерфейс основного раздела» of
// the menu of the root, reads the window, puts «Товары: создать» in, chooses
// «Панель навигации.Важное» and puts the list of Товары in, takes Контрагенты
// out and presses OK.
const mainSectionWalk = `(async () => {
  const studio = window.__studio, steps = {};
  const settle = () => new Promise(resolve => setTimeout(resolve, 120)).then(() => studio.settle());
  await settle();
  let item;
  for (let tries = 0; tries < 20 && !item; tries++) {
    document.querySelector('#tree summary').dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10})); await settle();
    item = [...document.querySelectorAll('.context-menu .context-menu-item')].find(entry => entry.textContent.includes('Открыть командный интерфейс основного раздела'));
  }
  item.click(); await settle();
  const dialog = document.querySelector('dialog[open]');
  const right = () => [...dialog.querySelectorAll('.section-commands .studio-tree-row')];
  const read = () => right().map(row => row.textContent + (row.querySelector('input') ? (row.querySelector('input').checked ? ' ✓' : ' -') : '') + (row.classList.contains('selected') ? ' *' : ''));
  const left = () => [...dialog.querySelectorAll('.main-section-available .studio-tree-row')];
  const button = text => [...dialog.querySelectorAll('.main-section-moves button')].find(item => item.textContent === text);
  steps.title = dialog.querySelector('h3').textContent;
  steps.caption = dialog.querySelector('.main-section-caption').textContent;
  steps.available = left().map(row => row.textContent);
  steps.panel = [...dialog.querySelectorAll('.object-window-panel button')].map(item => item.title);
  steps.before = read();
  left().find(row => row.textContent === 'Товары: создать').click(); await settle();
  button('>').click(); await settle();
  steps.created = read();
  right().find(row => row.textContent.startsWith('Панель навигации.Важное')).click(); await settle();
  left().find(row => row.textContent === 'Товары').click(); await settle();
  left().filter(row => row.textContent === 'Товары').pop().click(); await settle();
  button('>').click(); await settle();
  steps.listed = read();
  right().find(row => row.textContent === 'Контрагенты').click(); await settle();
  button('<').click(); await settle();
  steps.removed = read();
  [...dialog.querySelectorAll('button')].find(item => item.textContent === 'OK').click(); await settle();
  steps.open = !!document.querySelector('dialog[open]');
  // OK gives the root its command interface; «Сохранить» writes it, and
  // the window opened before that shows it.
  steps.unsaved = !document.querySelector('#save').disabled;
  steps.fileBefore = await (await fetch('/api/project/main-section')).json().then(window => (window.commandInterface.commandsPlacement || []).length);
  document.querySelector('#tree summary').dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10})); await settle();
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(entry => entry.textContent.includes('Открыть командный интерфейс основного раздела')).click(); await settle();
  steps.again = [...document.querySelector('dialog[open]').querySelectorAll('.section-commands .studio-tree-row')].map(row => row.textContent).filter(text => text.startsWith('Товары'));
  [...document.querySelector('dialog[open]').querySelectorAll('button')].find(item => item.textContent === 'Отмена').click(); await settle();
  document.querySelector('#save').click(); await settle();
  steps.saved = document.querySelector('#save').disabled;
  return steps;
})()`

// The window «Командный интерфейс основного раздела» is the prototype's
// (konfiguraciya.md, «Окна», «Досъёмка 27.10.2026», «Предсъёмка
// 28.10.2026»; snapshots konfiguraciya/21, 22, series-2.595/50-52, 69, 70,
// predsyomka-1/34-38): the menu of the root opens it; at the left the
// project, the branches of objects giving a section a command, the objects
// and their commands; at the right the commands placed, «Переместить
// команду» first; «>» puts a command to create one at the start of
// «Панель действий.Создать», unseen, and a list at the end of the group of
// the navigation panel chosen, visible; «<» takes the current one out, the
// next current; OK writes every command with its placement and its
// visibility, keeps a command written in the prototype's notation, and
// drops the one taken out. Defects caught: a command added elsewhere or of
// another visibility; the one taken out still written, or another current;
// the content not written whole - a placement or a visibility missing -
// or the written command lost.
func TestTheMainSectionIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	for _, name := range []string{"Товары", "Контрагенты"} {
		path := filepath.Join(root, "metadata", "catalogs", name, "object.yaml")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(string(content)+"use_standard_commands: true\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const (
		goods        = "10000000-0000-4000-8000-000000000101"
		counterparty = "10000000-0000-4000-8000-000000000102"
	)
	written := "commands_visibility:\n  - object: " + counterparty + "\n    standard: open-list\n    visibility:\n      common: true\n" +
		"  - object: " + counterparty + "\n    standard: create\n    visibility:\n      common: false\n  - written: \"0:11111111-1111-4111-8111-111111111111\"\n    visibility:\n      common: false\n" +
		"commands_placement:\n  - object: " + counterparty + "\n    standard: open-list\n    group:\n      standard: navigation-panel-important\n    placement: auto\n" +
		"  - object: " + counterparty + "\n    standard: create\n    group:\n      standard: actions-panel-create\n    placement: auto\n" +
		"  - written: \"0:11111111-1111-4111-8111-111111111111\"\n    group:\n      standard: navigation-panel-ordinary\n    placement: auto\n"
	if err := os.WriteFile(filepath.Join(root, project.MainSectionCommandInterfaceFile), []byte(written), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, mainSectionWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Title, Caption                                     string
		Available, Panel, Before, Created, Listed, Removed []string
		Open, Unsaved, Saved                               bool
		FileBefore                                         int
		Again                                              []string
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
	expect("the title", steps.Title, "Командный интерфейс основного раздела")
	expect("the caption", steps.Caption, "Доступные команды:")
	expect("the tree", steps.Available, []string{"ПродажиИСклад", "Общие", "Общие формы", "ДемоФормаТовара", "Демо формы товара", "Справочники", "Контрагенты", "Контрагенты",
		"Контрагенты: создать", "Товары", "Товары", "Товары: создать"})
	expect("the panel", steps.Panel, []string{"Переместить команду в другую группу команд", "Переместить вверх", "Переместить вниз", "Установить свойства по умолчанию",
		"Скрыть невидимые по умолчанию", "Выбор объекта: Роль"})
	groups := func(important, ordinary, create []string) []string {
		rows := append([]string{"Панель навигации.Важное"}, important...)
		rows = append(append(rows, "Панель навигации.Обычное"), ordinary...)
		rows = append(rows, "Панель навигации.См. также", "Панель действий.Создать")
		return append(append(rows, create...), "Панель действий.Отчеты", "Панель действий.Сервис")
	}
	expect("before", steps.Before, groups([]string{"Контрагенты ✓"}, nil, []string{"Контрагенты: создать -"}))
	expect("a creation put in", steps.Created, groups([]string{"Контрагенты ✓"}, nil, []string{"Товары: создать - *", "Контрагенты: создать -"}))
	expect("a list put in", steps.Listed, groups([]string{"Контрагенты ✓", "Товары ✓ *"}, nil, []string{"Товары: создать -", "Контрагенты: создать -"}))
	expect("taken out", steps.Removed, groups([]string{"Товары ✓ *"}, nil, []string{"Товары: создать -", "Контрагенты: создать -"}))
	expect("closed", steps.Open, false)
	// The owner's decision of 29.10.2026, В7 of series 2.646: OK changes the
	// root, «Сохранить» writes the file with configuration.yaml.
	expect("the root changed by OK", steps.Unsaved, true)
	expect("the file before «Сохранить»", steps.FileBefore, 3)
	expect("opened again before «Сохранить»", steps.Again, []string{"Товары", "Товары: создать"})
	expect("saved", steps.Saved, true)
	file, err := os.Open(filepath.Join(root, project.MainSectionCommandInterfaceFile))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	value, err := metadata.DecodeRootDescription(project.MainSectionCommandInterfaceFile, file)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(value)
	text := string(encoded)
	for _, want := range []string{
		`{"object":"` + goods + `","standard":"create","visibility":{"common":false}}`,
		`{"object":"` + goods + `","standard":"open-list","visibility":{"common":true}}`,
		`{"written":"0:11111111-1111-4111-8111-111111111111","visibility":{"common":false}}`,
		`{"object":"` + goods + `","standard":"create","group":{"standard":"actions-panel-create"},"placement":"auto"}`,
		`{"object":"` + goods + `","standard":"open-list","group":{"standard":"navigation-panel-important"},"placement":"auto"}`,
		`{"written":"0:11111111-1111-4111-8111-111111111111","group":{"standard":"navigation-panel-ordinary"},"placement":"auto"}`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the main section written misses %s:\n%s", want, text)
		}
	}
	if strings.Count(text, counterparty) != 2 || strings.Contains(text, `"object":"`+counterparty+`","standard":"open-list"`) {
		t.Errorf("the command taken out is still written:\n%s", text)
	}
}
