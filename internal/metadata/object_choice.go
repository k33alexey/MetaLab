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
// Input is the pair below that every referenced kind has.

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

// ObjectInput is what every kind that can be referenced carries: whether a new
// one may be made out of what the user typed, and whether what this user picked
// before is offered again.
type ObjectInput struct {
	CreateOnInput        UsageMode     `yaml:"create_on_input,omitempty" json:"createOnInput,omitempty"`
	ChoiceHistoryOnInput ChoiceHistory `yaml:"choice_history_on_input,omitempty" json:"choiceHistoryOnInput,omitempty"`
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
	case "", ChoiceHistoryAuto, ChoiceHistoryUse, ChoiceHistoryDontUse:
		return true
	default:
		return false
	}
}

func validateObjectInput(input ObjectInput) []string {
	var issues []string
	if !validUsageMode(input.CreateOnInput) {
		issues = append(issues, "create_on_input must be auto, use or dont-use")
	}
	if !validChoiceHistory(input.ChoiceHistoryOnInput) {
		issues = append(issues, "choice_history_on_input must be auto, use or dont-use")
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
	// Choosing only from a form and offering a quick choice are two answers to
	// one question, and under the first the second is never asked. The same
	// contradiction is already refused on an enumeration.
	if choice.ChoiceMode == ChoiceFromForm && choice.QuickChoice {
		issues = append(issues, "quick_choice contradicts choice_mode from-form")
	}
	return issues
}
