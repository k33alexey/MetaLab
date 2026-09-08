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
	constantID            = "10000000-0000-4000-8000-000000000001"
	enumerationID         = "20000000-0000-4000-8000-000000000001"
	enumValueID           = "20000000-0000-4000-8000-000000000002"
	definedTypeID         = "30000000-0000-4000-8000-000000000001"
	catalogID             = "40000000-0000-4000-8000-000000000001"
	attributeID           = "40000000-0000-4000-8000-000000000002"
	tablePartID           = "40000000-0000-4000-8000-000000000003"
	partFieldID           = "40000000-0000-4000-8000-000000000004"
	documentID            = "50000000-0000-4000-8000-000000000001"
	docAttributeID        = "50000000-0000-4000-8000-000000000002"
	informationRegisterID = "60000000-0000-4000-8000-000000000001"
	registerDimensionID   = "60000000-0000-4000-8000-000000000002"
	registerResourceID    = "60000000-0000-4000-8000-000000000003"
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

func TestDecodeCatalogPredefinedItems(t *testing.T) {
	t.Parallel()
	predefinedID := uuid.MustNew()
	definition, err := DecodeCatalog("catalog.yaml", strings.NewReader(`format: 1
id: `+catalogID+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true, unique: true}
description_length: 250
attributes:
  - id: `+attributeID+`
    name: ИНН
    title: {ru: ИНН}
    types: [{kind: string, length: 12}]
    required: true
predefined:
  - id: `+predefinedID.String()+`
    name: Основной
    description: Основной контрагент
    attributes:
      ИНН: {kind: string, data: "123456789012"}
`), metadataManifest())
	if err != nil {
		t.Fatal(err)
	}
	item, ok := definition.PredefinedItem("основной")
	if !ok || item.ID != predefinedID || item.Attributes["ИНН"].Data != "123456789012" {
		t.Fatalf("predefined=%+v found=%v", item, ok)
	}
	item.Attributes["ИНН"] = Value{Kind: StringType, Data: "changed"}
	again, _ := definition.PredefinedItem("Основной")
	if again.Attributes["ИНН"].Data != "123456789012" {
		t.Fatal("predefined lookup exposed mutable attributes")
	}
}

func TestLoadDocumentWithFormsAndReferences(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9}
description_length: 250
`)
	objectModule, managerModule := uuid.MustNew(), uuid.MustNew()
	objectForm, listForm, choiceForm := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	for _, id := range []uuid.UUID{objectModule, managerModule} {
		relative, _ := project.ModulePath(id)
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(relative)), []byte("// module\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []uuid.UUID{objectForm, listForm, choiceForm} {
		relative, _ := project.FormPath(id)
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(relative)), []byte("format: 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeMetadata(t, root, DocumentKind, documentID, `format: 1
id: `+documentID+`
name: Продажа
title: {ru: Продажа}
number: {type: string, length: 11, auto: true, unique: true, periodicity: year}
posting: true
attributes:
  - id: `+docAttributeID+`
    name: Контрагент
    title: {ru: Контрагент}
    types: [{kind: catalog, reference: `+catalogID+`}]
object_module: `+objectModule.String()+`
manager_module: `+managerModule.String()+`
forms:
  object: `+objectForm.String()+`
  list: `+listForm.String()+`
  choice: `+choiceForm.String()+`
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	document, ok := catalog.DocumentDefinition("продажа")
	if !ok || document.ID.String() != documentID || document.Number.Periodicity != NumberPeriodYear || document.Forms.Object == nil || *document.Forms.Object != objectForm {
		t.Fatalf("document=%+v found=%v", document, ok)
	}
	*document.Forms.Object = uuid.MustNew()
	again, _ := catalog.DocumentDefinition("Продажа")
	if *again.Forms.Object != objectForm {
		t.Fatal("document lookup exposed mutable form identity")
	}
}

func TestDecodeDocumentRejectsInvalidNumberAndReservedProperty(t *testing.T) {
	t.Parallel()
	_, err := DecodeDocument("document.yaml", strings.NewReader(`format: 1
id: `+documentID+`
name: Продажа
title: {ru: Продажа}
number: {type: boolean, length: 0, periodicity: week}
attributes:
  - id: `+docAttributeID+`
    name: Дата
    title: {ru: Дата}
    types: [{kind: date}]
`), metadataManifest())
	if err == nil || !strings.Contains(err.Error(), "number.type") || !strings.Contains(err.Error(), "periodicity") || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("DecodeDocument() error = %v", err)
	}
}

func TestLoadInformationRegisterValidatesRecorderAndClonesFields(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, DocumentKind, documentID, `format: 1
id: `+documentID+`
name: УстановкаЦен
title: {ru: Установка цен}
number: {type: string, length: 11, periodicity: year}
`)
	writeMetadata(t, root, InformationRegisterKind, informationRegisterID, `format: 1
id: `+informationRegisterID+`
name: Цены
title: {ru: Цены}
write_mode: recorder
periodicity: recorder-position
recorders: [`+documentID+`]
dimensions:
  - id: `+registerDimensionID+`
    name: Товар
    title: {ru: Товар}
    types: [{kind: uuid}]
resources:
  - id: `+registerResourceID+`
    name: Цена
    title: {ru: Цена}
    types: [{kind: number, precision: 15, scale: 2}]
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	register, ok := catalog.InformationRegisterDefinition("цены")
	if !ok || register.Periodicity != InformationRegisterPeriodRecorderPosition || register.Recorders[0].String() != documentID {
		t.Fatalf("register=%+v found=%v", register, ok)
	}
	register.Resources[0].Name = "Изменено"
	again, _ := catalog.InformationRegisterDefinition("Цены")
	if again.Resources[0].Name != "Цена" || len(catalog.InformationRegisterIDs()) != 1 {
		t.Fatal("information register lookup exposed mutable metadata")
	}
}

func TestDecodeInformationRegisterRejectsInvalidSemantics(t *testing.T) {
	t.Parallel()
	_, err := DecodeInformationRegister("register.yaml", strings.NewReader(`format: 1
id: `+informationRegisterID+`
name: Цены
title: {ru: Цены}
write_mode: independent
periodicity: recorder-position
recorders: [`+documentID+`]
dimensions:
  - id: `+registerDimensionID+`
    name: Период
    title: {ru: Период}
    types: [{kind: uuid}]
resources:
  - id: `+registerResourceID+`
    name: Значение
    title: {ru: Значение}
    types: [{kind: string}]
attributes:
  - id: `+uuid.MustNew().String()+`
    name: Значение
    title: {ru: Значение}
    types: [{kind: string}]
`), metadataManifest())
	if err == nil || !strings.Contains(err.Error(), "recorder-position") || !strings.Contains(err.Error(), "only allowed") ||
		!strings.Contains(err.Error(), "reserved") || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("DecodeInformationRegister() error = %v", err)
	}
}

func TestDecodeInformationRegisterAllowsDimensionOnly(t *testing.T) {
	t.Parallel()
	value, err := DecodeInformationRegister("dimension-only.yaml", strings.NewReader(`format: 1
id: `+informationRegisterID+`
name: Связи
title: {ru: Связи}
write_mode: independent
periodicity: none
dimensions:
  - id: `+registerDimensionID+`
    name: Объект
    title: {ru: Объект}
    types: [{kind: uuid}]
`), metadataManifest())
	if err != nil || len(value.Dimensions) != 1 || len(value.Resources) != 0 {
		t.Fatalf("dimension-only register=%+v error=%v", value, err)
	}
}

func TestLoadAccumulationRegisterValidatesRecorderAndClonesFields(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	recorder, register, dimension, resource, numericType := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	writeMetadata(t, root, DefinedTypeKind, numericType.String(), `format: 1
id: `+numericType.String()+`
name: ДенежнаяСумма
title: {ru: Денежная сумма}
types: [{kind: number, precision: 15, scale: 2}]
`)
	writeMetadata(t, root, DocumentKind, recorder.String(), `format: 1
id: `+recorder.String()+`
name: Продажа
title: {ru: Продажа}
number: {type: string, length: 11, periodicity: year}
`)
	writeMetadata(t, root, AccumulationRegisterKind, register.String(), `format: 1
id: `+register.String()+`
name: Продажи
title: {ru: Продажи}
kind: turnover
recorders: [`+recorder.String()+`]
dimensions:
  - id: `+dimension.String()+`
    name: Товар
    title: {ru: Товар}
    types: [{kind: uuid}]
resources:
  - id: `+resource.String()+`
    name: Сумма
    title: {ru: Сумма}
    types: [{kind: defined-type, reference: `+numericType.String()+`}]
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := catalog.AccumulationRegisterDefinition("продажи")
	if !ok || value.Kind != AccumulationRegisterTurnover || value.Recorders[0] != recorder {
		t.Fatalf("register=%+v found=%v", value, ok)
	}
	value.Resources[0].Name = "Изменено"
	again, _ := catalog.AccumulationRegisterDefinition("Продажи")
	if again.Resources[0].Name != "Сумма" || len(catalog.AccumulationRegisterIDs()) != 1 {
		t.Fatal("accumulation register lookup exposed mutable metadata")
	}
}

func metadataProject(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "project")
	if err := project.Initialize(root, metadataManifest()); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []Kind{ConstantKind, EnumerationKind, DefinedTypeKind, CatalogKind, DocumentKind, InformationRegisterKind, AccumulationRegisterKind} {
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
