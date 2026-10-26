package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// httpServiceWindowWalk opens the window of ОбменЗаказами and reads it page
// by page, types roots the prototype refuses and one it takes and reads the
// answers, reads the palette and saves.
const httpServiceWindowWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const host = document.querySelector('#http-service-editor');
  const settle = () => new Promise(resolve => setTimeout(resolve, 120)).then(() => studio.settle());
  const captions = () => [...host.querySelectorAll('.object-window-body .catalog-field > span:first-child')].map(item => item.textContent);
  const dialog = () => document.querySelector('dialog[open]');
  const typeRoot = async text => {
    const field = windowField('#http-service-editor', 'Корневой URL:').querySelector('input');
    field.focus(); field.value = text; field.dispatchEvent(new Event('input', {bubbles: true}));
    document.querySelector('#refresh').focus(); field.dispatchEvent(new Event('change', {bubbles: true})); await settle();
    const refusal = dialog()?.querySelector('h3')?.textContent || '';
    if (dialog()) { [...dialog().querySelectorAll('button')].find(item => item.textContent === 'ОК').click(); await settle(); }
    return refusal;
  };
  await openObject('ОбменЗаказами');
  steps.title = host.querySelector('.object-window-title').textContent;
  steps.pages = [...host.querySelectorAll('.object-window-page')].map(item => item.textContent);
  steps.main = {captions: captions(), root: windowField('#http-service-editor', 'Корневой URL:').querySelector('input').value};
  steps.refusals = [await typeRoot('a/b'), await typeRoot(''), await typeRoot('обмен')];
  await windowPage('Шаблоны URL');
  steps.templates = host.querySelector('.object-window-body').children.length;
  await windowPage('Управление сеансами');
  steps.sessions = captions();
  await windowPage('Прочее');
  steps.other = [...host.querySelectorAll('.object-window-body .object-window-button')].map(item => item.textContent);
  steps.palette = {title: document.querySelector('#inspector-title').textContent,
    rows: [...document.querySelectorAll('#properties .object-palette-row .object-palette-caption')].map(item => item.textContent),
    root: windowField('#properties', 'Корневой URL').querySelector('input').value};
  document.querySelector('#save').click(); await settle();
  steps.saved = !document.querySelector('#save').disabled;
  return steps;
})()`

// The window of an HTTP service is the prototype's (docs/block-00/
// http-servis.md, «Окно», «Проверки»; snapshots http-servis/01, 02, 08, 17,
// 18): «HTTP-сервис ОбменЗаказами», five pages; «Основные» with «Корневой
// URL:» after the comment; a root of a slash answered «Некорректный
// корневой URL», none «Корневой URL должен быть заполнен» when the field is
// left, «обмен» taken; «Управление сеансами» as a web service's; «Прочее»
// the button «Модуль» alone; «Шаблоны URL» empty until its item; the
// palette of the service with the root typed in the window; «Сохранить»
// writes the root. Defects caught: a caption or a page not the prototype's;
// a refused root taken silently or refused in words not the prototype's; a
// root taken refused; the window and the palette apart.
func TestTheWindowOfAnHTTPServiceIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addHTTPService(t, root)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, httpServiceWindowWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Title           string
		Pages, Refusals []string
		Main            struct {
			Captions []string
			Root     string
		}
		Templates       int
		Sessions, Other []string
		Palette         struct {
			Title string
			Rows  []string
			Root  string
		}
		Saved bool
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
	expect("title and pages", []any{steps.Title, steps.Pages}, []any{"HTTP-сервис ОбменЗаказами", []string{"Основные", "Подсистемы", "Шаблоны URL", "Управление сеансами", "Прочее"}})
	expect("«Основные»", steps.Main, struct {
		Captions []string
		Root     string
	}{[]string{"Имя:", "Синоним:", "Комментарий:", "Корневой URL:"}, "orders"})
	expect("the root typed", steps.Refusals, []string{"Некорректный корневой URL", "Корневой URL должен быть заполнен", ""})
	expect("«Шаблоны URL» until its item", steps.Templates, 0)
	expect("«Управление сеансами»", steps.Sessions, []string{"Повторное использование сеансов:", "Время жизни сеанса:"})
	expect("«Прочее»", steps.Other, []string{"Модуль"})
	expect("the palette", steps.Palette, struct {
		Title string
		Rows  []string
		Root  string
	}{"Свойства: ОбменЗаказами",
		[]string{"Имя", "Синоним", "Комментарий", "Корневой URL", "Модуль", "Повторное использование сеансов", "Время жизни сеанса"}, "обмен"})
	expect("saved", steps.Saved, false)
	service, _ := os.ReadFile(filepath.Join(root, "metadata", "http-services", "ОбменЗаказами", "object.yaml"))
	if !strings.Contains(string(service), "root_url: обмен") {
		t.Errorf("on disk the service is %s", service)
	}
}
