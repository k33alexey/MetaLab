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

// sectionCommandsWalk opens the subsystem Склад and its window «Командный
// интерфейс» by the button of «Основные», reads the groups, the commands
// and their boxes, takes the box of the list of Товары off, presses «Скрыть
// команды, невидимые по умолчанию», reads again, presses OK and «Сохранить»
// and reads what the subsystem keeps; then it opens the window from the menu
// of the tree.
const sectionCommandsWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 120)).then(() => studio.settle());
  const host = document.querySelector('#subsystem-editor');
  const dialog = () => document.querySelector('dialog.section-commands-dialog');
  const read = () => [...dialog().querySelectorAll('.section-commands > .studio-tree-node, .section-commands > .studio-tree-row')].map(group => {
    const row = group.matches('.studio-tree-row') ? group : group.querySelector(':scope > .studio-tree-row');
    const commands = group.matches('.studio-tree-row') ? [] : [...group.querySelectorAll('.studio-tree-leaf')]
      .map(item => item.textContent + (item.querySelector('input').checked ? ' ✓' : ''));
    return [row.textContent, ...commands];
  });
  await openObject('Склад');
  const button = [...host.querySelectorAll('.object-window-button')].find(item => item.textContent === 'Командный интерфейс');
  steps.button = !!button && !button.disabled;
  button.click(); await settle();
  if (!dialog()) return {problem: JSON.stringify(studio.problems)};
  steps.title = dialog().querySelector('h3').textContent;
  steps.panel = [...dialog().querySelectorAll('.object-window-panel button')].map(item => item.title + (item.disabled ? ' (недоступна)' : ''));
  steps.first = read();
  const box = [...dialog().querySelectorAll('input[type=checkbox]')].find(item => item.getAttribute('aria-label') === 'Видимость: Список товаров');
  box.click(); await settle();
  dialog().querySelector('.object-window-panel button[title="Скрыть команды, невидимые по умолчанию"]').click(); await settle();
  steps.hidden = read();
  [...dialog().querySelectorAll('.localized-title-actions button')].find(item => item.textContent === 'OK').click(); await settle();
  steps.unsaved = !document.querySelector('#save').disabled;
  document.querySelector('#save').click(); await settle();
  const answer = await (await studioRequest('/api/subsystem?path=' + encodeURIComponent('SUBSYSTEM_PATH'), {method: 'GET'})).text();
  if (!answer.startsWith('{')) return {problem: answer, problems: studio.problems};
  const written = JSON.parse(answer);
  steps.written = written.subsystem.commandInterface;
  const node = [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.querySelector(':scope > span:not(.studio-tree-sign)')?.textContent === 'Склад');
  node.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  const item = [...document.querySelectorAll('.context-menu .context-menu-item')].find(entry => entry.textContent === 'Открыть командный интерфейс');
  steps.menu = !!item && !item.disabled;
  item.click(); await settle();
  steps.fromMenu = read();
  return steps;
})()`

// The window «Командный интерфейс» of a subsystem is the prototype's
// (podsistema.md, «Командный интерфейс…», «Досъёмка 28.10.2026»; snapshots
// series-2.603/42, 43): the button of «Основные» and the menu of the tree
// open it; the panel «Переместить команду в другую группу команд», up and
// down unavailable, «Скрыть команды, невидимые по умолчанию»; the groups in
// the prototype's order, the commands of the content - a catalog's list by
// its presentation of a list, visible, and its creation by its synonym with
// «: создать», unseen; a register of accumulation's list, unseen - with
// their boxes; a catalog of folders and items also its folder's creation
// with «: создать группу», unseen; a common form of standard commands its
// opening in «Сервис» by its synonym, visible (podsistema.md, «Досъёмка
// 28.10.2026, вечер»); a box taken off hides its command with «Скрыть…»; OK and
// «Сохранить» write the visibility that differs from the default. Defects
// caught: the window not opened from either place, a group out of the
// prototype's order, a command in the wrong group, titled or seen wrongly,
// the default written, the change lost.
func TestTheCommandInterfaceOfASubsystemIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	for path, said := range map[string]string{
		filepath.Join(root, "metadata", "catalogs", "Товары", "object.yaml"):                       "use_standard_commands: true\nlist_presentation: {ru: Список товаров}\n",
		filepath.Join(root, "metadata", "accumulation-registers", "ОстаткиТоваров", "object.yaml"): "use_standard_commands: true\n",
		filepath.Join(root, "metadata", "catalogs", "Контрагенты", "object.yaml"):                  "use_standard_commands: true\nhierarchy: {enabled: true, kind: folders-and-items}\n",
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
	stock, _ := uuid.Parse("10000000-0000-4000-8000-000000000301")
	parties, _ := uuid.Parse("10000000-0000-4000-8000-000000000102")
	productForm, _ := uuid.Parse("10000000-0000-4000-8000-000000000501")
	var path string
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		created, err := workspace.CreateSubsystem("Склад")
		path = created.Path
		if err != nil {
			t.Fatal(err)
		}
		value := created.Subsystem
		value.Members = []uuid.UUID{products, stock, parties, productForm}
		if _, err := workspace.SaveSubsystemWindow(created.Path, SubsystemWindowValue{SubsystemDefinition: value}, created.Revision); err != nil {
			t.Fatal(err)
		}
	})
	result, err := browser.evaluate(ctx, page, strings.Replace(sectionCommandsWalk, "SUBSYSTEM_PATH", path, 1))
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Button, Unsaved, Menu   bool
		Title                   string
		Panel                   []string
		First, Hidden, FromMenu [][]string
		Written                 metadata.CommandInterface
		Problem                 string
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	if steps.Problem != "" {
		t.Fatalf("the subsystem does not read: %s", steps.Problem)
	}
	expect := func(what string, got, want any) {
		t.Helper()
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("%s: %s, want %s", what, gotJSON, wantJSON)
		}
	}
	expect("opened by the button", []any{steps.Button, steps.Title}, []any{true, "Командный интерфейс"})
	expect("the panel", steps.Panel, []string{"Переместить команду в другую группу команд (недоступна)", "Переместить вверх (недоступна)",
		"Переместить вниз (недоступна)", "Установить свойства по умолчанию (недоступна)", "Скрыть команды, невидимые по умолчанию"})
	groups := func(ordinary, create, tools []string) [][]string {
		return [][]string{{"Панель навигации.Важное"}, append([]string{"Панель навигации.Обычное"}, ordinary...), {"Панель навигации.См. также"},
			append([]string{"Панель действий.Создать"}, create...), {"Панель действий.Отчеты"}, append([]string{"Панель действий.Сервис"}, tools...)}
	}
	created := []string{"Контрагенты: создать", "Контрагенты: создать группу", "Товары: создать"}
	expect("the commands", steps.First, groups([]string{"Контрагенты ✓", "Остатки товаров", "Список товаров ✓"}, created, []string{"Демо формы товара ✓"}))
	expect("hidden", steps.Hidden, groups([]string{"Контрагенты ✓"}, nil, []string{"Демо формы товара ✓"}))
	expect("unsaved", steps.Unsaved, true)
	if len(steps.Written.CommandsVisibility) != 1 {
		t.Fatalf("written: %+v", steps.Written)
	}
	written := steps.Written.CommandsVisibility[0]
	if written.Object == nil || *written.Object != products || written.Standard != metadata.StandardOpenList || written.Common || len(written.Roles) != 0 {
		t.Errorf("written: %+v", written)
	}
	expect("from the menu", []any{steps.Menu, steps.FromMenu}, []any{true, groups([]string{"Контрагенты ✓", "Остатки товаров", "Список товаров"}, created, []string{"Демо формы товара ✓"})})
	if strings.Contains(string(result), "Ошибка") {
		t.Errorf("the walk met an error: %s", result)
	}
}
