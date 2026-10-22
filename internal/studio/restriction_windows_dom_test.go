package studio

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The windows over the restrictions of every role - «Все ограничения
// доступа» with its wizard, its group «Изменить», its copying and its filter
// - and the windows of one role that share their parts (block 2в; inventory
// of block 00, vse-ogranicheniya-dostupa.md, rol.md; help,
// form_NewRowLevelRightDlg, form_EditRowLevelRightDlg,
// form_CopyRowLevelInstrDlg, form_RolesGroupEditFilterDlg,
// form_subsystemchoicedlg). Each walk presses through a window as a person
// does and answers what it saw; the expectations are written in the tests,
// from the inventory and the help.

const (
	demoGoods       = "10000000-0000-4000-8000-000000000101"
	demoGoodsCode   = "10000000-0000-4000-8000-000000001101"
	demoPartners    = "10000000-0000-4000-8000-000000000102"
	demoPartnersTIN = "10000000-0000-4000-8000-000000001102"
)

// addRestrictionRoles gives the demo project its subsystems - Продажи with
// Склад under it, Склад holding Товары - and three roles: Кладовщик
// restricting the reading of Товары by Артикул and on «Прочие поля» and of
// Контрагенты by ИНН, with the template Шаблон; Продавец reading Товары, with
// Шаблон of another text and Другой; Бухгалтер with nothing.
func addRestrictionRoles(t *testing.T, root string) func(*Workspace) {
	t.Helper()
	addTreeSubsystems(t, root)
	return func(workspace *Workspace) {
		t.Helper()
		id := func(text string) uuid.UUID {
			value, err := uuid.Parse(text)
			if err != nil {
				t.Fatal(err)
			}
			return value
		}
		read := []metadata.PermissionOperation{metadata.PermissionRead}
		for _, role := range []struct {
			name      string
			objects   []metadata.ObjectPermission
			templates []metadata.RestrictionTemplate
		}{
			{"Кладовщик", []metadata.ObjectPermission{
				{Object: id(demoGoods), Operations: []metadata.PermissionOperation{metadata.PermissionRead, metadata.PermissionUpdate},
					Restrictions: []metadata.RightRestriction{{Right: metadata.PermissionRead, Fields: []string{demoGoodsCode}, Condition: "ГДЕ А"},
						{Right: metadata.PermissionRead, Condition: "ГДЕ Б"}}},
				{Object: id(demoPartners), Operations: read,
					Restrictions: []metadata.RightRestriction{{Right: metadata.PermissionRead, Fields: []string{demoPartnersTIN}, Condition: "ГДЕ В"}}},
			}, []metadata.RestrictionTemplate{{Name: "Шаблон", Condition: "ТЕКСТ1"}}},
			{"Продавец", []metadata.ObjectPermission{{Object: id(demoGoods), Operations: read}},
				[]metadata.RestrictionTemplate{{Name: "Шаблон", Condition: "ТЕКСТ2"}, {Name: "Другой", Condition: "Т"}}},
			{"Бухгалтер", nil, nil},
		} {
			created, err := workspace.CreateRole(role.name)
			if err != nil {
				t.Fatal(err)
			}
			created.Role.Objects, created.Role.RestrictionTemplates = role.objects, role.templates
			if _, err := workspace.SaveRole(created.Path, created.Role, created.Revision); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// restrictionWalkTools are what every walk below presses with.
const restrictionWalkTools = `
const studio = window.__studio;
const all = (selector, root = document) => [...root.querySelectorAll(selector)];
const byText = (selector, text, root = document) => all(selector, root).find(item => item.textContent.trim() === text);
const dialog = () => all('dialog[open]').at(-1);
const titleOf = () => dialog()?.querySelector('h3')?.textContent || null;
const action = text => byText('.localized-title-actions button', text, dialog());
const enabled = text => !!action(text) && !action(text).disabled;
const press = async text => { action(text).click(); await studio.settle(); };
const box = (caption, root = dialog()) => all('label', root).find(label => label.textContent.trim() === caption)?.querySelector('input');
const mark = async (caption, value, root) => { const input = box(caption, root); if (input.checked !== value || input.indeterminate) input.click(); await studio.settle(); };
const rows = () => all('.overview-block .role-list tbody tr[tabindex]');
const row = (...parts) => rows().find(item => parts.every(part => item.textContent.includes(part)));
const choose = async (target, keys = {}) => { target.dispatchEvent(new MouseEvent('click', {bubbles: true, ...keys})); await studio.settle(); };
const command = async key => { document.querySelector('.overview-block .role-bulk [data-command="' + key + '"]').click(); await studio.settle(); };
const menu = async (button, text) => { button.click(); await studio.settle(); byText('.context-menu .context-menu-item', text).click(); await studio.settle(); };
const filterLine = () => document.querySelector('.overview-filter-line')?.textContent.trim();
const overview = async name => {
  const roles = byText('#tree .studio-tree-row', 'Роли');
  roles.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  byText('.context-menu .context-menu-item', name).click();
  await studio.settle();
};
const tab = async text => { byText('.studio-tab', text).click(); await studio.settle(); };
`

// restrictionWizardWalk adds a restriction through «Новое ограничение
// доступа», noting at each step what may be pressed.
const restrictionWizardWalk = `(async () => {` + restrictionWalkTools + `
  await overview('Все ограничения доступа');
  const seen = {};
  // Ins over the list opens the wizard, as «Добавить» of the menu says.
  document.querySelector('.overview-block .role-list').dispatchEvent(new KeyboardEvent('keydown', {key: 'Insert', bubbles: true}));
  await studio.settle();
  seen.insertOpens = titleOf();
  seen.nextWithoutObject = enabled('Далее >');
  await mark('Товары', true);
  seen.nextWithObject = enabled('Далее >');
  const kind = all('label', dialog()).find(label => label.textContent.trim() === 'Справочники').querySelector('input');
  seen.kindPartly = kind.indeterminate;
  kind.click(); await studio.settle();
  seen.kindClickClears = !box('Товары').checked && !box('Контрагенты').checked;
  await mark('Товары', true); await mark('Контрагенты', true);
  byText('button', 'По подсистемам', dialog()).click(); await studio.settle();
  await mark('Склад', true); await press('Установить');
  seen.filterClearsObjects = !box('Товары').checked && !box('Контрагенты');
  await mark('Товары', true); await mark('Кладовщик', true);
  await press('Далее >');
  seen.primaryOnSecond = dialog().querySelector('.localized-title-actions button.primary')?.textContent;
  await mark('Чтение', true);
  seen.othersFirst = box('Прочие поля').checked;
  await mark('Артикул', true);
  seen.fieldTakesOthers = !box('Прочие поля').checked;
  await mark('Артикул', false);
  seen.lastFieldGivesOthers = box('Прочие поля').checked;
  await mark('Артикул', true); await mark('Прочие поля', true);
  seen.othersTakeFields = !box('Артикул').checked;
  await mark('Прочие поля', false);
  seen.okWithNothingMarked = enabled('ОК');
  await mark('Прочие поля', true);
  seen.okWithOthers = enabled('ОК');
  await press('< Назад');
  seen.okOnFirst = enabled('ОК');
  await press('Отмена');
  return seen;
})()`

// TestTheWizardOfANewRestrictionStepsAsThePrototypes catches the wizard of
// a new restriction letting a step go where the prototype's does not (snapshot
// master-shag1; ACCESS-RIGHTS, «Мастер, шаг 1»): «Далее >» with no object
// marked, «ОК» on the first step, «ОК» with neither a field nor «Прочие поля»;
// a click on a kind marked in part marking the rest instead of clearing it; a
// change of the filter by subsystems leaving objects marked; the fields and
// «Прочие поля» not taking each other's marks; Ins over the list not opening
// it; Enter pressing «< Назад» on the second step.
func TestTheWizardOfANewRestrictionStepsAsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, addRestrictionRoles(t, root))
	result, err := browser.evaluate(ctx, page, restrictionWizardWalk)
	if err != nil {
		t.Fatal(err)
	}
	var seen struct {
		InsertOpens                                                           string
		NextWithoutObject, NextWithObject, KindPartly                         bool
		KindClickClears, FilterClearsObjects                                  bool
		PrimaryOnSecond                                                       string
		OthersFirst, FieldTakesOthers, LastFieldGivesOthers, OthersTakeFields bool
		OkWithNothingMarked, OkWithOthers, OkOnFirst                          bool
	}
	if err := json.Unmarshal(result, &seen); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	for _, check := range []struct {
		ok   bool
		says string
	}{
		{seen.InsertOpens == "Новое ограничение доступа", "Ins over the list opened " + seen.InsertOpens},
		{!seen.NextWithoutObject, "«Далее >» is available with no object marked"},
		{seen.NextWithObject, "«Далее >» is unavailable with an object marked"},
		{seen.KindPartly, "a kind with one of its objects marked is not shown marked in part"},
		{seen.KindClickClears, "a click on a kind marked in part did not clear its objects"},
		{seen.FilterClearsObjects, "a filter by subsystems left objects marked or showed one it hides"},
		{seen.PrimaryOnSecond == "ОК", "Enter presses " + seen.PrimaryOnSecond + " on the second step"},
		{seen.OthersFirst, "«Прочие поля» is not marked while no field is"},
		{seen.FieldTakesOthers, "marking a field left «Прочие поля» marked"},
		{seen.LastFieldGivesOthers, "taking the last field away did not mark «Прочие поля»"},
		{seen.OthersTakeFields, "marking «Прочие поля» left a field marked"},
		{!seen.OkWithNothingMarked, "«ОК» is available with neither a field nor «Прочие поля» marked"},
		{seen.OkWithOthers, "«ОК» is unavailable with a right and «Прочие поля» marked"},
		{!seen.OkOnFirst, "«ОК» is available on the first step"},
	} {
		if !check.ok {
			t.Error(check.says)
		}
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}

// restrictionGroupWalk changes restrictions in a group and copies them.
const restrictionGroupWalk = `(async () => {` + restrictionWalkTools + `
  await overview('Все ограничения доступа');
  const seen = {};
  await choose(row('Контрагенты'));
  await choose(row('Товары', 'ГДЕ Б'), {shiftKey: true});
  seen.shiftChooses = all('.overview-block .role-list tbody tr.selected').length;
  await choose(row('Контрагенты'));
  await choose(row('Товары', 'ГДЕ А'), {ctrlKey: true});
  const state = caption => { const input = box(caption); return input.indeterminate ? 'grey' : input.checked ? 'marked' : 'off'; };
  const both = async () => { await choose(row('Контрагенты')); await choose(row('Товары', 'ГДЕ А'), {ctrlKey: true}); await command('edit'); };
  await command('edit');
  seen.firstStates = ['ИНН', 'Артикул', 'Код', 'Прочие поля'].map(state);
  seen.okUnchanged = enabled('ОК');
  await mark('Изменять состав полей', true);
  seen.okFlagAlone = enabled('ОК');
  await mark('Код', true);
  seen.okChanged = enabled('ОК');
  await press('ОК');
  seen.partnersField = row('Контрагенты')?.textContent || '';
  seen.goodsField = row('Товары', 'ГДЕ А')?.textContent || '';
  await both();
  seen.secondStates = ['ИНН', 'Артикул', 'Код', 'Прочие поля'].map(state);
  await mark('Изменять состав полей', true);
  box('ИНН').click(); await studio.settle();
  seen.greyClicked = state('ИНН');
  seen.okGreyTaken = enabled('ОК');
  await mark('Прочие поля', true);
  seen.othersStates = ['Артикул', 'Код'].map(state);
  seen.okOthers = enabled('ОК');
  await mark('Артикул', true);
  seen.okMarked = enabled('ОК');
  await press('ОК');
  seen.partnersAfterGrey = row('Контрагенты')?.textContent || '';
  seen.goodsAfterGrey = row('Товары', 'ГДЕ А')?.textContent || '';
  await choose(row('Товары', 'ГДЕ А'));
  await command('edit');
  await mark('Изменять состав полей', true);
  await mark('Прочие поля', true);
  seen.okOthersOnly = enabled('ОК');
  const text = dialog().querySelector('textarea');
  text.value = 'ГДЕ Д'; text.dispatchEvent(new Event('input', {bubbles: true})); await studio.settle();
  seen.okEdited = enabled('ОК');
  await press('ОК');
  seen.replaceAsked = titleOf();
  seen.goods = rows().filter(item => item.textContent.includes('Товары') && item.textContent.includes('Кладовщик')).map(item => item.textContent);
  await choose(row('Товары', 'ГДЕ Д'));
  await command('copy');
  seen.copyTitle = titleOf();
  seen.copyOkWithoutRoles = enabled('ОК');
  seen.copyOffersItsOwn = !!box('Кладовщик', dialog().querySelector('.role-copy-grid > :last-child'));
  await mark('Продавец', true);
  seen.copyOkWithRole = enabled('ОК');
  await press('Отмена');
  await tab('Шаблоны ограничений');
  await choose(row('Кладовщик', 'Шаблон'));
  await choose(row('Продавец', 'Другой'), {ctrlKey: true});
  await command('edit');
  seen.nameFlagApart = box('Изменять наименование').checked;
  seen.textFlagApart = box('Изменять текст').checked;
  await press('Отмена');
  for (const [answer, key] of [['Нет', 'No'], ['Отмена', 'Cancel']]) {
    await choose(row('Кладовщик', 'Шаблон'));
    await command('copy');
    await mark('Продавец', true);
    await press('ОК');
    seen['asked' + key] = titleOf();
    await press(answer);
    seen['after' + key] = titleOf();
    if (dialog()) await press('Отмена');
  }
  seen.sellerTemplate = row('Продавец', 'Шаблон')?.textContent || '';
  return seen;
})()`

// TestRestrictionsAreChangedAndCopiedInAGroupAsThePrototype catches the group
// «Изменить» and the copying of «Все ограничения доступа» going their own way
// (ACCESS-RIGHTS, «Правка ограничений группой, копирование», «Групповое
// «Изменить» состава полей»; configurator, 14.10, 18.10 and 22.10.2026,
// vse-ogranicheniya-dostupa.md «Досъёмка 22.10.2026»): Shift not choosing
// the rows from the last one clicked; a field some of the restrictions name
// not grey, or a grey one not kept as it was; «ОК» available before a field
// is marked or the condition edited - on the flag alone, on a grey field
// taken away or on «Прочие поля»; a click on a grey field marking it;
// «Прочие поля» leaving the grey fields; an object left with no field not
// taking «Прочие поля»; the question «Заменить?» asked when the restriction
// changed takes the place of one with the same fields (defects of 2.437,
// case 2в-23); the copy offered into the roles it copies from, or «ОК» with no role
// marked; «Изменять наименование» set over templates of different names; and
// «Нет» to the question about a template of the same name copying it all
// the same, or «Отмена» not going back to the window.
func TestRestrictionsAreChangedAndCopiedInAGroupAsThePrototype(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, addRestrictionRoles(t, root))
	result, err := browser.evaluate(ctx, page, restrictionGroupWalk)
	if err != nil {
		t.Fatal(err)
	}
	var seen struct {
		ShiftChooses                                               int
		FirstStates, SecondStates, OthersStates, Goods             []string
		OkUnchanged, OkFlagAlone, OkChanged, OkGreyTaken, OkOthers bool
		OkMarked, OkOthersOnly, OkEdited                           bool
		GreyClicked, PartnersField, GoodsField, ReplaceAsked       string
		PartnersAfterGrey, GoodsAfterGrey                          string
		CopyTitle                                                  string
		CopyOkWithoutRoles, CopyOffersItsOwn                       bool
		CopyOkWithRole                                             bool
		NameFlagApart, TextFlagApart                               bool
		AskedNo, AfterNo, AskedCancel, AfterCancel                 string
		SellerTemplate                                             string
	}
	if err := json.Unmarshal(result, &seen); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	const sameName = "Шаблоны с таким наименованием уже существуют в выбранных ролях. Заменять существующие шаблоны?"
	for _, check := range []struct {
		ok   bool
		says string
	}{
		{seen.ShiftChooses == 3, "Shift chose rows other than those from the last one clicked"},
		{slices.Equal(seen.FirstStates, []string{"grey", "grey", "off", "marked"}), fmt.Sprintf("ИНН, Артикул, Код and «Прочие поля» of two restrictions by ИНН and by Артикул stand %v", seen.FirstStates)},
		{!seen.OkUnchanged && !seen.OkFlagAlone, "«ОК» of the group change is available before a field is marked"},
		{seen.OkChanged, "«ОК» of the group change is unavailable after a field was marked"},
		{strings.Contains(seen.PartnersField, "Код, ИНН") && strings.Contains(seen.GoodsField, "Код, Артикул"),
			"Код marked and the grey fields kept gave " + seen.PartnersField + " and " + seen.GoodsField},
		{slices.Equal(seen.SecondStates, []string{"grey", "grey", "marked", "off"}), fmt.Sprintf("the second time the fields stand %v", seen.SecondStates)},
		{seen.GreyClicked == "off" && !seen.OkGreyTaken, "a click on the grey ИНН left it " + seen.GreyClicked + " (ОК available: " + fmt.Sprint(seen.OkGreyTaken) + ")"},
		{slices.Equal(seen.OthersStates, []string{"off", "off"}) && !seen.OkOthers, fmt.Sprintf("«Прочие поля» left Артикул and Код %v (ОК available: %v)", seen.OthersStates, seen.OkOthers)},
		{seen.OkMarked, "«ОК» is unavailable after Артикул was marked"},
		{strings.Contains(seen.PartnersAfterGrey, "Прочие поля") && strings.Contains(seen.GoodsAfterGrey, "Артикул") && !strings.Contains(seen.GoodsAfterGrey, "Код"),
			"Артикул alone marked gave " + seen.PartnersAfterGrey + " and " + seen.GoodsAfterGrey},
		{!seen.OkOthersOnly && seen.OkEdited, "«ОК» over «Прочие поля» alone is available, or not after the condition was edited"},
		{seen.ReplaceAsked == "", "the change taking the place of the restriction by «Прочие поля» asked " + seen.ReplaceAsked},
		{len(seen.Goods) == 1 && strings.Contains(seen.Goods[0], "ГДЕ Д"), fmt.Sprintf("Товары of Кладовщик are restricted by %q, want ГДЕ Д alone in the place of ГДЕ Б", seen.Goods)},
		{seen.CopyTitle == "Копировать ограничения", "the copy opened " + seen.CopyTitle},
		{!seen.CopyOkWithoutRoles, "«ОК» of the copy is available with no role marked"},
		{!seen.CopyOffersItsOwn, "the copy offers the role it copies from"},
		{seen.CopyOkWithRole, "«ОК» of the copy is unavailable with a role marked"},
		{!seen.NameFlagApart && !seen.TextFlagApart, "a flag of change is set over templates that differ in it"},
		{seen.AskedNo == sameName && seen.AskedCancel == sameName, "the copy of a template of the same name asked " + seen.AskedNo},
		{seen.AfterNo == "", "«Нет» left the window " + seen.AfterNo + " open"},
		{seen.AfterCancel == "Копировать шаблоны ограничений", "«Отмена» went to " + seen.AfterCancel},
		{strings.Contains(seen.SellerTemplate, "ТЕКСТ2"), "«Нет» copied the template all the same: " + seen.SellerTemplate},
	} {
		if !check.ok {
			t.Error(check.says)
		}
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}

// restrictionFilterWalk sets the filter of the restrictions, through its
// window and through the subsystems, and moves to «Все роли» and back.
const restrictionFilterWalk = `(async () => {` + restrictionWalkTools + `
  await overview('Все ограничения доступа');
  const seen = {};
  await command('filter');
  seen.everyMarked = all('input[type=checkbox]', dialog().querySelector('.role-filter-grid')).every(input => input.checked);
  await press('ОК');
  seen.lineAfterAll = filterLine();
  await command('filter');
  byText('button', 'По подсистемам', dialog()).click(); await studio.settle();
  await mark('Склад', true); await press('Установить');
  const objectBoxes = () => all('.role-wizard-objects input[type=checkbox]', dialog());
  seen.markedAfterSet = objectBoxes().filter(input => input.checked || input.indeterminate).length;
  objectBoxes()[0].click(); await studio.settle();
  await press('ОК');
  seen.objects = rows().map(item => item.firstChild.textContent);
  seen.line = filterLine();
  await command('filter');
  seen.markedBeforeOff = objectBoxes().filter(input => input.checked).length;
  byText('button', 'По подсистемам', dialog()).click(); await studio.settle();
  await press('Отключить');
  seen.shownAfterOff = objectBoxes().length;
  seen.markedAfterOff = objectBoxes().filter(input => input.checked || input.indeterminate).length;
  await press('Отмена');
  await overview('Все роли');
  await overview('Все ограничения доступа');
  seen.lineBack = filterLine();
  await menu(document.querySelector('.overview-block .menu-button'), 'Настройка списка...');
  const width = dialog().querySelector('input[aria-label="Ширина"]');
  width.value = '0'; width.dispatchEvent(new Event('change', {bubbles: true})); await studio.settle();
  seen.widthAfterZero = width.value;
  await press('Отмена');
  return seen;
})()`

// TestTheFilterOfRestrictionsIsThePrototypes catches the window of the
// filter of restrictions opening with something unmarked, everything marked
// taken for a condition, an object the filter by subsystems hides kept among
// those chosen, the filter outliving a move to «Все роли», and a width of 0
// taken by «Настройка списка» (configurator 14.10.2026 and 19.10.2026,
// snapshots vse-ogr-04, shabl-21). A change of the filter by subsystems -
// «Установить» or «Отключить» - unmarks every object, the ones still shown
// too, as the wizard does (configurator 22.10.2026, vse-ogranicheniya-
// dostupa.md); defect caught (2.437, case 2в-22): the objects left shown
// stayed marked.
func TestTheFilterOfRestrictionsIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, addRestrictionRoles(t, root))
	result, err := browser.evaluate(ctx, page, restrictionFilterWalk)
	if err != nil {
		t.Fatal(err)
	}
	var seen struct {
		EveryMarked                                                    bool
		LineAfterAll, Line, LineBack                                   string
		Objects                                                        []string
		WidthAfterZero                                                 string
		MarkedAfterSet, MarkedBeforeOff, ShownAfterOff, MarkedAfterOff int
	}
	if err := json.Unmarshal(result, &seen); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	if !seen.EveryMarked {
		t.Error("the filter opens with something unmarked")
	}
	if seen.LineAfterAll != "Отбор:" {
		t.Errorf("everything marked set a filter: %q", seen.LineAfterAll)
	}
	for _, object := range seen.Objects {
		if object != "Товары" {
			t.Errorf("a filter by Склад shows %v", seen.Objects)
			break
		}
	}
	if len(seen.Objects) != 2 || !strings.Contains(seen.Line, "Объекты: Товары") {
		t.Errorf("a filter by Склад shows %v under %q", seen.Objects, seen.Line)
	}
	if seen.MarkedAfterSet != 0 {
		t.Errorf("«Установить» left %d boxes of objects marked, want none", seen.MarkedAfterSet)
	}
	if seen.MarkedBeforeOff == 0 || seen.ShownAfterOff <= seen.MarkedBeforeOff || seen.MarkedAfterOff != 0 {
		t.Errorf("«Отключить» over %d marked boxes shows %d boxes with %d marked, want more shown and none marked",
			seen.MarkedBeforeOff, seen.ShownAfterOff, seen.MarkedAfterOff)
	}
	if seen.LineBack != "Отбор:" {
		t.Errorf("the filter outlived a move to «Все роли»: %q", seen.LineBack)
	}
	if seen.WidthAfterZero != "14" {
		t.Errorf("a width of 0 was taken: %q", seen.WidthAfterZero)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}

// roleWindowWalk works in the window of the role Кладовщик.
const roleWindowWalk = `(async () => {` + restrictionWalkTools + `
  byText('#tree .studio-tree-row', 'Кладовщик').click();
  await studio.settle();
  const seen = {};
  const editor = document.querySelector('#role-editor');
  const node = async id => { editor.querySelector('[data-object="' + id + '"]').click(); await studio.settle(); };
  const rights = () => Object.fromEntries(all('.check-list-row', editor).map(item => [item.dataset.key, item.querySelector('input').checked]));
  byText('.role-tree-group', 'Общие', editor).click(); await studio.settle();
  seen.markButtonsWithoutRights = all('.role-rights-head button', editor).every(button => button.disabled);
  await node('` + demoGoods + `');
  const list = editor.querySelector('.check-list');
  list.dispatchEvent(new KeyboardEvent('keydown', {key: ' ', bubbles: true})); await studio.settle();
  seen.spaceClears = rights().read;
  editor.querySelector('.check-list').dispatchEvent(new KeyboardEvent('keydown', {key: ' ', bubbles: true})); await studio.settle();
  seen.spaceMarks = rights().read;
  editor.querySelector('.role-restrictions [data-command="add"]').click(); await studio.settle();
  seen.takenField = box('Артикул').disabled;
  await mark('Прочие поля', false);
  seen.okWithNothing = enabled('ОК');
  await press('Отмена');
  const root = editor.querySelector('.role-tree-root').dataset.object;
  await menu(editor.querySelector('.role-tree-bar .menu-button'), 'Установить по подсистемам...');
  await mark('Склад', true); await mark('<Не входящие в подсистемы>', true);
  await press('Установить');
  await node(root);
  seen.rootRights = rights();
  editor.querySelector('.role-tree-bar .role-subsystems').click(); await studio.settle();
  seen.rootMarkedFirst = box('ПродажиИСклад').checked;
  await press('Установить');
  const filterButton = editor.querySelector('.role-tree-bar .role-subsystems');
  seen.pressed = filterButton.getAttribute('aria-pressed') + ' ' + filterButton.classList.contains('selected');
  seen.shownUnderEvery = ['Товары', 'Контрагенты', 'ПоступлениеТоваров'].filter(name => all('.role-tree-item', editor).some(item => item.title === name));
  const tree = byText('#tree .studio-tree-row', 'Роли');
  tree.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: innerWidth - 2, clientY: innerHeight - 2}));
  const shown = document.querySelector('.context-menu').getBoundingClientRect();
  seen.menuInside = shown.right <= innerWidth && shown.bottom <= innerHeight;
  document.querySelector('.context-menu .context-menu-item').dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape', bubbles: true}));
  await tab('Шаблоны ограничений');
  editor.querySelector('.role-templates [data-command="add"]').click(); await studio.settle();
  action('ОК').click(); await studio.settle();
  seen.refusalKeeps = titleOf();
  seen.refusal = dialog()?.querySelector('.dialog-problem')?.textContent || '';
  if (dialog()) await press('Отмена');
  return seen;
})()`

// TestTheWindowOfARoleKeepsToThePrototype catches the window of a role going
// its own way where it shares its parts with «Все ограничения доступа»
// (block-00/rol, ACCESS-RIGHTS «По подсистемам»): the buttons over the list
// of rights available on a node with no rights; the space not turning the
// current right over; a field another restriction names markable in the
// window of a restriction, and its «ОК» available with nothing marked;
// «Установить по подсистемам» giving the configuration its rights; the root
// of the window of the filter not standing for every subsystem; the button
// of the filter not pressed under it; a menu at the edge of the page standing
// out of it; and a refusal of the check of a window closing the window.
func TestTheWindowOfARoleKeepsToThePrototype(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, addRestrictionRoles(t, root))
	result, err := browser.evaluate(ctx, page, roleWindowWalk)
	if err != nil {
		t.Fatal(err)
	}
	var seen struct {
		MarkButtonsWithoutRights, SpaceClears, SpaceMarks bool
		TakenField, OkWithNothing                         bool
		RootRights                                        map[string]bool
		RootMarkedFirst                                   bool
		Pressed                                           string
		ShownUnderEvery                                   []string
		MenuInside                                        bool
		RefusalKeeps, Refusal                             string
	}
	if err := json.Unmarshal(result, &seen); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	for right, marked := range seen.RootRights {
		if marked {
			t.Errorf("«Установить по подсистемам» gave the configuration %s", right)
		}
	}
	for _, check := range []struct {
		ok   bool
		says string
	}{
		{seen.MarkButtonsWithoutRights, "the buttons over the list are available on «Общие», which has no rights"},
		{!seen.SpaceClears && seen.SpaceMarks, "the space does not turn the current right over"},
		{seen.TakenField, "a field another restriction names can be marked"},
		{!seen.OkWithNothing, "«ОК» of a restriction is available with nothing marked"},
		{len(seen.RootRights) > 0, "the configuration shows no rights"},
		{seen.RootMarkedFirst, "the window of the filter opens with the root unmarked"},
		{seen.Pressed == "true true", "the button of the filter is not shown pressed under it: " + seen.Pressed},
		{len(seen.ShownUnderEvery) == 3, "the root of the filter does not stand for every subsystem: shown " + strings.Join(seen.ShownUnderEvery, ", ")},
		{seen.MenuInside, "a menu at the edge of the page stands out of it"},
		{seen.RefusalKeeps == "Шаблон ограничения" && seen.Refusal == "Укажите наименование шаблона.",
			"a refusal of the check closed the window: " + seen.RefusalKeeps + " " + seen.Refusal},
	} {
		if !check.ok {
			t.Error(check.says)
		}
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
