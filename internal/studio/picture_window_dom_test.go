package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// pictureWindowWalk makes a common picture by «Добавить» and reads its
// window and palette as the inventory reads them; then it opens Звезда,
// which has an image, reads the picture and its type, marks «Доступность
// для выбора» in the window, clears the picture and saves.
const pictureWindowWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const host = document.querySelector('#picture-editor');
  const settle = () => new Promise(resolve => setTimeout(resolve, 80)).then(() => studio.settle());
  const read = () => ({
    title: host.querySelector('.object-window-title')?.textContent || '',
    captions: [...host.querySelectorAll('.catalog-field > span:first-child, .catalog-check > span')].map(item => item.textContent),
    appearance: windowField('#picture-editor', 'Доступность для оформления').querySelector('input').disabled,
    subsystems: windowField('#picture-editor', 'Подсистемы:').querySelector('button').disabled,
    buttons: [...host.querySelectorAll('.picture-window-buttons button')].map(item => item.textContent + (item.disabled ? ' (недоступна)' : '')),
    image: !!host.querySelector('.picture-window-image img'),
    type: host.querySelector('.picture-window-type span:last-child').textContent,
  });
  await openObject('Общие картинки');
  document.querySelector('#tree-add').click(); await settle();
  steps.created = read();
  steps.tree = !!treeRow('ОбщаяКартинка1');
  steps.paletteTitle = document.querySelector('#inspector-title').textContent;
  steps.tabs = [...document.querySelectorAll('#properties .studio-tab')].map(tab => tab.textContent);
  steps.palette = [...document.querySelectorAll('#properties .object-palette-row .object-palette-caption')].map(item => item.textContent);
  steps.paletteAppearance = windowField('#properties', 'Доступность для оформления').querySelector('input').disabled;
  await openObject('Звезда');
  steps.star = read();
  const drawn = host.querySelector('.picture-window-image img');
  await new Promise(resolve => drawn.complete ? resolve() : drawn.addEventListener('load', resolve, {once: true}));
  steps.drawn = drawn.naturalWidth;
  windowField('#picture-editor', 'Доступность для выбора').querySelector('input').click(); await settle();
  steps.marked = {appearance: read().appearance, palette: windowField('#properties', 'Доступность для выбора').querySelector('input').checked,
    unsaved: !document.querySelector('#save').disabled};
  [...host.querySelectorAll('.picture-window-buttons button')].find(item => item.textContent === 'Очистить').click(); await settle();
  steps.cleared = read();
  document.querySelector('#save').click(); await settle();
  steps.saved = {unsaved: !document.querySelector('#save').disabled, after: read()};
  return steps;
})()`

// The window of a common picture is the prototype's (docs/block-00/
// obshchaya-kartinka.md, «Окно» and «Палитра»; snapshots 01-palitra,
// 02-okno): «Добавить» opens it on ОбщаяКартинка1 - «Общая картинка
// ОбщаяКартинка1», the name, synonym, comment and subsystems, the two flags,
// «Доступность для оформления» unavailable until the picture is available
// for choice; the buttons in the prototype's order but «Редактировать» -
// Studio has no editor of pictures (the owner, 15.10.2026) - of which a
// picture of no image has only «Выбрать из файла»; the palette of one tab beside it.
// A picture with an image shows it and its type; a flag marked in the window
// is the palette's; «Очистить» and «Сохранить» leave no image, on disk too.
// Defects caught: a caption, a button or its order not the prototype's; a
// button of an item to come left available; the image not drawn; the window
// and the palette apart; a cleared image kept on disk.
func TestTheWindowOfAPictureIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		created, err := workspace.CreatePicture("Звезда")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := workspace.SavePictureEditor(created.Path, PictureWindowValue{CommonPictureDefinition: created.Picture,
			Image: &PictureImage{Format: "png", Content: pngImage(t, 24, 24, 250)}}, created.Revision); err != nil {
			t.Fatal(err)
		}
	})
	result, err := browser.evaluate(ctx, page, pictureWindowWalk)
	if err != nil {
		t.Fatal(err)
	}
	type window struct {
		Title, Type                   string
		Captions, Buttons             []string
		Appearance, Subsystems, Image bool
	}
	var steps struct {
		Created, Star, Cleared  window
		Tree, PaletteAppearance bool
		PaletteTitle            string
		Tabs, Palette           []string
		Drawn                   int
		Marked                  struct{ Appearance, Palette, Unsaved bool }
		Saved                   struct {
			Unsaved bool
			After   window
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
	captions := []string{"Имя:", "Синоним:", "Комментарий:", "Подсистемы:", "Доступность для выбора", "Доступность для оформления"}
	unavailable := func(enabled ...string) []string {
		all := []string{"Выбрать из файла", "Очистить", "Установить прозрачный фон", "Убрать прозрачность", "Включить набор вариантов",
			"Выключить набор вариантов", "Сохранить в файл"}
		for index, caption := range all {
			on := false
			for _, item := range enabled {
				on = on || item == caption
			}
			if !on {
				all[index] = caption + " (недоступна)"
			}
		}
		return all
	}
	expect("the new picture", steps.Created, window{Title: "Общая картинка ОбщаяКартинка1", Captions: captions, Buttons: unavailable("Выбрать из файла"),
		Appearance: true, Subsystems: true})
	expect("its node", steps.Tree, true)
	expect("the palette", []any{steps.PaletteTitle, steps.Tabs, steps.Palette, steps.PaletteAppearance},
		[]any{"Свойства: ОбщаяКартинка1", []string{"Основные"}, []string{"Имя", "Синоним", "Комментарий", "Картинка", "Доступность для выбора", "Доступность для оформления"}, true})
	expect("a picture with an image", steps.Star, window{Title: "Общая картинка Звезда", Type: "png", Captions: captions,
		Buttons: unavailable("Выбрать из файла", "Очистить", "Сохранить в файл"), Appearance: true, Subsystems: true, Image: true})
	expect("drawn", steps.Drawn, 24)
	expect("marked for choice", steps.Marked, struct{ Appearance, Palette, Unsaved bool }{false, true, true})
	expect("cleared", steps.Cleared, window{Title: "Общая картинка Звезда", Captions: captions, Buttons: unavailable("Выбрать из файла"), Subsystems: true})
	expect("saved", steps.Saved.Unsaved, false)
	expect("saved window", steps.Saved.After.Image, false)
	if _, err := os.Stat(filepath.Join(root, "metadata", "common-pictures", "Звезда", "100.png")); !os.IsNotExist(err) {
		t.Errorf("the cleared image is still on disk: %v", err)
	}
}
