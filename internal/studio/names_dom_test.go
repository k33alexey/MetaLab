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
