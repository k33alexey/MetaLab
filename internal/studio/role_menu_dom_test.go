package studio

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// roleMenuWalk reads the menus of the catalog Товары, of the branch Роли and
// of the role Кладовщик in the tree, then opens the role by «Открыть права»
// and reads what is open.
const roleMenuWalk = `(async () => {
  const studio = window.__studio, answer = {};
  const row = text => [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.querySelector(':scope > span:not(.studio-tree-sign)')?.textContent === text);
  const menu = item => {
    item.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
    return [...document.querySelectorAll('.context-menu .context-menu-item')].map(entry => entry.textContent);
  };
  answer.catalog = menu(row('Товары'));
  closeContextMenu();
  answer.branch = menu(row('Роли'));
  closeContextMenu();
  answer.role = menu(row('Кладовщик'));
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(entry => entry.textContent === 'Открыть права')?.click();
  await studio.settle();
  answer.path = document.querySelector('#file-path').textContent;
  answer.window = !document.querySelector('#role-editor').hidden;
  answer.selected = document.querySelector('#tree .selected')?.textContent || '';
  return answer;
})()`

// A role's menu in the tree begins with «Открыть права» over the common part,
// and it opens the window of the role's rights (inventory of 15.10.2026,
// rol.md, «Команды»; snapshot rol/08-menyu); a catalog has no such command -
// it is the role's own -, nor has the branch Роли, whose menu is «Все роли»
// and «Все ограничения доступа» (snapshot rol/01-menyu-vetki; its «Добавить»
// comes with the common part of the menu, an item of block 2с of its own).
// Defects caught: a role with no menu at all, as before 2.565; the command
// opening nothing or another object; the command offered on an object of
// another kind or on the branch.
func TestARoleOpensItsRightsFromItsMenu(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, roleMenuWalk)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Catalog, Role, Branch []string
		Path, Selected        string
		Window                bool
	}
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	if len(got.Role) == 0 || got.Role[0] != "Открыть права" {
		t.Errorf("the menu of the role is %v, want «Открыть права» first", got.Role)
	}
	for owner, menu := range map[string][]string{"the catalog": got.Catalog, "the branch Роли": got.Branch} {
		if slices.Contains(menu, "Открыть права") {
			t.Errorf("the menu of %s %v offers «Открыть права»", owner, menu)
		}
	}
	if !slices.Equal(got.Branch, []string{"Все роли", "Все ограничения доступа"}) {
		t.Errorf("the menu of the branch Роли is %v, want «Все роли», «Все ограничения доступа»", got.Branch)
	}
	if !got.Window || !strings.HasPrefix(got.Path, "metadata/roles/") || !strings.Contains(got.Selected, "Кладовщик") {
		t.Errorf("«Открыть права» opened %q (window shown %v, selected %q), want the window of Кладовщик", got.Path, got.Window, got.Selected)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
