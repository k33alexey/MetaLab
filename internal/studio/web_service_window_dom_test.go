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
  steps.operations = [...host.querySelectorAll('.object-window-body .studio-tree-row')].map(item => item.textContent.trim());
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
  const packagesField = () => windowField('#web-service-editor', 'Пакеты XDTO:');
  const choice = () => document.querySelector('dialog.xdto-packages-dialog');
  const rows = () => [...choice().querySelectorAll('.xdto-packages-row')];
  const answer = text => [...choice().querySelectorAll('button')].find(item => item.textContent === text).click();
  packagesField().querySelector('button').click(); await settle();
  steps.choice = {title: choice().querySelector('h3').textContent, count: rows().length,
    first: rows()[0].textContent, last: rows().at(-2).textContent, own: rows().at(-1).textContent, indented: rows().at(-1).classList.contains('xdto-packages-own'),
    marked: rows().filter(row => row.querySelector('input').checked).map(row => row.textContent)};
  rows().find(row => row.textContent === 'xdto').querySelector('input').click();
  answer('Отмена'); await settle();
  steps.cancelled = packagesField().querySelector('input').value;
  packagesField().querySelector('button').click(); await settle();
  rows().find(row => row.textContent === 'xdto').querySelector('input').click();
  rows().find(row => row.textContent === 'data/core').querySelector('input').click();
  answer('OK'); await settle();
  steps.chosen = packagesField().querySelector('input').value;
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
// namespaces with «…» opening «Выбор пакетов XDTO» (series-2.584/131-136)
// - the 66 namespaces of the platform from the first to the last, then,
// indented, the packages of the project, those chosen marked; «Отмена»
// changes nothing, OK keeps the chosen in their order and puts the new
// after them - the file of publication; «Операции»
// the tree of its operations. The palette shows what the window typed; «Сохранить»
// writes the service and the subsystem marked. Defects caught: a page, a
// caption or its place not the prototype's; a package shown by its
// identifier; a namespace of the platform missing or out of order; the
// choice written on «Отмена»; the window and the palette apart; a mark
// not written.
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
		Operations []string
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
		Choice struct {
			Title, First, Last, Own string
			Count                   int
			Indented                bool
			Marked                  []string
		}
		Cancelled, Chosen string
		Palette           struct {
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
	expect("«Операции»", steps.Operations, []string{"Остаток"})
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
		[]string{"URI пространства имен:", "Пакеты XDTO:", "Имя файла публикации:"}, []string{"Модуль"}, "http://example.org/goods,data/core", false, "stock.1cws"})
	expect("«Выбор пакетов XDTO»", steps.Choice, struct {
		Title, First, Last, Own string
		Count                   int
		Indented                bool
		Marked                  []string
	}{"Выбор пакетов XDTO", "data-composition-system/appearance-template", "xcf/enums",
		"http://example.org/goods", 67, true, []string{"data/core", "http://example.org/goods"}})
	expect("«Отмена» changes nothing", steps.Cancelled, "http://example.org/goods,data/core")
	expect("chosen: the kept in their order, the new after", steps.Chosen, "http://example.org/goods,xdto")
	expect("the palette", steps.Palette, struct {
		Title string
		Rows  []string
		Age   string
	}{"Свойства: СкладСервис",
		[]string{"Имя", "Синоним", "Комментарий", "Пакеты XDTO", "URI пространства имен", "Модуль", "Имя файла публикации", "Повторное использование сеансов", "Время жизни сеанса"}, "45"})
	expect("saved", steps.Saved, false)
	service, _ := os.ReadFile(filepath.Join(root, "metadata", "web-services", "СкладСервис", "object.yaml"))
	subsystem, _ := os.ReadFile(filepath.Join(root, "metadata", "subsystems", treeWarehouseSubsystem+".yaml"))
	if !strings.Contains(string(service), "session_max_age: 45") || !strings.Contains(string(service), "namespace: http://v8.1c.ru/8.1/xdto") ||
		strings.Contains(string(service), "8.1/data/core") || !strings.Contains(string(subsystem), webServiceGoods) {
		t.Errorf("on disk: the service %q, the subsystem %q", service, subsystem)
	}
}

// webServiceOperationsWalk opens «Операции» of СкладСервис: reads the panel
// and the tree, adds an operation and reads its palette on both tabs, names
// its procedure, adds a parameter and reads its palette, moves the new
// operation up, deletes the parameter after the question, and saves.
const webServiceOperationsWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const host = document.querySelector('#web-service-editor');
  const settle = () => new Promise(resolve => setTimeout(resolve, 80)).then(() => studio.settle());
  const panel = () => [...host.querySelectorAll('.object-window-panel button')].map(item => item.title + (item.disabled ? ' (недоступна)' : ''));
  const press = title => [...host.querySelectorAll('.object-window-panel button')].find(item => item.title === title).click();
  const icons = () => [...host.querySelectorAll('.object-window-panel button')].slice(0, 2).map(item => item.querySelector('use').getAttribute('href'));
  const rows = () => [...host.querySelectorAll('.object-window-body .studio-tree-row')].map(item => item.textContent.trim());
  const palette = () => ({title: document.querySelector('#inspector-title').textContent,
    tabs: [...document.querySelectorAll('#properties .studio-tab')].map(tab => tab.textContent),
    rows: [...document.querySelectorAll('#properties .object-palette-row .object-palette-caption')].map(item => item.textContent)});
  await openObject('СкладСервис');
  await windowPage('Операции');
  steps.opened = {panel: panel(), rows: rows(), icons: icons()};
  press('Добавить операцию'); await settle();
  steps.operation = {rows: rows(), chosen: host.querySelector('.object-window-body .studio-tree-row.selected')?.textContent.trim() || '', palette: palette(),
    type: windowField('#properties', 'Тип возвращаемого значения').querySelector('input').value,
    typeChoice: windowField('#properties', 'Тип возвращаемого значения').querySelector('button').disabled, panel: panel()};
  windowField('#properties', 'Тип возвращаемого значения').querySelector('button').click(); await settle();
  const choice = document.querySelector('dialog.xml-type-choice-dialog');
  const okButton = () => [...choice.querySelectorAll('.localized-title-actions button')].find(item => item.textContent === 'OK');
  steps.typeWindow = {title: choice.querySelector('h3').textContent,
    namespaces: [...choice.querySelector('.xml-type-choice').children].map(item => {
      const row = item.matches('.studio-tree-row') ? item : item.querySelector(':scope > .studio-tree-row');
      return row.textContent + ' ' + item.querySelectorAll('.studio-tree-children .studio-tree-row').length + (item.open ? ' (раскрыто)' : '');
    }),
    first: [...choice.querySelectorAll('details[open] .studio-tree-children .studio-tree-row')][0]?.textContent || '',
    marked: choice.querySelector('.studio-tree-row.selected')?.textContent || '', ok: !okButton().disabled};
  [...choice.querySelectorAll('.studio-tree-row')].find(item => item.textContent === 'int').click(); await settle();
  okButton().click(); await settle();
  steps.chosenType = windowField('#properties', 'Тип возвращаемого значения').querySelector('input').value;
  [...document.querySelectorAll('#properties .studio-tab')].find(tab => tab.textContent === 'Данные').click(); await settle();
  const lock = windowField('#properties', 'Режим управления блокировкой данных').querySelector('select');
  steps.lock = {value: lock.value, choices: [...lock.options].map(option => option.textContent)};
  [...document.querySelectorAll('#properties .studio-tab')].find(tab => tab.textContent === 'Основные').click(); await settle();
  const procedure = windowField('#properties', 'Имя процедуры').querySelector('input');
  procedure.focus(); procedure.value = 'Проверка'; procedure.dispatchEvent(new Event('input', {bubbles: true})); document.querySelector('#refresh').focus(); procedure.dispatchEvent(new Event('change', {bubbles: true})); await settle();
  press('Добавить параметр'); await settle();
  const direction = windowField('#properties', 'Направление передачи').querySelector('select');
  steps.parameter = {rows: rows(), palette: palette(), direction: direction.value, choices: [...direction.options].map(option => option.textContent),
    type: windowField('#properties', 'Тип значения').querySelector('input').value};
  [...host.querySelectorAll('.object-window-body .studio-tree-row')].find(item => item.textContent.trim() === 'Операция1').click(); await settle();
  press('Переместить вверх'); await settle();
  steps.moved = rows();
  [...host.querySelectorAll('.object-window-body .studio-tree-row')].find(item => item.textContent.trim() === 'Параметр1').click(); await settle();
  press('Удалить'); await settle();
  steps.question = document.querySelector('dialog[open] h3')?.textContent || '';
  [...document.querySelectorAll('dialog[open] button')].find(item => item.textContent === 'Да').click(); await settle();
  steps.deleted = rows();
  document.querySelector('#save').click(); await settle();
  steps.saved = !document.querySelector('#save').disabled;
  return steps;
})()`

// The page «Операции» of a web service is the prototype's (docs/block-00/
// web-servis.md, «Окно», «Палитра», «Новый сервис»; snapshots
// web-servis/02, 09, 10, 14, 15): the panel - «Добавить операцию»,
// «Добавить параметр» (unavailable with nothing chosen), the shared buttons,
// «Сортировать» unavailable - and the tree of the operations with their
// parameters; a new operation «Операция1» returning a string of XMLSchema
// with the lock managed, its palette on «Основные» and «Данные» of two
// values; a new parameter «Параметр1», a string, «Входной», of three
// directions; moving round the ends; deleting after the question;
// «…» of a type opening the window choosing it among XML Schema and the
// packages of the service; «Сохранить» writing it all. Defects caught: a
// default not the prototype's; a palette not the part's; the window of a
// type offering what the service does not use, or not the type before; the lock offering a third value; a part removed without the
// question; the operations not written.
func TestTheOperationsOfAWebServiceAreThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addWebService(t, root)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, webServiceOperationsWalk)
	if err != nil {
		t.Fatal(err)
	}
	type palette struct {
		Title      string
		Tabs, Rows []string
	}
	var steps struct {
		Opened    struct{ Panel, Rows, Icons []string }
		Operation struct {
			Rows, Panel  []string
			Chosen, Type string
			Palette      palette
			TypeChoice   bool
		}
		Lock struct {
			Value   string
			Choices []string
		}
		Parameter struct {
			Rows, Choices   []string
			Palette         palette
			Direction, Type string
		}
		Moved, Deleted []string
		TypeWindow     struct {
			Title, First, Marked string
			Namespaces           []string
			OK                   bool
		}
		ChosenType string
		Question   string
		Saved      bool
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
	expect("the page", steps.Opened, struct{ Panel, Rows, Icons []string }{[]string{"Добавить операцию", "Добавить параметр (недоступна)", "Изменить (недоступна)",
		"Удалить (недоступна)", "Переместить вверх (недоступна)", "Переместить вниз (недоступна)", "Сортировать (недоступна)"}, []string{"Остаток"},
		[]string{"#icon-web-service-operation-add", "#icon-parameter-add"}})
	expect("a new operation", []any{steps.Operation.Rows, steps.Operation.Chosen, steps.Operation.Palette, steps.Operation.Type, steps.Operation.TypeChoice},
		[]any{[]string{"Остаток", "Операция1"}, "Операция1", palette{"Свойства: Операция1", []string{"Основные", "Данные"},
			[]string{"Имя", "Синоним", "Комментарий", "Тип возвращаемого значения", "Возможно пустое значение", "В транзакции", "Имя процедуры"}},
			"string (http://www.w3.org/2001/XMLSchema)", false})
	// The service uses Товары and data/core: the window lists them and XML
	// Schema by the alphabet of the URI, XML Schema open - 91 types from
	// «all» - data/core of 46, Товары of no type yet (web-servis.md, «Выбор
	// типа из палитры параметра», the third pass); the type before - a
	// string - is marked and OK available.
	expect("the window choosing a type", steps.TypeWindow, struct {
		Title, First, Marked string
		Namespaces           []string
		OK                   bool
	}{"Выбор пакетов XDTO", "all", "string", []string{"http://example.org/goods 0", "data/core 46",
		"http://www.w3.org/2001/XMLSchema 91 (раскрыто)"}, true})
	expect("a type chosen", steps.ChosenType, "int (http://www.w3.org/2001/XMLSchema)")
	expect("the lock of an operation", steps.Lock, struct {
		Value   string
		Choices []string
	}{"managed", []string{"Автоматический", "Управляемый"}})
	expect("a new parameter", []any{steps.Parameter.Rows, steps.Parameter.Palette, steps.Parameter.Direction, steps.Parameter.Choices, steps.Parameter.Type},
		[]any{[]string{"Остаток", "Операция1", "Параметр1"}, palette{"Свойства: Параметр1", []string{"Основные"},
			[]string{"Имя", "Синоним", "Комментарий", "Тип значения", "Возможно пустое значение", "Направление передачи"}},
			"in", []string{"Входной", "Выходной", "Входной - Выходной"}, "string (http://www.w3.org/2001/XMLSchema)"})
	expect("moved up", steps.Moved, []string{"Операция1", "Параметр1", "Остаток"})
	expect("deleted", []any{steps.Question, steps.Deleted}, []any{"Удалить объект проекта?", []string{"Операция1", "Остаток"}})
	expect("saved", steps.Saved, false)
	service, _ := os.ReadFile(filepath.Join(root, "metadata", "web-services", "СкладСервис", "object.yaml"))
	if !strings.Contains(string(service), "name: Операция1") || !strings.Contains(string(service), "procedure: Проверка") || !strings.Contains(string(service), "name: int") || strings.Contains(string(service), "Параметр1") {
		t.Errorf("on disk the service is %s", service)
	}
}
