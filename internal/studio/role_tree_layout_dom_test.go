package studio

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// roleTreeLayoutWalk opens the window of the role Кладовщик and «Все роли»
// and reads the outline of each tree of rights: the rows beneath the root and
// beneath «Общие» and «Документы», by caption.
const roleTreeLayoutWalk = `(async () => {
  const studio = window.__studio, answer = {};
  const caption = details => details.querySelector(':scope > .studio-tree-row').textContent.trim();
  const beneath = details => [...details.querySelector(':scope > .studio-tree-children').children].map(item => item.matches('details') ? caption(item) : item.textContent.trim());
  const outline = host => {
    const top = [...host.children].filter(item => item.matches('details, .studio-tree-row'));
    const root = top[0], branch = text => [...root.querySelectorAll('details')].find(item => caption(item) === text);
    return {top: top.map(item => item.matches('details') ? caption(item) : item.textContent.trim()), icon: root.querySelector(':scope > .studio-tree-row use')?.getAttribute('href'),
      root: beneath(root), common: branch('Общие') ? beneath(branch('Общие')) : null, documents: branch('Документы') ? beneath(branch('Документы')) : null};
  };
  const tree = text => [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === text);
  tree('Кладовщик').click(); await studio.settle();
  answer.role = outline(document.querySelector('#role-editor .role-tree'));
  tree('Роли').dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent === 'Все роли').click();
  await studio.settle();
  answer.roles = outline(document.querySelector('#role-overview .role-tree'));
  return answer;
})()`

// The tree of rights stands as the project tree and the prototype's: the
// configuration at the top with everything beneath it, «Общие» first holding
// the common kinds, then the branches of the kinds in the prototype's order,
// the sequences inside «Документы» before the documents (inventory of block
// 00, rol.md - «корень конфигурации, «Общие», виды»; snapshots
// configurator-2026-10-14/rol-otbor-kassa, derevo/27 for the order; written
// out here, not read from the descriptors). Defects caught: the root a row
// of its own above the branches; no «Общие», a common form standing beside
// the catalogs; the sequences beside the documents rather
// than in them; the order of the branches not the prototype's.
func TestTheTreeOfRightsStandsAsTheProjectTree(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	writeProjectFile(t, root, "metadata/sequences/Партии/object.yaml",
		"format: 1\nid: "+uuid.MustNew().String()+"\nname: Партии\ntitle: {ru: Партии}\n")
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, roleTreeLayoutWalk)
	if err != nil {
		t.Fatal(err)
	}
	type outline struct {
		Top, Root, Common, Documents []string
		Icon                         string
	}
	var got struct{ Role, Roles outline }
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	for name, tree := range map[string]outline{"role": got.Role, "all roles": got.Roles} {
		if len(tree.Top) != 1 || tree.Icon != "#icon-configuration" {
			t.Errorf("%s: the tree begins %v (icon %s), want the configuration alone", name, tree.Top, tree.Icon)
		}
		if want := []string{"Общие", "Справочники", "Документы", "Регистры накопления"}; !slices.Equal(tree.Root, want) {
			t.Errorf("%s: beneath the root %v, want %v", name, tree.Root, want)
		}
		if !slices.Equal(tree.Common, []string{"Общие формы"}) {
			t.Errorf("%s: «Общие» holds %v, want the common forms - the one common kind of the project with rights", name, tree.Common)
		}
		if len(tree.Documents) < 2 || tree.Documents[0] != "Последовательности" || !strings.Contains(strings.Join(tree.Documents[1:], " "), "Продажа") {
			t.Errorf("%s: «Документы» holds %v, want the sequences, then the documents", name, tree.Documents)
		}
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
