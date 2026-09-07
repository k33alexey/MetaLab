package metadata

import (
	"context"
	"errors"
	"fmt"
)

type DocumentEvent string

const (
	DocumentEventFill         DocumentEvent = "fill"
	DocumentEventFillCheck    DocumentEvent = "fill-check"
	DocumentEventBefore       DocumentEvent = "before-write"
	DocumentEventOnWrite      DocumentEvent = "on-write"
	DocumentEventAfter        DocumentEvent = "after-write"
	DocumentEventBeforeDelete DocumentEvent = "before-delete"
)

var (
	ErrDocumentWriteCancelled  = errors.New("document write was cancelled by an event handler")
	ErrDocumentDeleteCancelled = errors.New("document deletion was cancelled by an event handler")
)

type DocumentEventHandler interface {
	HandleDocumentEvent(context.Context, DocumentEvent, *DocumentRecord) (cancel bool, err error)
}

type DocumentEventHandlerFunc func(context.Context, DocumentEvent, *DocumentRecord) (bool, error)

func (handler DocumentEventHandlerFunc) HandleDocumentEvent(ctx context.Context, event DocumentEvent, record *DocumentRecord) (bool, error) {
	return handler(ctx, event, record)
}

func dispatchDocumentEvent(ctx context.Context, handler DocumentEventHandler, event DocumentEvent, record *DocumentRecord) error {
	if handler == nil {
		return nil
	}
	cancel, err := handler.HandleDocumentEvent(ctx, event, record)
	if err != nil {
		return fmt.Errorf("document event %s: %w", event, err)
	}
	if !cancel {
		return nil
	}
	if event == DocumentEventBeforeDelete {
		return ErrDocumentDeleteCancelled
	}
	if event != DocumentEventFillCheck && event != DocumentEventBefore && event != DocumentEventOnWrite {
		return fmt.Errorf("document event %s cannot cancel an operation", event)
	}
	return ErrDocumentWriteCancelled
}
