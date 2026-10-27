package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pictureChoiceWalk opens the subsystem Продажи, shown with the hidden
// picture УстановитьВремя, and chooses its picture in «Выбор картинки»:
// the common picture Звезда by OK and save; АктивироватьЗадачу and
// «Отмена»; «Очистить» and OK, and save.
const pictureChoiceWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 150)).then(() => studio.settle());
  const field = () => windowField('#subsystem-editor', 'Картинка:');
  const dialog = () => document.querySelector('dialog.picture-choice-dialog');
  const open = async () => { field().querySelector('.field-choice').click(); await settle(); };
  const tab = text => [...dialog().querySelectorAll('.picture-choice-left .studio-tab')].find(item => item.textContent === text);
  const row = text => [...dialog().querySelectorAll('.object-window-list-row')].find(item => item.textContent === text);
  const button = text => [...dialog().querySelectorAll('button')].find(item => item.textContent === text);
  const read = () => {
    const rows = [...dialog().querySelectorAll('.object-window-list-row')];
    const shown = dialog().querySelector('.picture-choice-image');
    return {
      tab: dialog().querySelector('.picture-choice-left .studio-tab[aria-selected=true]')?.textContent || '',
      count: rows.length,
      first: rows.slice(0, 3).map(item => item.textContent + (item.querySelector('img') ? ' [картинка]' : '')),
      marked: rows.filter(item => item.classList.contains('current')).map(item => item.textContent),
      image: shown.querySelector('img') ? 'картинка' : shown.querySelector('svg use')?.getAttribute('href') || '',
      size: shown.querySelector('.picture-choice-size')?.textContent || '',
      clear: !button('Очистить').disabled,
    };
  };
  await openObject('Продажи');
  steps.field = field().querySelector('input').value;
  await open();
  steps.head = {title: dialog().querySelector('h3').textContent,
    tabs: [...dialog().querySelectorAll('.studio-tab')].map(item => item.textContent),
    buttons: [...dialog().querySelectorAll('button')].filter(item => !item.classList.contains('studio-tab'))
      .map(item => (item.textContent || item.title) + (item.disabled ? ' (недоступна)' : ''))};
  steps.hidden = {...read(), setTime: !!row('УстановитьВремя'), activate: !!row('АктивироватьЗадачу')};
  tab('Из проекта').click(); await settle();
  steps.common = read();
  row('Звезда').click(); await settle();
  steps.star = read();
  button('OK').click(); await settle();
  steps.chosen = field().querySelector('input').value;
  document.querySelector('#save').click(); await settle();
  const saved = async () => (await (await fetch('/api/subsystem?path=' + encodeURIComponent('metadata/subsystems/` + treeSalesSubsystem + `.yaml'))).json()).subsystem.picture || null;
  steps.saved = await saved();
  await open();
  steps.reopened = read();
  tab('Стандартные').click(); await settle();
  row('АктивироватьЗадачу').click(); await settle();
  steps.activate = read();
  button('Отмена').click(); await settle();
  steps.cancelled = field().querySelector('input').value;
  await open();
  button('Очистить').click(); await settle();
  steps.cleared = read();
  button('OK').click(); await settle();
  steps.emptied = field().querySelector('input').value;
  document.querySelector('#save').click(); await settle();
  steps.savedEmpty = await saved();
  return steps;
})()`

// «Выбор картинки» is the prototype's (docs/block-00/standartnye-kartinki.md,
// «Окно»; gruppa-komand.md, «Палитра»; snapshots standartnye-kartinki/19,
// 20): the tab «Из библиотеки» with «Из проекта» - the common pictures
// in the order of the tree, with their images - and «Стандартные» - the
// platform's by their Russian names, the hidden ones not offered; the
// picture set at the right with its size, «16 x 16» for a standard one, a
// hidden one too; no row marked on opening; the tab of the picture set
// first; «Очистить» available while a picture is chosen; OK writes the
// reference, «Отмена» nothing; the settings of loading stay with the
// reference, and with no picture as the prototype keeps them.
// Expectations are written from the inventory, not from icons.StandardPictures.
// Defects caught: a hidden picture offered, or not shown when set; a row
// marked on opening; the wrong tab first; the size not shown; the reference
// losing its settings, or written after «Отмена»; a common picture written
// by name instead of identifier; the field showing the English name.
func TestThePictureOfASubsystemIsChosenByThePrototypesWindow(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addTreeSubsystems(t, root)
	description := filepath.Join(root, "metadata", "subsystems", treeSalesSubsystem+".yaml")
	body, err := os.ReadFile(description)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(description, append(body, "picture:\n  standard: SetTime\n  load_transparent: true\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	var star string
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		for _, name := range []string{"Звезда", "Луна"} {
			created, err := workspace.CreatePicture(name)
			if err != nil {
				t.Fatal(err)
			}
			if name != "Звезда" {
				continue
			}
			star = created.Picture.ID.String()
			if _, err := workspace.SavePictureEditor(created.Path, PictureWindowValue{CommonPictureDefinition: created.Picture,
				Image: &PictureImage{Format: "png", Content: pngImage(t, 24, 20, 250)}}, created.Revision); err != nil {
				t.Fatal(err)
			}
		}
	})
	written := func() string {
		content, _ := os.ReadFile(description)
		text := string(content)
		if at := strings.Index(text, "picture:"); at >= 0 {
			return text[at:]
		}
		return ""
	}
	before := written()
	result, err := browser.evaluate(ctx, page, pictureChoiceWalk)
	if err != nil {
		t.Fatal(err)
	}
	type view struct {
		Tab               string
		Count             int
		First, Marked     []string
		Image, Size       string
		Clear             bool
		SetTime, Activate bool
	}
	var steps struct {
		Field, Chosen, Cancelled, Emptied string
		Head                              struct {
			Title         string
			Tabs, Buttons []string
		}
		Hidden, Common, Star, Reopened, Activate view
		Cleared                                  view
		Saved, SavedEmpty                        map[string]any
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
	expect("the field of a hidden picture", steps.Field, "УстановитьВремя")
	expect("the window", steps.Head, struct {
		Title         string
		Tabs, Buttons []string
	}{"Выбор картинки",
		[]string{"Из библиотеки", "Из проекта", "Стандартные"},
		[]string{"Добавить (недоступна)", "Изменить (недоступна)", "Удалить (недоступна)", "Переместить вверх (недоступна)", "Переместить вниз (недоступна)",
			"Очистить", "Сохранить (недоступна)", "OK", "Отмена"}})
	// 299 pictures in the help, 39 hidden from the window (inventory).
	expect("a hidden picture set", steps.Hidden, view{Tab: "Стандартные", Count: 260,
		First: []string{"АктивироватьЗадачу", "АктивныеПользователи", "БизнесПроцесс"}, Marked: []string{}, Image: "#icon-set-time", Size: "16 x 16",
		Clear: true, SetTime: false, Activate: true})
	expect("«Из проекта»", steps.Common, view{Tab: "Из проекта", Count: 2, First: []string{"Звезда [картинка]", "Луна"}, Marked: []string{},
		Image: "#icon-set-time", Size: "16 x 16", Clear: true})
	expect("Звезда chosen", steps.Star, view{Tab: "Из проекта", Count: 2, First: []string{"Звезда [картинка]", "Луна"}, Marked: []string{"Звезда"},
		Image: "картинка", Size: "24 x 20", Clear: true})
	expect("the field after OK", steps.Chosen, "Звезда")
	expect("reopened", steps.Reopened, view{Tab: "Из проекта", Count: 2, First: []string{"Звезда [картинка]", "Луна"}, Marked: []string{},
		Image: "картинка", Size: "24 x 20", Clear: true})
	expect("АктивироватьЗадачу chosen", steps.Activate, view{Tab: "Стандартные", Count: 260,
		First: []string{"АктивироватьЗадачу", "АктивныеПользователи", "БизнесПроцесс"}, Marked: []string{"АктивироватьЗадачу"}, Image: "#icon-activate-task",
		Size: "16 x 16", Clear: true})
	expect("the field after «Отмена»", steps.Cancelled, "Звезда")
	expect("cleared", steps.Cleared, view{Tab: "Из проекта", Count: 2, First: []string{"Звезда [картинка]", "Луна"}, Marked: []string{}})
	expect("the field after «Очистить»", steps.Emptied, "")
	if !strings.HasPrefix(before, "picture:\n  standard: SetTime\n  load_transparent: true\n") {
		t.Fatalf("the subsystem was given %q", before)
	}
	expect("saved with Звезда", steps.Saved, map[string]any{"common": star, "loadTransparent": true})
	expect("saved cleared", steps.SavedEmpty, map[string]any{"loadTransparent": true})
	if text := written(); strings.Contains(text, "standard:") || strings.Contains(text, "common:") {
		t.Errorf("the description still names a picture: %q", text)
	}
}
