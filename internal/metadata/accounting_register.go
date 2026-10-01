package metadata

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// AccountingRegisterResource is a resource of an accounting register. Its
// dimensions are the dimension every register has - RegisterDimension, where
// the balance and accounting flags of a dimension live - and the resource keeps
// a structure of its own because it has one property no dimension has.
//
// Balance says the amount is kept by double entry: the debit total equals the
// credit total. AccountingFlag ties the amount to the chart of accounts, as it
// does for a dimension. ExtDimensionAccountingFlag says in which kinds of ext
// dimension the amount is kept, and a dimension keeps no amount - which is why
// the help gives this one to the resource alone.
type AccountingRegisterResource struct {
	// Attribute is the whole common set - how the value is shown, how it is
	// chosen, what it starts as, whether it is checked and searched.
	Attribute                  `yaml:",inline" json:",inline"`
	Balance                    bool       `yaml:"balance,omitempty" json:"balance,omitempty"`
	AccountingFlag             *uuid.UUID `yaml:"accounting_flag,omitempty" json:"accountingFlag,omitempty"`
	ExtDimensionAccountingFlag *uuid.UUID `yaml:"ext_dimension_accounting_flag,omitempty" json:"extDimensionAccountingFlag,omitempty"`
}

// AccountingRegisterDefinition holds entries: an account on each side, sums and
// the analytics behind them.
type AccountingRegisterDefinition struct {
	Format int           `yaml:"format" json:"format"`
	ID     uuid.UUID     `yaml:"id" json:"id"`
	Name   string        `yaml:"name" json:"name"`
	Title  LocalizedText `yaml:"title" json:"title"` // ListPresentations and the help flag: a row of this kind is not an object a
	// person opens, so there is a list to name and no object - see
	// object_presentation.go.
	ListPresentations     `yaml:",inline" json:",inline"`
	IncludeHelpInContents bool `yaml:"include_help_in_contents,omitempty" json:"includeHelpInContents,omitempty"`
	// DataLock is how the platform locks records of this register while they
	// are written - see data_lock_settings.go. A register has the mode and no
	// fields: the prototype gives the list of fields only to the kinds that
	// have an object of their own.
	DataLock project.DataLockControlMode `yaml:"data_lock,omitempty" json:"dataLock,omitempty"`
	// FullTextSearch is whether the records of this register are in the
	// full-text index - see full_text_search.go.
	FullTextSearch FullTextSearchMode `yaml:"full_text_search,omitempty" json:"fullTextSearch,omitempty"`
	// AdditionalIndexes are the indexes this register asks the database for
	// beside the ones the platform builds - see additional_indexes.go.
	AdditionalIndexes []AdditionalIndex `yaml:"additional_indexes,omitempty" json:"additionalIndexes,omitempty"`

	// ChartOfAccounts is where the accounts come from, and with them the
	// analytics: a register has no ext dimensions of its own.
	ChartOfAccounts uuid.UUID `yaml:"chart_of_accounts" json:"chartOfAccounts"`
	// Correspondence is double entry: an entry names a debit account and a
	// credit account at once. Without it an entry touches one account, and
	// there are no two sides to tell apart.
	Correspondence bool `yaml:"correspondence,omitempty" json:"correspondence,omitempty"`
	// AllowTotalsSplitting permits concurrent writers to keep their own rows
	// of totals instead of queueing on one - see register_totals_mode.go. It
	// permits and does not switch, and this register has nothing yet to
	// switch: its totals arrive with posting, which is 1.0.2. The property is
	// carried and not executed.
	AllowTotalsSplitting bool `yaml:"allow_totals_splitting,omitempty" json:"allowTotalsSplitting,omitempty"`
	// PeriodAdjustmentLength orders entries beyond their period: of two with
	// equal periods, the smaller refinement is the earlier. Zero means the
	// register does not support refinement - see register_properties.go.
	PeriodAdjustmentLength int                          `yaml:"period_adjustment_length,omitempty" json:"periodAdjustmentLength,omitempty"`
	Dimensions             []RegisterDimension          `yaml:"dimensions,omitempty" json:"dimensions,omitempty"`
	Resources              []AccountingRegisterResource `yaml:"resources" json:"resources"`
	Attributes             []Attribute                  `yaml:"attributes,omitempty" json:"attributes,omitempty"`
	StandardAttributes     []StandardAttribute          `yaml:"standard_attributes,omitempty" json:"standardAttributes,omitempty"`
	Forms                  RegisterForms                `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands               []ObjectCommand              `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates              []ObjectTemplate             `yaml:"templates,omitempty" json:"templates,omitempty"`
}

// DecodeAccountingRegister reads and validates one accounting register.
func DecodeAccountingRegister(source string, reader io.Reader, configuration project.Project) (AccountingRegisterDefinition, error) {
	var value AccountingRegisterDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return AccountingRegisterDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateDataLockMode("data_lock", value.DataLock)...)
	issues = append(issues, validateFullTextSearch("full_text_search", value.FullTextSearch)...)
	if value.ChartOfAccounts.IsZero() {
		issues = append(issues, "chart_of_accounts is required: a register of entries without accounts records nothing")
	}
	if len(value.Resources) == 0 {
		issues = append(issues, "resources must contain at least one item: an entry with no amount is not an entry")
	}
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	// What a dimension and a resource share: the common set, the two sides,
	// and the flag of the chart of accounts.
	validateField := func(prefix string, field Attribute, balance bool, accountingFlag *uuid.UUID) {
		issues = append(issues, validateRegisterField(prefix, field, value.ID,
			names, ids, configuration, reservedAccountingRegisterName)...)
		// Two sides exist only under double entry. Marking a field as the
		// same on both sides of an entry that has one side says nothing, and
		// a setting that says nothing hides the one that would.
		if balance && !value.Correspondence {
			issues = append(issues, prefix+".balance needs correspondence: without two sides there is nothing for a value to be the same on")
		}
		if accountingFlag != nil && accountingFlag.IsZero() {
			issues = append(issues, prefix+".accounting_flag must be a non-zero UUID")
		}
		issues = append(issues, validateMovementFieldStorage(prefix, field)...)
	}
	for index, dimension := range value.Dimensions {
		prefix := fmt.Sprintf("dimensions[%d]", index)
		validateField(prefix, dimension.Attribute, dimension.Balance, dimension.AccountingFlag)
		issues = append(issues, validateRegisterDimension(prefix, dimension, accountingRegisterDimensions())...)
	}
	for index, resource := range value.Resources {
		prefix := fmt.Sprintf("resources[%d]", index)
		validateField(prefix, resource.Attribute, resource.Balance, resource.AccountingFlag)
		if resource.ExtDimensionAccountingFlag != nil && resource.ExtDimensionAccountingFlag.IsZero() {
			issues = append(issues, prefix+".ext_dimension_accounting_flag must be a non-zero UUID")
		}
		issues = append(issues, validateResourceIndexing(prefix, resource.Indexing)...)
	}
	issues = append(issues, validateAttributes("attributes", value.Attributes, configuration, func(name string) bool {
		return names[strings.ToLower(name)] || reservedAccountingRegisterName(name)
	})...)
	for index, attribute := range value.Attributes {
		issues = append(issues, validateMovementFieldStorage(fmt.Sprintf("attributes[%d]", index), attribute)...)
	}
	// How many ext dimensions an entry really has is the chart's to say, and
	// the chart is in another file: here the platform's ceiling is allowed and
	// load.go narrows it once the chart is read.
	issues = append(issues, validateListPresentations(value.ListPresentations, configuration)...)
	issues = append(issues, validatePeriodAdjustmentLength(value.PeriodAdjustmentLength)...)
	issues = append(issues, validateStandardAttributes("standard_attributes", value.StandardAttributes, accountingStandardFields(value.Correspondence, mentionedExtDimensions(value)), configuration)...)
	issues = append(issues, validateAdditionalIndexes(value.AdditionalIndexes, AccountingRegisterKind,
		recordIndexTables(accountingStandardFields(value.Correspondence, mentionedExtDimensions(value)),
			attributeNames(RegisterDimensionAttributes(value.Dimensions)), attributeNames(accountingResourceAttributes(value.Resources)),
			attributeNames(value.Attributes)))...)
	// Dimensions and resources belong in these two checks, and until now only
	// attributes were in them. That cost both ways. A choice parameter link
	// from an attribute to a dimension of the same register - «отбор по
	// организации проводки» - was refused as pointing outside the object,
	// because the list of fields the link could reach was built from
	// attributes alone. And a link drawn from a dimension was not looked at at
	// all, so one pointing nowhere went in and failed when the form opened.
	registerFields := []fieldGroup{
		{"dimensions", RegisterDimensionAttributes(value.Dimensions)},
		{"resources", accountingResourceAttributes(value.Resources)},
		{"attributes", value.Attributes},
	}
	issues = append(issues, validateFieldLinks(registerFields, nil, standardAttributeChoices("standard_attributes", value.StandardAttributes)...)...)
	issues = append(issues, validateAttributeUse(registerFields, nil, false, false)...)
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

func cloneAccountingRegisterResources(resources []AccountingRegisterResource) []AccountingRegisterResource {
	resources = slices.Clone(resources)
	for index := range resources {
		resources[index].Attribute = cloneAttribute(resources[index].Attribute)
		for _, flag := range []**uuid.UUID{&resources[index].AccountingFlag, &resources[index].ExtDimensionAccountingFlag} {
			if *flag != nil {
				id := **flag
				*flag = &id
			}
		}
	}
	return resources
}

func cloneAccountingRegister(value AccountingRegisterDefinition) AccountingRegisterDefinition {
	value.Title = cloneTitle(value.Title)
	value.Dimensions = cloneRegisterDimensions(value.Dimensions)
	value.Resources = cloneAccountingRegisterResources(value.Resources)
	value.Attributes = cloneAttributes(value.Attributes)
	value.Forms = cloneFormSet(value.Forms)
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
	value.StandardAttributes = cloneStandardAttributes(value.StandardAttributes)
	value.ListPresentations = cloneListPresentations(value.ListPresentations)
	return value
}

// accountingResourceAttributes is the resources as plain attributes, which is
// what the checks shared by every kind of object take.
func accountingResourceAttributes(resources []AccountingRegisterResource) []Attribute {
	result := make([]Attribute, 0, len(resources))
	for _, resource := range resources {
		result = append(result, resource.Attribute)
	}
	return result
}

// accountingRegisterFields is every field of an entry that carries a value of
// its own, dimensions and resources alike, as plain attributes - which is what
// they are to everything that does not care about double entry.
func accountingRegisterFields(item AccountingRegisterDefinition) []Attribute {
	fields := make([]Attribute, 0, len(item.Dimensions)+len(item.Resources))
	for _, field := range append(RegisterDimensionAttributes(item.Dimensions), accountingResourceAttributes(item.Resources)...) {
		fields = append(fields, cloneAttribute(field))
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

// mentionedExtDimensions is how many ext dimensions the register's own
// description speaks of - the highest N of a field СубконтоN, ВидСубконтоN or
// their English names among its standard attributes and additional indexes.
//
// The chart of accounts decides how many an entry really has, and it is in
// another file: when the register is read alone, the names of the ext
// dimension fields are generated up to what the description itself names, so
// that no number of ours stands between a register and a chart that allows
// more; load.go narrows it to the chart once the chart is read.
func mentionedExtDimensions(value AccountingRegisterDefinition) int {
	highest := 0
	mention := func(name string) {
		for _, prefix := range []string{"субконто", "видсубконто", "extdimension", "extdimensiontype"} {
			lowered := strings.ToLower(strings.TrimSpace(name))
			if !strings.HasPrefix(lowered, prefix) {
				continue
			}
			if number, err := strconv.Atoi(lowered[len(prefix):]); err == nil && number > highest {
				highest = number
			}
		}
	}
	for _, attribute := range value.StandardAttributes {
		mention(attribute.Name)
	}
	for _, index := range value.AdditionalIndexes {
		for _, field := range append(slices.Clone(index.IndexedFields), index.AdditionalFields...) {
			mention(field)
		}
	}
	return highest
}
