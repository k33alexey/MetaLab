package metadata

import (
	"strings"
	"testing"
)

// A source of several kinds takes only the events whose handler gets the same
// parameters on all of them. The two sources of the opis (14.10.2026): the
// objects of catalogs and of documents share seven events by name and are
// offered six - not BeforeWrite, whose handler a document gives the modes of
// writing and posting; the objects of catalogs and the record sets of
// accounting registers share three and are offered one. Both lists are
// written here, not read from the model.
//
// Defect caught: a composite source checked by the name of the event alone,
// so that a handler written for one kind is called with the parameters of
// another; or an event the designer offers refused.
func TestACompositeSourceTakesOnlyAnEventWithTheSameHandler(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		source        string
		offered, held []string
	}{
		{"[{kind: catalog-object}, {kind: document-object}]",
			[]string{"before-delete", "fill-check", "fill", "generate-from-data-history-version-processing", "on-copy", "on-write"},
			[]string{"before-write"}},
		{"[{kind: catalog-object}, {kind: accounting-register-record-set}]",
			[]string{"fill-check"}, []string{"before-write", "on-write"}},
	} {
		for _, event := range test.offered {
			root := subscriptionSourceProject(t)
			writeSubscription(t, root, test.source, event)
			if _, err := Load(root); err != nil {
				t.Errorf("%s, %s refused: %v", test.source, event, err)
			}
		}
		for _, event := range test.held {
			root := subscriptionSourceProject(t)
			writeSubscription(t, root, test.source, event)
			if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "a source of several kinds takes only an event whose handler gets the same on all of them") {
				t.Errorf("%s, %s: %v", test.source, event, err)
			}
		}
	}
}

// The managers of a report, a data processor, a settings storage and a filter
// criterion are a source, with the events the designer offers on them (opis,
// 14.10.2026): the getting of a form; the four of a storage; none on a
// criterion, so a subscription to it cannot be filled.
//
// Defect caught: the four managers refused as a source, as they were; an event
// the manager does not have accepted; a criterion given the event the help
// gives its manager's module, which the designer does not offer.
func TestASubscriptionListensToTheManagersOfAReportAProcessorAStorageAndACriterion(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ source, event, refusal string }{
		{"[{kind: report-manager}]", "form-get-processing", ""},
		{"[{kind: data-processor-manager}]", "form-get-processing", ""},
		{"[{kind: report-manager}, {kind: data-processor-manager}, {kind: catalog-manager}]", "form-get-processing", ""},
		{"[{kind: settings-storage-manager}]", "load-processing", ""},
		{"[{kind: settings-storage-manager}]", "save-processing", ""},
		{"[{kind: settings-storage-manager}]", "get-description-processing", ""},
		{"[{kind: settings-storage-manager}]", "set-description-processing", ""},
		{"[{kind: report-manager}]", "before-write", "is not an event of report-manager"},
		{"[{kind: settings-storage-manager}]", "form-get-processing", "is not an event of settings-storage-manager"},
		{"[{kind: filter-criterion-manager}]", "form-get-processing", "is not an event of filter-criterion-manager"},
	} {
		root := subscriptionSourceProject(t)
		writeSubscription(t, root, test.source, test.event)
		_, err := Load(root)
		switch {
		case test.refusal == "" && err != nil:
			t.Errorf("%s, %s refused: %v", test.source, test.event, err)
		case test.refusal != "" && (err == nil || !strings.Contains(err.Error(), test.refusal)):
			t.Errorf("%s, %s: %v, want %q", test.source, test.event, err, test.refusal)
		}
	}
}

// Every event a subscription may name has the parameters of its handler, and
// every parameters given a kind of their own are of an event the kind has.
//
// Defect caught: an event added to a kind without its parameters, which would
// compare as no parameters and pass a composite it should not; an exception
// written against a kind that lacks the event.
func TestEverySubscriptionEventHasItsHandler(t *testing.T) {
	t.Parallel()
	for kind, events := range subscriptionEvents {
		for _, event := range events {
			if _, ok := handlerParameters[event]; !ok {
				t.Errorf("%s.%s has no parameters of its handler", kind, event)
			}
		}
	}
	for kind, events := range handlerParametersOf {
		for event := range events {
			if _, ok := kindSubscriptionEvent(kind, eventSubscriptionName(event)); !ok {
				t.Errorf("%s has parameters for %s, which it does not have", kind, event)
			}
		}
	}
}
