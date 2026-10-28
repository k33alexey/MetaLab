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

// xdtoSchemaWalk opens the editor of Обмен, adds a value type and, not
// saving, presses «Экспорт XML-схемы...» of its panel, reading the file the
// browser is given and the window after; then it presses «Экспорт
// XML-схемы…» of the menu of Сломанный in the tree and reads the window.
const xdtoSchemaWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 120)).then(() => studio.settle());
  const host = document.querySelector('#xdto-package-editor');
  const saved = [];
  HTMLAnchorElement.prototype.click = function () { saved.push({name: this.download, href: this.href}); };
  const dialog = () => { const shown = [...document.querySelectorAll('dialog')].pop(); return shown ? shown.querySelector('h3').textContent : ''; };
  const close = async () => { [...document.querySelectorAll('dialog')].pop().querySelector('button').click(); await settle(); };
  await openObject('Обмен');
  [...host.querySelectorAll('.object-window-panel button')].find(button => button.textContent.startsWith('Добавить')).click(); await settle();
  [...document.querySelectorAll('.context-menu-item')].find(item => item.textContent === 'Тип значения').click(); await settle();
  const exporting = host.querySelector('.object-window-panel button[title="Экспорт XML-схемы..."]');
  steps.available = !exporting.disabled;
  exporting.click(); await settle();
  steps.success = dialog();
  await close();
  steps.file = saved.map(item => ({name: item.name, schema: item.href.slice(item.href.indexOf(',') + 1)}));
  steps.unsaved = !document.querySelector('#save').disabled;
  const broken = [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.querySelector(':scope > span:not(.studio-tree-sign)')?.textContent === 'Сломанный');
  broken.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent === 'Экспорт XML-схемы…').click(); await settle();
  steps.refused = dialog();
  await close();
  steps.files = saved.length;
  return steps;
})()`

// «Экспорт XML-схемы…» is the prototype's (paket-xdto.md, «Проверить пакет,
// экспорт схемы, окно типа»; snapshots series-2.603/26, 27): the editor's
// button writes the schema of the body it shows, saved or not, as a file
// named as the prototype's dialog names it, and says «Экспорт схемы XML
// успешно завершен»; the menu of a package in the tree writes the body on
// disk and, for a package the check refuses, says «При экспорте схемы XML
// были обнаружены ошибки. Экспорт невозможен» and writes nothing. Defects
// caught: the button left unavailable, the saved body exported in place of
// the one shown, a refused package written, the prototype's words changed.
func TestTheSchemaOfAPackageIsExportedAsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	writeXDTOPackageForStudioTest(t, root)
	folder := filepath.Join(root, "metadata", "xdto-packages", "Сломанный")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	description := "format: 1\nid: f2000000-0000-4000-8000-000000000002\nname: Сломанный\ntitle: {ru: Сломанный}\nnamespace: http://example.org/broken\n"
	if err := os.WriteFile(filepath.Join(folder, "object.yaml"), []byte(description), 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := os.Create(filepath.Join(folder, metadata.XDTOPackageContentFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := metadata.EncodeXDTOContent(body, metadata.XDTOContent{Namespace: "http://example.org/broken",
		Properties: []metadata.XDTOProperty{{Name: "Свойство1", Type: &metadata.XDTOName{Namespace: "http://example.org/goods", Name: "Товар"}}}}); err != nil {
		t.Fatal(err)
	}
	body.Close()
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, xdtoSchemaWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Available, Unsaved bool
		Success, Refused   string
		File               []struct{ Name, Schema string }
		Files              int
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	if !steps.Available || steps.Success != "Экспорт схемы XML успешно завершен" || !steps.Unsaved {
		t.Errorf("the editor's export: available %v, %q, unsaved %v", steps.Available, steps.Success, steps.Unsaved)
	}
	if len(steps.File) != 1 || steps.File[0].Name != "Без названия.xsd" {
		t.Fatalf("the file given: %+v", steps.File)
	}
	schema, err := base64.StdEncoding.DecodeString(steps.File[0].Schema)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(schema), "\uFEFF<xs:schema ") || !strings.Contains(string(schema), "\r\n\t<xs:simpleType name=\"ТипЗначения1\">\r\n") {
		t.Errorf("the schema of the body shown:\n%s", schema)
	}
	if steps.Refused != "При экспорте схемы XML были обнаружены ошибки. Экспорт невозможен" || steps.Files != 1 {
		t.Errorf("a package the check refuses: %q, files %d", steps.Refused, steps.Files)
	}
}
