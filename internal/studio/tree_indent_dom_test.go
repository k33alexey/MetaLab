package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// treeIndentWalk opens every branch of every tree of Studio - the project
// tree, the tree of rights of a role, the tree of «Все роли», the folders of
// the editor of a catalog - and reads, for every row, how deep it lies, where
// its icon stands and which icon it is.
const treeIndentWalk = `(async () => {
  const studio = window.__studio, answer = {};
  const rows = host => [...host.querySelectorAll('.studio-tree-row')];
  const read = async host => {
    host.querySelectorAll('details').forEach(item => { item.open = true; });
    await new Promise(resolve => requestAnimationFrame(resolve));
    return rows(host).filter(row => row.offsetParent !== null).map(row => {
      let depth = 0;
      for (let at = row.parentElement; at && at !== host; at = at.parentElement) if (at.classList.contains('studio-tree-children')) depth++;
      const icon = row.querySelector(':scope > svg');
      return {title: row.textContent.trim(), leaf: row.classList.contains('studio-tree-leaf'), depth,
        icon: icon ? icon.getBoundingClientRect().left : -1, href: icon?.querySelector('use')?.getAttribute('href') || ''};
    });
  };
  const tree = document.querySelector('#tree'), row = text => rows(tree).find(item => item.textContent.trim() === text);
  answer.project = await read(tree);
  answer.trees = [...document.querySelectorAll('details, summary')].filter(item => !item.closest('.studio-tree-node, .studio-tree-row')).length;
  row('Кладовщик').click(); await studio.settle();
  answer.role = await read(document.querySelector('#role-editor .role-tree'));
  const loose = host => [...host.querySelectorAll('.role-tree-item')].filter(item => !item.closest('.studio-tree-row')).map(item => item.textContent);
  answer.loose = loose(document.querySelector('#role-editor .role-tree'));
  row('Роли').dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent === 'Все роли').click();
  await studio.settle();
  answer.roles = await read(document.querySelector('#role-overview .role-tree'));
  answer.loose.push(...loose(document.querySelector('#role-overview .role-tree')));
  row('Товары').click(); await studio.settle();
  [...document.querySelectorAll('.object-window-page')].find(item => item.textContent === 'Данные').click(); await studio.settle();
  answer.catalog = await read(document.querySelector('#catalog-editor'));
  return answer;
})()`

type treeRow struct {
	Title, Href string
	Leaf        bool
	Depth       int
	Icon        float64
}

// Every row of every tree of Studio stands one step right of the row it lies
// under, a row without anything to unfold as much as one with: the icon of a
// row is where its depth puts it, whatever the branch, as in the prototype's
// trees; and every row has the icon the project tree gives the same thing
// (the owner's word of 19.10.2026, snapshot
// configurator-2026-10-14/rol-otbor-kassa). Defects caught: the rows that
// hold nothing - a subsystem, a role, an attribute, an object of the tree of
// rights - with their icon under their parent's, because the place of the
// sign of unfolding is kept only by the rows that have it (noticed by the
// owner 19.10.2026); a tree of a window drawn by its own code, without icons
// or with other ones than the project tree's; a row of a tree left without
// its icon, or an object of the tree of rights left a bare button, outside
// any row.
func TestEveryRowOfTheTreeStandsOneStepRightOfItsParent(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, treeIndentWalk)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Project, Role, Roles, Catalog []treeRow
		Trees                         int
		Loose                         []string
	}
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	if len(got.Loose) != 0 {
		t.Errorf("objects and parts of the tree of rights stand outside a row of the tree: %v", got.Loose)
	}
	if got.Trees != 0 {
		t.Errorf("%d folding rows of the page are not rows of treeNode", got.Trees)
	}
	for name, rows := range map[string][]treeRow{"project": got.Project, "role": got.Role, "all roles": got.Roles, "catalog": got.Catalog} {
		icons := map[int]float64{}
		leaves := 0
		for _, row := range rows {
			if row.Leaf {
				leaves++
			}
			if row.Href == "" {
				t.Errorf("%s: %q has no icon", name, row.Title)
			}
			if at, ok := icons[row.Depth]; !ok {
				icons[row.Depth] = row.Icon
			} else if at != row.Icon {
				t.Errorf("%s: %q (depth %d, leaf %v) has its icon at %v, the rows of its depth at %v", name, row.Title, row.Depth, row.Leaf, row.Icon, at)
			}
		}
		for depth := 1; depth < len(icons); depth++ {
			if step := icons[depth] - icons[depth-1]; step < 10 {
				t.Errorf("%s: depth %d stands %v right of depth %d, want one step", name, depth, step, depth-1)
			}
		}
		if want := map[string]int{"project": 4, "role": 4, "all roles": 4, "catalog": 1}[name]; len(icons) < want || name != "catalog" && leaves == 0 {
			t.Errorf("%s: %d rows, %d leaves, %d depths: too little to tell", name, len(rows), leaves, len(icons))
		}
	}
	// The icons, written out from the prototype's trees: the configuration,
	// a kind's branch and its object, the folder of attributes and an
	// attribute, a table part.
	want := map[string]string{"SalesDemo": "#icon-configuration", "Справочники": "#icon-catalog", "Контрагенты": "#icon-catalog",
		"Реквизиты": "#icon-attribute", "Табличные части": "#icon-tabular-section", "Документы": "#icon-document"}
	for name, rows := range map[string][]treeRow{"role": got.Role, "all roles": got.Roles} {
		for title, href := range want {
			found := false
			for _, row := range rows {
				if row.Title == title || title == "SalesDemo" && row.Depth == 0 && strings.HasSuffix(row.Href, "configuration") {
					found = true
					if row.Href != href {
						t.Errorf("%s: %q is drawn with %s, the project tree draws it with %s", name, row.Title, row.Href, href)
					}
				}
			}
			if !found {
				t.Errorf("%s: no row %q", name, title)
			}
		}
	}
	for _, row := range got.Catalog {
		if href := want[row.Title]; href != "" && row.Href != href {
			t.Errorf("catalog: %q is drawn with %s, want %s", row.Title, row.Href, href)
		}
	}
}

// No editor of Studio makes a folding row of its own: every tree is made of
// treeNode (studio-common.js), so the next tree stands as the project tree.
// Defects caught: a tree of a window built of its own details and summary,
// which the walk above would not open when the window is not opened by it.
func TestNoEditorMakesATreeItself(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("ui", "*"))
	if err != nil {
		t.Fatal(err)
	}
	made := regexp.MustCompile(`(?:node|createElement)\(\s*'(?:details|summary)'|<(?:details|summary)\b`)
	for _, file := range files {
		if filepath.Base(file) == "studio-common.js" || !strings.HasSuffix(file, ".js") && !strings.HasSuffix(file, ".html") {
			continue
		}
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if found := made.FindString(string(content)); found != "" {
			t.Errorf("%s makes a folding row itself (%s), not by treeNode", file, found)
		}
	}
}
