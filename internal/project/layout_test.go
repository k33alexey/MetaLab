package project

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestInitializeAndValidateCanonicalLayout(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "SalesDemo")
	manifest := testManifest()
	if err := Initialize(root, manifest); err != nil {
		t.Fatal(err)
	}
	loaded, err := ValidateLayout(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != manifest.ID || loaded.Name != manifest.Name {
		t.Fatalf("loaded manifest = %+v, want %+v", loaded, manifest)
	}
	for _, directory := range RootDirectories() {
		info, err := os.Stat(filepath.Join(root, directory, keepFile))
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("Git placeholder for %q: info=%v error=%v", directory, info, err)
		}
	}
}

func TestInitializeNeverOverwritesExistingPath(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "existing")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(root, "owned-by-user")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Initialize(root, testManifest()); !errors.Is(err, ErrProjectExists) {
		t.Fatalf("Initialize() error = %v, want ErrProjectExists", err)
	}
	content, err := os.ReadFile(sentinel)
	if err != nil || string(content) != "keep" {
		t.Fatalf("existing content = %q, error=%v", content, err)
	}
}

func TestInitializeRejectsInvalidManifestWithoutCreatingTarget(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "invalid")
	if err := Initialize(root, Project{}); err == nil {
		t.Fatal("Initialize() returned no error")
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target exists after rejected manifest: %v", err)
	}
}

func TestValidateLayoutRejectsMissingDirectory(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "project")
	if err := Initialize(root, testManifest()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "forms", keepFile)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "forms")); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateLayout(root); !errors.Is(err, ErrInvalidLayout) {
		t.Fatalf("ValidateLayout() error = %v, want ErrInvalidLayout", err)
	}
}

func TestCanonicalSourcePathsUseStableUUIDs(t *testing.T) {
	t.Parallel()

	id, err := uuid.Parse("018f1f72-3b4c-7d6e-8f90-123456789abc")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		path func(uuid.UUID) (string, error)
		want string
	}{
		{name: "module", path: ModulePath, want: path.Join("modules", id.String()+".bsl")},
		{name: "form", path: FormPath, want: path.Join("forms", id.String()+".yaml")},
		{name: "report", path: ReportPath, want: path.Join("reports", id.String()+".yaml")},
		{name: "test", path: TestPath, want: path.Join("tests", id.String()+".bsl")},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := test.path(id)
			if err != nil || got != test.want {
				t.Fatalf("path = %q, error=%v, want %q", got, err, test.want)
			}
		})
	}
	metadata, err := MetadataPath("catalogs", id)
	if err != nil || metadata != path.Join("metadata", "catalogs", id.String()+".yaml") {
		t.Fatalf("metadata path = %q, error=%v", metadata, err)
	}
	asset, err := AssetPath(id, ".png")
	if err != nil || asset != path.Join("assets", id.String()+".png") {
		t.Fatalf("asset path = %q, error=%v", asset, err)
	}
	if _, err := AssetPath(id, "../png"); err == nil {
		t.Fatal("AssetPath() accepted an unsafe extension")
	}
	if _, err := MetadataPath("unknown", id); err == nil {
		t.Fatal("MetadataPath() accepted an unknown kind")
	}
	if _, err := ModulePath(uuid.UUID{}); err == nil {
		t.Fatal("ModulePath() accepted a zero UUID")
	}
}

func TestMetadataCatalogContainsAgreedObjectTypes(t *testing.T) {
	t.Parallel()

	want := []string{
		"subsystems", "common-modules", "session-parameters", "roles", "common-attributes",
		"event-subscriptions", "scheduled-jobs", "defined-types", "common-commands", "common-forms",
		"common-templates", "common-pictures", "http-services", "styles", "languages", "constants",
		"settings-storages", "catalogs", "documents", "document-journals", "enumerations", "reports",
		"data-processors", "charts-of-characteristic-types", "charts-of-accounts", "information-registers",
		"accumulation-registers", "accounting-registers", "folders",
	}
	if !slices.Equal(MetadataKinds(), want) {
		t.Fatalf("MetadataKinds() = %v, want %v", MetadataKinds(), want)
	}
}

func TestDirectoryCatalogCannotBeMutatedByCaller(t *testing.T) {
	t.Parallel()

	directories := RootDirectories()
	directories[0] = "changed"
	if slices.Contains(RootDirectories(), "changed") {
		t.Fatal("RootDirectories() exposed internal state")
	}
	kinds := MetadataKinds()
	kinds[0] = "changed"
	if slices.Contains(MetadataKinds(), "changed") {
		t.Fatal("MetadataKinds() exposed internal state")
	}
}

func testManifest() Project {
	return Project{
		Format: CurrentFormat, ID: uuid.MustNew(), Name: "SalesDemo", Title: "Продажи и склад",
		DefaultLanguage: "ru", Languages: []Language{{Name: "Русский", Title: "Русский", Code: "ru"}},
	}
}
