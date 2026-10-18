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

// treeFilterWalk drives the tree's filter by subsystems as a developer
// would: chooses «Справочники», opens the window, and after each step answers
// what the branches show, whether the button is pressed and whether «Добавить»
// is offered.
const treeFilterWalk = `(async () => {
  const studio = window.__studio, steps = {};
  const caption = item => (item.querySelector(':scope > summary') || item).textContent.trim();
  const branch = text => [...document.querySelectorAll('#tree summary, #tree .leaf')].find(item => item.textContent.trim() === text);
  const children = text => { const item = branch(text); return item?.tagName === 'SUMMARY' ? [...item.nextElementSibling.children].map(caption) : []; };
  const state = () => ({catalogs: children('Справочники'), documents: children('Документы'), subsystems: children('Подсистемы'),
    registers: children('Регистры накопления'), forms: children('Общие формы'),
    pressed: document.querySelector('#tree-subsystems').getAttribute('aria-pressed'), add: !document.querySelector('#tree-add').disabled});
  const dialog = () => document.querySelector('dialog[open]');
  const box = text => [...dialog().querySelectorAll('label')].find(label => label.textContent === text).querySelector('input');
  const press = async text => { [...dialog().querySelectorAll('button')].find(button => button.textContent === text).click(); await studio.settle(); };
  const open = async () => { document.querySelector('#tree-subsystems').click(); await studio.settle(); };
  branch('Справочники').click(); await studio.settle();
  steps.before = state();
  await open();
  steps.window = [...dialog().querySelectorAll('label')].map(label => label.textContent + '=' + label.querySelector('input').checked);
  for (const text of ['Склад', '<Не входящие в подсистемы>', 'Включать объекты подчиненных подсистем']) box(text).click();
  await press('Установить');
  steps.sales = state();
  await open();
  box('Включать объекты подчиненных подсистем').click();
  await press('Установить');
  steps.beneath = state();
  document.querySelector('#refresh').click(); await studio.settle();
  steps.reread = state();
  await open();
  await press('Отключить');
  branch('Справочники').click(); await studio.settle();
  steps.off = state();
  return steps;
})()`

// The tree of the configuration filtered by subsystems in a real browser
// (ТЗ ACCESS-RIGHTS, «Дерево конфигурации»; help, «Отбор по подсистемам»).
// Defects caught: the button opens no window, or a window not as the
// prototype's (every subsystem marked, the nested one beneath, the
// subordinate flag set); the filter leaves an object of an unmarked subsystem,
// of none, or a subsystem not marked, or hides one marked; «Включать объекты
// подчиненных подсистем» does not take in the nested subsystem and what it
// holds; «Добавить» is offered under the filter, or not offered again after
// «Отключить»; the button does not show the filter in force; reading the
// project again drops the filter.
func TestConfigurationTreeFiltersBySubsystems(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addTreeSubsystems(t, root)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, treeFilterWalk)
	if err != nil {
		t.Fatal(err)
	}
	type state struct {
		Catalogs, Documents, Subsystems, Registers, Forms []string
		Pressed                                           string
		Add                                               bool
	}
	var steps struct {
		Before, Sales, Beneath, Reread, Off state
		Window                              []string
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	nested := []string{"Нумераторы", "Последовательности"}
	whole := state{Catalogs: []string{"Контрагенты", "Товары"}, Documents: append(slices.Clone(nested), "ПоступлениеТоваров", "ПродажаТоваров"),
		Subsystems: []string{"Продажи", "Склад"}, Registers: []string{"ОстаткиТоваров"}, Forms: []string{"ДемоФормаТовара"}, Pressed: "false", Add: true}
	sales := state{Catalogs: []string{"Контрагенты"}, Documents: append(slices.Clone(nested), "ПродажаТоваров"),
		Subsystems: []string{"Продажи"}, Pressed: "true"}
	beneath := sales
	beneath.Catalogs, beneath.Subsystems = []string{"Контрагенты", "Товары"}, []string{"Продажи", "Склад"}
	for _, check := range []struct {
		name      string
		got, want state
	}{{"before the filter", steps.Before, whole}, {"under Продажи alone", steps.Sales, sales}, {"with the subsystems beneath", steps.Beneath, beneath},
		{"after reading the project again", steps.Reread, beneath}, {"after «Отключить»", steps.Off, whole}} {
		got, want := check.got, check.want
		if !slices.Equal(got.Catalogs, want.Catalogs) || !slices.Equal(got.Documents, want.Documents) || !slices.Equal(got.Subsystems, want.Subsystems) ||
			!slices.Equal(got.Registers, want.Registers) || !slices.Equal(got.Forms, want.Forms) || got.Pressed != want.Pressed || got.Add != want.Add {
			t.Errorf("%s the tree shows %+v, want %+v", check.name, got, want)
		}
	}
	window := []string{"Продажи=true", "Склад=true", "<Не входящие в подсистемы>=true", "Включать объекты подчиненных подсистем=true", "Включать объекты родительских подсистем=false"}
	if !slices.Equal(steps.Window, window) {
		t.Errorf("the window opens as %v, want %v", steps.Window, window)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
