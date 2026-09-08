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
	DocumentEventPosting      DocumentEvent = "posting"
	DocumentEventUndoPosting  DocumentEvent = "undo-posting"
)

// DocumentWriteMode describes the persistent operation requested for a document.
type DocumentWriteMode string

const (
	DocumentWrite       DocumentWriteMode = "write"
	DocumentPost        DocumentWriteMode = "post"
	DocumentUndoPosting DocumentWriteMode = "undo-posting"
)

// DocumentPostingMode distinguishes regular and real-time posting requests.
type DocumentPostingMode string

const (
	DocumentPostingRegular  DocumentPostingMode = "regular"
	DocumentPostingRealTime DocumentPostingMode = "real-time"
)

var (
	ErrDocumentWriteCancelled   = errors.New("document write was cancelled by an event handler")
	ErrDocumentDeleteCancelled  = errors.New("document deletion was cancelled by an event handler")
	ErrDocumentPostingCancelled = errors.New("document posting was cancelled by an event handler")
)

type documentOperationContextKey struct{}

type documentOperation struct {
	writeMode   DocumentWriteMode
	postingMode DocumentPostingMode
}

func withDocumentOperation(ctx context.Context, writeMode DocumentWriteMode, postingMode DocumentPostingMode) context.Context {
	return context.WithValue(ctx, documentOperationContextKey{}, documentOperation{writeMode: writeMode, postingMode: postingMode})
}

func documentOperationFromContext(ctx context.Context) documentOperation {
	if operation, ok := ctx.Value(documentOperationContextKey{}).(documentOperation); ok {
		return operation
	}
	return documentOperation{writeMode: DocumentWrite, postingMode: DocumentPostingRegular}
}

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
	if event == DocumentEventPosting || event == DocumentEventUndoPosting {
		return ErrDocumentPostingCancelled
	}
	if event != DocumentEventFillCheck && event != DocumentEventBefore && event != DocumentEventOnWrite {
		return fmt.Errorf("document event %s cannot cancel an operation", event)
	}
	return ErrDocumentWriteCancelled
}
