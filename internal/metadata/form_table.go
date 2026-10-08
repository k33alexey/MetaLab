package metadata

// FormRowSelectionMode is what a table selects when a row is current: the
// cell or the whole row (help, TableRowSelectionMode).
type FormRowSelectionMode string

const (
	FormRowSelectionCell FormRowSelectionMode = "cell"
	FormRowSelectionRow  FormRowSelectionMode = "row"
)

// FormInitialListView is where a list opens (help, InitialListView).
type FormInitialListView string

const (
	FormInitialListAuto      FormInitialListView = "auto"
	FormInitialListBeginning FormInitialListView = "beginning"
	FormInitialListEnd       FormInitialListView = "end"
)

// FormInitialTreeView is how far a tree opens expanded (help,
// InitialTreeView).
type FormInitialTreeView string

const (
	FormInitialTreeNoExpand        FormInitialTreeView = "no-expand"
	FormInitialTreeExpandTopLevel  FormInitialTreeView = "expand-top-level"
	FormInitialTreeExpandAllLevels FormInitialTreeView = "expand-all-levels"
)

// FormRowInputMode is where a new row of a table is put (help,
// TableRowInputMode).
type FormRowInputMode string

const (
	FormRowInputEndOfList        FormRowInputMode = "end-of-list"
	FormRowInputEndOfWindow      FormRowInputMode = "end-of-window"
	FormRowInputAfterCurrentRow  FormRowInputMode = "after-current-row"
	FormRowInputBeforeCurrentRow FormRowInputMode = "before-current-row"
)

// FormTableLocation is where a table puts its command bar, its search
// string, its view status and its search control. The help gives each its
// own set (FormItemCommandBarLabelLocation, SearchStringLocation,
// ViewStatusLocation, SearchControlLocation); tableLocations holds them.
type FormTableLocation string

const (
	FormTableLocationAuto        FormTableLocation = "auto"
	FormTableLocationNone        FormTableLocation = "none"
	FormTableLocationTop         FormTableLocation = "top"
	FormTableLocationBottom      FormTableLocation = "bottom"
	FormTableLocationCommandBar  FormTableLocation = "command-bar"
	FormTableLocationFormCaption FormTableLocation = "form-caption"
	FormTableLocationPullFromTop FormTableLocation = "pull-from-top"
)

// FormSearchOnInput is whether typing into a table searches it (help,
// SearchInTableOnInput).
type FormSearchOnInput string

const (
	FormSearchOnInputAuto    FormSearchOnInput = "auto"
	FormSearchOnInputUse     FormSearchOnInput = "use"
	FormSearchOnInputDontUse FormSearchOnInput = "dont-use"
)

// FormRefreshRequest is how a table is asked to refresh in the mobile client
// (help, RefreshRequestMethod). None is a value of its own here, so empty is
// not said.
type FormRefreshRequest string

const (
	FormRefreshRequestNone                FormRefreshRequest = "none"
	FormRefreshRequestPullFromBottom      FormRefreshRequest = "pull-from-bottom"
	FormRefreshRequestPullFromTop         FormRefreshRequest = "pull-from-top"
	FormRefreshRequestPullFromTopOrBottom FormRefreshRequest = "pull-from-top-or-bottom"
)

// FormHorizontalCompression is what a table does when the screen is too
// narrow for it (help, TableBehaviorOnHorizontalCompression).
type FormHorizontalCompression string

const (
	FormHorizontalCompressionAuto                  FormHorizontalCompression = "auto"
	FormHorizontalCompressionHideItemsByImportance FormHorizontalCompression = "hide-items-by-importance"
	FormHorizontalCompressionMoveItemsByImportance FormHorizontalCompression = "move-items-by-importance"
)

// TableProperties is what a table has of its own (help, FormTable), apart
// from what it shares with a field - its size, title, look, scroll bars,
// dragging and output, carried in the groups of a field - and from what a
// table of a dynamic list or of the settings of a composition has.
//
// The help gives two more, which the prototype writes on no table of the
// exports: where the panel of the hierarchy stands, and what the table does
// while the main server is out of reach. They are not carried.
type TableProperties struct {
	RowSelectionMode FormRowSelectionMode `yaml:"row_selection_mode,omitempty" json:"rowSelectionMode,omitempty"`
	// NoHeader hides the header of the table and Footer shows its footer:
	// the prototype writes only these (1726 and 238 times). HeaderHeight
	// and FooterHeight are in rows; each is not said or a number, as the
	// prototype writes a footer of 0 rows once and the help names no
	// default to tell it from.
	NoHeader     bool `yaml:"no_header,omitempty" json:"noHeader,omitempty"`
	Footer       bool `yaml:"footer,omitempty" json:"footer,omitempty"`
	HeaderHeight *int `yaml:"header_height,omitempty" json:"headerHeight,omitempty"`
	FooterHeight *int `yaml:"footer_height,omitempty" json:"footerHeight,omitempty"`
	// NoHorizontalLines and NoVerticalLines hide the lines between rows and
	// columns, and UseAlternationRowColor alternates the colour of the rows;
	// the prototype writes only these (1387, 1322 and 3940 times).
	NoHorizontalLines      bool `yaml:"no_horizontal_lines,omitempty" json:"noHorizontalLines,omitempty"`
	NoVerticalLines        bool `yaml:"no_vertical_lines,omitempty" json:"noVerticalLines,omitempty"`
	UseAlternationRowColor bool `yaml:"use_alternation_row_color,omitempty" json:"useAlternationRowColor,omitempty"`
	// AutoInsertNewRow adds a row once the current one is filled in;
	// NoChangeRowSet keeps rows from being added and deleted, and
	// NoChangeRowOrder from being moved. The prototype writes only these
	// (4304, 1545 and 1531 times).
	AutoInsertNewRow bool `yaml:"auto_insert_new_row,omitempty" json:"autoInsertNewRow,omitempty"`
	NoChangeRowSet   bool `yaml:"no_change_row_set,omitempty" json:"noChangeRowSet,omitempty"`
	NoChangeRowOrder bool `yaml:"no_change_row_order,omitempty" json:"noChangeRowOrder,omitempty"`
	// AutoAddIncomplete adds a row as the table is entered while it has
	// none; yes, no or not said, as the help gives Undefined beside the two
	// and the prototype writes both (12 and 309 times).
	AutoAddIncomplete *bool            `yaml:"auto_add_incomplete,omitempty" json:"autoAddIncomplete,omitempty"`
	RowInputMode      FormRowInputMode `yaml:"row_input_mode,omitempty" json:"rowInputMode,omitempty"`
	// ChoiceMode makes choosing a row pick it for the one who opened the
	// form, and MultipleChoice lets several be picked; the prototype writes
	// only these (1166 and 74 times).
	ChoiceMode      bool                `yaml:"choice_mode,omitempty" json:"choiceMode,omitempty"`
	MultipleChoice  bool                `yaml:"multiple_choice,omitempty" json:"multipleChoice,omitempty"`
	InitialListView FormInitialListView `yaml:"initial_list_view,omitempty" json:"initialListView,omitempty"`
	InitialTreeView FormInitialTreeView `yaml:"initial_tree_view,omitempty" json:"initialTreeView,omitempty"`
	// CommandBarLocation, SearchStringLocation, ViewStatusLocation and
	// SearchControlLocation are where the table puts its command bar and
	// its additions. The prototype writes None for the search string 2499
	// times, which the help does not give it; it is carried as written, as
	// it means what it says on the other three.
	CommandBarLocation    FormTableLocation `yaml:"command_bar_location,omitempty" json:"commandBarLocation,omitempty"`
	SearchStringLocation  FormTableLocation `yaml:"search_string_location,omitempty" json:"searchStringLocation,omitempty"`
	ViewStatusLocation    FormTableLocation `yaml:"view_status_location,omitempty" json:"viewStatusLocation,omitempty"`
	SearchControlLocation FormTableLocation `yaml:"search_control_location,omitempty" json:"searchControlLocation,omitempty"`
	SearchOnInput         FormSearchOnInput `yaml:"search_on_input,omitempty" json:"searchOnInput,omitempty"`
	// HeightInTableRows is the height of the table in rows of data, the
	// height in characters applying while it is 0 (help). NoAutoMaxRowsCount
	// turns off the platform's own limit, so that MaxRowsCount applies, 0
	// being none (help, AutoMaxHeightInTableRows and MaxHeightInTableRows -
	// the prototype writes them AutoMaxRowsCount and MaxRowsCount, and only
	// the "off", 23 times).
	HeightInTableRows  int  `yaml:"height_in_table_rows,omitempty" json:"heightInTableRows,omitempty"`
	NoAutoMaxRowsCount bool `yaml:"no_auto_max_rows_count,omitempty" json:"noAutoMaxRowsCount,omitempty"`
	MaxRowsCount       int  `yaml:"max_rows_count,omitempty" json:"maxRowsCount,omitempty"`
	// RowsPicture is the set of pictures a row picks from by the number or
	// the boolean RowPictureDataPath names, written as the data path of an
	// element is.
	RowsPicture        *PictureReference `yaml:"rows_picture,omitempty" json:"rowsPicture,omitempty"`
	RowPictureDataPath string            `yaml:"row_picture_data_path,omitempty" json:"rowPictureDataPath,omitempty"`
	// RefreshRequest and BehaviorOnHorizontalCompression are of the mobile
	// client.
	RefreshRequest                  FormRefreshRequest        `yaml:"refresh_request,omitempty" json:"refreshRequest,omitempty"`
	BehaviorOnHorizontalCompression FormHorizontalCompression `yaml:"behavior_on_horizontal_compression,omitempty" json:"behaviorOnHorizontalCompression,omitempty"`
}

// tableLocations are where the help lets a table put each of its parts.
var tableLocations = map[string][]FormTableLocation{
	"command_bar_location": {FormTableLocationAuto, FormTableLocationNone, FormTableLocationTop, FormTableLocationBottom},
	"search_string_location": {FormTableLocationAuto, FormTableLocationNone, FormTableLocationTop, FormTableLocationBottom,
		FormTableLocationCommandBar, FormTableLocationFormCaption, FormTableLocationPullFromTop},
	"view_status_location":    {FormTableLocationAuto, FormTableLocationNone, FormTableLocationTop, FormTableLocationBottom},
	"search_control_location": {FormTableLocationAuto, FormTableLocationNone, FormTableLocationCommandBar},
}

func (table TableProperties) empty() bool {
	return table.RowSelectionMode == "" && !table.NoHeader && !table.Footer && table.HeaderHeight == nil && table.FooterHeight == nil &&
		!table.NoHorizontalLines && !table.NoVerticalLines && !table.UseAlternationRowColor && !table.AutoInsertNewRow &&
		!table.NoChangeRowSet && !table.NoChangeRowOrder && table.AutoAddIncomplete == nil && table.RowInputMode == "" &&
		!table.ChoiceMode && !table.MultipleChoice && table.InitialListView == "" && table.InitialTreeView == "" &&
		table.CommandBarLocation == "" && table.SearchStringLocation == "" && table.ViewStatusLocation == "" &&
		table.SearchControlLocation == "" && table.SearchOnInput == "" && table.HeightInTableRows == 0 && !table.NoAutoMaxRowsCount &&
		table.MaxRowsCount == 0 && table.RowsPicture == nil && table.RowPictureDataPath == "" && table.RefreshRequest == "" &&
		table.BehaviorOnHorizontalCompression == ""
}

func validateTableProperties(path string, table TableProperties, kind FormElementKind) []string {
	if table.empty() {
		return nil
	}
	if kind != FormElementTable {
		return []string{path + " has what only a table has"}
	}
	var issues []string
	issues = append(issues, oneOf(path+".row_selection_mode", table.RowSelectionMode, FormRowSelectionCell, FormRowSelectionRow)...)
	for _, size := range []struct {
		name  string
		value int
	}{{"height_in_table_rows", table.HeightInTableRows}, {"max_rows_count", table.MaxRowsCount}} {
		if size.value < 0 {
			issues = append(issues, path+"."+size.name+" must not be negative")
		}
	}
	for _, height := range []struct {
		name  string
		value *int
	}{{"header_height", table.HeaderHeight}, {"footer_height", table.FooterHeight}} {
		if height.value != nil && *height.value < 0 {
			issues = append(issues, path+"."+height.name+" must not be negative")
		}
	}
	issues = append(issues, oneOf(path+".row_input_mode", table.RowInputMode, FormRowInputEndOfList, FormRowInputEndOfWindow,
		FormRowInputAfterCurrentRow, FormRowInputBeforeCurrentRow)...)
	issues = append(issues, oneOf(path+".initial_list_view", table.InitialListView, FormInitialListAuto, FormInitialListBeginning, FormInitialListEnd)...)
	issues = append(issues, oneOf(path+".initial_tree_view", table.InitialTreeView, FormInitialTreeNoExpand, FormInitialTreeExpandTopLevel,
		FormInitialTreeExpandAllLevels)...)
	for _, location := range []struct {
		name  string
		value FormTableLocation
	}{
		{"command_bar_location", table.CommandBarLocation}, {"search_string_location", table.SearchStringLocation},
		{"view_status_location", table.ViewStatusLocation}, {"search_control_location", table.SearchControlLocation},
	} {
		issues = append(issues, oneOf(path+"."+location.name, location.value, tableLocations[location.name]...)...)
	}
	issues = append(issues, oneOf(path+".search_on_input", table.SearchOnInput, FormSearchOnInputAuto, FormSearchOnInputUse, FormSearchOnInputDontUse)...)
	issues = append(issues, validatePictureReference(path+".rows_picture", table.RowsPicture)...)
	if table.RowPictureDataPath != "" {
		issues = append(issues, validateElementDataPath(path+".row_picture_data_path", table.RowPictureDataPath)...)
	}
	issues = append(issues, oneOf(path+".refresh_request", table.RefreshRequest, FormRefreshRequestNone, FormRefreshRequestPullFromBottom,
		FormRefreshRequestPullFromTop, FormRefreshRequestPullFromTopOrBottom)...)
	issues = append(issues, oneOf(path+".behavior_on_horizontal_compression", table.BehaviorOnHorizontalCompression,
		FormHorizontalCompressionAuto, FormHorizontalCompressionHideItemsByImportance, FormHorizontalCompressionMoveItemsByImportance)...)
	return issues
}

func (table TableProperties) clone() TableProperties {
	table.HeaderHeight, table.FooterHeight = clonePointer(table.HeaderHeight), clonePointer(table.FooterHeight)
	table.AutoAddIncomplete = clonePointer(table.AutoAddIncomplete)
	table.RowsPicture = table.RowsPicture.clone()
	return table
}
