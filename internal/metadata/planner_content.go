package metadata

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The content a planner attribute of a form is made with in the designer.
// The prototype writes it whole into the settings of the attribute (3
// planners of the exports, 71 to 80 values each): the look of the planner,
// the period it shows and its bounds, the time scale and how it wraps, how
// items are drawn and the items themselves. It is carried, not drawn: a
// planner is drawn in block 8.
//
// The model follows the help (Planner, PlannerItem,
// PlannerRepresentationPeriod, TimeScale, TimeScaleItem); what the prototype
// writes and the help names nowhere is carried apart, in the state of an item
// and of the time scale, and the planner is noted.

// PlannerContent is the content of a planner.
type PlannerContent struct {
	// Items are the items of the planner, in the order written. The
	// dimensions and the background intervals (help, Planner.Dimensions,
	// BackgroundIntervals) are not carried: no planner of the exports has
	// one.
	Items []PlannerItem `yaml:"items,omitempty" json:"items,omitempty"`

	BackColor   *ColorValue  `yaml:"back_color,omitempty" json:"backColor,omitempty"`
	TextColor   *ColorValue  `yaml:"text_color,omitempty" json:"textColor,omitempty"`
	LineColor   *ColorValue  `yaml:"line_color,omitempty" json:"lineColor,omitempty"`
	BorderColor *ColorValue  `yaml:"border_color,omitempty" json:"borderColor,omitempty"`
	Font        *FontValue   `yaml:"font,omitempty" json:"font,omitempty"`
	Border      *BorderValue `yaml:"border,omitempty" json:"border,omitempty"`

	// BeginOfRepresentationPeriod and EndOfRepresentationPeriod bound what
	// the planner may be moved to; left out, a bound is not set (the
	// prototype writes the empty date for it).
	BeginOfRepresentationPeriod string `yaml:"begin_of_representation_period,omitempty" json:"beginOfRepresentationPeriod,omitempty"`
	EndOfRepresentationPeriod   string `yaml:"end_of_representation_period,omitempty" json:"endOfRepresentationPeriod,omitempty"`
	// CurrentRepresentationPeriods are the periods the planner shows (help,
	// Planner.CurrentRepresentationPeriods).
	CurrentRepresentationPeriods []PlannerRepresentationPeriod `yaml:"current_representation_periods,omitempty" json:"currentRepresentationPeriods,omitempty"`

	AlignItemBoundariesByTimeScale bool `yaml:"align_item_boundaries_by_time_scale,omitempty" json:"alignItemBoundariesByTimeScale,omitempty"`
	ShowWrappedHeaders             bool `yaml:"show_wrapped_headers,omitempty" json:"showWrappedHeaders,omitempty"`
	ShowWrappedTimeScaleHeaders    bool `yaml:"show_wrapped_time_scale_headers,omitempty" json:"showWrappedTimeScaleHeaders,omitempty"`
	// WrappedTimeScaleHeaderFormat is the format of the dates in the headers
	// of the wraps. The help gives it as a string; the prototype writes it as
	// a text in languages, and it is carried as written.
	WrappedTimeScaleHeaderFormat LocalizedText `yaml:"wrapped_time_scale_header_format,omitempty" json:"wrappedTimeScaleHeaderFormat,omitempty"`
	// PeriodicVariantUnit and PeriodicVariantRepetition are the interval the
	// time scale wraps at; the indents are counted in that unit from the
	// beginning and the end of a wrap.
	PeriodicVariantUnit       TimeScaleUnit `yaml:"periodic_variant_unit,omitempty" json:"periodicVariantUnit,omitempty"`
	PeriodicVariantRepetition int           `yaml:"periodic_variant_repetition,omitempty" json:"periodicVariantRepetition,omitempty"`
	TimeScaleWrapBeginIndent  int           `yaml:"time_scale_wrap_begin_indent,omitempty" json:"timeScaleWrapBeginIndent,omitempty"`
	TimeScaleWrapEndIndent    int           `yaml:"time_scale_wrap_end_indent,omitempty" json:"timeScaleWrapEndIndent,omitempty"`
	TimeScale                 TimeScale     `yaml:"time_scale" json:"timeScale"`

	ShowCurrentDate                    bool                            `yaml:"show_current_date,omitempty" json:"showCurrentDate,omitempty"`
	ItemsTimeRepresentation            PlannerItemsTimeRepresentation  `yaml:"items_time_representation,omitempty" json:"itemsTimeRepresentation,omitempty"`
	ItemsBehaviorWhenSpaceInsufficient PlannerItemsBehaviorOnLackSpace `yaml:"items_behavior_when_space_insufficient,omitempty" json:"itemsBehaviorWhenSpaceInsufficient,omitempty"`
	AutoColumnMinWidth                 bool                            `yaml:"auto_column_min_width,omitempty" json:"autoColumnMinWidth,omitempty"`
	AutoRowMinHeight                   bool                            `yaml:"auto_row_min_height,omitempty" json:"autoRowMinHeight,omitempty"`
	MinColumnWidth                     int                             `yaml:"min_column_width,omitempty" json:"minColumnWidth,omitempty"`
	MinRowHeight                       int                             `yaml:"min_row_height,omitempty" json:"minRowHeight,omitempty"`
	// FixDimensionsHeader and FixTimeScaleHeader fix a header when the
	// planner scrolls; left out, it is fixed or not by itself (help:
	// Undefined, the prototype writes "auto").
	FixDimensionsHeader *bool                   `yaml:"fix_dimensions_header,omitempty" json:"fixDimensionsHeader,omitempty"`
	FixTimeScaleHeader  *bool                   `yaml:"fix_time_scale_header,omitempty" json:"fixTimeScaleHeader,omitempty"`
	NewItemsTextType    PlannerNewItemsTextType `yaml:"new_items_text_type,omitempty" json:"newItemsTextType,omitempty"`
}

// PlannerRepresentationPeriod is one period a planner shows (help,
// PlannerRepresentationPeriod).
type PlannerRepresentationPeriod struct {
	Begin string `yaml:"begin,omitempty" json:"begin,omitempty"`
	End   string `yaml:"end,omitempty" json:"end,omitempty"`
}

// PlannerItem is one item of a planner (help, PlannerItem). Its value, the
// values of the dimensions it stands in, its picture, schedule, actions and
// replacing items are not carried: every item of the exports has none.
type PlannerItem struct {
	Text string `yaml:"text,omitempty" json:"text,omitempty"`
	// TextFormatted says the text is a formatted string (help, PlannerItem.Text:
	// a string or a formatted string).
	TextFormatted bool         `yaml:"text_formatted,omitempty" json:"textFormatted,omitempty"`
	ToolTip       string       `yaml:"tooltip,omitempty" json:"tooltip,omitempty"`
	Begin         string       `yaml:"begin,omitempty" json:"begin,omitempty"`
	End           string       `yaml:"end,omitempty" json:"end,omitempty"`
	BackColor     *ColorValue  `yaml:"back_color,omitempty" json:"backColor,omitempty"`
	TextColor     *ColorValue  `yaml:"text_color,omitempty" json:"textColor,omitempty"`
	BorderColor   *ColorValue  `yaml:"border_color,omitempty" json:"borderColor,omitempty"`
	Font          *FontValue   `yaml:"font,omitempty" json:"font,omitempty"`
	Border        *BorderValue `yaml:"border,omitempty" json:"border,omitempty"`
	// Deleted and ReplacementDate are what an item replacing an occurrence of
	// a repeated one says of it: the occurrence is gone, and the date it
	// stood at. The help gives the date as read only; the prototype writes it.
	Deleted         bool                      `yaml:"deleted,omitempty" json:"deleted,omitempty"`
	ReplacementDate string                    `yaml:"replacement_date,omitempty" json:"replacementDate,omitempty"`
	EnableEditMode  PlannerItemEnableEditMode `yaml:"enable_edit_mode,omitempty" json:"enableEditMode,omitempty"`
	// State is what the prototype writes of the item and the help names
	// nowhere.
	State PlannerItemState `yaml:"state" json:"state"`
}

// PlannerItemState is the prototype's own state of a planner item. Each
// field is the prototype's value of the same name.
type PlannerItemState struct {
	ID string `yaml:"id,omitempty" json:"id,omitempty"` // id
}

type (
	PlannerItemsTimeRepresentation  string
	PlannerItemsBehaviorOnLackSpace string
	PlannerNewItemsTextType         string
	PlannerItemEnableEditMode       string
)

// The lists of the help (PlannerItemsTimeRepresentation,
// PlannerItemsBehaviorOnLackOfSpace, NewPlannerItemsTextType,
// PlannerItemEnableEditMode), each as the model writes names.
var (
	plannerItemsTimeRepresentations = []PlannerItemsTimeRepresentation{"begin-and-end-time", "begin-time", "dont-display"}
	plannerItemsBehaviors           = []PlannerItemsBehaviorOnLackSpace{"collapse-items", "show-all-items"}
	plannerNewItemsTextTypes        = []PlannerNewItemsTextType{"string", "formatted-string"}
	plannerItemEnableEditModes      = []PlannerItemEnableEditMode{"disable-drag-and-stretch", "disable-edit", "disable-stretch", "enable-edit"}
)

// validatePlannerContent checks the content of a planner against itself.
func validatePlannerContent(path string, value *PlannerContent) []string {
	if value == nil {
		return nil
	}
	issues := validateChartLook(path, value.BackColor, value.TextColor, value.Font, value.Border, value.BorderColor)
	issues = append(issues, validateChartColor(path+".line_color", value.LineColor)...)
	issues = append(issues, validateChartText(path+".wrapped_time_scale_header_format", value.WrappedTimeScaleHeaderFormat)...)
	issues = append(issues, oneOfList(path+".periodic_variant_unit", value.PeriodicVariantUnit, timeScaleUnits, false)...)
	issues = append(issues, validateTimeScale(path+".time_scale", value.TimeScale)...)
	issues = append(issues, oneOfList(path+".items_time_representation", value.ItemsTimeRepresentation, plannerItemsTimeRepresentations, false)...)
	issues = append(issues, oneOfList(path+".items_behavior_when_space_insufficient", value.ItemsBehaviorWhenSpaceInsufficient, plannerItemsBehaviors, false)...)
	issues = append(issues, oneOfList(path+".new_items_text_type", value.NewItemsTextType, plannerNewItemsTextTypes, false)...)
	for _, number := range []struct {
		name  string
		value int
	}{{"periodic_variant_repetition", value.PeriodicVariantRepetition}, {"time_scale_wrap_begin_indent", value.TimeScaleWrapBeginIndent},
		{"time_scale_wrap_end_indent", value.TimeScaleWrapEndIndent}, {"min_column_width", value.MinColumnWidth}, {"min_row_height", value.MinRowHeight}} {
		if number.value < 0 {
			issues = append(issues, path+"."+number.name+" must not be negative")
		}
	}
	for _, date := range []struct{ name, value string }{{"begin_of_representation_period", value.BeginOfRepresentationPeriod},
		{"end_of_representation_period", value.EndOfRepresentationPeriod}} {
		_, dateIssues := ganttDate(path+"."+date.name, date.value)
		issues = append(issues, dateIssues...)
	}
	for index, period := range value.CurrentRepresentationPeriods {
		at := fmt.Sprintf("%s.current_representation_periods[%d]", path, index)
		_, beginIssues := ganttDate(at+".begin", period.Begin)
		_, endIssues := ganttDate(at+".end", period.End)
		issues = append(append(issues, beginIssues...), endIssues...)
	}
	for index, item := range value.Items {
		at := fmt.Sprintf("%s.items[%d]", path, index)
		issues = append(issues, validateChartLook(at, item.BackColor, item.TextColor, item.Font, item.Border, item.BorderColor)...)
		for _, date := range []struct{ name, value string }{{"begin", item.Begin}, {"end", item.End}, {"replacement_date", item.ReplacementDate}} {
			_, dateIssues := ganttDate(at+"."+date.name, date.value)
			issues = append(issues, dateIssues...)
		}
		issues = append(issues, oneOfList(at+".enable_edit_mode", item.EnableEditMode, plannerItemEnableEditModes, false)...)
		if item.State.ID != "" {
			if _, err := uuid.Parse(item.State.ID); err != nil {
				issues = append(issues, at+".state.id must be a UUID")
			}
		}
	}
	return issues
}

// stateFields names what the prototype wrote of its own state of the
// planner's items and time scale, for the note.
func (value *PlannerContent) stateFields() string {
	var names []string
	for index, item := range value.Items {
		if item.State != (PlannerItemState{}) {
			names = append(names, fmt.Sprintf("items[%d].state", index))
		}
	}
	if value.TimeScale.CurrentLevel != 0 {
		names = append(names, "time_scale.current_level")
	}
	for index, item := range value.TimeScale.Items {
		if item.LabelsTicks != 0 {
			names = append(names, fmt.Sprintf("time_scale.items[%d].labels_ticks", index))
		}
	}
	return strings.Join(names, ", ")
}

// plannerParts walks the content of a planner.
func (value *PlannerContent) plannerParts(visit func(path string, part any)) {
	walkChartParts(reflect.ValueOf(value).Elem(), visit)
}

// styleItems lists the style items the planner takes a colour, a font or a
// border from.
func (value *PlannerContent) styleItems() []styleItemUse { return styleItemsIn(value.plannerParts) }

// clone copies the content whole: nothing the copy holds is shared.
func (value *PlannerContent) clone() *PlannerContent {
	if value == nil {
		return nil
	}
	return deepCopy(reflect.ValueOf(value)).Interface().(*PlannerContent)
}
