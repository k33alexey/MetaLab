package metadata

import (
	"slices"

	"github.com/k33alexey/MetaLab/internal/kinds"
)

// What the window «Редактирование типа данных» offers where.
//
// The prototype chooses every type in one window, and only what its tree holds
// depends on the place (docs/block-00/obshchee.md, «Окно «Редактирование типа
// данных»», 23.10.2026). Studio keeps the one window too and asks the model
// what to offer in it: Catalog.TypeOffer. What a place offers is never wider
// than what the model accepts there (TestEveryOfferedTypeIsAccepted); the
// model takes more - what earlier versions offered and an import brings.

// TypeChoicePlace is a place of Studio a type is chosen in. Each stands for the
// places the prototype's window offers one list in.
type TypeChoicePlace string

const (
	// StoredFieldChoice is a field the database stores: a constant, an
	// attribute of an object and of its table part.
	StoredFieldChoice TypeChoicePlace = "stored-field"
	// CommandParameterChoice is the type of the parameter of a command.
	CommandParameterChoice TypeChoicePlace = "command-parameter"
	// CharacteristicValueChoice is the value type of the characteristics of a
	// chart of characteristic types.
	CharacteristicValueChoice TypeChoicePlace = "characteristic-value"
	// FilterCriterionChoice is the type of a filter criterion.
	FilterCriterionChoice TypeChoicePlace = "filter-criterion"
	// RunningObjectChoice is an attribute of a data processor or a report, and
	// FormAttributeChoice an attribute of a form: the window offers both one
	// list (snapshots tip-59…62 and redaktor-formy 53, 58), the model checks
	// each where it stands.
	RunningObjectChoice TypeChoicePlace = "running-object"
	FormAttributeChoice TypeChoicePlace = "form-attribute"
	// SessionParameterChoice is the type of a session parameter.
	SessionParameterChoice TypeChoicePlace = "session-parameter"
	// DefinedTypeChoice is the type of a defined type.
	DefinedTypeChoice TypeChoicePlace = "defined-type"
	// SubscriptionSourceChoice is the source of an event subscription.
	SubscriptionSourceChoice TypeChoicePlace = "subscription-source"
	// ExternalFieldChoice is a field of a table of an external data source,
	// and ExternalFunctionChoice the value of a function of one: the window
	// offers both one list, the field without «Составной тип данных».
	ExternalFieldChoice    TypeChoicePlace = "external-field"
	ExternalFunctionChoice TypeChoicePlace = "external-function"
	// CubeDimensionChoice is a dimension of a cube of an external source.
	CubeDimensionChoice TypeChoicePlace = "cube-dimension"
	// NumericResourceChoice is a resource of a register of accumulation, of
	// accounting or of calculation: one window for the three.
	NumericResourceChoice TypeChoicePlace = "numeric-resource"
)

// referenceRows is how a place offers the references of a kind.
type referenceRows int

const (
	// noReferences offers none.
	noReferences referenceRows = iota
	// referencesInFolders offers the objects of the kind under a folder that
	// is not a type itself.
	referencesInFolders
	// referencesUnderSets offers the objects under the set of the kind, which
	// is a type of its own: «СправочникСсылка» marked is any catalog's
	// reference.
	referencesUnderSets
)

// choicePlace is what one place offers, from the top of the tree down.
type choicePlace struct {
	// model is where the model checks what the window builds.
	model typePlace
	// arbitrary is the first row «Произвольный»: no type at all, which the
	// model takes as any.
	arbitrary bool
	// primitives are the rows of the types that name no object.
	primitives []TypeKind
	references referenceRows
	// characteristics is the folder «Характеристика», the charts of
	// characteristic types; definedTypes is the folder «ОпределяемыйТип»;
	// anyReference is the row «ЛюбаяСсылка».
	characteristics, definedTypes, anyReference bool
	// sourceReferences are the kinds of an external source - an object table,
	// a dimension table - whose objects of every source the place offers in
	// a folder of their reference, shown when it holds something.
	sourceReferences []Kind
	// folders are the folders of the types living in memory that name an
	// object - the object of a catalog, the record set of a register, the
	// manager of a catalog - each holding the objects of its kind; then come
	// the types living in memory that name none, values, then the folder
	// «Системные перечисления», holding the enumerations of the platform the
	// model takes in the place: 24 in an attribute of a form or a data
	// processor and in a defined type, three in a session parameter, and no
	// folder where it takes none (opis, «Системные перечисления по местам»).
	// Last come wholeKinds: rows of types naming an object whose box marks
	// the kind as a whole, holding its objects.
	folders    []TypeKind
	values     []TypeKind
	wholeKinds []TypeKind
	// required says the window does not close on «ОК» with nothing marked:
	// «Необходимо выбрать тип значения.». Where the type may be empty it
	// closes silently.
	required bool
	// single is a place without «Составной тип данных»: marking a type takes
	// the one before away.
	single bool
}

// choicePlaces are the places the prototype's window was gone through in,
// with what it offered there (the opis, «Состав по местам»).
var choicePlaces = map[TypeChoicePlace]choicePlace{
	StoredFieldChoice: {model: placeStored, primitives: []TypeKind{NumberType, StringType, DateType, BooleanType, ValueStorageType, UUIDType},
		references: referencesUnderSets, characteristics: true, definedTypes: true, anyReference: true, required: true},
	CommandParameterChoice:    {model: placeCommandParameter, references: referencesInFolders, definedTypes: true},
	CharacteristicValueChoice: {model: placeStored, primitives: []TypeKind{NumberType, StringType, DateType, BooleanType}, references: referencesInFolders, definedTypes: true, required: true},
	FilterCriterionChoice: {model: placeFilterCriterion, arbitrary: true, primitives: []TypeKind{NumberType, StringType, DateType, BooleanType, UUIDType},
		references: referencesUnderSets, characteristics: true, definedTypes: true, anyReference: true},
	RunningObjectChoice: memoryChoice(placeRunningObject),
	FormAttributeChoice: memoryChoice(placeFormAttribute),
	// A session parameter: the stored primitives, binary data and Null, the
	// references with their sets, a type description, the fixed collections
	// and the three enumerations of accounting; no arbitrary, a type
	// required (opis, parametr-seansa.md; snapshots parametr-seansa/04, 05).
	SessionParameterChoice: {model: placeSessionParameter,
		primitives: []TypeKind{NumberType, StringType, DateType, BooleanType, ValueStorageType, UUIDType, BinaryDataType, NullType},
		references: referencesUnderSets, characteristics: true, definedTypes: true, anyReference: true,
		values: []TypeKind{TypeDescriptionType, FixedArrayType, FixedMapType, FixedStructureType}, required: true},
	// A defined type: arbitrary, the primitives and the values of the
	// interface, the references in folders without a box - no set, no
	// characteristic, no defined type, no any reference - the folders of the
	// objects, the record sets and the managers, the values, the
	// enumerations of the platform, and the manager of a settings storage,
	// whose box is the kind as a whole (opis of 24.10.2026,
	// opredelyaemyj-tip.md, «Досъёмка», snapshots series-2.531/01-08). The
	// prototype gives every constant a pair of folders, the key and the
	// manager of its value, one constant in each; the window here has one
	// folder of each type with the constants inside (PLATFORM-MODEL.md,
	// «Выбор типа»).
	DefinedTypeChoice: {model: placeDefinedType, arbitrary: true,
		primitives: []TypeKind{NumberType, StringType, DateType, BooleanType, ValueStorageType, UUIDType, DynamicListType,
			ValueTableType, ValueTreeType, ValueListType, FormattedStringType, BinaryDataType, SpreadsheetDocumentType,
			FormattedDocumentType, TextDocumentType, PDFDocumentType, NullType},
		references: referencesInFolders,
		folders: []TypeKind{CatalogObjectType, DocumentObjectType, CharacteristicTypesObjectType, AccountsObjectType,
			CalculationTypesObjectType, ReportObjectType, DataProcessorObjectType, BusinessProcessObjectType, TaskObjectType,
			ExchangePlanObjectType, InformationRegisterRecordSetType, AccumulationRegisterRecordSetType,
			AccountingRegisterRecordSetType, CalculationRegisterRecordSetType,
			CatalogManagerType, DocumentManagerType, EnumerationManagerType, CharacteristicTypesManagerType, AccountsManagerType,
			CalculationTypesManagerType, ReportManagerType, DataProcessorManagerType, BusinessProcessManagerType, TaskManagerType,
			ExchangePlanManagerType, DocumentJournalManagerType, InformationRegisterManagerType, AccumulationRegisterManagerType,
			AccountingRegisterManagerType, CalculationRegisterManagerType,
			ConstantValueKeyType, ConstantValueManagerType, FilterCriterionManagerType},
		values: []TypeKind{SettingsComposerType, GraphicalSchemaType, GeographicalSchemaType, ChartType, GanttChartType,
			DendrogramType, TypeDescriptionType, PictureType, ColorType, FontType, StandardBeginningDateType, StandardPeriodType,
			LineType, BorderType, FixedArrayType, FixedMapType, FixedStructureType, ChartLabelAreaType, ChartScaleType,
			ChartAxisType, ChartColorPaletteDescriptionType, ChartReferenceLineType, ChartReferenceBandType},
		wholeKinds: []TypeKind{SettingsStorageManagerType}},
	// The source of a subscription: the folder of the defined types, then a
	// row for every kind it listens to, whose box is the kind as a whole and
	// which holds its objects - the objects but those of a report and a data
	// processor, the record sets of the registers and of a recalculation, the
	// managers, and last the record set of a sequence and the manager of a
	// settings storage (opis of 24.10.2026, podpiska-na-sobytie.md,
	// «Досъёмка», snapshots series-2.531/09, 10). A new subscription has no
	// source, so the window closes with nothing marked.
	SubscriptionSourceChoice: {model: placeSubscriptionSource, definedTypes: true,
		wholeKinds: []TypeKind{CatalogObjectType, DocumentObjectType, CharacteristicTypesObjectType, AccountsObjectType,
			CalculationTypesObjectType, BusinessProcessObjectType, TaskObjectType, ExchangePlanObjectType,
			InformationRegisterRecordSetType, AccumulationRegisterRecordSetType, AccountingRegisterRecordSetType,
			CalculationRegisterRecordSetType, RecalculationRecordSetType,
			CatalogManagerType, DocumentManagerType, EnumerationManagerType, CharacteristicTypesManagerType, AccountsManagerType,
			CalculationTypesManagerType, ReportManagerType, DataProcessorManagerType, BusinessProcessManagerType, TaskManagerType,
			ExchangePlanManagerType, DocumentJournalManagerType, InformationRegisterManagerType, AccumulationRegisterManagerType,
			AccountingRegisterManagerType, CalculationRegisterManagerType, ConstantValueManagerType, FilterCriterionManagerType,
			SequenceRecordSetType, SettingsStorageManagerType}},
	// A field of a table of an external source and the value of a function
	// of one: the simple types and binary data, the defined types, and the
	// references to the object tables and the dimension tables of every
	// source - a reference to another source's the designer takes (opis of
	// 24.10.2026, vneshnij-istochnik.md, «Досъёмка (серия 2.531)»,
	// snapshots series-2.531/16, 17, 19, 21-23). A field has no «Составной
	// тип данных» and must have a type; a function may return none.
	ExternalFieldChoice: {model: placeExternalField, primitives: externalPrimitives, definedTypes: true, sourceReferences: sourceTables,
		required: true, single: true},
	ExternalFunctionChoice: {model: placeExternalField, primitives: externalPrimitives, definedTypes: true, sourceReferences: sourceTables},
	// A dimension of a cube: the defined types and the dimension tables of
	// every cube of every source, one of them and no other type (opis of
	// 24.10.2026, vneshnij-istochnik.md, snapshots series-2.531/18, 20). Its
	// types are read as a stored field's; validateCubeDimensionType holds
	// the rest.
	CubeDimensionChoice: {model: placeStored, definedTypes: true, sourceReferences: []Kind{ExternalDimensionTableKind},
		required: true, single: true},
	// A resource of a register of accumulation, of accounting or of
	// calculation: the number and every defined type, each of them alone
	// (opis of 24.10.2026, resurs.md, snapshots series-2.531/11-15). The
	// window has «Составной тип данных», and nothing reaches a composite.
	NumericResourceChoice: {model: placeNumericResource, primitives: []TypeKind{NumberType}, definedTypes: true, required: true},
}

// sourceTables are the kinds whose references a field and a function of a
// source may hold.
var sourceTables = []Kind{ExternalDataSourceTableKind, ExternalDimensionTableKind}

// externalPrimitives are the simple types a field of a source may hold.
var externalPrimitives = []TypeKind{NumberType, StringType, DateType, BooleanType, UUIDType, BinaryDataType}

// memoryChoice is what the window offers an attribute of a data processor, a
// report and a form: arbitrary, the primitives and the values of the
// interface, the references, then the objects and the record sets, the other
// values and the enumerations of the platform (opis, «Состав по местам»,
// snapshots tip-59…62). Neither a value storage nor binary data is offered
// there.
func memoryChoice(model typePlace) choicePlace {
	return choicePlace{model: model, arbitrary: true,
		primitives: []TypeKind{NumberType, StringType, DateType, BooleanType, UUIDType, DynamicListType, ValueTableType,
			ValueTreeType, ValueListType, FormattedStringType, SpreadsheetDocumentType, FormattedDocumentType,
			TextDocumentType, PDFDocumentType, NullType},
		references: referencesUnderSets, characteristics: true, definedTypes: true, anyReference: true,
		folders: []TypeKind{CatalogObjectType, DocumentObjectType, CharacteristicTypesObjectType, AccountsObjectType,
			CalculationTypesObjectType, ReportObjectType, DataProcessorObjectType, BusinessProcessObjectType, TaskObjectType,
			ExchangePlanObjectType, InformationRegisterRecordSetType, InformationRegisterRecordManagerType,
			AccumulationRegisterRecordSetType, AccountingRegisterRecordSetType, CalculationRegisterRecordSetType},
		values: []TypeKind{ConstantsSetType, ReportObjectType, SettingsComposerType, GraphicalSchemaType,
			GeographicalSchemaType, ChartType, GanttChartType, DendrogramType, PlannerType, TypeDescriptionType, PictureType,
			ColorType, FontType, StandardBeginningDateType, StandardPeriodType}}
}

// typeNames are the names of the types the window shows by a word of their own
// rather than by the kind of an object (help, 8.3.27: the pages of the types).
var typeNames = map[TypeKind]kinds.Words{
	NumberType: {Russian: "Число", English: "Number"}, StringType: {Russian: "Строка", English: "String"},
	DateType: {Russian: "Дата", English: "Date"}, BooleanType: {Russian: "Булево", English: "Boolean"},
	ValueStorageType:          {Russian: "ХранилищеЗначения", English: "ValueStorage"},
	UUIDType:                  {Russian: "УникальныйИдентификатор", English: "UUID"},
	AnyReferenceSet:           {Russian: "ЛюбаяСсылка", English: "AnyRef"},
	CharacteristicSet:         {Russian: "Характеристика", English: "Characteristic"},
	DynamicListType:           {Russian: "ДинамическийСписок", English: "DynamicList"},
	ValueTableType:            {Russian: "ТаблицаЗначений", English: "ValueTable"},
	ValueTreeType:             {Russian: "ДеревоЗначений", English: "ValueTree"},
	ValueListType:             {Russian: "СписокЗначений", English: "ValueList"},
	FormattedStringType:       {Russian: "ФорматированнаяСтрока", English: "FormattedString"},
	SpreadsheetDocumentType:   {Russian: "ТабличныйДокумент", English: "SpreadsheetDocument"},
	FormattedDocumentType:     {Russian: "ФорматированныйДокумент", English: "FormattedDocument"},
	TextDocumentType:          {Russian: "ТекстовыйДокумент", English: "TextDocument"},
	NullType:                  {Russian: "Null", English: "Null"},
	ConstantsSetType:          {Russian: "КонстантыНабор", English: "ConstantsSet"},
	SettingsComposerType:      {Russian: "КомпоновщикНастроекКомпоновкиДанных", English: "DataCompositionSettingsComposer"},
	GraphicalSchemaType:       {Russian: "ГрафическаяСхема", English: "GraphicalSchema"},
	GeographicalSchemaType:    {Russian: "ГеографическаяСхема", English: "GeographicalSchema"},
	ChartType:                 {Russian: "Диаграмма", English: "Chart"},
	GanttChartType:            {Russian: "ДиаграммаГанта", English: "GanttChart"},
	DendrogramType:            {Russian: "Дендрограмма", English: "Dendrogram"},
	PlannerType:               {Russian: "Планировщик", English: "Planner"},
	TypeDescriptionType:       {Russian: "ОписаниеТипов", English: "TypeDescription"},
	PictureType:               {Russian: "Картинка", English: "Picture"},
	ColorType:                 {Russian: "Цвет", English: "Color"},
	FontType:                  {Russian: "Шрифт", English: "Font"},
	StandardBeginningDateType: {Russian: "СтандартнаяДатаНачала", English: "StandardBeginningDate"},
	StandardPeriodType:        {Russian: "СтандартныйПериод", English: "StandardPeriod"},
	BinaryDataType:            {Russian: "ДвоичныеДанные", English: "BinaryData"},
	FixedArrayType:            {Russian: "ФиксированныйМассив", English: "FixedArray"},
	FixedMapType:              {Russian: "ФиксированноеСоответствие", English: "FixedMap"},
	FixedStructureType:        {Russian: "ФиксированнаяСтруктура", English: "FixedStructure"},
	// The English names are the names of their types XDTO as well
	// (ownKindsByName).
	PDFDocumentType:                  {Russian: "ДокументPDF", English: "PDFDocument"},
	LineType:                         {Russian: "Линия", English: "Line"},
	BorderType:                       {Russian: "Рамка", English: "Border"},
	ChartLabelAreaType:               {Russian: "ОбластьПодписиДиаграммы", English: "ChartLabelArea"},
	ChartScaleType:                   {Russian: "ШкалаДиаграммы", English: "ChartScale"},
	ChartAxisType:                    {Russian: "ОсьДиаграммы", English: "ChartAxis"},
	ChartColorPaletteDescriptionType: {Russian: "ОписаниеПалитрыЦветовДиаграммы", English: "ChartColorPaletteDescription"},
	ChartReferenceLineType:           {Russian: "ИнформационнаяЛинияДиаграммы", English: "ChartReferenceLine"},
	ChartReferenceBandType:           {Russian: "ИнформационныйИнтервалДиаграммы", English: "ChartReferenceBand"},
	ConstantValueKeyType:             {Russian: "КонстантаКлючЗначения", English: "ConstantValueKey"},
	// The manager of a record has no module, so no ЭтотОбъект names it
	// (typeName).
	InformationRegisterRecordManagerType: {Russian: "РегистрСведенийМенеджерЗаписи", English: "InformationRegisterRecordManager"},
}

// objectTypeModules are the modules whose ЭтотОбъект a type living in memory
// is, by which kinds.Descriptor.ThisObject names the type: the object of a
// catalog is СправочникОбъект, the record set of a register
// РегистрСведенийНаборЗаписей.
var objectTypeModules = map[TypeKind]kinds.Module{
	CatalogObjectType: kinds.ObjectModule, DocumentObjectType: kinds.ObjectModule,
	CharacteristicTypesObjectType: kinds.ObjectModule, AccountsObjectType: kinds.ObjectModule,
	CalculationTypesObjectType: kinds.ObjectModule, ReportObjectType: kinds.ObjectModule,
	DataProcessorObjectType: kinds.ObjectModule, BusinessProcessObjectType: kinds.ObjectModule,
	TaskObjectType: kinds.ObjectModule, ExchangePlanObjectType: kinds.ObjectModule,
	InformationRegisterRecordSetType: kinds.RecordSetModule, AccumulationRegisterRecordSetType: kinds.RecordSetModule,
	AccountingRegisterRecordSetType: kinds.RecordSetModule, CalculationRegisterRecordSetType: kinds.RecordSetModule,
	RecalculationRecordSetType: kinds.RecordSetModule, SequenceRecordSetType: kinds.RecordSetModule,
	ConstantValueManagerType: kinds.ValueModule,
}

// managerTypes are the types of the manager of an object of a kind, which
// kinds.Descriptor.Manager names: СправочникМенеджер, ХранилищеНастроекМенеджер.
var managerTypes = map[TypeKind]bool{
	CatalogManagerType: true, DocumentManagerType: true, EnumerationManagerType: true, CharacteristicTypesManagerType: true,
	AccountsManagerType: true, CalculationTypesManagerType: true, ExchangePlanManagerType: true,
	BusinessProcessManagerType: true, TaskManagerType: true, InformationRegisterManagerType: true,
	AccumulationRegisterManagerType: true, AccountingRegisterManagerType: true, CalculationRegisterManagerType: true,
	DocumentJournalManagerType: true, ReportManagerType: true, DataProcessorManagerType: true,
	SettingsStorageManagerType: true, FilterCriterionManagerType: true,
}

// typeName is the name the window shows a type by: its own word, the type of
// ЭтотОбъект of the module the type is the object of, or the type of the
// manager of the kind.
func typeName(kind TypeKind) kinds.Words {
	if name, ok := typeNames[kind]; ok {
		return name
	}
	if module, ok := objectTypeModules[kind]; ok {
		if object, ok := kinds.Must(typeOwners[kind]).ThisObject(module); ok {
			return object.Name
		}
	}
	if managerTypes[kind] {
		if manager, ok := kinds.Must(typeOwners[kind]).Manager(); ok {
			return manager.Name
		}
	}
	return kinds.Words{}
}

// systemEnumerationsName is the folder of the enumerations of the platform.
// The help names the folder in no language; the English words are ours.
var systemEnumerationsName = kinds.Words{Russian: "Системные перечисления", English: "System enumerations"}

// arbitraryName is the row of no type. The help names the row only in
// Russian; the English word is ours.
var arbitraryName = kinds.Words{Russian: "Произвольный", English: "Arbitrary"}

// TypeOffer is what the window offers in a place, as Studio draws it.
type TypeOffer struct {
	Place TypeChoicePlace `json:"place"`
	// Required refuses «ОК» with nothing marked.
	Required bool `json:"required"`
	// Single is a window without «Составной тип данных».
	Single bool           `json:"single,omitempty"`
	Rows   []TypeOfferRow `json:"rows"`
	Limits TypeLimits     `json:"limits"`
	// NewTypes are the qualifiers a type takes when it is marked anew, by its
	// kind; a kind not listed takes none.
	NewTypes map[TypeKind]Type `json:"newTypes"`
}

// TypeOfferRow is one row of the tree at its top level.
type TypeOfferRow struct {
	// Kind is the type the row's box marks; empty for a folder, which has no
	// box and is not a type.
	Kind TypeKind `json:"kind,omitempty"`
	// Arbitrary is the row of no type: marking it leaves the description
	// empty.
	Arbitrary bool `json:"arbitrary,omitempty"`
	// Alone is a type the place takes only by itself: marked beside another,
	// the window asks whether to exclude the others (aloneTypes). On a folder
	// it is the types of the objects inside.
	Alone bool        `json:"alone,omitempty"`
	Name  kinds.Words `json:"name"`
	// Icon is the picture of the kind of object the row's types name, empty
	// where there is none.
	Icon string `json:"icon,omitempty"`
	// ObjectKind is the type of the rows inside, each naming one of Objects
	// through Type.Reference.
	ObjectKind TypeKind              `json:"objectKind,omitempty"`
	Objects    []ReferenceTypeObject `json:"objects,omitempty"`
	// Types are the rows inside a folder that are types naming no object:
	// the enumerations of the platform.
	Types []TypeOfferRow `json:"types,omitempty"`
}

// TypeLimits are the model's ceilings on the qualifiers, which the window
// checks as the value is entered.
type TypeLimits struct {
	StringLength      int `json:"stringLength"`
	FixedStringLength int `json:"fixedStringLength"`
	NumberDigits      int `json:"numberDigits"`
	BinaryLength      int `json:"binaryLength"`
}

// newTypes are the qualifiers of a type marked anew: a string of 10 of
// variable length, a number of 10 digits and none after the point, without
// «Неотрицательное» (opis, «Тип значения и квалификаторы»), a date of the date
// alone - the first of «Состав даты».
var newTypes = map[TypeKind]Type{
	StringType: {Kind: StringType, Length: 10},
	NumberType: {Kind: NumberType, Precision: 10},
	DateType:   {Kind: DateType, DateParts: DateOnlyParts},
}

// TypeOffer is what the window offers in the place, with the objects of the
// catalog its rows name, and false for a place it does not know.
func (catalog *Catalog) TypeOffer(place TypeChoicePlace) (TypeOffer, bool) {
	at, ok := choicePlaces[place]
	if !ok {
		return TypeOffer{}, false
	}
	offer := TypeOffer{
		Place: place, Required: at.required, Single: at.single,
		Limits: TypeLimits{StringLength: maxStringLength, FixedStringLength: maxFixedStringLength, NumberDigits: maxNumberDigits,
			BinaryLength: maxExternalStringLength},
		NewTypes: newTypes,
	}
	if at.arbitrary {
		offer.Rows = append(offer.Rows, TypeOfferRow{Arbitrary: true, Name: arbitraryName})
	}
	typeRow := func(kind TypeKind) TypeOfferRow {
		return TypeOfferRow{Kind: kind, Name: typeName(kind), Alone: standsAlone(kind, at.model)}
	}
	for _, kind := range at.primitives {
		offer.Rows = append(offer.Rows, typeRow(kind))
	}
	if at.references != noReferences {
		for _, kind := range kinds.ReferenceKinds() {
			info, _ := kindInfoOf(kind)
			descriptor := kinds.Must(kind)
			for index, reference := range info.references {
				row := TypeOfferRow{Name: descriptor.References()[index], Icon: descriptor.Icon, ObjectKind: reference.kind, Objects: catalog.referenceTypeObjects(kind)}
				if at.references == referencesUnderSets {
					row.Kind = reference.set
				}
				offer.Rows = append(offer.Rows, row)
			}
		}
	}
	if at.characteristics {
		offer.Rows = append(offer.Rows, TypeOfferRow{Name: typeNames[CharacteristicSet], ObjectKind: CharacteristicSet,
			Objects: catalog.referenceTypeObjects(ChartOfCharacteristicTypesKind)})
	}
	if at.definedTypes {
		offer.Rows = append(offer.Rows, TypeOfferRow{Name: kinds.Must(DefinedTypeKind).Name, ObjectKind: DefinedType,
			Objects: catalog.referenceTypeObjects(DefinedTypeKind), Alone: standsAlone(DefinedType, at.model)})
	}
	offer.Rows = append(offer.Rows, catalog.sourceReferenceRows(at.sourceReferences)...)
	if at.anyReference {
		offer.Rows = append(offer.Rows, TypeOfferRow{Kind: AnyReferenceSet, Name: typeNames[AnyReferenceSet]})
	}
	for _, kind := range at.folders {
		offer.Rows = append(offer.Rows, TypeOfferRow{Name: typeName(kind), ObjectKind: kind, Objects: catalog.objectsToShow(typeOwners[kind], nil),
			Alone: standsAlone(kind, at.model)})
	}
	for _, kind := range at.values {
		offer.Rows = append(offer.Rows, typeRow(kind))
	}
	folder := TypeOfferRow{Name: systemEnumerationsName}
	for _, enumeration := range kinds.SystemEnumerations {
		if kind := TypeKind(enumeration.Kind); allowedIn(kind, at.model) {
			folder.Types = append(folder.Types, TypeOfferRow{Kind: kind, Name: enumeration.Name})
		}
	}
	if len(folder.Types) > 0 {
		offer.Rows = append(offer.Rows, folder)
	}
	for _, kind := range at.wholeKinds {
		row := typeRow(kind)
		row.ObjectKind, row.Objects = kind, catalog.objectsToShow(typeOwners[kind], nil)
		offer.Rows = append(offer.Rows, row)
	}
	return offer, true
}

// sourceReferenceRows are the folders of the references to something of an
// external source of the kinds asked for: the object tables of every source,
// «Источник.Таблица», and the dimension tables of every cube of every source,
// «Источник.Куб.Таблица», the sources in the order of the tree (snapshot
// series-2.531/22). A folder holding nothing is not shown (snapshot 16).
func (catalog *Catalog) sourceReferenceRows(asked []Kind) []TypeOfferRow {
	var tables, dimensions []ReferenceTypeObject
	for _, listed := range catalog.referenceTypeObjects(ExternalDataSourceKind) {
		source, _ := catalog.ExternalDataSourceByID(listed.ID)
		for _, table := range source.Tables {
			if table.ObjectTable() {
				tables = append(tables, ReferenceTypeObject{ID: table.ID, Name: source.Name + "." + table.Name, Title: table.Title})
			}
		}
		for _, cube := range source.Cubes {
			for _, table := range cube.DimensionTables {
				dimensions = append(dimensions, ReferenceTypeObject{ID: table.ID, Name: source.Name + "." + cube.Name + "." + table.Name, Title: table.Title})
			}
		}
	}
	var rows []TypeOfferRow
	for _, folder := range []struct {
		kind    Kind
		objects []ReferenceTypeObject
	}{{ExternalDataSourceTableKind, tables}, {ExternalDimensionTableKind, dimensions}} {
		if len(folder.objects) > 0 && slices.Contains(asked, folder.kind) {
			descriptor := kinds.Must(folder.kind)
			rows = append(rows, TypeOfferRow{Name: descriptor.References()[0], Icon: descriptor.Icon, ObjectKind: referenceTo(folder.kind),
				Objects: folder.objects})
		}
	}
	return rows
}
