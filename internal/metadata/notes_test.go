package metadata

import (
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
