package publication

import (
	"context"
	"path/filepath"
	"testing"
)

// The bundled example project is the one source tree that is not built by a
// test: if inspection of it ever stops working, everything downstream of
// "Сохранить данные" is broken for a real project, not just a fixture.
func TestSalesAndWarehouseDemoInspects(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "examples", "sales-and-warehouse")
	manifest, err := inspect(context.Background(), root, SourceState{Dirty: true})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ProjectName != "ПродажиИСклад" || len(manifest.DocumentIDs) != 2 || len(manifest.AccumulationRegisterIDs) != 1 {
		t.Fatalf("manifest = %+v", manifest)
	}
}
