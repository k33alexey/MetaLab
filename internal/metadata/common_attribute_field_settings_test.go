package metadata

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// commonAttributeWithSettings is a common attribute carrying the whole field:
// what it is shown by, how it is chosen, what it starts as.
func commonAttributeWithSettings(objects ...uuid.UUID) CommonAttributeDefinition {
	attribute := commonAttributeFixture(objects...)
	attribute.Comment = "Менеджер, ответственный за объект"
	attribute.Presentation = FieldPresentation{
		ToolTip: LocalizedText{"ru": "Ответственный менеджер"},
		Mask:    "!999",
	}
	attribute.Choice = FieldChoice{QuickChoice: UsageUse, FoldersAndItems: ChoiceTargetItems}
	attribute.Filling = FieldFilling{Value: &Value{Kind: StringType, Data: "не назначен"}}
	return attribute
}

// The defect this catches: a common attribute carried a name, a type and three
// flags, and nothing of what any field carries. ТЗ asks for «полный набор
// свойств реквизита» here, and the reason is in the mechanism rather than in
// tidiness: a common attribute becomes an ordinary attribute of every object it
// targets, so a format it cannot hold is a format none of those objects get.
func TestCommonAttributeCarriesTheWholeField(t *testing.T) {
	t.Parallel()
	attribute := commonAttributeWithSettings(uuid.MustNew())
	var encoded bytes.Buffer
	if err := Encode(&encoded, attribute); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeCommonAttribute("common-attribute.yaml", bytes.NewReader(encoded.Bytes()), metadataConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Comment != attribute.Comment {
		t.Fatalf("comment = %q", decoded.Comment)
	}
	if decoded.Presentation.ToolTip["ru"] == "" || decoded.Presentation.Mask != "!999" {
		t.Fatalf("presentation = %+v", decoded.Presentation)
	}
	if decoded.Choice.QuickChoice != UsageUse || decoded.Choice.FoldersAndItems != ChoiceTargetItems {
		t.Fatalf("choice = %+v", decoded.Choice)
	}
	if decoded.Filling.Value == nil || decoded.Filling.Value.Data != "не назначен" {
		t.Fatalf("filling = %+v", decoded.Filling)
	}
}

// Kept on the definition alone the settings would describe a field nothing
// sees: what the objects get is the propagated attribute, and it is the one the
// forms and the schema read.
func TestCommonAttributePropagatesTheWholeField(t *testing.T) {
	t.Parallel()
	first := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Товары", Title: LocalizedText{"ru": "Товары"},
		Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 100}
	second := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Склады", Title: LocalizedText{"ru": "Склады"},
		Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 100}
	attribute := commonAttributeWithSettings(first.ID, second.ID)
	catalog, err := NewCatalogSnapshotWithCommonAttributes(metadataConfiguration(), nil, nil, nil,
		[]CatalogDefinition{first, second}, nil, nil, nil, nil, nil, nil, []CommonAttributeDefinition{attribute})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Товары", "Склады"} {
		definition, ok := catalog.CatalogDefinition(name)
		if !ok || len(definition.Attributes) != 1 {
			t.Fatalf("%s: attributes = %+v", name, definition.Attributes)
		}
		field := definition.Attributes[0]
		if field.Comment == "" || field.Presentation.ToolTip["ru"] == "" || field.Presentation.Mask != "!999" {
			t.Fatalf("%s: the propagated field lost what it is shown by: %+v", name, field)
		}
		if field.Choice.QuickChoice != UsageUse || field.Filling.Value == nil {
			t.Fatalf("%s: the propagated field lost its choice or its filling: %+v", name, field)
		}
	}

	// Each object gets a copy of its own. Built once and appended to every
	// target, the attribute would give them all the very same maps, and the
	// first thing to change one field would change it everywhere. Nothing
	// changes them in place today, which is why this is checked by identity and
	// not by mutating: it is what keeps the defect from being written later.
	sharedTitle := reflect.ValueOf(catalog.Catalogs[0].Attributes[0].Title).Pointer() ==
		reflect.ValueOf(catalog.Catalogs[1].Attributes[0].Title).Pointer()
	sharedTip := reflect.ValueOf(catalog.Catalogs[0].Attributes[0].Presentation.ToolTip).Pointer() ==
		reflect.ValueOf(catalog.Catalogs[1].Attributes[0].Presentation.ToolTip).Pointer()
	if sharedTitle || sharedTip {
		t.Fatalf("two objects share one propagated field: title=%v tooltip=%v", sharedTitle, sharedTip)
	}
	if catalog.Catalogs[0].Attributes[0].Filling.Value == catalog.Catalogs[1].Attributes[0].Filling.Value {
		t.Fatal("two objects share one filling value")
	}
}

func TestCommonAttributeRefusesSettingsThatSayNothing(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*CommonAttributeDefinition){
		// A bound of another type than the field is compared with nothing, so it
		// rejects nothing - in every object the attribute is propagated into.
		"граница другого типа": func(a *CommonAttributeDefinition) {
			a.Presentation.MinValue = &Value{Kind: NumberType, Data: "1"}
		},
		"значение заполнения другого типа": func(a *CommonAttributeDefinition) {
			a.Filling.Value = &Value{Kind: NumberType, Data: "1"}
		},
		"неизвестный режим выбора": func(a *CommonAttributeDefinition) {
			a.Choice.QuickChoice = "иногда"
		},
		// A common attribute is declared away from the objects it will join, so
		// which siblings a link could reach depends on the object - there is no
		// field here for it to point at.
		"связь параметра выбора": func(a *CommonAttributeDefinition) {
			a.Choice.ParameterLinks = []ChoiceParameterLink{{Name: "Отбор.Владелец", Source: FieldPath{Attribute: uuid.MustNew()}}}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			attribute := commonAttributeWithSettings(uuid.MustNew())
			mutate(&attribute)
			if err := ValidateCommonAttribute("common-attribute.yaml", attribute, metadataConfiguration()); err == nil {
				t.Fatal("the common attribute was accepted")
			}
		})
	}
}

// A copy from the catalog shares nothing with it: the settings are maps and a
// pointer, and a caller that changes what it got must not change what the next
// caller gets.
func TestCommonAttributeCopyKeepsItsOwnSettings(t *testing.T) {
	t.Parallel()
	target := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Товары", Title: LocalizedText{"ru": "Товары"},
		Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 100}
	attribute := commonAttributeWithSettings(target.ID)
	catalog, err := NewCatalogSnapshotWithCommonAttributes(metadataConfiguration(), nil, nil, nil,
		[]CatalogDefinition{target}, nil, nil, nil, nil, nil, nil, []CommonAttributeDefinition{attribute})
	if err != nil {
		t.Fatal(err)
	}
	first, ok := catalog.CommonAttributeByID(attribute.ID)
	if !ok {
		t.Fatal("the common attribute did not come back")
	}
	first.Presentation.ToolTip["ru"] = "изменено"
	first.Filling.Value.Data = "изменено"
	second, _ := catalog.CommonAttributeByID(attribute.ID)
	if second.Presentation.ToolTip["ru"] != "Ответственный менеджер" || second.Filling.Value.Data != "не назначен" {
		t.Fatalf("the copy shared its settings: %+v / %+v", second.Presentation.ToolTip, second.Filling.Value)
	}
}

// Encoding and reading back must not lose the settings either: the file is what
// the configuration keeps, and what a round trip drops is gone for good.
func TestCommonAttributeRoundTripKeepsTheSettings(t *testing.T) {
	t.Parallel()
	attribute := commonAttributeWithSettings(uuid.MustNew())
	var encoded bytes.Buffer
	if err := Encode(&encoded, attribute); err != nil {
		t.Fatal(err)
	}
	text := encoded.String()
	for _, word := range []string{"comment", "presentation", "tooltip", "mask", "choice", "quick_choice", "filling"} {
		if !strings.Contains(text, word) {
			t.Fatalf("the encoded common attribute has no %q:\n%s", word, text)
		}
	}
}
