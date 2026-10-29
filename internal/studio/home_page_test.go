package studio

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The page of the home page is written as the loader reads it, and only
// over what was read. Defects caught: a page made over one somebody else
// made meanwhile; a page written over a revision gone stale; a page the
// loader refuses written all the same, or half written; the revision not
// answered after the first save, so the second is refused.
func TestTheHomePageIsWrittenOnlyOverWhatWasRead(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, project.HomePageFile)
	window, err := workspace.ReadHomePage()
	if err != nil {
		t.Fatal(err)
	}
	if window.Revision != "" || window.Page.Template != metadata.TwoColumnsEqualWidthHomePage {
		t.Fatalf("a project of no page reads %+v", window)
	}
	common, _ := uuid.Parse("10000000-0000-4000-8000-000000000501")
	page := metadata.HomePage{Template: metadata.OneColumnHomePage, Left: []metadata.HomePageForm{{Form: common, Height: 10, Visibility: project.InterfaceVisibility{Common: true}}}}

	if _, err := workspace.SaveHomePage(page, "0000000000000000000000000000000000000000000000000000000000000000"); !errors.Is(err, ErrSourceChanged) {
		t.Errorf("a page over a revision of no file: %v", err)
	}
	if err := os.WriteFile(file, []byte("template: two-columns-equal-width\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SaveHomePage(page, ""); !errors.Is(err, ErrSourceChanged) {
		t.Errorf("a page made over one made meanwhile: %v", err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}

	saved, err := workspace.SaveHomePage(page, "")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision == "" || saved.Page.Template != metadata.OneColumnHomePage || len(saved.Page.Left) != 1 || saved.Names[common] != "ОбщаяФорма.ДемоФормаТовара" {
		t.Fatalf("the first save answered %+v", saved)
	}
	before, _ := os.ReadFile(file)
	wrong := page
	wrong.Right = page.Left
	if _, err := workspace.SaveHomePage(wrong, saved.Revision); err == nil {
		t.Error("a page of one column with a right column was written")
	}
	if after, _ := os.ReadFile(file); string(after) != string(before) {
		t.Errorf("a refused page changed the file:\n%s", after)
	}
	page.MainSectionCommands = metadata.MainSectionCommandsTop
	again, err := workspace.SaveHomePage(page, saved.Revision)
	if err != nil {
		t.Fatalf("the second save over the revision answered: %v", err)
	}
	if _, err := workspace.SaveHomePage(page, saved.Revision); !errors.Is(err, ErrSourceChanged) {
		t.Errorf("a save over a stale revision: %v", err)
	}
	if again.Page.MainSectionCommands != metadata.MainSectionCommandsTop {
		t.Errorf("the second save answered %+v", again.Page)
	}
}
