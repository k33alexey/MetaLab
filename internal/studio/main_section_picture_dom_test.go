package studio

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

// mainSectionPictureWalk opens «Картинка основного раздела: Открыть» of the
// palette of the root and reads «Выбор картинки»; closes it by «Отмена»;
// chooses a file in it, presses OK and «Сохранить»; opens it again, clears
// the picture, presses OK and «Сохранить».
const mainSectionPictureWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;
  window.chooseFileContent = async () => ({name: 'раздел.png', content: window.__image});
  const dialog = () => document.querySelector('dialog[open]');
  const palette = () => document.querySelector('#properties');
  const button = caption => [...dialog().querySelectorAll('button')].find(item => item.textContent === caption);
  const open = async () => {
    document.querySelector('#tree summary').click(); await studio.settle();
    palette().querySelector('.studio-tab[data-tab="presentation"]').click(); await studio.settle();
    [...palette().querySelectorAll('.object-palette-row')].find(item => item.firstChild.textContent === 'Картинка основного раздела').querySelector('button').click();
    await studio.settle();
  };
  const read = () => ({title: dialog().querySelector('h3').textContent,
    buttons: [...dialog().querySelectorAll('button')].map(item => item.textContent + (item.disabled ? ' -' : '')),
    image: dialog().querySelector('.picture-window-image img')?.getAttribute('src') || '',
    type: dialog().querySelector('.picture-window-type span:last-child').textContent});
  const unsaved = () => !document.querySelector('#save').disabled;
  await open();
  steps.empty = read();
  button('Отмена').click(); await studio.settle();
  steps.cancelled = unsaved();
  await open();
  button('Выбрать из файла').click(); await studio.settle();
  steps.chosen = read();
  button('OK').click(); await studio.settle();
  steps.changed = unsaved();
  steps.beforeSave = (await (await fetch('/api/project-configuration')).json()).picture.images;
  document.querySelector('#save').click(); await studio.settle();
  steps.saved = !unsaved();
  await open();
  steps.reopened = read();
  button('Очистить').click(); await studio.settle();
  steps.cleared = read();
  button('OK').click(); await studio.settle();
  document.querySelector('#save').click(); await studio.settle();
  steps.clearedSaved = !unsaved();
  return steps;
})()`

// «Картинка основного раздела: Открыть» of the palette of the root is the
// prototype's «Выбор картинки» (konfiguraciya.md, «Досъёмка 30.10.2026»;
// snapshots dosyomka-2651/07, 08): the picture with «Тип картинки» under it,
// the buttons of the window of a common picture but «Сохранить в файл» in a
// column with OK and «Отмена» under them; the transparency unavailable, an
// item of its own. What OK gives is the root's until «Сохранить», which
// writes the folder of the picture; «Отмена» changes nothing. Defects
// caught: the row unavailable or opening nothing; a button missing, extra
// or out of the prototype's order; a file chosen not drawn or not typed;
// OK not marking the root changed or the picture written before
// «Сохранить»; «Отмена» changing the root; a picture saved and not read
// back; a cleared picture leaving its folder.
func TestTheMainSectionPictureIsChosenAsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	image := pngImage(t, 16, 16, 90)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	if _, err := browser.evaluate(ctx, page, `window.__image = '`+base64.StdEncoding.EncodeToString(image)+`'`); err != nil {
		t.Fatal(err)
	}
	result, err := browser.evaluate(ctx, page, mainSectionPictureWalk)
	if err != nil {
		t.Fatal(err)
	}
	type window struct {
		Title, Image, Type string
		Buttons            []string
	}
	var steps struct {
		Empty, Chosen, Reopened, Cleared        window
		Cancelled, Changed, Saved, ClearedSaved bool
		BeforeSave                              []string
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	want := []string{"Выбрать из файла", "Очистить -", "Установить прозрачный фон -", "Убрать прозрачность -", "Включить набор вариантов",
		"Выключить набор вариантов -", "Редактировать -", "OK", "Отмена"}
	if steps.Empty.Title != "Выбор картинки" || !slices.Equal(steps.Empty.Buttons, want) || steps.Empty.Image != "" || steps.Empty.Type != "" {
		t.Errorf("«Выбор картинки» of no picture is %+v, want the buttons %q", steps.Empty, want)
	}
	if steps.Cancelled {
		t.Error("«Отмена» marked the root changed")
	}
	want[1] = "Очистить"
	for name, shown := range map[string]window{"chosen": steps.Chosen, "reopened": steps.Reopened} {
		if !slices.Equal(shown.Buttons, want) || shown.Image == "" || shown.Type != "png" {
			t.Errorf("the picture %s shows %+v, want an image of png and the buttons %q", name, shown, want)
		}
	}
	if len(steps.BeforeSave) != 0 {
		t.Errorf("OK wrote the picture before «Сохранить»: %q", steps.BeforeSave)
	}
	if !strings.HasPrefix(steps.Reopened.Image, "/api/project/picture/image?name=100.png") {
		t.Errorf("the saved picture is drawn from %q, want its image 100.png", steps.Reopened.Image)
	}
	if !steps.Changed || !steps.Saved {
		t.Errorf("OK marked the root changed %v, «Сохранить» saved it %v", steps.Changed, steps.Saved)
	}
	if steps.Cleared.Image != "" || !steps.ClearedSaved {
		t.Errorf("«Очистить» shows %+v, saved %v", steps.Cleared, steps.ClearedSaved)
	}
	if _, err := os.Stat(filepath.Join(root, project.MainSectionPictureDirectory)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the cleared picture left its folder: %v", err)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
