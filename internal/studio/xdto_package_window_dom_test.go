package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// xdtoPackageWalk opens the package Обмен and builds its body as the
// inventory walks the editor: «Добавить ▾» at the root - every node - at a
// value type, at an object type, at a property; ✓ and ✕ of a value; a node
// deleted after the question, one moved up; «Сохранить»; the editor opened
// again. The signs of the rows and of the items of «Добавить ▾» are read as
// the names of their icons.
const xdtoPackageWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 80)).then(() => studio.settle());
  const host = document.querySelector('#xdto-package-editor');
  const rows = () => [...host.querySelectorAll('.studio-tree-row')];
  const sign = element => (element.querySelector('use')?.getAttribute('href') || '').replace('#icon-', '');
  const menuSigns = () => [...document.querySelectorAll('.context-menu-item')].map(sign);
  const tree = () => rows().map(row => '  '.repeat(row.closest('.xdto-tree') ? (() => { let depth = 0, at = row.parentElement; while (at && !at.classList.contains('xdto-tree')) { if (at.classList.contains('studio-tree-children')) depth++; at = at.parentElement; } return depth; })() : 0) + row.textContent);
  const choose = async caption => { rows().filter(row => row.textContent === caption).pop().click(); await settle(); };
  const menu = async () => {
    [...host.querySelectorAll('.object-window-panel button')].find(button => button.textContent.startsWith('Добавить')).click(); await settle();
    const items = [...document.querySelectorAll('.context-menu-item')].map(item => item.textContent + (item.disabled ? ' (недоступна)' : ''));
    return items;
  };
  const add = async caption => { const items = await menu(); const item = [...document.querySelectorAll('.context-menu-item')].find(item => item.textContent === caption);
    if (!item) throw new Error('no ' + caption + ' in ' + JSON.stringify(items) + ' tree ' + JSON.stringify(tree()) + ' ' + JSON.stringify(studio.problems)); item.click(); await settle(); };
  const answer = async text => { const dialog = [...document.querySelectorAll('dialog')].pop(); const title = dialog.querySelector('h3').textContent;
    [...dialog.querySelectorAll('button')].find(item => item.textContent === text).click(); await settle(); return title; };
  const palette = () => [...document.querySelectorAll('#properties .object-palette-row')].map(row => {
    const field = row.querySelector('input, select');
    return row.querySelector('.object-palette-caption').textContent + ': ' + (field ? (field.tagName === 'SELECT' ? field.selectedOptions[0]?.textContent : field.value) : '') +
      (row.querySelector('.palette-unset') ? ' (не задано)' : '');
  });
  const paletteButton = async (caption, title) => { [...document.querySelectorAll('#properties .object-palette-row')].find(row => row.querySelector('.object-palette-caption').textContent === caption)
    .querySelector('button[title^="' + title + '"]').click(); await settle(); };
  const panel = async title => { host.querySelector('.object-window-panel button[title="' + title + '"]').click(); await settle(); };
  await openObject('Обмен');
  const packageRow = [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.querySelector(':scope > span:not(.studio-tree-sign)')?.textContent === 'Обмен');
  packageRow.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  steps.menu = [...document.querySelectorAll('.context-menu .context-menu-item')].slice(0, 3).map(item => item.textContent + (item.disabled ? ' (недоступна)' : '') +
    (getComputedStyle(item).fontWeight >= 600 ? ' (жирный)' : ''));
  closeContextMenu();
  steps.title = host.querySelector('.object-window-title').textContent;
  steps.opened = tree();
  steps.packagePalette = {title: document.querySelector('#inspector-title').textContent, rows: palette()};
  [...document.querySelectorAll('#properties .object-palette-row button')].find(item => item.textContent === 'Открыть').click(); await settle();
  steps.rootChosen = rows().filter(row => row.classList.contains('selected')).map(row => row.textContent);
  steps.rootPalette = palette();
  steps.rootMenu = await menu();
  steps.rootSigns = menuSigns();
  document.body.click(); await settle();
  for (const caption of ['Директива импорта', 'Свойство', 'Тип значения', 'Тип объекта']) { await choose('Пакет (http://example.org/goods)'); await add(caption); }
  steps.added = tree();
  await choose('ТипЗначения1');
  steps.valueMenu = await menu();
  steps.valueSigns = menuSigns();
  await add('Образец');
  await choose('ТипЗначения1');
  await add('Перечисление');
  await choose('ТипЗначения1');
  await add('Определение типа');
  await choose('ТипОбъекта1');
  steps.objectMenu = await menu();
  await add('Свойство');
  steps.innerProperty = palette();
  steps.propertyMenu = await menu();
  steps.propertySigns = menuSigns();
  await add('Определение типа');
  steps.question = await answer('{xdto}ObjectType');
  await choose('ТипОбъекта1');
  await choose('Свойство1');
  steps.propertyMenuAfter = (await menu()).filter(item => item.startsWith('Определение'));
  document.body.click(); await settle();
  await paletteButton('Минимальное количество', 'Задать значение');
  await paletteButton('Возможно пустое', 'Задать значение');
  steps.said = palette();
  await paletteButton('Возможно пустое', 'Очистить');
  steps.taken = palette().filter(row => row.startsWith('Возможно'));
  await paletteButton('Тип', 'Задать значение');
  const choice = document.querySelector('dialog.xml-type-choice-dialog');
  const spaces = [...choice.querySelector('.xml-type-choice').children].map(item => (item.matches('.studio-tree-row') ? item : item.querySelector(':scope > .studio-tree-row')).textContent);
  const own = [...choice.querySelector('.xml-type-choice').children].find(item => item.querySelector(':scope > .studio-tree-row')?.textContent === 'http://example.org/goods');
  steps.typeWindow = {count: spaces.length, first: spaces[0], last: spaces.at(-1), own: [...own.querySelectorAll('.studio-tree-children .studio-tree-row')].map(item => item.textContent),
    ok: [...choice.querySelectorAll('.localized-title-actions button')].find(item => item.textContent === 'OK').disabled};
  [...own.querySelectorAll('.studio-tree-children .studio-tree-row')].find(item => item.textContent === 'ТипЗначения1').click(); await settle();
  [...choice.querySelectorAll('.localized-title-actions button')].find(item => item.textContent === 'OK').click(); await settle();
  steps.typed = palette().filter(row => row.startsWith('Тип:'));
  await choose('Пакет (http://example.org/goods)');
  await add('Свойство');
  await panel('Переместить вверх');
  steps.moved = tree();
  steps.signs = rows().map(row => row.textContent + ': ' + sign(row));
  await choose('НовыйЭлемент1');
  await panel('Удалить');
  steps.deleteQuestion = await answer('Да');
  steps.deleted = tree();
  document.querySelector('#save').click(); await settle();
  steps.unsaved = !document.querySelector('#save').disabled;
  await openObject('Обмен');
  steps.reopened = tree();
  await openObject('XDTO-пакеты');
  document.querySelector('#tree-add').click(); await settle();
  steps.created = {title: host.querySelector('.object-window-title').textContent, tree: tree(), palette: palette()};
  return steps;
})()`

// The editor of an XDTO package is the prototype's (docs/block-00/
// paket-xdto.md, «Окно — редактор пакета», «Узлы редактора»; snapshots
// paket-xdto/02-04, series-2.584/139-178): titled «XDTO пакет <Имя>:
// Пакет» over the palette of the package - the name, the synonym, the
// comment, the URI, «Пакет: Открыть», which chooses the root «Пакет
// (<пространство>)» with its palette - the URI and the two qualified forms
// not said, «Ложь» greyed; «Добавить ▾» at the root
// offers the import, the property, the value type, the object type, which
// stand in their folders in that order under the names of the prototype;
// at a value type the definition of a type, the pattern, the enumeration
// («НовыйЭлемент1» in its folder); at an object type the property, whose
// palette says the name alone; at a property the definition, asked
// «Выберите тип объекта», then unavailable; ✓ says the default of a value,
// ✕ takes it back; a node moves up among its siblings; «Удалить» asks
// «Удалить элемент?» and takes the enumeration away; «Сохранить» writes the body in the prototype's form,
// read again the same. «Добавить» makes «ПакетXDTO1» of
// «http://www.sample-package.org», its body saying the qualified form of an
// element, not of an attribute, and opens its editor (paket-xdto.md, «Новый
// пакет»).
// Its menu opens with «Открыть пакет» in bold, «Проверить пакет»
// unavailable until the panel «Служебные сообщения» and «Экспорт
// XML-схемы…».
// Every node bears the sign of its kind, as the prototype's tree draws it
// (snapshots paket-xdto/02, series-2.584/172, 176), a folder the sign of
// what it holds, «Определения типов» a T with two squares, a definition
// under a property the ball with the square below; every item of
// «Добавить ▾» the sign of what it adds (paket-xdto/04, series-2.584/159,
// 174), «Образец» too.
// Defects caught: a new package not the prototype's; the menu not the
// prototype's; a node, a folder, a caption or a name not the
// prototype's, or out of its order; «Добавить» offering what the node does
// not hold; a node or an item of «Добавить ▾» without its sign or with
// the sign of another kind; a value not said written, or said and not written; the body not
// written, written in another form, or read again otherwise.
func TestTheEditorOfAnXDTOPackageIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	writeXDTOPackageForStudioTest(t, root)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, xdtoPackageWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Title                                                              string
		Opened, RootPalette, RootMenu, Added, ValueMenu, ObjectMenu        []string
		InnerProperty, PropertyMenu, PropertyMenuAfter, Said, Taken, Moved []string
		Deleted, Reopened, Signs, RootSigns, ValueSigns, PropertySigns     []string
		Question, DeleteQuestion                                           string
		PackagePalette                                                     struct {
			Title string
			Rows  []string
		}
		RootChosen []string
		TypeWindow struct {
			Count       int
			First, Last string
			Own         []string
			OK          bool
		}
		Typed   []string
		Menu    []string
		Created struct {
			Title         string
			Tree, Palette []string
		}
		Unsaved bool
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
	expect("the title", steps.Title, "XDTO пакет Обмен: Пакет")
	expect("the menu of the package", steps.Menu, []string{"Открыть пакет (жирный)", "Проверить пакет (недоступна)", "Экспорт XML-схемы…"})
	expect("opened", steps.Opened, []string{"Пакет (http://example.org/goods)"})
	expect("the palette of the package", steps.PackagePalette, struct {
		Title string
		Rows  []string
	}{"Свойства: Обмен", []string{"Имя: Обмен", "Синоним: Обмен", "Комментарий: ", "URI пространства имен: http://example.org/goods", "Пакет: "}})
	expect("«Пакет: Открыть» chooses the root", steps.RootChosen, []string{"Пакет (http://example.org/goods)"})
	expect("the palette of the root", steps.RootPalette, []string{"URI пространства имен: http://example.org/goods",
		"Квалифицированная форма элемента: Ложь (не задано)", "Квалифицированная форма атрибута: Ложь (не задано)"})
	expect("«Добавить» at the root", steps.RootMenu, []string{"Директива импорта", "Свойство", "Тип значения", "Тип объекта"})
	expect("added", steps.Added, []string{"Пакет (http://example.org/goods)", "  Директивы импорта", "    http://www.imported-package.org",
		"  Свойства", "    Свойство1", "  Типы значений", "    ТипЗначения1", "  Типы объектов", "    ТипОбъекта1"})
	expect("«Добавить» at a value type", steps.ValueMenu, []string{"Определение типа", "Образец", "Перечисление"})
	expect("«Добавить» at an object type", steps.ObjectMenu, []string{"Свойство"})
	expect("a property of an object type", steps.InnerProperty, []string{"Имя: Свойство1", "Ссылка:  (не задано)", "Тип:  (не задано)",
		"Минимальное количество: 1 (не задано)", "Максимальное количество: 1 (не задано)", "Возможно пустое: Ложь (не задано)",
		"Фиксированное: Ложь (не задано)", "По умолчанию:  (не задано)", "Форма: Элемент (не задано)", "Локальное имя:  (не задано)",
		"Квалифицированное:  (не задано)"})
	expect("«Добавить» at a property", steps.PropertyMenu, []string{"Определение типа"})
	expect("the question of a definition", steps.Question, "Выберите тип объекта")
	expect("a property holding its definition", steps.PropertyMenuAfter, []string{"Определение типа (недоступна)"})
	expect("said by ✓", steps.Said[3:6], []string{"Минимальное количество: 1", "Максимальное количество: 1 (не задано)", "Возможно пустое: Ложь"})
	expect("taken back by ✕", steps.Taken, []string{"Возможно пустое: Ложь (не задано)"})
	// Every namespace - SOAP, the platform's 66, the package's own, XML
	// Schema - by the alphabet of the URI; the package's own holding the
	// types the editor holds; OK unavailable until a type is marked.
	expect("the window choosing a type of a node", steps.TypeWindow, struct {
		Count       int
		First, Last string
		Own         []string
		OK          bool
	}{69, "http://example.org/goods", "http://www.w3.org/2001/XMLSchema", []string{"ТипЗначения1", "ТипОбъекта1"}, true})
	expect("a type of a node chosen", steps.Typed, []string{"Тип: ТипЗначения1 (http://example.org/goods)"})
	expect("moved up", steps.Moved[3:6], []string{"  Свойства", "    Свойство2", "    Свойство1"})
	expect("the question of a deletion", steps.DeleteQuestion, "Удалить элемент?")
	expect("deleted", steps.Deleted, []string{"Пакет (http://example.org/goods)", "  Директивы импорта", "    http://www.imported-package.org",
		"  Свойства", "    Свойство2", "    Свойство1", "  Типы значений", "    ТипЗначения1", "      Определения типов", "        Определение типа",
		"      Образец", "        НовыйЭлемент1", "  Типы объектов", "    ТипОбъекта1", "      Свойство1", "        Определение типа"})
	expect("the signs of the nodes", steps.Signs, []string{"Пакет (http://example.org/goods): xdto-package",
		"Директивы импорта: xdto-import", "http://www.imported-package.org: xdto-import",
		"Свойства: xdto-property", "Свойство2: xdto-property", "Свойство1: xdto-property",
		"Типы значений: xdto-value-type", "ТипЗначения1: xdto-value-type", "Определения типов: xdto-type-definitions",
		"Определение типа: xdto-value-type", "Образец: xdto-pattern", "НовыйЭлемент1: xdto-pattern",
		"Перечисление: xdto-enumeration", "НовыйЭлемент1: xdto-enumeration",
		"Типы объектов: xdto-object-type", "ТипОбъекта1: xdto-object-type", "Свойство1: xdto-property",
		"Определение типа: xdto-property-type-definition"})
	expect("the signs of «Добавить» at the root", steps.RootSigns, []string{"xdto-import", "xdto-property", "xdto-value-type", "xdto-object-type"})
	expect("the signs of «Добавить» at a value type", steps.ValueSigns, []string{"xdto-value-type", "xdto-pattern", "xdto-enumeration"})
	expect("the signs of «Добавить» at a property", steps.PropertySigns, []string{"xdto-property-type-definition"})
	expect("saved", steps.Unsaved, false)
	expect("reopened", steps.Reopened, steps.Deleted)
	expect("a new package", steps.Created, struct {
		Title         string
		Tree, Palette []string
	}{"XDTO пакет ПакетXDTO1: Пакет", []string{"Пакет (http://www.sample-package.org)"},
		[]string{"Имя: ПакетXDTO1", "Синоним: ", "Комментарий: ", "URI пространства имен: http://www.sample-package.org", "Пакет: "}})
	created, err := os.ReadFile(filepath.Join(root, "metadata", "xdto-packages", "ПакетXDTO1", "content.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "\uFEFF<package xmlns=\"http://v8.1c.ru/8.1/xdto\" xmlns:xs=\"http://www.w3.org/2001/XMLSchema\" xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\" " +
		"targetNamespace=\"http://www.sample-package.org\" elementFormQualified=\"true\" attributeFormQualified=\"false\"/>"; string(created) != want {
		t.Errorf("the body of a new package: %q", created)
	}
	written, err := os.ReadFile(filepath.Join(root, "metadata", "xdto-packages", "Обмен", "content.xml"))
	if err != nil {
		t.Fatal(err)
	}
	want := "\uFEFF<package xmlns=\"http://v8.1c.ru/8.1/xdto\" xmlns:xs=\"http://www.w3.org/2001/XMLSchema\" xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\" targetNamespace=\"http://example.org/goods\">\r\n" +
		"\t<import namespace=\"http://www.imported-package.org\"/>\r\n" +
		"\t<property name=\"Свойство2\"/>\r\n" +
		"\t<property name=\"Свойство1\"/>\r\n" +
		"\t<valueType name=\"ТипЗначения1\">\r\n" +
		"\t\t<typeDef xsi:type=\"ValueType\"/>\r\n" +
		"\t\t<pattern>НовыйЭлемент1</pattern>\r\n" +
		"\t</valueType>\r\n" +
		"\t<objectType name=\"ТипОбъекта1\">\r\n" +
		"\t\t<property xmlns:d3p1=\"http://example.org/goods\" name=\"Свойство1\" type=\"d3p1:ТипЗначения1\" lowerBound=\"1\">\r\n" +
		"\t\t\t<typeDef xsi:type=\"ObjectType\"/>\r\n" +
		"\t\t</property>\r\n" +
		"\t</objectType>\r\n" +
		"</package>"
	if string(written) != want {
		t.Errorf("the body written:\n%s\nwant\n%s", strings.ReplaceAll(string(written), "\r", ""), strings.ReplaceAll(want, "\r", ""))
	}
}

// writeXDTOPackageForStudioTest writes the package Обмен of the namespace
// http://example.org/goods with no body.
func writeXDTOPackageForStudioTest(t *testing.T, root string) {
	t.Helper()
	folder := filepath.Join(root, "metadata", "xdto-packages", "Обмен")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	description := "format: 1\nid: f2000000-0000-4000-8000-000000000001\nname: Обмен\ntitle: {ru: Обмен}\nnamespace: http://example.org/goods\n"
	if err := os.WriteFile(filepath.Join(folder, "object.yaml"), []byte(description), 0o644); err != nil {
		t.Fatal(err)
	}
}
