package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// subsystemWindowWalk opens the subsystem Продажи and reads its window page
// by page and its palette tab by tab; marks the option Артикулы and saves;
// then makes a new subsystem by «Добавить» and reads its flags.
const subsystemWindowWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const host = document.querySelector('#subsystem-editor');
  const settle = () => new Promise(resolve => setTimeout(resolve, 80)).then(() => studio.settle());
  const page = () => [...host.querySelectorAll('.object-window-fields > :not(.object-window-checks), .object-window-checks > *')].map(item => {
    const box = item.querySelector('input[type=checkbox]');
    const button = item.matches('button') ? item : null;
    return (item.querySelector('span')?.textContent || item.textContent) + (box ? (box.checked ? ' [x]' : ' [ ]') : '') + (button?.disabled ? ' (недоступна)' : '');
  });
  const palette = () => [...document.querySelectorAll('#properties .studio-tab')].map(tab => tab.textContent);
  await openObject('Продажи');
  steps.title = host.querySelector('.object-window-title')?.textContent || '';
  steps.pages = [...host.querySelectorAll('.object-window-page')].map(item => item.textContent);
  steps.main = page();
  await windowPage('Функциональные опции'); steps.options = page();
  [...host.querySelectorAll('.object-window-checks input[type=checkbox]')][0].click(); await settle();
  await windowPage('Состав');
  const composition = () => {
    const offered = host.querySelector('.subsystem-composition'), held = host.querySelector('.subsystem-composition-held');
    const rows = (tree, open) => [...tree.querySelectorAll('.studio-tree-row')].filter(row => !open || !row.closest('details:not([open])') || row.parentElement.matches('details')).map(row => {
      const box = row.querySelector('input[type=checkbox]');
      return row.textContent + (box ? (box.checked ? ' [x]' : ' [ ]') : '');
    });
    return {branches: [...offered.children].map(item => (item.matches('.studio-tree-row') ? item : item.querySelector('.studio-tree-row')).textContent), shown: rows(offered, true), held: rows(held, false),
      remove: !host.querySelector('.subsystem-composition-caption button').disabled};
  };
  steps.composition = composition();
  const box = [...host.querySelectorAll('.subsystem-composition .studio-tree-row')].find(row => row.textContent === 'Товары').querySelector('input');
  box.click(); await settle();
  steps.marked = composition().held;
  [...host.querySelectorAll('.subsystem-composition-held .studio-tree-row')].find(row => row.textContent === 'ПродажаТоваров').click(); await settle();
  steps.chosen = composition().remove;
  host.querySelector('.subsystem-composition-caption button').click(); await settle();
  steps.removed = composition().held;
  await windowPage('Прочее'); steps.other = page();
  [...document.querySelectorAll('#properties .object-palette-row button')].find(item => item.textContent === 'Открыть').click(); await settle();
  steps.openedByPalette = host.querySelector('.object-window-page[aria-selected=true]')?.textContent || '';
  steps.palette = palette();
  document.querySelector('#save').click(); await settle();
  steps.saved = document.querySelector('#save').disabled;
  await openObject('Подсистемы');
  document.querySelector('#tree-add').click(); await settle();
  steps.created = {title: host.querySelector('.object-window-title')?.textContent || '', main: page()};
  await windowPage('Прочее'); steps.createdOther = page();
  return steps;
})()`

// The window of a subsystem is the prototype's (docs/block-00/
// podsistema.md, «Окно», «Палитра», «Новая подсистема»; snapshots
// podsistema/04-07, 17, 18): «Основные» - the name, the synonym, the
// comment, «Включать в командный интерфейс», «Подсистема с одной командой»,
// «Командный интерфейс» unavailable until its window, the explanation, the
// picture; «Функциональные опции» - every option, marked where the
// subsystem is in its content, a mark written into the option by
// «Сохранить»; «Состав» - the objects of the project by their branches,
// a box at each, and the objects held below with ✕, written by
// «Сохранить», shown by «Состав: Открыть» of the palette; «Прочее» -
// «Справочная информация» unavailable until the
// help, «Включать в содержание справки»; the palette of three tabs.
// «Добавить» makes «Подсистема1» in the command interface, not of one
// command, in the contents of the help.
// Defects caught: a page, a row or a tab not the prototype's; a mark of an
// option not written into the option; a new subsystem not the prototype's.
func TestTheWindowOfASubsystemIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addTreeSubsystems(t, root)
	addCatalogOptions(t, root)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, subsystemWindowWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Title                                string
		Pages, Main, Options, Other, Palette []string
		CreatedOther                         []string
		Saved                                bool
		Composition                          struct {
			Branches, Shown, Held []string
			Remove                bool
		}
		Marked, Removed []string
		Chosen          bool
		OpenedByPalette string
		Created         struct {
			Title string
			Main  []string
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
	expect("the title", steps.Title, "Подсистема Продажи")
	expect("the pages", steps.Pages, []string{"Основные", "Функциональные опции", "Состав", "Прочее"})
	main := []string{"Имя:", "Синоним:", "Комментарий:", "Включать в командный интерфейс [ ]", "Подсистема с одной командой [ ]",
		"Командный интерфейс (недоступна)", "Пояснение:", "Картинка:"}
	expect("«Основные»", steps.Main, main)
	expect("«Функциональные опции»", steps.Options, []string{"Функциональные опции, в которых участвует подсистема:", "Артикулы [ ]", "Склады [ ]"})
	expect("«Прочее»", steps.Other, []string{"Справочная информация (недоступна)", "Включать в содержание справки [ ]"})
	expect("the palette", steps.Palette, []string{"Основные", "Представление", "Справочная информация"})
	// The demo holds catalogs, documents, registers and common forms; Продажи
	// holds Контрагенты and ПродажаТоваров: their branches stand open.
	// Every kind of the 38 stands, those of no object too, as in the project
	// tree: «Общие», then the kinds of the root in its order.
	expect("«Состав»", steps.Composition.Branches, []string{"Общие", "Константы", "Справочники", "Документы", "Журналы документов", "Перечисления",
		"Отчёты", "Обработки", "Планы видов характеристик", "Планы счетов", "Планы видов расчёта", "Регистры сведений", "Регистры накопления",
		"Регистры бухгалтерии", "Регистры расчёта", "Бизнес-процессы", "Задачи"})
	expect("«Состав», held", steps.Composition.Held, []string{"Справочники", "Контрагенты", "Документы", "ПродажаТоваров"})
	expect("«Состав», ✕ with nothing chosen", steps.Composition.Remove, false)
	expect("a box marked", steps.Marked, []string{"Справочники", "Контрагенты", "Товары", "Документы", "ПродажаТоваров"})
	expect("✕ with a row chosen", steps.Chosen, true)
	expect("✕", steps.Removed, []string{"Справочники", "Контрагенты", "Товары"})
	expect("«Состав: Открыть»", steps.OpenedByPalette, "Состав")
	expect("saved", steps.Saved, true)
	option, _ := os.ReadFile(filepath.Join(root, "metadata", "functional-options", optionArticles+".yaml"))
	if !strings.Contains(string(option), "- kind: subsystems\n    object: "+treeSalesSubsystem) {
		t.Errorf("the mark is not written into the option:\n%s", option)
	}
	description, _ := os.ReadFile(filepath.Join(root, "metadata", "subsystems", treeSalesSubsystem+".yaml"))
	if !strings.Contains(string(description), "members:\n  - 10000000-0000-4000-8000-000000000102\n  - 10000000-0000-4000-8000-000000000101\n") {
		t.Errorf("the composition is not written:\n%s", description)
	}
	created := append([]string(nil), main...)
	created[3], created[4] = "Включать в командный интерфейс [x]", "Подсистема с одной командой [ ]"
	expect("a new subsystem", steps.Created, struct {
		Title string
		Main  []string
	}{"Подсистема Подсистема1", created})
	expect("«Прочее» of a new subsystem", steps.CreatedOther, []string{"Справочная информация (недоступна)", "Включать в содержание справки [x]"})
}
