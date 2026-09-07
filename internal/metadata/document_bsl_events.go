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

func DocumentObjectModuleName(definition DocumentDefinition) string {
	return "DocumentObject" + strings.ReplaceAll(definition.ID.String(), "-", "")
}

func CompileDocumentObjectModule(definition DocumentDefinition, filename, source string) (*bytecode.Program, []syntax.Diagnostic) {
	return compiler.CompileModules([]compiler.ModuleSource{{
		Name: DocumentObjectModuleName(definition), Filename: filename, Source: source,
		PredefinedVariables: []string{catalogThisObjectRU, catalogThisObjectEN},
	}})
}

type DocumentBSLEvents struct {
	mu         sync.Mutex
	runtime    *Runtime
	context    *vm.Context
	definition DocumentDefinition
	module     string
}

func NewDocumentBSLEvents(runtime *Runtime, context *vm.Context, definition DocumentDefinition) (*DocumentBSLEvents, error) {
	if runtime == nil || context == nil || definition.ID.IsZero() {
		return nil, fmt.Errorf("document BSL events require runtime, context and document definition")
	}
	return &DocumentBSLEvents{
		runtime: runtime, context: context, definition: cloneDocumentDefinition(definition), module: DocumentObjectModuleName(definition),
	}, nil
}

func (handler *DocumentBSLEvents) HandleDocumentEvent(ctx context.Context, event DocumentEvent, record *DocumentRecord) (bool, error) {
	if record == nil || record.Reference.DocumentID != handler.definition.ID {
		return false, fmt.Errorf("document event received an incompatible record")
	}
	russian, english, arguments := documentEventRoutine(event, handler.definition)
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
		return false, fmt.Errorf("document event %s: %w", event, err)
	}
	handler.mu.Lock()
	defer handler.mu.Unlock()
	value, err := handler.runtime.wrapDocumentRecord(handler.definition, record)
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
	object := valueRuntimeDocumentObject(value)
	object.mu.Lock()
	if err := handler.runtime.syncDocumentTables(object); err != nil {
		object.mu.Unlock()
		return false, err
	}
	updated := cloneDocumentRecord(object.record)
	object.mu.Unlock()
	*record = *updated
	if event == DocumentEventFillCheck || event == DocumentEventBefore || event == DocumentEventOnWrite || event == DocumentEventBeforeDelete {
		if len(final) == 0 {
			return false, fmt.Errorf("document event %s did not return its cancellation argument", event)
		}
		cancel, ok := final[0].AsBoolean()
		if !ok {
			return false, fmt.Errorf("document event %s cancellation argument must remain boolean", event)
		}
		return cancel, nil
	}
	return false, nil
}

func documentEventRoutine(event DocumentEvent, definition DocumentDefinition) (string, string, []bytecode.Value) {
	switch event {
	case DocumentEventFill:
		return "ОбработкаЗаполнения", "FillProcessing", []bytecode.Value{bytecode.Undefined(), bytecode.Boolean(true)}
	case DocumentEventFillCheck:
		names := make([]bytecode.Value, 0)
		for _, attribute := range definition.Attributes {
			if attribute.Required {
				names = append(names, bytecode.String(attribute.Name))
			}
		}
		return "ОбработкаПроверкиЗаполнения", "FillCheckProcessing", []bytecode.Value{bytecode.Boolean(false), bytecode.Array(names...)}
	case DocumentEventBefore:
		return "ПередЗаписью", "BeforeWrite", []bytecode.Value{
			bytecode.Boolean(false), bytecode.String("Write"), bytecode.String("DoNotPost"),
		}
	case DocumentEventOnWrite:
		return "ПриЗаписи", "OnWrite", []bytecode.Value{bytecode.Boolean(false)}
	case DocumentEventAfter:
		return "ПослеЗаписи", "AfterWrite", nil
	case DocumentEventBeforeDelete:
		return "ПередУдалением", "BeforeDelete", []bytecode.Value{bytecode.Boolean(false)}
	default:
		return "", "", nil
	}
}

func valueRuntimeDocumentObject(value bytecode.Value) *documentObject {
	object, _ := value.AsRuntimeObject()
	result, _ := object.(*documentObject)
	return result
}
