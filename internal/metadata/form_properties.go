package metadata

import (
	"slices"
	"strings"
)

// FormWindowOpeningMode is how the window of a form is opened (help,
// FormWindowOpeningMode). Empty is Independent, which the prototype never
// writes: 3301 forms of the three configurations being moved lock the owner
// window, 70 the whole interface.
type FormWindowOpeningMode string

const (
	FormWindowIndependent        FormWindowOpeningMode = "independent"
	FormWindowLockOwnerWindow    FormWindowOpeningMode = "lock-owner-window"
	FormWindowLockWholeInterface FormWindowOpeningMode = "lock-whole-interface"
)

// FormCommandBarLocation is where the automatic command bar of a form stands
// (help, FormCommandBarLabelLocation). Empty is Auto, which the prototype
// never writes.
type FormCommandBarLocation string

const (
	FormCommandBarAuto   FormCommandBarLocation = "auto"
	FormCommandBarTop    FormCommandBarLocation = "top"
	FormCommandBarBottom FormCommandBarLocation = "bottom"
	FormCommandBarNone   FormCommandBarLocation = "none"
)

// FormEnterKeyBehavior is what Enter does in a form (help,
// EnterKeyBehaviorType). Empty is ControlNavigation, which the prototype
// never writes; 25 forms press the default button.
type FormEnterKeyBehavior string

const (
	FormEnterControlNavigation FormEnterKeyBehavior = "control-navigation"
	FormEnterDefaultButton     FormEnterKeyBehavior = "default-button"
)

// FormConversations is whether a form shows its conversations (help,
// FormConversationsRepresentation). Empty is Auto, which the prototype never
// writes.
type FormConversations string

const (
	FormConversationsAuto     FormConversations = "auto"
	FormConversationsShow     FormConversations = "show"
	FormConversationsDontShow FormConversations = "dont-show"
)

// FormWindow is the window of a form and how it behaves: the properties the
// help gives a client application form (ClientApplicationForm), as the form
// description of the prototype keeps them. They are carried here and run by
// the form (blocks 7 and 8).
//
// What is on until turned off is kept as its "off", as the prototype writes
// only that: AutoTitle, ShowTitle, ShowCloseButton, Enabled, AutoURL,
// AutoFillCheck and Customizable are written false and never true.
type FormWindow struct {
	WindowOpeningMode FormWindowOpeningMode `yaml:"window_opening_mode,omitempty" json:"windowOpeningMode,omitempty"`
	// NoAutoTitle shows the title alone, without what the main attribute adds
	// to it (3060 forms).
	NoAutoTitle     bool `yaml:"no_auto_title,omitempty" json:"noAutoTitle,omitempty"`
	HideTitle       bool `yaml:"hide_title,omitempty" json:"hideTitle,omitempty"`
	HideCloseButton bool `yaml:"hide_close_button,omitempty" json:"hideCloseButton,omitempty"`
	// CommandBarLocation is where the automatic command bar stands; none
	// takes it away (1303 forms).
	CommandBarLocation FormCommandBarLocation `yaml:"command_bar_location,omitempty" json:"commandBarLocation,omitempty"`
	EnterKeyBehavior   FormEnterKeyBehavior   `yaml:"enter_key_behavior,omitempty" json:"enterKeyBehavior,omitempty"`
	Disabled           bool                   `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	Conversations      FormConversations      `yaml:"conversations,omitempty" json:"conversations,omitempty"`
	// NoAutoURL makes the form return the URL of the main form of its window
	// rather than its own.
	NoAutoURL       bool `yaml:"no_auto_url,omitempty" json:"noAutoURL,omitempty"`
	NoAutoFillCheck bool `yaml:"no_auto_fill_check,omitempty" json:"noAutoFillCheck,omitempty"`
	// AutoSaveDataInSettings saves the data of the form in its settings
	// without being asked (623 forms); SaveDataInSettings saves what the
	// attributes list for it (105). Both are off unless turned on: the
	// prototype writes Use and UseList, never DontUse.
	AutoSaveDataInSettings bool `yaml:"auto_save_data_in_settings,omitempty" json:"autoSaveDataInSettings,omitempty"`
	SaveDataInSettings     bool `yaml:"save_data_in_settings,omitempty" json:"saveDataInSettings,omitempty"`
	// NotCustomizable is the prototype's Customizable written false (485
	// forms). The help names no such property of a form, so what it switches
	// off is not known; it is carried as written and nothing runs it
	// (STUDIO-FORM-DESIGNER.md).
	NotCustomizable bool `yaml:"not_customizable,omitempty" json:"notCustomizable,omitempty"`
}

func validateFormWindow(window FormWindow) []string {
	var issues []string
	issues = append(issues, oneOf("window_opening_mode", window.WindowOpeningMode, FormWindowIndependent, FormWindowLockOwnerWindow, FormWindowLockWholeInterface)...)
	issues = append(issues, oneOf("command_bar_location", window.CommandBarLocation, FormCommandBarAuto, FormCommandBarTop, FormCommandBarBottom, FormCommandBarNone)...)
	issues = append(issues, oneOf("enter_key_behavior", window.EnterKeyBehavior, FormEnterControlNavigation, FormEnterDefaultButton)...)
	issues = append(issues, oneOf("conversations", window.Conversations, FormConversationsAuto, FormConversationsShow, FormConversationsDontShow)...)
	return issues
}

// FormVerticalScroll is how a form scrolls up and down (help,
// VerticalFormScroll). Empty is Auto, which the prototype never writes.
type FormVerticalScroll string

const (
	FormScrollAuto              FormVerticalScroll = "auto"
	FormScrollUse               FormVerticalScroll = "use"
	FormScrollUseWithoutStretch FormVerticalScroll = "use-without-stretch"
	FormScrollUseIfNecessary    FormVerticalScroll = "use-if-necessary"
)

// ChildrenGroup is how the items inside a form or a group are laid out
// (help, ChildFormItemsGroup). Empty is Vertical, which the prototype never
// writes.
type ChildrenGroup string

const (
	ChildrenVertical             ChildrenGroup = "vertical"
	ChildrenHorizontal           ChildrenGroup = "horizontal"
	ChildrenAlwaysHorizontal     ChildrenGroup = "always-horizontal"
	ChildrenHorizontalIfPossible ChildrenGroup = "horizontal-if-possible"
)

// ChildrenWidth is how wide the items laid side by side are against one
// another (help, ChildFormItemsWidth). Empty is Auto.
type ChildrenWidth string

const (
	ChildrenWidthAuto          ChildrenWidth = "auto"
	ChildrenWidthEqual         ChildrenWidth = "equal"
	ChildrenWidthLeftNarrowest ChildrenWidth = "left-narrowest"
	ChildrenWidthLeftNarrow    ChildrenWidth = "left-narrow"
	ChildrenWidthLeftWide      ChildrenWidth = "left-wide"
	ChildrenWidthLeftWidest    ChildrenWidth = "left-widest"
)

// ItemHorizontalAlign is where an item stands across (help,
// ItemHorizontalLocation). Empty is Auto.
type ItemHorizontalAlign string

const (
	ItemHorizontalAuto   ItemHorizontalAlign = "auto"
	ItemHorizontalLeft   ItemHorizontalAlign = "left"
	ItemHorizontalCenter ItemHorizontalAlign = "center"
	ItemHorizontalRight  ItemHorizontalAlign = "right"
)

// ItemVerticalAlign is where an item stands up and down (help,
// ItemVerticalAlign). Empty is Auto.
type ItemVerticalAlign string

const (
	ItemVerticalAuto   ItemVerticalAlign = "auto"
	ItemVerticalTop    ItemVerticalAlign = "top"
	ItemVerticalCenter ItemVerticalAlign = "center"
	ItemVerticalBottom ItemVerticalAlign = "bottom"
)

// ItemsAndTitlesAlign is how the items and their titles are lined up (help,
// ItemsAndTitlesAlignVariant). Empty is Auto. TitlesLeftDataAuto is not among
// the values the help gives; the prototype writes it (once in the forms being
// moved, three times in its demo base), so it is carried as written and what
// it does is not known (STUDIO-FORM-DESIGNER.md).
type ItemsAndTitlesAlign string

const (
	ItemsAndTitlesAuto               ItemsAndTitlesAlign = "auto"
	ItemsAndTitlesNone               ItemsAndTitlesAlign = "none"
	ItemsLeftTitlesLeft              ItemsAndTitlesAlign = "items-left-titles-left"
	ItemsLeftTitlesRight             ItemsAndTitlesAlign = "items-left-titles-right"
	ItemsRightTitlesLeft             ItemsAndTitlesAlign = "items-right-titles-left"
	ItemsRightTitlesRight            ItemsAndTitlesAlign = "items-right-titles-right"
	ItemsAndTitlesTitlesLeftDataAuto ItemsAndTitlesAlign = "titles-left-data-auto"
)

// ItemSpacing is the space between items (help, FormItemSpacing). Empty is
// Auto.
type ItemSpacing string

const (
	ItemSpacingAuto       ItemSpacing = "auto"
	ItemSpacingNone       ItemSpacing = "none"
	ItemSpacingHalf       ItemSpacing = "half"
	ItemSpacingSingle     ItemSpacing = "single"
	ItemSpacingOneAndHalf ItemSpacing = "one-and-half"
	ItemSpacingDouble     ItemSpacing = "double"
)

// FormScaleVariant is the scale of a form: normal or compact. The help does
// not name it for a form; it gives these values to the scale of the forms of
// the client (ClientApplicationFormScaleVariant), and the prototype writes
// Normal 41 and Compact 7 times in a form, never Auto. Empty is Auto.
type FormScaleVariant string

const (
	FormScaleAuto    FormScaleVariant = "auto"
	FormScaleNormal  FormScaleVariant = "normal"
	FormScaleCompact FormScaleVariant = "compact"
)

// CollapseByImportance is whether the items of a form are collapsed by their
// importance to fit a narrow screen (help, CollapseFormItemsByImportance).
// Empty is Auto.
type CollapseByImportance string

const (
	CollapseByImportanceAuto    CollapseByImportance = "auto"
	CollapseByImportanceUse     CollapseByImportance = "use"
	CollapseByImportanceDontUse CollapseByImportance = "dont-use"
)

// FormLayout is how a form lays out what it holds (help,
// ClientApplicationForm). Width and height are in characters; 0 is chosen by
// the platform. The prototype sets no limit on them: the forms being moved
// are 18 to 400 wide and 2 to 150 high.
type FormLayout struct {
	VerticalScroll       FormVerticalScroll   `yaml:"vertical_scroll,omitempty" json:"verticalScroll,omitempty"`
	Width                int                  `yaml:"width,omitempty" json:"width,omitempty"`
	Height               int                  `yaml:"height,omitempty" json:"height,omitempty"`
	ChildrenGroup        ChildrenGroup        `yaml:"children_group,omitempty" json:"childrenGroup,omitempty"`
	ChildrenWidth        ChildrenWidth        `yaml:"children_width,omitempty" json:"childrenWidth,omitempty"`
	HorizontalAlign      ItemHorizontalAlign  `yaml:"horizontal_align,omitempty" json:"horizontalAlign,omitempty"`
	VerticalAlign        ItemVerticalAlign    `yaml:"vertical_align,omitempty" json:"verticalAlign,omitempty"`
	ItemsAndTitlesAlign  ItemsAndTitlesAlign  `yaml:"items_and_titles_align,omitempty" json:"itemsAndTitlesAlign,omitempty"`
	VerticalSpacing      ItemSpacing          `yaml:"vertical_spacing,omitempty" json:"verticalSpacing,omitempty"`
	HorizontalSpacing    ItemSpacing          `yaml:"horizontal_spacing,omitempty" json:"horizontalSpacing,omitempty"`
	ScaleVariant         FormScaleVariant     `yaml:"scale_variant,omitempty" json:"scaleVariant,omitempty"`
	CollapseByImportance CollapseByImportance `yaml:"collapse_by_importance,omitempty" json:"collapseByImportance,omitempty"`
}

// oneOf refuses a value that is none of the given ones; the empty value is
// the default and always stands.
func oneOf[T ~string](name string, value T, allowed ...T) []string {
	if value == "" || slices.Contains(allowed, value) {
		return nil
	}
	names := make([]string, len(allowed))
	for index, item := range allowed {
		names[index] = string(item)
	}
	return []string{name + " must be " + strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]}
}

func validateFormLayout(layout FormLayout) []string {
	var issues []string
	issues = append(issues, oneOf("vertical_scroll", layout.VerticalScroll, FormScrollAuto, FormScrollUse, FormScrollUseWithoutStretch, FormScrollUseIfNecessary)...)
	if layout.Width < 0 {
		issues = append(issues, "width must not be negative")
	}
	if layout.Height < 0 {
		issues = append(issues, "height must not be negative")
	}
	issues = append(issues, oneOf("children_group", layout.ChildrenGroup, ChildrenVertical, ChildrenHorizontal, ChildrenAlwaysHorizontal, ChildrenHorizontalIfPossible)...)
	issues = append(issues, oneOf("children_width", layout.ChildrenWidth, ChildrenWidthAuto, ChildrenWidthEqual, ChildrenWidthLeftNarrowest, ChildrenWidthLeftNarrow, ChildrenWidthLeftWide, ChildrenWidthLeftWidest)...)
	issues = append(issues, oneOf("horizontal_align", layout.HorizontalAlign, ItemHorizontalAuto, ItemHorizontalLeft, ItemHorizontalCenter, ItemHorizontalRight)...)
	issues = append(issues, oneOf("vertical_align", layout.VerticalAlign, ItemVerticalAuto, ItemVerticalTop, ItemVerticalCenter, ItemVerticalBottom)...)
	issues = append(issues, oneOf("items_and_titles_align", layout.ItemsAndTitlesAlign, ItemsAndTitlesAuto, ItemsAndTitlesNone, ItemsLeftTitlesLeft, ItemsLeftTitlesRight, ItemsRightTitlesLeft, ItemsRightTitlesRight, ItemsAndTitlesTitlesLeftDataAuto)...)
	spacings := []ItemSpacing{ItemSpacingAuto, ItemSpacingNone, ItemSpacingHalf, ItemSpacingSingle, ItemSpacingOneAndHalf, ItemSpacingDouble}
	issues = append(issues, oneOf("vertical_spacing", layout.VerticalSpacing, spacings...)...)
	issues = append(issues, oneOf("horizontal_spacing", layout.HorizontalSpacing, spacings...)...)
	issues = append(issues, oneOf("scale_variant", layout.ScaleVariant, FormScaleAuto, FormScaleNormal, FormScaleCompact)...)
	issues = append(issues, oneOf("collapse_by_importance", layout.CollapseByImportance, CollapseByImportanceAuto, CollapseByImportanceUse, CollapseByImportanceDontUse)...)
	return issues
}
