package studio

import (
	"encoding/json"
	"slices"
	"testing"
)

// roleTemplateWindow opens «Все ограничения доступа» from the branch «Роли»,
// goes to «Шаблоны ограничений», opens the template and answers what the
// window shows: its title, what stands in it from the top, whether the flags
// are set, its buttons and whether they stand to the right of the content.
const roleTemplateWindow = `(async () => {
  const studio = window.__studio;
  const roles = [...document.querySelectorAll('#tree summary, #tree .leaf')].find(item => item.textContent.trim() === 'Роли');
  roles.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent === 'Все ограничения доступа').click();
  await studio.settle();
  [...document.querySelectorAll('.studio-tab')].find(item => item.textContent === 'Шаблоны ограничений').click();
  await studio.settle();
  document.querySelector('.overview-block .role-list tbody tr[tabindex]').dispatchEvent(new MouseEvent('dblclick', {bubbles: true}));
  await studio.settle();
  const dialog = document.querySelector('dialog[open]');
  const form = dialog.querySelector('.role-restriction-form'), actions = dialog.querySelector('.localized-title-actions');
  const answer = {
    title: dialog.querySelector('h3').textContent,
    content: [...form.children].map(item => item.matches('label') ? 'flag:' + item.textContent.trim() : item.matches('input') ? 'input' : item.classList.contains('role-restriction-caption') ? item.textContent : 'editor'),
    flags: [...form.querySelectorAll(':scope > label input')].map(input => input.checked),
    buttons: [...actions.querySelectorAll('button')].map(button => button.textContent),
    beside: actions.getBoundingClientRect().left >= form.getBoundingClientRect().right,
  };
  [...actions.querySelectorAll('button')].find(button => button.textContent === 'Отмена').click();
  await studio.settle();
  return answer;
})()`

// The window that changes one template of restrictions is the prototype's
// (snapshot configurator-2026-10-18/shabl-16; help, form_EditRowTemplatesDlg):
// titled for one template, «Изменять наименование» above «Наименование:» and
// its field, «Изменять текст» above «Текст шаблона:» and the text, both set
// when one template is changed, and ОК, Отмена, Справка in a column to the
// right. Defects caught: the title in the plural for one template; the flags
// missing for one template or standing below their fields; the captions
// without their colon; the buttons in a row under the content or without
// «Справка».
func TestTheWindowOfATemplateIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) { addRestrictedRole(t, workspace, root) })
	result, err := browser.evaluate(ctx, page, roleTemplateWindow)
	if err != nil {
		t.Fatal(err)
	}
	var window struct {
		Title   string
		Content []string
		Flags   []bool
		Buttons []string
		Beside  bool
	}
	if err := json.Unmarshal(result, &window); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	if window.Title != "Изменение шаблона ограничений" {
		t.Errorf("the window is titled %q", window.Title)
	}
	want := []string{"flag:Изменять наименование", "Наименование:", "input", "flag:Изменять текст", "Текст шаблона:", "editor"}
	if !slices.Equal(window.Content, want) {
		t.Errorf("the window shows %v, want %v", window.Content, want)
	}
	if !slices.Equal(window.Flags, []bool{true, true}) {
		t.Errorf("the flags stand %v, want both set", window.Flags)
	}
	if !slices.Equal(window.Buttons, []string{"ОК", "Отмена", "Справка"}) || !window.Beside {
		t.Errorf("the buttons are %v, beside the content: %v; want ОК, Отмена, Справка to the right", window.Buttons, window.Beside)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
