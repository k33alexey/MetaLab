package studio

import (
	"encoding/json"
	"slices"
	"testing"
)

// typeAloneWalk opens the window of the type of the attribute Артикул of the
// catalog Товары, a string, turns «Составной тип данных» on, marks a number
// beside it, then a value storage: answers «Нет» to the question, marks the
// value storage again and answers «Да»; then marks a number beside the value
// storage and answers «Да» again. It answers the question asked and the rows
// marked after each step.
const typeAloneWalk = `(async () => {
  const studio = window.__studio, seen = {};
  const pause = () => new Promise(resolve => setTimeout(resolve, 50));
  const top = () => [...document.querySelectorAll('dialog[open]')].at(-1);
  const editor = () => [...document.querySelectorAll('dialog[open] .type-editor')].at(-1);
  const row = caption => [...editor().querySelectorAll('.type-editor-row')].find(line => line.querySelector('.type-editor-caption').textContent === caption);
  const marked = () => [...editor().querySelectorAll('.type-editor-row')].filter(line => line.querySelector('input')?.checked)
    .map(line => line.querySelector('.type-editor-caption').textContent);
  const mark = async caption => { row(caption).querySelector('input').click(); await pause(); };
  const question = () => top().querySelector('.type-editor') ? '' : top().querySelector('h3').textContent;
  const answer = async text => { [...top().querySelectorAll('button')].find(button => button.textContent === text).click(); await pause(); };
  [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === 'Товары').click();
  await studio.settle();
  [...document.querySelectorAll('.object-window-page')].find(item => item.textContent === 'Данные').click();
  await studio.settle();
  [...document.querySelectorAll('#catalog-editor .object-window-body .studio-tree-row')].find(item => item.textContent.trim() === 'Артикул').click();
  await studio.settle();
  document.querySelector('#properties button.field-choice[title="Редактирование типа данных"]').click();
  await pause();
  [...editor().querySelectorAll('label')].find(label => label.textContent.includes('Составной тип данных')).querySelector('input').click();
  await pause();
  await mark('Число');
  seen.beside = {question: question(), marked: marked()};
  await mark('ХранилищеЗначения');
  seen.storage = question();
  await answer('Нет');
  seen.no = marked();
  await mark('ХранилищеЗначения');
  await answer('Да');
  seen.yes = marked();
  await mark('Число');
  seen.earlier = question();
  await answer('Да');
  seen.after = marked();
  return seen;
})()`

// A type the place takes only by itself asks, as the prototype's window
// does, before it takes the others away - «Выбран тип, который не может быть
// включен в составной тип данных. Будут исключены остальные типы данных.
// Продолжить?» (snapshot configurator-2026-10-24/series-2.531/12) - and a
// type marked beside one that does asks «Ранее был выбран тип, который не
// может быть включен в составной тип данных и будет исключен. Продолжить?»
// (configurator-2026-10-25/series-2.535/10); «Нет» leaves the marks as they
// were, «Да» leaves the new type alone. The texts are written out from the
// snapshots. Defect caught: the window letting a value storage stand beside a
// string in a composite, which the save then refuses («stands only by
// itself»), as it did until 2.553; a question where types may stand together;
// «Нет» marking all the same.
func TestTheTypeWindowAsksBeforeATypeThatStandsAloneTakesTheOthersAway(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	answer, err := browser.evaluate(ctx, page, typeAloneWalk)
	if err != nil {
		t.Fatal(err)
	}
	var seen struct {
		Beside struct {
			Question string   `json:"question"`
			Marked   []string `json:"marked"`
		} `json:"beside"`
		Storage, Earlier string
		No, Yes, After   []string
	}
	if err := json.Unmarshal(answer, &seen); err != nil {
		t.Fatalf("the walk answered %s: %v", answer, err)
	}
	if seen.Beside.Question != "" || !slices.Equal(seen.Beside.Marked, []string{"Число", "Строка"}) {
		t.Errorf("a number beside a string: asked %q, marked %v", seen.Beside.Question, seen.Beside.Marked)
	}
	if want := "Выбран тип, который не может быть включен в составной тип данных. Будут исключены остальные типы данных. Продолжить?"; seen.Storage != want {
		t.Errorf("a value storage beside them asks %q, want %q", seen.Storage, want)
	}
	if !slices.Equal(seen.No, []string{"Число", "Строка"}) {
		t.Errorf("after «Нет» the marks are %v", seen.No)
	}
	if !slices.Equal(seen.Yes, []string{"ХранилищеЗначения"}) {
		t.Errorf("after «Да» the marks are %v", seen.Yes)
	}
	if want := "Ранее был выбран тип, который не может быть включен в составной тип данных и будет исключен. Продолжить?"; seen.Earlier != want {
		t.Errorf("a number beside the value storage asks %q, want %q", seen.Earlier, want)
	}
	if !slices.Equal(seen.After, []string{"Число"}) {
		t.Errorf("after the second «Да» the marks are %v", seen.After)
	}
}
