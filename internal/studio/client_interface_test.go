package studio

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The interface the window writes defines the five panels of the platform,
// placed or not, as the prototype's file does (dosyomka-2649, the export
// after the edits), keeping the representation it gave the panel of
// sections. Defect caught: a file written with the definitions the window
// happened to hold - none for a panel never shown.
func TestTheClientInterfaceWrittenDefinesEveryPanel(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	read, err := workspace.ReadClientInterface()
	if err != nil {
		t.Fatal(err)
	}
	id, panel := uuid.MustNew(), metadata.PanelID(metadata.SectionsPanel)
	value := metadata.ClientInterface{Left: []metadata.ClientInterfaceItem{{ID: &id, Panel: &panel}},
		Panels: []metadata.ClientInterfacePanel{{ID: metadata.SectionsPanel, PanelRepresentation: metadata.SectionsPanelText}}}
	if _, err := workspace.SaveClientInterface(value, read.Revision); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(root, project.ClientInterfaceFile))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	written, err := metadata.DecodeRootDescription(project.ClientInterfaceFile, file)
	if err != nil {
		t.Fatal(err)
	}
	var defined []string
	for _, panel := range written.(metadata.ClientInterface).Panels {
		defined = append(defined, panel.ID.String()+":"+string(panel.PanelRepresentation))
	}
	want := []string{"b553047f-c9aa-4157-978d-448ecad24248:text", "13322b22-3960-4d68-93a6-fe2dd7f28ca3:", "c933ac92-92cd-459d-81cc-e0c8a83ced99:",
		"cbab57f2-a0f3-4f0a-89ea-4cb19570ab75:", "b2735bd3-d822-4430-ba59-c9e869693b24:"}
	if !slices.Equal(defined, want) {
		t.Errorf("the panels defined are %v, want %v", defined, want)
	}
}
