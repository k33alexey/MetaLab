package metadata

import (
	"context"
	"errors"
	"fmt"
)

type CatalogEvent string

const (
	CatalogEventFill      CatalogEvent = "fill"
	CatalogEventFillCheck CatalogEvent = "fill-check"
	CatalogEventBefore    CatalogEvent = "before-write"
	CatalogEventOnWrite   CatalogEvent = "on-write"
	CatalogEventAfter     CatalogEvent = "after-write"
)

var ErrCatalogWriteCancelled = errors.New("catalog write was cancelled by an event handler")

// CatalogEventHandler receives catalog lifecycle events in their fixed order.
// Returning cancel is supported only by fill-check, before-write and on-write.
type CatalogEventHandler interface {
	HandleCatalogEvent(context.Context, CatalogEvent, *CatalogRecord) (cancel bool, err error)
}

type CatalogEventHandlerFunc func(context.Context, CatalogEvent, *CatalogRecord) (bool, error)

func (handler CatalogEventHandlerFunc) HandleCatalogEvent(ctx context.Context, event CatalogEvent, record *CatalogRecord) (bool, error) {
	return handler(ctx, event, record)
}

func dispatchCatalogEvent(ctx context.Context, handler CatalogEventHandler, event CatalogEvent, record *CatalogRecord) error {
	if handler == nil {
		return nil
	}
	cancel, err := handler.HandleCatalogEvent(ctx, event, record)
	if err != nil {
		return fmt.Errorf("catalog event %s: %w", event, err)
	}
	if cancel {
		if event != CatalogEventFillCheck && event != CatalogEventBefore && event != CatalogEventOnWrite {
			return fmt.Errorf("catalog event %s cannot cancel an operation", event)
		}
		return ErrCatalogWriteCancelled
	}
	return nil
}
