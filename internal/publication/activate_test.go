package publication

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestActivateRequiresConfirmationBeforeReadingPackageOrDatabase(t *testing.T) {
	_, _, err := Activate(context.Background(), nil, ActivationRequest{})
	if !errors.Is(err, schemadiff.ErrConfirmationRequired) {
		t.Fatalf("Activate() error = %v", err)
	}
}

func TestCurrentAndListVersionsValidateArguments(t *testing.T) {
	if _, _, err := Current(context.Background(), nil); err == nil {
		t.Fatal("Current() accepted a nil database")
	}
	if _, err := ListVersions(context.Background(), nil, 10); err == nil {
		t.Fatal("ListVersions() accepted a nil database")
	}
}

func TestActivateRejectsSchemaNotDerivedFromPackage(t *testing.T) {
	t.Parallel()
	root := publicationProject(t)
	id := uuid.MustNew()
	relative, err := project.MetadataPath("catalogs", id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(relative))), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "format: 1\nid: " + id.String() + "\nname: Товары\ntitle: {ru: Товары}\n" +
		"code: {type: string, length: 9}\ndescription_length: 250\n"
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(relative)), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	commit := strings.Repeat("a", 40)
	packagePath := filepath.Join(t.TempDir(), "catalog.mlpkg")
	if _, err := BuildFile(context.Background(), root, packagePath, SourceState{GitCommit: commit}); err != nil {
		t.Fatal(err)
	}
	_, _, err = Activate(context.Background(), &pgxpool.Pool{}, ActivationRequest{
		PackagePath: packagePath, Desired: schemadiff.Schema{Name: schemadiff.ApplicationSchema, Exists: true},
		ExpectedGitCommit: commit, Mode: ActivationDebug, Confirmed: true,
	})
	if err == nil || !strings.Contains(err.Error(), "does not match packaged metadata") {
		t.Fatalf("Activate() error = %v", err)
	}
}
