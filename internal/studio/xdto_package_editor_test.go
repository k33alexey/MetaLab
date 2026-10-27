package studio

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
)

// The editor of an XDTO package saves the body and the description as one
// (paket-xdto.md; xdto_package_editor.go): the body is written in the
// prototype's form under the namespace of the package, a namespace changed
// in the palette of the root goes into both; a body changed outside the
// editor since it was read refuses the save; a description refused puts
// the body back as it was.
// Defects caught: the body not written, or written under the namespace
// before; a save over a body somebody else changed; a body left changed
// beside a description the model refused.
func TestTheEditorOfAnXDTOPackageSavesTheBodyWithTheDescription(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	writeXDTOPackageForStudioTest(t, root)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	relative := "metadata/xdto-packages/Обмен/object.yaml"
	body := filepath.Join(root, "metadata", "xdto-packages", "Обмен", "content.xml")
	opened, err := workspace.ReadXDTOPackageEditor(relative)
	if err != nil {
		t.Fatal(err)
	}
	window := XDTOPackageWindowValue{XDTOPackageDefinition: opened.Package, Content: opened.Content}
	window.Namespace = "http://example.org/goods/2"
	window.Content.Imports = []string{"http://example.org/base"}
	saved, err := workspace.SaveXDTOPackageWindow(relative, window, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	written, _ := os.ReadFile(body)
	if !strings.Contains(string(written), `targetNamespace="http://example.org/goods/2"`) || !strings.Contains(string(written), `<import namespace="http://example.org/base"/>`) {
		t.Fatalf("the body written: %s", written)
	}
	if saved.Package.Namespace != "http://example.org/goods/2" || len(saved.Content.Imports) != 1 {
		t.Fatalf("read after the save: %+v", saved)
	}
	if err := os.WriteFile(body, []byte(strings.Replace(string(written), "base", "other", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SaveXDTOPackageWindow(saved.Path, window, saved.Revision); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("a body changed outside the editor: %v", err)
	}
	changed, err := workspace.ReadXDTOPackageEditor(saved.Path)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(body)
	refused := XDTOPackageWindowValue{XDTOPackageDefinition: changed.Package, Content: changed.Content}
	refused.Namespace = "http://example.org/with space"
	refused.Content.Imports = append(refused.Content.Imports, "http://example.org/new")
	if _, err := workspace.SaveXDTOPackageWindow(changed.Path, refused, changed.Revision); err == nil {
		t.Fatal("a namespace with a space was taken")
	}
	after, _ := os.ReadFile(body)
	if string(after) != string(before) {
		t.Errorf("the body is left changed beside the refused description:\n%s", after)
	}
	if _, err := metadata.Load(root); err != nil {
		t.Errorf("the project after the refusal: %v", err)
	}
}
