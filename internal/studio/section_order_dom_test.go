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

// sectionOrderWalk opens the window «Командный интерфейс» of the subsystem
// Склад and moves the list of Товары into «Панель навигации.Важное», tries
// the first command of «Создать» up, moves «Товары: создать» up, moves the
// second subsection up by Cmd+Shift+Up, reads the menu of the row of a group,
// presses OK and «Сохранить»; opens the window again, reads it, brings
// «Создать» back to the automatic order and the list of Товары to its
// defaults, presses OK and «Сохранить» again; opens it once more and moves
// «Контрагенты: создать» into «Сервис», where a command stands already.
const sectionOrderWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 120)).then(() => studio.settle());
  const host = document.querySelector('#subsystem-editor');
  const dialog = () => document.querySelector('dialog.section-commands-dialog');
  const read = () => [...dialog().querySelectorAll('.section-commands > .studio-tree-node, .section-commands > .studio-tree-row')].map(group => {
    const row = group.matches('.studio-tree-row') ? group : group.querySelector(':scope > .studio-tree-row');
    const commands = group.matches('.studio-tree-row') ? [] : [...group.querySelectorAll('.studio-tree-leaf')]
      .map(item => item.textContent + (item.querySelector('input')?.checked ? ' ✓' : ''));
    return [row.textContent, ...commands];
  });
  const row = text => [...dialog().querySelectorAll('.studio-tree-row')].find(item => item.textContent === text);
  const button = title => dialog().querySelector('.object-window-panel button[title="' + title + '"]');
  const panel = () => [...dialog().querySelectorAll('.object-window-panel button')].map(item => item.title + (item.disabled ? ' (недоступна)' : ''));
  const menu = async text => {
    row(text).dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
    await settle();
    return [...document.querySelectorAll('.context-menu .context-menu-item')];
  };
  const answer = async (selector, text) => {
    [...document.querySelectorAll(selector + ' .localized-title-actions button')].find(item => item.textContent === text).click();
    await settle();
  };
  const open = async () => {
    [...host.querySelectorAll('.object-window-button')].find(item => item.textContent === 'Командный интерфейс').click();
    await settle();
  };
  const save = async () => {
    await answer('dialog.section-commands-dialog', 'OK');
    document.querySelector('#save').click(); await settle();
    const text = await (await studioRequest('/api/subsystem?path=' + encodeURIComponent('SUBSYSTEM_PATH'), {method: 'GET'})).text();
    return text.startsWith('{') ? JSON.parse(text).subsystem.commandInterface || {} : {problem: text};
  };
  await openObject('Склад');
  await open();
  if (!dialog()) return {problem: JSON.stringify(studio.problems)};
  steps.first = read();
  row('Список товаров').click(); await settle();
  steps.onCommand = panel();
  button('Переместить команду в другую группу команд').click(); await settle();
  const choice = document.querySelector('dialog.list-choice-dialog');
  steps.choiceTitle = choice.querySelector('h3').textContent;
  steps.offered = [...choice.querySelectorAll('.object-window-list-row')].map(item => item.textContent + (item.classList.contains('current') ? ' •' : ''));
  [...choice.querySelectorAll('.object-window-list-row')].find(item => item.textContent === 'Панель навигации.Важное').click(); await settle();
  await answer('dialog.list-choice-dialog', 'OK');
  steps.moved = read();
  row('Контрагенты: создать').click(); await settle();
  button('Переместить вверх').click(); await settle();
  steps.firstUp = read();
  row('Товары: создать').click(); await settle();
  button('Переместить вверх').click(); await settle();
  steps.ordered = read();
  row('Я раздел').click(); await settle();
  steps.onSubsection = panel();
  dialog().querySelector('.section-commands').dispatchEvent(new KeyboardEvent('keydown', {key: 'ArrowUp', metaKey: true, shiftKey: true, bubbles: true}));
  await settle();
  steps.subsections = read().at(-1);
  row('Панель действий.Создать (ручной порядок)').click(); await settle();
  steps.onGroup = panel();
  steps.groupMenu = (await menu('Панель действий.Создать (ручной порядок)')).map(item => item.textContent + (item.disabled ? ' (недоступен)' : ''));
  document.dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape'}));
  document.querySelector('.context-menu')?.remove();
  steps.written = await save();
  await open();
  steps.again = read();
  const restore = (await menu('Панель действий.Создать (ручной порядок)')).find(item => item.textContent === 'Восстановить автоматический порядок');
  restore.click(); await settle();
  (await menu('Список товаров')).find(item => item.textContent === 'Установить свойства по умолчанию').click(); await settle();
  steps.restored = read();
  steps.writtenAgain = await save();
  await open();
  row('Контрагенты: создать').click(); await settle();
  button('Переместить команду в другую группу команд').click(); await settle();
  const second = document.querySelector('dialog.list-choice-dialog');
  steps.offeredActions = [...second.querySelectorAll('.object-window-list-row')].map(item => item.textContent);
  [...second.querySelectorAll('.object-window-list-row')].find(item => item.textContent === 'Панель действий.Сервис').click(); await settle();
  await answer('dialog.list-choice-dialog', 'OK');
  steps.toTools = read()[5];
  await answer('dialog.section-commands-dialog', 'Отмена');
  return steps;
})()`

// The commands of a section move into another group and are ordered by
// hand as the prototype's (podsistema.md, «Досъёмка 28.10.2026, поздний
// вечер»; snapshots vopr-2.636/07-28): «Подразделы» lists the subsystems
// under it in the command interface by the alphabet of their synonyms, not
// by their names; the buttons act on a
// command, up, down and the default on a subsection, none on a group;
// «Выбор группы команд» offers the groups of the panel the command stands
// in, the first current, and the command goes to the end of the group
// chosen; up on the first command does nothing; up makes a group «(ручной
// порядок)»; Cmd+Shift+Up orders «Подразделы»; the menu of a group offers
// only «Восстановить автоматический порядок» and «Настройка списка...»,
// which every row has; OK writes the placement with
// the visibility said, the whole order of the group with GroupsOrder, the
// order of «Подразделы»; the window opened again shows what is written;
// the automatic order and the defaults take it away; a command moved into a
// group of commands goes to its end, not among them by the alphabet, and a
// command of the actions panel is offered its groups. Defects caught: a
// subsystem out of the command interface among «Подразделы», a group of the
// other panel offered, the moved command not where it was put, an order
// going round its ends, the order of a group written in part or without
// GroupsOrder, a placement written without its visibility, what is written
// not shown again, the automatic order or the defaults leaving it written,
// a moved command sorted among the group's own.
func TestTheCommandsOfASectionMoveAndAreOrderedAsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	for path, said := range map[string]string{
		filepath.Join(root, "metadata", "catalogs", "Товары", "object.yaml"):      "use_standard_commands: true\nlist_presentation: {ru: Список товаров}\n",
		filepath.Join(root, "metadata", "catalogs", "Контрагенты", "object.yaml"): "use_standard_commands: true\nhierarchy: {enabled: true, kind: folders-and-items}\n",
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(string(content)+said), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The common form is titled to stand before a command moved into its
	// group by the alphabet.
	form := filepath.Join(root, "metadata", "common-forms", "ДемоФормаТовара", "form.yaml")
	content, err := os.ReadFile(form)
	if err != nil {
		t.Fatal(err)
	}
	retitled := strings.Replace(string(content), "title: {ru: Демо формы товара,", "title: {ru: Ящик товаров,", 1)
	if retitled == string(content) {
		t.Fatal("the title of the common form is not where it was")
	}
	if err := os.WriteFile(form, []byte(retitled), 0o644); err != nil {
		t.Fatal(err)
	}
	products, _ := uuid.Parse("10000000-0000-4000-8000-000000000101")
	parties, _ := uuid.Parse("10000000-0000-4000-8000-000000000102")
	productForm, _ := uuid.Parse("10000000-0000-4000-8000-000000000501")
	var path string
	children := map[string]uuid.UUID{}
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		created, err := workspace.CreateSubsystem("Склад")
		if err != nil {
			t.Fatal(err)
		}
		path = created.Path
		value := created.Subsystem
		value.Members = []uuid.UUID{products, parties, productForm}
		if _, err := workspace.SaveSubsystemWindow(created.Path, SubsystemWindowValue{SubsystemDefinition: value}, created.Revision); err != nil {
			t.Fatal(err)
		}
		// The project reads its subsystems by their identifiers: the two
		// subsections are titled against that order, so that only the
		// alphabet of the titles puts them as the window shows them.
		var made []SubsystemEditorSource
		for range 3 {
			child, err := workspace.CreateSubsystemChild(SubsystemChild{Parent: value.ID})
			if err != nil {
				t.Fatal(err)
			}
			made = append(made, child)
		}
		if made[1].Subsystem.ID.String() < made[0].Subsystem.ID.String() {
			made[0], made[1] = made[1], made[0]
		}
		for index, child := range []struct {
			name, title string
			shown       bool
		}{{"А", "Я раздел", true}, {"Б", "Б раздел", true}, {"Скрытый", "Скрытый раздел", false}} {
			named := made[index].Subsystem
			named.Name, named.Title, named.IncludeInCommandInterface = child.name, metadata.LocalizedText{"ru": child.title}, child.shown
			if _, err := workspace.SaveSubsystemWindow(made[index].Path, SubsystemWindowValue{SubsystemDefinition: named}, made[index].Revision); err != nil {
				t.Fatal(err)
			}
			children[child.name] = named.ID
		}
		automaticSubsections(t, workspace, created.Path)
	})
	result, err := browser.evaluate(ctx, page, strings.Replace(sectionOrderWalk, "SUBSYSTEM_PATH", path, 1))
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Problem                          string
		ChoiceTitle                      string
		First, Moved, FirstUp, Ordered   [][]string
		Again, Restored                  [][]string
		OnCommand, OnSubsection, OnGroup []string
		Offered, Subsections, GroupMenu  []string
		OfferedActions, ToTools          []string
		Written, WrittenAgain            metadata.CommandInterface
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
	section := func(important, ordinary []string, create string, created []string, subsections string, under []string) [][]string {
		return [][]string{append([]string{"Панель навигации.Важное"}, important...), append([]string{"Панель навигации.Обычное"}, ordinary...),
			{"Панель навигации.См. также"}, append([]string{create}, created...), {"Панель действий.Отчеты"}, {"Панель действий.Сервис", "Ящик товаров ✓"},
			append([]string{subsections}, under...)}
	}
	auto := []string{"Контрагенты: создать", "Контрагенты: создать группу", "Товары: создать"}
	expect("the window", steps.First, section(nil, []string{"Контрагенты ✓", "Список товаров ✓"}, "Панель действий.Создать", auto,
		"Подразделы", []string{"Б раздел", "Я раздел"}))
	expect("the buttons on a command", steps.OnCommand, []string{"Переместить команду в другую группу команд", "Переместить вверх", "Переместить вниз",
		"Установить свойства по умолчанию", "Скрыть команды, невидимые по умолчанию", "Выбор объекта: Роль"})
	expect("the choice", []any{steps.ChoiceTitle, steps.Offered}, []any{"Выбор группы команд",
		[]string{"Панель навигации.Важное •", "Панель навигации.Обычное", "Панель навигации.См. также"}})
	expect("moved", steps.Moved, section([]string{"Список товаров ✓"}, []string{"Контрагенты ✓"}, "Панель действий.Создать", auto,
		"Подразделы", []string{"Б раздел", "Я раздел"}))
	expect("up on the first", steps.FirstUp, steps.Moved)
	ordered := []string{"Контрагенты: создать", "Товары: создать", "Контрагенты: создать группу"}
	expect("ordered", steps.Ordered, section([]string{"Список товаров ✓"}, []string{"Контрагенты ✓"}, "Панель действий.Создать (ручной порядок)", ordered,
		"Подразделы", []string{"Б раздел", "Я раздел"}))
	expect("the buttons on a subsection", steps.OnSubsection, []string{"Переместить команду в другую группу команд (недоступна)", "Переместить вверх",
		"Переместить вниз", "Установить свойства по умолчанию", "Скрыть команды, невидимые по умолчанию", "Выбор объекта: Роль"})
	expect("subsections ordered", steps.Subsections, []string{"Подразделы (ручной порядок)", "Я раздел", "Б раздел"})
	expect("the buttons on a group", steps.OnGroup, []string{"Переместить команду в другую группу команд (недоступна)", "Переместить вверх (недоступна)",
		"Переместить вниз (недоступна)", "Установить свойства по умолчанию (недоступна)", "Скрыть команды, невидимые по умолчанию", "Выбор объекта: Роль"})
	expect("the menu of a group", steps.GroupMenu, []string{"Переместить команду (недоступен)", "Переместить вверхCmd+Shift+Up (недоступен)",
		"Переместить внизCmd+Shift+Down (недоступен)", "Установить свойства по умолчанию (недоступен)", "Восстановить автоматический порядок", "Настройка списка..."})
	written := steps.Written
	if len(written.CommandsPlacement) != 1 || written.CommandsPlacement[0].Object == nil || *written.CommandsPlacement[0].Object != products ||
		written.CommandsPlacement[0].Standard != metadata.StandardOpenList || written.CommandsPlacement[0].Group.Standard != metadata.NavigationPanelImportant ||
		written.CommandsPlacement[0].Placement != metadata.PlacementAuto {
		t.Errorf("the placement written: %+v", written.CommandsPlacement)
	}
	if len(written.CommandsVisibility) != 1 || *written.CommandsVisibility[0].Object != products || !written.CommandsVisibility[0].Common {
		t.Errorf("the visibility of the moved command is not said: %+v", written.CommandsVisibility)
	}
	var order []string
	for _, item := range written.CommandsOrder {
		if item.Group.Standard != metadata.ActionsPanelCreate {
			t.Errorf("an order of another group: %+v", item)
		}
		name := map[uuid.UUID]string{products: "Товары", parties: "Контрагенты"}[*item.Object]
		order = append(order, name+":"+string(item.Standard))
	}
	expect("the order written", order, []string{"Контрагенты:create", "Товары:create", "Контрагенты:create-folder"})
	expect("the groups ordered", written.GroupsOrder, []metadata.InterfaceGroup{{Standard: metadata.ActionsPanelCreate}})
	expect("the subsections ordered", written.SubsystemsOrder, []uuid.UUID{children["А"], children["Б"]})
	expect("opened again", steps.Again, section([]string{"Список товаров ✓"}, []string{"Контрагенты ✓"}, "Панель действий.Создать (ручной порядок)", ordered,
		"Подразделы (ручной порядок)", []string{"Я раздел", "Б раздел"}))
	expect("restored", steps.Restored, section(nil, []string{"Контрагенты ✓", "Список товаров ✓"}, "Панель действий.Создать", auto,
		"Подразделы (ручной порядок)", []string{"Я раздел", "Б раздел"}))
	again := steps.WrittenAgain
	if len(again.CommandsPlacement)+len(again.CommandsVisibility)+len(again.CommandsOrder)+len(again.GroupsOrder) != 0 {
		t.Errorf("the defaults leave written: %+v", again)
	}
	expect("the subsections kept", again.SubsystemsOrder, []uuid.UUID{children["А"], children["Б"]})
	expect("the actions panel offered", steps.OfferedActions, []string{"Панель действий.Создать", "Панель действий.Отчеты", "Панель действий.Сервис"})
	expect("moved to the end", steps.ToTools, []string{"Панель действий.Сервис", "Ящик товаров ✓", "Контрагенты: создать"})
}
