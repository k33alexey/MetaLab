package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
)

// xdtoUnionWalk opens the value type Ключ of the package Обмен, says its
// members by ✓ in «Редактирование списка значений» - two rows chosen by the
// window of a type, a copy taken away, an empty row left - saves, opens the
// list again and leaves it by «Отмена», and clears it by ✕.
const xdtoUnionWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 80)).then(() => studio.settle());
  const host = document.querySelector('#xdto-package-editor');
  const choose = async caption => { [...host.querySelectorAll('.studio-tree-row')].filter(row => row.textContent === caption).pop().click(); await settle(); };
  const row = () => [...document.querySelectorAll('#properties .object-palette-row')].find(item => item.querySelector('.object-palette-caption').textContent === 'Типы объединения');
  const list = () => document.querySelector('dialog.value-list-edit-dialog');
  const read = () => ({rows: [...list().querySelectorAll('.value-list-edit-row')].map(item => item.querySelector('input').value + (item.classList.contains('current') ? ' *' : '')),
    buttons: [...list().querySelectorAll('.object-window-list-head button')].map(item => item.title + (item.disabled ? ' (недоступна)' : ''))});
  const panel = async title => { list().querySelector('.object-window-list-head button[title="' + title + '"]').click(); await settle(); };
  const pickType = async (space, name) => {
    const choice = document.querySelector('dialog.xml-type-choice-dialog');
    const branch = [...choice.querySelector('.xml-type-choice').children].find(item => item.querySelector(':scope > .studio-tree-row')?.textContent === space);
    steps.spaces ??= {soap: [...[...choice.querySelector('.xml-type-choice').children].find(item => item.querySelector(':scope > .studio-tree-row')?.textContent === 'http://schemas.xmlsoap.org/soap/envelope/')
      .querySelectorAll('.studio-tree-children .studio-tree-row')].map(item => item.textContent)};
    [...branch.querySelectorAll('.studio-tree-children .studio-tree-row')].find(item => item.textContent === name).click(); await settle();
    [...choice.querySelectorAll('.localized-title-actions button')].find(item => item.textContent === 'OK').click(); await settle();
  };
  const answer = async text => { [...list().querySelectorAll('.localized-title-actions button')].find(item => item.textContent === text).click(); await settle(); };
  await openObject('Обмен');
  await choose('Ключ');
  steps.unset = {value: row().querySelector('input').value, unset: !!row().querySelector('.palette-unset')};
  row().querySelector('button[title^="Задать значение"]').click(); await settle();
  steps.empty = {title: list().querySelector('h3').textContent, caption: list().querySelector('.object-window-list-head span').textContent, ...read(),
    answers: [...list().querySelectorAll('.localized-title-actions button')].map(item => item.textContent)};
  await panel('Добавить');
  steps.added = read();
  list().querySelector('.value-list-edit-row.current .field-choice').click(); await settle();
  await pickType('http://schemas.xmlsoap.org/soap/envelope/', 'Body');
  await panel('Добавить');
  await panel('Изменить');
  await pickType('http://schemas.xmlsoap.org/soap/envelope/', 'Envelope');
  steps.two = read();
  await panel('Скопировать');
  steps.copied = read();
  await panel('Удалить');
  steps.removed = read();
  await panel('Добавить');
  await answer('OK');
  steps.field = row().querySelector('input').value;
  document.querySelector('#save').click(); await settle();
  await choose('Ключ');
  row().querySelector('.field-choice').click(); await settle();
  steps.reopened = read();
  list().querySelectorAll('.value-list-edit-row')[0].click(); await settle();
  await panel('Добавить');
  steps.inserted = read();
  list().querySelectorAll('.value-list-edit-row')[0].click(); await settle();
  await panel('Удалить');
  await answer('Отмена');
  steps.cancelled = row().querySelector('input').value;
  row().querySelector('button[title^="Очистить"]').click(); await settle();
  steps.cleared = row().querySelector('input').value;
  return steps;
})()`

// The members of a union of an XDTO package are edited as the prototype's
// (docs/block-00/paket-xdto.md, «Проверить пакет, экспорт схемы, окно
// типа»; snapshots series-2.603/19-21): ✓ and «…» of «Типы объединения»
// open «Редактирование списка значений» - «Список значений:», «Добавить»,
// «Скопировать», «Изменить», «Удалить», «Сохранить в файл»; OK, «Отмена»,
// «Справка» - a row is chosen by «…» or «Изменить» in the window of a type,
// which now holds the types of the platform's namespaces (the envelope of
// SOAP - six); the field writes «{namespace}name» joined by «; », the body
// the members as the prototype; an empty row is dropped; «Отмена» keeps
// the list; ✕ clears it.
// Defects caught: the list window not opened, or a button of it missing;
// a row added elsewhere than after the current, a copy not a copy, the
// current row not removed; a member written in another form; an empty row
// written; «Отмена» changing the list; the platform's namespaces empty.
func TestTheMembersOfAnXDTOUnionAreEditedByTheListWindow(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	writeXDTOPackageForStudioTest(t, root)
	body := "\uFEFF<package xmlns=\"http://v8.1c.ru/8.1/xdto\" xmlns:xs=\"http://www.w3.org/2001/XMLSchema\" " +
		"xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\" targetNamespace=\"http://example.org/goods\">\r\n" +
		"\t<valueType name=\"Ключ\" variety=\"Union\"/>\r\n</package>"
	content := filepath.Join(root, "metadata", "xdto-packages", "Обмен", metadata.XDTOPackageContentFile)
	if err := os.WriteFile(content, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, xdtoUnionWalk)
	if err != nil {
		t.Fatal(err)
	}
	type list struct{ Rows, Buttons []string }
	var steps struct {
		Unset struct {
			Value string
			Unset bool
		}
		Empty struct {
			Title, Caption         string
			Rows, Buttons, Answers []string
		}
		Added, Two, Copied, Removed list
		Reopened, Inserted          list
		Field, Cancelled, Cleared   string
		Spaces                      struct{ Soap []string }
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
	const soap = "{http://schemas.xmlsoap.org/soap/envelope/}"
	on := []string{"Добавить", "Скопировать", "Изменить", "Удалить", "Сохранить в файл (недоступна)"}
	expect("not said", steps.Unset, struct {
		Value string
		Unset bool
	}{"", true})
	expect("the window", steps.Empty, struct {
		Title, Caption         string
		Rows, Buttons, Answers []string
	}{"Редактирование списка значений", "Список значений:",
		[]string{}, []string{"Добавить", "Скопировать (недоступна)", "Изменить (недоступна)", "Удалить (недоступна)", "Сохранить в файл (недоступна)"},
		[]string{"OK", "Отмена", "Справка"}})
	expect("added", steps.Added, list{[]string{" *"}, on})
	expect("the envelope of SOAP in the window of a type", steps.Spaces.Soap, []string{"Body", "detail", "encodingStyle", "Envelope", "Fault", "Header"})
	expect("two", steps.Two, list{[]string{soap + "Body", soap + "Envelope *"}, on})
	expect("copied", steps.Copied, list{[]string{soap + "Body", soap + "Envelope", soap + "Envelope *"}, on})
	expect("removed", steps.Removed, list{[]string{soap + "Body", soap + "Envelope *"}, on})
	expect("the field", steps.Field, soap+"Body; "+soap+"Envelope")
	expect("reopened", steps.Reopened, list{[]string{soap + "Body *", soap + "Envelope"}, on})
	expect("inserted after the current", steps.Inserted, list{[]string{soap + "Body", " *", soap + "Envelope"}, on})
	expect("after «Отмена»", steps.Cancelled, soap+"Body; "+soap+"Envelope")
	expect("cleared", steps.Cleared, "")
	written, err := os.ReadFile(content)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), `memberTypes="{http://schemas.xmlsoap.org/soap/envelope/}Body {http://schemas.xmlsoap.org/soap/envelope/}Envelope"`) {
		t.Errorf("the body: %s", written)
	}
}
