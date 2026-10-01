package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// AccountKind is what an account's balance means - the prototype's вид счёта.
// It is the platform's own closed list, not something a configuration invents.
type AccountKind string

const (
	// ActiveAccount always carries a debit balance.
	ActiveAccount AccountKind = "active"
	// PassiveAccount always carries a credit balance.
	PassiveAccount AccountKind = "passive"
	// ActivePassiveAccount carries a debit balance when positive and a credit
	// one when negative.
	ActivePassiveAccount AccountKind = "active-passive"
)

// AccountingFlag is a named checkbox a chart of accounts declares for itself:
// "Количественный", "Валютный", "Суммовой". The platform knows none of these
// names in advance - the application names them, every account gets a checkbox
// per declared flag, and a register later refuses a movement that fills a
// field the account's flag does not allow.
//
// It is a field, and the whole palette of a field is on it. A name, a synonym
// and an identifier were all we carried, and the export writes twenty-six
// properties on every accounting flag and on every ext dimension accounting
// flag: the format the checkbox is shown in, its tooltip, its filling value,
// its choice form and the rest. Three of the twenty-nine an attribute has are
// missing there, and the syntax assistant agrees with the silence - an
// accounting flag has no indexing, no full-text search and no «использование»,
// which is why validateAccountingFlagStorage refuses all three.
//
// The type is not a choice. «Тип: Булево» is what the help says of the value on
// an account and of the field in a query alike, so an empty list means boolean
// and anything else is refused. Carrying the property still matters: the export
// writes <Type> on every flag, and a property dropped on import is the report's
// third category.
type AccountingFlag struct {
	Attribute `yaml:",inline" json:",inline"`
}

// accountingFlagTypes is the one type a flag's value has. An empty list in a
// file means the same thing: a property with a single possible value is not
// worth a line in every chart of accounts.
func accountingFlagTypes(flag AccountingFlag) []Type {
	if len(flag.Types) == 0 {
		return []Type{{Kind: BooleanType}}
	}
	return flag.Types
}

// isBooleanType says whether a declared type is the boolean an accounting flag
// is allowed, and nothing else - no qualifiers, no second type in the set.
func isBooleanType(types []Type) bool {
	return len(types) == 1 && types[0].Kind == BooleanType && types[0].Reference == nil
}

// PredefinedAccountExtDimension is one kind of analytics on a predefined
// account, with the per-analytics flags that the chart declared.
type PredefinedAccountExtDimension struct {
	// Characteristic names the predefined element of the chart of
	// characteristic types that serves as the kind of analytics.
	Characteristic string          `yaml:"characteristic" json:"characteristic"`
	TurnoverOnly   bool            `yaml:"turnover_only,omitempty" json:"turnoverOnly,omitempty"`
	Flags          map[string]bool `yaml:"flags,omitempty" json:"flags,omitempty"`
}

// PredefinedAccount is an account the configuration itself brings, with
// everything that makes an account an account: what its balance means, whether
// it is off balance, its place among the others, its flags and its analytics.
// A predefined account described by name and code alone would arrive stripped
// of its accounting meaning, which is the one thing it exists for.
type PredefinedAccount struct {
	ID          uuid.UUID   `yaml:"id" json:"id"`
	Name        string      `yaml:"name" json:"name"`
	Code        string      `yaml:"code,omitempty" json:"code,omitempty"`
	Description string      `yaml:"description,omitempty" json:"description,omitempty"`
	Kind        AccountKind `yaml:"kind" json:"kind"`
	OffBalance  bool        `yaml:"off_balance,omitempty" json:"offBalance,omitempty"`
	// Parent names the predefined account this one is a subaccount of. The
	// hierarchy itself is not implemented yet; the structure is kept so that
	// it is not lost on the way in.
	Parent string `yaml:"parent,omitempty" json:"parent,omitempty"`
	// Order is the account's place among the others, as the configuration
	// wrote it. It is carried, not derived: 124 of the 966 predefined accounts
	// of the configurations being moved have an order the code does not give
	// - a leading space, an off-balance account ordered «Заб01» under the code
	// 01 - so deriving it would rewrite what a bookkeeper chose.
	Order         string                          `yaml:"order,omitempty" json:"order,omitempty"`
	Flags         map[string]bool                 `yaml:"flags,omitempty" json:"flags,omitempty"`
	ExtDimensions []PredefinedAccountExtDimension `yaml:"ext_dimensions,omitempty" json:"extDimensions,omitempty"`
}

// ChartOfAccountsDefinition describes a chart of accounts: the accounts an
// application keeps its books on, what analytics each of them carries, and the
// flags a bookkeeping register checks its movements against.
type ChartOfAccountsDefinition struct {
	Format int           `yaml:"format" json:"format"`
	ID     uuid.UUID     `yaml:"id" json:"id"`
	Name   string        `yaml:"name" json:"name"`
	Title  LocalizedText `yaml:"title" json:"title"`
	// Presentations is how this object is named to the person using it -
	// see object_presentation.go.
	Presentations `yaml:",inline" json:",inline"` // ObjectChoice is how a value of this kind is entered and picked - see
	// object_choice.go.
	ObjectChoice `yaml:",inline" json:",inline"`
	// BasedOn are the objects one of these may be made out of, the list the
	// command to make it offers - see based_on.go.
	BasedOn []uuid.UUID `yaml:"based_on,omitempty" json:"basedOn,omitempty"`
	// DataLock is how the platform locks a row of this object while it is
	// written, and DataLockFields are the fields it may be locked by - see
	// data_lock_settings.go.
	DataLock       project.DataLockControlMode `yaml:"data_lock,omitempty" json:"dataLock,omitempty"`
	DataLockFields []ObjectField               `yaml:"data_lock_fields,omitempty" json:"dataLockFields,omitempty"`
	// FullTextSearch is whether this object is in the full-text index at all -
	// see full_text_search.go.
	FullTextSearch FullTextSearchMode `yaml:"full_text_search,omitempty" json:"fullTextSearch,omitempty"`
	// DataHistorySettings is whether this object takes part in data history
	// and the two flags that go with it - see data_history.go.
	DataHistorySettings `yaml:",inline" json:",inline"`
	// AdditionalIndexes are the indexes this object asks the database for
	// beside the ones the platform builds - see additional_indexes.go.
	AdditionalIndexes []AdditionalIndex `yaml:"additional_indexes,omitempty" json:"additionalIndexes,omitempty"`

	Code              CatalogCode `yaml:"code" json:"code"`
	DescriptionLength int         `yaml:"description_length" json:"descriptionLength"`
	// CodeMask describes how an account code is built - "@@.@@" is two
	// positions, a dot, two more. The syntax assistant says only that it is
	// a string describing the structure of the code; its grammar is the one
	// of an input mask (Mask of a text box): the positions ! 9 # N U X ^ h @,
	// any other symbol a separator, a backslash making the next symbol a
	// separator, and several masks joined by ";". The charts being moved
	// write "@@@@@   ", "@@@@@@@@@" and "XXXXXXXX".
	CodeMask string `yaml:"code_mask,omitempty" json:"codeMask,omitempty"`
	// OrderLength is the width of the order string. It does not depend on the
	// mask: the designer saves a mask of five with an order of three (checked
	// by the owner on the platform, 01.10.2026).
	OrderLength int `yaml:"order_length,omitempty" json:"orderLength,omitempty"`
	// AutoOrderByCode makes ordering by code order by the derived order
	// instead, so that 41.1 sorts after 9 rather than before it.
	AutoOrderByCode bool `yaml:"auto_order_by_code,omitempty" json:"autoOrderByCode,omitempty"`
	// ExtDimensionTypes is the chart of characteristic types that supplies the
	// kinds of analytics; MaxExtDimensionCount is how many an account may
	// carry at once.
	ExtDimensionTypes           *uuid.UUID             `yaml:"ext_dimension_types,omitempty" json:"extDimensionTypes,omitempty"`
	MaxExtDimensionCount        int                    `yaml:"max_ext_dimension_count,omitempty" json:"maxExtDimensionCount,omitempty"`
	AccountingFlags             []AccountingFlag       `yaml:"accounting_flags,omitempty" json:"accountingFlags,omitempty"`
	ExtDimensionAccountingFlags []AccountingFlag       `yaml:"ext_dimension_accounting_flags,omitempty" json:"extDimensionAccountingFlags,omitempty"`
	Attributes                  []Attribute            `yaml:"attributes,omitempty" json:"attributes,omitempty"`
	TableParts                  []TablePart            `yaml:"table_parts,omitempty" json:"tableParts,omitempty"`
	Characteristics             []ObjectCharacteristic `yaml:"characteristics,omitempty" json:"characteristics,omitempty"`
	StandardAttributes          []StandardAttribute    `yaml:"standard_attributes,omitempty" json:"standardAttributes,omitempty"`
	StandardTableParts          []StandardTablePart    `yaml:"standard_table_parts,omitempty" json:"standardTableParts,omitempty"`
	Forms                       ObjectForms            `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands                    []ObjectCommand        `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates                   []ObjectTemplate       `yaml:"templates,omitempty" json:"templates,omitempty"`
	List                        ListSettings           `yaml:"list,omitempty" json:"list,omitempty"`
	Predefined                  []PredefinedAccount    `yaml:"predefined,omitempty" json:"predefined,omitempty"`
	PredefinedDataUpdate        PredefinedDataUpdate   `yaml:"predefined_data_update,omitempty" json:"predefinedDataUpdate,omitempty"`
}

// DecodeChartOfAccounts reads and validates one chart of accounts.
func DecodeChartOfAccounts(source string, reader io.Reader, configuration project.Project) (ChartOfAccountsDefinition, error) {
	var value ChartOfAccountsDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return ChartOfAccountsDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateReferenceObjectShape(referenceObjectShape{
		code:              value.Code,
		descriptionLength: value.DescriptionLength,
		attributes:        value.Attributes,
		tableParts:        value.TableParts,
		forms:             HierarchicalObjectForms{ObjectForms: value.Forms},
		list:              value.List,
		reservedName:      reservedChartOfAccountsName,
		codeSeries:        true,
		checkUnique:       true,
		codeMayBeAbsent:   true,
		// codeAllowedLength stays off: the shape of an account code is the code
		// mask's to decide, and the prototype gives the chart no such property.
		predefinedDataUpdate: value.PredefinedDataUpdate,
		presentation:         value.Presentations,
		choice:               value.ObjectChoice,
		basedOn:              value.BasedOn,
		dataLock:             value.DataLock,
		dataLockFields:       value.DataLockFields,
		fullTextSearch:       value.FullTextSearch,
		dataHistory:          value.DataHistorySettings,
		additionalIndexes:    value.AdditionalIndexes,
		kind:                 ChartOfAccountsKind,
		standardAttributes:   value.StandardAttributes,
		standardTableParts:   value.StandardTableParts,
		extraFields: []fieldGroup{
			{"accounting_flags", accountingFlagFields(value.AccountingFlags)},
			{"ext_dimension_accounting_flags", accountingFlagFields(value.ExtDimensionAccountingFlags)},
		},
	}, configuration)...)
	issues = append(issues, validateAccountingFlags("accounting_flags", value.AccountingFlags, configuration)...)
	issues = append(issues, validateAccountingFlags("ext_dimension_accounting_flags", value.ExtDimensionAccountingFlags, configuration)...)
	if value.MaxExtDimensionCount < 0 {
		issues = append(issues, "max_ext_dimension_count must not be negative")
	}
	if value.ExtDimensionTypes != nil && value.ExtDimensionTypes.IsZero() {
		issues = append(issues, "ext_dimension_types must be a non-zero UUID")
	}
	// Analytics without a chart to take them from, or a chart nobody is
	// allowed to use, are two halves of the same mistake.
	if value.ExtDimensionTypes == nil && value.MaxExtDimensionCount > 0 {
		issues = append(issues, "max_ext_dimension_count needs ext_dimension_types to take the kinds of analytics from")
	}
	if value.ExtDimensionTypes != nil && value.MaxExtDimensionCount == 0 {
		issues = append(issues, "ext_dimension_types is useless while max_ext_dimension_count is zero")
	}
	if len(value.ExtDimensionAccountingFlags) > 0 && value.ExtDimensionTypes == nil {
		issues = append(issues, "ext_dimension_accounting_flags need ext_dimension_types: there is nothing to flag")
	}
	issues = append(issues, validateCodeMask(value)...)
	issues = append(issues, validatePredefinedAccounts(value)...)
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, configuration)...)
	issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
	issues = append(issues, validateObjectCharacteristics(value.Characteristics)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return ChartOfAccountsDefinition{}, err
	}
	return value, nil
}

func validateCodeMask(value ChartOfAccountsDefinition) []string {
	var issues []string
	// Any symbol is either a position or a separator, so what can be wrong is
	// only what no mask is: a control character.
	for _, symbol := range value.CodeMask {
		if unicode.IsControl(symbol) {
			issues = append(issues, "code_mask must not contain control characters")
			break
		}
	}
	// The help names no ceiling on the order - the charts being moved go up
	// to 9 - so the only one is the database's.
	if value.OrderLength < 0 || value.OrderLength > maxVarcharLength {
		issues = append(issues, fmt.Sprintf("order_length must be 0..%d", maxVarcharLength))
	}
	if value.AutoOrderByCode && value.OrderLength == 0 {
		issues = append(issues, "auto_order_by_code needs an order_length to write the order into")
	}
	return issues
}

// accountingFlagFields is the flags as plain fields, for the checks that do not
// care which kind of field this is.
func accountingFlagFields(flags []AccountingFlag) []Attribute {
	result := make([]Attribute, 0, len(flags))
	for _, flag := range flags {
		result = append(result, flag.Attribute)
	}
	return result
}

func validateAccountingFlags(path string, flags []AccountingFlag, configuration project.Project) []string {
	var issues []string
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for index, flag := range flags {
		prefix := fmt.Sprintf("%s[%d]", path, index)
		if flag.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if ids[flag.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		ids[flag.ID] = true
		if !validIdentifier(flag.Name) {
			issues = append(issues, prefix+".name must be a valid identifier")
		}
		folded := strings.ToLower(flag.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		if reservedChartOfAccountsName(folded) {
			issues = append(issues, prefix+".name is reserved")
		}
		names[folded] = true
		issues = append(issues, validateTitle(prefix+".title", flag.Title, configuration)...)
		if !isBooleanType(accountingFlagTypes(flag)) {
			issues = append(issues, prefix+".types must be boolean alone: an accounting flag is a checkbox and has no other type")
		}
		field := flag.Attribute
		field.Types = accountingFlagTypes(flag)
		issues = append(issues, validateFieldSettings(prefix, field, configuration)...)
		issues = append(issues, validateFieldStorage(prefix, field)...)
		issues = append(issues, validateAccountingFlagStorage(prefix, flag)...)
	}
	return issues
}

// validateAccountingFlagStorage refuses the three properties of a field that an
// accounting flag does not have. The export writes twenty-six properties on a
// flag and twenty-nine on a catalog attribute, and these three are the
// difference; the syntax assistant lists neither of them among the properties of
// ПризнакУчетаПланаСчетов or ПризнакУчетаСубконтоПланаСчетов.
//
// Why refuse rather than ignore: a setting nobody reads looks like a setting.
// Indexing an account's checkbox reads as a promise that a query by it is fast,
// and there is no index; «использование» reads as a division between items and
// folders, and a chart of accounts has neither.
func validateAccountingFlagStorage(prefix string, flag AccountingFlag) []string {
	var issues []string
	if flag.Indexing != "" {
		issues = append(issues, prefix+".indexing belongs to an attribute: an account is found by its code, never by a checkbox")
	}
	if flag.FullTextSearch != "" {
		issues = append(issues, prefix+".full_text_search belongs to an attribute: a checkbox carries no text to search")
	}
	if flag.Use != "" {
		issues = append(issues, prefix+".use belongs to an attribute of a catalog or a chart of characteristic types, and this is neither")
	}
	return issues
}

func validatePredefinedAccounts(value ChartOfAccountsDefinition) []string {
	flagNames := make(map[string]bool, len(value.AccountingFlags))
	for _, flag := range value.AccountingFlags {
		flagNames[strings.ToLower(flag.Name)] = true
	}
	extFlagNames := make(map[string]bool, len(value.ExtDimensionAccountingFlags))
	for _, flag := range value.ExtDimensionAccountingFlags {
		extFlagNames[strings.ToLower(flag.Name)] = true
	}
	var issues []string
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for index, account := range value.Predefined {
		prefix := fmt.Sprintf("predefined[%d]", index)
		if account.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if ids[account.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		ids[account.ID] = true
		if !validIdentifier(account.Name) || utf8.RuneCountInString(account.Name) > maxNameLength {
			issues = append(issues, prefix+".name must be a valid identifier of at most 255 characters")
		}
		folded := strings.ToLower(account.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		names[folded] = true
		switch account.Kind {
		case ActiveAccount, PassiveAccount, ActivePassiveAccount:
		default:
			issues = append(issues, prefix+".kind must be active, passive or active-passive")
		}
		if value.Code.Length == 0 {
			if account.Code != "" {
				issues = append(issues, prefix+".code is given, and the code is switched off by a length of 0")
			}
		} else if account.Code == "" {
			if !value.Code.Auto {
				issues = append(issues, prefix+".code is required when automatic codes are disabled")
			}
		} else if _, err := normalizeCatalogCode(value.Code, account.Code); err != nil {
			issues = append(issues, prefix+".code is invalid: "+err.Error())
		}
		// The order lies in a column as wide as the order length - or the code
		// length where the order length is not given - so a longer one is the
		// database's limit, not ours.
		if width := orderColumnWidth(value); utf8.RuneCountInString(account.Order) > width {
			issues = append(issues, fmt.Sprintf("%s.order must not exceed %d characters", prefix, width))
		}
		if utf8.RuneCountInString(account.Description) > value.DescriptionLength {
			issues = append(issues, fmt.Sprintf("%s.description must not exceed %d characters", prefix, value.DescriptionLength))
		}
		for name := range account.Flags {
			if !flagNames[strings.ToLower(name)] {
				issues = append(issues, prefix+".flags."+name+" is not a declared accounting flag")
			}
		}
		if len(account.ExtDimensions) > value.MaxExtDimensionCount {
			issues = append(issues, fmt.Sprintf("%s carries %d kinds of analytics, more than the %d allowed",
				prefix, len(account.ExtDimensions), value.MaxExtDimensionCount))
		}
		seen := map[string]bool{}
		for position, dimension := range account.ExtDimensions {
			where := fmt.Sprintf("%s.ext_dimensions[%d]", prefix, position)
			if dimension.Characteristic == "" {
				issues = append(issues, where+".characteristic is required")
			}
			key := strings.ToLower(dimension.Characteristic)
			if seen[key] {
				issues = append(issues, where+".characteristic repeats on the same account")
			}
			seen[key] = true
			for name := range dimension.Flags {
				if !extFlagNames[strings.ToLower(name)] {
					issues = append(issues, where+".flags."+name+" is not a declared analytics flag")
				}
			}
		}
	}
	// A subaccount whose parent is not there loses its place in the chart.
	for index, account := range value.Predefined {
		if account.Parent != "" && !names[strings.ToLower(account.Parent)] {
			issues = append(issues, fmt.Sprintf("predefined[%d].parent %s is not a predefined account of this chart", index, account.Parent))
		}
		if account.Parent != "" && strings.EqualFold(account.Parent, account.Name) {
			issues = append(issues, fmt.Sprintf("predefined[%d] cannot be its own parent", index))
		}
	}
	return issues
}

// reservedChartOfAccountsName keeps the catalog's own names and the account's
// standard attributes: order, whether it is off balance, and what its balance
// means.
func reservedChartOfAccountsName(name string) bool {
	switch foldStandardName(name) {
	case "accounttype", "видсчета":
		// Two spellings of the kind of account that are not the prototype's
		// own - the prototype calls the field Вид - but which a developer
		// reaches for, and which would then shadow it.
		return true
	default:
		return reservedStandardName(ChartOfAccountsKind, name) || reservedRowVersionName(name)
	}
}

// AccountCodeOrder derives the order string of an account from its code and the
// chart's mask: each fragment of the code is pushed to the right of its place
// in the mask, so that "41.1" under "@@@.@@@" orders as " 41.  1" and sorts
// where a bookkeeper expects it rather than where the alphabet puts it.
func AccountCodeOrder(mask, code string, length int) string {
	if mask == "" {
		return code
	}
	maskParts, separators := splitCodeMask(mask)
	codeParts := splitCodeBySeparators(code, separators)
	var builder strings.Builder
	for index, part := range maskParts {
		if index > 0 {
			builder.WriteRune(separators[index-1])
		}
		fragment := ""
		if index < len(codeParts) {
			fragment = codeParts[index]
		}
		if pad := part - utf8.RuneCountInString(fragment); pad > 0 {
			builder.WriteString(strings.Repeat(" ", pad))
		}
		builder.WriteString(fragment)
	}
	order := builder.String()
	if length > 0 {
		runes := []rune(order)
		if len(runes) > length {
			return string(runes[:length])
		}
		return order + strings.Repeat(" ", length-len(runes))
	}
	return order
}

// orderColumnWidth is how wide the order of an account may be: the order
// length, or the code length where the order length is not given.
func orderColumnWidth(definition ChartOfAccountsDefinition) int {
	if definition.OrderLength > 0 {
		return definition.OrderLength
	}
	return definition.Code.Length
}

// codeMaskPositions are the symbols of an input mask that stand for a
// character of the code; every other symbol is a separator.
const codeMaskPositions = "!9#NUX^h@"

// splitCodeMask reads the first of the masks - the one a code is ordered by -
// into the widths of its fragments and the separators between them. A
// backslash makes the next symbol a separator even where it is a position.
func splitCodeMask(mask string) ([]int, []rune) {
	if first, _, found := strings.Cut(mask, ";"); found {
		mask = first
	}
	var widths []int
	var separators []rune
	width, escaped := 0, false
	for _, symbol := range mask {
		switch {
		case escaped:
			escaped = false
		case symbol == '\\':
			escaped = true
			continue
		case strings.ContainsRune(codeMaskPositions, symbol):
			width++
			continue
		}
		widths = append(widths, width)
		separators = append(separators, symbol)
		width = 0
	}
	return append(widths, width), separators
}

func splitCodeBySeparators(code string, separators []rune) []string {
	if len(separators) == 0 {
		return []string{code}
	}
	return strings.FieldsFunc(code, func(symbol rune) bool {
		return slices.Contains(separators, symbol)
	})
}

func cloneAccountingFlags(values []AccountingFlag) []AccountingFlag {
	result := slices.Clone(values)
	for index := range result {
		result[index].Attribute = cloneAttribute(result[index].Attribute)
	}
	return result
}

func cloneChartOfAccounts(value ChartOfAccountsDefinition) ChartOfAccountsDefinition {
	value.Title = cloneTitle(value.Title)
	value.Attributes = cloneAttributes(value.Attributes)
	value.TableParts = cloneTableParts(value.TableParts)
	value.AccountingFlags = cloneAccountingFlags(value.AccountingFlags)
	value.ExtDimensionAccountingFlags = cloneAccountingFlags(value.ExtDimensionAccountingFlags)
	if value.ExtDimensionTypes != nil {
		id := *value.ExtDimensionTypes
		value.ExtDimensionTypes = &id
	}
	value.Forms = cloneFormSet(value.Forms)
	value.List.SearchFields = slices.Clone(value.List.SearchFields)
	value.Predefined = slices.Clone(value.Predefined)
	for index := range value.Predefined {
		value.Predefined[index].Flags = cloneFlagValues(value.Predefined[index].Flags)
		dimensions := slices.Clone(value.Predefined[index].ExtDimensions)
		for position := range dimensions {
			dimensions[position].Flags = cloneFlagValues(dimensions[position].Flags)
		}
		value.Predefined[index].ExtDimensions = dimensions
	}
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
	value.Characteristics = cloneObjectCharacteristics(value.Characteristics)
	value.StandardAttributes = cloneStandardAttributes(value.StandardAttributes)
	value.StandardTableParts = cloneStandardTableParts(value.StandardTableParts)
	value.Presentations = clonePresentations(value.Presentations)
	value.ObjectInput = cloneObjectInput(value.ObjectInput)
	value.BasedOn = slices.Clone(value.BasedOn)
	value.DataLockFields = cloneDataLockFields(value.DataLockFields)
	return value
}

func cloneFlagValues(values map[string]bool) map[string]bool {
	if values == nil {
		return nil
	}
	result := make(map[string]bool, len(values))
	for name, value := range values {
		result[name] = value
	}
	return result
}

// ChartOfAccounts returns one chart by name, folded case.
func (catalog *Catalog) ChartOfAccounts(name string) (ChartOfAccountsDefinition, bool) {
	index, ok := catalog.chartOfAccountsByName[strings.ToLower(name)]
	if !ok {
		return ChartOfAccountsDefinition{}, false
	}
	return cloneChartOfAccounts(catalog.ChartsOfAccounts[index]), true
}

// ChartOfAccountsByID returns one chart by its identifier.
func (catalog *Catalog) ChartOfAccountsByID(id uuid.UUID) (ChartOfAccountsDefinition, bool) {
	index, ok := catalog.chartOfAccountsByID[id]
	if !ok {
		return ChartOfAccountsDefinition{}, false
	}
	return cloneChartOfAccounts(catalog.ChartsOfAccounts[index]), true
}

// extDimensionTableName is the analytics table of a chart. The section is the
// platform's own, not an attribute of the configuration, so it has no UUID to
// be named by and takes a derived name from the chart itself.
func extDimensionTableName(id uuid.UUID) string { return physicalObjectName("td", id) }

func (catalog *Catalog) chartOfAccountsTables(definition ChartOfAccountsDefinition) (schemadiff.Table, []schemadiff.Table, error) {
	tableName, err := PhysicalCatalogTable(definition.ID)
	if err != nil {
		return schemadiff.Table{}, nil, err
	}
	orderLength := orderColumnWidth(definition)
	table := schemadiff.Table{
		Name: tableName,
		Columns: []schemadiff.Column{
			{Name: "ref", Type: "uuid", Nullable: false},
			{Name: "version", Type: "bigint", Nullable: false, Default: "1"},
			{Name: "description", Type: fmt.Sprintf("character varying(%d)", definition.DescriptionLength), Nullable: false, Default: "''::character varying"},
			{Name: "account_kind", Type: "character varying(16)", Nullable: false, Default: "'active'::character varying"},
			{Name: "off_balance", Type: "boolean", Nullable: false, Default: "false"},
			{Name: "deletion_mark", Type: "boolean", Nullable: false, Default: "false"},
			{Name: "predefined_name", Type: "character varying(128)", Nullable: true},
		},
		Constraints: []schemadiff.Constraint{
			{Name: physicalObjectName("pk", definition.ID), Type: "primary_key", Definition: "PRIMARY KEY (ref)"},
			{Name: physicalObjectName("up", definition.ID), Type: "unique", Definition: "UNIQUE (predefined_name)"},
			{Name: physicalObjectName("ct", definition.ID), Type: "check",
				Definition: "CHECK (account_kind IN ('active', 'passive', 'active-passive'))"},
		},
		Indexes: []schemadiff.Index{
			{Name: physicalObjectName("im", definition.ID), Method: "btree", Keys: []string{"deletion_mark"}},
		},
	}
	appendCodeColumn(&table, definition.ID, definition.Code)
	// The order is derived from the code and the mask, so it is stored rather
	// than computed on every read: it is what the list sorts by. With neither
	// an order nor a code there is nothing to order by, and no column.
	if orderLength > 0 {
		table.Columns = append(table.Columns, schemadiff.Column{Name: "account_order", Type: fmt.Sprintf("character varying(%d)", orderLength), Nullable: false, Default: "''::character varying"})
		table.Indexes = append(table.Indexes, schemadiff.Index{Name: physicalObjectName("id", definition.ID), Method: "btree", Keys: []string{"account_order"}})
	}
	// Every declared flag is a checkbox on every account, so it is a column.
	for _, flag := range definition.AccountingFlags {
		column, err := PhysicalAttributeColumn(flag.ID)
		if err != nil {
			return schemadiff.Table{}, nil, err
		}
		table.Columns = append(table.Columns, schemadiff.Column{Name: column, Type: "boolean", Nullable: false, Default: "false"})
	}
	// A chart of accounts nests always: subaccounts are accounts under an
	// account, and the prototype has no flag to turn that off. There are no
	// folders either - every row is an account.
	appendHierarchyColumns(&table, definition.ID, Hierarchy{Enabled: true, Kind: ItemsHierarchy})
	for _, attribute := range definition.Attributes {
		if err := catalog.appendAttributeSchema(&table, attribute); err != nil {
			return schemadiff.Table{}, nil, fmt.Errorf("chart of accounts %s attribute %s: %w", definition.Name, attribute.Name, err)
		}
	}
	appendListSearchIndexes(&table, definition.ID, definition.List, []string{"Description", "Code"}, definition.Attributes, codeAndDescriptionColumns(definition.Code, definition.DescriptionLength))
	parts, err := catalog.tablePartTables("chart of accounts", definition.Name, definition.ID, definition.TableParts)
	if err != nil {
		return schemadiff.Table{}, nil, err
	}
	if definition.ExtDimensionTypes != nil {
		analytics, err := catalog.extDimensionTable(definition, tableName)
		if err != nil {
			return schemadiff.Table{}, nil, err
		}
		parts = append(parts, analytics)
	}
	return table, parts, nil
}

// extDimensionTable is the standard tabular section of analytics: which kinds
// of analytics an account carries, whether each is kept for turnovers only,
// and the per-analytics flags. The flags sit here rather than on the account
// because quantity may be kept by goods and not by warehouse on one account.
func (catalog *Catalog) extDimensionTable(definition ChartOfAccountsDefinition, ownerTable string) (schemadiff.Table, error) {
	characteristics, err := schemadiff.TableName(*definition.ExtDimensionTypes)
	if err != nil {
		return schemadiff.Table{}, err
	}
	table := schemadiff.Table{
		Name: extDimensionTableName(definition.ID),
		Columns: []schemadiff.Column{
			{Name: "owner_ref", Type: "uuid", Nullable: false},
			{Name: "line_no", Type: "integer", Nullable: false},
			{Name: "ext_dimension_type", Type: "uuid", Nullable: false},
			{Name: "turnover_only", Type: "boolean", Nullable: false, Default: "false"},
		},
		Constraints: []schemadiff.Constraint{
			{Name: physicalObjectName("pt", definition.ID), Type: "primary_key", Definition: "PRIMARY KEY (owner_ref, line_no)"},
			{Name: physicalObjectName("fk", definition.ID), Type: "foreign_key",
				Definition: "FOREIGN KEY (owner_ref) REFERENCES " + schemadiff.ApplicationSchema + "." + ownerTable + "(ref) ON DELETE CASCADE"},
			{Name: physicalObjectName("fd", definition.ID), Type: "foreign_key",
				Definition: "FOREIGN KEY (ext_dimension_type) REFERENCES " + schemadiff.ApplicationSchema + "." + characteristics + "(ref) DEFERRABLE INITIALLY DEFERRED"},
			{Name: physicalObjectName("ur", definition.ID), Type: "unique", Definition: "UNIQUE (owner_ref, ext_dimension_type)"},
		},
	}
	for _, flag := range definition.ExtDimensionAccountingFlags {
		column, err := PhysicalAttributeColumn(flag.ID)
		if err != nil {
			return schemadiff.Table{}, err
		}
		table.Columns = append(table.Columns, schemadiff.Column{Name: column, Type: "boolean", Nullable: false, Default: "false"})
	}
	return table, nil
}
