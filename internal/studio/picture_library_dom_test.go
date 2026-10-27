package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// pictureLibraryWalk opens «Все картинки» from the menu of the picture
// Луна and reads «Библиотека картинок проекта»; it saves Звезда to a
// file, clears it, deletes Луна, adds a picture, and opens the window of
// the current one by «Изменить».
const pictureLibraryWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 120)).then(() => studio.settle());
  const library = () => document.querySelector('dialog.picture-library');
  const button = text => [...library().querySelectorAll('button')].find(item => item.textContent === text || item.title === text);
  const read = () => ({
    title: library().querySelector('.picture-library-title').textContent,
    rows: [...library().querySelectorAll('.picture-library-row')].map(item => item.textContent + (item.classList.contains('current') ? ' *' : '')),
    image: !!library().querySelector('.picture-library-image img'),
    buttons: [...library().querySelectorAll('button')].map(item => (item.textContent || item.title) + (item.disabled ? ' (недоступна)' : '')),
  });
  const row = text => [...library().querySelectorAll('.picture-library-row')].find(item => item.textContent === text);
  treeRow('Луна').dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  steps.menu = [...document.querySelector('.context-menu').children].slice(0, 2).map(entry => entry.textContent.trim());
  [...document.querySelectorAll('.context-menu-item')].find(item => item.textContent.trim() === 'Все картинки').click();
  await settle();
  steps.opened = read();
  steps.modal = library().matches(':modal');
  row('Звезда').click(); await settle();
  steps.star = read();
  let saved = null;
  const click = HTMLAnchorElement.prototype.click;
  HTMLAnchorElement.prototype.click = function () { saved = {name: this.download, href: this.getAttribute('href')}; };
  button('Сохранить в файл').click();
  HTMLAnchorElement.prototype.click = click;
  steps.saved = saved && saved.name;
  button('Очистить').click(); await settle();
  steps.cleared = read();
  row('Луна').click(); await settle();
  button('Удалить').click(); await settle();
  steps.deleted = {library: read(), tree: !!treeRow('Луна')};
  button('Добавить').click(); await settle();
  steps.added = {library: read().rows, window: document.querySelector('#picture-editor .object-window-title')?.textContent};
  row('Звезда').click(); await settle();
  button('Изменить').click(); await settle();
  steps.changed = {window: document.querySelector('#picture-editor .object-window-title')?.textContent, open: !!library()};
  button('Закрыть').click(); await settle();
  steps.closed = !library();
  return steps;
})()`

// «Библиотека картинок проекта» - the prototype's «Библиотека картинок
// конфигурации», named as the product names it (ML-STUDIO.md, «„Проект“
// вместо „конфигурации“») - is the prototype's (docs/block-00/
// obshchaya-kartinka.md, «Команды», «Досъёмка 27.10.2026»; snapshot
// series-2.584/099): «Все картинки» stands first in the menu of a picture;
// the window stands over Studio without shutting it, lists the pictures in
// the order of the tree with the first current - not the one it was called
// on - and shows the image of the current; «Сохранить в файл» names the
// file after the picture; «Очистить» takes the image away at once, on disk
// too; «Удалить» deletes the picture at once and makes the row before
// current; «Добавить» makes the next picture and opens its window,
// «Изменить» opens the window of the current, the library staying open;
// the arrows stand unavailable until the tree moves objects.
// Defects caught: the library opened on the picture called from, or as a
// modal window shutting Studio; a picture of the tree missing or out of
// order; «Очистить» or «Удалить» asking or not reaching the disk; the
// window of a picture not opened by «Добавить» or «Изменить».
func TestThePictureLibraryIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		for _, name := range []string{"Звезда", "Луна"} {
			created, err := workspace.CreatePicture(name)
			if err != nil {
				t.Fatal(err)
			}
			if name != "Звезда" {
				continue
			}
			if _, err := workspace.SavePictureEditor(created.Path, PictureWindowValue{CommonPictureDefinition: created.Picture,
				Image: &PictureImage{Format: "png", Content: pngImage(t, 24, 24, 250)}}, created.Revision); err != nil {
				t.Fatal(err)
			}
		}
	})
	result, err := browser.evaluate(ctx, page, pictureLibraryWalk)
	if err != nil {
		t.Fatal(err)
	}
	type window struct {
		Title         string
		Rows, Buttons []string
		Image         bool
	}
	var steps struct {
		Menu                  []string
		Opened, Star, Cleared window
		Modal, Closed         bool
		Saved                 string
		Deleted               struct {
			Library window
			Tree    bool
		}
		Added struct {
			Library []string
			Window  string
		}
		Changed struct {
			Window string
			Open   bool
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
	buttons := func(image bool) []string {
		off := map[bool]string{true: "", false: " (недоступна)"}[image]
		return []string{"Переместить вверх (недоступна)", "Переместить вниз (недоступна)", "Очистить" + off, "Сохранить в файл" + off, "Закрыть",
			"Добавить", "Изменить", "Удалить"}
	}
	// The common part of the menu is an item of its own («контекстное меню
	// дерева»): «Все картинки» stands alone first.
	expect("the menu of a picture", steps.Menu, []string{"Все картинки"})
	expect("opened", steps.Opened, window{Title: "Библиотека картинок проекта", Rows: []string{"Звезда *", "Луна"}, Image: true, Buttons: buttons(true)})
	expect("not modal", steps.Modal, false)
	expect("Звезда", steps.Star, window{Title: "Библиотека картинок проекта", Rows: []string{"Звезда *", "Луна"}, Image: true, Buttons: buttons(true)})
	expect("saved to a file", steps.Saved, "Звезда.png")
	expect("cleared", steps.Cleared, window{Title: "Библиотека картинок проекта", Rows: []string{"Звезда *", "Луна"}, Buttons: buttons(false)})
	if _, err := os.Stat(filepath.Join(root, "metadata", "common-pictures", "Звезда", "100.png")); !os.IsNotExist(err) {
		t.Errorf("the cleared image is still on disk: %v", err)
	}
	expect("deleted", steps.Deleted.Library.Rows, []string{"Звезда *"})
	expect("deleted from the tree", steps.Deleted.Tree, false)
	if _, err := os.Stat(filepath.Join(root, "metadata", "common-pictures", "Луна")); !os.IsNotExist(err) {
		t.Errorf("the deleted picture is still on disk: %v", err)
	}
	expect("added", steps.Added, struct {
		Library []string
		Window  string
	}{[]string{"Звезда *", "ОбщаяКартинка1"}, "Общая картинка ОбщаяКартинка1"})
	expect("changed", steps.Changed, struct {
		Window string
		Open   bool
	}{"Общая картинка Звезда", true})
	expect("closed", steps.Closed, true)
}
