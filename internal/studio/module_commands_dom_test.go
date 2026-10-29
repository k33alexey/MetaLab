package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// moduleCommandsWalk reads the row of the common module ОбщегоНазначения
// and its menu, opens its module by a double click as a developer does - two
// clicks, then the double click - and reads what is open; then reads the
// root's menu and opens the module of external connection from it; then opens
// the manager module of the catalog Товары, which has none yet, and the object
// module of the document ПродажаТоваров from their menus; reads the menu of
// the table of no objects Остатки of the source Склад.
const moduleCommandsWalk = `(async () => {
  const studio = window.__studio, answer = {};
  const rows = () => [...document.querySelectorAll('#tree .studio-tree-row')];
  const row = text => rows().find(item => item.querySelector(':scope > span:not(.studio-tree-sign)')?.textContent === text);
  const menu = item => {
    item.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
    return [...document.querySelector('.context-menu').children].map(entry => entry.classList.contains('context-menu-separator') ? '|' :
      [entry.firstChild.nodeType === 1 ? entry.querySelector('use')?.getAttribute('href') : '', entry.childNodes[1]?.textContent ?? entry.textContent,
       getComputedStyle(entry).fontWeight].join(' '));
  };
  const common = row('ОбщегоНазначения');
  answer.commonRow = common.className;
  answer.commonMenu = menu(common).slice(0, 2);
  closeContextMenu();
  answer.moduleRows = rows().map(item => item.querySelector(':scope > span:not(.studio-tree-sign)')?.textContent).filter(text => /^Модуль( приложения| сеанса| внешнего соединения)?$/.test(text));
  // The first click is still opening the description when the second and
  // the double click come, as a hand's are.
  common.dispatchEvent(new MouseEvent('click', {bubbles: true, detail: 1}));
  await new Promise(resolve => setTimeout(resolve, 0));
  for (const type of ['click', 'dblclick']) common.dispatchEvent(new MouseEvent(type, {bubbles: true, detail: 2}));
  await studio.settle(); await studio.settle();
  answer.opened = {path: document.querySelector('#file-path').textContent, text: document.querySelector('#editor').value};
  const root = document.querySelector('#tree summary');
  answer.rootMenu = menu(root);
  answer.rootAvailable = [...document.querySelectorAll('.context-menu .context-menu-item')].filter(item => !item.disabled).map(item => item.textContent.trim());
  closeContextMenu();
  root.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent.trim() === 'Все картинки').click();
  await studio.settle();
  answer.library = document.querySelector('dialog.picture-library .picture-library-title')?.textContent || '';
  document.querySelector('dialog.picture-library')?.close();
  root.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent === 'Открыть модуль внешнего соединения').click();
  await studio.settle();
  answer.external = {path: document.querySelector('#file-path').textContent, text: document.querySelector('#editor').value};
  const pick = (item, text) => {
    answer[text] = menu(item).slice(0, 3);
    [...document.querySelectorAll('.context-menu .context-menu-item')].find(entry => entry.textContent === text).click();
  };
  pick(row('Товары'), 'Открыть модуль менеджера');
  await studio.settle();
  answer.manager = {path: document.querySelector('#file-path').textContent, text: document.querySelector('#editor').value};
  pick(row('ПродажаТоваров'), 'Открыть модуль объекта');
  await studio.settle();
  answer.object = {path: document.querySelector('#file-path').textContent, text: document.querySelector('#editor').value};
  menu(row('Остатки'));
  answer.table = [...document.querySelectorAll('.context-menu .context-menu-item')].slice(0, 3).map(entry => entry.textContent + (entry.disabled ? ' -' : ' +'));
  closeContextMenu();
  return answer;
})()`

// A module is no row of the tree: a common module opens its text by its own
// action, «Открыть модуль», bold at the head of its menu and done by a double
// click; the root opens its three modules from the head of its menu, in the
// prototype's order and words (inventory of block 00, obshchij-modul.md,
// konfiguraciya.md; snapshots obshchij-modul/12-kontekstnoe-menyu,
// konfiguraciya/15-menyu; the owner's word of 20.10.2026). Defects caught: a
// row «Модуль» under the common module or a row of a module of the root; the
// common module drawn as a branch; «Открыть модуль» missing, not first, not
// bold or without its icon; a double click that opens the description of the
// common module rather than its module, or that the first click of it, still
// opening, refuses; the root's commands missing, out of order, bold, not
// apart from «Все роли», or not opening the module, made the first time; the
// commands of an object's modules missing from the head of its menu, bold, or
// not opening the module - the one with no text yet made empty, the one with
// text shown as written (spravochnik.md, dokument.md, 2.405); the object
// module of a table of no objects offered available (vneshnij-istochnik.md).
func TestModulesOpenFromTheMenuOfTheirObject(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	moduleID, commonID := uuid.MustNew(), uuid.MustNew()
	modulePath, err := project.ModulePath(moduleID)
	if err != nil {
		t.Fatal(err)
	}
	commonPath, err := project.MetadataPath("common-modules", commonID)
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		modulePath: "Процедура ОбщаяПроцедура() Экспорт\nКонецПроцедуры\n",
		commonPath: "format: 1\nid: " + commonID.String() + "\nname: ОбщегоНазначения\ntitle: {ru: Общего назначения}\nmodule: " + moduleID.String() + "\nserver: true\n",
		"metadata/external-data-sources/Склад/object.yaml": "format: 1\nid: " + uuid.MustNew().String() + "\nname: Склад\ntitle: {ru: Склад}\n",
		"metadata/external-data-sources/Склад/tables/Остатки/object.yaml": "format: 1\nid: " + uuid.MustNew().String() +
			"\nname: Остатки\ntitle: {ru: Остатки}\nname_in_data_source: dbo.Stock\ndata_type: non-object\n",
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
	result, err := browser.evaluate(ctx, page, moduleCommandsWalk)
	if err != nil {
		t.Fatal(err)
	}
	type file struct{ Path, Text string }
	var got struct {
		CommonRow                                       string
		CommonMenu, ModuleRows, RootMenu, RootAvailable []string
		Library                                         string
		Opened, External, Manager, Object               file
		Table                                           []string
		CatalogMenu                                     []string `json:"Открыть модуль менеджера"`
		DocumentMenu                                    []string `json:"Открыть модуль объекта"`
	}
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	if !strings.Contains(got.CommonRow, "studio-tree-leaf") || len(got.ModuleRows) != 0 {
		t.Errorf("the common module is drawn as %q, rows of modules %v; want a row with nothing beneath and no module rows", got.CommonRow, got.ModuleRows)
	}
	if len(got.CommonMenu) == 0 || got.CommonMenu[0] != "#icon-open-module Открыть модуль 600" {
		t.Errorf("the menu of the common module begins %v, want «Открыть модуль» first, bold, with its icon", got.CommonMenu)
	}
	if got.Opened.Path != modulePath || !strings.Contains(got.Opened.Text, "ОбщаяПроцедура") {
		t.Errorf("a double click on the common module opened %q (%q), want its module %s", got.Opened.Path, got.Opened.Text, modulePath)
	}
	// The root's own part of the menu (konfiguraciya.md, «Команды»;
	// snapshot konfiguraciya/15-menyu), in the prototype's order and
	// groups, «конфигурации» said as «проекта»; the mobile client's
	// composition is not carried; «Добавить ▸» last (TestTheRootAddsAsThePrototype).
	wantRoot := []string{"#icon-open-module Открыть модуль приложения 400", "#icon-open-module Открыть модуль сеанса 400",
		"#icon-open-module Открыть модуль внешнего соединения 400", "|",
		" Открыть командный интерфейс проекта 400", " Открыть рабочую область начальной страницы 400",
		" Открыть командный интерфейс основного раздела 400", " Открыть интерфейс клиентского приложения 400", "|",
		" Справочная информация 400", "|", " Все подсистемы 400", " Все роли 400", " Все ограничения доступа 400", " Все картинки 400", "|", " Добавить 400"}
	if !slices.Equal(got.RootMenu, wantRoot) {
		t.Errorf("the root's menu is %v, want %v", got.RootMenu, wantRoot)
	}
	// Available: the modules and the windows Studio has (the command
	// interface of the root, TestTheCommandInterfaceOfTheRootIsThePrototypes;
	// the work area of the home page, TestTheHomePageWindowIsThePrototypes);
	// the rest wait for their windows.
	if want := []string{"Открыть модуль приложения", "Открыть модуль сеанса", "Открыть модуль внешнего соединения", "Открыть командный интерфейс проекта",
		"Открыть рабочую область начальной страницы", "Все подсистемы",
		"Все роли", "Все ограничения доступа", "Все картинки", "Добавить▸"}; !slices.Equal(got.RootAvailable, want) || got.Library != "Библиотека картинок проекта" {
		t.Errorf("the root's menu offers %v and «Все картинки» opens %q, want %v and the library", got.RootAvailable, got.Library, want)
	}
	if got.External.Path != project.ExternalConnectionModuleFile || !strings.Contains(got.External.Text, "ПриНачалеРаботыСистемы") {
		t.Errorf("«Открыть модуль внешнего соединения» opened %q (%q), want %s with its routines", got.External.Path, got.External.Text, project.ExternalConnectionModuleFile)
	}
	wantObject := []string{"#icon-open-module Открыть модуль объекта 400", "#icon-open-module Открыть модуль менеджера 400"}
	for name, menu := range map[string][]string{"Товары": got.CatalogMenu, "ПродажаТоваров": got.DocumentMenu} {
		if len(menu) < 2 || !slices.Equal(menu[:2], wantObject) {
			t.Errorf("the menu of %s begins %v, want %v", name, menu, wantObject)
		}
	}
	if got.Manager.Path != "metadata/catalogs/Товары/МодульМенеджера.bsl" || got.Manager.Text != "" {
		t.Errorf("«Открыть модуль менеджера» of Товары opened %q (%q), want its manager module made empty", got.Manager.Path, got.Manager.Text)
	}
	if got.Object.Path != "metadata/documents/ПродажаТоваров/МодульОбъекта.bsl" || !strings.Contains(got.Object.Text, "ОбработкаПроведения") {
		t.Errorf("«Открыть модуль объекта» of ПродажаТоваров opened %q (%q), want its object module", got.Object.Path, got.Object.Text)
	}
	wantTable := []string{"Открыть модуль объекта -", "Открыть модуль набора записей +", "Открыть модуль менеджера +"}
	if !slices.Equal(got.Table, wantTable) {
		t.Errorf("the menu of the table of no objects begins %v, want %v", got.Table, wantTable)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
