package metadata

import "fmt"

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
	DontIndex                  IndexMode = "dont-index"
	IndexField                 IndexMode = "index"
	IndexWithAdditionalOrder   IndexMode = "index-with-additional-order"
	maxFieldFillValueTextBytes           = 1 << 20
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

// forFolders says whether this use reaches folders at all.
func (use AttributeUse) forFolders() bool {
	return use == UseForFolder || use == UseForFolderAndItem
}

// FieldFilling is what a new value starts as. The value itself is one half;
// the other is whether a new object takes it at all, because an object may be
// filled from what created it instead.
type FieldFilling struct {
	Value *Value `yaml:"value,omitempty" json:"value,omitempty"`
	// FromFillingValue fills a new object's field from Value. Without it the
	// value stands in the description and is used only where something asks
	// for it by name.
	FromFillingValue bool `yaml:"from_filling_value,omitempty" json:"fromFillingValue,omitempty"`
}

// validateFieldStorage checks what a field says about filling, finding and
// belonging.
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
	if !validAttributeUse(attribute.Use) {
		issues = append(issues, prefix+".use must be for-item, for-folder or for-folder-and-item")
	}
	if filling := attribute.Filling.Value; filling != nil {
		issues = append(issues, validateFieldBound(prefix+".filling.value", *filling, attribute.Types)...)
		if len(filling.Data) > maxFieldFillValueTextBytes {
			issues = append(issues, fmt.Sprintf("%s.filling.value must not be longer than %d bytes", prefix, maxFieldFillValueTextBytes))
		}
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
			case attribute.Use.forFolders() && !folders:
				issues = append(issues, path+".use reaches folders, and this object has none")
			}
		}
	}
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
