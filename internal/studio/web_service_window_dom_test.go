package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// webServiceWindowWalk opens the window of the web service СкладСервис and reads
// it page by page as the inventory reads it, with its palette; marks the
// subsystem Склад, changes the lifetime of a session in the window and
// saves.
const webServiceWindowWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const host = document.querySelector('#web-service-editor');
  const settle = () => new Promise(resolve => setTimeout(resolve, 80)).then(() => studio.settle());
  const captions = () => [...host.querySelectorAll('.object-window-body .catalog-field > span:first-child')].map(item => item.textContent);
  await openObject('СкладСервис');
  steps.title = host.querySelector('.object-window-title').textContent;
  steps.pages = [...host.querySelectorAll('.object-window-page')].map(item => item.textContent);
  steps.main = {captions: captions(), name: windowField('#web-service-editor', 'Имя:').querySelector('input').value};
  await windowPage('Подсистемы');
  steps.subsystems = [...host.querySelectorAll('.object-window-body .studio-tree-row')].map(item => item.textContent.trim());
  [...host.querySelectorAll('.object-window-body .studio-tree-row')].find(item => item.textContent.trim() === 'Склад').querySelector('input[type=checkbox]').click(); await settle();
  await windowPage('Операции');
  steps.operations = host.querySelector('.object-window-body').children.length;
  await windowPage('Управление сеансами');
  steps.sessions = {captions: captions(), above: windowField('#web-service-editor', 'Повторное использование сеансов:').classList.contains('catalog-field-above'),
    reuse: windowField('#web-service-editor', 'Повторное использование сеансов:').querySelector('select').value,
    age: windowField('#web-service-editor', 'Время жизни сеанса:').querySelector('input').value};
  const age = windowField('#web-service-editor', 'Время жизни сеанса:').querySelector('input');
  age.focus(); age.value = '45'; document.querySelector('#refresh').focus(); age.dispatchEvent(new Event('change', {bubbles: true})); await settle();
  await windowPage('Прочее');
  steps.other = {captions: captions(), buttons: [...host.querySelectorAll('.object-window-body .object-window-button')].map(item => item.textContent),
    packages: windowField('#web-service-editor', 'Пакеты XDTO:').querySelector('input').value,
    choose: windowField('#web-service-editor', 'Пакеты XDTO:').querySelector('button').disabled,
    file: windowField('#web-service-editor', 'Имя файла публикации:').querySelector('input').value};
  steps.palette = {title: document.querySelector('#inspector-title').textContent,
    rows: [...document.querySelectorAll('#properties .object-palette-row .object-palette-caption')].map(item => item.textContent),
    age: windowField('#properties', 'Время жизни сеанса').querySelector('input').value};
  document.querySelector('#save').click(); await settle();
  steps.saved = !document.querySelector('#save').disabled;
  return steps;
})()`

// The window of a web service is the prototype's (docs/block-00/
// web-servis.md, «Окно»; snapshots web-servis/01, 03, 04): «Web-сервис
// Склад», five pages; «Основные» - the name, the synonym, the comment;
// «Подсистемы» - the tree of the subsystems with boxes; «Управление
// сеансами» - the reuse of sessions above its list and the lifetime;
// «Прочее» - the button «Модуль», the namespace, the packages by their
// namespaces with «…» unavailable, the file of publication; «Операции»
// empty until its item. The palette shows what the window typed; «Сохранить»
// writes the service and the subsystem marked. Defects caught: a page, a
// caption or its place not the prototype's; a package shown by its
// identifier; the window and the palette apart; a mark not written.
func TestTheWindowOfAWebServiceIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addTreeSubsystems(t, root)
	addWebService(t, root)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, webServiceWindowWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Title string
		Pages []string
		Main  struct {
			Captions []string
			Name     string
		}
		Subsystems []string
		Operations int
		Sessions   struct {
			Captions   []string
			Above      bool
			Reuse, Age string
		}
		Other struct {
			Captions, Buttons []string
			Packages          string
			Choose            bool
			File              string
		}
		Palette struct {
			Title string
			Rows  []string
			Age   string
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
	expect("title and pages", []any{steps.Title, steps.Pages}, []any{"Web-сервис СкладСервис", []string{"Основные", "Подсистемы", "Операции", "Управление сеансами", "Прочее"}})
	expect("«Основные»", steps.Main, struct {
		Captions []string
		Name     string
	}{[]string{"Имя:", "Синоним:", "Комментарий:"}, "СкладСервис"})
	expect("«Подсистемы»", steps.Subsystems, []string{"Продажи", "Склад"})
	expect("«Операции» until its item", steps.Operations, 0)
	expect("«Управление сеансами»", steps.Sessions, struct {
		Captions   []string
		Above      bool
		Reuse, Age string
	}{
		[]string{"Повторное использование сеансов:", "Время жизни сеанса:"}, true, "auto", "20"})
	expect("«Прочее»", steps.Other, struct {
		Captions, Buttons []string
		Packages          string
		Choose            bool
		File              string
	}{
		[]string{"URI пространства имен:", "Пакеты XDTO:", "Имя файла публикации:"}, []string{"Модуль"}, "http://example.org/goods,http://v8.1c.ru/8.1/data/core", true, "stock.1cws"})
	expect("the palette", steps.Palette, struct {
		Title string
		Rows  []string
		Age   string
	}{"Свойства: СкладСервис",
		[]string{"Имя", "Синоним", "Комментарий", "Пакеты XDTO", "URI пространства имен", "Модуль", "Имя файла публикации", "Повторное использование сеансов", "Время жизни сеанса"}, "45"})
	expect("saved", steps.Saved, false)
	service, _ := os.ReadFile(filepath.Join(root, "metadata", "web-services", "СкладСервис", "object.yaml"))
	subsystem, _ := os.ReadFile(filepath.Join(root, "metadata", "subsystems", treeWarehouseSubsystem+".yaml"))
	if !strings.Contains(string(service), "session_max_age: 45") || !strings.Contains(string(subsystem), webServiceGoods) {
		t.Errorf("on disk: the service %q, the subsystem %q", service, subsystem)
	}
}
