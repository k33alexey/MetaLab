package metadata

// Участие в полнотекстовом поиске — whether the object as a whole is in the
// index at all.
//
// The flag on a field was done long ago; the one on the object was not, and
// without it the field's flag answers a question nobody asked: a field is
// indexed inside an object, and an object outside the index has no inside.
//
// The carriers are the twelve kinds that keep data of their own - a catalog, a
// document, the three charts, an exchange plan, a business process, a task and
// all four registers - plus a common attribute, which carries the field's
// flag the way any field does. The syntax assistant gives exactly that list,
// and the demonstration configuration writes the property on exactly those
// kinds. A constant, a sequence and a recalculation have it nowhere.
//
// Two values, and the same two the field's flag has: Использовать and
// НеИспользовать. There is no "let the platform decide" here, and accepting
// one would accept a setting nothing can act on - the reason is written out
// beside the field's flag, in attribute_storage.go.

// FullTextSearchMode is whether this object's data is in the full-text index.
type FullTextSearchMode string

const (
	FullTextSearchUse     FullTextSearchMode = "use"
	FullTextSearchDontUse FullTextSearchMode = "dont-use"
)

func validFullTextSearchMode(mode FullTextSearchMode) bool {
	switch mode {
	case "", FullTextSearchUse, FullTextSearchDontUse:
		return true
	default:
		return false
	}
}

func validateFullTextSearch(path string, mode FullTextSearchMode) []string {
	if validFullTextSearchMode(mode) {
		return nil
	}
	return []string{path + " must be use or dont-use"}
}

// validateFullTextSearchOnInputPair is the one thing that could not be checked
// until the object had a flag of its own: input by string may be told to
// search the full-text index, and an object that is not in the index finds
// nothing that way. The setting reads as working and does nothing, which is
// what we refuse everywhere else too.
//
// Only this pair is refused. A field flagged for the index inside an object
// that is not in it is a different matter and is left alone: the
// demonstration configuration does it four hundred and eighty-four times, and
// it reads as what it is - the field is ready for the day the object joins the
// index. The pair above is not like that, because the object cannot be found
// by input by string in the meantime and nothing says why.
func validateFullTextSearchOnInputPair(object FullTextSearchMode, onInput FullTextSearchOnInput) []string {
	if object == FullTextSearchDontUse && onInput == FullTextOnInputUse {
		return []string{"full_text_search_on_input needs full_text_search: this object is not in the index, so nothing is found by it"}
	}
	return nil
}
