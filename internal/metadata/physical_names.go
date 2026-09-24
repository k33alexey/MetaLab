package metadata

import (
	"strings"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// standardColumnTitles names the columns every stored object has. They are not
// attributes of anything, so nothing else can translate them.
var standardColumnTitles = map[string]string{
	"ref": "Ссылка", "version": "Версия", "code": "Код", "description": "Наименование",
	"value_type": "Тип значения", "account_order": "Порядок", "account_kind": "Вид счёта",
	"parent": "Родитель", "is_folder": "Это группа",
	"action_period_is_base": "Период действия базовый", "calculation_type": "Вид расчёта",
	"started": "Стартован", "completed": "Завершён", "head_task": "Главная задача",
	"executed": "Выполнена", "business_process": "Бизнес-процесс", "route_point": "Точка маршрута",
	"base_chart":  "План видов расчёта базы",
	"off_balance": "Забалансовый", "ext_dimension_type": "Вид субконто", "turnover_only": "Только обороты",
	"deletion_mark": "Пометка удаления", "predefined_name": "Имя предопределённых данных",
	"owner_ref": "Владелец строки", "line_no": "Номер строки", "number": "Номер", "date": "Дата",
	"posted": "Проведён", "period": "Период", "recorder_type": "Тип регистратора", "recorder_ref": "Регистратор",
	"active": "Активность", "movement_kind": "Вид движения", "record_id": "Идентификатор записи",
}

// PhysicalNames maps the PostgreSQL names of this configuration onto the names
// a developer knows. A migration plan speaks in t_<uuid> and c_<uuid>, which is
// exactly right for PostgreSQL and useless to the person deciding whether to
// confirm it: "будет округлена колонка c_9f8a…" names nothing they can act on.
//
// A name that cannot be resolved is simply absent - the caller keeps showing the
// physical one rather than inventing a translation.
func (catalog *Catalog) PhysicalNames() map[string]string {
	result := make(map[string]string, 64)
	if catalog == nil {
		return result
	}
	for name, title := range standardColumnTitles {
		result[name] = title
	}
	// Indexes and constraints are named by the UUID of what they serve, with a
	// two-letter prefix. Translating them too is what keeps a line like
	// "будет удалён i_9f8a…" out of the confirmation dialog.
	derived := map[string]string{
		"i": "индекс", "ic": "индекс по коду", "id": "индекс", "im": "индекс по пометке удаления",
		"in": "индекс", "ip": "индекс", "ir": "индекс", "is": "индекс поиска", "cm": "проверка",
		"cs": "проверка", "ct": "проверка", "pt": "проверка", "pk": "первичный ключ", "fk": "внешний ключ",
		"up": "уникальность предопределённых", "uq": "уникальность", "ur": "уникальность",
	}
	addDerived := func(id uuid.UUID, title string) {
		identifier := strings.ReplaceAll(id.String(), "-", "")
		for prefix, kind := range derived {
			result[prefix+"_"+identifier] = title + " · " + kind
		}
	}
	addTable := func(id uuid.UUID, title string) {
		if name, err := schemadiff.TableName(id); err == nil {
			result[name] = title
		}
		addDerived(id, title)
	}
	addColumn := func(id uuid.UUID, title string) {
		if name, err := schemadiff.ColumnName(id); err == nil {
			result[name] = title
		}
		addDerived(id, title)
	}
	addAttributes := func(owner string, attributes []Attribute) {
		for _, attribute := range attributes {
			addColumn(attribute.ID, owner+"."+attribute.Name)
		}
	}
	addParts := func(owner string, parts []TablePart) {
		for _, part := range parts {
			addTable(part.ID, owner+"."+part.Name)
			addAttributes(owner+"."+part.Name, part.Attributes)
		}
	}
	for _, item := range catalog.Catalogs {
		owner := "Справочник." + item.Name
		addTable(item.ID, owner)
		addAttributes(owner, item.Attributes)
		addParts(owner, item.TableParts)
	}
	for _, item := range catalog.ChartsOfCharacteristicTypes {
		owner := "ПланВидовХарактеристик." + item.Name
		addTable(item.ID, owner)
		addAttributes(owner, item.Attributes)
		addParts(owner, item.TableParts)
	}
	for _, item := range catalog.ChartsOfAccounts {
		owner := "ПланСчетов." + item.Name
		addTable(item.ID, owner)
		addAttributes(owner, item.Attributes)
		addParts(owner, item.TableParts)
		// A flag is a column of the account, and the confirmation dialog has to
		// be able to say which flag it is about.
		for _, flag := range item.AccountingFlags {
			addColumn(flag.ID, owner+".ПризнакУчёта."+flag.Name)
		}
		for _, flag := range item.ExtDimensionAccountingFlags {
			addColumn(flag.ID, owner+".ПризнакУчётаСубконто."+flag.Name)
		}
		if item.ExtDimensionTypes != nil {
			result[extDimensionTableName(item.ID)] = owner + ".ВидыСубконто"
		}
	}
	for _, item := range catalog.ChartsOfCalculationTypes {
		owner := "ПланВидовРасчёта." + item.Name
		addTable(item.ID, owner)
		addAttributes(owner, item.Attributes)
		addParts(owner, item.TableParts)
		result[competitionTableName("tl", item.ID)] = owner + ".ВедущиеВидыРасчёта"
		if item.ActionPeriodUse {
			result[competitionTableName("tw", item.ID)] = owner + ".ВытесняющиеВидыРасчёта"
		}
		if item.BaseDependency != "" && item.BaseDependency != NoBaseDependency {
			result[competitionTableName("tb", item.ID)] = owner + ".БазовыеВидыРасчёта"
		}
	}
	for _, item := range catalog.BusinessProcesses {
		owner := "БизнесПроцесс." + item.Name
		addTable(item.ID, owner)
		addAttributes(owner, item.Attributes)
		addParts(owner, item.TableParts)
	}
	for _, item := range catalog.Tasks {
		owner := "Задача." + item.Name
		addTable(item.ID, owner)
		addAttributes(owner, item.Attributes)
		addAttributes(owner+".Адресация", addressingAsAttributes(item))
		addParts(owner, item.TableParts)
	}
	for _, item := range catalog.Documents {
		owner := "Документ." + item.Name
		addTable(item.ID, owner)
		addAttributes(owner, item.Attributes)
		addParts(owner, item.TableParts)
	}
	for _, item := range catalog.InformationRegisters {
		owner := "РегистрСведений." + item.Name
		addTable(item.ID, owner)
		addAttributes(owner, item.Dimensions)
		addAttributes(owner, item.Resources)
		addAttributes(owner, item.Attributes)
	}
	for _, item := range catalog.AccumulationRegisters {
		owner := "РегистрНакопления." + item.Name
		addTable(item.ID, owner)
		addAttributes(owner, item.Dimensions)
		addAttributes(owner, item.Resources)
		addAttributes(owner, item.Attributes)
	}
	return result
}
