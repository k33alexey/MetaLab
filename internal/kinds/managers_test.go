package kinds

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestManagerIsTheHelpsType catches a kind left without the manager the help
// gives it, or given one it lacks; a type of managers.yaml that lost or
// doubled a member; and a type nobody's manager is. The expectation is the
// help's (8.3.27: the types <Вид>Менеджер.<Имя>, counted without the members
// it writes as <Имя ...>, which depend on the object), written here rather
// than read from managers.
func TestManagerIsTheHelpsType(t *testing.T) {
	expected := map[Kind]string{
		Constant: "КонстантаМенеджер/ConstantManager 4", Catalog: "СправочникМенеджер/CatalogManager 22",
		Document: "ДокументМенеджер/DocumentManager 13", DocumentJournal: "ЖурналДокументовМенеджер/DocumentJournalManager 3",
		Enumeration: "ПеречислениеМенеджер/EnumManager 9", Report: "ОтчетМенеджер/ReportManager 3",
		DataProcessor:              "ОбработкаМенеджер/DataProcessorManager 3",
		ChartOfCharacteristicTypes: "ПланВидовХарактеристикМенеджер/ChartOfCharacteristicTypesManager 22",
		ChartOfAccounts:            "ПланСчетовМенеджер/ChartOfAccountsManager 19",
		ChartOfCalculationTypes:    "ПланВидовРасчетаМенеджер/ChartOfCalculationTypesManager 17",
		InformationRegister:        "РегистрСведенийМенеджер/InformationRegisterManager 18",
		AccumulationRegister:       "РегистрНакопленияМенеджер/AccumulationRegisterManager 33",
		AccountingRegister:         "РегистрБухгалтерииМенеджер/AccountingRegisterManager 24",
		CalculationRegister:        "РегистрРасчетаМенеджер/CalculationRegisterManager 10",
		Recalculation:              "ПерерасчетМенеджер/RecalculationManager 1",
		BusinessProcess:            "БизнесПроцессМенеджер/BusinessProcessManager 16",
		Task:                       "ЗадачаМенеджер/TaskManager 14", ExchangePlan: "ПланОбменаМенеджер/ExchangePlanManager 15",
		Sequence:                "ПоследовательностьМенеджер/SequenceManager 7",
		FilterCriterion:         "КритерийОтбораМенеджер/FilterCriterionManager 2",
		SettingsStorage:         "ХранилищеНастроекМенеджер/SettingsStorageManager 8",
		ExternalDataSource:      "ВнешнийИсточникДанныхМенеджер/ExternalDataSourceManager 15",
		ExternalDataSourceTable: "ВнешнийИсточникДанныхТаблицаМенеджер/ExternalDataSourceTableManager 10",
		ExternalCube:            "ВнешнийИсточникДанныхКубМенеджер/ExternalDataSourceCubeManager 6",
		ExternalDimensionTable:  "ВнешнийИсточникДанныхКубТаблицаИзмеренияМенеджер/ExternalDataSourceCubeDimensionTableManager 5",
		WSReference:             "WSСсылкаМенеджер/WSReferenceManager 2",
	}
	used := map[string]bool{}
	for _, item := range All() {
		manager, ok := item.Manager()
		got := ""
		if ok {
			used[manager.Name.Russian] = true
			got = manager.Name.Russian + "/" + manager.Name.English + " " + strconv.Itoa(len(manager.Members))
			var names []string
			for _, member := range manager.Members {
				names = append(names, strings.ToLower(member.Name.Russian))
				if strings.HasPrefix(member.Name.Russian, "<") {
					t.Errorf("%s has %s, which depends on the object", manager.Name.Russian, member.Name.Russian)
				}
			}
			slices.Sort(names)
			if len(slices.Compact(slices.Clone(names))) != len(names) {
				t.Errorf("%s has a member twice: %v", manager.Name.Russian, names)
			}
		}
		if got != expected[item.Kind] {
			t.Errorf("the manager of %s is %q, the help gives %q", item.Kind, got, expected[item.Kind])
		}
	}
	for name := range managerTypes {
		if !used[name] {
			t.Errorf("managers.yaml describes %s, which is no kind's manager", name)
		}
	}
}

// TestManagerMethodsAreSignedAsTheHelpSigns catches a method read with the
// parameters of a shorter form of it, a default left out, and one guessed
// where the help names a value the type does not have. The parameters are
// the help's, written here.
func TestManagerMethodsAreSignedAsTheHelpSigns(t *testing.T) {
	for _, test := range []struct {
		kind           Kind
		method, signed string
	}{
		{Catalog, "НайтиПоКоду", "Код, ПоискПоПолномуКоду = Ложь, Родитель, Владелец"},
		// The second form adds the date of the interval.
		{BusinessProcess, "НайтиПоНомеру", "НомерБизнесПроцесса, ДатаИнтервала"},
		{ExternalDataSource, "НачатьТранзакцию", "РежимБлокировок = РежимУправленияБлокировкойДанных.Автоматический, " +
			"УровеньИзоляции = УровеньИзоляцииТранзакций.Авто"},
		// «Автоматически» is no value of ОбновлениеПредопределенныхДанных.
		{ChartOfAccounts, "УстановитьОбновлениеПредопределенныхДанных", "ОбновлениеПредопределенных"},
	} {
		manager, _ := Must(test.kind).Manager()
		index := slices.IndexFunc(manager.Members, func(member Member) bool { return member.Method && member.Name.Russian == test.method })
		if index < 0 {
			t.Errorf("%s has no %s", manager.Name.Russian, test.method)
			continue
		}
		var labels []string
		optional := 0
		for _, parameter := range manager.Members[index].Parameters {
			labels = append(labels, parameter.Label())
			if parameter.Optional {
				optional++
			}
		}
		if got := strings.Join(labels, ", "); got != test.signed {
			t.Errorf("%s.%s is signed (%s), the help signs (%s)", manager.Name.Russian, test.method, got, test.signed)
		}
		if test.kind == BusinessProcess && optional != 1 {
			t.Errorf("%s.%s: the parameter the shorter form leaves out is not optional", manager.Name.Russian, test.method)
		}
	}
}

// TestManagersAreReachedAsTheHelpReachesThem catches a collection of the
// global context the help does not give, or one it gives left out; a kind
// inside another not reached through its owner's manager, or reached by a
// word the owner's manager has no property of; and the functions of a source
// not called through it. The help's: the properties of Глобальный контекст,
// ВнешнийИсточникДанныхМенеджер.Таблицы, .Кубы, .<Имя функции>(),
// ВнешнийИсточникДанныхКубМенеджер.ТаблицыИзмерений, РегистрРасчетаМенеджер.
// Перерасчеты.
func TestManagersAreReachedAsTheHelpReachesThem(t *testing.T) {
	var global []string
	for _, item := range All() {
		if item.InGlobalContext() {
			global = append(global, item.Collection.Russian)
		}
	}
	slices.Sort(global)
	want := []string{"WSСсылки", "БизнесПроцессы", "ВнешниеИсточникиДанных", "Документы", "ЖурналыДокументов", "Задачи",
		"Константы", "КритерииОтбора", "Обработки", "Отчеты", "ПараметрыСеанса", "Перечисления", "ПланыВидовРасчета",
		"ПланыВидовХарактеристик", "ПланыОбмена", "ПланыСчетов", "Последовательности", "РегистрыБухгалтерии",
		"РегистрыНакопления", "РегистрыРасчета", "РегистрыСведений", "Справочники", "ХранилищаНастроек"}
	if !slices.Equal(global, want) {
		t.Errorf("the global context has %v, the help gives %v", global, want)
	}
	for _, test := range []struct {
		owner Kind
		word  string
		inner Kind
	}{
		{ExternalDataSource, "Таблицы", ExternalDataSourceTable},
		{ExternalDataSource, "cubes", ExternalCube},
		{ExternalCube, "ТаблицыИзмерений", ExternalDimensionTable},
		{CalculationRegister, "Recalculations", Recalculation},
		{ExternalDataSource, "Функции", ""},
		{Catalog, "Таблицы", ""},
	} {
		inner, ok := Must(test.owner).Inner(test.word)
		if inner.Kind != test.inner || ok != (test.inner != "") {
			t.Errorf("%s.%s reaches %q, the help reaches %q", test.owner, test.word, inner.Kind, test.inner)
			continue
		}
		if !ok {
			continue
		}
		manager, _ := Must(test.owner).Manager()
		if !slices.ContainsFunc(manager.Members, func(member Member) bool { return !member.Method && member.Name.Is(test.word) }) {
			t.Errorf("%s reaches %s by %s, which it has no property of", manager.Name.Russian, test.inner, test.word)
		}
	}
	var methods []Kind
	for _, item := range All() {
		if item.OwnerMethod() {
			methods = append(methods, item.Kind)
		}
	}
	if !slices.Equal(methods, []Kind{ExternalFunction}) {
		t.Errorf("called through the owner's manager: %v; the help calls a function of a source alone", methods)
	}
}
