package metadata

import (
	"testing"
)

const (
	entriesChart     = "acc00000-0000-4000-8000-000000000001"
	entriesDocument  = "acc00000-0000-4000-8000-000000000002"
	entriesRegister  = "acc00000-0000-4000-8000-000000000003"
	entriesKinds     = "acc00000-0000-4000-8000-000000000004"
	entriesCompanies = "acc00000-0000-4000-8000-000000000005"

	entriesFlag     = "acc00000-0000-4000-8000-000000000010"
	entriesExtFlag  = "acc00000-0000-4000-8000-000000000011"
	entriesCompany  = "acc00000-0000-4000-8000-000000000012"
	entriesCurrency = "acc00000-0000-4000-8000-000000000013"
	entriesSum      = "acc00000-0000-4000-8000-000000000014"
	entriesFxSum    = "acc00000-0000-4000-8000-000000000015"
)

func entriesProject(t *testing.T, correspondence bool, fields string) string {
	t.Helper()
	root := metadataProject(t)
	catalogNamed(t, root, entriesCompanies, "Организации")
	writeMetadata(t, root, ChartOfCharacteristicTypesKind, entriesKinds, `format: 1
id: `+entriesKinds+`
name: ВидыСубконто
title: {ru: Виды субконто}
code: {type: string, length: 9, auto: true}
description_length: 100
value_type: [{kind: catalog, reference: `+entriesCompanies+`}]
`)
	writeMetadata(t, root, ChartOfAccountsKind, entriesChart, `format: 1
id: `+entriesChart+`
name: Основной
title: {ru: Основной}
code: {type: string, length: 9, auto: false}
description_length: 100
ext_dimension_types: `+entriesKinds+`
max_ext_dimension_count: 2
accounting_flags:
  - {id: `+entriesFlag+`, name: Валютный, title: {ru: Валютный}}
ext_dimension_accounting_flags:
  - {id: `+entriesExtFlag+`, name: Суммовой, title: {ru: Суммовой}}
`)
	writeMetadata(t, root, DocumentKind, entriesDocument, `format: 1
id: `+entriesDocument+`
name: Операция
title: {ru: Операция}
number: {type: string, length: 9, auto: true, periodicity: none}
posting: true
`)
	correspondenceLine := "correspondence: false"
	if correspondence {
		correspondenceLine = "correspondence: true"
	}
	writeMetadata(t, root, AccountingRegisterKind, entriesRegister, `format: 1
id: `+entriesRegister+`
name: Хозрасчетный
title: {ru: Хозрасчётный}
chart_of_accounts: `+entriesChart+`
`+correspondenceLine+`
recorders: [`+entriesDocument+`]
`+fields+`
`)
	return root
}

const entriesStandardFields = `dimensions:
  - {id: ` + entriesCompany + `, name: Организация, title: {ru: Организация}, types: [{kind: catalog, reference: ` + entriesCompanies + `}], balance: true, indexing: index}
  - {id: ` + entriesCurrency + `, name: Валюта, title: {ru: Валюта}, types: [{kind: catalog, reference: ` + entriesCompanies + `}], accounting_flag: ` + entriesFlag + `}
resources:
  - {id: ` + entriesSum + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}], balance: true}
  - {id: ` + entriesFxSum + `, name: ВалютнаяСумма, title: {ru: Валютная сумма}, types: [{kind: number, precision: 15, scale: 2}], ext_dimension_accounting_flag: ` + entriesExtFlag + `}`

// Under double entry a field that is the same on both sides is kept once and a
// field that is not is kept twice. That difference is the whole of what the
// balance flag means, and it has to show up in the table or it means nothing.
func TestAccountingRegisterKeepsBothSidesOfAnEntry(t *testing.T) {
	t.Parallel()
	root := entriesProject(t, true, entriesStandardFields)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	register, ok := catalog.AccountingRegister("Хозрасчетный")
	if !ok {
		t.Fatal("the accounting register did not load")
	}
	if !register.Correspondence || register.ChartOfAccounts.IsZero() {
		t.Fatalf("the register lost what it makes entries against: %+v", register)
	}
	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	table, err := PhysicalAccountingRegisterTable(register.ID)
	if err != nil {
		t.Fatal(err)
	}
	columns := map[string]bool{}
	for _, item := range schema.Tables {
		if item.Name != table {
			continue
		}
		for _, column := range item.Columns {
			columns[column.Name] = true
		}
	}
	if len(columns) == 0 {
		t.Fatal("the register has no table of entries")
	}
	// Two accounts, because an entry under double entry has two sides.
	for _, column := range []string{"account_dr", "account_cr"} {
		if !columns[column] {
			t.Fatalf("the entry has no %s", column)
		}
	}
	if columns["account"] {
		t.Fatal("a register with correspondence must not keep a single account beside its two")
	}
	// A balance field once; a non-balance field on each side.
	balance, err := PhysicalAttributeColumn(mustUUID(t, entriesCompany))
	if err != nil {
		t.Fatal(err)
	}
	if !columns[balance] {
		t.Fatal("a balance dimension lost its single column")
	}
	for _, debit := range []bool{true, false} {
		name, err := sideColumn(mustUUID(t, entriesCurrency), debit)
		if err != nil {
			t.Fatal(err)
		}
		if !columns[name] {
			t.Fatalf("a non-balance dimension is missing the column of one side: %s", name)
		}
	}
	// Analytics come from the chart: two slots, each per side, each holding
	// which kind of analytics it is and the value.
	for _, column := range []string{"ext1_dr_kind", "ext1_dr_value", "ext2_cr_kind", "ext2_cr_value"} {
		if !columns[column] {
			t.Fatalf("the entry has no %s", column)
		}
	}
}

// Without correspondence an entry has one side, so there is one account, every
// field is kept once, and the analytics are not doubled either.
func TestAccountingRegisterWithoutCorrespondenceHasOneSide(t *testing.T) {
	t.Parallel()
	root := entriesProject(t, false, `resources:
  - {id: `+entriesSum+`, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	register, _ := catalog.AccountingRegister("Хозрасчетный")
	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	table, err := PhysicalAccountingRegisterTable(register.ID)
	if err != nil {
		t.Fatal(err)
	}
	columns := map[string]bool{}
	for _, item := range schema.Tables {
		if item.Name == table {
			for _, column := range item.Columns {
				columns[column.Name] = true
			}
		}
	}
	if !columns["account"] {
		t.Fatal("an entry on one side still names an account")
	}
	for _, column := range []string{"account_dr", "account_cr", "ext1_dr_kind"} {
		if columns[column] {
			t.Fatalf("an entry on one side must not carry %s", column)
		}
	}
	if !columns["ext1_kind"] || !columns["ext2_value"] {
		t.Fatal("analytics are carried whether or not there are two sides")
	}
}

// Each of these would leave a column that exists and is never filled, or a
// setting that reads as working and does nothing.
func TestAccountingRegisterRefusesWhatWouldStayEmpty(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct {
		correspondence bool
		fields         string
	}{
		"балансовый без корреспонденции": {false, `resources:
  - {id: ` + entriesSum + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}], balance: true}`},
		"признак учёта чужого плана": {true, `resources:
  - {id: ` + entriesSum + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}], accounting_flag: ` + entriesExtFlag + `}`},
		"признак учёта субконто чужого плана": {true, `resources:
  - {id: ` + entriesSum + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}], ext_dimension_accounting_flag: ` + entriesFlag + `}`},
		"без ресурсов": {true, `dimensions:
  - {id: ` + entriesCompany + `, name: Организация, title: {ru: Организация}, types: [{kind: catalog, reference: ` + entriesCompanies + `}]}`},
		"имя занято стандартным полем": {true, `resources:
  - {id: ` + entriesSum + `, name: Период, title: {ru: Период}, types: [{kind: number, precision: 15, scale: 2}]}`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := entriesProject(t, broken.correspondence, broken.fields)
			if _, err := Load(root); err == nil {
				t.Fatal("a register that would keep an empty column was accepted")
			}
		})
	}
}
