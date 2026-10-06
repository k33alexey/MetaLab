package metadata

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
	switch window.WindowOpeningMode {
	case "", FormWindowIndependent, FormWindowLockOwnerWindow, FormWindowLockWholeInterface:
	default:
		issues = append(issues, "window_opening_mode must be independent, lock-owner-window or lock-whole-interface")
	}
	switch window.CommandBarLocation {
	case "", FormCommandBarAuto, FormCommandBarTop, FormCommandBarBottom, FormCommandBarNone:
	default:
		issues = append(issues, "command_bar_location must be auto, top, bottom or none")
	}
	switch window.EnterKeyBehavior {
	case "", FormEnterControlNavigation, FormEnterDefaultButton:
	default:
		issues = append(issues, "enter_key_behavior must be control-navigation or default-button")
	}
	switch window.Conversations {
	case "", FormConversationsAuto, FormConversationsShow, FormConversationsDontShow:
	default:
		issues = append(issues, "conversations must be auto, show or dont-show")
	}
	return issues
}
