package metadata

import (
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
code: {type: string, length: 5, auto: true}
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
	for _, column := range []string{"ext_dimension_type", "turnover_only", extFlagColumn} {
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
code: {type: string, length: 5, auto: true}
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
