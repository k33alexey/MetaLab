package studio

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
)

// roleBranchFilterWalk opens the window of the role Кладовщик, reads the
// rights of the branch «Справочники», filters the tree by the subsystem Склад,
// which holds Товары alone, reads the branch again, presses its box of «Ввод
// по строке» and saves the role.
const roleBranchFilterWalk = `(async () => {
  const studio = window.__studio, editor = document.querySelector('#role-editor'), answer = {};
  const rights = () => [...editor.querySelectorAll('.role-permissions .check-list-row')].map(row => {
    const input = row.querySelector('input');
    return {caption: row.textContent.trim(), checked: input.checked, mixed: input.indeterminate};
  });
  const branch = () => [...editor.querySelectorAll('.role-tree-group')].find(item => item.textContent.trim() === 'Справочники');
  [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === 'Кладовщик').click();
  await studio.settle();
  branch().click(); await studio.settle(); answer.whole = rights();
  editor.querySelector('.role-tree-bar .role-subsystems').click(); await studio.settle();
  const dialog = document.querySelector('dialog[open]');
  [...dialog.querySelectorAll('.role-subsystem-tree label')].find(item => item.textContent.trim() === 'Склад').querySelector('input').click();
  await studio.settle();
  [...dialog.querySelectorAll('button')].find(button => button.textContent === 'Установить').click(); await studio.settle();
  answer.filtered = rights();
  [...editor.querySelectorAll('.role-permissions .check-list-row')].find(row => row.textContent.trim() === 'Ввод по строке').querySelector('input').click();
  await studio.settle();
  answer.pressed = rights();
  document.querySelector('#save').click(); await studio.settle();
  return answer;
})()`

// Under a filter by subsystems the branch of a kind gives the rights of the
// objects shown alone, as the prototype's (configurator 8.3.27, pass of
// 21.10.2026, prava/45-48): its summary is theirs and a click on it sets the
// right at them and at no object the filter hides. The expected marks are
// written from the role the test makes: Товары has «Чтение» and «Изменение»,
// Контрагенты nothing. Defects caught: the branch summing the hidden objects
// under the filter (grey where the shown one has the right); a click setting
// the right at the hidden ones too; the panel not redrawn when the filter is
// set.
func TestUnderAFilterTheBranchGivesTheRightsOfWhatItShows(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addTreeSubsystems(t, root)
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, roleBranchFilterWalk)
	if err != nil {
		t.Fatal(err)
	}
	type right struct {
		Caption        string
		Checked, Mixed bool
	}
	var got struct{ Whole, Filtered, Pressed []right }
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	state := func(rights []right, caption string) right {
		for _, item := range rights {
			if item.Caption == caption {
				return item
			}
		}
		t.Fatalf("no right %s among %v", caption, rights)
		return right{}
	}
	if read := state(got.Whole, "Чтение"); read.Checked || !read.Mixed {
		t.Errorf("without the filter «Чтение» of the branch is %+v, want grey", read)
	}
	for _, caption := range []string{"Чтение", "Изменение"} {
		if item := state(got.Filtered, caption); !item.Checked || item.Mixed {
			t.Errorf("under the filter %s of the branch is %+v, want marked: Товары alone is shown and has it", caption, item)
		}
	}
	if item := state(got.Pressed, "Ввод по строке"); !item.Checked || item.Mixed {
		t.Errorf("after a click under the filter «Ввод по строке» is %+v, want marked", item)
	}
	configuration, err := metadata.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	role, ok := configuration.Role("Кладовщик")
	if !ok {
		t.Fatal("the role is not saved")
	}
	given := map[string][]metadata.PermissionOperation{}
	for _, item := range role.Objects {
		given[item.Object.String()] = item.Operations
	}
	if !slices.Contains(given["10000000-0000-4000-8000-000000000101"], metadata.PermissionInputByString) {
		t.Errorf("Товары, shown, was not given «Ввод по строке»: %v", given)
	}
	if slices.Contains(given["10000000-0000-4000-8000-000000000102"], metadata.PermissionInputByString) {
		t.Errorf("Контрагенты, hidden by the filter, was given «Ввод по строке»: %v", given)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
