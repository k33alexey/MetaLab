package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// noteExamples writes, for each kind of note, the smallest project in which the
// model accepts a state of that kind. The test below refuses a kind that has no
// example here, so a dropped refusal cannot get a kind of note without a proof
// that the note appears.
var noteExamples = map[NoteKind]func(t *testing.T, root string){
	NoteUnresolvedReference: func(t *testing.T, root string) {
		id := uuid.MustNew().String()
		writeMetadata(t, root, SubsystemKind, id, "format: 1\nid: "+id+"\nname: Продажи\ntitle: {ru: Продажи}\nmembers: ["+uuid.MustNew().String()+"]\n")
	},
	NoteUnresolvedPath: func(t *testing.T, root string) {
		noteCatalog(t, root, "", noteField("string", "    choice: {parameter_links: [{name: Отбор.Владелец, source: {unresolved: \"-3\"}}]}\n"))
	},
	NoteUnusedBound: func(t *testing.T, root string) {
		noteCatalog(t, root, "", noteField("string", "    presentation: {min_value: \"0\"}\n"))
	},
	NoteFillingNotHeld: func(t *testing.T, root string) {
		noteCatalog(t, root, "", noteField("string", "    filling: {value: {kind: boolean, data: \"true\"}}\n"))
	},
	NoteValueOfVanishedType: func(t *testing.T, root string) {
		noteCatalog(t, root, "", noteField("string",
			"    filling: {value: {kind: unresolved-reference, data: \"466cbe70-c94c-4cdc-a0fb-f9f9084bdef2.00000000-0000-0000-0000-000000000000\"}}\n"))
	},
	NoteInactiveHierarchy: func(t *testing.T, root string) {
		noteCatalog(t, root, "hierarchy: {kind: folders-and-items, folders_on_top: true, level_count: 2}\n", "")
	},
	NoteFolderUseWithoutFolds: func(t *testing.T, root string) {
		noteCatalog(t, root, "hierarchy: {enabled: true, kind: items}\n", noteField("string", "    use: for-folder-and-item\n"))
	},
	NoteFieldOutsideIndex: func(t *testing.T, root string) {
		noteCatalog(t, root, "full_text_search: dont-use\n", noteField("string", "    full_text_search: use\n"))
	},
	NoteFieldOutsideHistory: func(t *testing.T, root string) {
		noteCatalog(t, root, "data_history: dont-use\n", noteField("string", "    data_history: use\n"))
	},
	NoteInactivePosting: func(t *testing.T, root string) {
		id := uuid.MustNew().String()
		writeMetadata(t, root, DocumentKind, id, "format: 1\nid: "+id+"\nname: Заметка\ntitle: {ru: Заметка}\n"+
			"number: {type: string, length: 11, periodicity: year}\nposting: {allowed: false, real_time: deny, records_deletion: auto}\n")
	},
	NotePictureWithoutPicture: func(t *testing.T, root string) {
		noteCatalog(t, root, "commands:\n  - {id: "+uuid.MustNew().String()+", name: Подбор, title: {ru: Подбор}, representation: picture}\n", "")
		writeCommandModule(t, root, CatalogKind, "Товары", "Подбор")
	},
	NoteLevelOfHierarchyTable: func(t *testing.T, root string) {
		writeSalesCube(t, root, noteCubeBody(), goodsDimTableBody+"level_number: 1\n")
	},
	NoteParameterUseNoType: func(t *testing.T, root string) {
		noteCatalog(t, root, "commands:\n  - {id: "+uuid.MustNew().String()+", name: Подбор, title: {ru: Подбор}, parameter_use: single}\n", "")
		writeCommandModule(t, root, CatalogKind, "Товары", "Подбор")
	},
}

// noteCubeBody is the sales cube without the bound its date dimension carries:
// a bound on a date is a note of its own.
func noteCubeBody() string {
	return strings.Replace(salesCubeBody, "    presentation: {min_value: \"2020-01-01T00:00:00Z\"}\n", "", 1)
}

// noteCatalog writes a catalog with the properties given and, when there is
// one, the attribute given.
func noteCatalog(t *testing.T, root, properties, attribute string) {
	t.Helper()
	id := uuid.MustNew().String()
	body := "format: 1\nid: " + id + "\nname: Товары\ntitle: {ru: Товары}\ncode: {type: string, length: 9, auto: true}\ndescription_length: 150\n" + properties
	if attribute != "" {
		body += "attributes:\n" + attribute
	}
	writeMetadata(t, root, CatalogKind, id, body)
}

// noteField is one attribute of the given primitive type with the lines given.
func noteField(kind, lines string) string {
	return "  - id: " + uuid.MustNew().String() + "\n    name: Поле\n    title: {ru: Поле}\n    types: [{kind: " + kind + ", length: 10}]\n" + lines
}

// Every kind of note is described for the report and has a way to be found.
//
// Defect caught: a kind added to the list with no words for the report - the
// import report would show a code nobody can read - or with neither a rule nor
// a place in the load, so its notes never appear; and a rule for a kind the
// report does not list, whose notes would be dropped.
func TestEveryNoteKindIsDescribed(t *testing.T) {
	t.Parallel()
	seen := map[NoteKind]bool{}
	for _, info := range NoteKinds() {
		if info.Kind == "" || info.Meaning == "" || info.Behaviour == "" {
			t.Errorf("kind %q is not described: %+v", info.Kind, info)
		}
		if seen[info.Kind] {
			t.Errorf("kind %q is listed twice", info.Kind)
		}
		seen[info.Kind] = true
		if noteRules[info.Kind] == nil && info.Kind != NoteUnresolvedReference {
			t.Errorf("kind %q has no rule and is not collected by the load: its notes never appear", info.Kind)
		}
	}
	for kind := range noteRules {
		if !seen[kind] {
			t.Errorf("rule for kind %q, which the report does not list", kind)
		}
	}
}

// A clean project carries no notes, and each kind of note appears on its
// example - and only that kind.
//
// Defect caught: a kind of note that never fires (the refusal was dropped and
// the state is accepted silently - the one thing the rule forbids); a rule that
// fires on a sound project, burying the real notes; and a kind with no example,
// which is how the first defect would come back unseen.
func TestEveryNoteKindAppearsOnItsExample(t *testing.T) {
	t.Parallel()
	clean, err := Load(metadataProject(t))
	if err != nil {
		t.Fatal(err)
	}
	if notes := clean.Notes(); len(notes) != 0 {
		t.Fatalf("a sound project carries notes: %+v", notes)
	}
	for _, info := range NoteKinds() {
		example, ok := noteExamples[info.Kind]
		if !ok {
			t.Errorf("kind %q has no example in noteExamples", info.Kind)
			continue
		}
		t.Run(string(info.Kind), func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			example(t, root)
			catalog, err := Load(root)
			if err != nil {
				t.Fatalf("the example is refused, not accepted: %v", err)
			}
			notes := catalog.Notes()
			if len(notes) == 0 {
				t.Fatal("the example is accepted silently: no note")
			}
			for _, note := range notes {
				if note.Kind != info.Kind || note.Where == "" {
					t.Fatalf("note %+v on the example of %q", note, info.Kind)
				}
			}
		})
	}
}

// The sound twin of every example carries no note: the same setting where it
// does act.
//
// Defect caught: a rule that fires on the setting itself rather than on the
// setting where it cannot act - a bound on a number, a filling of the field's
// own type, a hierarchy that is on, «for folder and item» with folders, a
// field in the index of an indexed object, posting settings of a document that
// posts, a command drawn by a picture it has, a parameter mode with a type, a
// hierarchical dimension table at level 0.
// Such a rule buries the real notes under thousands of false ones.
func TestSoundSettingsCarryNoNotes(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	commandID := uuid.MustNew().String()
	id := uuid.MustNew().String()
	writeMetadata(t, root, CatalogKind, id, "format: 1\nid: "+id+"\nname: Товары\ntitle: {ru: Товары}\ncode: {type: string, length: 9, auto: true}\ndescription_length: 150\n"+
		"hierarchy: {enabled: true, kind: folders-and-items, folders_on_top: true, limit_levels: true, level_count: 2}\n"+
		"full_text_search: use\ndata_history: use\n"+
		"commands:\n  - {id: "+commandID+", name: Подбор, title: {ru: Подбор}, representation: picture, picture: {standard: Начислить}, "+
		"parameter: [{kind: catalog, reference: "+id+"}], parameter_use: single}\n"+
		"attributes:\n"+
		"  - id: "+uuid.MustNew().String()+"\n    name: Цена\n    title: {ru: Цена}\n    types: [{kind: number, precision: 15, scale: 2}]\n"+
		"    presentation: {min_value: \"0,5\", max_value: \"100\"}\n    filling: {value: {kind: number, data: \"1\"}}\n"+
		"    use: for-folder-and-item\n    full_text_search: use\n    data_history: use\n"+
		"  - id: "+uuid.MustNew().String()+"\n    name: Комментарий\n    title: {ru: Комментарий}\n    types: [{kind: string, length: 10}]\n"+
		"    filling: {value: {kind: undefined, data: \"\"}}\n")
	writeCommandModule(t, root, CatalogKind, "Товары", "Подбор")
	writeSalesCube(t, root, noteCubeBody(), goodsDimTableBody)
	document := uuid.MustNew().String()
	writeMetadata(t, root, DocumentKind, document, "format: 1\nid: "+document+"\nname: Заметка\ntitle: {ru: Заметка}\n"+
		"number: {type: string, length: 11, periodicity: year}\nposting: {allowed: true, real_time: deny, records_deletion: auto}\n")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if notes := catalog.Notes(); len(notes) != 0 {
		t.Fatalf("sound settings carry notes: %+v", notes)
	}
}
