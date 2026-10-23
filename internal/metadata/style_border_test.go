package metadata

import (
	"strings"
	"testing"
)

// A border is drawn one of the eight ways the window «Выбор рамки» offers
// (opis of the style item, 14.10.2026); the rounded one, which the help gives
// to the border of a group alone, is taken nowhere - not by a style item, not
// by a style, not by a field of a form or a part of a chart, all of which a
// border passes through validateBorderValue. A style item of the type «Рамка»
// may be left with no value, as the designer leaves a new one; a colour and a
// font may not. The eight are written here, not read from the model.
//
// Defect caught: the rounded border taken in a style item of any thickness,
// which the designer cannot make; a border item with no value refused, which
// the designer saves; a colour or a font left with no value taken.
func TestABorderIsOneOfTheEightAndABorderItemMayBeEmpty(t *testing.T) {
	t.Parallel()
	for _, line := range []string{"none", "single", "double", "indented", "embossed", "underline", "double-underline", "overline"} {
		if issues := validateBorderValue("border", BorderValue{Source: AbsoluteBorder, Line: BorderLine(line), Width: 1}); len(issues) != 0 {
			t.Errorf("%s refused: %v", line, issues)
		}
	}
	if issues := validateBorderValue("border", BorderValue{Source: AbsoluteBorder, Line: "rounded", Width: 1}); len(issues) != 1 || !strings.Contains(issues[0], "border.line is not a way a border is drawn") {
		t.Errorf("a rounded border: %v", issues)
	}
	for name, test := range map[string]struct {
		itemType StyleItemType
		value    string
		want     string
	}{
		"рамка без значения":      {BorderStyleItem, "  {}\n", ""},
		"скруглённая рамка":       {BorderStyleItem, "  border: {source: absolute, line: rounded, width: 1}\n", "value.border.line is not a way a border is drawn"},
		"цвет без значения":       {ColorStyleItem, "  {}\n", "value must be a colour, a font or a border"},
		"шрифт без значения":      {FontStyleItem, "  {}\n", "value must be a colour, a font or a border"},
		"рамка с чужим значением": {BorderStyleItem, "  color: {source: auto}\n", "value is not a border"},
	} {
		root := metadataProject(t)
		writeStyleItem(t, root, styleItemFont, "Элемент", test.itemType, test.value)
		_, err := Load(root)
		switch {
		case test.want == "" && err != nil:
			t.Errorf("%s refused: %v", name, err)
		case test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)):
			t.Errorf("%s: %v, want %q", name, err, test.want)
		}
	}
	// A style setting a standard border item takes no rounded border either.
	root := metadataProject(t)
	writeMetadata(t, root, StyleKind, styleItemDerive, "format: 1\nid: "+styleItemDerive+"\nname: Стиль\ntitle: {ru: Стиль}\nitems:\n"+
		"  - standard: РамкаЭлементаУправления\n    value: {border: {source: absolute, line: rounded, width: 1}}\n")
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "border.line is not a way a border is drawn") {
		t.Errorf("a style setting a rounded border: %v", err)
	}
}
