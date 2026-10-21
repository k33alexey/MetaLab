package studio

import (
	"encoding/json"
	"testing"
)

// editorFieldsWalk opens the window of the root, of a catalog and of a
// language and answers, for each, the left and the right edge of every field
// of its rows - an input, a list to choose from, a text, and a field with
// buttons inside its frame measured by its frame - with the caption of the
// row it stands in.
const editorFieldsWalk = `(async () => {
  const studio = window.__studio, result = {};
  const node = text => [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === text);
  const open = async text => { node(text).click(); await studio.settle(); };
  const edges = host => [...document.querySelectorAll(host + ' .catalog-field')].flatMap(label => {
    const fields = [...label.querySelectorAll('.field-with-buttons, input:not([type=checkbox]):not([type=radio]), select, textarea')]
      .filter(field => !field.closest('.field-with-buttons') || field.classList.contains('field-with-buttons'));
    return fields.filter(field => field.checkVisibility()).map(field => {
      const place = field.getBoundingClientRect();
      return {caption: label.firstChild.textContent.trim(), left: Math.round(place.left), right: Math.round(place.right)};
    });
  });
  document.querySelector('#tree summary').click(); await studio.settle();
  result.root = edges('#project-editor');
  await open('Товары');
  result.catalog = edges('#catalog-editor');
  await open('Українська');
  result.language = edges('#languages-editor');
  return result;
})()`

// The fields of one window stand under one ruler, as the prototype's do:
// each starts where the others start and ends where they end, whatever it is
// - a string, a number, a list to choose from, a text over several lines, a
// field with the magnifier inside it. Defect caught: a list to choose from of
// the window of the root wider than every other field of it (420 pixels
// against 280 - a rule of 2.34 that outlived the width of 2.386), which the
// owner saw on «Язык по умолчанию» on 21.10.2026.
func TestTheFieldsOfAWindowStandUnderOneRuler(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	answer, err := browser.evaluate(ctx, page, editorFieldsWalk)
	if err != nil {
		t.Fatal(err)
	}
	type edge struct {
		Caption     string
		Left, Right int
	}
	var windows map[string][]edge
	if err := json.Unmarshal(answer, &windows); err != nil {
		t.Fatalf("the walk answered %s: %v", answer, err)
	}
	for _, window := range []string{"root", "catalog", "language"} {
		fields := windows[window]
		// The root has a list to choose from and a text over several lines,
		// so it is not held by plain inputs alone.
		if len(fields) < 3 {
			t.Fatalf("the window of the %s shows %d fields: %+v", window, len(fields), fields)
		}
		for _, field := range fields[1:] {
			if field.Left != fields[0].Left || field.Right != fields[0].Right {
				t.Errorf("in the window of the %s «%s» stands from %d to %d, «%s» from %d to %d",
					window, field.Caption, field.Left, field.Right, fields[0].Caption, fields[0].Left, fields[0].Right)
			}
		}
	}
}
