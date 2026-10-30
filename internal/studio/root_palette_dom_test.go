package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// rootPaletteWalk selects the root of the tree and answers its palette: the
// title, the tabs, the rows of «Основные» and the buttons of the main
// language; then opens «…» of the main language, chooses English by a double
// click, opens «…» of the main roles and of the dictionaries, a module and
// the home page by their «Открыть», and saves.
const rootPaletteWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;
  const dialog = () => document.querySelector('dialog[open]');
  const palette = () => document.querySelector('#properties');
  const row = caption => [...palette().querySelectorAll('.object-palette-row')].find(item => item.firstChild.textContent === caption);
  const tab = async key => { palette().querySelector('.studio-tab[data-tab="' + key + '"]').click(); await studio.settle(); };
  const select = async () => { document.querySelector('#tree summary').click(); await studio.settle(); };
  await select();
  steps.title = document.querySelector('#inspector-title').textContent;
  steps.work = document.querySelector('#editor-host').hidden;
  steps.tabs = [...palette().querySelectorAll('.studio-tab')].map(item => item.textContent);
  steps.main = [...palette().querySelectorAll('.object-palette-row')].map(item => item.firstChild.textContent);
  steps.dictionaries = [...row('Дополнительные словари полнотекстового поиска').querySelectorAll('button')].map(item => item.title + (item.disabled ? ' -' : ''));
  row('Основные роли').querySelector('button').click(); await studio.settle();
  steps.roles = dialog().querySelector('h3').textContent;
  [...dialog().querySelectorAll('button')].find(item => item.textContent === 'Отмена').click(); await studio.settle();
  row('Дополнительные словари полнотекстового поиска').querySelector('button').click(); await studio.settle();
  steps.dictionaryWindow = {title: dialog().querySelector('h3').textContent,
    rows: [...dialog().querySelectorAll('.studio-tree-row')].filter(item => item.checkVisibility()).map(item => item.textContent.trim())};
  dialog().querySelector('input[aria-label="Синонимы"]').click();
  [...dialog().querySelectorAll('button')].find(item => item.textContent === 'ОК').click(); await studio.settle();
  steps.dictionary = row('Дополнительные словари полнотекстового поиска').querySelector('input').value;
  await tab('presentation');
  steps.language = row('Основной язык').querySelector('input').value;
  steps.languageButtons = [...row('Основной язык').querySelectorAll('button')].map(item => item.title);
  row('Основной язык').querySelector('button').click(); await studio.settle();
  steps.choice = {title: dialog().querySelector('h3').textContent, rows: [...dialog().querySelectorAll('.object-window-list-row')].map(item => item.textContent),
    icons: [...dialog().querySelectorAll('.object-window-list-row')].map(item => item.querySelector('use')?.getAttribute('href') || ''),
    current: dialog().querySelector('.object-window-list-row.current')?.textContent};
  // A double click is two clicks and the double click, as the browser sends it.
  const english = () => [...dialog().querySelectorAll('.object-window-list-row')].find(item => item.textContent === 'English');
  english().click(); await studio.settle();
  english().dispatchEvent(new MouseEvent('dblclick', {bubbles: true}));
  await studio.settle();
  steps.chosen = row('Основной язык').querySelector('input').value;
  steps.changed = !document.querySelector('#save').disabled;
  steps.windows = ['Командный интерфейс', 'Рабочая область начальной страницы', 'Командный интерфейс основного раздела', 'Картинка основного раздела', 'Интерфейс клиентского приложения']
    .map(caption => caption + (row(caption).querySelector('button').disabled ? ' -' : ''));
  document.querySelector('#save').click(); await studio.settle();
  steps.saved = document.querySelector('#save').disabled;
  await tab('main');
  row('Модуль приложения').querySelector('button').click(); await studio.settle();
  steps.module = document.querySelector('#file-path').textContent;
  await select();
  await tab('presentation');
  row('Рабочая область начальной страницы').querySelector('button').click(); await studio.settle();
  steps.homePage = document.querySelector('#file-path').textContent;
  await select();
  await tab('presentation');
  row('Командный интерфейс').querySelector('button').click(); await studio.settle();
  steps.commandInterface = dialog()?.matches('.section-commands-dialog') ? dialog().querySelector('h3').textContent : '';
  [...dialog().querySelectorAll('button')].find(item => item.textContent === 'Отмена').click(); await studio.settle();
  // «Показывать только важные»: the tabs and the rows of «Представление».
  palette().querySelector('button[title="Показывать только важные"]').click(); await studio.settle();
  steps.importantTabs = [...palette().querySelectorAll('.studio-tab')].map(item => item.textContent);
  await tab('presentation');
  steps.importantRows = [...palette().querySelectorAll('.object-palette-row, .object-palette-separator')].map(item => item.matches('hr') ? '|' : item.firstChild.textContent);
  palette().querySelector('button[title="Показывать только важные"]').click(); await studio.settle();
  return steps;
})()`

// The root has no window: selecting it shows its palette, the prototype's
// five tabs (konfiguraciya.md, «Палитра»), and its rows do what the root's
// menu does. Defects caught: the root opening a window of its own in the
// work area again; a tab or a row of «Основные» lost or out of order; the
// main language cleared by a cross, chosen in a window not titled by its
// kind, or not written by «Сохранить»; «…» of the dictionaries offering a
// window the inventory does not show; a row «Открыть» opening nothing or
// the picture of the main section offering a window Studio has not;
// «Командный интерфейс: Открыть» missing or opening no window; «…» of the
// dictionaries unavailable, titled by a kind, with a branch closed, or its
// choice not shown or not written with its kind; a row of «Выбор объекта:
// Язык» without the kind's icon; «Показывать только важные» keeping
// «Справка» or a row of «Представление» the prototype's does not keep.
func TestThePaletteOfTheRootIsTheWayIntoIt(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	// A dictionary may be kept in a common template or in a constant.
	template, constant := uuid.MustNew(), uuid.MustNew()
	for path, content := range map[string]string{
		"metadata/common-templates/Словарь/object.yaml": "format: 1\nid: " + template.String() + "\nname: Словарь\ntitle: {ru: Словарь}\nkind: spreadsheet\n",
		"metadata/constants/Синонимы/object.yaml":       "format: 1\nid: " + constant.String() + "\nname: Синонимы\ntitle: {ru: Синонимы}\ntypes: [{kind: boolean}]\n",
	} {
		target := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, rootPaletteWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Title, Language, Roles, Chosen, Module, HomePage, Dictionary, CommandInterface string
		Work, Changed, Saved                                                           bool
		Tabs, Main, Dictionaries, LanguageButtons                                      []string
		Windows, ImportantTabs, ImportantRows                                          []string
		Choice                                                                         struct {
			Title, Current string
			Rows, Icons    []string
		}
		DictionaryWindow struct {
			Title string
			Rows  []string
		}
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	if steps.Title != "Свойства: ПродажиИСклад" || !steps.Work {
		t.Errorf("the root shows %q with the work area hidden %v, want its palette and no window", steps.Title, steps.Work)
	}
	if want := []string{"Основные", "Представление", "Разработка", "Справка", "Совместимость"}; !slices.Equal(steps.Tabs, want) {
		t.Errorf("the tabs are %q, want %q", steps.Tabs, want)
	}
	if want := []string{"Имя", "Синоним", "Комментарий", "Вариант встроенного языка", "Основные роли", "Модуль приложения", "Модуль сеанса",
		"Модуль внешнего соединения", "Дополнительные словари полнотекстового поиска", "Хранилище общих настроек",
		"Хранилище пользовательских настроек отчетов", "Хранилище вариантов отчетов", "Хранилище настроек данных форм",
		"Хранилище пользовательских настроек динамических списков", "Хранилище внешних данных навигационных ссылок"}; !slices.Equal(steps.Main, want) {
		t.Errorf("«Основные» shows %q, want %q", steps.Main, want)
	}
	if want := []string{"Дополнительные словари полнотекстового поиска", "Очистить: Дополнительные словари полнотекстового поиска -"}; !slices.Equal(steps.Dictionaries, want) {
		t.Errorf("the dictionaries have %q, want %q", steps.Dictionaries, want)
	}
	// «…» of the dictionaries: «Выбор объекта», the common templates under
	// «Общие» and the constants, every branch open (dosyomka-2651/02 to 04).
	if want := []string{"Общие", "Общие макеты", "Словарь", "Константы", "Синонимы"}; steps.DictionaryWindow.Title != "Выбор объекта" || !slices.Equal(steps.DictionaryWindow.Rows, want) {
		t.Errorf("«…» of the dictionaries opens %+v, want «Выбор объекта» with %q", steps.DictionaryWindow, want)
	}
	if steps.Dictionary != "Константа.Синонимы" {
		t.Errorf("the chosen dictionary shows %q", steps.Dictionary)
	}
	if steps.Roles != "Выбор объекта: Роль" {
		t.Errorf("«…» of the main roles opens %q", steps.Roles)
	}
	if steps.Language != "Русский" || !slices.Equal(steps.LanguageButtons, []string{"Основной язык"}) {
		t.Errorf("the main language shows %q with %q, want Русский and «…» alone", steps.Language, steps.LanguageButtons)
	}
	if steps.Choice.Title != "Выбор объекта: Язык" || !slices.Equal(steps.Choice.Rows, []string{"Русский", "Українська", "English"}) || steps.Choice.Current != "Русский" ||
		!slices.Equal(steps.Choice.Icons, []string{"#icon-language", "#icon-language", "#icon-language"}) {
		t.Errorf("«…» of the main language opens %+v", steps.Choice)
	}
	if steps.Chosen != "English" || !steps.Changed || !steps.Saved {
		t.Errorf("after the choice the language shows %q, changed %v, saved %v", steps.Chosen, steps.Changed, steps.Saved)
	}
	if want := []string{"Командный интерфейс", "Рабочая область начальной страницы", "Командный интерфейс основного раздела", "Картинка основного раздела -", "Интерфейс клиентского приложения"}; !slices.Equal(steps.Windows, want) {
		t.Errorf("the windows of «Представление» are %q, want %q", steps.Windows, want)
	}
	if steps.CommandInterface != "Командный интерфейс" {
		t.Errorf("«Командный интерфейс: Открыть» opened %q, want the window «Командный интерфейс»", steps.CommandInterface)
	}
	if want := []string{"Основные", "Представление", "Разработка", "Совместимость"}; !slices.Equal(steps.ImportantTabs, want) {
		t.Errorf("«Показывать только важные» keeps the tabs %q, want %q", steps.ImportantTabs, want)
	}
	if want := []string{"Основная форма отчета", "Основная форма настроек отчета", "Основная форма варианта отчета",
		"Основная форма настроек динамического списка", "Основная форма поиска", "|", "Основной макет оформления отчета"}; !slices.Equal(steps.ImportantRows, want) {
		t.Errorf("«Показывать только важные» keeps on «Представление» %q, want %q", steps.ImportantRows, want)
	}
	if steps.Module != project.ApplicationModuleFile || steps.HomePage != "Рабочая область начальной страницы" {
		t.Errorf("«Открыть» opened %q and %q", steps.Module, steps.HomePage)
	}
	file, err := os.Open(filepath.Join(root, project.ConfigurationFile))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	configuration, err := project.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.DefaultLanguage != "en" {
		t.Errorf("the saved main language is %q, want en", configuration.DefaultLanguage)
	}
	if want := []project.DictionaryReference{{Kind: project.ConstantDictionary, Object: constant}}; !slices.Equal(configuration.AdditionalFullTextSearchDictionaries, want) {
		t.Errorf("the saved dictionaries are %+v, want %+v", configuration.AdditionalFullTextSearchDictionaries, want)
	}
}
