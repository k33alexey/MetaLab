package metadata

import (
	"strings"
	"testing"
)

const postingDocumentID = "50000000-0000-4000-8000-000000000041"

func postingDocumentYAML(body string) string {
	return `format: 1
id: ` + postingDocumentID + `
name: Накладная
title: {ru: Накладная}
number: {type: string, length: 11, auto: true, periodicity: year}
` + body
}

// All six settings survive the round trip. Five of them were read and dropped
// before this point: the model carried one boolean, so a document that denied
// real-time posting or kept its records on unposting came in looking exactly
// like one that did neither.
func TestDocumentKeepsAllSixPostingSettings(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, DocumentKind, postingDocumentID, postingDocumentYAML(`posting:
  allowed: true
  real_time: deny
  records_deletion: on-unpost
  records_writing: selected
  sequence_filling: off
  privileged: true
  unpost_privileged: true
`))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	document, ok := catalog.DocumentDefinition("накладная")
	if !ok {
		t.Fatal("the document was not read")
	}
	want := DocumentPosting{
		Allowed: true, RealTime: RealTimePostingDeny,
		RecordsDeletion: RegisterRecordsDeleteOnUnpost, RecordsWriting: RegisterRecordsWriteSelected,
		SequenceFilling: SequenceFillOff, Privileged: true, UnpostPrivileged: true,
	}
	if document.Posting != want {
		t.Fatalf("posting = %+v, want %+v", document.Posting, want)
	}
}

// Each of the five enumerations has the values the prototype has and no others.
// A misspelling read as silence would be the worst of the three outcomes: it
// looks like the default, and the default is the permissive answer.
func TestPostingSettingsTakeOnlyThePrototypesValues(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]struct {
		posting string
		message string
	}{
		"оперативное проведение":         {"real_time: sometimes", "posting.real_time must be allow or deny"},
		"удаление движений":              {"records_deletion: maybe", "posting.records_deletion must be auto, on-unpost or off"},
		"запись движений":                {"records_writing: all", "posting.records_writing must be selected or modified"},
		"заполнение последовательностей": {"sequence_filling: later", "posting.sequence_filling must be auto or off"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, DocumentKind, postingDocumentID,
				postingDocumentYAML("posting:\n  allowed: true\n  "+body.posting+"\n"))
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), body.message) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// Five of the six describe how posting happens. A document that forbids posting
// has no "how", and saying otherwise is a setting on an operation that cannot
// occur - which reads as if it were in force.
//
// Sequence filling is the exception and stays allowed: a sequence follows
// documents by date whether they are posted or not.
func TestHowAPostingHappensNeedsAPostingToHappen(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]struct {
		posting string
		refused bool
	}{
		"оперативное проведение":            {"real_time: deny", true},
		"удаление движений":                 {"records_deletion: off", true},
		"запись движений":                   {"records_writing: selected", true},
		"привилегированный режим":           {"privileged: true", true},
		"отмена в привилегированном режиме": {"unpost_privileged: true", true},
		// A sequence follows a document by its date, posted or not.
		"заполнение последовательностей": {"sequence_filling: off", false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, DocumentKind, postingDocumentID,
				postingDocumentYAML("posting:\n  "+body.posting+"\n"))
			_, err := Load(root)
			switch {
			case body.refused && err == nil:
				t.Fatal("a setting about posting was accepted on a document that forbids posting")
			case body.refused && !strings.Contains(err.Error(), "posting.allowed is off"):
				t.Fatalf("err = %v", err)
			case !body.refused && err != nil:
				t.Fatalf("sequence filling was refused on an unposted document: %v", err)
			}
		})
	}
}

// Silence means what ML did before the settings existed, and that is the point:
// a document that says nothing behaves exactly as it did, so the six settings
// are an addition rather than a change.
func TestUnstatedPostingSettingsKeepTheOldBehaviour(t *testing.T) {
	t.Parallel()
	if !AllowsRealTimePosting("") {
		t.Error("an unstated real-time setting must allow posting by the current moment")
	}
	for name, mode := range map[string]RegisterRecordsDeletion{"unstated": "", "auto": RegisterRecordsDeleteAuto} {
		for _, writeMode := range []DocumentWriteMode{DocumentPost, DocumentUndoPosting} {
			if !DeletesRegisterRecords(mode, writeMode) {
				t.Errorf("%s deletion on %s: records must be wiped", name, writeMode)
			}
		}
	}
	// On unposting only: nothing is wiped when posting begins, which is what
	// the help says in so many words.
	if DeletesRegisterRecords(RegisterRecordsDeleteOnUnpost, DocumentPost) {
		t.Error("records were wiped at the start of posting under on-unpost")
	}
	if !DeletesRegisterRecords(RegisterRecordsDeleteOnUnpost, DocumentUndoPosting) {
		t.Error("records were kept on unposting under on-unpost")
	}
	for _, writeMode := range []DocumentWriteMode{DocumentPost, DocumentUndoPosting} {
		if DeletesRegisterRecords(RegisterRecordsDeleteOff, writeMode) {
			t.Errorf("records were wiped on %s although deletion is off", writeMode)
		}
	}
}
