package studio

import (
	"encoding/json"
	"slices"
	"testing"
)

// roleOverviewCaptionsWalk opens «Все роли» from the menu of the branch
// «Роли», chooses the catalog Товары and reads the captions over the tree,
// over the table of rights and over the restrictions; then chooses the role
// Кладовщик by its column and reads the restrictions' caption and columns.
const roleOverviewCaptionsWalk = `(async () => {
  const studio = window.__studio, answer = {};
  const roles = [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === 'Роли');
  roles.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent === 'Все роли').click();
  await studio.settle();
  const host = document.querySelector('#role-overview');
  host.querySelector('.role-tree-item[title="Товары"]').click(); await studio.settle();
  answer.tree = host.querySelector('.overview-sidebar').firstElementChild.textContent;
  answer.rights = host.querySelector('.overview-panel .overview-caption')?.textContent || '';
  answer.rightsBeforeGrid = !!(host.querySelector('.overview-panel .overview-caption')?.compareDocumentPosition(host.querySelector('.overview-grid')) & Node.DOCUMENT_POSITION_FOLLOWING);
  answer.unchosen = host.querySelector('.role-restrictions h4')?.textContent || '';
  [...host.querySelectorAll('.overview-grid thead button.overview-role')].find(button => button.textContent === 'Кладовщик').click(); await studio.settle();
  answer.chosen = host.querySelector('.role-restrictions h4')?.textContent || '';
  answer.columns = [...host.querySelectorAll('.role-restrictions table thead th')].map(cell => cell.textContent);
  return answer;
})()`

// «Все роли» captions its parts as the prototype's (configurator 8.3.27,
// pass after the series of 2.413, snapshot prava/19; block-00/rol): «Объекты:»
// over the tree, «Права:» over the table of rights, «Ограничения доступа к
// данным:» over the restrictions - while no role is chosen too - with the
// columns «Поля» and «Ограничение доступа», the caption and the columns
// being those of the window of a role. Written out here from the snapshot.
// Defects caught: the old heading «Ограничение доступа к данным» left in
// either state of the window; a caption missing over the tree or the table;
// the column «Поле».
func TestTheWindowOfAllRolesIsCaptionedAsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, roleOverviewCaptionsWalk)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Tree, Rights, Unchosen, Chosen string
		RightsBeforeGrid               bool
		Columns                        []string
	}
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	if got.Tree != "Объекты:" {
		t.Errorf("over the tree %q, want «Объекты:»", got.Tree)
	}
	if got.Rights != "Права:" || !got.RightsBeforeGrid {
		t.Errorf("over the table of rights %q (before it %v), want «Права:» over it", got.Rights, got.RightsBeforeGrid)
	}
	for state, heading := range map[string]string{"no role chosen": got.Unchosen, "Кладовщик chosen": got.Chosen} {
		if heading != "Ограничения доступа к данным:" {
			t.Errorf("%s: the restrictions headed %q, want «Ограничения доступа к данным:»", state, heading)
		}
	}
	if want := []string{"Поля", "Ограничение доступа"}; !slices.Equal(got.Columns, want) {
		t.Errorf("the columns of the restrictions %v, want %v", got.Columns, want)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
