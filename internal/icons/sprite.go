package icons

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"sync"
)

// The rule of the style, the one place it is written (ICONS.md, «Визуальный
// язык»): a line is 1 px with round ends and joins, the corners of a body are
// rounded by 2, a body is filled with its colour at about a fifth of its
// strength, and on a dark theme the outlines and details lighten - the page
// says how much by --icon-lighten (Studio: 28%), the fill and the accents stay.
const (
	cornerRadius = 2
	bodyFill     = 22 // per cent of the colour of the kind
	onAccentInk  = "#6b4e00"
)

func outline(color string) string {
	return "color-mix(in srgb," + color + ",#fff var(--icon-lighten,0%))"
}

func paint(role Role, kind, accent string) string {
	switch role {
	case Body:
		return "fill:color-mix(in srgb," + kind + " " + strconv.Itoa(bodyFill) + "%,transparent);stroke:" + outline(kind)
	case Detail:
		return "fill:none;stroke:" + outline(kind)
	case Accent:
		return "fill:" + accent + ";stroke:none"
	case Line:
		return "fill:none;stroke:" + accent
	default:
		return "fill:none;stroke:" + onAccentInk
	}
}

func number(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func (part Part) element() string {
	switch {
	case part.Rect != nil:
		radius := ""
		if part.Role == Body {
			radius = ` rx="` + number(cornerRadius) + `"`
		}
		return fmt.Sprintf(`<rect x="%s" y="%s" width="%s" height="%s"%s`, number(part.Rect[0]), number(part.Rect[1]), number(part.Rect[2]), number(part.Rect[3]), radius)
	case part.Circle != nil:
		return fmt.Sprintf(`<circle cx="%s" cy="%s" r="%s"`, number(part.Circle[0]), number(part.Circle[1]), number(part.Circle[2]))
	default:
		return `<path d="` + html.EscapeString(part.Path) + `"`
	}
}

// Symbol is the icon as an SVG symbol, named "icon-<id>".
func (icon Icon) Symbol() string {
	var out strings.Builder
	out.WriteString(`<symbol id="icon-` + icon.ID + `" viewBox="0 0 16 16"><g style="stroke-width:1;stroke-linecap:round;stroke-linejoin:round">`)
	for _, part := range icon.Parts {
		accent := icon.Accent
		if accent == "" {
			accent = icon.Color
		}
		if part.Color != "" {
			accent = part.Color
		}
		out.WriteString(part.element())
		out.WriteString(` style="` + paint(part.Role, icon.Color, accent) + `"/>`)
	}
	out.WriteString(`</g></symbol>`)
	return out.String()
}

var sprite = sync.OnceValue(func() []byte {
	var out strings.Builder
	out.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="0" height="0" style="position:absolute" aria-hidden="true">`)
	for _, icon := range table {
		out.WriteString(icon.Symbol())
	}
	out.WriteString(`</svg>`)
	return []byte(out.String())
})

// Sprite is every icon as one SVG document of symbols: a page puts it in once
// and draws an icon by <use href="#icon-<id>">.
func Sprite() []byte {
	return sprite()
}
