package metadata

import (
	"strings"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The types a value can have in memory and not in the database.
//
// A type description of the prototype holds more than what an attribute
// stores: the object of a document or a catalog, the record set of a
// register, the manager of a constant's value, and values that exist only
// while code runs - a fixed structure, a value table, a standard period. The
// configurations being moved give them to defined types (3748 times, every
// one of them a source of event subscriptions), to session parameters
// (fixed collections, 61 times) and to the attributes of data processors and
// reports (109 times). Without them in the model those descriptions did not
// load at all.
//
// They are carried, not executed: what a value of one of them does is the
// language's, block 5. What the model decides is where each may stand - see
// typePlace - and that none of them reaches a column.

// The object types. Each names the object of metadata it is the object, the
// record set or the manager of, through Type.Reference, the way a reference
// type names its object.
const (
	CatalogObjectType             TypeKind = "catalog-object"
	DocumentObjectType            TypeKind = "document-object"
	CharacteristicTypesObjectType TypeKind = "chart-of-characteristic-types-object"
	AccountsObjectType            TypeKind = "chart-of-accounts-object"
	CalculationTypesObjectType    TypeKind = "chart-of-calculation-types-object"
	ExchangePlanObjectType        TypeKind = "exchange-plan-object"
	BusinessProcessObjectType     TypeKind = "business-process-object"
	TaskObjectType                TypeKind = "task-object"
	// The object of a report or of a data processor, and the manager of one
	// record of an information register: what an attribute of a running
	// object, a defined type and an attribute of a form hold (energy keeps a
	// report object in 13 attributes of its reports; the forms of erp hold
	// data processor objects 443 times, report objects 68, record managers
	// 23).
	ReportObjectType                     TypeKind = "report-object"
	DataProcessorObjectType              TypeKind = "data-processor-object"
	InformationRegisterRecordManagerType TypeKind = "information-register-record-manager"

	InformationRegisterRecordSetType  TypeKind = "information-register-record-set"
	AccumulationRegisterRecordSetType TypeKind = "accumulation-register-record-set"
	AccountingRegisterRecordSetType   TypeKind = "accounting-register-record-set"
	CalculationRegisterRecordSetType  TypeKind = "calculation-register-record-set"
	SequenceRecordSetType             TypeKind = "sequence-record-set"
	RecalculationRecordSetType        TypeKind = "recalculation-record-set"

	ConstantValueManagerType TypeKind = "constant-value-manager"
	// ConstantValueKeyType is the key of the value of a constant, which names
	// the constant (help: КонстантаКлючЗначения.<Имя константы>, 8.3.13). The
	// editor of the type of a defined type offers it, in a composite too, and
	// no other place (opis of 24.10.2026, opredelyaemyj-tip.md, «Досъёмка»);
	// the exports never write it.
	ConstantValueKeyType TypeKind = "constant-value-key"

	CatalogManagerType              TypeKind = "catalog-manager"
	DocumentManagerType             TypeKind = "document-manager"
	EnumerationManagerType          TypeKind = "enumeration-manager"
	CharacteristicTypesManagerType  TypeKind = "chart-of-characteristic-types-manager"
	AccountsManagerType             TypeKind = "chart-of-accounts-manager"
	CalculationTypesManagerType     TypeKind = "chart-of-calculation-types-manager"
	ExchangePlanManagerType         TypeKind = "exchange-plan-manager"
	BusinessProcessManagerType      TypeKind = "business-process-manager"
	TaskManagerType                 TypeKind = "task-manager"
	InformationRegisterManagerType  TypeKind = "information-register-manager"
	AccumulationRegisterManagerType TypeKind = "accumulation-register-manager"
	AccountingRegisterManagerType   TypeKind = "accounting-register-manager"
	CalculationRegisterManagerType  TypeKind = "calculation-register-manager"
	DocumentJournalManagerType      TypeKind = "document-journal-manager"
	// The managers of a report, a data processor, a settings storage and a
	// filter criterion: the editor of the type offers them as the source of
	// a subscription and in a defined type (opis of 14.10.2026).
	ReportManagerType          TypeKind = "report-manager"
	DataProcessorManagerType   TypeKind = "data-processor-manager"
	SettingsStorageManagerType TypeKind = "settings-storage-manager"
	FilterCriterionManagerType TypeKind = "filter-criterion-manager"
)

// The value types: no object of metadata behind them.
const (
	FixedStructureType        TypeKind = "fixed-structure"
	FixedArrayType            TypeKind = "fixed-array"
	FixedMapType              TypeKind = "fixed-map"
	ValueTableType            TypeKind = "value-table"
	ValueTreeType             TypeKind = "value-tree"
	ValueListType             TypeKind = "value-list"
	StandardPeriodType        TypeKind = "standard-period"
	StandardBeginningDateType TypeKind = "standard-beginning-date"
	ConstantsSetType          TypeKind = "constants-set"
	ReportBuilderType         TypeKind = "report-builder"
	SettingsComposerType      TypeKind = "settings-composer"
	ChartType                 TypeKind = "chart"
	SpreadsheetDocumentType   TypeKind = "spreadsheet-document"
	TypeDescriptionType       TypeKind = "type-description"
	BinaryDataType            TypeKind = "binary-data"
	// The enumerations of the platform the model's own rules name; every one
	// of the editor's is a kind as well (systemEnumerationKinds).
	AccountKindType            TypeKind = "account-kind"
	AccountingRecordKindType   TypeKind = "accounting-record-kind"
	AccumulationRecordKindType TypeKind = "accumulation-record-kind"
	HorizontalAlignType        TypeKind = "horizontal-align"
	VerticalAlignType          TypeKind = "vertical-align"
	// PlatformType is a type the platform defines and the model does not list
	// one by one - Отбор, ТипДиаграммы and the rest - written by its name
	// (Type.Name) as the prototype writes it. The help gives an attribute of
	// a report or a data processor an arbitrary type, so a closed list here
	// would refuse what the prototype keeps (energy: Отбор 3, ТипДиаграммы 1).
	// It is carried, not executed, and is a note (NotePlatformTypeByName).
	PlatformType TypeKind = "platform"
	// VanishedType is a type of an object the configuration no longer has:
	// the prototype then writes the identifier of the type instead of its
	// name (v8:TypeId; a command of erp and 16 of a sample of 8.3.21). The
	// identifier is kept in Type.Reference; nothing resolves it. A remnant of
	// what was deleted, and so an error of the project (collectRemnants).
	VanishedType TypeKind = "vanished-type"
)

// The types only a form holds: what the form designer offers for an attribute
// of a form beyond what any other place may hold (help, the form designer and
// the types of form data - ValueTable aside, the eight it names: dynamic list,
// Gantt chart, chart, dendrogram, spreadsheet document, graphical and
// geographical schema), and the values of the interface a form shows. The
// configurations being moved hold them 3451 (dynamic list), 307 (formatted
// string), 271 (colour), 145 (picture), 47 (formatted document), 28 (font), 24
// (text document), 23 (Null), 15 (Gantt chart), 3 (planner, graphical
// schema) and 2 (geographical schema) times, and never anywhere but in a
// form.
const (
	DynamicListType        TypeKind = "dynamic-list"
	FormattedStringType    TypeKind = "formatted-string"
	ColorType              TypeKind = "color"
	FontType               TypeKind = "font"
	PictureType            TypeKind = "picture"
	FormattedDocumentType  TypeKind = "formatted-document"
	TextDocumentType       TypeKind = "text-document"
	GanttChartType         TypeKind = "gantt-chart"
	PlannerType            TypeKind = "planner"
	DendrogramType         TypeKind = "dendrogram"
	GraphicalSchemaType    TypeKind = "graphical-schema"
	GeographicalSchemaType TypeKind = "geographical-schema"
	// NullType is Null as a type of its own: a form holds it beside another
	// type, a value list or a field of a composition, to say the value may be
	// absent (23 times, all in forms). Not "null": a description written in
	// YAML would read that as no kind at all.
	NullType TypeKind = "null-type"
)

// The types of the platform the window of the type offers by rows of their own
// (owner's decision of 24.10.2026, question В2 of the series 2.531;
// PLATFORM-MODEL.md, «Что окно типа предлагает поимённо, модель знает своим
// видом»): the PDF document, the line, the border and six parts of a chart.
// The help (8.3.27) names each and the type XDTO it is written by; the
// window's English name of each is that type's (opis of 23.10.2026,
// opredelyaemyj-tip.md, «Досъёмка»). They live in memory. The window offers the
// PDF document to an attribute of a form, a data processor and a report
// (obshchee.md, «Состав по местам») and all nine to a defined type
// (opredelyaemyj-tip.md), where each stands alone; none of the exports has
// any of them.
const (
	PDFDocumentType                  TypeKind = "pdf-document"
	LineType                         TypeKind = "line"
	BorderType                       TypeKind = "border"
	ChartLabelAreaType               TypeKind = "chart-label-area"
	ChartScaleType                   TypeKind = "chart-scale"
	ChartAxisType                    TypeKind = "chart-axis"
	ChartColorPaletteDescriptionType TypeKind = "chart-color-palette-description"
	ChartReferenceLineType           TypeKind = "chart-reference-line"
	ChartReferenceBandType           TypeKind = "chart-reference-band"
)

// windowPlatformKinds are those types, in the order of the window.
var windowPlatformKinds = []TypeKind{PDFDocumentType, LineType, BorderType, ChartLabelAreaType, ChartScaleType,
	ChartAxisType, ChartColorPaletteDescriptionType, ChartReferenceLineType, ChartReferenceBandType}

// definedTypeOnlyKinds are the kinds living in memory only the editor of a
// defined type offers: those of them but the PDF document, and the key of the
// value of a constant.
var definedTypeOnlyKinds = map[TypeKind]bool{
	ConstantValueKeyType: true, LineType: true, BorderType: true, ChartLabelAreaType: true, ChartScaleType: true, ChartAxisType: true,
	ChartColorPaletteDescriptionType: true, ChartReferenceLineType: true, ChartReferenceBandType: true,
}

var formOnlyKinds = map[TypeKind]bool{
	DynamicListType: true, FormattedStringType: true, ColorType: true, FontType: true, PictureType: true,
	FormattedDocumentType: true, TextDocumentType: true, GanttChartType: true, PlannerType: true,
	DendrogramType: true, GraphicalSchemaType: true, GeographicalSchemaType: true, NullType: true,
}

var valueTypeKinds = map[TypeKind]bool{
	FixedStructureType: true, FixedArrayType: true, FixedMapType: true,
	ValueTableType: true, ValueTreeType: true, ValueListType: true,
	StandardPeriodType: true, StandardBeginningDateType: true,
	ConstantsSetType: true, ReportBuilderType: true, SettingsComposerType: true,
	ChartType: true, SpreadsheetDocumentType: true,
	TypeDescriptionType: true, BinaryDataType: true,
	PlatformType: true,
}

// systemEnumerationKinds are the enumerations of the platform the editor of a
// type offers in its folder «Системные перечисления», by the kind the project
// writes each by (kinds.SystemEnumerations). They live in memory: an
// attribute of a form, of a data processor and of a report and a defined
// type hold every one, a session parameter the three of accounting
// (sessionParameterValueTypes), a stored field none (opis of 24.10.2026).
var systemEnumerationKinds = func() map[TypeKind]kinds.SystemEnumeration {
	byKind := map[TypeKind]kinds.SystemEnumeration{}
	for _, enumeration := range kinds.SystemEnumerations {
		byKind[TypeKind(enumeration.Kind)] = enumeration
	}
	return byKind
}()

// ownKindsByName are the types of the platform the model knows by a kind of its
// own - the enumerations of the platform and windowPlatformKinds - by the name
// of their type XDTO, which a type of the platform written by its name would
// carry.
var ownKindsByName = func() map[string]TypeKind {
	byName := map[string]TypeKind{}
	for kind, enumeration := range systemEnumerationKinds {
		byName[enumeration.Name.English] = kind
	}
	for _, kind := range windowPlatformKinds {
		byName[typeNames[kind].English] = kind
	}
	return byName
}()

func init() {
	for kind := range systemEnumerationKinds {
		valueTypeKinds[kind] = true
	}
	for _, kind := range windowPlatformKinds {
		valueTypeKinds[kind] = true
	}
}

// systemEnumerationValues are the values of an enumeration of the platform as
// the project writes them: ActivePassive as active-passive.
func systemEnumerationValues(kind TypeKind) []string {
	enumeration := systemEnumerationKinds[kind]
	values := make([]string, len(enumeration.Values))
	for index, value := range enumeration.Values {
		values[index] = kebab(value.English)
	}
	return values
}

// ownKindOfPlatformName is the kind of its own a type of the platform written
// by its name stands for, and false when it names none.
func ownKindOfPlatformName(name string) (TypeKind, bool) {
	kind, ok := ownKindsByName[localTypeName(name)]
	return kind, ok
}

// localTypeName is the name of a type of the platform without the prefix of
// its space, which the export chooses itself - d5p1, d7p1 (opis of
// 23.10.2026, opredelyaemyj-tip.md, «Досъёмка») - and which is not compared.
func localTypeName(name string) string {
	if _, local, found := strings.Cut(name, ":"); found {
		return local
	}
	return name
}

// isObjectType and isValueType say a kind lives in memory only; neither is
// ever a column.
func isObjectType(kind TypeKind) bool { return objectTypes[kind] }
func isValueType(kind TypeKind) bool  { return valueTypeKinds[kind] }

// SingleType returns the one type of a type description, and false when there
// is not exactly one. A description may be empty: an attribute of a report or
// a data processor left without a type is arbitrary. So a list of types is
// never indexed directly anywhere - TestNoTypeListIsIndexedDirectly holds every
// reader of a type description to this function.
func SingleType(types []Type) (Type, bool) {
	if len(types) != 1 {
		return Type{}, false
	}
	return types[0], true
}

// isOneStringOrNumber says a type description is a single string or a single
// number.
func isOneStringOrNumber(types []Type) bool {
	single, ok := SingleType(types)
	return ok && (single.Kind == StringType || single.Kind == NumberType)
}

// typePlace is where a type description stands, because what it may hold
// depends on that and on nothing else.
type typePlace int

const (
	// placeStored is a field written to the database: an attribute of a
	// catalog or a document, a dimension, a resource, a constant. Only what
	// can be a column.
	placeStored typePlace = iota
	// placeDefinedType is the type of a defined type. Anything a field of any
	// place may hold, objects and values mixed - checked in the designer by
	// the owner on 01.10.2026 - except another defined type and the sets the
	// platform fills itself, which the help forbids.
	placeDefinedType
	// placeSessionParameter is the type of a session parameter: what the help
	// lists - the stored types, the fixed collections, a type description,
	// binary data and the kinds of account and of movement.
	placeSessionParameter
	// placeRunningObject is an attribute of a data processor or a report
	// itself, which lives as long as the object runs: the help gives it an
	// arbitrary type.
	placeRunningObject
	// placeCommandParameter is the parameter of a command: what the command is
	// offered beside and handed when it runs. Never stored - so a reference to
	// a table of an external source and the type of a vanished object may
	// stand there - and never an object or a value of memory.
	placeCommandParameter
	// placeFilterCriterion is the type of a filter criterion: what its
	// content is searched by. It takes what a stored field takes; what may
	// stand beside what in it the designer was not asked (aloneTypes).
	placeFilterCriterion
	// placeFormAttribute is an attribute of a form or a column of one: data
	// the form holds while it is open. Like a running object's attribute it
	// may be left with no type, which is an arbitrary one, and may hold what
	// lives in memory only and what the infobase does not store - the help
	// names a reference to a table of an external source as allowed in an
	// attribute of a managed form.
	placeFormAttribute
	// placeExternalField is a field of a table of an external data source: a
	// column of the other database, never one of ours. It takes what a stored
	// field takes and binary data besides, with its qualifiers - the
	// configurator writes such fields (lombard1, 58), and the editor of the
	// field's type gives the simple types (opis, vneshnij-istochnik.md).
	placeExternalField
	// placeSubscriptionSource is the source of an event subscription: the
	// objects, record sets and managers it listens to. It is checked by
	// validateEventSubscription, not by validateTypesIn; the window of the
	// type offers what that takes.
	placeSubscriptionSource
)

// storedNowhere says a kind may stand in a type description of something the
// infobase does not store - a running object, a command parameter, a defined
// type - and in no stored field: a reference to an external source is a key
// of another database (the help: «may be used in attributes of a managed
// form»), and a vanished type has nothing to point at. The schema refuses
// either when a defined type brings it into a stored field.
func storedNowhere(kind TypeKind) bool {
	return storedAs(kind) == notStored || kind == VanishedType
}

// mayStandUnstored says where a kind of storedNowhere is allowed.
func mayStandUnstored(place typePlace) bool {
	return place == placeRunningObject || place == placeCommandParameter || place == placeDefinedType || place == placeFormAttribute
}

// sessionParameterValueTypes are the types beyond the stored ones the help
// lets a session parameter hold. The help («ПараметрыСеанса.<Имя параметра
// сеанса>») lists the stored types - number, string, date, boolean, value
// storage, unique identifier, any reference - and besides them the three
// fixed collections, a type description, binary data, the kind of account,
// the kinds of accounting and accumulation movement, Null and Undefined.
// Undefined is no type of its own: it is what a composite type holds when it
// holds nothing. Null the designer offers in the editor of the type, alone
// and in a composite, and «Проверка конфигурации» takes both (opis of the
// session parameter, 14.10.2026).
var sessionParameterValueTypes = map[TypeKind]bool{
	FixedStructureType: true, FixedArrayType: true, FixedMapType: true,
	TypeDescriptionType: true, BinaryDataType: true,
	AccountKindType: true, AccountingRecordKindType: true, AccumulationRecordKindType: true,
	NullType: true,
}

// aloneTypes are, by place, the types that stand in a type description only by
// themselves: the editor of the type asked to put one beside another type
// warns «Выбран тип, который не может быть включен в составной тип данных»
// and excludes the others (opis of 14.10.2026, every type tried one by one,
// snapshots sostavnoj-tip 04, 05, 10, 11; METADATA-OBJECTS.md, «Типы,
// которые не входят в составной»). «Произвольный» is one too, and alone by
// being no type at all. A session parameter has none - the designer takes
// all of its types in a composite. The managers of a defined type are the
// kinds' (kindInfo.aloneInDefinedType), added in init. The type of a filter
// criterion takes a unique identifier alone: beside a string the editor warns
// «Выбран тип, который не может быть включен в составной тип данных» (opis
// of 23.10.2026, kriterij-otbora.md, «Досъёмка»). An attribute of a form and
// of a data processor takes nothing alone (the same pass).
var aloneTypes = map[typePlace]map[TypeKind]bool{
	placeStored:          {ValueStorageType: true, UUIDType: true},
	placeFilterCriterion: {UUIDType: true},
	placeDefinedType: {
		ValueStorageType: true, UUIDType: true, BinaryDataType: true, NullType: true,
		DynamicListType: true, SpreadsheetDocumentType: true, FormattedDocumentType: true, TextDocumentType: true,
		PictureType: true, ColorType: true, FontType: true, GraphicalSchemaType: true, GeographicalSchemaType: true,
		ChartType: true, GanttChartType: true, DendrogramType: true, SettingsComposerType: true, StandardPeriodType: true,
		PDFDocumentType: true, LineType: true, BorderType: true, ChartLabelAreaType: true, ChartScaleType: true,
		ChartAxisType: true, ChartColorPaletteDescriptionType: true, ChartReferenceLineType: true, ChartReferenceBandType: true,
	},
}

// standsAlone says a kind stands in the place only by itself.
func standsAlone(kind TypeKind, place typePlace) bool { return aloneTypes[place][kind] }

// formKindAllowedIn says whether a kind of formOnlyKinds may stand in the
// place. An attribute of a data processor or a report holds every one of them,
// as one of a form: the editor of its type offers the same list (opis of
// 23.10.2026, «Состав по местам», snapshots tip-59…62). A defined type holds
// every one of them but the planner: the designer offers them in its editor
// of the type and saves them, and a search of that editor finds no planner
// (opis of the defined type, 14.10.2026). Null is the one of them a session
// parameter may hold too.
func formKindAllowedIn(kind TypeKind, place typePlace) bool {
	switch place {
	case placeFormAttribute, placeRunningObject:
		return true
	case placeDefinedType:
		return kind != PlannerType
	case placeSessionParameter:
		return sessionParameterValueTypes[kind]
	default:
		return false
	}
}

// notInDefinedType are the kinds living in memory a defined type does not hold.
var notInDefinedType = map[TypeKind]bool{
	InformationRegisterRecordManagerType: true, RecalculationRecordSetType: true, SequenceRecordSetType: true,
}

// allowedIn says whether a kind that lives in memory may stand in the place.
// Stored kinds are not asked: they may stand anywhere.
func allowedIn(kind TypeKind, place typePlace) bool {
	switch place {
	case placeDefinedType:
		// The editor of the type of a defined type does not offer the
		// manager of a record of a register of information, nor the record
		// sets of a recalculation and of a sequence: its search finds none
		// of them (opis of the defined type, 14.10.2026, «Нет в редакторе»).
		return !notInDefinedType[kind]
	case placeRunningObject, placeFormAttribute:
		return !definedTypeOnlyKinds[kind]
	case placeSessionParameter:
		return sessionParameterValueTypes[kind]
	case placeExternalField:
		return kind == BinaryDataType
	default:
		return false
	}
}

func placeName(place typePlace) string {
	switch place {
	case placeSessionParameter:
		return "in a session parameter"
	case placeDefinedType:
		return "in a defined type"
	case placeRunningObject:
		return "in an attribute of a data processor or a report"
	case placeCommandParameter:
		return "in the parameter of a command"
	case placeFormAttribute:
		return "in an attribute of a form"
	case placeFilterCriterion:
		return "in the type of a filter criterion"
	case placeExternalField:
		return "in a field of an external data source"
	case placeSubscriptionSource:
		return "in the source of a subscription"
	default:
		return "in a field the database stores"
	}
}

// hasRecalculation says a recalculation of some calculation register has the
// identifier.
func (catalog *Catalog) hasRecalculation(id uuid.UUID) bool {
	_, ok := catalog.recalculationOf(id)
	return ok
}

// recalculationOf finds a recalculation of some calculation register by its
// identifier: a recalculation is subordinate and has no index of its own.
func (catalog *Catalog) recalculationOf(id uuid.UUID) (Recalculation, bool) {
	for _, register := range catalog.CalculationRegisters {
		for _, recalculation := range register.Recalculations {
			if recalculation.ID == id {
				return recalculation, true
			}
		}
	}
	return Recalculation{}, false
}
