package metadata

import (
	"context"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A name the configuration may give its own field is free unless the
// prototype keeps it. The names kept are the prototype's - ВерсияДанных on
// the eight reference kinds, СчетДт and СчетКт of an entry with
// correspondence - and every name that was ours is free: Версия (6/6/6
// catalogs of the test configurations), ГлавнаяЗадача (a business process of
// erp), the keys of rights and policies, the spellings a developer might reach
// for.
//
// Defect caught: a configuration refused over a name only ML keeps, and a
// name of the prototype let through, so that a field shadows the standard one.
func TestServiceNamesLeaveTheConfigurationsNamesFree(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		kind     string
		reserved func(string) bool
		free     []string
		kept     []string
	}{
		{"catalog", reservedCatalogObjectName, []string{"Версия", "Version"}, []string{"ВерсияДанных", "DataVersion"}},
		{"document", reservedDocumentObjectName, []string{"Версия", "Version"}, []string{"ВерсияДанных", "DataVersion"}},
		{"chart of characteristic types", reservedChartOfCharacteristicTypesName, []string{"Версия"}, []string{"ВерсияДанных"}},
		{"chart of accounts", reservedChartOfAccountsName, []string{"Версия", "ВидСчета", "AccountType"}, []string{"ВерсияДанных"}},
		{"chart of calculation types", reservedCalculationTypeName, []string{"Версия", "ActionPeriodIsBase"}, []string{"ВерсияДанных"}},
		{"exchange plan", reservedExchangePlanName, []string{"Версия"}, []string{"ВерсияДанных"}},
		{"business process", reservedBusinessProcessName, []string{"Версия", "ГлавнаяЗадача"}, []string{"ВерсияДанных"}},
		{"task", reservedTaskName, []string{"Версия"}, []string{"ВерсияДанных"}},
		{"information register", reservedInformationRegisterName, []string{"RecordID", "ИдентификаторЗаписи"}, nil},
		{"accumulation register", reservedAccumulationRegisterName, []string{"RecordID", "MovementKind"}, nil},
		{"accounting register", reservedAccountingRegisterName, []string{"RecordID"}, []string{"СчетДт", "AccountCr"}},
		{"calculation register", reservedCalculationRegisterName,
			[]string{"RecordID", "Период", "Period", "ActionPeriodStart", "BasePeriodEnd", "Reversing"}, nil},
	} {
		for _, name := range test.free {
			if test.reserved(name) {
				t.Errorf("%s: %s is kept, and only ML keeps it", test.kind, name)
			}
		}
		for _, name := range test.kept {
			if !test.reserved(name) {
				t.Errorf("%s: %s is free, and the prototype keeps it", test.kind, name)
			}
		}
	}

	body := func(name string) string {
		return "format: 1\nid: " + uuid.MustNew().String() + "\nname: ВнешниеКомпоненты\ntitle: {ru: Внешние компоненты}\n" +
			"code: {type: string, length: 9}\ndescription_length: 150\nattributes:\n" +
			"  - {id: " + uuid.MustNew().String() + ", name: " + name + ", title: {ru: Поле}, types: [{kind: string, length: 10}]}\n"
	}
	if _, err := DecodeCatalog("object.yaml", strings.NewReader(body("Версия")), metadataConfiguration()); err != nil {
		t.Fatalf("an attribute named Версия, as in erp, refused: %v", err)
	}
	if _, err := DecodeCatalog("object.yaml", strings.NewReader(body("ВерсияДанных")), metadataConfiguration()); err == nil || !strings.Contains(err.Error(), "is reserved") {
		t.Fatalf("an attribute named ВерсияДанных: %v", err)
	}
}

// The version of an object's data is ВерсияДанных and DataVersion, a string
// as in the prototype, and read-only; the old name Версия is gone.
//
// Defect caught: the property left under our name, so that code moved from
// the prototype reads nothing; the version given as a number, so that code
// comparing it with a saved string never matches; and the version writable.
func TestDataVersionIsThePrototypesNameAndAString(t *testing.T) {
	t.Parallel()
	definition := CatalogDefinition{
		ID: uuid.MustNew(), Name: "Товары", Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 250,
	}
	catalog := &Catalog{Catalogs: []CatalogDefinition{definition}}
	catalog.catalogByName = map[string]int{"товары": 0}
	catalog.catalogByID = map[uuid.UUID]int{definition.ID: 0}
	run := func(t *testing.T, body string) (*CatalogRecord, error) {
		t.Helper()
		runtime := &Runtime{catalog: catalog, events: make(map[uuid.UUID]CatalogEventHandler)}
		program, diagnostics := CompileCatalogObjectModule(definition, "object.bsl", "&НаСервере\nПроцедура ОбработкаЗаполнения(ДанныеЗаполнения, СтандартнаяОбработка)\n"+body+"\nКонецПроцедуры")
		if len(diagnostics) != 0 {
			t.Fatal(diagnostics)
		}
		machine, err := vm.New(program)
		if err != nil {
			t.Fatal(err)
		}
		handler, err := NewCatalogBSLEvents(runtime, machine.NewContextWithMetadata(runtime), definition)
		if err != nil {
			t.Fatal(err)
		}
		record := &CatalogRecord{
			Reference: CatalogReference{CatalogID: definition.ID, ObjectID: uuid.MustNew()},
			Version:   7, Attributes: map[uuid.UUID]Value{}, TableParts: map[uuid.UUID][]CatalogRow{},
		}
		_, err = handler.HandleCatalogEvent(context.Background(), CatalogEventFill, record)
		return record, err
	}
	record, err := run(t, `Если ЭтотОбъект.ВерсияДанных = "7" И ThisObject.DataVersion <> 7 Тогда
    ЭтотОбъект.Наименование = "строка " + ЭтотОбъект.ВерсияДанных;
КонецЕсли;`)
	if err != nil || record.Description != "строка 7" {
		t.Fatalf("ВерсияДанных is not the string 7: description %q, error %v", record.Description, err)
	}
	if _, err := run(t, `ЭтотОбъект.Наименование = ЭтотОбъект.Версия;`); err == nil {
		t.Fatal("the old name Версия still reads the version")
	}
	if _, err := run(t, `ЭтотОбъект.ВерсияДанных = "8";`); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("ВерсияДанных is writable: %v", err)
	}
}
