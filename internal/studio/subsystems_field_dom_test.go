package studio

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// subsystemsFieldWalk opens the common picture Звезда, «…» of its field
// «Подсистемы», reads the window, marks Склад, presses OK and «Сохранить»;
// then the style Основной: marks Продажи by the window, «Отмена» once,
// OK the next time, «Сохранить».
const subsystemsFieldWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 120)).then(() => studio.settle());
  const dialog = () => document.querySelector('dialog[open]');
  const choose = async (host, mark, answer) => {
    windowField(host, 'Подсистемы:').querySelector('button').click(); await settle();
    const read = {title: dialog().querySelector('h3').textContent,
      rows: [...dialog().querySelectorAll('.studio-tree-row')].map(row => row.textContent + (row.querySelector('input') ? (row.querySelector('input').disabled ? ' -' : row.querySelector('input').checked ? ' ✓' : ' ·') : '')),
      buttons: [...dialog().querySelectorAll('.localized-title-actions button')].map(item => item.textContent)};
    dialog().querySelector('input[aria-label="' + mark + '"]').click();
    [...dialog().querySelectorAll('.localized-title-actions button')].find(item => item.textContent === answer).click(); await settle();
    read.field = windowField(host, 'Подсистемы:').querySelector('input').value;
    return read;
  };
  await openObject('Звезда');
  steps.picture = await choose('#picture-editor', 'Склад', 'OK');
  document.querySelector('#save').click(); await settle();
  steps.pictureSaved = document.querySelector('#save').disabled;
  await openObject('Основной');
  steps.styleCancelled = await choose('#style-editor', 'Продажи', 'Отмена');
  steps.style = await choose('#style-editor', 'Продажи', 'OK');
  document.querySelector('#save').click(); await settle();
  return steps;
})()`

// The field «Подсистемы» of the window of a common picture and of a style
// chooses in the window «Подсистемы» (obshchee.md, «Окно объекта», «Окно
// поля „Подсистемы“ с „…“»): the tree of the page «Подсистемы» under the
// root of the project - its box unavailable, what it does not shot (В4) -
// OK, «Отмена», «Справка»; OK shows the subsystems marked in the field and
// «Сохранить» writes the object into their content, «Отмена» changes
// nothing. Defects caught: «…» unavailable; the window not the page's
// tree or without the root; OK not shown in the field; the picture or the
// style not written into the content of the subsystem; «Отмена» marking.
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
		Title, Field  string
		Rows, Buttons []string
	}
	var steps struct {
		Picture, Style, StyleCancelled window
		PictureSaved                   bool
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	want := window{Title: "Подсистемы", Rows: []string{"ПродажиИСклад -", "Продажи ·", "Склад ·"}, Buttons: []string{"OK", "Отмена", "Справка"}, Field: "Склад"}
	gotJSON, _ := json.Marshal(steps.Picture)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("the window of the picture: %s, want %s", gotJSON, wantJSON)
	}
	if steps.StyleCancelled.Field != "" || steps.Style.Field != "Продажи" || !steps.PictureSaved {
		t.Errorf("the style's field after «Отмена» %q, after OK %q; the picture saved %v", steps.StyleCancelled.Field, steps.Style.Field, steps.PictureSaved)
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
