package metadata

import (
	"strings"
	"testing"
)

const (
	presentationCatalog  = "c8000000-0000-4000-8000-000000000001"
	presentationPartners = "c8000000-0000-4000-8000-000000000002"
	presentationAmount   = "c8000000-0000-4000-8000-000000000010"
	presentationSecret   = "c8000000-0000-4000-8000-000000000011"
	presentationPartner  = "c8000000-0000-4000-8000-000000000012"
	presentationContract = "c8000000-0000-4000-8000-000000000013"
	presentationPart     = "c8000000-0000-4000-8000-000000000020"
	presentationLine     = "c8000000-0000-4000-8000-000000000021"
	presentationMissing  = "c8000000-0000-4000-8000-0000000000ff"
)

// presentationOrder is a catalog whose fields carry the whole set: a number
// shown one way and entered another, a password, and a field picked from a
// list narrowed by the field above it.
func presentationOrder(attributes string) string {
	return `format: 1
id: ` + presentationCatalog + `
name: Заказы
title: {ru: Заказы}
code: {type: string, length: 9, auto: true}
description_length: 150
` + attributes
}

func TestFieldKeepsHowItIsShownAndPicked(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	catalogNamed(t, root, presentationPartners, "Партнеры")
	writeMetadata(t, root, CatalogKind, presentationCatalog, presentationOrder(`attributes:
  - id: `+presentationAmount+`
    name: Сумма
    title: {ru: Сумма}
    comment: Сумма заказа с налогами
    types: [{kind: number, precision: 15, scale: 2}]
    presentation:
      format: {ru: "ЧДЦ=2"}
      edit_format: {ru: "ЧДЦ=2; ЧН=0"}
      tooltip: {ru: Сумма заказа}
      mark_negatives: true
      min_value: {kind: number, data: "0"}
      max_value: {kind: number, data: "1000000"}
  - id: `+presentationSecret+`
    name: Пароль
    title: {ru: Пароль}
    types: [{kind: string, length: 100}]
    presentation: {mask: "999-999", password: true, multi_line: false, extended_edit: true}
  - id: `+presentationPartner+`
    name: Партнер
    title: {ru: Партнёр}
    types: [{kind: catalog, reference: `+presentationPartners+`}]
    choice:
      quick_choice: use
      create_on_input: dont-use
      history_on_input: auto
      folders_and_items: items
      parameters:
        - {name: Отбор.Действует, values: [{kind: boolean, data: "true"}]}
  - id: `+presentationContract+`
    name: Договор
    title: {ru: Договор}
    types: [{kind: catalog, reference: `+presentationPartners+`}]
    choice:
      parameter_links:
        - {name: Отбор.Владелец, source: {attribute: `+presentationPartner+`}, change: clear}
      link_by_type: {source: {attribute: `+presentationPartner+`}, item: 0}
table_parts:
  - id: `+presentationPart+`
    name: Строки
    title: {ru: Строки}
    attributes:
      - id: `+presentationLine+`
        name: Номенклатура
        title: {ru: Номенклатура}
        types: [{kind: catalog, reference: `+presentationPartners+`}]
        choice:
          parameter_links:
            - {name: Отбор.Владелец, source: {attribute: `+presentationPartner+`}}
`))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	orders, ok := catalog.CatalogDefinition("Заказы")
	if !ok || len(orders.Attributes) != 4 {
		t.Fatalf("the catalog did not load: found=%v %+v", ok, orders.Attributes)
	}
	amount, secret, partner, contract := orders.Attributes[0], orders.Attributes[1], orders.Attributes[2], orders.Attributes[3]
	switch {
	case amount.Comment == "":
		t.Fatalf("the comment was lost: %+v", amount)
	case len(amount.Presentation.Format) == 0 || len(amount.Presentation.EditFormat) == 0:
		t.Fatalf("how the value is shown and how it is entered are two settings: %+v", amount.Presentation)
	case len(amount.Presentation.ToolTip) == 0 || !amount.Presentation.MarkNegatives:
		t.Fatalf("the presentation was lost: %+v", amount.Presentation)
	case amount.Presentation.MinValue == nil || amount.Presentation.MaxValue == nil:
		t.Fatalf("the bounds were lost: %+v", amount.Presentation)
	case secret.Presentation.Mask != "999-999" || !secret.Presentation.Password || !secret.Presentation.ExtendedEdit:
		t.Fatalf("how the field is entered was lost: %+v", secret.Presentation)
	case partner.Choice.QuickChoice != UsageUse || partner.Choice.CreateOnInput != UsageDontUse:
		t.Fatalf("the three-valued switches were lost: %+v", partner.Choice)
	case partner.Choice.FoldersAndItems != ChoiceTargetItems:
		t.Fatalf("what a choice may land on was lost: %+v", partner.Choice)
	case len(partner.Choice.Parameters) != 1 || len(partner.Choice.Parameters[0].Values) != 1:
		t.Fatalf("the fixed choice parameter was lost: %+v", partner.Choice)
	case len(contract.Choice.ParameterLinks) != 1 || contract.Choice.ParameterLinks[0].Change != ValueChangeClear:
		t.Fatalf("the link to another field was lost: %+v", contract.Choice)
	case contract.Choice.LinkByType == nil || contract.Choice.LinkByType.Source.Attribute.String() != presentationPartner:
		t.Fatalf("the link by type was lost: %+v", contract.Choice)
	case len(orders.TableParts[0].Attributes[0].Choice.ParameterLinks) != 1:
		t.Fatalf("a field of a table part lost its link: %+v", orders.TableParts[0].Attributes[0])
	}

	orders.Attributes[0].Presentation.MinValue.Data = "999"
	orders.Attributes[3].Choice.ParameterLinks[0].Name = "Другое"
	again, _ := catalog.CatalogDefinition("Заказы")
	if again.Attributes[0].Presentation.MinValue.Data != "0" || again.Attributes[3].Choice.ParameterLinks[0].Name != "Отбор.Владелец" {
		t.Fatal("a field's settings were handed out by reference")
	}
}

// A link takes a choice parameter from another field of the same object. One
// pointing at a field that is not there narrows nothing, and the user picks
// out of everything with nothing said about it.
func TestFieldLinksMustPointAtAFieldOfTheObject(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ body, want string }{
		"связь на несуществующий реквизит": {`attributes:
  - id: ` + presentationContract + `
    name: Договор
    title: {ru: Договор}
    types: [{kind: string, length: 50}]
    choice:
      parameter_links: [{name: Отбор.Владелец, source: {attribute: ` + presentationMissing + `}}]`,
			"parameter_links[0].source is not a field of this object"},
		"связь на чужую табличную часть": {`attributes:
  - id: ` + presentationContract + `
    name: Договор
    title: {ru: Договор}
    types: [{kind: string, length: 50}]
    choice:
      parameter_links:
        - {name: Отбор.Владелец, source: {table_part: ` + presentationMissing + `, attribute: ` + presentationLine + `}}
table_parts:
  - id: ` + presentationPart + `
    name: Строки
    title: {ru: Строки}
    attributes:
      - {id: ` + presentationLine + `, name: Номенклатура, title: {ru: Номенклатура}, types: [{kind: string, length: 50}]}`,
			"is not a field of this object"},
		"связь по типу в никуда": {`attributes:
  - id: ` + presentationContract + `
    name: Договор
    title: {ru: Договор}
    types: [{kind: string, length: 50}]
    choice:
      link_by_type: {source: {attribute: ` + presentationMissing + `}}`,
			"link_by_type.source is not a field of this object"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeCatalog("object.yaml", strings.NewReader(presentationOrder(broken.body+"\n")), metadataConfiguration())
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

// The settings of a field are checked where the field is written down.
func TestBrokenFieldSettingsAreRefused(t *testing.T) {
	t.Parallel()
	field := func(settings string) string {
		return `attributes:
  - id: ` + presentationAmount + `
    name: Сумма
    title: {ru: Сумма}
    types: [{kind: number, precision: 15, scale: 2}]
` + settings
	}
	for name, broken := range map[string]struct{ body, want string }{
		"граница другого типа": {field(`    presentation: {min_value: {kind: string, data: "0"}}`),
			"is of a type the field cannot hold"},
		"переключатель не из трёх": {field(`    choice: {quick_choice: maybe}`),
			"must be auto, use or dont-use"},
		"выбор не из четырёх": {field(`    choice: {folders_and_items: anything}`),
			"must be auto, items, folders or folders-and-items"},
		"параметр без значения": {field(`    choice: {parameters: [{name: Отбор.Вид, values: []}]}`),
			"values must contain at least one value"},
		"несколько значений молча": {field(`    choice:
      parameters:
        - name: Отбор.Вид
          values: [{kind: number, data: "1"}, {kind: number, data: "2"}]`),
			"must say it is a list"},
		"параметр задан и связан разом": {field(`    choice:
      parameters: [{name: Отбор.Вид, values: [{kind: number, data: "1"}]}]
      parameter_links: [{name: отбор.вид, source: {attribute: ` + presentationAmount + `}}]`),
			"is already set or linked"},
		"связь меняется неизвестно как": {field(`    choice:
      parameter_links: [{name: Отбор.Вид, source: {attribute: ` + presentationAmount + `}, change: erase}]`),
			"must be clear or dont-change"},
		"форма выбора без объекта": {field(`    choice: {form: {kind: catalogs, name: ФормаВыбора}}`),
			"object must be set beside the kind"},
		"форма выбора без вида": {field(`    choice: {form: {object: ` + presentationPartners + `, name: ФормаВыбора}}`),
			"kind must be set beside the object"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeCatalog("object.yaml", strings.NewReader(presentationOrder(broken.body+"\n")), metadataConfiguration())
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}

	// A bound is not checked against a description the configuration may widen:
	// what a set holds is not written down in the field, so nothing here can
	// say whether the bound fits.
	_, err := DecodeCatalog("object.yaml", strings.NewReader(presentationOrder(`attributes:
  - id: `+presentationAmount+`
    name: Ссылка2
    title: {ru: Ссылка}
    types: [{kind: catalog-ref}]
    presentation: {min_value: {kind: string, data: "0"}}
`)), metadataConfiguration())
	if err != nil {
		t.Fatalf("a bound beside an open type description was refused: %v", err)
	}
}
