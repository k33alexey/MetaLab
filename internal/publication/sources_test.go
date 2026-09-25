package publication

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Inspection is what "Сохранить данные" reads the project through, so the
// same directory has to produce the same manifest every time: the content
// digest is what tells a database whether anything changed at all.
func TestInspectIsDeterministicOverProjectSources(t *testing.T) {
	t.Parallel()
	root := publicationProject(t)
	modulePath, _ := project.ModulePath(uuid.MustNew())
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(modulePath)), []byte("Процедура Тест()\nКонецПроцедуры\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := inspect(context.Background(), root, SourceState{GitCommit: "0123456789abcdef0123456789abcdef01234567"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := inspect(context.Background(), root, SourceState{GitCommit: "0123456789abcdef0123456789abcdef01234567"})
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("second inspection differs: %+v error=%v", second, err)
	}
	if len(first.ContentSHA256) != 64 || first.Format != CurrentPackageFormat || first.ProjectName != "PackageDemo" {
		t.Fatalf("manifest = %+v", first)
	}
	if !slicesContainPath(first.Files, project.ManifestFile) || !slicesContainPath(first.Files, modulePath) {
		t.Fatalf("files = %+v", first.Files)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(modulePath)), []byte("Процедура Другой()\nКонецПроцедуры\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := inspect(context.Background(), root, SourceState{})
	if err != nil || changed.ContentSHA256 == first.ContentSHA256 {
		t.Fatalf("source change did not change the content digest: %+v error=%v", changed, err)
	}
}

func TestInspectRejectsMalformedAndUnsupportedSources(t *testing.T) {
	t.Parallel()
	root := publicationProject(t)
	formID := uuid.MustNew()
	formPath, _ := project.ObjectFormPath("documents", "Продажа", "Invalid")
	writeSourceFile(t, root, formPath, []byte("format: 1\nid: "+formID.String()+"\nname: Invalid\ntitle: {ru: Invalid}\nkind: unsupported\n"))
	if _, err := inspect(context.Background(), root, SourceState{}); err == nil {
		t.Fatal("inspect accepted a malformed managed form")
	}
	if err := os.RemoveAll(filepath.Join(root, "metadata", "documents")); err != nil {
		t.Fatal(err)
	}

	constantID := uuid.MustNew()
	constantPath, _ := project.MetadataPath("constants", constantID)
	writeSourceFile(t, root, constantPath, []byte("format: 1\nid: "+constantID.String()+"\nname: Invalid\ntitle: {de: Ungültig}\ntypes: [{kind: boolean}]\n"))
	if _, err := inspect(context.Background(), root, SourceState{}); err == nil || !strings.Contains(err.Error(), "unconfigured language") {
		t.Fatalf("unconfigured language error = %v", err)
	}
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(constantPath))); err != nil {
		t.Fatal(err)
	}

	if _, err := inspect(context.Background(), root, SourceState{GitCommit: "short"}); err == nil {
		t.Fatal("inspect accepted a shortened Git commit")
	}
}

// The manifest carries the identity of everything the database has to create,
// so a missing kind here means silently missing tables after saving.
func TestInspectCarriesSchemaIdentityOfEveryStoredKind(t *testing.T) {
	t.Parallel()
	root := publicationProject(t)
	constantID := uuid.MustNew()
	constantPath, _ := project.MetadataPath("constants", constantID)
	writeSourceFile(t, root, constantPath, []byte("format: 1\nid: "+constantID.String()+"\nname: Режим\ntitle: {ru: Режим}\ntypes: [{kind: boolean}]\n"))

	catalogID, attributeID := uuid.MustNew(), uuid.MustNew()
	catalogPath, _ := project.ObjectMetadataPath("catalogs", "Товары")
	writeSourceFile(t, root, catalogPath, []byte("format: 1\nid: "+catalogID.String()+"\nname: Товары\ntitle: {ru: Товары}\n"+
		"code: {type: string, length: 9, auto: true, unique: true}\ndescription_length: 250\n"+
		"attributes:\n  - id: "+attributeID.String()+"\n    name: Артикул\n    title: {ru: Артикул}\n    types: [{kind: string, length: 32}]\n    indexed: true\n"))

	documentID, formID := uuid.MustNew(), uuid.MustNew()
	modulePath, _ := project.ObjectModulePath("documents", "Продажа", project.ObjectModuleFile)
	writeSourceFile(t, root, modulePath, []byte("Процедура ПриЗаписи(Отказ)\nКонецПроцедуры\n"))
	formPath, _ := project.ObjectFormPath("documents", "Продажа", "DocumentForm")
	writeSourceFile(t, root, formPath, managedFormYAML(t, formID, "DocumentForm"))
	documentPath, _ := project.ObjectMetadataPath("documents", "Продажа")
	writeSourceFile(t, root, documentPath, []byte("format: 1\nid: "+documentID.String()+"\nname: Продажа\ntitle: {ru: Продажа}\n"+
		"number: {type: string, length: 11, auto: false, unique: true, periodicity: year}\nposting: true\n"+
		"forms: {object: DocumentForm}\n"))

	informationID, informationDimensionID, informationResourceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	informationPath, _ := project.ObjectMetadataPath("information-registers", "КурсыВалют")
	writeSourceFile(t, root, informationPath, []byte("format: 1\nid: "+informationID.String()+"\nname: КурсыВалют\ntitle: {ru: Курсы валют}\n"+
		"write_mode: independent\nperiodicity: day\n"+
		"dimensions:\n  - id: "+informationDimensionID.String()+"\n    name: Валюта\n    title: {ru: Валюта}\n    types: [{kind: catalog, reference: "+catalogID.String()+"}]\n"+
		"resources:\n  - id: "+informationResourceID.String()+"\n    name: Курс\n    title: {ru: Курс}\n    types: [{kind: number, precision: 15, scale: 4}]\n"))

	accumulationID, accumulationDimensionID, accumulationResourceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	accumulationPath, _ := project.ObjectMetadataPath("accumulation-registers", "Продажи")
	writeSourceFile(t, root, accumulationPath, []byte("format: 1\nid: "+accumulationID.String()+"\nname: Продажи\ntitle: {ru: Продажи}\nkind: turnover\n"+
		"dimensions:\n  - id: "+accumulationDimensionID.String()+"\n    name: Товар\n    title: {ru: Товар}\n    types: [{kind: string, length: 100}]\n"+
		"resources:\n  - id: "+accumulationResourceID.String()+"\n    name: Сумма\n    title: {ru: Сумма}\n    types: [{kind: number, precision: 15, scale: 2}]\n"+
		"recorders: ["+documentID.String()+"]\n"))

	manifest, err := inspect(context.Background(), root, SourceState{})
	if err != nil {
		t.Fatal(err)
	}
	identities := map[string][]uuid.UUID{
		"constant":              {constantID},
		"catalog":               {catalogID},
		"document":              {documentID},
		"information register":  {informationID},
		"accumulation register": {accumulationID},
	}
	got := map[string][]uuid.UUID{
		"constant":              manifest.ConstantIDs,
		"catalog":               manifest.CatalogIDs,
		"document":              manifest.DocumentIDs,
		"information register":  manifest.InformationRegisterIDs,
		"accumulation register": manifest.AccumulationRegisterIDs,
	}
	for kind, want := range identities {
		if !reflect.DeepEqual(got[kind], want) {
			t.Fatalf("%s identities = %v, want %v", kind, got[kind], want)
		}
	}
	if len(manifest.SchemaSHA256) != 64 || manifest.Runtime.Project.Name != "PackageDemo" {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestInspectHonoursCancellationAndRejectsSymlinks(t *testing.T) {
	root := publicationProject(t)
	modulePath, _ := project.ModulePath(uuid.MustNew())
	writeSourceFile(t, root, modulePath, []byte("Процедура Тест()\nКонецПроцедуры\n"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := inspect(ctx, root, SourceState{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled inspect() error = %v", err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	linkPath, _ := project.ModulePath(uuid.MustNew())
	if err := os.Symlink(filepath.Join(root, project.ManifestFile), filepath.Join(root, filepath.FromSlash(linkPath))); err != nil {
		t.Fatal(err)
	}
	if _, err := inspect(context.Background(), root, SourceState{}); err == nil {
		t.Fatal("inspect accepted a symbolic link")
	}
}

func slicesContainPath(entries []FileEntry, path string) bool {
	for _, entry := range entries {
		if entry.Path == path {
			return true
		}
	}
	return false
}

func writeSourceFile(t *testing.T, root, relative string, content []byte) {
	t.Helper()
	absolute := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absolute, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func managedFormYAML(t *testing.T, id uuid.UUID, name string) []byte {
	t.Helper()
	var content bytes.Buffer
	form := metadata.ManagedForm{Format: metadata.CurrentFormat, ID: id, Name: name, Title: metadata.LocalizedText{"ru": name}, Kind: metadata.ObjectForm}
	if err := metadata.Encode(&content, form); err != nil {
		t.Fatal(err)
	}
	return content.Bytes()
}

func publicationProject(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "project")
	manifest := project.Project{
		Format: project.CurrentFormat, ID: uuid.MustNew(), Name: "PackageDemo", Title: "Package Demo",
		DefaultLanguage: "ru", Languages: []project.Language{{ID: uuid.MustNew(), Name: "Русский", Title: "Русский", Code: "ru"}},
	}
	if err := project.Initialize(root, manifest); err != nil {
		t.Fatal(err)
	}
	return root
}
