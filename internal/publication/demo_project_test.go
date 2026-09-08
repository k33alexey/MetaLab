package publication

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSalesAndWarehouseDemoBuildsAndVerifies(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "examples", "sales-and-warehouse")
	packagePath := filepath.Join(t.TempDir(), "sales-and-warehouse"+PackageExtension)
	built, err := BuildFile(context.Background(), root, packagePath, SourceState{Dirty: true})
	if err != nil {
		t.Fatal(err)
	}
	verified, err := VerifyFile(context.Background(), packagePath)
	if err != nil {
		t.Fatal(err)
	}
	if built.ProjectName != "ПродажиИСклад" || verified.ContentSHA256 != built.ContentSHA256 || len(verified.DocumentIDs) != 2 || len(verified.AccumulationRegisterIDs) != 1 {
		t.Fatalf("built=%+v verified=%+v", built, verified)
	}
}
