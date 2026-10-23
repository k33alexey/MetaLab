// Package metadata defines validated application metadata loaded from an ML Project.
package metadata

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
	"go.yaml.in/yaml/v3"
)

const (
	CurrentFormat = 1
)

// Kind is a kind of metadata object; what it is called is the kinds
// package's.
type Kind = kinds.Kind

// fullName is the full name of an object, as the prototype writes it in
// Russian (help, ПолноеИмя): Справочник.Банки.
func fullName(kind Kind, name string) string {
	return kinds.Must(kind).Name.Russian + "." + name
}

// kindNamed reports whether a word of code names one object of a kind: the
// first part of a full name, of a source of a query, of a space of a lock.
func kindNamed(word string, kind Kind) bool {
	return kinds.Must(kind).Name.Is(word)
}

const (
	ConstantKind             = kinds.Constant
	SessionParameterKind     = kinds.SessionParameter
	EnumerationKind          = kinds.Enumeration
	DefinedTypeKind          = kinds.DefinedType
	CatalogKind              = kinds.Catalog
	DocumentKind             = kinds.Document
	InformationRegisterKind  = kinds.InformationRegister
	AccumulationRegisterKind = kinds.AccumulationRegister
	// ChartOfCharacteristicTypesKind holds the kinds of characteristic a
	// configuration lets its users invent without changing the configuration.
	ChartOfCharacteristicTypesKind = kinds.ChartOfCharacteristicTypes
	// ChartOfAccountsKind holds the accounts an application keeps its books on.
	ChartOfAccountsKind = kinds.ChartOfAccounts
	// ChartOfCalculationTypesKind holds kinds of accrual and deduction.
	ChartOfCalculationTypesKind = kinds.ChartOfCalculationTypes
	// BusinessProcessKind holds routes a process walks; TaskKind holds the
	// assignments created along the way.
	BusinessProcessKind = kinds.BusinessProcess
	TaskKind            = kinds.Task
	// ExchangePlanKind holds what enters an exchange and who it is exchanged
	// with.
	ExchangePlanKind = kinds.ExchangePlan
	// The three kinds that live around documents: numbering shared by several
	// of them, the order they must be posted in, and a common list of them.
	// AccountingRegisterKind holds entries: an account on each side, sums and
	// the analytics behind them.
	AccountingRegisterKind = kinds.AccountingRegister
	// CalculationRegisterKind holds results of calculation, with the period
	// they act over and the displacement between them.
	CalculationRegisterKind = kinds.CalculationRegister
	// ReportKind and DataProcessorKind keep no data: their attributes and
	// table parts live only while the object runs.
	ReportKind          = kinds.Report
	DataProcessorKind   = kinds.DataProcessor
	NumeratorKind       = kinds.Numerator
	SequenceKind        = kinds.Sequence
	DocumentJournalKind = kinds.DocumentJournal
)

type TypeKind string

const (
	StringType  TypeKind = "string"
	NumberType  TypeKind = "number"
	BooleanType TypeKind = "boolean"
	DateType    TypeKind = "date"
	// UUIDType is the unique identifier: a value of its own, not a reference.
	// A developer gives it to an attribute, a resource, a constant or a
	// session parameter - the prototype has the type (УникальныйИдентификатор)
	// and the configurations being moved use it in hundreds of type
	// descriptions. It carries no referential integrity and no presentation
	// beyond its text; the empty value is the all-zero identifier. The
	// platform stores its own identities - record identifiers, references -
	// as the same kind of value.
	UUIDType        TypeKind = "uuid"
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

// referenceSets are the sets read off the kind of object alone: any reference,
// and the set of each reference type of a kind (kindInfo.references, filled in
// with them). The two that are read off the configuration instead - a defined
// type and a characteristic - carry a reference and are handled beside them.
var referenceSets = map[TypeKind]bool{AnyReferenceSet: true}

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

// CodeSeries says within what a code is unique and automatically assigned:
// the whole object, one place in the subordination, or one owner whatever the
// parent.
//
// It belongs to the code and not to the hierarchy, and the demonstration
// configuration says why: three of the five catalogs numbered within their
// owner have no hierarchy at all. Held under the hierarchy, their numbering
// had nowhere to be written down.
type CodeSeries string

const (
	WholeObjectSeries   CodeSeries = "whole"
	SubordinationSeries CodeSeries = "within-subordination"
	// WithinOwnerSeries numbers and checks among the rows of one owner, across
	// different parents. Only a catalog has it - an account and a kind of
	// characteristic have no owner - and only a subordinate one.
	WithinOwnerSeries CodeSeries = "within-owner-subordination"
)

// The ceilings of the prototype on the shape of a field, each checked in the
// designer by the owner on 01.10.2026 and 05.10.2026 - the help names none of
// them, and the three configurations being moved stay inside every one: at
// most 5 levels of a hierarchy, codes of at most 50 characters, numbers of at
// most 32 digits with at most as many after the point, limited strings of at
// most 1024 and descriptions of at most 150 characters. A ceiling of ours
// without such a source refuses on import what the prototype saves.
const (
	// maxHierarchyLevelCount is the most levels a hierarchy may be limited to:
	// 10, at either kind of hierarchy - 6 and 10 taken, 11 «Отменить ввод?»
	// (opis of the catalog, 14.10.2026, docs/block-00/spravochnik.md). The 5
	// it once was is the most the exports keep, read as a ceiling. Only a
	// catalog has a count of levels.
	maxHierarchyLevelCount = 10
	// maxExtDimensionCount is the most analytics an account of a chart of
	// accounts may have: 50 taken, 51 «Отменить ввод?» (checked in the
	// designer, 14.10.2026, METADATA-OBJECTS.md «Пределы на состав»). The
	// configurations being moved keep 3 at most.
	maxExtDimensionCount = 50
	// maxCodeLength is the longest code of a reference object and the longest
	// number of a document, a business process or a task, string or numeric.
	maxCodeLength = 50
	// maxNumberDigits is the most digits of a number and of its fraction.
	maxNumberDigits = 32
	// maxStringLength is the longest limited string; longer is unlimited, a
	// length of 0.
	maxStringLength = 1024
	// maxFixedStringLength is the longest string of fixed length: at a longer
	// one the palette makes «Фиксированная» unavailable (checked in the
	// designer on a constant and an attribute of a catalog, 14.10.2026,
	// METADATA-OBJECTS.md «Пределы на состав»). The window of the type lets
	// it through - whence, it seems, the two predefined characteristics of
	// lombard1 of a fixed string of 128 - a roughness of the platform neither
	// Studio nor the model copies (owner, 14.10.2026; PLATFORM-MODEL.md
	// «Выбор типа»): a longer fixed string is refused wherever a type stands.
	maxFixedStringLength = 100
	// maxDescriptionLength is the longest description of a catalog, a chart
	// of characteristic types, of accounts or of calculation types and a task:
	// 150 on all of them in the configurations being moved.
	maxDescriptionLength = 150
	// maxExchangePlanDescriptionLength is the longest description of an
	// exchange plan, which takes more: 250, the designer's ceiling, checked by
	// the owner on 01.10.2026 - and exactly what the configurations being
	// moved use.
	maxExchangePlanDescriptionLength = 250
	// maxNameLength is the longest name of an object, a field or anything
	// else named by an identifier - see project.MaxNameLength.
	maxNameLength = project.MaxNameLength
	// maxVarcharLength is PostgreSQL's own ceiling on character varying(n),
	// which a length nobody else limits - the order of an account - still
	// runs into when the column is built.
	maxVarcharLength = 10_485_760
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
type LocalizedText = project.LocalizedText

// TitleLanguage is the language a reader is shown texts in. It is chosen
// among the configuration's languages (platform.ApplicationLanguage); a text
// with no translation in it shows the name of what it belongs to
// (LocalizedText.Presentation).
type TitleLanguage struct {
	Code string `json:"code"`
}

// Presentation is what text shows to this reader - see
// LocalizedText.Presentation.
func (language TitleLanguage) Presentation(text LocalizedText, name string) string {
	return text.Presentation(language.Code, name)
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
	// Name is the name of a platform type as the prototype writes it - see
	// PlatformType. No other kind has one.
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
}

// Constant is a single value existing in one instance.
//
// It is shown and edited like anything else, so it carries what that needs:
// how it is described, which form opens it, whether the platform offers its
// own commands for it, and how it is locked and historicised. The properties
// of presenting and choosing the value itself - format, mask, choice
// parameters and the rest - are the same ones an attribute has and are
// described there, not twice.
type Constant struct {
	Format  int           `yaml:"format"`
	ID      uuid.UUID     `yaml:"id"`
	Name    string        `yaml:"name"`
	Title   LocalizedText `yaml:"title"`
	Comment string        `yaml:"comment,omitempty"`
	// Explanation and ExtendedPresentation are how the value is described to
	// the user: one as a sentence beside it, one as the name of its own form.
	Explanation          LocalizedText `yaml:"explanation,omitempty"`
	ExtendedPresentation LocalizedText `yaml:"extended_presentation,omitempty"`
	Types                []Type        `yaml:"types"`
	Default              *Value        `yaml:"default,omitempty"`
	// DefaultForm names the form that opens the value. A constant keeps no
	// forms of its own, so it is one of the configuration's common forms - a
	// reference to another object, and therefore by identifier.
	DefaultForm *uuid.UUID `yaml:"default_form,omitempty"`
	// UseStandardCommands decides whether the platform offers its own commands
	// for this constant.
	UseStandardCommands bool `yaml:"use_standard_commands,omitempty"`
	// Presentation, Choice and FillChecking are the settings of a value that
	// somebody types in, and a constant is exactly that: it has a type, a form
	// and a person entering it. The prototype gives it the format and the
	// format of editing, the tooltip, the mask, the multi-line and password
	// modes, the marking of negatives, the bounds, the extended editing and
	// the whole of choice.
	//
	// It does not give a constant indexing, use, a filling value or full-text
	// search, and neither do we: there is one row of one value, so there is
	// nothing to index and nothing to search; there is nowhere for it to
	// belong; and a value that exists once is not filled in anew.
	Presentation FieldPresentation `yaml:"presentation,omitempty"`
	Choice       FieldChoice       `yaml:"choice,omitempty"`
	FillChecking FillCheck         `yaml:"fill_checking,omitempty"`
	// DataHistorySettings is whether the value takes part in data history and
	// the two flags that go with it - see data_history.go. A constant is one
	// of the ten kinds that carry all three.
	DataHistorySettings `yaml:",inline"`
	// DataLock is how the value is locked while it is written - see
	// data_lock_settings.go. A constant has the mode and no fields: it is one
	// value, and there is nothing in it to lock by.
	DataLock project.DataLockControlMode `yaml:"data_lock,omitempty"`
	// CommonAttributeFields are the fields the value gains from the common
	// attributes that separate it «независимо и совместно»: the value is
	// then kept per data area, and the field says which - see
	// propagateCommonAttributes. They are not written in the constant's
	// file, because a common attribute is described where it is and not in
	// the objects it reaches.
	CommonAttributeFields []Attribute `yaml:"-"`
}

// SessionParameter is a server-memory-only value scoped to one session's lifetime.
// Unlike Constant it is never persisted to PostgreSQL.
type SessionParameter struct {
	Format  int           `yaml:"format"`
	ID      uuid.UUID     `yaml:"id"`
	Name    string        `yaml:"name"`
	Title   LocalizedText `yaml:"title"`
	Types   []Type        `yaml:"types"`
	Comment string        `yaml:"comment,omitempty"`
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
// Two answers, as the help gives them (ИсторияВыбораПриВводе: Авто,
// НеИспользовать) and as the configurations being moved write them, on
// objects and on fields alike: there is no third that turns it on.
type ChoiceHistory string

const (
	ChoiceHistoryAuto    ChoiceHistory = "auto"
	ChoiceHistoryDontUse ChoiceHistory = "dont-use"
)

// EnumerationForms are the forms an enumeration shows itself through. It has
// no form of a single value: a value is not edited, it is written by the
// developer and only ever chosen.
//
// Each of the two has an auxiliary form beside it - a second list or a second
// choice form, used where the main one does not fit.
type EnumerationForms struct {
	List            string `yaml:"list,omitempty" json:"list,omitempty"`
	Choice          string `yaml:"choice,omitempty" json:"choice,omitempty"`
	AuxiliaryList   string `yaml:"auxiliary_list,omitempty" json:"auxiliaryList,omitempty"`
	AuxiliaryChoice string `yaml:"auxiliary_choice,omitempty" json:"auxiliaryChoice,omitempty"`
}

func (forms EnumerationForms) slots() []formSlot {
	return []formSlot{
		{"forms.list", forms.List},
		{"forms.choice", forms.Choice},
		{"forms.auxiliary_list", forms.AuxiliaryList},
		{"forms.auxiliary_choice", forms.AuxiliaryChoice},
	}
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
	ListPresentation         LocalizedText          `yaml:"list_presentation,omitempty"`
	ExtendedListPresentation LocalizedText          `yaml:"extended_list_presentation,omitempty"`
	Values                   []EnumerationValue     `yaml:"values,omitempty"`
	Characteristics          []ObjectCharacteristic `yaml:"characteristics,omitempty"`
	// How a value of this enumeration is picked where it is asked for.
	ChoiceMode           ChoiceMode    `yaml:"choice_mode,omitempty"`
	QuickChoice          bool          `yaml:"quick_choice,omitempty"`
	ChoiceHistoryOnInput ChoiceHistory `yaml:"choice_history_on_input,omitempty"`
	// UseStandardCommands decides whether the platform offers its own commands
	// for this object - opening the list and the rest.
	UseStandardCommands bool                `yaml:"use_standard_commands,omitempty"`
	StandardAttributes  []StandardAttribute `yaml:"standard_attributes,omitempty"`
	Forms               EnumerationForms    `yaml:"forms,omitempty"`
	Commands            []ObjectCommand     `yaml:"commands,omitempty"`
	Templates           []ObjectTemplate    `yaml:"templates,omitempty"`
}

type DefinedTypeObject struct {
	Format int           `yaml:"format"`
	ID     uuid.UUID     `yaml:"id"`
	Name   string        `yaml:"name"`
	Title  LocalizedText `yaml:"title"`
	Types  []Type        `yaml:"types"`
	// Comment is for the developer and is not localized. Every metadata object
	// of the prototype has one, and the demonstration configuration writes it on
	// all 73 defined types.
	Comment string `yaml:"comment,omitempty"`
}

type CatalogCode struct {
	Type   TypeKind `yaml:"type" json:"type"`
	Length int      `yaml:"length" json:"length"`
	Auto   bool     `yaml:"auto" json:"auto"`
	Unique bool     `yaml:"unique" json:"unique"`
	// FixedLength is the allowed length of a string code: a fixed one is padded
	// with spaces to its full width, a variable one is stored as typed. It is
	// the same setting a string attribute carries, spelled the same way - see
	// Type.FixedLength - and it means nothing for a numeric code, which is why
	// the help says the property has meaning only when the code is a string.
	//
	// Absent means variable, which is the prototype's own default and what 109
	// of the 123 coded objects of the demonstration configuration say.
	FixedLength bool `yaml:"fixed_length,omitempty" json:"fixedLength,omitempty"`
	// Series is the range a code is unique and auto-assigned within.
	Series CodeSeries `yaml:"series,omitempty" json:"series,omitempty"`
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
	//
	// No values of the object's attributes: the prototype's dialog of a
	// predefined item has the parent, the name, the code, the description and
	// the fields of its kind, and none of the attributes even of a catalog
	// that has many (ERP, «ВидыКонтактнойИнформации», 21.10.2026), and no
	// export writes one. A value given here would be a field of ML alone.
	Parent   string `yaml:"parent,omitempty" json:"parent,omitempty"`
	IsFolder bool   `yaml:"is_folder,omitempty" json:"isFolder,omitempty"`
}

// EffectiveCode is the code a predefined item is created with: the code
// written, and nothing when the catalog's code is switched off by a length of
// 0 - the item then keeps the code it had while there was one, and there is no
// field to put it in. A code of spaces alone is the empty code too, one value
// and one record (METADATA-OBJECTS.md, «Код предопределённого элемента»): with
// automatic codes the item gets the next code, without them the empty one, and
// a numeric one is zero. Everything that creates an item reads its code here.
func (item PredefinedCatalogItem) EffectiveCode(code CatalogCode) string {
	if code.Length == 0 || emptyPredefinedCode(item.Code) {
		return ""
	}
	return item.Code
}

type Attribute struct {
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	Types   []Type        `yaml:"types" json:"types"`
	// FillChecking is whether the platform checks that the field was filled
	// in, and it replaced a flag that made the column NOT NULL. The prototype
	// has no such column and no such refusal - see attribute_storage.go.
	FillChecking FillCheck `yaml:"fill_checking,omitempty" json:"fillChecking,omitempty"`
	// Indexing is what the database is asked to build for this field. It
	// replaced a flag: the prototype has three answers, not two.
	Indexing IndexMode `yaml:"indexing,omitempty" json:"indexing,omitempty"`
	// Filling is what a new value starts as; FullTextSearch and DataHistory
	// say whether the field is searched and whether its versions are kept;
	// Use says whether the field belongs to items, to folders or to both.
	Filling        FieldFilling `yaml:"filling,omitempty" json:"filling,omitempty"`
	FullTextSearch UsageMode    `yaml:"full_text_search,omitempty" json:"fullTextSearch,omitempty"`
	DataHistory    UsageMode    `yaml:"data_history,omitempty" json:"dataHistory,omitempty"`
	Use            AttributeUse `yaml:"use,omitempty" json:"use,omitempty"`
	// How the value is shown and entered, and how it is picked. Neither
	// changes what is stored, and both change what the user gets - see
	// attribute_presentation.go.
	Presentation FieldPresentation `yaml:"presentation,omitempty" json:"presentation,omitempty"`
	Choice       FieldChoice       `yaml:"choice,omitempty" json:"choice,omitempty"`
	// BinaryDataStorage and BinaryDataStorageField put a value of the type
	// ХранилищеЗначения into a binary data storage of the base instead of the
	// table - since 8.3.26, on an attribute and on a resource. The mode is use
	// or do not use; with use, the field may name a boolean field of the same
	// object that decides it row by row. The storages themselves, built in or
	// external over S3, are a setting of the running base, not of the
	// configuration. Carried and checked; the writing into a storage is not
	// done - see validateBinaryDataStorage.
	BinaryDataStorage      UsageMode  `yaml:"binary_data_storage,omitempty" json:"binaryDataStorage,omitempty"`
	BinaryDataStorageField *uuid.UUID `yaml:"binary_data_storage_field,omitempty" json:"binaryDataStorageField,omitempty"`
}

type TablePart struct {
	ID         uuid.UUID     `yaml:"id" json:"id"`
	Name       string        `yaml:"name" json:"name"`
	Title      LocalizedText `yaml:"title" json:"title"`
	Attributes []Attribute   `yaml:"attributes" json:"attributes"`
	// StandardAttributes is what the developer changed about the one field the
	// platform gives a line - its number. See standard_attributes.go.
	StandardAttributes []StandardAttribute `yaml:"standard_attributes,omitempty" json:"standardAttributes,omitempty"`
	Comment            string              `yaml:"comment,omitempty" json:"comment,omitempty"`
	// ToolTip is the sentence shown beside the part.
	ToolTip LocalizedText `yaml:"tooltip,omitempty" json:"tooltip,omitempty"`
	// FillChecking puts the part into the automatic check of filling: an empty
	// table part then stops a write the way an empty attribute does.
	FillChecking FillCheck `yaml:"fill_checking,omitempty" json:"fillChecking,omitempty"`
	// LineNumberLength is the decimal width the line number of a row is stored
	// in, 5 to 9. It is about storage, so a part whose rows never reach the
	// database has none - see tablePartRules.
	LineNumberLength int `yaml:"line_number_length,omitempty" json:"lineNumberLength,omitempty"`
	// Use is whom the part belongs to - items, folders or both. It is the same
	// setting an attribute carries, and the prototype writes it on the part
	// itself and never on a field of one.
	Use AttributeUse `yaml:"use,omitempty" json:"use,omitempty"`
}

// ListSettings controls bounded server-side lists without embedding SQL in metadata.
type ListSettings struct {
	PageSize     int      `yaml:"page_size,omitempty" json:"pageSize,omitempty"`
	SearchFields []string `yaml:"search_fields,omitempty" json:"searchFields,omitempty"`
}

// CatalogDefinition describes one ML catalog and its persistent record shape.
type CatalogDefinition struct {
	Format int           `yaml:"format" json:"format"`
	ID     uuid.UUID     `yaml:"id" json:"id"`
	Name   string        `yaml:"name" json:"name"`
	Title  LocalizedText `yaml:"title" json:"title"`
	// Presentations is how this object is named to the person using it -
	// see object_presentation.go.
	Presentations `yaml:",inline" json:",inline"` // ObjectChoice is how a value of this kind is entered and picked - see
	// object_choice.go.
	ObjectChoice `yaml:",inline" json:",inline"`
	// BasedOn are the objects one of these may be made out of, the list the
	// command to make it offers - see based_on.go.
	BasedOn []uuid.UUID `yaml:"based_on,omitempty" json:"basedOn,omitempty"`
	// DataLock is how the platform locks a row of this object while it is
	// written, and DataLockFields are the fields it may be locked by - see
	// data_lock_settings.go.
	DataLock       project.DataLockControlMode `yaml:"data_lock,omitempty" json:"dataLock,omitempty"`
	DataLockFields []ObjectField               `yaml:"data_lock_fields,omitempty" json:"dataLockFields,omitempty"`
	// FullTextSearch is whether this object is in the full-text index at all -
	// see full_text_search.go.
	FullTextSearch FullTextSearchMode `yaml:"full_text_search,omitempty" json:"fullTextSearch,omitempty"`
	// DataHistorySettings is whether this object takes part in data history
	// and the two flags that go with it - see data_history.go.
	DataHistorySettings `yaml:",inline" json:",inline"`
	// AdditionalIndexes are the indexes this object asks the database for
	// beside the ones the platform builds - see additional_indexes.go.
	AdditionalIndexes []AdditionalIndex `yaml:"additional_indexes,omitempty" json:"additionalIndexes,omitempty"`

	Code              CatalogCode `yaml:"code" json:"code"`
	DescriptionLength int         `yaml:"description_length" json:"descriptionLength"`
	Hierarchy         Hierarchy   `yaml:"hierarchy,omitempty" json:"hierarchy,omitempty"`
	// Owners are the objects whose elements a row of this catalog belongs to,
	// and Subordination says whether it belongs to their items, their folders
	// or either - see catalog_owners.go.
	Owners             []uuid.UUID             `yaml:"owners,omitempty" json:"owners,omitempty"`
	Subordination      SubordinationKind       `yaml:"subordination,omitempty" json:"subordination,omitempty"`
	Attributes         []Attribute             `yaml:"attributes,omitempty" json:"attributes,omitempty"`
	TableParts         []TablePart             `yaml:"table_parts,omitempty" json:"tableParts,omitempty"`
	Characteristics    []ObjectCharacteristic  `yaml:"characteristics,omitempty" json:"characteristics,omitempty"`
	StandardAttributes []StandardAttribute     `yaml:"standard_attributes,omitempty" json:"standardAttributes,omitempty"`
	Forms              HierarchicalObjectForms `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands           []ObjectCommand         `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates          []ObjectTemplate        `yaml:"templates,omitempty" json:"templates,omitempty"`
	List               ListSettings            `yaml:"list,omitempty" json:"list,omitempty"`
	Predefined         []PredefinedCatalogItem `yaml:"predefined,omitempty" json:"predefined,omitempty"`
	// PredefinedDataUpdate decides what happens to the rows behind Predefined
	// when the configuration is updated - see predefined_data_update.go.
	PredefinedDataUpdate PredefinedDataUpdate `yaml:"predefined_data_update,omitempty" json:"predefinedDataUpdate,omitempty"`
}

// indexedObject is what the catalog knows of an object by its identifier
// alone: its kind and its name as written; an object inside another is named
// with its owner, Источник.Таблица.
type indexedObject struct {
	kind Kind
	name string
	// owner is the object this one lies inside, as the configuration tree
	// shows it: the subsystem a subsystem is nested in, the calculation
	// register of a recalculation, the source of a table or a cube, the cube
	// of a dimension table. Nil for an object of the configuration itself.
	owner *uuid.UUID
	// title is the synonym of an object inside another, which has no list of
	// its own to read it from.
	title LocalizedText
}

// Catalog is an immutable-by-convention snapshot of the supported metadata kinds.
type Catalog struct {
	Project project.Project
	// HomePage, ClientInterface and MainSectionCommandInterface are what the
	// application shows before anything is opened - see root_interface.go.
	HomePage                    *HomePage
	ClientInterface             *ClientInterface
	MainSectionCommandInterface CommandInterface
	Roles                       []RoleDefinition
	// rolesLoaded tells whether Roles is the configuration's roles or a stand-in.
	// The role editor validates one edited role against a catalog that
	// deliberately holds no others, and the root's default roles must not be
	// judged against that list: they would all look missing.
	rolesLoaded bool
	// outside are the objects of the project a runtime snapshot names and
	// holds no description of (RuntimeSnapshot.Outside), by their kind; nil
	// for a catalog read from the project, which holds them all.
	outside         map[uuid.UUID]Kind
	roleByName      map[string]int
	roleByID        map[uuid.UUID]int
	Subsystems      []SubsystemDefinition
	subsystemByName map[string]int
	subsystemByID   map[uuid.UUID]int
	// objectForms is what each object's forms folder held when the project was
	// read, kind by kind and object by object. Nothing declares a form, so
	// this is the only place that knows an object has one.
	//
	// Keys are folded, because a name is unique among its siblings with case
	// ignored; the names themselves are kept as written, because they are read
	// by people - in the tree, and in the name a form's module compiles under.
	objectForms map[Kind]map[string]objectFormIndex
	// commonFormNames is every common form of the configuration, folded, read
	// once when the project is read. A role may open a common form instead of
	// one of the object's own, and then this is what says the form is there.
	commonFormNames map[string]bool
	// commonFormsRead are the common forms as the load read them, kept only
	// until their attributes are checked; formTypeLists are the type
	// descriptions of the attributes of every form, kept for the notes.
	commonFormsRead []ManagedForm
	formTypeLists   []formTypeList
	// formAttributeNames are the names of the attributes of every form, by
	// the address of its module folded (foldedModuleAddress), for the
	// module's code (ModuleNamer).
	formAttributeNames map[project.ModuleAddress][]string
	// formNotes are the notes about the elements of the forms - see
	// formNote.
	formNotes                       []formNote
	formTitles                      []formTitle
	Constants                       []Constant
	SessionParameters               []SessionParameter
	sessionParameterByName          map[string]int
	sessionParameterByID            map[uuid.UUID]int
	CommonAttributes                []CommonAttributeDefinition
	commonAttributeByName           map[string]int
	commonAttributeByID             map[uuid.UUID]int
	CommonModules                   []CommonModuleDefinition
	commonModuleByName              map[string]int
	commonModuleByID                map[uuid.UUID]int
	commonModuleByModuleID          map[uuid.UUID]int
	EventSubscriptions              []EventSubscriptionDefinition
	eventSubscriptionByName         map[string]int
	eventSubscriptionByID           map[uuid.UUID]int
	Enumerations                    []Enumeration
	DefinedTypes                    []DefinedTypeObject
	Catalogs                        []CatalogDefinition
	Documents                       []DocumentDefinition
	ChartsOfCharacteristicTypes     []ChartOfCharacteristicTypesDefinition
	ChartsOfAccounts                []ChartOfAccountsDefinition
	ChartsOfCalculationTypes        []ChartOfCalculationTypesDefinition
	BusinessProcesses               []BusinessProcessDefinition
	Tasks                           []TaskDefinition
	ExchangePlans                   []ExchangePlanDefinition
	Numerators                      []NumeratorDefinition
	Sequences                       []SequenceDefinition
	DocumentJournals                []DocumentJournalDefinition
	InformationRegisters            []InformationRegisterDefinition
	AccumulationRegisters           []AccumulationRegisterDefinition
	AccountingRegisters             []AccountingRegisterDefinition
	CalculationRegisters            []CalculationRegisterDefinition
	Reports                         []ReportDefinition
	DataProcessors                  []DataProcessorDefinition
	FunctionalOptions               []FunctionalOptionDefinition
	functionalOptionByName          map[string]int
	functionalOptionByID            map[uuid.UUID]int
	FunctionalOptionParameters      []FunctionalOptionParameterDefinition
	functionalOptionParameterByName map[string]int
	functionalOptionParameterByID   map[uuid.UUID]int
	FilterCriteria                  []FilterCriterionDefinition
	filterCriterionByName           map[string]int
	filterCriterionByID             map[uuid.UUID]int
	SettingsStorages                []SettingsStorageDefinition
	settingsStorageByName           map[string]int
	settingsStorageByID             map[uuid.UUID]int
	ScheduledJobs                   []ScheduledJobDefinition
	scheduledJobByName              map[string]int
	scheduledJobByID                map[uuid.UUID]int
	CommonCommands                  []CommonCommandDefinition
	commonCommandByName             map[string]int
	commonCommandByID               map[uuid.UUID]int
	CommandGroups                   []CommandGroupDefinition
	commandGroupByName              map[string]int
	commandGroupByID                map[uuid.UUID]int
	// unresolved is what the last load found pointing at nothing - see
	// UnresolvedReference.
	unresolved []UnresolvedReference
	// helpPages is every page of help the last load found - see
	// collectHelpPages.
	helpPages []helpPage
	// objects holds every object the last load read, of every kind, by its
	// identifier: the top level, a common form, a language, and what lies
	// inside an object as an object of a kind of its own - a recalculation, a
	// table or a cube of an external data source. It is the one place a
	// reference that may point at an object of any kind is resolved by - the
	// content of a subsystem - and turned into the object's kind and name once
	// found - the owner of a choice form.
	objects map[uuid.UUID]indexedObject
	// identifiers says what every identifier the last load checked for
	// uniqueness belongs to, in the words a message names it with: an object,
	// its attribute, its table part. It is what an error about a physical
	// table names the table's owner by.
	identifiers map[uuid.UUID]string
	// formOwnerKinds holds the kind of each object that may keep forms, by
	// its identifier, as the loader indexed its folder: what a choice form's
	// kind is checked against.
	formOwnerKinds           map[uuid.UUID]Kind
	CommonTemplates          []CommonTemplateDefinition
	commonTemplateByName     map[string]int
	commonTemplateByID       map[uuid.UUID]int
	XDTOPackages             []XDTOPackageDefinition
	xdtoPackageByName        map[string]int
	xdtoPackageByID          map[uuid.UUID]int
	WebServices              []WebServiceDefinition
	webServiceByName         map[string]int
	webServiceByID           map[uuid.UUID]int
	HTTPServices             []HTTPServiceDefinition
	httpServiceByName        map[string]int
	httpServiceByID          map[uuid.UUID]int
	WSReferences             []WSReferenceDefinition
	wsReferenceByName        map[string]int
	wsReferenceByID          map[uuid.UUID]int
	WebSocketClients         []WebSocketClientDefinition
	webSocketClientByName    map[string]int
	webSocketClientByID      map[uuid.UUID]int
	ExternalDataSources      []ExternalDataSourceDefinition
	externalDataSourceByName map[string]int
	externalDataSourceByID   map[uuid.UUID]int
	// externalTableByID finds a table of any source by its identifier, which
	// is how a field that refers to it names it.
	externalTableByID map[uuid.UUID]externalTableLocation
	// externalCubeByID and externalDimensionTableByID find a cube of any
	// source and a dimension table of any cube the same way.
	externalCubeByID                 map[uuid.UUID]externalCubeLocation
	externalDimensionTableByID       map[uuid.UUID]externalDimensionTableLocation
	CommonPictures                   []CommonPictureDefinition
	commonPictureByName              map[string]int
	commonPictureByID                map[uuid.UUID]int
	StyleItems                       []StyleItemDefinition
	styleItemByName                  map[string]int
	styleItemByID                    map[uuid.UUID]int
	Styles                           []StyleDefinition
	styleByName                      map[string]int
	styleByID                        map[uuid.UUID]int
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

// ObjectIDs is the identifier of every object the catalog holds, kind by
// kind: the platform's own tables key what an object leaves in them by it.
// It reads the objects themselves, not the index of a load, so a catalog
// put together in place answers too.
func (catalog *Catalog) ObjectIDs() []uuid.UUID {
	var ids []uuid.UUID
	for _, item := range kindInfos {
		objects, ok := catalog.objectsOf(item.kind)
		if !ok {
			continue
		}
		for index := range objects.Len() {
			ids = append(ids, headOf(objects.Index(index).Interface()).id)
		}
	}
	return ids
}

func (catalog *Catalog) ConstantIDs() []uuid.UUID {
	return itemIDs(catalog.Constants)
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

func (catalog *Catalog) DefinedTypeByID(id uuid.UUID) (DefinedTypeObject, bool) {
	index, ok := catalog.definedTypeByID[id]
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
	return itemIDs(catalog.Catalogs)
}

func (definition CatalogDefinition) PredefinedItem(name string) (PredefinedCatalogItem, bool) {
	for _, item := range definition.Predefined {
		if strings.EqualFold(item.Name, name) {
			return item, true
		}
	}
	return PredefinedCatalogItem{}, false
}

func (definition CatalogDefinition) PredefinedByID(id uuid.UUID) (PredefinedCatalogItem, bool) {
	for _, item := range definition.Predefined {
		if item.ID == id {
			return item, true
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
	return itemIDs(catalog.Documents)
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
	return itemIDs(catalog.InformationRegisters)
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
	return itemIDs(catalog.AccumulationRegisters)
}

func DecodeConstant(source string, reader io.Reader, configuration project.Project) (Constant, error) {
	return decodeDescription(source, reader, configuration, func(value Constant, configuration project.Project) []string {
		var issues []string
		issues = append(issues, validateTypes("types", value.Types)...)
		for name, text := range map[string]LocalizedText{
			"explanation": value.Explanation, "extended_presentation": value.ExtendedPresentation,
		} {
			issues = append(issues, validateTitle(name, text, configuration)...)
		}
		if value.DefaultForm != nil && value.DefaultForm.IsZero() {
			issues = append(issues, "default_form must be a non-zero UUID")
		}
		issues = append(issues, validateValueSettings(Attribute{
			Types: value.Types, Presentation: value.Presentation, Choice: value.Choice,
		}, configuration)...)
		// The choice of a constant is that of a field but for one setting: a
		// constant has no creation on input - the palette does not show it at
		// any type, the help does not give it, and no constant of the exports
		// has one (2061 of them) (owner, 14.10.2026, METADATA-OBJECTS
		// «Константы»). Like indexing, it is refused, whatever its value.
		if value.Choice.CreateOnInput != "" {
			issues = append(issues, "choice.create_on_input: a constant has no creation on input")
		}
		if !validFillCheck(value.FillChecking) {
			issues = append(issues, "fill_checking must be dont-check or show-error")
		}
		// A constant stands alone: there are no sibling fields for a choice
		// parameter link to take its value from, so a link that names one names
		// something that is not there. It may take one from another constant.
		issues = append(issues, validateFieldLinks(nil, nil, nil, choiceHolder{"", value.Choice})...)
		issues = append(issues, validateDataHistory(value.DataHistorySettings)...)
		issues = append(issues, validateDataLockMode("data_lock", value.DataLock)...)
		return issues
	})
}

func DecodeSessionParameter(source string, reader io.Reader, configuration project.Project) (SessionParameter, error) {
	return decodeDescription(source, reader, configuration, func(value SessionParameter, configuration project.Project) []string {
		// No name is the platform's: ТекущийПользователь is the configuration's
		// own parameter, declared in all three configurations being moved and set
		// by their session modules (owner, 01.10.2026). The user of ML is another
		// layer and never appears among these names - see session_values.go.
		return validateTypesIn("types", value.Types, placeSessionParameter)
	})
}

func DecodeEnumeration(source string, reader io.Reader, configuration project.Project) (Enumeration, error) {
	return decodeDescription(source, reader, configuration, func(value Enumeration, configuration project.Project) []string {
		// An enumeration with no values is saved by the configurator and occurs in
		// the configurations being moved - erp СтатьиБезАналитики, acc
		// РежимыОбменаДанными. Its attribute can hold only the empty reference.
		var issues []string
		names, ids := namesWithin(""), map[uuid.UUID]bool{}
		for index, item := range value.Values {
			prefix := fmt.Sprintf("values[%d]", index)
			if item.ID.IsZero() {
				issues = append(issues, prefix+".id must be a non-zero UUID")
			}
			if ids[item.ID] {
				issues = append(issues, prefix+".id must be unique")
			}
			ids[item.ID] = true
			issues = append(issues, names.take(prefix, item.Name)...)
			issues = append(issues, validateTitle(prefix+".title", item.Title, configuration)...)
		}
		for name, text := range map[string]LocalizedText{
			"explanation": value.Explanation, "list_presentation": value.ListPresentation,
			"extended_list_presentation": value.ExtendedListPresentation,
		} {
			issues = append(issues, validateTitle(name, text, configuration)...)
		}
		// An enumeration has three of the six settings of choice, and they mean here
		// exactly what they mean on the kinds that have all six - so they are checked
		// by the same code. See object_choice.go.
		if !validChoiceMode(value.ChoiceMode) {
			issues = append(issues, "choice_mode must be both-ways, from-form or quick-choice")
		}
		if !validChoiceHistory(value.ChoiceHistoryOnInput) {
			issues = append(issues, "choice_history_on_input must be auto or dont-use")
		}
		// Quick choice under «from form» is carried and noted, as on the other
		// kinds - see validateObjectChoice.
		issues = append(issues, validateStandardAttributes("standard_attributes", value.StandardAttributes, standardFieldsOfKind(EnumerationKind), configuration)...)
		issues = append(issues, validateFieldLinks(standardFieldsOfKind(EnumerationKind), nil, nil, standardAttributeChoices("standard_attributes", value.StandardAttributes)...)...)
		issues = append(issues, validateFormSlots(value.Forms.slots())...)
		issues = append(issues, validateObjectCommands(value.Commands, configuration)...)
		issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
		issues = append(issues, validateObjectCharacteristics(value.Characteristics)...)
		return issues
	})
}

func DecodeDefinedType(source string, reader io.Reader, configuration project.Project) (DefinedTypeObject, error) {
	return decodeDescription(source, reader, configuration, func(value DefinedTypeObject, configuration project.Project) []string {
		return validateTypesIn("types", value.Types, placeDefinedType)
	})
}

func DecodeCatalog(source string, reader io.Reader, configuration project.Project) (CatalogDefinition, error) {
	return decodeDescription(source, reader, configuration, func(value CatalogDefinition, configuration project.Project) []string {
		var issues []string
		issues = append(issues, validateReferenceObjectShape(referenceObjectShape{
			code:                   value.Code,
			descriptionLength:      value.DescriptionLength,
			attributes:             value.Attributes,
			tableParts:             value.TableParts,
			forms:                  value.Forms,
			list:                   value.List,
			hierarchy:              value.Hierarchy,
			predefined:             value.Predefined,
			reservedName:           reservedCatalogObjectName,
			attributeUse:           true,
			codeSeries:             true,
			ownerSeries:            true,
			codeAllowedLength:      true,
			codeType:               true,
			codeMayBeAbsent:        true,
			descriptionMayBeAbsent: true,
			autonumbering:          true,
			checkUnique:            true,
			predefinedDataUpdate:   value.PredefinedDataUpdate,
			presentation:           value.Presentations,
			choice:                 value.ObjectChoice,
			basedOn:                value.BasedOn,
			dataLock:               value.DataLock,
			dataLockFields:         value.DataLockFields,
			fullTextSearch:         value.FullTextSearch,
			dataHistory:            value.DataHistorySettings,
			additionalIndexes:      value.AdditionalIndexes,
			kind:                   CatalogKind,
			standardAttributes:     value.StandardAttributes,
		}, configuration)...)
		issues = append(issues, validateCatalogSubordination(value.Owners, value.Subordination)...)
		issues = append(issues, validateObjectCommands(value.Commands, configuration)...)
		issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
		issues = append(issues, validateObjectCharacteristics(value.Characteristics)...)
		return issues
	})
}

// referenceObjectShape is everything a reference object repeats from the
// catalog: a code, a description, attributes, table parts, forms, list
// settings and predefined elements. Charts of characteristic
// types, charts of accounts, charts of calculation types, business processes
// and tasks all repeat it, and each adds its own on top - so the repeated part
// is checked in one place rather than copied per kind, where the copies drift.
type referenceObjectShape struct {
	code              CatalogCode
	descriptionLength int
	attributes        []Attribute
	tableParts        []TablePart
	// forms carries the folder roles as well, because whether they are
	// allowed at all is decided by the hierarchy, which is right here. A kind
	// whose hierarchy has no folders - a chart of accounts, where every row is
	// an account - simply leaves them empty.
	forms      HierarchicalObjectForms
	list       ListSettings
	hierarchy  Hierarchy
	predefined []PredefinedCatalogItem
	// reservedName says which attribute names the kind keeps for itself. A
	// kind with standard attributes of its own passes its own answer.
	reservedName func(string) bool
	// codeSeries says this kind numbers its codes within a range at all, and
	// ownerSeries that one of the ranges may be an owner. In the prototype a
	// catalog has all three ranges, a chart of characteristic types and a
	// chart of accounts have two - neither of them has an owner - and a chart
	// of calculation types and an exchange plan have no such property at all.
	codeSeries  bool
	ownerSeries bool
	// codeAllowedLength says this kind lets the developer choose between a
	// fixed and a variable code. A chart of accounts does not: the shape of an
	// account's code is the code mask's to decide, and the prototype gives the
	// chart no such property at all.
	codeAllowedLength bool
	// codeMayBeAbsent and descriptionMayBeAbsent say a code or a description
	// of length 0 is allowed: it is then
	// switched off - no field on the object, no column, no search by it. The
	// designer allows it on a catalog and a chart of characteristic types,
	// checked by the owner on 01.10.2026 (ИТС 1590 says otherwise of the
	// chart; the designer of 8.3.27 has it); 720 catalogs of the
	// configurations being moved have no code. A chart of accounts and a
	// chart of calculation types may have no code either, the owner checked
	// the same day, and so may a chart of characteristic types and a chart of
	// accounts by the opis of 15.10.2026. The description of a chart of
	// accounts and of calculation types was not checked: a length of 0 there
	// is carried the same way and is a note (NoteLengthZeroUnchecked). An
	// exchange plan has neither of length 0: the designer refuses both.
	codeMayBeAbsent        bool
	descriptionMayBeAbsent bool
	// autonumbering and checkUnique say this kind has automatic codes and the
	// check that a code is not repeated. The help gives both to a catalog and
	// a chart of characteristic types, only the check to a chart of accounts,
	// and neither to a chart of calculation types or an exchange plan: their
	// codes are typed in, and the configurations being moved never write the
	// two properties on them. A property a kind does not have reads as a
	// setting and does nothing, so it is refused.
	autonumbering bool
	checkUnique   bool
	// codeType says this kind lets the developer choose whether the code is a
	// string or a number. Three kinds do not, and their code is always a
	// string: a chart of characteristic types, an exchange plan and a chart of
	// accounts have no such property in the prototype at all. A code declared
	// numeric on one of them describes a setting the platform would not read.
	codeType bool
	// predefinedDataUpdate is what the object says happens to its predefined
	// rows when the configuration is updated. Only the four kinds that keep
	// predefined data carry it - see predefined_data_update.go.
	predefinedDataUpdate PredefinedDataUpdate
	// presentation is how the object is named to a person. All five reference
	// kinds carry the whole set - see object_presentation.go.
	presentation Presentations
	// extraFields are fields of the object that do not lie in `attributes`: a
	// chart of accounts' accounting flags and ext dimension accounting flags.
	// They are here so that a choice link written on one of them is resolved
	// against the object it belongs to, the same way an ordinary attribute's is.
	extraFields []fieldGroup
	// choice is how a value of the kind is entered and picked. All five
	// reference kinds carry the whole set - see object_choice.go.
	choice ObjectChoice
	// basedOn is what this object may be made out of - see based_on.go. All
	// five reference kinds carry it, as do the three numbered ones.
	basedOn []uuid.UUID
	// dataLock and dataLockFields are how a row of this object is locked and
	// by which fields - see data_lock_settings.go.
	dataLock       project.DataLockControlMode
	dataLockFields []ObjectField
	// fullTextSearch is whether the object is in the index - see
	// full_text_search.go.
	fullTextSearch FullTextSearchMode
	// dataHistory is whether the object takes part in data history, and what
	// it asks for while it does - see data_history.go.
	dataHistory DataHistorySettings
	// additionalIndexes are the indexes asked for by hand - see
	// additional_indexes.go.
	additionalIndexes []AdditionalIndex
	// attributeUse says this kind's attributes may say whom they belong to -
	// items, folders or both. Only a catalog and a chart of characteristic
	// types may: the help says so, and the demonstration configuration writes
	// the setting on those two kinds and on no other.
	attributeUse bool
	// kind says which kind of object this is, so that the standard fields and
	// the standard table parts the platform gives it can be looked up. The
	// descriptions the developer wrote about them are below.
	kind               Kind
	standardAttributes []StandardAttribute
	standardTableParts []StandardTablePart
}

// validateHierarchy checks the settings of a hierarchy.
//
// It does not check them against each other. A kind of hierarchy on a flat
// object, a level count nobody limits, folders on top where there are no
// folders all mean nothing - and the prototype writes every one of them on
// every object, with the defaults of the dialog: the configurations being
// moved carry hierarchy settings on 745 flat catalogs, a level count without
// a limit on 1066, folders on top over a hierarchy of items on 63. Refusing
// them refused the configurations. They are carried as written and read only
// where the setting they depend on is on.
func validateHierarchy(hierarchy Hierarchy) []string {
	var issues []string
	if !hierarchy.Enabled {
		if hierarchy.Kind != "" && hierarchy.Kind != FoldersAndItemsHierarchy && hierarchy.Kind != ItemsHierarchy {
			issues = append(issues, "hierarchy.kind must be folders-and-items or items")
		}
		if hierarchy.LevelCount < 0 || hierarchy.LevelCount > maxHierarchyLevelCount {
			issues = append(issues, fmt.Sprintf("hierarchy.level_count must be 0..%d", maxHierarchyLevelCount))
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
	if hierarchy.LimitLevels && (hierarchy.LevelCount < 1 || hierarchy.LevelCount > maxHierarchyLevelCount) {
		issues = append(issues, fmt.Sprintf("hierarchy.level_count must be 1..%d when levels are limited", maxHierarchyLevelCount))
	}
	if !hierarchy.LimitLevels && (hierarchy.LevelCount < 0 || hierarchy.LevelCount > maxHierarchyLevelCount) {
		issues = append(issues, fmt.Sprintf("hierarchy.level_count must be 0..%d", maxHierarchyLevelCount))
	}
	return issues
}

// validateCodeSeries checks the range a code is numbered within against the
// kind that carries it. A range a kind does not have is a setting nothing
// reads, and the code then numbers itself by a rule nobody wrote.
func validateCodeSeries(shape referenceObjectShape) []string {
	if shape.code.Series == "" {
		return nil
	}
	if !shape.codeSeries {
		return []string{"code.series belongs to a kind that numbers within a range, and this is not one"}
	}
	switch shape.code.Series {
	case WholeObjectSeries, SubordinationSeries:
		return nil
	case WithinOwnerSeries:
		if shape.ownerSeries {
			return nil
		}
		return []string{"code.series within-owner-subordination belongs to a catalog, which is the only kind with an owner"}
	}
	if shape.ownerSeries {
		return []string{"code.series must be whole, within-subordination or within-owner-subordination"}
	}
	return []string{"code.series must be whole or within-subordination"}
}

// validateCodeAllowedLength checks the choice between a fixed and a variable
// code against the kind that made it and against the code it is about.
func validateCodeAllowedLength(shape referenceObjectShape) []string {
	if !shape.code.FixedLength {
		return nil
	}
	if !shape.codeAllowedLength {
		return []string{"code.fixed_length belongs to a kind that chooses the length of its code, and this one takes it from the code mask"}
	}
	// A number is not padded to a width, so the setting does nothing on a
	// numeric code - and the prototype writes it there all the same, left
	// from when the code was a string: 3/5/0 catalogs and 6 in energy. The
	// help says the setting has a meaning for a string code, not that it is
	// forbidden on a number. It is carried and acts on nothing - the schema
	// asks the type first - and is a note (NoteFixedLengthOfNumber).
	return nil
}

// validateCodeProperties refuses automatic codes and the uniqueness check on a
// kind that has neither - see referenceObjectShape.autonumbering.
func validateCodeProperties(shape referenceObjectShape) []string {
	var issues []string
	if shape.code.Auto && !shape.autonumbering {
		issues = append(issues, "code.auto belongs to a kind with automatic codes, and this one has its codes typed in")
	}
	if shape.code.Unique && !shape.checkUnique {
		issues = append(issues, "code.unique belongs to a kind that checks its codes for repeats, and this one has no such check")
	}
	return issues
}

// validateCodeType checks the choice of what a code is against the kind that
// made it. Only a catalog and a chart of calculation types have the choice; for
// the rest the code is a string, and saying otherwise is describing a property
// the prototype does not give that kind.
func validateCodeType(shape referenceObjectShape) []string {
	if shape.codeType || shape.code.Type == StringType {
		return nil
	}
	return []string{"code.type must be string: this kind of object has no choice of what its code is"}
}

func validateReferenceObjectShape(shape referenceObjectShape, configuration project.Project) []string {
	var issues []string
	shortest := 1
	if shape.codeMayBeAbsent {
		shortest = 0
	}
	switch shape.code.Type {
	case StringType, NumberType:
		if shape.code.Length < shortest || shape.code.Length > maxCodeLength {
			issues = append(issues, fmt.Sprintf("code.length must be %d..%d", shortest, maxCodeLength))
		}
	default:
		issues = append(issues, "code.type must be string or number")
	}
	descriptionCeiling := maxDescriptionLength
	if shape.kind == ExchangePlanKind {
		descriptionCeiling = maxExchangePlanDescriptionLength
	}
	// A description of length 0 is switched off the way a code is, on the
	// same two kinds - see codeMayBeAbsent; 22 catalogs of the configurations
	// being moved have none.
	shortestDescription := 1
	if shape.descriptionMayBeAbsent {
		shortestDescription = 0
	}
	if shape.descriptionLength < shortestDescription || shape.descriptionLength > descriptionCeiling {
		issues = append(issues, fmt.Sprintf("description_length must be %d..%d", shortestDescription, descriptionCeiling))
	}
	issues = append(issues, validateHierarchy(shape.hierarchy)...)
	issues = append(issues, validateCodeSeries(shape)...)
	issues = append(issues, validateCodeAllowedLength(shape)...)
	issues = append(issues, validateCodeType(shape)...)
	issues = append(issues, validateCodeProperties(shape)...)
	issues = append(issues, validatePredefinedDataUpdate(shape.predefinedDataUpdate)...)
	issues = append(issues, validatePresentations(shape.presentation, configuration)...)
	issues = append(issues, validateObjectChoice(shape.choice)...)
	issues = append(issues, validateInputByString(shape.choice.InputByString, shape.kind, shape.attributes, shape.switchedOff()...)...)
	issues = append(issues, validateBasedOn(shape.basedOn)...)
	issues = append(issues, validateDataLockMode("data_lock", shape.dataLock)...)
	issues = append(issues, validateDataLockFields(shape.dataLockFields, shape.kind, shape.attributes)...)
	issues = append(issues, validateFullTextSearch("full_text_search", shape.fullTextSearch)...)
	issues = append(issues, validateDataHistory(shape.dataHistory)...)
	issues = append(issues, validateAdditionalIndexes(shape.additionalIndexes, shape.kind,
		objectIndexTables(shape.kind, shape.attributes, shape.tableParts))...)
	reserved := shape.reservedName
	if reserved == nil {
		reserved = reservedCatalogObjectName
	}
	issues = append(issues, validateAttributes("attributes", shape.attributes, configuration, reserved)...)
	attributeNames := make(map[string]bool, len(shape.attributes))
	for _, attribute := range shape.attributes {
		attributeNames[strings.ToLower(attribute.Name)] = true
	}
	issues = append(issues, validateTableParts(shape.tableParts, attributeNames, configuration, reserved, tablePartRules{
		stored: true, use: shape.attributeUse,
		folders: shape.hierarchy.Enabled && shape.hierarchy.Kind == FoldersAndItemsHierarchy,
	})...)
	issues = append(issues, validateStandardAttributes("standard_attributes", shape.standardAttributes, standardFieldsOfKind(shape.kind), configuration)...)
	issues = append(issues, validateStandardTableParts("standard_table_parts", shape.standardTableParts, standardTablePartsOfKind(shape.kind), configuration)...)
	links := append(standardAttributeChoices("standard_attributes", shape.standardAttributes),
		standardTablePartChoices("standard_table_parts", shape.standardTableParts)...)
	links = append(links, tablePartStandardChoices(shape.tableParts)...)
	fields := append([]fieldGroup{{"attributes", shape.attributes}}, shape.extraFields...)
	issues = append(issues, validateFieldLinks(standardFieldsOfKind(shape.kind), fields, shape.tableParts, links...)...)
	issues = append(issues, validateAttributeUse([]fieldGroup{{"attributes", shape.attributes}}, shape.tableParts,
		shape.attributeUse, shape.hierarchy.Enabled && shape.hierarchy.Kind == FoldersAndItemsHierarchy)...)
	issues = append(issues, validateFormSlots(shape.forms.slots())...)
	issues = append(issues, validateFolderForms(shape.forms, shape.hierarchy)...)
	listFields := map[string]TypeKind{"code": shape.code.Type, "description": StringType}
	if shape.code.Length == 0 {
		delete(listFields, "code")
	}
	if shape.descriptionLength == 0 {
		delete(listFields, "description")
	}
	issues = append(issues, validateListSettings(shape.list, shape.attributes, listFields)...)
	return append(issues, validatePredefinedItems(shape)...)
}

// switchedOff names the standard fields a length of 0 takes away.
func (shape referenceObjectShape) switchedOff() []string {
	var fields []string
	if shape.code.Length == 0 {
		fields = append(fields, "Код")
	}
	if shape.descriptionLength == 0 {
		fields = append(fields, "Наименование")
	}
	return fields
}

// predefinedHead is what a predefined element carries alike on a catalog, a
// chart of characteristic types, a chart of accounts and a chart of
// calculation types: its identity, name, code and description, and the name
// of the element it sits under, which bounds the range of a code numbered
// within subordination.
type predefinedHead struct {
	id          uuid.UUID
	name        string
	code        string
	description string
	parent      string
}

func (item PredefinedCatalogItem) head() predefinedHead {
	return predefinedHead{id: item.ID, name: item.Name, code: item.Code, description: item.Description, parent: item.Parent}
}

func predefinedHeads[T interface{ head() predefinedHead }](items []T) []predefinedHead {
	heads := make([]predefinedHead, len(items))
	for index, item := range items {
		heads[index] = item.head()
	}
	return heads
}

// emptyPredefinedCode says a code is the empty code: one of spaces alone is
// the same value as none, as the prototype's window of a predefined element
// reads it.
func emptyPredefinedCode(value string) bool {
	return strings.Trim(value, " ") == ""
}

// validatePredefinedHeads checks the heads of the predefined elements of one
// object, one way for the four kinds that have them, and returns the folded
// names it has seen, for the checks of parents and lists that follow.
func validatePredefinedHeads(heads []predefinedHead, code CatalogCode, descriptionLength int) ([]string, map[string]bool) {
	var issues []string
	names, ids := namesWithin(""), map[uuid.UUID]bool{}
	for index, head := range heads {
		prefix := fmt.Sprintf("predefined[%d]", index)
		if head.id.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if ids[head.id] {
			issues = append(issues, prefix+".id must be unique")
		}
		ids[head.id] = true
		issues = append(issues, names.take(prefix, head.name)...)
		// One rule for the four kinds - METADATA-OBJECTS.md, «Код
		// предопределённого элемента — одно правило на четыре вида». The
		// empty code is allowed with automatic codes and without them: with
		// them the element gets the next code when it is created, without
		// them it keeps the empty one; the empty numeric code is zero. A code
		// on an element of an object whose code is switched off is the code it
		// had while there was one: the prototype keeps it (106/40/75 items of
		// 4/3/7 catalogs), and the items of objects that never had a code
		// carry an empty one. It is carried, creates nothing - see
		// PredefinedCatalogItem.EffectiveCode - and is a note.
		switch {
		case code.Length == 0:
		case emptyPredefinedCode(head.code):
		default:
			if _, err := normalizeCatalogCode(code, head.code); err != nil {
				issues = append(issues, prefix+".code is invalid: "+err.Error())
			}
		}
		if !utf8.ValidString(head.description) {
			issues = append(issues, prefix+".description must be UTF-8 text")
		} else if utf8.RuneCountInString(head.description) > descriptionLength {
			issues = append(issues, fmt.Sprintf("%s.description must not exceed %d characters", prefix, descriptionLength))
		}
	}
	return append(issues, predefinedCodeRepeats(heads, code)...), names.taken
}

// predefinedCodeRepeats refuses a code repeated among the predefined elements
// of an object that checks its codes for repeats - rule 6 of METADATA-OBJECTS.md,
// «Код предопределённого элемента», as the configurator's window refuses it,
// but naming both elements and the code. Empty codes with automatic codes are
// no repeat: each element gets a next code of its own. A code numbered within
// subordination repeats among the rows of one parent, folders and items
// together, as the configurator walk of 14.10.2026 found; a predefined element
// has no owner, so within an owner is the whole object. Two codes are one when
// the column holds them as one: the empty code and one of spaces, two numbers of
// one value, a fixed code and the same code padded.
func predefinedCodeRepeats(heads []predefinedHead, code CatalogCode) []string {
	if code.Length == 0 || !code.Unique {
		return nil
	}
	type place struct{ parent, code string }
	var issues []string
	first := map[place]string{}
	for index, head := range heads {
		value := head.code
		if emptyPredefinedCode(value) {
			if code.Auto {
				continue
			}
			value = ""
		}
		value, err := normalizeCatalogCode(code, value)
		if err != nil {
			continue // refused above as an invalid code
		}
		if code.FixedLength {
			value = strings.TrimRight(value, " ")
		}
		key := place{code: value}
		if code.Series == SubordinationSeries {
			key.parent = strings.ToLower(head.parent)
		}
		if name, ok := first[key]; ok {
			issues = append(issues, fmt.Sprintf("predefined[%d] %s: code %q repeats the code of predefined %s, and this object checks its codes for repeats",
				index, head.name, head.code, name))
			continue
		}
		first[key] = head.name
	}
	return issues
}

func validatePredefinedItems(shape referenceObjectShape) []string {
	issues, _ := validatePredefinedHeads(predefinedHeads(shape.predefined), shape.code, shape.descriptionLength)
	for index, item := range shape.predefined {
		prefix := fmt.Sprintf("predefined[%d]", index)
		// The kind alone does not make folders: a flat object carries the kind
		// of the hierarchy it would have, the way the prototype writes it.
		if item.IsFolder && !(shape.hierarchy.Enabled && shape.hierarchy.Kind == FoldersAndItemsHierarchy) {
			issues = append(issues, prefix+".is_folder needs a hierarchy of folders and items")
		}
		if item.Parent != "" && !shape.hierarchy.Enabled {
			issues = append(issues, prefix+".parent needs a hierarchy")
		}
		if item.Parent != "" && strings.EqualFold(item.Parent, item.Name) {
			issues = append(issues, prefix+".parent is the item itself")
		}
	}
	return append(issues, validatePredefinedTree(shape.predefined, shape.hierarchy)...)
}

// validatePredefinedTree resolves the parents the predefined items name.
//
// Until the three standard fields reached the data this could be left alone,
// because the tree was never written: the synchronization created every
// predefined row at the top level whatever the description said. Now that it
// is written, a parent that is not there, one that is an item where only a
// folder holds rows, or a ring of parents is a tree the database would refuse
// halfway through creating it - with a message about a foreign key, naming
// none of the items involved.
func validatePredefinedTree(items []PredefinedCatalogItem, hierarchy Hierarchy) []string {
	byName := make(map[string]PredefinedCatalogItem, len(items))
	for _, item := range items {
		byName[strings.ToLower(item.Name)] = item
	}
	var issues []string
	for index, item := range items {
		if item.Parent == "" {
			continue
		}
		prefix := fmt.Sprintf("predefined[%d]", index)
		parent, ok := byName[strings.ToLower(item.Parent)]
		if !ok {
			issues = append(issues, prefix+".parent "+item.Parent+" is not a predefined item of this object")
			continue
		}
		if hierarchy.Kind == FoldersAndItemsHierarchy && !parent.IsFolder {
			issues = append(issues, prefix+".parent "+parent.Name+" is an item, and only a folder holds rows")
		}
		// Walking up from every item is enough to find a ring, and the walk is
		// bounded by the number of items: a ring is reached again before that.
		seen := map[string]bool{strings.ToLower(item.Name): true}
		for current := parent; ; {
			key := strings.ToLower(current.Name)
			if seen[key] {
				issues = append(issues, prefix+".parent makes a ring through "+current.Name)
				break
			}
			seen[key] = true
			if current.Parent == "" {
				break
			}
			next, ok := byName[strings.ToLower(current.Parent)]
			if !ok {
				break
			}
			current = next
		}
	}
	return append(issues, validatePredefinedLevels(items, byName, hierarchy)...)
}

// validatePredefinedLevels holds the predefined tree to the limit of levels by
// the rule a row written later is held to - see checkTreePlacement: the level
// of items is a level, so a folder takes one more than it stands at. Without it
// the description is read and the base refuses it when the items are created.
// A ring or a missing parent is reported by the walk above, so here the walk
// only stops on them.
func validatePredefinedLevels(items []PredefinedCatalogItem, byName map[string]PredefinedCatalogItem, hierarchy Hierarchy) []string {
	if !hierarchy.Enabled || !hierarchy.LimitLevels {
		return nil
	}
	folders := hierarchy.Kind == FoldersAndItemsHierarchy
	var issues []string
	for index, item := range items {
		levels := 1
		if folders && item.IsFolder {
			levels = 2
		}
		for current, steps := item, 0; current.Parent != "" && steps < len(items); steps++ {
			parent, ok := byName[strings.ToLower(current.Parent)]
			if !ok {
				break
			}
			levels++
			current = parent
		}
		if levels > hierarchy.LevelCount {
			issues = append(issues, fmt.Sprintf("predefined[%d] %s takes %d levels counting the level of items, and the hierarchy allows %d",
				index, item.Name, levels, hierarchy.LevelCount))
		}
	}
	return issues
}

func validateAttributes(path string, attributes []Attribute, configuration project.Project, reserved func(string) bool) []string {
	return validateAttributesIn(path, attributes, configuration, reserved, placeStored)
}

func validateAttributesIn(path string, attributes []Attribute, configuration project.Project, reserved func(string) bool, place typePlace) []string {
	var issues []string
	names, ids := namesWithin(""), map[uuid.UUID]bool{}
	for index, attribute := range attributes {
		prefix := fmt.Sprintf("%s[%d]", path, index)
		if attribute.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if ids[attribute.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		ids[attribute.ID] = true
		issues = append(issues, names.take(prefix, attribute.Name)...)
		if reserved != nil && reserved(foldedName(attribute.Name)) {
			issues = append(issues, prefix+".name is reserved")
		}
		issues = append(issues, validateTitle(prefix+".title", attribute.Title, configuration)...)
		issues = append(issues, validateTypesIn(prefix+".types", attribute.Types, place)...)
		issues = append(issues, validateFieldSettings(prefix, attribute, configuration)...)
		issues = append(issues, validateFieldStorage(prefix, attribute)...)
	}
	return issues
}

func reservedCatalogObjectName(name string) bool {
	return reservedStandardName(CatalogKind, name) || reservedDataVersionName(name)
}

// reservedDataVersionName keeps ВерсияДанных, the version of the data every
// stored object of the eight reference kinds carries - a property of the
// reference, the object and the selection in the prototype, and a field of
// the query table. A concurrent write is caught by it. The column under it is
// ours and named apart, so an attribute named Версия is free, as it is in the
// prototype (6/6/6 catalogs of the test configurations have one).
func reservedDataVersionName(name string) bool {
	switch foldStandardName(name) {
	case "версияданных", "dataversion":
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
	_, err := io.Copy(writer, &content)
	return err
}

func decodeStrict(source string, reader io.Reader, target any) error {
	// No ceiling on the size - see project.DecodeSource.
	content, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read %s: %w", source, err)
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

// decodeDescription is how a description of any kind is read: strictly, so
// that a field the model does not know is refused rather than dropped, and
// then checked by checkDescription. A step every description takes goes here
// and nowhere else.
func decodeDescription[T any](source string, reader io.Reader, configuration project.Project, kindIssues func(T, project.Project) []string) (T, error) {
	var value T
	if err := decodeStrict(source, reader, &value); err != nil {
		var zero T
		return zero, err
	}
	if err := checkDescription(source, value, configuration, kindIssues); err != nil {
		var zero T
		return zero, err
	}
	return value, nil
}

// checkDescription checks a description of any kind: first its head, which
// every description has, then what its kind adds. Every issue is reported at
// once, and a description in another format says so, whatever else is wrong.
func checkDescription[T any](source string, value T, configuration project.Project, kindIssues func(T, project.Project) []string) error {
	head := headOf(value)
	issues := append(validateBase(head, configuration), kindIssues(value, configuration)...)
	return issuesError(source, head.format, issues)
}

// descriptionHead is what every description says of itself, whatever its
// kind: the format it is written in, its identifier, its name and its synonym.
type descriptionHead struct {
	format int
	id     uuid.UUID
	name   string
	title  LocalizedText
}

// headOf reads the head of a description. Every type a description is read
// into has the four fields, and every list of the catalog keeps descriptions
// of that type (TestEverySourceKindIsChecked).
func headOf(value any) descriptionHead {
	item := reflect.ValueOf(value)
	return descriptionHead{
		format: int(item.FieldByName("Format").Int()),
		id:     item.FieldByName("ID").Interface().(uuid.UUID),
		name:   item.FieldByName("Name").String(),
		title:  item.FieldByName("Title").Interface().(LocalizedText),
	}
}

// object is the head of a description as the catalog gives it out.
func (head descriptionHead) object() ObjectHead {
	return ObjectHead{ID: head.id, Name: head.name, Title: head.title}
}

func validateBase(head descriptionHead, configuration project.Project) []string {
	var issues []string
	if head.format != CurrentFormat {
		issues = append(issues, fmt.Sprintf("format must be %d", CurrentFormat))
	}
	if head.id.IsZero() {
		issues = append(issues, "id must be a non-zero UUID")
	}
	if !validIdentifier(head.name) {
		issues = append(issues, "name must start with a letter or an underscore and contain only letters, digits or underscores")
	}
	if utf8.RuneCountInString(head.name) > maxNameLength {
		issues = append(issues, "name must not exceed 255 characters")
	}
	return append(issues, validateTitle("title", head.title, configuration)...)
}

func validateTitle(path string, title LocalizedText, configuration project.Project) []string {
	var issues []string
	// No translation at all is a text left empty, and the prototype saves it:
	// wherever it is shown, the name stands in (checked by the owner on the
	// platform, 01.10.2026; the configurations being moved leave it empty 38,
	// 37 and 24 times - attributes, forms, templates, modules, pictures).
	// A translation in a language the configuration does not declare (erp,
	// acc and sb carry «en» beside ru and uk) and one with no code at all (the
	// synonym of a standard table part of a chart, 7/6/1) are kept: the help
	// ties a text to no list. Both are notes. Only a code that cannot be a code
	// is refused.
	for language, value := range title {
		if language != "" && !project.LanguageCodeShape(language) {
			issues = append(issues, path+"."+language+" is not a language code")
		}
		if !validLocalizedText(value) {
			issues = append(issues, path+"."+language+" must say something in printable characters, line breaks allowed")
		}
	}
	return issues
}

// standsUnqualified checks a type of a kind that takes no qualifier: where the
// place does not take the kind, the refusal says what the kind is.
func standsUnqualified(prefix string, item Type, allowed bool, what string, place typePlace) []string {
	if !allowed {
		return []string{prefix + ".kind " + string(item.Kind) + " " + what + " and cannot stand " + placeName(place)}
	}
	if item.Length != 0 || item.Precision != 0 || item.Scale != 0 || item.FixedLength || item.NonNegative || item.DateParts != "" {
		return []string{prefix + " has unsupported qualifiers"}
	}
	return nil
}

func validateTypes(path string, types []Type) []string {
	return validateTypesIn(path, types, placeStored)
}

// validateTypesIn checks a type description where it stands - see typePlace.
func validateTypesIn(path string, types []Type, place typePlace) []string {
	// No ceiling on the number of types: the prototype names none, and the
	// configurations being moved have type descriptions of more than six
	// hundred. A composite type is stored as one value column whatever its
	// size.
	// An attribute of a running object may be left with no type at all, and
	// the prototype takes it as arbitrary: the configurations being moved do
	// it 20, 16 and 15 times, every time on the object and never in a table
	// part. An attribute of a form is left so 6253, 4910 and 2023 times. A
	// defined type may be «Произвольный» too: the designer offers it first in
	// the editor of the type and «Проверка конфигурации» takes it (opis of
	// the defined type, 14.10.2026); a stored field given such a defined type
	// is refused when the database is built. A stored field must say what it
	// holds.
	if len(types) == 0 {
		if place == placeRunningObject || place == placeFormAttribute || place == placeDefinedType {
			return nil
		}
		return []string{path + " must contain at least one type"}
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
		_, referenced := typeOwners[item.Kind]
		referenced = referenced || item.Kind == VanishedType
		// The object of any report: an attribute of a form may be typed so
		// without naming the report - the main attribute of the common forms
		// of a report, of its settings and of its variant, which one form
		// serves for every report. The prototype writes it 31 times in the
		// configurations being moved; no other object type stands unnamed in
		// them.
		anyReport := item.Kind == ReportObjectType && place == placeFormAttribute && item.Reference == nil
		if referenced && !anyReport && (item.Reference == nil || item.Reference.IsZero()) {
			issues = append(issues, prefix+".reference is required")
		}
		if !referenced && item.Reference != nil {
			issues = append(issues, prefix+".reference is not allowed")
		}
		// Nothing is refused for referencing itself. A chart may allow
		// characteristics of itself: the configurator offers the chart in its
		// own value type and saves it (checked on 8.3.27 by the owner,
		// 04.10.2026). A defined type in itself is refused below with every
		// defined type in a defined type, and one elsewhere naming its owner
		// names no defined type and is refused at load.
		switch {
		case item.Kind == PlatformType && strings.TrimSpace(item.Name) == "":
			issues = append(issues, prefix+".name is required: a platform type is known by its name")
		case item.Kind != PlatformType && item.Name != "":
			issues = append(issues, prefix+".name belongs to a platform type")
		}
		if storedNowhere(item.Kind) {
			issues = append(issues, standsUnqualified(prefix, item, mayStandUnstored(place), "is not a type of anything the infobase stores", place)...)
			continue
		}
		if len(types) > 1 && standsAlone(item.Kind, place) {
			issues = append(issues, prefix+".kind "+string(item.Kind)+" stands only by itself "+placeName(place)+": the designer excludes every other type beside it")
		}
		if place == placeDefinedType && IsTypeSet(item.Kind) {
			issues = append(issues, prefix+".kind "+string(item.Kind)+" cannot stand in a defined type: the designer offers neither another defined type nor a set the platform fills itself")
			continue
		}
		if formOnlyKinds[item.Kind] {
			issues = append(issues, standsUnqualified(prefix, item, formKindAllowedIn(item.Kind, place), "is a type of an attribute of a form", place)...)
			continue
		}
		// The designer does not offer binary data as the type of an attribute,
		// of an object or of a form alike (checked by the owner, 30.09.2026),
		// and none of the configurations being moved has one in a form.
		if place == placeFormAttribute && item.Kind == BinaryDataType {
			issues = append(issues, prefix+".kind binary-data cannot stand in an attribute of a form: the designer does not offer it")
			continue
		}
		if isObjectType(item.Kind) || isValueType(item.Kind) {
			issues = append(issues, standsUnqualified(prefix, item, allowedIn(item.Kind, place), "lives in memory only", place)...)
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
			if item.Length < 0 || item.Length > maxStringLength {
				issues = append(issues, fmt.Sprintf("%s.length must be 0..%d, 0 being unlimited", prefix, maxStringLength))
			}
			if item.Precision != 0 || item.Scale != 0 {
				issues = append(issues, prefix+" has invalid numeric qualifiers")
			}
			// A fixed-length string is padded to its length, so there has to
			// be a length to pad to.
			if item.FixedLength && item.Length == 0 {
				issues = append(issues, prefix+".fixed_length requires a length")
			}
			if item.FixedLength && item.Length > maxFixedStringLength {
				issues = append(issues, fmt.Sprintf("%s.fixed_length allows a length of at most %d: the designer makes «Фиксированная» unavailable at a longer one", prefix, maxFixedStringLength))
			}
		case NumberType:
			// A number of no length is a number of any length. Only a form
			// holds one - 113, 20 and 37 times with any sign and 7, 5 and 6
			// non-negative, never in a field of an object, a report or a
			// data processor - and the form keeps it in memory, where no
			// column needs a precision.
			unlimited := place == placeFormAttribute && item.Precision == 0
			if !unlimited && (item.Precision < 1 || item.Precision > maxNumberDigits) {
				issues = append(issues, fmt.Sprintf("%s.precision must be 1..%d", prefix, maxNumberDigits))
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
		default:
			// A boolean, an identifier, a value storage, a reference and a set
			// take no qualifier.
			if item.Kind != BooleanType && item.Kind != UUIDType && item.Kind != ValueStorageType &&
				storedAs(item.Kind) == storageUnsaid && !IsTypeSet(item.Kind) {
				issues = append(issues, prefix+".kind is unsupported")
			} else if item.Length != 0 || item.Precision != 0 || item.Scale != 0 {
				issues = append(issues, prefix+" has unsupported qualifiers")
			}
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

// validIdentifier is the prototype's rule for a name: a letter or an
// underscore first, then letters, digits and underscores. The designer's own
// hint says a name starts with a letter, but it accepts "_1111" for an object
// and an attribute alike - the underscore counts as a letter (checked by the
// owner on the platform, 01.10.2026) - and the configurations being moved keep
// such names: enumeration values _100 and _50_50, and some 3600 names with an
// underscore inside.
func validIdentifier(value string) bool {
	for index, symbol := range []rune(value) {
		if symbol == '_' || unicode.IsLetter(symbol) {
			continue
		}
		if index == 0 || !unicode.IsDigit(symbol) {
			return false
		}
	}
	return value != ""
}

// siblingNames are the names a list of siblings has taken so far, folded: the
// values of an enumeration, the attributes of an object, its table parts, its
// commands, the parameters of a form. A name is the prototype's identifier
// (validIdentifier) of at most 255 characters (METADATA-OBJECTS «Пределы на
// состав»), unique among its siblings with case ignored - one rule, which
// eighteen places once wrote out each for itself, some without the length.
type siblingNames struct {
	// within ends the refusal of a repeated name, after "must be unique": "
	// within commands", ": a reference to a route point carries this name".
	within string
	taken  map[string]bool
}

func namesWithin(within string) *siblingNames {
	return &siblingNames{within: within, taken: map[string]bool{}}
}

// take checks the name of the sibling at prefix and takes it.
func (names *siblingNames) take(prefix, name string) []string {
	var issues []string
	if !validIdentifier(name) || utf8.RuneCountInString(name) > maxNameLength {
		issues = append(issues, fmt.Sprintf("%s.name must be a valid identifier of at most %d characters", prefix, maxNameLength))
	}
	folded := foldedName(name)
	if names.taken[folded] {
		issues = append(issues, prefix+".name must be unique"+names.within)
	}
	names.taken[folded] = true
	return issues
}

// validLocalizedText is a translation as the prototype keeps it: text that
// says something, over as many lines as it needs and as long as it needs. A
// synonym, a tooltip, an explanation and a caption on a route map are all
// written on several lines in the configurations being moved - about 1150
// tooltips, 39 synonyms, 70 explanations, 49 titles - and two tooltips of erp
// run past 512 characters. The 512 this used to stop at had no source. Other
// control characters are still refused: nothing types them, and they would
// break whatever the text is shown in.
// validLocalizedText accepts a translation that says something, or says
// spaces alone: the prototype keeps a tooltip of one space (sb, 24 times).
// Such a text is kept as written,
// reads as empty where it is shown, and is a note. An empty translation is no
// translation and is refused.
func validLocalizedText(value string) bool {
	if !utf8.ValidString(value) || value == "" {
		return false
	}
	for _, symbol := range value {
		if unicode.IsControl(symbol) && symbol != '\n' && symbol != '\r' && symbol != '\t' {
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
	value.Explanation = cloneTitle(value.Explanation)
	value.ExtendedPresentation = cloneTitle(value.ExtendedPresentation)
	if value.Default != nil {
		defaultValue := *value.Default
		value.Default = &defaultValue
	}
	if value.DefaultForm != nil {
		id := *value.DefaultForm
		value.DefaultForm = &id
	}
	settings := cloneFieldSettings(Attribute{Presentation: value.Presentation, Choice: value.Choice})
	value.Presentation, value.Choice = settings.Presentation, settings.Choice
	value.CommonAttributeFields = cloneAttributes(value.CommonAttributeFields)
	return value
}
func cloneSessionParameter(value SessionParameter) SessionParameter {
	value.Title, value.Types = cloneTitle(value.Title), cloneTypes(value.Types)
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
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
	value.Characteristics = cloneObjectCharacteristics(value.Characteristics)
	value.StandardAttributes = cloneStandardAttributes(value.StandardAttributes)
	return value
}
func cloneDefinedType(value DefinedTypeObject) DefinedTypeObject {
	value.Title, value.Types = cloneTitle(value.Title), cloneTypes(value.Types)
	return value
}

func cloneCatalogDefinition(value CatalogDefinition) CatalogDefinition {
	value.Title = cloneTitle(value.Title)
	value.Owners = slices.Clone(value.Owners)
	value.Attributes = cloneAttributes(value.Attributes)
	value.TableParts = cloneTableParts(value.TableParts)
	value.Forms = cloneFormSet(value.Forms)
	value.List.SearchFields = slices.Clone(value.List.SearchFields)
	value.Predefined = slices.Clone(value.Predefined)
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
	value.Characteristics = cloneObjectCharacteristics(value.Characteristics)
	value.StandardAttributes = cloneStandardAttributes(value.StandardAttributes)
	value.Presentations = clonePresentations(value.Presentations)
	value.ObjectInput = cloneObjectInput(value.ObjectInput)
	value.BasedOn = slices.Clone(value.BasedOn)
	value.DataLockFields = cloneDataLockFields(value.DataLockFields)
	value.AdditionalIndexes = cloneAdditionalIndexes(value.AdditionalIndexes)
	return value
}

func cloneAttributes(value []Attribute) []Attribute {
	result := slices.Clone(value)
	for index := range result {
		result[index] = cloneAttribute(result[index])
	}
	return result
}

// cloneAttribute copies one field with everything it owns. It exists apart from
// cloneAttributes because the fields of a register are not a plain slice of
// attributes: a dimension of a calculation register and a field of an
// accounting register carry an attribute plus their own, and they have to copy
// the attribute part the same way or the copy would share what it shows and how
// it is chosen with the original.
func cloneAttribute(value Attribute) Attribute {
	value.Title = cloneTitle(value.Title)
	value.Types = cloneTypes(value.Types)
	if value.BinaryDataStorageField != nil {
		id := *value.BinaryDataStorageField
		value.BinaryDataStorageField = &id
	}
	return cloneFieldFilling(cloneFieldSettings(value))
}
