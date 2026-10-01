package metadata

import "github.com/k33alexey/MetaLab/internal/uuid"

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

	InformationRegisterRecordSetType  TypeKind = "information-register-record-set"
	AccumulationRegisterRecordSetType TypeKind = "accumulation-register-record-set"
	AccountingRegisterRecordSetType   TypeKind = "accounting-register-record-set"
	CalculationRegisterRecordSetType  TypeKind = "calculation-register-record-set"
	SequenceRecordSetType             TypeKind = "sequence-record-set"
	RecalculationRecordSetType        TypeKind = "recalculation-record-set"

	ConstantValueManagerType TypeKind = "constant-value-manager"

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
)

// The value types: no object of metadata behind them.
const (
	FixedStructureType         TypeKind = "fixed-structure"
	FixedArrayType             TypeKind = "fixed-array"
	FixedMapType               TypeKind = "fixed-map"
	ValueTableType             TypeKind = "value-table"
	ValueTreeType              TypeKind = "value-tree"
	ValueListType              TypeKind = "value-list"
	StandardPeriodType         TypeKind = "standard-period"
	StandardBeginningDateType  TypeKind = "standard-beginning-date"
	ConstantsSetType           TypeKind = "constants-set"
	ReportBuilderType          TypeKind = "report-builder"
	SettingsComposerType       TypeKind = "settings-composer"
	ChartType                  TypeKind = "chart"
	SpreadsheetDocumentType    TypeKind = "spreadsheet-document"
	TypeDescriptionType        TypeKind = "type-description"
	BinaryDataType             TypeKind = "binary-data"
	AccountKindType            TypeKind = "account-kind"
	AccountingRecordKindType   TypeKind = "accounting-record-kind"
	AccumulationRecordKindType TypeKind = "accumulation-record-kind"
)

// objectTypeOwners says, for every object type, which object of metadata its
// reference must name.
var objectTypeOwners = map[TypeKind]func(*Catalog, uuid.UUID) bool{
	CatalogObjectType:                 func(c *Catalog, id uuid.UUID) bool { _, ok := c.catalogByID[id]; return ok },
	DocumentObjectType:                func(c *Catalog, id uuid.UUID) bool { _, ok := c.documentByID[id]; return ok },
	CharacteristicTypesObjectType:     func(c *Catalog, id uuid.UUID) bool { _, ok := c.chartOfCharacteristicTypesByID[id]; return ok },
	AccountsObjectType:                func(c *Catalog, id uuid.UUID) bool { _, ok := c.chartOfAccountsByID[id]; return ok },
	CalculationTypesObjectType:        func(c *Catalog, id uuid.UUID) bool { _, ok := c.chartOfCalculationTypesByID[id]; return ok },
	ExchangePlanObjectType:            func(c *Catalog, id uuid.UUID) bool { _, ok := c.exchangePlanByID[id]; return ok },
	BusinessProcessObjectType:         func(c *Catalog, id uuid.UUID) bool { _, ok := c.businessProcessByID[id]; return ok },
	TaskObjectType:                    func(c *Catalog, id uuid.UUID) bool { _, ok := c.taskByID[id]; return ok },
	InformationRegisterRecordSetType:  func(c *Catalog, id uuid.UUID) bool { _, ok := c.informationRegisterByID[id]; return ok },
	AccumulationRegisterRecordSetType: func(c *Catalog, id uuid.UUID) bool { _, ok := c.accumulationRegisterByID[id]; return ok },
	AccountingRegisterRecordSetType:   func(c *Catalog, id uuid.UUID) bool { _, ok := c.accountingRegisterByID[id]; return ok },
	CalculationRegisterRecordSetType:  func(c *Catalog, id uuid.UUID) bool { _, ok := c.calculationRegisterByID[id]; return ok },
	SequenceRecordSetType:             func(c *Catalog, id uuid.UUID) bool { _, ok := c.sequenceByID[id]; return ok },
	RecalculationRecordSetType:        (*Catalog).hasRecalculation,
	ConstantValueManagerType:          func(c *Catalog, id uuid.UUID) bool { _, ok := c.constantByID[id]; return ok },
	CatalogManagerType:                func(c *Catalog, id uuid.UUID) bool { _, ok := c.catalogByID[id]; return ok },
	DocumentManagerType:               func(c *Catalog, id uuid.UUID) bool { _, ok := c.documentByID[id]; return ok },
	EnumerationManagerType:            func(c *Catalog, id uuid.UUID) bool { _, ok := c.enumerationByID[id]; return ok },
	CharacteristicTypesManagerType:    func(c *Catalog, id uuid.UUID) bool { _, ok := c.chartOfCharacteristicTypesByID[id]; return ok },
	AccountsManagerType:               func(c *Catalog, id uuid.UUID) bool { _, ok := c.chartOfAccountsByID[id]; return ok },
	CalculationTypesManagerType:       func(c *Catalog, id uuid.UUID) bool { _, ok := c.chartOfCalculationTypesByID[id]; return ok },
	ExchangePlanManagerType:           func(c *Catalog, id uuid.UUID) bool { _, ok := c.exchangePlanByID[id]; return ok },
	BusinessProcessManagerType:        func(c *Catalog, id uuid.UUID) bool { _, ok := c.businessProcessByID[id]; return ok },
	TaskManagerType:                   func(c *Catalog, id uuid.UUID) bool { _, ok := c.taskByID[id]; return ok },
	InformationRegisterManagerType:    func(c *Catalog, id uuid.UUID) bool { _, ok := c.informationRegisterByID[id]; return ok },
	AccumulationRegisterManagerType:   func(c *Catalog, id uuid.UUID) bool { _, ok := c.accumulationRegisterByID[id]; return ok },
	AccountingRegisterManagerType:     func(c *Catalog, id uuid.UUID) bool { _, ok := c.accountingRegisterByID[id]; return ok },
	CalculationRegisterManagerType:    func(c *Catalog, id uuid.UUID) bool { _, ok := c.calculationRegisterByID[id]; return ok },
	DocumentJournalManagerType:        func(c *Catalog, id uuid.UUID) bool { _, ok := c.documentJournalByID[id]; return ok },
}

var valueTypeKinds = map[TypeKind]bool{
	FixedStructureType: true, FixedArrayType: true, FixedMapType: true,
	ValueTableType: true, ValueTreeType: true, ValueListType: true,
	StandardPeriodType: true, StandardBeginningDateType: true,
	ConstantsSetType: true, ReportBuilderType: true, SettingsComposerType: true,
	ChartType: true, SpreadsheetDocumentType: true,
	TypeDescriptionType: true, BinaryDataType: true,
	AccountKindType: true, AccountingRecordKindType: true, AccumulationRecordKindType: true,
}

// isObjectType and isValueType say a kind lives in memory only; neither is
// ever a column.
func isObjectType(kind TypeKind) bool { _, ok := objectTypeOwners[kind]; return ok }
func isValueType(kind TypeKind) bool  { return valueTypeKinds[kind] }

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
)

// sessionParameterValueTypes are the value types the help lets a session
// parameter hold.
var sessionParameterValueTypes = map[TypeKind]bool{
	FixedStructureType: true, FixedArrayType: true, FixedMapType: true,
	TypeDescriptionType: true, BinaryDataType: true,
	AccountKindType: true, AccountingRecordKindType: true, AccumulationRecordKindType: true,
}

// allowedIn says whether a kind that lives in memory may stand in the place.
// Stored kinds are not asked: they may stand anywhere.
func allowedIn(kind TypeKind, place typePlace) bool {
	switch place {
	case placeDefinedType, placeRunningObject:
		return true
	case placeSessionParameter:
		return sessionParameterValueTypes[kind]
	default:
		return false
	}
}

// platformFilledSets are the parts of a type description whose composition
// the platform gathers itself, which a defined type may not hold.
var platformFilledSets = map[TypeKind]bool{
	AnyReferenceSet: true, CatalogSet: true, DocumentSet: true, EnumerationSet: true,
	CharacteristicTypesSet: true, AccountSet: true, CalculationTypeSet: true,
	BusinessProcessSet: true, RoutePointSet: true, TaskSet: true, ExchangePlanSet: true,
	CharacteristicSet: true,
}

func placeName(place typePlace) string {
	switch place {
	case placeSessionParameter:
		return "in a session parameter"
	case placeDefinedType:
		return "in a defined type"
	case placeRunningObject:
		return "in an attribute of a data processor or a report"
	default:
		return "in a field the database stores"
	}
}

// hasRecalculation says a recalculation of some calculation register has the
// identifier: a recalculation is subordinate and has no index of its own.
func (catalog *Catalog) hasRecalculation(id uuid.UUID) bool {
	for _, register := range catalog.CalculationRegisters {
		for _, recalculation := range register.Recalculations {
			if recalculation.ID == id {
				return true
			}
		}
	}
	return false
}
