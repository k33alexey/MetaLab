package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
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
  const windowField = (host, caption) => [...document.querySelectorAll(host + ' .catalog-field, ' + host + ' .catalog-check, ' + host + ' .object-palette-row')]
    .find(label => label.querySelector('span').textContent === caption);
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
  steps.afterNext = {page: current(), bar: bar()};
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
			Page string
			Bar  bar
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
		Page string
		Bar  bar
	}{"Подсистемы", bar{}})
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
	// The last tab is captioned «Справка» (the owner's decision of 26.10.2026).
	expect("tabs", steps.Tabs, []string{"Основные", "Данные", "Представление", "Справка"})
	expect("chosen tab", steps.Chosen, "Основные")
	expect("«Основные»", steps.Main, []string{"Имя", "Синоним", "Комментарий", "Модуль объекта", "Модуль менеджера"})
	expect("only important", steps.Important, []string{"Основные", "Данные", "Представление"})
	expect("search", steps.Found, struct{ Rows, Tabs, Marked []string }{[]string{"Модуль объекта", "Модуль менеджера"}, []string{}, []string{"Модуль", "Модуль"}})
	// Every row of every tab: five of «Основные», twenty-six of «Данные»,
	// twenty of «Представление» and two of «Справка».
	expect("alphabet", steps.Alphabet, listed{[]string{"Длина кода", "Имя", "Комментарий", "Модуль менеджера", "Модуль объекта", "Синоним"}, []string{}, 53, true})
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

// palettePresentationWalk reads the tabs «Представление» and «Справка» of
// the palette of the catalog Товары, which has its main form of the object:
// the rows, the lines between groups, what is unavailable, the forms, the
// explanation of several lines, the rows «только важные» keeps; then opens
// the form by the magnifier.
const palettePresentationWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const palette = () => document.querySelector('#properties .object-palette');
  const rows = () => [...palette().querySelectorAll('.object-palette-row')];
  const captions = () => rows().map(row => row.querySelector('.object-palette-caption').textContent);
  const row = caption => rows().find(item => item.querySelector('.object-palette-caption').textContent === caption);
  const unavailable = () => rows().filter(item => [...item.querySelectorAll('input, select, textarea, button')].every(element => element.disabled))
    .map(item => item.querySelector('.object-palette-caption').textContent);
  const settle = () => new Promise(resolve => setTimeout(resolve, 50)).then(() => studio.settle());
  const tab = async caption => { [...palette().querySelectorAll('.studio-tab')].find(item => item.textContent === caption).click(); await settle(); };
  await openObject('Товары');
  await tab('Представление');
  steps.count = rows().length;
  steps.separators = palette().querySelectorAll('.object-palette-separator').length;
  steps.unavailable = unavailable();
  steps.form = row('Основная форма объекта').querySelector('input').value;
  steps.formButtons = [...row('Основная форма объекта').querySelectorAll('button')].map(button => button.title + (button.disabled ? ' (недоступна)' : ''));
  steps.explanation = !!row('Пояснение').querySelector('textarea');
  palette().querySelector('.object-palette-bar button[title="Показывать только важные"]').click(); await settle();
  steps.important = captions();
  palette().querySelector('.object-palette-bar button[title="Показывать только важные"]').click(); await settle();
  await tab('Справка');
  steps.help = captions();
  steps.helpUnavailable = unavailable();
  await tab('Представление');
  row('Основная форма объекта').querySelector('button[title="Открыть: Основная форма объекта"]').click(); await settle();
  steps.opened = document.querySelector('#file-path').textContent;
  return steps;
})()`

// The tabs «Представление» and «Справка» of the palette of a catalog are
// the prototype's (docs/block-00/spravochnik.md, «Палитра свойств»;
// snapshots palitra-predstavlenie-ne-ierarhicheskiy, vazhnye-predstavlenie,
// palitra-spravochnaya-informaciya), the last captioned «Справка» (the
// owner's decision of 26.10.2026): twenty rows in three groups; the forms of
// a group unavailable at a catalog without a hierarchy of groups; a main form
// by its name with «…», the cross and the magnifier, «…» unavailable while
// its window is not shot; the explanation of several lines; «только важные»
// keeps three rows of the input by string; «Справка» - the flag of the
// contents and the window of the help, unavailable until built; the
// magnifier opens the form. Defects caught: a row or a group missing; the
// forms of a group available without groups; «только важные» keeping more or
// less; the magnifier opening nothing.
func TestThePresentationOfThePaletteOfACatalogIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	writeProjectFile(t, root, "metadata/catalogs/Товары/forms/ФормаЭлемента/form.yaml",
		"format: 1\nid: 10000000-0000-4000-8000-00000000f101\nname: ФормаЭлемента\ntitle: {ru: Форма элемента}\nkind: object\n")
	description := filepath.Join(root, "metadata", "catalogs", "Товары", "object.yaml")
	content, err := os.ReadFile(description)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(description, append(content, []byte("forms: {object: ФормаЭлемента}\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, palettePresentationWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Count, Separators                                          int
		Unavailable, FormButtons, Important, Help, HelpUnavailable []string
		Form, Opened                                               string
		Explanation                                                bool
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
	expect("rows of «Представление»", steps.Count, 20)
	expect("lines between groups", steps.Separators, 2)
	expect("unavailable", steps.Unavailable, []string{"Основная форма группы", "Основная форма выб. группы"})
	expect("the main form of the object", steps.Form, "ФормаЭлемента")
	expect("its buttons", steps.FormButtons, []string{"Основная форма объекта (недоступна)", "Очистить: Основная форма объекта", "Открыть: Основная форма объекта"})
	expect("explanation of several lines", steps.Explanation, true)
	expect("«только важные»", steps.Important, []string{"Способ поиска строки при вводе по строке", "Полнотекстовый поиск при вводе по строке",
		"Режим получения данных выбора при вводе по строке"})
	expect("«Справка»", steps.Help, []string{"Включать в содержание справки", "Справочная информация"})
	expect("«Справка» unavailable", steps.HelpUnavailable, []string{"Справочная информация"})
	expect("the form opened", steps.Opened, "metadata/catalogs/Товары/forms/ФормаЭлемента/form.yaml")
}

// partPaletteWalk chooses in the window of the catalog Товары its string
// attribute Артикул, its number attribute Цена, its table part Состав and
// the attribute Количество of it, and reads the palette of each: the tabs,
// the rows of a tab, values; it types a length beyond the ceiling of a
// string and a length of the line number beyond its bound, answering «Да».
const partPaletteWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const palette = () => document.querySelector('#properties .object-palette');
  const settle = () => new Promise(resolve => setTimeout(resolve, 50)).then(() => studio.settle());
  const tabs = () => [...palette().querySelectorAll('.studio-tab')].map(tab => tab.textContent);
  const tab = async caption => { [...palette().querySelectorAll('.studio-tab')].find(item => item.textContent === caption).click(); await settle(); };
  const rows = () => [...palette().querySelectorAll('.object-palette-row .object-palette-caption')].map(item => item.textContent);
  const field = caption => windowField('#properties', caption).querySelector('input, select');
  const read = async () => { const result = {title: document.querySelector('#inspector-title').textContent, tabs: tabs()}; for (const caption of tabs()) { await tab(caption); result[caption] = rows(); } await tab(tabs()[0]); return result; };
  const commit = async (caption, value) => {
    const input = field(caption);
    input.focus(); input.value = value; document.querySelector('#refresh').focus(); input.dispatchEvent(new Event('change', {bubbles: true})); await settle();
    const dialog = document.querySelector('dialog[open]'), question = dialog?.querySelector('h3').textContent || '';
    if (dialog) { [...dialog.querySelectorAll('button')].find(button => button.textContent === 'Да').click(); await settle(); }
    return {question, value: field(caption).value};
  };
  await openObject('Товары');
  await windowPage('Данные');
  await windowRow('Артикул');
  steps.string = await read();
  steps.length = field('Длина').value;
  steps.beyond = await commit('Длина', '2000');
  windowField('#properties', 'Неограниченная длина').querySelector('input').click(); await settle();
  steps.unlimited = field('Длина').value;
  await tab('Использование');
  steps.indexing = field('Индексировать').value;
  await windowRow('Цена');
  steps.number = await read();
  steps.precision = [field('Длина').value, field('Точность').value];
  await windowRow('Состав');
  steps.part = await read();
  await tab('Данные');
  steps.lineNumber = await commit('Длина номера строки', '10');
  steps.lineNumberTaken = await commit('Длина номера строки', '9');
  await windowRow('Количество');
  steps.partAttribute = await read();
  return steps;
})()`

// The palette of a part chosen in the window of a catalog is the
// prototype's (docs/block-00/rekvizit.md, tablichnaya-chast.md, «Палитра»;
// snapshots spravochnik/rekvizit-*): an attribute - «Основные»,
// «Использование», «Представление», its rows by the kind of its type, the
// use only at an attribute of the catalog itself; a table part -
// «Основные», «Данные», «Использование», «Представление». A qualifier is
// written by the rules of the window of the type: a length of a string
// beyond its ceiling is asked about, «Неограниченная длина» makes it 0; the
// length of the line number takes 5 to 9. The expectations are written out
// from the inventory. Defects caught: the old panel of five fields in place
// of the palette; a row of a string at a number or back; the use at an
// attribute of a table part; a qualifier written past the model's ceiling.
func TestThePaletteOfAPartOfACatalogIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		goods, err := workspace.ReadCatalogEditor("metadata/catalogs/Товары/object.yaml")
		if err != nil {
			t.Fatal(err)
		}
		goods.Catalog.Attributes = append(goods.Catalog.Attributes, metadata.Attribute{ID: uuid.MustNew(), Name: "Цена",
			Types: []metadata.Type{{Kind: metadata.NumberType, Precision: 15, Scale: 2}}})
		goods.Catalog.TableParts = append(goods.Catalog.TableParts, metadata.TablePart{ID: uuid.MustNew(), Name: "Состав",
			Attributes: []metadata.Attribute{{ID: uuid.MustNew(), Name: "Количество", Types: []metadata.Type{{Kind: metadata.NumberType, Precision: 10}}}}})
		if _, err := workspace.SaveCatalogEditor(goods.Path, goods.Catalog, goods.Revision); err != nil {
			t.Fatal(err)
		}
	})
	result, err := browser.evaluate(ctx, page, partPaletteWalk)
	if err != nil {
		t.Fatal(err)
	}
	type answered struct{ Question, Value string }
	var steps struct {
		String, Number, Part, PartAttribute map[string]any
		Length, Unlimited, Indexing         string
		Beyond, LineNumber, LineNumberTaken answered
		Precision                           []string
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
	presentationOfAll := []string{"Заполнять из данных заполнения", "Значение заполнения", "Проверка заполнения", "Быстрый выбор",
		"Создание при вводе", "История выбора при вводе", "Связь по типу"}
	expect("an attribute of a string", steps.String, map[string]any{"title": "Свойства: Артикул", "tabs": []string{"Основные", "Использование", "Представление"},
		"Основные":      []string{"Имя", "Синоним", "Комментарий", "Тип", "Длина", "Допустимая длина", "Неограниченная длина"},
		"Использование": []string{"Использование", "Индексировать", "Полнотекстовый поиск", "История данных"},
		"Представление": append([]string{"Режим пароля", "Подсказка", "Маска", "Многострочный режим", "Расширенное редактирование"}, presentationOfAll...)})
	expect("the length of the string", steps.Length, "50")
	expect("a length beyond the ceiling", steps.Beyond, answered{"В поле введены некорректные данные. Отменить ввод?", "50"})
	expect("an unlimited length", steps.Unlimited, "0")
	expect("indexing", steps.Indexing, "index")
	expect("an attribute of a number", steps.Number, map[string]any{"title": "Свойства: Цена", "tabs": []string{"Основные", "Использование", "Представление"},
		"Основные":      []string{"Имя", "Синоним", "Комментарий", "Тип", "Длина", "Точность", "Неотрицательное"},
		"Использование": []string{"Использование", "Индексировать", "Полнотекстовый поиск", "История данных"},
		"Представление": append([]string{"Формат", "Формат редактирования", "Подсказка", "Выделять отрицательные", "Минимальное значение", "Максимальное значение"}, presentationOfAll...)})
	expect("the qualifiers of the number", steps.Precision, []string{"15", "2"})
	expect("a table part", steps.Part, map[string]any{"title": "Свойства: Состав", "tabs": []string{"Основные", "Данные", "Использование", "Представление"},
		"Основные": []string{"Имя", "Синоним", "Комментарий"}, "Данные": []string{"Стандартные реквизиты", "Длина номера строки"},
		"Использование": []string{"Использование"}, "Представление": []string{"Подсказка", "Проверка заполнения"}})
	expect("a line number of 10", steps.LineNumber.Question, "В поле введены некорректные данные. Отменить ввод?")
	expect("a line number of 9", steps.LineNumberTaken, answered{"", "9"})
	expect("an attribute of a table part", steps.PartAttribute["Использование"], []string{"Индексировать", "Полнотекстовый поиск", "История данных"})
}

// constantPaletteWalk adds a constant by «Добавить» on the branch
// «Константы» and reads what opens: the work area, the palette, its tabs and
// rows by the kind of the type, «только важные»; it types a name, changes
// the length of the string, saves, and opens the module of the value.
const constantPaletteWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const palette = () => document.querySelector('#properties .object-palette');
  const settle = () => new Promise(resolve => setTimeout(resolve, 50)).then(() => studio.settle());
  const tabs = () => [...palette().querySelectorAll('.studio-tab')].map(tab => tab.textContent);
  const tab = async caption => { [...palette().querySelectorAll('.studio-tab')].find(item => item.textContent === caption).click(); await settle(); };
  const rows = () => [...palette().querySelectorAll('.object-palette-row .object-palette-caption')].map(item => item.textContent);
  const field = caption => windowField('#properties', caption).querySelector('input, select');
  await openObject('Константы');
  document.querySelector('#tree-add').click(); await settle();
  steps.dialog = !!document.querySelector('dialog[open]');
  steps.tree = !!treeRow('Константа1');
  steps.workArea = [...document.querySelectorAll('main > div')].filter(item => !item.hidden && item.offsetParent !== null && item.id !== 'editor-host' && !item.classList.contains('editor-bar')).map(item => item.id);
  steps.title = document.querySelector('#inspector-title').textContent;
  steps.tabs = tabs();
  steps.main = rows();
  steps.values = {name: field('Имя').value, length: field('Длина').value, fixed: field('Допустимая длина').value};
  await tab('Данные');
  steps.data = {rows: rows(), lock: field('Режим управления блокировкой данных').value, history: field('История данных').value,
    flags: [...palette().querySelectorAll('.object-palette-row input[type=checkbox]')].map(box => box.disabled)};
  await tab('Представление');
  steps.presentation = rows();
  palette().querySelector('.object-palette-bar button[title="Показывать только важные"]').click(); await settle();
  steps.important = tabs();
  palette().querySelector('.object-palette-bar button[title="Показывать только важные"]').click(); await settle();
  await tab('Основные');
  const name = field('Имя');
  name.focus(); name.value = 'ОсновнаяВалюта'; name.dispatchEvent(new Event('input', {bubbles: true})); await settle();
  steps.renamed = {title: document.querySelector('#inspector-title').textContent, synonym: field('Синоним').value};
  const length = field('Длина');
  length.focus(); length.value = '25'; document.querySelector('#refresh').focus(); length.dispatchEvent(new Event('change', {bubbles: true})); await settle();
  document.querySelector('#save').click(); await settle();
  steps.saved = {unsaved: !document.querySelector('#save').disabled, tree: !!treeRow('ОсновнаяВалюта'), length: field('Длина').value};
  [...palette().querySelectorAll('.object-palette-row')].find(row => row.textContent.startsWith('Модуль менеджера значения')).querySelector('button').click(); await settle();
  steps.module = document.querySelector('#file-path').textContent;
  await openObject('Константы');
  document.querySelector('#tree-add').click(); await settle();
  const another = field('Имя');
  another.focus(); another.value = 'основнаявалюта'; another.dispatchEvent(new Event('input', {bubbles: true}));
  document.querySelector('#refresh').focus(); another.dispatchEvent(new Event('change', {bubbles: true})); await settle();
  steps.repeated = document.querySelector('dialog[open] h3')?.textContent || '';
  return steps;
})()`

// A constant has no window: «Добавить» makes Константа1 with the
// prototype's values of a new one and opens its palette, the work area left
// empty (docs/block-00/konstanta.md, «Окно редактирования», «Новая
// константа»; snapshots konstanta/novaya-*, palitra-*): three tabs, «только
// важные» keeping «Основные» and «Данные»; the rows of a string; the managed
// lock and the history not used, its flags unavailable; the name typed
// renames the palette and the synonym follows it; a length typed is written
// and saved; the module of the value opens from its row. The expectations are
// written out from the inventory. Defects caught: «Добавить» refused on the
// branch of constants; a new constant without the prototype's values; a
// window of a constant; the palette of a constant missing its rows, or the
// rows of another kind of type; an edit of the palette not saved.
func TestThePaletteOfAConstantIsThePrototypes(t *testing.T) {
	ctx, browser, page := openStudioInBrowser(t, copyDemoProjectForStudioTest(t), func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, constantPaletteWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Dialog, Tree            bool
		WorkArea, Tabs, Main    []string
		Presentation, Important []string
		Title, Module           string
		Values                  struct{ Name, Length, Fixed string }
		Data                    struct {
			Rows          []string
			Lock, History string
			Flags         []bool
		}
		Renamed struct{ Title, Synonym string }
		Saved   struct {
			Unsaved, Tree bool
			Length        string
		}
		Repeated string
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
	expect("«Добавить»", []bool{steps.Dialog, steps.Tree}, []bool{false, true})
	expect("the work area", steps.WorkArea, []string{})
	expect("title", steps.Title, "Свойства: Константа1")
	expect("tabs", steps.Tabs, []string{"Основные", "Данные", "Представление"})
	expect("«Основные» of a string", steps.Main, []string{"Имя", "Синоним", "Комментарий", "Тип", "Длина", "Допустимая длина", "Неограниченная длина",
		"Модуль менеджера", "Модуль менеджера значения"})
	expect("a new constant", steps.Values, struct{ Name, Length, Fixed string }{"Константа1", "10", "false"})
	expect("«Данные»", steps.Data, struct {
		Rows          []string
		Lock, History string
		Flags         []bool
	}{[]string{"Режим управления блокировкой данных", "История данных", "Обновлять историю данных сразу после записи",
		"Выполнять обработку после записи версии истории данных"}, "managed", "dont-use", []bool{true, true}})
	expect("«Представление» of a string", steps.Presentation, []string{"Использовать стандартные команды", "Основная форма", "Расширенное представление",
		"Пояснение", "Режим пароля", "Подсказка", "Маска", "Многострочный режим", "Расширенное редактирование", "Проверка заполнения",
		"Быстрый выбор", "История выбора при вводе", "Связь по типу"})
	expect("«только важные»", steps.Important, []string{"Основные", "Данные"})
	expect("renamed", steps.Renamed, struct{ Title, Synonym string }{"Свойства: ОсновнаяВалюта", "Основная валюта"})
	expect("saved", steps.Saved, struct {
		Unsaved, Tree bool
		Length        string
	}{false, true, "25"})
	// The name of a constant is checked against the other constants, as
	// every name in its branch (obshchee.md, «Имя и синоним»).
	expect("a name of another constant", steps.Repeated, "Имя не уникально!")
	expect("the module of the value", steps.Module, "metadata/constants/ОсновнаяВалюта/МодульЗначения.bsl")
}

// rowsPagesWalk reads the pages «Поле ввода» and «Прочее» of the window of
// the catalog Товары: captions, the frame, buttons and what is unavailable;
// it marks «Быстрый выбор» and reads the palette, turns the history on and
// opens the object module by its button.
const rowsPagesWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const host = document.querySelector('#catalog-editor');
  const body = () => host.querySelector('.object-window-body');
  const settle = () => new Promise(resolve => setTimeout(resolve, 50)).then(() => studio.settle());
  const captions = () => [...body().querySelectorAll('.catalog-field > span:first-child, .catalog-check > span:last-child')].map(item => item.textContent);
  await openObject('Товары');
  await windowPage('Поле ввода');
  steps.input = captions();
  steps.frame = [...body().querySelectorAll('fieldset')].map(frame => ({legend: frame.querySelector('legend').textContent,
    rows: [...frame.querySelectorAll('.catalog-field > span:first-child')].map(item => item.textContent)}));
  windowField('#catalog-editor', 'Быстрый выбор').querySelector('input').click(); await settle();
  [...document.querySelectorAll('#properties .studio-tab')].find(tab => tab.textContent === 'Представление').click(); await settle();
  steps.palette = windowField('#properties', 'Быстрый выбор').querySelector('input').checked;
  await windowPage('Прочее');
  steps.other = captions();
  steps.buttons = [...body().querySelectorAll('.object-window-button')].map(button => button.textContent + (button.disabled ? ' (недоступна)' : ''));
  const flags = () => ['Обновлять историю данных сразу после записи', 'Выполнять обработку после записи версии истории данных']
    .map(caption => windowField('#catalog-editor', caption).querySelector('input').disabled);
  steps.withoutHistory = flags();
  const history = windowField('#catalog-editor', 'История данных:').querySelector('select');
  history.value = 'use'; history.dispatchEvent(new Event('change', {bubbles: true})); await settle();
  steps.withHistory = flags();
  [...body().querySelectorAll('.object-window-button')].find(button => button.textContent === 'Модуль объекта').click(); await settle();
  steps.module = document.querySelector('#file-path').textContent;
  return steps;
})()`

// The pages «Поле ввода» and «Прочее» of the window of a catalog are the
// prototype's (docs/block-00/spravochnik.md, «Окно редактирования», pages 9
// and 15; snapshots okno-09, okno-15), built of the palette's rows: the
// window's captions, the frame «Ввод по строке» of four rows, the buttons of
// the modules, the predefined items and the help - the last two unavailable
// until their windows are built; the flags of the history unavailable
// without it. A flag marked in the window is the palette's too, and a module
// opens by its button. Defects caught: a row missing or out of its frame; a
// value of the window kept apart from the palette's; the flags of the
// history available without it; a button of a module opening nothing.
func TestThePagesOfACatalogBuiltOfThePalette(t *testing.T) {
	ctx, browser, page := openStudioInBrowser(t, copyDemoProjectForStudioTest(t), func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, rowsPagesWalk)
	if err != nil {
		t.Fatal(err)
	}
	type frame struct {
		Legend string
		Rows   []string
	}
	var steps struct {
		Input, Other, Buttons       []string
		Frame                       []frame
		Palette                     bool
		WithoutHistory, WithHistory []bool
		Module                      string
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
	expect("«Поле ввода»", steps.Input, []string{"Быстрый выбор", "Создание при вводе:", "Ввод по строке:", "Способ поиска строки:", "Полнотекстовый поиск:",
		"Режим получения данных выбора:", "История выбора при вводе:"})
	expect("the frame", steps.Frame, []frame{{"Ввод по строке", []string{"Ввод по строке:", "Способ поиска строки:", "Полнотекстовый поиск:", "Режим получения данных выбора:"}}})
	expect("the palette follows the window", steps.Palette, true)
	expect("«Прочее»", steps.Other, []string{"Обновление предопределенных данных:", "Поля блокировки данных:", "Полнотекстовый поиск:", "История данных:",
		"Обновлять историю данных сразу после записи", "Выполнять обработку после записи версии истории данных", "Включать в содержание справки"})
	expect("buttons", steps.Buttons, []string{"Модуль объекта", "Модуль менеджера", "Предопределенные (недоступна)", "Справочная информация (недоступна)"})
	expect("history off", steps.WithoutHistory, []bool{true, true})
	expect("history on", steps.WithHistory, []bool{false, false})
	expect("the module", steps.Module, "metadata/catalogs/Товары/МодульОбъекта.bsl")
}

// partsPagesWalk reads the pages «Формы», «Команды» and «Макеты» of the
// window of the catalog Товары, which has a form, two commands and two
// templates: the frame of the main forms, the trees, the panels; it opens
// the form, moves a command round the top, saves, and opens the command's
// module.
const partsPagesWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const host = document.querySelector('#catalog-editor');
  const body = () => host.querySelector('.object-window-body');
  const settle = () => new Promise(resolve => setTimeout(resolve, 50)).then(() => studio.settle());
  const rows = () => [...body().querySelectorAll('.studio-tree-row')].map(item => item.textContent.trim());
  const buttons = () => [...body().querySelectorAll('.object-window-panel button')].map(button => button.title + (button.disabled ? ' (недоступна)' : ''));
  await openObject('Товары');
  await windowPage('Формы');
  steps.frame = {legend: body().querySelector('fieldset legend').textContent,
    rows: [...body().querySelectorAll('fieldset .catalog-field > span:first-child')].map(item => item.textContent),
    unavailable: [...body().querySelectorAll('fieldset .catalog-field')].filter(label => label.querySelector('input').disabled).map(label => label.querySelector('span').textContent),
    object: windowField('#catalog-editor', 'Элемента:').querySelector('input').value};
  steps.forms = rows();
  await windowRow('ФормаЭлемента');
  steps.formPanel = buttons();
  await windowPage('Команды');
  steps.commands = rows();
  steps.flag = !!windowField('#catalog-editor', 'Использовать стандартные команды');
  await windowRow('Печать');
  steps.commandPanel = buttons();
  body().querySelector('.object-window-panel button[title="Переместить вверх"]').click(); await settle();
  steps.movedUp = rows();
  await windowPage('Макеты');
  steps.templates = rows();
  steps.designers = [...body().querySelectorAll('.menu-button')].map(button => button.textContent + (button.disabled ? ' (недоступна)' : ''));
  document.querySelector('#save').click(); await settle();
  steps.saved = document.querySelector('#save').disabled;
  await windowPage('Команды');
  await windowRow('Подбор');
  body().querySelector('.object-window-panel button[title=Изменить]').click(); await settle();
  steps.module = document.querySelector('#file-path').textContent;
  await openObject('Товары');
  await windowPage('Формы');
  await windowRow('ФормаЭлемента');
  body().querySelector('.object-window-panel button[title=Изменить]').click(); await settle();
  steps.form = document.querySelector('#file-path').textContent;
  return steps;
})()`

// The pages «Формы», «Команды» and «Макеты» of the window of a catalog are
// the prototype's (docs/block-00/spravochnik.md, «Окно редактирования»,
// pages 8, 10 and 11; snapshots okno-08, okno-10, okno-11): the frame of
// five main forms - those of a group unavailable at a catalog without
// groups - and the tree of the forms; the tree of the commands and the flag
// of the standard commands; the tree of the templates and «Конструкторы».
// A form opens by «Изменить», a command its module; a command moves round
// the top and the order is saved. Defects caught: a page left empty; a form
// of a group available without groups; a part of another object in a tree;
// «Изменить» opening nothing; a move not kept by the save.
func TestTheFormsCommandsAndTemplatesOfACatalogAreThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	writeProjectFile(t, root, "metadata/catalogs/Товары/forms/ФормаЭлемента/form.yaml",
		"format: 1\nid: 10000000-0000-4000-8000-00000000f101\nname: ФормаЭлемента\ntitle: {ru: Форма элемента}\nkind: object\n")
	writeProjectFile(t, root, "metadata/catalogs/Товары/commands/Подбор/МодульКоманды.bsl", "\n")
	writeProjectFile(t, root, "metadata/catalogs/Товары/commands/Печать/МодульКоманды.bsl", "\n")
	description := filepath.Join(root, "metadata", "catalogs", "Товары", "object.yaml")
	content, err := os.ReadFile(description)
	if err != nil {
		t.Fatal(err)
	}
	content = append(content, []byte("forms: {object: ФормаЭлемента}\n"+
		"commands:\n  - {id: "+uuid.MustNew().String()+", name: Подбор, title: {ru: Подбор}, group: form-command-bar-important}\n"+
		"  - {id: "+uuid.MustNew().String()+", name: Печать, title: {ru: Печать}, group: form-command-bar-important}\n"+
		"templates:\n  - {id: "+uuid.MustNew().String()+", name: Этикетка, title: {ru: Этикетка}, kind: spreadsheet}\n"+
		"  - {id: "+uuid.MustNew().String()+", name: Ценник, title: {ru: Ценник}, kind: spreadsheet}\n")...)
	if err := os.WriteFile(description, content, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, partsPagesWalk)
	if err != nil {
		t.Fatal(err)
	}
	type frame struct {
		Legend      string
		Rows        []string
		Unavailable []string
		Object      string
	}
	var steps struct {
		Frame                                    frame
		Forms, FormPanel, Commands, CommandPanel []string
		MovedUp, Templates, Designers            []string
		Flag, Saved                              bool
		Module, Form                             string
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
	expect("the frame of the forms", steps.Frame, frame{"Формы", []string{"Элемента:", "Группы:", "Списка:", "Выбора:", "Выбора группы:"},
		[]string{"Группы:", "Выбора группы:"}, "ФормаЭлемента"})
	expect("the tree of the forms", steps.Forms, []string{"Формы", "ФормаЭлемента"})
	expect("the panel of a form", steps.FormPanel, []string{"Добавить (недоступна)", "Изменить", "Удалить (недоступна)",
		"Переместить вверх (недоступна)", "Переместить вниз (недоступна)", "Сортировать (недоступна)"})
	expect("the tree of the commands", steps.Commands, []string{"Команды", "Подбор", "Печать"})
	expect("the flag of the standard commands", steps.Flag, true)
	expect("the panel of a command", steps.CommandPanel, []string{"Добавить (недоступна)", "Изменить", "Удалить (недоступна)",
		"Переместить вверх", "Переместить вниз", "Сортировать (недоступна)"})
	expect("a command moved up", steps.MovedUp, []string{"Команды", "Печать", "Подбор"})
	expect("the tree of the templates", steps.Templates, []string{"Макеты", "Этикетка", "Ценник"})
	expect("«Конструкторы»", steps.Designers, []string{"Конструкторы (недоступна)"})
	expect("saved", steps.Saved, true)
	expect("the module of a command", steps.Module, "metadata/catalogs/Товары/commands/Подбор/МодульКоманды.bsl")
	expect("the form", steps.Form, "metadata/catalogs/Товары/forms/ФормаЭлемента/form.yaml")
	saved, err := os.ReadFile(description)
	if err != nil {
		t.Fatal(err)
	}
	if text := string(saved); !strings.Contains(text, "name: Печать") || strings.Index(text, "name: Печать") > strings.Index(text, "name: Подбор") {
		t.Errorf("the order of the commands is not saved:\n%s", text)
	}
}

// subsystemsPageWalk opens the page «Подсистемы» of the window of the
// catalog Товары, which lies in Склад under Продажи: reads the tree and its
// marks; marks Продажи and unmarks it - the window stays unchanged - then
// marks Продажи, unmarks Склад, saves and opens the window anew.
const subsystemsPageWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const host = document.querySelector('#catalog-editor');
  const body = () => host.querySelector('.object-window-body');
  const settle = () => new Promise(resolve => setTimeout(resolve, 50)).then(() => studio.settle());
  const rows = () => [...body().querySelectorAll('.studio-tree-row')].filter(row => row.offsetParent !== null)
    .map(row => row.textContent.trim() + (row.querySelector('input').checked ? ' ✓' : ''));
  const box = name => [...body().querySelectorAll('.studio-tree-row')].find(row => row.textContent.trim() === name).querySelector('input');
  await openObject('Товары');
  await windowPage('Подсистемы');
  steps.caption = body().querySelector('.object-window-caption').textContent;
  steps.shown = rows();
  box('Продажи').click(); await settle();
  steps.marked = !document.querySelector('#save').disabled;
  box('Продажи').click(); await settle();
  steps.unmarked = !document.querySelector('#save').disabled;
  box('Продажи').click(); box('Склад').click(); await settle();
  document.querySelector('#save').click(); await settle();
  steps.saved = document.querySelector('#save').disabled;
  await openObject('Контрагенты');
  await openObject('Товары');
  await windowPage('Подсистемы');
  body().querySelectorAll('details').forEach(item => { item.open = true; });
  steps.reopened = rows();
  return steps;
})()`

// The page «Подсистемы» of the window of a catalog is the prototype's
// (docs/block-00/spravochnik.md, page 2; snapshot okno-02): the caption, the
// subsystems by their parents, a box at each, marked where the catalog is,
// the one holding a marked subsystem open; a mark changes the window and
// the mark taken back leaves it as it was; «Сохранить» writes the marks
// into the subsystems and the window opened anew shows them. Defects
// caught: the page empty; a subsystem out of its parent; a mark of a
// subsystem the catalog is not in; a mark that leaves the window unchanged
// or a mark taken back that leaves it changed; a mark the save does not
// write.
func TestTheSubsystemsPageOfACatalogIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addTreeSubsystems(t, root)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, subsystemsPageWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Caption                 string
		Shown, Reopened         []string
		Marked, Unmarked, Saved bool
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
	expect("caption", steps.Caption, "Подсистемы, в которых участвует справочник:")
	expect("the tree", steps.Shown, []string{"Продажи", "Склад ✓"})
	expect("a mark changes the window", steps.Marked, true)
	expect("a mark taken back", steps.Unmarked, false)
	expect("saved", steps.Saved, true)
	expect("opened anew", steps.Reopened, []string{"Продажи ✓", "Склад"})
}

// optionsPageWalk opens the page «Функциональные опции» of the window of the
// catalog Товары, which is in the content of Склады and has its attribute
// in Артикулы: reads the list and its marks; marks Артикулы and unmarks it -
// the window stays unchanged - then marks Артикулы, unmarks Склады, saves
// and opens the window anew.
const optionsPageWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const host = document.querySelector('#catalog-editor');
  const body = () => host.querySelector('.object-window-body');
  const settle = () => new Promise(resolve => setTimeout(resolve, 50)).then(() => studio.settle());
  const rows = () => [...body().querySelectorAll('.check-list-row')]
    .map(row => row.textContent.trim() + (row.querySelector('input').checked ? ' ✓' : ''));
  const box = name => [...body().querySelectorAll('.check-list-row')].find(row => row.textContent.trim() === name).querySelector('input');
  await openObject('Товары');
  await windowPage('Функциональные опции');
  steps.caption = body().querySelector('.object-window-caption').textContent;
  steps.shown = rows();
  steps.icons = body().querySelectorAll('.check-list-row svg').length;
  steps.current = body().querySelector('.check-list-row.current')?.textContent;
  box('Артикулы').click(); await settle();
  steps.marked = !document.querySelector('#save').disabled;
  box('Артикулы').click(); await settle();
  steps.unmarked = !document.querySelector('#save').disabled;
  box('Артикулы').click(); box('Склады').click(); await settle();
  document.querySelector('#save').click(); await settle();
  steps.saved = document.querySelector('#save').disabled;
  await openObject('Контрагенты');
  await openObject('Товары');
  await windowPage('Функциональные опции');
  steps.reopened = rows();
  return steps;
})()`

// The page «Функциональные опции» of the window of a catalog is the
// prototype's (docs/block-00/spravochnik.md, page 3; snapshot okno-03): the
// caption, the options of the project in a list, a box at each and no
// icon, the first row current, marked where the catalog itself is in the content - not where only
// its attribute is; a mark changes the window and the mark taken back
// leaves it as it was; «Сохранить» writes the marks into the options and
// the window opened anew shows them. Defects caught: the page empty; an
// option marked for a part of the catalog; a mark that leaves the window
// unchanged or a mark taken back that leaves it changed; a mark the save
// does not write.
func TestTheOptionsPageOfACatalogIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addCatalogOptions(t, root)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, optionsPageWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Caption                 string
		Shown, Reopened         []string
		Icons                   int
		Current                 string
		Marked, Unmarked, Saved bool
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
	expect("caption", steps.Caption, "Функциональные опции, в которых участвует справочник:")
	expect("the list", steps.Shown, []string{"Артикулы", "Склады ✓"})
	expect("icons", steps.Icons, 0)
	expect("the current row", steps.Current, "Артикулы")
	expect("a mark changes the window", steps.Marked, true)
	expect("a mark taken back", steps.Unmarked, false)
	expect("saved", steps.Saved, true)
	expect("opened anew", steps.Reopened, []string{"Артикулы ✓", "Склады"})
}

// exchangePageWalk opens the page «Обмен данными» of the window of the
// catalog Товары, which Полный registers by the application's code: reads
// the table; marks all - Полный keeps its autoregistration, Филиалы comes
// with «Разрешить» - unmarks all and marks all again; chooses «Разрешить»
// for Полный and «Запретить» for Филиалы, saves and opens the window anew.
const exchangePageWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const host = document.querySelector('#catalog-editor');
  const body = () => host.querySelector('.object-window-body');
  const settle = () => new Promise(resolve => setTimeout(resolve, 50)).then(() => studio.settle());
  const rows = () => [...body().querySelectorAll('.object-window-table tr')].slice(1).map(row => row.cells[0].textContent.trim() +
    (row.querySelector('input').checked ? ' ✓' : '') + (row.querySelector('select') ? ' ' + row.querySelector('select').selectedOptions[0]?.textContent : ''));
  const row = name => [...body().querySelectorAll('.object-window-table tr')].find(item => item.cells[0].textContent.trim() === name);
  const button = title => [...body().querySelectorAll('.object-window-list-head button')].find(item => item.title === title);
  await openObject('Товары');
  await windowPage('Обмен данными');
  steps.head = [...body().querySelectorAll('.object-window-table th')].map(cell => cell.textContent);
  steps.buttons = [...body().querySelectorAll('.object-window-list-head button')].map(item => item.title);
  steps.icons = body().querySelectorAll('.object-window-table td svg').length;
  steps.shown = rows();
  steps.current = body().querySelector('.object-window-table tr.current')?.cells[0].textContent.trim();
  button('Отметить все элементы').click(); await settle();
  steps.markedAll = rows();
  button('Снять отметку со всех элементов').click(); await settle();
  steps.unmarkedAll = rows();
  button('Отметить все элементы').click(); await settle();
  steps.remarked = rows();
  const choose = (name, value) => { const select = row(name).querySelector('select'); select.value = value; select.dispatchEvent(new Event('change', {bubbles: true})); };
  choose('Полный', 'allow'); choose('Филиалы', 'deny'); await settle();
  steps.changed = !document.querySelector('#save').disabled;
  document.querySelector('#save').click(); await settle();
  steps.saved = document.querySelector('#save').disabled;
  await openObject('Контрагенты');
  await openObject('Товары');
  await windowPage('Обмен данными');
  steps.reopened = rows();
  return steps;
})()`

// The page «Обмен данными» of the window of a catalog is the prototype's
// (docs/block-00/spravochnik.md, page 14; snapshot okno-14): the two
// buttons over a table «План обмена» - «Авторегистрация», a box and the icon
// of a plan at each row, the first current, the autoregistration beside a
// marked plan; «Снять отметку со всех элементов» unmarks every plan,
// «Отметить все элементы» marks every one, a plan marked anew coming with
// «Разрешить» (plan-obmena.md, the new object of «Состав») and one marked
// before keeping its own; «Сохранить» writes the marks and the window opened
// anew shows them. Defects caught: the page empty; the autoregistration not
// shown, lost by a mark or not «Разрешить» on a plan marked anew; the
// buttons doing nothing; a change the save does not write.
func TestTheExchangePageOfACatalogIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addCatalogExchangePlans(t, root)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, exchangePageWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Head, Buttons                                     []string
		Shown, MarkedAll, UnmarkedAll, Remarked, Reopened []string
		Icons                                             int
		Current                                           string
		Changed, Saved                                    bool
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
	expect("the head of the table", steps.Head, []string{"План обмена", "Авторегистрация"})
	expect("the buttons", steps.Buttons, []string{"Отметить все элементы", "Снять отметку со всех элементов"})
	expect("icons of the plans", steps.Icons, 2)
	expect("the table", steps.Shown, []string{"Полный ✓ Запретить", "Филиалы"})
	expect("the current row", steps.Current, "Полный")
	expect("marked all", steps.MarkedAll, []string{"Полный ✓ Запретить", "Филиалы ✓ Разрешить"})
	expect("unmarked all", steps.UnmarkedAll, []string{"Полный", "Филиалы"})
	expect("marked all anew", steps.Remarked, []string{"Полный ✓ Разрешить", "Филиалы ✓ Разрешить"})
	expect("a change changes the window", steps.Changed, true)
	expect("saved", steps.Saved, true)
	expect("opened anew", steps.Reopened, []string{"Полный ✓ Разрешить", "Филиалы ✓ Запретить"})
}

// basedOnPageWalk opens the page «Ввод на основании» of the window of the
// catalog Товары, the basis of the document ПродажаТоваров: reads the two
// lists; takes ПродажаТоваров away; chooses Товары for «Вводится на
// основании», then Контрагенты for «Является основанием для», saves and
// opens the window anew.
const basedOnPageWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const host = document.querySelector('#catalog-editor');
  const body = () => host.querySelector('.object-window-body');
  const click = async element => { element.click(); await studio.settle(); };
  const top = () => [...document.querySelectorAll('dialog[open]')].at(-1);
  const press = async text => { [...top().querySelectorAll('button')].find(button => button.textContent === text).click(); await new Promise(resolve => setTimeout(resolve, 50)); await studio.settle(); };
  const heads = () => [...body().querySelectorAll('.object-window-list-head')];
  const lists = () => [...body().querySelectorAll('.object-window-list')].map(list => [...list.querySelectorAll('.object-window-list-row')].map(row => row.textContent));
  const choose = async (at, name) => {
    await click(heads()[at].querySelector('button[title=Изменить]'));
    top().querySelectorAll('.object-choice-tree details').forEach(item => { item.open = true; });
    [...top().querySelectorAll('.object-choice-tree .studio-tree-row')].find(row => row.textContent.trim() === name).querySelector('input').click();
    await press('ОК');
    return lists();
  };
  await openObject('Товары');
  await windowPage('Ввод на основании');
  steps.heads = heads().map(head => head.querySelector('span').textContent);
  steps.shown = lists();
  steps.removeOwn = heads()[0].querySelector('button[title=Удалить]').disabled;
  const designer = body().querySelector('.object-window-designer');
  steps.designer = {text: designer.textContent, disabled: designer.disabled};
  await click(heads()[1].querySelector('button[title=Удалить]'));
  steps.removed = lists();
  steps.changed = !document.querySelector('#save').disabled;
  steps.itself = await choose(0, 'Товары');
  steps.partners = await choose(1, 'Контрагенты');
  document.querySelector('#save').click(); await new Promise(resolve => setTimeout(resolve, 50)); await studio.settle();
  steps.saved = document.querySelector('#save').disabled;
  await openObject('Контрагенты');
  await openObject('Товары');
  await windowPage('Ввод на основании');
  steps.reopened = lists();
  return steps;
})()`

// The page «Ввод на основании» of the window of a catalog is the
// prototype's (docs/block-00/spravochnik.md, page 12; snapshot okno-12):
// two lists by their captions, each with «Изменить» and «Удалить», the
// objects by their full names, «Конструктор ввода на основании»
// unavailable; «Удалить» takes the current object away, «Изменить» chooses
// by «Выбор объекта»; the catalog chosen as its own basis stands in both
// lists; a table of an external source, which «Выбор объекта» does not
// offer, stays in the list chosen anew; the objects stand in the order of
// the tree, the tables after them; «Сохранить» writes both and the window
// opened anew shows them.
// Defects caught: the page empty; «Является основанием для» not read off
// the other objects; «Удалить» or «Изменить» doing nothing; the catalog
// its own basis in one list only; a table lost by a choice; the order of
// the list changing on opening anew; the save not writing the other
// objects.
func TestTheBasedOnPageOfACatalogIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	enterSalesOnGoods(t, root)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, basedOnPageWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Heads                                      []string
		Shown, Removed, Itself, Partners, Reopened [][]string
		RemoveOwn, Changed, Saved                  bool
		Designer                                   struct {
			Text     string
			Disabled bool
		}
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
	expect("captions", steps.Heads, []string{"Вводится на основании:", "Является основанием для:"})
	table := "ВнешнийИсточникДанных.Склад.Таблица.Остатки"
	expect("the lists", steps.Shown, [][]string{{}, {"Документ.ПродажаТоваров", table}})
	expect("«Удалить» of an empty list unavailable", steps.RemoveOwn, true)
	expect("the designer", steps.Designer, struct {
		Text     string
		Disabled bool
	}{"Конструктор ввода на основании", true})
	expect("taken away", steps.Removed, [][]string{{}, {table}})
	expect("a removal changes the window", steps.Changed, true)
	expect("its own basis", steps.Itself, [][]string{{"Справочник.Товары"}, {"Справочник.Товары", table}})
	expect("the basis of Контрагенты", steps.Partners, [][]string{{"Справочник.Товары"}, {"Справочник.Контрагенты", "Справочник.Товары", table}})
	expect("saved", steps.Saved, true)
	expect("opened anew", steps.Reopened, [][]string{{"Справочник.Товары"}, {"Справочник.Контрагенты", "Справочник.Товары", table}})
}

// rightsPageWalk opens the page «Права» of the window of the catalog
// Товары, which Кладовщик reads under a restriction and Менеджер does not:
// reads the roles, the rights and the restrictions of Кладовщик; chooses
// Менеджер, marks «Интерактивное удаление», clears «Чтение», marks
// «Просмотр», saves and opens the window anew.
const rightsPageWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const host = document.querySelector('#catalog-editor');
  const body = () => host.querySelector('.object-window-body');
  const settle = () => new Promise(resolve => setTimeout(resolve, 50)).then(() => studio.settle());
  const roles = () => [...body().querySelectorAll('.object-window-list-row')].map(row => row.textContent + (row.classList.contains('current') ? ' *' : ''));
  const rights = () => [...body().querySelectorAll('.check-list-row')].filter(row => row.querySelector('input').checked).map(row => row.textContent);
  const right = name => [...body().querySelectorAll('.check-list-row')].find(row => row.textContent === name).querySelector('input');
  const restrictions = () => [...body().querySelectorAll('.role-restrictions tbody tr')].map(row => [...row.cells].map(cell => cell.textContent).join(' | '));
  await openObject('Товары');
  await windowPage('Права');
  steps.captions = [...body().querySelectorAll('.object-window-caption, .role-restrictions h4')].map(item => item.textContent);
  steps.roles = roles();
  steps.rights = rights();
  steps.current = body().querySelector('.check-list-row.current')?.textContent;
  steps.restrictions = restrictions();
  [...body().querySelectorAll('.object-window-list-row')].find(row => row.textContent === 'Менеджер').click(); await settle();
  steps.manager = {roles: roles(), rights: rights(), restrictions: restrictions()};
  right('Интерактивное удаление').click(); await settle();
  steps.deleting = rights();
  right('Чтение').click(); await settle();
  steps.unread = rights();
  right('Просмотр').click(); await settle();
  steps.viewing = rights();
  document.querySelector('#save').click(); await settle();
  steps.saved = document.querySelector('#save').disabled;
  await openObject('Контрагенты');
  await openObject('Товары');
  await windowPage('Права');
  [...body().querySelectorAll('.object-window-list-row')].find(row => row.textContent === 'Менеджер').click(); await settle();
  steps.reopened = rights();
  return steps;
})()`

// The page «Права» of the window of a catalog is the prototype's
// (docs/block-00/spravochnik.md, page 13; snapshot okno-13): «Роли:», the
// first role current; «Права:», the rights the role gives on the catalog;
// «Ограничения доступа к данным:» of the right current; another role chosen
// shows its own; marking a right marks the rights it needs and clearing one
// clears those needing it (rol.md, «Права зависят друг от друга»);
// «Сохранить» writes the role and the window opened anew shows it. Defects
// caught: the page empty; another role's rights or restrictions shown; the
// rights marked one by one, apart from what they need; a change the save
// does not write.
func TestTheRightsPageOfACatalogIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) { addCatalogRoles(t, workspace) })
	result, err := browser.evaluate(ctx, page, rightsPageWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Captions, Roles, Rights, Restrictions []string
		Current                               string
		Manager                               struct{ Roles, Rights, Restrictions []string }
		Deleting, Unread, Viewing, Reopened   []string
		Saved                                 bool
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
	expect("captions", steps.Captions, []string{"Роли:", "Права:", "Ограничения доступа к данным:"})
	expect("the roles", steps.Roles, []string{"Кладовщик *", "Менеджер"})
	expect("the rights of Кладовщик", steps.Rights, []string{"Чтение"})
	expect("the current right", steps.Current, "Чтение")
	expect("the restrictions of Кладовщик", steps.Restrictions, []string{"Прочие поля | Истина"})
	expect("Менеджер chosen", steps.Manager.Roles, []string{"Кладовщик", "Менеджер *"})
	expect("the rights of Менеджер", steps.Manager.Rights, []string{})
	expect("the restrictions of Менеджер", steps.Manager.Restrictions, []string{"Ограничений нет"})
	expect("«Интерактивное удаление» marked", steps.Deleting, []string{"Чтение", "Изменение", "Удаление", "Просмотр", "Редактирование", "Интерактивное удаление"})
	expect("«Чтение» cleared", steps.Unread, []string{})
	expect("«Просмотр» marked", steps.Viewing, []string{"Чтение", "Просмотр"})
	expect("saved", steps.Saved, true)
	expect("opened anew", steps.Reopened, []string{"Чтение", "Просмотр"})
}
