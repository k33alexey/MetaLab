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
  // A register of accumulation offers no field: «Прочие поля» stand marked
  // and unavailable.
  document.querySelector('.overview-block .role-list').dispatchEvent(new KeyboardEvent('keydown', {key: 'Insert', bubbles: true}));
  await studio.settle();
  byText('button', 'По подсистемам', dialog()).click(); await studio.settle();
  await press('Отключить');
  await mark('Остатки товаров', true); await mark('Кладовщик', true);
  await press('Далее >');
  await mark('Чтение', true);
  seen.registerFields = all('.role-fields-choice .role-restriction-fields label', dialog()).map(label => label.textContent.trim());
  seen.registerOthers = box('Прочие поля').checked && box('Прочие поля').disabled;
  seen.okRegister = enabled('ОК');
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
// it; Enter pressing «< Назад» on the second step; a register of
// accumulation offering fields, or «Прочие поля» it can take away (the
// seventh pass, pola-45…47).
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
		RegisterFields                                                        []string
		RegisterOthers, OkRegister                                            bool
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
		{len(seen.RegisterFields) == 0, fmt.Sprintf("a register of accumulation offers the fields %v", seen.RegisterFields)},
		{seen.RegisterOthers, "«Прочие поля» of a register of accumulation are not marked and unavailable"},
		{seen.OkRegister, "«ОК» is unavailable for a register of accumulation with «Чтение» marked"},
	} {
		if !check.ok {
			t.Error(check.says)
		}
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}

// Identifiers of the objects writeEveryRestrictableKind writes that a role
// restricts below.
const (
	everySequence      = "e1000000-0000-4000-8000-000000000001"
	everyRecalculation = "e1000000-0000-4000-8000-000000000002"
	everyTable         = "e1000000-0000-4000-8000-000000000003"
)

// writeEveryRestrictableKind writes an object of every kind a role may
// restrict, each named as its kind in one word, and a role Кладовщик
// restricting the reading of the sequence, the recalculation and the table
// of the external source - the kinds the wizard does not offer.
func writeEveryRestrictableKind(t *testing.T, root string) func(*Workspace) {
	t.Helper()
	id := func() string { return uuid.MustNew().String() }
	head := func(name, identifier string) string {
		return "format: 1\nid: " + identifier + "\nname: " + name + "\ntitle: {ru: " + name + "}\n"
	}
	write := func(path, content string) { writeProjectFile(t, root, path+"/object.yaml", content) }
	const code = "code: {type: string, length: 9, auto: false}\ndescription_length: 50\n"
	const number = "number: {type: string, length: 9, auto: true, periodicity: none}\n"
	sum := func() string {
		return "resources:\n  - {id: " + id() + ", name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}\n"
	}
	document, chart, calculation, task := id(), id(), id(), id()
	write("metadata/catalogs/Справочник", head("Справочник", id())+code)
	write("metadata/documents/Документ", head("Документ", document)+number)
	write("metadata/document-journals/Журнал", head("Журнал", id())+"documents: ["+document+"]\n")
	write("metadata/charts-of-characteristic-types/Характеристики", head("Характеристики", id())+code+"value_type: [{kind: boolean}]\n")
	write("metadata/charts-of-accounts/Счета", head("Счета", chart)+code)
	write("metadata/charts-of-calculation-types/Расчеты", head("Расчеты", calculation)+code)
	write("metadata/information-registers/Сведения", head("Сведения", id())+"write_mode: independent\nperiodicity: none\n"+sum())
	write("metadata/accumulation-registers/Накопления", head("Накопления", id())+"kind: turnover\n"+sum())
	write("metadata/accounting-registers/Бухгалтерия", head("Бухгалтерия", id())+"chart_of_accounts: "+chart+"\n"+sum())
	write("metadata/calculation-registers/Начисления", head("Начисления", id())+"chart_of_calculation_types: "+calculation+
		"\nperiodicity: month\n"+sum()+"recalculations:\n  - {id: "+everyRecalculation+", name: Перерасчет, title: {ru: Перерасчет}}\n")
	write("metadata/business-processes/Процесс", head("Процесс", id())+number+"task: "+task+"\n")
	write("metadata/tasks/Задача", head("Задача", task)+number+"description_length: 50\n")
	write("metadata/exchange-plans/Обмен", head("Обмен", id())+code)
	write("metadata/constants/Константа", head("Константа", id())+"types: [{kind: boolean}]\n")
	write("metadata/sequences/Последовательность", head("Последовательность", everySequence)+"documents: ["+document+"]\n")
	write("metadata/external-data-sources/Источник", head("Источник", id()))
	write("metadata/external-data-sources/Источник/tables/Таблица", head("Таблица", everyTable)+"name_in_data_source: dbo.T\ndata_type: non-object\n")
	write("metadata/external-data-sources/Источник/cubes/Куб", head("Куб", id())+"name_in_data_source: C\n")
	write("metadata/external-data-sources/Источник/cubes/Куб/dimension-tables/Измерение", head("Измерение", id())+"name_in_data_source: D\n")
	return func(workspace *Workspace) {
		t.Helper()
		created, err := workspace.CreateRole("Кладовщик")
		if err != nil {
			t.Fatal(err)
		}
		read := []metadata.PermissionOperation{metadata.PermissionRead}
		for _, text := range []string{everySequence, everyRecalculation, everyTable} {
			object, err := uuid.Parse(text)
			if err != nil {
				t.Fatal(err)
			}
			created.Role.Objects = append(created.Role.Objects, metadata.ObjectPermission{Object: object, Operations: read,
				Restrictions: []metadata.RightRestriction{{Right: metadata.PermissionRead, Condition: "ГДЕ ИСТИНА"}}})
		}
		if _, err := workspace.SaveRole(created.Path, created.Role, created.Revision); err != nil {
			t.Fatal(err)
		}
	}
}

// restrictionKindsWalk reads the tree of objects of the first step of the
// wizard and of the window «Отбор», row by row with its depth.
const restrictionKindsWalk = `(async () => {` + restrictionWalkTools + `
  await overview('Все ограничения доступа');
  const tree = () => all('.role-wizard-objects .role-fields-row', dialog()).map(item => item.style.getPropertyValue('--depth') + ' ' + item.querySelector('label').textContent.trim());
  const seen = {};
  document.querySelector('.overview-block .role-list').dispatchEvent(new KeyboardEvent('keydown', {key: 'Insert', bubbles: true}));
  await studio.settle();
  seen.wizard = tree();
  await press('Отмена');
  await command('filter');
  seen.filter = tree();
  await press('Отмена');
  return seen;
})()`

// TestTheWizardOffersTheKindsThePrototypesDoes catches the first step of the
// wizard «Новое ограничение доступа» offering an object the prototype's does
// not, or leaving out one it does (review 2.464, Р1; the seventh pass of
// 22.10.2026, vse-ogranicheniya-dostupa.md «Что мастер предлагает»): the
// wizard offered whatever restricts - the sequence, the recalculation and the
// objects of the external source - where the prototype's offers catalogs,
// documents, journals, the three charts, the registers of four kinds,
// processes, tasks and exchange plans. The window «Отбор» keeps every object
// a restriction is on, these three too: they are restricted in the window
// of a role, and a filter that lost them could not show their restrictions.
// The expectation is written out here, not read from the descriptors. The
// exchange plans stand first as a kind of their own, not inside «Общие» as
// the pass saw them: where «Общие» stands in the wizard was not seen, a
// question of the series of 2.465 (docs/series/2026-10-23-2.465.md, В1).
func TestTheWizardOffersTheKindsThePrototypesDoes(t *testing.T) {
	root := createProject(t)
	ctx, browser, page := openStudioInBrowser(t, root, writeEveryRestrictableKind(t, root))
	result, err := browser.evaluate(ctx, page, restrictionKindsWalk)
	if err != nil {
		t.Fatal(err)
	}
	var seen struct{ Wizard, Filter []string }
	if err := json.Unmarshal(result, &seen); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	wizard := []string{"0 Продажи и склад", "1 Планы обмена", "2 Обмен", "1 Справочники", "2 Справочник", "1 Документы", "2 Документ",
		"1 Журналы документов", "2 Журнал", "1 Планы видов характеристик", "2 Характеристики", "1 Планы счетов", "2 Счета",
		"1 Планы видов расчёта", "2 Расчеты", "1 Регистры сведений", "2 Сведения", "1 Регистры накопления", "2 Накопления",
		"1 Регистры бухгалтерии", "2 Бухгалтерия", "1 Регистры расчёта", "2 Начисления", "1 Бизнес-процессы", "2 Процесс",
		"1 Задачи", "2 Задача"}
	if !slices.Equal(seen.Wizard, wizard) {
		t.Errorf("the wizard offers\n%v\nwant\n%v", seen.Wizard, wizard)
	}
	filter := []string{"0 Продажи и склад", "1 Последовательности", "2 Последовательность", "1 Перерасчёты", "2 Начисления.Перерасчет",
		"1 Таблицы", "2 Источник.Таблица"}
	if !slices.Equal(seen.Filter, filter) {
		t.Errorf("the filter offers\n%v\nwant\n%v", seen.Filter, filter)
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
  // Two rows on «Прочие поля» with conditions that differ, as
  // gruppovoe-dve-stroki-01…03.
  await choose(row('Контрагенты'));
  await choose(row('Товары', 'ГДЕ Б'), {ctrlKey: true});
  await command('edit');
  const flags = () => ['Изменять состав полей', 'Изменять ограничение'].map(caption => (box(caption).checked ? 'marked' : 'off') + (box(caption).disabled ? ' disabled' : ''));
  seen.twoFlags = flags();
  seen.twoOkOpen = enabled('ОК');
  await mark('Код', true);
  seen.twoOkField = enabled('ОК');
  await mark('Прочие поля', true);
  seen.twoOkOthers = enabled('ОК');
  seen.twoFieldAfterOthers = state('Код');
  await press('Отмена');
  // One row opens as the window of one restriction, as
  // gruppovoe-odna-stroka-01…03.
  await choose(row('Товары', 'ГДЕ А'));
  await command('edit');
  seen.oneFlags = flags();
  seen.oneOkOpen = enabled('ОК');
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
// taking «Прочие поля»; «ОК» left available when «Прочие поля» take the
// field marked (2.456, gruppovoe-dve-stroki-03); one row opening with the
// flags of a group to change or «ОК» waiting, not as the window of one
// restriction (2.456, gruppovoe-odna-stroka-01…03); the question
// «Заменить?» asked when the restriction changed takes the place of one
// with the same fields (defects of 2.437, case 2в-23); the copy offered into the roles it copies from, or «ОК» with no role
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
		TwoFlags, OneFlags                                         []string
		TwoOkOpen, TwoOkField, TwoOkOthers, OneOkOpen              bool
		TwoFieldAfterOthers                                        string
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
		{slices.Equal(seen.TwoFlags, []string{"marked", "off"}) && !seen.TwoOkOpen, fmt.Sprintf("two rows on «Прочие поля» of conditions that differ open with the flags %v, ОК available: %v", seen.TwoFlags, seen.TwoOkOpen)},
		{seen.TwoOkField && !seen.TwoOkOthers && seen.TwoFieldAfterOthers == "off",
			fmt.Sprintf("over two rows «ОК» after a field is %v, after «Прочие поля» %v, the field left %s", seen.TwoOkField, seen.TwoOkOthers, seen.TwoFieldAfterOthers)},
		{slices.Equal(seen.OneFlags, []string{"marked disabled", "marked disabled"}) && seen.OneOkOpen,
			fmt.Sprintf("one row opens with the flags %v, ОК available: %v - want both marked and unavailable, «ОК» at once", seen.OneFlags, seen.OneOkOpen)},
		{seen.OkOthersOnly && seen.OkEdited, "«ОК» of one row is unavailable on «Прочие поля» or after the condition was edited"},
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
  const atDepth = depth => all('.role-wizard-objects .role-fields-row', dialog()).filter(item => item.style.getPropertyValue('--depth') === String(depth)).map(item => item.querySelector('label').textContent.trim()).sort();
  seen.offeredKinds = atDepth(1); seen.offeredObjects = atDepth(2);
  seen.offeredFields = all('label', dialog().querySelector('.role-filter-grid').children[1]).map(label => label.textContent.trim()).sort();
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
		OfferedKinds, OfferedObjects, OfferedFields                    []string
		WidthAfterZero                                                 string
		MarkedAfterSet, MarkedBeforeOff, ShownAfterOff, MarkedAfterOff int
	}
	if err := json.Unmarshal(result, &seen); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	if !seen.EveryMarked {
		t.Error("the filter opens with something unmarked")
	}
	// Only the objects some restriction is on, and only the fields
	// restrictions name (otbor-podsistemy-01): the documents and the register
	// of the demo, restrictable but restricted by no role, are not offered.
	if strings.Join(seen.OfferedKinds, ",") != "Справочники" || strings.Join(seen.OfferedObjects, ",") != "Контрагенты,Товары" {
		t.Errorf("the filter offers the kinds %v and the objects %v, want Справочники with Контрагенты and Товары", seen.OfferedKinds, seen.OfferedObjects)
	}
	if strings.Join(seen.OfferedFields, ",") != "Артикул,ИНН" {
		t.Errorf("the filter offers the fields %v, want Артикул and ИНН", seen.OfferedFields)
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

// absentFieldWalk looks at the restrictions of the reading of Товары, which
// have no hierarchy, of Аналитик - by Ссылка and Родитель, by ЭтоГруппа - and
// of Ревизор - by Родитель: in «Все ограничения доступа», in its filter, in
// the window of Аналитик and in the window of its restriction, then writes
// that restriction unchanged and saves the role.
const absentFieldWalk = `(async () => {` + restrictionWalkTools + `
  await overview('Все ограничения доступа');
  const seen = {};
  seen.listRows = rows().filter(item => item.textContent.includes('Товары')).map(item => [...item.children].map(cell => cell.textContent.trim()).join('|'));
  await command('filter');
  seen.filterOffers = ['Ссылка', 'Родитель', 'ЭтоГруппа'].filter(caption => !!box(caption));
  await press('Отмена');
  byText('#tree .studio-tree-row', 'Аналитик').click();
  await studio.settle();
  const editor = document.querySelector('#role-editor');
  seen.warning = !editor.querySelector('.role-warning').hidden;
  editor.querySelector('[data-object="` + demoGoods + `"]').click(); await studio.settle();
  const restrictions = () => all('.role-restrictions tbody tr', editor).filter(item => item.textContent.includes('ГДЕ')).map(item => [...item.children].map(cell => cell.textContent.trim()).join('|'));
  seen.roleRows = restrictions();
  await choose(all('.role-restrictions tbody tr', editor).find(item => item.textContent.includes('ГДЕ А')));
  editor.querySelector('.role-restrictions [data-command="edit"]').click(); await studio.settle();
  seen.windowOffers = ['Ссылка', 'Родитель', 'ЭтоГруппа'].filter(caption => !!box(caption));
  seen.windowMarked = ['Ссылка', 'Прочие поля'].filter(caption => box(caption)?.checked);
  await mark('Ссылка', false);
  seen.othersAfterLast = box('Прочие поля').checked;
  await mark('Ссылка', true);
  await press('ОК');
  document.querySelector('#save').click(); await studio.settle();
  seen.savedRows = restrictions();
  return seen;
})()`

// TestARestrictionByAFieldTheObjectHasNotIsShownAsThePrototypes catches
// Studio going its own way with a restriction naming a field the object has
// no longer - Родитель and ЭтоГруппа of a catalog with no hierarchy
// (configurator 22-23.10.2026, the eighth and the ninth passes, absent-08…14,
// absent-20…27; ACCESS-RIGHTS «Ограничение по полю, которого у объекта
// нет»): «Все ограничения доступа», its filter, the window of the role and
// the window of the restriction know no such field, a restriction naming
// only such fields is not shown, and the role is shown with no warning of
// rights it cannot keep; writing the role takes the field away and the
// restriction only by such fields with it, and a role not written keeps
// both.
// Defects caught: the field named in the lists (until 2.460) or offered by
// the filter or the window, or counted by the window as marked; a restriction only by such fields shown, as
// «Прочие поля» or by the key; the role offered «Убрать недоступные права»
// (2.440); the window of the role written with the field kept, or answered
// with it after the save; a role not written changed.
func TestARestrictionByAFieldTheObjectHasNotIsShownAsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	goods, err := uuid.Parse(demoGoods)
	if err != nil {
		t.Fatal(err)
	}
	read := metadata.PermissionRead
	written := map[string][]metadata.RightRestriction{
		"Аналитик": {{Right: read, Fields: []string{"ref", "parent"}, Condition: "ГДЕ А"}, {Right: read, Fields: []string{"isfolder"}, Condition: "ГДЕ Б"}},
		"Ревизор":  {{Right: read, Fields: []string{"parent"}, Condition: "ГДЕ В"}},
	}
	// Past Studio, which would take the fields away (a file edited by hand).
	for name, restrictions := range written {
		role := metadata.RoleDefinition{Format: metadata.CurrentFormat, ID: uuid.MustNew(), Name: name, Title: metadata.LocalizedText{"ru": name},
			Objects: []metadata.ObjectPermission{{Object: goods, Operations: []metadata.PermissionOperation{read}, Restrictions: restrictions}}}
		if err := metadata.WriteRole(root, role); err != nil {
			t.Fatal(err)
		}
	}
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, absentFieldWalk)
	if err != nil {
		t.Fatal(err)
	}
	var seen struct {
		ListRows, FilterOffers, RoleRows, WindowOffers, WindowMarked, SavedRows []string
		Warning, OthersAfterLast                                                bool
	}
	if err := json.Unmarshal(result, &seen); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	list, rows := []string{"Товары|Аналитик|Чтение|Ссылка|ГДЕ А"}, []string{"Ссылка|ГДЕ А"}
	for _, check := range []struct {
		ok   bool
		says string
	}{
		{slices.Equal(seen.ListRows, list), fmt.Sprintf("«Все ограничения доступа» list %q, want %q", seen.ListRows, list)},
		{slices.Equal(seen.FilterOffers, []string{"Ссылка"}), fmt.Sprintf("the filter offers %v, want Ссылка alone", seen.FilterOffers)},
		{!seen.Warning, "the role is shown with the warning of rights it cannot keep"},
		{slices.Equal(seen.RoleRows, rows), fmt.Sprintf("the window of the role lists %q, want %q", seen.RoleRows, rows)},
		{slices.Equal(seen.WindowOffers, []string{"Ссылка"}), fmt.Sprintf("the window of the restriction offers %v, want Ссылка alone", seen.WindowOffers)},
		{slices.Equal(seen.WindowMarked, []string{"Ссылка"}), fmt.Sprintf("the window of the restriction marks %v, want Ссылка alone", seen.WindowMarked)},
		{seen.OthersAfterLast, "the window of the restriction left «Прочие поля» clear with its last field taken away: it counts Родитель"},
		{slices.Equal(seen.SavedRows, rows), fmt.Sprintf("the window of the role lists %q after the save, want %q", seen.SavedRows, rows)},
	} {
		if !check.ok {
			t.Error(check.says)
		}
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	overview, err := workspace.ReadAllRoles()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"Аналитик": "[{read [ref] ГДЕ А}]", "Ревизор": "[{read [parent] ГДЕ В}]"}
	for _, entry := range overview.Roles {
		if got := fmt.Sprint(entry.Role.Objects[0].Restrictions); got != want[entry.Role.Name] {
			t.Errorf("role %s lies with %s, want %s", entry.Role.Name, got, want[entry.Role.Name])
		}
	}
}

// restrictionGroupFlagsWalk opens the group «Изменить» over two rows whose
// fields and conditions differ, so both flags open clear, and changes each
// half with its flag clear.
const restrictionGroupFlagsWalk = `(async () => {` + restrictionWalkTools + `
  await overview('Все ограничения доступа');
  const seen = {};
  const flags = () => ['Изменять состав полей', 'Изменять ограничение'].map(caption => box(caption).checked ? 'marked' : 'off');
  const open = async () => { await choose(row('Контрагенты')); await choose(row('Товары', 'ГДЕ А'), {ctrlKey: true}); await command('edit'); };
  await open();
  seen.openFlags = flags();
  seen.fieldOpen = !box('Код').disabled;
  seen.conditionOpen = !dialog().querySelector('textarea').disabled;
  seen.okOpen = enabled('ОК');
  await mark('Прочие поля', true);
  seen.othersFlags = flags();
  seen.okOthers = enabled('ОК');
  await press('Отмена');
  await open();
  seen.greyBefore = box('Артикул').indeterminate;
  await choose(box('Артикул'));
  seen.greyFlags = flags();
  seen.greyCleared = !box('Артикул').checked && !box('Артикул').indeterminate;
  seen.okGrey = enabled('ОК');
  await press('Отмена');
  await open();
  byText('button', 'Отметить все', dialog()).click(); await studio.settle();
  seen.allFlags = flags();
  seen.allMarked = box('Код').checked && box('Артикул').checked;
  seen.okAll = enabled('ОК');
  byText('button', 'Снять все', dialog()).click(); await studio.settle();
  seen.noneFlags = flags();
  seen.noneOthers = box('Прочие поля').checked;
  await press('Отмена');
  await open();
  await mark('Код', true);
  seen.fieldFlags = flags();
  seen.okField = enabled('ОК');
  const text = dialog().querySelector('textarea');
  text.value = 'ГДЕ Н'; text.dispatchEvent(new Event('input', {bubbles: true})); await studio.settle();
  seen.typedFlags = flags();
  await press('ОК');
  seen.closed = !dialog();
  seen.written = rows().filter(item => item.textContent.includes('ГДЕ Н')).map(item => item.textContent);
  return seen;
})()`

// TestTheGroupWindowLocksNeitherHalfByItsFlag catches the group «Изменить»
// of «Все ограничения доступа» locking the fields or the condition while
// its flag is clear (until 2.459), where the prototype's keeps both open and
// a field marked marks «Изменять состав полей» itself (configurator
// 22.10.2026, absent-06, gruppa-pole-otmechaet-flag-sam), and the condition
// typed marks «Изменять ограничение» and is written to every row, as the
// owner decided on 23.10.2026 (question В4 of the series 2.455; the
// prototype's leaves it unwritten). Caught too: «Прочие поля» marking the
// flag of the fields (gruppa-flag-snyat-prochie-polya-otmecheny), and «ОК»
// available on «Прочие поля» with the flag clear; a click on a grey field
// leaving the flag clear, and «Отметить все» or «Снять все» marking it
// (until 2.461), where the prototype's does the other way round in both
// (configurator 23.10.2026, the ninth pass, v1-*).
func TestTheGroupWindowLocksNeitherHalfByItsFlag(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, addRestrictionRoles(t, root))
	result, err := browser.evaluate(ctx, page, restrictionGroupFlagsWalk)
	if err != nil {
		t.Fatal(err)
	}
	var seen struct {
		OpenFlags, OthersFlags, FieldFlags, TypedFlags []string
		GreyFlags, AllFlags, NoneFlags                 []string
		FieldOpen, ConditionOpen, OkOpen, OkOthers     bool
		GreyBefore, GreyCleared, OkGrey                bool
		AllMarked, OkAll, NoneOthers, OkField, Closed  bool
		Written                                        []string
	}
	if err := json.Unmarshal(result, &seen); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	off, fields, both := []string{"off", "off"}, []string{"marked", "off"}, []string{"marked", "marked"}
	for _, check := range []struct {
		ok   bool
		says string
	}{
		{slices.Equal(seen.OpenFlags, off), fmt.Sprintf("the flags open %v, want both clear", seen.OpenFlags)},
		{seen.FieldOpen, "a field is locked while «Изменять состав полей» is clear"},
		{seen.ConditionOpen, "the condition is locked while «Изменять ограничение» is clear"},
		{!seen.OkOpen, "«ОК» is available before anything is changed"},
		{slices.Equal(seen.OthersFlags, off), fmt.Sprintf("«Прочие поля» left the flags %v, want both clear", seen.OthersFlags)},
		{!seen.OkOthers, "«ОК» is available on «Прочие поля» with the flag clear"},
		{seen.GreyBefore, "Артикул, named by one of the two rows, does not open grey"},
		{slices.Equal(seen.GreyFlags, fields), fmt.Sprintf("a click on the grey field left the flags %v, want %v", seen.GreyFlags, fields)},
		{seen.GreyCleared, "a click on the grey field did not take it away"},
		{!seen.OkGrey, "«ОК» is available after the grey field is taken away"},
		{slices.Equal(seen.AllFlags, off), fmt.Sprintf("«Отметить все» left the flags %v, want both clear", seen.AllFlags)},
		{seen.AllMarked, "«Отметить все» did not mark every field"},
		{!seen.OkAll, "«ОК» is available after «Отметить все» with the flag clear"},
		{slices.Equal(seen.NoneFlags, off), fmt.Sprintf("«Снять все» left the flags %v, want both clear", seen.NoneFlags)},
		{seen.NoneOthers, "«Снять все» did not mark «Прочие поля»"},
		{slices.Equal(seen.FieldFlags, fields), fmt.Sprintf("marking a field left the flags %v, want %v", seen.FieldFlags, fields)},
		{seen.OkField, "«ОК» is unavailable after a field is marked"},
		{slices.Equal(seen.TypedFlags, both), fmt.Sprintf("typing the condition left the flags %v, want %v", seen.TypedFlags, both)},
		{seen.Closed, "«ОК» did not close the window"},
		{len(seen.Written) == 2 && strings.Contains(seen.Written[0], "Код") && strings.Contains(seen.Written[1], "Код"),
			fmt.Sprintf("the rows written with the condition typed: %v, want both, with Код", seen.Written)},
	} {
		if !check.ok {
			t.Error(check.says)
		}
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
