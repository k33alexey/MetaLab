package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The source of a subscription is a type description, the prototype's: the
// object of one document, every document, a constant's value manager, the
// manager of every catalog, a defined type of objects. The defect it catches
// is the one the model had - a list of catalogs, documents and two kinds of
// register, which lost on import most of the 16 800 types the configurations
// being moved give their subscriptions - and an event a kind of the source
// does not have.

const (
	sourceDocument = "52000000-0000-4000-8000-000000000001"
	sourceOther    = "52000000-0000-4000-8000-000000000002"
	sourceDefined  = "32000000-0000-4000-8000-000000000001"
	sourceConstant = "12000000-0000-4000-8000-000000000001"
	sourceModule   = "62000000-0000-4000-8000-000000000001"
	sourceCode     = "62000000-0000-4000-8000-000000000002"
	sourceID       = "63000000-0000-4000-8000-000000000001"
)

func subscriptionSourceProject(t *testing.T) string {
	t.Helper()
	root := metadataProject(t)
	for _, document := range []struct{ id, name string }{{sourceDocument, "Продажа"}, {sourceOther, "Покупка"}} {
		writeMetadata(t, root, DocumentKind, document.id, "format: 1\nid: "+document.id+"\nname: "+document.name+
			"\ntitle: {ru: "+document.name+"}\nnumber: {type: string, length: 9, auto: true, periodicity: none}\n")
	}
	writeMetadata(t, root, DefinedTypeKind, sourceDefined, "format: 1\nid: "+sourceDefined+
		"\nname: ОбъектПродажи\ntitle: {ru: Объект продажи}\ntypes: [{kind: document-object, reference: "+sourceDocument+"}]\n")
	writeMetadata(t, root, ConstantKind, sourceConstant, "format: 1\nid: "+sourceConstant+
		"\nname: Флаг\ntitle: {ru: Флаг}\ntypes: [{kind: boolean}]\n")
	if err := os.WriteFile(filepath.Join(root, "modules", sourceCode+".bsl"),
		[]byte("Процедура Обработать(Источник, Отказ) Экспорт\nКонецПроцедуры\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeMetadata(t, root, CommonModuleKind, sourceModule, "format: 1\nid: "+sourceModule+
		"\nname: Подписки\ntitle: {ru: Подписки}\nserver: true\nmodule: "+sourceCode+"\n")
	return root
}

func writeSubscription(t *testing.T, root, source, event string) {
	t.Helper()
	writeMetadata(t, root, EventSubscriptionKind, sourceID, "format: 1\nid: "+sourceID+
		"\nname: Подписка\ntitle: {ru: Подписка}\nsource: "+source+"\nevent: "+event+
		"\nmodule: "+sourceModule+"\nprocedure: Обработать\n")
}

func TestASubscriptionListensToWhatThePrototypeLetsItListenTo(t *testing.T) {
	t.Parallel()
	for name, testCase := range map[string]struct{ source, event string }{
		"объект одного документа":     {"[{kind: document-object, reference: " + sourceDocument + "}]", "before-write"},
		"все документы":               {"[{kind: document-object}]", "posting"},
		"через определяемый тип":      {"[{kind: defined-type, reference: " + sourceDefined + "}]", "on-set-new-number"},
		"менеджер значения константы": {"[{kind: constant-value-manager, reference: " + sourceConstant + "}]", "on-write"},
		"менеджер всех справочников":  {"[{kind: catalog-manager}]", "form-get-processing"},
		"все планы обмена":            {"[{kind: exchange-plan-object}]", "on-send-data-to-slave"},
		"объекты и наборы вместе":     {"[{kind: document-object}, {kind: information-register-record-set}, {kind: catalog-object}]", "before-write"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := subscriptionSourceProject(t)
			writeSubscription(t, root, testCase.source, testCase.event)
			catalog, err := Load(root)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			subscription, ok := catalog.EventSubscription("Подписка")
			if !ok || len(subscription.Source) == 0 || subscription.Event != testCase.event {
				t.Fatalf("the subscription lost its source: %+v", subscription)
			}
		})
	}
}

func TestASubscriptionEventIsOneEveryKindOfTheSourceHas(t *testing.T) {
	t.Parallel()
	for name, testCase := range map[string]struct{ source, event, message string }{
		"проведение у справочника": {"[{kind: catalog-object}]", "posting", "is not an event of catalog-object"},
		"новый код у документа":    {"[{kind: document-object}]", "on-set-new-code", "is not an event of document-object"},
		"форма у объекта":          {"[{kind: document-object}]", "form-get-processing", "is not an event of document-object"},
		"проведение у одного из":   {"[{kind: document-object}, {kind: catalog-object}]", "posting", "is not an event of catalog-object"},
		"через определяемый тип":   {"[{kind: defined-type, reference: " + sourceDefined + "}]", "on-set-new-code", "is not an event of document-object"},
		"неизвестное событие":      {"[{kind: document-object}]", "on-open", "is an event of no object"},
		// «После записи» is a form's event and no object's: the prototype's
		// object and record set modules have none, so a subscription has none.
		"после записи":                   {"[{kind: catalog-object}]", "after-write", "is an event of no object"},
		"ссылка вместо объекта":          {"[{kind: document, reference: " + sourceDocument + "}]", "before-write", "is not an object, a record set or a manager"},
		"объект неизвестного документа":  {"[{kind: document-object, reference: 52000000-0000-4000-8000-000000000099}]", "before-write", "of unknown object"},
		"объект справочника на документ": {"[{kind: catalog-object, reference: " + sourceDocument + "}]", "before-write", "of unknown object"},
		"неизвестный определяемый тип":   {"[{kind: defined-type, reference: 32000000-0000-4000-8000-000000000099}]", "before-write", "unknown defined type"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := subscriptionSourceProject(t)
			writeSubscription(t, root, testCase.source, testCase.event)
			if _, err := Load(root); err == nil || !strings.Contains(err.Error(), testCase.message) {
				t.Fatalf("error = %v, expected it to say %q", err, testCase.message)
			}
		})
	}
}

// What a subscription listens to at run time is what its source names: the
// one document named, every document for the kind as a whole, the documents
// of a defined type - and not the document next to them.
func TestASubscriptionReachesTheObjectsItsSourceNames(t *testing.T) {
	t.Parallel()
	for name, testCase := range map[string]struct {
		source        string
		document      bool
		other         bool
		catalogObject bool
	}{
		"один документ":          {"[{kind: document-object, reference: " + sourceDocument + "}]", true, false, false},
		"все документы":          {"[{kind: document-object}]", true, true, false},
		"через определяемый тип": {"[{kind: defined-type, reference: " + sourceDefined + "}]", true, false, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := subscriptionSourceProject(t)
			writeSubscription(t, root, testCase.source, "before-write")
			catalog, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			reaches := func(kind TypeKind, id string) bool {
				parsed, err := uuid.Parse(id)
				if err != nil {
					t.Fatal(err)
				}
				return len(catalog.eventSubscriptionsFor(kind, parsed)) == 1
			}
			if got := reaches(DocumentObjectType, sourceDocument); got != testCase.document {
				t.Fatalf("the named document reached = %v", got)
			}
			if got := reaches(DocumentObjectType, sourceOther); got != testCase.other {
				t.Fatalf("the other document reached = %v", got)
			}
			if got := reaches(CatalogObjectType, sourceDocument); got != testCase.catalogObject {
				t.Fatalf("a catalog object of the same identifier reached = %v", got)
			}
		})
	}
}
