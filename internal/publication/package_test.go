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
