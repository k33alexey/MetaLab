package icons

import (
	"encoding/xml"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
)

// TestEveryKindIsDrawn catches a kind added to the descriptors without an
// icon, or naming one the set does not draw: its branch and its objects would
// stand in the tree as an empty square.
func TestEveryKindIsDrawn(t *testing.T) {
	for _, descriptor := range kinds.All() {
		if !Has(descriptor.Icon) {
			t.Errorf("%s names icon %q, which the set does not draw", descriptor.Kind, descriptor.Icon)
		}
	}
}

var coordinates = regexp.MustCompile(`-?\d+(\.\d+)?`)

// TestEveryFormElementIsDrawn catches a kind of form element with no icon,
// or one the set does not draw - an empty square in the form editor's tree -
// and an element drawn unlike the prototype, which draws every field one red
// dash, every group one folder and both decorations one picture
// (docs/block-00/redaktor-formy.md; the expectations are written here, not
// read from the table the code draws by).
func TestEveryFormElementIsDrawn(t *testing.T) {
	icons := metadata.FormElementIcons()
	for kind, icon := range icons {
		if !Has(icon) {
			t.Errorf("form element %s names icon %q, which the set does not draw", kind, icon)
		}
	}
	for kind, want := range map[metadata.FormElementKind]string{
		"input-field": "form-field", "check-box-field": "form-field", "planner-field": "form-field",
		"usual-group": "catalog-group", "pages": "catalog-group", "page": "catalog-group", "column-group": "catalog-group",
		"command-bar": "catalog-group", "button-group": "catalog-group", "popup": "catalog-group",
		"label-decoration": "form-decoration", "picture-decoration": "form-decoration",
		"table": "form-table", "button": "form-button",
		"search-string-addition": "form-search-string", "search-control-addition": "form-search-control",
		"view-status-addition": "form-view-status",
	} {
		if icons[kind] != want {
			t.Errorf("form element %s is drawn %q, the prototype draws it %q", kind, icons[kind], want)
		}
	}
}

// TestIconsAreDrawnInTheGrid catches an icon that the sprite cannot hold: a
// name used twice (one symbol hides the other), a part with no geometry or
// with two, a colour that is not a colour, a shape reaching out of the 16×16
// grid and cut off by it.
func TestIconsAreDrawnInTheGrid(t *testing.T) {
	color := regexp.MustCompile(`^#[0-9a-f]{6}$`)
	seen := map[string]bool{}
	for _, icon := range All() {
		if icon.ID == "" || seen[icon.ID] {
			t.Errorf("icon %q is unnamed or named twice", icon.ID)
		}
		seen[icon.ID] = true
		if !color.MatchString(icon.Color) || icon.Accent != "" && !color.MatchString(icon.Accent) {
			t.Errorf("%s: colours %q and %q", icon.ID, icon.Color, icon.Accent)
		}
		if len(icon.Parts) == 0 {
			t.Errorf("%s has no parts", icon.ID)
		}
		for index, part := range icon.Parts {
			geometries := 0
			var low, high []float64
			if part.Rect != nil {
				geometries++
				low = append(low, part.Rect[0], part.Rect[1])
				high = append(high, part.Rect[0]+part.Rect[2], part.Rect[1]+part.Rect[3])
			}
			if part.Circle != nil {
				geometries++
				low = append(low, part.Circle[0]-part.Circle[2], part.Circle[1]-part.Circle[2])
				high = append(high, part.Circle[0]+part.Circle[2], part.Circle[1]+part.Circle[2])
			}
			if part.Path != "" {
				geometries++
				for _, value := range coordinates.FindAllString(part.Path, -1) {
					number, _ := strconv.ParseFloat(value, 64)
					low, high = append(low, number), append(high, number)
				}
			}
			if geometries != 1 {
				t.Errorf("%s part %d has %d geometries", icon.ID, index, geometries)
			}
			if part.Color != "" && !color.MatchString(part.Color) {
				t.Errorf("%s part %d: colour %q", icon.ID, index, part.Color)
			}
			for _, value := range low {
				if value < 0 {
					t.Errorf("%s part %d reaches %v, out of the grid", icon.ID, index, value)
				}
			}
			for _, value := range high {
				if value > 16 {
					t.Errorf("%s part %d reaches %v, out of the grid", icon.ID, index, value)
				}
			}
		}
	}
}

// TestSpriteHoldsEverySymbol catches a sprite a page cannot use: markup that
// does not parse, or a symbol missing or named apart from its icon, so that
// <use href="#icon-<id>"> finds nothing.
func TestSpriteHoldsEverySymbol(t *testing.T) {
	var sprite struct {
		Symbols []struct {
			ID      string `xml:"id,attr"`
			ViewBox string `xml:"viewBox,attr"`
		} `xml:"symbol"`
	}
	if err := xml.Unmarshal(Sprite(), &sprite); err != nil {
		t.Fatalf("the sprite does not parse: %v", err)
	}
	if len(sprite.Symbols) != len(All()) {
		t.Fatalf("the sprite holds %d symbols for %d icons", len(sprite.Symbols), len(All()))
	}
	for index, icon := range All() {
		if symbol := sprite.Symbols[index]; symbol.ID != "icon-"+icon.ID || symbol.ViewBox != "0 0 16 16" {
			t.Errorf("icon %s is the symbol %+v", icon.ID, symbol)
		}
	}
}

// TestRolesArePaintedAsTheStyle catches the rule of the style lost or turned
// round (ICONS.md, «Визуальный язык»): the body outlined in the colour of the
// kind, lightening on a dark theme, and filled with it at a fifth; details in
// the colour of the kind; an accent solid in its own; a plate filled with
// the page's background so that it hides what lies under it; a mark twice
// as thick as a line, in the accent colour; the rim of a coin a line in a
// darker shade of its accent. The expectation is the
// approved sample's (requirements/assets/icons-style.html), written out here.
func TestRolesArePaintedAsTheStyle(t *testing.T) {
	icon := Icon{ID: "probe", Color: "#112233", Accent: "#445566", Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13), path(Detail, "M2 2H4"), circle(Accent, 8, 8, 2), path(Line, "M2 4H4"),
		colored(rect(Accent, 2, 2, 2, 2), "#778899"), colored(rect(Body, 3, 3, 2, 2), "#aabbcc"),
		rect(Plate, 4, 4, 2, 2), path(Mark, "M2 6H4"), path(Rim, "M2 8H4"),
		path(InkMark, "M2 10H4"), path(InkSpot, "M2 12H4V14Z"),
	}}
	symbol := icon.Symbol()
	for _, want := range []string{
		`<rect x="1.5" y="1.5" width="13" height="13" rx="2" style="fill:color-mix(in srgb,#112233 22%,transparent);stroke:color-mix(in srgb,#112233,#fff var(--icon-lighten,0%))"/>`,
		`<path d="M2 2H4" style="fill:none;stroke:color-mix(in srgb,#112233,#fff var(--icon-lighten,0%))"/>`,
		`<circle cx="8" cy="8" r="2" style="fill:#445566;stroke:none"/>`,
		`<path d="M2 4H4" style="fill:none;stroke:#445566"/>`,
		`<rect x="2" y="2" width="2" height="2" style="fill:#778899;stroke:none"/>`,
		`<rect x="3" y="3" width="2" height="2" rx="2" style="fill:color-mix(in srgb,#aabbcc 22%,transparent);stroke:color-mix(in srgb,#aabbcc,#fff var(--icon-lighten,0%))"/>`,
		`<rect x="4" y="4" width="2" height="2" rx="2" style="fill:var(--icon-plate,#fff);stroke:color-mix(in srgb,#112233,#fff var(--icon-lighten,0%))"/>`,
		`<path d="M2 6H4" style="fill:none;stroke-width:2;stroke:#445566"/>`,
		`<path d="M2 8H4" style="fill:none;stroke-width:1;stroke:color-mix(in srgb,#445566,#5a3d00 30%)"/>`,
		`<path d="M2 10H4" style="fill:none;stroke-width:2;stroke:color-mix(in srgb,#112233,#fff calc(var(--icon-lighten,0%)*3))"/>`,
		`<path d="M2 12H4V14Z" style="fill:color-mix(in srgb,#112233,#fff calc(var(--icon-lighten,0%)*3));stroke:none"/>`,
		`stroke-width:1;stroke-linecap:round;stroke-linejoin:round`,
	} {
		if !strings.Contains(symbol, want) {
			t.Errorf("the symbol lacks %s:\n%s", want, symbol)
		}
	}
}

// TestTheLibraryHoldsEveryStandardPicture catches a picture of the library
// lost or written twice, a name in one language not resolving to the picture,
// a hidden picture offered or an offered one hidden, and a picture naming an
// icon the set does not draw. The expectations come from the syntax
// assistant of 8.3.27 and the window «Выбор картинки» (299 pictures, 39 of
// them not in the window; docs/block-00/standartnye-kartinki.md), not from
// the table.
func TestTheLibraryHoldsEveryStandardPicture(t *testing.T) {
	pictures := StandardPictures()
	if len(pictures) != 299 {
		t.Errorf("the library holds %d pictures, the help lists 299", len(pictures))
	}
	names := map[string]bool{}
	hidden := 0
	for _, picture := range pictures {
		for _, name := range []string{picture.Name, picture.Russian, picture.Spelling} {
			if name == "" {
				continue
			}
			if names[name] {
				t.Errorf("%s names two pictures", name)
			}
			names[name] = true
		}
		if picture.Hidden {
			hidden++
		}
		if picture.Icon != "" && !Has(picture.Icon) {
			t.Errorf("%s names icon %q, which the set does not draw", picture.Name, picture.Icon)
		}
	}
	if hidden != 39 {
		t.Errorf("%d pictures are hidden from the window, the prototype hides 39", hidden)
	}
	for name, want := range map[string]string{
		"Печать": "Print", "Print": "Print", "СоздатьЭлементСписка": "CreateListItem",
		"ОформлениеЗнакВоcклицательныйЗнак": "AppearanceExclamationMarkIcon",
		"ОформлениеЗнакВосклицательныйЗнак": "AppearanceExclamationMarkIcon",
		"ОформлениеВоcклицательныйЗнак":     "AppearanceExclamationMark",
	} {
		if picture, ok := LookupStandard(name); !ok || picture.Name != want {
			t.Errorf("%s resolves to %+v, it is %s", name, picture, want)
		}
	}
	for name, want := range map[string]bool{"УстановитьВремя": true, "Выбрать": true, "СправочникОбъект": true, "Печать": false, "ПолеВводаКалендарь": false} {
		if picture, _ := LookupStandard(name); picture.Hidden != want {
			t.Errorf("%s hidden = %v, the window %s", name, picture.Hidden, map[bool]string{true: "has no row for it", false: "offers it"}[want])
		}
	}
	if _, ok := LookupStandard("Начислить"); ok {
		t.Error("a name the library has not resolved to a picture")
	}
}
