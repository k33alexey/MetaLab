package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MetaLab does not carry bots and integration services at all, and a folder of
// either in a project is refused with that reason. The defect on the other side
// is the quiet one: before this, any folder in metadata/ that no kind reads was
// passed over by the load that saves the data, while Studio's tree refused it -
// a set of objects that went nowhere and said nothing.
func TestAFolderOfNoKindIsRefusedAndARefusedKindSaysWhy(t *testing.T) {
	t.Parallel()
	for folder, message := range map[string]string{
		"bots":                 "bots are not carried by MetaLab",
		"integration-services": "integration services are not carried by MetaLab",
		"interfaces":           "not a kind of metadata object",
	} {
		t.Run(folder, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			if err := os.MkdirAll(filepath.Join(root, "metadata", folder), 0o755); err != nil {
				t.Fatal(err)
			}
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), message) || !strings.Contains(err.Error(), "metadata/"+folder) {
				t.Fatalf("error = %v, expected it to name metadata/%s and say %q", err, folder, message)
			}
		})
	}
	// Every such folder is named at once, not the first of them.
	root := metadataProject(t)
	for _, folder := range []string{"bots", "заметки"} {
		if err := os.MkdirAll(filepath.Join(root, "metadata", folder), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "metadata/bots") || !strings.Contains(err.Error(), "metadata/заметки") {
		t.Fatalf("error = %v, expected both folders named", err)
	}
	// And a project with nothing but the known folders still loads.
	if _, err := Load(metadataProject(t)); err != nil {
		t.Fatalf("a clean project was refused: %v", err)
	}
}
