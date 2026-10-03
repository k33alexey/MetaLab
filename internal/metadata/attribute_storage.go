package metadata

import (
	"fmt"
	"slices"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Beside how a field is shown and picked, a field says what is put in it to
// begin with, and how the database is to help find it later. The prototype
// keeps five such settings on every attribute, and the demonstration
// configuration sets four of them on almost every one.

// IndexMode is what the platform calls indexing: none, an index, or an index
// with additional ordering. It is three-valued and not a flag, and that is the
// prototype's own shape - the syntax assistant of 8.3.27 lists exactly these
// three for an attribute, a table part attribute and a register dimension.
//
// What the third one adds physically the help does not say. It says only what
// both indexing modes make possible - a selection may be filtered and ordered
// by such a field - so both build an index here and the difference is carried
// and not yet acted upon. Guessing the composition of somebody else's index is
// how a configuration gets a plan it never had.
type IndexMode string

const (
	DontIndex                IndexMode = "dont-index"
	IndexField               IndexMode = "index"
	IndexWithAdditionalOrder IndexMode = "index-with-additional-order"
)

// indexes says whether the field gets an index of its own.
func (mode IndexMode) indexes() bool {
	return mode == IndexField || mode == IndexWithAdditionalOrder
}

func validIndexMode(mode IndexMode) bool {
	switch mode {
	case "", DontIndex, IndexField, IndexWithAdditionalOrder:
		return true
	default:
		return false
	}
}

// AttributeUse says whether a field belongs to items, to folders, or to both.
// It exists for attributes of catalogs and of charts of characteristic types
// and nowhere else - the help says so, and the demonstration configuration
// writes it on all 1122 attributes of those two kinds and on none of the
// documents'.
type AttributeUse string

const (
	UseForItem          AttributeUse = "for-item"
	UseForFolder        AttributeUse = "for-folder"
	UseForFolderAndItem AttributeUse = "for-folder-and-item"
)

func validAttributeUse(use AttributeUse) bool {
	switch use {
	case "", UseForItem, UseForFolder, UseForFolderAndItem:
		return true
	default:
		return false
	}
}

// FieldFilling is what the standard filling of a new object puts into the
// field, and from where.
type FieldFilling struct {
	// Value is the filling value: what the field gets when nothing came for
	// it from the filling data, or when the field does not take filling data
	// at all.
	Value *Value `yaml:"value,omitempty" json:"value,omitempty"`
	// FromFillingValue is the prototype's FillFromFillingValue, «заполнять из
	// данных заполнения», and despite its English name it is about the filling
	// data, not the filling value: on, the field takes what the filling data
	// carries for it - in practice the filter of the list a new object is
	// created from; off, or with nothing there for it, the field gets Value.
	// The platform sets it itself on the parent, the owner and the leading
	// dimensions of an information register. Carried; the standard filling is
	// not executed yet.
	FromFillingValue bool `yaml:"from_filling_value,omitempty" json:"fromFillingValue,omitempty"`
}

// validateFieldStorage checks what a field says about filling, finding and
// belonging.
// validateMovementFieldStorage refuses what a field of a register of movements
// does not have: a filling value, filling from filling data, and data history.
//
// All three belong to a field a person fills in on a new object. A movement is
// not filled in by hand - it is written by posting, from the document - so there
// is no new record for a filling value to prefill. And data history keeps
// versions of an object; a movement has no object of its own, it has a recorder,
// and the versions live in that recorder's history.
//
// The export says the same, and says it in a complete record: a catalog
// attribute is written with 29 properties, a field of an accumulation,
// accounting or calculation register with 24, and the five missing are these
// three plus indexing and use. An information register is the exception on every
// count - its records are entered by hand - and keeps them all.
func validateMovementFieldStorage(prefix string, field Attribute) []string {
	var issues []string
	if field.Filling.Value != nil || field.Filling.FromFillingValue {
		issues = append(issues, prefix+".filling belongs to a field somebody fills in: a movement is written by posting, not by hand")
	}
	if field.DataHistory != "" {
		issues = append(issues, prefix+".data_history belongs to an object: a movement has a recorder, and its versions live in that recorder's history")
	}
	return issues
}

// validateResourceIndexing refuses an index on a resource of a register that is
// not an information register: «ОбъектМетаданных: Ресурс.Индексирование — для
// ресурсов регистра сведений». A record of a register of movements is found by
// its dimensions and its periods, never by an amount, so an index on an amount
// is a table nobody queries and a write everybody pays for.
//
// Refusing rather than quietly turning it off: a property silently dropped is
// exactly what the import report calls «перенесено с потерей смысла», and a
// register whose resource was meant to be indexed would come out looking
// migrated. The demo export shows nothing is lost by refusing - not one
// resource of the accumulation, accounting and calculation registers is
// indexed, while 52 of 699 information register resources are.
func validateResourceIndexing(prefix string, indexing IndexMode) []string {
	if indexing == "" || indexing == DontIndex {
		return nil
	}
	return []string{prefix + ".indexing belongs to a dimension: a record of this register is never found by an amount"}
}

func validateFieldStorage(prefix string, attribute Attribute) []string {
	var issues []string
	if !validIndexMode(attribute.Indexing) {
		issues = append(issues, prefix+".indexing must be dont-index, index or index-with-additional-order")
	}
	// Full-text search and data history are two-valued in the prototype, not
	// three: there is no "let the platform decide" for either, and accepting
	// one here would be accepting a setting nothing can act on.
	for name, mode := range map[string]UsageMode{
		"full_text_search": attribute.FullTextSearch, "data_history": attribute.DataHistory,
	} {
		switch mode {
		case "", UsageUse, UsageDontUse:
		default:
			issues = append(issues, prefix+"."+name+" must be use or dont-use")
		}
	}
	if !validFillCheck(attribute.FillChecking) {
		issues = append(issues, prefix+".fill_checking must be dont-check or show-error")
	}
	if !validAttributeUse(attribute.Use) {
		issues = append(issues, prefix+".use must be for-item, for-folder or for-folder-and-item")
	}
	// A filling value of a type the field cannot hold is carried and fills
	// nothing - see EffectiveFillingValue.
	if filling := attribute.Filling.Value; filling != nil {
		issues = append(issues, validateDesignTimeValue(prefix+".filling.value", *filling)...)
	}
	return issues
}

// validateAttributeUse checks where a field may say whom it belongs to. Two
// rules, and each catches a different silence: a kind that has no folders at
// all cannot have a field only folders have, and an object whose hierarchy has
// no folders has no groups for such a field to belong to.
func validateAttributeUse(groups []fieldGroup, parts []TablePart, kindHasFolders, folders bool) []string {
	var issues []string
	for _, group := range groups {
		for index, attribute := range group.fields {
			path := fmt.Sprintf("%s[%d]", group.path, index)
			switch {
			case attribute.Use == "":
			case !kindHasFolders:
				issues = append(issues, path+".use belongs to an attribute of a catalog or a chart of characteristic types, and this is neither")
			// «For folder and item» on an object without folders is what the
			// prototype leaves behind when folders are switched off: the
			// configurations being moved carry it on 45 attributes. It reads as
			// «for item» there. «For folder» alone has nothing to fall back to.
			case attribute.Use == UseForFolder && !folders:
				issues = append(issues, path+".use reaches folders, and this object has none")
			}
		}
	}
	// Every kind hands its field groups over here, so the check of the binary
	// data storage rides along: it needs the same groups, to find the field
	// the switch names.
	issues = append(issues, validateBinaryDataStorage(groups, parts)...)
	// A line of a table part belongs to the row that holds it, and the row
	// belongs to one object: there is nothing for a line to belong to apart
	// from that. The prototype writes the setting on the table part itself and
	// never on a field of one.
	for partIndex, part := range parts {
		for index, attribute := range part.Attributes {
			if attribute.Use != "" {
				issues = append(issues, fmt.Sprintf("table_parts[%d].attributes[%d].use belongs to an attribute of the object, not of a table part", partIndex, index))
			}
		}
	}
	return issues
}

func cloneFieldFilling(attribute Attribute) Attribute {
	attribute.Filling.Value = cloneValuePointer(attribute.Filling.Value)
	return attribute
}

// FillCheck is the prototype's check of whether a field was filled in. It is
// not a constraint of the database and never was: the help of 8.3.27 says the
// check runs in ПроверитьЗаполнение, raises ОбработкаПроверкиЗаполнения and
// tells the user where to fix it, while the value itself sits in the table
// untroubled. Unfilled there means equal to the default of the field's type,
// which is why the database never sees a NULL to refuse.
//
// The one hard refusal the platform does have belongs to a register dimension
// and is a setting of its own - deny incomplete values - not this one.
type FillCheck string

const (
	DontCheckFilling FillCheck = "dont-check"
	ShowFillingError FillCheck = "show-error"
)

func validFillCheck(check FillCheck) bool {
	switch check {
	case "", DontCheckFilling, ShowFillingError:
		return true
	default:
		return false
	}
}

// checked says whether the field takes part in the automatic check.
func (check FillCheck) checked() bool { return check == ShowFillingError }

// validateBinaryDataStorage checks where a value is kept in a binary data
// storage. Three rules, each against a setting that would be read by nothing:
// the mode is use or do not use, and only a value of the type ХранилищеЗначения
// has anything to put there; the switch appears only with use; and the switch
// is a boolean attribute of the same object - of the same table part for a
// field of one - because it is read on the same row as the value it decides
// about. An attribute and nothing else: the configurator offers the attributes
// of the object to pick from, and neither a dimension nor a resource of a
// register, even for the switch of a resource.
func validateBinaryDataStorage(groups []fieldGroup, parts []TablePart) []string {
	var issues []string
	check := func(path string, field Attribute, siblings map[uuid.UUID]Attribute) {
		switch field.BinaryDataStorage {
		case "", UsageUse, UsageDontUse:
		default:
			issues = append(issues, path+".binary_data_storage must be use or dont-use")
		}
		if field.BinaryDataStorage != "" && !slices.ContainsFunc(field.Types, func(item Type) bool { return item.Kind == ValueStorageType }) {
			issues = append(issues, path+".binary_data_storage belongs to a field of the type value storage: nothing else is kept in a binary data storage")
		}
		if field.BinaryDataStorageField == nil {
			return
		}
		if field.BinaryDataStorage != UsageUse {
			issues = append(issues, path+".binary_data_storage_field needs binary_data_storage: use - with the storage not in use there is nothing to switch")
			return
		}
		switcher, ok := siblings[*field.BinaryDataStorageField]
		if !ok {
			issues = append(issues, path+".binary_data_storage_field names no attribute of this object")
			return
		}
		if single, ok := SingleType(switcher.Types); !ok || single.Kind != BooleanType {
			issues = append(issues, path+".binary_data_storage_field must name a boolean field: it decides yes or no for each row")
		}
	}
	siblings := map[uuid.UUID]Attribute{}
	for _, group := range groups {
		if group.path != "attributes" {
			continue
		}
		for _, field := range group.fields {
			siblings[field.ID] = field
		}
	}
	for _, group := range groups {
		for index, field := range group.fields {
			check(fmt.Sprintf("%s[%d]", group.path, index), field, siblings)
		}
	}
	for partIndex, part := range parts {
		lines := map[uuid.UUID]Attribute{}
		for _, field := range part.Attributes {
			lines[field.ID] = field
		}
		for index, field := range part.Attributes {
			check(fmt.Sprintf("table_parts[%d].attributes[%d]", partIndex, index), field, lines)
		}
	}
	return issues
}
