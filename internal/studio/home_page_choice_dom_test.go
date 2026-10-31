package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
)

// homePageChoiceWalk opens the work area of the home page, «Добавить»:
// reads the branch of Товары in «Выбор управляемой формы», presses OK on
// it and reads the refusal; chooses the list form; adds the common form;
// makes the first row current, copies it and cancels the choice; makes the
// first row visible to Первая explicitly, as the common box says.
const homePageChoiceWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;
  const settle = () => new Promise(resolve => setTimeout(resolve, 120)).then(() => studio.settle());
  await settle();
  const root = document.querySelector('#tree summary');
  root.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10})); await settle();
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent.includes('Открыть рабочую область начальной страницы')).click(); await settle();
  const host = document.querySelector('#home-page');
  const column = () => host.querySelector('.home-page-left');
  const current = () => [...column().querySelectorAll('.home-page-row')].map(row => row.classList.contains('current'));
  const button = title => [...column().querySelectorAll('.object-window-panel button')].find(item => item.title === title);
  const dialogs = () => [...document.querySelectorAll('dialog[open]')];
  const dialog = () => dialogs().at(-1);
  const press = text => [...dialog().querySelectorAll('button')].find(item => item.textContent === text).click();
  const rowNamed = name => [...dialog().querySelectorAll('.studio-tree-row')].find(row => row.textContent === name);
  button('Добавить новый элемент').click(); await settle();
  const tree = dialog().querySelector('.object-choice-tree'), now = () => dialog().querySelector('.studio-tree-row.selected')?.textContent || '';
  steps.keys = [(document.activeElement === tree ? 'дерево ' : 'не дерево ') + now()];
  for (const key of ['с', 'ArrowRight', 'ArrowRight', 'ArrowRight', 'ArrowRight', 'ArrowRight', 'ArrowRight']) {
    tree.dispatchEvent(new KeyboardEvent('keydown', {key, bubbles: true})); steps.keys.push(now());
  }
  tree.dispatchEvent(new KeyboardEvent('keydown', {key: 'ArrowLeft', bubbles: true})); steps.keys.push(now());
  const search = dialog().querySelector('.object-choice-search input');
  const typed = async text => {
    search.value = text; search.dispatchEvent(new Event('input', {bubbles: true})); await settle();
    return [...dialog().querySelectorAll('.studio-tree-row')].map(row => [row.textContent,
      [...row.querySelectorAll('.studio-search-match')].map(part => part.textContent).join('+'), row.classList.contains('studio-search-path')]);
  };
  steps.searched = {list: await typed('списк'), goods: await typed('ТОВАР'), forms: await typed('форм')};
  search.dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter', bubbles: true})); await settle();
  steps.enter = dialogs().length === 2 ? dialog().querySelector('h3').textContent : '';
  press('OK'); await settle();
  await typed('');
  const goods = rowNamed('Товары');
  goods.click(); await settle();
  steps.under = [...goods.parentElement.querySelectorAll(':scope > .studio-tree-children .studio-tree-row')].map(row => row.textContent);
  press('OK'); await settle();
  steps.refusal = {count: dialogs().length, text: dialog().querySelector('h3').textContent};
  press('OK'); await settle();
  steps.stays = dialog().querySelector('h3').textContent;
  rowNamed('ФормаСписка').click(); await settle(); press('OK'); await settle();
  button('Добавить новый элемент').click(); await settle();
  rowNamed('ДемоФормаТовара').click(); await settle(); press('OK'); await settle();
  column().querySelectorAll('.home-page-row')[0].dispatchEvent(new MouseEvent('mousedown', {bubbles: true})); await settle();
  button('Добавить новый элемент копированием текущего').click(); await settle();
  steps.copied = now();
  press('Отмена'); await settle();
  steps.afterCancel = current();
  column().querySelector('.home-page-roles').click(); await settle();
  for (let time = 0; time < 2; time++) { dialog().querySelector('.role-visibility[aria-label="Видимость: Первая"]').click(); await settle(); }
  steps.turned = dialog().querySelector('.role-visibility[aria-label="Видимость: Первая"]').dataset.state;
  press('OK'); await settle();
  steps.link = column().querySelector('.home-page-roles').textContent;
  return steps;
})()`

// «Выбор управляемой формы» and «Отмена» of the work area of the home page
// are the prototype's (konfiguraciya.md, «Досъёмка 29.10.2026, вечер»; В2,
// В4, В5 of series 2.646): under an object the branch «Формы», the forms in
// it; the search narrows the tree by the names of the rows, in any case,
// the found part red, a branch shown only for what it holds grey, Enter in
// it is OK (snapshots dosyomka-2653/34, 35); the keys of a tree of choice,
// the focus on it, the current row the first or the form chosen
// (obshchee.md, «Окна выбора — одно поведение»); OK on a branch refused
// «Объект не выбран.», the window staying;
// «Отмена» of the choice after «Скопировать» makes current again the row
// copied; a role said explicitly - even as the common box says - makes the
// link «Различная». Defects caught: the forms right under the object; OK on
// a branch unavailable, closing the window or choosing nothing silently;
// the focus in the search, no current row, the chosen form not current;
// the search by the full name - Товары finding its ФормаСписка -, the found
// part not drawn, a branch of the path drawn as a match, a branch holding
// what is searched drawn grey;
// the row before the last made current after «Отмена»; the link
// «Одинаковая» while a role is said.
func TestTheChoiceOfAFormOfTheHomePageIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	common := filepath.Join(root, "metadata", "common-forms", "ДемоФормаТовара", "form.yaml")
	content, err := os.ReadFile(common)
	if err != nil {
		t.Fatal(err)
	}
	form := strings.Replace(strings.Replace(string(content), "10000000-0000-4000-8000-000000000501", "10000000-0000-4000-8000-000000000601", 1),
		"name: ДемоФормаТовара", "name: ФормаСписка", 1)
	directory := filepath.Join(root, "metadata", "catalogs", "Товары", "forms", "ФормаСписка")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "form.yaml"), []byte(form), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		role, err := workspace.CreateRole("Р1")
		if err != nil {
			t.Fatal(err)
		}
		role.Role.Title = metadata.LocalizedText{"ru": "Первая"}
		if _, err := workspace.SaveRole(role.Path, role.Role, role.Revision); err != nil {
			t.Fatal(err)
		}
	})
	result, err := browser.evaluate(ctx, page, homePageChoiceWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Under   []string
		Refusal struct {
			Count int
			Text  string
		}
		Searched struct {
			List, Goods, Forms [][]any
		}
		Keys        []string
		Copied      string
		Enter       string
		Stays       string
		AfterCancel []bool
		Turned      string
		Link        string
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
	// The tree opens with the focus and the first row current where nothing
	// is chosen, the form chosen where one is (dosyomka-2667/12, 15); a
	// letter, → into the branches down to a form, ← to its branch.
	expect("keys", steps.Keys, []string{"дерево Общие", "Справочники", "Справочники", "Товары", "Товары", "Формы", "Формы", "ФормаСписка", "Формы"})
	expect("current in the choice of a copy", steps.Copied, "ФормаСписка")
	type row = []any
	expect("search «списк»", steps.Searched.List, [][]any{row{"Справочники", "", true}, row{"Товары", "", true}, row{"Формы", "", true}, row{"ФормаСписка", "Списк", false}})
	expect("search «ТОВАР»", steps.Searched.Goods, [][]any{row{"Общие", "", true}, row{"Общие формы", "", true}, row{"ДемоФормаТовара", "Товар", false}})
	expect("search «форм»", steps.Searched.Forms, [][]any{row{"Общие", "", true}, row{"Общие формы", "", true}, row{"ДемоФормаТовара", "Форм", false},
		row{"Справочники", "", true}, row{"Товары", "", true}, row{"Формы", "Форм", false}, row{"ФормаСписка", "Форм", false}})
	expect("Enter in the search on nothing chosen", steps.Enter, "Объект не выбран.")
	expect("under the object", steps.Under, []string{"Формы", "ФормаСписка"})
	expect("OK on a branch", steps.Refusal, struct {
		Count int
		Text  string
	}{2, "Объект не выбран."})
	expect("the window after the refusal", steps.Stays, "Выбор управляемой формы")
	expect("current after «Отмена» of a copy", steps.AfterCancel, []bool{true, false})
	expect("the role said", steps.Turned, "on")
	expect("the link", steps.Link, "Различная")
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
