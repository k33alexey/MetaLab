package metadata

// How a value of a kind is entered and picked where one is asked for. Six
// settings, and again the set differs per kind: the syntax assistant gives the
// whole of it to the five reference kinds - a catalog, a chart of characteristic
// types, a chart of accounts, a chart of calculation types, an exchange plan -
// and a subset to the three numbered ones.
//
//	                    EditType  DefaultPres  QuickChoice  ChoiceMode  Input
//	five reference kinds     +          +            +           +        +
//	Задача                  +          +            -           -        +
//	БизнесПроцесс           +          -            -           -        +
//	Документ                -          -            -           -        +
//
// Input is the group below that every referenced kind has, and it is six settings
// rather than the two it started as: the four about typing joined it, and they
// have the same eight carriers - see input_by_string.go.

// ReferencePresentation is which of an object's two names stands for it where
// one line is all there is room for.
type ReferencePresentation string

const (
	PresentAsCode        ReferencePresentation = "as-code"
	PresentAsDescription ReferencePresentation = "as-description"
)

func validReferencePresentation(mode ReferencePresentation) bool {
	switch mode {
	case "", PresentAsCode, PresentAsDescription:
		return true
	default:
		return false
	}
}

// TaskPresentation is the same setting for a task, and its values are not the
// same: a task has no code. It is numbered, so the choice is between the number
// and the description, and the prototype gives the task an enumeration of its
// own for exactly that reason.
//
// Beware the help here, because it reads the other way at first. The property
// page for Задача.ОсновноеПредставление carries the same boilerplate sentence as
// every other kind's - "например, ВВидеКода, ВВидеНаименования" - and that
// sentence is wrong for a task. The type it names, ОсновноеПредставлениеЗадачи,
// has exactly two values on its own page: ВВидеНаименования and ВВидеНомера.
// The type page enumerates, the property page gives a generic example, and the
// enumeration wins.
//
// The structure agrees. A task's standard fields are Ссылка, Номер, Дата,
// Наименование, Выполнена, ТочкаМаршрута, БизнесПроцесс and ПометкаУдаления -
// see standard_attributes.go, checked against the demonstration configuration.
// There is no Код for "as code" to point at.
type TaskPresentation string

const (
	PresentTaskAsNumber      TaskPresentation = "as-number"
	PresentTaskAsDescription TaskPresentation = "as-description"
)

func validTaskPresentation(mode TaskPresentation) bool {
	switch mode {
	case "", PresentTaskAsNumber, PresentTaskAsDescription:
		return true
	default:
		return false
	}
}

// ObjectInput is what every kind that can be referenced carries: how the object
// is found by what the user typed, whether a new one may be made out of it, and
// whether what this user picked before is offered again.
//
// The six settings are the whole of the "Поле ввода" tab that the syntax
// assistant gives to these eight kinds and to no others. The four that are about
// typing live in input_by_string.go.
type ObjectInput struct {
	CreateOnInput        UsageMode     `yaml:"create_on_input,omitempty" json:"createOnInput,omitempty"`
	ChoiceHistoryOnInput ChoiceHistory `yaml:"choice_history_on_input,omitempty" json:"choiceHistoryOnInput,omitempty"`
	// InputByString is the fields the typed text is looked for in, in the order
	// they are searched. The order is data, not presentation: the found objects
	// are offered in the order of the fields they were found by, and the
	// demonstration configuration disagrees with itself about it on purpose -
	// thirty-nine objects search Наименование before Код and three the other way
	// round. An empty list means the object is not entered by string at all, and
	// two catalogs of the demonstration configuration say exactly that.
	InputByString    []ObjectField    `yaml:"input_by_string,omitempty" json:"inputByString,omitempty"`
	SearchStringMode SearchStringMode `yaml:"search_string_mode,omitempty" json:"searchStringMode,omitempty"`
	// FullTextSearchOnInput searches the full-text index instead of comparing
	// the field, and ChoiceDataGetMode says whether the user waits for the
	// search. Both are constant across the demonstration configuration, which
	// proves nothing about the platform: the export shows what it uses, not what
	// there is.
	FullTextSearchOnInput FullTextSearchOnInput `yaml:"full_text_search_on_input,omitempty" json:"fullTextSearchOnInput,omitempty"`
	ChoiceDataGetMode     ChoiceDataGetMode     `yaml:"choice_data_get_mode,omitempty" json:"choiceDataGetMode,omitempty"`
}

// ObjectChoice is the whole of it, for the five reference kinds.
type ObjectChoice struct {
	// EditType is whether a row is edited in a form of its own, in the list, or
	// either way. The enumeration is shared with the information register, which
	// has this and nothing else of the group - see register_properties.go.
	EditType EditType `yaml:"edit_type,omitempty" json:"editType,omitempty"`
	// DefaultPresentation is which of the code and the description stands for the
	// object in one line.
	DefaultPresentation ReferencePresentation `yaml:"default_presentation,omitempty" json:"defaultPresentation,omitempty"`
	// QuickChoice offers the values in a drop-down list instead of opening a
	// form. The prototype spells one idea three ways, and each spelling belongs
	// to a different thing:
	//
	//	the object itself        Булево                     - this field
	//	an attribute of it       ИспользованиеБыстрогоВыбора: Авто/Использовать/
	//	                         НеИспользовать             - UsageMode, see
	//	                                                      attribute_presentation.go
	//	a field on a form        Булево, Неопределено       - belongs to the form,
	//	                                                      not here
	//
	// So a plain flag here is not a simplification: an object either offers a
	// quick choice or does not. Deferring is what an attribute may do, because
	// an attribute can leave the answer to the object it points at.
	QuickChoice bool `yaml:"quick_choice,omitempty" json:"quickChoice,omitempty"`
	// ChoiceMode is how the choice happens at all - from a form, quickly, or
	// either way.
	ChoiceMode  ChoiceMode `yaml:"choice_mode,omitempty" json:"choiceMode,omitempty"`
	ObjectInput `yaml:",inline" json:",inline"`
}

// validChoiceMode and validChoiceHistory check the two enumerations the
// prototype shares between an object and an enumeration. They live here rather
// than beside the types because this is where the whole group of settings is
// answered for, and the enumeration calls them too - two copies of the same list
// of values drift apart the moment one of them gains a value.
func validChoiceMode(mode ChoiceMode) bool {
	switch mode {
	case "", ChoiceBothWays, ChoiceFromForm, ChoiceQuickOnly:
		return true
	default:
		return false
	}
}

func validChoiceHistory(mode ChoiceHistory) bool {
	switch mode {
	case "", ChoiceHistoryAuto, ChoiceHistoryDontUse:
		return true
	default:
		return false
	}
}

// validateObjectInput checks what is answerable about the group without the
// object: the values of the enumerations. The list of searched fields is only
// answerable beside the kind and the attributes that carry it, so it is checked
// by validateInputByString, which both shape validators call.
func validateObjectInput(input ObjectInput) []string {
	var issues []string
	if !validUsageMode(input.CreateOnInput) {
		issues = append(issues, "create_on_input must be auto, use or dont-use")
	}
	if !validChoiceHistory(input.ChoiceHistoryOnInput) {
		issues = append(issues, "choice_history_on_input must be auto or dont-use")
	}
	if !validSearchStringMode(input.SearchStringMode) {
		issues = append(issues, "search_string_mode must be begin or any-part")
	}
	if !validFullTextSearchOnInput(input.FullTextSearchOnInput) {
		issues = append(issues, "full_text_search_on_input must be use or dont-use")
	}
	if !validChoiceDataGetMode(input.ChoiceDataGetMode) {
		issues = append(issues, "choice_data_get_mode must be directly or background")
	}
	return issues
}

func validateObjectChoice(choice ObjectChoice) []string {
	issues := validateObjectInput(choice.ObjectInput)
	if !validEditType(choice.EditType) {
		issues = append(issues, "edit_type must be in-dialog, in-list or both-ways")
	}
	if !validReferencePresentation(choice.DefaultPresentation) {
		issues = append(issues, "default_presentation must be as-code or as-description")
	}
	if !validChoiceMode(choice.ChoiceMode) {
		issues = append(issues, "choice_mode must be both-ways, from-form or quick-choice")
	}
	// Choosing only from a form leaves the quick-choice flag with nothing to
	// do, and the configurator writes it there all the same, as it writes the
	// settings of posting a document that does not post: the flag is carried,
	// offers nothing under «from form», and is a note.
	return issues
}
