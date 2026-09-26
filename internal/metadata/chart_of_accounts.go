package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"
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
type AccountingFlag struct {
	ID    uuid.UUID     `yaml:"id" json:"id"`
	Name  string        `yaml:"name" json:"name"`
	Title LocalizedText `yaml:"title" json:"title"`
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
	Parent        string                          `yaml:"parent,omitempty" json:"parent,omitempty"`
	Flags         map[string]bool                 `yaml:"flags,omitempty" json:"flags,omitempty"`
	ExtDimensions []PredefinedAccountExtDimension `yaml:"ext_dimensions,omitempty" json:"extDimensions,omitempty"`
}

// ChartOfAccountsDefinition describes a chart of accounts: the accounts an
// application keeps its books on, what analytics each of them carries, and the
// flags a bookkeeping register checks its movements against.
type ChartOfAccountsDefinition struct {
	Format            int           `yaml:"format" json:"format"`
	ID                uuid.UUID     `yaml:"id" json:"id"`
	Name              string        `yaml:"name" json:"name"`
	Title             LocalizedText `yaml:"title" json:"title"`
	Code              CatalogCode   `yaml:"code" json:"code"`
	DescriptionLength int           `yaml:"description_length" json:"descriptionLength"`
	// CodeMask describes how an account code is built - "@@.@@" is two
	// positions, a dot, two more. The order of accounts is derived from it.
	CodeMask string `yaml:"code_mask,omitempty" json:"codeMask,omitempty"`
	// OrderLength is the width of the derived order string.
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
	Forms                       ObjectForms            `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands                    []ObjectCommand        `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates                   []ObjectTemplate       `yaml:"templates,omitempty" json:"templates,omitempty"`
	List                        ListSettings           `yaml:"list,omitempty" json:"list,omitempty"`
	Predefined                  []PredefinedAccount    `yaml:"predefined,omitempty" json:"predefined,omitempty"`
}

// maxExtDimensions is the prototype's own ceiling on analytics per account.
const maxExtDimensions = 8

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
	}, configuration)...)
	issues = append(issues, validateAccountingFlags("accounting_flags", value.AccountingFlags, configuration)...)
	issues = append(issues, validateAccountingFlags("ext_dimension_accounting_flags", value.ExtDimensionAccountingFlags, configuration)...)
	if value.MaxExtDimensionCount < 0 || value.MaxExtDimensionCount > maxExtDimensions {
		issues = append(issues, fmt.Sprintf("max_ext_dimension_count must be 0..%d", maxExtDimensions))
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
	for _, symbol := range value.CodeMask {
		if symbol == '@' || symbol == '.' || symbol == '-' || symbol == '/' || symbol == ' ' {
			continue
		}
		issues = append(issues, "code_mask may only contain @ and separators . - / and a space")
		break
	}
	if value.CodeMask != "" && !strings.ContainsRune(value.CodeMask, '@') {
		issues = append(issues, "code_mask without a single @ describes no code at all")
	}
	if value.OrderLength < 0 || value.OrderLength > 128 {
		issues = append(issues, "order_length must be 0..128")
	}
	if value.CodeMask != "" && value.OrderLength < utf8.RuneCountInString(value.CodeMask) {
		issues = append(issues, "order_length must fit the code mask")
	}
	if value.AutoOrderByCode && value.OrderLength == 0 {
		issues = append(issues, "auto_order_by_code needs an order_length to write the order into")
	}
	return issues
}

func validateAccountingFlags(path string, flags []AccountingFlag, configuration project.Project) []string {
	if len(flags) > 64 {
		return []string{path + " must not contain more than 64 flags"}
	}
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
	}
	return issues
}

func validatePredefinedAccounts(value ChartOfAccountsDefinition) []string {
	if len(value.Predefined) > maxObjectsPerKind {
		return []string{fmt.Sprintf("predefined must not contain more than %d items", maxObjectsPerKind)}
	}
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
		if !validIdentifier(account.Name) || utf8.RuneCountInString(account.Name) > 128 {
			issues = append(issues, prefix+".name must be a valid identifier of at most 128 characters")
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
		if account.Code == "" {
			if !value.Code.Auto {
				issues = append(issues, prefix+".code is required when automatic codes are disabled")
			}
		} else if _, err := normalizeCatalogCode(value.Code, account.Code); err != nil {
			issues = append(issues, prefix+".code is invalid: "+err.Error())
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
	switch strings.ToLower(name) {
	case "порядок", "order", "забалансовый", "offbalance", "вид", "accounttype", "видсчета", "видсчёта":
		return true
	default:
		return reservedCatalogObjectName(name)
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

func splitCodeMask(mask string) ([]int, []rune) {
	var widths []int
	var separators []rune
	width := 0
	for _, symbol := range mask {
		if symbol == '@' {
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
		result[index].Title = cloneTitle(result[index].Title)
	}
	return result
}

func cloneChartOfAccounts(value ChartOfAccountsDefinition) ChartOfAccountsDefinition {
	value.Title = cloneTitle(value.Title)
	value.Attributes = cloneAttributes(value.Attributes)
	value.TableParts = slices.Clone(value.TableParts)
	for index := range value.TableParts {
		value.TableParts[index].Title = cloneTitle(value.TableParts[index].Title)
		value.TableParts[index].Attributes = cloneAttributes(value.TableParts[index].Attributes)
	}
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
	orderLength := definition.OrderLength
	if orderLength == 0 {
		orderLength = definition.Code.Length
	}
	table := schemadiff.Table{
		Name: tableName,
		Columns: []schemadiff.Column{
			{Name: "ref", Type: "uuid", Nullable: false},
			{Name: "version", Type: "bigint", Nullable: false, Default: "1"},
			{Name: "code", Type: codeSQLType(definition.Code), Nullable: false},
			{Name: "description", Type: fmt.Sprintf("character varying(%d)", definition.DescriptionLength), Nullable: false, Default: "''::character varying"},
			// The order is derived from the code and the mask, so it is stored
			// rather than computed on every read: it is what the list sorts by.
			{Name: "account_order", Type: fmt.Sprintf("character varying(%d)", orderLength), Nullable: false, Default: "''::character varying"},
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
			{Name: physicalObjectName("id", definition.ID), Method: "btree", Keys: []string{"account_order"}},
		},
	}
	if definition.Code.Unique {
		table.Constraints = append(table.Constraints, schemadiff.Constraint{Name: physicalObjectName("uq", definition.ID), Type: "unique", Definition: "UNIQUE (code)"})
	} else {
		table.Indexes = append(table.Indexes, schemadiff.Index{Name: physicalObjectName("ic", definition.ID), Method: "btree", Keys: []string{"code"}})
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
	appendHierarchyColumns(&table, definition.ID, Hierarchy{Enabled: true, Kind: ItemsHierarchy, Series: SubordinationSeries})
	for _, attribute := range definition.Attributes {
		if err := catalog.appendAttributeSchema(&table, attribute); err != nil {
			return schemadiff.Table{}, nil, fmt.Errorf("chart of accounts %s attribute %s: %w", definition.Name, attribute.Name, err)
		}
	}
	appendListSearchIndexes(&table, definition.ID, definition.List, []string{"Description", "Code"}, definition.Attributes, map[string]listColumn{
		"code": {name: "code", kind: definition.Code.Type}, "description": {name: "description", kind: StringType},
	})
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
