package studio

import (
	"encoding/json"
	"slices"
	"testing"
)

// roleGroupRightsWalk opens the window of the role Кладовщик, marks «Просмотр»
// of the standard attribute Код of the catalog Товары, then chooses its folder
// «Стандартные реквизиты» and presses the box of «Просмотр» twice; then the
// branch «Справочники», where it presses the space, and «Общие»; then marks
// «Просмотр» on the folder «Реквизиты» of the document ПродажаТоваров and
// chooses the attribute Количество of its table part. After each
// step it reads the rights listed: caption, marked, grey, unavailable.
const roleGroupRightsWalk = `(async () => {
  const studio = window.__studio, steps = [], editor = document.querySelector('#role-editor');
  const goods = '10000000-0000-4000-8000-000000000101';
  const read = () => steps.push({
    heading: editor.querySelector('.role-permissions h3')?.textContent || '',
    rights: [...editor.querySelectorAll('.role-permissions .check-list-row')].map(row => {
      const input = row.querySelector('input');
      return {caption: row.textContent.trim(), checked: input.checked, mixed: input.indeterminate, disabled: input.disabled};
    }),
    description: editor.querySelector('.role-description').textContent,
    chosen: editor.querySelector('.role-tree-group.selected')?.dataset.group || '',
  });
  const box = text => [...editor.querySelectorAll('.role-permissions .check-list-row')].find(row => row.textContent.trim() === text).querySelector('input');
  const group = text => [...editor.querySelectorAll('.role-tree-group')].find(item => item.textContent.trim() === text);
  [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === 'Кладовщик').click();
  await studio.settle();
  const folder = editor.querySelector('[data-group="' + goods + ' standard-attributes"]');
  folder.closest('details').querySelector('.role-tree-item[title="Код"]').click();
  await studio.settle();
  box('Просмотр').click(); await studio.settle();
  editor.querySelector('[data-group="' + goods + ' standard-attributes"]').click(); await studio.settle(); read();
  box('Просмотр').click(); await studio.settle(); read();
  box('Просмотр').click(); await studio.settle(); read();
  group('Справочники').click(); await studio.settle(); read();
  editor.querySelector('.role-permissions .check-list').dispatchEvent(new KeyboardEvent('keydown', {key: ' ', bubbles: true})); await studio.settle(); read();
  group('Общие').click(); await studio.settle(); read();
  const sale = editor.querySelector('[data-group="10000000-0000-4000-8000-000000000202 attributes"]');
  sale.click(); await studio.settle();
  box('Просмотр').click(); await studio.settle(); read();
  editor.querySelector('[data-group="10000000-0000-4000-8000-000000000202 table-parts"]').closest('details').querySelector('.role-tree-item[title="Количество"]').click();
  await studio.settle(); read();
  return steps;
})()`

// A folder of parts and the branch of a kind show the rights of what they
// hold as the prototype's (configurator 8.3.27, pass of 20.10.2026,
// block-00/rol «Права папки и ветви вида», snapshots prava/06-11): a right
// marked at every one is marked, at none cleared, at some grey; the box of a
// folder sets the right at every part and clears it, the grey one setting
// it. The branch of a kind shows its objects' rights so, its boxes
// unavailable while what a click does there is unknown; «Общие» has no
// rights and no description. The expected marks are written from the role
// the test makes, not read from the code. Defects caught: a folder or a
// branch only naming its group; the summary not grey where the parts differ;
// the grey box clearing rather than setting; a click on the folder setting
// one part; a branch rewriting the rights of every object of its kind; the
// folder «Реквизиты» taking in the attributes of the table parts.
func TestFoldersAndBranchesOfTheTreeOfRightsShowTheRightsOfWhatTheyHold(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, roleGroupRightsWalk)
	if err != nil {
		t.Fatal(err)
	}
	type right struct {
		Caption                  string
		Checked, Mixed, Disabled bool
	}
	type step struct {
		Heading, Description, Chosen string
		Rights                       []right
	}
	var steps []step
	if err := json.Unmarshal(result, &steps); err != nil || len(steps) != 8 {
		t.Fatalf("%v: %s", err, result)
	}
	folder, set, cleared, catalogs, space, common, attributes, quantity := steps[0], steps[1], steps[2], steps[3], steps[4], steps[5], steps[6], steps[7]
	want := func(name string, got step, rights ...right) {
		t.Helper()
		if !slices.Equal(got.Rights, rights) {
			t.Errorf("%s: the rights %+v, want %+v", name, got.Rights, rights)
		}
	}
	if folder.Heading != "Стандартные реквизиты" || folder.Chosen != "10000000-0000-4000-8000-000000000101 standard-attributes" {
		t.Errorf("the folder chosen: headed %q, chosen in the tree %q", folder.Heading, folder.Chosen)
	}
	want("the folder with «Просмотр» of Код alone", folder, right{"Просмотр", false, true, false}, right{"Редактирование", false, false, false})
	want("after a click on the grey box", set, right{"Просмотр", true, false, false}, right{"Редактирование", false, false, false})
	want("after a click on the marked box", cleared, right{"Просмотр", false, false, false}, right{"Редактирование", false, false, false})
	if folder.Description != "Описание:Право видеть в интерфейсе\n(Просмотр)" {
		t.Errorf("the folder: described %q, want «Просмотр»", folder.Description)
	}
	if catalogs.Heading != "Справочники" || len(catalogs.Rights) == 0 {
		t.Fatalf("the branch «Справочники»: headed %q with rights %v", catalogs.Heading, catalogs.Rights)
	}
	// Товары has «Чтение» and «Изменение», Контрагенты has neither.
	for _, got := range catalogs.Rights {
		grey := got.Caption == "Чтение" || got.Caption == "Изменение"
		if got.Checked || got.Mixed != grey || !got.Disabled {
			t.Errorf("the branch «Справочники»: %+v, want grey %v and unavailable", got, grey)
		}
	}
	if !slices.Equal(space.Rights, catalogs.Rights) {
		t.Errorf("the space on the branch changed its rights: %+v, were %+v", space.Rights, catalogs.Rights)
	}
	if common.Heading != "Общие" || len(common.Rights) != 0 || common.Description != "Описание:" {
		t.Errorf("«Общие»: headed %q with rights %v, described %q; want none and no description", common.Heading, common.Rights, common.Description)
	}
	want("the folder «Реквизиты» of ПродажаТоваров after a click", attributes, right{"Просмотр", true, false, false}, right{"Редактирование", false, false, false})
	want("the attribute Количество of its table part", quantity, right{"Просмотр", false, false, false}, right{"Редактирование", false, false, false})
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
