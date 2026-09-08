package metadata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSalesAndWarehouseDemoProject(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "examples", "sales-and-warehouse")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Catalogs) != 2 || len(catalog.Documents) != 2 || len(catalog.AccumulationRegisters) != 1 {
		t.Fatalf("demo metadata: catalogs=%d documents=%d accumulation registers=%d", len(catalog.Catalogs), len(catalog.Documents), len(catalog.AccumulationRegisters))
	}
	for _, document := range catalog.Documents {
		form, err := catalog.DocumentForm(document.Name, ObjectForm, "uk")
		if err != nil || !form.Generated || !hasFormCommand(form.Commands, "Post") || !hasFormCommand(form.Commands, "Movements") {
			t.Fatalf("document %s form=%+v error=%v", document.Name, form, err)
		}
		if document.ObjectModule == nil {
			t.Fatalf("document %s has no posting module", document.Name)
		}
		source, err := os.ReadFile(filepath.Join(root, "modules", document.ObjectModule.String()+".bsl"))
		if err != nil {
			t.Fatal(err)
		}
		if _, diagnostics := CompileDocumentObjectModule(document, document.Name+".bsl", string(source)); len(diagnostics) != 0 {
			t.Fatalf("document %s module diagnostics: %v", document.Name, diagnostics)
		}
	}
	if _, err := catalog.ApplicationSchema(); err != nil {
		t.Fatal(err)
	}
}
