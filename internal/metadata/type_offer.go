package metadata

import "github.com/k33alexey/MetaLab/internal/kinds"

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

// choicePlace is what one place offers, from the top of the tree down. Every
// place has «Составной тип данных».
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
	// required says the window does not close on «ОК» with nothing marked:
	// «Необходимо выбрать тип значения.». Where the type may be empty it
	// closes silently.
	required bool
}

// choicePlaces are the places the prototype's window was gone through in,
// with what it offered there (the opis, «Состав по местам»). The places whose
// window offers what lives in memory only - an attribute of a data processor,
// a report and a form, a defined type, a session parameter, the fields of an
// external source - wait for the model to take all of it (BLOCKS, block 2б).
var choicePlaces = map[TypeChoicePlace]choicePlace{
	StoredFieldChoice: {model: placeStored, primitives: []TypeKind{NumberType, StringType, DateType, BooleanType, ValueStorageType, UUIDType},
		references: referencesUnderSets, characteristics: true, definedTypes: true, anyReference: true, required: true},
	CommandParameterChoice:    {model: placeCommandParameter, references: referencesInFolders, definedTypes: true},
	CharacteristicValueChoice: {model: placeStored, primitives: []TypeKind{NumberType, StringType, DateType, BooleanType}, references: referencesInFolders, definedTypes: true, required: true},
	FilterCriterionChoice: {model: placeStored, arbitrary: true, primitives: []TypeKind{NumberType, StringType, DateType, BooleanType, UUIDType},
		references: referencesUnderSets, characteristics: true, definedTypes: true, anyReference: true},
}

// typeNames are the names of the types the window shows by a word of their own
// rather than by the kind of an object (help, 8.3.27: the pages of the types).
var typeNames = map[TypeKind]kinds.Words{
	NumberType: {Russian: "Число", English: "Number"}, StringType: {Russian: "Строка", English: "String"},
	DateType: {Russian: "Дата", English: "Date"}, BooleanType: {Russian: "Булево", English: "Boolean"},
	ValueStorageType:  {Russian: "ХранилищеЗначения", English: "ValueStorage"},
	UUIDType:          {Russian: "УникальныйИдентификатор", English: "UUID"},
	AnyReferenceSet:   {Russian: "ЛюбаяСсылка", English: "AnyRef"},
	CharacteristicSet: {Russian: "Характеристика", English: "Characteristic"},
}

// arbitraryName is the row of no type. The help names the row only in
// Russian; the English word is ours.
var arbitraryName = kinds.Words{Russian: "Произвольный", English: "Arbitrary"}

// TypeOffer is what the window offers in a place, as Studio draws it.
type TypeOffer struct {
	Place TypeChoicePlace `json:"place"`
	// Required refuses «ОК» with nothing marked.
	Required bool           `json:"required"`
	Rows     []TypeOfferRow `json:"rows"`
	Limits   TypeLimits     `json:"limits"`
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
	Arbitrary bool        `json:"arbitrary,omitempty"`
	Name      kinds.Words `json:"name"`
	// Icon is the picture of the kind of object the row's types name, empty
	// where there is none.
	Icon string `json:"icon,omitempty"`
	// ObjectKind is the type of the rows inside, each naming one of Objects
	// through Type.Reference.
	ObjectKind TypeKind              `json:"objectKind,omitempty"`
	Objects    []ReferenceTypeObject `json:"objects,omitempty"`
}

// TypeLimits are the model's ceilings on the qualifiers, which the window
// checks as the value is entered.
type TypeLimits struct {
	StringLength      int `json:"stringLength"`
	FixedStringLength int `json:"fixedStringLength"`
	NumberDigits      int `json:"numberDigits"`
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
		Place: place, Required: at.required,
		Limits:   TypeLimits{StringLength: maxStringLength, FixedStringLength: maxFixedStringLength, NumberDigits: maxNumberDigits},
		NewTypes: newTypes,
	}
	if at.arbitrary {
		offer.Rows = append(offer.Rows, TypeOfferRow{Arbitrary: true, Name: arbitraryName})
	}
	for _, kind := range at.primitives {
		offer.Rows = append(offer.Rows, TypeOfferRow{Kind: kind, Name: typeNames[kind]})
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
			Objects: catalog.referenceTypeObjects(DefinedTypeKind)})
	}
	if at.anyReference {
		offer.Rows = append(offer.Rows, TypeOfferRow{Kind: AnyReferenceSet, Name: typeNames[AnyReferenceSet]})
	}
	return offer, true
}
