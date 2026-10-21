package studio

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// roleTreeLayoutWalk opens the window of the role Кладовщик and «Все роли»
// and reads the outline of each tree of rights: the rows beneath the root and
// beneath «Общие» and «Документы», by caption.
const roleTreeLayoutWalk = `(async () => {
  const studio = window.__studio, answer = {};
  const caption = details => details.querySelector(':scope > .studio-tree-row').textContent.trim();
  const beneath = details => [...details.querySelector(':scope > .studio-tree-children').children].map(item => item.matches('details') ? caption(item) : item.textContent.trim());
  const outline = host => {
    const top = [...host.children].filter(item => item.matches('details, .studio-tree-row'));
    const root = top[0], branch = text => [...root.querySelectorAll('details')].find(item => caption(item) === text);
    return {top: top.map(item => item.matches('details') ? caption(item) : item.textContent.trim()), icon: root.querySelector(':scope > .studio-tree-row use')?.getAttribute('href'),
      root: beneath(root), common: branch('Общие') ? beneath(branch('Общие')) : null, documents: branch('Документы') ? beneath(branch('Документы')) : null};
  };
  const tree = text => [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === text);
  tree('Кладовщик').click(); await studio.settle();
  answer.role = outline(document.querySelector('#role-editor .role-tree'));
  tree('Роли').dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent === 'Все роли').click();
  await studio.settle();
  answer.roles = outline(document.querySelector('#role-overview .role-tree'));
  return answer;
})()`

// The tree of rights stands as the project tree and the prototype's: the
// configuration at the top with everything beneath it, «Общие» first holding
// the common kinds, then the branches of the kinds in the prototype's order,
// the sequences inside «Документы» before the documents (inventory of block
// 00, rol.md - «корень конфигурации, «Общие», виды»; snapshots
// configurator-2026-10-14/rol-otbor-kassa, derevo/27 for the order; written
// out here, not read from the descriptors). Every kind rights are given on
// has its branch, the empty ones too - the project has no constant, no
// journal, no external source (snapshots configurator-2026-10-20/prava/19 and 30; zapis.md
// «Дерево прав»), the enumerations among them, shown for their commands
// (prava/42-44); none of the integration services the owner refused. Defects caught: the root a row
// of its own above the branches; no «Общие», a common form standing beside
// the catalogs; the sequences beside the documents rather than in them; the
// order of the branches not the prototype's; a branch without objects left
// out.
func TestTheTreeOfRightsStandsAsTheProjectTree(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	writeProjectFile(t, root, "metadata/sequences/Партии/object.yaml",
		"format: 1\nid: "+uuid.MustNew().String()+"\nname: Партии\ntitle: {ru: Партии}\n")
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, roleTreeLayoutWalk)
	if err != nil {
		t.Fatal(err)
	}
	type outline struct {
		Top, Root, Common, Documents []string
		Icon                         string
	}
	var got struct{ Role, Roles outline }
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	for name, tree := range map[string]outline{"role": got.Role, "all roles": got.Roles} {
		if len(tree.Top) != 1 || tree.Icon != "#icon-configuration" {
			t.Errorf("%s: the tree begins %v (icon %s), want the configuration alone", name, tree.Top, tree.Icon)
		}
		if want := []string{"Общие", "Константы", "Справочники", "Документы", "Журналы документов", "Перечисления", "Отчёты", "Обработки",
			"Планы видов характеристик", "Планы счетов", "Планы видов расчёта", "Регистры сведений", "Регистры накопления",
			"Регистры бухгалтерии", "Регистры расчёта", "Бизнес-процессы", "Задачи", "Внешние источники данных"}; !slices.Equal(tree.Root, want) {
			t.Errorf("%s: beneath the root %v, want %v", name, tree.Root, want)
		}
		if want := []string{"Подсистемы", "Параметры сеанса", "Общие реквизиты", "Планы обмена", "Критерии отбора", "Общие команды",
			"Общие формы", "Web-сервисы", "HTTP-сервисы"}; !slices.Equal(tree.Common, want) {
			t.Errorf("%s: «Общие» holds %v, want %v", name, tree.Common, want)
		}
		if len(tree.Documents) < 2 || tree.Documents[0] != "Последовательности" || !strings.Contains(strings.Join(tree.Documents[1:], " "), "Продажа") {
			t.Errorf("%s: «Документы» holds %v, want the sequences, then the documents", name, tree.Documents)
		}
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}

// roleTreeOpeningWalk opens the window of the role Кладовщик and «Все роли»
// and reads how each tree of rights stands: the captions of the nodes open,
// the row chosen, the head of the panel. In the window of the role it opens
// «Справочники», chooses Товары and reads again; from «Все роли», with Товары
// chosen and the role's column, it opens the role on Товары; then it opens
// «Все роли» again, opens «Справочники», leaves for the role and comes back.
const roleTreeOpeningWalk = `(async () => {
  const studio = window.__studio, answer = {};
  const caption = details => details.querySelector(':scope > .studio-tree-row').textContent.trim();
  const state = host => ({open: [...host.querySelectorAll('.role-tree details')].filter(item => item.open).map(caption),
    chosen: [...host.querySelectorAll('.role-tree .role-tree-item.selected')].map(item => item.textContent.trim()),
    panel: host.querySelector('h3')?.textContent.trim() || ''});
  const branch = (host, text) => [...host.querySelectorAll('.role-tree details')].find(item => caption(item) === text);
  const item = (host, text) => [...host.querySelectorAll('.role-tree .role-tree-item')].find(button => button.textContent.trim() === text);
  const tree = text => [...document.querySelectorAll('#tree .studio-tree-row')].find(row => row.textContent.trim() === text);
  const allRoles = async () => {
    tree('Роли').dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
    [...document.querySelectorAll('.context-menu .context-menu-item')].find(entry => entry.textContent === 'Все роли').click();
    await studio.settle();
  };
  const role = document.querySelector('#role-editor'), roles = document.querySelector('#role-overview');
  tree('Кладовщик').click(); await studio.settle();
  answer.role = state(role);
  const catalogs = branch(role, 'Справочники'); catalogs.open = true;
  await new Promise(done => catalogs.addEventListener('toggle', done, {once: true}));
  item(role, 'Товары').click(); await studio.settle();
  answer.roleAfterChoice = state(role);
  await allRoles();
  answer.roles = state(roles);
  let overviewCatalogs = branch(roles, 'Справочники'); overviewCatalogs.open = true;
  await new Promise(done => overviewCatalogs.addEventListener('toggle', done, {once: true}));
  item(roles, 'Товары').click(); await studio.settle();
  [...roles.querySelectorAll('thead button.overview-role')].find(button => button.textContent.trim() === 'Кладовщик')?.click();
  await studio.settle();
  const open = [...roles.querySelectorAll('button.overview-role')].find(button => button.textContent === 'Открыть роль');
  answer.link = !!open;
  open?.click(); await studio.settle();
  answer.revealed = state(role);
  await allRoles();
  overviewCatalogs = branch(roles, 'Справочники'); overviewCatalogs.open = true;
  await new Promise(done => overviewCatalogs.addEventListener('toggle', done, {once: true}));
  tree('Кладовщик').click(); await studio.settle();
  await allRoles();
  answer.rolesAgain = state(roles);
  return answer;
})()`

// The tree of rights opens as the prototype's: in the window of a role and in
// «Все роли» only the configuration is open and it is chosen, its rights
// shown; «Общие» and the branches of the kinds are closed (опись rol.md,
// snapshots configurator-2026-10-20/menyu-01, -02; written out here). What
// is opened stays open while the window is, and the window opened again
// stands as at first; a role opened from «Все роли» on an object shows the
// object, the branch of its kind open. Defects caught: the branches drawn
// open always; nothing chosen when the window opens; a branch opened by hand
// closing at the next choice; what was opened kept into the next opening;
// the role opened on an object it hides in a closed branch.
func TestTheTreeOfRightsOpensWithTheConfigurationAlone(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, roleTreeOpeningWalk)
	if err != nil {
		t.Fatal(err)
	}
	type state struct {
		Open, Chosen []string
		Panel        string
	}
	var got struct {
		Role, RoleAfterChoice, Roles, Revealed, RolesAgain state
		Link                                               bool
	}
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	for name, opened := range map[string]state{"role": got.Role, "all roles": got.Roles, "all roles again": got.RolesAgain} {
		if len(opened.Open) != 1 || len(opened.Chosen) != 1 || opened.Chosen[0] != opened.Open[0] || opened.Panel != opened.Chosen[0] {
			t.Errorf("%s: opened with %v open, %v chosen, the panel of %q; want the configuration alone open, chosen and shown", name, opened.Open, opened.Chosen, opened.Panel)
		}
	}
	if !slices.Equal(got.RoleAfterChoice.Open, []string{got.Role.Open[0], "Справочники"}) || !slices.Equal(got.RoleAfterChoice.Chosen, []string{"Товары"}) {
		t.Errorf("after opening «Справочники» and choosing Товары: %v open, %v chosen; want the branch kept open", got.RoleAfterChoice.Open, got.RoleAfterChoice.Chosen)
	}
	if !got.Link {
		t.Fatal("«Все роли» gave no «Открыть роль» for Товары and the role Кладовщик")
	}
	if !slices.Contains(got.Revealed.Open, "Справочники") || !slices.Equal(got.Revealed.Chosen, []string{"Товары"}) {
		t.Errorf("the role opened from «Все роли» on Товары: %v open, %v chosen; want Товары shown in its open branch", got.Revealed.Open, got.Revealed.Chosen)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}

// roleTreeRevealWalk chooses the recalculation Перерасчет in «Все роли» with
// the role Кладовщик and opens the role on it, and reads where the role
// window stands: the row chosen and whether every node above it is open.
const roleTreeRevealWalk = `(async () => {
  const studio = window.__studio;
  const tree = text => [...document.querySelectorAll('#tree .studio-tree-row')].find(row => row.textContent.trim() === text);
  const item = (host, text) => [...host.querySelectorAll('.role-tree .role-tree-item')].find(button => button.textContent.trim() === text);
  const role = document.querySelector('#role-editor'), roles = document.querySelector('#role-overview');
  tree('Роли').dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(entry => entry.textContent === 'Все роли').click();
  await studio.settle();
  item(roles, 'Перерасчет').click(); await studio.settle();
  [...roles.querySelectorAll('thead button.overview-role')].find(button => button.textContent.trim() === 'Кладовщик')?.click();
  await studio.settle();
  const open = [...roles.querySelectorAll('button.overview-role')].find(button => button.textContent === 'Открыть роль');
  if (!open) return {link: false};
  open.click(); await studio.settle();
  const chosen = role.querySelector('.role-tree .role-tree-item.selected'), closed = [];
  for (let details = chosen?.parentElement.closest('details'); details; details = details.parentElement.closest('details'))
    if (!details.open) closed.push(details.querySelector(':scope > .studio-tree-row').textContent.trim());
  return {link: true, chosen: chosen?.textContent.trim() || '', closed};
})()`

// A role opened from «Все роли» on an object inside another shows it: the
// configuration, the branch of the owner's kind, the owner and its folder of
// the recalculations are opened, the recalculation chosen. Defect caught:
// the folder of the objects inside an owner left closed, the object chosen
// out of sight.
func TestTheRoleOpenedOnAnObjectInsideAnotherShowsIt(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	chart := uuid.MustNew().String()
	writeProjectFile(t, root, "metadata/charts-of-calculation-types/Начисления/object.yaml", "format: 1\nid: "+chart+
		"\nname: Начисления\ntitle: {ru: Начисления}\ncode: {type: string, length: 9, auto: false}\ndescription_length: 100\n")
	writeProjectFile(t, root, "metadata/calculation-registers/Начисления/object.yaml", "format: 1\nid: "+uuid.MustNew().String()+
		"\nname: Начисления\ntitle: {ru: Начисления}\nchart_of_calculation_types: "+chart+"\nperiodicity: month\nresources:\n"+
		"  - {id: "+uuid.MustNew().String()+", name: Результат, title: {ru: Результат}, types: [{kind: number, precision: 15, scale: 2}]}\n"+
		"recalculations:\n  - {id: "+uuid.MustNew().String()+", name: Перерасчет, title: {ru: Перерасчет}}\n")
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, roleTreeRevealWalk)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Link   bool
		Chosen string
		Closed []string
	}
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	if !got.Link {
		t.Fatal("«Все роли» gave no «Открыть роль» for Перерасчет and the role Кладовщик")
	}
	if got.Chosen != "Перерасчет" || len(got.Closed) > 0 {
		t.Errorf("the role opened on Перерасчет: %q chosen, %v closed above it; want Перерасчет chosen and everything above it open", got.Chosen, got.Closed)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}

// roleEmptyBranchWalk opens the window of the role Кладовщик, chooses the
// branch «Константы», which holds nothing, reads the rights listed, presses
// the box of «Чтение» and reads them again; then chooses «Перечисления» and
// reads its rights.
const roleEmptyBranchWalk = `(async () => {
  const studio = window.__studio, editor = document.querySelector('#role-editor'), answer = {};
  const rights = () => [...editor.querySelectorAll('.role-permissions .check-list-row')].map(row => ({caption: row.textContent.trim(), checked: row.querySelector('input').checked}));
  [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === 'Кладовщик').click();
  await studio.settle();
  [...editor.querySelectorAll('.role-tree-group')].find(item => item.textContent.trim() === 'Константы').click(); await studio.settle();
  answer.listed = rights();
  [...editor.querySelectorAll('.role-permissions .check-list-row')].find(row => row.textContent.trim() === 'Чтение').querySelector('input').click(); await studio.settle();
  answer.pressed = rights();
  [...editor.querySelectorAll('.role-tree-group')].find(item => item.textContent.trim() === 'Перечисления').click(); await studio.settle();
  answer.enumerations = rights();
  return answer;
})()`

// An empty branch shows the rights of its kind, as the prototype's
// «Константы» and «Последовательности» (pass of 20.10.2026, zapis.md «Дерево
// прав», snapshot prava/10): the rights of a constant ML keeps, written out here from
// the pass; a click marks nothing, there being nothing to mark. The branch
// «Перечисления» has no rights at all (prava/42). Defects caught: an empty
// branch with no rights or with another kind's; the enumerations given
// rights of their own; a click on
// it marking the box with nothing behind it.
func TestAnEmptyBranchShowsTheRightsOfItsKind(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, roleEmptyBranchWalk)
	if err != nil {
		t.Fatal(err)
	}
	type right struct {
		Caption string
		Checked bool
	}
	var got struct{ Listed, Pressed, Enumerations []right }
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	var want []right
	// The pass showed seven rights of the history of data besides, which ML
	// does not keep (ACCESS-RIGHTS, «Права истории данных»).
	for _, caption := range []string{"Чтение", "Изменение", "Просмотр", "Редактирование"} {
		want = append(want, right{Caption: caption})
	}
	if !slices.Equal(got.Listed, want) {
		t.Errorf("the empty branch «Константы» lists %v, want %v", got.Listed, want)
	}
	if !slices.Equal(got.Pressed, want) {
		t.Errorf("after a click on «Чтение» there: %v, want nothing marked", got.Pressed)
	}
	if len(got.Enumerations) != 0 {
		t.Errorf("the branch «Перечисления» lists %v, want no rights", got.Enumerations)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
