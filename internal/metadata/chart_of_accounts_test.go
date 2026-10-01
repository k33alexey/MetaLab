package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const accountsChartYAML = `format: 1
id: ` + characteristicsID + `
name: ВидыСубконто
title: {ru: Виды субконто}
code: {type: string, length: 9, auto: true}
description_length: 100
value_type: [{kind: string, length: 100}]
predefined:
  - id: 70000000-0000-4000-8000-000000000010
    name: Контрагенты
    code: "000000001"
    description: Контрагенты
  - id: 70000000-0000-4000-8000-000000000011
    name: Склады
    code: "000000002"
    description: Склады
`

// A chart of accounts is the accounts, their flags and their analytics. All
// three have to survive the round trip: an account without its flags or its
// subconto is a name and a number, and the books built on it mean nothing.
func TestLoadChartOfAccountsWithFlagsAndAnalytics(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, ChartOfCharacteristicTypesKind, characteristicsID, accountsChartYAML)
	writeMetadata(t, root, ChartOfAccountsKind, accountsID, `format: 1
id: `+accountsID+`
name: Основной
title: {ru: Основной}
code: {type: string, length: 5, auto: false}
description_length: 120
code_mask: "@@.@@"
order_length: 5
auto_order_by_code: true
ext_dimension_types: `+characteristicsID+`
max_ext_dimension_count: 3
accounting_flags:
  - id: `+accountFlagID+`
    name: Количественный
    title: {ru: Количественный}
ext_dimension_accounting_flags:
  - id: `+extDimensionFlagID+`
    name: Суммовой
    title: {ru: Суммовой}
predefined:
  - id: 80000000-0000-4000-8000-000000000010
    name: Товары
    code: "41"
    description: Товары
    kind: active
    flags: {Количественный: true}
    ext_dimensions:
      - characteristic: Склады
        flags: {Суммовой: true}
  - id: 80000000-0000-4000-8000-000000000011
    name: ТоварыНаСкладе
    code: "41.01"
    description: Товары на складе
    kind: active
    parent: Товары
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	chart, ok := catalog.ChartOfAccounts("Основной")
	if !ok {
		t.Fatal("the chart of accounts did not load")
	}
	if len(chart.AccountingFlags) != 1 || len(chart.ExtDimensionAccountingFlags) != 1 {
		t.Fatalf("flags = %+v / %+v", chart.AccountingFlags, chart.ExtDimensionAccountingFlags)
	}
	if len(chart.Predefined) != 2 {
		t.Fatalf("predefined accounts = %d, want 2", len(chart.Predefined))
	}
	goods := chart.Predefined[0]
	if goods.Kind != ActiveAccount || !goods.Flags["Количественный"] {
		t.Fatalf("the account lost its kind or its flag: %+v", goods)
	}
	if len(goods.ExtDimensions) != 1 || goods.ExtDimensions[0].Characteristic != "Склады" || !goods.ExtDimensions[0].Flags["Суммовой"] {
		t.Fatalf("the account lost its analytics: %+v", goods.ExtDimensions)
	}
	if chart.Predefined[1].Parent != "Товары" {
		t.Fatalf("a subaccount lost its place: %+v", chart.Predefined[1])
	}
}

// Storage carries what the accounting means: the derived order, what the
// balance means, whether the account is off balance, a column per declared flag
// and a table of analytics whose flags sit per line, not per account.
func TestChartOfAccountsStorageCarriesFlagsAndAnalytics(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, ChartOfCharacteristicTypesKind, characteristicsID, accountsChartYAML)
	writeMetadata(t, root, ChartOfAccountsKind, accountsID, `format: 1
id: `+accountsID+`
name: Основной
title: {ru: Основной}
code: {type: string, length: 5}
description_length: 120
code_mask: "@@.@@"
order_length: 5
ext_dimension_types: `+characteristicsID+`
max_ext_dimension_count: 2
accounting_flags:
  - id: `+accountFlagID+`
    name: Валютный
    title: {ru: Валютный}
ext_dimension_accounting_flags:
  - id: `+extDimensionFlagID+`
    name: Количественный
    title: {ru: Количественный}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	accountTable, err := PhysicalCatalogTable(catalog.ChartsOfAccounts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	flagColumn, err := PhysicalAttributeColumn(catalog.ChartsOfAccounts[0].AccountingFlags[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	extFlagColumn, err := PhysicalAttributeColumn(catalog.ChartsOfAccounts[0].ExtDimensionAccountingFlags[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	analyticsTable := extDimensionTableName(catalog.ChartsOfAccounts[0].ID)
	var accountColumns, analyticsColumns map[string]bool
	for _, table := range schema.Tables {
		columns := map[string]bool{}
		for _, column := range table.Columns {
			columns[column.Name] = true
		}
		switch table.Name {
		case accountTable:
			accountColumns = columns
		case analyticsTable:
			analyticsColumns = columns
		}
	}
	if accountColumns == nil || analyticsColumns == nil {
		t.Fatal("the chart of accounts is missing its own table or its analytics table")
	}
	for _, column := range []string{"account_order", "account_kind", "off_balance", flagColumn} {
		if !accountColumns[column] {
			t.Fatalf("the account table has no %s", column)
		}
	}
	// "predefined" is the standard field of every line of analytics: the
	// prototype tells a kind of analytics the configuration gave the account
	// from one the user added, and without the column the two are one.
	for _, column := range []string{"ext_dimension_type", "turnover_only", "predefined", extFlagColumn} {
		if !analyticsColumns[column] {
			t.Fatalf("the analytics table has no %s", column)
		}
	}
	if accountColumns[extFlagColumn] {
		t.Fatal("an analytics flag must sit per line of analytics, not on the account")
	}
	names := catalog.PhysicalNames()
	if names[analyticsTable] == "" || names[flagColumn] == "" {
		t.Fatal("the migration dialog would name the analytics table or a flag by its UUID")
	}
}

// Nothing may point at accounting that is not declared: analytics of a kind
// nobody declared, a flag nobody declared, or a parent account that is not in
// the chart - each is a hole the books would never report.
func TestChartOfAccountsRefusesUndeclaredAccounting(t *testing.T) {
	t.Parallel()
	for name, predefined := range map[string]string{
		"неизвестный вид субконто": `
predefined:
  - id: 80000000-0000-4000-8000-000000000010
    name: Товары
    code: "41"
    kind: active
    ext_dimensions:
      - characteristic: Договоры
`,
		"неизвестный признак учёта": `
predefined:
  - id: 80000000-0000-4000-8000-000000000010
    name: Товары
    code: "41"
    kind: active
    flags: {Валютный: true}
`,
		"неизвестный родитель": `
predefined:
  - id: 80000000-0000-4000-8000-000000000010
    name: Товары
    code: "41"
    kind: active
    parent: Материалы
`,
		"неизвестный вид счёта": `
predefined:
  - id: 80000000-0000-4000-8000-000000000010
    name: Товары
    code: "41"
    kind: смешанный
`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, ChartOfCharacteristicTypesKind, characteristicsID, accountsChartYAML)
			writeMetadata(t, root, ChartOfAccountsKind, accountsID, `format: 1
id: `+accountsID+`
name: Основной
title: {ru: Основной}
code: {type: string, length: 5, auto: false}
description_length: 120
ext_dimension_types: `+characteristicsID+`
max_ext_dimension_count: 2
accounting_flags:
  - id: `+accountFlagID+`
    name: Количественный
    title: {ru: Количественный}
`+predefined)
			if _, err := Load(root); err == nil {
				t.Fatal("undeclared accounting was accepted")
			}
		})
	}
}

// The order of accounts is the code laid into the mask: fragments pushed right,
// gaps filled with spaces. Ordering by the code itself would put 41.1 before 9.
func TestAccountOrderFollowsTheCodeMask(t *testing.T) {
	t.Parallel()
	for _, item := range []struct{ mask, code, want string }{
		{"@@@.@@@", "41.1", " 41.  1"},
		{"@@@.@@@", "9", "  9.   "},
		{"@@.@@", "41.01", "41.01"},
		{"", "41.1", "41.1"},
	} {
		got := AccountCodeOrder(item.mask, item.code, 0)
		if got != item.want {
			t.Fatalf("order of %q under %q = %q, want %q", item.code, item.mask, got, item.want)
		}
	}
	if got := AccountCodeOrder("@@@.@@@", "41.1", 4); got != " 41." {
		t.Fatalf("a short order must be cut to its length, got %q", got)
	}
	if got := AccountCodeOrder("@@", "41", 5); !strings.HasPrefix(got, "41") || len(got) != 5 {
		t.Fatalf("a long order must be padded to its length, got %q", got)
	}
}

// A role must be able to reach the new kinds. Rights are resolved per kind, so
// a kind the resolver does not know is not merely invisible in the editor: a
// role that grants anything on it refuses to load at all, and the whole project
// goes down with it rather than one line of the import report.
func TestRightsReachChartsOfAccountsAndCharacteristics(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, ChartOfCharacteristicTypesKind, characteristicsID, accountsChartYAML)
	writeMetadata(t, root, ChartOfAccountsKind, accountsID, `format: 1
id: `+accountsID+`
name: Основной
title: {ru: Основной}
code: {type: string, length: 5}
description_length: 120
ext_dimension_types: `+characteristicsID+`
max_ext_dimension_count: 2
accounting_flags:
  - id: `+accountFlagID+`
    name: Количественный
    title: {ru: Количественный}
`)
	if err := os.MkdirAll(filepath.Join(root, "metadata", "roles"), 0o755); err != nil {
		t.Fatal(err)
	}
	roleID := "90000000-0000-4000-8000-000000000001"
	if err := os.WriteFile(filepath.Join(root, "metadata", "roles", roleID+".yaml"), []byte(`format: 1
id: `+roleID+`
name: Бухгалтер
title: {ru: Бухгалтер}
objects:
  - object: `+accountsID+`
    operations: [read, create, update]
    fields:
      - field: `+accountFlagID+`
        operations: [read]
      - field: accountkind
        operations: [read, update]
  - object: `+characteristicsID+`
    operations: [read]
    fields:
      - field: valuetype
        operations: [read]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err != nil {
		t.Fatalf("a role granting rights on the new kinds was refused: %v", err)
	}
	schema, err := LoadPermissionSchema(root)
	if err != nil {
		t.Fatal(err)
	}
	var accounts, characteristics *PermissionObject
	for index := range schema.Objects {
		switch schema.Objects[index].Kind {
		case ChartOfAccountsKind:
			accounts = &schema.Objects[index]
		case ChartOfCharacteristicTypesKind:
			characteristics = &schema.Objects[index]
		}
	}
	if accounts == nil || characteristics == nil {
		t.Fatal("the role editor does not show the new kinds at all")
	}
	var flagField bool
	for _, field := range accounts.Fields {
		if field.Name == "Количественный" {
			flagField = true
		}
	}
	if !flagField {
		t.Fatal("an accounting flag must be a field a role can restrict, named as the application named it")
	}
}

// chartWithMask is a chart of accounts with nothing but a code, a mask and the
// order, and one predefined account.
func chartWithMask(mask string, orderLength int, order string) string {
	return fmt.Sprintf(`format: 1
id: %s
name: Основной
title: {ru: Основной}
code: {type: string, length: 9, auto: false}
description_length: 120
code_mask: %q
order_length: %d
predefined:
  - id: 80000000-0000-4000-8000-000000000010
    name: Забалансовый
    code: "01"
    description: Забалансовый
    kind: active
    order: %q
`, accountsID, mask, orderLength, order)
}

// The mask of the code follows the grammar of an input mask, which is the only
// grammar the help gives a mask: positions ! 9 # N U X ^ h @, any other symbol a
// separator, a backslash escaping, several masks joined by ";". The order
// length does not depend on the mask (checked by the owner on the platform,
// 01.10.2026).
//
// Defect caught: sb's mask «XXXXXXXX» refused (only @ and four separators were
// known), an order shorter than the mask refused, and a mask with a control
// character accepted.
func TestCodeMaskFollowsTheGrammarOfAnInputMask(t *testing.T) {
	t.Parallel()
	for _, mask := range []string{"XXXXXXXX", "@@@@@   ", "@@@@@@@@@", "99.999;@@@", `\@@@`, "NN-UU"} {
		if _, err := DecodeChartOfAccounts("chart.yaml", strings.NewReader(chartWithMask(mask, 9, "")), metadataConfiguration()); err != nil {
			t.Errorf("mask %q refused: %v", mask, err)
		}
	}
	if _, err := DecodeChartOfAccounts("chart.yaml", strings.NewReader(chartWithMask("@@@@@", 3, "")), metadataConfiguration()); err != nil {
		t.Errorf("an order of three under a mask of five refused: %v", err)
	}
	if _, err := DecodeChartOfAccounts("chart.yaml", strings.NewReader(chartWithMask("@@\t@@", 9, "")), metadataConfiguration()); err == nil {
		t.Error("a mask with a control character accepted")
	}

	// Every position counts, not only @; an escaped one is a separator; the
	// first of several masks orders the code.
	for _, item := range []struct{ mask, code, want string }{
		{"XX.XX", "1.2", " 1. 2"},
		{"99-99", "4-1", " 4- 1"},
		{`9\99`, "1", "19 "},
		{"@@.@@;@@@", "1.1", " 1. 1"},
	} {
		if got := AccountCodeOrder(item.mask, item.code, 0); got != item.want {
			t.Errorf("order of %q under %q = %q, want %q", item.code, item.mask, got, item.want)
		}
	}
}

// The order of a predefined account is carried as the configuration wrote it:
// 124 of 966 are not what the code would give - «Заб01» under the code 01, a
// leading space in sb. It is bounded only by the column it lies in.
//
// Defect caught: the order dropped at load, so the bookkeeper's ordering of the
// off-balance accounts is lost; and an order longer than its column accepted,
// which the database would refuse when the account is written.
func TestPredefinedAccountCarriesItsOrder(t *testing.T) {
	t.Parallel()
	value, err := DecodeChartOfAccounts("chart.yaml", strings.NewReader(chartWithMask("@@@@@", 9, "Заб01")), metadataConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	if value.Predefined[0].Order != "Заб01" {
		t.Fatalf("order = %q", value.Predefined[0].Order)
	}
	if _, err := DecodeChartOfAccounts("chart.yaml", strings.NewReader(chartWithMask("@@@@@", 3, "Заб01")), metadataConfiguration()); err == nil {
		t.Fatal("an order longer than its column accepted")
	}
	// With no order length the column is as wide as the code.
	if _, err := DecodeChartOfAccounts("chart.yaml", strings.NewReader(chartWithMask("@@@@@", 0, "0123456789")), metadataConfiguration()); err == nil {
		t.Fatal("an order longer than the code accepted where the order takes the code's width")
	}
}
