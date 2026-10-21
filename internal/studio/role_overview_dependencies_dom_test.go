package studio

import (
	"encoding/json"
	"slices"
	"testing"
)

// roleOverviewDependenciesWalk opens «Все роли» from the menu of the branch
// «Роли», chooses the catalog Контрагенты, which the role Кладовщик gives
// nothing, presses the cell of Кладовщик and «Интерактивное удаление» and
// reads the rights marked in its column; then presses its «Ввод по строке»
// and its marked «Чтение» and reads them again; then presses «Ввод по
// строке» in the column of all roles and reads the column of Кладовщик.
const roleOverviewDependenciesWalk = `(async () => {
  const studio = window.__studio, answer = {};
  const roles = [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === 'Роли');
  roles.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent === 'Все роли').click();
  await studio.settle();
  const host = document.querySelector('#role-overview');
  host.querySelector('.role-tree-item[title="Контрагенты"]').click(); await studio.settle();
  const cell = right => host.querySelector('input[aria-label="Кладовщик — ' + right + '"]');
  const column = () => [...host.querySelectorAll('.overview-grid tbody tr')].slice(1)
    .filter(row => row.querySelector('input[aria-label^="Кладовщик — "]').checked)
    .map(row => row.querySelector('th span').textContent);
  cell('Интерактивное удаление').click(); await studio.settle(); answer.deletion = column();
  cell('Ввод по строке').click(); await studio.settle();
  cell('Чтение').click(); await studio.settle(); answer.reading = column();
  host.querySelector('input[aria-label="Ввод по строке — во всех ролях"]').click(); await studio.settle(); answer.all = column();
  return answer;
})()`

// «Все роли» ties the rights as the window of a role does and as the
// prototype's (configurator 8.3.27, pass of 21.10.2026, prava/49-51): a
// right marked in a cell marks in the same column the rights it needs, a
// right cleared clears those needing it; the column of all roles does so in
// every role. Written out from the snapshots. Defects caught: «Все роли»
// marking a right alone, or reading the map of rights apart from the window
// of a role; the column of all roles setting the right without what it
// needs.
func TestTheWindowOfAllRolesTiesTheRightsAsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, roleOverviewDependenciesWalk)
	if err != nil {
		t.Fatal(err)
	}
	var got struct{ Deletion, Reading, All []string }
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	if want := []string{"Чтение", "Изменение", "Удаление", "Просмотр", "Редактирование", "Интерактивное удаление"}; !slices.Equal(got.Deletion, want) {
		t.Errorf("after «Интерактивное удаление» in the cell: %v, want %v", got.Deletion, want)
	}
	if len(got.Reading) != 0 {
		t.Errorf("after «Ввод по строке» and «Чтение» cleared: %v, want nothing", got.Reading)
	}
	if want := []string{"Чтение", "Просмотр", "Ввод по строке"}; !slices.Equal(got.All, want) {
		t.Errorf("after «Ввод по строке» in all roles: %v, want %v", got.All, want)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
