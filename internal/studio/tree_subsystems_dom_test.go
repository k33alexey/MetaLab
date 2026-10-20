package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The ids of the example's objects and of the two subsystems the tests give
// it: Продажи holds Контрагенты and ПродажаТоваров, Склад lies in Продажи and
// holds Товары; the register ОстаткиТоваров and the common form lie in none.
const (
	treeSalesSubsystem     = "30000000-0000-4000-8000-000000000301"
	treeWarehouseSubsystem = "30000000-0000-4000-8000-000000000302"
)

func addTreeSubsystems(t *testing.T, root string) {
	t.Helper()
	directory := filepath.Join(root, "metadata", "subsystems")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	for id, body := range map[string]string{
		treeSalesSubsystem: "format: 1\nid: " + treeSalesSubsystem + "\nname: Продажи\ntitle: {ru: Продажи}\nmembers:\n  - 10000000-0000-4000-8000-000000000102\n  - 10000000-0000-4000-8000-000000000202\n",
		treeWarehouseSubsystem: "format: 1\nid: " + treeWarehouseSubsystem + "\nname: Склад\ntitle: {ru: Склад}\nparent: " + treeSalesSubsystem +
			"\nmembers:\n  - 10000000-0000-4000-8000-000000000101\n",
	} {
		if err := os.WriteFile(filepath.Join(directory, id+".yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// The snapshot of the tree carries what its filter by subsystems selects by:
// the subsystems, the one each lies in, and what each holds. Defects caught:
// the filter has nothing to choose from or misses a subsystem's members; a
// nested subsystem loses its parent, and «Включать объекты подчиненных
// подсистем» no longer reaches it; the filter offered over a project whose
// descriptions do not read, where membership is unknown.
func TestSnapshotCarriesTheFilterBySubsystems(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addTreeSubsystems(t, root)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Subsystems == nil {
		t.Fatal("the snapshot of a project that reads offers no filter by subsystems")
	}
	sales, err := uuid.Parse(treeSalesSubsystem)
	if err != nil {
		t.Fatal(err)
	}
	warehouse, err := uuid.Parse(treeWarehouseSubsystem)
	if err != nil {
		t.Fatal(err)
	}
	members := snapshot.Subsystems.Subsystems
	if got := members[sales]; len(got) != 2 || got[0].String() != "10000000-0000-4000-8000-000000000102" || got[1].String() != "10000000-0000-4000-8000-000000000202" {
		t.Errorf("Продажи holds %v", got)
	}
	if got := members[warehouse]; len(got) != 1 || got[0].String() != "10000000-0000-4000-8000-000000000101" {
		t.Errorf("Склад holds %v", got)
	}
	var names []string
	for _, item := range snapshot.Subsystems.Objects {
		names = append(names, item.Name)
		if item.ID == warehouse && (item.Owner == nil || *item.Owner != sales) {
			t.Errorf("Склад lies in %v, want Продажи", item.Owner)
		}
		if item.ID == sales && item.Owner != nil {
			t.Errorf("Продажи lies in %v, want nothing", item.Owner)
		}
	}
	if !slices.Equal(names, []string{"Продажи", "Склад"}) {
		t.Errorf("the filter knows the objects %v, want the two subsystems", names)
	}
	// A description that does not read: the tree still opens, the filter is
	// not offered.
	if err := os.WriteFile(filepath.Join(root, "metadata", "subsystems", treeWarehouseSubsystem+".yaml"),
		[]byte("format: 1\nid: "+treeWarehouseSubsystem+"\nname: Склад\ntitle: {ru: Склад}\nparent: 30000000-0000-4000-8000-000000000399\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot, err = workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Subsystems != nil {
		t.Error("the filter by subsystems is offered over a project whose descriptions do not read")
	}
}

// A subsystem that lies in another stands under it in the tree, as in the
// prototype's (inventory of block 00, «Подсистема», snapshot
// 20-vlozhennaya-sklad), and the branch counts it among its objects. One
// whose parent is not in the project, or whose parents come back to itself,
// stays at the top, so the tree still shows what the loader refuses. Defects
// caught: a nested subsystem listed beside its parent; the count of the
// branch losing the nested ones; a subsystem with a broken chain of parents
// vanishing from the tree or the tree looping on it.
func TestNestedSubsystemStandsUnderItsParent(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addTreeSubsystems(t, root)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	branch := func() Node {
		t.Helper()
		snapshot, err := workspace.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		node, ok := findNodeByID(snapshot.Tree, "metadata/subsystems")
		if !ok {
			t.Fatal("the tree has no branch of subsystems")
		}
		return node
	}
	titles := func(nodes []Node) []string {
		var result []string
		for _, node := range nodes {
			result = append(result, node.Title)
		}
		return result
	}
	subsystems := branch()
	if got := titles(subsystems.Children); !slices.Equal(got, []string{"Продажи"}) {
		t.Fatalf("the branch shows %v at its top, want Продажи alone", got)
	}
	if got := titles(subsystems.Children[0].Children); !slices.Equal(got, []string{"Склад"}) {
		t.Errorf("Продажи holds %v in the tree, want Склад", got)
	}
	if got := subsystems.Properties[1]; got.Name != "Объектов" || got.Value != "2" {
		t.Errorf("the branch counts %v, want both subsystems", got)
	}
	write := func(id, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, "metadata", "subsystems", id+".yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Склад under a parent the project has not: at the top.
	write(treeWarehouseSubsystem, "format: 1\nid: "+treeWarehouseSubsystem+"\nname: Склад\ntitle: {ru: Склад}\nparent: 30000000-0000-4000-8000-000000000399\n")
	if got := titles(branch().Children); !slices.Equal(got, []string{"Продажи", "Склад"}) {
		t.Errorf("with its parent missing the branch shows %v, want both at the top", got)
	}
	// Продажи and Склад each the parent of the other: both at the top.
	write(treeWarehouseSubsystem, "format: 1\nid: "+treeWarehouseSubsystem+"\nname: Склад\ntitle: {ru: Склад}\nparent: "+treeSalesSubsystem+"\n")
	write(treeSalesSubsystem, "format: 1\nid: "+treeSalesSubsystem+"\nname: Продажи\ntitle: {ru: Продажи}\nparent: "+treeWarehouseSubsystem+"\n")
	subsystems = branch()
	if got := titles(subsystems.Children); !slices.Equal(got, []string{"Продажи", "Склад"}) || len(subsystems.Children[0].Children)+len(subsystems.Children[1].Children) != 0 {
		t.Errorf("with a ring of parents the branch shows %v, want both at the top and nothing under them", got)
	}
}

// treeFilterWalk drives the tree's filter by subsystems as a developer
// would: chooses «Справочники», opens the window, and after each step answers
// what the branches show, whether the button is pressed and whether «Добавить»
// is offered.
const treeFilterWalk = `(async () => {
  const studio = window.__studio, steps = {};
  const caption = item => (item.querySelector(':scope > summary') || item).textContent.trim();
  const branch = text => [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === text);
  // A branch the tree does not show at all answers null, one shown empty [].
  const children = text => { const item = branch(text); return !item ? null : item.tagName === 'SUMMARY' ? [...item.nextElementSibling.children].map(caption) : []; };
  const state = () => ({catalogs: children('Справочники'), documents: children('Документы'), subsystems: children('Подсистемы'), sales: children('Продажи'),
    registers: children('Регистры накопления'), forms: children('Общие формы'), constants: children('Константы'),
    pressed: document.querySelector('#tree-subsystems').getAttribute('aria-pressed'), add: !document.querySelector('#tree-add').disabled});
  const dialog = () => document.querySelector('dialog[open]');
  const box = text => [...dialog().querySelectorAll('label')].find(label => label.textContent === text).querySelector('input');
  const press = async text => { [...dialog().querySelectorAll('button')].find(button => button.textContent === text).click(); await studio.settle(); };
  const open = async () => { document.querySelector('#tree-subsystems').click(); await studio.settle(); };
  branch('Справочники').click(); await studio.settle();
  steps.before = state();
  await open();
  steps.window = [...dialog().querySelectorAll('label')].map(label => { const input = label.querySelector('input'); return label.textContent + '=' + (input.checked ? 'x' : input.indeterminate ? '~' : '-'); });
  for (const text of ['Продажи', 'Включать объекты подчиненных подсистем']) box(text).click();
  await press('Установить');
  steps.sales = state();
  await open();
  box('Включать объекты подчиненных подсистем').click();
  await press('Установить');
  steps.beneath = state();
  document.querySelector('#refresh').click(); await studio.settle();
  steps.reread = state();
  // Every mark taken away, «Установить» drops the filter in force, as
  // «Отключить» does (configurator-2026-10-20/otbor-09, 10).
  await open();
  box('ПродажиИСклад').click(); box('ПродажиИСклад').click();
  await press('Установить');
  branch('Справочники').click(); await studio.settle();
  steps.emptied = state();
  await open();
  box('Продажи').click();
  await press('Установить');
  await open();
  await press('Отключить');
  branch('Справочники').click(); await studio.settle();
  steps.off = state();
  return steps;
})()`

// The tree of the configuration filtered by subsystems in a real browser
// (ТЗ ACCESS-RIGHTS, «Дерево конфигурации»; help, «Отбор по подсистемам»).
// Defects caught: the button opens no window, or a window not as the
// prototype's (the root marked, every subsystem beneath it in the third
// state, the nested one beneath, the subordinate flag set); the filter leaves an object of an unmarked subsystem,
// of none, or a subsystem not marked, or hides one marked; «Включать объекты
// подчиненных подсистем» does not take in the nested subsystem and what it
// holds; the nested subsystem, which stands under its parent, left in the
// tree under its parent when the filter does not take it in; a branch of a kind left with no object still shown under the
// filter (the owner's decision of 18.10.2026: it is not), or a branch empty
// of itself hidden without one; «Установить» with every mark taken away
// leaving the filter in force; «Добавить» is offered under the filter, or
// not offered again after «Отключить»; the button does not show the filter
// in force; reading the project again drops the filter.
func TestConfigurationTreeFiltersBySubsystems(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addTreeSubsystems(t, root)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, treeFilterWalk)
	if err != nil {
		t.Fatal(err)
	}
	type state struct {
		Catalogs, Documents, Subsystems, Sales, Registers, Forms, Constants []string
		Pressed                                                             string
		Add                                                                 bool
	}
	var steps struct {
		Before, Sales, Beneath, Reread, Emptied, Off state
		Window                                       []string
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	// nil is a branch not shown, an empty list one shown with nothing in it:
	// the example has no constant, numerator or sequence, and without the
	// filter their branches stand empty; under it, a branch with nothing
	// left - those, the register, the common forms - is not shown.
	nested := []string{"Нумераторы", "Последовательности"}
	whole := state{Catalogs: []string{"Контрагенты", "Товары"}, Documents: append(slices.Clone(nested), "ПоступлениеТоваров", "ПродажаТоваров"),
		Subsystems: []string{"Продажи"}, Sales: []string{"Склад"}, Registers: []string{"ОстаткиТоваров"}, Forms: []string{"ДемоФормаТовара"}, Constants: []string{},
		Pressed: "false", Add: true}
	sales := state{Catalogs: []string{"Контрагенты"}, Documents: []string{"ПродажаТоваров"}, Subsystems: []string{"Продажи"}, Sales: []string{}, Pressed: "true"}
	beneath := sales
	beneath.Catalogs, beneath.Sales = []string{"Контрагенты", "Товары"}, []string{"Склад"}
	for _, check := range []struct {
		name      string
		got, want state
	}{{"before the filter", steps.Before, whole}, {"under Продажи alone", steps.Sales, sales}, {"with the subsystems beneath", steps.Beneath, beneath},
		{"after reading the project again", steps.Reread, beneath}, {"after «Установить» with nothing marked", steps.Emptied, whole},
		{"after «Отключить»", steps.Off, whole}} {
		got, want := check.got, check.want
		same := func(a, b []string) bool { return (a == nil) == (b == nil) && slices.Equal(a, b) }
		if !same(got.Catalogs, want.Catalogs) || !same(got.Documents, want.Documents) || !same(got.Subsystems, want.Subsystems) || !same(got.Sales, want.Sales) ||
			!same(got.Registers, want.Registers) || !same(got.Forms, want.Forms) || !same(got.Constants, want.Constants) ||
			got.Pressed != want.Pressed || got.Add != want.Add {
			t.Errorf("%s the tree shows %+v, want %+v", check.name, got, want)
		}
	}
	window := []string{"ПродажиИСклад=x", "Продажи=~", "Склад=~", "<Не входящие в подсистемы>=~", "Включать объекты подчиненных подсистем=x", "Включать объекты родительских подсистем=-"}
	if !slices.Equal(steps.Window, window) {
		t.Errorf("the window opens as %v, want %v", steps.Window, window)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}

// subsystemFilterWindow opens the window of the filter from the tree of the
// configuration and answers, after each click, every row of its tree - the
// caption, the depth, the box checked, in the third state or not, whether
// the row folds and is shown - then the buttons and whether they stand to the
// right of the tree; then the same window of «Снять по подсистемам...» of the
// role Кладовщик, with the place of each button in the column.
const subsystemFilterWindow = `(async () => {
  const studio = window.__studio, steps = {};
  const dialog = () => document.querySelector('dialog[open]');
  const rows = () => [...dialog().querySelectorAll('.role-subsystem-tree .role-fields-row')].map(row => {
    const input = row.querySelector('input');
    return [row.querySelector('label').textContent, row.style.getPropertyValue('--depth'), input.checked ? 'x' : input.indeterminate ? '~' : '-',
      row.querySelector('button.role-fields-toggle') ? 'fold' : '', row.offsetParent ? 'shown' : 'hidden'].join(' ');
  });
  const row = text => [...dialog().querySelectorAll('.role-subsystem-tree .role-fields-row')].find(item => item.querySelector('label').textContent === text);
  const click = async (text, step) => { row(text).querySelector('input').click(); await studio.settle(); steps[step] = rows(); };
  const buttons = () => [...dialog().querySelectorAll('.localized-title-actions > *')];
  const column = () => buttons().map(item => ({text: item.textContent, top: item.getBoundingClientRect().top}));
  document.querySelector('#tree-subsystems').click(); await studio.settle();
  steps.opened = rows();
  steps.buttons = buttons().map(item => item.textContent);
  steps.beside = dialog().querySelector('.localized-title-actions').getBoundingClientRect().left >= dialog().querySelector('.role-subsystem-tree').getBoundingClientRect().right;
  steps.flags = [...dialog().querySelectorAll('.role-subsystem-flag')].map(label => label.querySelector('input').getBoundingClientRect().left > label.querySelector('span').getBoundingClientRect().right);
  await click('Склад', 'nestedAlone');
  await click('Склад', 'nestedOff');
  await click('Продажи', 'parent');
  await click('<Не входящие в подсистемы>', 'parentAndNone');
  await click('Склад', 'alone');
  await click('Продажи', 'parentOver');
  await click('ПродажиИСклад', 'rootMarked');
  await click('ПродажиИСклад', 'rootOff');
  row('Продажи').querySelector('button').click(); await studio.settle();
  steps.folded = rows();
  buttons().find(item => item.textContent === 'Установить').click(); await studio.settle();
  steps.emptySet = {open: !!dialog(), pressed: document.querySelector('#tree-subsystems').getAttribute('aria-pressed')};
  [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === 'Кладовщик').click(); await studio.settle();
  [...document.querySelectorAll('#role-editor button')].find(item => item.textContent === 'Действия').click();
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent === 'Снять по подсистемам...').click();
  await studio.settle();
  steps.apply = column();
  // Every mark taken away, «Снять» takes no right away and leaves the role
  // as it was (configurator-2026-10-20/otbor-11, 12).
  const rights = () => [...document.querySelectorAll('#role-editor input[type=checkbox]')].filter(input => input.checked).length;
  steps.applyRoot = row('ПродажиИСклад').querySelector('input').checked;
  row('ПродажиИСклад').querySelector('input').click(); await studio.settle();
  const before = rights();
  buttons().find(item => item.textContent === 'Снять').click(); await studio.settle();
  steps.applyEmpty = {open: !!dialog(), changed: !document.querySelector('#save').disabled, status: document.querySelector('#status').textContent, same: rights() === before};
  return steps;
})()`

// The window of the filter by subsystems is the prototype's (snapshots
// obolochka/32, derevo/21, configurator-2026-10-14/rol-snyat-po-podsistemam-
// okno, derevo/24, 27, 28; help, form_subsystemchoicedlg; the owner's
// decision of 19.10.2026 on the root; the clicks shot again on 20.10.2026,
// configurator-2026-10-20/otbor-01…12). Defects caught: no row of the
// configuration at the root, or the subsystems not beneath it, the nested one
// not beneath its parent or the parent not folding; the window opened
// without a filter with every row marked rather than the root alone and the
// rows beneath in the third state; a row under a marked one - the root or a
// parent - shown empty rather than in the third state, or marked by it; a
// parent left unmarked with a nested one marked shown empty; a click at a
// row not marked that leaves the mark above it (the parent's) or beneath it
// (the nested one's), or takes away a mark of another branch (the row of
// none); a click at a marked row that does not take its mark away; a click
// at the root that marks the rows beneath rather than itself alone, or at
// the root marked that leaves any mark; folding that leaves the nested row
// shown; «Установить» with nothing marked refused, or setting a filter; the
// buttons in a row under the tree, or the flags' boxes before their words;
// «Снять по подсистемам...» closing up the place of «Отключить», or, with
// every mark taken away, refusing, staying open or taking rights away.
func TestTheWindowOfTheFilterBySubsystemsIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addTreeSubsystems(t, root)
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) { addRestrictedRole(t, workspace, root) })
	result, err := browser.evaluate(ctx, page, subsystemFilterWindow)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Opened, NestedAlone, NestedOff, Parent, ParentAndNone, Alone, ParentOver, RootMarked, RootOff, Folded []string
		EmptySet                                                                                              struct {
			Open    bool
			Pressed string
		}
		Buttons []string
		Beside  bool
		Flags   []bool
		Apply   []struct {
			Text string
			Top  float64
		}
		ApplyRoot  bool
		ApplyEmpty struct {
			Open, Changed, Same bool
			Status              string
		}
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	tree := func(root, sales, warehouse, none string, shown string) []string {
		return []string{"ПродажиИСклад 0 " + root + "  shown", "Продажи 1 " + sales + " fold shown", "Склад 2 " + warehouse + "  " + shown, "<Не входящие в подсистемы> 1 " + none + "  shown"}
	}
	for _, check := range []struct {
		name      string
		got, want []string
	}{
		{"as opened", steps.Opened, tree("x", "~", "~", "~", "shown")},
		{"after a click at the nested one under the root", steps.NestedAlone, tree("~", "~", "x", "-", "shown")},
		{"after a click at the nested one marked", steps.NestedOff, tree("-", "-", "-", "-", "shown")},
		{"with the parent marked", steps.Parent, tree("~", "x", "~", "-", "shown")},
		{"with the parent and the row of none marked", steps.ParentAndNone, tree("~", "x", "~", "x", "shown")},
		{"after a click at the nested one under the parent", steps.Alone, tree("~", "~", "x", "x", "shown")},
		{"after a click at the parent over the nested one", steps.ParentOver, tree("~", "x", "~", "x", "shown")},
		{"after a click at the root in part", steps.RootMarked, tree("x", "~", "~", "~", "shown")},
		{"after a click at the root marked", steps.RootOff, tree("-", "-", "-", "-", "shown")},
		{"with the parent folded", steps.Folded, tree("-", "-", "-", "-", "hidden")},
	} {
		if !slices.Equal(check.got, check.want) {
			t.Errorf("%s the window shows %q, want %q", check.name, check.got, check.want)
		}
	}
	if steps.EmptySet.Open || steps.EmptySet.Pressed != "false" {
		t.Errorf("«Установить» with nothing marked leaves the window open %v, the filter pressed %q; want it closed with no filter", steps.EmptySet.Open, steps.EmptySet.Pressed)
	}
	if !slices.Equal(steps.Buttons, []string{"Установить", "Отключить", "Отмена", "Справка"}) || !steps.Beside {
		t.Errorf("the buttons are %v, beside the tree: %v; want Установить, Отключить, Отмена, Справка to the right", steps.Buttons, steps.Beside)
	}
	if !slices.Equal(steps.Flags, []bool{true, true}) {
		t.Errorf("the flags' boxes stand after their words: %v, want both", steps.Flags)
	}
	// The place of «Отключить» stays empty: «Отмена» stands lower under
	// «Снять» than «Справка» under «Отмена».
	apply := steps.Apply
	if len(apply) != 4 || apply[0].Text != "Снять" || apply[1].Text != "" || apply[2].Text != "Отмена" || apply[3].Text != "Справка" ||
		apply[2].Top-apply[0].Top < 1.5*(apply[3].Top-apply[2].Top) {
		t.Errorf("«Снять по подсистемам...» shows its buttons as %+v, want Снять, an empty place, Отмена, Справка", apply)
	}
	if empty := steps.ApplyEmpty; !steps.ApplyRoot || empty.Open || empty.Changed || !empty.Same {
		t.Errorf("«Снять по подсистемам...» opened with the root marked %v; with nothing marked it left the window open %v, the role changed %v (%q), its rights as they were %v; want closed, unchanged",
			steps.ApplyRoot, empty.Open, empty.Changed, empty.Status, empty.Same)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}

// treePathWalk filters the tree by Склад alone, which lies in Продажи beside
// Доставка (which holds ПоступлениеТоваров), then takes in its parents too, and answers what the branches show
// each time.
const treePathWalk = `(async () => {
  const studio = window.__studio, steps = {};
  const caption = item => (item.querySelector(':scope > summary') || item).textContent.trim();
  const branch = text => [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === text);
  const children = text => { const item = branch(text); return !item ? null : item.tagName === 'SUMMARY' ? [...item.nextElementSibling.children].map(caption) : []; };
  const state = () => ({catalogs: children('Справочники'), documents: children('Документы'), subsystems: children('Подсистемы'), sales: children('Продажи')});
  const dialog = () => document.querySelector('dialog[open]');
  const box = text => [...dialog().querySelectorAll('label')].find(label => label.textContent === text).querySelector('input');
  const press = async text => { [...dialog().querySelectorAll('button')].find(button => button.textContent === text).click(); await studio.settle(); };
  const open = async () => { document.querySelector('#tree-subsystems').click(); await studio.settle(); };
  await open();
  box('Склад').click();
  await press('Установить');
  steps.nested = state();
  await open();
  box('Включать объекты родительских подсистем').click();
  await press('Установить');
  steps.parents = state();
  return steps;
})()`

// The tree of the configuration under a filter by a nested subsystem
// (inventory of block 00, «Отбор по подсистемам», snapshots derevo/22-23).
// Defects caught: the parent of the marked subsystem hidden, and with it the
// marked one, which stands under it; the parent shown with what it holds
// though only the nested one is marked; a subsystem beside the marked one
// shown as a path it is not; with «Включать объекты родительских подсистем»
// the parent shown without the subsystems beneath it, or with what the one
// beside holds.
func TestConfigurationTreeShowsThePathToAMarkedSubsystem(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addTreeSubsystems(t, root)
	const delivery = "30000000-0000-4000-8000-000000000303"
	if err := os.WriteFile(filepath.Join(root, "metadata", "subsystems", delivery+".yaml"),
		[]byte("format: 1\nid: "+delivery+"\nname: Доставка\ntitle: {ru: Доставка}\nparent: "+treeSalesSubsystem+"\nmembers:\n  - 10000000-0000-4000-8000-000000000201\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, treePathWalk)
	if err != nil {
		t.Fatal(err)
	}
	type state struct{ Catalogs, Documents, Subsystems, Sales []string }
	var steps struct{ Nested, Parents state }
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	same := func(a, b []string) bool { return (a == nil) == (b == nil) && slices.Equal(a, b) }
	for _, check := range []struct {
		name      string
		got, want state
	}{
		// Склад alone: Продажи is the path to it, Контрагенты and
		// ПродажаТоваров, which Продажи holds, are not shown, nor Доставка.
		{"under Склад alone", steps.Nested, state{Catalogs: []string{"Товары"}, Subsystems: []string{"Продажи"}, Sales: []string{"Склад"}}},
		// With the parents: Продажи with what it holds and both beneath it,
		// but not ПоступлениеТоваров, which Доставка holds.
		{"with the parents", steps.Parents, state{Catalogs: []string{"Контрагенты", "Товары"}, Documents: []string{"ПродажаТоваров"},
			Subsystems: []string{"Продажи"}, Sales: []string{"Доставка", "Склад"}}},
	} {
		if !same(check.got.Catalogs, check.want.Catalogs) || !same(check.got.Documents, check.want.Documents) ||
			!same(check.got.Subsystems, check.want.Subsystems) || !same(check.got.Sales, check.want.Sales) {
			t.Errorf("%s the tree shows %+v, want %+v", check.name, check.got, check.want)
		}
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
