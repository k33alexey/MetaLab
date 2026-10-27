package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// documentWindowWalk opens the document ПродажаТоваров of the example and
// reads its window page by page and its palette tab by tab; marks «Число»
// of «Тип номера» on «Нумерация» and saves.
const documentWindowWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const host = document.querySelector('#document-editor');
  const settle = () => new Promise(resolve => setTimeout(resolve, 100)).then(() => studio.settle());
  const captions = () => [...host.querySelectorAll('.object-window-body .catalog-field > span:first-child, .object-window-body .catalog-check > span, .object-window-body legend, .object-window-body .studio-radio, .object-window-body .object-window-caption, .object-window-body button:not(.field-button):not(.icon-only):not([title])')]
    .map(item => item.textContent + (item.matches('button') && item.disabled ? ' (недоступна)' : '') + (item.matches('.studio-radio') && item.querySelector('input').checked ? ' (•)' : ''));
  const trees = () => [...host.querySelectorAll('.object-window-body .object-window-tree > details > .studio-tree-row, .object-window-body .object-window-tree > .studio-tree-row')].map(item => item.textContent.trim());
  const tabs = () => [...document.querySelectorAll('#properties .studio-tab')].map(tab => tab.textContent);
  const paletteRows = () => [...document.querySelectorAll('#properties .object-palette-row .object-palette-caption')].map(item => item.textContent);
  const paletteTab = async caption => { [...document.querySelectorAll('#properties .studio-tab')].find(tab => tab.textContent === caption).click(); await settle(); };
  await openObject('ПродажаТоваров');
  steps.title = host.querySelector('.object-window-title')?.textContent || '';
  steps.pages = [...host.querySelectorAll('.object-window-page')].map(item => item.textContent);
  steps.main = captions();
  await windowPage('Данные'); steps.data = {captions: captions(), trees: trees()};
  await windowPage('Нумерация'); steps.numbering = captions();
  await windowPage('Формы'); steps.forms = captions();
  await windowPage('Прочее'); steps.other = captions();
  steps.tabs = tabs();
  steps.palette = {};
  for (const caption of tabs()) { await paletteTab(caption); steps.palette[caption] = paletteRows(); }
  await windowPage('Нумерация');
  [...host.querySelectorAll('.object-window-body .studio-radio')].find(item => item.textContent === 'Число').querySelector('input').click(); await settle();
  document.querySelector('#save').click(); await settle();
  steps.saved = document.querySelector('#save').disabled;
  return steps;
})()`

// The window of a document is the prototype's (docs/block-00/dokument.md,
// «Окно», «Палитра»; snapshots dokument/01-03, 09, 12, 13): sixteen pages;
// «Основные» - the name, the synonym, the comment, the presentations and
// the explanation; «Данные» - the trees of the attributes and the table
// parts, no lengths, no type of the code, no main presentation;
// «Нумерация» - «Автонумерация», «Нумератор:», «Длина номера:»,
// «Контроль уникальности», «Тип номера» as radio buttons, «Периодичность:»;
// «Формы» - «Документа:», «Списка:», «Выбора:»; «Прочее» as an exchange
// plan's; the palette of four tabs, «Данные» and «Представление» in the
// order of the snapshots. «Число» of «Тип номера» is written by
// «Сохранить». Expectations are written from the inventory.
// Defects caught: a page or a row not the prototype's or out of order; the
// lengths or the frames of a catalog shown on «Данные»; «Тип номера» drawn
// as a list; a row of the palette the document has not, or missing; the
// window not writing the document.
func TestTheWindowOfADocumentIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, documentWindowWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Title                                      string
		Pages, Main, Numbering, Forms, Other, Tabs []string
		Data                                       struct{ Captions, Trees []string }
		Palette                                    map[string][]string
		Saved                                      bool
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
	expect("the title", steps.Title, "Документ ПродажаТоваров")
	expect("the pages", steps.Pages, []string{"Основные", "Подсистемы", "Функциональные опции", "Данные", "Нумерация", "Движения", "Последовательности",
		"Журналы", "Формы", "Поле ввода", "Команды", "Макеты", "Ввод на основании", "Права", "Обмен данными", "Прочее"})
	expect("«Основные»", steps.Main, []string{"Имя", "Синоним", "Комментарий", "Представление объекта", "Расширенное представление объекта",
		"Представление списка", "Расширенное представление списка", "Пояснение"})
	expect("«Данные»", steps.Data.Captions, []string{"Стандартные реквизиты (недоступна)", "Характеристики (недоступна)", "Общие реквизиты (недоступна)",
		"Дополнительные индексы (недоступна)"})
	expect("the trees of «Данные»", steps.Data.Trees, []string{"Реквизиты", "Табличные части"})
	expect("«Нумерация»", steps.Numbering, []string{"Автонумерация", "Нумератор:", "Длина номера:", "Контроль уникальности", "Тип номера", "Число",
		"Строка (•)", "Периодичность:"})
	expect("«Формы»", steps.Forms, []string{"Формы", "Документа:", "Списка:", "Выбора:"})
	expect("«Прочее»", steps.Other, []string{"Модуль объекта", "Модуль менеджера", "Поля блокировки данных:", "Полнотекстовый поиск:", "История данных:",
		"Обновлять историю данных сразу после записи", "Выполнять обработку после записи версии истории данных", "Справочная информация (недоступна)",
		"Включать в содержание справки"})
	expect("the palette's tabs", steps.Tabs, []string{"Основные", "Данные", "Представление", "Справка"})
	expect("the palette's «Основные»", steps.Palette["Основные"], []string{"Имя", "Синоним", "Комментарий", "Модуль объекта", "Модуль менеджера"})
	expect("the palette's «Данные»", steps.Palette["Данные"], []string{"Нумератор", "Тип номера", "Длина номера", "Допустимая длина номера", "Периодичность",
		"Контроль уникальности", "Автонумерация", "Стандартные реквизиты", "Характеристики", "Вводится на основании", "Проведение", "Оперативное проведение",
		"Удаление движений", "Запись движений при проведении", "Заполнение последовательностей", "Движения", "Прив. режим при проведении",
		"Прив. режим при отмене проведения", "Поля блокировки данных", "Режим управления блокировкой данных", "Полнотекстовый поиск", "История данных",
		"Обновлять историю данных сразу после записи", "Выполнять обработку после записи версии истории данных", "Дополнительные индексы"})
	expect("the palette's «Представление»", steps.Palette["Представление"], []string{"Использовать стандартные команды", "Ввод по строке", "Создание при вводе",
		"Способ поиска строки при вводе по строке", "Полнотекстовый поиск при вводе по строке", "Режим получения данных выбора при вводе по строке",
		"История выбора при вводе", "Основная форма объекта", "Основная форма списка", "Основная форма выбора", "Представление объекта",
		"Расширенное представление объекта", "Представление списка", "Расширенное представление списка", "Пояснение"})
	expect("saved", steps.Saved, true)
	written, _ := os.ReadFile(filepath.Join(root, "metadata", "documents", "ПродажаТоваров", "object.yaml"))
	if !strings.Contains(string(written), "type: number") {
		t.Errorf("the document written: %s", written)
	}
}
