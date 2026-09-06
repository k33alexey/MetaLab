package metadata

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	constantID    = "10000000-0000-4000-8000-000000000001"
	enumerationID = "20000000-0000-4000-8000-000000000001"
	enumValueID   = "20000000-0000-4000-8000-000000000002"
	definedTypeID = "30000000-0000-4000-8000-000000000001"
	catalogID     = "40000000-0000-4000-8000-000000000001"
	attributeID   = "40000000-0000-4000-8000-000000000002"
	tablePartID   = "40000000-0000-4000-8000-000000000003"
	partFieldID   = "40000000-0000-4000-8000-000000000004"
)

func TestDecodeMetadataIsStrictAndLocalized(t *testing.T) {
	t.Parallel()
	manifest := metadataManifest()
	constant, err := DecodeConstant("constant.yaml", strings.NewReader(`format: 1
id: `+constantID+`
name: ОсновнаяВалюта
title:
  ru: Основная валюта
  uk: Основна валюта
types:
  - kind: string
    length: 3
`), manifest)
	if err != nil {
		t.Fatal(err)
	}
	if constant.Title.Resolve("uk", manifest.Languages) != "Основна валюта" || constant.Title.Resolve("en", manifest.Languages) != "Основная валюта" {
		t.Fatalf("localized title fallback = %q", constant.Title.Resolve("en", manifest.Languages))
	}
	_, err = DecodeConstant("constant.yaml", strings.NewReader(`format: 1
id: `+constantID+`
name: Invalid
title: {de: Titel}
types: [{kind: string}]
unknown: true
`), manifest)
	if err == nil || !strings.Contains(err.Error(), "field unknown") {
		t.Fatalf("strict decode error = %v", err)
	}
	_, err = DecodeConstant("constant.yaml", strings.NewReader(strings.Replace(`format: 1
id: `+constantID+`
name: Invalid
title: {ru: Заголовок}
types: [{kind: string}]
`, "format: 1", "format: 2", 1)), manifest)
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("format error = %v", err)
	}
}

func TestLoadCrossValidatesAndIndexesMetadata(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, EnumerationKind, enumerationID, `format: 1
id: `+enumerationID+`
name: Статусы
title: {ru: Статусы}
values:
  - id: `+enumValueID+`
    name: Активен
    title: {ru: Активен}
`)
	writeMetadata(t, root, DefinedTypeKind, definedTypeID, `format: 1
id: `+definedTypeID+`
name: КодИлиСтатус
title: {ru: Код или статус}
types:
  - kind: string
    length: 9
  - kind: enumeration
    reference: `+enumerationID+`
`)
	writeMetadata(t, root, ConstantKind, constantID, `format: 1
id: `+constantID+`
name: ТекущийСтатус
title: {ru: Текущий статус}
types:
  - kind: defined-type
    reference: `+definedTypeID+`
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	constant, ok := catalog.Constant("текущийстатус")
	if !ok || constant.ID.String() != constantID {
		t.Fatalf("constant = %+v, found=%v", constant, ok)
	}
	constant.Types[0].Kind = BooleanType
	again, _ := catalog.Constant("ТекущийСтатус")
	if again.Types[0].Kind != DefinedType {
		t.Fatal("catalog lookup exposed mutable type storage")
	}
	enumeration, ok := catalog.Enumeration("СТАТУСЫ")
	if !ok || enumeration.Values[0].ID.String() != enumValueID {
		t.Fatalf("enumeration = %+v, found=%v", enumeration, ok)
	}
}

func TestLoadRejectsBrokenReferencesIdentityAndCycles(t *testing.T) {
	t.Parallel()
	t.Run("filename", func(t *testing.T) {
		root := metadataProject(t)
		writeMetadata(t, root, ConstantKind, constantID, `format: 1
id: 10000000-0000-4000-8000-000000000002
name: Значение
title: {ru: Значение}
types: [{kind: boolean}]
`)
		if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "does not match filename") {
			t.Fatalf("Load() error = %v", err)
		}
	})
	t.Run("unknown reference", func(t *testing.T) {
		root := metadataProject(t)
		writeMetadata(t, root, ConstantKind, constantID, `format: 1
id: `+constantID+`
name: Значение
title: {ru: Значение}
types: [{kind: enumeration, reference: `+enumerationID+`}]
`)
		if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "unknown enumeration") {
			t.Fatalf("Load() error = %v", err)
		}
	})
	t.Run("cycle", func(t *testing.T) {
		root := metadataProject(t)
		other := "30000000-0000-4000-8000-000000000002"
		writeMetadata(t, root, DefinedTypeKind, definedTypeID, `format: 1
id: `+definedTypeID+`
name: Первый
title: {ru: Первый}
types: [{kind: defined-type, reference: `+other+`}]
`)
		writeMetadata(t, root, DefinedTypeKind, other, `format: 1
id: `+other+`
name: Второй
title: {ru: Второй}
types: [{kind: defined-type, reference: `+definedTypeID+`}]
`)
		if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "cycle") {
			t.Fatalf("Load() error = %v", err)
		}
	})
}

func TestNormalizeConstantValueExpandsDefinedTypes(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, EnumerationKind, enumerationID, `format: 1
id: `+enumerationID+`
name: Статусы
title: {ru: Статусы}
values: [{id: `+enumValueID+`, name: Активен, title: {ru: Активен}}]
`)
	writeMetadata(t, root, DefinedTypeKind, definedTypeID, `format: 1
id: `+definedTypeID+`
name: ЗначениеНастройки
title: {ru: Значение настройки}
types:
  - {kind: number, precision: 5, scale: 2}
  - {kind: enumeration, reference: `+enumerationID+`}
`)
	writeMetadata(t, root, ConstantKind, constantID, `format: 1
id: `+constantID+`
name: Настройка
title: {ru: Настройка}
types: [{kind: defined-type, reference: `+definedTypeID+`}]
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	constant, _ := catalog.Constant("Настройка")
	number, err := catalog.NormalizeValue(constant, Value{Kind: NumberType, Data: "001.20"})
	if err != nil || number.Data != "1.2" {
		t.Fatalf("number = %+v, error=%v", number, err)
	}
	enumeration, err := catalog.NormalizeValue(constant, Value{Kind: EnumerationType, Data: enumValueID})
	if err != nil || enumeration.Data != enumValueID {
		t.Fatalf("enumeration = %+v, error=%v", enumeration, err)
	}
	if _, err := catalog.NormalizeValue(constant, Value{Kind: NumberType, Data: "1234.56"}); err == nil {
		t.Fatal("NormalizeValue accepted excess precision")
	}
	if _, err := catalog.NormalizeValue(constant, Value{Kind: EnumerationType, Data: constantID}); err == nil {
		t.Fatal("NormalizeValue accepted a foreign enumeration UUID")
	}
}

func TestLoadCatalogWithAttributesTablePartsAndSelfReference(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true, unique: true}
description_length: 250
attributes:
  - id: `+attributeID+`
    name: Родитель
    title: {ru: Родитель}
    types: [{kind: catalog, reference: `+catalogID+`}]
    indexed: true
table_parts:
  - id: `+tablePartID+`
    name: Контакты
    title: {ru: Контакты}
    attributes:
      - id: `+partFieldID+`
        name: Телефон
        title: {ru: Телефон}
        types: [{kind: string, length: 32}]
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	definition, ok := catalog.CatalogDefinition("контрагенты")
	if !ok || definition.ID.String() != catalogID || len(definition.TableParts) != 1 {
		t.Fatalf("catalog = %+v, found=%v", definition, ok)
	}
	definition.Attributes[0].Name = "Changed"
	again, _ := catalog.CatalogDefinition("Контрагенты")
	if again.Attributes[0].Name != "Родитель" {
		t.Fatal("catalog lookup exposed mutable attribute storage")
	}
}

func TestDecodeCatalogRejectsInvalidShape(t *testing.T) {
	t.Parallel()
	_, err := DecodeCatalog("catalog.yaml", strings.NewReader(`format: 1
id: `+catalogID+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: boolean, length: 0}
description_length: 0
attributes:
  - id: `+attributeID+`
    name: Ссылка
    title: {ru: Ссылка}
    types: [{kind: string}]
`), metadataManifest())
	if err == nil || !strings.Contains(err.Error(), "code.type") || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("DecodeCatalog() error = %v", err)
	}
}

func TestDecodeCatalogRejectsAmbiguousObjectProperties(t *testing.T) {
	t.Parallel()
	_, err := DecodeCatalog("catalog.yaml", strings.NewReader(`format: 1
id: `+catalogID+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9}
description_length: 250
attributes:
  - id: `+attributeID+`
    name: Контакты
    title: {ru: Контакты}
    types: [{kind: string}]
table_parts:
  - id: `+tablePartID+`
    name: Контакты
    title: {ru: Контакты}
    attributes: []
`), metadataManifest())
	if err == nil || !strings.Contains(err.Error(), "conflicts with an attribute") {
		t.Fatalf("DecodeCatalog() error = %v", err)
	}
}

func metadataProject(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "project")
	if err := project.Initialize(root, metadataManifest()); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []Kind{ConstantKind, EnumerationKind, DefinedTypeKind, CatalogKind} {
		if err := os.MkdirAll(filepath.Join(root, "metadata", string(kind)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func metadataManifest() project.Project {
	return project.Project{
		Format: project.CurrentFormat,
		ID:     uuid.MustNew(), Name: "MetadataTest", Title: "Metadata Test", DefaultLanguage: "ru",
		Languages: []project.Language{{Name: "Русский", Title: "Русский", Code: "ru"}, {Name: "Українська", Title: "Українська", Code: "uk"}},
	}
}

func writeMetadata(t *testing.T, root string, kind Kind, id, content string) {
	t.Helper()
	path := filepath.Join(root, "metadata", string(kind), id+".yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMetadataEncodeIsStable(t *testing.T) {
	t.Parallel()
	value, err := DecodeConstant("constant.yaml", strings.NewReader(`format: 1
id: `+constantID+`
name: Значение
title: {uk: Значення, ru: Значение}
types: [{kind: string, length: 10}]
`), metadataManifest())
	if err != nil {
		t.Fatal(err)
	}
	var first, second bytes.Buffer
	if err := Encode(&first, value); err != nil {
		t.Fatal(err)
	}
	restored, err := DecodeConstant("constant.yaml", bytes.NewReader(first.Bytes()), metadataManifest())
	if err != nil {
		t.Fatal(err)
	}
	if err := Encode(&second, restored); err != nil || first.String() != second.String() {
		t.Fatalf("stable encode error=%v\nfirst=%s\nsecond=%s", err, first.String(), second.String())
	}
}
