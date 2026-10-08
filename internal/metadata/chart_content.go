package metadata

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
)

// The content a chart attribute of a form is made with in the designer: its
// type, its series and points with their values, how its title, legend, plot
// area and labels look, its scales. The prototype writes it whole into the
// settings of the attribute (31 charts of the exports, 155 to 305 values
// each), and a chart that arrives without it is a chart the developer has to
// draw again. It is carried, not drawn: a chart is drawn in block 8.
//
// The model follows the help (Chart and the objects it is made of - the
// series, the point, the title, legend and plot areas, the scale, the value):
// what the prototype writes is placed on the property of the help it is. The
// prototype also writes values the help names nowhere - its own state of the
// object; they are carried apart, in ChartState, and the chart is noted.
//
// The values are the prototype's own lists, written the way this model writes
// names; a list is the help's, joined with what the exports write.

// ChartNumber is a number of a chart written as the prototype writes it: a
// decimal of any length. The place of an area is a fraction of the chart
// written to 27 digits ("0.017094017094017094017094017"), which a float
// would not keep.
type ChartNumber string

var chartNumberPattern = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

// ChartContent is the content of a chart.
type ChartContent struct {
	Type ChartKind `yaml:"type" json:"type"`

	// Series and Points are the series and the points of the chart, in the
	// order of the chart. SeriesCount and PointCount are their numbers as the
	// prototype writes them apart from the lists: once in the exports a
	// chart counts a series it describes nothing of (sb), so the number is
	// kept and the list may be shorter.
	Series      []ChartSeries `yaml:"series,omitempty" json:"series,omitempty"`
	SeriesCount int           `yaml:"series_count,omitempty" json:"seriesCount,omitempty"`
	Points      []ChartPoint  `yaml:"points,omitempty" json:"points,omitempty"`
	PointCount  int           `yaml:"point_count,omitempty" json:"pointCount,omitempty"`
	// SummarySeries is the series that sums the others (help,
	// Chart.SummarySeries), described as a series is.
	SummarySeries *ChartSeries `yaml:"summary_series,omitempty" json:"summarySeries,omitempty"`
	// Values are the values of the chart, one for each series and point,
	// one list as the prototype writes it: series by series, each with all
	// its points in their order - the first series at every point, then the
	// second (a run of the platform, 08.10.2026: 2 series × 3 points set by
	// code and serialized). ValueAt reads it.
	Values []ChartValue `yaml:"values,omitempty" json:"values,omitempty"`
	// ActiveSeries and ActivePoint are the series and the point the chart
	// stands on, by their number from 0; -1 is none (help,
	// Chart.ActiveSeries, Chart.ActivePoint).
	ActiveSeries int `yaml:"active_series" json:"activeSeries"`
	ActivePoint  int `yaml:"active_point" json:"activePoint"`
	// NextSeriesID and NextPointID are what the next series and point added
	// in the designer are numbered with.
	NextSeriesID int `yaml:"next_series_id,omitempty" json:"nextSeriesId,omitempty"`
	NextPointID  int `yaml:"next_point_id,omitempty" json:"nextPointId,omitempty"`
	// AutoSeriesText and AutoPointText name a series and a point by itself
	// where its text is not given.
	AutoSeriesText bool `yaml:"auto_series_text,omitempty" json:"autoSeriesText,omitempty"`
	AutoPointText  bool `yaml:"auto_point_text,omitempty" json:"autoPointText,omitempty"`

	Labels     ChartLabels     `yaml:"labels" json:"labels"`
	TitleArea  ChartTitleArea  `yaml:"title_area" json:"titleArea"`
	LegendArea ChartLegendArea `yaml:"legend_area" json:"legendArea"`
	PlotArea   ChartPlotArea   `yaml:"plot_area" json:"plotArea"`

	Transparent bool         `yaml:"transparent,omitempty" json:"transparent,omitempty"`
	BackColor   *ColorValue  `yaml:"back_color,omitempty" json:"backColor,omitempty"`
	Border      *BorderValue `yaml:"border,omitempty" json:"border,omitempty"`
	BorderColor *ColorValue  `yaml:"border_color,omitempty" json:"borderColor,omitempty"`

	// MaxSeries limits the series shown, by count or by per cent (help,
	// Chart.MaxSeries, MaxSeriesCount, MaxSeriesPercent).
	MaxSeries        ChartMaxSeries `yaml:"max_series,omitempty" json:"maxSeries,omitempty"`
	MaxSeriesCount   int            `yaml:"max_series_count,omitempty" json:"maxSeriesCount,omitempty"`
	MaxSeriesPercent int            `yaml:"max_series_percent,omitempty" json:"maxSeriesPercent,omitempty"`
	// AutoMaxValue and AutoMinValue take the bounds of the values axis from
	// the values; MaxValue and MinValue are the bounds otherwise.
	AutoMaxValue  bool        `yaml:"auto_max_value,omitempty" json:"autoMaxValue,omitempty"`
	MaxValue      ChartNumber `yaml:"max_value,omitempty" json:"maxValue,omitempty"`
	AutoMinValue  bool        `yaml:"auto_min_value,omitempty" json:"autoMinValue,omitempty"`
	MinValue      ChartNumber `yaml:"min_value,omitempty" json:"minValue,omitempty"`
	BaseValue     ChartNumber `yaml:"base_value,omitempty" json:"baseValue,omitempty"`
	HideBaseValue bool        `yaml:"hide_base_value,omitempty" json:"hideBaseValue,omitempty"`

	SpaceMode            ChartSpaceMode            `yaml:"space_mode,omitempty" json:"spaceMode,omitempty"`
	AutoSeriesSeparation ChartAutoSeriesSeparation `yaml:"auto_series_separation,omitempty" json:"autoSeriesSeparation,omitempty"`
	Orientation          ChartOrientation          `yaml:"orientation,omitempty" json:"orientation,omitempty"`
	Outline              bool                      `yaml:"outline,omitempty" json:"outline,omitempty"`
	Light                bool                      `yaml:"light,omitempty" json:"light,omitempty"`
	Gradient             bool                      `yaml:"gradient,omitempty" json:"gradient,omitempty"`
	AutoTransposition    bool                      `yaml:"auto_transposition,omitempty" json:"autoTransposition,omitempty"`
	Animation            ChartAnimation            `yaml:"animation,omitempty" json:"animation,omitempty"`
	SelectionMode        ChartSelectionMode        `yaml:"selection_mode,omitempty" json:"selectionMode,omitempty"`
	// ColorPalette is the palette the series are coloured from; the
	// prototype writes it twice, as the palette and again in the
	// description of the palette, and the second only now and then.
	ColorPalette            ChartColorPalette  `yaml:"color_palette,omitempty" json:"colorPalette,omitempty"`
	ColorPaletteDescription *ChartColorPalette `yaml:"color_palette_description,omitempty" json:"colorPaletteDescription,omitempty"`

	SplineMode              ChartSplineMode `yaml:"spline_mode,omitempty" json:"splineMode,omitempty"`
	SplineStrain            int             `yaml:"spline_strain,omitempty" json:"splineStrain,omitempty"`
	SemitransparencyPercent int             `yaml:"semitransparency_percent,omitempty" json:"semitransparencyPercent,omitempty"`
	DonutChartInnerRadius   int             `yaml:"donut_chart_inner_radius,omitempty" json:"donutChartInnerRadius,omitempty"`
	FunnelNeckHeight        int             `yaml:"funnel_neck_height,omitempty" json:"funnelNeckHeight,omitempty"`
	FunnelNeckWidth         int             `yaml:"funnel_neck_width,omitempty" json:"funnelNeckWidth,omitempty"`
	FunnelSpace             int             `yaml:"funnel_space,omitempty" json:"funnelSpace,omitempty"`
	Gauge                   ChartGauge      `yaml:"gauge" json:"gauge"`
	// DataSourceMode says the chart is filled from a data source rather
	// than point by point (help, Chart.DataSource); no chart of the exports
	// is, and none names a source.
	DataSourceMode bool `yaml:"data_source_mode,omitempty" json:"dataSourceMode,omitempty"`

	State ChartState `yaml:"state" json:"state"`
}

// ChartSeries is one series, or the summary series (help, ChartSeries).
type ChartSeries struct {
	// ID is the number the series is known by in the chart.
	ID            int           `yaml:"id" json:"id"`
	Text          LocalizedText `yaml:"text,omitempty" json:"text,omitempty"`
	Color         *ColorValue   `yaml:"color,omitempty" json:"color,omitempty"`
	ColorPriority bool          `yaml:"color_priority,omitempty" json:"colorPriority,omitempty"`
	Indicator     bool          `yaml:"indicator,omitempty" json:"indicator,omitempty"`
	Line          *RouteStroke  `yaml:"line,omitempty" json:"line,omitempty"`
	Marker        ChartMarker   `yaml:"marker,omitempty" json:"marker,omitempty"`
	// ShowInChart is the help's ShowGraphicalDataRepresentationInChart.
	ShowInChart ChartShowMode `yaml:"show_in_chart,omitempty" json:"showInChart,omitempty"`
	// TextChanged and Expand are written for every series and point and are
	// named nowhere in the help.
	TextChanged bool `yaml:"text_changed,omitempty" json:"textChanged,omitempty"`
	Expand      bool `yaml:"expand,omitempty" json:"expand,omitempty"`
}

// ChartPoint is one point (help, ChartPoint); the prototype writes it as it
// writes a series, a line and a marker of its own included.
type ChartPoint = ChartSeries

// ChartValue is the value of one series at one point (help, ChartValue):
// the number, and its tooltip. The prototype writes every value a number,
// with no details.
type ChartValue struct {
	Value   ChartNumber `yaml:"value" json:"value"`
	ToolTip string      `yaml:"tool_tip,omitempty" json:"toolTip,omitempty"`
}

// ValueAt is the value of a series at a point, both by their number from 0;
// false is a chart that holds no value there.
func (value *ChartContent) ValueAt(series, point int) (ChartValue, bool) {
	if series < 0 || series >= value.SeriesCount || point < 0 || point >= value.PointCount || len(value.Values) == 0 {
		return ChartValue{}, false
	}
	return value.Values[series*value.PointCount+point], true
}

// ChartBounds is where an area stands in the chart, as fractions of it from
// its left and top edge (help, ChartTitleArea.Left and the rest).
type ChartBounds struct {
	Left   ChartNumber `yaml:"left" json:"left"`
	Top    ChartNumber `yaml:"top" json:"top"`
	Right  ChartNumber `yaml:"right" json:"right"`
	Bottom ChartNumber `yaml:"bottom" json:"bottom"`
}

// ChartAreaLook is what the title, the legend and the plot area are drawn
// with alike.
type ChartAreaLook struct {
	Transparent bool         `yaml:"transparent,omitempty" json:"transparent,omitempty"`
	BackColor   *ColorValue  `yaml:"back_color,omitempty" json:"backColor,omitempty"`
	TextColor   *ColorValue  `yaml:"text_color,omitempty" json:"textColor,omitempty"`
	Font        *FontValue   `yaml:"font,omitempty" json:"font,omitempty"`
	Border      *BorderValue `yaml:"border,omitempty" json:"border,omitempty"`
	BorderColor *ColorValue  `yaml:"border_color,omitempty" json:"borderColor,omitempty"`
	Bounds      ChartBounds  `yaml:"bounds" json:"bounds"`
}

// ChartTitleArea is the title (help, ChartTitleArea). Shown is the
// prototype's own switch beside the placement.
type ChartTitleArea struct {
	ChartAreaLook `yaml:",inline"`
	Text          LocalizedText           `yaml:"text,omitempty" json:"text,omitempty"`
	Shown         bool                    `yaml:"shown,omitempty" json:"shown,omitempty"`
	Placement     ChartTitleAreaPlacement `yaml:"placement,omitempty" json:"placement,omitempty"`
}

// ChartLegendArea is the legend (help, ChartLegendArea).
type ChartLegendArea struct {
	ChartAreaLook `yaml:",inline"`
	Shown         bool                 `yaml:"shown,omitempty" json:"shown,omitempty"`
	Placement     ChartLegendPlacement `yaml:"placement,omitempty" json:"placement,omitempty"`
	Scrolling     bool                 `yaml:"scrolling,omitempty" json:"scrolling,omitempty"`
}

// ChartPlotArea is where the chart is drawn, with its scales and its table
// of data (help, ChartPlotArea).
type ChartPlotArea struct {
	ChartAreaLook  `yaml:",inline"`
	Placement      ChartPlotAreaPlacement `yaml:"placement,omitempty" json:"placement,omitempty"`
	ShowScales     bool                   `yaml:"show_scales,omitempty" json:"showScales,omitempty"`
	ScaleLines     *RouteStroke           `yaml:"scale_lines,omitempty" json:"scaleLines,omitempty"`
	ScaleColor     *ColorValue            `yaml:"scale_color,omitempty" json:"scaleColor,omitempty"`
	SurfaceColor   *ColorValue            `yaml:"surface_color,omitempty" json:"surfaceColor,omitempty"`
	RadarScaleType ChartRadarScaleType    `yaml:"radar_scale_type,omitempty" json:"radarScaleType,omitempty"`
	// The table of data under the chart.
	ShowDataTable            bool        `yaml:"show_data_table,omitempty" json:"showDataTable,omitempty"`
	VerticalLinesDataTable   bool        `yaml:"vertical_lines_data_table,omitempty" json:"verticalLinesDataTable,omitempty"`
	HorizontalLinesDataTable bool        `yaml:"horizontal_lines_data_table,omitempty" json:"horizontalLinesDataTable,omitempty"`
	KeysInDataTable          bool        `yaml:"keys_in_data_table,omitempty" json:"keysInDataTable,omitempty"`
	AlignDataTable           ChartAlign  `yaml:"align_data_table,omitempty" json:"alignDataTable,omitempty"`
	DataTableFormat          string      `yaml:"data_table_format,omitempty" json:"dataTableFormat,omitempty"`
	ShowSeriesScale          bool        `yaml:"show_series_scale,omitempty" json:"showSeriesScale,omitempty"`
	ShowPointsScale          bool        `yaml:"show_points_scale,omitempty" json:"showPointsScale,omitempty"`
	ShowValuesScale          bool        `yaml:"show_values_scale,omitempty" json:"showValuesScale,omitempty"`
	PointsScale              *ChartScale `yaml:"points_scale,omitempty" json:"pointsScale,omitempty"`
	SeriesScale              *ChartScale `yaml:"series_scale,omitempty" json:"seriesScale,omitempty"`
	ValuesScale              *ChartScale `yaml:"values_scale,omitempty" json:"valuesScale,omitempty"`
}

// ChartLabels are the labels of the values (help, Chart.LabelType and the
// properties of a label beside it).
type ChartLabels struct {
	Type      ChartLabelType     `yaml:"type,omitempty" json:"type,omitempty"`
	Delimiter string             `yaml:"delimiter,omitempty" json:"delimiter,omitempty"`
	Location  ChartLabelLocation `yaml:"location,omitempty" json:"location,omitempty"`
	// ValueFormat and PercentFormat are format strings, one for each
	// language.
	ValueFormat   LocalizedText `yaml:"value_format,omitempty" json:"valueFormat,omitempty"`
	PercentFormat LocalizedText `yaml:"percent_format,omitempty" json:"percentFormat,omitempty"`
	TextColor     *ColorValue   `yaml:"text_color,omitempty" json:"textColor,omitempty"`
	Font          *FontValue    `yaml:"font,omitempty" json:"font,omitempty"`
	Transparent   bool          `yaml:"transparent,omitempty" json:"transparent,omitempty"`
	BackColor     *ColorValue   `yaml:"back_color,omitempty" json:"backColor,omitempty"`
	Border        *BorderValue  `yaml:"border,omitempty" json:"border,omitempty"`
	BorderColor   *ColorValue   `yaml:"border_color,omitempty" json:"borderColor,omitempty"`
}

// ChartScale is a scale of the plot area (help, ChartScale): what the
// prototype writes of it, which is a few properties at a time.
type ChartScale struct {
	GridLinesShowMode  ChartShowMode           `yaml:"grid_lines_show_mode,omitempty" json:"gridLinesShowMode,omitempty"`
	LabelAngle         int                     `yaml:"label_angle,omitempty" json:"labelAngle,omitempty"`
	LabelFont          *FontValue              `yaml:"label_font,omitempty" json:"labelFont,omitempty"`
	LabelFormat        LocalizedText           `yaml:"label_format,omitempty" json:"labelFormat,omitempty"`
	LabelOrientation   ChartLabelsOrientation  `yaml:"label_orientation,omitempty" json:"labelOrientation,omitempty"`
	MaxLabelRows       int                     `yaml:"max_label_rows,omitempty" json:"maxLabelRows,omitempty"`
	ScaleLabelLocation ChartScaleLabelLocation `yaml:"scale_label_location,omitempty" json:"scaleLabelLocation,omitempty"`
	ScaleLine          *RouteStroke            `yaml:"scale_line,omitempty" json:"scaleLine,omitempty"`
	ScaleLocation      ChartScaleLocation      `yaml:"scale_location,omitempty" json:"scaleLocation,omitempty"`
	ScaleMarkLocation  ChartScaleMarkLocation  `yaml:"scale_mark_location,omitempty" json:"scaleMarkLocation,omitempty"`
	ShowInChart        ChartShowMode           `yaml:"show_in_chart,omitempty" json:"showInChart,omitempty"`
	ShowTitle          ChartShowMode           `yaml:"show_title,omitempty" json:"showTitle,omitempty"`
	TitlePlacement     ChartScaleTitlePlace    `yaml:"title_placement,omitempty" json:"titlePlacement,omitempty"`
	TitleText          LocalizedText           `yaml:"title_text,omitempty" json:"titleText,omitempty"`
	TitleTextMode      ChartScaleTitleText     `yaml:"title_text_mode,omitempty" json:"titleTextMode,omitempty"`
	// TitleArea is how the title of the scale is drawn (help,
	// ChartScale.TitleArea); no place of its own.
	TitleArea *ChartScaleTitleArea `yaml:"title_area,omitempty" json:"titleArea,omitempty"`
}

// ChartScaleTitleArea is the look of the title of a scale.
type ChartScaleTitleArea struct {
	BackColor   *ColorValue  `yaml:"back_color,omitempty" json:"backColor,omitempty"`
	TextColor   *ColorValue  `yaml:"text_color,omitempty" json:"textColor,omitempty"`
	Font        *FontValue   `yaml:"font,omitempty" json:"font,omitempty"`
	Border      *BorderValue `yaml:"border,omitempty" json:"border,omitempty"`
	BorderColor *ColorValue  `yaml:"border_color,omitempty" json:"borderColor,omitempty"`
}

// ChartGauge is what a gauge chart is drawn with (help, the Gauge
// properties of Chart). QualityBands are its coloured bands; the prototype
// writes none, and two switches of its own beside them.
type ChartGauge struct {
	ValueRepresentation    ChartGaugeValue       `yaml:"value_representation,omitempty" json:"valueRepresentation,omitempty"`
	BeginAngle             int                   `yaml:"begin_angle,omitempty" json:"beginAngle,omitempty"`
	EndAngle               int                   `yaml:"end_angle,omitempty" json:"endAngle,omitempty"`
	Thickness              int                   `yaml:"thickness,omitempty" json:"thickness,omitempty"`
	LabelsLocation         ChartGaugeLabelsPlace `yaml:"labels_location,omitempty" json:"labelsLocation,omitempty"`
	LabelsArcDirection     bool                  `yaml:"labels_arc_direction,omitempty" json:"labelsArcDirection,omitempty"`
	BushThickness          int                   `yaml:"bush_thickness,omitempty" json:"bushThickness,omitempty"`
	BushColor              *ColorValue           `yaml:"bush_color,omitempty" json:"bushColor,omitempty"`
	QualityBandsUseText    bool                  `yaml:"quality_bands_use_text,omitempty" json:"qualityBandsUseText,omitempty"`
	QualityBandsUseToolTip bool                  `yaml:"quality_bands_use_tool_tip,omitempty" json:"qualityBandsUseToolTip,omitempty"`
}

// ChartState is what the prototype writes of a chart that the help names
// nowhere: its own state of the object. All of it is carried as written and
// none of it acted upon, and a chart that has it is noted
// (NoteChartStateUnexplained). Each field is the prototype's value of the
// same name.
type ChartState struct {
	RebuildTime         int                    `yaml:"rebuild_time,omitempty" json:"rebuildTime,omitempty"`                   // rebuildTime
	ElementsInitialized bool                   `yaml:"elements_initialized,omitempty" json:"elementsInitialized,omitempty"`   // elementsIsInit
	TitleInitialized    bool                   `yaml:"title_initialized,omitempty" json:"titleInitialized,omitempty"`         // titleIsInit
	LegendInitialized   bool                   `yaml:"legend_initialized,omitempty" json:"legendInitialized,omitempty"`       // legendIsInit
	ChartInitialized    bool                   `yaml:"chart_initialized,omitempty" json:"chartInitialized,omitempty"`         // chartIsInit
	RandomizedNewValues bool                   `yaml:"randomized_new_values,omitempty" json:"randomizedNewValues,omitempty"`  // isRandomizedNewValues
	SeriesDesign        bool                   `yaml:"series_design,omitempty" json:"seriesDesign,omitempty"`                 // isSeriesDesign
	PointsDesign        bool                   `yaml:"points_design,omitempty" json:"pointsDesign,omitempty"`                 // isPointsDesign
	Transposition       bool                   `yaml:"transposition,omitempty" json:"transposition,omitempty"`                // isTransposition
	Transposed          bool                   `yaml:"transposed,omitempty" json:"transposed,omitempty"`                      // isTransposed
	ShowScaleVL         bool                   `yaml:"show_scale_vl,omitempty" json:"showScaleVl,omitempty"`                  // isShowScaleVL
	ValuesScaleFormat   string                 `yaml:"values_scale_format,omitempty" json:"valuesScaleFormat,omitempty"`      // vsFormat
	XLabelsOrientation  ChartLabelsOrientation `yaml:"x_labels_orientation,omitempty" json:"xLabelsOrientation,omitempty"`    // xLabelsOrientation
	PiePoint            int                    `yaml:"pie_point,omitempty" json:"piePoint,omitempty"`                         // realPiePoint
	StockSeries         int                    `yaml:"stock_series,omitempty" json:"stockSeries,omitempty"`                   // realStockSeries
	MultiStageLinkLine  *RouteStroke           `yaml:"multi_stage_link_line,omitempty" json:"multiStageLinkLine,omitempty"`   // multiStageLinkLine
	MultiStageLinkColor *ColorValue            `yaml:"multi_stage_link_color,omitempty" json:"multiStageLinkColor,omitempty"` // multiStageLinkColor
	PointsDropLines     ChartShowMode          `yaml:"points_drop_lines,omitempty" json:"pointsDropLines,omitempty"`          // pointsDropLinesShowMode
}

// IsEmpty reports whether the prototype wrote nothing of its own state.
func (state ChartState) IsEmpty() bool { return state.fields() == "" }

// fields names what of the state is written, for the note.
func (state ChartState) fields() string {
	var names []string
	add := func(name string, set bool) {
		if set {
			names = append(names, name)
		}
	}
	add("rebuild_time", state.RebuildTime != 0)
	add("elements_initialized", state.ElementsInitialized)
	add("title_initialized", state.TitleInitialized)
	add("legend_initialized", state.LegendInitialized)
	add("chart_initialized", state.ChartInitialized)
	add("randomized_new_values", state.RandomizedNewValues)
	add("series_design", state.SeriesDesign)
	add("points_design", state.PointsDesign)
	add("transposition", state.Transposition)
	add("transposed", state.Transposed)
	add("show_scale_vl", state.ShowScaleVL)
	add("values_scale_format", state.ValuesScaleFormat != "")
	add("x_labels_orientation", state.XLabelsOrientation != "")
	add("pie_point", state.PiePoint != 0)
	add("stock_series", state.StockSeries != 0)
	add("multi_stage_link_line", state.MultiStageLinkLine != nil)
	add("multi_stage_link_color", state.MultiStageLinkColor != nil)
	add("points_drop_lines", state.PointsDropLines != "")
	return strings.Join(names, ", ")
}

type (
	ChartKind                 string
	ChartMaxSeries            string
	ChartSpaceMode            string
	ChartAutoSeriesSeparation string
	ChartOrientation          string
	ChartAnimation            string
	ChartSelectionMode        string
	ChartColorPalette         string
	ChartSplineMode           string
	ChartMarker               string
	ChartShowMode             string
	ChartTitleAreaPlacement   string
	ChartLegendPlacement      string
	ChartPlotAreaPlacement    string
	ChartRadarScaleType       string
	ChartAlign                string
	ChartLabelType            string
	ChartLabelLocation        string
	ChartLabelsOrientation    string
	ChartScaleLabelLocation   string
	ChartScaleLocation        string
	ChartScaleMarkLocation    string
	ChartScaleTitlePlace      string
	ChartScaleTitleText       string
	ChartGaugeValue           string
	ChartGaugeLabelsPlace     string
)

// The lists of the help (ChartType, MaxSeries and the rest), each as the
// model writes names.
var (
	chartKinds = []ChartKind{"area", "bar", "bar-3d", "bar-graph", "bubble", "ceil-graph", "column", "column-3d",
		"concave-surface", "convex-surface", "donut", "donut-3d", "funnel", "funnel-3d", "gauge", "honeycomb", "line",
		"normalized-area", "normalized-bar", "normalized-bar-3d", "normalized-column", "normalized-column-3d",
		"normalized-funnel", "normalized-funnel-3d", "open-high-low-close", "pie", "pie-3d", "pyramid-graph",
		"radar-area", "radar-line", "radar-normalized-area", "radar-stacked-area", "radar-stacked-line", "scatter",
		"shaded-surface", "stacked-area", "stacked-bar", "stacked-bar-3d", "stacked-column", "stacked-column-3d",
		"stacked-line", "step", "stock", "surface", "tape-graph", "waterfall", "wireframe-surface"}
	chartMaxSeries        = []ChartMaxSeries{"not-defined", "limited", "percent"}
	chartSpaceModes       = []ChartSpaceMode{"none", "half", "full"}
	chartSeriesSeparation = []ChartAutoSeriesSeparation{"none", "all", "maximum", "minimum"}
	chartOrientations     = []ChartOrientation{"south-west", "south-east"}
	chartAnimations       = []ChartAnimation{"auto", "use", "dont-use"}
	chartSelectionModes   = []ChartSelectionMode{"auto", "none", "points-selection", "values-selection"}
	chartColorPalettes    = []ChartColorPalette{"auto", "blue", "bright", "cold", "custom", "gradient", "gray", "green",
		"orange", "palette-32", "palette-8", "pastel", "soft", "soft-adaptive", "warm", "yellow"}
	chartSplineModes  = []ChartSplineMode{"none", "smooth-curve"}
	chartMarkers      = []ChartMarker{"auto", "none", "alternation", "circle", "rect", "rhomb"}
	chartShowModes    = []ChartShowMode{"auto", "show", "dont-show"}
	chartTitlePlaces  = []ChartTitleAreaPlacement{"auto", "none", "top", "bottom", "left-top", "left-bottom", "right-top", "right-bottom", "use-coordinates"}
	chartLegendPlaces = []ChartLegendPlacement{"auto", "none", "top", "bottom", "left", "right", "use-coordinates"}
	chartPlotPlaces   = []ChartPlotAreaPlacement{"auto", "empty-space", "use-coordinates"}
	chartRadarScales  = []ChartRadarScaleType{"circle", "polygon"}
	chartAligns       = []ChartAlign{"auto", "left", "center", "right", "justify"}
	chartLabelTypes   = []ChartLabelType{"none", "percent", "point", "point-percent", "point-size", "point-value",
		"point-value-percent", "point-value-size", "series", "series-percent", "series-point", "series-point-percent",
		"series-point-size", "series-point-value", "series-point-value-percent", "series-point-value-size",
		"series-size", "series-value", "series-value-percent", "series-value-size", "value", "value-percent", "value-size"}
	chartLabelLocations = []ChartLabelLocation{"auto", "bottom-left", "bottom-right", "center", "edge", "edge-auto",
		"edge-inside", "empty-space", "top-and-left-specified", "top-left", "top-right"}
	chartLabelOrientations = []ChartLabelsOrientation{"auto", "horizontal", "vertical", "custom-angle"}
	chartScaleLabelPlaces  = []ChartScaleLabelLocation{"auto", "inside", "outside", "none"}
	chartScalePlaces       = []ChartScaleLocation{"auto", "base-value", "edge"}
	chartScaleMarkPlaces   = []ChartScaleMarkLocation{"auto", "center", "inside", "outside", "none"}
	chartScaleTitlePlaces  = []ChartScaleTitlePlace{"plot-area", "special-area", "with-axis"}
	chartScaleTitleTexts   = []ChartScaleTitleText{"auto", "auto-text", "use-text"}
	chartGaugeValues       = []ChartGaugeValue{"needle", "sector"}
	chartGaugeLabelPlaces  = []ChartGaugeLabelsPlace{"at-scale", "inside-scale"}
)

// validateChartContent checks the content of a chart against itself.
func validateChartContent(path string, value *ChartContent) []string {
	if value == nil {
		return nil
	}
	var issues []string
	issues = append(issues, oneOfList(path+".type", value.Type, chartKinds, true)...)
	if value.SeriesCount < len(value.Series) {
		issues = append(issues, path+".series_count must count every series described")
	}
	if value.PointCount < len(value.Points) {
		issues = append(issues, path+".point_count must count every point described")
	}
	if len(value.Values) != 0 && len(value.Values) != value.SeriesCount*value.PointCount {
		issues = append(issues, fmt.Sprintf("%s.values must hold one value for each series and point, %d", path, value.SeriesCount*value.PointCount))
	}
	for index, series := range value.Series {
		issues = append(issues, validateChartSeries(fmt.Sprintf("%s.series[%d]", path, index), series)...)
	}
	for index, point := range value.Points {
		issues = append(issues, validateChartSeries(fmt.Sprintf("%s.points[%d]", path, index), point)...)
	}
	if value.SummarySeries != nil {
		issues = append(issues, validateChartSeries(path+".summary_series", *value.SummarySeries)...)
	}
	for index, item := range value.Values {
		issues = append(issues, validateChartNumber(fmt.Sprintf("%s.values[%d].value", path, index), item.Value, true)...)
	}
	if value.ActiveSeries < -1 || value.ActiveSeries >= max(value.SeriesCount, 1) {
		issues = append(issues, path+".active_series must be the number of a series of the chart, or -1")
	}
	if value.ActivePoint < -1 || value.ActivePoint >= max(value.PointCount, 1) {
		issues = append(issues, path+".active_point must be the number of a point of the chart, or -1")
	}
	for _, number := range []struct {
		name  string
		value int
	}{{"series_count", value.SeriesCount}, {"point_count", value.PointCount}, {"next_series_id", value.NextSeriesID},
		{"next_point_id", value.NextPointID}, {"max_series_count", value.MaxSeriesCount}, {"spline_strain", value.SplineStrain},
		{"donut_chart_inner_radius", value.DonutChartInnerRadius}, {"funnel_neck_height", value.FunnelNeckHeight},
		{"funnel_neck_width", value.FunnelNeckWidth}, {"funnel_space", value.FunnelSpace},
		{"gauge.thickness", value.Gauge.Thickness}, {"gauge.bush_thickness", value.Gauge.BushThickness},
		{"state.rebuild_time", value.State.RebuildTime}, {"state.pie_point", value.State.PiePoint},
		{"state.stock_series", value.State.StockSeries}} {
		if number.value < 0 {
			issues = append(issues, path+"."+number.name+" must not be negative")
		}
	}
	for _, percent := range []struct {
		name  string
		value int
	}{{"max_series_percent", value.MaxSeriesPercent}, {"semitransparency_percent", value.SemitransparencyPercent}} {
		if percent.value < 0 || percent.value > 100 {
			issues = append(issues, path+"."+percent.name+" must be a percentage")
		}
	}
	issues = append(issues, validateChartNumber(path+".max_value", value.MaxValue, false)...)
	issues = append(issues, validateChartNumber(path+".min_value", value.MinValue, false)...)
	issues = append(issues, validateChartNumber(path+".base_value", value.BaseValue, false)...)
	issues = append(issues, oneOfList(path+".max_series", value.MaxSeries, chartMaxSeries, false)...)
	issues = append(issues, oneOfList(path+".space_mode", value.SpaceMode, chartSpaceModes, false)...)
	issues = append(issues, oneOfList(path+".auto_series_separation", value.AutoSeriesSeparation, chartSeriesSeparation, false)...)
	issues = append(issues, oneOfList(path+".orientation", value.Orientation, chartOrientations, false)...)
	issues = append(issues, oneOfList(path+".animation", value.Animation, chartAnimations, false)...)
	issues = append(issues, oneOfList(path+".selection_mode", value.SelectionMode, chartSelectionModes, false)...)
	issues = append(issues, oneOfList(path+".color_palette", value.ColorPalette, chartColorPalettes, false)...)
	if value.ColorPaletteDescription != nil {
		issues = append(issues, oneOfList(path+".color_palette_description", *value.ColorPaletteDescription, chartColorPalettes, true)...)
	}
	issues = append(issues, oneOfList(path+".spline_mode", value.SplineMode, chartSplineModes, false)...)
	issues = append(issues, validateChartLook(path, value.BackColor, nil, nil, value.Border, value.BorderColor)...)
	issues = append(issues, validateChartLabels(path+".labels", value.Labels)...)
	issues = append(issues, validateChartArea(path+".title_area", value.TitleArea.ChartAreaLook)...)
	issues = append(issues, validateChartText(path+".title_area.text", value.TitleArea.Text)...)
	issues = append(issues, oneOfList(path+".title_area.placement", value.TitleArea.Placement, chartTitlePlaces, false)...)
	issues = append(issues, validateChartArea(path+".legend_area", value.LegendArea.ChartAreaLook)...)
	issues = append(issues, oneOfList(path+".legend_area.placement", value.LegendArea.Placement, chartLegendPlaces, false)...)
	issues = append(issues, validateChartPlotArea(path+".plot_area", value.PlotArea)...)
	issues = append(issues, validateChartGauge(path+".gauge", value.Gauge)...)
	state := value.State
	issues = append(issues, oneOfList(path+".state.x_labels_orientation", state.XLabelsOrientation, chartLabelOrientations, false)...)
	issues = append(issues, oneOfList(path+".state.points_drop_lines", state.PointsDropLines, chartShowModes, false)...)
	issues = append(issues, validateRouteStroke(path+".state.multi_stage_link_line", state.MultiStageLinkLine)...)
	issues = append(issues, validateChartColor(path+".state.multi_stage_link_color", state.MultiStageLinkColor)...)
	return issues
}

func validateChartSeries(path string, series ChartSeries) []string {
	var issues []string
	if series.ID < 0 {
		issues = append(issues, path+".id must not be negative")
	}
	issues = append(issues, validateChartText(path+".text", series.Text)...)
	issues = append(issues, validateChartColor(path+".color", series.Color)...)
	issues = append(issues, validateRouteStroke(path+".line", series.Line)...)
	issues = append(issues, oneOfList(path+".marker", series.Marker, chartMarkers, false)...)
	issues = append(issues, oneOfList(path+".show_in_chart", series.ShowInChart, chartShowModes, false)...)
	return issues
}

func validateChartLabels(path string, labels ChartLabels) []string {
	var issues []string
	issues = append(issues, oneOfList(path+".type", labels.Type, chartLabelTypes, false)...)
	issues = append(issues, oneOfList(path+".location", labels.Location, chartLabelLocations, false)...)
	issues = append(issues, validateChartText(path+".value_format", labels.ValueFormat)...)
	issues = append(issues, validateChartText(path+".percent_format", labels.PercentFormat)...)
	issues = append(issues, validateChartLook(path, labels.BackColor, labels.TextColor, labels.Font, labels.Border, labels.BorderColor)...)
	return issues
}

func validateChartArea(path string, area ChartAreaLook) []string {
	issues := validateChartLook(path, area.BackColor, area.TextColor, area.Font, area.Border, area.BorderColor)
	for _, side := range []struct {
		name  string
		value ChartNumber
	}{{"left", area.Bounds.Left}, {"top", area.Bounds.Top}, {"right", area.Bounds.Right}, {"bottom", area.Bounds.Bottom}} {
		issues = append(issues, validateChartNumber(path+".bounds."+side.name, side.value, false)...)
	}
	return issues
}

func validateChartPlotArea(path string, area ChartPlotArea) []string {
	issues := validateChartArea(path, area.ChartAreaLook)
	issues = append(issues, oneOfList(path+".placement", area.Placement, chartPlotPlaces, false)...)
	issues = append(issues, validateRouteStroke(path+".scale_lines", area.ScaleLines)...)
	issues = append(issues, validateChartColor(path+".scale_color", area.ScaleColor)...)
	issues = append(issues, validateChartColor(path+".surface_color", area.SurfaceColor)...)
	issues = append(issues, oneOfList(path+".radar_scale_type", area.RadarScaleType, chartRadarScales, false)...)
	issues = append(issues, oneOfList(path+".align_data_table", area.AlignDataTable, chartAligns, false)...)
	for _, scale := range []struct {
		name  string
		value *ChartScale
	}{{"points_scale", area.PointsScale}, {"series_scale", area.SeriesScale}, {"values_scale", area.ValuesScale}} {
		if scale.value != nil {
			issues = append(issues, validateChartScale(path+"."+scale.name, *scale.value)...)
		}
	}
	return issues
}

func validateChartScale(path string, scale ChartScale) []string {
	var issues []string
	issues = append(issues, oneOfList(path+".grid_lines_show_mode", scale.GridLinesShowMode, chartShowModes, false)...)
	if scale.LabelAngle < -360 || scale.LabelAngle > 360 {
		issues = append(issues, path+".label_angle must be an angle in degrees")
	}
	if scale.MaxLabelRows < 0 {
		issues = append(issues, path+".max_label_rows must not be negative")
	}
	if scale.LabelFont != nil {
		issues = append(issues, validateFontValue(path+".label_font", *scale.LabelFont)...)
	}
	issues = append(issues, validateChartText(path+".label_format", scale.LabelFormat)...)
	issues = append(issues, oneOfList(path+".label_orientation", scale.LabelOrientation, chartLabelOrientations, false)...)
	issues = append(issues, oneOfList(path+".scale_label_location", scale.ScaleLabelLocation, chartScaleLabelPlaces, false)...)
	issues = append(issues, validateRouteStroke(path+".scale_line", scale.ScaleLine)...)
	issues = append(issues, oneOfList(path+".scale_location", scale.ScaleLocation, chartScalePlaces, false)...)
	issues = append(issues, oneOfList(path+".scale_mark_location", scale.ScaleMarkLocation, chartScaleMarkPlaces, false)...)
	issues = append(issues, oneOfList(path+".show_in_chart", scale.ShowInChart, chartShowModes, false)...)
	issues = append(issues, oneOfList(path+".show_title", scale.ShowTitle, chartShowModes, false)...)
	issues = append(issues, oneOfList(path+".title_placement", scale.TitlePlacement, chartScaleTitlePlaces, false)...)
	issues = append(issues, validateChartText(path+".title_text", scale.TitleText)...)
	issues = append(issues, oneOfList(path+".title_text_mode", scale.TitleTextMode, chartScaleTitleTexts, false)...)
	if area := scale.TitleArea; area != nil {
		issues = append(issues, validateChartLook(path+".title_area", area.BackColor, area.TextColor, area.Font, area.Border, area.BorderColor)...)
	}
	return issues
}

func validateChartGauge(path string, gauge ChartGauge) []string {
	var issues []string
	issues = append(issues, oneOfList(path+".value_representation", gauge.ValueRepresentation, chartGaugeValues, false)...)
	issues = append(issues, oneOfList(path+".labels_location", gauge.LabelsLocation, chartGaugeLabelPlaces, false)...)
	for _, angle := range []struct {
		name  string
		value int
	}{{"begin_angle", gauge.BeginAngle}, {"end_angle", gauge.EndAngle}} {
		if angle.value < -360 || angle.value > 360 {
			issues = append(issues, path+"."+angle.name+" must be an angle in degrees")
		}
	}
	issues = append(issues, validateChartColor(path+".bush_color", gauge.BushColor)...)
	return issues
}

// validateChartLook checks the colours, the font and the border of one part
// of a chart, each named after the part.
func validateChartLook(path string, back, text *ColorValue, font *FontValue, border *BorderValue, borderColor *ColorValue) []string {
	var issues []string
	for _, color := range []struct {
		name  string
		value *ColorValue
	}{{"back_color", back}, {"text_color", text}, {"border_color", borderColor}} {
		if color.value != nil {
			issues = append(issues, validateColorValue(path+"."+color.name, *color.value)...)
		}
	}
	if font != nil {
		issues = append(issues, validateFontValue(path+".font", *font)...)
	}
	if border != nil {
		issues = append(issues, validateBorderValue(path+".border", *border)...)
	}
	return issues
}

// validateChartNumber checks a number written as the prototype writes one;
// required says it may not be left out.
func validateChartNumber(path string, value ChartNumber, required bool) []string {
	if value == "" && !required || chartNumberPattern.MatchString(string(value)) {
		return nil
	}
	return []string{path + " must be a decimal number"}
}

// chartAnyLanguage is the language the prototype writes a text of a chart in
// when it is no language of the configuration: "#", 23 texts of the summary
// series and the formats of the exports. What it means is not known; it is
// carried as written and noted (NoteChartTextAnyLanguage).
const chartAnyLanguage = "#"

// validateChartText checks a text of a chart as a title is checked, "#"
// taken for a language.
func validateChartText(path string, text LocalizedText) []string {
	var issues []string
	for language, value := range text {
		if language != "" && language != chartAnyLanguage && !project.LanguageCodeShape(language) {
			issues = append(issues, path+"."+language+" is not a language code")
		}
		if !validLocalizedText(value) {
			issues = append(issues, path+"."+language+" must say something in printable characters, line breaks allowed")
		}
	}
	return issues
}

// validateChartColor checks one colour of a chart.
func validateChartColor(path string, color *ColorValue) []string {
	if color == nil {
		return nil
	}
	return validateColorValue(path, *color)
}

// oneOfList checks a value against its list; required says it may not be
// left out.
func oneOfList[T ~string](path string, value T, list []T, required bool) []string {
	if value == "" && !required || slices.Contains(list, value) {
		return nil
	}
	names := make([]string, len(list))
	for index, item := range list {
		names[index] = string(item)
	}
	return []string{path + " must be one of " + strings.Join(names, ", ")}
}

// chartParts walks the content of a chart and hands every colour, font,
// border and text in it to visit, named by its path in the model.
func (value *ChartContent) chartParts(visit func(path string, part any)) {
	walkChartParts(reflect.ValueOf(value).Elem(), visit)
}

// walkChartParts walks content of a chart, of a Gantt chart or of a planner
// and hands every colour, font, border and text in it to visit. It walks by
// reflection so that a part added to the model is not left out of the
// resolution and the notes.
func walkChartParts(root reflect.Value, visit func(path string, part any)) {
	var walk func(path string, current reflect.Value)
	walk = func(path string, current reflect.Value) {
		switch current.Kind() {
		case reflect.Pointer:
			if current.IsNil() {
				return
			}
			switch part := current.Interface().(type) {
			case *ColorValue, *FontValue, *BorderValue:
				visit(path, part)
				return
			}
			walk(path, current.Elem())
		case reflect.Map:
			if text, ok := current.Interface().(LocalizedText); ok && len(text) != 0 {
				visit(path, text)
			}
		case reflect.Slice:
			for index := range current.Len() {
				walk(fmt.Sprintf("%s[%d]", path, index), current.Index(index))
			}
		case reflect.Struct:
			for index := range current.NumField() {
				field := current.Type().Field(index)
				name, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
				next := path
				if name != "" {
					next = strings.TrimPrefix(path+"."+name, ".")
				}
				walk(next, current.Field(index))
			}
		}
	}
	walk("", root)
}

// styleItems lists the style items of the configuration the chart takes a
// colour, a font or a border from, each with the type it must be.
func (value *ChartContent) styleItems() []styleItemUse { return styleItemsIn(value.chartParts) }

// styleItemsIn lists the style items the parts walk hands out are taken
// from, each with the type it must be.
func styleItemsIn(parts func(visit func(path string, part any))) []styleItemUse {
	var uses []styleItemUse
	parts(func(path string, part any) {
		var reference *StyleItemReference
		var itemType StyleItemType
		switch part := part.(type) {
		case *ColorValue:
			reference, itemType = part.From, ColorStyleItem
		case *FontValue:
			reference, itemType = part.From, FontStyleItem
		case *BorderValue:
			reference, itemType = part.From, BorderStyleItem
		}
		if use, ok := styleItemUseOf(path, itemType, reference); ok {
			uses = append(uses, use)
		}
	})
	return uses
}

// anyLanguageTexts names the texts of the chart written in "#".
func (value *ChartContent) anyLanguageTexts() []string { return anyLanguageTextsIn(value.chartParts) }

// anyLanguageTextsIn names the texts the parts walk hands out that are
// written in "#".
func anyLanguageTextsIn(parts func(visit func(path string, part any))) []string {
	var paths []string
	parts(func(path string, part any) {
		if text, ok := part.(LocalizedText); ok {
			if _, found := text[chartAnyLanguage]; found {
				paths = append(paths, path)
			}
		}
	})
	return paths
}

// clone copies the content whole: nothing the copy holds is shared.
func (value *ChartContent) clone() *ChartContent {
	if value == nil {
		return nil
	}
	return deepCopy(reflect.ValueOf(value)).Interface().(*ChartContent)
}

// deepCopy copies a value with everything behind its pointers, slices and
// maps.
func deepCopy(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return value
		}
		result := reflect.New(value.Type().Elem())
		result.Elem().Set(deepCopy(value.Elem()))
		return result
	case reflect.Slice:
		if value.IsNil() {
			return value
		}
		result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for index := range value.Len() {
			result.Index(index).Set(deepCopy(value.Index(index)))
		}
		return result
	case reflect.Map:
		if value.IsNil() {
			return value
		}
		result := reflect.MakeMapWithSize(value.Type(), value.Len())
		for _, key := range value.MapKeys() {
			result.SetMapIndex(key, deepCopy(value.MapIndex(key)))
		}
		return result
	case reflect.Struct:
		result := reflect.New(value.Type()).Elem()
		result.Set(value)
		for index := range value.NumField() {
			if result.Field(index).CanSet() {
				result.Field(index).Set(deepCopy(value.Field(index)))
			}
		}
		return result
	}
	return value
}
