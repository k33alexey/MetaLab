package studio

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// subsystemsFieldWalk opens the common picture Звезда, «…» of its field
// «Подсистемы», reads the window, marks Склад and the root, presses OK and
// «Сохранить», reading «Состав» of the root; then the style Основной: marks
// Продажи by the window, «Отмена» once, OK the next time with the root
// too, «Сохранить»;
// then the picture again: takes the mark of the root away and saves; then
// the style again.
const subsystemsFieldWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 120)).then(() => studio.settle());
  const dialog = () => document.querySelector('dialog[open]');
  const boxes = () => [...dialog().querySelectorAll('.studio-tree-row')].map(row => row.textContent + (row.querySelector('input') ? (row.querySelector('input').disabled ? ' -' : row.querySelector('input').checked ? ' ✓' : ' ·') : ''));
  const content = async () => (await (await fetch('/api/project-configuration')).json()).configuration.content || [];
  const choose = async (host, marks, answer) => {
    windowField(host, 'Подсистемы:').querySelector('button').click(); await settle();
    const read = {title: dialog().querySelector('h3').textContent, rows: boxes(),
      buttons: [...dialog().querySelectorAll('.localized-title-actions button')].map(item => item.textContent)};
    for (const mark of marks) dialog().querySelector('input[aria-label="' + mark + '"]').click();
    read.marked = boxes();
    [...dialog().querySelectorAll('.localized-title-actions button')].find(item => item.textContent === answer).click(); await settle();
    read.field = windowField(host, 'Подсистемы:').querySelector('input').value;
    return read;
  };
  await openObject('Звезда');
  steps.picture = await choose('#picture-editor', ['Склад', 'ПродажиИСклад'], 'OK');
  steps.contentBefore = await content();
  document.querySelector('#save').click(); await settle();
  steps.pictureSaved = document.querySelector('#save').disabled;
  steps.contentSaved = await content();
  await openObject('Основной');
  steps.styleCancelled = await choose('#style-editor', ['Продажи'], 'Отмена');
  steps.style = await choose('#style-editor', ['Продажи', 'ПродажиИСклад'], 'OK');
  document.querySelector('#save').click(); await settle();
  steps.contentStyle = await content();
  await openObject('Звезда');
  steps.reopened = windowField('#picture-editor', 'Подсистемы:').querySelector('input').value;
  steps.unrooted = await choose('#picture-editor', ['ПродажиИСклад'], 'OK');
  document.querySelector('#save').click(); await settle();
  steps.contentAfter = await content();
  await openObject('Основной');
  steps.styleReopened = windowField('#style-editor', 'Подсистемы:').querySelector('input').value;
  return steps;
})()`

// The field «Подсистемы» of the window of a common picture and of a style
// chooses in the window «Подсистемы» (obshchee.md, «Окно объекта», «Окно
// поля „Подсистемы“ с „…“», «Окно поля „Подсистемы“»; snapshots
// dosyomka-2653/25-39): the tree of the page «Подсистемы» under the root of
// the project, OK, «Отмена», «Справка»; the box of the root is a mark of
// its own, nothing below it marked by it; OK shows the root first and the
// subsystems marked in the field, «Сохранить» writes the object into the
// content of the subsystems and into «Состав» of the root, «Отмена»
// changes nothing; the window opened again starts from what is written,
// and the mark of the root taken away takes the object out of «Состав».
// Defects caught: «…» unavailable; the window not the page's tree or
// without the root; the box of the root unavailable, marking below it or
// not written; «Состав» written before «Сохранить»; the field not showing
// the root; the style not written into «Состав», or written over the
// picture, not after it; the box not starting
// from what is written; the mark taken away leaving the object there.
func TestTheFieldOfSubsystemsChoosesInTheWindowSubsystems(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	var picture, style uuid.UUID
	subsystems := map[string]string{}
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		made, err := workspace.CreatePicture("Звезда")
		if err != nil {
			t.Fatal(err)
		}
		picture = made.Picture.ID
		styled, err := workspace.CreateStyle("Основной")
		if err != nil {
			t.Fatal(err)
		}
		style = styled.Style.ID
		for _, name := range []string{"Продажи", "Склад"} {
			subsystem, err := workspace.CreateSubsystem(name)
			if err != nil {
				t.Fatal(err)
			}
			subsystems[name] = subsystem.Path
		}
	})
	result, err := browser.evaluate(ctx, page, subsystemsFieldWalk)
	if err != nil {
		t.Fatal(err)
	}
	type window struct {
		Title, Field          string
		Rows, Buttons, Marked []string
	}
	var steps struct {
		Picture, Style, StyleCancelled, Unrooted                window
		PictureSaved                                            bool
		Reopened, StyleReopened                                 string
		ContentBefore, ContentSaved, ContentStyle, ContentAfter []uuid.UUID
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	want := window{Title: "Подсистемы", Rows: []string{"ПродажиИСклад ·", "Продажи ·", "Склад ·"}, Buttons: []string{"OK", "Отмена", "Справка"},
		Marked: []string{"ПродажиИСклад ✓", "Продажи ·", "Склад ✓"}, Field: "ПродажиИСклад,Склад"}
	gotJSON, _ := json.Marshal(steps.Picture)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("the window of the picture: %s, want %s", gotJSON, wantJSON)
	}
	if steps.StyleCancelled.Field != "" || steps.Style.Field != "ПродажиИСклад,Продажи" || !steps.PictureSaved {
		t.Errorf("the style's field after «Отмена» %q, after OK %q; the picture saved %v", steps.StyleCancelled.Field, steps.Style.Field, steps.PictureSaved)
	}
	if len(steps.ContentBefore) != 0 || !slices.Equal(steps.ContentSaved, []uuid.UUID{picture}) || !slices.Equal(steps.ContentStyle, []uuid.UUID{picture, style}) || !slices.Equal(steps.ContentAfter, []uuid.UUID{style}) {
		t.Errorf("«Состав» of the root before «Сохранить» %v, after it %v, after the style %v, after the mark taken away %v; want none, the picture, the picture and the style, the style",
			steps.ContentBefore, steps.ContentSaved, steps.ContentStyle, steps.ContentAfter)
	}
	if steps.StyleReopened != "ПродажиИСклад,Продажи" {
		t.Errorf("the style opened again shows %q, want the root and Продажи", steps.StyleReopened)
	}
	if steps.Reopened != "ПродажиИСклад,Склад" || !slices.Equal(steps.Unrooted.Rows, []string{"ПродажиИСклад ✓", "Продажи ·", "Склад ✓"}) || steps.Unrooted.Field != "Склад" {
		t.Errorf("the picture opened again shows %q, its window %v, the field after the root taken away %q", steps.Reopened, steps.Unrooted.Rows, steps.Unrooted.Field)
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	for name, member := range map[string]uuid.UUID{"Склад": picture, "Продажи": style} {
		opened, err := workspace.ReadSubsystemEditor(subsystems[name])
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(opened.Subsystem.Members, member) {
			t.Errorf("%s holds %v, want the object written into it", name, opened.Subsystem.Members)
		}
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
