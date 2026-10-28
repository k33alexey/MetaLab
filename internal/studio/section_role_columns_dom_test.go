package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// sectionRoleColumnsWalk opens the window «Командный интерфейс» of the
// subsystem Склад and reads the columns of the roles and the boxes of the
// list of Товары and of its creation; scrolls the columns to their end;
// clicks the box of Четвёртая at the list once and at the creation twice,
// and the box of Вторая at the list three times; filters by Первая; presses
// OK and «Сохранить»; opens the window again, reads the boxes, sets the
// creation to its defaults, presses OK and «Сохранить» again.
const sectionRoleColumnsWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 120)).then(() => studio.settle());
  const host = document.querySelector('#subsystem-editor');
  const dialog = () => document.querySelector('dialog.section-commands-dialog');
  const heads = () => [...dialog().querySelectorAll('.section-roles-head .section-role-cell')].map(cell => cell.textContent);
  const boxes = command => [...dialog().querySelectorAll('.role-visibility')].filter(box => box.getAttribute('aria-label').endsWith(': ' + command))
    .map(box => box.getAttribute('aria-label').split(': ')[1] + ' ' + box.getAttribute('aria-checked'));
  const box = (role, command) => dialog().querySelector('.role-visibility[aria-label="Видимость: ' + role + ': ' + command + '"]');
  const click = async (role, command) => { if (!box(role, command)) throw new Error('no box ' + role + ' ' + command + ' in ' + JSON.stringify([heads(), boxes(command), scroller().scrollLeft, scroller().clientWidth, getComputedStyle(scroller()).width])); box(role, command).click(); await settle(); };
  const scroller = () => dialog().querySelector('.section-roles-scroll');
  const open = async () => {
    [...host.querySelectorAll('.object-window-button')].find(item => item.textContent === 'Командный интерфейс').click();
    await settle();
  };
  const save = async () => {
    [...dialog().querySelectorAll('.localized-title-actions button')].find(item => item.textContent === 'OK').click(); await settle();
    document.querySelector('#save').click(); await settle();
    const text = await (await studioRequest('/api/subsystem?path=' + encodeURIComponent('SUBSYSTEM_PATH'), {method: 'GET'})).text();
    return text.startsWith('{') ? JSON.parse(text).subsystem.commandInterface || {} : {problem: text};
  };
  await openObject('Склад');
  await open();
  if (!dialog()) return {problem: JSON.stringify(studio.problems)};
  steps.first = [heads(), !scroller().hidden, boxes('Список товаров'), boxes('Товары: создать')];
  scroller().scrollLeft = 10000; scroller().dispatchEvent(new Event('scroll')); await settle();
  steps.scrolled = [heads(), boxes('Список товаров')];
  await click('Четвёртая', 'Список товаров');
  await click('Четвёртая', 'Товары: создать');
  await click('Четвёртая', 'Товары: создать');
  steps.clicked = [boxes('Список товаров'), boxes('Товары: создать')];
  scroller().scrollLeft = 0; scroller().dispatchEvent(new Event('scroll')); await settle();
  for (let time = 0; time < 3; time++) await click('Вторая', 'Список товаров');
  steps.round = boxes('Список товаров');
  dialog().querySelector('button[title="Выбор объекта: Роль"]').click(); await settle();
  const choice = document.querySelector('dialog.object-choice-dialog');
  choice.querySelector('input.object-choice-box[aria-label="Р1"]').click();
  [...choice.querySelectorAll('.localized-title-actions button')].find(item => item.textContent === 'ОК').click(); await settle();
  steps.filtered = [heads(), scroller().hidden];
  steps.written = await save();
  await open();
  scroller().scrollLeft = 10000; scroller().dispatchEvent(new Event('scroll')); await settle();
  steps.again = [boxes('Список товаров'), boxes('Товары: создать')];
  [...dialog().querySelectorAll('.studio-tree-row')].find(row => row.textContent === 'Товары: создать').click(); await settle();
  dialog().querySelector('.object-window-panel button[title="Установить свойства по умолчанию"]').click(); await settle();
  steps.defaults = boxes('Товары: создать');
  steps.writtenAgain = await save();
  return steps;
})()`

// The columns of the roles of the window «Командный интерфейс» of a
// subsystem are the prototype's (obshchee.md, «Список команд», «Видимость
// по ролям»; snapshots vopr-2.636/01, 17, predsyomka-1/02, 03): a column a
// role, by its synonym, in the order of the project, three in sight and the
// rest by the scroll; a box grey and marked while the role says nothing,
// even where the common box is off; a click goes off, on and back to grey;
// the filter leaves the columns of its roles; OK writes what the roles say,
// the window opened again shows it, and the defaults take it away. Defects
// caught: a role's column missing or out of order, the columns not
// scrolled, a box of the default shown off where the common box is off, the
// round of a click wrong, the columns of other roles left under the filter,
// what the roles say not written, not shown again or left by the defaults.
func TestTheColumnsOfTheRolesOfASectionAreThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	path0 := filepath.Join(root, "metadata", "catalogs", "Товары", "object.yaml")
	content, err := os.ReadFile(path0)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path0, []byte(string(content)+"use_standard_commands: true\nlist_presentation: {ru: Список товаров}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	products, _ := uuid.Parse("10000000-0000-4000-8000-000000000101")
	ids := map[string]uuid.UUID{}
	var path string
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		for _, made := range [][2]string{{"Р1", "Первая"}, {"Р2", "Вторая"}, {"Р3", "Третья"}, {"Р4", "Четвёртая"}} {
			role, err := workspace.CreateRole(made[0])
			if err != nil {
				t.Fatal(err)
			}
			role.Role.Title = metadata.LocalizedText{"ru": made[1]}
			if _, err := workspace.SaveRole(role.Path, role.Role, role.Revision); err != nil {
				t.Fatal(err)
			}
			ids[made[1]] = role.Role.ID
		}
		created, err := workspace.CreateSubsystem("Склад")
		if err != nil {
			t.Fatal(err)
		}
		path = created.Path
		value := created.Subsystem
		value.Members = []uuid.UUID{products}
		if _, err := workspace.SaveSubsystemWindow(created.Path, SubsystemWindowValue{SubsystemDefinition: value}, created.Revision); err != nil {
			t.Fatal(err)
		}
	})
	result, err := browser.evaluate(ctx, page, strings.Replace(sectionRoleColumnsWalk, "SUBSYSTEM_PATH", path, 1))
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Problem                            string
		First, Scrolled, Clicked, Filtered []any
		Again                              []any
		Round, Defaults                    []string
		Written, WrittenAgain              metadata.CommandInterface
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	if steps.Problem != "" {
		t.Fatalf("the walk met: %s", steps.Problem)
	}
	expect := func(what string, got, want any) {
		t.Helper()
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("%s: %s, want %s", what, gotJSON, wantJSON)
		}
	}
	grey := []string{"Первая mixed", "Вторая mixed", "Третья mixed"}
	expect("the columns", steps.First, []any{[]string{"Первая", "Вторая", "Третья"}, true, grey, grey})
	expect("scrolled", steps.Scrolled, []any{[]string{"Вторая", "Третья", "Четвёртая"}, []string{"Вторая mixed", "Третья mixed", "Четвёртая mixed"}})
	expect("clicked", steps.Clicked, []any{[]string{"Вторая mixed", "Третья mixed", "Четвёртая false"}, []string{"Вторая mixed", "Третья mixed", "Четвёртая true"}})
	expect("round", steps.Round, grey)
	expect("filtered", steps.Filtered, []any{[]string{"Первая"}, true})
	byCommand := map[metadata.StandardCommand]metadata.InterfaceCommandVisibility{}
	for _, item := range steps.Written.CommandsVisibility {
		byCommand[item.Standard] = item
	}
	list, creation := byCommand[metadata.StandardOpenList], byCommand[metadata.StandardCreate]
	expect("the list written", []any{len(steps.Written.CommandsVisibility), list.Common, list.Roles}, []any{2, true, []map[string]any{{"role": ids["Четвёртая"], "visible": false}}})
	expect("the creation written", []any{creation.Common, creation.Roles}, []any{false, []map[string]any{{"role": ids["Четвёртая"], "visible": true}}})
	expect("again", steps.Again, []any{[]string{"Вторая mixed", "Третья mixed", "Четвёртая false"}, []string{"Вторая mixed", "Третья mixed", "Четвёртая true"}})
	expect("the defaults", steps.Defaults, []string{"Вторая mixed", "Третья mixed", "Четвёртая mixed"})
	if len(steps.WrittenAgain.CommandsVisibility) != 1 || steps.WrittenAgain.CommandsVisibility[0].Standard != metadata.StandardOpenList {
		t.Errorf("the defaults leave written: %+v", steps.WrittenAgain.CommandsVisibility)
	}
}
