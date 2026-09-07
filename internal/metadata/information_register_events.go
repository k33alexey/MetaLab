package metadata

import (
	"context"
	"errors"
	"fmt"
)

type InformationRegisterEvent string

const (
	InformationRegisterEventBeforeWrite InformationRegisterEvent = "before-write"
	InformationRegisterEventOnWrite     InformationRegisterEvent = "on-write"
	InformationRegisterEventAfterWrite  InformationRegisterEvent = "after-write"
)

var ErrInformationRegisterWriteCancelled = errors.New("information register write was cancelled by an event handler")

type InformationRegisterEventHandler interface {
	HandleInformationRegisterEvent(context.Context, InformationRegisterEvent, *InformationRegisterRecordSet, bool) (cancel bool, err error)
}

type InformationRegisterEventHandlerFunc func(context.Context, InformationRegisterEvent, *InformationRegisterRecordSet, bool) (bool, error)

func (handler InformationRegisterEventHandlerFunc) HandleInformationRegisterEvent(ctx context.Context, event InformationRegisterEvent, set *InformationRegisterRecordSet, replace bool) (bool, error) {
	return handler(ctx, event, set, replace)
}

func dispatchInformationRegisterEvent(ctx context.Context, handler InformationRegisterEventHandler, event InformationRegisterEvent, set *InformationRegisterRecordSet, replace bool) error {
	if handler == nil {
		return nil
	}
	cancel, err := handler.HandleInformationRegisterEvent(ctx, event, set, replace)
	if err != nil {
		return fmt.Errorf("information register event %s: %w", event, err)
	}
	if !cancel {
		return nil
	}
	if event == InformationRegisterEventAfterWrite {
		return fmt.Errorf("information register event %s cannot cancel an operation", event)
	}
	return ErrInformationRegisterWriteCancelled
}
