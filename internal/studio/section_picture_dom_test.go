package studio

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
)

// sectionPictureWalk opens the window «Командный интерфейс» of Продажи and
// reads the signs of the rows of «Подразделы», then the window of the root
// and the signs of its sections.
const sectionPictureWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 150)).then(() => studio.settle());
  const dialog = () => document.querySelector('dialog.section-commands-dialog');
  const signs = () => Object.fromEntries([...dialog().querySelectorAll('.studio-tree-row')].map(row => {
    const sign = row.children[1];
    return [row.textContent, sign.tagName === 'IMG' ? 'img ' + decodeURIComponent(sign.getAttribute('src')).replace(/.*name=/, '') : sign.querySelector('use')?.getAttribute('href') || ''];
  }));
  await openObject('Продажи');
  [...document.querySelectorAll('#subsystem-editor .object-window-button')].find(item => item.textContent === 'Командный интерфейс').click(); await settle();
  steps.subsystem = signs();
  [...dialog().querySelectorAll('.localized-title-actions button')].find(item => item.textContent === 'Отмена').click(); await settle();
  const root = document.querySelector('#tree summary');
  root.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent === 'Открыть командный интерфейс проекта').click(); await settle();
  steps.root = signs();
  return steps;
})()`

// The row of a section in the windows of a command interface shows the
// picture of its subsystem (podsistema.md, «Досъёмка 29.10.2026», В6;
// snapshots dosyomka-2642/09-13, series-2.595/36, 67; the owner's decision
// of 29.10.2026): a picture of the platform by its icon, a common picture
// by its image; a subsystem of no picture the sign of a subsystem - in
// «Подразделы» of a subsystem and among the sections of the root alike.
// Defects caught: every section drawn with the sign of a subsystem; a
// picture of the platform or a common one not drawn; a subsystem of no
// picture drawn with none.
func TestTheRowOfASectionShowsThePictureOfItsSubsystem(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		picture, err := workspace.CreatePicture("Знак")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := workspace.SavePictureEditor(picture.Path, PictureWindowValue{CommonPictureDefinition: picture.Picture,
			Image: &PictureImage{Format: "png", Content: pngImage(t, 16, 16, 200)}}, picture.Revision); err != nil {
			t.Fatal(err)
		}
		common := picture.Picture.ID
		parent, err := workspace.CreateSubsystem("Продажи")
		if err != nil {
			t.Fatal(err)
		}
		for _, made := range []struct {
			name    string
			picture *metadata.PictureReference
			parent  bool
		}{
			{"Платформенная", &metadata.PictureReference{Standard: "BusinessProcess"}, true},
			{"Общая", &metadata.PictureReference{Common: &common}, true},
			{"Простая", nil, true},
			{"Корневая", &metadata.PictureReference{Standard: "ActiveUsers"}, false},
		} {
			var source SubsystemEditorSource
			if made.parent {
				source, err = workspace.CreateSubsystemChild(SubsystemChild{Parent: parent.Subsystem.ID})
			} else {
				source, err = workspace.CreateSubsystem(made.name)
			}
			if err != nil {
				t.Fatal(err)
			}
			value := source.Subsystem
			value.Name, value.Title, value.Picture = made.name, metadata.LocalizedText{"ru": made.name}, made.picture
			if _, err := workspace.SaveSubsystemWindow(source.Path, SubsystemWindowValue{SubsystemDefinition: value}, source.Revision); err != nil {
				t.Fatal(err)
			}
		}
	})
	result, err := browser.evaluate(ctx, page, sectionPictureWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct{ Subsystem, Root map[string]string }
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	for name, want := range map[string]string{"Платформенная": "#icon-business-process", "Общая": "img 100.png", "Простая": "#icon-subsystem"} {
		if got := steps.Subsystem[name]; got != want {
			t.Errorf("the subsection %s shows %q, want %q", name, got, want)
		}
	}
	for name, want := range map[string]string{"Корневая": "#icon-active-users", "Продажи": "#icon-subsystem"} {
		if got := steps.Root[name]; got != want {
			t.Errorf("the section %s of the root shows %q, want %q", name, got, want)
		}
	}
	var problems []string
	for _, problem := range browser.problems() {
		if !strings.Contains(problem, "favicon") {
			problems = append(problems, problem)
		}
	}
	if !slices.Equal(problems, nil) {
		t.Errorf("browser: %q", problems)
	}
}
