package studio

import (
	"encoding/json"
	"strings"
	"testing"
)

// overviewHeadWalk opens «Все ограничения доступа» and «Все роли» from the
// menu of the branch «Роли» and reads the head of each window.
const overviewHeadWalk = `(async () => {
  const studio = window.__studio, answer = {};
  const open = async text => {
    const roles = [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === 'Роли');
    roles.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
    [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent === text).click();
    await studio.settle();
    const host = document.querySelector('#role-overview'), head = host.querySelector('.overview-header');
    return {text: host.textContent, head: head ? [...head.children].map(child => child.className) : null};
  };
  answer.restrictions = await open('Все ограничения доступа');
  answer.roles = await open('Все роли');
  return answer;
})()`

// Neither window over all roles counts the roles: the prototype's do not
// (snapshot configurator-2026-10-18/shabl-01, inventory
// vse-ogranicheniya-dostupa.md; the owner's word of 20.10.2026). The head of
// «Все ограничения доступа» holds its strip of tabs alone, «Все роли» has no
// head. Defects caught: «Ролей в проекте: N» left in a window; a head of
// nothing left over «Все роли»; something beside the tabs in the head.
func TestWindowsOverAllRolesCountNoRoles(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, overviewHeadWalk)
	if err != nil {
		t.Fatal(err)
	}
	type window struct {
		Text string
		Head []string
	}
	var got struct{ Restrictions, Roles window }
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	for name, item := range map[string]window{"Все ограничения доступа": got.Restrictions, "Все роли": got.Roles} {
		if strings.Contains(item.Text, "Ролей в проекте") {
			t.Errorf("«%s» counts the roles", name)
		}
	}
	if len(got.Restrictions.Head) != 1 || got.Restrictions.Head[0] != "studio-tabs" {
		t.Errorf("the head of «Все ограничения доступа» holds %v, want its strip of tabs alone", got.Restrictions.Head)
	}
	if got.Roles.Head != nil {
		t.Errorf("«Все роли» has a head of %v, want none", got.Roles.Head)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
