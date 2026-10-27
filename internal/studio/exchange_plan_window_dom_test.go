package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// exchangePlanWindowWalk opens the exchange plan Филиалы and reads its
// window page by page and its palette tab by tab; marks «Распределенная
// информационная база», adds an attribute on «Данные» and saves.
const exchangePlanWindowWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const host = document.querySelector('#exchange-plan-editor');
  const settle = () => new Promise(resolve => setTimeout(resolve, 100)).then(() => studio.settle());
  const captions = () => [...host.querySelectorAll('.object-window-body .catalog-field > span:first-child, .object-window-body .catalog-check > span, .object-window-body legend, .object-window-body .object-window-caption, .object-window-body button:not(.field-button):not(.icon-only):not([title])')]
    .map(item => item.textContent + (item.matches('button') && item.disabled ? ' (недоступна)' : ''));
  const tabs = () => [...document.querySelectorAll('#properties .studio-tab')].map(tab => tab.textContent);
  const paletteRows = () => [...document.querySelectorAll('#properties .object-palette-row .object-palette-caption')].map(item => item.textContent);
  const paletteTab = async caption => { [...document.querySelectorAll('#properties .studio-tab')].find(tab => tab.textContent === caption).click(); await settle(); };
  await openObject('Филиалы');
  steps.title = host.querySelector('.object-window-title')?.textContent || '';
  steps.pages = [...host.querySelectorAll('.object-window-page')].map(item => item.textContent);
  steps.main = captions();
  await windowPage('Данные'); steps.data = captions();
  await windowPage('Формы'); steps.forms = captions();
  await windowPage('Прочее'); steps.other = captions();
  await windowPage('Подсистемы'); steps.subsystems = captions();
  steps.tabs = tabs();
  steps.palette = {};
  for (const caption of tabs()) { await paletteTab(caption); steps.palette[caption] = paletteRows(); }
  await windowPage('Основные');
  windowField('#exchange-plan-editor', 'Распределенная информационная база').querySelector('input').click(); await settle();
  await windowPage('Данные');
  [...host.querySelectorAll('.object-window-panel')][0].querySelector('button[title=Добавить]').click(); await settle();
  steps.chosen = host.querySelector('.object-window-body .studio-tree-row.selected')?.textContent.trim() || '';
  document.querySelector('#save').click(); await settle();
  steps.saved = document.querySelector('#save').disabled;
  return steps;
})()`

// The window of an exchange plan is the prototype's (docs/block-00/
// plan-obmena.md, «Окно», «Палитра»; snapshots plan-obmena/01, 02, 10, 12,
// 14): the pages of a catalog's window without «Иерархия», «Владельцы»,
// «Нумерация» and «Обмен данными»; «Основные» - the name, the synonym, the
// comment, «Распределенная информационная база», «Состав» (unavailable
// until its window), the presentations and the explanation; «Данные» -
// the lengths, «Основное представление» without «Тип кода», the trees and
// the buttons of the windows; «Формы» - «Узла:», «Списка:», «Выбора:»;
// «Прочее» - the modules, the storage, the help, no predefined items; the
// palette of four tabs, «Данные» in the order of snapshot 14. The flag and
// an attribute are written by «Сохранить».
// Expectations are written from the inventory, not from the descriptor.
// Defects caught: a page of a catalog the plan has not shown, or one it has
// missing; «Тип кода» shown; a row of a page or the palette not the
// prototype's or out of order; the window not writing the plan.
func TestTheWindowOfAnExchangePlanIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	folder := filepath.Join(root, "metadata", "exchange-plans", "Филиалы")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	description := "format: 1\nid: 50000000-0000-4000-8000-000000000801\nname: Филиалы\ntitle: {ru: Филиалы}\n" +
		"code: {type: string, length: 9}\ndescription_length: 25\n"
	if err := os.WriteFile(filepath.Join(folder, "object.yaml"), []byte(description), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, exchangePlanWindowWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Title, Chosen                                     string
		Pages, Main, Data, Forms, Other, Subsystems, Tabs []string
		Palette                                           map[string][]string
		Saved                                             bool
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
	expect("the title", steps.Title, "План обмена Филиалы")
	expect("the pages", steps.Pages, []string{"Основные", "Подсистемы", "Функциональные опции", "Данные", "Формы", "Поле ввода", "Команды", "Макеты",
		"Ввод на основании", "Права", "Прочее"})
	expect("«Основные»", steps.Main, []string{"Имя", "Синоним", "Комментарий", "Распределенная информационная база", "Состав (недоступна)",
		"Представление объекта", "Расширенное представление объекта", "Представление списка", "Расширенное представление списка", "Пояснение"})
	expect("«Данные»", steps.Data, []string{"Длина кода", "Длина наименования", "Основное представление",
		"Стандартные реквизиты (недоступна)", "Характеристики (недоступна)", "Общие реквизиты (недоступна)", "Дополнительные индексы (недоступна)"})
	expect("«Формы»", steps.Forms, []string{"Формы", "Узла:", "Списка:", "Выбора:"})
	expect("«Прочее»", steps.Other, []string{"Модуль объекта", "Модуль менеджера", "Поля блокировки данных:", "Полнотекстовый поиск:", "История данных:",
		"Обновлять историю данных сразу после записи", "Выполнять обработку после записи версии истории данных", "Справочная информация (недоступна)",
		"Включать в содержание справки"})
	expect("«Подсистемы»", steps.Subsystems, []string{"Подсистемы, в которых участвует план обмена:"})
	expect("the palette's tabs", steps.Tabs, []string{"Основные", "Данные", "Представление", "Справка"})
	expect("the palette's «Основные»", steps.Palette["Основные"], []string{"Имя", "Синоним", "Комментарий", "Состав", "Модуль объекта", "Модуль менеджера",
		"Распределенная информационная база"})
	expect("the palette's «Данные»", steps.Palette["Данные"], []string{"Длина кода", "Допустимая длина кода", "Длина наименования", "Стандартные реквизиты",
		"Характеристики", "Вводится на основании", "Поля блокировки данных", "Режим управления блокировкой данных", "Полнотекстовый поиск", "История данных",
		"Обновлять историю данных сразу после записи", "Выполнять обработку после записи версии истории данных", "Дополнительные индексы"})
	expect("an attribute added", steps.Chosen, "Реквизит1")
	expect("saved", steps.Saved, true)
	written, _ := os.ReadFile(filepath.Join(folder, "object.yaml"))
	if !strings.Contains(string(written), "distributed_info_base: true") || !strings.Contains(string(written), "name: Реквизит1") {
		t.Errorf("the plan written: %s", written)
	}
}
