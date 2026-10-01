package metadata

import (
	"fmt"
	"slices"

	"github.com/k33alexey/MetaLab/internal/project"
)

// The drawing of a route map, as the designer saves it. None of it changes
// where a process goes; all of it is what the developer drew, and a map that
// arrives with its colours, fonts, ports and margins reset is a map they have
// to draw again. The values are the prototype's own lists, written the way
// this model writes names.

// RouteStrokeStyle is how a line or a border is drawn.
type RouteStrokeStyle string

const (
	NoStroke               RouteStrokeStyle = "none"
	SolidStroke            RouteStrokeStyle = "solid"
	DottedStroke           RouteStrokeStyle = "dotted"
	DashedStroke           RouteStrokeStyle = "dashed"
	DashDottedStroke       RouteStrokeStyle = "dash-dotted"
	DashDottedDottedStroke RouteStrokeStyle = "dash-dotted-dotted"
)

// RouteStroke is a line of the map or the border of a box on it: the style,
// the width and whether the line leaves a gap where another crosses it.
type RouteStroke struct {
	Style RouteStrokeStyle `yaml:"style" json:"style"`
	Width int              `yaml:"width,omitempty" json:"width,omitempty"`
	Gap   bool             `yaml:"gap,omitempty" json:"gap,omitempty"`
}

// RouteArrow is how an end of a line is drawn.
type RouteArrow string

const (
	NoArrow     RouteArrow = "none"
	FilledArrow RouteArrow = "filled"
	BlankArrow  RouteArrow = "blank"
)

// RouteTextLocation is where the caption of a line stands.
type RouteTextLocation string

const (
	FirstSegmentText RouteTextLocation = "first-segment"
	MiddleText       RouteTextLocation = "middle"
)

// RouteGridMode is how the grid of the map is drawn.
type RouteGridMode string

const (
	NoGrid    RouteGridMode = "none"
	DotsGrid  RouteGridMode = "dots"
	ChessGrid RouteGridMode = "chess"
	LinesGrid RouteGridMode = "lines"
)

// RouteFitPage is how the map is fitted to a printed page.
type RouteFitPage string

const (
	AutoFitPage           RouteFitPage = "auto"
	PageWidthFitPage      RouteFitPage = "page-width"
	ProportionallyFitPage RouteFitPage = "proportionally"
)

var (
	routeHorizontalAligns = []string{"auto", "center", "justify", "left", "right"}
	routeVerticalAligns   = []string{"top", "center", "bottom"}
	routePictureLocations = []string{"left", "right", "top", "bottom", "center"}
	routePictureSizes     = []string{"auto-size", "auto-size-ignore-scale", "by-font-size", "proportionally", "real-size", "real-size-ignore-scale", "stretch", "tile"}
	routeShapes           = []string{"Block", "Document", "DownArrow", "Ellipse", "File", "Folder", "HorizontalBrackets", "LeftArrow", "LeftRightArrow", "None", "RightArrow", "UpArrow", "UpDownArrow", "VerticalBrackets"}
	routeStrokeStyles     = []RouteStrokeStyle{NoStroke, SolidStroke, DottedStroke, DashedStroke, DashDottedStroke, DashDottedDottedStroke}
	routeArrows           = []RouteArrow{NoArrow, FilledArrow, BlankArrow}
	routeTextLocations    = []RouteTextLocation{FirstSegmentText, MiddleText}
	routeGridModes        = []RouteGridMode{NoGrid, DotsGrid, ChessGrid, LinesGrid}
	routeFitPages         = []RouteFitPage{AutoFitPage, PageWidthFitPage, ProportionallyFitPage}
)

// RouteLook is what every item of the map is drawn with. Left out, a value is
// the designer's default; written, it is what the developer chose.
//
// GroupNumber is the group of items moved together on the drawing, TabOrder
// the order focus walks the items in, and ZOrder which item lies over which.
type RouteLook struct {
	ToolTip         LocalizedText `yaml:"tooltip,omitempty" json:"tooltip,omitempty"`
	TabOrder        int           `yaml:"tab_order,omitempty" json:"tabOrder,omitempty"`
	ZOrder          int           `yaml:"z_order,omitempty" json:"zOrder,omitempty"`
	GroupNumber     int           `yaml:"group_number,omitempty" json:"groupNumber,omitempty"`
	BackColor       *ColorValue   `yaml:"back_color,omitempty" json:"backColor,omitempty"`
	TextColor       *ColorValue   `yaml:"text_color,omitempty" json:"textColor,omitempty"`
	LineColor       *ColorValue   `yaml:"line_color,omitempty" json:"lineColor,omitempty"`
	Font            *FontValue    `yaml:"font,omitempty" json:"font,omitempty"`
	HorizontalAlign string        `yaml:"horizontal_align,omitempty" json:"horizontalAlign,omitempty"`
	VerticalAlign   string        `yaml:"vertical_align,omitempty" json:"verticalAlign,omitempty"`
	PictureLocation string        `yaml:"picture_location,omitempty" json:"pictureLocation,omitempty"`
	Hyperlink       bool          `yaml:"hyperlink,omitempty" json:"hyperlink,omitempty"`
	Transparent     bool          `yaml:"transparent,omitempty" json:"transparent,omitempty"`
	// Border, Picture and PictureSize are what a box adds: a point or a
	// shape drawn on the map. A line has none of them, and a shape has a
	// picture but no border of its own - its outline is the shape.
	Border      *RouteStroke      `yaml:"border,omitempty" json:"border,omitempty"`
	Picture     *PictureReference `yaml:"picture,omitempty" json:"picture,omitempty"`
	PictureSize string            `yaml:"picture_size,omitempty" json:"pictureSize,omitempty"`
}

// RouteEnd is where a line is attached: to an item of the map, by its name,
// at one of its ports. A decorative line may leave an end free.
type RouteEnd struct {
	Item string `yaml:"item,omitempty" json:"item,omitempty"`
	Port int    `yaml:"port,omitempty" json:"port,omitempty"`
}

// RouteSegment is a segment of a line the developer moved by hand. The
// designer keeps it apart from the corners, because a segment moved by hand
// stays where it was put when the items it joins are moved.
type RouteSegment struct {
	Index int         `yaml:"index" json:"index"`
	Start RouteVertex `yaml:"start" json:"start"`
	End   RouteVertex `yaml:"end" json:"end"`
}

// RouteLineLook is what a line adds to the look of an item: how it is drawn,
// its arrows, where its caption stands and the segments moved by hand.
type RouteLineLook struct {
	Stroke       *RouteStroke      `yaml:"stroke,omitempty" json:"stroke,omitempty"`
	BeginArrow   RouteArrow        `yaml:"begin_arrow,omitempty" json:"beginArrow,omitempty"`
	EndArrow     RouteArrow        `yaml:"end_arrow,omitempty" json:"endArrow,omitempty"`
	TextLocation RouteTextLocation `yaml:"text_location,omitempty" json:"textLocation,omitempty"`
	Segments     []RouteSegment    `yaml:"segments,omitempty" json:"segments,omitempty"`
}

// RoutePrint is how the map is printed: margins in millimetres, colour or
// black and white, and how it is fitted to the page.
type RoutePrint struct {
	TopMargin     int          `yaml:"top_margin,omitempty" json:"topMargin,omitempty"`
	LeftMargin    int          `yaml:"left_margin,omitempty" json:"leftMargin,omitempty"`
	BottomMargin  int          `yaml:"bottom_margin,omitempty" json:"bottomMargin,omitempty"`
	RightMargin   int          `yaml:"right_margin,omitempty" json:"rightMargin,omitempty"`
	BlackAndWhite bool         `yaml:"black_and_white,omitempty" json:"blackAndWhite,omitempty"`
	FitPage       RouteFitPage `yaml:"fit_page,omitempty" json:"fitPage,omitempty"`
}

// RouteMapLook is what the map as a whole is drawn and printed with.
type RouteMapLook struct {
	BackColor *ColorValue   `yaml:"back_color,omitempty" json:"backColor,omitempty"`
	Grid      bool          `yaml:"grid,omitempty" json:"grid,omitempty"`
	GridMode  RouteGridMode `yaml:"grid_mode,omitempty" json:"gridMode,omitempty"`
	// GridHorizontalStep and GridVerticalStep are the cell of the grid the
	// items snap to.
	GridHorizontalStep int         `yaml:"grid_horizontal_step,omitempty" json:"gridHorizontalStep,omitempty"`
	GridVerticalStep   int         `yaml:"grid_vertical_step,omitempty" json:"gridVerticalStep,omitempty"`
	Print              *RoutePrint `yaml:"print,omitempty" json:"print,omitempty"`
}

func validateRouteLook(path string, look *RouteLook, box bool, configuration project.Project) []string {
	if look == nil {
		return nil
	}
	var issues []string
	if len(look.ToolTip) > 0 {
		issues = append(issues, validateTitle(path+".tooltip", look.ToolTip, configuration)...)
	}
	if look.TabOrder < 0 || look.ZOrder < 0 || look.GroupNumber < 0 {
		issues = append(issues, path+" orders and groups are counted from zero")
	}
	for name, color := range map[string]*ColorValue{"back_color": look.BackColor, "text_color": look.TextColor, "line_color": look.LineColor} {
		if color != nil {
			issues = append(issues, validateColorValue(path+"."+name, *color)...)
		}
	}
	if look.Font != nil {
		issues = append(issues, validateFontValue(path+".font", *look.Font)...)
	}
	issues = append(issues, validateRouteChoice(path+".horizontal_align", look.HorizontalAlign, routeHorizontalAligns)...)
	issues = append(issues, validateRouteChoice(path+".vertical_align", look.VerticalAlign, routeVerticalAligns)...)
	issues = append(issues, validateRouteChoice(path+".picture_location", look.PictureLocation, routePictureLocations)...)
	issues = append(issues, validateRouteChoice(path+".picture_size", look.PictureSize, routePictureSizes)...)
	issues = append(issues, validatePictureReference(path+".picture", look.Picture)...)
	issues = append(issues, validateRouteStroke(path+".border", look.Border)...)
	if !box && (look.Border != nil || look.Picture != nil || look.PictureSize != "") {
		issues = append(issues, path+" gives a line a border or a picture, which only a box on the map has")
	}
	return issues
}

func validateRouteLineLook(path string, look RouteLineLook) []string {
	var issues []string
	issues = append(issues, validateRouteStroke(path+".stroke", look.Stroke)...)
	if look.BeginArrow != "" && !slices.Contains(routeArrows, look.BeginArrow) {
		issues = append(issues, path+".begin_arrow must be none, filled or blank")
	}
	if look.EndArrow != "" && !slices.Contains(routeArrows, look.EndArrow) {
		issues = append(issues, path+".end_arrow must be none, filled or blank")
	}
	if look.TextLocation != "" && !slices.Contains(routeTextLocations, look.TextLocation) {
		issues = append(issues, path+".text_location must be first-segment or middle")
	}
	seen := map[int]bool{}
	for index, segment := range look.Segments {
		if segment.Index < 0 || seen[segment.Index] {
			issues = append(issues, fmt.Sprintf("%s.segments[%d].index must be a segment of the line, once", path, index))
		}
		seen[segment.Index] = true
	}
	return issues
}

func validateRouteStroke(path string, stroke *RouteStroke) []string {
	if stroke == nil {
		return nil
	}
	var issues []string
	if !slices.Contains(routeStrokeStyles, stroke.Style) {
		issues = append(issues, path+".style is not a way a line is drawn")
	}
	if stroke.Width < 0 {
		issues = append(issues, path+".width must not be negative")
	}
	return issues
}

func validateRouteMapLook(look *RouteMapLook) []string {
	if look == nil {
		return nil
	}
	var issues []string
	if look.BackColor != nil {
		issues = append(issues, validateColorValue("route.look.back_color", *look.BackColor)...)
	}
	if look.GridMode != "" && !slices.Contains(routeGridModes, look.GridMode) {
		issues = append(issues, "route.look.grid_mode must be none, dots, chess or lines")
	}
	if look.GridHorizontalStep < 0 || look.GridVerticalStep < 0 {
		issues = append(issues, "route.look grid steps must not be negative")
	}
	if print := look.Print; print != nil {
		if print.TopMargin < 0 || print.LeftMargin < 0 || print.BottomMargin < 0 || print.RightMargin < 0 {
			issues = append(issues, "route.look.print margins must not be negative")
		}
		if print.FitPage != "" && !slices.Contains(routeFitPages, print.FitPage) {
			issues = append(issues, "route.look.print.fit_page must be auto, page-width or proportionally")
		}
	}
	return issues
}

func validateRouteChoice(path, value string, allowed []string) []string {
	if value == "" || slices.Contains(allowed, value) {
		return nil
	}
	return []string{fmt.Sprintf("%s must be one of %v", path, allowed)}
}

func cloneRouteLook(look *RouteLook) *RouteLook {
	if look == nil {
		return nil
	}
	value := *look
	value.ToolTip = cloneTitle(value.ToolTip)
	value.BackColor = clonePointer(value.BackColor)
	value.TextColor = clonePointer(value.TextColor)
	value.LineColor = clonePointer(value.LineColor)
	value.Font = clonePointer(value.Font)
	value.Border = clonePointer(value.Border)
	value.Picture = clonePointer(value.Picture)
	return &value
}

func cloneRouteLineLook(look RouteLineLook) RouteLineLook {
	look.Stroke = clonePointer(look.Stroke)
	look.Segments = slices.Clone(look.Segments)
	return look
}

func cloneRouteMapLook(look *RouteMapLook) *RouteMapLook {
	if look == nil {
		return nil
	}
	value := *look
	value.BackColor = clonePointer(value.BackColor)
	value.Print = clonePointer(value.Print)
	return &value
}

// clonePointer copies the value a pointer holds. The values cloned with it
// hold pointers of their own only to style items and pictures, which are
// shared by identity and never written through.
func clonePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
