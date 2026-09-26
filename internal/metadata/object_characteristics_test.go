package metadata

import (
	"strings"
	"testing"
)

const (
	characteristicKinds        = "c7000000-0000-4000-8000-000000000001"
	characteristicSets         = "c7000000-0000-4000-8000-000000000002"
	characteristicSetsPart     = "c7000000-0000-4000-8000-000000000003"
	characteristicSetsProperty = "c7000000-0000-4000-8000-000000000004"
	characteristicSetsName     = "c7000000-0000-4000-8000-000000000005"
	characteristicGoods        = "c7000000-0000-4000-8000-000000000010"
	characteristicGoodsPart    = "c7000000-0000-4000-8000-000000000011"
	characteristicGoodsKind    = "c7000000-0000-4000-8000-000000000012"
	characteristicGoodsValue   = "c7000000-0000-4000-8000-000000000013"
	characteristicGoodsNote    = "c7000000-0000-4000-8000-000000000014"
	characteristicInvoice      = "c7000000-0000-4000-8000-000000000020"
	characteristicRegister     = "c7000000-0000-4000-8000-000000000030"
	characteristicRegObject    = "c7000000-0000-4000-8000-000000000031"
	characteristicRegKind      = "c7000000-0000-4000-8000-000000000032"
	characteristicRegValue     = "c7000000-0000-4000-8000-000000000033"
	characteristicContactKinds = "c7000000-0000-4000-8000-000000000040"
	characteristicContactGroup = "c7000000-0000-4000-8000-000000000041"
	characteristicGoodsContact = "c7000000-0000-4000-8000-000000000042"
	characteristicMissing      = "c7000000-0000-4000-8000-0000000000ff"
)

// characteristicsProject writes the shape the reference configuration uses: a
// chart of characteristic types holding the properties themselves, a catalog
// of sets whose table part says which object gets which of them, and an
// information register holding what was filled in. The object under test keeps
// a table part of its own as the second place values may lie.
func characteristicsProject(t *testing.T) string {
	t.Helper()
	root := metadataProject(t)
	writeMetadata(t, root, ChartOfCharacteristicTypesKind, characteristicKinds, `format: 1
id: `+characteristicKinds+`
name: ВидыСвойств
title: {ru: Виды свойств}
code: {type: string, length: 9}
description_length: 150
value_type: [{kind: string, length: 100}]
`)
	writeMetadata(t, root, CatalogKind, characteristicSets, `format: 1
id: `+characteristicSets+`
name: НаборыСвойств
title: {ru: Наборы свойств}
code: {type: string, length: 9, auto: true}
description_length: 150
table_parts:
  - id: `+characteristicSetsPart+`
    name: Свойства
    title: {ru: Свойства}
    attributes:
      - {id: `+characteristicSetsProperty+`, name: Свойство, title: {ru: Свойство}, types: [{kind: chart-of-characteristic-types, reference: `+characteristicKinds+`}]}
      - {id: `+characteristicSetsName+`, name: ИмяНабора, title: {ru: Имя набора}, types: [{kind: string, length: 100}]}
`)
	writeMetadata(t, root, CatalogKind, characteristicContactKinds, `format: 1
id: `+characteristicContactKinds+`
name: ВидыКонтактнойИнформации
title: {ru: Виды контактной информации}
code: {type: string, length: 9, auto: true}
description_length: 150
hierarchy: {enabled: true, kind: folders-and-items}
`)
	writeMetadata(t, root, InformationRegisterKind, characteristicRegister, `format: 1
id: `+characteristicRegister+`
name: ДополнительныеСведения
title: {ru: Дополнительные сведения}
write_mode: independent
periodicity: none
dimensions:
  - id: `+characteristicRegObject+`
    name: Объект
    title: {ru: Объект}
    types:
      - {kind: catalog, reference: `+characteristicGoods+`}
      - {kind: document, reference: `+characteristicInvoice+`}
  - {id: `+characteristicRegKind+`, name: Свойство, title: {ru: Свойство}, types: [{kind: chart-of-characteristic-types, reference: `+characteristicKinds+`}]}
resources:
  - {id: `+characteristicRegValue+`, name: Значение, title: {ru: Значение}, types: [{kind: string, length: 100}]}
`)
	writeMetadata(t, root, DocumentKind, characteristicInvoice, `format: 1
id: `+characteristicInvoice+`
name: Счет
title: {ru: Счёт}
number: {type: string, length: 9, auto: true, periodicity: none}
characteristics:
  - types:
      table: {kind: catalogs, object: `+characteristicSets+`, table_part: `+characteristicSetsPart+`}
      key: {attribute: `+characteristicSetsProperty+`}
      filter: {attribute: `+characteristicSetsName+`}
      filter_value: {kind: string, data: Документ_Счет}
    values:
      table: {kind: information-registers, object: `+characteristicRegister+`}
      object: {attribute: `+characteristicRegObject+`}
      type: {attribute: `+characteristicRegKind+`}
      value: {attribute: `+characteristicRegValue+`}
`)
	writeMetadata(t, root, CatalogKind, characteristicGoods, goodsWithCharacteristics(`characteristics:
  - types:
      table: {kind: catalogs, object: `+characteristicSets+`, table_part: `+characteristicSetsPart+`}
      key: {attribute: `+characteristicSetsProperty+`}
      filter: {attribute: `+characteristicSetsName+`}
      filter_value: {kind: string, data: Справочник_Номенклатура}
      data_path: {standard: ref}
    values:
      table: {kind: catalogs, object: `+characteristicGoods+`, table_part: `+characteristicGoodsPart+`}
      object: {standard: ref}
      type: {attribute: `+characteristicGoodsKind+`}
      value: {attribute: `+characteristicGoodsValue+`}
  - types:
      table: {kind: catalogs, object: `+characteristicSets+`, table_part: `+characteristicSetsPart+`}
      key: {attribute: `+characteristicSetsProperty+`}
    values:
      table: {kind: information-registers, object: `+characteristicRegister+`}
      object: {attribute: `+characteristicRegObject+`}
      type: {attribute: `+characteristicRegKind+`}
      value: {attribute: `+characteristicRegValue+`}
`))
	return root
}

// goodsWithCharacteristics is the object under test with one block of
// characteristics or another written into it.
func goodsWithCharacteristics(characteristics string) string {
	return `format: 1
id: ` + characteristicGoods + `
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
table_parts:
  - id: ` + characteristicGoodsPart + `
    name: ДополнительныеРеквизиты
    title: {ru: Дополнительные реквизиты}
    attributes:
      - {id: ` + characteristicGoodsKind + `, name: Свойство, title: {ru: Свойство}, types: [{kind: chart-of-characteristic-types, reference: ` + characteristicKinds + `}]}
      - {id: ` + characteristicGoodsValue + `, name: Значение, title: {ru: Значение}, types: [{kind: string, length: 100}]}
      - {id: ` + characteristicGoodsContact + `, name: ВидКонтакта, title: {ru: Вид контакта}, types: [{kind: catalog, reference: ` + characteristicContactKinds + `}]}
attributes:
  - {id: ` + characteristicGoodsNote + `, name: Комментарий, title: {ru: Комментарий}, types: [{kind: string, length: 100}]}
` + characteristics
}

// An object says where its characteristics come from and where their values
// lie, and it says it about two pairs of tables at once. Nothing of either
// half may be dropped on the way in: a description read halfway reads no
// values at all.
func TestObjectKeepsWhereItsCharacteristicsComeFrom(t *testing.T) {
	t.Parallel()
	root := characteristicsProject(t)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	goods, ok := catalog.CatalogDefinition("Номенклатура")
	if !ok {
		t.Fatal("the catalog did not load")
	}
	if len(goods.Characteristics) != 2 {
		t.Fatalf("the characteristics were lost: %+v", goods.Characteristics)
	}
	own, register := goods.Characteristics[0], goods.Characteristics[1]
	switch {
	case own.Types.Table.TablePart == nil || own.Types.Table.TablePart.String() != characteristicSetsPart:
		t.Fatalf("the table the kinds are listed in lost half of its address: %+v", own.Types.Table)
	case own.Types.Filter == nil || own.Types.FilterValue == nil || own.Types.FilterValue.Data != "Справочник_Номенклатура":
		t.Fatalf("the filter that cuts the table down to this object was lost: %+v", own.Types)
	case own.Types.DataPath == nil || own.Types.DataPath.Standard != RefStandardField:
		t.Fatalf("the field the data path is built from was lost: %+v", own.Types)
	case own.Values.Object.Standard != RefStandardField:
		t.Fatalf("a standard field of the table of values was lost: %+v", own.Values)
	case register.Values.Table.Kind != InformationRegisterKind || register.Values.Table.TablePart != nil:
		t.Fatalf("the register holding the values was lost: %+v", register.Values.Table)
	case register.Types.Filter != nil || register.Types.FilterValue != nil:
		t.Fatalf("a filter appeared where the description has none: %+v", register.Types)
	}

	// A document keeps such a description the same way a catalog does: all
	// nine reference kinds carry one.
	invoice, ok := catalog.DocumentDefinition("Счет")
	if !ok || len(invoice.Characteristics) != 1 {
		t.Fatalf("the document lost its characteristics: found=%v %+v", ok, invoice.Characteristics)
	}

	goods.Characteristics[0].Types.Key.Standard = CodeStandardField
	again, _ := catalog.CatalogDefinition("Номенклатура")
	if again.Characteristics[0].Types.Key.Standard != "" {
		t.Fatal("a characteristic was handed out by reference")
	}
}

// A description pointing at a table or a field that is not there reads
// nothing, and the object then silently has no additional properties at all.
// Same for the two joins that hold the halves together: the wrong field in
// either of them is a pair of tables that never meets.
func TestCharacteristicsMustResolveAgainstTheConfiguration(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ body, want string }{
		"таблицы видов нет в конфигурации": {`characteristics:
  - types:
      table: {kind: catalogs, object: ` + characteristicMissing + `}
      key: {standard: ref}
    values:
      table: {kind: information-registers, object: ` + characteristicRegister + `}
      object: {attribute: ` + characteristicRegObject + `}
      type: {attribute: ` + characteristicRegKind + `}
      value: {attribute: ` + characteristicRegValue + `}`,
			"which is not in the configuration"},
		"табличной части нет у объекта": {`characteristics:
  - types:
      table: {kind: catalogs, object: ` + characteristicSets + `, table_part: ` + characteristicMissing + `}
      key: {attribute: ` + characteristicSetsProperty + `}
    values:
      table: {kind: information-registers, object: ` + characteristicRegister + `}
      object: {attribute: ` + characteristicRegObject + `}
      type: {attribute: ` + characteristicRegKind + `}
      value: {attribute: ` + characteristicRegValue + `}`,
			"which that object does not have"},
		"ключ указывает на чужой реквизит": {`characteristics:
  - types:
      table: {kind: catalogs, object: ` + characteristicSets + `, table_part: ` + characteristicSetsPart + `}
      key: {attribute: ` + characteristicGoodsKind + `}
    values:
      table: {kind: information-registers, object: ` + characteristicRegister + `}
      object: {attribute: ` + characteristicRegObject + `}
      type: {attribute: ` + characteristicRegKind + `}
      value: {attribute: ` + characteristicRegValue + `}`,
			"which that table does not have"},
		"стандартного поля у таблицы нет": {`characteristics:
  - types:
      table: {kind: catalogs, object: ` + characteristicSets + `, table_part: ` + characteristicSetsPart + `}
      key: {attribute: ` + characteristicSetsProperty + `}
      filter: {standard: parent}
      filter_value: {kind: string, data: Справочник_Номенклатура}
    values:
      table: {kind: information-registers, object: ` + characteristicRegister + `}
      object: {attribute: ` + characteristicRegObject + `}
      type: {attribute: ` + characteristicRegKind + `}
      value: {attribute: ` + characteristicRegValue + `}`,
			"names the standard field parent, which that table does not have"},
		"значение отбора не лезет в поле отбора": {`characteristics:
  - types:
      table: {kind: catalogs, object: ` + characteristicSets + `, table_part: ` + characteristicSetsPart + `}
      key: {attribute: ` + characteristicSetsProperty + `}
      filter: {attribute: ` + characteristicSetsName + `}
      filter_value: {kind: boolean, data: "true"}
    values:
      table: {kind: information-registers, object: ` + characteristicRegister + `}
      object: {attribute: ` + characteristicRegObject + `}
      type: {attribute: ` + characteristicRegKind + `}
      value: {attribute: ` + characteristicRegValue + `}`,
			"does not allow value kind boolean"},
		"значения хранятся не про этот объект": {`characteristics:
  - types:
      table: {kind: catalogs, object: ` + characteristicSets + `, table_part: ` + characteristicSetsPart + `}
      key: {attribute: ` + characteristicSetsProperty + `}
    values:
      table: {kind: information-registers, object: ` + characteristicRegister + `}
      object: {attribute: ` + characteristicRegKind + `}
      type: {attribute: ` + characteristicRegKind + `}
      value: {attribute: ` + characteristicRegValue + `}`,
			"keeps no values of it"},
		"вид значения не тот, которым ключуется таблица видов": {`characteristics:
  - types:
      table: {kind: catalogs, object: ` + characteristicSets + `, table_part: ` + characteristicSetsPart + `}
      key: {attribute: ` + characteristicSetsName + `}
    values:
      table: {kind: information-registers, object: ` + characteristicRegister + `}
      object: {attribute: ` + characteristicRegObject + `}
      type: {attribute: ` + characteristicRegKind + `}
      value: {attribute: ` + characteristicRegValue + `}`,
			"the two tables never join"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := characteristicsProject(t)
			writeMetadata(t, root, CatalogKind, characteristicGoods, goodsWithCharacteristics(broken.body+"\n"))
			_, err := Load(root)
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

// The shape of a description is checked before anything is resolved.
func TestBrokenCharacteristicDescriptionsAreRefused(t *testing.T) {
	t.Parallel()
	values := `    values:
      table: {kind: information-registers, object: ` + characteristicRegister + `}
      object: {attribute: ` + characteristicRegObject + `}
      type: {attribute: ` + characteristicRegKind + `}
      value: {attribute: ` + characteristicRegValue + `}`
	for name, broken := range map[string]struct{ body, want string }{
		"отбор без значения": {`characteristics:
  - types:
      table: {kind: catalogs, object: ` + characteristicSets + `, table_part: ` + characteristicSetsPart + `}
      key: {attribute: ` + characteristicSetsProperty + `}
      filter: {attribute: ` + characteristicSetsName + `}
` + values, "filter_value must be set beside the filter field"},
		"значение без отбора": {`characteristics:
  - types:
      table: {kind: catalogs, object: ` + characteristicSets + `, table_part: ` + characteristicSetsPart + `}
      key: {attribute: ` + characteristicSetsProperty + `}
      filter_value: {kind: string, data: Справочник_Номенклатура}
` + values, "filter must be set beside the filter value"},
		"поле названо дважды": {`characteristics:
  - types:
      table: {kind: catalogs, object: ` + characteristicSets + `, table_part: ` + characteristicSetsPart + `}
      key: {attribute: ` + characteristicSetsProperty + `, standard: ref}
` + values, "not both"},
		"поле не названо вовсе": {`characteristics:
  - types:
      table: {kind: catalogs, object: ` + characteristicSets + `, table_part: ` + characteristicSetsPart + `}
      key: {}
` + values, "must name an attribute or a standard field"},
		"стандартного поля такого нет": {`characteristics:
  - types:
      table: {kind: catalogs, object: ` + characteristicSets + `, table_part: ` + characteristicSetsPart + `}
      key: {standard: владелец}
` + values, "standard is not a standard field"},
		"характеристика не читается из отчёта": {`characteristics:
  - types:
      table: {kind: reports, object: ` + characteristicSets + `}
      key: {standard: ref}
` + values, "not a kind of object a characteristic is read from"},
		"одна характеристика дважды": {`characteristics:
  - types:
      table: {kind: catalogs, object: ` + characteristicSets + `, table_part: ` + characteristicSetsPart + `}
      key: {attribute: ` + characteristicSetsProperty + `}
` + values + `
  - types:
      table: {kind: catalogs, object: ` + characteristicSets + `, table_part: ` + characteristicSetsPart + `}
      key: {attribute: ` + characteristicSetsProperty + `}
` + values, "repeats a characteristic the object already has"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeCatalog("object.yaml", strings.NewReader(goodsWithCharacteristics(broken.body+"\n")), metadataConfiguration())
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

// The other shape the reference configuration uses: the kinds are a catalog of
// their own rather than a table part, keyed by its reference and cut down by
// its parent - so the filter value is a reference too, and it is checked
// against the field the way any value is checked against any field.
func TestCharacteristicsReadAWholeCatalogOfKinds(t *testing.T) {
	t.Parallel()
	root := characteristicsProject(t)
	writeMetadata(t, root, CatalogKind, characteristicGoods, goodsWithCharacteristics(`characteristics:
  - types:
      table: {kind: catalogs, object: `+characteristicContactKinds+`}
      key: {standard: ref}
      filter: {standard: parent}
      filter_value: {kind: catalog, object: `+characteristicContactKinds+`, data: `+characteristicContactGroup+`}
    values:
      table: {kind: catalogs, object: `+characteristicGoods+`, table_part: `+characteristicGoodsPart+`}
      object: {standard: ref}
      type: {attribute: `+characteristicGoodsContact+`}
      value: {attribute: `+characteristicGoodsValue+`}
`))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	goods, _ := catalog.CatalogDefinition("Номенклатура")
	if len(goods.Characteristics) != 1 {
		t.Fatalf("the characteristic was lost: %+v", goods.Characteristics)
	}
	value := goods.Characteristics[0].Types.FilterValue
	if value == nil || value.Kind != CatalogType || value.Object.String() != characteristicContactKinds {
		t.Fatalf("a reference filter value lost the object it points at: %+v", value)
	}
}
