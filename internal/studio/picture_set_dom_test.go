package studio

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
)

// pictureSetWalk opens Звезда, a picture of one image, turns its set of
// variants on, opens «Редактировать» and reads the editor; adds a row,
// gives it a density and, by «…», a file (chooseFileContent answers
// LARGE); adds a row of the same key and a row of no picture, presses OK
// and reads the refusal, gives the second row a direction, marks «Режим
// коллекции», presses OK and saves; then it turns the set off and saves.
const pictureSetWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const host = document.querySelector('#picture-editor');
  const settle = () => new Promise(resolve => setTimeout(resolve, 80)).then(() => studio.settle());
  const enabled = () => [...host.querySelectorAll('.picture-window-buttons button')].filter(item => !item.disabled).map(item => item.textContent);
  const press = caption => [...host.querySelectorAll('.picture-window-buttons button')].find(item => item.textContent === caption).click();
  const type = () => host.querySelector('.picture-window-type span:last-child').textContent;
  const dialog = className => document.querySelector('dialog.' + className);
  const answer = (className, caption) => [...dialog(className).querySelectorAll('.localized-title-actions button')].find(item => item.textContent === caption).click();
  const choose = (select, value) => { select.value = value; select.dispatchEvent(new Event('change', {bubbles: true})); };
  const table = () => {
    const set = dialog('picture-set-dialog');
    return {
      heads: [...set.querySelectorAll('thead th')].map(item => item.textContent),
      rows: [...set.querySelectorAll('tbody tr')].map(tr => [...tr.querySelectorAll('select')].map(select => select.selectedOptions[0]?.textContent || '')
        .concat([tr.querySelector('input[readonly]').value])),
    };
  };
  window.chooseFileContent = async () => ({name: 'большая.png', content: 'LARGE'});
  await openObject('Звезда');
  steps.single = {enabled: enabled(), type: type()};
  press('Включить набор вариантов'); await settle();
  steps.on = {enabled: enabled(), type: type(), unsaved: !document.querySelector('#save').disabled};
  press('Редактировать'); await settle();
  const set = dialog('picture-set-dialog');
  steps.editor = {title: set.querySelector('h3').textContent, ...table(),
    panel: [...set.querySelectorAll('.object-window-panel button')].map(item => item.title),
    collection: set.querySelector('.catalog-check input').checked};
  set.querySelector('.object-window-panel button[title="Добавить"]').click(); await settle();
  steps.added = table().rows[1];
  choose(set.querySelectorAll('tbody tr')[1].querySelector('select'), '200');
  set.querySelectorAll('tbody tr')[1].querySelector('button[title="Выбрать картинку"]').click(); await settle();
  const variant = dialog('picture-variant-dialog');
  steps.variant = {title: variant.querySelector('h3').textContent, empty: variant.querySelector('.picture-variant-image').classList.contains('empty'),
    buttons: [...variant.querySelectorAll('button')].map(item => item.textContent + (item.disabled ? ' (недоступна)' : ''))};
  [...variant.querySelectorAll('button')].find(item => item.textContent === 'Выбрать из файла').click(); await settle();
  await new Promise(resolve => setTimeout(resolve, 200));
  steps.variantChosen = variant.querySelector('.picture-variant-size').textContent;
  answer('picture-variant-dialog', 'OK'); await settle();
  steps.chosen = table().rows[1];
  set.querySelector('.object-window-panel button[title="Добавить"]').click(); await settle();
  choose(set.querySelectorAll('tbody tr')[2].querySelector('select'), '100');
  set.querySelectorAll('tbody tr')[2].querySelector('button[title="Выбрать картинку"]').click(); await settle();
  [...dialog('picture-variant-dialog').querySelectorAll('button')].find(item => item.textContent === 'Выбрать из файла').click(); await settle();
  answer('picture-variant-dialog', 'OK'); await settle();
  set.querySelector('.object-window-panel button[title="Добавить"]').click(); await settle();
  answer('picture-set-dialog', 'OK'); await settle();
  steps.refused = set.querySelector('.dialog-problem').hidden ? '' : set.querySelector('.dialog-problem').textContent;
  choose(set.querySelectorAll('tbody tr')[2].querySelectorAll('select')[1], 'rtl');
  set.querySelector('.catalog-check input').click(); await settle();
  steps.collection = table().heads;
  answer('picture-set-dialog', 'OK'); await settle();
  steps.edited = {open: !!dialog('picture-set-dialog'), type: type()};
  document.querySelector('#save').click(); await settle();
  steps.saved = {unsaved: !document.querySelector('#save').disabled, enabled: enabled()};
  const written = await (await studioRequest('/api/picture?path=' + encodeURIComponent('metadata/common-pictures/Звезда/object.yaml'), {method: 'GET'})).json();
  steps.written = {variants: written.picture.variants, images: written.images};
  press('Выключить набор вариантов'); await settle();
  steps.off = {enabled: enabled(), type: type()};
  document.querySelector('#save').click(); await settle();
  steps.offSaved = !document.querySelector('#save').disabled;
  return steps;
})()`

// The set of variants of a picture is the prototype's (docs/block-00/
// obshchaya-kartinka.md, «Окно», «Досъёмка 28.10.2026»; ML-STUDIO.md,
// «Набор вариантов»): «Включить набор вариантов» makes the image the one
// variant «Малая (ldpi)» with no window, the type zip, «Выключить» and
// «Редактировать» available, «Включить» and «Сохранить в файл» not; the
// editor is titled with the name of the picture, has «Добавить»,
// «Скопировать», «Изменить», «Удалить» and the columns «Плотность экрана —
// Направление — Веб-браузер (для веб-клиента) — Картинка» with no
// «Шаблонная» and «Интерфейс»; «Добавить» is a row of no density and no
// picture; «…» is the window of the variant titled with its density, a
// crossed square, «Сохранить в файл» unavailable until there is an image,
// whose size it shows; two rows of one key are refused with the
// prototype's words; a row of no picture is dropped; «Режим коллекции»
// adds the cells. «Сохранить» writes the set; «Выключить» leaves the image
// of the variant nearest to ldpi. Defects caught: a button of a set left
// in the state of a single picture, a column of the prototype ML has not,
// the density of a new row filled in, the refusal missing or not the
// prototype's, a row of no picture written, the set not written or turned
// off to the wrong variant.
func TestTheSetOfVariantsOfAPictureIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	small, large := pngImage(t, 16, 16, 250), pngImage(t, 32, 32, 20)
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		created, err := workspace.CreatePicture("Звезда")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := workspace.SavePictureEditor(created.Path, PictureWindowValue{CommonPictureDefinition: created.Picture,
			Image: &PictureImage{Format: "png", Content: small}}, created.Revision); err != nil {
			t.Fatal(err)
		}
	})
	result, err := browser.evaluate(ctx, page, strings.Replace(pictureSetWalk, "LARGE", base64.StdEncoding.EncodeToString(large), 1))
	if err != nil {
		t.Fatal(err)
	}
	type state struct {
		Enabled []string
		Type    string
	}
	var steps struct {
		Single, Off state
		On          struct {
			Enabled []string
			Type    string
			Unsaved bool
		}
		Editor struct {
			Title        string
			Panel, Heads []string
			Rows         [][]string
			Collection   bool
		}
		Written struct {
			Variants []metadata.PictureVariant
			Images   []string
		}
		Added, Chosen []string
		Variant       struct {
			Title   string
			Empty   bool
			Buttons []string
		}
		VariantChosen, Refused string
		Collection             []string
		Edited                 struct {
			Open bool
			Type string
		}
		Saved struct {
			Unsaved bool
			Enabled []string
		}
		OffSaved bool
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
	expect("a picture of one image", steps.Single, state{[]string{"Выбрать из файла", "Очистить", "Включить набор вариантов", "Сохранить в файл"}, "png"})
	expect("the set turned on", steps.On, struct {
		Enabled []string
		Type    string
		Unsaved bool
	}{[]string{"Выбрать из файла", "Очистить", "Выключить набор вариантов", "Редактировать"}, "zip", true})
	expect("the editor", steps.Editor, struct {
		Title        string
		Panel, Heads []string
		Rows         [][]string
		Collection   bool
	}{"Звезда", []string{"Добавить", "Скопировать", "Изменить", "Удалить"},
		[]string{"Плотность экрана", "Направление", "Веб-браузер (для веб-клиента)", "Картинка"},
		[][]string{{"Малая (ldpi)", "Любое", "Любой", "Картинка: png"}}, false})
	expect("a row added", steps.Added, []string{"", "Любое", "Любой", ""})
	expect("the window of a variant", steps.Variant, struct {
		Title   string
		Empty   bool
		Buttons []string
	}{"Большая (hdpi)", true, []string{"Выбрать из файла", "Очистить", "Сохранить в файл (недоступна)", "OK", "Отмена"}})
	expect("its size", steps.VariantChosen, "32 x 32")
	expect("the row chosen", steps.Chosen, []string{"Большая (hdpi)", "Любое", "Любой", "Картинка: png"})
	expect("two rows of one key", steps.Refused, `Картинка содержит не уникальные варианты "Малая (ldpi)", "Любое", "Любой".`)
	expect("the mode of a collection", steps.Collection, []string{"Плотность экрана", "Направление", "Веб-браузер (для веб-клиента)", "Картинка", "Ширина ячейки", "Высота ячейки"})
	expect("the editor closed", steps.Edited, struct {
		Open bool
		Type string
	}{false, "zip"})
	expect("saved", steps.Saved, struct {
		Unsaved bool
		Enabled []string
	}{false, []string{"Выбрать из файла", "Очистить", "Выключить набор вариантов", "Редактировать"}})

	expect("the set written", steps.Written, struct {
		Variants []metadata.PictureVariant
		Images   []string
	}{[]metadata.PictureVariant{{File: "100.png", Density: 100}, {File: "200.png", Density: 200}, {File: "100-rtl.png", Density: 100, Direction: "rtl"}},
		[]string{"100-rtl.png", "100.png", "200.png"}})
	folder := filepath.Join(root, "metadata", "common-pictures", "Звезда")
	expect("turned off", steps.Off, state{[]string{"Выбрать из файла", "Очистить", "Включить набор вариантов", "Сохранить в файл"}, "png"})
	expect("turned off and saved", steps.OffSaved, false)
	entries, err := os.ReadDir(folder)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	expect("the folder turned off", names, []string{"100.png", "object.yaml"})
	if content, _ := os.ReadFile(filepath.Join(folder, "100.png")); string(content) != string(small) {
		t.Error("turned off: the image is not the ldpi variant's")
	}
	if _, err := metadata.Load(root); err != nil {
		t.Fatalf("the project does not load: %v", err)
	}
}
