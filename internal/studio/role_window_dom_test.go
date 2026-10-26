package studio

import (
	"encoding/json"
	"testing"
)

// roleBuilderWalk opens the role Кладовщик and reads its window and its
// palette; chooses the tab «Шаблоны ограничений», presses «Права: Открыть»
// of the palette, then types a new name into the palette.
const roleBuilderWalk = `(async () => {
  const studio = window.__studio, steps = {};
  const host = document.querySelector('#role-editor');
  const palette = () => document.querySelector('#properties .object-palette');
  [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === 'Кладовщик').click();
  await studio.settle();
  steps.title = host.querySelector('.role-window-title')?.textContent;
  steps.fieldsInWindow = host.querySelectorAll('input:not([type=checkbox]):not([type=search]), textarea').length;
  steps.flags = [...host.querySelectorAll('.role-flags .role-check')].map(item => item.textContent);
  steps.flagsLast = host.lastElementChild?.classList.contains('role-flags');
  steps.paletteTitle = document.querySelector('#inspector-title').textContent;
  steps.rows = [...palette().querySelectorAll('.object-palette-row .object-palette-caption')].map(item => item.textContent);
  const rights = [...palette().querySelectorAll('.object-palette-row')].find(row => row.textContent.startsWith('Права')).querySelector('button');
  steps.rightsAvailable = !rights.disabled;
  [...host.querySelectorAll('.studio-tab')].find(tab => tab.textContent === 'Шаблоны ограничений').click(); await studio.settle();
  steps.templates = !host.querySelector('.role-templates').hidden;
  rights.click(); await studio.settle();
  steps.rightsShown = !host.querySelector('.role-body').hidden && host.querySelector('.role-templates').hidden;
  const name = [...palette().querySelectorAll('.object-palette-row')].find(row => row.textContent.startsWith('Имя')).querySelector('input');
  name.value = 'Кладовщица'; name.dispatchEvent(new Event('input', {bubbles: true})); await studio.settle();
  steps.renamed = {title: host.querySelector('.role-window-title').textContent, palette: document.querySelector('#inspector-title').textContent,
    changed: !document.querySelector('#save').disabled};
  return steps;
})()`

// The window of a role is built as the prototype's (docs/block-00/rol.md,
// «Палитра» and «Окно»; snapshots rol/02-novaya-rol, rol/07-flagi):
// «Роль <Имя>: Права», no name, synonym or comment in it, the three flags at
// its bottom; the name, the synonym and the comment in the palette, «Свойства:
// <Имя>», with «Права: Открыть», which shows the rights from the tab of the
// templates; a name typed in the palette renames the window. Defects caught:
// the name and the flags left at the top of the window (BLOCKS, «роль по
// построителю»); the palette of a role empty; «Права: Открыть» unavailable
// or doing nothing; the window keeping the name it was opened with.
func TestTheWindowOfARoleIsBuiltAsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		if _, err := workspace.CreateRole("Кладовщик"); err != nil {
			t.Fatal(err)
		}
	})
	result, err := browser.evaluate(ctx, page, roleBuilderWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Title, PaletteTitle                                string
		FieldsInWindow                                     int
		Flags, Rows                                        []string
		FlagsLast, RightsAvailable, Templates, RightsShown bool
		Renamed                                            struct {
			Title, Palette string
			Changed        bool
		}
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
	expect("the title of the window", steps.Title, "Роль Кладовщик: Права")
	expect("fields of text in the window", steps.FieldsInWindow, 0)
	expect("the flags", steps.Flags, []string{"Устанавливать права для новых объектов",
		"Устанавливать права для реквизитов и табличных частей по умолчанию", "Независимые права подчиненных объектов"})
	expect("the flags at the bottom", steps.FlagsLast, true)
	expect("the title of the palette", steps.PaletteTitle, "Свойства: Кладовщик")
	expect("the rows of the palette", steps.Rows, []string{"Имя", "Синоним", "Комментарий", "Права"})
	expect("«Права: Открыть» available", steps.RightsAvailable, true)
	expect("the templates chosen", steps.Templates, true)
	expect("«Права: Открыть» shows the rights", steps.RightsShown, true)
	expect("renamed", steps.Renamed, struct {
		Title, Palette string
		Changed        bool
	}{"Роль Кладовщица: Права", "Свойства: Кладовщица", true})
}
