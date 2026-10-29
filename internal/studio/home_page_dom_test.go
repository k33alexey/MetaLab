package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// homePageWalk opens «Открыть рабочую область начальной страницы» of the
// menu of the root and goes through the window: adds the list form of Товары
// and the common form, adds one more and cancels it, copies the common form,
// moves the copy up, types the heights, makes the first form different for
// the role Первая, carries a form to the right, switches to one column and
// back, chooses «Снизу», deletes the last row of the left and saves.
const homePageWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;
  const settle = () => new Promise(resolve => setTimeout(resolve, 120)).then(() => studio.settle());
  await settle();
  const root = document.querySelector('#tree summary');
  root.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10})); await settle();
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent.includes('Открыть рабочую область начальной страницы')).click(); await settle();
  const host = document.querySelector('#home-page');
  const column = side => host.querySelector('.home-page-' + side);
  const rows = side => column(side) ? [...column(side).querySelectorAll('.home-page-row')].map(row => [row.querySelector('.home-page-form').textContent, row.querySelector('.home-page-height').value,
    row.querySelector('input[type=checkbox]').checked, row.querySelector('.home-page-roles').textContent, row.classList.contains('current')]) : null;
  const button = (side, title) => [...column(side).querySelectorAll('.object-window-panel button')].find(item => item.title === title);
  const dialog = () => document.querySelector('dialog[open]');
  const press = text => [...dialog().querySelectorAll('button')].find(item => item.textContent === text).click();
  const choose = async name => { [...dialog().querySelectorAll('.studio-tree-row')].find(row => row.textContent === name).click(); await settle(); press('OK'); await settle(); };
  steps.title = document.querySelector('#file-path').textContent;
  steps.captions = [...host.querySelectorAll('.home-page-caption')].map(item => item.textContent);
  steps.buttons = [...column('left').querySelectorAll('.object-window-panel button')].map(item => item.title + (item.disabled ? ' -' : ''));
  steps.head = [...column('left').querySelectorAll('.home-page-head span')].map(item => item.textContent);
  steps.fields = [...host.querySelectorAll('.home-page-field')].map(field => field.querySelector('span').textContent + ' ' + field.querySelector('select').selectedOptions[0].textContent);
  steps.templates = [...host.querySelectorAll('.home-page-field select')[1].options].map(item => item.textContent);
  steps.places = [...host.querySelectorAll('.home-page-field select')[0].options].map(item => item.textContent);
  button('left', 'Добавить новый элемент').click(); await settle();
  steps.choice = dialog().querySelector('h3').textContent;
  steps.added = rows('left');
  steps.branches = [...dialog().querySelectorAll('.object-choice-tree .studio-tree-row')].map(row => row.textContent);
  [...dialog().querySelectorAll('.studio-tree-row')].find(row => row.textContent === 'Товары').click(); await settle();
  await choose('ФормаСписка');
  steps.focus = document.activeElement.className;
  button('left', 'Добавить новый элемент').click(); await settle();
  await choose('ДемоФормаТовара');
  button('left', 'Добавить новый элемент').click(); await settle();
  press('Отмена'); await settle();
  steps.cancelled = rows('left');
  button('left', 'Добавить новый элемент копированием текущего').click(); await settle();
  steps.copyChosen = dialog().querySelector('.studio-tree-row.selected')?.textContent;
  press('OK'); await settle();
  button('left', 'Переместить текущий элемент вверх').click(); await settle();
  steps.copied = rows('left');
  const height = at => column('left').querySelectorAll('.home-page-height')[at];
  const type = (input, text) => { input.value = text; input.dispatchEvent(new Event('input', {bubbles: true})); input.dispatchEvent(new Event('change', {bubbles: true})); };
  type(height(0), '0'); type(height(1), '1234567'); type(height(2), '4x2');
  await settle();
  steps.heights = rows('left').map(row => row[1]);
  column('left').querySelector('.home-page-roles').click(); await settle();
  steps.settings = [dialog().querySelector('h3').textContent, ...[...dialog().querySelectorAll('.visibility-settings-role')].map(row => row.textContent + ' ' + row.querySelector('.role-visibility').dataset.state)];
  dialog().querySelector('.role-visibility[aria-label="Видимость: Первая"]').click(); await settle();
  steps.turned = dialog().querySelector('.role-visibility[aria-label="Видимость: Первая"]').dataset.state;
  press('OK'); await settle();
  steps.different = rows('left')[0][3];
  column('left').querySelectorAll('.home-page-row')[1].dispatchEvent(new MouseEvent('mousedown', {bubbles: true})); await settle();
  [...host.querySelectorAll('.home-page-moves button')].find(item => item.textContent === '>').click(); await settle();
  steps.carriedLeft = rows('left'); steps.carriedRight = rows('right');
  const template = host.querySelectorAll('.home-page-field select')[1];
  template.value = 'one-column'; template.dispatchEvent(new Event('change')); await settle();
  steps.oneColumn = [host.querySelectorAll('.home-page-column').length, host.querySelectorAll('.home-page-caption').length, !!host.querySelector('.home-page-moves')];
  const again = host.querySelectorAll('.home-page-field select')[1];
  again.value = 'two-columns-variable-width'; again.dispatchEvent(new Event('change')); await settle();
  steps.back = rows('right');
  const place = host.querySelectorAll('.home-page-field select')[0];
  place.value = 'bottom'; place.dispatchEvent(new Event('change')); await settle();
  column('left').querySelectorAll('.home-page-row')[1].dispatchEvent(new MouseEvent('mousedown', {bubbles: true})); await settle();
  button('left', 'Удалить текущий элемент').click(); await settle();
  steps.deleted = rows('left');
  steps.dirty = !document.querySelector('#save').disabled;
  document.querySelector('#save').click(); await settle();
  steps.saved = document.querySelector('#save').disabled;
  steps.status = document.querySelector('#status').textContent;
  return steps;
})()`

// The window «Рабочая область начальной страницы» is the prototype's
// (konfiguraciya.md, «Окна», «Досъёмка 27.10.2026»; snapshots
// konfiguraciya/18-20, series-2.595/41-49): opened from the menu of the
// root on a project with no page - two columns of equal width; the panel and
// the head of a column; «Добавить» - a row at the end, of the height 10,
// visible, «Одинаковая», and «Выбор управляемой формы» from «Общие» down to
// the forms of an object; «Отмена» takes the row away; «Скопировать» opens
// the choice on the form copied and keeps a form twice; up; the height of
// six digits at most, 0 empty; «Настройка видимости» with the roles, a
// click going round, the link «Различная» after it; «>» to the end of the
// right, the next row of the left current; one column hides the right and
// keeps its forms; «Удалить» makes the row before current; «Сохранить»
// writes the file of the page. Defects caught: a row added elsewhere or of
// other defaults; the choice not cancelled; the copy not offered its form;
// a height not limited or 0 written; the role's box not going round or the
// link not changing; the carry or the deletion making another row current;
// the right column lost; the page not written or written otherwise.
func TestTheHomePageWindowIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	common := filepath.Join(root, "metadata", "common-forms", "ДемоФормаТовара", "form.yaml")
	content, err := os.ReadFile(common)
	if err != nil {
		t.Fatal(err)
	}
	const listForm = "10000000-0000-4000-8000-000000000601"
	form := strings.Replace(strings.Replace(string(content), "10000000-0000-4000-8000-000000000501", listForm, 1), "name: ДемоФормаТовара", "name: ФормаСписка", 1)
	directory := filepath.Join(root, "metadata", "catalogs", "Товары", "forms", "ФормаСписка")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "form.yaml"), []byte(form), 0o644); err != nil {
		t.Fatal(err)
	}
	roles := map[string]uuid.UUID{}
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		for _, made := range [][2]string{{"Р1", "Первая"}, {"Р2", "Вторая"}} {
			role, err := workspace.CreateRole(made[0])
			if err != nil {
				t.Fatal(err)
			}
			role.Role.Title = metadata.LocalizedText{"ru": made[1]}
			if _, err := workspace.SaveRole(role.Path, role.Role, role.Revision); err != nil {
				t.Fatal(err)
			}
			roles[made[1]] = role.Role.ID
		}
	})
	result, err := browser.evaluate(ctx, page, homePageWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Title, Choice, Focus, CopyChosen, Turned, Different, Status        string
		Captions, Buttons, Head, Fields, Templates, Places, Branches       []string
		Heights, Settings                                                  []string
		Added, Cancelled, Copied, CarriedLeft, CarriedRight, Back, Deleted [][]any
		OneColumn                                                          []any
		Dirty, Saved                                                       bool
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	expect := func(what string, got, want any) {
		t.Helper()
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("%s: %s, want %s", what, gotJSON, wantJSON)
		}
	}
	const (
		list   = "Справочник.Товары.Форма.ФормаСписка"
		demo   = "ОбщаяФорма.ДемоФормаТовара"
		same   = "Одинаковая"
		differ = "Различная"
	)
	expect("the title", steps.Title, "Рабочая область начальной страницы")
	expect("the captions", steps.Captions, []string{"Левая колонка:", "Правая колонка:"})
	expect("the panel", steps.Buttons, []string{"Добавить новый элемент", "Добавить новый элемент копированием текущего -", "Удалить текущий элемент -",
		"Переместить текущий элемент вверх -", "Переместить текущий элемент вниз -"})
	expect("the head", steps.Head, []string{"Форма", "Высота", "Видимость"})
	expect("the fields", steps.Fields, []string{"Отображение командного интерфейса основного раздела: Не использовать",
		"Шаблон начальной страницы: Две колонки одинаковой ширины"})
	expect("the templates", steps.Templates, []string{"Одна колонка", "Две колонки одинаковой ширины", "Две колонки разной ширины (2:1)"})
	expect("the places", steps.Places, []string{"Сверху", "Снизу", "Не использовать"})
	expect("the choice", steps.Choice, "Выбор управляемой формы")
	expect("the row added", steps.Added, [][]any{{"", "10", true, same, true}})
	if len(steps.Branches) < 4 || !slices.Contains(steps.Branches, "Товары") || !slices.Contains(steps.Branches, "ФормаСписка") || slices.Contains(steps.Branches, "Контрагенты") {
		t.Errorf("the tree of the forms: %q", steps.Branches)
	} else {
		at := slices.Index(steps.Branches, "Общие формы")
		expect("the head of the tree", steps.Branches[0], "Общие")
		if at < 0 || at+1 >= len(steps.Branches) || steps.Branches[at+1] != "ДемоФормаТовара" {
			t.Errorf("the common forms: %q", steps.Branches)
		}
	}
	expect("the focus after OK", steps.Focus, "home-page-height")
	expect("cancelled", steps.Cancelled, [][]any{{list, "10", true, same, false}, {demo, "10", true, same, true}})
	expect("the copy offered its form", steps.CopyChosen, "ДемоФормаТовара")
	expect("copied and up", steps.Copied, [][]any{{list, "10", true, same, false}, {demo, "10", true, same, true}, {demo, "10", true, same, false}})
	expect("the heights", steps.Heights, []string{"", "123456", "42"})
	expect("«Настройка видимости»", steps.Settings, []string{"Настройка видимости", "Первая default", "Вторая default"})
	expect("a click on a role", steps.Turned, "off")
	expect("the link", steps.Different, differ)
	expect("carried: the left", steps.CarriedLeft, [][]any{{list, "", true, differ, false}, {demo, "42", true, same, true}})
	expect("carried: the right", steps.CarriedRight, [][]any{{demo, "123456", true, same, true}})
	expect("one column", steps.OneColumn, []any{1, 0, false})
	expect("two columns again", steps.Back, [][]any{{demo, "123456", true, same, true}})
	expect("deleted", steps.Deleted, [][]any{{list, "", true, differ, true}})
	expect("changed", steps.Dirty, true)
	expect("saved", steps.Saved, true)
	file, err := os.Open(filepath.Join(root, project.HomePageFile))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	value, err := metadata.DecodeRootDescription(project.HomePageFile, file)
	if err != nil {
		t.Fatal(err)
	}
	expect("the page written", value, metadata.HomePage{
		Template: metadata.TwoColumnsVariableWidthHomePage, MainSectionCommands: metadata.MainSectionCommandsBottom,
		Left: []metadata.HomePageForm{{Form: mustUUID(t, listForm),
			Visibility: project.InterfaceVisibility{Common: true, Roles: []project.RoleVisibility{{Role: roles["Первая"], Visible: false}}}}},
		Right: []metadata.HomePageForm{{Form: mustUUID(t, "10000000-0000-4000-8000-000000000501"), Height: 123456,
			Visibility: project.InterfaceVisibility{Common: true}}},
	})
}
