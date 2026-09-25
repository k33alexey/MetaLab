package metadata

import (
	"fmt"
	"io"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	// StyleItemKind holds the named pieces a look is assembled from: one
	// colour, one font or one border, under a name the interface asks for.
	StyleItemKind Kind = "style-items"
	// StyleKind holds the looks themselves - a set of style items with the
	// values this look gives them.
	StyleKind Kind = "styles"
)

// maxStyleItemsPerStyle is where a style stops being a look and becomes a
// list. The platform's own standard set is of this order, and a configuration
// adds to it rather than replacing it.
const maxStyleItemsPerStyle = 1024

// StyleItemType says what one style item is. There are three and only three:
// a colour, a font, a border. Anything an interface is drawn with is one of
// them or is built out of them.
type StyleItemType string

const (
	ColorStyleItem  StyleItemType = "color"
	FontStyleItem   StyleItemType = "font"
	BorderStyleItem StyleItemType = "border"
)

// StyleItemReference names the style item a value is taken from: either one of
// the platform's own standard items, by name, or one of the configuration's
// own, by identifier. It is shaped like PictureReference and for the same
// reason - one source, never two and never none.
//
// The standard set belongs to the platform and is fixed where the client draws
// it, so a standard name is checked for shape here and for existence there.
type StyleItemReference struct {
	Standard string     `yaml:"standard,omitempty" json:"standard,omitempty"`
	Item     *uuid.UUID `yaml:"item,omitempty" json:"item,omitempty"`
}

// ColorSource says where a colour comes from.
type ColorSource string

const (
	// AbsoluteColor is the colour itself, written as #RRGGBB.
	AbsoluteColor ColorSource = "absolute"
	// WebColor names a colour of the web palette.
	//
	// The palette is not checked against a list of our own. The names a real
	// configuration uses are the web palette as the prototype has it, and it
	// is not the CSS one: it carries names CSS never had. A list of ours would
	// refuse a configuration that is perfectly correct, which is worse than
	// letting the client resolve the name where it draws it.
	WebColor ColorSource = "web"
	// SystemColor names a colour of the operating system's own palette.
	SystemColor ColorSource = "system"
	// AutoColor leaves the colour to the platform: what is automatic here
	// depends on what the colour is for, and only the client knows that.
	AutoColor ColorSource = "auto"
	// StyleColor takes the colour from another style item.
	StyleColor ColorSource = "style"
)

// ColorValue is the value of a style item that is a colour.
type ColorValue struct {
	Source ColorSource `yaml:"source" json:"source"`
	// RGB is the colour itself, #RRGGBB, when the source is absolute.
	RGB string `yaml:"rgb,omitempty" json:"rgb,omitempty"`
	// Name is the name in the web or system palette.
	Name string              `yaml:"name,omitempty" json:"name,omitempty"`
	From *StyleItemReference `yaml:"from,omitempty" json:"from,omitempty"`
}

// FontSource says where a font comes from.
type FontSource string

const (
	// AbsoluteFont is the font itself, by its face name.
	AbsoluteFont FontSource = "absolute"
	// SystemFont names a font of the operating system.
	SystemFont FontSource = "system"
	// AutoFont leaves the font to the platform.
	AutoFont FontSource = "auto"
	// StyleFont takes the font from another style item and changes what it
	// needs to: a heading is the ordinary font in bold, and saying so keeps
	// it a heading when the ordinary font changes.
	StyleFont FontSource = "style"
)

// FontValue is the value of a style item that is a font.
//
// The four switches are absolute whatever the source: a font taken from
// another style item is that font with these switches as written, which is how
// one look stays one look while a single base font decides its shape.
type FontValue struct {
	Source FontSource `yaml:"source" json:"source"`
	// Face is the face name when the font is absolute, and the name in the
	// system's own list when it is a system font.
	Face string `yaml:"face,omitempty" json:"face,omitempty"`
	// Size is the height in logical units. Left out, it is taken from what the
	// font is based on - which only a font with a base can do.
	Size float64 `yaml:"size,omitempty" json:"size,omitempty"`
	// Scale is a percentage; left out it is a hundred, the size as written.
	Scale     int                 `yaml:"scale,omitempty" json:"scale,omitempty"`
	From      *StyleItemReference `yaml:"from,omitempty" json:"from,omitempty"`
	Bold      bool                `yaml:"bold,omitempty" json:"bold,omitempty"`
	Italic    bool                `yaml:"italic,omitempty" json:"italic,omitempty"`
	Underline bool                `yaml:"underline,omitempty" json:"underline,omitempty"`
	Strikeout bool                `yaml:"strikeout,omitempty" json:"strikeout,omitempty"`
}

// BorderSource says where a border comes from.
type BorderSource string

const (
	AbsoluteBorder BorderSource = "absolute"
	StyleBorder    BorderSource = "style"
)

// BorderLine is how a border is drawn. A border that is not drawn is one of
// the nine, not the absence of a border: an element told to have no border and
// an element told nothing are different things.
type BorderLine string

const (
	NoBorderLine          BorderLine = "none"
	SingleBorderLine      BorderLine = "single"
	DoubleBorderLine      BorderLine = "double"
	IndentedBorderLine    BorderLine = "indented"
	EmbossedBorderLine    BorderLine = "embossed"
	UnderlineBorderLine   BorderLine = "underline"
	DoubleUnderlineBorder BorderLine = "double-underline"
	OverlineBorderLine    BorderLine = "overline"
	RoundedBorderLine     BorderLine = "rounded"
)

// maxBorderWidth is the thickest border there is. A thicker one is not drawn
// thicker, so accepting it would be storing a number nothing reads.
const maxBorderWidth = 5

// BorderValue is the value of a style item that is a border.
type BorderValue struct {
	Source BorderSource        `yaml:"source" json:"source"`
	Line   BorderLine          `yaml:"line,omitempty" json:"line,omitempty"`
	Width  int                 `yaml:"width,omitempty" json:"width,omitempty"`
	From   *StyleItemReference `yaml:"from,omitempty" json:"from,omitempty"`
}

// StyleItemValue is what a style item is worth. Exactly one of the three is
// filled, and which one is decided by the item's type: the type and the value
// are two halves of one statement, and halves that disagree describe nothing.
type StyleItemValue struct {
	Color  *ColorValue  `yaml:"color,omitempty" json:"color,omitempty"`
	Font   *FontValue   `yaml:"font,omitempty" json:"font,omitempty"`
	Border *BorderValue `yaml:"border,omitempty" json:"border,omitempty"`
}

// StyleItemDefinition is one named piece a look is assembled from.
type StyleItemDefinition struct {
	Format  int            `yaml:"format" json:"format"`
	ID      uuid.UUID      `yaml:"id" json:"id"`
	Name    string         `yaml:"name" json:"name"`
	Title   LocalizedText  `yaml:"title" json:"title"`
	Comment string         `yaml:"comment,omitempty" json:"comment,omitempty"`
	Type    StyleItemType  `yaml:"type" json:"type"`
	Value   StyleItemValue `yaml:"value" json:"value"`
}

// StyleItemSetting is what one style makes of one style item: the same item,
// a value of this look's own. A style that gave every item the item's own
// value would be a style that changes nothing.
type StyleItemSetting struct {
	Item  uuid.UUID      `yaml:"item" json:"item"`
	Value StyleItemValue `yaml:"value" json:"value"`
}

// StyleDefinition is one look: a set of style items and what this look makes
// of them.
//
// A project has one active style and no personal or role-wise overrides, so a
// style is a choice made once for the whole configuration rather than a
// preference carried per user.
type StyleDefinition struct {
	Format  int                `yaml:"format" json:"format"`
	ID      uuid.UUID          `yaml:"id" json:"id"`
	Name    string             `yaml:"name" json:"name"`
	Title   LocalizedText      `yaml:"title" json:"title"`
	Comment string             `yaml:"comment,omitempty" json:"comment,omitempty"`
	Items   []StyleItemSetting `yaml:"items,omitempty" json:"items,omitempty"`
}

// DecodeStyleItem reads and validates one style item.
func DecodeStyleItem(source string, reader io.Reader, manifest project.Project) (StyleItemDefinition, error) {
	var value StyleItemDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return StyleItemDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	issues = append(issues, validateStyleItemValue("value", value.Type, value.Value, true)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return StyleItemDefinition{}, err
	}
	return value, nil
}

// DecodeStyle reads and validates one style.
func DecodeStyle(source string, reader io.Reader, manifest project.Project) (StyleDefinition, error) {
	var value StyleDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return StyleDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	if len(value.Items) > maxStyleItemsPerStyle {
		issues = append(issues, fmt.Sprintf("items must not contain more than %d items", maxStyleItemsPerStyle))
	} else {
		seen := map[uuid.UUID]bool{}
		for index, setting := range value.Items {
			prefix := fmt.Sprintf("items[%d]", index)
			if setting.Item.IsZero() {
				issues = append(issues, prefix+".item must be a non-zero UUID")
			}
			if seen[setting.Item] {
				issues = append(issues, prefix+".item is set twice by one style")
			}
			seen[setting.Item] = true
			// Which of the three the value must be is decided by the item this
			// setting is for, and that is known only once the whole project is
			// read; here the value is checked for being one value at all.
			issues = append(issues, validateStyleItemValue(prefix+".value", "", setting.Value, false)...)
		}
	}
	if err := issuesError(source, value.Format, issues); err != nil {
		return StyleDefinition{}, err
	}
	return value, nil
}

// validateStyleItemValue checks that a value is exactly one of the three and
// that the one it is is complete. withType says whether the caller knows which
// of the three it has to be - a style item declares its type beside the value,
// a style's setting takes it from the item it is for.
func validateStyleItemValue(path string, itemType StyleItemType, value StyleItemValue, withType bool) []string {
	var issues []string
	if withType {
		switch itemType {
		case ColorStyleItem, FontStyleItem, BorderStyleItem:
		default:
			return []string{"type must be color, font or border"}
		}
	}
	filled := 0
	for _, set := range []bool{value.Color != nil, value.Font != nil, value.Border != nil} {
		if set {
			filled++
		}
	}
	switch filled {
	case 1:
	case 0:
		return append(issues, path+" must be a colour, a font or a border")
	default:
		return append(issues, path+" is more than one of a colour, a font and a border")
	}
	if withType {
		mismatch := (itemType == ColorStyleItem && value.Color == nil) ||
			(itemType == FontStyleItem && value.Font == nil) ||
			(itemType == BorderStyleItem && value.Border == nil)
		if mismatch {
			return append(issues, fmt.Sprintf("%s is not a %s, which is what this style item is", path, itemType))
		}
	}
	switch {
	case value.Color != nil:
		issues = append(issues, validateColorValue(path+".color", *value.Color)...)
	case value.Font != nil:
		issues = append(issues, validateFontValue(path+".font", *value.Font)...)
	case value.Border != nil:
		issues = append(issues, validateBorderValue(path+".border", *value.Border)...)
	}
	return issues
}

func validateColorValue(path string, value ColorValue) []string {
	var issues []string
	switch value.Source {
	case AbsoluteColor:
		if !validHexColor(value.RGB) {
			issues = append(issues, path+".rgb must be a colour written as #RRGGBB")
		}
	case WebColor, SystemColor:
		if !validIdentifier(value.Name) {
			issues = append(issues, path+".name must name a colour of the palette")
		}
	case AutoColor:
	case StyleColor:
		issues = append(issues, validateStyleItemReference(path+".from", value.From)...)
	default:
		return []string{path + ".source must be absolute, web, system, auto or style"}
	}
	if value.Source != AbsoluteColor && value.RGB != "" {
		issues = append(issues, path+".rgb belongs to an absolute colour only")
	}
	if value.Source != WebColor && value.Source != SystemColor && value.Name != "" {
		issues = append(issues, path+".name belongs to a colour of a palette only")
	}
	if value.Source != StyleColor && value.From != nil {
		issues = append(issues, path+".from belongs to a colour taken from a style item only")
	}
	return issues
}

func validateFontValue(path string, value FontValue) []string {
	var issues []string
	switch value.Source {
	case AbsoluteFont:
		if value.Face == "" {
			issues = append(issues, path+".face must name the font")
		}
		// An absolute font has nothing to take a size from.
		if value.Size <= 0 {
			issues = append(issues, path+".size must be a positive height")
		}
	case SystemFont:
		if value.Face == "" {
			issues = append(issues, path+".face must name the font of the system")
		}
	case AutoFont:
	case StyleFont:
		issues = append(issues, validateStyleItemReference(path+".from", value.From)...)
	default:
		return []string{path + ".source must be absolute, system, auto or style"}
	}
	if value.Source != AbsoluteFont && value.Source != SystemFont && value.Face != "" {
		issues = append(issues, path+".face belongs to a font named by its face only")
	}
	if value.Source != StyleFont && value.From != nil {
		issues = append(issues, path+".from belongs to a font taken from a style item only")
	}
	if value.Size < 0 {
		issues = append(issues, path+".size must not be negative")
	}
	if value.Scale < 0 || value.Scale > 1000 {
		issues = append(issues, path+".scale must be a percentage between 0 and 1000")
	}
	return issues
}

func validateBorderValue(path string, value BorderValue) []string {
	var issues []string
	switch value.Source {
	case AbsoluteBorder:
		if !validBorderLine(value.Line) {
			issues = append(issues, path+".line is not a way a border is drawn")
		}
	case StyleBorder:
		issues = append(issues, validateStyleItemReference(path+".from", value.From)...)
		if value.Line != "" {
			issues = append(issues, path+".line belongs to an absolute border only")
		}
	default:
		return []string{path + ".source must be absolute or style"}
	}
	if value.Source != StyleBorder && value.From != nil {
		issues = append(issues, path+".from belongs to a border taken from a style item only")
	}
	if value.Width < 0 || value.Width > maxBorderWidth {
		issues = append(issues, fmt.Sprintf("%s.width must be between 0 and %d", path, maxBorderWidth))
	}
	return issues
}

// validateStyleItemReference checks that a value taken from a style item names
// one source, not two and not none - the same rule a picture is named by.
func validateStyleItemReference(path string, reference *StyleItemReference) []string {
	if reference == nil {
		return []string{path + " must name the style item the value is taken from"}
	}
	switch {
	case reference.Standard != "" && reference.Item != nil:
		return []string{path + " names both a standard style item and one of the configuration"}
	case reference.Standard == "" && reference.Item == nil:
		return []string{path + " must name a standard style item or one of the configuration"}
	case reference.Standard != "" && !validIdentifier(reference.Standard):
		return []string{path + ".standard must be a valid identifier"}
	case reference.Item != nil && reference.Item.IsZero():
		return []string{path + ".item must be a non-zero UUID"}
	}
	return nil
}

func validBorderLine(line BorderLine) bool {
	switch line {
	case NoBorderLine, SingleBorderLine, DoubleBorderLine, IndentedBorderLine, EmbossedBorderLine,
		UnderlineBorderLine, DoubleUnderlineBorder, OverlineBorderLine, RoundedBorderLine:
		return true
	default:
		return false
	}
}

func validHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, symbol := range value[1:] {
		switch {
		case symbol >= '0' && symbol <= '9', symbol >= 'a' && symbol <= 'f', symbol >= 'A' && symbol <= 'F':
		default:
			return false
		}
	}
	return true
}

func cloneStyleItemValue(value StyleItemValue) StyleItemValue {
	if value.Color != nil {
		colour := *value.Color
		colour.From = cloneStyleItemReference(colour.From)
		value.Color = &colour
	}
	if value.Font != nil {
		font := *value.Font
		font.From = cloneStyleItemReference(font.From)
		value.Font = &font
	}
	if value.Border != nil {
		border := *value.Border
		border.From = cloneStyleItemReference(border.From)
		value.Border = &border
	}
	return value
}

func cloneStyleItemReference(reference *StyleItemReference) *StyleItemReference {
	if reference == nil {
		return nil
	}
	copied := *reference
	if reference.Item != nil {
		id := *reference.Item
		copied.Item = &id
	}
	return &copied
}

func cloneStyleItem(value StyleItemDefinition) StyleItemDefinition {
	value.Title = cloneTitle(value.Title)
	value.Value = cloneStyleItemValue(value.Value)
	return value
}

func cloneStyle(value StyleDefinition) StyleDefinition {
	value.Title = cloneTitle(value.Title)
	items := make([]StyleItemSetting, len(value.Items))
	for index, setting := range value.Items {
		setting.Value = cloneStyleItemValue(setting.Value)
		items[index] = setting
	}
	value.Items = items
	return value
}

// StyleItem returns one style item by name, folded case.
func (catalog *Catalog) StyleItem(name string) (StyleItemDefinition, bool) {
	index, ok := catalog.styleItemByName[strings.ToLower(name)]
	if !ok {
		return StyleItemDefinition{}, false
	}
	return cloneStyleItem(catalog.StyleItems[index]), true
}

// StyleItemByID returns one style item by identifier - which is how a style
// and another style item refer to it.
func (catalog *Catalog) StyleItemByID(id uuid.UUID) (StyleItemDefinition, bool) {
	index, ok := catalog.styleItemByID[id]
	if !ok {
		return StyleItemDefinition{}, false
	}
	return cloneStyleItem(catalog.StyleItems[index]), true
}

// Style returns one style by name, folded case.
func (catalog *Catalog) Style(name string) (StyleDefinition, bool) {
	index, ok := catalog.styleByName[strings.ToLower(name)]
	if !ok {
		return StyleDefinition{}, false
	}
	return cloneStyle(catalog.Styles[index]), true
}

// StyleByID returns one style by identifier - which is how the configuration
// root names the style the application is drawn with.
func (catalog *Catalog) StyleByID(id uuid.UUID) (StyleDefinition, bool) {
	index, ok := catalog.styleByID[id]
	if !ok {
		return StyleDefinition{}, false
	}
	return cloneStyle(catalog.Styles[index]), true
}

// validateStyleReferences resolves everything a style and a style item point
// at: the items a style sets, and the item a value is taken from.
//
// A value taken from an item of another type describes nothing - a colour
// cannot be a font - and a value taken, through however many steps, from
// itself has no value at all. Both would be found only when a form is drawn,
// which is far from where the mistake was made.
func (catalog *Catalog) validateStyleReferences() error {
	for _, item := range catalog.StyleItems {
		if err := catalog.resolveStyleItemSource("style item "+item.Name, item.Type, item.Value); err != nil {
			return err
		}
	}
	for _, style := range catalog.Styles {
		for _, setting := range style.Items {
			index, ok := catalog.styleItemByID[setting.Item]
			if !ok {
				return fmt.Errorf("style %s sets unknown style item %s", style.Name, setting.Item)
			}
			item := catalog.StyleItems[index]
			owner := fmt.Sprintf("style %s, style item %s", style.Name, item.Name)
			if issues := validateStyleItemValue("value", item.Type, setting.Value, true); len(issues) > 0 {
				return fmt.Errorf("%s: %s", owner, strings.Join(issues, "; "))
			}
			if err := catalog.resolveStyleItemSource(owner, item.Type, setting.Value); err != nil {
				return err
			}
		}
	}
	return catalog.validateStyleItemCycles()
}

// resolveStyleItemSource checks the item a value is taken from, when it is
// taken from one of the configuration's own rather than from the platform's
// standard set.
func (catalog *Catalog) resolveStyleItemSource(owner string, itemType StyleItemType, value StyleItemValue) error {
	reference := styleItemSource(value)
	if reference == nil || reference.Item == nil {
		return nil
	}
	index, ok := catalog.styleItemByID[*reference.Item]
	if !ok {
		return fmt.Errorf("%s takes its value from unknown style item %s", owner, reference.Item)
	}
	if source := catalog.StyleItems[index]; source.Type != itemType {
		return fmt.Errorf("%s takes its value from style item %s, which is a %s and not a %s",
			owner, source.Name, source.Type, itemType)
	}
	return nil
}

// styleItemSource returns the reference a value is taken from, whichever of
// the three the value is.
func styleItemSource(value StyleItemValue) *StyleItemReference {
	switch {
	case value.Color != nil && value.Color.Source == StyleColor:
		return value.Color.From
	case value.Font != nil && value.Font.Source == StyleFont:
		return value.Font.From
	case value.Border != nil && value.Border.Source == StyleBorder:
		return value.Border.From
	default:
		return nil
	}
}

// validateStyleItemCycles refuses a chain of style items that comes back to
// where it started. Such a chain has no value at the end of it, and nothing
// short of walking it says so.
func (catalog *Catalog) validateStyleItemCycles() error {
	const (
		visiting = 1
		visited  = 2
	)
	state := make(map[uuid.UUID]int, len(catalog.StyleItems))
	var walk func(id uuid.UUID, path []string) error
	walk = func(id uuid.UUID, path []string) error {
		switch state[id] {
		case visited:
			return nil
		case visiting:
			return fmt.Errorf("style items take their value from each other in a circle: %s",
				strings.Join(append(path, path[0]), " -> "))
		}
		state[id] = visiting
		index, ok := catalog.styleItemByID[id]
		if ok {
			if reference := styleItemSource(catalog.StyleItems[index].Value); reference != nil && reference.Item != nil {
				if err := walk(*reference.Item, append(path, catalog.StyleItems[index].Name)); err != nil {
					return err
				}
			}
		}
		state[id] = visited
		return nil
	}
	for _, item := range catalog.StyleItems {
		if err := walk(item.ID, nil); err != nil {
			return err
		}
	}
	return nil
}
