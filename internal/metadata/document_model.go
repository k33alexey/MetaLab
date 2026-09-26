package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

type NumberPeriodicity string

const (
	NumberPeriodNone    NumberPeriodicity = "none"
	NumberPeriodYear    NumberPeriodicity = "year"
	NumberPeriodQuarter NumberPeriodicity = "quarter"
	NumberPeriodMonth   NumberPeriodicity = "month"
	NumberPeriodDay     NumberPeriodicity = "day"
)

type DocumentNumber struct {
	Type        TypeKind          `yaml:"type"`
	Length      int               `yaml:"length"`
	Auto        bool              `yaml:"auto"`
	Unique      bool              `yaml:"unique"`
	Periodicity NumberPeriodicity `yaml:"periodicity"`
}

// DocumentDefinition describes one ML document and its persistent record shape.
type DocumentDefinition struct {
	Format int            `yaml:"format"`
	ID     uuid.UUID      `yaml:"id"`
	Name   string         `yaml:"name"`
	Title  LocalizedText  `yaml:"title"`
	Number DocumentNumber `yaml:"number"`
	// Numerator names a numbering shared with other kinds of document. When it
	// is named the document declares no number of its own: two sources for one
	// number is one too many, and the shared one wins by definition.
	Numerator       *uuid.UUID             `yaml:"numerator,omitempty"`
	Posting         bool                   `yaml:"posting,omitempty"`
	Attributes      []Attribute            `yaml:"attributes,omitempty"`
	TableParts      []TablePart            `yaml:"table_parts,omitempty"`
	Characteristics []ObjectCharacteristic `yaml:"characteristics,omitempty"`
	Forms           ObjectForms            `yaml:"forms,omitempty"`
	Commands        []ObjectCommand        `yaml:"commands,omitempty"`
	Templates       []ObjectTemplate       `yaml:"templates,omitempty"`
	List            ListSettings           `yaml:"list,omitempty"`
}

func DecodeDocument(source string, reader io.Reader, configuration project.Project) (DocumentDefinition, error) {
	var value DocumentDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return DocumentDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	shape := numberedObjectShape{
		number:       value.Number,
		attributes:   value.Attributes,
		tableParts:   value.TableParts,
		forms:        value.Forms,
		list:         value.List,
		reservedName: reservedDocumentObjectName,
	}
	if value.Numerator != nil {
		// The number comes from the numerator, and it is filled in once the
		// whole project is read. Checking the empty block here would report a
		// missing type for a number this document does not declare.
		shape.numberFromElsewhere = true
		if value.Numerator.IsZero() {
			issues = append(issues, "numerator must be a non-zero UUID")
		}
		if value.Number != (DocumentNumber{}) {
			issues = append(issues, "number is set by the numerator this document shares, so it must not be declared here as well")
		}
	}
	issues = append(issues, validateNumberedObjectShape(shape, configuration)...)
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, configuration)...)
	issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
	issues = append(issues, validateObjectCharacteristics(value.Characteristics)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return DocumentDefinition{}, err
	}
	return value, nil
}

// numberedObjectShape is what a numbered object repeats: a number with its
// settings, attributes, table parts, its own modules, forms and list settings.
// Documents, business processes and tasks all repeat it, and each adds its own
// on top - so the repeated part is checked in one place rather than copied per
// kind, where the copies drift.
type numberedObjectShape struct {
	number       DocumentNumber
	attributes   []Attribute
	tableParts   []TablePart
	forms        ObjectForms
	list         ListSettings
	reservedName func(string) bool
	// numberFromElsewhere says the number is not declared here and will be
	// filled in from the object that owns it.
	numberFromElsewhere bool
}

// validateNumberShape checks a number on its own, apart from the object that
// carries it: a numerator is nothing but one of these.
func validateNumberShape(number DocumentNumber) []string {
	var issues []string
	switch number.Type {
	case StringType:
		if number.Length < 1 || number.Length > 128 {
			issues = append(issues, "number.length must be 1..128 for string numbers")
		}
	case NumberType:
		if number.Length < 1 || number.Length > 38 {
			issues = append(issues, "number.length must be 1..38 for numeric numbers")
		}
	default:
		issues = append(issues, "number.type must be string or number")
	}
	switch number.Periodicity {
	case NumberPeriodNone, NumberPeriodYear, NumberPeriodQuarter, NumberPeriodMonth, NumberPeriodDay:
	default:
		issues = append(issues, "number.periodicity must be none, year, quarter, month or day")
	}
	return issues
}

func validateNumberedObjectShape(shape numberedObjectShape, configuration project.Project) []string {
	var issues []string
	if !shape.numberFromElsewhere {
		issues = validateNumberShape(shape.number)
	}
	reserved := shape.reservedName
	if reserved == nil {
		reserved = reservedDocumentObjectName
	}
	issues = append(issues, validateAttributes("attributes", shape.attributes, configuration, reserved)...)
	attributeNames := make(map[string]bool, len(shape.attributes))
	for _, attribute := range shape.attributes {
		attributeNames[strings.ToLower(attribute.Name)] = true
	}
	issues = append(issues, validateTableParts(shape.tableParts, attributeNames, configuration, reserved)...)
	issues = append(issues, validateFormSlots(shape.forms.slots())...)
	return append(issues, validateListSettings(shape.list, shape.attributes, map[string]TypeKind{
		"number": shape.number.Type,
	})...)
}

// validateTableParts checks the table parts of any object that has them. The
// names of parts and of attributes share one space: a part named like an
// attribute would be two things answering to one name.
func validateTableParts(parts []TablePart, attributeNames map[string]bool, configuration project.Project, reserved func(string) bool) []string {
	var issues []string
	if len(parts) > 128 {
		issues = append(issues, "table_parts must not contain more than 128 items")
	}
	if reserved == nil {
		reserved = func(string) bool { return false }
	}
	partNames, partIDs := map[string]bool{}, map[uuid.UUID]bool{}
	for index, part := range parts {
		prefix := fmt.Sprintf("table_parts[%d]", index)
		if part.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if partIDs[part.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		partIDs[part.ID] = true
		if !validIdentifier(part.Name) {
			issues = append(issues, prefix+".name must be a valid identifier")
		}
		folded := strings.ToLower(part.Name)
		if partNames[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		if reserved(folded) {
			issues = append(issues, prefix+".name is reserved")
		}
		if attributeNames[folded] {
			issues = append(issues, prefix+".name conflicts with an attribute")
		}
		partNames[folded] = true
		issues = append(issues, validateTitle(prefix+".title", part.Title, configuration)...)
		issues = append(issues, validateAttributes(prefix+".attributes", part.Attributes, configuration, nil)...)
	}
	return issues
}

// cloneTableParts copies the parts and everything inside them.
func cloneTableParts(parts []TablePart) []TablePart {
	parts = slices.Clone(parts)
	for index := range parts {
		parts[index].Title = cloneTitle(parts[index].Title)
		parts[index].Attributes = cloneAttributes(parts[index].Attributes)
	}
	return parts
}

func reservedDocumentObjectName(name string) bool {
	switch strings.ToLower(name) {
	case "ссылка", "ref", "номер", "number", "дата", "date", "проведен", "проведён", "posted", "версия", "version",
		"пометкаудаления", "deletionmark":
		return true
	default:
		return false
	}
}

func cloneDocumentDefinition(value DocumentDefinition) DocumentDefinition {
	value.Title = cloneTitle(value.Title)
	value.Attributes = cloneAttributes(value.Attributes)
	value.TableParts = slices.Clone(value.TableParts)
	for index := range value.TableParts {
		value.TableParts[index].Title = cloneTitle(value.TableParts[index].Title)
		value.TableParts[index].Attributes = cloneAttributes(value.TableParts[index].Attributes)
	}
	value.Forms = cloneFormSet(value.Forms)
	value.List.SearchFields = slices.Clone(value.List.SearchFields)
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
	value.Characteristics = cloneObjectCharacteristics(value.Characteristics)
	return value
}

// cloneFormSet is what a copy of a set of form roles costs now that a role is
// a name: nothing. It is kept so the callers that hand out copies still read as
// copying every part of what they hand out, and it takes any set because every
// kind has its own.
func cloneFormSet[Forms any](forms Forms) Forms { return forms }
