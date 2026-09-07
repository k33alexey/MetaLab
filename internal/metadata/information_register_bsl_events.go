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

func InformationRegisterRecordSetModuleName(definition InformationRegisterDefinition) string {
	return "InformationRegisterRecordSet" + strings.ReplaceAll(definition.ID.String(), "-", "")
}

func CompileInformationRegisterRecordSetModule(definition InformationRegisterDefinition, filename, source string) (*bytecode.Program, []syntax.Diagnostic) {
	return compiler.CompileModules([]compiler.ModuleSource{{
		Name: InformationRegisterRecordSetModuleName(definition), Filename: filename, Source: source,
		PredefinedVariables: []string{catalogThisObjectRU, catalogThisObjectEN},
	}})
}

type InformationRegisterBSLEvents struct {
	mu         sync.Mutex
	runtime    *Runtime
	context    *vm.Context
	definition InformationRegisterDefinition
	module     string
}

func NewInformationRegisterBSLEvents(runtime *Runtime, context *vm.Context, definition InformationRegisterDefinition) (*InformationRegisterBSLEvents, error) {
	if runtime == nil || context == nil || definition.ID.IsZero() {
		return nil, fmt.Errorf("information register BSL events require runtime, context and definition")
	}
	return &InformationRegisterBSLEvents{
		runtime: runtime, context: context, definition: cloneInformationRegisterDefinition(definition),
		module: InformationRegisterRecordSetModuleName(definition),
	}, nil
}

func (handler *InformationRegisterBSLEvents) HandleInformationRegisterEvent(ctx context.Context, event InformationRegisterEvent, set *InformationRegisterRecordSet, replace bool) (bool, error) {
	if set == nil || set.RegisterID != handler.definition.ID {
		return false, fmt.Errorf("information register event received an incompatible record set")
	}
	russian, english, arguments := informationRegisterEventRoutine(event, replace)
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
		return false, fmt.Errorf("information register event %s: %w", event, err)
	}
	handler.mu.Lock()
	defer handler.mu.Unlock()
	value, err := handler.runtime.wrapInformationRegisterRecordSet(handler.definition, set)
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
	object := valueRuntimeInformationRegisterRecordSet(value)
	object.mu.RLock()
	updated := cloneInformationRegisterRecordSet(object.set)
	object.mu.RUnlock()
	*set = *updated
	if event == InformationRegisterEventBeforeWrite || event == InformationRegisterEventOnWrite {
		if len(final) != 2 {
			return false, fmt.Errorf("information register event %s did not return its arguments", event)
		}
		cancel, ok := final[0].AsBoolean()
		if !ok {
			return false, fmt.Errorf("information register event %s cancellation argument must remain boolean", event)
		}
		if replacement, ok := final[1].AsBoolean(); !ok || replacement != replace {
			return false, fmt.Errorf("information register event %s replacement argument must remain boolean", event)
		}
		return cancel, nil
	}
	return false, nil
}

func informationRegisterEventRoutine(event InformationRegisterEvent, replace bool) (string, string, []bytecode.Value) {
	switch event {
	case InformationRegisterEventBeforeWrite:
		return "ПередЗаписью", "BeforeWrite", []bytecode.Value{bytecode.Boolean(false), bytecode.Boolean(replace)}
	case InformationRegisterEventOnWrite:
		return "ПриЗаписи", "OnWrite", []bytecode.Value{bytecode.Boolean(false), bytecode.Boolean(replace)}
	case InformationRegisterEventAfterWrite:
		return "ПослеЗаписи", "AfterWrite", []bytecode.Value{bytecode.Boolean(replace)}
	default:
		return "", "", nil
	}
}

func valueRuntimeInformationRegisterRecordSet(value bytecode.Value) *informationRegisterRecordSetObject {
	object, _ := value.AsRuntimeObject()
	result, _ := object.(*informationRegisterRecordSetObject)
	return result
}
