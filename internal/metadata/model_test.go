package metadata

import (
	"bytes"
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
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
	characteristicsID     = "70000000-0000-4000-8000-000000000001"
	characteristicAttrID  = "70000000-0000-4000-8000-000000000002"
	characteristicPartID  = "70000000-0000-4000-8000-000000000003"
	accountsID            = "80000000-0000-4000-8000-000000000001"
	accountFlagID         = "80000000-0000-4000-8000-000000000002"
	extDimensionFlagID    = "80000000-0000-4000-8000-000000000003"
)

func TestDecodeMetadataIsStrictAndLocalized(t *testing.T) {
	t.Parallel()
	configuration := metadataConfiguration()
	constant, err := DecodeConstant("constant.yaml", strings.NewReader(`format: 1
id: `+constantID+`
name: ОсновнаяВалюта
title:
  ru: Основная валюта
  uk: Основна валюта
types:
  - kind: string
    length: 3
`), configuration)
	if err != nil {
		t.Fatal(err)
	}
	if constant.Title.Resolve("uk", configuration.DefaultLanguage, configuration.Languages) != "Основна валюта" || constant.Title.Resolve("en", configuration.DefaultLanguage, configuration.Languages) != "Основная валюта" {
		t.Fatalf("localized title fallback = %q", constant.Title.Resolve("en", configuration.DefaultLanguage, configuration.Languages))
	}
	_, err = DecodeConstant("constant.yaml", strings.NewReader(`format: 1
id: `+constantID+`
name: Invalid
title: {de: Titel}
types: [{kind: string}]
unknown: true
`), configuration)
	if err == nil || !strings.Contains(err.Error(), "field unknown") {
		t.Fatalf("strict decode error = %v", err)
	}
	_, err = DecodeConstant("constant.yaml", strings.NewReader(strings.Replace(`format: 1
id: `+constantID+`
name: Invalid
title: {ru: Заголовок}
types: [{kind: string}]
`, "format: 1", "format: 2", 1)), configuration)
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
	// A constant is found by the folder it lies in, so that is what its own
	// name has to agree with - the identifier inside is its own.
	t.Run("folder", func(t *testing.T) {
		root := metadataProject(t)
		directory := filepath.Join(root, "metadata", string(ConstantKind), "ДругоеИмя")
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "format: 1\nid: " + constantID + "\nname: Значение\ntitle: {ru: Значение}\ntypes: [{kind: boolean}]\n"
		if err := os.WriteFile(filepath.Join(directory, project.ObjectMetadataFile), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "lies in a folder called") {
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
	// A defined type does not hold another one - the designer does not offer
	// it - so a cycle of defined types cannot be written at all; it is refused
	// at the first defined type in another.
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
		if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "cannot stand in a defined type") {
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
	enumeration, err := catalog.NormalizeValue(constant, Value{Kind: EnumerationType, Data: enumValueID, Object: mustUUID(t, enumerationID)})
	if err != nil || enumeration.Data != enumValueID {
		t.Fatalf("enumeration = %+v, error=%v", enumeration, err)
	}
	if _, err := catalog.NormalizeValue(constant, Value{Kind: NumberType, Data: "1234.56"}); err == nil {
		t.Fatal("NormalizeValue accepted excess precision")
	}
	if _, err := catalog.NormalizeValue(constant, Value{Kind: EnumerationType, Data: constantID, Object: mustUUID(t, enumerationID)}); err == nil {
		t.Fatal("NormalizeValue accepted a foreign enumeration UUID")
	}
	// A value naming an enumeration the constant does not allow is refused by
	// the object it names, not by the value it carries.
	if _, err := catalog.NormalizeValue(constant, Value{Kind: EnumerationType, Data: enumValueID, Object: mustUUID(t, constantID)}); err == nil {
		t.Fatal("NormalizeValue accepted a value of a foreign enumeration")
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
description_length: 150
list:
  page_size: 50
  search_fields: [Description]
attributes:
  - id: `+attributeID+`
    name: ГоловнойКонтрагент
    title: {ru: Головной контрагент}
    types: [{kind: catalog, reference: `+catalogID+`}]
    indexing: index
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
	if !ok || definition.ID.String() != catalogID || len(definition.TableParts) != 1 || definition.List.PageSize != 50 || !slices.Equal(definition.List.SearchFields, []string{"Description"}) {
		t.Fatalf("catalog = %+v, found=%v", definition, ok)
	}
	definition.Attributes[0].Name = "Changed"
	definition.List.SearchFields[0] = "Changed"
	again, _ := catalog.CatalogDefinition("Контрагенты")
	if again.Attributes[0].Name != "ГоловнойКонтрагент" || again.List.SearchFields[0] != "Description" {
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
`), metadataConfiguration())
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
description_length: 150
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
`), metadataConfiguration())
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
description_length: 150
attributes:
  - id: `+attributeID+`
    name: ИНН
    title: {ru: ИНН}
    types: [{kind: string, length: 12}]
    fill_checking: show-error
predefined:
  - id: `+predefinedID.String()+`
    name: Основной
    description: Основной контрагент
    attributes:
      ИНН: {kind: string, data: "123456789012"}
`), metadataConfiguration())
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
description_length: 150
`)
	objectForm, listForm := uuid.MustNew(), uuid.MustNew()
	if err := os.MkdirAll(filepath.Join(root, "metadata", "documents", "Продажа", "forms"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Nothing declares a module: the file lying under the name of its role is
	// the whole of the declaration.
	for _, role := range []string{project.ObjectModuleFile, project.ManagerModuleFile} {
		relative, _ := project.ObjectModulePath("documents", "Продажа", role)
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(relative)), []byte("// module\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Two forms, three slots: the same list form is both the main list and the
	// main choice, the way the prototype's own catalog of users does it.
	for form, id := range map[string]uuid.UUID{"ФормаДокумента": objectForm, "ФормаСписка": listForm} {
		relative, _ := project.ObjectFormPath("documents", "Продажа", form)
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(path.Dir(relative))), 0o755); err != nil {
			t.Fatal(err)
		}
		body := "format: 1\nid: " + id.String() + "\nname: " + form + "\ntitle: {ru: " + form + "}\nkind: list\n"
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(relative)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeMetadata(t, root, DocumentKind, documentID, `format: 1
id: `+documentID+`
name: Продажа
title: {ru: Продажа}
number: {type: string, length: 11, auto: true, unique: true, periodicity: year}
posting: {allowed: true}
attributes:
  - id: `+docAttributeID+`
    name: Контрагент
    title: {ru: Контрагент}
    types: [{kind: catalog, reference: `+catalogID+`}]
forms:
  object: ФормаДокумента
  list: ФормаСписка
  choice: ФормаСписка
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	document, ok := catalog.DocumentDefinition("продажа")
	if !ok || document.ID.String() != documentID || document.Number.Periodicity != NumberPeriodYear ||
		document.Forms.Object != "ФормаДокумента" || document.Forms.List != document.Forms.Choice {
		t.Fatalf("document=%+v found=%v", document, ok)
	}
	// The slot carries a name; the identifier comes from the form itself, and
	// that is what the descriptor handed to the portal and ML App says.
	descriptor, err := catalog.DocumentForm("Продажа", ObjectForm, "ru")
	if err != nil || descriptor.Generated || descriptor.SourceID == nil || *descriptor.SourceID != objectForm {
		t.Fatalf("descriptor=%+v error=%v", descriptor, err)
	}
	if id, ok := catalog.ObjectFormID(DocumentKind, "ПРОДАЖА", "формасписка"); !ok || id != listForm {
		t.Fatalf("the list form was not found by name: %s %v", id, ok)
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
`), metadataConfiguration())
	if err == nil || !strings.Contains(err.Error(), "number.type") || !strings.Contains(err.Error(), "periodicity") || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("DecodeDocument() error = %v", err)
	}
}

func TestLoadInformationRegisterValidatesRecorderAndClonesFields(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9}
description_length: 150
`)
	writeMetadata(t, root, DocumentKind, documentID, `format: 1
id: `+documentID+`
name: УстановкаЦен
title: {ru: Установка цен}
number: {type: string, length: 11, periodicity: year}
movements: [`+informationRegisterID+`]
`)
	writeMetadata(t, root, InformationRegisterKind, informationRegisterID, `format: 1
id: `+informationRegisterID+`
name: Цены
title: {ru: Цены}
write_mode: recorder
periodicity: recorder-position
dimensions:
  - id: `+registerDimensionID+`
    name: Товар
    title: {ru: Товар}
    types: [{kind: catalog, reference: `+catalogID+`}]
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
	recorders := catalog.RegisterRecorders(register.ID)
	if !ok || register.Periodicity != InformationRegisterPeriodRecorderPosition || len(recorders) != 1 || recorders[0].String() != documentID {
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
dimensions:
  - id: `+registerDimensionID+`
    name: Период
    title: {ru: Период}
    types: [{kind: catalog, reference: `+catalogID+`}]
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
`), metadataConfiguration())
	if err == nil || !strings.Contains(err.Error(), "recorder-position") ||
		!strings.Contains(err.Error(), "reserved") || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("DecodeInformationRegister() error = %v", err)
	}
}

// The unique identifier is a type the prototype gives an attribute, and the
// configurations being moved declare it in hundreds of type descriptions; it
// was refused as "platform identity", and every one of them failed to load.
// Declared, it is a value column of its own - no foreign key, since it points
// at nothing - and the all-zero identifier is its empty value, not an error.
func TestAUniqueIdentifierIsATypeAnAttributeTakes(t *testing.T) {
	t.Parallel()
	register, err := DecodeInformationRegister("identity.yaml", strings.NewReader(`format: 1
id: `+informationRegisterID+`
name: Цены
title: {ru: Цены}
write_mode: independent
periodicity: none
dimensions:
  - id: `+registerDimensionID+`
    name: Объект
    title: {ru: Объект}
    types: [{kind: uuid}]
`), metadataConfiguration())
	if err != nil {
		t.Fatalf("DecodeInformationRegister() error = %v", err)
	}
	catalog := &Catalog{}
	storage, err := catalog.attributeStorage(register.Dimensions[0].Types)
	if err != nil {
		t.Fatal(err)
	}
	if storage.sqlType != "uuid" || storage.composite || storage.referenceObject != nil {
		t.Fatalf("storage = %+v, want a plain uuid column", storage)
	}
	for _, text := range []string{"00000000-0000-0000-0000-000000000000", "6F9619FF-8B86-D011-B42D-00CF4FC964FF"} {
		value, valid, reason := catalog.normalizeAs(Value{Kind: UUIDType, Data: text}, Type{Kind: UUIDType})
		if !valid || value.Data != strings.ToLower(text) {
			t.Fatalf("normalize %s = %+v, %v, %s", text, value, valid, reason)
		}
	}
	if _, valid, _ := catalog.normalizeAs(Value{Kind: UUIDType, Data: "not-an-identifier"}, Type{Kind: UUIDType}); valid {
		t.Fatal("a text that is no identifier was taken")
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
    types: [{kind: catalog, reference: `+catalogID+`}]
`), metadataConfiguration())
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
	writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9}
description_length: 150
`)
	writeMetadata(t, root, DocumentKind, recorder.String(), `format: 1
id: `+recorder.String()+`
name: Продажа
title: {ru: Продажа}
number: {type: string, length: 11, periodicity: year}
movements: [`+register.String()+`]
`)
	writeMetadata(t, root, AccumulationRegisterKind, register.String(), `format: 1
id: `+register.String()+`
name: Продажи
title: {ru: Продажи}
kind: turnover
dimensions:
  - id: `+dimension.String()+`
    name: Товар
    title: {ru: Товар}
    types: [{kind: catalog, reference: `+catalogID+`}]
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
	if !ok || value.Kind != AccumulationRegisterTurnover || catalog.RegisterRecorders(value.ID)[0] != recorder {
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
	if err := project.Initialize(root, metadataConfiguration()); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []Kind{ConstantKind, EnumerationKind, DefinedTypeKind, CatalogKind, DocumentKind, InformationRegisterKind, AccumulationRegisterKind, ChartOfCharacteristicTypesKind, ChartOfAccountsKind, ChartOfCalculationTypesKind, BusinessProcessKind, TaskKind, ExchangePlanKind, NumeratorKind, SequenceKind, DocumentJournalKind, AccountingRegisterKind, CalculationRegisterKind, ReportKind, DataProcessorKind, FunctionalOptionKind, FunctionalOptionParameterKind, FilterCriterionKind, SettingsStorageKind, ScheduledJobKind} {
		if err := os.MkdirAll(filepath.Join(root, "metadata", string(kind)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func metadataConfiguration() project.Project {
	return project.Project{
		Format: project.CurrentFormat,
		ID:     uuid.MustNew(), Name: "MetadataTest", Title: project.LocalizedText{"ru": "Metadata Test"}, DefaultLanguage: "ru",
		Languages: []project.Language{{ID: uuid.MustNew(), Name: "Русский", Title: project.LocalizedText{"ru": "Русский"}, Code: "ru"}, {ID: uuid.MustNew(), Name: "Українська", Title: project.LocalizedText{"uk": "Українська"}, Code: "uk"}},
	}
}

// writeMetadata writes one metadata object's source file, using the
// per-object folder layout (metadata/<kind>/<name>/object.yaml) for catalogs,
// documents and registers, and the flat layout for every other kind.
//
// The folder is named by the object, so the helper reads the name out of what
// it is about to write. A body with no name at all keeps the identifier, which
// is how a test writes a folder the loader will refuse.
func writeMetadata(t *testing.T, root string, kind Kind, id, content string) {
	t.Helper()
	path := filepath.Join(root, "metadata", string(kind), id+".yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	named, keepsFolder := project.NamedFolderFiles(string(kind))
	if slices.Contains(project.ObjectFolderKinds(), string(kind)) ||
		(keepsFolder && slices.Contains(named, project.ObjectMetadataFile)) {
		directory := filepath.Join(root, "metadata", string(kind), objectFolderName(content, id))
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		path = filepath.Join(directory, "object.yaml")
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// objectFolderName reads the object's name out of the body being written.
func objectFolderName(content, fallback string) string {
	for _, line := range strings.Split(content, "\n") {
		if name, found := strings.CutPrefix(line, "name: "); found {
			return strings.TrimSpace(name)
		}
	}
	return fallback
}

func TestMetadataEncodeIsStable(t *testing.T) {
	t.Parallel()
	value, err := DecodeConstant("constant.yaml", strings.NewReader(`format: 1
id: `+constantID+`
name: Значение
title: {uk: Значення, ru: Значение}
types: [{kind: string, length: 10}]
`), metadataConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	var first, second bytes.Buffer
	if err := Encode(&first, value); err != nil {
		t.Fatal(err)
	}
	restored, err := DecodeConstant("constant.yaml", bytes.NewReader(first.Bytes()), metadataConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	if err := Encode(&second, restored); err != nil || first.String() != second.String() {
		t.Fatalf("stable encode error=%v\nfirst=%s\nsecond=%s", err, first.String(), second.String())
	}
}

// What a reader sees when their own language is missing is a decision, and it
// belongs to the project: its default language, not whichever translation
// happens to be stored first.
func TestLocalizedTextFallsBackThroughTheProjectDefault(t *testing.T) {
	t.Parallel()
	configured := []project.Language{
		{ID: uuid.MustNew(), Name: "Українська", Title: project.LocalizedText{"uk": "Українська"}, Code: "uk"},
		{ID: uuid.MustNew(), Name: "Русский", Title: project.LocalizedText{"ru": "Русский"}, Code: "ru"},
		{ID: uuid.MustNew(), Name: "English", Title: project.LocalizedText{"en": "English"}, Code: "en"},
	}
	text := LocalizedText{"ru": "Товары", "uk": "Товари"}
	if value := text.Resolve("en", "ru", configured); value != "Товары" {
		t.Fatalf("missing translation resolved to %q, want the project default", value)
	}
	if value := text.Resolve("uk", "ru", configured); value != "Товари" {
		t.Fatalf("own language resolved to %q", value)
	}
	// Без основного языка остаётся прежний порядок: первый настроенный.
	if value := text.Resolve("en", "", configured); value != "Товари" {
		t.Fatalf("without a default the configured order decides, got %q", value)
	}
	// Регион — уточнение языка, а не другой язык, в обе стороны.
	if value := text.Resolve("uk-UA", "ru", configured); value != "Товари" {
		t.Fatalf("regional code resolved to %q", value)
	}
	regional := LocalizedText{"uk-UA": "Товари", "ru": "Товары"}
	if value := regional.Resolve("uk", "ru", configured); value != "Товари" {
		t.Fatalf("base code did not reach the regional translation, got %q", value)
	}
	// Ничего настроенного и ничего по умолчанию — хоть что-то непустое.
	if value := (LocalizedText{"de": "Waren"}).Resolve("en", "ru", configured); value != "Waren" {
		t.Fatalf("last resort resolved to %q", value)
	}
	if value := (LocalizedText{}).Resolve("en", "ru", configured); value != "" {
		t.Fatalf("empty text resolved to %q", value)
	}
}

// Qualifiers are not decoration: each one changes what a value may be or what
// it means, and a qualifier on the wrong kind is a mistake worth naming.
func TestAttributeQualifiersConstrainValuesAndKinds(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		types string
		want  string
	}{
		{"date parts on a number", "[{kind: number, precision: 5, scale: 0, date_parts: date}]", "date_parts is allowed for dates only"},
		{"fixed length on a number", "[{kind: number, precision: 5, scale: 0, fixed_length: true}]", "fixed_length is allowed for strings only"},
		{"non-negative on a string", "[{kind: string, length: 5, non_negative: true}]", "non_negative is allowed for numbers only"},
		{"fixed length without a length", "[{kind: string, fixed_length: true}]", "fixed_length requires a length"},
		{"unknown date parts", "[{kind: date, date_parts: quarter}]", "date_parts must be date, time or date-time"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeInformationRegister("qualifiers.yaml", strings.NewReader(`format: 1
id: `+informationRegisterID+`
name: Цены
title: {ru: Цены}
write_mode: independent
periodicity: none
dimensions:
  - id: `+registerDimensionID+`
    name: Значение
    title: {ru: Значение}
    types: `+test.types+`
`), metadataConfiguration())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

// A date attribute that is about a day must not carry a time nobody entered:
// every later comparison would trip over it.
func TestDatePartsAndSignAreEnforcedOnWrite(t *testing.T) {
	t.Parallel()
	catalog := &Catalog{}
	day, ok, reason := catalog.normalizeAs(Value{Kind: DateType, Data: "2026-09-23T14:35:07Z"}, Type{Kind: DateType, DateParts: DateOnlyParts})
	if !ok || day.Data != "2026-09-23T00:00:00Z" {
		t.Fatalf("date-only = %+v ok=%v reason=%q", day, ok, reason)
	}
	clock, ok, _ := catalog.normalizeAs(Value{Kind: DateType, Data: "2026-09-23T14:35:07Z"}, Type{Kind: DateType, DateParts: TimeOnlyParts})
	if !ok || clock.Data != "0001-01-01T14:35:07Z" {
		t.Fatalf("time-only = %+v", clock)
	}
	if _, ok, reason := catalog.normalizeAs(Value{Kind: NumberType, Data: "-1"}, Type{Kind: NumberType, Precision: 5, Scale: 0, NonNegative: true}); ok {
		t.Fatalf("negative accepted for a non-negative number: %q", reason)
	}
}

// Expansion drops duplicates, and a qualifier is what tells two otherwise
// identical types apart. Leaving qualifiers out of that comparison loses a
// member of a composite type without a word.
func TestExpansionKeepsTypesThatDifferOnlyByQualifier(t *testing.T) {
	t.Parallel()
	catalog := &Catalog{}
	expanded, err := catalog.expandTypes([]Type{
		{Kind: DateType, DateParts: DateOnlyParts},
		{Kind: DateType, DateParts: TimeOnlyParts},
		{Kind: StringType, Length: 10},
		{Kind: StringType, Length: 10, FixedLength: true},
		{Kind: StringType, Length: 10},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(expanded) != 4 {
		t.Fatalf("expanded to %d types, want 4: %+v", len(expanded), expanded)
	}
}

// A hierarchy setting that depends on another one is carried even when that
// one is off: the prototype writes every setting of the dialog on every object,
// and the configurations being moved carry a kind of hierarchy on 745 flat
// catalogs, a level count nobody limits on 1066, folders on top over a
// hierarchy of items on 63. What is refused is a value that is wrong on its
// own, or a hierarchy that is on and says nothing it needs.
func TestHierarchySettingsAreCarriedAsThePrototypeWritesThem(t *testing.T) {
	t.Parallel()
	load := func(t *testing.T, hierarchy string) error {
		root := metadataProject(t)
		writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: Товары
title: {ru: Товары}
code: {type: string, length: 9}
description_length: 100
`+hierarchy+`
`)
		_, err := Load(root)
		return err
	}
	for name, hierarchy := range map[string]string{
		"вид иерархии без самой иерархии": "hierarchy: {kind: folders-and-items, level_count: 2, folders_on_top: true}",
		"группы сверху без групп":         "hierarchy: {enabled: true, kind: items, folders_on_top: true}",
		"уровни без ограничения":          "hierarchy: {enabled: true, kind: items, level_count: 2}",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := load(t, hierarchy); err != nil {
				t.Fatalf("the shape the prototype writes was refused: %v", err)
			}
		})
	}
	for name, refused := range map[string]struct{ hierarchy, message string }{
		"иерархия без вида":                   {"hierarchy: {enabled: true}", "hierarchy.kind is required"},
		"ограничение без числа уровней":       {"hierarchy: {enabled: true, kind: items, limit_levels: true}", "level_count must be 1.."},
		"неизвестный вид":                     {"hierarchy: {enabled: true, kind: деревья}", "hierarchy.kind must be"},
		"неизвестный вид без иерархии":        {"hierarchy: {kind: деревья}", "hierarchy.kind must be"},
		"отрицательное число уровней":         {"hierarchy: {level_count: -1}", "level_count must be 0.."},
		"отрицательное число без ограничения": {"hierarchy: {enabled: true, kind: items, level_count: -1}", "level_count must be 0.."},
		// The kind of a hierarchy that is off is carried, and it is not a
		// hierarchy: a predefined folder on such a flat object is still refused.
		"группа при виде без иерархии": {"hierarchy: {kind: folders-and-items}\npredefined:\n  - {id: 6f0a0000-0000-4000-8000-000000000001, name: Группа, code: \"001\", is_folder: true}",
			"is_folder needs a hierarchy of folders and items"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := load(t, refused.hierarchy); err == nil || !strings.Contains(err.Error(), refused.message) {
				t.Fatalf("error = %v, expected it to name %q", err, refused.message)
			}
		})
	}

	// The range a code is numbered within belongs to the code, not to the
	// hierarchy, so it is refused there.
	_, err := DecodeCatalog("object.yaml", strings.NewReader(`format: 1
id: `+catalogID+`
name: Товары
title: {ru: Товары}
code: {type: string, length: 9, series: своя}
description_length: 100
`), metadataConfiguration())
	if err == nil || !strings.Contains(err.Error(), "code.series must be") {
		t.Fatalf("an unknown code series was accepted: %v", err)
	}
}

// A hierarchical object stores the parent, and where folders exist it stores
// which rows are folders. Without the columns the tree arrives flat, which is
// the silent loss the whole block exists to prevent.
func TestHierarchyReachesStorage(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true, series: within-subordination}
description_length: 100
hierarchy: {enabled: true, kind: folders-and-items, folders_on_top: true, limit_levels: true, level_count: 3}
predefined:
  - id: 40000000-0000-4000-8000-000000000020
    name: Поставщики
    description: Поставщики
    is_folder: true
  - id: 40000000-0000-4000-8000-000000000021
    name: ОсновнойПоставщик
    description: Основной поставщик
    parent: Поставщики
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	item, _ := catalog.CatalogDefinition("Контрагенты")
	if !item.Hierarchy.Enabled || item.Hierarchy.Kind != FoldersAndItemsHierarchy || item.Hierarchy.LevelCount != 3 {
		t.Fatalf("hierarchy did not survive loading: %+v", item.Hierarchy)
	}
	if !item.Predefined[0].IsFolder || item.Predefined[1].Parent != "Поставщики" {
		t.Fatalf("predefined tree was flattened: %+v", item.Predefined)
	}
	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	table, err := PhysicalCatalogTable(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	var columns, keys int
	for _, candidate := range schema.Tables {
		if candidate.Name != table {
			continue
		}
		for _, column := range candidate.Columns {
			if column.Name == "parent" || column.Name == "is_folder" {
				columns++
			}
		}
		for _, constraint := range candidate.Constraints {
			if constraint.Type == "foreign_key" && strings.Contains(constraint.Definition, "(parent)") {
				keys++
			}
		}
	}
	if columns != 2 || keys != 1 {
		t.Fatalf("hierarchy columns = %d, parent keys = %d", columns, keys)
	}
}

// The predefined tree is held to the limit of levels by the rule a written row
// is: the level of items is a level, so with two levels a folder at the top
// holds items and a folder in a folder is refused, and with one level a folder
// has no room at all - the prototype, checked on the platform 01.10.2026. In a
// hierarchy of items an item is its own folder and the limit counts rows: with
// two levels an item at the top holds one level and that is all. Read without
// the check, such a tree was accepted and refused by the base when its items
// were created.
func TestPredefinedTreeKeepsToTheLevels(t *testing.T) {
	t.Parallel()
	item := func(name, parent string, folder bool) PredefinedCatalogItem {
		return PredefinedCatalogItem{ID: uuid.MustNew(), Name: name, Parent: parent, IsFolder: folder}
	}
	folders := func(levels int) Hierarchy {
		return Hierarchy{Enabled: true, Kind: FoldersAndItemsHierarchy, LimitLevels: true, LevelCount: levels}
	}
	items := func(levels int) Hierarchy {
		return Hierarchy{Enabled: true, Kind: ItemsHierarchy, LimitLevels: true, LevelCount: levels}
	}
	for name, testCase := range map[string]struct {
		hierarchy Hierarchy
		tree      []PredefinedCatalogItem
		refused   bool
	}{
		"группа с элементами при двух уровнях": {folders(2), []PredefinedCatalogItem{item("Группа", "", true), item("Элемент", "Группа", false)}, false},
		"группа в группе при двух уровнях":     {folders(2), []PredefinedCatalogItem{item("Группа", "", true), item("Вложенная", "Группа", true)}, true},
		"группа при одном уровне":              {folders(1), []PredefinedCatalogItem{item("Группа", "", true)}, true},
		"элемент при одном уровне":             {folders(1), []PredefinedCatalogItem{item("Элемент", "", false)}, false},
		"элемент в элементе при двух уровнях":  {items(2), []PredefinedCatalogItem{item("Верх", "", false), item("Низ", "Верх", false)}, false},
		"три элемента вглубь при двух уровнях": {items(2), []PredefinedCatalogItem{item("Верх", "", false), item("Середина", "Верх", false), item("Низ", "Середина", false)}, true},
		"без ограничения":                      {Hierarchy{Enabled: true, Kind: FoldersAndItemsHierarchy}, []PredefinedCatalogItem{item("А", "", true), item("Б", "А", true), item("В", "Б", true)}, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			issues := validatePredefinedTree(testCase.tree, testCase.hierarchy)
			if refused := len(issues) != 0; refused != testCase.refused {
				t.Fatalf("refused = %v, want %v: %v", refused, testCase.refused, issues)
			}
			if testCase.refused && !strings.Contains(strings.Join(issues, "; "), "levels counting the level of items") {
				t.Fatalf("refused for another reason: %v", issues)
			}
		})
	}
}
