package metadata

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestCatalogEventCancellationContract(t *testing.T) {
	t.Parallel()
	handler := CatalogEventHandlerFunc(func(_ context.Context, _ CatalogEvent, _ *CatalogRecord) (bool, error) {
		return true, nil
	})
	for _, event := range []CatalogEvent{CatalogEventFillCheck, CatalogEventBefore, CatalogEventOnWrite} {
		if err := dispatchCatalogEvent(context.Background(), handler, event, &CatalogRecord{}); !errors.Is(err, ErrCatalogWriteCancelled) {
			t.Fatalf("event %s error = %v", event, err)
		}
	}
	if err := dispatchCatalogEvent(context.Background(), handler, CatalogEventAfter, &CatalogRecord{}); err == nil {
		t.Fatal("after-write cancellation was accepted")
	}
}

func TestCatalogEventsCannotChangeRecordIdentity(t *testing.T) {
	t.Parallel()
	id := uuid.MustNew()
	catalog := &Catalog{
		Catalogs:      []CatalogDefinition{{ID: id, Name: "Товары"}},
		catalogByName: map[string]int{"товары": 0}, catalogByID: map[uuid.UUID]int{id: 0},
	}
	repository := &CatalogRepository{catalog: catalog}
	changing := CatalogEventHandlerFunc(func(_ context.Context, _ CatalogEvent, record *CatalogRecord) (bool, error) {
		record.Version++
		return false, nil
	})
	if _, err := repository.New(context.Background(), "Товары", changing); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("New() error = %v", err)
	}
	record := &CatalogRecord{Reference: CatalogReference{CatalogID: id, ObjectID: uuid.MustNew()}}
	if err := repository.Save(context.Background(), record, changing); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("Save() error = %v", err)
	}
}
