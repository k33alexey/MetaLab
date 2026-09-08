package metadata

import (
	"context"
	"errors"
	"fmt"
)

type AccumulationRegisterEvent string

const (
	AccumulationRegisterEventBeforeWrite AccumulationRegisterEvent = "before-write"
	AccumulationRegisterEventOnWrite     AccumulationRegisterEvent = "on-write"
	AccumulationRegisterEventAfterWrite  AccumulationRegisterEvent = "after-write"
)

var ErrAccumulationRegisterWriteCancelled = errors.New("accumulation register write was cancelled by an event handler")

type AccumulationRegisterEventHandler interface {
	HandleAccumulationRegisterEvent(context.Context, AccumulationRegisterEvent, *AccumulationRegisterRecordSet, bool) (cancel bool, err error)
}

type AccumulationRegisterEventHandlerFunc func(context.Context, AccumulationRegisterEvent, *AccumulationRegisterRecordSet, bool) (bool, error)

func (handler AccumulationRegisterEventHandlerFunc) HandleAccumulationRegisterEvent(ctx context.Context, event AccumulationRegisterEvent, set *AccumulationRegisterRecordSet, replace bool) (bool, error) {
	return handler(ctx, event, set, replace)
}

func dispatchAccumulationRegisterEvent(ctx context.Context, handler AccumulationRegisterEventHandler, event AccumulationRegisterEvent, set *AccumulationRegisterRecordSet, replace bool) error {
	if handler == nil {
		return nil
	}
	cancel, err := handler.HandleAccumulationRegisterEvent(ctx, event, set, replace)
	if err != nil {
		return fmt.Errorf("accumulation register event %s: %w", event, err)
	}
	if !cancel {
		return nil
	}
	if event == AccumulationRegisterEventAfterWrite {
		return fmt.Errorf("accumulation register event %s cannot cancel an operation", event)
	}
	return ErrAccumulationRegisterWriteCancelled
}
