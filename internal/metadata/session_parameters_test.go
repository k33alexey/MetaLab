package metadata

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const sessionParameterID = "70000000-0000-4000-8000-000000000001"

func TestDecodeSessionParameterStrictAndLocalized(t *testing.T) {
	t.Parallel()
	configuration := metadataConfiguration()
	parameter, err := DecodeSessionParameter("session-parameter.yaml", strings.NewReader(`format: 1
id: `+sessionParameterID+`
name: ТекущийСотрудник
title:
  ru: Текущий сотрудник
  uk: Поточний користувач
types:
  - kind: string
    length: 50
`), configuration)
	if err != nil {
		t.Fatal(err)
	}
	if parameter.Title.Resolve("uk", configuration.DefaultLanguage, configuration.Languages) != "Поточний користувач" || parameter.Title.Resolve("en", configuration.DefaultLanguage, configuration.Languages) != "Текущий сотрудник" {
		t.Fatalf("localized title fallback = %q", parameter.Title.Resolve("en", configuration.DefaultLanguage, configuration.Languages))
	}
	_, err = DecodeSessionParameter("session-parameter.yaml", strings.NewReader(`format: 1
id: `+sessionParameterID+`
name: Invalid
title: {de: Titel}
types: [{kind: string}]
unknown: true
`), configuration)
	if err == nil || !strings.Contains(err.Error(), "field unknown") {
		t.Fatalf("strict decode error = %v", err)
	}
	_, err = DecodeSessionParameter("session-parameter.yaml", strings.NewReader(strings.Replace(`format: 1
id: `+sessionParameterID+`
name: Invalid
title: {ru: Заголовок}
types: [{kind: string}]
`, "format: 1", "format: 2", 1)), configuration)
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("format error = %v", err)
	}
}

func TestCatalogSessionParameterLookupReturnsIsolatedCopies(t *testing.T) {
	t.Parallel()
	parameter := SessionParameter{
		Format: CurrentFormat, ID: uuid.MustNew(), Name: "ТекущийСотрудник", Title: LocalizedText{"ru": "Текущий сотрудник"},
		Types: []Type{{Kind: StringType, Length: 50}},
	}
	catalog, err := NewCatalogSnapshotWithSessionParameters(metadataConfiguration(), nil, nil, nil, nil, nil, nil, nil, nil, nil, []SessionParameter{parameter})
	if err != nil {
		t.Fatal(err)
	}
	byName, ok := catalog.SessionParameter("ТекущийСотрудник")
	if !ok || byName.ID != parameter.ID {
		t.Fatalf("session parameter by name = %+v, %v", byName, ok)
	}
	byID, ok := catalog.SessionParameterByID(parameter.ID)
	if !ok || byID.Name != "ТекущийСотрудник" {
		t.Fatalf("session parameter by id = %+v, %v", byID, ok)
	}
	byID.Types[0].Length = 999
	again, _ := catalog.SessionParameterByID(parameter.ID)
	if again.Types[0].Length != 50 {
		t.Fatal("session parameter lookup exposed mutable metadata")
	}
}

func TestRuntimeSessionParameterGetSetRoundTrip(t *testing.T) {
	t.Parallel()
	withDefault := SessionParameter{
		Format: CurrentFormat, ID: uuid.MustNew(), Name: "ТекущийСотрудник", Title: LocalizedText{"ru": "Текущий сотрудник"},
		Types: []Type{{Kind: StringType, Length: 50}},
	}
	withoutDefault := SessionParameter{
		Format: CurrentFormat, ID: uuid.MustNew(), Name: "СчётчикЗапросов", Title: LocalizedText{"ru": "Счётчик запросов"},
		Types: []Type{{Kind: NumberType, Precision: 9}},
	}
	catalog, err := NewCatalogSnapshotWithSessionParameters(metadataConfiguration(), nil, nil, nil, nil, nil, nil, nil, nil, nil,
		[]SessionParameter{withDefault, withoutDefault})
	if err != nil {
		t.Fatal(err)
	}
	runtime := &Runtime{catalog: catalog, sessionParameters: make(map[string]bytecode.Value)}
	ctx := context.Background()

	// Reading a parameter nobody set is an error, as on the platform (checked
	// by the owner, 01.10.2026) - not Undefined and not a default.
	if _, err := runtime.GetSessionParameter(ctx, "СчётчикЗапросов"); err == nil || !strings.Contains(err.Error(), "is not set") {
		t.Fatalf("an unset parameter was read without an error: %v", err)
	}
	if err := runtime.SetSessionParameter(ctx, "текущийсотрудник", bytecode.String("Иванов")); err != nil {
		t.Fatal(err)
	}
	if value, err := runtime.GetSessionParameter(ctx, "ТЕКУЩИЙСОТРУДНИК"); err != nil || value.String() != "Иванов" {
		t.Fatalf("stored value not case-insensitive = %v, error = %v", value, err)
	}
	if err := runtime.SetSessionParameter(ctx, "ТекущийСотрудник", bytecode.Number(42)); err == nil {
		t.Fatal("session parameter accepted a value of a disallowed kind")
	}
	if value, _ := runtime.GetSessionParameter(ctx, "ТекущийСотрудник"); value.String() != "Иванов" {
		t.Fatalf("rejected set must not change the stored value, got %v", value)
	}
	if _, err := runtime.GetSessionParameter(ctx, "Неизвестный"); err == nil || err.Error() != `unknown session parameter "Неизвестный"` {
		t.Fatalf("unknown session parameter get error = %v", err)
	}
	if err := runtime.SetSessionParameter(ctx, "Неизвестный", bytecode.String("x")); err == nil || err.Error() != `unknown session parameter "Неизвестный"` {
		t.Fatalf("unknown session parameter set error = %v", err)
	}
}
