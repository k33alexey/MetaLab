package metadata

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestDocumentEventCancellationContract(t *testing.T) {
	t.Parallel()
	handler := DocumentEventHandlerFunc(func(_ context.Context, _ DocumentEvent, _ *DocumentRecord) (bool, error) {
		return true, nil
	})
	for _, event := range []DocumentEvent{DocumentEventFillCheck, DocumentEventBefore, DocumentEventOnWrite} {
		if err := dispatchDocumentEvent(context.Background(), handler, event, &DocumentRecord{}); !errors.Is(err, ErrDocumentWriteCancelled) {
			t.Fatalf("event %s error = %v", event, err)
		}
	}
	for _, event := range []DocumentEvent{DocumentEventFill, DocumentEventAfter} {
		if err := dispatchDocumentEvent(context.Background(), handler, event, &DocumentRecord{}); err == nil {
			t.Fatalf("event %s accepted cancellation", event)
		}
	}
}

func TestDocumentEventsCannotChangeImmutableState(t *testing.T) {
	t.Parallel()
	id := uuid.MustNew()
	catalog := &Catalog{
		Documents:      []DocumentDefinition{{ID: id, Name: "Продажа"}},
		documentByName: map[string]int{"продажа": 0}, documentByID: map[uuid.UUID]int{id: 0},
	}
	repository := &DocumentRepository{catalog: catalog, now: func() time.Time { return time.Now().UTC() }}
	changing := DocumentEventHandlerFunc(func(_ context.Context, _ DocumentEvent, record *DocumentRecord) (bool, error) {
		record.Posted = !record.Posted
		return false, nil
	})
	if _, err := repository.New(context.Background(), "Продажа", changing); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("New() error = %v", err)
	}
	record := &DocumentRecord{Reference: DocumentReference{DocumentID: id, ObjectID: uuid.MustNew()}, Date: time.Now().UTC()}
	if err := repository.Save(context.Background(), record, changing); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("Save() error = %v", err)
	}
}
