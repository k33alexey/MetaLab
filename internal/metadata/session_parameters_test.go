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
	if parameter.Title.Presentation("uk", parameter.Name) != "Поточний користувач" || parameter.Title.Presentation("en", parameter.Name) != "ТекущийСотрудник" {
		t.Fatalf("localized title = %q", parameter.Title.Presentation("en", parameter.Name))
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

// Null is a type a session parameter may hold, alone and in a composite: the
// help names it among the types of the value, the designer offers it in the
// editor of the type and «Проверка конфигурации» takes both (opis of the
// session parameter, 14.10.2026). A field the database stores still may not
// hold it.
//
// Defect caught: a session parameter of Null refused, as when Null was a type
// of an attribute of a form alone; Null let into a stored field along with it.
func TestNullIsATypeOfASessionParameter(t *testing.T) {
	t.Parallel()
	for name, types := range map[string]string{
		"Null":           "[{kind: null-type}]",
		"Null в составе": "[{kind: string, length: 10}, {kind: null-type}, {kind: fixed-array}, {kind: binary-data}]",
	} {
		_, err := DecodeSessionParameter("session-parameter.yaml", strings.NewReader("format: 1\nid: "+sessionParameterID+
			"\nname: Параметр\ntitle: {ru: Параметр}\ntypes: "+types+"\n"), metadataConfiguration())
		if err != nil {
			t.Fatalf("%s: a session parameter of Null was refused: %v", name, err)
		}
	}
	_, err := DecodeCatalog("object.yaml", strings.NewReader(ceilingCatalog("{type: string, length: 9}", "100")+
		"attributes:\n  - {id: "+uuid.MustNew().String()+", name: Реквизит, title: {ru: Реквизит}, types: [{kind: null-type}]}\n"),
		metadataConfiguration())
	if err == nil || !strings.Contains(err.Error(), "null-type is a type of an attribute of a form and cannot stand in a field the database stores") {
		t.Fatalf("an attribute of a catalog of Null: %v", err)
	}
}
