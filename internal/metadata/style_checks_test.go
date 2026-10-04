package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// What a style sets for an item of the configuration is checked once the
// project is read, against that item: a value taken from an item that is not
// there is refused, a value of two kinds at once is refused, and a border
// says where it is taken from only when it is taken from a style.
func TestAStyleSettingIsCheckedAgainstTheItemItIsFor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, value, want string
	}{
		{"taken from an item that is not there", "{color: {source: style, from: {item: " + uuid.MustNew().String() + "}}}", "takes its value from unknown style item"},
		{"two kinds at once", "{color: {source: absolute, rgb: '#8AB4F8'}, font: {source: absolute, face: Arial, size: 16}}", "is more than one of a colour, a font and a border"},
		{"an absolute border taken from somewhere", "{border: {source: absolute, line: single, width: 1, from: {standard: ControlBorder}}}", ".from belongs to a border taken from a style item only"},
		{"a border taken from a style", "{border: {source: style, from: {standard: ControlBorder}}}", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			itemType, value := ColorStyleItem, "  color: {source: absolute, rgb: '#1C55AE'}\n"
			if strings.Contains(test.value, "border") {
				itemType, value = BorderStyleItem, "  border: {source: absolute, line: single, width: 1}\n"
			}
			writeStyleItem(t, root, styleItemColor, "Оформление", itemType, value)
			writeMetadata(t, root, StyleKind, styleID, "format: 1\nid: "+styleID+"\nname: Тёмный\ntitle: {ru: Тёмный}\n"+
				"items:\n  - item: "+styleItemColor+"\n    value: "+test.value+"\n")
			_, err := Load(root)
			switch {
			case test.want == "" && err != nil:
				t.Fatalf("the style was refused: %v", err)
			case test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)):
				t.Fatalf("Load() error = %v, want it to say %q", err, test.want)
			}
		})
	}
}

// A grid drawn with no step is a grid of the map's own spacing; only a
// negative step is refused.
func TestARouteMapGridMayHaveNoStep(t *testing.T) {
	t.Parallel()
	route := func(step string) string {
		return `
  look: {grid: true, grid_horizontal_step: ` + step + `, grid_vertical_step: ` + step + `}
  points:
    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:
    - {from: Старт, to: Завершение}
`
	}
	if _, err := Load(routeOnlyProcess(t, route("0"))); err != nil {
		t.Fatalf("a grid with no step was refused: %v", err)
	}
	if _, err := Load(routeOnlyProcess(t, route("-1"))); err == nil || !strings.Contains(err.Error(), "grid steps must not be negative") {
		t.Fatalf("a negative step: %v", err)
	}
}

// A field of the list's search gets an index whether it is a column of the
// object's own or an attribute.
func TestEverySearchFieldOfAListGetsAnIndex(t *testing.T) {
	t.Parallel()
	owner, attribute := uuid.MustNew(), uuid.MustNew()
	attributes := []Attribute{{ID: attribute, Name: "Артикул", Types: []Type{{Kind: StringType, Length: 20}}}}
	system := map[string]listColumn{"code": {name: "code", kind: StringType}}
	var table schemadiff.Table
	appendListSearchIndexes(&table, owner, ListSettings{SearchFields: []string{"Code", "Артикул"}}, nil, attributes, system)
	column, _ := PhysicalAttributeColumn(attribute)
	var keys []string
	for _, index := range table.Indexes {
		keys = append(keys, strings.Join(index.Keys, ","))
	}
	joined := strings.Join(keys, ";")
	if len(table.Indexes) != 2 || !strings.Contains(joined, "lower(code::text)") || !strings.Contains(joined, column) {
		t.Fatalf("indexes = %v", keys)
	}
}

// A font is scaled by 999 per cent at most and a border is 5 thick at most:
// that is what the configurator takes (8.3.27, checked by the owner
// 04.10.2026). Each bound is tried on both sides, and so is nought.
func TestAStyleValueStaysWithinWhatTheConfiguratorTakes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		itemType StyleItemType
		value    string
		taken    bool
	}{
		{"scale 999", FontStyleItem, "font: {source: absolute, face: Arial, size: 10, scale: 999}", true},
		{"scale 1000", FontStyleItem, "font: {source: absolute, face: Arial, size: 10, scale: 1000}", false},
		{"scale 0", FontStyleItem, "font: {source: absolute, face: Arial, size: 10, scale: 0}", true},
		{"border 5", BorderStyleItem, "border: {source: absolute, line: single, width: 5}", true},
		{"border 6", BorderStyleItem, "border: {source: absolute, line: single, width: 6}", false},
		{"border 0", BorderStyleItem, "border: {source: absolute, line: single, width: 0}", true},
		{"border -1", BorderStyleItem, "border: {source: absolute, line: single, width: -1}", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeStyleItem(t, root, styleItemColor, "Оформление", test.itemType, "  "+test.value+"\n")
			_, err := Load(root)
			if test.taken && err != nil {
				t.Fatalf("refused: %v", err)
			}
			if !test.taken && (err == nil || !strings.Contains(err.Error(), "must be")) {
				t.Fatalf("taken: %v", err)
			}
		})
	}
}
