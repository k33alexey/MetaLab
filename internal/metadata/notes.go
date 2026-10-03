package metadata

import (
	"slices"
)

// NoteKind names one kind of state the model accepts and ML does not act upon
// as written: the state contradicts itself, or does not apply where it stands,
// and still the prototype saves it.
//
// The model accepts such a state rather than refusing it, because a refusal
// costs the whole configuration and accepting costs nothing. It never accepts
// one silently: every place is a Note, the import report lists them all, and
// each is looked into on import - see CONFORMANCE.md, «Принято, но не
// исполняется». A refusal dropped without a note kind is a defect.
//
// A note is about an anomaly, not about a mechanism ML has not built yet: data
// history carried and not kept is the second category of the report by
// property, and needs no note.
type NoteKind string

const (
	// NoteUnresolvedReference is a reference by identifier to an object the
	// configuration does not hold - see UnresolvedReference.
	NoteUnresolvedReference NoteKind = "unresolved-reference"
)

// Note is one place where the model accepted a state of a NoteKind.
type Note struct {
	Kind NoteKind
	// Where names the place the way messages about it do.
	Where string
	// Written is what stands there, when the place alone does not say it: the
	// identifier a reference points at, the value that does not fit.
	Written string
}

// NoteKindInfo describes a kind of note for the import report: what the state
// is, and what ML does with it.
type NoteKindInfo struct {
	Kind NoteKind
	// Meaning says what the state is and how it comes about in the prototype.
	Meaning string
	// Behaviour says what ML does with it: carries it, and acts as if what.
	Behaviour string
}

// noteKinds is every kind of note, in the order the report lists them. A kind
// is either collected while the configuration is read (the references, which
// only the load resolves) or found afterwards by its rule in noteRules.
var noteKinds = []NoteKindInfo{
	{
		Kind: NoteUnresolvedReference,
		Meaning: "Ссылка по идентификатору на объект, которого в конфигурации нет: " +
			"объект удалили, а ссылку оставили. Прототип такую конфигурацию сохраняет и открывает.",
		Behaviour: "Ссылка несётся как записана и действует как незаданная.",
	},
}

// noteRules finds the places of a kind in a loaded catalog, calling note for
// each. A kind collected during the load has no rule.
var noteRules = map[NoteKind]func(catalog *Catalog, note func(where, written string)){}

// NoteKinds lists every kind of note with its description, in report order.
func NoteKinds() []NoteKindInfo {
	return slices.Clone(noteKinds)
}

// Notes lists every place the last load accepted a state of some NoteKind,
// kind by kind in report order and, within a kind, in the order the places
// were met.
func (catalog *Catalog) Notes() []Note {
	var notes []Note
	for _, info := range noteKinds {
		if info.Kind == NoteUnresolvedReference {
			for _, item := range catalog.unresolved {
				notes = append(notes, Note{Kind: NoteUnresolvedReference, Where: item.Where, Written: item.ID.String()})
			}
			continue
		}
		if rule := noteRules[info.Kind]; rule != nil {
			rule(catalog, func(where, written string) {
				notes = append(notes, Note{Kind: info.Kind, Where: where, Written: written})
			})
		}
	}
	return notes
}
