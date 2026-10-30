package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
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
  await tab('presentation');
  steps.language = row('Основной язык').querySelector('input').value;
  steps.languageButtons = [...row('Основной язык').querySelectorAll('button')].map(item => item.title);
  row('Основной язык').querySelector('button').click(); await studio.settle();
  steps.choice = {title: dialog().querySelector('h3').textContent, rows: [...dialog().querySelectorAll('.object-window-list-row')].map(item => item.textContent),
    current: dialog().querySelector('.object-window-list-row.current')?.textContent};
  // A double click is two clicks and the double click, as the browser sends it.
  const english = () => [...dialog().querySelectorAll('.object-window-list-row')].find(item => item.textContent === 'English');
  english().click(); await studio.settle();
  english().dispatchEvent(new MouseEvent('dblclick', {bubbles: true}));
  await studio.settle();
  steps.chosen = row('Основной язык').querySelector('input').value;
  steps.changed = !document.querySelector('#save').disabled;
  steps.windows = ['Рабочая область начальной страницы', 'Командный интерфейс основного раздела', 'Картинка основного раздела', 'Интерфейс клиентского приложения']
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
  return steps;
})()`

// The root has no window: selecting it shows its palette, the prototype's
// five tabs (konfiguraciya.md, «Палитра»), and its rows do what the root's
// menu does. Defects caught: the root opening a window of its own in the
// work area again; a tab or a row of «Основные» lost or out of order; the
// main language cleared by a cross, chosen in a window not titled by its
// kind, or not written by «Сохранить»; «…» of the dictionaries offering a
// window the inventory does not show; a row «Открыть» opening nothing or
// the picture of the main section offering a window Studio has not.
func TestThePaletteOfTheRootIsTheWayIntoIt(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, rootPaletteWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Title, Language, Roles, Chosen, Module, HomePage string
		Work, Changed, Saved                             bool
		Tabs, Main, Dictionaries, LanguageButtons        []string
		Windows                                          []string
		Choice                                           struct {
			Title, Current string
			Rows           []string
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
	if want := []string{"Дополнительные словари полнотекстового поиска -", "Очистить: Дополнительные словари полнотекстового поиска -"}; !slices.Equal(steps.Dictionaries, want) {
		t.Errorf("the dictionaries have %q, want %q", steps.Dictionaries, want)
	}
	if steps.Roles != "Выбор объекта: Роль" {
		t.Errorf("«…» of the main roles opens %q", steps.Roles)
	}
	if steps.Language != "Русский" || !slices.Equal(steps.LanguageButtons, []string{"Основной язык"}) {
		t.Errorf("the main language shows %q with %q, want Русский and «…» alone", steps.Language, steps.LanguageButtons)
	}
	if steps.Choice.Title != "Выбор объекта: Язык" || !slices.Equal(steps.Choice.Rows, []string{"Русский", "Українська", "English"}) || steps.Choice.Current != "Русский" {
		t.Errorf("«…» of the main language opens %+v", steps.Choice)
	}
	if steps.Chosen != "English" || !steps.Changed || !steps.Saved {
		t.Errorf("after the choice the language shows %q, changed %v, saved %v", steps.Chosen, steps.Changed, steps.Saved)
	}
	if want := []string{"Рабочая область начальной страницы", "Командный интерфейс основного раздела", "Картинка основного раздела -", "Интерфейс клиентского приложения"}; !slices.Equal(steps.Windows, want) {
		t.Errorf("the windows of «Представление» are %q, want %q", steps.Windows, want)
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
}
