package studio

import (
	"encoding/json"
	"slices"
	"testing"
)

// roleMarkAllWalk opens the window of the role Кладовщик on the catalog
// Товары and reads the row over the list of rights; presses «Отметить все
// элементы», reads the marks of Товары, then those of Контрагенты; presses
// «Снять отметки со всех элементов» on Товары; and on the branch
// «Справочники» reads the buttons, presses «Отметить все элементы» and reads
// the marks of the branch, then those of Контрагенты.
const roleMarkAllWalk = `(async () => {
  const studio = window.__studio, editor = document.querySelector('#role-editor'), answer = {};
  const marks = () => [...editor.querySelectorAll('.role-permissions .check-list-row input')].map(input => input.checked);
  const object = name => [...editor.querySelectorAll('.role-tree-item[data-object]')].find(item => item.title === name);
  const head = () => editor.querySelector('.role-permissions .role-rights-head');
  const press = label => head().querySelector('button[aria-label="' + label + '"]').click();
  [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === 'Кладовщик').click();
  await studio.settle();
  object('Товары').click(); await studio.settle();
  answer.head = [...head().children].map(item => item.matches('button') ? item.getAttribute('aria-label') : item.textContent);
  answer.above = !!(head().compareDocumentPosition(editor.querySelector('.role-permissions .check-list')) & Node.DOCUMENT_POSITION_FOLLOWING);
  answer.oldButtons = [...editor.querySelectorAll('.role-permissions button')].filter(button => /Разрешить всё|Снять все права/.test(button.textContent)).length;
  press('Отметить все элементы'); await studio.settle(); answer.marked = marks();
  object('Контрагенты').click(); await studio.settle(); answer.other = marks();
  object('Товары').click(); await studio.settle();
  press('Снять отметки со всех элементов'); await studio.settle(); answer.cleared = marks();
  [...editor.querySelectorAll('.role-tree-group')].find(item => item.textContent.trim() === 'Справочники').click(); await studio.settle();
  answer.branch = [...head().querySelectorAll('button')].map(button => button.disabled);
  press('Отметить все элементы'); await studio.settle(); answer.branchMarked = marks();
  object('Контрагенты').click(); await studio.settle(); answer.otherAfterBranch = marks();
  return answer;
})()`

// Over the list of rights of the window of a role stand «Права:» and the
// platform's two buttons, as the prototype's (snapshot block-00/rol/03, pass
// of 20.10.2026, block-00/rol «Над списком прав две кнопки»; the second
// button's tooltip is the pair's in «Настройка списка», shabl-26): they mark
// and clear every right of the node chosen and nothing beside it; on the
// branch of a kind, as its boxes do (prava/20-23), every right of every
// object of the kind. Defects caught: the old «Разрешить всё» under the
// list; the buttons marking the rights of other objects; the buttons
// unavailable on a branch, or marking there only the object chosen last.
func TestTheButtonsOverTheListOfRightsMarkTheNodeChosen(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, roleMarkAllWalk)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Head                   []string
		Above                  bool
		OldButtons             int
		Marked, Other, Cleared []bool
		Branch                 []bool
		BranchMarked           []bool
		OtherAfterBranch       []bool
	}
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	if want := []string{"Права:", "Отметить все элементы", "Снять отметки со всех элементов"}; !slices.Equal(got.Head, want) || !got.Above || got.OldButtons != 0 {
		t.Errorf("over the list %v (above it %v), %d old buttons; want %v over it and none", got.Head, got.Above, got.OldButtons, want)
	}
	if len(got.Marked) == 0 || slices.Contains(got.Marked, false) {
		t.Errorf("Товары after «Отметить все элементы»: %v, want every right marked", got.Marked)
	}
	if slices.Contains(got.Other, true) {
		t.Errorf("Контрагенты after the button on Товары: %v, want nothing marked", got.Other)
	}
	if len(got.Cleared) == 0 || slices.Contains(got.Cleared, true) {
		t.Errorf("Товары after «Снять отметки со всех элементов»: %v, want nothing marked", got.Cleared)
	}
	if !slices.Equal(got.Branch, []bool{false, false}) {
		t.Errorf("on the branch «Справочники» the buttons are unavailable %v, want neither", got.Branch)
	}
	if len(got.BranchMarked) == 0 || slices.Contains(got.BranchMarked, false) {
		t.Errorf("the branch after «Отметить все элементы»: %v, want every right marked", got.BranchMarked)
	}
	if len(got.OtherAfterBranch) == 0 || slices.Contains(got.OtherAfterBranch, false) {
		t.Errorf("Контрагенты after the button on the branch: %v, want every right marked", got.OtherAfterBranch)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
