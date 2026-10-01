package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Beyond a catalog, a chart of characteristic types and a document, the owner
// checked in the designer on 01.10.2026 which kinds take a length of 0: the
// code of a chart of accounts and of a chart of calculation types, and the
// number of a business process and of a task, may be 0; the code of an exchange
// plan may not. A kind not checked keeps what it had. The defect on one side is
// a configuration the designer saves refused on import, on the other a column
// of no width that the database will not build.
func TestTheOtherKindsTakeALengthOfZeroAsTheDesignerDoes(t *testing.T) {
	t.Parallel()
	configuration := metadataConfiguration()
	for name, testCase := range map[string]struct {
		decode  func() error
		refused string
	}{
		"код плана счетов": {func() error {
			_, err := DecodeChartOfAccounts("object.yaml", strings.NewReader("format: 1\nid: "+accountsID+"\nname: Основной\ntitle: {ru: Основной}\ncode: {type: string, length: 0}\ndescription_length: 100\n"), configuration)
			return err
		}, ""},
		"код плана видов расчёта": {func() error {
			_, err := DecodeChartOfCalculationTypes("object.yaml", strings.NewReader("format: 1\nid: "+calcTypesStandardID+"\nname: Начисления\ntitle: {ru: Начисления}\ncode: {type: string, length: 0}\ndescription_length: 100\n"), configuration)
			return err
		}, ""},
		"номер бизнес-процесса": {func() error {
			_, err := DecodeBusinessProcess("object.yaml", strings.NewReader("format: 1\nid: "+businessProcessID+"\nname: Задание\ntitle: {ru: Задание}\nnumber: {type: string, length: 0, periodicity: none}\n"), configuration)
			return err
		}, ""},
		"номер задачи": {func() error {
			_, err := DecodeTask("object.yaml", strings.NewReader("format: 1\nid: "+taskID+"\nname: Задача\ntitle: {ru: Задача}\nnumber: {type: string, length: 0, periodicity: none}\ndescription_length: 150\n"), configuration)
			return err
		}, ""},
		"код плана обмена": {func() error {
			_, err := DecodeExchangePlan("object.yaml", strings.NewReader("format: 1\nid: "+exchangePlanID+"\nname: Филиалы\ntitle: {ru: Филиалы}\ncode: {type: string, length: 0}\ndescription_length: 100\n"), configuration)
			return err
		}, "code.length must be 1..50"},
		// The description of a chart of accounts was not checked, and stays.
		"наименование плана счетов": {func() error {
			_, err := DecodeChartOfAccounts("object.yaml", strings.NewReader("format: 1\nid: "+accountsID+"\nname: Основной\ntitle: {ru: Основной}\ncode: {type: string, length: 5}\ndescription_length: 0\n"), configuration)
			return err
		}, "description_length must be 1..150"},
		"код предопределённого счёта": {func() error {
			_, err := DecodeChartOfAccounts("object.yaml", strings.NewReader("format: 1\nid: "+accountsID+"\nname: Основной\ntitle: {ru: Основной}\ncode: {type: string, length: 0}\ndescription_length: 100\n"+
				"predefined:\n  - {id: 81000000-0000-4000-8000-000000000001, name: Касса, code: \"50\", kind: active}\n"), configuration)
			return err
		}, "code is switched off"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := testCase.decode()
			if testCase.refused == "" {
				if err != nil {
					t.Fatalf("a length of 0 the designer takes was refused: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), testCase.refused) {
				t.Fatalf("error = %v, expected it to say %q", err, testCase.refused)
			}
		})
	}
}

func TestTheOtherKindsBuildNoColumnForWhatIsSwitchedOff(t *testing.T) {
	t.Parallel()
	accounts, calc, process, task := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{
		ChartsOfAccounts:         []ChartOfAccountsDefinition{{ID: accounts, Name: "Основной", Code: CatalogCode{Type: StringType}, DescriptionLength: 100}},
		ChartsOfCalculationTypes: []ChartOfCalculationTypesDefinition{{ID: calc, Name: "Начисления", Code: CatalogCode{Type: StringType}, DescriptionLength: 100}},
		BusinessProcesses:        []BusinessProcessDefinition{{ID: process, Name: "Задание", Number: DocumentNumber{Type: StringType, Periodicity: NumberPeriodNone}}},
		Tasks:                    []TaskDefinition{{ID: task, Name: "Задача", Number: DocumentNumber{Type: StringType, Periodicity: NumberPeriodNone}, DescriptionLength: 150}},
		chartOfAccountsByName:    map[string]int{"основной": 0}, chartOfAccountsByID: map[uuid.UUID]int{accounts: 0},
		chartOfCalculationTypesByName: map[string]int{"начисления": 0}, chartOfCalculationTypesByID: map[uuid.UUID]int{calc: 0},
		businessProcessByName: map[string]int{"задание": 0}, businessProcessByID: map[uuid.UUID]int{process: 0},
		taskByName: map[string]int{"задача": 0}, taskByID: map[uuid.UUID]int{task: 0},
	}
	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatalf("the schema refused kinds with a switched off field: %v", err)
	}
	tableOf := func(id uuid.UUID) schemadiff.Table {
		name, _ := schemadiff.TableName(id)
		for _, table := range schema.Tables {
			if table.Name == name {
				return table
			}
		}
		t.Fatalf("no table for %s", id)
		return schemadiff.Table{}
	}
	for label, check := range map[string]struct {
		id      uuid.UUID
		columns []string
	}{
		"план счетов":        {accounts, []string{"code", "account_order"}},
		"план видов расчёта": {calc, []string{"code"}},
		"бизнес-процесс":     {process, []string{"number"}},
		"задача":             {task, []string{"number"}},
	} {
		table := tableOf(check.id)
		for _, column := range table.Columns {
			for _, absent := range check.columns {
				if column.Name == absent {
					t.Fatalf("%s got a column %s", label, absent)
				}
			}
		}
	}
}
