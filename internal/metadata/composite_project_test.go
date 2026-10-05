package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The tests of block 2 each end one kind of object, in a project that holds
// that kind and what it needs. Import meets something else: every kind at
// once, each naming the others. The composite project below is that: every
// kind the loader reads, linked to one another the way a configuration links
// them, so that a check of one kind that trips over another, or a reference
// that resolves only when its target is alone, shows up here and nowhere else.

// compositeID gives the objects of the composite project identifiers of their
// own, fixed so that a test can break exactly one reference to one of them.
func compositeID(n int) string { return fmt.Sprintf("c0de0000-0000-4000-8000-%012d", n) }

var (
	cmpModuleSource      = compositeID(1)
	cmpCommonModule      = compositeID(2)
	cmpUsers             = compositeID(3)
	cmpSessionUser       = compositeID(4)
	cmpWarehouses        = compositeID(5)
	cmpGoods             = compositeID(6)
	cmpGoodsKind         = compositeID(7)
	cmpGoodsAmount       = compositeID(8)
	cmpGoodsProperty     = compositeID(9)
	cmpEnumeration       = compositeID(10)
	cmpDefinedType       = compositeID(11)
	cmpConstant          = compositeID(12)
	cmpCharacteristics   = compositeID(13)
	cmpAccounts          = compositeID(14)
	cmpCalculationTypes  = compositeID(15)
	cmpExchangePlan      = compositeID(16)
	cmpDocument          = compositeID(17)
	cmpDocumentWarehouse = compositeID(18)
	cmpPrices            = compositeID(19)
	cmpPricesGoods       = compositeID(20)
	cmpBalances          = compositeID(21)
	cmpBalancesGoods     = compositeID(22)
	cmpEntries           = compositeID(23)
	cmpPayroll           = compositeID(24)
	cmpPayrollPerson     = compositeID(25)
	cmpTask              = compositeID(26)
	cmpProcess           = compositeID(27)
	cmpJournal           = compositeID(28)
	cmpSequence          = compositeID(29)
	cmpNumerator         = compositeID(30)
	cmpCriterion         = compositeID(31)
	cmpReport            = compositeID(32)
	cmpProcessor         = compositeID(33)
	cmpOption            = compositeID(34)
	cmpOptionParameter   = compositeID(35)
	cmpSettingsStorage   = compositeID(36)
	cmpScheduledJob      = compositeID(37)
	cmpSubscription      = compositeID(38)
	cmpCommonAttribute   = compositeID(39)
	cmpCommandGroup      = compositeID(40)
	cmpCommonCommand     = compositeID(41)
	cmpCommonTemplate    = compositeID(42)
	cmpCommonPicture     = compositeID(43)
	cmpCommonForm        = compositeID(44)
	cmpStyleItem         = compositeID(45)
	cmpStyle             = compositeID(46)
	cmpXDTOPackage       = compositeID(47)
	cmpWebService        = compositeID(48)
	cmpWSReference       = compositeID(49)
	cmpHTTPService       = compositeID(50)
	cmpWebSocketClient   = compositeID(51)
	cmpRole              = compositeID(52)
	cmpSubsystem         = compositeID(53)
	cmpRecalculation     = compositeID(54)
	cmpEntriesSum        = compositeID(55)
	cmpBalancesQuantity  = compositeID(56)
	cmpPricesPrice       = compositeID(57)
	cmpPayrollSum        = compositeID(58)
	cmpRecalcDimension   = compositeID(59)
	cmpJournalColumn     = compositeID(60)
	cmpSequenceDimension = compositeID(61)
	cmpEnumValue         = compositeID(62)
	cmpUsersName         = compositeID(63)
)

// compositeProject writes the composite project and returns its root.
func compositeProject(t *testing.T) string {
	t.Helper()
	root := metadataProject(t)
	head := func(id, name string) string {
		return "format: 1\nid: " + id + "\nname: " + name + "\ntitle: {ru: " + name + "}\n"
	}
	number := func(numeric string) string {
		return "number: {type: string, length: 11, auto: true, periodicity: " + numeric + "}\n"
	}

	// The code everything that runs code calls into.
	writeFile(t, filepath.Join(root, "modules", cmpModuleSource+".bsl"),
		"Процедура ПередЗаписью(Источник, Отказ) Экспорт\nКонецПроцедуры\n\nПроцедура Выполнить() Экспорт\nКонецПроцедуры\n")
	writeMetadata(t, root, CommonModuleKind, cmpCommonModule, head(cmpCommonModule, "Общий")+"server: true\nmodule: "+cmpModuleSource+"\n")

	// Values: an enumeration, a defined type, a chart of kinds of property.
	writeMetadata(t, root, EnumerationKind, cmpEnumeration, head(cmpEnumeration, "ВидыТоваров")+
		"values:\n  - {id: "+cmpEnumValue+", name: Товар, title: {ru: Товар}}\n")
	writeMetadata(t, root, DefinedTypeKind, cmpDefinedType, head(cmpDefinedType, "ДенежнаяСумма")+
		"types: [{kind: number, precision: 15, scale: 2}]\n")
	writeMetadata(t, root, ChartOfCharacteristicTypesKind, cmpCharacteristics, head(cmpCharacteristics, "ВидыСубконто")+
		"code: {type: string, length: 9, auto: true}\ndescription_length: 100\n"+
		"value_type: [{kind: catalog, reference: "+cmpGoods+"}, {kind: catalog, reference: "+cmpWarehouses+"}]\n")

	// Catalogs, each naming a kind of value from elsewhere.
	writeMetadata(t, root, CatalogKind, cmpUsers, head(cmpUsers, "Пользователи")+
		"code: {type: string, length: 9, auto: true}\ndescription_length: 100\n"+
		"attributes:\n  - {id: "+cmpUsersName+", name: Имя, title: {ru: Имя}, types: [{kind: string, length: 50}]}\n")
	writeMetadata(t, root, CatalogKind, cmpWarehouses, head(cmpWarehouses, "Склады")+
		"code: {type: string, length: 9, auto: true}\ndescription_length: 100\n")
	writeMetadata(t, root, CatalogKind, cmpGoods, head(cmpGoods, "Номенклатура")+
		"code: {type: string, length: 9, auto: true}\ndescription_length: 150\nattributes:\n"+
		"  - {id: "+cmpGoodsKind+", name: Вид, title: {ru: Вид}, types: [{kind: enumeration, reference: "+cmpEnumeration+"}]}\n"+
		"  - {id: "+cmpGoodsAmount+", name: Цена, title: {ru: Цена}, types: [{kind: defined-type, reference: "+cmpDefinedType+"}]}\n"+
		"  - {id: "+cmpGoodsProperty+", name: Свойство, title: {ru: Свойство}, types: [{kind: characteristic, reference: "+cmpCharacteristics+"}]}\n")
	writeMetadata(t, root, SessionParameterKind, cmpSessionUser, head(cmpSessionUser, "ТекущийПользователь")+
		"types: [{kind: catalog, reference: "+cmpUsers+"}]\n")
	writeMetadata(t, root, ConstantKind, cmpConstant, head(cmpConstant, "ОсновнойСклад")+
		"types: [{kind: catalog, reference: "+cmpWarehouses+"}]\n")

	// Charts.
	writeMetadata(t, root, ChartOfAccountsKind, cmpAccounts, head(cmpAccounts, "Хозрасчетный")+
		"code: {type: string, length: 9, auto: false}\ndescription_length: 100\n"+
		"ext_dimension_types: "+cmpCharacteristics+"\nmax_ext_dimension_count: 2\n")
	writeMetadata(t, root, ChartOfCalculationTypesKind, cmpCalculationTypes, head(cmpCalculationTypes, "Начисления")+
		"code: {type: string, length: 9}\ndescription_length: 100\n")
	writeMetadata(t, root, ExchangePlanKind, cmpExchangePlan, head(cmpExchangePlan, "Филиалы")+
		"code: {type: string, length: 36, auto: false}\ndescription_length: 150\n"+
		"content:\n  - {kind: catalogs, object: "+cmpGoods+", auto_record: allow}\n")

	// Registers, and the document that writes all four kinds of them.
	writeMetadata(t, root, InformationRegisterKind, cmpPrices, head(cmpPrices, "Цены")+
		"write_mode: recorder\nperiodicity: none\n"+
		"dimensions:\n  - {id: "+cmpPricesGoods+", name: Номенклатура, title: {ru: Номенклатура}, types: [{kind: catalog, reference: "+cmpGoods+"}]}\n"+
		"resources:\n  - {id: "+cmpPricesPrice+", name: Цена, title: {ru: Цена}, types: [{kind: defined-type, reference: "+cmpDefinedType+"}]}\n")
	writeMetadata(t, root, AccumulationRegisterKind, cmpBalances, head(cmpBalances, "Остатки")+
		"kind: balance\n"+
		"dimensions:\n  - {id: "+cmpBalancesGoods+", name: Номенклатура, title: {ru: Номенклатура}, types: [{kind: catalog, reference: "+cmpGoods+"}]}\n"+
		"resources:\n  - {id: "+cmpBalancesQuantity+", name: Количество, title: {ru: Количество}, types: [{kind: number, precision: 15, scale: 3}]}\n")
	writeMetadata(t, root, AccountingRegisterKind, cmpEntries, head(cmpEntries, "Хозрасчетный")+
		"chart_of_accounts: "+cmpAccounts+"\ncorrespondence: true\n"+
		"resources:\n  - {id: "+cmpEntriesSum+", name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}\n")
	writeMetadata(t, root, CalculationRegisterKind, cmpPayroll, head(cmpPayroll, "Начисления")+
		"chart_of_calculation_types: "+cmpCalculationTypes+"\nperiodicity: month\n"+
		"dimensions:\n  - {id: "+cmpPayrollPerson+", name: Лицо, title: {ru: Лицо}, types: [{kind: catalog, reference: "+cmpUsers+"}]}\n"+
		"resources:\n  - {id: "+cmpPayrollSum+", name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}\n"+
		"recalculations:\n  - {id: "+cmpRecalculation+", name: Перерасчет, title: {ru: Перерасчёт}, dimensions: [{id: "+cmpRecalcDimension+
		", name: Лицо, title: {ru: Лицо}, register_dimension: "+cmpPayrollPerson+", leading_data: ["+cmpPayrollPerson+"]}]}\n")
	writeMetadata(t, root, NumeratorKind, cmpNumerator, head(cmpNumerator, "Сквозной")+"number: {type: string, length: 11, periodicity: year}\n")
	writeMetadata(t, root, DocumentKind, cmpDocument, head(cmpDocument, "Поступление")+
		"numerator: "+cmpNumerator+"\n"+
		"posting: {allowed: true}\nmovements: ["+cmpPrices+", "+cmpBalances+", "+cmpEntries+", "+cmpPayroll+"]\n"+
		"attributes:\n  - {id: "+cmpDocumentWarehouse+", name: Склад, title: {ru: Склад}, types: [{kind: catalog, reference: "+cmpWarehouses+"}]}\n")
	writeMetadata(t, root, DocumentJournalKind, cmpJournal, head(cmpJournal, "Журнал")+"documents: ["+cmpDocument+"]\n"+
		"columns:\n  - {id: "+cmpJournalColumn+", name: Склад, title: {ru: Склад}, references: ["+cmpDocumentWarehouse+"]}\n")
	writeMetadata(t, root, SequenceKind, cmpSequence, head(cmpSequence, "Последовательность")+"documents: ["+cmpDocument+"]\n"+
		"dimensions:\n  - {id: "+cmpSequenceDimension+", name: Склад, title: {ru: Склад}, types: [{kind: catalog, reference: "+cmpWarehouses+"}], document_attributes: ["+cmpDocumentWarehouse+"]}\n")
	writeMetadata(t, root, FilterCriterionKind, cmpCriterion, head(cmpCriterion, "Связанные")+
		"types: [{kind: document, reference: "+cmpDocument+"}]\n")

	// Tasks and the process that hands them out.
	writeMetadata(t, root, TaskKind, cmpTask, head(cmpTask, "Задача")+number("none")+"description_length: 150\naddressing_attributes: []\n")
	writeMetadata(t, root, BusinessProcessKind, cmpProcess, head(cmpProcess, "Задание")+number("year")+"task: "+cmpTask+"\n")

	// Reports, processors, options, storages, jobs, subscriptions.
	writeMetadata(t, root, ReportKind, cmpReport, head(cmpReport, "Ведомость"))
	writeMetadata(t, root, DataProcessorKind, cmpProcessor, head(cmpProcessor, "Заполнение")+
		"attributes:\n  - {id: "+compositeID(64)+", name: Документ, title: {ru: Документ}, types: [{kind: document, reference: "+cmpDocument+"}]}\n")
	writeMetadata(t, root, FunctionalOptionKind, cmpOption, head(cmpOption, "УчётПоСкладу")+
		"location: {kind: constants, object: "+cmpConstant+"}\n")
	writeMetadata(t, root, FunctionalOptionParameterKind, cmpOptionParameter, head(cmpOptionParameter, "Склад")+
		"use:\n  - {kind: catalogs, object: "+cmpWarehouses+"}\n")
	writeMetadata(t, root, SettingsStorageKind, cmpSettingsStorage, head(cmpSettingsStorage, "Варианты"))
	writeMetadata(t, root, ScheduledJobKind, cmpScheduledJob, head(cmpScheduledJob, "Обновление")+
		"module: "+cmpCommonModule+"\nprocedure: Выполнить\n")
	writeMetadata(t, root, EventSubscriptionKind, cmpSubscription, head(cmpSubscription, "ПроверкаТовара")+
		"source: [{kind: catalog-object, reference: "+cmpGoods+"}]\nevent: before-write\nmodule: "+cmpCommonModule+"\nprocedure: ПередЗаписью\n")
	writeMetadata(t, root, CommonAttributeKind, cmpCommonAttribute, head(cmpCommonAttribute, "Ответственный")+
		"types: [{kind: catalog, reference: "+cmpUsers+"}]\ncontent: [{metadata: "+cmpDocument+", use: use}]\n")

	// The interface: commands, templates, pictures, forms, styles.
	writeMetadata(t, root, CommandGroupKind, cmpCommandGroup, head(cmpCommandGroup, "Сервис")+"category: actions-panel\n")
	writeCommonCommand(t, root, "ОткрытьОстатки", head(cmpCommonCommand, "ОткрытьОстатки")+
		"group_ref: "+cmpCommandGroup+"\nparameter: [{kind: catalog, reference: "+cmpGoods+"}]\nparameter_use: single\n", true)
	writeCommonTemplate(t, root, cmpCommonTemplate, "Бланк", TextTemplate)
	writeCommonPicture(t, root, cmpCommonPicture, "Логотип", "")
	writeCommonForm(t, root, "АдреснаяКнига", head(cmpCommonForm, "АдреснаяКнига")+"kind: common\n")
	writeStyleItem(t, root, cmpStyleItem, "ЦветВажного", ColorStyleItem, "  color: {source: absolute, rgb: '#C00000'}\n")
	writeMetadata(t, root, StyleKind, cmpStyle, head(cmpStyle, "Основной")+
		"items:\n  - {item: "+cmpStyleItem+", value: {color: {source: absolute, rgb: '#FF0000'}}}\n")

	// Exchange with the outside.
	writeXDTOPackage(t, root, cmpXDTOPackage, "Товары", "http://example.org/goods/1.0", "")
	writeMetadata(t, root, WebServiceKind, cmpWebService, head(cmpWebService, "Обмен")+
		"namespace: http://example.org/exchange/1.0\npackages: [{package: "+cmpXDTOPackage+"}]\n")
	writeWSReference(t, root, cmpWSReference, "Поставщик", "location_url: http://example.org\n", "")
	writeHTTPService(t, root, cmpHTTPService, "Биллинг", "root_url: billing\n")
	writeWebSocketClient(t, root, cmpWebSocketClient, "Биржа", "")
	writeSalesCube(t, root, noteCubeBody(), goodsDimTableBody)
	writeExternalTable(t, root, "Склад", goodsTable, "Товары",
		strings.Replace(goodsBody, `unfilled_parent_value: {kind: string, data: ""}`, `unfilled_parent_value: {kind: undefined, data: ""}`, 1))

	// Who may do what, and where it is shown.
	writeMetadata(t, root, RoleKind, cmpRole, head(cmpRole, "Кладовщик")+
		"objects:\n  - {object: "+cmpGoods+", operations: [read]}\n  - {object: "+cmpDocument+", operations: [read, create]}\n")
	writeMetadata(t, root, SubsystemKind, cmpSubsystem, head(cmpSubsystem, "Склад")+
		"members: ["+cmpGoods+", "+cmpDocument+", "+cmpReport+"]\n")
	return root
}

// compositeReferences is every reference of the composite project from an
// object of one kind to an object of another: the kind and name of the
// object that refers, and the identifier it refers to.
var compositeReferences = []struct {
	what     string
	kind     Kind
	referrer string
	target   string
	// noted is a reference the prototype keeps when its target is gone, and
	// so do we, with a note: the rest are refused.
	noted bool
}{
	{"параметр сеанса → справочник", SessionParameterKind, "ТекущийПользователь", cmpUsers, false},
	{"константа → справочник", ConstantKind, "ОсновнойСклад", cmpWarehouses, false},
	{"справочник → перечисление", CatalogKind, "Номенклатура", cmpEnumeration, false},
	{"справочник → определяемый тип", CatalogKind, "Номенклатура", cmpDefinedType, false},
	{"справочник → план видов характеристик", CatalogKind, "Номенклатура", cmpCharacteristics, false},
	{"план видов характеристик → справочник", ChartOfCharacteristicTypesKind, "ВидыСубконто", cmpGoods, false},
	{"план видов характеристик → второй справочник", ChartOfCharacteristicTypesKind, "ВидыСубконто", cmpWarehouses, false},
	{"план счетов → план видов характеристик", ChartOfAccountsKind, "Хозрасчетный", cmpCharacteristics, false},
	{"план обмена → справочник", ExchangePlanKind, "Филиалы", cmpGoods, false},
	{"регистр сведений → справочник", InformationRegisterKind, "Цены", cmpGoods, false},
	{"регистр сведений → определяемый тип", InformationRegisterKind, "Цены", cmpDefinedType, false},
	{"регистр накопления → справочник", AccumulationRegisterKind, "Остатки", cmpGoods, false},
	{"регистр бухгалтерии → план счетов", AccountingRegisterKind, "Хозрасчетный", cmpAccounts, false},
	{"регистр расчёта → план видов расчёта", CalculationRegisterKind, "Начисления", cmpCalculationTypes, false},
	{"регистр расчёта → справочник", CalculationRegisterKind, "Начисления", cmpUsers, false},
	{"документ → нумератор", DocumentKind, "Поступление", cmpNumerator, false},
	{"документ → регистр сведений", DocumentKind, "Поступление", cmpPrices, false},
	{"документ → регистр накопления", DocumentKind, "Поступление", cmpBalances, false},
	{"документ → регистр бухгалтерии", DocumentKind, "Поступление", cmpEntries, false},
	{"документ → регистр расчёта", DocumentKind, "Поступление", cmpPayroll, false},
	{"документ → справочник", DocumentKind, "Поступление", cmpWarehouses, false},
	{"журнал → документ", DocumentJournalKind, "Журнал", cmpDocument, false},
	{"журнал → реквизит документа", DocumentJournalKind, "Журнал", cmpDocumentWarehouse, false},
	{"последовательность → документ", SequenceKind, "Последовательность", cmpDocument, false},
	{"последовательность → реквизит документа", SequenceKind, "Последовательность", cmpDocumentWarehouse, false},
	{"последовательность → справочник", SequenceKind, "Последовательность", cmpWarehouses, false},
	{"критерий отбора → документ", FilterCriterionKind, "Связанные", cmpDocument, false},
	{"бизнес-процесс → задача", BusinessProcessKind, "Задание", cmpTask, false},
	{"обработка → документ", DataProcessorKind, "Заполнение", cmpDocument, false},
	{"функциональная опция → константа", FunctionalOptionKind, "УчётПоСкладу", cmpConstant, false},
	{"параметр опций → справочник", FunctionalOptionParameterKind, "Склад", cmpWarehouses, false},
	{"общий модуль → текст модуля", CommonModuleKind, "Общий", cmpModuleSource, false},
	{"регламентное задание → общий модуль", ScheduledJobKind, "Обновление", cmpCommonModule, false},
	{"подписка → справочник", EventSubscriptionKind, "ПроверкаТовара", cmpGoods, false},
	{"подписка → общий модуль", EventSubscriptionKind, "ПроверкаТовара", cmpCommonModule, false},
	{"общий реквизит → справочник", CommonAttributeKind, "Ответственный", cmpUsers, false},
	{"общий реквизит → документ", CommonAttributeKind, "Ответственный", cmpDocument, false},
	{"общая команда → группа команд", CommonCommandKind, "ОткрытьОстатки", cmpCommandGroup, true},
	{"общая команда → справочник", CommonCommandKind, "ОткрытьОстатки", cmpGoods, false},
	{"стиль → элемент стиля", StyleKind, "Основной", cmpStyleItem, false},
	{"веб-сервис → пакет XDTO", WebServiceKind, "Обмен", cmpXDTOPackage, true},
	{"роль → справочник", RoleKind, "Кладовщик", cmpGoods, false},
	{"роль → документ", RoleKind, "Кладовщик", cmpDocument, false},
	{"подсистема → справочник", SubsystemKind, "Склад", cmpGoods, true},
	{"подсистема → документ", SubsystemKind, "Склад", cmpDocument, true},
	{"подсистема → отчёт", SubsystemKind, "Склад", cmpReport, true},
}

// breakReference points every mention of target in the description of the
// named object of the kind at an identifier nothing has.
func breakReference(t *testing.T, root string, kind Kind, referrer, target string) {
	t.Helper()
	var found []string
	err := filepath.WalkDir(filepath.Join(root, "metadata", string(kind)), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".yaml" {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(content), "\nname: "+referrer+"\n") && strings.Contains(string(content), target) {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Fatalf("%s %s referring to %s lies in %d files: %v", kind, referrer, target, len(found), found)
	}
	content, _ := os.ReadFile(found[0])
	writeFile(t, found[0], strings.ReplaceAll(string(content), target, compositeID(999)))
}

// named turns a lookup by name of one kind into one the table below can hold.
func named[T any](find func(*Catalog, string) (T, bool)) func(*Catalog, string) (uuid.UUID, bool) {
	return func(catalog *Catalog, name string) (uuid.UUID, bool) {
		value, ok := find(catalog, name)
		if !ok {
			return uuid.UUID{}, false
		}
		return reflect.ValueOf(value).FieldByName("ID").Interface().(uuid.UUID), true
	}
}

// identified does the same for a lookup by identifier.
func identified[T any](find func(*Catalog, uuid.UUID) (T, bool)) func(*Catalog, uuid.UUID) (string, bool) {
	return func(catalog *Catalog, id uuid.UUID) (string, bool) {
		value, ok := find(catalog, id)
		if !ok {
			return "", false
		}
		return reflect.ValueOf(value).FieldByName("Name").String(), true
	}
}

// compositeCube is the cube of the composite project, found the way a
// developer finds it: through its source.
func compositeCube(catalog *Catalog) (ExternalCube, bool) {
	source, ok := catalog.ExternalDataSource("Склад")
	if !ok || len(source.Cubes) != 1 {
		return ExternalCube{}, false
	}
	return source.Cubes[0], true
}

// Every kind the loader reads lies in the composite project, loads beside all
// the others, and is found both by its name and by its identifier.
//
// Defect caught: a check of one kind that refuses a project because of
// another kind lying beside it, which no project of one kind can show; an
// object loaded but left out of the index of its kind or of the index of all
// objects, so that a reference to it by identifier finds nothing; and a kind
// added to the loader that this project, and so this test, does not hold.
func TestCompositeProjectFindsEveryKindByNameAndIdentifier(t *testing.T) {
	t.Parallel()
	root := compositeProject(t)
	for _, kind := range project.MetadataKinds() {
		entries, err := os.ReadDir(filepath.Join(root, "metadata", kind))
		if kind == "languages" {
			// The languages are the configuration's own, kept in it.
			continue
		}
		if err != nil || len(entries) == 0 {
			t.Errorf("the composite project holds nothing of %s (%v)", kind, err)
		}
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	looked := map[string]bool{}
	for _, object := range []struct {
		kind   string
		name   string
		id     string
		byName func(*Catalog, string) (uuid.UUID, bool)
		// byID is the lookup of the kind itself, where it has one; every
		// object is also looked up among all objects regardless.
		byID func(*Catalog, uuid.UUID) (string, bool)
	}{
		{"subsystem", "Склад", cmpSubsystem, named((*Catalog).Subsystem), identified((*Catalog).SubsystemByID)},
		{"common module", "Общий", cmpCommonModule, named((*Catalog).CommonModule), identified((*Catalog).CommonModuleByID)},
		{"session parameter", "ТекущийПользователь", cmpSessionUser, named((*Catalog).SessionParameter), identified((*Catalog).SessionParameterByID)},
		{"role", "Кладовщик", cmpRole, named((*Catalog).Role), identified((*Catalog).RoleByID)},
		{"common attribute", "Ответственный", cmpCommonAttribute, named((*Catalog).CommonAttribute), identified((*Catalog).CommonAttributeByID)},
		{"exchange plan", "Филиалы", cmpExchangePlan, named((*Catalog).ExchangePlan), identified((*Catalog).ExchangePlanByID)},
		{"filter criterion", "Связанные", cmpCriterion, named((*Catalog).FilterCriterion), nil},
		{"event subscription", "ПроверкаТовара", cmpSubscription, named((*Catalog).EventSubscription), identified((*Catalog).EventSubscriptionByID)},
		{"scheduled job", "Обновление", cmpScheduledJob, named((*Catalog).ScheduledJob), nil},
		{"functional option", "УчётПоСкладу", cmpOption, named((*Catalog).FunctionalOption), nil},
		{"functional option parameter", "Склад", cmpOptionParameter, named((*Catalog).FunctionalOptionParameter), nil},
		{"defined type", "ДенежнаяСумма", cmpDefinedType, named((*Catalog).DefinedType), identified((*Catalog).DefinedTypeByID)},
		{"settings storage", "Варианты", cmpSettingsStorage, named((*Catalog).SettingsStorage), nil},
		{"common command", "ОткрытьОстатки", cmpCommonCommand, named((*Catalog).CommonCommand), nil},
		{"command group", "Сервис", cmpCommandGroup, named((*Catalog).CommandGroup), nil},
		{"common template", "Бланк", cmpCommonTemplate, named((*Catalog).CommonTemplate), identified((*Catalog).CommonTemplateByID)},
		{"common picture", "Логотип", cmpCommonPicture, named((*Catalog).CommonPicture), identified((*Catalog).CommonPictureByID)},
		{"XDTO package", "Товары", cmpXDTOPackage, named((*Catalog).XDTOPackage), identified((*Catalog).XDTOPackageByID)},
		{"web service", "Обмен", cmpWebService, named((*Catalog).WebService), nil},
		{"HTTP service", "Биллинг", cmpHTTPService, named((*Catalog).HTTPService), nil},
		{"WS reference", "Поставщик", cmpWSReference, named((*Catalog).WSReference), identified((*Catalog).WSReferenceByID)},
		{"WebSocket client", "Биржа", cmpWebSocketClient, named((*Catalog).WebSocketClient), identified((*Catalog).WebSocketClientByID)},
		{"style item", "ЦветВажного", cmpStyleItem, named((*Catalog).StyleItem), identified((*Catalog).StyleItemByID)},
		{"style", "Основной", cmpStyle, named((*Catalog).Style), identified((*Catalog).StyleByID)},
		{"constant", "ОсновнойСклад", cmpConstant, named((*Catalog).Constant), identified((*Catalog).ConstantByID)},
		{"catalog", "Номенклатура", cmpGoods, named((*Catalog).CatalogDefinition), identified((*Catalog).CatalogByID)},
		{"document", "Поступление", cmpDocument, named((*Catalog).DocumentDefinition), identified((*Catalog).DocumentByID)},
		{"numerator", "Сквозной", cmpNumerator, named((*Catalog).Numerator), identified((*Catalog).NumeratorByID)},
		{"sequence", "Последовательность", cmpSequence, named((*Catalog).Sequence), identified((*Catalog).SequenceByID)},
		{"document journal", "Журнал", cmpJournal, named((*Catalog).DocumentJournal), identified((*Catalog).DocumentJournalByID)},
		{"enumeration", "ВидыТоваров", cmpEnumeration, named((*Catalog).Enumeration), nil},
		{"report", "Ведомость", cmpReport, named((*Catalog).Report), nil},
		{"data processor", "Заполнение", cmpProcessor, named((*Catalog).DataProcessor), nil},
		{"chart of characteristic types", "ВидыСубконто", cmpCharacteristics, named((*Catalog).ChartOfCharacteristicTypes), identified((*Catalog).ChartOfCharacteristicTypesByID)},
		{"chart of accounts", "Хозрасчетный", cmpAccounts, named((*Catalog).ChartOfAccounts), identified((*Catalog).ChartOfAccountsByID)},
		{"chart of calculation types", "Начисления", cmpCalculationTypes, named((*Catalog).ChartOfCalculationTypes), identified((*Catalog).ChartOfCalculationTypesByID)},
		{"information register", "Цены", cmpPrices, named((*Catalog).InformationRegisterDefinition), identified((*Catalog).InformationRegisterByID)},
		{"accumulation register", "Остатки", cmpBalances, named((*Catalog).AccumulationRegisterDefinition), identified((*Catalog).AccumulationRegisterByID)},
		{"accounting register", "Хозрасчетный", cmpEntries, named((*Catalog).AccountingRegister), identified((*Catalog).AccountingRegisterByID)},
		{"calculation register", "Начисления", cmpPayroll, named((*Catalog).CalculationRegister), identified((*Catalog).CalculationRegisterByID)},
		{"business process", "Задание", cmpProcess, named((*Catalog).BusinessProcess), identified((*Catalog).BusinessProcessByID)},
		{"task", "Задача", cmpTask, named((*Catalog).Task), identified((*Catalog).TaskByID)},
		{"external data source", "Склад", warehouseSource, named((*Catalog).ExternalDataSource), identified((*Catalog).ExternalDataSourceByID)},
		{"external data source table", "Товары", goodsTable, func(catalog *Catalog, name string) (uuid.UUID, bool) {
			source, _ := catalog.ExternalDataSource("Склад")
			for _, table := range source.Tables {
				if table.Name == name {
					return table.ID, true
				}
			}
			return uuid.UUID{}, false
		}, func(catalog *Catalog, id uuid.UUID) (string, bool) {
			table, _, ok := catalog.ExternalTableByID(id)
			return table.Name, ok
		}},
		{"external data source cube", "Продажи", salesCube, func(catalog *Catalog, name string) (uuid.UUID, bool) {
			cube, ok := compositeCube(catalog)
			return cube.ID, ok && cube.Name == name
		}, nil},
		{"external data source dimension table", "Товары", goodsDimTable, func(catalog *Catalog, name string) (uuid.UUID, bool) {
			cube, _ := compositeCube(catalog)
			for _, table := range cube.DimensionTables {
				if table.Name == name {
					return table.ID, true
				}
			}
			return uuid.UUID{}, false
		}, nil},
		{"common form", "АдреснаяКнига", cmpCommonForm, func(catalog *Catalog, name string) (uuid.UUID, bool) {
			forms, err := ReadCommonForms(root, catalog.Project)
			for _, form := range forms {
				if err == nil && form.Name == name && catalog.commonFormNames[strings.ToLower(name)] {
					return form.ID, true
				}
			}
			return uuid.UUID{}, false
		}, nil},
	} {
		id, err := uuid.Parse(object.id)
		if err != nil {
			t.Fatal(err)
		}
		if found, ok := object.byName(catalog, object.name); !ok || found != id {
			t.Errorf("%s %s by name: %s, %v", object.kind, object.name, found, ok)
		}
		if kind := catalog.objectKindByID[id]; kind != object.kind {
			t.Errorf("%s %s by identifier among all objects: %q", object.kind, object.name, kind)
		}
		if object.byID != nil {
			if name, ok := object.byID(catalog, id); !ok || name != object.name {
				t.Errorf("%s %s by identifier: %q, %v", object.kind, object.name, name, ok)
			}
		}
		looked[object.kind] = true
	}
	// A kind the composite project holds and the table above does not look
	// for would go untried.
	for id, kind := range catalog.objectKindByID {
		if !looked[kind] {
			t.Errorf("%s %s is in the project and its kind is not looked up", kind, id)
		}
	}
	// A recalculation lies inside its register and is reached through it.
	register, _ := catalog.CalculationRegister("Начисления")
	if len(register.Recalculations) != 1 || register.Recalculations[0].ID.String() != cmpRecalculation || register.Recalculations[0].Name != "Перерасчет" {
		t.Errorf("the recalculation of the register: %+v", register.Recalculations)
	}
}

// The composite project is sound, so it carries no note.
//
// Defect caught: a note raised on one kind by the mere presence of another -
// the import report would then show every configuration as faulty.
func TestCompositeProjectCarriesNoNotes(t *testing.T) {
	t.Parallel()
	catalog, err := Load(compositeProject(t))
	if err != nil {
		t.Fatal(err)
	}
	if notes := catalog.Notes(); len(notes) != 0 {
		t.Fatalf("the sound composite project carries notes: %+v", notes)
	}
}

// Every reference from one kind to another resolves in the composite project,
// and each one, broken alone, is either refused or noted - in both cases
// naming the object that holds it. Never silently accepted.
//
// Defect caught: a reference left unchecked, so that an object pointing at
// nothing loads clean and fails only when a user opens it; a refusal that
// does not say where; and a reference carried without a note, which the
// import report would then not show.
func TestCompositeProjectRefusesOrNotesEveryBrokenReference(t *testing.T) {
	t.Parallel()
	for _, reference := range compositeReferences {
		t.Run(reference.what, func(t *testing.T) {
			t.Parallel()
			root := compositeProject(t)
			breakReference(t, root, reference.kind, reference.referrer, reference.target)
			catalog, err := Load(root)
			if !reference.noted {
				if err == nil {
					t.Fatalf("a reference to nothing was accepted; notes: %+v", catalog.Notes())
				}
				if !strings.Contains(err.Error(), reference.referrer) || !strings.Contains(err.Error(), compositeID(999)) {
					t.Fatalf("the refusal does not say where: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("a reference the prototype keeps was refused: %v", err)
			}
			notes := catalog.Notes()
			if len(notes) != 1 || notes[0].Kind != NoteUnresolvedReference ||
				!strings.Contains(notes[0].Where, reference.referrer) || notes[0].Written != compositeID(999) {
				t.Fatalf("notes = %+v", notes)
			}
		})
	}
}

// Every reference of the composite project is listed above: a kind's
// reference to another kind that is not there would go untested.
//
// Defect caught: the list of references falling behind the project, so that
// a reference added to the project is never broken by the test above.
func TestCompositeReferencesCoverTheProject(t *testing.T) {
	t.Parallel()
	root := compositeProject(t)
	listed := map[string]bool{}
	for _, reference := range compositeReferences {
		listed[string(reference.kind)+"/"+reference.referrer+"/"+reference.target] = true
	}
	// Each object's own identifiers, and those of what it holds inside it,
	// are not references.
	own := map[string]bool{}
	for n := 1; n <= 64; n++ {
		own[compositeID(n)] = true
	}
	err := filepath.WalkDir(filepath.Join(root, "metadata"), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".yaml" {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(filepath.Join(root, "metadata"), path)
		kind := strings.Split(filepath.ToSlash(relative), "/")[0]
		var name string
		var declared []string
		for _, line := range strings.Split(string(content), "\n") {
			if value, found := strings.CutPrefix(line, "name: "); found && name == "" {
				name = value
			}
			for rest := line; ; {
				index := strings.Index(rest, "id: ")
				if index < 0 || len(rest) < index+4+36 {
					break
				}
				// "id: " ends "chart_id: " and the like too; only a field
				// called id itself declares.
				if index == 0 || rest[index-1] == ' ' || rest[index-1] == '{' {
					declared = append(declared, rest[index+4:index+4+36])
				}
				rest = rest[index+4:]
			}
		}
		for n := 1; n <= 64; n++ {
			id := compositeID(n)
			if !strings.Contains(string(content), id) || slices.Contains(declared, id) {
				continue
			}
			if !listed[kind+"/"+name+"/"+id] {
				t.Errorf("%s %s refers to %s and the reference is not listed", kind, name, id)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
