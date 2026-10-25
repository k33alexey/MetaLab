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
	var steps struct{ Followed, ByHand, NewTitle, NewName, NewFollowed string }
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
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
