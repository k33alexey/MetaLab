package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// subsectionInsideWalk opens «Все подсистемы», chooses Продажи and reads
// the subsection Склад in its command interface before and after opening
// it; clicks the box of a command of the subsection and of a role and reads
// them and «Сохранить» again.
const subsectionInsideWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;
  const settle = () => new Promise(resolve => setTimeout(resolve, 150)).then(() => studio.settle());
  const host = document.querySelector('#subsystem-overview');
  [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === 'Подсистемы').dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10})); await settle();
  [...document.querySelectorAll('.context-menu-item')].find(item => item.childNodes[0].textContent === 'Все подсистемы').click(); await settle();
  [...host.querySelectorAll('.subsystem-overview-tree .studio-tree-row')].find(item => item.textContent === 'Продажи').click(); await settle();
  const commands = () => host.querySelector('.subsystem-overview-commands');
  const subsection = () => [...commands().querySelectorAll('.studio-tree-row')].find(item => item.textContent === 'Склад');
  const inside = () => [...subsection().parentElement.querySelectorAll(':scope > .studio-tree-children .studio-tree-row')].map(row =>
    row.textContent + [...row.querySelectorAll('input[type=checkbox]')].map(box => (box.checked ? ' ✓' : ' ·') + (box.disabled ? '-' : '')).join('') +
    [...row.querySelectorAll('.role-visibility')].map(box => ' ' + box.getAttribute('aria-checked')).join(''));
  steps.closed = {open: subsection().parentElement.open, rows: inside().length};
  subsection().querySelector('.studio-tree-sign').click(); await settle();
  steps.opened = inside();
  const box = subsection().parentElement.querySelector(':scope > .studio-tree-children .role-visibility');
  box.click(); await settle();
  // Drawn anew: closed and opened again.
  subsection().querySelector('.studio-tree-sign').click(); await settle();
  subsection().querySelector('.studio-tree-sign').click(); await settle();
  steps.clicked = inside();
  steps.unsaved = !document.querySelector('#save').disabled;
  return steps;
})()`

// A subsection of the command interface of a subsystem in «Все
// подсистемы» opens into the groups of its own section with their
// commands, the boxes of their visibility and the columns of the roles, as
// its own command interface says them (podsistema.md, «Досъёмка
// 29.10.2026»; snapshots dosyomka-2642/03-08), read only: the prototype's
// boxes there click and lose what was clicked, an error of the platform not
// copied (В4 of series 2.642). Defects caught: the subsection not opening;
// its commands not those of its section, or their boxes not what its
// command interface says; a box there editable or marking the project
// changed.
func TestASubsectionOfAllSubsystemsOpensIntoItsGroups(t *testing.T) {
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
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		role, err := workspace.CreateRole("Р1")
		if err != nil {
			t.Fatal(err)
		}
		parent, err := workspace.CreateSubsystem("Продажи")
		if err != nil {
			t.Fatal(err)
		}
		child, err := workspace.CreateSubsystemChild(SubsystemChild{Parent: parent.Subsystem.ID})
		if err != nil {
			t.Fatal(err)
		}
		value := child.Subsystem
		value.Name, value.Title, value.Members = "Склад", metadata.LocalizedText{"ru": "Склад"}, []uuid.UUID{products}
		value.CommandInterface.CommandsVisibility = []metadata.InterfaceCommandVisibility{{
			InterfaceCommand:    metadata.InterfaceCommand{Object: &products, Standard: metadata.StandardOpenList},
			InterfaceVisibility: project.InterfaceVisibility{Common: false, Roles: []project.RoleVisibility{{Role: role.Role.ID, Visible: true}}}}}
		if _, err := workspace.SaveSubsystemWindow(child.Path, SubsystemWindowValue{SubsystemDefinition: value}, child.Revision); err != nil {
			t.Fatal(err)
		}
	})
	result, err := browser.evaluate(ctx, page, subsectionInsideWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Closed struct {
			Open bool
			Rows int
		}
		Opened, Clicked []string
		Unsaved         bool
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	if steps.Closed.Open || steps.Closed.Rows > 1 {
		t.Errorf("the subsection stands %+v before it is opened, want closed", steps.Closed)
	}
	want := []string{"Панель навигации.Важное", "Панель навигации.Обычное", "Товары ·- true", "Панель навигации.См. также",
		"Панель действий.Создать", "Товары: создать ·- mixed", "Панель действий.Отчеты", "Панель действий.Сервис"}
	expect := func(what string, got []string) {
		t.Helper()
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("%s: %s, want %s", what, gotJSON, wantJSON)
		}
	}
	expect("opened", steps.Opened)
	expect("after a click", steps.Clicked)
	if steps.Unsaved {
		t.Error("a click in the subsection marked the project changed")
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
