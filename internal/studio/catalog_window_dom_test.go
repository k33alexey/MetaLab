package studio

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// objectWindowSteps are the steps every walk through the window of an object
// takes: open an object of the tree, choose a page of the window, choose a
// row of a tree of the window, find the field of a caption in the window or
// in the palette beside it. They stand before the walk's own steps.
const objectWindowSteps = `
  const treeRow = text => [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === text);
  const openObject = async text => { treeRow(text).click(); await window.__studio.settle(); };
  const windowPage = async caption => { [...document.querySelectorAll('.object-window-page')].find(item => item.textContent === caption).click(); await window.__studio.settle(); };
  const windowRow = async caption => { [...document.querySelectorAll('.object-window-body .studio-tree-row')].find(item => item.textContent.trim() === caption).click(); await window.__studio.settle(); };
  const windowField = (host, caption) => [...document.querySelectorAll(host + ' .catalog-field, ' + host + ' .catalog-check')].find(label => label.querySelector('span').textContent === caption);
`

// catalogWindowWalk opens the window of the catalog Товары and reads it as
// the prototype's window is read in the inventory: the title, the pages and
// the one chosen, the buttons at the bottom, the captions of «Основные» and
// where each stands, the page «Данные» - lengths, frames, trees, panels and
// buttons - and what a code of no length leaves available; then it adds an
// attribute by the panel, moves it round the ends, renames the catalog and
// reads «Действия».
const catalogWindowWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const host = document.querySelector('#catalog-editor');
  const button = text => [...host.querySelectorAll('.object-window-bar button')].find(item => item.textContent.startsWith(text));
  const bar = () => ({back: button('<Назад').disabled, next: button('Далее>').disabled});
  const current = () => host.querySelector('.object-window-page[aria-selected=true]').textContent;
  const captions = () => [...host.querySelectorAll('.object-window-body .catalog-field > span:first-child, .object-window-body .catalog-check > span')].map(item => item.textContent);
  const rows = () => [...host.querySelectorAll('.object-window-body .studio-tree-row')].map(item => item.textContent.trim());
  await openObject('Товары');
  steps.title = host.querySelector('.object-window-title').textContent;
  steps.pages = [...host.querySelectorAll('.object-window-page')].map(item => item.textContent);
  steps.first = current();
  steps.buttons = [...host.querySelectorAll('.object-window-bar button')].map(item => item.textContent);
  steps.firstBar = bar();
  steps.main = captions();
  const place = caption => {
    const label = windowField('#catalog-editor', caption), text = label.querySelector('span').getBoundingClientRect();
    const field = label.querySelector('.field-with-buttons, input, textarea').getBoundingClientRect();
    return text.bottom <= field.top ? 'above' : 'beside';
  };
  steps.places = ['Имя', 'Синоним', 'Комментарий', 'Представление объекта', 'Пояснение'].map(place);
  steps.explanationArea = !!windowField('#catalog-editor', 'Пояснение').querySelector('textarea');
  button('Далее>').click(); await studio.settle();
  steps.afterNext = {page: current(), empty: !host.querySelector('.object-window-body').children.length, bar: bar()};
  await windowPage('Прочее');
  steps.lastBar = bar();
  await windowPage('Данные');
  steps.data = captions();
  steps.frames = [...host.querySelectorAll('.object-window-body fieldset')].map(frame => ({legend: frame.querySelector('legend').textContent,
    choices: [...frame.querySelectorAll('label')].map(label => label.textContent), checked: frame.querySelector('input:checked')?.parentElement.textContent || '', disabled: frame.disabled}));
  steps.rows = rows();
  steps.panels = [...host.querySelectorAll('.object-window-panel')].map(panel => [...panel.querySelectorAll('button')].map(item => item.title + (item.disabled ? ' (недоступна)' : '')));
  steps.windows = [...host.querySelectorAll('.object-window-buttons button')].map(item => item.textContent + (item.disabled ? ' (недоступна)' : ''));
  const length = windowField('#catalog-editor', 'Длина кода').querySelector('input');
  length.value = '0'; length.dispatchEvent(new Event('change', {bubbles: true})); await studio.settle();
  steps.codeTypeAtZero = host.querySelector('.object-window-body fieldset').disabled;
  await windowPage('Нумерация');
  steps.numbering = captions();
  steps.numberingAtZero = [...host.querySelectorAll('.object-window-body .catalog-check input')].map(input => input.disabled);
  await windowPage('Данные');
  [...host.querySelectorAll('.object-window-panel')][0].querySelector('button[title=Добавить]').click(); await studio.settle();
  steps.added = {rows: rows(), chosen: host.querySelector('.object-window-body .studio-tree-row.selected')?.textContent.trim() || '',
    palette: windowField('#properties', 'Имя')?.querySelector('input').value || ''};
  const up = () => [...host.querySelectorAll('.object-window-panel')][0].querySelector('button[title="Переместить вверх"]');
  up().click(); await studio.settle();
  steps.upOnce = rows();
  up().click(); await studio.settle();
  steps.upRound = rows();
  await windowPage('Основные');
  const name = windowField('#catalog-editor', 'Имя').querySelector('input');
  name.value = 'Номенклатура'; name.dispatchEvent(new Event('input', {bubbles: true})); await studio.settle();
  steps.renamed = host.querySelector('.object-window-title').textContent;
  button('Действия').click(); await studio.settle();
  steps.actions = [...document.querySelectorAll('.context-menu .context-menu-item')].map(item => item.textContent.trim() + (item.disabled || item.getAttribute('aria-disabled') === 'true' ? ' (недоступна)' : ''));
  closeContextMenu(); await studio.settle();
  button('Закрыть').click(); await studio.settle();
  steps.closed = host.hidden;
  return steps;
})()`

// The window of a catalog is the prototype's (docs/block-00/spravochnik.md,
// «Окно редактирования»; obshchee.md, «Окно объекта»; snapshots
// spravochnik/okno-01, okno-06, okno-07, okno-menyu-deystviya): «Справочник
// <Имя>», fifteen pages at the left, «Основные» first, «<Назад» unavailable
// on the first page and «Далее>» on the last; on «Основные» the name, the
// synonym and the comment beside their captions and the presentations and
// the explanation under theirs; on «Данные» the two lengths, the frames
// «Тип кода» and «Основное представление», the trees of the attributes and
// of the table parts with their panels, and the four buttons of windows not
// built yet; at a code of no length «Тип кода», «Автонумерация» and
// «Контроль уникальности» unavailable. The expectations are written out
// from the inventory, not read from the descriptor. Defects caught: the
// window of 2.562 - a list of the parts at the left and «Справочник ·
// свойства» - kept; a page left out or moved; a button of the bottom
// missing or available where the prototype's is not; a caption beside its
// field where the prototype puts it above; the code type left available at a
// code of no length; a part added by the panel not chosen or not shown in
// the palette; a move up that stops at the top instead of going round; the
// title not following the name; «Изменить», «Скопировать», «Удалить» missing
// from «Действия» or available there.
func TestTheWindowOfACatalogIsThePrototypes(t *testing.T) {
	ctx, browser, page := openStudioInBrowser(t, copyDemoProjectForStudioTest(t), func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, catalogWindowWalk)
	if err != nil {
		t.Fatal(err)
	}
	type bar struct{ Back, Next bool }
	type frame struct {
		Legend, Checked string
		Choices         []string
		Disabled        bool
	}
	var steps struct {
		Title, First, Renamed      string
		Pages, Buttons, Main, Data []string
		Places                     []string
		ExplanationArea, Closed    bool
		FirstBar, LastBar          bar
		AfterNext                  struct {
			Page  string
			Empty bool
			Bar   bar
		}
		Frames                   []frame
		Rows, Windows, Numbering []string
		Panels                   [][]string
		CodeTypeAtZero           bool
		NumberingAtZero          []bool
		Added                    struct {
			Rows            []string
			Chosen, Palette string
		}
		UpOnce, UpRound, Actions []string
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
	expect("title", steps.Title, "Справочник Товары")
	expect("pages", steps.Pages, []string{"Основные", "Подсистемы", "Функциональные опции", "Иерархия", "Владельцы", "Данные", "Нумерация",
		"Формы", "Поле ввода", "Команды", "Макеты", "Ввод на основании", "Права", "Обмен данными", "Прочее"})
	expect("first page", steps.First, "Основные")
	expect("bottom buttons", steps.Buttons, []string{"Действия", "<Назад", "Далее>", "Закрыть", "Справка"})
	expect("bottom on the first page", steps.FirstBar, bar{Back: true})
	expect("bottom on the last page", steps.LastBar, bar{Next: true})
	expect("after «Далее>»", steps.AfterNext, struct {
		Page  string
		Empty bool
		Bar   bar
	}{"Подсистемы", true, bar{}})
	expect("«Основные»", steps.Main, []string{"Имя", "Синоним", "Комментарий", "Представление объекта", "Расширенное представление объекта",
		"Представление списка", "Расширенное представление списка", "Пояснение"})
	expect("captions beside or above", steps.Places, []string{"beside", "beside", "beside", "above", "above"})
	expect("explanation of several lines", steps.ExplanationArea, true)
	expect("«Данные»", steps.Data, []string{"Длина кода", "Длина наименования"})
	expect("frames", steps.Frames, []frame{
		{Legend: "Тип кода", Choices: []string{"Число", "Строка"}, Checked: "Строка"},
		{Legend: "Основное представление", Choices: []string{"В виде кода", "В виде наименования"}},
	})
	expect("trees", steps.Rows, []string{"Реквизиты", "Артикул", "Табличные части"})
	expect("panels", steps.Panels, [][]string{
		{"Добавить", "Изменить (недоступна)", "Удалить (недоступна)", "Переместить вверх (недоступна)", "Переместить вниз (недоступна)", "Сортировать (недоступна)"},
		{"Добавить табличную часть", "Добавить реквизит (недоступна)", "Изменить (недоступна)", "Удалить (недоступна)",
			"Переместить вверх (недоступна)", "Переместить вниз (недоступна)", "Сортировать (недоступна)"},
	})
	expect("windows of «Данные»", steps.Windows, []string{"Стандартные реквизиты (недоступна)", "Характеристики (недоступна)",
		"Общие реквизиты (недоступна)", "Дополнительные индексы (недоступна)"})
	expect("code type at a code of no length unavailable", steps.CodeTypeAtZero, true)
	expect("«Нумерация»", steps.Numbering, []string{"Автонумерация", "Контроль уникальности"})
	expect("«Нумерация» at a code of no length", steps.NumberingAtZero, []bool{true, true})
	expect("added by the panel", steps.Added, struct {
		Rows            []string
		Chosen, Palette string
	}{[]string{"Реквизиты", "Артикул", "Реквизит1", "Табличные части"}, "Реквизит1", "Реквизит1"})
	expect("moved up once", steps.UpOnce, []string{"Реквизиты", "Реквизит1", "Артикул", "Табличные части"})
	expect("moved up round the top", steps.UpRound, []string{"Реквизиты", "Артикул", "Реквизит1", "Табличные части"})
	expect("title after the name", steps.Renamed, "Справочник Номенклатура")
	if !slices.Equal(steps.Actions[len(steps.Actions)-3:], []string{"ИзменитьF2 (недоступна)", "СкопироватьF9 (недоступна)", "УдалитьDel (недоступна)"}) {
		t.Errorf("«Действия» of the window: %q, want the object's commands and then «Изменить», «Скопировать», «Удалить» unavailable", steps.Actions)
	}
	if !slices.Contains(steps.Actions, "Открыть модуль объекта") || !slices.Contains(steps.Actions, "Открыть модуль менеджера") {
		t.Errorf("«Действия» of the window: %q, want the commands of the object's modules", steps.Actions)
	}
	expect("closed by «Закрыть»", steps.Closed, true)
}

// catalogPagesWalk reads the pages «Иерархия», «Владельцы» and «Нумерация»
// of the window of the catalog Товары: what stands on each and what is
// unavailable, before and after the flags that open the rest; on
// «Владельцы» it opens «Выбор объекта», reads its branches, and chooses the
// catalog itself, then Контрагенты (owned by Товары), then Справочник,
// reading the refusal, the list and the use of subordination after each.
const catalogPagesWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const host = document.querySelector('#catalog-editor');
  const body = () => host.querySelector('.object-window-body');
  const captions = () => [...body().querySelectorAll('.catalog-field > span:first-child, .catalog-check > span, legend, .object-window-list-head > span')].map(item => item.textContent);
  const unavailable = () => [...body().querySelectorAll('input, select, fieldset')].filter(item => item.disabled).map(item =>
    item.tagName === 'FIELDSET' ? item.querySelector('legend').textContent : item.closest('label').querySelector('span').textContent);
  const box = caption => windowField('#catalog-editor', caption).querySelector('input');
  const click = async element => { element.click(); await studio.settle(); };
  const top = () => [...document.querySelectorAll('dialog[open]')].at(-1);
  const dialogCaption = () => top()?.querySelector('h3').textContent || '';
  // A dialog closes by an event of its own, after the click: the request
  // its answer makes starts only then.
  const press = async text => { [...top().querySelectorAll('button')].find(button => button.textContent === text).click(); await new Promise(resolve => setTimeout(resolve, 50)); await studio.settle(); };
  const choose = async name => {
    await click(host.querySelector('.object-window-list-head button[title=Изменить]'));
    const rows = () => [...top().querySelectorAll('.object-choice-tree .studio-tree-row')];
    if (!steps.choice) steps.choice = {title: dialogCaption(), branches: [...top().querySelectorAll('.object-choice-tree > details > .studio-tree-row')].map(row => row.textContent.trim()),
      common: [...top().querySelectorAll('.object-choice-tree > details:first-child .studio-tree-node .studio-tree-row')].map(row => row.textContent.trim())};
    top().querySelectorAll('.object-choice-tree details').forEach(item => { item.open = true; });
    const row = rows().find(row => row.textContent.trim() === name);
    if (!row) throw new Error('no row ' + name + ' among ' + JSON.stringify(rows().map(item => item.textContent)) + ' dialog ' + dialogCaption());
    row.querySelector('input').click();
    await press('ОК');
    const refusal = dialogCaption();
    if (refusal) await press('ОК');
    return {refusal, owners: [...body().querySelectorAll('.object-window-list-row')].map(row => row.textContent),
      subordination: !windowField('#catalog-editor', 'Использование подчинения:').querySelector('select').disabled};
  };
  await openObject('Товары');
  await windowPage('Иерархия');
  steps.hierarchy = captions();
  steps.hierarchyOff = unavailable();
  await click(box('Иерархический справочник'));
  steps.hierarchyOn = {unavailable: unavailable(), kind: windowField('#catalog-editor', 'Вид иерархии:').querySelector('select').value};
  await click(box('Ограничение количества уровней иерархии'));
  steps.limited = {unavailable: unavailable(), levels: windowField('#catalog-editor', 'Количество уровней иерархии').querySelector('input').value,
    ceiling: windowField('#catalog-editor', 'Количество уровней иерархии').querySelector('input').max};
  await windowPage('Владельцы');
  steps.owners = captions();
  steps.ownersEmpty = {remove: host.querySelector('.object-window-list-head button[title=Удалить]').disabled, unavailable: unavailable()};
  steps.itself = await choose('Товары');
  steps.ring = await choose('Контрагенты');
  steps.taken = await choose('Справочник');
  await click(host.querySelector('.object-window-list-head button[title=Удалить]'));
  steps.removed = [...body().querySelectorAll('.object-window-list-row')].map(row => row.textContent);
  await windowPage('Нумерация');
  steps.numbering = captions();
  steps.series = [...body().querySelectorAll('fieldset label')].map(label => label.textContent);
  await windowPage('Данные');
  const length = windowField('#catalog-editor', 'Длина кода').querySelector('input');
  length.value = '0'; length.dispatchEvent(new Event('change', {bubbles: true})); await studio.settle();
  await windowPage('Нумерация');
  steps.numberingAtZero = unavailable();
  return steps;
})()`

// The pages «Иерархия», «Владельцы» and «Нумерация» of the window of a
// catalog are the prototype's (docs/block-00/spravochnik.md, «Окно
// редактирования», «Проверки»; snapshots okno-04, ierarhiya-vklyuchena,
// okno-05, vladelcy-pyat-vidov, vladelcy-otkaz-*, okno-07, novyy-numeraciya):
// without the flag of the hierarchy its four settings are unavailable, with
// it the count only while the levels are not limited, a hierarchy switched on
// is one of folders and items, a limit switched on is of two levels by the
// model's ceiling of 10; without owners «Удалить» and the use of
// subordination are unavailable; «Выбор объекта» offers the five kinds that
// may own a catalog, exchange plans inside «Общие»; the catalog itself and a
// catalog it owns are refused in the prototype's words and not taken, another
// is taken by its full name; at a code of no length the numbering is
// unavailable whole. The expectations are written out from the inventory.
// Defects caught: a setting hidden instead of unavailable, or available
// without its flag; a hierarchy switched on with no kind, which the model
// refuses on save; a ring or the catalog itself taken as an owner; the window
// of the owners offering kinds that cannot own, or out of the tree's order;
// the series of codes left available at a code of no length.
func TestTheHierarchyOwnersAndNumberingOfACatalogAreThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	prepare := writeEveryRestrictableKind(t, root)
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		prepare(workspace)
		goods, err := workspace.ReadCatalogEditor("metadata/catalogs/Товары/object.yaml")
		if err != nil {
			t.Fatal(err)
		}
		partners, err := workspace.ReadCatalogEditor("metadata/catalogs/Контрагенты/object.yaml")
		if err != nil {
			t.Fatal(err)
		}
		partners.Catalog.Owners = []uuid.UUID{goods.Catalog.ID}
		if _, err := workspace.SaveCatalogEditor(partners.Path, partners.Catalog, partners.Revision); err != nil {
			t.Fatal(err)
		}
	})
	result, err := browser.evaluate(ctx, page, catalogPagesWalk)
	if err != nil {
		t.Fatal(err)
	}
	type chosen struct {
		Refusal       string
		Owners        []string
		Subordination bool
	}
	var steps struct {
		Hierarchy, HierarchyOff, Owners, Numbering, Series, NumberingAtZero, Removed []string
		HierarchyOn                                                                  struct {
			Unavailable []string
			Kind        string
		}
		Limited struct {
			Unavailable     []string
			Levels, Ceiling string
		}
		OwnersEmpty struct {
			Remove      bool
			Unavailable []string
		}
		Choice struct {
			Title            string
			Branches, Common []string
		}
		Itself, Ring, Taken chosen
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
	settings := []string{"Вид иерархии:", "Размещать группы сверху", "Ограничение количества уровней иерархии", "Количество уровней иерархии"}
	expect("«Иерархия»", steps.Hierarchy, append([]string{"Иерархический справочник"}, settings...))
	expect("«Иерархия» without the flag", steps.HierarchyOff, settings)
	expect("«Иерархия» with the flag", steps.HierarchyOn, struct {
		Unavailable []string
		Kind        string
	}{[]string{"Количество уровней иерархии"}, "folders-and-items"})
	expect("«Иерархия» limited", steps.Limited, struct {
		Unavailable     []string
		Levels, Ceiling string
	}{[]string{}, "2", "10"})
	expect("«Владельцы»", steps.Owners, []string{"Список владельцев справочника:", "Использование подчинения:"})
	expect("«Владельцы» without owners", steps.OwnersEmpty, struct {
		Remove      bool
		Unavailable []string
	}{true, []string{"Использование подчинения:"}})
	expect("«Выбор объекта»", steps.Choice, struct {
		Title            string
		Branches, Common []string
	}{"Выбор объекта", []string{"Общие", "Справочники", "Планы видов характеристик", "Планы счетов", "Планы видов расчёта"}, []string{"Планы обмена", "Обмен"}})
	expect("the catalog itself", steps.Itself, chosen{"Данный справочник не может быть установлен в качестве владельца! (Товары)", []string{}, false})
	expect("a catalog it owns", steps.Ring, chosen{"Данный справочник не может быть установлен в качестве владельца! (Контрагенты)", []string{}, false})
	expect("another catalog", steps.Taken, chosen{"", []string{"Справочник.Справочник"}, true})
	expect("after «Удалить»", steps.Removed, []string{})
	expect("«Нумерация»", steps.Numbering, []string{"Автонумерация", "Контроль уникальности", "Серии кодов"})
	expect("series", steps.Series, []string{"Во всем справочнике", "В пределах подчинения", "В пределах подчинения владельцу"})
	expect("«Нумерация» at a code of no length", steps.NumberingAtZero, []string{"Автонумерация", "Контроль уникальности", "Серии кодов"})
}

// paletteWalk opens the catalog Товары and reads its palette: the title,
// the buttons, the tabs, the rows of «Основные»; then «Показывать только
// важные», the search by «модуль», the alphabet, a name typed in the palette
// and «Отменить редактирование», and «Открыть» of the module of the object.
const paletteWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const palette = () => document.querySelector('#properties .object-palette');
  const tabs = () => [...palette().querySelectorAll('.studio-tab')].map(tab => tab.textContent);
  const rows = () => [...palette().querySelectorAll('.object-palette-row .object-palette-caption')].map(item => item.textContent);
  const button = title => palette().querySelector('.object-palette-bar button[title="' + title + '"]');
  const click = async element => { element.click(); await studio.settle(); };
  await openObject('Товары');
  steps.title = document.querySelector('#inspector-title').textContent;
  steps.buttons = [...palette().querySelectorAll('.object-palette-bar button')].map(item => item.title + (item.disabled ? ' (недоступна)' : ''));
  steps.tabs = tabs();
  steps.chosen = palette().querySelector('.studio-tab[aria-selected=true]').textContent;
  steps.main = rows();
  await click(button('Показывать только важные'));
  steps.important = tabs();
  await click(button('Показывать только важные'));
  const search = palette().querySelector('.object-palette-search input');
  search.value = 'модуль'; search.dispatchEvent(new Event('input', {bubbles: true})); await studio.settle();
  steps.found = {rows: rows(), tabs: tabs(), marked: [...palette().querySelectorAll('.object-palette-match')].map(item => item.textContent)};
  search.value = ''; search.dispatchEvent(new Event('input', {bubbles: true})); await studio.settle();
  await click(button('Сортировка по алфавиту'));
  steps.alphabet = {rows: rows().filter(caption => ['Имя', 'Синоним', 'Комментарий', 'Модуль объекта', 'Модуль менеджера', 'Длина кода'].includes(caption)), tabs: tabs(),
    count: rows().length, sorted: rows().every((caption, at, all) => !at || all[at - 1].localeCompare(caption, 'ru') <= 0)};
  await click(button('Сортировка по категориям'));
  const name = [...palette().querySelectorAll('.object-palette-row')].find(row => row.textContent.startsWith('Имя')).querySelector('input');
  name.focus(); name.value = 'ТоварыСклада'; name.dispatchEvent(new Event('input', {bubbles: true})); await studio.settle();
  steps.typed = {title: document.querySelector('#catalog-editor .object-window-title').textContent, cancel: button('Отменить редактирование').disabled,
    window: windowField('#catalog-editor', 'Имя').querySelector('input').value};
  await click(button('Отменить редактирование'));
  steps.cancelled = {value: name.value, title: document.querySelector('#catalog-editor .object-window-title').textContent};
  const inWindow = windowField('#catalog-editor', 'Комментарий').querySelector('input');
  inWindow.focus(); inWindow.value = 'для склада'; inWindow.dispatchEvent(new Event('input', {bubbles: true})); await studio.settle();
  steps.fromWindow = [...palette().querySelectorAll('.object-palette-row')].find(item => item.textContent.startsWith('Комментарий')).querySelector('input').value;
  const row = [...palette().querySelectorAll('.object-palette-row')].find(item => item.textContent.startsWith('Модуль объекта'));
  row.click();
  steps.explanation = palette().querySelector('.object-palette-explanation').textContent !== '';
  await click(row.querySelector('button'));
  steps.module = document.querySelector('#file-path').textContent;
  return steps;
})()`

// The palette of a catalog is the prototype's (docs/block-00/obolochka.md,
// «Палитра свойств»; spravochnik.md, «Палитра свойств»; snapshots
// palitra-osnovnye, vazhnye-osnovnye, palitra-spravochnaya-informaciya):
// «Свойства: <Имя>»; the five buttons, the two of editing unavailable until
// a field is edited; four tabs, «Основные» chosen; on it the name, the
// synonym, the comment and the two modules; «только важные» takes
// «Справочная информация» away; the search lists the matching properties of
// every tab without the tabs, the match marked; the alphabet lists them all
// in its order without the tabs; a name typed in the palette renames the
// window at once and «Отменить редактирование» brings the name back; a row
// chosen shows its explanation; «Открыть» opens the module. The
// expectations are written out from the inventory. Defects caught: the
// read-only list of the tree in place of the palette; a tab or a row
// missing; «только важные» that keeps or takes the wrong tab; a search or
// an alphabet keeping the tabs; the editing buttons available with nothing
// edited, or «Отменить» leaving the typed name; the window not following
// the palette; «Открыть» opening nothing.
func TestThePaletteOfACatalogIsThePrototypes(t *testing.T) {
	ctx, browser, page := openStudioInBrowser(t, copyDemoProjectForStudioTest(t), func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, paletteWalk)
	if err != nil {
		t.Fatal(err)
	}
	type listed struct {
		Rows, Tabs []string
		Count      int
		Sorted     bool
	}
	var steps struct {
		Title, Chosen, Module          string
		Buttons, Tabs, Main, Important []string
		Found                          struct{ Rows, Tabs, Marked []string }
		Alphabet                       listed
		Typed                          struct {
			Title, Window string
			Cancel        bool
		}
		FromWindow  string
		Cancelled   struct{ Value, Title string }
		Explanation bool
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
	expect("title", steps.Title, "Свойства: Товары")
	expect("buttons", steps.Buttons, []string{"Сортировка по алфавиту", "Сортировка по категориям", "Показывать только важные",
		"Отменить редактирование (недоступна)", "Применить (недоступна)"})
	expect("tabs", steps.Tabs, []string{"Основные", "Данные", "Представление", "Справочная информация"})
	expect("chosen tab", steps.Chosen, "Основные")
	expect("«Основные»", steps.Main, []string{"Имя", "Синоним", "Комментарий", "Модуль объекта", "Модуль менеджера"})
	expect("only important", steps.Important, []string{"Основные", "Данные", "Представление"})
	expect("search", steps.Found, struct{ Rows, Tabs, Marked []string }{[]string{"Модуль объекта", "Модуль менеджера"}, []string{}, []string{"Модуль", "Модуль"}})
	// Every row of every tab, five of «Основные» and twenty-six of «Данные».
	expect("alphabet", steps.Alphabet, listed{[]string{"Длина кода", "Имя", "Комментарий", "Модуль менеджера", "Модуль объекта", "Синоним"}, []string{}, 31, true})
	expect("typed in the palette", steps.Typed, struct {
		Title, Window string
		Cancel        bool
	}{"Справочник ТоварыСклада", "ТоварыСклада", false})
	expect("typed in the window, shown in the palette", steps.FromWindow, "для склада")
	expect("cancelled", steps.Cancelled, struct{ Value, Title string }{"Товары", "Справочник Товары"})
	expect("explanation", steps.Explanation, true)
	expect("module opened", steps.Module, "metadata/catalogs/Товары/МодульОбъекта.bsl")
}

// paletteDataWalk opens the tab «Данные» of the palette of the catalog
// Товары and reads its rows and what is unavailable; then switches the
// hierarchy on, makes the code of no length, chooses a basis in «Выбор
// объекта» and clears it, turns the history on and «только важные», reading
// the rows, the window and the values after each.
const paletteDataWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const palette = () => document.querySelector('#properties .object-palette');
  const rows = () => [...palette().querySelectorAll('.object-palette-row')];
  const captions = () => rows().map(row => row.querySelector('.object-palette-caption').textContent);
  const row = caption => rows().find(item => item.querySelector('.object-palette-caption').textContent === caption);
  const control = caption => row(caption).querySelector('input, select, button');
  const unavailable = () => rows().filter(item => [...item.querySelectorAll('input, select, button')].every(element => element.disabled))
    .map(item => item.querySelector('.object-palette-caption').textContent);
  const settle = () => new Promise(resolve => setTimeout(resolve, 50)).then(() => studio.settle());
  const change = async (caption, value) => {
    const element = control(caption);
    if (element.type === 'checkbox') element.checked = value; else element.value = value;
    element.dispatchEvent(new Event('change', {bubbles: true})); await settle();
  };
  await openObject('Товары');
  [...palette().querySelectorAll('.studio-tab')].find(tab => tab.textContent === 'Данные').click(); await settle();
  steps.captions = captions();
  steps.separators = palette().querySelectorAll('.object-palette-separator').length;
  steps.unavailable = unavailable();
  await change('Иерархический', true);
  steps.hierarchy = {kind: control('Вид иерархии').value, unavailable: unavailable().filter(caption => /иерарх|уровн|групп/i.test(caption))};
  await windowPage('Иерархия');
  steps.windowFlag = windowField('#catalog-editor', 'Иерархический справочник').querySelector('input').checked;
  await change('Длина кода', '0');
  steps.codeless = unavailable().filter(caption => /код|Контроль|Автонумерация|Серии/i.test(caption));
  row('Вводится на основании').querySelector('button[title="Вводится на основании"]').click(); await settle();
  const dialog = [...document.querySelectorAll('dialog[open]')].at(-1);
  dialog.querySelectorAll('details').forEach(item => { item.open = true; });
  [...dialog.querySelectorAll('.studio-tree-row')].find(item => item.textContent.trim() === 'Контрагенты').querySelector('input').click();
  [...dialog.querySelectorAll('button')].find(button => button.textContent === 'ОК').click(); await settle();
  steps.basis = control('Вводится на основании').value;
  row('Вводится на основании').querySelector('button[title^="Очистить"]').click(); await settle();
  steps.cleared = control('Вводится на основании').value;
  await change('История данных', 'use');
  steps.history = unavailable().filter(caption => /истори/i.test(caption));
  palette().querySelector('.object-palette-bar button[title="Показывать только важные"]').click(); await settle();
  steps.importantGone = steps.captions.filter(caption => !captions().includes(caption));
  palette().querySelector('.object-palette-bar button[title="Показывать только важные"]').click(); await settle();
  return steps;
})()`

// The tab «Данные» of the palette of a catalog is the prototype's
// (docs/block-00/spravochnik.md, «Палитра свойств»; snapshots
// palitra-dannye-vladelcy-kod0, vazhnye-dannye): its rows in the order of
// the inventory with the line after the hierarchy; the settings of the
// hierarchy unavailable without its flag, the use of subordination without
// owners, the two flags of the history without the history, the windows of
// the object not built yet unavailable; the flag of the hierarchy switched in
// the palette is the window's flag too, a hierarchy switched on is of
// folders and items; at a code of no length the type, the series, the
// control and the numbering unavailable and the allowed length available; a
// basis chosen in «Выбор объекта» is shown by its full name and the cross
// clears it; «только важные» takes the characteristics and the fields of
// the data lock away. The expectations are written out from the inventory.
// Defects caught: a row missing or out of place; a setting available without
// what it needs or hidden instead of unavailable; the palette and the window
// keeping two values of one property; a hierarchy switched on with no kind.
func TestTheDataOfThePaletteOfACatalogIsThePrototypes(t *testing.T) {
	ctx, browser, page := openStudioInBrowser(t, copyDemoProjectForStudioTest(t), func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, paletteDataWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Captions, Unavailable, Codeless, History, ImportantGone []string
		Separators                                              int
		Hierarchy                                               struct {
			Kind        string
			Unavailable []string
		}
		WindowFlag     bool
		Basis, Cleared string
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
	expect("«Данные»", steps.Captions, []string{"Иерархический", "Вид иерархии", "Ограничивать кол-во уровней", "Количество уровней",
		"Размещать группы сверху", "Владельцы", "Использование подчинения", "Длина кода", "Длина наименования", "Тип кода",
		"Допустимая длина кода", "Серии кодов", "Контроль уникальности", "Автонумерация", "Стандартные реквизиты", "Характеристики",
		"Предопределенные", "Обновление предопределенных данных", "Вводится на основании", "Поля блокировки данных",
		"Режим управления блокировкой данных", "Полнотекстовый поиск", "История данных", "Обновлять историю данных сразу после записи",
		"Выполнять обработку после записи версии истории данных", "Дополнительные индексы"})
	expect("the line after the hierarchy", steps.Separators, 1)
	expect("unavailable at first", steps.Unavailable, []string{"Вид иерархии", "Ограничивать кол-во уровней", "Количество уровней",
		"Размещать группы сверху", "Использование подчинения", "Стандартные реквизиты", "Характеристики", "Предопределенные",
		"Обновлять историю данных сразу после записи", "Выполнять обработку после записи версии истории данных", "Дополнительные индексы"})
	expect("hierarchy switched on", steps.Hierarchy, struct {
		Kind        string
		Unavailable []string
	}{"folders-and-items", []string{"Количество уровней"}})
	expect("the window's flag", steps.WindowFlag, true)
	expect("a code of no length", steps.Codeless, []string{"Тип кода", "Серии кодов", "Контроль уникальности", "Автонумерация"})
	expect("a basis chosen", steps.Basis, "Справочник.Контрагенты")
	expect("a basis cleared", steps.Cleared, "")
	expect("history on", steps.History, []string{})
	expect("«только важные» takes away", steps.ImportantGone, []string{"Характеристики", "Поля блокировки данных"})
}
