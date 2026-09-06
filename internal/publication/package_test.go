package publication

import (
	"archive/zip"
	"bytes"
	"context"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestBuildFileIsDeterministicAndVerifiable(t *testing.T) {
	root := publicationProject(t)
	modulePath, _ := project.ModulePath(uuid.MustNew())
	formPath, _ := project.FormPath(uuid.MustNew())
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(modulePath)), []byte("Процедура Тест()\nКонецПроцедуры\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(formPath)), []byte("format: 1\nname: Main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := SourceState{GitCommit: "0123456789abcdef0123456789abcdef01234567", Dirty: false}
	firstPath, secondPath := filepath.Join(t.TempDir(), "first.mlpkg"), filepath.Join(t.TempDir(), "second.mlpkg")
	first, err := BuildFile(context.Background(), root, firstPath, state)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildFile(context.Background(), root, secondPath, state)
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, _ := os.ReadFile(firstPath)
	secondBytes, _ := os.ReadFile(secondPath)
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatalf("identical sources produced different packages: %x != %x", sha256.Sum256(firstBytes), sha256.Sum256(secondBytes))
	}
	if !reflect.DeepEqual(first, second) || first.Format != CurrentPackageFormat || first.GitCommit != state.GitCommit || len(first.Files) != 3 {
		t.Fatalf("manifest = %+v, second = %+v", first, second)
	}
	verified, err := VerifyFile(context.Background(), firstPath)
	if err != nil || !reflect.DeepEqual(verified, first) {
		t.Fatalf("verified manifest=%+v error=%v", verified, err)
	}
	archive, err := zip.OpenReader(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if len(archive.File) != 4 || archive.File[0].Name != "package.json" {
		t.Fatalf("archive entries = %+v", archive.File)
	}
	manifestFile, err := archive.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	var decoded Manifest
	decodeErr := json.NewDecoder(manifestFile).Decode(&decoded)
	_ = manifestFile.Close()
	if decodeErr != nil || decoded.ContentSHA256 != first.ContentSHA256 || decoded.ProjectID != first.ProjectID {
		t.Fatalf("decoded manifest=%+v error=%v", decoded, decodeErr)
	}
	corrupted := filepath.Join(t.TempDir(), "corrupted.mlpkg")
	rewritePackageSource(t, firstPath, corrupted)
	if _, err := VerifyFile(context.Background(), corrupted); err == nil {
		t.Fatal("VerifyFile accepted modified package content")
	}
}

func TestBuildChangesDigestAndAtomicallyReplacesDestination(t *testing.T) {
	root := publicationProject(t)
	modulePath, _ := project.ModulePath(uuid.MustNew())
	absolute := filepath.Join(root, filepath.FromSlash(modulePath))
	if err := os.WriteFile(absolute, []byte("Первый();\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "project.mlpkg")
	first, err := BuildFile(context.Background(), root, destination, SourceState{Dirty: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absolute, []byte("Второй();\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := BuildFile(context.Background(), root, destination, SourceState{Dirty: true})
	if err != nil {
		t.Fatal(err)
	}
	if first.ContentSHA256 == second.ContentSHA256 {
		t.Fatal("source change did not change publication digest")
	}
}

func TestBuildRejectsInvalidSourcesWithoutPublishing(t *testing.T) {
	root := publicationProject(t)
	formPath, _ := project.FormPath(uuid.MustNew())
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(formPath)), []byte("broken: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "invalid.mlpkg")
	if _, err := BuildFile(context.Background(), root, destination, SourceState{}); err == nil {
		t.Fatal("BuildFile accepted malformed YAML")
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid package was published: %v", err)
	}
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(formPath))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "modules", "manual.bsl"), []byte("Тест();\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildFile(context.Background(), root, destination, SourceState{}); err == nil {
		t.Fatal("BuildFile accepted a non-UUID source path")
	}
}

func TestBuildRejectsInvalidSupportedMetadata(t *testing.T) {
	t.Parallel()
	root := publicationProject(t)
	id := uuid.MustNew()
	relative, err := project.MetadataPath("constants", id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(relative))), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "format: 1\nid: " + id.String() + "\nname: Invalid\ntitle: {de: Ungültig}\ntypes: [{kind: boolean}]\n"
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(relative)), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(context.Background(), root, &bytes.Buffer{}, SourceState{}); err == nil || !strings.Contains(err.Error(), "unconfigured language") {
		t.Fatalf("Build() error = %v", err)
	}
}

func TestPackageCarriesVerifiedConstantIDs(t *testing.T) {
	t.Parallel()
	root := publicationProject(t)
	id := uuid.MustNew()
	relative, _ := project.MetadataPath("constants", id)
	absolute := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "format: 1\nid: " + id.String() + "\nname: Режим\ntitle: {ru: Режим}\ntypes: [{kind: boolean}]\n"
	if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join(t.TempDir(), "constant.mlpkg")
	built, err := BuildFile(context.Background(), root, packagePath, SourceState{})
	if err != nil || len(built.ConstantIDs) != 1 || built.ConstantIDs[0] != id {
		t.Fatalf("built=%+v error=%v", built, err)
	}
	verified, err := VerifyFile(context.Background(), packagePath)
	if err != nil || !reflect.DeepEqual(verified.ConstantIDs, built.ConstantIDs) {
		t.Fatalf("verified=%+v error=%v", verified, err)
	}
}

func TestPackageCarriesCatalogSchemaIdentity(t *testing.T) {
	t.Parallel()
	root := publicationProject(t)
	id := uuid.MustNew()
	attributeID := uuid.MustNew()
	relative, _ := project.MetadataPath("catalogs", id)
	absolute := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "format: 1\nid: " + id.String() + "\nname: Товары\ntitle: {ru: Товары}\n" +
		"code: {type: string, length: 9, auto: true, unique: true}\ndescription_length: 250\n" +
		"attributes:\n  - id: " + attributeID.String() + "\n    name: Артикул\n    title: {ru: Артикул}\n    types: [{kind: string, length: 32}]\n    indexed: true\n"
	if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join(t.TempDir(), "catalog.mlpkg")
	built, err := BuildFile(context.Background(), root, packagePath, SourceState{})
	if err != nil || len(built.CatalogIDs) != 1 || built.CatalogIDs[0] != id || len(built.SchemaSHA256) != 64 {
		t.Fatalf("built=%+v error=%v", built, err)
	}
	verified, err := VerifyFile(context.Background(), packagePath)
	if err != nil || !reflect.DeepEqual(verified, built) {
		t.Fatalf("verified=%+v error=%v", verified, err)
	}
}

func TestPackageCarriesDocumentSchemaAndSources(t *testing.T) {
	t.Parallel()
	root := publicationProject(t)
	documentID, moduleID, formID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	modulePath, _ := project.ModulePath(moduleID)
	formPath, _ := project.FormPath(formID)
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(modulePath)), []byte("Процедура ПриЗаписи(Отказ)\nКонецПроцедуры\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(formPath)), []byte("format: 1\nname: DocumentForm\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	relative, _ := project.MetadataPath("documents", documentID)
	absolute := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "format: 1\nid: " + documentID.String() + "\nname: Продажа\ntitle: {ru: Продажа}\n" +
		"number: {type: string, length: 11, auto: false, unique: true, periodicity: year}\nposting: true\n" +
		"object_module: " + moduleID.String() + "\nforms: {object: " + formID.String() + "}\n"
	if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join(t.TempDir(), "document.mlpkg")
	built, err := BuildFile(context.Background(), root, packagePath, SourceState{})
	if err != nil || len(built.DocumentIDs) != 1 || built.DocumentIDs[0] != documentID || len(built.SchemaSHA256) != 64 {
		t.Fatalf("built=%+v error=%v", built, err)
	}
	verified, err := VerifyFile(context.Background(), packagePath)
	if err != nil || !reflect.DeepEqual(verified, built) {
		t.Fatalf("verified=%+v error=%v", verified, err)
	}
}

func TestBuildHonoursCancellationAndRejectsSymlinks(t *testing.T) {
	root := publicationProject(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Build(ctx, root, &bytes.Buffer{}, SourceState{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Build() error = %v", err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	modulePath, _ := project.ModulePath(uuid.MustNew())
	if err := os.Symlink(filepath.Join(root, project.ManifestFile), filepath.Join(root, filepath.FromSlash(modulePath))); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(context.Background(), root, &bytes.Buffer{}, SourceState{}); err == nil {
		t.Fatal("Build accepted a symbolic link")
	}
}

func TestBuildDetectsConcurrentSourceChange(t *testing.T) {
	root := publicationProject(t)
	testPath, _ := project.TestPath(uuid.MustNew())
	absolute := filepath.Join(root, filepath.FromSlash(testPath))
	if err := os.WriteFile(absolute, []byte("Первый();\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	assetPath, _ := project.AssetPath(uuid.MustNew(), ".bin")
	asset := make([]byte, 256<<10)
	if _, err := crand.Read(asset); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(assetPath)), asset, 0o644); err != nil {
		t.Fatal(err)
	}
	writer := &mutatingWriter{mutate: func() {
		if err := os.WriteFile(absolute, []byte("Второй();\n"), 0o644); err != nil {
			t.Error(err)
		}
	}}
	if _, err := Build(context.Background(), root, writer, SourceState{}); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("concurrent Build() error = %v", err)
	}
	if _, err := Build(context.Background(), root, &bytes.Buffer{}, SourceState{GitCommit: "short"}); err == nil {
		t.Fatal("Build accepted a shortened Git commit")
	}
}

func publicationProject(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "project")
	manifest := project.Project{
		Format: project.CurrentFormat, ID: uuid.MustNew(), Name: "PackageDemo", Title: "Package Demo",
		DefaultLanguage: "ru", Languages: []project.Language{{Name: "Русский", Title: "Русский", Code: "ru"}},
	}
	if err := project.Initialize(root, manifest); err != nil {
		t.Fatal(err)
	}
	return root
}

type mutatingWriter struct {
	buffer bytes.Buffer
	once   sync.Once
	mutate func()
}

func (writer *mutatingWriter) Write(value []byte) (int, error) {
	writer.once.Do(writer.mutate)
	return writer.buffer.Write(value)
}

func rewritePackageSource(t *testing.T, source, destination string) {
	t.Helper()
	archive, err := zip.OpenReader(source)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	file, err := os.Create(destination)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for index, original := range archive.File {
		entry, err := writer.Create(original.Name)
		if err != nil {
			t.Fatal(err)
		}
		if index == 1 {
			_, err = entry.Write(bytes.Repeat([]byte{'x'}, int(original.UncompressedSize64)))
		} else {
			reader, openErr := original.Open()
			if openErr != nil {
				t.Fatal(openErr)
			}
			_, err = io.Copy(entry, reader)
			_ = reader.Close()
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
