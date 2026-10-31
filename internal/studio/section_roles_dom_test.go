package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// sectionRolesWalk opens the window «Командный интерфейс» of the subsystem
// Склад, chooses in «Выбор объекта: Роль» the role Просмотр, then Добавление
// too, then Пусто alone, reading the tree and the field each time; reads the
// list of the field, takes its first value back by it, then «<Не
// установлен>»; and chooses no role at all.
const sectionRolesWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 120)).then(() => studio.settle());
  const host = document.querySelector('#subsystem-editor');
  const dialog = () => document.querySelector('dialog.section-commands-dialog');
  const read = () => [...dialog().querySelectorAll('.section-commands > .studio-tree-node, .section-commands > .studio-tree-row')].map(group => {
    const row = group.matches('.studio-tree-row') ? group : group.querySelector(':scope > .studio-tree-row');
    const commands = group.matches('.studio-tree-row') ? [] : [...group.querySelectorAll('.studio-tree-leaf')].map(item => item.textContent);
    return [row.textContent, ...commands];
  });
  const field = () => dialog().querySelector('select[aria-label="Отбор по ролям"]');
  const shown = () => field().selectedOptions[0].textContent;
  const choose = async (...names) => {
    dialog().querySelector('button[title="Выбор объекта: Роль"]').click(); await settle();
    const choice = document.querySelector('dialog.object-choice-dialog');
    const rows = [...choice.querySelectorAll('.studio-tree-row')].map(item => item.textContent);
    rows.unshift(choice.querySelector('details.studio-tree-node').open ? 'открыт' : 'закрыт');
    for (const box of choice.querySelectorAll('input.object-choice-box')) {
      if (box.checked !== names.includes(box.getAttribute('aria-label'))) box.click();
    }
    [...choice.querySelectorAll('.localized-title-actions button')].find(item => item.textContent === 'ОК').click(); await settle();
    return rows;
  };
  const pick = async text => {
    const option = [...field().options].find(item => item.textContent === text);
    field().value = option.value;
    field().dispatchEvent(new Event('change', {bubbles: true}));
    await settle();
  };
  await openObject('Склад');
  [...host.querySelectorAll('.object-window-button')].find(item => item.textContent === 'Командный интерфейс').click();
  await settle();
  if (!dialog()) return {problem: JSON.stringify(studio.problems)};
  steps.before = [shown(), read()];
  dialog().querySelector('button[title="Выбор объекта: Роль"]').click(); await settle();
  {
    const choice = document.querySelector('dialog.object-choice-dialog');
    const now = () => choice.querySelector('.studio-tree-row.selected')?.textContent || '';
    steps.typed = [document.activeElement === choice.querySelector('.object-choice-tree') ? 'дерево' : 'не дерево', now()];
    for (const key of ['п', 'П', 'п', 'д', 'я', 'Д']) {
      document.activeElement.dispatchEvent(new KeyboardEvent('keydown', {key, bubbles: true}));
      steps.typed.push(now());
    }
    choice.querySelectorAll('.studio-tree-row')[1].click();
    steps.typed.push(now());
    const search = choice.querySelector('.object-choice-search input');
    search.value = 'СМОТР'; search.dispatchEvent(new Event('input', {bubbles: true})); await settle();
    steps.searched = [...choice.querySelectorAll('.studio-tree-row')].map(row => [row.textContent,
      [...row.querySelectorAll('.studio-search-match')].map(part => part.textContent).join('+'), row.classList.contains('studio-search-path')]);
    [...choice.querySelectorAll('.localized-title-actions button')].find(item => item.textContent === 'Отмена').click(); await settle();
  }
  steps.choice = await choose('Просмотр');
  steps.viewing = [shown(), read()];
  await choose('Просмотр', 'Добавление');
  steps.adding = [shown(), read()];
  await choose('Пусто');
  steps.empty = [shown(), read()];
  steps.list = [...field().options].map(item => item.textContent);
  await pick('Просмотр');
  steps.picked = [shown(), read()];
  await pick('<Не установлен>');
  steps.unset = [shown(), read().length];
  await choose('Просмотр');
  await choose();
  steps.none = [shown(), read().length];
  return steps;
})()`

// «Отбор по ролям» of the window «Командный интерфейс» of a subsystem is the
// prototype's (obshchee.md, «Видимость по ролям»; snapshots
// series-2.603/44-47, vopr-2.636/02-06): «Выбор объекта: Роль» shows the
// project open and its roles by their names in the order of the tree (of
// their making here); a role
// with «Просмотр» of a catalog keeps its list and not its creation, one with
// «Интерактивное добавление» its creation too, roles add up; a group without
// a command they open goes, empty groups too, and a subsection whose content
// and the subsystems under it they open nothing of - «Подразделы» with the
// last; the field shows the
// roles by their names, its list the values it had, the last first, and
// takes one back; «<Не установлен>» and a choice of no role take the filter
// away. The choice opens with the focus on its tree, the root current; a
// letter goes to the next row on it, round, the root among them, a letter
// no row begins with stays, a click makes a row current (snapshots
// dosyomka-2653/20-24). A search, in any case, leaves the roles holding it,
// the found part red, the root grey (snapshot dosyomka-2653/34). Defects
// caught: the found part not drawn, the case taken into account in the
// search, the root drawn as a match; the focus left in the search, no
// current row, a letter searching from the first row, no round, the case
// taken into account, a right taken for another, a role's rights taken for
// another's, the roles not added up, an empty group or a closed subsection
// left, a subsection open by a subsystem under it left out, the root of the
// choice closed, the field or its list wrong, the filter not taken away.
func TestTheFilterByRolesOfASectionIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	for path, said := range map[string]string{
		filepath.Join(root, "metadata", "catalogs", "Товары", "object.yaml"):      "use_standard_commands: true\nlist_presentation: {ru: Список товаров}\n",
		filepath.Join(root, "metadata", "catalogs", "Контрагенты", "object.yaml"): "use_standard_commands: true\n",
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(string(content)+said), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	products, _ := uuid.Parse("10000000-0000-4000-8000-000000000101")
	parties, _ := uuid.Parse("10000000-0000-4000-8000-000000000102")
	productForm, _ := uuid.Parse("10000000-0000-4000-8000-000000000501")
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		operations := func(names ...metadata.PermissionOperation) []metadata.PermissionOperation { return names }
		for _, made := range []struct {
			name    string
			objects []metadata.ObjectPermission
		}{
			{"Просмотр", []metadata.ObjectPermission{{Object: products, Operations: operations(metadata.PermissionRead, metadata.PermissionView)}}},
			{"Добавление", []metadata.ObjectPermission{{Object: products, Operations: operations(metadata.PermissionRead, metadata.PermissionInsert,
				metadata.PermissionInteractiveInsert)}}},
			{"Пусто", []metadata.ObjectPermission{{Object: parties, Operations: operations(metadata.PermissionRead)}}},
		} {
			role, err := workspace.CreateRole(made.name)
			if err != nil {
				t.Fatal(err)
			}
			role.Role.Objects = made.objects
			if _, err := workspace.SaveRole(role.Path, role.Role, role.Revision); err != nil {
				t.Fatal(err)
			}
		}
		created, err := workspace.CreateSubsystem("Склад")
		if err != nil {
			t.Fatal(err)
		}
		value := created.Subsystem
		value.Members = []uuid.UUID{products, parties, productForm}
		if _, err := workspace.SaveSubsystemWindow(created.Path, SubsystemWindowValue{SubsystemDefinition: value}, created.Revision); err != nil {
			t.Fatal(err)
		}
		// Партнёрский holds what Просмотр does not open, but a subsystem
		// under it holds what it does; Закрытый holds nothing it opens.
		made := map[string]uuid.UUID{"Склад": value.ID}
		for _, child := range []struct {
			name, parent string
			member       uuid.UUID
		}{{"Товарный", "Склад", products}, {"Партнёрский", "Склад", parties}, {"Закрытый", "Склад", parties}, {"Внутренний", "Партнёрский", products}} {
			fresh, err := workspace.CreateSubsystemChild(SubsystemChild{Parent: made[child.parent]})
			if err != nil {
				t.Fatal(err)
			}
			named := fresh.Subsystem
			named.Name, named.Members = child.name, []uuid.UUID{child.member}
			if _, err := workspace.SaveSubsystemWindow(fresh.Path, SubsystemWindowValue{SubsystemDefinition: named}, fresh.Revision); err != nil {
				t.Fatal(err)
			}
			made[child.name] = named.ID
		}
		automaticSubsections(t, workspace, created.Path)
	})
	result, err := browser.evaluate(ctx, page, sectionRolesWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Problem                                             string
		Choice, List, Typed                                 []string
		Before, Viewing, Adding, Empty, Picked, Unset, None []any
		Searched                                            [][]any
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
	all := [][]string{{"Панель навигации.Важное"}, {"Панель навигации.Обычное", "Контрагенты", "Список товаров"}, {"Панель навигации.См. также"},
		{"Панель действий.Создать", "Контрагенты: создать", "Товары: создать"}, {"Панель действий.Отчеты"}, {"Панель действий.Сервис", "Демо формы товара"},
		{"Подразделы", "Закрытый", "Партнёрский", "Товарный"}}
	expect("before", steps.Before, []any{"<Не установлен>", all})
	expect("the choice", steps.Choice, []string{"открыт", "ПродажиИСклад", "Просмотр", "Добавление", "Пусто"})
	expect("typed in the choice", steps.Typed, []string{"дерево", "ПродажиИСклад", "Просмотр", "Пусто", "ПродажиИСклад", "Добавление", "Добавление", "Добавление", "Просмотр"})
	expect("searched in the choice", steps.Searched, [][]any{{"ПродажиИСклад", "", true}, {"Просмотр", "смотр", false}})
	expect("by viewing", steps.Viewing, []any{"Просмотр", [][]string{{"Панель навигации.Обычное", "Список товаров"}, {"Подразделы", "Партнёрский", "Товарный"}}})
	expect("by adding too", steps.Adding, []any{"Просмотр, Добавление", [][]string{{"Панель навигации.Обычное", "Список товаров"},
		{"Панель действий.Создать", "Товары: создать"}, {"Подразделы", "Партнёрский", "Товарный"}}})
	expect("by a role opening nothing", steps.Empty, []any{"Пусто", [][]string{}})
	expect("the list", steps.List, []string{"<Не установлен>", "Пусто", "Просмотр, Добавление", "Просмотр"})
	expect("picked from the list", steps.Picked, []any{"Просмотр", [][]string{{"Панель навигации.Обычное", "Список товаров"}, {"Подразделы", "Партнёрский", "Товарный"}}})
	expect("not set", steps.Unset, []any{"<Не установлен>", len(all)})
	expect("no role chosen", steps.None, []any{"<Не установлен>", len(all)})
}
