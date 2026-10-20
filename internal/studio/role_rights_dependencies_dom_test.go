package studio

import (
	"encoding/json"
	"slices"
	"testing"
)

// roleRightsDependenciesWalk opens the window of the role Кладовщик on the
// catalog Контрагенты, which it gives nothing, presses the box of
// «Интерактивное удаление» and reads the rights marked; then presses
// «Ввод по строке» and the marked «Чтение» and reads them again; then marks «Редактирование» of the attribute Количество of the
// table part of ПродажаТоваров and reads its rights and the document's.
const roleRightsDependenciesWalk = `(async () => {
  const studio = window.__studio, editor = document.querySelector('#role-editor'), answer = {};
  const marked = () => [...editor.querySelectorAll('.role-permissions .check-list-row')].filter(row => row.querySelector('input').checked).map(row => row.textContent.trim());
  const box = text => [...editor.querySelectorAll('.role-permissions .check-list-row')].find(row => row.textContent.trim() === text).querySelector('input');
  [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === 'Кладовщик').click();
  await studio.settle();
  editor.querySelector('.role-tree-item[title="Контрагенты"]').click(); await studio.settle();
  box('Интерактивное удаление').click(); await studio.settle(); answer.deletion = marked();
  box('Ввод по строке').click(); await studio.settle();
  box('Чтение').click(); await studio.settle(); answer.reading = marked();
  editor.querySelector('[data-group="10000000-0000-4000-8000-000000000202 table-parts"]').closest('details').querySelector('.role-tree-item[title="Количество"]').click();
  await studio.settle();
  box('Редактирование').click(); await studio.settle(); answer.part = marked();
  editor.querySelector('.role-tree-item[title="ПродажаТоваров"]').click(); await studio.settle(); answer.document = marked();
  return answer;
})()`

// The window of a role ties the rights as the prototype's (configurator
// 8.3.27, passes of 20.10.2026, block-00/rol «Права зависят друг от друга»,
// snapshots prava/20-25 and 33-41), through the map the server gives:
// marking «Интерактивное удаление» of a catalog marks «Чтение»,
// «Изменение», «Удаление», «Просмотр» and «Редактирование»; clearing
// «Чтение» clears every right of a catalog ML keeps (the pass kept «Изменение
// настроек истории данных», a right ML does not keep); the
// editing of an attribute marks its viewing and nothing of its object.
// Written out from the snapshots. Defects caught: the window marking a right
// alone; the server's map not reaching the window, or read under another
// kind; a part taking the rights of its object.
func TestTheWindowOfARoleTiesTheRightsAsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, roleRightsDependenciesWalk)
	if err != nil {
		t.Fatal(err)
	}
	var got struct{ Deletion, Reading, Part, Document []string }
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	if want := []string{"Чтение", "Изменение", "Удаление", "Просмотр", "Редактирование", "Интерактивное удаление"}; !slices.Equal(got.Deletion, want) {
		t.Errorf("after «Интерактивное удаление»: %v, want %v", got.Deletion, want)
	}
	if len(got.Reading) != 0 {
		t.Errorf("after «Ввод по строке» and «Чтение» cleared: %v, want nothing", got.Reading)
	}
	if want := []string{"Просмотр", "Редактирование"}; !slices.Equal(got.Part, want) {
		t.Errorf("the attribute after «Редактирование»: %v, want %v", got.Part, want)
	}
	if len(got.Document) != 0 {
		t.Errorf("ПродажаТоваров after the editing of its attribute: %v, want nothing", got.Document)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
