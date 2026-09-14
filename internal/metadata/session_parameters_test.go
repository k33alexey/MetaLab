package metadata

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const sessionParameterID = "70000000-0000-4000-8000-000000000001"

func TestDecodeSessionParameterStrictAndLocalized(t *testing.T) {
	t.Parallel()
	manifest := metadataManifest()
	parameter, err := DecodeSessionParameter("session-parameter.yaml", strings.NewReader(`format: 1
id: `+sessionParameterID+`
name: ТекущийПользователь
title:
  ru: Текущий пользователь
  uk: Поточний користувач
types:
  - kind: string
    length: 50
`), manifest)
	if err != nil {
		t.Fatal(err)
	}
	if parameter.Title.Resolve("uk", manifest.Languages) != "Поточний користувач" || parameter.Title.Resolve("en", manifest.Languages) != "Текущий пользователь" {
		t.Fatalf("localized title fallback = %q", parameter.Title.Resolve("en", manifest.Languages))
	}
	_, err = DecodeSessionParameter("session-parameter.yaml", strings.NewReader(`format: 1
id: `+sessionParameterID+`
name: Invalid
title: {de: Titel}
types: [{kind: string}]
unknown: true
`), manifest)
	if err == nil || !strings.Contains(err.Error(), "field unknown") {
		t.Fatalf("strict decode error = %v", err)
	}
	_, err = DecodeSessionParameter("session-parameter.yaml", strings.NewReader(strings.Replace(`format: 1
id: `+sessionParameterID+`
name: Invalid
title: {ru: Заголовок}
types: [{kind: string}]
`, "format: 1", "format: 2", 1)), manifest)
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("format error = %v", err)
	}
}

func TestCatalogSessionParameterLookupReturnsIsolatedCopies(t *testing.T) {
	t.Parallel()
	parameter := SessionParameter{
		Format: CurrentFormat, ID: uuid.MustNew(), Name: "ТекущийПользователь", Title: LocalizedText{"ru": "Текущий пользователь"},
		Types: []Type{{Kind: StringType, Length: 50}},
	}
	catalog, err := NewCatalogSnapshotWithSessionParameters(metadataManifest(), nil, nil, nil, nil, nil, nil, nil, nil, nil, []SessionParameter{parameter})
	if err != nil {
		t.Fatal(err)
	}
	byName, ok := catalog.SessionParameter("ТекущийПользователь")
	if !ok || byName.ID != parameter.ID {
		t.Fatalf("session parameter by name = %+v, %v", byName, ok)
	}
	byID, ok := catalog.SessionParameterByID(parameter.ID)
	if !ok || byID.Name != "ТекущийПользователь" {
		t.Fatalf("session parameter by id = %+v, %v", byID, ok)
	}
	byID.Types[0].Length = 999
	again, _ := catalog.SessionParameterByID(parameter.ID)
	if again.Types[0].Length != 50 {
		t.Fatal("session parameter lookup exposed mutable metadata")
	}
}

func TestLoadValidatesSessionParameterDefaults(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	if err := os.MkdirAll(filepath.Join(root, "metadata", string(SessionParameterKind)), 0o755); err != nil {
		t.Fatal(err)
	}
	validID := uuid.MustNew()
	writeMetadata(t, root, SessionParameterKind, validID.String(), "format: 1\nid: "+validID.String()+
		"\nname: ТекущийПользователь\ntitle: {ru: Текущий пользователь}\ntypes: [{kind: string, length: 50}]\ndefault: {kind: string, data: Гость}\n")
	if _, err := load(root, true); err != nil {
		t.Fatalf("valid session parameter default rejected: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "metadata", string(SessionParameterKind), validID.String()+".yaml"),
		[]byte("format: 1\nid: "+validID.String()+"\nname: ТекущийПользователь\ntitle: {ru: Текущий пользователь}\ntypes: [{kind: string, length: 50}]\ndefault: {kind: number, data: \"5\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := load(root, true); err == nil || !strings.Contains(err.Error(), "session parameter") {
		t.Fatalf("session parameter default of a disallowed kind was accepted: %v", err)
	}
}

func TestRuntimeSessionParameterGetSetRoundTrip(t *testing.T) {
	t.Parallel()
	defaultValue := Value{Kind: StringType, Data: "Гость"}
	withDefault := SessionParameter{
		Format: CurrentFormat, ID: uuid.MustNew(), Name: "ТекущийПользователь", Title: LocalizedText{"ru": "Текущий пользователь"},
		Types: []Type{{Kind: StringType, Length: 50}}, Default: &defaultValue,
	}
	withoutDefault := SessionParameter{
		Format: CurrentFormat, ID: uuid.MustNew(), Name: "СчётчикЗапросов", Title: LocalizedText{"ru": "Счётчик запросов"},
		Types: []Type{{Kind: NumberType, Precision: 9}},
	}
	catalog, err := NewCatalogSnapshotWithSessionParameters(metadataManifest(), nil, nil, nil, nil, nil, nil, nil, nil, nil,
		[]SessionParameter{withDefault, withoutDefault})
	if err != nil {
		t.Fatal(err)
	}
	runtime := &Runtime{catalog: catalog, sessionParameters: make(map[string]bytecode.Value)}
	ctx := context.Background()

	if value, err := runtime.GetSessionParameter(ctx, "ТекущийПользователь"); err != nil || value.String() != "Гость" {
		t.Fatalf("default value = %v, error = %v", value, err)
	}
	if value, err := runtime.GetSessionParameter(ctx, "СчётчикЗапросов"); err != nil || value.Kind() != bytecode.UndefinedKind {
		t.Fatalf("unset value without default = %v, error = %v", value, err)
	}
	if err := runtime.SetSessionParameter(ctx, "текущийпользователь", bytecode.String("Иванов")); err != nil {
		t.Fatal(err)
	}
	if value, err := runtime.GetSessionParameter(ctx, "ТЕКУЩИЙПОЛЬЗОВАТЕЛЬ"); err != nil || value.String() != "Иванов" {
		t.Fatalf("stored value not case-insensitive = %v, error = %v", value, err)
	}
	if err := runtime.SetSessionParameter(ctx, "ТекущийПользователь", bytecode.Number(42)); err == nil {
		t.Fatal("session parameter accepted a value of a disallowed kind")
	}
	if value, _ := runtime.GetSessionParameter(ctx, "ТекущийПользователь"); value.String() != "Иванов" {
		t.Fatalf("rejected set must not change the stored value, got %v", value)
	}
	if _, err := runtime.GetSessionParameter(ctx, "Неизвестный"); err == nil || err.Error() != `unknown session parameter "Неизвестный"` {
		t.Fatalf("unknown session parameter get error = %v", err)
	}
	if err := runtime.SetSessionParameter(ctx, "Неизвестный", bytecode.String("x")); err == nil || err.Error() != `unknown session parameter "Неизвестный"` {
		t.Fatalf("unknown session parameter set error = %v", err)
	}
}
