package icons

import (
	"encoding/xml"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/kinds"
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
// the colour of the kind; an accent solid in its own. The expectation is the
// approved sample's (requirements/assets/icons-style.html), written out here.
func TestRolesArePaintedAsTheStyle(t *testing.T) {
	icon := Icon{ID: "probe", Color: "#112233", Accent: "#445566", Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13), path(Detail, "M2 2H4"), circle(Accent, 8, 8, 2), path(Line, "M2 4H4"),
		colored(rect(Accent, 2, 2, 2, 2), "#778899"), colored(rect(Body, 3, 3, 2, 2), "#aabbcc"),
	}}
	symbol := icon.Symbol()
	for _, want := range []string{
		`<rect x="1.5" y="1.5" width="13" height="13" rx="2" style="fill:color-mix(in srgb,#112233 22%,transparent);stroke:color-mix(in srgb,#112233,#fff var(--icon-lighten,0%))"/>`,
		`<path d="M2 2H4" style="fill:none;stroke:color-mix(in srgb,#112233,#fff var(--icon-lighten,0%))"/>`,
		`<circle cx="8" cy="8" r="2" style="fill:#445566;stroke:none"/>`,
		`<path d="M2 4H4" style="fill:none;stroke:#445566"/>`,
		`<rect x="2" y="2" width="2" height="2" style="fill:#778899;stroke:none"/>`,
		`<rect x="3" y="3" width="2" height="2" rx="2" style="fill:color-mix(in srgb,#aabbcc 22%,transparent);stroke:color-mix(in srgb,#aabbcc,#fff var(--icon-lighten,0%))"/>`,
		`stroke-width:1;stroke-linecap:round;stroke-linejoin:round`,
	} {
		if !strings.Contains(symbol, want) {
			t.Errorf("the symbol lacks %s:\n%s", want, symbol)
		}
	}
}
