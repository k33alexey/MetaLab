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

// ObjectForms binds optional managed forms to their stable source UUIDs.
type ObjectForms struct {
	Object *uuid.UUID `yaml:"object,omitempty" json:"object,omitempty"`
	List   *uuid.UUID `yaml:"list,omitempty" json:"list,omitempty"`
	Choice *uuid.UUID `yaml:"choice,omitempty" json:"choice,omitempty"`
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
	Numerator     *uuid.UUID   `yaml:"numerator,omitempty"`
	Posting       bool         `yaml:"posting,omitempty"`
	Attributes    []Attribute  `yaml:"attributes,omitempty"`
	TableParts    []TablePart  `yaml:"table_parts,omitempty"`
	ObjectModule  *uuid.UUID   `yaml:"object_module,omitempty"`
	ManagerModule *uuid.UUID   `yaml:"manager_module,omitempty"`
	Forms         ObjectForms  `yaml:"forms,omitempty"`
	List          ListSettings `yaml:"list,omitempty"`
}

func DecodeDocument(source string, reader io.Reader, manifest project.Project) (DocumentDefinition, error) {
	var value DocumentDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return DocumentDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	shape := numberedObjectShape{
		number:        value.Number,
		attributes:    value.Attributes,
		tableParts:    value.TableParts,
		objectModule:  value.ObjectModule,
		managerModule: value.ManagerModule,
		forms:         value.Forms,
		list:          value.List,
		reservedName:  reservedDocumentObjectName,
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
	issues = append(issues, validateNumberedObjectShape(shape, manifest)...)
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
	number        DocumentNumber
	attributes    []Attribute
	tableParts    []TablePart
	objectModule  *uuid.UUID
	managerModule *uuid.UUID
	forms         ObjectForms
	list          ListSettings
	reservedName  func(string) bool
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

func validateNumberedObjectShape(shape numberedObjectShape, manifest project.Project) []string {
	var issues []string
	if !shape.numberFromElsewhere {
		issues = validateNumberShape(shape.number)
	}
	reserved := shape.reservedName
	if reserved == nil {
		reserved = reservedDocumentObjectName
	}
	issues = append(issues, validateAttributes("attributes", shape.attributes, manifest, reserved)...)
	attributeNames := make(map[string]bool, len(shape.attributes))
	for _, attribute := range shape.attributes {
		attributeNames[strings.ToLower(attribute.Name)] = true
	}
	issues = append(issues, validateTableParts(shape.tableParts, attributeNames, manifest, reserved)...)
	for name, module := range map[string]*uuid.UUID{"object_module": shape.objectModule, "manager_module": shape.managerModule} {
		if module != nil && module.IsZero() {
			issues = append(issues, name+" must be a non-zero UUID")
		}
	}
	if shape.objectModule != nil && shape.managerModule != nil && *shape.objectModule == *shape.managerModule {
		issues = append(issues, "object_module and manager_module must be different")
	}
	issues = append(issues, validateObjectForms(shape.forms)...)
	return append(issues, validateListSettings(shape.list, shape.attributes, map[string]TypeKind{
		"number": shape.number.Type,
	})...)
}

// validateTableParts checks the table parts of any object that has them. The
// names of parts and of attributes share one space: a part named like an
// attribute would be two things answering to one name.
func validateTableParts(parts []TablePart, attributeNames map[string]bool, manifest project.Project, reserved func(string) bool) []string {
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
		issues = append(issues, validateTitle(prefix+".title", part.Title, manifest)...)
		issues = append(issues, validateAttributes(prefix+".attributes", part.Attributes, manifest, nil)...)
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

func validateObjectForms(forms ObjectForms) []string {
	var issues []string
	seen := map[uuid.UUID]string{}
	for name, id := range map[string]*uuid.UUID{"forms.object": forms.Object, "forms.list": forms.List, "forms.choice": forms.Choice} {
		if id == nil {
			continue
		}
		if id.IsZero() {
			issues = append(issues, name+" must be a non-zero UUID")
			continue
		}
		if previous, exists := seen[*id]; exists {
			issues = append(issues, name+" duplicates "+previous)
		}
		seen[*id] = name
	}
	return issues
}

func cloneDocumentDefinition(value DocumentDefinition) DocumentDefinition {
	value.Title = cloneTitle(value.Title)
	value.Attributes = cloneAttributes(value.Attributes)
	value.TableParts = slices.Clone(value.TableParts)
	for index := range value.TableParts {
		value.TableParts[index].Title = cloneTitle(value.TableParts[index].Title)
		value.TableParts[index].Attributes = cloneAttributes(value.TableParts[index].Attributes)
	}
	if value.ObjectModule != nil {
		id := *value.ObjectModule
		value.ObjectModule = &id
	}
	if value.ManagerModule != nil {
		id := *value.ManagerModule
		value.ManagerModule = &id
	}
	value.Forms = cloneObjectForms(value.Forms)
	value.List.SearchFields = slices.Clone(value.List.SearchFields)
	return value
}

func cloneObjectForms(forms ObjectForms) ObjectForms {
	if forms.Object != nil {
		id := *forms.Object
		forms.Object = &id
	}
	if forms.List != nil {
		id := *forms.List
		forms.List = &id
	}
	if forms.Choice != nil {
		id := *forms.Choice
		forms.Choice = &id
	}
	return forms
}
