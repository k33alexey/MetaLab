package studio

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/metadata"
)

// openAllRestrictions opens «Все ограничения доступа» from the branch «Роли».
const openAllRestrictions = `
  const studio = window.__studio;
  const roles = [...document.querySelectorAll('#tree summary, #tree .leaf')].find(item => item.textContent.trim() === 'Роли');
  roles.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent === 'Все ограничения доступа').click();
  await studio.settle();
  const list = () => document.querySelector('.overview-block .settable-list');
  const headings = () => [...list().querySelectorAll('.list-heading')].map(item => item.textContent + (item.dataset.sort === 'ascending' ? '▲' : item.dataset.sort === 'descending' ? '▼' : ''));
  const column = key => [...list().querySelectorAll('tbody tr')].map(row => row.querySelector('[data-column="' + key + '"]')?.textContent);
  const chosen = () => [...list().querySelectorAll('tbody tr')].findIndex(row => row.classList.contains('selected'));
`

// listSettingsWalk walks the list of restrictions and its «Настройка
// списка»: what the list shows as opened, after a click on the heading
// «Роль» and a second one, the window with «Поля» taken off and applied,
// and what the server keeps after OK.
const listSettingsWalk = `(async () => {` + openAllRestrictions + `
  const answer = {opened: {headings: headings(), roles: column('role'), chosen: chosen(), rows: column('role').length}};
  const condition = list().querySelector('tbody [data-column="condition"]');
  answer.conditionCell = {auto: condition.classList.contains('list-auto'), maxHeight: getComputedStyle(condition).maxHeight};
  const icon = item => item.querySelector('use')?.getAttribute('href')?.replace('#icon-', '') || '';
  answer.panel = [...document.querySelectorAll('.overview-block .role-bulk > *')].map(item => item.matches('button') ? (item.getAttribute('aria-label') || item.textContent) + '/' + icon(item) : '|');
  list().querySelector('.list-heading[data-column="role"]').click();
  await studio.settle();
  answer.byRole = {headings: headings(), roles: column('role')};
  list().querySelector('.list-heading[data-column="role"]').click();
  await studio.settle();
  answer.byRoleBack = {headings: headings(), roles: column('role')};
  [...document.querySelectorAll('.overview-block .role-bulk button')].find(button => button.textContent === 'Действия').click();
  const items = [...document.querySelectorAll('.context-menu .context-menu-item')];
  answer.menu = items.map(item => item.textContent);
  answer.menuIcons = items.map(icon);
  items.find(item => item.textContent.startsWith('Настройка списка')).click();
  await studio.settle();
  const dialog = document.querySelector('dialog[open]');
  const buttons = () => [...dialog.querySelectorAll('.localized-title-actions button')];
  answer.window = {title: dialog.querySelector('h3').textContent, tabs: [...dialog.querySelectorAll('.list-settings-tab')].map(item => item.textContent),
    columns: [...dialog.querySelectorAll('.list-settings-item')].map(item => item.textContent), buttons: buttons().map(item => item.textContent),
    applyBefore: buttons().find(item => item.textContent === 'Применить').disabled};
  const right = [...dialog.querySelectorAll('.list-settings-item')].find(item => item.textContent === 'Право');
  right.querySelector('span').click();
  answer.window.captionChooses = right.classList.contains('selected') && right.querySelector('input').checked;
  // The tab opened holds the focus: a click on «Прочее» moves it there
  // from «Колонки», which had it when the window opened.
  const other = [...dialog.querySelectorAll('.list-settings-tab')][1], columnsTab = [...dialog.querySelectorAll('.list-settings-tab')][0];
  other.click();
  answer.window.focusOnOther = document.activeElement === other && other.getAttribute('aria-selected') === 'true';
  columnsTab.click();
  answer.window.focusBack = document.activeElement === columnsTab;
  [...dialog.querySelectorAll('.list-settings-item')].find(item => item.textContent === 'Поля').querySelector('input').click();
  answer.window.applyAfter = buttons().find(item => item.textContent === 'Применить').disabled;
  buttons().find(item => item.textContent === 'Применить').click();
  await studio.settle();
  answer.applied = {open: !!document.querySelector('dialog[open]'), headings: headings()};
  buttons().find(item => item.textContent === 'ОК').click();
  await studio.settle();
  answer.kept = await (await fetch('/api/settings/lists')).json();
  return answer;
})()`

// listSettingsAfterReload opens the window again in a page loaded anew.
const listSettingsAfterReload = `(async () => {` + openAllRestrictions + `
  return {headings: headings(), roles: column('role')};
})()`

// The list of «Все ограничения доступа» is laid out and set up as the
// prototype's (snapshots configurator-2026-10-18/vse-ogr-01, -10…19; help,
// form_customizegriddlg): the prototype's columns, ordered by «Объект», the
// list opening on its last row, the restriction five lines high at most; a
// click on a heading orders by its column, a second one turns the order
// round; the commands on the bar by their icons alone and in the menu with
// them, «Настройка списка...» only in «Действия», its window with
// «Колонки» and «Прочее», «Применить» available after a change and showing
// it at once; the setting kept for the user and read again by a page loaded
// anew; a click on the caption of a column in the window chooses it and
// leaves its box; the tab opened holds the focus (vse-ogr-16). Defects caught: the column «Реквизиты» or «Условие» in place of the
// prototype's; the list opening at its top; the heading not ordering, or
// not turning the order; a command on the bar by its words or without its
// icon, a command of the menu without one; a button «Настройка списка» on
// the panel; «Применить»
// available with nothing changed or not showing the change; the setting
// lost with the page.
func TestTheListOfAllRestrictionsIsSetUpAsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	directory := t.TempDir()
	var workspace *Workspace
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		workspace = opened
		opened.KeepUserSettingsIn(directory)
		addRestrictedRole(t, opened, root)
		addSecondRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, listSettingsWalk)
	if err != nil {
		t.Fatal(err)
	}
	type view struct {
		Headings []string
		Roles    []string
		Chosen   int
		Rows     int
	}
	var walk struct {
		Opened        view
		ConditionCell struct {
			Auto      bool
			MaxHeight string
		}
		Panel      []string
		ByRole     view
		ByRoleBack view
		Menu       []string
		MenuIcons  []string
		Window     struct {
			Title          string
			Tabs           []string
			Columns        []string
			Buttons        []string
			ApplyBefore    bool
			ApplyAfter     bool
			CaptionChooses bool
			FocusOnOther   bool
			FocusBack      bool
		}
		Applied struct {
			Open     bool
			Headings []string
		}
		Kept struct {
			Lists map[string]struct {
				Columns []struct {
					Key     string
					Visible bool
				}
				Sort struct {
					Column     string
					Descending bool
				}
			}
		}
	}
	if err := json.Unmarshal(result, &walk); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	if want := []string{"Объект▲", "Роль", "Право", "Поля", "Ограничение доступа"}; !slices.Equal(walk.Opened.Headings, want) {
		t.Errorf("the list opens with %v, want %v", walk.Opened.Headings, want)
	}
	if walk.Opened.Rows != 2 || walk.Opened.Chosen != walk.Opened.Rows-1 {
		t.Errorf("the list of %d rows opens on row %d, want the last", walk.Opened.Rows, walk.Opened.Chosen)
	}
	if !walk.ConditionCell.Auto || walk.ConditionCell.MaxHeight != "80px" {
		t.Errorf("the restriction is %+v, want autoheight up to 5 lines of 16px", walk.ConditionCell)
	}
	if !slices.Equal(walk.ByRole.Roles, []string{"Кладовщик", "Продавец"}) || walk.ByRole.Headings[1] != "Роль▲" || walk.ByRole.Headings[0] != "Объект" {
		t.Errorf("a click on «Роль» shows %v %v", walk.ByRole.Headings, walk.ByRole.Roles)
	}
	if !slices.Equal(walk.ByRoleBack.Roles, []string{"Продавец", "Кладовщик"}) || walk.ByRoleBack.Headings[1] != "Роль▼" {
		t.Errorf("a second click on «Роль» shows %v %v", walk.ByRoleBack.Headings, walk.ByRoleBack.Roles)
	}
	// The prototype's bar and menu (snapshot vse-ogr-01): «Действия», then
	// the commands by their icons alone in groups, «Вывести список...» and
	// «Настройка списка...» only in the menu; every command of the menu
	// with its icon.
	if want := []string{"Действия/", "Добавить/add", "Изменить/change", "Удалить/delete", "|", "Копировать ограничения доступа/clone", "|",
		"Установить отбор.../filter-and-sort", "Отбор по значению в текущей колонке/filter-by-current-value", "Отключить отбор/clear-filter"}; !slices.Equal(walk.Panel, want) {
		t.Errorf("the bar is %v, want %v", walk.Panel, want)
	}
	if want := []string{"add", "change", "delete", "clone", "filter-and-sort", "filter-by-current-value", "clear-filter", "list-settings"}; !slices.Equal(walk.MenuIcons, want) || walk.Menu[len(walk.Menu)-1] != "Настройка списка..." {
		t.Errorf("the menu is %v with icons %v, want icons %v", walk.Menu, walk.MenuIcons, want)
	}
	if walk.Window.Title != "Настройка списка" || !slices.Equal(walk.Window.Tabs, []string{"Колонки", "Прочее"}) ||
		!slices.Equal(walk.Window.Columns, []string{"Объект", "Роль", "Право", "Поля", "Ограничение доступа"}) ||
		!slices.Equal(walk.Window.Buttons, []string{"ОК", "Отмена", "Применить", "Справка"}) {
		t.Errorf("the window is %+v", walk.Window)
	}
	if !walk.Window.FocusOnOther || !walk.Window.FocusBack {
		t.Errorf("the focus does not follow the tab opened: on «Прочее» %v, back on «Колонки» %v", walk.Window.FocusOnOther, walk.Window.FocusBack)
	}
	if !walk.Window.CaptionChooses {
		t.Error("a click on the caption of «Право» did not choose it or took its box away")
	}
	if !walk.Window.ApplyBefore || walk.Window.ApplyAfter {
		t.Errorf("«Применить» unavailable before a change: %v, after it: %v; want true, false", walk.Window.ApplyBefore, walk.Window.ApplyAfter)
	}
	if !walk.Applied.Open || slices.Contains(walk.Applied.Headings, "Поля") {
		t.Errorf("after «Применить» the window is open: %v and the list shows %v", walk.Applied.Open, walk.Applied.Headings)
	}
	kept := walk.Kept.Lists["all-restrictions"]
	hidden := []string{}
	for _, column := range kept.Columns {
		if !column.Visible {
			hidden = append(hidden, column.Key)
		}
	}
	if !slices.Equal(hidden, []string{"field"}) || kept.Sort.Column != "role" || !kept.Sort.Descending {
		t.Errorf("the server keeps %+v", kept)
	}
	if lists, err := workspace.ListSettings(); err != nil || lists["all-restrictions"] == nil {
		t.Errorf("the workspace keeps %v, %v", lists, err)
	}

	// The page before the reload is marked, so that the wait below is the
	// page loaded anew's and not the old one's.
	if _, err := browser.evaluate(ctx, page, `window.__beforeReload = true`); err != nil {
		t.Fatal(err)
	}
	if _, err := browser.call(ctx, page, "Page.reload", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	// The old page may still answer, or be gone in the middle of an
	// answer: the new one is asked again until it has drawn its tree.
	for loaded := false; !loaded; {
		if ctx.Err() != nil {
			t.Fatal("the page loaded anew never drew its tree")
		}
		answer, err := browser.evaluate(ctx, page, `!window.__beforeReload && !!document.querySelector('#tree summary, #tree .leaf') && !window.__studio.pending`)
		loaded = err == nil && string(answer) == "true"
		if !loaded {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if _, err := browser.evaluate(ctx, page, `StudioListSettings.load()`); err != nil {
		t.Fatal(err)
	}
	result, err = browser.evaluate(ctx, page, listSettingsAfterReload)
	if err != nil {
		t.Fatal(err)
	}
	var again view
	if err := json.Unmarshal(result, &again); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	if want := []string{"Объект", "Роль▼", "Право", "Ограничение доступа"}; !slices.Equal(again.Headings, want) || !slices.Equal(again.Roles, []string{"Продавец", "Кладовщик"}) {
		t.Errorf("a page loaded anew shows %v %v, want %v by role from the last", again.Headings, again.Roles, want)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}

// addSecondRestrictedRole gives the project a second role restricting the
// catalog Товары, so that the list of restrictions has two rows to order.
func addSecondRestrictedRole(t *testing.T, workspace *Workspace, root string) {
	t.Helper()
	configuration, err := metadata.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	var goods *metadata.CatalogDefinition
	for index := range configuration.Catalogs {
		if configuration.Catalogs[index].Name == "Товары" {
			goods = &configuration.Catalogs[index]
		}
	}
	if goods == nil {
		t.Fatal("the example has no catalog Товары")
	}
	created, err := workspace.CreateRole("Продавец")
	if err != nil {
		t.Fatal(err)
	}
	created.Role.Objects = []metadata.ObjectPermission{{Object: goods.ID, Operations: []metadata.PermissionOperation{metadata.PermissionRead},
		Restrictions: []metadata.RightRestriction{{Right: metadata.PermissionRead, Condition: "ГДЕ ИСТИНА"}}}}
	if _, err := workspace.SaveRole(created.Path, created.Role, created.Revision); err != nil {
		t.Fatal(err)
	}
}

// The edge of the heading of a column that changes its size is drawn by
// the mouse (help, form_customizegriddlg, «Изменение размера»): the edge
// follows the mouse though the other columns give way, the new width is
// kept for the user, and the press on the edge does not order the list.
// Defects caught: the edge lagging behind the mouse as the others shrink;
// the width lost; a drag or a press on the edge taken for a click on the
// heading.
func TestTheEdgeOfAHeadingFollowsTheMouseAndIsKept(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	var workspace *Workspace
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		workspace = opened
		opened.KeepUserSettingsIn(t.TempDir())
		addRestrictedRole(t, opened, root)
	})
	raw, err := browser.evaluate(ctx, page, `(async () => {`+openAllRestrictions+`
	  const edge = list().querySelector('.list-heading[data-column="role"] .list-resize').getBoundingClientRect();
	  return {x: edge.left + 2, y: edge.top + edge.height / 2, width: list().querySelector('.list-heading[data-column="role"]').getBoundingClientRect().width};
	})()`)
	if err != nil {
		t.Fatal(err)
	}
	var edge struct{ X, Y, Width float64 }
	if err := json.Unmarshal(raw, &edge); err != nil {
		t.Fatal(err)
	}
	// A press and release on the edge without moving is no click on the
	// heading: the list keeps its order.
	for _, kind := range []string{"mousePressed", "mouseReleased"} {
		if _, err := browser.call(ctx, page, "Input.dispatchMouseEvent", map[string]any{"type": kind, "x": edge.X, "y": edge.Y, "button": "left", "clickCount": 1}); err != nil {
			t.Fatal(err)
		}
	}
	const drawn = 120
	for _, step := range []map[string]any{
		{"type": "mousePressed", "x": edge.X, "y": edge.Y, "button": "left", "clickCount": 1},
		{"type": "mouseMoved", "x": edge.X + drawn/2, "y": edge.Y, "button": "left", "buttons": 1},
		{"type": "mouseMoved", "x": edge.X + drawn, "y": edge.Y, "button": "left", "buttons": 1},
		{"type": "mouseReleased", "x": edge.X + drawn, "y": edge.Y, "button": "left", "clickCount": 1},
	} {
		if _, err := browser.call(ctx, page, "Input.dispatchMouseEvent", step); err != nil {
			t.Fatal(err)
		}
	}
	raw, err = browser.evaluate(ctx, page, `(async () => { await window.__studio.settle();
	  const list = document.querySelector('.overview-block .settable-list');
	  return {width: list.querySelector('.list-heading[data-column="role"]').getBoundingClientRect().width,
	    sorted: [...list.querySelectorAll('.list-heading[data-sort]')].map(heading => heading.dataset.column)}; })()`)
	if err != nil {
		t.Fatal(err)
	}
	var after struct {
		Width  float64
		Sorted []string
	}
	if err := json.Unmarshal(raw, &after); err != nil {
		t.Fatal(err)
	}
	// One character of the list's font is about 7 px: the width is whole
	// characters.
	if wanted := edge.Width + drawn; after.Width < wanted-8 || after.Width > wanted+8 {
		t.Errorf("the column is %.0f px after drawing its edge from %.0f px by %d px", after.Width, edge.Width, drawn)
	}
	if !slices.Equal(after.Sorted, []string{"object"}) {
		t.Errorf("after the drag the list is ordered by %v, want still by object", after.Sorted)
	}
	lists, err := workspace.ListSettings()
	if err != nil {
		t.Fatal(err)
	}
	kept, _ := json.Marshal(lists["all-restrictions"])
	var setting struct {
		Columns []struct {
			Key   string
			Width int
		}
	}
	if err := json.Unmarshal(kept, &setting); err != nil || len(setting.Columns) < 2 || setting.Columns[1].Key != "role" || setting.Columns[1].Width <= 14 {
		t.Errorf("the user keeps %s", kept)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}

// templateListWalk opens «Настройка списка» of the list of templates and
// reads what the window gives each column; then on the list of restrictions
// hides «Поля», applies, enters 11 lines of height and closes by «Отмена».
const templateListWalk = `(async () => {` + openAllRestrictions + `
  const answer = {};
  const settings = async () => {
    [...document.querySelectorAll('.overview-block .role-bulk button')].find(button => button.textContent === 'Действия').click();
    [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent.startsWith('Настройка списка')).click();
    await studio.settle();
    return document.querySelector('dialog[open]');
  };
  const field = (dialog, label) => dialog.querySelector('[aria-label="' + label + '"]');
  const columnOf = dialog => ({width: field(dialog, 'Ширина').value, height: field(dialog, 'Высота').value,
    auto: [...dialog.querySelectorAll('.list-settings-check input')][0].checked});
  const choose = (dialog, title) => [...dialog.querySelectorAll('.list-settings-item')].find(item => item.textContent === title).querySelector('span').click();
  const press = (dialog, text) => [...dialog.querySelectorAll('.localized-title-actions button')].find(item => item.textContent === text).click();
  [...document.querySelectorAll('.overview-tab')].find(item => item.textContent === 'Шаблоны ограничений').click();
  await studio.settle();
  let dialog = await settings();
  answer.templates = {};
  for (const title of ['Роль', 'Наименование', 'Текст шаблона']) { choose(dialog, title); answer.templates[title] = columnOf(dialog); }
  press(dialog, 'Отмена');
  await studio.settle();
  [...document.querySelectorAll('.overview-tab')].find(item => item.textContent === 'Ограничения доступа').click();
  await studio.settle();
  dialog = await settings();
  choose(dialog, 'Ограничение доступа');
  const height = field(dialog, 'Высота');
  height.value = '11'; height.dispatchEvent(new Event('change', {bubbles: true}));
  answer.heightAfterEleven = height.value;
  height.value = '10'; height.dispatchEvent(new Event('change', {bubbles: true}));
  answer.heightAfterTen = height.value;
  [...dialog.querySelectorAll('.list-settings-item')].find(item => item.textContent === 'Поля').querySelector('input').click();
  press(dialog, 'Применить');
  await studio.settle();
  answer.applied = headings();
  press(dialog, 'Отмена');
  await studio.settle();
  answer.cancelled = headings();
  answer.kept = await (await fetch('/api/settings/lists')).json();
  return answer;
})()`

// The window of «Настройка списка» gives the columns of the templates what
// the prototype's does - «Роль» and «Наименование» 14 characters and a line,
// «Текст шаблона» 38 characters and up to 10 lines by its text - takes a
// height of 10 lines and not of 11, and «Отмена» after «Применить» takes the
// applied setting back from the list and from what the user keeps
// (snapshots configurator-2026-10-19/shabl-19…26; the expectations are
// written here from them). Defects caught: the columns of the templates by
// the look of shabl-01 (2 and 8 lines, 39 characters); a height above the
// prototype's limit taken; «Отмена» leaving the list as «Применить» set it,
// or the list taken back while the user's file keeps the applied setting.
func TestTheListSettingsOfTemplatesAndCancelAreThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	directory := t.TempDir()
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		opened.KeepUserSettingsIn(directory)
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, templateListWalk)
	if err != nil {
		t.Fatal(err)
	}
	type column struct {
		Width, Height string
		Auto          bool
	}
	var walk struct {
		Templates         map[string]column
		HeightAfterEleven string
		HeightAfterTen    string
		Applied           []string
		Cancelled         []string
		Kept              map[string]struct {
			Columns []struct {
				Key     string
				Visible bool
			}
		}
	}
	if err := json.Unmarshal(result, &walk); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	want := map[string]column{"Роль": {"14", "1", false}, "Наименование": {"14", "1", false}, "Текст шаблона": {"38", "10", true}}
	for title, expected := range want {
		if walk.Templates[title] != expected {
			t.Errorf("the window gives «%s» %+v, want %+v", title, walk.Templates[title], expected)
		}
	}
	if walk.HeightAfterEleven != "5" || walk.HeightAfterTen != "10" {
		t.Errorf("the height took %q after 11 and %q after 10, want 5 kept and 10 taken", walk.HeightAfterEleven, walk.HeightAfterTen)
	}
	if slices.Contains(walk.Applied, "Поля") || !slices.Contains(walk.Cancelled, "Поля") {
		t.Errorf("the list shows %v applied and %v cancelled, want «Поля» hidden and then back", walk.Applied, walk.Cancelled)
	}
	for _, item := range walk.Kept["all-restrictions"].Columns {
		if item.Key == "field" && !item.Visible {
			t.Errorf("the user keeps «Поля» hidden after «Отмена»: %+v", walk.Kept["all-restrictions"])
		}
	}
}
