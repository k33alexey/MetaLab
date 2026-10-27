package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wsReferenceWalk imports a description by «Добавить» of the WS references,
// reads the window of the address, the palette and the menu of the
// reference made; then imports a text that is no WSDL and reads the
// refusal, and cancels a third import.
func wsReferenceWalk(good, bad string) string {
	return `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 80)).then(() => studio.settle());
  const dialog = () => document.querySelector('dialog[open]');
  const press = caption => [...dialog().querySelectorAll('button')].find(item => item.textContent === caption).click();
  const type = text => { const field = dialog().querySelector('input'); field.value = text; field.dispatchEvent(new Event('input', {bubbles: true})); };
  await openObject('WS-ссылки');
  document.querySelector('#tree-add').click(); await settle();
  steps.window = {title: dialog()?.querySelector('h3')?.textContent || '', fields: dialog().querySelectorAll('input').length,
    buttons: [...dialog().querySelectorAll('button')].map(item => item.textContent)};
  type(` + jsString(good) + `); press('ОК'); await settle();
  steps.made = {tree: !!treeRow('WSСсылка1'), title: document.querySelector('#inspector-title').textContent,
    rows: [...document.querySelectorAll('#properties .object-palette-row .object-palette-caption')].map(item => item.textContent),
    address: windowField('#properties', 'URL источника').querySelector('.object-palette-address').textContent,
    open: windowField('#properties', 'WS-ссылка').querySelector('button').disabled};
  treeRow('WSСсылка1').dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, clientX: 50, clientY: 50})); await settle();
  steps.menu = [...document.querySelectorAll('.context-menu .context-menu-item')].map(item => item.textContent + (item.disabled ? ' (недоступна)' : '') + (item.classList.contains('context-menu-main') ? ' (жирный)' : ''));
  document.dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape', bubbles: true})); closeContextMenu(); await settle();
  await openObject('WS-ссылки');
  document.querySelector('#tree-add').click(); await settle();
  type(` + jsString(bad) + `); press('ОК'); await settle();
  steps.refusal = dialog()?.querySelector('h3')?.textContent || '';
  press('ОК'); await settle();
  await openObject('WS-ссылки');
  document.querySelector('#tree-add').click(); await settle();
  press('Отмена'); await settle();
  steps.references = [...document.querySelectorAll('#tree .studio-tree-row')].filter(row => row.textContent.trim().startsWith('WSСсылка')).length;
  return steps;
})()`
}

// jsString is a text as a literal of JavaScript.
func jsString(text string) string {
	encoded, _ := json.Marshal(text)
	return string(encoded)
}

// «Добавить» of the WS references is the prototype's import
// (docs/block-00/ws-ssylka.md; snapshots 01, 03, 10, 12): the window
// «Введите URL импортируемого WSDL» of one line with OK and «Отмена»; the
// reference made shows its palette - the name, the synonym, the comment,
// «URL источника» as an address, «WS-ссылка: Открыть» available - the
// window viewing the description; its menu has «Просмотр», bold, and
// «Импорт WSDL…». A text that is not WSDL is
// refused in the prototype's words, and «Отмена» makes nothing. Defects
// caught: a reference made without its description; the address not shown
// or editable; the menu of the reference without its commands; a refused
// description making a reference.
func TestTheImportOfAWSReferenceIsThePrototypes(t *testing.T) {
	files := t.TempDir()
	good, bad := filepath.Join(files, "stock.wsdl"), filepath.Join(files, "note.txt")
	if err := os.WriteFile(good, []byte(wsdlDescription), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("не WSDL"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, browser, page := openStudioInBrowser(t, copyDemoProjectForStudioTest(t), func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, wsReferenceWalk(good, bad))
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Window struct {
			Title   string
			Fields  int
			Buttons []string
		}
		Made struct {
			Tree, Open     bool
			Title, Address string
			Rows           []string
		}
		Menu       []string
		Refusal    string
		References int
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
	expect("the window of the address", steps.Window, struct {
		Title   string
		Fields  int
		Buttons []string
	}{"Введите URL импортируемого WSDL", 1, []string{"ОК", "Отмена"}})
	expect("the reference made", []any{steps.Made.Tree, steps.Made.Title, steps.Made.Rows, steps.Made.Open},
		[]any{true, "Свойства: WSСсылка1", []string{"Имя", "Синоним", "Комментарий", "URL источника", "WS-ссылка"}, false})
	if !strings.HasPrefix(steps.Made.Address, "file:///") || !strings.HasSuffix(steps.Made.Address, "/stock.wsdl") {
		t.Errorf("the address shown is %q", steps.Made.Address)
	}
	expect("its menu", steps.Menu, []string{"Просмотр (жирный)", "Импорт WSDL…"})
	expect("a text that is no WSDL", steps.Refusal, "Ошибка разбора WSDL: неверный формат WSDL описания")
	expect("references in the tree", steps.References, 1)
}
