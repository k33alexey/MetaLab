package studio

import (
	"encoding/json"
	"testing"
)

// namesWalk types names into the fields of a name - of a catalog, of its
// attribute, of a language and of the project - and answers, for each, the
// message the field was answered with when it was left and whether it has
// the focus again after «ОК».
const namesWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;
  const node = text => [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === text);
  const open = async text => { node(text).click(); await studio.settle(); };
  const nameInput = host => [...document.querySelectorAll(host + ' .catalog-field')].find(label => label.firstChild.textContent === 'Имя').querySelector('input');
  const type = async (host, text) => {
    const input = nameInput(host);
    input.focus();
    input.value = text;
    input.dispatchEvent(new Event('input', {bubbles: true}));
    // The field is left: the focus is elsewhere when it is answered.
    document.querySelector('#refresh').focus();
    input.dispatchEvent(new Event('change', {bubbles: true}));
    await studio.settle();
    const dialog = document.querySelector('dialog[open]');
    if (!dialog) return {message: ''};
    const message = dialog.querySelector('h3').textContent;
    [...dialog.querySelectorAll('button')].find(button => button.textContent === 'ОК').click();
    await studio.settle();
    return {message, focused: document.activeElement === nameInput(host)};
  };
  await open('Товары');
  steps.repeat = await type('#catalog-editor', 'контрагенты');
  steps.rule = await type('#catalog-editor', 'Товары склада');
  steps.free = await type('#catalog-editor', 'Номенклатура');
  [...document.querySelectorAll('#catalog-editor .catalog-list-item')].find(item => item.textContent === 'Артикул').click();
  await studio.settle();
  steps.standard = await type('#catalog-editor', 'Код');
  steps.empty = await type('#catalog-editor', '');
  document.querySelector('#refresh').click();
  await studio.settle();
  for (const dialog of document.querySelectorAll('dialog[open]')) dialog.close();
  await open('Українська');
  steps.language = await type('#languages-editor', 'english');
  document.querySelector('#tree summary').click();
  await studio.settle();
  steps.project = await type('#project-editor', 'Продажи и склад');
  return steps;
})()`

// TestAFieldOfANameRefusesAtOnce catches a name refused only by the save, in
// English, or not at all: the prototype refuses it as soon as the field is
// left, with its own words, and the field stays to be put right
// (docs/block-00/obshchee.md, «Имя и синоним»). Each field of a name Studio
// has asks the one check of the server.
func TestAFieldOfANameRefusesAtOnce(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, namesWalk)
	if err != nil {
		t.Fatal(err)
	}
	type answered struct {
		Message string
		Focused bool
	}
	var steps map[string]answered
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	for step, want := range map[string]string{
		"repeat": refusedNotUnique, "rule": refusedRule, "free": "", "standard": refusedNotUnique,
		"empty": refusedEmpty, "language": refusedNotUnique, "project": refusedRule,
	} {
		got := steps[step]
		if got.Message != want || want != "" && !got.Focused {
			t.Errorf("%s: answered %+v, want %q and the field to be put right", step, got, want)
		}
	}
}

// titleWalk types names into the window of a catalog and answers the title
// shown beside the name after each: following the name, then put right by
// hand and no longer following; and the title a new attribute gets.
const titleWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;
  const node = text => [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === text);
  const label = caption => [...document.querySelectorAll('#catalog-editor .catalog-field')].find(item => item.firstChild.textContent === caption);
  const write = async (caption, text) => {
    const input = label(caption).querySelector('input');
    input.value = text;
    input.dispatchEvent(new Event('input', {bubbles: true}));
    await studio.settle();
  };
  const title = () => label('Заголовок').querySelector('input').value;
  node('Товары').click(); await studio.settle();
  steps.properties = document.querySelector('#catalog-editor .catalog-list-item').textContent;
  await write('Имя', 'ТоварыСклада');
  steps.followed = title();
  await write('Заголовок', 'Свои товары');
  await write('Имя', 'Номенклатура');
  steps.byHand = title();
  [...document.querySelectorAll('#catalog-editor button')].find(button => button.textContent === '+ Реквизит').click();
  await studio.settle();
  steps.newTitle = title();
  steps.newName = label('Имя').querySelector('input').value;
  await write('Имя', 'ЦенаЗакупки');
  steps.newFollowed = title();
  return steps;
})()`

// TestTheTitleFollowsTheName catches the title of an object or a part that
// does not follow its name, follows it after being put right by hand, or is
// given to a new part as a fixed text: the prototype's new object has none
// and its synonym follows the name typed (docs/block-00/obshchee.md, «Текст
// из имени»; konstanta.md, «Новая константа»).
func TestTheTitleFollowsTheName(t *testing.T) {
	ctx, browser, page := openStudioInBrowser(t, copyDemoProjectForStudioTest(t), func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, titleWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct{ Properties, Followed, ByHand, NewTitle, NewName, NewFollowed string }
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	// The window names its kind by the descriptor's word.
	if steps.Properties != "Справочник · свойства" {
		t.Errorf("the window of a catalog names itself %q", steps.Properties)
	}
	if steps.Followed != "Товары склада" || steps.ByHand != "Свои товары" || steps.NewTitle != "" || steps.NewName == "" || steps.NewFollowed != "Цена закупки" {
		t.Errorf("the title beside the name: %+v; want «Товары склада», then «Свои товары» kept, an empty title of a new attribute, then «Цена закупки»", steps)
	}
}

// addWalk presses «Добавить» on the branch of catalogs and answers whether a
// dialog asked anything, the name the window opened on, and the names of two
// attributes and a table part added in it.
const addWalk = `(async () => {
  const studio = window.__studio, steps = {};
  const node = text => [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === text);
  const name = () => [...document.querySelectorAll('#catalog-editor .catalog-field')].find(item => item.firstChild.textContent === 'Имя').querySelector('input').value;
  const press = async text => { [...document.querySelectorAll('#catalog-editor button')].find(button => button.textContent === text).click(); await studio.settle(); };
  node('Справочники').click(); await studio.settle();
  document.querySelector('#tree-add').click(); await studio.settle();
  steps.dialog = !!document.querySelector('dialog[open]');
  steps.object = name();
  steps.tree = !!node('Справочник1');
  await press('+ Реквизит');
  steps.first = name();
  await press('Справочник · свойства');
  await press('+ Реквизит');
  steps.second = name();
  await press('+ Табличная часть');
  steps.part = name();
  return steps;
})()`

// TestAddOpensTheNextObjectAtOnce catches «Добавить» asking a name in a
// dialog, or naming a new object or part otherwise than «<Вид>N» from one:
// the prototype makes Справочник1 at once and opens its window, and its parts
// are Реквизит1, Реквизит2, ТабличнаяЧасть1 (docs/block-00/obshchee.md,
// «Добавить» без диалога имени).
func TestAddOpensTheNextObjectAtOnce(t *testing.T) {
	ctx, browser, page := openStudioInBrowser(t, copyDemoProjectForStudioTest(t), func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, addWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Dialog, Tree                bool
		Object, First, Second, Part string
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	if steps.Dialog || !steps.Tree || steps.Object != "Справочник1" || steps.First != "Реквизит1" || steps.Second != "Реквизит2" || steps.Part != "ТабличнаяЧасть1" {
		t.Errorf("«Добавить» gave %+v; want no dialog, Справочник1 in the tree and its window, Реквизит1, Реквизит2, ТабличнаяЧасть1", steps)
	}
}

// limitsWalk opens the windows of a catalog, of a role, of a language and of
// the project and answers the ceilings their fields are set by.
const limitsWalk = `(async () => {
  const studio = window.__studio, steps = {};
  const node = text => [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === text);
  const input = (host, caption) => [...document.querySelectorAll(host + ' .catalog-field, ' + host + ' .role-identity label')]
    .find(label => label.firstChild.textContent === caption).querySelector('input, textarea');
  const ceiling = element => element.type === 'number' ? Number(element.max || 0) : element.maxLength;
  node('Товары').click(); await studio.settle();
  for (const caption of ['Имя', 'Заголовок', 'Длина кода', 'Длина наименования']) steps['catalog ' + caption] = ceiling(input('#catalog-editor', caption));
  node('Кладовщик').click(); await studio.settle();
  for (const caption of ['Имя роли', 'Заголовок', 'Комментарий']) steps['role ' + caption] = ceiling(input('#role-editor', caption));
  node('Українська').click(); await studio.settle();
  for (const caption of ['Имя', 'Синоним', 'Комментарий']) steps['language ' + caption] = ceiling(input('#languages-editor', caption));
  document.querySelector('#tree summary').click(); await studio.settle();
  for (const caption of ['Имя', 'Синоним']) steps['project ' + caption] = ceiling(input('#project-editor', caption));
  return steps;
})()`

// TestTheFieldsOfAWindowAreSetByTheModel catches a window keeping a ceiling
// of its own: a name of 128 where the model and the prototype take 255, a
// synonym of 512 and a comment of 4000 where the model sets none, a code and
// a description bounded by numbers of the page (METADATA-OBJECTS.md,
// «Пределы на состав»). A browser answers -1 for a text with no ceiling.
func TestTheFieldsOfAWindowAreSetByTheModel(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		if _, err := workspace.CreateRole("Кладовщик"); err != nil {
			t.Fatal(err)
		}
	})
	result, err := browser.evaluate(ctx, page, limitsWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps map[string]int
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	for field, want := range map[string]int{
		"catalog Имя": 255, "catalog Заголовок": -1, "catalog Длина кода": 50, "catalog Длина наименования": 150,
		"role Имя роли": 255, "role Заголовок": -1, "role Комментарий": -1,
		"language Имя": 255, "language Синоним": -1, "language Комментарий": -1,
		"project Имя": 255, "project Синоним": -1,
	} {
		if got, ok := steps[field]; !ok || got != want {
			t.Errorf("%s is set to %d (shown %v), want %d", field, got, ok, want)
		}
	}
}

// beyondWalk types numbers beyond their ceilings into the window of a
// catalog and answers the question asked, what «Да» and «Нет» leave in the
// field, and whether the catalog took the refused number.
const beyondWalk = `(async () => {
  const studio = window.__studio, steps = {};
  const node = text => [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === text);
  const input = caption => [...document.querySelectorAll('#catalog-editor .catalog-field')].find(label => label.firstChild.textContent === caption).querySelector('input');
  const commit = async (caption, text) => {
    const field = input(caption);
    // The field is left before it is answered, as a click elsewhere leaves it.
    field.focus(); field.value = text; document.querySelector('#refresh').focus(); field.dispatchEvent(new Event('change', {bubbles: true}));
    await studio.settle();
    return document.querySelector('dialog[open]');
  };
  const answer = async text => { [...document.querySelectorAll('dialog[open] button')].find(button => button.textContent === text).click(); await studio.settle(); };
  node('Товары').click(); await studio.settle();
  const before = input('Длина кода').value;
  steps.question = (await commit('Длина кода', '51'))?.querySelector('h3').textContent || '';
  await answer('Да');
  steps.yes = input('Длина кода').value === before;
  steps.unchanged = document.querySelector('#status').textContent !== 'Изменён';
  await commit('Длина кода', '51');
  await answer('Нет');
  steps.no = input('Длина кода').value === '51' && document.activeElement === input('Длина кода');
  steps.negative = !!(await commit('Длина наименования', '-1'));
  await answer('Да');
  steps.fraction = !!(await commit('Длина наименования', '2.5'));
  await answer('Да');
  steps.taken = !(await commit('Длина наименования', '120')) && document.querySelector('#status').textContent === 'Изменён';
  // The window of the type asks the same question of a length of 1025.
  [...document.querySelectorAll('#catalog-editor .catalog-list-item')].find(item => item.textContent === 'Артикул').click();
  await studio.settle();
  document.querySelector('#catalog-editor button[title="Редактирование типа данных"]').click();
  await studio.settle();
  const length = document.querySelector('dialog[open] .type-editor-number');
  const lengthBefore = length.value;
  length.focus(); length.value = '1025'; length.dispatchEvent(new Event('change', {bubbles: true}));
  await studio.settle();
  const asked = [...document.querySelectorAll('dialog[open]')].pop();
  steps.typeQuestion = asked.querySelector('h3').textContent;
  [...asked.querySelectorAll('button')].find(button => button.textContent === 'Да').click();
  await studio.settle();
  steps.typeBack = document.querySelector('dialog[open] .type-editor-number').value === lengthBefore;
  return steps;
})()`

// TestAValueBeyondItsCeilingAsksToCancel catches a number beyond its
// ceiling taken, cut silently or refused silently: the prototype asks «В
// поле введены некорректные данные. Отменить ввод?», «Да» brings back the
// value before, «Нет» leaves the field to be put right (docs/block-00/
// obshchee.md, «Пределы в окне»; the owner, 14.10.2026: everywhere).
func TestAValueBeyondItsCeilingAsksToCancel(t *testing.T) {
	ctx, browser, page := openStudioInBrowser(t, copyDemoProjectForStudioTest(t), func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, beyondWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Question, TypeQuestion                                  string
		Yes, Unchanged, No, Negative, Fraction, Taken, TypeBack bool
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	if steps.Question != "В поле введены некорректные данные. Отменить ввод?" || !steps.Yes || !steps.Unchanged || !steps.No ||
		!steps.Negative || !steps.Fraction || !steps.Taken || steps.TypeQuestion != steps.Question || !steps.TypeBack {
		t.Errorf("a value beyond its ceiling: %+v", steps)
	}
}

// roleNameWalk opens the role Кладовщик and types into the name of its
// window a name the rule refuses, then a name of two words, then a comment;
// it answers the refusal, whether the field was given back, the title after
// the second name, and whether the window was marked changed by each.
const roleNameWalk = `(async () => {
  const studio = window.__studio, steps = {};
  const field = caption => [...document.querySelectorAll('#role-editor .role-identity label')].find(label => label.firstChild.textContent === caption).querySelector('input, textarea');
  [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === 'Кладовщик').click();
  await studio.settle();
  const name = field('Имя роли');
  const type = async (input, text) => {
    input.focus();
    input.value = text;
    input.dispatchEvent(new Event('input', {bubbles: true}));
    document.querySelector('#refresh').focus();
    input.dispatchEvent(new Event('change', {bubbles: true}));
    await studio.settle();
  };
  await type(name, 'Товары склада');
  const dialog = document.querySelector('dialog[open]');
  steps.refusal = dialog?.querySelector('h3').textContent || '';
  [...(dialog?.querySelectorAll('button') || [])].find(button => button.textContent === 'ОК')?.click();
  await studio.settle();
  steps.focused = document.activeElement === name;
  await type(name, 'СтаршийКладовщик');
  steps.title = field('Заголовок').value;
  steps.changedByName = !document.querySelector('#save').disabled;
  document.querySelector('#save').click(); await studio.settle(); await studio.settle();
  steps.saved = document.querySelector('#save').disabled;
  const comment = field('Комментарий');
  comment.value = 'Склад';
  comment.dispatchEvent(new Event('input', {bubbles: true}));
  await studio.settle();
  steps.changedByComment = !document.querySelector('#save').disabled;
  return steps;
})()`

// The name of a role is the field of every name of Studio (nameInput,
// studio-common.js): a name the rule refuses is answered at once with the
// prototype's words and the field is given back to be put right
// (obshchee.md, «Имя и синоним»), the title follows the name (obshchee.md,
// «Текст из имени»), and the comment is a text field of Studio that marks
// the window changed. Defects caught: the role's field built by hand left
// without the check, the following title or the ceiling (the ceiling is
// TestEveryFieldTakesTheModelsCeiling's); a comment that changes nothing
// until the field is left.
func TestTheNameOfARoleIsTheFieldOfEveryName(t *testing.T) {
	ctx, browser, page := openStudioInBrowser(t, copyDemoProjectForStudioTest(t), func(workspace *Workspace) {
		if _, err := workspace.CreateRole("Кладовщик"); err != nil {
			t.Fatal(err)
		}
	})
	result, err := browser.evaluate(ctx, page, roleNameWalk)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Refusal, Title                                  string
		Focused, ChangedByName, Saved, ChangedByComment bool
	}
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	if got.Refusal != refusedRule || !got.Focused {
		t.Errorf("the role's name «Товары склада» was answered %q (field given back: %v), want %q", got.Refusal, got.Focused, refusedRule)
	}
	if got.Title != "Старший кладовщик" || !got.ChangedByName || !got.Saved {
		t.Errorf("after the name СтаршийКладовщик the title is %q, changed %v, saved %v; want «Старший кладовщик», changed, saved", got.Title, got.ChangedByName, got.Saved)
	}
	if !got.ChangedByComment {
		t.Error("a comment typed did not mark the window changed")
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
