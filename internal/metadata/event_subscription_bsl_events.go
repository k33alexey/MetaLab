package metadata

import (
	"context"
	"fmt"
	"sync"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
)

// catalogEventHandlers combines a catalog's own object module handler (if
// any) with every event subscription targeting it. Every handler observes
// every event; each subscription silently ignores events it was not
// declared for. Cancellation from any handler cancels the operation, but
// every handler still runs — matching how 1C keeps calling remaining
// subscribers once Отказ is already set.
type catalogEventHandlers []CatalogEventHandler

func (list catalogEventHandlers) HandleCatalogEvent(ctx context.Context, event CatalogEvent, record *CatalogRecord) (bool, error) {
	cancelled := false
	for _, handler := range list {
		if handler == nil {
			continue
		}
		cancel, err := handler.HandleCatalogEvent(ctx, event, record)
		if err != nil {
			return false, err
		}
		cancelled = cancelled || cancel
	}
	return cancelled, nil
}

type documentEventHandlers []DocumentEventHandler

func (list documentEventHandlers) HandleDocumentEvent(ctx context.Context, event DocumentEvent, record *DocumentRecord) (bool, error) {
	cancelled := false
	for _, handler := range list {
		if handler == nil {
			continue
		}
		cancel, err := handler.HandleDocumentEvent(ctx, event, record)
		if err != nil {
			return false, err
		}
		cancelled = cancelled || cancel
	}
	return cancelled, nil
}

type informationRegisterEventHandlers []InformationRegisterEventHandler

func (list informationRegisterEventHandlers) HandleInformationRegisterEvent(ctx context.Context, event InformationRegisterEvent, set *InformationRegisterRecordSet, replace bool) (bool, error) {
	cancelled := false
	for _, handler := range list {
		if handler == nil {
			continue
		}
		cancel, err := handler.HandleInformationRegisterEvent(ctx, event, set, replace)
		if err != nil {
			return false, err
		}
		cancelled = cancelled || cancel
	}
	return cancelled, nil
}

type accumulationRegisterEventHandlers []AccumulationRegisterEventHandler

func (list accumulationRegisterEventHandlers) HandleAccumulationRegisterEvent(ctx context.Context, event AccumulationRegisterEvent, set *AccumulationRegisterRecordSet, replace bool) (bool, error) {
	cancelled := false
	for _, handler := range list {
		if handler == nil {
			continue
		}
		cancel, err := handler.HandleAccumulationRegisterEvent(ctx, event, set, replace)
		if err != nil {
			return false, err
		}
		cancelled = cancelled || cancel
	}
	return cancelled, nil
}

// catalogEventSubscriptionHandler calls one common module procedure for one
// catalog's lifecycle events, matching a real 1C subscription handler
// signature: Source is passed explicitly as the first argument, followed by
// the same arguments the catalog's own module would receive.
type catalogEventSubscriptionHandler struct {
	mu           sync.Mutex
	runtime      *Runtime
	context      *vm.Context
	definition   CatalogDefinition
	subscription EventSubscriptionDefinition
	module       string
}

func (handler *catalogEventSubscriptionHandler) HandleCatalogEvent(ctx context.Context, event CatalogEvent, record *CatalogRecord) (bool, error) {
	if string(event) != handler.subscription.Event {
		return false, nil
	}
	if record == nil || record.Reference.CatalogID != handler.definition.ID {
		return false, fmt.Errorf("event subscription %s received an incompatible record", handler.subscription.Name)
	}
	if !handler.context.HasRoutine(handler.module, handler.subscription.Procedure) {
		return false, fmt.Errorf("event subscription %s references unknown procedure %s.%s", handler.subscription.Name, handler.module, handler.subscription.Procedure)
	}
	guarded, err := enterBSLEvent(ctx, handler)
	if err != nil {
		return false, fmt.Errorf("event subscription %s: %w", handler.subscription.Name, err)
	}
	handler.mu.Lock()
	defer handler.mu.Unlock()
	value, err := handler.runtime.wrapCatalogRecord(handler.definition, record)
	if err != nil {
		return false, err
	}
	_, _, arguments := catalogEventRoutine(event, handler.definition)
	_, final, err := handler.context.CallContextMutable(guarded, handler.module+"."+handler.subscription.Procedure, append([]bytecode.Value{value}, arguments...)...)
	if err != nil {
		return false, err
	}
	object := valueRuntimeCatalogObject(value)
	object.mu.Lock()
	if err := handler.runtime.syncCatalogTables(object); err != nil {
		object.mu.Unlock()
		return false, err
	}
	updated := cloneCatalogRecord(object.record)
	object.mu.Unlock()
	*record = *updated
	if event == CatalogEventFillCheck || event == CatalogEventBefore || event == CatalogEventOnWrite || event == CatalogEventBeforeDelete {
		if len(final) < 2 {
			return false, fmt.Errorf("event subscription %s did not return its cancellation argument", handler.subscription.Name)
		}
		cancel, ok := final[1].AsBoolean()
		if !ok {
			return false, fmt.Errorf("event subscription %s cancellation argument must remain boolean", handler.subscription.Name)
		}
		return cancel, nil
	}
	return false, nil
}

// documentEventSubscriptionHandler is the document analogue of
// catalogEventSubscriptionHandler. Movements are not threaded through as a
// separate argument: the wrapped Source object already exposes "Движения"
// as an ordinary property, exactly as a real subscription handler would
// read it via Источник.Движения.
type documentEventSubscriptionHandler struct {
	mu           sync.Mutex
	runtime      *Runtime
	context      *vm.Context
	definition   DocumentDefinition
	subscription EventSubscriptionDefinition
	module       string
}

func (handler *documentEventSubscriptionHandler) HandleDocumentEvent(ctx context.Context, event DocumentEvent, record *DocumentRecord) (bool, error) {
	if string(event) != handler.subscription.Event {
		return false, nil
	}
	if record == nil || record.Reference.DocumentID != handler.definition.ID {
		return false, fmt.Errorf("event subscription %s received an incompatible record", handler.subscription.Name)
	}
	if !handler.context.HasRoutine(handler.module, handler.subscription.Procedure) {
		return false, fmt.Errorf("event subscription %s references unknown procedure %s.%s", handler.subscription.Name, handler.module, handler.subscription.Procedure)
	}
	guarded, err := enterBSLEvent(ctx, handler)
	if err != nil {
		return false, fmt.Errorf("event subscription %s: %w", handler.subscription.Name, err)
	}
	handler.mu.Lock()
	defer handler.mu.Unlock()
	value, err := handler.runtime.wrapDocumentRecord(handler.definition, record)
	if err != nil {
		return false, err
	}
	_, _, arguments := documentEventRoutine(ctx, event, handler.definition)
	_, final, err := handler.context.CallContextMutable(guarded, handler.module+"."+handler.subscription.Procedure, append([]bytecode.Value{value}, arguments...)...)
	if err != nil {
		return false, err
	}
	object := valueRuntimeDocumentObject(value)
	object.mu.Lock()
	if err := handler.runtime.syncDocumentTables(object); err != nil {
		object.mu.Unlock()
		return false, err
	}
	updated := cloneDocumentRecord(object.record)
	object.mu.Unlock()
	*record = *updated
	var cancelled bool
	if event == DocumentEventFillCheck || event == DocumentEventBefore || event == DocumentEventOnWrite || event == DocumentEventBeforeDelete ||
		event == DocumentEventPosting || event == DocumentEventUndoPosting {
		if len(final) < 2 {
			return false, fmt.Errorf("event subscription %s did not return its cancellation argument", handler.subscription.Name)
		}
		cancel, ok := final[1].AsBoolean()
		if !ok {
			return false, fmt.Errorf("event subscription %s cancellation argument must remain boolean", handler.subscription.Name)
		}
		cancelled = cancel
	}
	if event == DocumentEventPosting && !cancelled {
		if err := handler.runtime.writeDocumentMovements(guarded, object.movements); err != nil {
			return false, err
		}
	}
	return cancelled, nil
}

type informationRegisterEventSubscriptionHandler struct {
	mu           sync.Mutex
	runtime      *Runtime
	context      *vm.Context
	definition   InformationRegisterDefinition
	subscription EventSubscriptionDefinition
	module       string
}

func (handler *informationRegisterEventSubscriptionHandler) HandleInformationRegisterEvent(ctx context.Context, event InformationRegisterEvent, set *InformationRegisterRecordSet, replace bool) (bool, error) {
	if string(event) != handler.subscription.Event {
		return false, nil
	}
	if set == nil || set.RegisterID != handler.definition.ID {
		return false, fmt.Errorf("event subscription %s received an incompatible record set", handler.subscription.Name)
	}
	if !handler.context.HasRoutine(handler.module, handler.subscription.Procedure) {
		return false, fmt.Errorf("event subscription %s references unknown procedure %s.%s", handler.subscription.Name, handler.module, handler.subscription.Procedure)
	}
	guarded, err := enterBSLEvent(ctx, handler)
	if err != nil {
		return false, fmt.Errorf("event subscription %s: %w", handler.subscription.Name, err)
	}
	handler.mu.Lock()
	defer handler.mu.Unlock()
	value, err := handler.runtime.wrapInformationRegisterRecordSet(handler.definition, set)
	if err != nil {
		return false, err
	}
	_, _, arguments := informationRegisterEventRoutine(event, replace)
	_, final, err := handler.context.CallContextMutable(guarded, handler.module+"."+handler.subscription.Procedure, append([]bytecode.Value{value}, arguments...)...)
	if err != nil {
		return false, err
	}
	object := valueRuntimeInformationRegisterRecordSet(value)
	object.mu.RLock()
	updated := cloneInformationRegisterRecordSet(object.set)
	object.mu.RUnlock()
	*set = *updated
	if event == InformationRegisterEventBeforeWrite || event == InformationRegisterEventOnWrite {
		if len(final) < 3 {
			return false, fmt.Errorf("event subscription %s did not return its arguments", handler.subscription.Name)
		}
		cancel, ok := final[1].AsBoolean()
		if !ok {
			return false, fmt.Errorf("event subscription %s cancellation argument must remain boolean", handler.subscription.Name)
		}
		if replacement, ok := final[2].AsBoolean(); !ok || replacement != replace {
			return false, fmt.Errorf("event subscription %s replacement argument must remain boolean", handler.subscription.Name)
		}
		return cancel, nil
	}
	return false, nil
}

type accumulationRegisterEventSubscriptionHandler struct {
	mu           sync.Mutex
	runtime      *Runtime
	context      *vm.Context
	definition   AccumulationRegisterDefinition
	subscription EventSubscriptionDefinition
	module       string
}

func (handler *accumulationRegisterEventSubscriptionHandler) HandleAccumulationRegisterEvent(ctx context.Context, event AccumulationRegisterEvent, set *AccumulationRegisterRecordSet, replace bool) (bool, error) {
	if string(event) != handler.subscription.Event {
		return false, nil
	}
	if set == nil || set.RegisterID != handler.definition.ID {
		return false, fmt.Errorf("event subscription %s received an incompatible record set", handler.subscription.Name)
	}
	if !handler.context.HasRoutine(handler.module, handler.subscription.Procedure) {
		return false, fmt.Errorf("event subscription %s references unknown procedure %s.%s", handler.subscription.Name, handler.module, handler.subscription.Procedure)
	}
	guarded, err := enterBSLEvent(ctx, handler)
	if err != nil {
		return false, fmt.Errorf("event subscription %s: %w", handler.subscription.Name, err)
	}
	handler.mu.Lock()
	defer handler.mu.Unlock()
	value, err := bytecode.Object(&accumulationRegisterRecordSetObject{definition: handler.definition, set: cloneAccumulationRegisterRecordSet(set), runtime: handler.runtime})
	if err != nil {
		return false, err
	}
	_, _, arguments := accumulationRegisterEventRoutine(event, replace)
	_, final, err := handler.context.CallContextMutable(guarded, handler.module+"."+handler.subscription.Procedure, append([]bytecode.Value{value}, arguments...)...)
	if err != nil {
		return false, err
	}
	opaque, _ := value.AsRuntimeObject()
	object := opaque.(*accumulationRegisterRecordSetObject)
	object.mu.RLock()
	updated := cloneAccumulationRegisterRecordSet(object.set)
	object.mu.RUnlock()
	*set = *updated
	if event == AccumulationRegisterEventBeforeWrite || event == AccumulationRegisterEventOnWrite {
		if len(final) < 3 {
			return false, fmt.Errorf("event subscription %s did not return its arguments", handler.subscription.Name)
		}
		cancel, ok := final[1].AsBoolean()
		if !ok {
			return false, fmt.Errorf("event subscription %s cancellation argument must remain boolean", handler.subscription.Name)
		}
		if replacement, ok := final[2].AsBoolean(); !ok || replacement != replace {
			return false, fmt.Errorf("event subscription %s replacement argument must remain boolean", handler.subscription.Name)
		}
		return cancel, nil
	}
	return false, nil
}
