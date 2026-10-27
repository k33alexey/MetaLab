package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// commonAttributeWalk makes a common attribute by «Добавить» and reads its
// palette tab by tab; sets the auto-use and saves.
const commonAttributeWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 80)).then(() => studio.settle());
  const rows = () => [...document.querySelectorAll('#properties .object-palette-row')].map(row => {
    const field = row.querySelector('select, input:not([type=checkbox]), button.object-palette-link');
    const box = row.querySelector('input[type=checkbox]');
    const value = box ? (box.checked ? '[x]' : '[ ]') : field ? (field.tagName === 'SELECT' ? field.selectedOptions[0]?.textContent : field.tagName === 'BUTTON' ? field.textContent + (field.disabled ? ' (недоступна)' : '') : field.value) : '';
    return row.querySelector('.object-palette-caption').textContent + ': ' + value;
  });
  const tab = async text => { [...document.querySelectorAll('#properties .studio-tab')].find(item => item.textContent === text).click(); await settle(); };
  await openObject('Общие реквизиты');
  document.querySelector('#tree-add').click(); await settle();
  steps.title = document.querySelector('#inspector-title').textContent;
  steps.tabs = [...document.querySelectorAll('#properties .studio-tab')].map(item => item.textContent);
  steps.main = rows();
  await tab('Использование'); steps.use = rows();
  await tab('Представление'); steps.presentation = rows().map(row => row.split(':')[0]);
  await tab('Использование');
  const auto = [...document.querySelectorAll('#properties .object-palette-row')].find(row => row.querySelector('.object-palette-caption').textContent === 'Автоиспользование').querySelector('select');
  auto.value = 'use'; auto.dispatchEvent(new Event('change', {bubbles: true})); await settle();
  document.querySelector('#save').click(); await settle();
  steps.saved = document.querySelector('#save').disabled;
  return steps;
})()`

// The palette of a common attribute is the prototype's (docs/block-00/
// obshchij-rekvizit.md, «Палитра», «Новый реквизит»): «Добавить» makes
// «ОбщийРеквизит1» - a string of 10 of variable length; «Использование» -
// «Состав: Открыть» unavailable until its window, no auto-use, no
// separation, not indexed, in the full-text search and the history of
// data; «Представление» - the rows of a string field, its filling and
// choice; the auto-use set is written by «Сохранить».
// Defects caught: a tab, a row or a default not the prototype's; a value
// set not written.
func TestThePaletteOfACommonAttributeIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, commonAttributeWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Title                         string
		Tabs, Main, Use, Presentation []string
		Saved                         bool
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
	expect("the title", steps.Title, "Свойства: ОбщийРеквизит1")
	expect("the tabs", steps.Tabs, []string{"Основные", "Использование", "Представление"})
	expect("«Основные»", steps.Main, []string{"Имя: ОбщийРеквизит1", "Синоним: ", "Комментарий: ", "Тип: Строка", "Длина: 10",
		"Допустимая длина: Переменная", "Неограниченная длина: [ ]"})
	expect("«Использование»", steps.Use, []string{"Состав: Открыть (недоступна)", "Автоиспользование: Не использовать",
		"Разделение данных: Не использовать", "Индексировать: Не индексировать", "Полнотекстовый поиск: Использовать", "История данных: Использовать"})
	expect("«Представление»", steps.Presentation, []string{"Режим пароля", "Подсказка", "Маска", "Многострочный режим", "Расширенное редактирование",
		"Заполнять из данных заполнения", "Значение заполнения", "Проверка заполнения", "Быстрый выбор", "Создание при вводе", "История выбора при вводе"})
	expect("saved", steps.Saved, true)
	entries, _ := os.ReadDir(filepath.Join(root, "metadata", "common-attributes"))
	found := false
	for _, entry := range entries {
		content, _ := os.ReadFile(filepath.Join(root, "metadata", "common-attributes", entry.Name()))
		found = found || strings.Contains(string(content), "auto_use: use")
	}
	if !found {
		t.Errorf("the auto-use is not written: %v", entries)
	}
}
