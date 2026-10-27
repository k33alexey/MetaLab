package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// wsViewDescription is a description written for the test: a schema of a
// global element of its own type and an enumeration, importing «1.xsd»; a
// service of one port and one operation, unwrapped.
const wsViewDescription = `<definitions xmlns="http://schemas.xmlsoap.org/wsdl/" xmlns:tns="urn:shop" xmlns:s="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:shop">
  <types>
    <s:schema targetNamespace="urn:shop" elementFormDefault="qualified">
      <s:import namespace="urn:goods" schemaLocation="file://1.xsd"/>
      <s:element name="Price"><s:complexType><s:sequence><s:element name="code" type="s:string" minOccurs="1"/></s:sequence></s:complexType></s:element>
      <s:element name="PriceResponse"><s:complexType><s:sequence><s:element name="sum" type="s:decimal" minOccurs="0"/></s:sequence></s:complexType></s:element>
      <s:simpleType name="Kind"><s:restriction base="s:string"><s:enumeration value="Home"/></s:restriction></s:simpleType>
    </s:schema>
  </types>
  <message name="PriceIn"><part name="parameters" element="tns:Price"/></message>
  <message name="PriceOut"><part name="parameters" element="tns:PriceResponse"/></message>
  <portType name="ShopPort"><operation name="Price"><documentation>Цена товара</documentation><input message="tns:PriceIn"/><output message="tns:PriceOut"/></operation></portType>
  <binding name="ShopSoap" type="tns:ShopPort"/>
  <service name="Shop"><port name="ShopSoap" binding="tns:ShopSoap"/></service>
</definitions>`

// wsViewWalk opens the window viewing the description by «Просмотр» of the
// menu of the reference and reads it: the title, the tree, the palettes of
// the description, of an operation, a parameter, a property and a package;
// closes it and opens it again by «WS-ссылка: Открыть» of the palette and
// by a double click.
const wsViewWalk = `(async () => {
  const studio = window.__studio, steps = {};` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 80)).then(() => studio.settle());
  const view = () => document.querySelector('dialog.ws-reference-view');
  const tree = () => [...view().querySelectorAll('.studio-tree-row')].map(row => row.textContent);
  const palette = () => [view().querySelector('.ws-reference-view-palette-title')?.textContent || '',
    ...[...view().querySelectorAll('.read-only-palette-row')].map(row => row.querySelector('.read-only-palette-caption').textContent + ': ' +
      row.querySelector('.read-only-palette-value').textContent + (row.querySelector('.read-only-palette-value use') ? ' ✓' : '') +
      (row.querySelector('.palette-unset') ? ' (не задано)' : ''))];
  const choose = async caption => { [...view().querySelectorAll('.studio-tree-row')].filter(row => row.textContent === caption).pop().click(); await settle(); };
  await openObject('Склад');
  const row = [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.querySelector(':scope > span:not(.studio-tree-sign)')?.textContent === 'Склад');
  row.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  steps.menu = [...document.querySelectorAll('.context-menu .context-menu-item')].slice(0, 2).map(item => item.textContent + (item.disabled ? ' (недоступна)' : '') +
    (getComputedStyle(item).fontWeight >= 600 ? ' (жирный)' : ''));
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent === 'Просмотр').click(); await settle();
  steps.title = view().querySelector('h3').textContent;
  steps.tree = tree();
  steps.root = palette();
  await choose('Price'); steps.operation = palette();
  await choose('code'); steps.parameter = palette();
  steps.chosenShown = !view().querySelector('.studio-tree-row.selected').closest('details:not([open])');
  await choose('Kind'); steps.valueType = palette().slice(0, 4);
  await choose('Пакет (urn:shop)'); steps.package = palette();
  [...view().querySelectorAll('button')].find(item => item.textContent === 'Закрыть').click(); await settle();
  steps.closed = !view();
  [...document.querySelectorAll('#properties .object-palette-row button')].find(item => item.textContent === 'Открыть').click(); await settle();
  steps.fromPalette = !!view();
  view().close(); await settle();
  row.dispatchEvent(new MouseEvent('dblclick', {bubbles: true, cancelable: true})); await settle();
  steps.fromDoubleClick = !!view();
  return steps;
})()`

// The window viewing the description of a WS reference is the prototype's
// (ws-ssylka.md, «Окно»; snapshots ws-ssylka/04-09, series-2.595/16-34):
// «Просмотр», bold, first in the menu of the reference, «WS-ссылка:
// Открыть» and a double click open «WS-ссылка <Имя>: WS-ссылка [Только для
// чтения]»: «Модель данных» - a package a schema, the imported one too,
// with its folders - and «Web-сервисы» - the service, its port, the
// operation, its parameter; the palette of the description, of the
// operation - its documentation, its type of return, «Возможно пустое» - of
// the parameter, of a value type - the values the schema says marked ✓,
// the rest their defaults - and of the package.
// Defects caught: the window not opening by one of its three ways; a node
// or a palette not the model's; a value of the description not marked, or
// a default shown as said.
func TestTheWindowViewingAWSReferenceIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	folder := filepath.Join(root, "metadata", "ws-references", "Склад")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"object.yaml":    "format: 1\nid: f6000000-0000-4000-8000-000000000001\nname: Склад\ntitle: {ru: Склад}\n",
		"definition.xml": wsViewDescription,
		"1.xsd":          `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:goods"><xs:element name="Item" type="xs:string"/></xs:schema>`,
	} {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, wsViewWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Menu, Tree, Root, Operation, Parameter, ValueType, Package []string
		Title                                                      string
		Closed, FromPalette, FromDoubleClick                       bool
		ChosenShown                                                bool
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
	expect("the menu", steps.Menu, []string{"Просмотр (жирный)", "Импорт WSDL…"})
	expect("the title", steps.Title, "WS-ссылка Склад: WS-ссылка [Только для чтения]")
	expect("the tree", steps.Tree, []string{"Модель данных", "Пакет (urn:shop)", "Директивы импорта", "urn:goods", "Свойства", "Price", "PriceResponse",
		"Типы значений", "Kind", "Перечисление", "Home", "Типы объектов", "Price", "code", "PriceResponse", "sum",
		"Пакет (urn:goods)", "Свойства", "Item", "Web-сервисы", "Shop", "ShopSoap", "Price", "code"})
	expect("the palette of the description", steps.Root, []string{"Свойства: Описание определения Web-сервисов", "Описание определения Web-сервисов: "})
	expect("an operation", steps.Operation, []string{"Свойства: Price", "Имя: Price", "Описание операции: Цена товара",
		"Тип возвращаемого значения: decimal (http://www.w3.org/2001/XMLSchema)", "Возможно пустое: Истина", "Описание возвращаемого значения: "})
	expect("a parameter", steps.Parameter, []string{"Свойства: code", "Имя: code", "Тип: string (http://www.w3.org/2001/XMLSchema)", "Направление: Входной",
		"Возможно пустое: Ложь", "Описание параметра операции: "})
	expect("a value type", steps.ValueType, []string{"Свойства: Kind", "Имя: Kind ✓", "Базовый тип: string (http://www.w3.org/2001/XMLSchema) ✓",
		"Вариант: Атомарный (не задано)"})
	expect("a package", steps.Package, []string{"Свойства: Пакет (urn:shop)", "URI пространства имен: urn:shop",
		"Квалифицированная форма элемента: Истина ✓", "Квалифицированная форма атрибута: Ложь (не задано)"})
	expect("the node chosen stands shown", steps.ChosenShown, true)
	expect("the three ways", []bool{steps.Closed, steps.FromPalette, steps.FromDoubleClick}, []bool{true, true, true})
}
