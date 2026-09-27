package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// maxDocumentMovements is where a list of registers stops being a list.
const maxDocumentMovements = 256

type NumberPeriodicity string

const (
	NumberPeriodNone    NumberPeriodicity = "none"
	NumberPeriodYear    NumberPeriodicity = "year"
	NumberPeriodQuarter NumberPeriodicity = "quarter"
	NumberPeriodMonth   NumberPeriodicity = "month"
	NumberPeriodDay     NumberPeriodicity = "day"
)

type DocumentNumber struct {
	Type   TypeKind `yaml:"type"`
	Length int      `yaml:"length"`
	Auto   bool     `yaml:"auto"`
	Unique bool     `yaml:"unique"`
	// FixedLength is the allowed length of a string number, the same setting a
	// code carries - see CatalogCode.FixedLength. It lives here rather than on
	// each kind because a document, a business process, a task and a numerator
	// all describe their number through this one shape, and the prototype gives
	// the property to exactly those four.
	FixedLength bool              `yaml:"fixed_length,omitempty"`
	Periodicity NumberPeriodicity `yaml:"periodicity"`
}

// DocumentDefinition describes one ML document and its persistent record shape.
type DocumentDefinition struct {
	Format int           `yaml:"format"`
	ID     uuid.UUID     `yaml:"id"`
	Name   string        `yaml:"name"`
	Title  LocalizedText `yaml:"title"`
	// Presentations is how this object is named to the person using it -
	// see object_presentation.go.
	Presentations `yaml:",inline" json:",inline"` // A document is picked but not edited in a list, and stands for nothing in
	// one line: it keeps only the pair every referenced kind has.
	ObjectInput `yaml:",inline" json:",inline"`
	// BasedOn are the objects one of these may be made out of, the list the
	// command to make it offers - see based_on.go.
	BasedOn []uuid.UUID `yaml:"based_on,omitempty" json:"basedOn,omitempty"`

	Number DocumentNumber `yaml:"number"`
	// Numerator names a numbering shared with other kinds of document. When it
	// is named the document declares no number of its own: two sources for one
	// number is one too many, and the shared one wins by definition.
	Numerator *uuid.UUID `yaml:"numerator,omitempty"`
	// Posting is the six settings that describe being posted, not one flag -
	// see document_posting.go.
	Posting DocumentPosting `yaml:"posting,omitempty"`
	// Movements are the registers this document writes records into. The link
	// is described from the document's side, and only from there: in the
	// prototype not one of the four kinds of register has a property listing
	// its documents, while a document names its registers outright. Held the
	// other way round, a register had to be reopened every time a document
	// started writing into it, and the document - the thing that does the
	// writing - said nothing about it.
	Movements          []uuid.UUID            `yaml:"movements,omitempty"`
	Attributes         []Attribute            `yaml:"attributes,omitempty"`
	TableParts         []TablePart            `yaml:"table_parts,omitempty"`
	Characteristics    []ObjectCharacteristic `yaml:"characteristics,omitempty"`
	StandardAttributes []StandardAttribute    `yaml:"standard_attributes,omitempty"`
	Forms              ObjectForms            `yaml:"forms,omitempty"`
	Commands           []ObjectCommand        `yaml:"commands,omitempty"`
	Templates          []ObjectTemplate       `yaml:"templates,omitempty"`
	List               ListSettings           `yaml:"list,omitempty"`
}

func DecodeDocument(source string, reader io.Reader, configuration project.Project) (DocumentDefinition, error) {
	var value DocumentDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return DocumentDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	shape := numberedObjectShape{
		number:             value.Number,
		attributes:         value.Attributes,
		tableParts:         value.TableParts,
		forms:              value.Forms,
		list:               value.List,
		reservedName:       reservedDocumentObjectName,
		kind:               DocumentKind,
		standardAttributes: value.StandardAttributes,
		presentation:       value.Presentations,
		input:              value.ObjectInput,
		basedOn:            value.BasedOn,
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
	issues = append(issues, validateDocumentPosting(value.Posting)...)
	issues = append(issues, validateNumberedObjectShape(shape, configuration)...)
	if len(value.Movements) > maxDocumentMovements {
		issues = append(issues, fmt.Sprintf("movements must not contain more than %d registers", maxDocumentMovements))
	}
	issues = append(issues, validateUniqueIDs("movements", value.Movements)...)
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
	// kind says which kind of object this is, so that the standard fields the
	// platform gives it can be looked up; standardAttributes is what the
	// developer wrote about them. None of the numbered kinds has a standard
	// table part.
	kind               Kind
	standardAttributes []StandardAttribute
	// presentation is how the object is named to a person; all three numbered
	// kinds carry the whole set.
	presentation Presentations
	// input is the pair of settings every referenced kind has. What each of the
	// three numbered kinds adds to it is checked where that kind is decoded.
	input ObjectInput
	// basedOn is what this object may be made out of - see based_on.go. All
	// three numbered kinds carry it, as do the five reference ones.
	basedOn []uuid.UUID
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
	// A number padded to a width is a string padded to a width; a numeric
	// number has digits, and nothing to pad with spaces.
	if number.FixedLength && number.Type != StringType {
		issues = append(issues, "number.fixed_length is allowed for string numbers only")
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
	// A numbered object stores its rows, and none of the numbered kinds lets a
	// part say whom it belongs to: that is a catalog's and a chart of
	// characteristic types' alone.
	issues = append(issues, validateTableParts(shape.tableParts, attributeNames, configuration, reserved, tablePartRules{stored: true})...)
	issues = append(issues, validateStandardAttributes("standard_attributes", shape.standardAttributes, standardFieldsOfKind(shape.kind), configuration)...)
	issues = append(issues, validatePresentations(shape.presentation, configuration)...)
	issues = append(issues, validateObjectInput(shape.input)...)
	issues = append(issues, validateInputByString(shape.input.InputByString, shape.kind, shape.attributes)...)
	issues = append(issues, validateBasedOn(shape.basedOn)...)
	links := append(standardAttributeChoices("standard_attributes", shape.standardAttributes), tablePartStandardChoices(shape.tableParts)...)
	issues = append(issues, validateFieldLinks([]fieldGroup{{"attributes", shape.attributes}}, shape.tableParts, links...)...)
	issues = append(issues, validateAttributeUse([]fieldGroup{{"attributes", shape.attributes}}, shape.tableParts, false, false)...)
	issues = append(issues, validateFormSlots(shape.forms.slots())...)
	return append(issues, validateListSettings(shape.list, shape.attributes, map[string]TypeKind{
		"number": shape.number.Type,
	})...)
}

// tablePartRules are the things about a table part that its owner decides rather
// than the part itself.
type tablePartRules struct {
	// stored says the rows live in the database. A report's table part exists
	// only while the report runs, so the width the line number is stored in
	// means nothing there - and the help lists the kinds that have the property
	// without naming reports or data processors.
	stored bool
	// use says this kind lets a part say whom it belongs to - items, folders or
	// both. Only a catalog and a chart of characteristic types may, exactly as
	// for an attribute, and the demonstration configuration writes the setting
	// on 111 parts, all of them theirs.
	use bool
	// folders says the object has folders, so a part that reaches them can.
	folders bool
}

// maxLineNumberLength and minLineNumberLength bound the decimal width the line
// number of a row is stored in. The help gives the range outright: 5 to 9.
const (
	minLineNumberLength = 5
	maxLineNumberLength = 9
)

// validateTableParts checks the table parts of any object that has them. The
// names of parts and of attributes share one space: a part named like an
// attribute would be two things answering to one name.
func validateTableParts(parts []TablePart, attributeNames map[string]bool, configuration project.Project, reserved func(string) bool, rules tablePartRules) []string {
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
		issues = append(issues, validateStandardAttributes(prefix+".standard_attributes", part.StandardAttributes, tablePartStandardFields, configuration)...)
		issues = append(issues, validateTablePartProperties(prefix, part, rules, configuration)...)
	}
	return issues
}

// validateTablePartProperties checks the five settings a part carries beyond its
// name and its fields.
func validateTablePartProperties(prefix string, part TablePart, rules tablePartRules, configuration project.Project) []string {
	var issues []string
	if len(part.ToolTip) > 0 {
		issues = append(issues, validateTitle(prefix+".tooltip", part.ToolTip, configuration)...)
	}
	if !validFillCheck(part.FillChecking) {
		issues = append(issues, prefix+".fill_checking must be dont-check or show-error")
	}
	switch {
	case part.LineNumberLength == 0:
	case !rules.stored:
		issues = append(issues, prefix+".line_number_length is the width a line number is stored in, and the rows of this kind of table part are never stored")
	case part.LineNumberLength < minLineNumberLength || part.LineNumberLength > maxLineNumberLength:
		issues = append(issues, fmt.Sprintf("%s.line_number_length must be %d..%d", prefix, minLineNumberLength, maxLineNumberLength))
	}
	switch {
	case part.Use == "":
	case !validAttributeUse(part.Use):
		issues = append(issues, prefix+".use must be for-item, for-folder or for-folder-and-item")
	case !rules.use:
		issues = append(issues, prefix+".use belongs to a table part of a catalog or a chart of characteristic types, and this is neither")
	case part.Use.forFolders() && !rules.folders:
		issues = append(issues, prefix+".use reaches folders, and this object has none")
	}
	return issues
}

// cloneTableParts copies the parts and everything inside them.
func cloneTableParts(parts []TablePart) []TablePart {
	parts = slices.Clone(parts)
	for index := range parts {
		parts[index].Title = cloneTitle(parts[index].Title)
		parts[index].ToolTip = cloneTitle(parts[index].ToolTip)
		parts[index].Attributes = cloneAttributes(parts[index].Attributes)
		parts[index].StandardAttributes = cloneStandardAttributes(parts[index].StandardAttributes)
	}
	return parts
}

func reservedDocumentObjectName(name string) bool {
	return reservedStandardName(DocumentKind, name) || reservedRowVersionName(name)
}

func cloneDocumentDefinition(value DocumentDefinition) DocumentDefinition {
	value.Title = cloneTitle(value.Title)
	value.Attributes = cloneAttributes(value.Attributes)
	value.TableParts = cloneTableParts(value.TableParts)
	value.Forms = cloneFormSet(value.Forms)
	value.List.SearchFields = slices.Clone(value.List.SearchFields)
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
	value.Characteristics = cloneObjectCharacteristics(value.Characteristics)
	value.Movements = slices.Clone(value.Movements)
	value.StandardAttributes = cloneStandardAttributes(value.StandardAttributes)
	value.Presentations = clonePresentations(value.Presentations)
	value.ObjectInput = cloneObjectInput(value.ObjectInput)
	value.BasedOn = slices.Clone(value.BasedOn)
	return value
}

// cloneFormSet is what a copy of a set of form roles costs now that a role is
// a name: nothing. It is kept so the callers that hand out copies still read as
// copying every part of what they hand out, and it takes any set because every
// kind has its own.
func cloneFormSet[Forms any](forms Forms) Forms { return forms }
