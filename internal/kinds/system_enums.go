package kinds

// The enumerations of the platform the editor of a type offers as types, in
// its folder «Системные перечисления» (opis of 24.10.2026, obshchee.md,
// «Системные перечисления по местам»): 24 of them, in the order of the
// window - three of the data of accounting, three of the interface, eighteen
// of the analysis of data. The help (8.3.27) gives each its names, its
// values and the type XDTO it is written by, and says of every one that an
// attribute of a managed form may hold it.
//
// This is the one list of them: the model takes them as types by it, and
// the language is to read their values from it too (BLOCKS, block 5,
// «системные перечисления — одна таблица»).

// The spaces of the types XDTO the enumerations are written in.
const (
	enterpriseSpace   = "http://v8.1c.ru/8.1/data/enterprise"
	interfaceSpace    = "http://v8.1c.ru/8.1/data/ui"
	dataAnalysisSpace = "http://v8.1c.ru/8.2/data/data-analysis"
)

// SystemEnumeration is one enumeration of the platform.
type SystemEnumeration struct {
	// Name is the enumeration's name in the language; its English one is
	// the name of its type XDTO as well.
	Name Words
	// Kind is the word the project writes the type by.
	Kind string
	// Space is the namespace of its type XDTO.
	Space  string
	Values []Words
}

// SystemEnumerations are the enumerations of the platform the editor of a type
// offers, in the order of the window.
var SystemEnumerations = []SystemEnumeration{
	{Words{"ВидДвиженияНакопления", "AccumulationRecordType"}, "accumulation-record-kind", enterpriseSpace, []Words{{"Приход", "Receipt"}, {"Расход", "Expense"}}},
	{Words{"ВидСчета", "AccountType"}, "account-kind", enterpriseSpace, []Words{{"АктивноПассивный", "ActivePassive"}, {"Активный", "Active"}, {"Пассивный", "Passive"}}},
	{Words{"ВидДвиженияБухгалтерии", "AccountingRecordType"}, "accounting-record-kind", enterpriseSpace, []Words{{"Дебет", "Debit"}, {"Кредит", "Credit"}}},
	{Words{"ГоризонтальноеПоложение", "HorizontalAlign"}, "horizontal-align", interfaceSpace, []Words{{"Авто", "Auto"}, {"Лево", "Left"}, {"ПоШирине", "Justify"}, {"Право", "Right"}, {"Центр", "Center"}}},
	{Words{"ВертикальноеПоложение", "VerticalAlign"}, "vertical-align", interfaceSpace, []Words{{"Верх", "Top"}, {"Низ", "Bottom"}, {"Центр", "Center"}}},
	{Words{"РежимИзмененияРазмера", "SizeChangeMode"}, "size-change-mode", interfaceSpace, []Words{{"БыстроеИзменение", "QuickChange"}, {"Обычный", "Normal"}}},
	{Words{"ВидДанныхАнализа", "AnalysisDataType"}, "analysis-data-type", dataAnalysisSpace, []Words{{"Дискретные", "Discrete"}, {"Непрерывные", "Contiguous"}}},
	{Words{"МетодКластеризации", "ClusterizationMethod"}, "clusterization-method", dataAnalysisSpace, []Words{{"БлижняяСвязь", "NearestNeighbor"}, {"ДальняяСвязь", "FurthestNeighbor"}, {"КСредних", "KMeans"}, {"ЦентрТяжести", "Centroid"}}},
	{Words{"ТипЕдиницыИнтервалаВремениАнализаДанных", "DataAnalysisTimeIntervalUnitType"}, "data-analysis-time-interval-unit-type", dataAnalysisSpace, []Words{
		{"Год", "Year"}, {"Декада", "TenDays"}, {"День", "Day"}, {"Квартал", "Quarter"}, {"Месяц", "Month"}, {"Минута", "Minute"},
		{"Неделя", "Week"}, {"Полугодие", "HalfYear"}, {"Секунда", "Second"}, {"ТекущаяДекада", "CurrentTenDays"},
		{"ТекущаяМинута", "CurrentMinute"}, {"ТекущаяНеделя", "CurrentWeek"}, {"ТекущееПолугодие", "CurrentHalfYear"},
		{"ТекущийГод", "CurrentYear"}, {"ТекущийДень", "CurrentDay"}, {"ТекущийКвартал", "CurrentQuarter"},
		{"ТекущийМесяц", "CurrentMonth"}, {"ТекущийЧас", "CurrentHour"}, {"Час", "Hour"},
	}},
	{Words{"ТипЗаполненияТаблицыРезультатаАнализаДанных", "DataAnalysisResultTableFillType"}, "data-analysis-result-table-fill-type", dataAnalysisSpace, []Words{{"ВсеПоля", "AllFields"}, {"ИспользуемыеПоля", "UsedFields"}, {"КлючевыеПоля", "KeyFields"}, {"НеЗаполнять", "DontFill"}}},
	{Words{"ТипИспользованияЧисловыхЗначенийАнализаДанных", "DataAnalysisNumericValueUseType"}, "data-analysis-numeric-value-use-type", dataAnalysisSpace, []Words{{"КакБулево", "AsBoolean"}, {"КакЧисло", "AsNumeric"}}},
	{Words{"ТипИсточникаДанныхПоискаАссоциаций", "AssociationRulesDataSourceType"}, "association-rules-data-source-type", dataAnalysisSpace, []Words{{"Объектный", "Object"}, {"Событийный", "Event"}}},
	{Words{"ТипКолонкиАнализаДанныхДеревоРешений", "DataAnalysisColumnTypeDecisionTree"}, "data-analysis-column-type-decision-tree", dataAnalysisSpace, []Words{{"Входная", "Input"}, {"НеИспользуемая", "NotUsed"}, {"Прогнозируемая", "Predictable"}}},
	{Words{"ТипКолонкиАнализаДанныхКластеризация", "DataAnalysisColumnTypeClusterization"}, "data-analysis-column-type-clusterization", dataAnalysisSpace, []Words{{"Входная", "Input"}, {"ВходнаяИПрогнозируемая", "InputAndPredictable"}, {"Ключ", "Key"}, {"НеИспользуемая", "NotUsed"}, {"Прогнозируемая", "Predictable"}}},
	{Words{"ТипКолонкиАнализаДанныхОбщаяСтатистика", "DataAnalysisColumnTypeSummaryStatistics"}, "data-analysis-column-type-summary-statistics", dataAnalysisSpace, []Words{{"Входная", "Input"}, {"НеИспользуемая", "NotUsed"}}},
	{Words{"ТипКолонкиАнализаДанныхПоискАссоциаций", "DataAnalysisColumnTypeAssociationRules"}, "data-analysis-column-type-association-rules", dataAnalysisSpace, []Words{{"НеИспользуемая", "NotUsed"}, {"Объект", "Object"}, {"Элемент", "Item"}}},
	{Words{"ТипКолонкиАнализаДанныхПоискПоследовательностей", "DataAnalysisColumnTypeSequentialPatterns"}, "data-analysis-column-type-sequential-patterns", dataAnalysisSpace, []Words{{"Время", "Time"}, {"НеИспользуемая", "NotUsed"}, {"Последовательность", "Sequence"}, {"Элемент", "Item"}}},
	{Words{"ТипКолонкиМоделиПрогноза", "PredictionModelColumnType"}, "prediction-model-column-type", dataAnalysisSpace, []Words{{"Входная", "Input"}, {"КолонкаИсточникаДанных", "DataSourceColumn"}, {"Прогнозируемая", "Predictable"}}},
	{Words{"ТипМерыРасстоянияАнализаДанных", "DataAnalysisDistanceMetricType"}, "data-analysis-distance-metric-type", dataAnalysisSpace, []Words{{"ЕвклидоваМетрика", "Euclidean"}, {"ЕвклидоваМетрикаВКвадрате", "SquaredEuclidean"}, {"МетрикаГорода", "CityBlock"}, {"МетрикаДоминирования", "Maximum"}}},
	{Words{"ТипОтсеченияПравилАссоциации", "AssociationRulesPruneType"}, "association-rules-prune-type", dataAnalysisSpace, []Words{{"Избыточные", "Redundant"}, {"Покрытые", "Covered"}}},
	{Words{"ТипСтандартизацииАнализаДанных", "DataAnalysisStandardizationType"}, "data-analysis-standardization-type", dataAnalysisSpace, []Words{{"НеСтандартизировать", "DontStandardize"}, {"Стандартизировать", "Standardize"}}},
	{Words{"ТипУпрощенияДереваРешений", "DecisionTreeSimplificationType"}, "decision-tree-simplification-type", dataAnalysisSpace, []Words{{"НеУпрощать", "DontSimplify"}, {"Упрощать", "Simplify"}}},
	{Words{"ТипУпорядочиванияПравилАссоциацииАнализаДанных", "DataAnalysisAssociationRulesOrderType"}, "data-analysis-association-rules-order-type", dataAnalysisSpace, []Words{{"ПоДостоверности", "ByConfidence"}, {"ПоЗначимости", "ByImportance"}, {"ПоКоличествуСлучаев", "BySupport"}}},
	{Words{"ТипУпорядочиванияШаблоновПоследовательностейАнализаДанных", "DataAnalysisSequentialPatternsOrderType"}, "data-analysis-sequential-patterns-order-type", dataAnalysisSpace, []Words{{"ПоДлине", "ByLength"}, {"ПоКоличествуСлучаев", "BySupport"}}},
}
