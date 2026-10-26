package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

// addGermanLanguage gives the example a fourth language with no translation
// of anything but its own synonym, so that a string of the window of
// translations stands for a language the text says nothing in. Its synonym is
// written in Russian and in German, not in English, and differs from its
// name, so that its caption shows which language it is named in and that,
// with no synonym in the language of Studio, it is named by its name.
func addGermanLanguage(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, "configuration.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	const after = "    code: en\n"
	if strings.Count(text, after) != 1 {
		t.Fatalf("the example's configuration.yaml no longer ends its English language with %q", after)
	}
	text = strings.Replace(text, after, after+"  - id: 4f1c2d3e-0000-4000-8000-0000000000de\n    name: Deutsch\n    title: {de: Deutsch-Sprache, ru: Немецкий}\n    code: de\n", 1)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// localizedFieldWalk opens the window of the root and answers how its
// synonym is drawn - the inputs of the field, its value, whether the
// magnifier lies inside the input's frame at the right edge and whether the
// field is as wide as «Имя» - then opens «Строки на разных языках» and
// answers its title, captions, strings, buttons, whether each caption stands
// above its string and the buttons to the right of the strings, and which
// string has the focus. It writes a string and cancels, writes two and
// presses ОК, switches the language of Studio, and answers the same field
// in the windows of a catalog, of a role and of a language.
const localizedFieldWalk = `(async () => {
  const studio = window.__studio, steps = {};
  const dialog = () => document.querySelector('dialog[open]');
  const node = text => [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === text);
  const open = async text => { node(text).click(); await studio.settle(); };
  const field = (host, caption) => [...document.querySelectorAll(host + ' .catalog-field, ' + host + ' .role-identity label')]
    .find(label => label.firstChild.textContent === caption);
  const drawn = (host, caption) => {
    const label = field(host, caption), inputs = label.querySelectorAll('input, textarea');
    const button = label.querySelector('.field-with-buttons button');
    if (!button) return {inputs: inputs.length};
    const frame = inputs[0].getBoundingClientRect(), place = button.getBoundingClientRect();
    return {inputs: inputs.length, value: inputs[0].value, button: button.title, disabled: button.disabled,
      inside: place.left >= frame.left && place.right <= frame.right && place.top >= frame.top && place.bottom <= frame.bottom && frame.right - place.right < 4,
      width: Math.round(label.querySelector('.field-with-buttons').getBoundingClientRect().width)};
  };
  const translations = () => {
    const labels = [...dialog().querySelectorAll('label')], actions = dialog().querySelector('.localized-title-actions');
    return {title: dialog().querySelector('h3').textContent,
      captions: labels.map(label => label.firstChild.textContent), strings: labels.map(label => label.querySelector('input').value),
      above: labels.every(label => label.firstChild.getBoundingClientRect().bottom <= label.querySelector('input').getBoundingClientRect().top),
      buttons: [...actions.children].map(button => button.textContent),
      beside: labels.every(label => actions.getBoundingClientRect().left >= label.getBoundingClientRect().right),
      column: actions.children[1].getBoundingClientRect().top > actions.children[0].getBoundingClientRect().bottom,
      focused: labels.findIndex(label => label.querySelector('input') === document.activeElement)};
  };
  const lupa = async (host, caption) => { field(host, caption).querySelector('.field-with-buttons button').click(); await studio.settle(); };
  const write = (index, text) => { const input = dialog().querySelectorAll('label input')[index]; input.value = text; input.dispatchEvent(new Event('input', {bubbles: true})); };
  const press = async caption => { [...dialog().querySelectorAll('.localized-title-actions button')].find(button => button.textContent === caption).click(); await studio.settle(); };
  document.querySelector('#tree summary').click(); await studio.settle();
  steps.synonym = drawn('#project-editor', 'Синоним');
  steps.name = Math.round(field('#project-editor', 'Имя').querySelector('input').getBoundingClientRect().width);
  steps.detailed = drawn('#project-editor', 'Подробная информация');
  await lupa('#project-editor', 'Синоним');
  steps.window = translations();
  write(1, 'Отменено');
  await press('Отмена');
  steps.cancelled = {value: drawn('#project-editor', 'Синоним').value, saved: document.querySelector('#save').disabled};
  await lupa('#project-editor', 'Синоним');
  steps.afterCancel = translations().strings;
  write(0, 'Продажи и склады');
  write(3, 'Verkauf und Lager');
  await press('ОК');
  steps.written = {value: drawn('#project-editor', 'Синоним').value, saved: document.querySelector('#save').disabled};
  await lupa('#project-editor', 'Синоним');
  steps.afterOK = translations().strings;
  await press('Отмена');
  const language = document.querySelector('#studio-language');
  language.value = 'en'; language.dispatchEvent(new Event('change')); await studio.settle();
  steps.english = drawn('#project-editor', 'Синоним').value;
  await lupa('#project-editor', 'Синоним');
  steps.inEnglish = translations();
  await press('Отмена');
  language.value = 'ru'; language.dispatchEvent(new Event('change')); await studio.settle();
  document.querySelector('#save').click(); await studio.settle();
  steps.saved = document.querySelector('#save').disabled;
  await open('Товары');
  steps.catalog = drawn('#catalog-editor', 'Синоним');
  await open('Кладовщик');
  steps.role = drawn('#role-editor', 'Заголовок');
  await open('Українська');
  steps.language = drawn('#languages-editor', 'Синоним');
  await open('English');
  steps.locked = drawn('#languages-editor', 'Синоним');
  return steps;
})()`

// A localized text is one field, as the prototype's (the owner's snapshots of
// 19.10.2026 in materials/block-00/konfiguraciya; ML-STUDIO.md, «многоязычное
// поле», «кнопки поля — внутри поля справа»): the translation in the
// language Studio edits in, the magnifier inside the frame at the right, and
// the window «Строки на разных языках» with a caption above the string of
// every language and ОК, «Отмена» in a column to the right. Defects caught: a
// field per language spread out in the window of the root again; the
// magnifier outside the field, or the field with it wider or narrower than
// «Имя»; a language of the project without a translation left out of the
// window, or a language named on itself and not in the language of Studio, or
// the captions beside the strings, or the buttons under them, or «Справка»
// among them; the window written into on «Отмена», or not written
// on ОК, or the field not showing what ОК wrote; the field deaf to the
// language of Studio; the catalog, the role or the language keeping an
// editor of its own; the magnifier of a locked language opening a window
// that writes.
func TestTheLocalizedFieldIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addGermanLanguage(t, root)
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) { addRestrictedRole(t, workspace, root) })
	result, err := browser.evaluate(ctx, page, localizedFieldWalk)
	if err != nil {
		t.Fatal(err)
	}
	type drawn struct {
		Inputs           int
		Value, Button    string
		Disabled, Inside bool
		Width            int
	}
	type window struct {
		Title                 string
		Captions, Strings     []string
		Buttons               []string
		Above, Beside, Column bool
		Focused               int
	}
	type saved struct {
		Value string
		Saved bool
	}
	var steps struct {
		Synonym, Detailed, Catalog, Role, Language, Locked drawn
		Name                                               int
		Window                                             window
		Cancelled, Written                                 saved
		AfterCancel, AfterOK                               []string
		English                                            string
		InEnglish                                          window
		Saved                                              bool
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	const lupa = "Строки на разных языках"
	field := func(where string, got drawn, value string, disabled bool) {
		t.Helper()
		if got.Inputs != 1 || got.Button != lupa || !got.Inside || got.Disabled != disabled || got.Value != value {
			t.Errorf("%s is drawn as %+v, want one input showing %q with the magnifier inside it at the right (disabled %v)", where, got, value, disabled)
		}
	}
	field("the synonym of the root", steps.Synonym, "Продажи и склад", false)
	if steps.Synonym.Width != steps.Name {
		t.Errorf("the synonym with its magnifier is %d px wide, «Имя» %d px", steps.Synonym.Width, steps.Name)
	}
	field("«Подробная информация»", steps.Detailed, "", false)
	want := window{Title: lupa, Captions: []string{"Русский", "Українська", "English", "Немецкий"},
		Strings: []string{"Продажи и склад", "Продажі і склад", "Sales and warehouse", ""}, Buttons: []string{"ОК", "Отмена"},
		Above: true, Beside: true, Column: true, Focused: 0}
	got := steps.Window
	if got.Title != want.Title || !slices.Equal(got.Captions, want.Captions) || !slices.Equal(got.Strings, want.Strings) || !slices.Equal(got.Buttons, want.Buttons) ||
		got.Above != want.Above || got.Beside != want.Beside || got.Column != want.Column || got.Focused != want.Focused {
		t.Errorf("the window of translations opens as %+v, want %+v", got, want)
	}
	if steps.Cancelled != (saved{Value: "Продажи и склад", Saved: true}) || !slices.Equal(steps.AfterCancel, want.Strings) {
		t.Errorf("after «Отмена» the field is %+v and the window reopens with %v: «Отмена» wrote", steps.Cancelled, steps.AfterCancel)
	}
	if steps.Written != (saved{Value: "Продажи и склады", Saved: false}) {
		t.Errorf("after ОК the field is %+v, want the new synonym and the file changed", steps.Written)
	}
	if written := []string{"Продажи и склады", "Продажі і склад", "Sales and warehouse", "Verkauf und Lager"}; !slices.Equal(steps.AfterOK, written) {
		t.Errorf("after ОК the window reopens with %v, want %v", steps.AfterOK, written)
	}
	if steps.English != "Sales and warehouse" {
		t.Errorf("in English the synonym shows %q", steps.English)
	}
	// A language is named by its synonym in the language of Studio, and by
	// its name where it has none in it - not in the default language: the
	// German one has no English synonym and is named Deutsch, not Немецкий.
	if english := []string{"Русский", "Українська", "English", "Deutsch"}; !slices.Equal(steps.InEnglish.Captions, english) || steps.InEnglish.Focused != 2 {
		t.Errorf("in English the window of translations names the languages %v and focuses string %d, want %v and the English one, 2",
			steps.InEnglish.Captions, steps.InEnglish.Focused, english)
	}
	if !steps.Saved {
		t.Error("the root written in the window of translations was not saved")
	}
	file, err := os.Open(filepath.Join(root, "configuration.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	configuration, err := project.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	for code, synonym := range map[string]string{"ru": "Продажи и склады", "uk": "Продажі і склад", "en": "Sales and warehouse", "de": "Verkauf und Lager"} {
		if configuration.Title[code] != synonym {
			t.Errorf("the saved synonym in %s is %q, want %q", code, configuration.Title[code], synonym)
		}
	}
	field("the title of the catalog Товары", steps.Catalog, "Товары", false)
	// A role made in Studio has no synonym, as the prototype's new object has
	// none (TestANewObjectHasNoSynonym): the field shows nothing.
	field("the title of the role Кладовщик", steps.Role, "", false)
	// The language names itself in itself alone: in Russian, the language of
	// Studio, its synonym says nothing, and the field shows nothing.
	field("the synonym of the language Українська", steps.Language, "", false)
	field("the synonym of the locked language English", steps.Locked, "", true)
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
