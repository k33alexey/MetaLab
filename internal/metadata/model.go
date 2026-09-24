// Package metadata defines validated application metadata loaded from an ML Project.
package metadata

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
	"go.yaml.in/yaml/v3"
)

const (
	CurrentFormat      = 1
	maxObjectsPerKind  = 100_000
	maxEnumerationVals = 1 << 20
)

type Kind string

const (
	ConstantKind             Kind = "constants"
	SessionParameterKind     Kind = "session-parameters"
	EnumerationKind          Kind = "enumerations"
	DefinedTypeKind          Kind = "defined-types"
	CatalogKind              Kind = "catalogs"
	DocumentKind             Kind = "documents"
	InformationRegisterKind  Kind = "information-registers"
	AccumulationRegisterKind Kind = "accumulation-registers"
	// ChartOfCharacteristicTypesKind holds the kinds of characteristic a
	// configuration lets its users invent without changing the configuration.
	ChartOfCharacteristicTypesKind Kind = "charts-of-characteristic-types"
	// ChartOfAccountsKind holds the accounts an application keeps its books on.
	ChartOfAccountsKind Kind = "charts-of-accounts"
	// ChartOfCalculationTypesKind holds kinds of accrual and deduction.
	ChartOfCalculationTypesKind Kind = "charts-of-calculation-types"
	// BusinessProcessKind holds routes a process walks; TaskKind holds the
	// assignments created along the way.
	BusinessProcessKind Kind = "business-processes"
	TaskKind            Kind = "tasks"
	// ExchangePlanKind holds what enters an exchange and who it is exchanged
	// with.
	ExchangePlanKind Kind = "exchange-plans"
	// The three kinds that live around documents: numbering shared by several
	// of them, the order they must be posted in, and a common list of them.
	// AccountingRegisterKind holds entries: an account on each side, sums and
	// the analytics behind them.
	AccountingRegisterKind Kind = "accounting-registers"
	// CalculationRegisterKind holds results of calculation, with the period
	// they act over and the displacement between them.
	CalculationRegisterKind Kind = "calculation-registers"
	// ReportKind and DataProcessorKind keep no data: their attributes and
	// table parts live only while the object runs.
	ReportKind          Kind = "reports"
	DataProcessorKind   Kind = "data-processors"
	NumeratorKind       Kind = "document-numerators"
	SequenceKind        Kind = "sequences"
	DocumentJournalKind Kind = "document-journals"
)

type TypeKind string

const (
	StringType  TypeKind = "string"
	NumberType  TypeKind = "number"
	BooleanType TypeKind = "boolean"
	DateType    TypeKind = "date"
	// ObjectUUIDType is how the platform stores its own object identity -
	// record identifiers, references, the current-user session parameter. It
	// is not a type a developer can give an attribute: the prototype has no
	// such attribute type, and a reference declared as a bare identifier
	// loses referential integrity, presentation, filtering and input by
	// string - everything that makes a reference worth having.
	ObjectUUIDType  TypeKind = "obj-uuid"
	EnumerationType TypeKind = "enumeration"
	DefinedType     TypeKind = "defined-type"
	CatalogType     TypeKind = "catalog"
	DocumentType    TypeKind = "document"
	// CharacteristicTypesType is a reference to one kind of characteristic -
	// an element of a chart of characteristic types, the way a catalog
	// reference points at one element of a catalog.
	CharacteristicTypesType TypeKind = "chart-of-characteristic-types"
	// AccountType is a reference to one account of a chart of accounts.
	AccountType TypeKind = "chart-of-accounts"
	// CalculationTypeType is a reference to one kind of accrual or deduction.
	CalculationTypeType TypeKind = "chart-of-calculation-types"
	// BusinessProcessType and TaskType are references to one process and one
	// task; RoutePointType is a reference to a point of a process's route -
	// there is no object kind behind it, the process itself produces it.
	BusinessProcessType TypeKind = "business-process"
	TaskType            TypeKind = "task"
	// ExchangePlanType is a reference to one node of an exchange plan.
	ExchangePlanType TypeKind = "exchange-plan"
	RoutePointType   TypeKind = "route-point"
	// The eleven reference sets. A set is not the name of a type: it is an
	// element of a type description in its own right, standing beside concrete
	// types and mixing freely with them. What a set contains is never written
	// down in the attribute - it is read off the configuration, so an object
	// added tomorrow falls into the set by itself, without anybody reopening
	// the attribute that uses it.
	AnyReferenceSet        TypeKind = "any-ref"
	CatalogSet             TypeKind = "catalog-ref"
	DocumentSet            TypeKind = "document-ref"
	EnumerationSet         TypeKind = "enumeration-ref"
	CharacteristicTypesSet TypeKind = "chart-of-characteristic-types-ref"
	AccountSet             TypeKind = "chart-of-accounts-ref"
	CalculationTypeSet     TypeKind = "chart-of-calculation-types-ref"
	BusinessProcessSet     TypeKind = "business-process-ref"
	RoutePointSet          TypeKind = "route-point-ref"
	TaskSet                TypeKind = "task-ref"
	ExchangePlanSet        TypeKind = "exchange-plan-ref"
	// CharacteristicSet is the twelfth set and the thirteenth element: what it
	// contains is decided by a chart of characteristic types, not by a kind of
	// object. DefinedType is the other one of that pair.
	CharacteristicSet TypeKind = "characteristic"
	// ValueStorageType holds a value of any shape, opaque to the database.
	// It is storable but cannot be form data - reading it costs a round trip
	// and it has no presentation to show in a field.
	ValueStorageType TypeKind = "value-storage"
)

// referenceSets are the sets read off the kind of object alone. The two that
// are read off the configuration instead - a defined type and a characteristic
// - carry a reference and are handled beside them.
var referenceSets = map[TypeKind]bool{
	AnyReferenceSet: true, CatalogSet: true, DocumentSet: true, EnumerationSet: true,
	CharacteristicTypesSet: true, AccountSet: true, CalculationTypeSet: true,
	BusinessProcessSet: true, RoutePointSet: true, TaskSet: true, ExchangePlanSet: true,
}

// IsTypeSet says whether an element of a type description is a set rather than
// a concrete type. Storage turns on this: a set stores the way a composite
// type stores, whatever it happens to contain today.
func IsTypeSet(kind TypeKind) bool {
	return referenceSets[kind] || kind == CharacteristicSet || kind == DefinedType
}

// HierarchyKind is what a parent may be. With folders and items only a folder
// may be a parent, and an element is one or the other; with items alone every
// element is equal and any of them may be a parent.
type HierarchyKind string

const (
	FoldersAndItemsHierarchy HierarchyKind = "folders-and-items"
	ItemsHierarchy           HierarchyKind = "items"
)

// CodeSeries says within what a code is unique and auto-numbered: the whole
// object, or one level of subordination.
type CodeSeries string

const (
	WholeObjectSeries      CodeSeries = "whole"
	SubordinationSeries    CodeSeries = "within-subordination"
	maxHierarchyLevelCount            = 32
)

// DateParts is the prototype's "состав даты" qualifier: which parts of a
// moment an attribute is about. Storage does not change - a date is always a
// moment - but what is shown, entered and compared does.
type DateParts string

const (
	DateAndTimeParts DateParts = "date-time"
	DateOnlyParts    DateParts = "date"
	TimeOnlyParts    DateParts = "time"
)

var (
	ErrUnsupportedFormat = errors.New("unsupported metadata format")
	ErrDuplicateName     = errors.New("duplicate metadata name")
	ErrDuplicateID       = errors.New("duplicate metadata UUID")
)

// LocalizedText stores translations by configured language code.
type LocalizedText map[string]string

// Resolve returns a translation by a fixed chain: the language asked for, then
// the same language without its region, then the project's default language,
// then the configured languages in order, and finally any non-empty value at
// all.
//
// The chain is fixed on purpose. It used to fall through the configured list
// straight away, so which translation a user saw when theirs was empty depended
// on the order languages happened to be stored in - two projects with the same
// languages could answer differently, and neither answer was the project's own
// default. What a reader gets when their language is missing is a decision, and
// it belongs to the project.
func (text LocalizedText) Resolve(language, defaultLanguage string, configured []project.Language) string {
	if value := strings.TrimSpace(text[language]); value != "" {
		return value
	}
	// "uk-UA" reads a translation stored as "uk", and the other way round: a
	// region is a refinement of a language, not a different one.
	if base, _, ok := strings.Cut(language, "-"); ok && base != "" {
		if value := strings.TrimSpace(text[base]); value != "" {
			return value
		}
	}
	// A stored "uk-UA" answers a request for "uk" as well: the two are the same
	// language, and which side carries the region is an accident of how each was
	// written down.
	if requested := languageBase(language); requested != "" {
		for _, key := range sortedKeys(text) {
			if languageBase(key) != requested {
				continue
			}
			if value := strings.TrimSpace(text[key]); value != "" {
				return value
			}
		}
	}
	if value := strings.TrimSpace(text[defaultLanguage]); value != "" {
		return value
	}
	for _, item := range configured {
		if value := strings.TrimSpace(text[item.Code]); value != "" {
			return value
		}
	}
	for _, key := range sortedKeys(text) {
		if value := strings.TrimSpace(text[key]); value != "" {
			return value
		}
	}
	return ""
}

// sortedKeys keeps every fallback that walks the stored translations
// deterministic - two readers with the same data must get the same answer.
func sortedKeys(text LocalizedText) []string {
	keys := make([]string, 0, len(text))
	for key := range text {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// TitleLanguage is the whole question "in which language" - the reader's
// language together with the project's own fallbacks. It travels as one value
// because the three parts are never useful apart: a component handed only a
// language code cannot answer correctly when that translation is missing, which
// is exactly when the answer matters.
type TitleLanguage struct {
	Code       string             `json:"code"`
	Default    string             `json:"default"`
	Configured []project.Language `json:"configured,omitempty"`
}

// ProjectLanguage returns the chain for one project and one reader.
func ProjectLanguage(manifest project.Project, code string) TitleLanguage {
	return TitleLanguage{Code: code, Default: manifest.DefaultLanguage, Configured: manifest.Languages}
}

// Resolve answers with this chain.
func (chain TitleLanguage) Resolve(text LocalizedText) string {
	return text.Resolve(chain.Code, chain.Default, chain.Configured)
}

// languageBase drops the region from a locale code: "uk-UA" and "uk" are the
// same language.
func languageBase(code string) string {
	base, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(code)), "-")
	return base
}

// Type describes one allowed scalar value. References use stable metadata UUIDs.
type Type struct {
	Kind      TypeKind   `yaml:"kind" json:"kind"`
	Reference *uuid.UUID `yaml:"reference,omitempty" json:"reference,omitempty"`
	Length    int        `yaml:"length,omitempty" json:"length,omitempty"`
	Precision int        `yaml:"precision,omitempty" json:"precision,omitempty"`
	Scale     int        `yaml:"scale,omitempty" json:"scale,omitempty"`
	// Qualifiers, one per prototype qualifier that changes meaning rather
	// than kind: a fixed-length string is padded and compared as such, a
	// non-negative number refuses negatives on write, and date parts say
	// which half of a moment the attribute is actually about.
	FixedLength bool      `yaml:"fixed_length,omitempty" json:"fixedLength,omitempty"`
	NonNegative bool      `yaml:"non_negative,omitempty" json:"nonNegative,omitempty"`
	DateParts   DateParts `yaml:"date_parts,omitempty" json:"dateParts,omitempty"`
}

type Constant struct {
	Format  int           `yaml:"format"`
	ID      uuid.UUID     `yaml:"id"`
	Name    string        `yaml:"name"`
	Title   LocalizedText `yaml:"title"`
	Types   []Type        `yaml:"types"`
	Default *Value        `yaml:"default,omitempty"`
}

// SessionParameter is a server-memory-only value scoped to one session's lifetime.
// Unlike Constant it is never persisted to PostgreSQL.
type SessionParameter struct {
	Format  int           `yaml:"format"`
	ID      uuid.UUID     `yaml:"id"`
	Name    string        `yaml:"name"`
	Title   LocalizedText `yaml:"title"`
	Types   []Type        `yaml:"types"`
	Default *Value        `yaml:"default,omitempty"`
}

type EnumerationValue struct {
	ID      uuid.UUID     `yaml:"id"`
	Name    string        `yaml:"name"`
	Title   LocalizedText `yaml:"title"`
	Comment string        `yaml:"comment,omitempty"`
}

// ChoiceMode says how a value of a reference kind is picked: from a list that
// drops down under the field, from a form opened for the purpose, or either
// way.
type ChoiceMode string

const (
	ChoiceBothWays  ChoiceMode = "both-ways"
	ChoiceFromForm  ChoiceMode = "from-form"
	ChoiceQuickOnly ChoiceMode = "quick-choice"
)

// ChoiceHistory says whether what the user picked before is offered first.
type ChoiceHistory string

const (
	ChoiceHistoryAuto    ChoiceHistory = "auto"
	ChoiceHistoryUse     ChoiceHistory = "use"
	ChoiceHistoryDontUse ChoiceHistory = "dont-use"
)

// EnumerationForms are the forms an enumeration shows itself through. It has
// no form of a single value: a value is not edited, it is written by the
// developer and only ever chosen.
//
// Each of the two has an auxiliary form beside it - a second list or a second
// choice form, used where the main one does not fit.
type EnumerationForms struct {
	List            *uuid.UUID `yaml:"list,omitempty" json:"list,omitempty"`
	Choice          *uuid.UUID `yaml:"choice,omitempty" json:"choice,omitempty"`
	AuxiliaryList   *uuid.UUID `yaml:"auxiliary_list,omitempty" json:"auxiliaryList,omitempty"`
	AuxiliaryChoice *uuid.UUID `yaml:"auxiliary_choice,omitempty" json:"auxiliaryChoice,omitempty"`
}

// Enumeration is a closed list of values the developer writes and the user
// cannot change. That is all it keeps of its own - but it is shown, chosen
// from and acted upon like any other object, so it has presentations, forms,
// a manager module, commands and templates the same way.
type Enumeration struct {
	Format  int           `yaml:"format"`
	ID      uuid.UUID     `yaml:"id"`
	Name    string        `yaml:"name"`
	Title   LocalizedText `yaml:"title"`
	Comment string        `yaml:"comment,omitempty"`
	// Explanation is the sentence shown where the object is offered - in the
	// panel of actions, next to the command that opens it.
	Explanation LocalizedText `yaml:"explanation,omitempty"`
	// ListPresentation names the list of values for the user; the extended one
	// is used where there is room for a longer wording.
	ListPresentation         LocalizedText      `yaml:"list_presentation,omitempty"`
	ExtendedListPresentation LocalizedText      `yaml:"extended_list_presentation,omitempty"`
	Values                   []EnumerationValue `yaml:"values"`
	// How a value of this enumeration is picked where it is asked for.
	ChoiceMode           ChoiceMode    `yaml:"choice_mode,omitempty"`
	QuickChoice          bool          `yaml:"quick_choice,omitempty"`
	ChoiceHistoryOnInput ChoiceHistory `yaml:"choice_history_on_input,omitempty"`
	// UseStandardCommands decides whether the platform offers its own commands
	// for this object - opening the list and the rest.
	UseStandardCommands bool             `yaml:"use_standard_commands,omitempty"`
	ManagerModule       *uuid.UUID       `yaml:"manager_module,omitempty"`
	Forms               EnumerationForms `yaml:"forms,omitempty"`
	Commands            []ObjectCommand  `yaml:"commands,omitempty"`
	Templates           []ObjectTemplate `yaml:"templates,omitempty"`
}

type DefinedTypeObject struct {
	Format int           `yaml:"format"`
	ID     uuid.UUID     `yaml:"id"`
	Name   string        `yaml:"name"`
	Title  LocalizedText `yaml:"title"`
	Types  []Type        `yaml:"types"`
}

type CatalogCode struct {
	Type   TypeKind `yaml:"type" json:"type"`
	Length int      `yaml:"length" json:"length"`
	Auto   bool     `yaml:"auto" json:"auto"`
	Unique bool     `yaml:"unique" json:"unique"`
}

// Hierarchy is how an object nests inside itself. It is off by default: a flat
// object is the common case, and a hierarchy nobody asked for costs a column
// and an index on every table.
type Hierarchy struct {
	Enabled bool          `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Kind    HierarchyKind `yaml:"kind,omitempty" json:"kind,omitempty"`
	// FoldersOnTop is about showing, not storing: folders stand above items in
	// a hierarchical list. It says nothing about who may be a parent.
	FoldersOnTop bool `yaml:"folders_on_top,omitempty" json:"foldersOnTop,omitempty"`
	// LimitLevels caps how deep the nesting may go; without it depth is
	// unlimited.
	LimitLevels bool `yaml:"limit_levels,omitempty" json:"limitLevels,omitempty"`
	LevelCount  int  `yaml:"level_count,omitempty" json:"levelCount,omitempty"`
	// Series says whether an automatic code is unique across the whole object
	// or within one parent.
	Series CodeSeries `yaml:"series,omitempty" json:"series,omitempty"`
}

// PredefinedCatalogItem binds configuration identity to one stable catalog reference.
type PredefinedCatalogItem struct {
	ID          uuid.UUID `yaml:"id" json:"id"`
	Name        string    `yaml:"name" json:"name"`
	Code        string    `yaml:"code,omitempty" json:"code,omitempty"`
	Description string    `yaml:"description,omitempty" json:"description,omitempty"`
	// Parent names the predefined item this one sits under, and IsFolder says
	// it is a folder rather than an item. A configuration brings whole trees
	// of predefined data, and flattening them on the way in would lose the
	// only thing that made them a tree.
	Parent     string           `yaml:"parent,omitempty" json:"parent,omitempty"`
	IsFolder   bool             `yaml:"is_folder,omitempty" json:"isFolder,omitempty"`
	Attributes map[string]Value `yaml:"attributes,omitempty" json:"attributes,omitempty"`
}

type Attribute struct {
	ID       uuid.UUID     `yaml:"id" json:"id"`
	Name     string        `yaml:"name" json:"name"`
	Title    LocalizedText `yaml:"title" json:"title"`
	Types    []Type        `yaml:"types" json:"types"`
	Required bool          `yaml:"required,omitempty" json:"required,omitempty"`
	Indexed  bool          `yaml:"indexed,omitempty" json:"indexed,omitempty"`
}

type TablePart struct {
	ID         uuid.UUID     `yaml:"id" json:"id"`
	Name       string        `yaml:"name" json:"name"`
	Title      LocalizedText `yaml:"title" json:"title"`
	Attributes []Attribute   `yaml:"attributes" json:"attributes"`
}

// ListSettings controls bounded server-side lists without embedding SQL in metadata.
type ListSettings struct {
	PageSize     int      `yaml:"page_size,omitempty" json:"pageSize,omitempty"`
	SearchFields []string `yaml:"search_fields,omitempty" json:"searchFields,omitempty"`
}

// CatalogDefinition describes one ML catalog and its persistent record shape.
type CatalogDefinition struct {
	Format            int                     `yaml:"format" json:"format"`
	ID                uuid.UUID               `yaml:"id" json:"id"`
	Name              string                  `yaml:"name" json:"name"`
	Title             LocalizedText           `yaml:"title" json:"title"`
	Code              CatalogCode             `yaml:"code" json:"code"`
	DescriptionLength int                     `yaml:"description_length" json:"descriptionLength"`
	Hierarchy         Hierarchy               `yaml:"hierarchy,omitempty" json:"hierarchy,omitempty"`
	Attributes        []Attribute             `yaml:"attributes,omitempty" json:"attributes,omitempty"`
	TableParts        []TablePart             `yaml:"table_parts,omitempty" json:"tableParts,omitempty"`
	ObjectModule      *uuid.UUID              `yaml:"object_module,omitempty" json:"objectModule,omitempty"`
	ManagerModule     *uuid.UUID              `yaml:"manager_module,omitempty" json:"managerModule,omitempty"`
	Forms             ObjectForms             `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands          []ObjectCommand         `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates         []ObjectTemplate        `yaml:"templates,omitempty" json:"templates,omitempty"`
	List              ListSettings            `yaml:"list,omitempty" json:"list,omitempty"`
	Predefined        []PredefinedCatalogItem `yaml:"predefined,omitempty" json:"predefined,omitempty"`
}

// Catalog is an immutable-by-convention snapshot of the supported metadata kinds.
type Catalog struct {
	Project                          project.Project
	Roles                            []RoleDefinition
	roleByName                       map[string]int
	roleByID                         map[uuid.UUID]int
	Subsystems                       []SubsystemDefinition
	subsystemByName                  map[string]int
	subsystemByID                    map[uuid.UUID]int
	Constants                        []Constant
	SessionParameters                []SessionParameter
	sessionParameterByName           map[string]int
	sessionParameterByID             map[uuid.UUID]int
	CommonAttributes                 []CommonAttributeDefinition
	commonAttributeByName            map[string]int
	commonAttributeByID              map[uuid.UUID]int
	CommonModules                    []CommonModuleDefinition
	commonModuleByName               map[string]int
	commonModuleByID                 map[uuid.UUID]int
	commonModuleByModuleID           map[uuid.UUID]int
	EventSubscriptions               []EventSubscriptionDefinition
	eventSubscriptionByName          map[string]int
	eventSubscriptionByID            map[uuid.UUID]int
	Enumerations                     []Enumeration
	DefinedTypes                     []DefinedTypeObject
	Catalogs                         []CatalogDefinition
	Documents                        []DocumentDefinition
	ChartsOfCharacteristicTypes      []ChartOfCharacteristicTypesDefinition
	ChartsOfAccounts                 []ChartOfAccountsDefinition
	ChartsOfCalculationTypes         []ChartOfCalculationTypesDefinition
	BusinessProcesses                []BusinessProcessDefinition
	Tasks                            []TaskDefinition
	ExchangePlans                    []ExchangePlanDefinition
	Numerators                       []NumeratorDefinition
	Sequences                        []SequenceDefinition
	DocumentJournals                 []DocumentJournalDefinition
	InformationRegisters             []InformationRegisterDefinition
	AccumulationRegisters            []AccumulationRegisterDefinition
	AccountingRegisters              []AccountingRegisterDefinition
	CalculationRegisters             []CalculationRegisterDefinition
	Reports                          []ReportDefinition
	DataProcessors                   []DataProcessorDefinition
	FunctionalOptions                []FunctionalOptionDefinition
	functionalOptionByName           map[string]int
	functionalOptionByID             map[uuid.UUID]int
	constantByName                   map[string]int
	constantByID                     map[uuid.UUID]int
	enumerationByName                map[string]int
	definedTypeByName                map[string]int
	enumerationByID                  map[uuid.UUID]int
	definedTypeByID                  map[uuid.UUID]int
	catalogByName                    map[string]int
	catalogByID                      map[uuid.UUID]int
	documentByName                   map[string]int
	documentByID                     map[uuid.UUID]int
	informationRegisterByName        map[string]int
	informationRegisterByID          map[uuid.UUID]int
	accumulationRegisterByName       map[string]int
	accumulationRegisterByID         map[uuid.UUID]int
	chartOfCharacteristicTypesByName map[string]int
	chartOfCharacteristicTypesByID   map[uuid.UUID]int
	chartOfAccountsByName            map[string]int
	chartOfAccountsByID              map[uuid.UUID]int
	chartOfCalculationTypesByName    map[string]int
	chartOfCalculationTypesByID      map[uuid.UUID]int
	businessProcessByName            map[string]int
	businessProcessByID              map[uuid.UUID]int
	taskByName                       map[string]int
	taskByID                         map[uuid.UUID]int
	exchangePlanByName               map[string]int
	exchangePlanByID                 map[uuid.UUID]int
	numeratorByName                  map[string]int
	numeratorByID                    map[uuid.UUID]int
	sequenceByName                   map[string]int
	sequenceByID                     map[uuid.UUID]int
	documentJournalByName            map[string]int
	documentJournalByID              map[uuid.UUID]int
	accountingRegisterByName         map[string]int
	accountingRegisterByID           map[uuid.UUID]int
	calculationRegisterByName        map[string]int
	calculationRegisterByID          map[uuid.UUID]int
	reportByName                     map[string]int
	reportByID                       map[uuid.UUID]int
	dataProcessorByName              map[string]int
	dataProcessorByID                map[uuid.UUID]int
}

func (catalog *Catalog) ConstantByID(id uuid.UUID) (Constant, bool) {
	index, ok := catalog.constantByID[id]
	if !ok {
		return Constant{}, false
	}
	return cloneConstant(catalog.Constants[index]), true
}

func (catalog *Catalog) ConstantIDs() []uuid.UUID {
	if len(catalog.Constants) == 0 {
		return nil
	}
	result := make([]uuid.UUID, len(catalog.Constants))
	for index := range catalog.Constants {
		result[index] = catalog.Constants[index].ID
	}
	return result
}

func (catalog *Catalog) Constant(name string) (Constant, bool) {
	index, ok := catalog.constantByName[strings.ToLower(name)]
	if !ok {
		return Constant{}, false
	}
	return cloneConstant(catalog.Constants[index]), true
}

func (catalog *Catalog) SessionParameterByID(id uuid.UUID) (SessionParameter, bool) {
	index, ok := catalog.sessionParameterByID[id]
	if !ok {
		return SessionParameter{}, false
	}
	return cloneSessionParameter(catalog.SessionParameters[index]), true
}

func (catalog *Catalog) SessionParameter(name string) (SessionParameter, bool) {
	index, ok := catalog.sessionParameterByName[strings.ToLower(name)]
	if !ok {
		return SessionParameter{}, false
	}
	return cloneSessionParameter(catalog.SessionParameters[index]), true
}

func (catalog *Catalog) Enumeration(name string) (Enumeration, bool) {
	index, ok := catalog.enumerationByName[strings.ToLower(name)]
	if !ok {
		return Enumeration{}, false
	}
	return cloneEnumeration(catalog.Enumerations[index]), true
}

func (catalog *Catalog) EnumerationValue(enumeration, value string) (EnumerationValue, bool) {
	item, ok := catalog.Enumeration(enumeration)
	if !ok {
		return EnumerationValue{}, false
	}
	for _, candidate := range item.Values {
		if strings.EqualFold(candidate.Name, value) {
			candidate.Title = cloneTitle(candidate.Title)
			return candidate, true
		}
	}
	return EnumerationValue{}, false
}

// ResolvedTypes expands nested defined types into concrete allowed types.
func (catalog *Catalog) ResolvedTypes(name string) ([]Type, bool) {
	item, ok := catalog.DefinedType(name)
	if !ok {
		return nil, false
	}
	types, err := catalog.expandTypes(item.Types, nil)
	return cloneTypes(types), err == nil
}

func (catalog *Catalog) DefinedType(name string) (DefinedTypeObject, bool) {
	index, ok := catalog.definedTypeByName[strings.ToLower(name)]
	if !ok {
		return DefinedTypeObject{}, false
	}
	return cloneDefinedType(catalog.DefinedTypes[index]), true
}

func (catalog *Catalog) CatalogDefinition(name string) (CatalogDefinition, bool) {
	index, ok := catalog.catalogByName[strings.ToLower(name)]
	if !ok {
		return CatalogDefinition{}, false
	}
	return cloneCatalogDefinition(catalog.Catalogs[index]), true
}

func (catalog *Catalog) CatalogByID(id uuid.UUID) (CatalogDefinition, bool) {
	index, ok := catalog.catalogByID[id]
	if !ok {
		return CatalogDefinition{}, false
	}
	return cloneCatalogDefinition(catalog.Catalogs[index]), true
}

func (catalog *Catalog) CatalogIDs() []uuid.UUID {
	if len(catalog.Catalogs) == 0 {
		return nil
	}
	result := make([]uuid.UUID, len(catalog.Catalogs))
	for index := range catalog.Catalogs {
		result[index] = catalog.Catalogs[index].ID
	}
	return result
}

func (definition CatalogDefinition) PredefinedItem(name string) (PredefinedCatalogItem, bool) {
	for _, item := range definition.Predefined {
		if strings.EqualFold(item.Name, name) {
			return clonePredefinedCatalogItem(item), true
		}
	}
	return PredefinedCatalogItem{}, false
}

func (definition CatalogDefinition) PredefinedByID(id uuid.UUID) (PredefinedCatalogItem, bool) {
	for _, item := range definition.Predefined {
		if item.ID == id {
			return clonePredefinedCatalogItem(item), true
		}
	}
	return PredefinedCatalogItem{}, false
}

func (catalog *Catalog) DocumentDefinition(name string) (DocumentDefinition, bool) {
	index, ok := catalog.documentByName[strings.ToLower(name)]
	if !ok {
		return DocumentDefinition{}, false
	}
	return cloneDocumentDefinition(catalog.Documents[index]), true
}

func (catalog *Catalog) DocumentByID(id uuid.UUID) (DocumentDefinition, bool) {
	index, ok := catalog.documentByID[id]
	if !ok {
		return DocumentDefinition{}, false
	}
	return cloneDocumentDefinition(catalog.Documents[index]), true
}

func (catalog *Catalog) DocumentIDs() []uuid.UUID {
	if len(catalog.Documents) == 0 {
		return nil
	}
	result := make([]uuid.UUID, len(catalog.Documents))
	for index := range catalog.Documents {
		result[index] = catalog.Documents[index].ID
	}
	return result
}

func (catalog *Catalog) InformationRegisterDefinition(name string) (InformationRegisterDefinition, bool) {
	index, ok := catalog.informationRegisterByName[strings.ToLower(name)]
	if !ok {
		return InformationRegisterDefinition{}, false
	}
	return cloneInformationRegisterDefinition(catalog.InformationRegisters[index]), true
}

func (catalog *Catalog) InformationRegisterByID(id uuid.UUID) (InformationRegisterDefinition, bool) {
	index, ok := catalog.informationRegisterByID[id]
	if !ok {
		return InformationRegisterDefinition{}, false
	}
	return cloneInformationRegisterDefinition(catalog.InformationRegisters[index]), true
}

func (catalog *Catalog) InformationRegisterIDs() []uuid.UUID {
	if len(catalog.InformationRegisters) == 0 {
		return nil
	}
	result := make([]uuid.UUID, len(catalog.InformationRegisters))
	for index := range catalog.InformationRegisters {
		result[index] = catalog.InformationRegisters[index].ID
	}
	return result
}

func (catalog *Catalog) AccumulationRegisterDefinition(name string) (AccumulationRegisterDefinition, bool) {
	index, ok := catalog.accumulationRegisterByName[strings.ToLower(name)]
	if !ok {
		return AccumulationRegisterDefinition{}, false
	}
	return cloneAccumulationRegisterDefinition(catalog.AccumulationRegisters[index]), true
}

func (catalog *Catalog) AccumulationRegisterByID(id uuid.UUID) (AccumulationRegisterDefinition, bool) {
	index, ok := catalog.accumulationRegisterByID[id]
	if !ok {
		return AccumulationRegisterDefinition{}, false
	}
	return cloneAccumulationRegisterDefinition(catalog.AccumulationRegisters[index]), true
}

func (catalog *Catalog) AccumulationRegisterIDs() []uuid.UUID {
	if len(catalog.AccumulationRegisters) == 0 {
		return nil
	}
	result := make([]uuid.UUID, len(catalog.AccumulationRegisters))
	for index := range catalog.AccumulationRegisters {
		result[index] = catalog.AccumulationRegisters[index].ID
	}
	return result
}

func DecodeConstant(source string, reader io.Reader, manifest project.Project) (Constant, error) {
	var value Constant
	if err := decodeStrict(source, reader, &value); err != nil {
		return Constant{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	issues = append(issues, validateTypes("types", value.Types, uuid.UUID{})...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return Constant{}, err
	}
	return value, nil
}

func DecodeSessionParameter(source string, reader io.Reader, manifest project.Project) (SessionParameter, error) {
	var value SessionParameter
	if err := decodeStrict(source, reader, &value); err != nil {
		return SessionParameter{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	issues = append(issues, validateTypes("types", value.Types, uuid.UUID{})...)
	if ReservedSessionParameter(value.Name) {
		// The platform resolves this name itself on every read path, without
		// running BSL. Letting a project declare it too would make the value
		// depend on which layer happened to answer first.
		issues = append(issues, fmt.Sprintf("name %q is reserved by the platform", value.Name))
	}
	if err := issuesError(source, value.Format, issues); err != nil {
		return SessionParameter{}, err
	}
	return value, nil
}

func DecodeEnumeration(source string, reader io.Reader, manifest project.Project) (Enumeration, error) {
	var value Enumeration
	if err := decodeStrict(source, reader, &value); err != nil {
		return Enumeration{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	if len(value.Values) == 0 || len(value.Values) > maxEnumerationVals {
		issues = append(issues, "values must contain 1..1048576 items")
	}
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for index, item := range value.Values {
		prefix := fmt.Sprintf("values[%d]", index)
		if item.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if ids[item.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		ids[item.ID] = true
		if !validIdentifier(item.Name) {
			issues = append(issues, prefix+".name must be a valid identifier")
		}
		folded := strings.ToLower(item.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		names[folded] = true
		issues = append(issues, validateTitle(prefix+".title", item.Title, manifest)...)
	}
	for name, text := range map[string]LocalizedText{
		"explanation": value.Explanation, "list_presentation": value.ListPresentation,
		"extended_list_presentation": value.ExtendedListPresentation,
	} {
		if len(text) > 0 {
			issues = append(issues, validateTitle(name, text, manifest)...)
		}
	}
	switch value.ChoiceMode {
	case "", ChoiceBothWays, ChoiceFromForm, ChoiceQuickOnly:
	default:
		issues = append(issues, "choice_mode must be both-ways, from-form or quick-choice")
	}
	switch value.ChoiceHistoryOnInput {
	case "", ChoiceHistoryAuto, ChoiceHistoryUse, ChoiceHistoryDontUse:
	default:
		issues = append(issues, "choice_history_on_input must be auto, use or dont-use")
	}
	// Choosing only from a form and offering a quick choice are two answers to
	// one question, and the second one is then never asked.
	if value.ChoiceMode == ChoiceFromForm && value.QuickChoice {
		issues = append(issues, "quick_choice contradicts choice_mode from-form")
	}
	for name, id := range map[string]*uuid.UUID{
		"manager_module": value.ManagerModule, "forms.list": value.Forms.List,
		"forms.choice": value.Forms.Choice, "forms.auxiliary_list": value.Forms.AuxiliaryList,
		"forms.auxiliary_choice": value.Forms.AuxiliaryChoice,
	} {
		if id != nil && id.IsZero() {
			issues = append(issues, name+" must be a non-zero UUID")
		}
	}
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, manifest, value.ManagerModule)...)
	issues = append(issues, validateObjectTemplates(value.Templates, manifest)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return Enumeration{}, err
	}
	return value, nil
}

func DecodeDefinedType(source string, reader io.Reader, manifest project.Project) (DefinedTypeObject, error) {
	var value DefinedTypeObject
	if err := decodeStrict(source, reader, &value); err != nil {
		return DefinedTypeObject{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	issues = append(issues, validateTypes("types", value.Types, value.ID)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return DefinedTypeObject{}, err
	}
	return value, nil
}

func DecodeCatalog(source string, reader io.Reader, manifest project.Project) (CatalogDefinition, error) {
	var value CatalogDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return CatalogDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	issues = append(issues, validateReferenceObjectShape(referenceObjectShape{
		code:              value.Code,
		descriptionLength: value.DescriptionLength,
		attributes:        value.Attributes,
		tableParts:        value.TableParts,
		objectModule:      value.ObjectModule,
		managerModule:     value.ManagerModule,
		forms:             value.Forms,
		list:              value.List,
		hierarchy:         value.Hierarchy,
		predefined:        value.Predefined,
		reservedName:      reservedCatalogObjectName,
	}, manifest)...)
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, manifest, value.ObjectModule, value.ManagerModule)...)
	issues = append(issues, validateObjectTemplates(value.Templates, manifest)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return CatalogDefinition{}, err
	}
	return value, nil
}

// referenceObjectShape is everything a reference object repeats from the
// catalog: a code, a description, attributes, table parts, its own modules,
// forms, list settings and predefined elements. Charts of characteristic
// types, charts of accounts, charts of calculation types, business processes
// and tasks all repeat it, and each adds its own on top - so the repeated part
// is checked in one place rather than copied per kind, where the copies drift.
type referenceObjectShape struct {
	code              CatalogCode
	descriptionLength int
	attributes        []Attribute
	tableParts        []TablePart
	objectModule      *uuid.UUID
	managerModule     *uuid.UUID
	forms             ObjectForms
	list              ListSettings
	hierarchy         Hierarchy
	predefined        []PredefinedCatalogItem
	// reservedName says which attribute names the kind keeps for itself. A
	// kind with standard attributes of its own passes its own answer.
	reservedName func(string) bool
}

// validateHierarchy checks the settings against each other, because each of
// them is meaningless without the one it depends on: a kind of hierarchy on a
// flat object, a level count nobody limits, folders on top where there are no
// folders. Left unchecked, such a setting reads as working and does nothing.
func validateHierarchy(hierarchy Hierarchy) []string {
	var issues []string
	if !hierarchy.Enabled {
		if hierarchy.Kind != "" || hierarchy.FoldersOnTop || hierarchy.LimitLevels || hierarchy.LevelCount != 0 {
			issues = append(issues, "hierarchy settings need hierarchy.enabled")
		}
		return issues
	}
	switch hierarchy.Kind {
	case FoldersAndItemsHierarchy, ItemsHierarchy:
	case "":
		issues = append(issues, "hierarchy.kind is required: folders-and-items or items")
	default:
		issues = append(issues, "hierarchy.kind must be folders-and-items or items")
	}
	if hierarchy.FoldersOnTop && hierarchy.Kind == ItemsHierarchy {
		issues = append(issues, "hierarchy.folders_on_top is meaningless without folders")
	}
	if hierarchy.LimitLevels && (hierarchy.LevelCount < 1 || hierarchy.LevelCount > maxHierarchyLevelCount) {
		issues = append(issues, fmt.Sprintf("hierarchy.level_count must be 1..%d when levels are limited", maxHierarchyLevelCount))
	}
	if !hierarchy.LimitLevels && hierarchy.LevelCount != 0 {
		issues = append(issues, "hierarchy.level_count needs hierarchy.limit_levels")
	}
	switch hierarchy.Series {
	case "", WholeObjectSeries, SubordinationSeries:
	default:
		issues = append(issues, "hierarchy.series must be whole or within-subordination")
	}
	return issues
}

func validateReferenceObjectShape(shape referenceObjectShape, manifest project.Project) []string {
	var issues []string
	switch shape.code.Type {
	case StringType:
		if shape.code.Length < 1 || shape.code.Length > 128 {
			issues = append(issues, "code.length must be 1..128 for string codes")
		}
	case NumberType:
		if shape.code.Length < 1 || shape.code.Length > 38 {
			issues = append(issues, "code.length must be 1..38 for number codes")
		}
	default:
		issues = append(issues, "code.type must be string or number")
	}
	if shape.descriptionLength < 1 || shape.descriptionLength > 1_048_576 {
		issues = append(issues, "description_length must be 1..1048576")
	}
	issues = append(issues, validateHierarchy(shape.hierarchy)...)
	reserved := shape.reservedName
	if reserved == nil {
		reserved = reservedCatalogObjectName
	}
	issues = append(issues, validateAttributes("attributes", shape.attributes, manifest, reserved)...)
	attributeNames := make(map[string]bool, len(shape.attributes))
	for _, attribute := range shape.attributes {
		attributeNames[strings.ToLower(attribute.Name)] = true
	}
	if len(shape.tableParts) > 128 {
		issues = append(issues, "table_parts must not contain more than 128 items")
	}
	partNames, partIDs := map[string]bool{}, map[uuid.UUID]bool{}
	for index, part := range shape.tableParts {
		prefix := fmt.Sprintf("table_parts[%d]", index)
		if part.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if partIDs[part.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		partIDs[part.ID] = true
		if !validIdentifier(part.Name) {
			issues = append(issues, prefix+".name must be a valid identifier")
		}
		folded := strings.ToLower(part.Name)
		if partNames[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		if reserved(folded) {
			issues = append(issues, prefix+".name is reserved")
		}
		if attributeNames[folded] {
			issues = append(issues, prefix+".name conflicts with an attribute")
		}
		partNames[folded] = true
		issues = append(issues, validateTitle(prefix+".title", part.Title, manifest)...)
		issues = append(issues, validateAttributes(prefix+".attributes", part.Attributes, manifest, nil)...)
	}
	for name, module := range map[string]*uuid.UUID{"object_module": shape.objectModule, "manager_module": shape.managerModule} {
		if module != nil && module.IsZero() {
			issues = append(issues, name+" must be a non-zero UUID")
		}
	}
	if shape.objectModule != nil && shape.managerModule != nil && *shape.objectModule == *shape.managerModule {
		issues = append(issues, "object_module and manager_module must be different")
	}
	issues = append(issues, validateObjectForms(shape.forms)...)
	issues = append(issues, validateListSettings(shape.list, shape.attributes, map[string]TypeKind{
		"code": shape.code.Type, "description": StringType,
	})...)
	return append(issues, validatePredefinedItems(shape)...)
}

func validatePredefinedItems(shape referenceObjectShape) []string {
	if len(shape.predefined) > maxObjectsPerKind {
		return []string{fmt.Sprintf("predefined must not contain more than %d items", maxObjectsPerKind)}
	}
	var issues []string
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for index, item := range shape.predefined {
		prefix := fmt.Sprintf("predefined[%d]", index)
		if item.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if ids[item.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		ids[item.ID] = true
		if !validIdentifier(item.Name) || utf8.RuneCountInString(item.Name) > 128 {
			issues = append(issues, prefix+".name must be a valid identifier of at most 128 characters")
		}
		folded := strings.ToLower(item.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		names[folded] = true
		if item.IsFolder && shape.hierarchy.Kind != FoldersAndItemsHierarchy {
			issues = append(issues, prefix+".is_folder needs a hierarchy of folders and items")
		}
		if item.Parent != "" && !shape.hierarchy.Enabled {
			issues = append(issues, prefix+".parent needs a hierarchy")
		}
		if item.Code == "" {
			if !shape.code.Auto {
				issues = append(issues, prefix+".code is required when automatic codes are disabled")
			}
		} else if _, err := normalizeCatalogCode(shape.code, item.Code); err != nil {
			issues = append(issues, prefix+".code is invalid: "+err.Error())
		}
		if !utf8.ValidString(item.Description) || utf8.RuneCountInString(item.Description) > shape.descriptionLength {
			issues = append(issues, fmt.Sprintf("%s.description must not exceed %d characters", prefix, shape.descriptionLength))
		}
		attributeNames := map[string]bool{}
		for name := range item.Attributes {
			attribute, ok := findCatalogAttribute(shape.attributes, name)
			if !ok {
				issues = append(issues, prefix+".attributes."+name+" is unknown")
				continue
			}
			key := strings.ToLower(attribute.Name)
			if attributeNames[key] {
				issues = append(issues, prefix+".attributes contains duplicate "+attribute.Name)
			}
			attributeNames[key] = true
		}
		for _, attribute := range shape.attributes {
			if attribute.Required && !attributeNames[strings.ToLower(attribute.Name)] {
				issues = append(issues, prefix+".attributes."+attribute.Name+" is required")
			}
		}
	}
	return issues
}

func validateAttributes(path string, attributes []Attribute, manifest project.Project, reserved func(string) bool) []string {
	if len(attributes) > 1024 {
		return []string{path + " must not contain more than 1024 items"}
	}
	var issues []string
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for index, attribute := range attributes {
		prefix := fmt.Sprintf("%s[%d]", path, index)
		if attribute.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if ids[attribute.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		ids[attribute.ID] = true
		if !validIdentifier(attribute.Name) {
			issues = append(issues, prefix+".name must be a valid identifier")
		}
		folded := strings.ToLower(attribute.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		if reserved != nil && reserved(folded) {
			issues = append(issues, prefix+".name is reserved")
		}
		names[folded] = true
		issues = append(issues, validateTitle(prefix+".title", attribute.Title, manifest)...)
		issues = append(issues, validateTypes(prefix+".types", attribute.Types, uuid.UUID{})...)
	}
	return issues
}

func reservedCatalogObjectName(name string) bool {
	switch strings.ToLower(name) {
	case "ссылка", "ref", "код", "code", "наименование", "description", "версия", "version",
		"пометкаудаления", "deletionmark", "имяпредопределенныхданных", "имяпредопределённыхданных", "predefineddataname":
		return true
	default:
		return false
	}
}

func Encode(writer io.Writer, value any) error {
	var content bytes.Buffer
	encoder := yaml.NewEncoder(&content)
	encoder.SetIndent(2)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode metadata: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("close metadata encoder: %w", err)
	}
	if content.Len() > project.MaxYAMLDocumentBytes {
		return project.ErrYAMLDocumentTooLarge
	}
	_, err := io.Copy(writer, &content)
	return err
}

func decodeStrict(source string, reader io.Reader, target any) error {
	content, err := io.ReadAll(io.LimitReader(reader, project.MaxYAMLDocumentBytes+1))
	if err != nil {
		return fmt.Errorf("read %s: %w", source, err)
	}
	if len(content) > project.MaxYAMLDocumentBytes {
		return fmt.Errorf("decode %s: %w", source, project.ErrYAMLDocumentTooLarge)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", source, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return fmt.Errorf("decode trailing YAML in %s: %w", source, err)
		}
		return fmt.Errorf("decode %s: multiple YAML documents are not allowed", source)
	}
	return nil
}

func validateBase(format int, id uuid.UUID, name string, title LocalizedText, manifest project.Project) []string {
	var issues []string
	if format != CurrentFormat {
		issues = append(issues, fmt.Sprintf("format must be %d", CurrentFormat))
	}
	if id.IsZero() {
		issues = append(issues, "id must be a non-zero UUID")
	}
	if !validIdentifier(name) {
		issues = append(issues, "name must start with a letter and contain only letters or digits")
	}
	if utf8.RuneCountInString(name) > 128 {
		issues = append(issues, "name must not exceed 128 characters")
	}
	return append(issues, validateTitle("title", title, manifest)...)
}

func validateTitle(path string, title LocalizedText, manifest project.Project) []string {
	var issues []string
	configured := make(map[string]bool, len(manifest.Languages))
	for _, language := range manifest.Languages {
		configured[language.Code] = true
	}
	if len(title) == 0 {
		return []string{path + " must contain at least one translation"}
	}
	for language, value := range title {
		if !configured[language] {
			issues = append(issues, path+"."+language+" uses an unconfigured language")
		}
		if !validText(value, 512) {
			issues = append(issues, path+"."+language+" must contain 1..512 printable characters")
		}
	}
	return issues
}

func validateTypes(path string, types []Type, self uuid.UUID) []string {
	if len(types) == 0 || len(types) > 32 {
		return []string{path + " must contain 1..32 types"}
	}
	var issues []string
	seen := map[string]bool{}
	for index, item := range types {
		prefix := fmt.Sprintf("%s[%d]", path, index)
		key := string(item.Kind)
		if item.Reference != nil {
			key += ":" + item.Reference.String()
		}
		if seen[key] {
			issues = append(issues, prefix+" duplicates an allowed type")
		}
		seen[key] = true
		referenced := item.Kind == CharacteristicSet ||
			item.Kind == EnumerationType || item.Kind == DefinedType || item.Kind == CatalogType ||
			item.Kind == DocumentType || item.Kind == CharacteristicTypesType || item.Kind == AccountType ||
			item.Kind == CalculationTypeType || item.Kind == BusinessProcessType || item.Kind == TaskType ||
			item.Kind == ExchangePlanType ||
			item.Kind == RoutePointType
		if referenced && (item.Reference == nil || item.Reference.IsZero()) {
			issues = append(issues, prefix+".reference is required")
		}
		if !referenced && item.Reference != nil {
			issues = append(issues, prefix+".reference is not allowed")
		}
		if (item.Kind == DefinedType || item.Kind == CharacteristicSet) && item.Reference != nil && *item.Reference == self {
			issues = append(issues, prefix+" cannot reference itself")
		}
		if item.Kind == ObjectUUIDType {
			issues = append(issues, prefix+".kind obj-uuid is reserved for platform identity and cannot be declared; use a reference type")
			continue
		}
		if item.DateParts != "" && item.Kind != DateType {
			issues = append(issues, prefix+".date_parts is allowed for dates only")
		}
		if item.FixedLength && item.Kind != StringType {
			issues = append(issues, prefix+".fixed_length is allowed for strings only")
		}
		if item.NonNegative && item.Kind != NumberType {
			issues = append(issues, prefix+".non_negative is allowed for numbers only")
		}
		switch item.Kind {
		case StringType:
			if item.Length < 0 || item.Length > 1_048_576 {
				issues = append(issues, prefix+".length must be 0..1048576")
			}
			if item.Precision != 0 || item.Scale != 0 {
				issues = append(issues, prefix+" has invalid numeric qualifiers")
			}
			// A fixed-length string is padded to its length, so there has to
			// be a length to pad to.
			if item.FixedLength && item.Length == 0 {
				issues = append(issues, prefix+".fixed_length requires a length")
			}
		case NumberType:
			if item.Precision < 1 || item.Precision > 38 {
				issues = append(issues, prefix+".precision must be 1..38")
			}
			if item.Scale < 0 || item.Scale > item.Precision {
				issues = append(issues, prefix+".scale must be 0..precision")
			}
			if item.Length != 0 {
				issues = append(issues, prefix+".length is not allowed")
			}
		case DateType:
			switch item.DateParts {
			case "", DateAndTimeParts, DateOnlyParts, TimeOnlyParts:
			default:
				issues = append(issues, prefix+".date_parts must be date, time or date-time")
			}
			if item.Length != 0 || item.Precision != 0 || item.Scale != 0 {
				issues = append(issues, prefix+" has unsupported qualifiers")
			}
		case BooleanType, ValueStorageType, EnumerationType, DefinedType, CatalogType, DocumentType, CharacteristicTypesType, AccountType, CalculationTypeType,
			BusinessProcessType, TaskType, ExchangePlanType, RoutePointType,
			AnyReferenceSet, CatalogSet, DocumentSet, EnumerationSet, CharacteristicTypesSet, AccountSet,
			CalculationTypeSet, BusinessProcessSet, RoutePointSet, TaskSet, ExchangePlanSet, CharacteristicSet:
			if item.Length != 0 || item.Precision != 0 || item.Scale != 0 {
				issues = append(issues, prefix+" has unsupported qualifiers")
			}
		default:
			issues = append(issues, prefix+".kind is unsupported")
		}
	}
	return issues
}

func issuesError(source string, format int, issues []string) error {
	if len(issues) == 0 {
		return nil
	}
	err := fmt.Errorf("validate %s: %s", source, strings.Join(issues, "; "))
	if format != CurrentFormat {
		return fmt.Errorf("%w: %w", ErrUnsupportedFormat, err)
	}
	return err
}

func validIdentifier(value string) bool {
	for index, symbol := range []rune(value) {
		if index == 0 && !unicode.IsLetter(symbol) {
			return false
		}
		if !unicode.IsLetter(symbol) && !unicode.IsDigit(symbol) {
			return false
		}
	}
	return value != ""
}

func validText(value string, maximum int) bool {
	if !utf8.ValidString(value) || strings.TrimSpace(value) == "" || utf8.RuneCountInString(value) > maximum {
		return false
	}
	for _, symbol := range value {
		if unicode.IsControl(symbol) {
			return false
		}
	}
	return true
}

func cloneTypes(items []Type) []Type {
	result := slices.Clone(items)
	for index := range result {
		if result[index].Reference != nil {
			reference := *result[index].Reference
			result[index].Reference = &reference
		}
	}
	return result
}
func cloneTitle(value LocalizedText) LocalizedText {
	result := make(LocalizedText, len(value))
	for key, text := range value {
		result[key] = text
	}
	return result
}
func cloneConstant(value Constant) Constant {
	value.Title, value.Types = cloneTitle(value.Title), cloneTypes(value.Types)
	if value.Default != nil {
		defaultValue := *value.Default
		value.Default = &defaultValue
	}
	return value
}
func cloneSessionParameter(value SessionParameter) SessionParameter {
	value.Title, value.Types = cloneTitle(value.Title), cloneTypes(value.Types)
	if value.Default != nil {
		defaultValue := *value.Default
		value.Default = &defaultValue
	}
	return value
}
func cloneEnumeration(value Enumeration) Enumeration {
	value.Title = cloneTitle(value.Title)
	value.Explanation = cloneTitle(value.Explanation)
	value.ListPresentation = cloneTitle(value.ListPresentation)
	value.ExtendedListPresentation = cloneTitle(value.ExtendedListPresentation)
	value.Values = slices.Clone(value.Values)
	for index := range value.Values {
		value.Values[index].Title = cloneTitle(value.Values[index].Title)
	}
	for _, id := range []**uuid.UUID{&value.ManagerModule, &value.Forms.List, &value.Forms.Choice,
		&value.Forms.AuxiliaryList, &value.Forms.AuxiliaryChoice} {
		if *id != nil {
			copied := **id
			*id = &copied
		}
	}
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
	return value
}
func cloneDefinedType(value DefinedTypeObject) DefinedTypeObject {
	value.Title, value.Types = cloneTitle(value.Title), cloneTypes(value.Types)
	return value
}

func cloneCatalogDefinition(value CatalogDefinition) CatalogDefinition {
	value.Title = cloneTitle(value.Title)
	value.Attributes = cloneAttributes(value.Attributes)
	value.TableParts = slices.Clone(value.TableParts)
	for index := range value.TableParts {
		value.TableParts[index].Title = cloneTitle(value.TableParts[index].Title)
		value.TableParts[index].Attributes = cloneAttributes(value.TableParts[index].Attributes)
	}
	if value.ObjectModule != nil {
		id := *value.ObjectModule
		value.ObjectModule = &id
	}
	if value.ManagerModule != nil {
		id := *value.ManagerModule
		value.ManagerModule = &id
	}
	value.Forms = cloneObjectForms(value.Forms)
	value.List.SearchFields = slices.Clone(value.List.SearchFields)
	value.Predefined = slices.Clone(value.Predefined)
	for index := range value.Predefined {
		value.Predefined[index] = clonePredefinedCatalogItem(value.Predefined[index])
	}
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
	return value
}

func clonePredefinedCatalogItem(value PredefinedCatalogItem) PredefinedCatalogItem {
	if value.Attributes == nil {
		return value
	}
	value.Attributes = maps.Clone(value.Attributes)
	return value
}

func cloneAttributes(value []Attribute) []Attribute {
	result := slices.Clone(value)
	for index := range result {
		result[index].Title = cloneTitle(result[index].Title)
		result[index].Types = cloneTypes(result[index].Types)
	}
	return result
}
