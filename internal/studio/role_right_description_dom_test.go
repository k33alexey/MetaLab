package studio

import (
	"encoding/json"
	"slices"
	"testing"
)

// roleRightDescriptionWalk opens the window of the role Кладовщик and walks
// its list of rights as the owner would: the configuration as it opens, a
// right chosen by its caption, the catalog Товары, a right with no
// restriction, the document ПродажаТоваров, then the arrow and the space in
// the list. After each step it reads the current right, the description
// under the tree and the restrictions listed.
const roleRightDescriptionWalk = `(async () => {
  const studio = window.__studio, steps = [], editor = document.querySelector('#role-editor');
  const read = () => {
    const current = editor.querySelector('.role-permissions .check-list-row.current');
    const section = editor.querySelector('.role-restrictions');
    steps.push({
      rights: [...editor.querySelectorAll('.role-permissions .check-list-row')].map(row => row.textContent.trim()),
      current: current ? current.textContent.trim() : '',
      checked: current ? current.querySelector('input').checked : false,
      description: editor.querySelector('.role-description').textContent,
      heading: section ? section.querySelector('h4').textContent : '',
      restrictions: section ? section.querySelectorAll('.role-list tbody tr[tabindex]').length : -1,
      canAdd: section ? !section.querySelector('button[data-command=add]')?.disabled : false,
    });
  };
  const row = text => [...editor.querySelectorAll('.role-permissions .check-list-row')].find(item => item.textContent.trim() === text);
  // An object by its name, which the hint of its caption is: the caption is its synonym.
  const object = name => [...editor.querySelectorAll('.role-tree-item[data-object]')].find(item => item.title === name);
  [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === 'Кладовщик').click();
  await studio.settle(); read();
  row('Вывод').click(); await studio.settle(); read();
  object('Товары').click(); await studio.settle(); read();
  row('Просмотр').click(); await studio.settle(); read();
  object('ПродажаТоваров').click(); await studio.settle(); read();
  const list = editor.querySelector('.role-permissions .check-list');
  list.dispatchEvent(new KeyboardEvent('keydown', {key: 'ArrowDown', bubbles: true})); await studio.settle(); read();
  editor.querySelector('.role-permissions .check-list').dispatchEvent(new KeyboardEvent('keydown', {key: ' ', bubbles: true})); await studio.settle(); read();
  return steps;
})()`

// The list of rights of the window of a role is the prototype's
// (configurator 8.3.27, pass of 20.10.2026, block-00/rol): one current
// right, the description of which stands under the tree with its name in
// brackets, and whose restrictions are listed under the heading «Ограничения
// доступа к данным:»; the current right stays on the next node that has it,
// elsewhere the first is current; the arrows move it, the space marks it.
// Defects caught: the captions made from the name («Режим основного окна
// обычный»); no description, or one of another right than the current; the
// current right lost on choosing another object; the restrictions listed by
// a tab of their own rather than for the current right; the old heading
// «Ограничение доступа к данным»; the keys of the list doing nothing.
func TestTheListOfRightsHasACurrentRightDescribedUnderTheTree(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, roleRightDescriptionWalk)
	if err != nil {
		t.Fatal(err)
	}
	type step struct {
		Rights                        []string
		Current, Description, Heading string
		Checked, CanAdd               bool
		Restrictions                  int
	}
	var steps []step
	if err := json.Unmarshal(result, &steps); err != nil || len(steps) != 7 {
		t.Fatalf("%v: %s", err, result)
	}
	opened, output, goods, view, sale, down, space := steps[0], steps[1], steps[2], steps[3], steps[4], steps[5], steps[6]
	if !slices.Contains(opened.Rights, `Режим основного окна "Обычный"`) || opened.Current != "Администрирование" ||
		opened.Description != "Описание:Право выполнять административные функции\n(Администрирование)" {
		t.Errorf("as it opens: the current right %q, described %q, among %v", opened.Current, opened.Description, opened.Rights)
	}
	if opened.Heading != "Ограничения доступа к данным:" || opened.Restrictions != 0 || opened.CanAdd {
		t.Errorf("the configuration: the restrictions headed %q, %d listed, adding offered %v; want the heading, none, none",
			opened.Heading, opened.Restrictions, opened.CanAdd)
	}
	if output.Current != "Вывод" || output.Description != "Описание:Право печатать, сохранять в файл и копировать в буфер обмена\n(Вывод)" {
		t.Errorf("after a click on «Вывод»: the current right %q, described %q", output.Current, output.Description)
	}
	if goods.Current != "Чтение" || goods.Restrictions != 1 || !goods.CanAdd {
		t.Errorf("on Товары, which has no «Вывод»: the current right %q with %d restrictions (adding %v), want «Чтение» with its one",
			goods.Current, goods.Restrictions, goods.CanAdd)
	}
	if view.Current != "Просмотр" || view.Restrictions != 0 || view.CanAdd || view.Description != "Описание:Право видеть в интерфейсе\n(Просмотр)" {
		t.Errorf("after a click on «Просмотр»: %q, %d restrictions, adding %v, described %q; want no restriction can be put on it",
			view.Current, view.Restrictions, view.CanAdd, view.Description)
	}
	if sale.Current != "Просмотр" {
		t.Errorf("on ПродажаТоваров the current right is %q, want «Просмотр» kept", sale.Current)
	}
	if down.Current != "Интерактивное добавление" || down.Checked {
		t.Errorf("after the arrow down: %q (marked %v), want «Интерактивное добавление», not marked", down.Current, down.Checked)
	}
	if space.Current != "Интерактивное добавление" || !space.Checked {
		t.Errorf("after the space: %q (marked %v), want it marked", space.Current, space.Checked)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
