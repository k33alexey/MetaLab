package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// findInTreeWalk opens «Все подсистемы», chooses Продажи and reads the end
// of the menu of a group, then «Найти в дереве» on the command of Товары by
// the menu, on the subsection Склад by Cmd+T, on the own command of Товары,
// and what the project tree and the work area show after each.
const findInTreeWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;
  const settle = () => new Promise(resolve => setTimeout(resolve, 150)).then(() => studio.settle());
  const host = document.querySelector('#subsystem-overview');
  [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === 'Подсистемы').dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10})); await settle();
  [...document.querySelectorAll('.context-menu-item')].find(item => item.childNodes[0].textContent === 'Все подсистемы').click(); await settle();
  [...host.querySelectorAll('.subsystem-overview-tree .studio-tree-row')].find(item => item.textContent === 'Продажи').click(); await settle();
  const row = text => [...host.querySelectorAll('.subsystem-overview-commands .studio-tree-row')].find(item => item.textContent.startsWith(text));
  const menu = item => {
    item.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
    return [...document.querySelector('.context-menu').children].map(entry => entry.matches('.context-menu-separator') ? '|' : entry.textContent + (entry.disabled ? ' -' : ''));
  };
  const chosen = () => ({tree: document.querySelector('#tree .studio-tree-row.selected')?.textContent || '', window: !host.hidden});
  steps.group = menu(row('Панель навигации.Обычное')).slice(-5);
  closeContextMenu();
  steps.command = menu(row('Товары')).slice(-5);
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent.startsWith('Найти в дереве')).click(); await settle();
  steps.foundCommand = chosen();
  row('Склад').click(); await settle();
  host.querySelector('.subsystem-overview-commands .section-commands').dispatchEvent(new KeyboardEvent('keydown', {key: 't', code: 'KeyT', metaKey: true, bubbles: true}));
  await settle();
  steps.foundSubsection = chosen();
  // After Склад, so that a row found is not the one chosen before.
  menu(row('Остатки товаров'));
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent.startsWith('Найти в дереве')).click(); await settle();
  steps.foundOwn = chosen();
  return steps;
})()`

// «Найти в дереве» (Cmd+T) of the list of commands of «Все подсистемы»
// (podsistema.md, «Досъёмка 29.10.2026», В5; snapshots dosyomka-2642/14-18)
// stands before «Настройка списка...» between lines, unavailable on a
// group; on a command it chooses the object the command opens in the
// project tree - of an object's own command the object - on a subsection
// its subsystem, the window staying. Defects
// caught: the item missing, out of place or available on a group; another
// row chosen; the object opened in place of the window; Cmd+T doing
// nothing.
func TestFindInTreeOfAllSubsystemsIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	path0 := filepath.Join(root, "metadata", "catalogs", "Товары", "object.yaml")
	content, err := os.ReadFile(path0)
	if err != nil {
		t.Fatal(err)
	}
	// An own command of Товары in the navigation panel: «Найти в дереве»
	// finds Товары, not the command.
	own := "commands:\n  - id: " + uuid.MustNew().String() + "\n    name: ОстаткиТоваров\n    title: {ru: Остатки товаров}\n    group: navigation-panel-ordinary\n"
	if err := os.WriteFile(path0, []byte(string(content)+"use_standard_commands: true\n"+own), 0o644); err != nil {
		t.Fatal(err)
	}
	module, err := project.ObjectCommandModulePath("catalogs", "Товары", "ОстаткиТоваров")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(module)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, module), []byte("&НаКлиенте\nПроцедура ОбработкаКоманды(Параметр, Параметры)\nКонецПроцедуры\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	products, _ := uuid.Parse("10000000-0000-4000-8000-000000000101")
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		parent, err := workspace.CreateSubsystem("Продажи")
		if err != nil {
			t.Fatal(err)
		}
		value := parent.Subsystem
		value.Members = []uuid.UUID{products}
		if _, err := workspace.SaveSubsystemWindow(parent.Path, SubsystemWindowValue{SubsystemDefinition: value}, parent.Revision); err != nil {
			t.Fatal(err)
		}
		child, err := workspace.CreateSubsystemChild(SubsystemChild{Parent: parent.Subsystem.ID})
		if err != nil {
			t.Fatal(err)
		}
		named := child.Subsystem
		named.Name, named.Title = "Склад", metadata.LocalizedText{"ru": "Склад"}
		if _, err := workspace.SaveSubsystemWindow(child.Path, SubsystemWindowValue{SubsystemDefinition: named}, child.Revision); err != nil {
			t.Fatal(err)
		}
	})
	result, err := browser.evaluate(ctx, page, findInTreeWalk)
	if err != nil {
		t.Fatal(err)
	}
	type chosen struct {
		Tree   string
		Window bool
	}
	var steps struct {
		Group, Command                []string
		FoundCommand, FoundSubsection chosen
		FoundOwn                      chosen
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	if want := []string{"Восстановить автоматический порядок", "|", "Найти в деревеCmd+T -", "|", "Настройка списка..."}; !slices.Equal(steps.Group, want) {
		t.Errorf("the menu of a group ends %q, want %q", steps.Group, want)
	}
	if want := []string{"Восстановить автоматический порядок -", "|", "Найти в деревеCmd+T", "|", "Настройка списка..."}; !slices.Equal(steps.Command, want) {
		t.Errorf("the menu of a command ends %q, want %q", steps.Command, want)
	}
	if steps.FoundCommand != (chosen{"Товары", true}) {
		t.Errorf("«Найти в дереве» on the command chose %+v, want Товары in the tree and the window kept", steps.FoundCommand)
	}
	if steps.FoundOwn != (chosen{"Товары", true}) {
		t.Errorf("«Найти в дереве» on an own command chose %+v, want its object Товары", steps.FoundOwn)
	}
	if steps.FoundSubsection != (chosen{"Склад", true}) {
		t.Errorf("Cmd+T on the subsection chose %+v, want Склад in the tree and the window kept", steps.FoundSubsection)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
