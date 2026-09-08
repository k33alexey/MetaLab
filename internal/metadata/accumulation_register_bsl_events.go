package metadata

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
)

func AccumulationRegisterRecordSetModuleName(definition AccumulationRegisterDefinition) string {
	return "AccumulationRegisterRecordSet" + strings.ReplaceAll(definition.ID.String(), "-", "")
}

func CompileAccumulationRegisterRecordSetModule(definition AccumulationRegisterDefinition, filename, source string) (*bytecode.Program, []syntax.Diagnostic) {
	return compiler.CompileModules([]compiler.ModuleSource{{
		Name: AccumulationRegisterRecordSetModuleName(definition), Filename: filename, Source: source,
		PredefinedVariables: []string{catalogThisObjectRU, catalogThisObjectEN},
	}})
}

type AccumulationRegisterBSLEvents struct {
	mu         sync.Mutex
	runtime    *Runtime
	context    *vm.Context
	definition AccumulationRegisterDefinition
	module     string
}

func NewAccumulationRegisterBSLEvents(runtime *Runtime, context *vm.Context, definition AccumulationRegisterDefinition) (*AccumulationRegisterBSLEvents, error) {
	if runtime == nil || context == nil || definition.ID.IsZero() {
		return nil, fmt.Errorf("accumulation register BSL events require runtime, context and definition")
	}
	return &AccumulationRegisterBSLEvents{runtime: runtime, context: context, definition: cloneAccumulationRegisterDefinition(definition), module: AccumulationRegisterRecordSetModuleName(definition)}, nil
}

func (handler *AccumulationRegisterBSLEvents) HandleAccumulationRegisterEvent(ctx context.Context, event AccumulationRegisterEvent, set *AccumulationRegisterRecordSet, replace bool) (bool, error) {
	if set == nil || set.RegisterID != handler.definition.ID {
		return false, fmt.Errorf("accumulation register event received an incompatible record set")
	}
	russian, english, arguments := accumulationRegisterEventRoutine(event, replace)
	routine := ""
	if handler.context.HasRoutine(handler.module, russian) {
		routine = russian
	} else if handler.context.HasRoutine(handler.module, english) {
		routine = english
	} else {
		return false, nil
	}
	guarded, err := enterBSLEvent(ctx, handler)
	if err != nil {
		return false, fmt.Errorf("accumulation register event %s: %w", event, err)
	}
	handler.mu.Lock()
	defer handler.mu.Unlock()
	value, err := bytecode.Object(&accumulationRegisterRecordSetObject{definition: handler.definition, set: cloneAccumulationRegisterRecordSet(set), runtime: handler.runtime})
	if err != nil {
		return false, err
	}
	if err := handler.context.SetModuleVariable(handler.module, catalogThisObjectRU, value); err != nil {
		return false, err
	}
	if err := handler.context.SetModuleVariable(handler.module, catalogThisObjectEN, value); err != nil {
		_ = handler.context.SetModuleVariable(handler.module, catalogThisObjectRU, bytecode.Undefined())
		return false, err
	}
	defer func() {
		_ = handler.context.SetModuleVariable(handler.module, catalogThisObjectRU, bytecode.Undefined())
		_ = handler.context.SetModuleVariable(handler.module, catalogThisObjectEN, bytecode.Undefined())
	}()
	_, final, err := handler.context.CallContextMutable(guarded, handler.module+"."+routine, arguments...)
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
		if len(final) != 2 {
			return false, fmt.Errorf("accumulation register event %s did not return its arguments", event)
		}
		cancel, ok := final[0].AsBoolean()
		if !ok {
			return false, fmt.Errorf("accumulation register event %s cancellation argument must remain boolean", event)
		}
		if replacement, ok := final[1].AsBoolean(); !ok || replacement != replace {
			return false, fmt.Errorf("accumulation register event %s replacement argument must remain boolean", event)
		}
		return cancel, nil
	}
	return false, nil
}

func accumulationRegisterEventRoutine(event AccumulationRegisterEvent, replace bool) (string, string, []bytecode.Value) {
	switch event {
	case AccumulationRegisterEventBeforeWrite:
		return "ПередЗаписью", "BeforeWrite", []bytecode.Value{bytecode.Boolean(false), bytecode.Boolean(replace)}
	case AccumulationRegisterEventOnWrite:
		return "ПриЗаписи", "OnWrite", []bytecode.Value{bytecode.Boolean(false), bytecode.Boolean(replace)}
	case AccumulationRegisterEventAfterWrite:
		return "ПослеЗаписи", "AfterWrite", []bytecode.Value{bytecode.Boolean(replace)}
	default:
		return "", "", nil
	}
}
