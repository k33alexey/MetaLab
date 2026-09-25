package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// AutoRecord says who writes a change of a registered object into what the
// nodes still owe each other: the platform itself on every write, or the
// application code and nobody else.
//
// The choice is not a convenience setting. Automatic registration writes a row
// for every node on every write of the object, and a plan that turns it on
// where the application meant to decide for itself doubles the traffic of the
// exchange without changing a line of visible behaviour.
type AutoRecord string

const (
	AutoRecordAllow AutoRecord = "allow"
	AutoRecordDeny  AutoRecord = "deny"
	// The reference export carries six hundred and thirty entries in one plan,
	// so the limit is set where it stops being a plan and starts being a bug.
	maxExchangePlanContent = 4096
)

// ExchangePlanContentItem is one object whose changes the plan registers.
type ExchangePlanContentItem struct {
	Kind   Kind       `yaml:"kind" json:"kind"`
	Object uuid.UUID  `yaml:"object" json:"object"`
	Auto   AutoRecord `yaml:"auto_record,omitempty" json:"autoRecord,omitempty"`
}

// ExchangePlanDefinition describes what enters an exchange and who it is
// exchanged with. A node is a reference object like any other - it has a code,
// a description, attributes and table parts - and carries on top of them what
// the two sides still owe each other.
type ExchangePlanDefinition struct {
	Format            int           `yaml:"format" json:"format"`
	ID                uuid.UUID     `yaml:"id" json:"id"`
	Name              string        `yaml:"name" json:"name"`
	Title             LocalizedText `yaml:"title" json:"title"`
	Code              CatalogCode   `yaml:"code" json:"code"`
	DescriptionLength int           `yaml:"description_length" json:"descriptionLength"`
	// Content is what is registered, with the kind of registration beside each
	// entry. A plan with no content registers nothing and exchanges nothing.
	Content []ExchangePlanContentItem `yaml:"content,omitempty" json:"content,omitempty"`
	// IncludeExtensions says whether extensions of the configuration travel to
	// the nodes along with the data.
	IncludeExtensions bool `yaml:"include_extensions,omitempty" json:"includeExtensions,omitempty"`
	// DistributedInfoBase is carried, shown and never acted upon. The platform
	// does not replicate a configuration between nodes, and saying so out loud
	// in the model is the whole point: a flag raised in the source
	// configuration and dropped on the way in would be a silent loss, which is
	// the one thing forbidden here. The import report shows it as transferred
	// and not implemented.
	DistributedInfoBase bool             `yaml:"distributed_info_base,omitempty" json:"distributedInfoBase,omitempty"`
	Attributes          []Attribute      `yaml:"attributes,omitempty" json:"attributes,omitempty"`
	TableParts          []TablePart      `yaml:"table_parts,omitempty" json:"tableParts,omitempty"`
	Forms               ObjectForms      `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands            []ObjectCommand  `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates           []ObjectTemplate `yaml:"templates,omitempty" json:"templates,omitempty"`
	List                ListSettings     `yaml:"list,omitempty" json:"list,omitempty"`
}

// registrableKinds are the kinds of object whose changes an exchange plan can
// register. Enumerations, reports and data processors are not among them, and
// not by omission: an enumeration has no data to change, and a report has no
// data at all.
var registrableKinds = []Kind{
	ConstantKind, CatalogKind, DocumentKind, SequenceKind,
	ChartOfCharacteristicTypesKind, ChartOfAccountsKind, ChartOfCalculationTypesKind,
	BusinessProcessKind, TaskKind,
	InformationRegisterKind, AccumulationRegisterKind, AccountingRegisterKind, CalculationRegisterKind,
}

// pendingRegistrableKinds are registrable too, and this version does not model
// them yet. Naming them apart is what separates "this kind cannot be
// registered" from "this kind is not described yet" - two different answers,
// and a plan arriving from another system deserves the true one. Every kind
// that was here has since been described; the list is kept because the
// distinction it makes will be needed again.
var pendingRegistrableKinds []Kind

// DecodeExchangePlan reads and validates one exchange plan.
func DecodeExchangePlan(source string, reader io.Reader, configuration project.Project) (ExchangePlanDefinition, error) {
	var value ExchangePlanDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return ExchangePlanDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateReferenceObjectShape(referenceObjectShape{
		code:              value.Code,
		descriptionLength: value.DescriptionLength,
		attributes:        value.Attributes,
		tableParts:        value.TableParts,
		forms:             HierarchicalObjectForms{ObjectForms: value.Forms},
		list:              value.List,
		reservedName:      reservedExchangePlanName,
	}, configuration)...)
	issues = append(issues, validateExchangePlanContent(value)...)
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, configuration)...)
	issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return ExchangePlanDefinition{}, err
	}
	return value, nil
}

func validateExchangePlanContent(value ExchangePlanDefinition) []string {
	if len(value.Content) > maxExchangePlanContent {
		return []string{fmt.Sprintf("content must not contain more than %d items", maxExchangePlanContent)}
	}
	var issues []string
	seen := map[uuid.UUID]bool{}
	for index, item := range value.Content {
		prefix := fmt.Sprintf("content[%d]", index)
		switch {
		case slices.Contains(registrableKinds, item.Kind):
		case slices.Contains(pendingRegistrableKinds, item.Kind):
			issues = append(issues, prefix+".kind can be registered, but this version does not describe that kind of object yet")
		default:
			issues = append(issues, prefix+".kind is not a kind whose changes can be registered")
		}
		if item.Object.IsZero() {
			issues = append(issues, prefix+".object must be a non-zero UUID")
		}
		// The same object twice is two answers to one question - register
		// automatically or not - and nothing says which of them holds.
		if seen[item.Object] {
			issues = append(issues, prefix+".object is already in the content of this plan")
		}
		seen[item.Object] = true
		switch item.Auto {
		case "", AutoRecordAllow, AutoRecordDeny:
		default:
			issues = append(issues, prefix+".auto_record must be allow or deny")
		}
		// A plan that registers itself would record every node as changed
		// every time any node is written, and the exchange would never settle.
		if item.Object == value.ID {
			issues = append(issues, prefix+" puts the plan into its own content")
		}
	}
	return issues
}

// reservedExchangePlanName keeps the standard attributes of a node: the code
// and description every reference object has, the mark that says which node is
// this one, and the two counters the sides keep of what they have sent and
// received.
func reservedExchangePlanName(name string) bool {
	switch strings.ToLower(name) {
	case "ссылка", "ref", "код", "code", "наименование", "description", "версия", "version",
		"пометкаудаления", "deletionmark", "этотузел", "thisnode",
		"номеротправленного", "sentno", "номерпринятого", "receivedno":
		return true
	default:
		return false
	}
}

func cloneExchangePlan(value ExchangePlanDefinition) ExchangePlanDefinition {
	value.Title = cloneTitle(value.Title)
	value.Content = slices.Clone(value.Content)
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
	return value
}

// ExchangePlan returns one exchange plan by name, folded case.
func (catalog *Catalog) ExchangePlan(name string) (ExchangePlanDefinition, bool) {
	index, ok := catalog.exchangePlanByName[strings.ToLower(name)]
	if !ok {
		return ExchangePlanDefinition{}, false
	}
	return cloneExchangePlan(catalog.ExchangePlans[index]), true
}

// ExchangePlanByID returns one exchange plan by its identifier.
func (catalog *Catalog) ExchangePlanByID(id uuid.UUID) (ExchangePlanDefinition, bool) {
	index, ok := catalog.exchangePlanByID[id]
	if !ok {
		return ExchangePlanDefinition{}, false
	}
	return cloneExchangePlan(catalog.ExchangePlans[index]), true
}

func (catalog *Catalog) exchangePlanTables(definition ExchangePlanDefinition) (schemadiff.Table, []schemadiff.Table, error) {
	tableName, err := PhysicalCatalogTable(definition.ID)
	if err != nil {
		return schemadiff.Table{}, nil, err
	}
	table := schemadiff.Table{
		Name: tableName,
		Columns: []schemadiff.Column{
			{Name: "ref", Type: "uuid", Nullable: false},
			{Name: "version", Type: "bigint", Nullable: false, Default: "1"},
			{Name: "code", Type: codeSQLType(definition.Code), Nullable: false},
			{Name: "description", Type: fmt.Sprintf("character varying(%d)", definition.DescriptionLength), Nullable: false, Default: "''::character varying"},
			// Which node is this base, and what the two sides have sent and
			// received. The counters are what makes an exchange resumable
			// after a break instead of starting from the beginning.
			{Name: "this_node", Type: "boolean", Nullable: false, Default: "false"},
			{Name: "sent_no", Type: "bigint", Nullable: false, Default: "0"},
			{Name: "received_no", Type: "bigint", Nullable: false, Default: "0"},
			{Name: "deletion_mark", Type: "boolean", Nullable: false, Default: "false"},
		},
		Constraints: []schemadiff.Constraint{
			{Name: physicalObjectName("pk", definition.ID), Type: "primary_key", Definition: "PRIMARY KEY (ref)"},
		},
		Indexes: []schemadiff.Index{
			{Name: physicalObjectName("im", definition.ID), Method: "btree", Keys: []string{"deletion_mark"}},
			// A base is one base. Two nodes both claiming to be this one is a
			// state no exchange recovers from, so the database refuses it
			// rather than the code remembering to.
			{Name: physicalObjectName("un", definition.ID), Unique: true, Method: "btree", Keys: []string{"this_node"}, Predicate: "this_node"},
		},
	}
	if definition.Code.Unique {
		table.Constraints = append(table.Constraints, schemadiff.Constraint{Name: physicalObjectName("uq", definition.ID), Type: "unique", Definition: "UNIQUE (code)"})
	} else {
		table.Indexes = append(table.Indexes, schemadiff.Index{Name: physicalObjectName("ic", definition.ID), Method: "btree", Keys: []string{"code"}})
	}
	for _, attribute := range definition.Attributes {
		if err := catalog.appendAttributeSchema(&table, attribute); err != nil {
			return schemadiff.Table{}, nil, fmt.Errorf("exchange plan %s attribute %s: %w", definition.Name, attribute.Name, err)
		}
	}
	appendListSearchIndexes(&table, definition.ID, definition.List, []string{"Description", "Code"}, definition.Attributes, map[string]listColumn{
		"code": {name: "code", kind: definition.Code.Type}, "description": {name: "description", kind: StringType},
	})
	parts, err := catalog.tablePartTables("exchange plan", definition.Name, definition.ID, definition.TableParts)
	if err != nil {
		return schemadiff.Table{}, nil, err
	}
	return table, parts, nil
}
