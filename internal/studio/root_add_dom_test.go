package studio

import (
	"encoding/json"
	"testing"
)

// rootAddWalk opens «Добавить ▸» of the menu of the root, reads what stands
// before it and its items, their signs and which are available, and makes a
// subsystem by it.
const rootAddWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;
  const settle = () => new Promise(resolve => setTimeout(resolve, 80)).then(() => studio.settle());
  const root = document.querySelector('#tree summary');
  root.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10})); await settle();
  const entries = within => [...within.children].filter(item => item.matches('.context-menu-item, .context-menu-separator'));
  const top = entries(document.querySelector('.context-menu'));
  const parent = top.findIndex(item => item.classList.contains('context-menu-parent'));
  steps.tail = top.slice(parent - 2, parent + 1).map(item => item.matches('.context-menu-separator') ? '|' : item.childNodes[1]?.textContent ?? item.textContent);
  top.find(item => item.classList.contains('context-menu-parent')).click(); await settle();
  const submenu = document.querySelector('.context-submenu');
  steps.items = entries(submenu).map(item => item.matches('.context-menu-separator') ? '|' :
    item.childNodes[1].textContent + (item.querySelector('.context-menu-key') ? ' ' + item.querySelector('.context-menu-key').textContent : '') +
    (item.querySelector('use') ? '' : ' (без значка)'));
  steps.available = entries(submenu).filter(item => item.matches('.context-menu-item') && !item.disabled).map(item => item.childNodes[1].textContent);
  entries(submenu).find(item => item.childNodes[1]?.textContent === 'Подсистема').click(); await settle();
  steps.created = document.querySelector('#subsystem-editor .object-window-title')?.textContent || '';
  return steps;
})()`

// «Добавить ▸» of the menu of the root is the prototype's
// (docs/block-00/konfiguraciya.md, «Команды»; snapshot
// configurator-2026-10-27/series-2.595/35): after «Все картинки» and a line,
// the common kinds from «Язык» to «Определяемый тип» in the prototype's
// order - «Бот» not carried - each with its sign, then a line and «Добавить»
// (Ins) unavailable; an item is available where the tree's «Добавить» of its
// branch makes an object, and makes it as that does. Defects caught: an item
// missing, extra - a common form, a style, an applied kind - or out of the
// prototype's order; an item offered for a kind Studio does not make; an
// item making nothing or another kind.
func TestTheRootAddsAsThePrototype(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, rootAddWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Tail, Items, Available []string
		Created                string
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
	expect("before «Добавить ▸»", steps.Tail, []string{"Все картинки", "|", "Добавить"})
	expect("«Добавить ▸»", steps.Items, []string{"Язык", "Подсистема", "Элемент стиля", "Общая картинка", "Параметр сеанса", "Роль",
		"Общий макет", "Критерий отбора", "Общий модуль", "Общий реквизит", "План обмена", "XDTO-пакет", "Web-сервис", "HTTP-сервис",
		"WS-ссылка", "WebSocket-клиент", "Подписка на событие", "Регламентное задание", "Хранилище настроек", "Функциональная опция",
		"Параметр функциональных опций", "Определяемый тип", "|", "Добавить Ins"})
	// What Studio makes now - the kinds with a window that makes a new
	// object (the tree's «Добавить»): the list grows with the windows; a
	// web service and an HTTP service have windows and are not made yet.
	expect("available", steps.Available, []string{"Подсистема", "Общая картинка", "Роль", "Общий макет", "Общий реквизит", "План обмена",
		"XDTO-пакет", "WS-ссылка", "WebSocket-клиент"})
	expect("made", steps.Created, "Подсистема Подсистема1")
}
