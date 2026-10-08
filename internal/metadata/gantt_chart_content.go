package metadata

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// The content a Gantt chart attribute of a form is made with in the
// designer. The prototype writes it whole into the settings of the attribute
// (15 Gantt charts of the exports, 236 to 250 values each): a chart whole -
// the title, legend and plot areas, their look, the border, which is what
// the help gives a Gantt chart of the areas of a chart - and beside it what
// is a Gantt chart's own: the time scale, the whole interval and how the
// scale is kept, how intervals and their texts are drawn, the links, the
// background intervals. It is carried, not drawn: a Gantt chart is drawn in
// block 8.
//
// The model follows the help (GanttChart, GanttChartPlotArea, TimeScale,
// TimeScaleItem, GanttChartPoint, GanttChartSeries); what the prototype
// writes and the help names nowhere is carried apart, in GanttChartState, and
// the chart is noted.

// GanttChartContent is the content of a Gantt chart.
type GanttChartContent struct {
	// Chart is the chart the Gantt chart is drawn with, as a chart attribute
	// keeps it (ChartContent): the prototype writes it whole, a chart type and
	// a summary series included, which a Gantt chart does not use.
	Chart ChartContent `yaml:"chart" json:"chart"`
	// Points and Series are the root of the points and of the series: the
	// point and the series the others are put under, with the look a point
	// and a series take (help, GanttChart.Points, GanttChart.Series). No
	// Gantt chart of the exports holds a point or a series of its own.
	Points GanttChartRoot `yaml:"points" json:"points"`
	Series GanttChartRoot `yaml:"series" json:"series"`
	// PlotArea is what the plot area of a Gantt chart has beyond the plot
	// area of the chart: the time scale, the links, the text (help,
	// GanttChartPlotArea).
	PlotArea GanttChartPlotArea `yaml:"plot_area" json:"plotArea"`

	ShowEmptyValues bool `yaml:"show_empty_values,omitempty" json:"showEmptyValues,omitempty"`
	// ScaleKeeping is how the scale of the visible part is kept, and
	// PeriodicVariantUnit and PeriodicVariantRepetition are the interval seen
	// when it is kept periodic (help, GanttChart.ScaleKeeping and the rest).
	ScaleKeeping              GanttChartScaleKeeping `yaml:"scale_keeping,omitempty" json:"scaleKeeping,omitempty"`
	PeriodicVariantUnit       TimeScaleUnit          `yaml:"periodic_variant_unit,omitempty" json:"periodicVariantUnit,omitempty"`
	PeriodicVariantRepetition int                    `yaml:"periodic_variant_repetition,omitempty" json:"periodicVariantRepetition,omitempty"`
	// AutoDetectWholeInterval takes the whole interval from the values;
	// BeginOfWholeInterval and EndOfWholeInterval are the interval otherwise,
	// dates written as the prototype writes one in a form. The help gives
	// them as read only; the prototype writes them, and SetWholeInterval sets
	// them.
	AutoDetectWholeInterval bool   `yaml:"auto_detect_whole_interval,omitempty" json:"autoDetectWholeInterval,omitempty"`
	BeginOfWholeInterval    string `yaml:"begin_of_whole_interval,omitempty" json:"beginOfWholeInterval,omitempty"`
	EndOfWholeInterval      string `yaml:"end_of_whole_interval,omitempty" json:"endOfWholeInterval,omitempty"`

	IntervalRepresentation     GanttChartIntervalRepresentation `yaml:"interval_representation,omitempty" json:"intervalRepresentation,omitempty"`
	IntervalTextRepresentation GanttChartShowMode               `yaml:"interval_text_representation,omitempty" json:"intervalTextRepresentation,omitempty"`
	ValueTextRepresentation    GanttChartValueText              `yaml:"value_text_representation,omitempty" json:"valueTextRepresentation,omitempty"`
	VerticalStretch            GanttChartVerticalStretch        `yaml:"vertical_stretch,omitempty" json:"verticalStretch,omitempty"`
	VerticalScroll             bool                             `yaml:"vertical_scroll,omitempty" json:"verticalScroll,omitempty"`

	State GanttChartState `yaml:"state" json:"state"`
}

// GanttChartRoot is the root of the points or of the series of a Gantt
// chart: its text and the look of a point (help, GanttChartPoint) or of a
// series (help, GanttChartSeries). The prototype writes the colours apart,
// as the content of the item. A series has no font, back or text colour of
// its own, a point no hatch between intervals, and the prototype writes none
// of them.
type GanttChartRoot struct {
	Text        LocalizedText `yaml:"text,omitempty" json:"text,omitempty"`
	Font        *FontValue    `yaml:"font,omitempty" json:"font,omitempty"`
	Color       *ColorValue   `yaml:"color,omitempty" json:"color,omitempty"`
	SecondColor *ColorValue   `yaml:"second_color,omitempty" json:"secondColor,omitempty"`
	BackColor   *ColorValue   `yaml:"back_color,omitempty" json:"backColor,omitempty"`
	TextColor   *ColorValue   `yaml:"text_color,omitempty" json:"textColor,omitempty"`
	// BetweenIntervalsHatchColor is the colour a series hatches the gap
	// between its intervals with (help, GanttChartSeries).
	BetweenIntervalsHatchColor *ColorValue `yaml:"between_intervals_hatch_color,omitempty" json:"betweenIntervalsHatchColor,omitempty"`
	// AutoText names a point or a series by itself where its text is not
	// given (help, GanttChart.AutoPointText, AutoSeriesText). The chart
	// inside writes a switch of the same meaning of its own.
	AutoText bool `yaml:"auto_text,omitempty" json:"autoText,omitempty"`
	// State is what the prototype writes of the root and the help names
	// nowhere.
	State GanttChartRootState `yaml:"state" json:"state"`
}

// GanttChartRootState is the prototype's own state of the root of the
// points or of the series: the keys of the item in its tree, its data and
// two switches. Each field is the prototype's value of the same name.
type GanttChartRootState struct {
	ItemKey                  int64 `yaml:"item_key,omitempty" json:"itemKey,omitempty"`                                     // itemKey
	Key                      int64 `yaml:"key,omitempty" json:"key,omitempty"`                                              // key
	ParentKey                int64 `yaml:"parent_key,omitempty" json:"parentKey,omitempty"`                                 // parentKey
	LeftKey                  int64 `yaml:"left_key,omitempty" json:"leftKey,omitempty"`                                     // leftKey
	RightKey                 int64 `yaml:"right_key,omitempty" json:"rightKey,omitempty"`                                   // rightKey
	ExtKey                   int64 `yaml:"ext_key,omitempty" json:"extKey,omitempty"`                                       // extKey
	CacheKey                 int64 `yaml:"cache_key,omitempty" json:"cacheKey,omitempty"`                                   // cacheKey
	BaseData                 int64 `yaml:"base_data,omitempty" json:"baseData,omitempty"`                                   // baseData
	TestMode                 bool  `yaml:"test_mode,omitempty" json:"testMode,omitempty"`                                   // testMode
	UseValuesReverseBehavior bool  `yaml:"use_values_reverse_behavior,omitempty" json:"useValuesReverseBehavior,omitempty"` // useValuesReverseBehavior
}

// GanttChartPlotArea is what the plot area of a Gantt chart has beyond the
// plot area of a chart (help, GanttChartPlotArea).
type GanttChartPlotArea struct {
	// Title is written left of the time scale, over the column of points.
	Title     LocalizedText `yaml:"title,omitempty" json:"title,omitempty"`
	TimeScale TimeScale     `yaml:"time_scale" json:"timeScale"`
	// OutboundWholeIntervalColor is the colour of what lies beyond the whole
	// interval.
	OutboundWholeIntervalColor *ColorValue          `yaml:"outbound_whole_interval_color,omitempty" json:"outboundWholeIntervalColor,omitempty"`
	LinkLines                  *RouteStroke         `yaml:"link_lines,omitempty" json:"linkLines,omitempty"`
	LinkLinesColor             *ColorValue          `yaml:"link_lines_color,omitempty" json:"linkLinesColor,omitempty"`
	ShowPointsText             GanttChartShowMode   `yaml:"show_points_text,omitempty" json:"showPointsText,omitempty"`
	ShowData                   GanttChartShowMode   `yaml:"show_data,omitempty" json:"showData,omitempty"`
	TextPlacement              GanttChartTextPlaces `yaml:"text_placement,omitempty" json:"textPlacement,omitempty"`
}

// TimeScale is the time scale of a Gantt chart or a planner (help,
// TimeScale): where it stands, its look and its items, from the top.
type TimeScale struct {
	Location    TimeScaleLocation `yaml:"location,omitempty" json:"location,omitempty"`
	Transparent bool              `yaml:"transparent,omitempty" json:"transparent,omitempty"`
	BackColor   *ColorValue       `yaml:"back_color,omitempty" json:"backColor,omitempty"`
	TextColor   *ColorValue       `yaml:"text_color,omitempty" json:"textColor,omitempty"`
	Items       []TimeScaleItem   `yaml:"items,omitempty" json:"items,omitempty"`
	// CurrentLevel is written for every scale and named nowhere in the
	// help; it is part of the state of the chart that holds the scale.
	CurrentLevel int `yaml:"current_level,omitempty" json:"currentLevel,omitempty"`
}

// TimeScaleItem is one item of a time scale (help, TimeScaleItem). The
// labels of an item are not carried: no item of the exports has one, and the
// prototype writes in their place one number the help names nowhere,
// LabelsTicks, part of the state of the chart.
type TimeScaleItem struct {
	Unit                 TimeScaleUnit      `yaml:"unit,omitempty" json:"unit,omitempty"`
	Repetition           int                `yaml:"repetition,omitempty" json:"repetition,omitempty"`
	Visible              bool               `yaml:"visible,omitempty" json:"visible,omitempty"`
	PointLines           *RouteStroke       `yaml:"point_lines,omitempty" json:"pointLines,omitempty"`
	LineColor            *ColorValue        `yaml:"line_color,omitempty" json:"lineColor,omitempty"`
	DayFormat            TimeScaleDayFormat `yaml:"day_format,omitempty" json:"dayFormat,omitempty"`
	Format               LocalizedText      `yaml:"format,omitempty" json:"format,omitempty"`
	BackColor            *ColorValue        `yaml:"back_color,omitempty" json:"backColor,omitempty"`
	TextColor            *ColorValue        `yaml:"text_color,omitempty" json:"textColor,omitempty"`
	ShowPeriodicalLabels bool               `yaml:"show_periodical_labels,omitempty" json:"showPeriodicalLabels,omitempty"`
	LabelsTicks          int64              `yaml:"labels_ticks,omitempty" json:"labelsTicks,omitempty"`
}

// GanttChartState is what the prototype writes of a Gantt chart that the help
// names nowhere: where the visible part begins, the measure of the scale when
// it is kept by no variant, and the numbers it writes in place of the
// background intervals. Each field is the prototype's value of the same name.
// The background intervals themselves (help, GanttChart.BackgroundIntervals)
// are not carried: no Gantt chart of the exports has one.
type GanttChartState struct {
	VisualBegin                   string        `yaml:"visual_begin,omitempty" json:"visualBegin,omitempty"`                                       // visualBegin
	NoneVariantChars              int           `yaml:"none_variant_chars,omitempty" json:"noneVariantChars,omitempty"`                            // noneVariantChars
	NoneVariantMeasure            TimeScaleUnit `yaml:"none_variant_measure,omitempty" json:"noneVariantMeasure,omitempty"`                        // noneVariantMeasure
	BackgroundIntervalsTicks      int64         `yaml:"background_intervals_ticks,omitempty" json:"backgroundIntervalsTicks,omitempty"`            // backIntervals/ticks
	BackgroundIntervalsTicksInner int64         `yaml:"background_intervals_ticks_inner,omitempty" json:"backgroundIntervalsTicksInner,omitempty"` // backIntervals/collection/ticks
}

type (
	GanttChartScaleKeeping           string
	GanttChartIntervalRepresentation string
	GanttChartShowMode               string
	GanttChartValueText              string
	GanttChartVerticalStretch        string
	GanttChartTextPlaces             string
	TimeScaleUnit                    string
	TimeScaleLocation                string
	TimeScaleDayFormat               string
)

// The lists of the help (GanttChartScaleKeeping and the rest), each as the
// model writes names. GanttChartShowMode is the help's
// GanttChartIntervalTextRepresentation and ShowInGanttChart alike: the same
// three values.
var (
	ganttScaleKeepings          = []GanttChartScaleKeeping{"all-data", "auto", "fixed", "period"}
	ganttIntervalRepresentation = []GanttChartIntervalRepresentation{"flat", "gradient", "rhomb", "three-dimensional"}
	ganttShowModes              = []GanttChartShowMode{"auto", "dont-show", "show"}
	ganttValueTexts             = []GanttChartValueText{"none", "right"}
	ganttVerticalStretches      = []GanttChartVerticalStretch{"none", "stretch-rows", "stretch-rows-and-data"}
	ganttTextPlaces             = []GanttChartTextPlaces{"auto", "cut", "wrap"}
	timeScaleUnits              = []TimeScaleUnit{"second", "minute", "hour", "day", "week", "month", "quarter", "year"}
	timeScaleLocations          = []TimeScaleLocation{"top", "bottom", "left", "right"}
	timeScaleDayFormats         = []TimeScaleDayFormat{"month-day", "month-day-week-day", "week-day", "week-day-month-day"}
)

// ganttDateLayout is how the prototype writes a date in a form.
const ganttDateLayout = "2006-01-02T15:04:05"

// The bounds the help puts on the whole interval (GanttChart.SetWholeInterval):
// it begins no earlier than 01.01.1000 and ends no later than 01.01.3000.
var (
	ganttEarliest = time.Date(1000, 1, 1, 0, 0, 0, 0, time.UTC)
	ganttLatest   = time.Date(3000, 1, 1, 0, 0, 0, 0, time.UTC)
)

// validateGanttChartContent checks the content of a Gantt chart against
// itself.
func validateGanttChartContent(path string, value *GanttChartContent) []string {
	if value == nil {
		return nil
	}
	issues := validateChartContent(path+".chart", &value.Chart)
	issues = append(issues, validateGanttChartRoot(path+".points", value.Points, false)...)
	issues = append(issues, validateGanttChartRoot(path+".series", value.Series, true)...)
	area := value.PlotArea
	issues = append(issues, validateChartText(path+".plot_area.title", area.Title)...)
	issues = append(issues, validateTimeScale(path+".plot_area.time_scale", area.TimeScale)...)
	issues = append(issues, validateChartColor(path+".plot_area.outbound_whole_interval_color", area.OutboundWholeIntervalColor)...)
	issues = append(issues, validateRouteStroke(path+".plot_area.link_lines", area.LinkLines)...)
	issues = append(issues, validateChartColor(path+".plot_area.link_lines_color", area.LinkLinesColor)...)
	issues = append(issues, oneOfList(path+".plot_area.show_points_text", area.ShowPointsText, ganttShowModes, false)...)
	issues = append(issues, oneOfList(path+".plot_area.show_data", area.ShowData, ganttShowModes, false)...)
	issues = append(issues, oneOfList(path+".plot_area.text_placement", area.TextPlacement, ganttTextPlaces, false)...)
	issues = append(issues, oneOfList(path+".scale_keeping", value.ScaleKeeping, ganttScaleKeepings, false)...)
	issues = append(issues, oneOfList(path+".periodic_variant_unit", value.PeriodicVariantUnit, timeScaleUnits, false)...)
	issues = append(issues, oneOfList(path+".interval_representation", value.IntervalRepresentation, ganttIntervalRepresentation, false)...)
	issues = append(issues, oneOfList(path+".interval_text_representation", value.IntervalTextRepresentation, ganttShowModes, false)...)
	issues = append(issues, oneOfList(path+".value_text_representation", value.ValueTextRepresentation, ganttValueTexts, false)...)
	issues = append(issues, oneOfList(path+".vertical_stretch", value.VerticalStretch, ganttVerticalStretches, false)...)
	issues = append(issues, oneOfList(path+".state.none_variant_measure", value.State.NoneVariantMeasure, timeScaleUnits, false)...)
	for _, number := range []struct {
		name  string
		value int
	}{{"periodic_variant_repetition", value.PeriodicVariantRepetition}, {"state.none_variant_chars", value.State.NoneVariantChars}} {
		if number.value < 0 {
			issues = append(issues, path+"."+number.name+" must not be negative")
		}
	}
	begin, beginIssues := ganttDate(path+".begin_of_whole_interval", value.BeginOfWholeInterval)
	end, endIssues := ganttDate(path+".end_of_whole_interval", value.EndOfWholeInterval)
	issues = append(append(issues, beginIssues...), endIssues...)
	if !begin.IsZero() && begin.Before(ganttEarliest) {
		issues = append(issues, path+".begin_of_whole_interval must not be earlier than 1000-01-01T00:00:00")
	}
	if !end.IsZero() && end.After(ganttLatest) {
		issues = append(issues, path+".end_of_whole_interval must not be later than 3000-01-01T00:00:00")
	}
	if !begin.IsZero() && !end.IsZero() && end.Before(begin) {
		issues = append(issues, path+".end_of_whole_interval must not be earlier than its begin")
	}
	_, visualIssues := ganttDate(path+".state.visual_begin", value.State.VisualBegin)
	return append(issues, visualIssues...)
}

// validateGanttChartRoot checks the root of the points or, with series, of
// the series: each has the look the help gives it and no other.
func validateGanttChartRoot(path string, root GanttChartRoot, series bool) []string {
	issues := validateChartText(path+".text", root.Text)
	if series && (root.Font != nil || root.BackColor != nil || root.TextColor != nil) {
		issues = append(issues, path+" is a series, which has no font, back colour or text colour of its own")
	}
	if !series && root.BetweenIntervalsHatchColor != nil {
		issues = append(issues, path+".between_intervals_hatch_color belongs to a series")
	}
	if root.Font != nil {
		issues = append(issues, validateFontValue(path+".font", *root.Font)...)
	}
	for _, color := range []struct {
		name  string
		value *ColorValue
	}{{"color", root.Color}, {"second_color", root.SecondColor}, {"back_color", root.BackColor}, {"text_color", root.TextColor},
		{"between_intervals_hatch_color", root.BetweenIntervalsHatchColor}} {
		issues = append(issues, validateChartColor(path+"."+color.name, color.value)...)
	}
	return issues
}

func validateTimeScale(path string, scale TimeScale) []string {
	var issues []string
	issues = append(issues, oneOfList(path+".location", scale.Location, timeScaleLocations, false)...)
	issues = append(issues, validateChartColor(path+".back_color", scale.BackColor)...)
	issues = append(issues, validateChartColor(path+".text_color", scale.TextColor)...)
	if scale.CurrentLevel < 0 || scale.CurrentLevel >= max(len(scale.Items), 1) {
		issues = append(issues, path+".current_level must be the number of an item of the scale")
	}
	for index, item := range scale.Items {
		at := fmt.Sprintf("%s.items[%d]", path, index)
		issues = append(issues, oneOfList(at+".unit", item.Unit, timeScaleUnits, true)...)
		if item.Repetition < 0 {
			issues = append(issues, at+".repetition must not be negative")
		}
		issues = append(issues, validateRouteStroke(at+".point_lines", item.PointLines)...)
		issues = append(issues, validateChartColor(at+".line_color", item.LineColor)...)
		issues = append(issues, oneOfList(at+".day_format", item.DayFormat, timeScaleDayFormats, false)...)
		issues = append(issues, validateChartText(at+".format", item.Format)...)
		issues = append(issues, validateChartColor(at+".back_color", item.BackColor)...)
		issues = append(issues, validateChartColor(at+".text_color", item.TextColor)...)
	}
	return issues
}

// ganttDate reads a date of a Gantt chart; one left out is no date.
func ganttDate(path, value string) (time.Time, []string) {
	if value == "" {
		return time.Time{}, nil
	}
	date, err := time.Parse(ganttDateLayout, value)
	if err != nil || !formDateTime.MatchString(value) {
		return time.Time{}, []string{path + " must be a date written as " + ganttDateLayout}
	}
	return date, nil
}

// stateFields names what the prototype wrote of its own state of the Gantt
// chart, the roots and the time scale, for the note; the chart inside is
// noted on its own.
func (value *GanttChartContent) stateFields() string {
	var names []string
	add := func(name string, set bool) {
		if set {
			names = append(names, name)
		}
	}
	state := value.State
	add("visual_begin", state.VisualBegin != "")
	add("none_variant_chars", state.NoneVariantChars != 0)
	add("none_variant_measure", state.NoneVariantMeasure != "")
	add("background_intervals_ticks", state.BackgroundIntervalsTicks != 0)
	add("background_intervals_ticks_inner", state.BackgroundIntervalsTicksInner != 0)
	for _, root := range []struct {
		name  string
		state GanttChartRootState
	}{{"points", value.Points.State}, {"series", value.Series.State}} {
		if root.state != (GanttChartRootState{}) {
			names = append(names, root.name+".state")
		}
	}
	add("plot_area.time_scale.current_level", value.PlotArea.TimeScale.CurrentLevel != 0)
	for index, item := range value.PlotArea.TimeScale.Items {
		add(fmt.Sprintf("plot_area.time_scale.items[%d].labels_ticks", index), item.LabelsTicks != 0)
	}
	return strings.Join(names, ", ")
}

// ganttParts walks the content of a Gantt chart, the chart inside included.
func (value *GanttChartContent) ganttParts(visit func(path string, part any)) {
	walkChartParts(reflect.ValueOf(value).Elem(), visit)
}

// styleItems lists the style items the Gantt chart takes a colour, a font or
// a border from.
func (value *GanttChartContent) styleItems() []styleItemUse { return styleItemsIn(value.ganttParts) }

// clone copies the content whole: nothing the copy holds is shared.
func (value *GanttChartContent) clone() *GanttChartContent {
	if value == nil {
		return nil
	}
	return deepCopy(reflect.ValueOf(value)).Interface().(*GanttChartContent)
}
