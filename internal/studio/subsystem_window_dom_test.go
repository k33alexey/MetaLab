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
  await windowPage('Прочее'); steps.other = page();
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
// «Сохранить»; «Прочее» - «Справочная информация» unavailable until the
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
		Created                              struct {
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
	// «Состав» stands between the options and «Прочее» with its item.
	expect("the pages", steps.Pages, []string{"Основные", "Функциональные опции", "Прочее"})
	main := []string{"Имя:", "Синоним:", "Комментарий:", "Включать в командный интерфейс [ ]", "Подсистема с одной командой [ ]",
		"Командный интерфейс (недоступна)", "Пояснение:", "Картинка:"}
	expect("«Основные»", steps.Main, main)
	expect("«Функциональные опции»", steps.Options, []string{"Функциональные опции, в которых участвует подсистема:", "Артикулы [ ]", "Склады [ ]"})
	expect("«Прочее»", steps.Other, []string{"Справочная информация (недоступна)", "Включать в содержание справки [ ]"})
	expect("the palette", steps.Palette, []string{"Основные", "Представление", "Справочная информация"})
	expect("saved", steps.Saved, true)
	option, _ := os.ReadFile(filepath.Join(root, "metadata", "functional-options", optionArticles+".yaml"))
	if !strings.Contains(string(option), "- kind: subsystems\n    object: "+treeSalesSubsystem) {
		t.Errorf("the mark is not written into the option:\n%s", option)
	}
	created := append([]string(nil), main...)
	created[3], created[4] = "Включать в командный интерфейс [x]", "Подсистема с одной командой [ ]"
	expect("a new subsystem", steps.Created, struct {
		Title string
		Main  []string
	}{"Подсистема Подсистема1", created})
	expect("«Прочее» of a new subsystem", steps.CreatedOther, []string{"Справочная информация (недоступна)", "Включать в содержание справки [x]"})
}
