package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// sectionListSettingsWalk opens the window «Командный интерфейс» of the
// subsystem Склад, «Настройка списка...» of the menu of a row, and reads
// the columns of the window and of each the place, the size, the width, the
// height, the autoheight and the arrows; «Прочее»; marks none, then all but
// «Видимость» and Вторая, presses OK and reads the list; closes the window
// by «Отмена», opens it again and reads the list and what is kept.
const sectionListSettingsWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 120)).then(() => studio.settle());
  const host = document.querySelector('#subsystem-editor');
  const commands = () => document.querySelector('dialog.section-commands-dialog');
  const settings = () => document.querySelector('dialog.list-settings-dialog');
  const open = async () => {
    [...host.querySelectorAll('.object-window-button')].find(item => item.textContent === 'Командный интерфейс').click();
    await settle();
  };
  const list = () => ({head: [...commands().querySelectorAll('.section-commands-head > span, .section-roles-head > span:not(.section-roles-strip)')].map(item => item.textContent),
    roles: [...commands().querySelectorAll('.section-roles-head .section-role-cell')].map(cell => cell.textContent),
    boxes: commands().querySelectorAll('.section-commands input[type=checkbox]').length});
  await openObject('Склад');
  await open();
  steps.before = list();
  const row = [...commands().querySelectorAll('.studio-tree-row')].find(item => item.textContent.startsWith('Панель'));
  row.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  const items = [...document.querySelectorAll('.context-menu > *')];
  steps.menuEnd = items.slice(-2).map(item => item.matches('.context-menu-separator') ? '|' : item.textContent);
  items[items.length - 1].click(); await settle();
  const lines = () => [...settings().querySelectorAll('.list-settings-item')];
  steps.columns = lines().map(item => (item.classList.contains('list-settings-fixed') ? '✓ ' : '') + item.textContent.replace('✓', ''));
  const field = label => settings().querySelector('[aria-label="' + label + '"]');
  steps.each = [];
  for (const at of [0, 1, 2, 3, 4, 5]) {
    lines()[at].click(); await settle();
    steps.each.push([field('Положение').value + (field('Положение').disabled ? ' -' : ''), field('Изменение размера').value, field('Ширина').value,
      field('Высота').value, settings().querySelector('.list-settings-check input').checked,
      settings().querySelector('button[title="Переместить вверх"]').disabled]);
  }
  steps.opening = field('При открытии переходить').value;
  // «Снять отметку со всех» leaves «Команда»; then the columns but
  // «Видимость» and Вторая are marked one by one.
  settings().querySelector('button[title="Снять отметку со всех элементов"]').click(); await settle();
  steps.none = lines().filter(item => item.classList.contains('list-settings-fixed') || item.querySelector('input').checked).map(item => item.textContent.replace('✓', ''));
  for (const at of [3, 4, 6]) lines()[at].querySelector('input').click();
  [...settings().querySelectorAll('.localized-title-actions button')].find(item => item.textContent === 'ОК').click(); await settle();
  steps.applied = list();
  [...commands().querySelectorAll('.localized-title-actions button')].find(item => item.textContent === 'Отмена').click(); await settle();
  await open();
  steps.again = list();
  steps.kept = (await (await fetch('/api/settings/lists')).json()).lists['subsystem-commands'].columns.filter(column => !column.visible).map(column => column.key);
  return steps;
})()`

// «Настройка списка...» of the list of commands is the prototype's
// (obshchee.md, «Список команд»; snapshots predsyomka-1/05-17): the last
// item of the menu of a row, after a line; «Команда» twice, marked and
// never hidden, its place and the arrows unavailable - a new column of 40
// and the next row of 27 - «Видимость» and «Видимость по ролям» new
// columns of 8, these of autoheight, then the roles by their synonyms in
// the order of the project, the first on the next row, the rest in the same
// column, 87 wide; «При открытии переходить» at the beginning. A column
// hidden leaves the list, and the setting is kept for the window opened
// again. Defects caught: the item missing; a column, a number or a choice
// not the prototype's; «Команда» hidden by «Снять все» or movable; a
// hidden column still drawn; the setting lost on reopening.
func TestTheSettingOfTheListOfCommandsIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	path0 := filepath.Join(root, "metadata", "catalogs", "Товары", "object.yaml")
	content, err := os.ReadFile(path0)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path0, []byte(string(content)+"use_standard_commands: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	products, _ := uuid.Parse("10000000-0000-4000-8000-000000000101")
	ids := map[string]string{}
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		workspace.KeepUserSettingsIn(t.TempDir())
		for _, made := range [][2]string{{"Р1", "Первая"}, {"Р2", "Вторая"}, {"Р3", "Третья"}} {
			role, err := workspace.CreateRole(made[0])
			if err != nil {
				t.Fatal(err)
			}
			role.Role.Title = metadata.LocalizedText{"ru": made[1]}
			if _, err := workspace.SaveRole(role.Path, role.Role, role.Revision); err != nil {
				t.Fatal(err)
			}
			ids[made[1]] = role.Role.ID.String()
		}
		created, err := workspace.CreateSubsystem("Склад")
		if err != nil {
			t.Fatal(err)
		}
		value := created.Subsystem
		value.Members = []uuid.UUID{products}
		if _, err := workspace.SaveSubsystemWindow(created.Path, SubsystemWindowValue{SubsystemDefinition: value}, created.Revision); err != nil {
			t.Fatal(err)
		}
	})
	result, err := browser.evaluate(ctx, page, sectionListSettingsWalk)
	if err != nil {
		t.Fatal(err)
	}
	type shown struct {
		Head, Roles []string
		Boxes       int
	}
	var steps struct {
		MenuEnd, Columns, Kept, None []string
		Before                       shown
		Each                         [][]any
		Opening                      string
		Applied, Again               shown
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	expect := func(what string, got, want any) {
		t.Helper()
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("%s: %s, want %s", what, gotJSON, wantJSON)
		}
	}
	expect("the end of the menu", steps.MenuEnd, []string{"|", "Настройка списка..."})
	expect("the columns", steps.Columns, []string{"✓ Команда", "✓ Команда", "Видимость", "Видимость по ролям", "Первая", "Вторая", "Третья"})
	expect("each column", steps.Each, [][]any{
		{"new -", "change", "40", "1", true, true}, {"next-row -", "change", "27", "1", true, true},
		{"new", "change", "8", "1", true, false}, {"new", "change", "8", "1", true, false},
		{"next-row", "change", "87", "1", false, false}, {"same-column", "change", "87", "1", false, false}})
	expect("«Прочее»", steps.Opening, "beginning")
	expect("none marked", steps.None, []string{"Команда", "Команда"})
	expect("the list at first", steps.Before, shown{Head: []string{"Команда", "Видимость", "Видимость по ролям"}, Roles: []string{"Первая", "Вторая", "Третья"}, Boxes: 2})
	want := shown{Head: []string{"Команда", "Видимость по ролям"}, Roles: []string{"Первая", "Третья"}, Boxes: 0}
	expect("the list after OK", steps.Applied, want)
	expect("the list opened again", steps.Again, want)
	expect("kept hidden", steps.Kept, []string{"visibility", "role:" + ids["Вторая"]})
}
