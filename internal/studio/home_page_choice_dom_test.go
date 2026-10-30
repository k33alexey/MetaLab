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
// it; OK on a branch refused «Объект не выбран.», the window staying;
// «Отмена» of the choice after «Скопировать» makes current again the row
// copied; a role said explicitly - even as the common box says - makes the
// link «Различная». Defects caught: the forms right under the object; OK on
// a branch unavailable, closing the window or choosing nothing silently;
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
