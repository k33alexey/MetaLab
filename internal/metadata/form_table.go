package metadata

import (
	"regexp"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

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
	// DynamicList is what a table showing a dynamic list has besides.
	DynamicList *TableDynamicList `yaml:"dynamic_list,omitempty" json:"dynamicList,omitempty"`
	// ViewMode is whether a table of a filter or of the user settings of a
	// composition shows every item or only those of quick access (help, the
	// extensions of a table for them); NoNamedItemDetailedRepresentation
	// shows a named item of a filter or a conditional appearance in one row
	// with its presentation, which the prototype writes 205 times - the
	// help's default is a column of its own. The prototype writes both on
	// tables of the settings of a composition only.
	ViewMode                          FormSettingsViewMode `yaml:"view_mode,omitempty" json:"viewMode,omitempty"`
	NoNamedItemDetailedRepresentation bool                 `yaml:"no_named_item_detailed_representation,omitempty" json:"noNamedItemDetailedRepresentation,omitempty"`
	// Autofill is written on 267 tables - of the settings of a composition,
	// value tables and lists, always true - and the help does not know it on
	// a table at all. It is carried as written and noted
	// (NotePropertyOutsideHelp).
	Autofill bool `yaml:"autofill,omitempty" json:"autofill,omitempty"`
}

// FormSettingsViewMode is what a table of the settings of a composition
// shows (help, DataCompositionSettingsViewMode).
type FormSettingsViewMode string

const (
	FormSettingsViewAll         FormSettingsViewMode = "all"
	FormSettingsViewQuickAccess FormSettingsViewMode = "quick-access"
)

// FormUpdateOnDataChange is whether a list refreshes when its data is added
// or changed interactively (help, UpdateOnDataChange).
type FormUpdateOnDataChange string

const (
	FormUpdateOnDataChangeAuto       FormUpdateOnDataChange = "auto"
	FormUpdateOnDataChangeDontUpdate FormUpdateOnDataChange = "dont-update"
)

// TableDynamicList is what a table showing a dynamic list has besides (help,
// the extension of a table for a dynamic list). The prototype writes all of
// it on every such table - 3731 of them - and on no other, so it is one
// record, there or not: each value is the one written.
type TableDynamicList struct {
	// AutoRefresh refreshes the list every AutoRefreshPeriod seconds.
	AutoRefresh       bool `yaml:"auto_refresh,omitempty" json:"autoRefresh,omitempty"`
	AutoRefreshPeriod int  `yaml:"auto_refresh_period,omitempty" json:"autoRefreshPeriod,omitempty"`
	// Period is the period the list shows its data for.
	Period FormStandardPeriod `yaml:"period" json:"period"`
	// ChoiceFoldersAndItems is what of a hierarchy may be chosen.
	ChoiceFoldersAndItems FormFoldersAndItems `yaml:"choice_folders_and_items,omitempty" json:"choiceFoldersAndItems,omitempty"`
	// RestoreCurrentRow keeps the current row for the next opening.
	RestoreCurrentRow bool `yaml:"restore_current_row,omitempty" json:"restoreCurrentRow,omitempty"`
	// TopLevelParent is the item of a catalog, a chart of characteristic
	// types or a chart of accounts the list shows as its root; nil is none.
	// The prototype writes none on every table of the exports.
	TopLevelParent *Value `yaml:"top_level_parent,omitempty" json:"topLevelParent,omitempty"`
	// ShowRoot and AllowRootChoice show the root of a list drawn as a tree,
	// and let it be chosen.
	ShowRoot           bool                   `yaml:"show_root,omitempty" json:"showRoot,omitempty"`
	AllowRootChoice    bool                   `yaml:"allow_root_choice,omitempty" json:"allowRootChoice,omitempty"`
	UpdateOnDataChange FormUpdateOnDataChange `yaml:"update_on_data_change,omitempty" json:"updateOnDataChange,omitempty"`
	// AllowGettingCurrentRowURL lets the link to the current row be taken.
	AllowGettingCurrentRowURL bool `yaml:"allow_getting_current_row_url,omitempty" json:"allowGettingCurrentRowUrl,omitempty"`
	// UserSettingsGroup is the element of the form the user settings of the
	// list are shown in, named by its name (2141 times) or by the code of an
	// element (65, see validateFormLinkPath), carried as written.
	UserSettingsGroup string `yaml:"user_settings_group,omitempty" json:"userSettingsGroup,omitempty"`
}

// FormStandardPeriod is a standard period as the form keeps it: its variant,
// and for a custom one its dates, written as the prototype writes a date and
// left empty for the empty date (0001-01-01T00:00:00).
type FormStandardPeriod struct {
	Variant   FormStandardPeriodVariant `yaml:"variant" json:"variant"`
	StartDate string                    `yaml:"start_date,omitempty" json:"startDate,omitempty"`
	EndDate   string                    `yaml:"end_date,omitempty" json:"endDate,omitempty"`
}

// FormStandardPeriodVariant is one of the periods the platform knows (help,
// StandardPeriodVariant), spelled as the model spells an enumeration.
type FormStandardPeriodVariant string

var formStandardPeriodVariants = []FormStandardPeriodVariant{"custom",
	"today", "yesterday", "tomorrow", "this-week", "this-ten-days", "this-month", "this-quarter", "this-half-year", "this-year",
	"from-beginning-of-this-week", "from-beginning-of-this-ten-days", "from-beginning-of-this-month", "from-beginning-of-this-quarter",
	"from-beginning-of-this-half-year", "from-beginning-of-this-year",
	"till-end-of-this-week", "till-end-of-this-ten-days", "till-end-of-this-month", "till-end-of-this-quarter",
	"till-end-of-this-half-year", "till-end-of-this-year",
	"last-7-days", "last-week", "last-ten-days", "last-month", "last-quarter", "last-half-year", "last-year",
	"last-week-till-same-week-day", "last-ten-days-till-same-day-number", "last-month-till-same-date", "last-quarter-till-same-date",
	"last-half-year-till-same-date", "last-year-till-same-date",
	"next-7-days", "next-week", "next-ten-days", "next-month", "next-quarter", "next-half-year", "next-year",
	"next-week-till-same-week-day", "next-ten-days-till-same-day-number", "next-month-till-same-date", "next-quarter-till-same-date",
	"next-half-year-till-same-date", "next-year-till-same-date", "month"}

// formDateTime is a date as the prototype writes one in a form.
var formDateTime = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}$`)

func validateTableDynamicList(path string, list TableDynamicList) []string {
	var issues []string
	if list.AutoRefreshPeriod < 0 {
		issues = append(issues, path+".auto_refresh_period must not be negative")
	}
	issues = append(issues, oneOf(path+".period.variant", list.Period.Variant, formStandardPeriodVariants...)...)
	if list.Period.Variant == "" {
		issues = append(issues, path+".period.variant must be named")
	}
	for _, date := range []struct{ name, value string }{{"start_date", list.Period.StartDate}, {"end_date", list.Period.EndDate}} {
		if date.value != "" {
			if _, err := time.Parse("2006-01-02T15:04:05", date.value); err != nil || !formDateTime.MatchString(date.value) {
				issues = append(issues, path+".period."+date.name+" must be a date written as 2006-01-02T15:04:05")
			}
		}
	}
	issues = append(issues, oneOf(path+".choice_folders_and_items", list.ChoiceFoldersAndItems, FormFolders, FormItems, FormFoldersAndItemsBoth)...)
	if parent := list.TopLevelParent; parent != nil {
		switch parent.Kind {
		case CatalogType, CharacteristicTypesType, AccountType:
			if parent.Object.IsZero() || parent.Data == "" {
				issues = append(issues, path+".top_level_parent must name its object and its item")
			}
		case UndefinedValue, UnresolvedReferenceValue:
			issues = append(issues, validateDesignTimeValue(path+".top_level_parent", *parent)...)
		default:
			issues = append(issues, path+".top_level_parent must be an item of a catalog, a chart of characteristic types or a chart of accounts")
		}
	}
	issues = append(issues, oneOf(path+".update_on_data_change", list.UpdateOnDataChange, FormUpdateOnDataChangeAuto, FormUpdateOnDataChangeDontUpdate)...)
	if group := list.UserSettingsGroup; group != "" && !formElementCode.MatchString(group) &&
		(!validIdentifier(group) || utf8.RuneCountInString(group) > maxNameLength) {
		issues = append(issues, path+".user_settings_group must be the name or the code of an element of the form")
	}
	return issues
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
		table.BehaviorOnHorizontalCompression == "" && table.DynamicList == nil && table.ViewMode == "" &&
		!table.NoNamedItemDetailedRepresentation && !table.Autofill
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
	if table.DynamicList != nil {
		issues = append(issues, validateTableDynamicList(path+".dynamic_list", *table.DynamicList)...)
	}
	issues = append(issues, oneOf(path+".view_mode", table.ViewMode, FormSettingsViewAll, FormSettingsViewQuickAccess)...)
	return issues
}

func (table TableProperties) clone() TableProperties {
	table.HeaderHeight, table.FooterHeight = clonePointer(table.HeaderHeight), clonePointer(table.FooterHeight)
	table.AutoAddIncomplete = clonePointer(table.AutoAddIncomplete)
	table.RowsPicture = table.RowsPicture.clone()
	if list := table.DynamicList; list != nil {
		copied := *list
		copied.TopLevelParent = clonePointer(list.TopLevelParent)
		table.DynamicList = &copied
	}
	return table
}

// resolveFormTables resolves and notes what the tables of a form name: the
// item a dynamic list shows as its root, with the project; the element its
// user settings are shown in - the help makes it a group, and the prototype
// keeps an input field there twice (NoteUserSettingsGroupNotGroup) and a
// code 70 times (NoteFormReferenceAsWritten); and the autofill the help does
// not know (NotePropertyOutsideHelp); and an addition a table holds that
// names another element as its source (NoteHeldAdditionOfAnother); and an
// addition standing apart that names no table (NoteAdditionOfNoTable).
func (catalog *Catalog) resolveFormTables(where string, form ManagedForm) {
	kinds := map[string]FormElementKind{}
	var tables []ManagedFormElement
	held := map[uuid.UUID]bool{}
	var walk func(items []ManagedFormElement)
	walk = func(items []ManagedFormElement) {
		for _, item := range items {
			if _, seen := kinds[foldedName(item.Name)]; !seen {
				kinds[foldedName(item.Name)] = item.Kind
			}
			if item.Kind == FormElementTable {
				tables = append(tables, item)
				for _, addition := range item.tableAdditions() {
					if addition.addition != nil {
						held[addition.addition.ID] = true
					}
				}
			}
			if formElementClasses[item.Kind] == formAdditionClass && item.AdditionSource == "" && !held[item.ID] {
				catalog.noteForm(NoteAdditionOfNoTable, where+" element "+item.Name, string(item.Kind))
			}
			walk(item.Nested())
		}
	}
	walk(form.FormItems())
	for _, table := range tables {
		at := where + " element " + table.Name
		if table.Autofill {
			catalog.noteForm(NotePropertyOutsideHelp, at+" autofill", "true")
		}
		for _, held := range table.tableAdditions() {
			if held.addition != nil && held.addition.AdditionSource != "" {
				catalog.noteForm(NoteHeldAdditionOfAnother, at+" "+held.name, held.addition.AdditionSource)
			}
		}
		list := table.DynamicList
		if list == nil {
			continue
		}
		if group := list.UserSettingsGroup; formElementCode.MatchString(group) {
			catalog.noteElementCode(at+" user_settings_group", group)
		} else if kind, ok := kinds[foldedName(group)]; group != "" && ok && !slices.Contains(formGroupKinds, kind) {
			catalog.noteForm(NoteUserSettingsGroupNotGroup, at+" user_settings_group", group+" ("+string(kind)+")")
		}
		if parent := list.TopLevelParent; parent != nil {
			switch parent.Kind {
			case UnresolvedReferenceValue:
				catalog.unresolved = append(catalog.unresolved, UnresolvedReference{Where: at + " top_level_parent", Written: parent.Data})
			case CatalogType, CharacteristicTypesType, AccountType:
				if _, ok := catalog.objectKindByID[parent.Object]; !ok {
					catalog.noteUnresolved(at+" top_level_parent", parent.Object)
				}
			}
		}
	}
}

// TableAdditions are the search string, the view status and the search
// control of a table (help, FormTable.SearchStringRepresentation,
// ViewStatusRepresentation, SearchControl). The prototype writes all three
// inside every table - 9005 of each in the exports - beside its columns, not
// among them: each is an element of the form, with an identifier, a name
// code reaches it by, a context menu and what it is drawn with, and the
// table puts it where its locations say. An addition that stands elsewhere -
// in a command bar or a group, 351 in the exports - is an element of the
// tree naming its table (AdditionSource).
type TableAdditions struct {
	SearchStringAddition  *ManagedFormElement `yaml:"search_string_addition,omitempty" json:"searchStringAddition,omitempty"`
	ViewStatusAddition    *ManagedFormElement `yaml:"view_status_addition,omitempty" json:"viewStatusAddition,omitempty"`
	SearchControlAddition *ManagedFormElement `yaml:"search_control_addition,omitempty" json:"searchControlAddition,omitempty"`
}

// tableAdditions are the additions of a table with the place each stands in
// and the kind it is.
func (element ManagedFormElement) tableAdditions() []struct {
	name     string
	kind     FormElementKind
	addition *ManagedFormElement
} {
	return []struct {
		name     string
		kind     FormElementKind
		addition *ManagedFormElement
	}{
		{"search_string_addition", FormElementSearchStringAddition, element.SearchStringAddition},
		{"view_status_addition", FormElementViewStatusAddition, element.ViewStatusAddition},
		{"search_control_addition", FormElementSearchControlAddition, element.SearchControlAddition},
	}
}

// FormServerUnavailableBehavior is what an element does while the main
// server is out of reach (help, OnMainServerUnavalableBehavior).
type FormServerUnavailableBehavior string

const (
	FormServerUnavailableAuto        FormServerUnavailableBehavior = "auto"
	FormServerUnavailableDontChange  FormServerUnavailableBehavior = "dont-change-behavior"
	FormServerUnavailableMakeDisable FormServerUnavailableBehavior = "make-disable"
)

func (additions TableAdditions) clone() TableAdditions {
	for _, addition := range []**ManagedFormElement{&additions.SearchStringAddition, &additions.ViewStatusAddition, &additions.SearchControlAddition} {
		if *addition != nil {
			copied := cloneRuntimeFormElements([]ManagedFormElement{**addition})[0]
			*addition = &copied
		}
	}
	return additions
}
