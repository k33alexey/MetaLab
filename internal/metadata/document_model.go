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
	Object *uuid.UUID `yaml:"object,omitempty"`
	List   *uuid.UUID `yaml:"list,omitempty"`
	Choice *uuid.UUID `yaml:"choice,omitempty"`
}

// DocumentDefinition describes one ML document and its persistent record shape.
type DocumentDefinition struct {
	Format        int            `yaml:"format"`
	ID            uuid.UUID      `yaml:"id"`
	Name          string         `yaml:"name"`
	Title         LocalizedText  `yaml:"title"`
	Number        DocumentNumber `yaml:"number"`
	Posting       bool           `yaml:"posting,omitempty"`
	Attributes    []Attribute    `yaml:"attributes,omitempty"`
	TableParts    []TablePart    `yaml:"table_parts,omitempty"`
	ObjectModule  *uuid.UUID     `yaml:"object_module,omitempty"`
	ManagerModule *uuid.UUID     `yaml:"manager_module,omitempty"`
	Forms         ObjectForms    `yaml:"forms,omitempty"`
}

func DecodeDocument(source string, reader io.Reader, manifest project.Project) (DocumentDefinition, error) {
	var value DocumentDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return DocumentDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	switch value.Number.Type {
	case StringType:
		if value.Number.Length < 1 || value.Number.Length > 128 {
			issues = append(issues, "number.length must be 1..128 for string numbers")
		}
	case NumberType:
		if value.Number.Length < 1 || value.Number.Length > 38 {
			issues = append(issues, "number.length must be 1..38 for numeric numbers")
		}
	default:
		issues = append(issues, "number.type must be string or number")
	}
	switch value.Number.Periodicity {
	case NumberPeriodNone, NumberPeriodYear, NumberPeriodQuarter, NumberPeriodMonth, NumberPeriodDay:
	default:
		issues = append(issues, "number.periodicity must be none, year, quarter, month or day")
	}
	issues = append(issues, validateAttributes("attributes", value.Attributes, manifest, reservedDocumentObjectName)...)
	attributeNames := make(map[string]bool, len(value.Attributes))
	for _, attribute := range value.Attributes {
		attributeNames[strings.ToLower(attribute.Name)] = true
	}
	if len(value.TableParts) > 128 {
		issues = append(issues, "table_parts must not contain more than 128 items")
	}
	partNames, partIDs := map[string]bool{}, map[uuid.UUID]bool{}
	for index, part := range value.TableParts {
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
		if reservedDocumentObjectName(folded) {
			issues = append(issues, prefix+".name is reserved")
		}
		if attributeNames[folded] {
			issues = append(issues, prefix+".name conflicts with an attribute")
		}
		partNames[folded] = true
		issues = append(issues, validateTitle(prefix+".title", part.Title, manifest)...)
		issues = append(issues, validateAttributes(prefix+".attributes", part.Attributes, manifest, nil)...)
	}
	for name, module := range map[string]*uuid.UUID{"object_module": value.ObjectModule, "manager_module": value.ManagerModule} {
		if module != nil && module.IsZero() {
			issues = append(issues, name+" must be a non-zero UUID")
		}
	}
	if value.ObjectModule != nil && value.ManagerModule != nil && *value.ObjectModule == *value.ManagerModule {
		issues = append(issues, "object_module and manager_module must be different")
	}
	issues = append(issues, validateObjectForms(value.Forms)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return DocumentDefinition{}, err
	}
	return value, nil
}

func reservedDocumentObjectName(name string) bool {
	switch strings.ToLower(name) {
	case "ссылка", "ref", "номер", "number", "дата", "date", "проведен", "проведён", "posted", "версия", "version":
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
