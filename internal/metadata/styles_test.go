package metadata

import (
	"strings"
	"testing"
)

const (
	styleItemColor  = "e0000000-0000-4000-8000-000000000001"
	styleItemFont   = "e0000000-0000-4000-8000-000000000002"
	styleItemBorder = "e0000000-0000-4000-8000-000000000003"
	styleItemDerive = "e0000000-0000-4000-8000-000000000004"
	styleID         = "e0000000-0000-4000-8000-000000000010"
)

// writeStyleItem writes one style item: a name, a type, and the value that
// type calls for.
func writeStyleItem(t *testing.T, root, id, name string, itemType StyleItemType, value string) {
	t.Helper()
	writeMetadata(t, root, StyleItemKind, id, `format: 1
id: `+id+`
name: `+name+`
title: {ru: `+name+`}
type: `+string(itemType)+`
value:
`+value)
}

// All three kinds of style item are carried, each with the value its type
// calls for. A type the model does not know is a piece of the look lost
// without a word, and the look is then assembled out of what is left.
func TestStyleItemOfEveryTypeIsCarriedWithItsValue(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeStyleItem(t, root, styleItemColor, "ЦветГиперссылки", ColorStyleItem,
		"  color: {source: absolute, rgb: '#1C55AE'}\n")
	writeStyleItem(t, root, styleItemFont, "ШрифтОбычный", FontStyleItem,
		"  font: {source: absolute, face: Arial, size: 14, scale: 100}\n")
	writeStyleItem(t, root, styleItemBorder, "РамкаПоля", BorderStyleItem,
		"  border: {source: absolute, line: single, width: 1}\n")

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	colour, ok := catalog.StyleItem("ЦветГиперссылки")
	if !ok || colour.Type != ColorStyleItem || colour.Value.Color == nil || colour.Value.Color.RGB != "#1C55AE" {
		t.Fatalf("the colour lost its value: %+v", colour)
	}
	font, ok := catalog.StyleItem("ШрифтОбычный")
	if !ok || font.Value.Font == nil || font.Value.Font.Face != "Arial" || font.Value.Font.Size != 14 {
		t.Fatalf("the font lost its value: %+v", font)
	}
	border, ok := catalog.StyleItem("РамкаПоля")
	if !ok || border.Value.Border == nil || border.Value.Border.Line != SingleBorderLine || border.Value.Border.Width != 1 {
		t.Fatalf("the border lost its value: %+v", border)
	}

	// The catalog hands out copies, values inside them included.
	colour.Value.Color.RGB = "#000000"
	again, _ := catalog.StyleItem("ЦветГиперссылки")
	if again.Value.Color.RGB == "#000000" {
		t.Fatal("style item lookup exposed mutable metadata")
	}
}

// A value is taken from another style item so that one base decides the shape
// of everything built on it: a heading is the ordinary font in bold, and stays
// a heading when the ordinary font changes. Both sources are carried - the
// platform's standard set and the configuration's own items.
func TestStyleItemTakesItsValueFromAnotherItem(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeStyleItem(t, root, styleItemFont, "ШрифтОбычный", FontStyleItem,
		"  font: {source: absolute, face: Arial, size: 14}\n")
	writeStyleItem(t, root, styleItemDerive, "ШрифтЗаголовка", FontStyleItem,
		"  font: {source: style, from: {item: "+styleItemFont+"}, bold: true}\n")
	writeStyleItem(t, root, styleItemColor, "ЦветТекста", ColorStyleItem,
		"  color: {source: style, from: {standard: ЦветТекстаФормы}}\n")

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	heading, ok := catalog.StyleItem("ШрифтЗаголовка")
	if !ok || heading.Value.Font.From == nil || heading.Value.Font.From.Item == nil ||
		heading.Value.Font.From.Item.String() != styleItemFont || !heading.Value.Font.Bold {
		t.Fatalf("the heading font lost what it is built on: %+v", heading)
	}
	standard, ok := catalog.StyleItem("ЦветТекста")
	if !ok || standard.Value.Color.From == nil || standard.Value.Color.From.Standard != "ЦветТекстаФормы" {
		t.Fatalf("a value taken from the standard set was lost: %+v", standard)
	}
}

// A colour is written the four ways a real configuration writes it. The web
// palette is not checked against a list of ours on purpose: the names a real
// configuration uses are not the CSS ones, and a list of ours would refuse a
// configuration that is perfectly correct.
func TestColourIsWrittenEveryWayAConfigurationWritesIt(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	for index, value := range []string{
		"  color: {source: absolute, rgb: '#DCDCDC'}\n",
		"  color: {source: web, name: VioletRed}\n",
		"  color: {source: system, name: ButtonFace}\n",
		"  color: {source: auto}\n",
	} {
		writeStyleItem(t, root, styleIdentifier(index), "Цвет"+string('A'+rune(index)), ColorStyleItem, value)
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.StyleItems) != 4 {
		t.Fatalf("%d colours of 4 survived", len(catalog.StyleItems))
	}
	web, _ := catalog.StyleItem("ЦветB")
	if web.Value.Color.Source != WebColor || web.Value.Color.Name != "VioletRed" {
		t.Fatalf("a colour of the web palette was not carried as written: %+v", web.Value.Color)
	}
}

// A style is a set of style items and what this look makes of them. The value
// a style gives an item is allowed to differ from the item's own - that is the
// whole point of having more than one style.
func TestStyleSetsTheItemsItNames(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeStyleItem(t, root, styleItemColor, "ЦветГиперссылки", ColorStyleItem,
		"  color: {source: absolute, rgb: '#1C55AE'}\n")
	writeStyleItem(t, root, styleItemFont, "ШрифтОбычный", FontStyleItem,
		"  font: {source: absolute, face: Arial, size: 14}\n")
	writeMetadata(t, root, StyleKind, styleID, `format: 1
id: `+styleID+`
name: Тёмный
title: {ru: Тёмный}
comment: Для работы ночью
items:
  - item: `+styleItemColor+`
    value: {color: {source: absolute, rgb: '#8AB4F8'}}
  - item: `+styleItemFont+`
    value: {font: {source: absolute, face: Arial, size: 16}}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	style, ok := catalog.Style("Тёмный")
	if !ok || len(style.Items) != 2 || style.Comment != "Для работы ночью" {
		t.Fatalf("the style lost its composition: %+v", style)
	}
	if style.Items[0].Value.Color == nil || style.Items[0].Value.Color.RGB != "#8AB4F8" {
		t.Fatalf("the style did not keep the value it gives the item: %+v", style.Items[0])
	}
	// The item itself keeps the value it was given where it was declared: a
	// style overrides the value, it does not rewrite the item.
	item, _ := catalog.StyleItem("ЦветГиперссылки")
	if item.Value.Color.RGB != "#1C55AE" {
		t.Fatalf("a style rewrote the style item it sets: %+v", item.Value.Color)
	}

	style.Items[0].Value.Color.RGB = "#000000"
	again, _ := catalog.Style("Тёмный")
	if again.Items[0].Value.Color.RGB == "#000000" {
		t.Fatal("style lookup exposed mutable metadata")
	}
}

func TestStyleReferencesAreChecked(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		write     func(t *testing.T, root string)
		complains string
	}{
		"стиль настраивает несуществующий элемент": {func(t *testing.T, root string) {
			writeMetadata(t, root, StyleKind, styleID, `format: 1
id: `+styleID+`
name: Тёмный
title: {ru: Тёмный}
items:
  - item: `+styleItemColor+`
    value: {color: {source: auto}}
`)
		}, styleItemColor},
		"стиль даёт элементу значение не того типа": {func(t *testing.T, root string) {
			writeStyleItem(t, root, styleItemColor, "ЦветГиперссылки", ColorStyleItem,
				"  color: {source: auto}\n")
			writeMetadata(t, root, StyleKind, styleID, `format: 1
id: `+styleID+`
name: Тёмный
title: {ru: Тёмный}
items:
  - item: `+styleItemColor+`
    value: {font: {source: auto}}
`)
		}, "not a color"},
		"значение взято из элемента другого типа": {func(t *testing.T, root string) {
			writeStyleItem(t, root, styleItemColor, "ЦветГиперссылки", ColorStyleItem,
				"  color: {source: auto}\n")
			writeStyleItem(t, root, styleItemFont, "ШрифтЗаголовка", FontStyleItem,
				"  font: {source: style, from: {item: "+styleItemColor+"}}\n")
		}, "which is a color and not a font"},
		"значение взято из несуществующего элемента": {func(t *testing.T, root string) {
			writeStyleItem(t, root, styleItemFont, "ШрифтЗаголовка", FontStyleItem,
				"  font: {source: style, from: {item: "+styleItemDerive+"}}\n")
		}, styleItemDerive},
		"элемент берёт значение у самого себя": {func(t *testing.T, root string) {
			writeStyleItem(t, root, styleItemFont, "ШрифтЗаголовка", FontStyleItem,
				"  font: {source: style, from: {item: "+styleItemFont+"}}\n")
		}, "circle"},
		"два элемента берут значение друг у друга": {func(t *testing.T, root string) {
			writeStyleItem(t, root, styleItemFont, "ШрифтПервый", FontStyleItem,
				"  font: {source: style, from: {item: "+styleItemDerive+"}}\n")
			writeStyleItem(t, root, styleItemDerive, "ШрифтВторой", FontStyleItem,
				"  font: {source: style, from: {item: "+styleItemFont+"}}\n")
		}, "circle"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			test.write(t, root)
			_, err := Load(root)
			if err == nil {
				t.Fatal("the project loaded")
			}
			if !strings.Contains(err.Error(), test.complains) {
				t.Fatalf("the complaint does not say %q: %v", test.complains, err)
			}
		})
	}
}

func TestStyleItemValueIsChecked(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		itemType StyleItemType
		value    string
	}{
		"значения нет вовсе":               {ColorStyleItem, "  {}\n"},
		"значение двух видов сразу":        {ColorStyleItem, "  color: {source: auto}\n  font: {source: auto}\n"},
		"значение не того вида, что тип":   {ColorStyleItem, "  font: {source: auto}\n"},
		"цвет без своего значения":         {ColorStyleItem, "  color: {source: absolute}\n"},
		"цвет записан не шестнадцатерично": {ColorStyleItem, "  color: {source: absolute, rgb: синий}\n"},
		"источник цвета неизвестен":        {ColorStyleItem, "  color: {source: краска}\n"},
		"шрифт без имени":                  {FontStyleItem, "  font: {source: absolute, size: 14}\n"},
		"шрифту неоткуда взять размер":     {FontStyleItem, "  font: {source: absolute, face: Arial}\n"},
		"шрифт из стиля без основы":        {FontStyleItem, "  font: {source: style}\n"},
		"основа названа дважды": {FontStyleItem,
			"  font: {source: style, from: {standard: Обычный, item: " + styleItemColor + "}}\n"},
		"рамка не тем начертанием": {BorderStyleItem, "  border: {source: absolute, line: волнистая}\n"},
		"рамка толще, чем рисуется": {BorderStyleItem,
			"  border: {source: absolute, line: single, width: 9}\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeStyleItem(t, root, styleItemFont, "Элемент", test.itemType, test.value)
			if _, err := Load(root); err == nil {
				t.Fatal("the project loaded")
			}
		})
	}
}

func styleIdentifier(index int) string {
	const digits = "0123456789abcdef"
	return "e0000000-0000-4000-8000-0000000001" + string([]byte{digits[index/16], digits[index%16]})
}
