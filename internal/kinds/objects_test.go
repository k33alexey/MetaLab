package kinds

import (
	"slices"
	"strings"
	"testing"
)

// TestThisObjectIsTheHelpsType catches a kind whose object, record set or
// value manager is left without its type, a module given a type the help does
// not give it, a type of objects.yaml nobody's module runs around, and a type
// read with a member twice or without itself. The types are the help's
// (8.3.27: the property ЭтотОбъект of each), written here rather than read
// from thisObjects; a form, a command and a common module have ЭтотОбъект of
// another sort and no type here.
func TestThisObjectIsTheHelpsType(t *testing.T) {
	expected := map[Kind]string{
		Catalog: "МодульОбъекта=СправочникОбъект/CatalogObject", Document: "МодульОбъекта=ДокументОбъект/DocumentObject",
		ChartOfCharacteristicTypes: "МодульОбъекта=ПланВидовХарактеристикОбъект/ChartOfCharacteristicTypesObject",
		ChartOfAccounts:            "МодульОбъекта=ПланСчетовОбъект/ChartOfAccountsObject",
		ChartOfCalculationTypes:    "МодульОбъекта=ПланВидовРасчетаОбъект/ChartOfCalculationTypesObject",
		BusinessProcess:            "МодульОбъекта=БизнесПроцессОбъект/BusinessProcessObject",
		Task:                       "МодульОбъекта=ЗадачаОбъект/TaskObject", ExchangePlan: "МодульОбъекта=ПланОбменаОбъект/ExchangePlanObject",
		Report: "МодульОбъекта=ОтчетОбъект/ReportObject", DataProcessor: "МодульОбъекта=ОбработкаОбъект/DataProcessorObject",
		ExternalDataSourceTable: "МодульОбъекта=ВнешнийИсточникДанныхТаблицаОбъект/ExternalDataSourceTableObject " +
			"МодульНабораЗаписей=ВнешнийИсточникДанныхТаблицаНаборЗаписей/ExternalDataSourceTableRecordSet",
		ExternalDimensionTable: "МодульОбъекта=ВнешнийИсточникДанныхКубТаблицаИзмеренияОбъект/ExternalDataSourceCubeDimensionTableObject",
		ExternalCube:           "МодульНабораЗаписей=ВнешнийИсточникДанныхКубНаборЗаписей/ExternalDataSourceCubeRecordSet",
		InformationRegister:    "МодульНабораЗаписей=РегистрСведенийНаборЗаписей/InformationRegisterRecordSet",
		AccumulationRegister:   "МодульНабораЗаписей=РегистрНакопленияНаборЗаписей/AccumulationRegisterRecordSet",
		AccountingRegister:     "МодульНабораЗаписей=РегистрБухгалтерииНаборЗаписей/AccountingRegisterRecordSet",
		CalculationRegister:    "МодульНабораЗаписей=РегистрРасчетаНаборЗаписей/CalculationRegisterRecordSet",
		Recalculation:          "МодульНабораЗаписей=ПерерасчетНаборЗаписей/RecalculationRecordSet",
		Sequence:               "МодульНабораЗаписей=ПоследовательностьНаборЗаписей/SequenceRecordSet",
		Constant:               "МодульЗначения=КонстантаМенеджерЗначения/ConstantValueManager",
	}
	used := map[string]bool{}
	for _, item := range All() {
		var types []string
		for _, module := range append(item.Layout().Modules, FormModule, CommandModule) {
			objectType, ok := item.ThisObject(module)
			if !ok {
				continue
			}
			used[objectType.Name.Russian] = true
			types = append(types, string(module)+"="+objectType.Name.Russian+"/"+objectType.Name.English)
			if objectType.Declared != (module == ObjectModule) {
				t.Errorf("%s %s: its own fields are members = %v; only an object has them", item.Kind, module, objectType.Declared)
			}
			var names []string
			for _, member := range objectType.Members {
				names = append(names, strings.ToLower(member.Name.Russian))
			}
			slices.Sort(names)
			if len(slices.Compact(slices.Clone(names))) != len(names) {
				t.Errorf("%s has a member twice: %v", objectType.Name.Russian, names)
			}
			if !slices.ContainsFunc(objectType.Members, func(member Member) bool {
				return !member.Method && member.Name == ThisObject
			}) {
				t.Errorf("%s has no ЭтотОбъект, which the help gives every such type", objectType.Name.Russian)
			}
		}
		if got := strings.Join(types, " "); got != expected[item.Kind] {
			t.Errorf("ЭтотОбъект of %s is %q, the help gives %q", item.Kind, got, expected[item.Kind])
		}
	}
	for name := range objectTypes {
		if !used[name] {
			t.Errorf("objects.yaml describes %s, which no module runs around", name)
		}
	}
}

// TestParametersReadAsWritten catches a parameter of objects.yaml read with
// the wrong optionality or default: a required one, one marked optional by
// "?", one with the value it takes when left out.
func TestParametersReadAsWritten(t *testing.T) {
	member := readMember(Words{"Тип", "Type"}, []string{"Записать", "Write", "Запись", "Замещать = Истина", "Форма?"}, true)
	want := []Parameter{{Name: "Запись"}, {Name: "Замещать", Optional: true, Default: "Истина"}, {Name: "Форма", Optional: true}}
	if !slices.Equal(member.Parameters, want) || member.Name != (Words{"Записать", "Write"}) || !member.Method {
		t.Fatalf("read %+v, want %+v", member, want)
	}
	if got := want[1].Label(); got != "Замещать = Истина" {
		t.Fatalf("label %q", got)
	}
}
