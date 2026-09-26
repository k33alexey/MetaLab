package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// AccountingRegisterField is a dimension or a resource of an accounting
// register. Beyond what any attribute carries it holds the three things double
// entry needs.
//
// Balance says the value is one per entry rather than one per side: the company
// an entry belongs to is the same in debit and in credit, while the analytics
// of the two sides differ. A non-balance field is therefore kept twice, once
// for each side, and a balance field once.
//
// The two flags tie the field to the chart of accounts: an account that does
// not keep this flag does not keep this field either, and an entry that fills
// it anyway is filling a column the account has no meaning for.
type AccountingRegisterField struct {
	ID                         uuid.UUID     `yaml:"id" json:"id"`
	Name                       string        `yaml:"name" json:"name"`
	Title                      LocalizedText `yaml:"title" json:"title"`
	Types                      []Type        `yaml:"types" json:"types"`
	Indexing                   IndexMode     `yaml:"indexing,omitempty" json:"indexing,omitempty"`
	Balance                    bool          `yaml:"balance,omitempty" json:"balance,omitempty"`
	AccountingFlag             *uuid.UUID    `yaml:"accounting_flag,omitempty" json:"accountingFlag,omitempty"`
	ExtDimensionAccountingFlag *uuid.UUID    `yaml:"ext_dimension_accounting_flag,omitempty" json:"extDimensionAccountingFlag,omitempty"`
}

// AccountingRegisterDefinition holds entries: an account on each side, sums and
// the analytics behind them.
type AccountingRegisterDefinition struct {
	Format int           `yaml:"format" json:"format"`
	ID     uuid.UUID     `yaml:"id" json:"id"`
	Name   string        `yaml:"name" json:"name"`
	Title  LocalizedText `yaml:"title" json:"title"`
	// ChartOfAccounts is where the accounts come from, and with them the
	// analytics: a register has no ext dimensions of its own.
	ChartOfAccounts uuid.UUID `yaml:"chart_of_accounts" json:"chartOfAccounts"`
	// Correspondence is double entry: an entry names a debit account and a
	// credit account at once. Without it an entry touches one account, and
	// there are no two sides to tell apart.
	Correspondence bool `yaml:"correspondence,omitempty" json:"correspondence,omitempty"`
	// TotalsSplitting lets concurrent writers keep their own rows of totals
	// instead of queueing on one.
	TotalsSplitting    bool                      `yaml:"totals_splitting,omitempty" json:"totalsSplitting,omitempty"`
	Dimensions         []AccountingRegisterField `yaml:"dimensions,omitempty" json:"dimensions,omitempty"`
	Resources          []AccountingRegisterField `yaml:"resources" json:"resources"`
	Attributes         []Attribute               `yaml:"attributes,omitempty" json:"attributes,omitempty"`
	StandardAttributes []StandardAttribute       `yaml:"standard_attributes,omitempty" json:"standardAttributes,omitempty"`
	Forms              RegisterForms             `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands           []ObjectCommand           `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates          []ObjectTemplate          `yaml:"templates,omitempty" json:"templates,omitempty"`
}

// DecodeAccountingRegister reads and validates one accounting register.
func DecodeAccountingRegister(source string, reader io.Reader, configuration project.Project) (AccountingRegisterDefinition, error) {
	var value AccountingRegisterDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return AccountingRegisterDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	if value.ChartOfAccounts.IsZero() {
		issues = append(issues, "chart_of_accounts is required: a register of entries without accounts records nothing")
	}
	if len(value.Resources) == 0 {
		issues = append(issues, "resources must contain at least one item: an entry with no amount is not an entry")
	}
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for _, group := range []struct {
		path   string
		fields []AccountingRegisterField
	}{{"dimensions", value.Dimensions}, {"resources", value.Resources}} {
		for index, field := range group.fields {
			prefix := fmt.Sprintf("%s[%d]", group.path, index)
			if field.ID.IsZero() {
				issues = append(issues, prefix+".id must be a non-zero UUID")
			}
			if ids[field.ID] {
				issues = append(issues, prefix+".id must be unique")
			}
			ids[field.ID] = true
			if !validIdentifier(field.Name) {
				issues = append(issues, prefix+".name must be a valid identifier")
			}
			folded := strings.ToLower(field.Name)
			if names[folded] || reservedAccountingRegisterName(folded) {
				issues = append(issues, prefix+".name is taken")
			}
			names[folded] = true
			issues = append(issues, validateTitle(prefix+".title", field.Title, configuration)...)
			issues = append(issues, validateTypes(prefix+".types", field.Types, value.ID)...)
			// Two sides exist only under double entry. Marking a field as the
			// same on both sides of an entry that has one side says nothing,
			// and a setting that says nothing hides the one that would.
			if field.Balance && !value.Correspondence {
				issues = append(issues, prefix+".balance needs correspondence: without two sides there is nothing for a value to be the same on")
			}
			if field.AccountingFlag != nil && field.AccountingFlag.IsZero() {
				issues = append(issues, prefix+".accounting_flag must be a non-zero UUID")
			}
			if field.ExtDimensionAccountingFlag != nil && field.ExtDimensionAccountingFlag.IsZero() {
				issues = append(issues, prefix+".ext_dimension_accounting_flag must be a non-zero UUID")
			}
			if !validIndexMode(field.Indexing) {
				issues = append(issues, prefix+".indexing must be dont-index, index or index-with-additional-order")
			}
		}
	}
	issues = append(issues, validateAttributes("attributes", value.Attributes, configuration, func(name string) bool {
		return names[strings.ToLower(name)] || reservedAccountingRegisterName(name)
	})...)
	// How many ext dimensions an entry really has is the chart's to say, and
	// the chart is in another file: here the platform's ceiling is allowed and
	// load.go narrows it once the chart is read.
	issues = append(issues, validateStandardAttributes("standard_attributes", value.StandardAttributes, accountingStandardFields(value.Correspondence, maxExtDimensions), configuration)...)
	issues = append(issues, validateFieldLinks([]fieldGroup{{"attributes", value.Attributes}}, nil, standardAttributeChoices("standard_attributes", value.StandardAttributes)...)...)
	issues = append(issues, validateAttributeUse([]fieldGroup{{"attributes", value.Attributes}}, nil, false, false)...)
	issues = append(issues, validateFormSlots(value.Forms.slots())...)
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, configuration)...)
	issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return AccountingRegisterDefinition{}, err
	}
	return value, nil
}

// reservedAccountingRegisterName keeps the standard fields of an entry: when it
// was made and by what, which line of it, whether it counts, and the accounts
// of its two sides.
func reservedAccountingRegisterName(name string) bool {
	switch foldStandardName(name) {
	case "счетдт", "accountdr", "счеткт", "accountcr", "recordid":
		// The accounts of the two sides are how the one standard field Счет is
		// stored under double entry, and recordid is the key of a stored row.
		return true
	default:
		return reservedStandardName(AccountingRegisterKind, name)
	}
}

func cloneAccountingRegisterFields(fields []AccountingRegisterField) []AccountingRegisterField {
	fields = slices.Clone(fields)
	for index := range fields {
		fields[index].Title = cloneTitle(fields[index].Title)
		fields[index].Types = cloneTypes(fields[index].Types)
		for _, flag := range []**uuid.UUID{&fields[index].AccountingFlag, &fields[index].ExtDimensionAccountingFlag} {
			if *flag != nil {
				id := **flag
				*flag = &id
			}
		}
	}
	return fields
}

func cloneAccountingRegister(value AccountingRegisterDefinition) AccountingRegisterDefinition {
	value.Title = cloneTitle(value.Title)
	value.Dimensions = cloneAccountingRegisterFields(value.Dimensions)
	value.Resources = cloneAccountingRegisterFields(value.Resources)
	value.Attributes = cloneAttributes(value.Attributes)
	value.Forms = cloneFormSet(value.Forms)
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
	value.StandardAttributes = cloneStandardAttributes(value.StandardAttributes)
	return value
}

// accountingRegisterFields is every field of an entry that carries a value of
// its own, dimensions and resources alike, as plain attributes - which is what
// they are to everything that does not care about double entry.
func accountingRegisterFields(item AccountingRegisterDefinition) []Attribute {
	fields := make([]Attribute, 0, len(item.Dimensions)+len(item.Resources))
	for _, group := range [][]AccountingRegisterField{item.Dimensions, item.Resources} {
		for _, field := range group {
			fields = append(fields, Attribute{ID: field.ID, Name: field.Name, Title: field.Title, Types: field.Types, Indexing: field.Indexing})
		}
	}
	return fields
}

// AccountingRegister returns one accounting register by name, folded case.
func (catalog *Catalog) AccountingRegister(name string) (AccountingRegisterDefinition, bool) {
	index, ok := catalog.accountingRegisterByName[strings.ToLower(name)]
	if !ok {
		return AccountingRegisterDefinition{}, false
	}
	return cloneAccountingRegister(catalog.AccountingRegisters[index]), true
}

// AccountingRegisterByID returns one accounting register by its identifier.
func (catalog *Catalog) AccountingRegisterByID(id uuid.UUID) (AccountingRegisterDefinition, bool) {
	index, ok := catalog.accountingRegisterByID[id]
	if !ok {
		return AccountingRegisterDefinition{}, false
	}
	return cloneAccountingRegister(catalog.AccountingRegisters[index]), true
}
