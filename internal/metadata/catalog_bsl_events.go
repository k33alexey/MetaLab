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

const (
	catalogThisObjectRU = "ЭтотОбъект"
	catalogThisObjectEN = "ThisObject"
)

// CatalogObjectModuleName returns the deterministic internal name of an object module.
func CatalogObjectModuleName(definition CatalogDefinition) string {
	return "CatalogObject" + strings.ReplaceAll(definition.ID.String(), "-", "")
}

// CompileCatalogObjectModule compiles a catalog object module with its predefined object context.
func CompileCatalogObjectModule(definition CatalogDefinition, filename, source string) (*bytecode.Program, []syntax.Diagnostic) {
	return compiler.CompileModules([]compiler.ModuleSource{{
		Name: CatalogObjectModuleName(definition), Filename: filename, Source: source,
		PredefinedVariables: []string{catalogThisObjectRU, catalogThisObjectEN},
	}})
}

// CatalogBSLEvents invokes optional predefined procedures in one catalog object module.
type CatalogBSLEvents struct {
	mu         sync.Mutex
	runtime    *Runtime
	context    *vm.Context
	definition CatalogDefinition
	module     string
}

func NewCatalogBSLEvents(runtime *Runtime, context *vm.Context, definition CatalogDefinition) (*CatalogBSLEvents, error) {
	if runtime == nil || context == nil || definition.ID.IsZero() {
		return nil, fmt.Errorf("catalog BSL events require runtime, context and catalog definition")
	}
	return &CatalogBSLEvents{runtime: runtime, context: context, definition: cloneCatalogDefinition(definition), module: CatalogObjectModuleName(definition)}, nil
}

func (handler *CatalogBSLEvents) HandleCatalogEvent(ctx context.Context, event CatalogEvent, record *CatalogRecord) (bool, error) {
	if record == nil || record.Reference.CatalogID != handler.definition.ID {
		return false, fmt.Errorf("catalog event received an incompatible record")
	}
	russian, english, arguments := catalogEventRoutine(event, handler.definition)
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
		return false, fmt.Errorf("catalog event %s: %w", event, err)
	}
	handler.mu.Lock()
	defer handler.mu.Unlock()
	value, err := handler.runtime.wrapCatalogRecord(handler.definition, record)
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
		if len(final) == 0 {
			return false, fmt.Errorf("catalog event %s did not return its cancellation argument", event)
		}
		cancel, ok := final[0].AsBoolean()
		if !ok {
			return false, fmt.Errorf("catalog event %s cancellation argument must remain boolean", event)
		}
		return cancel, nil
	}
	return false, nil
}

func catalogEventRoutine(event CatalogEvent, definition CatalogDefinition) (string, string, []bytecode.Value) {
	switch event {
	case CatalogEventFill:
		return "ОбработкаЗаполнения", "FillProcessing", []bytecode.Value{bytecode.Undefined(), bytecode.Boolean(true)}
	case CatalogEventFillCheck:
		names := make([]bytecode.Value, 0)
		for _, attribute := range definition.Attributes {
			if attribute.Required {
				names = append(names, bytecode.String(attribute.Name))
			}
		}
		return "ОбработкаПроверкиЗаполнения", "FillCheckProcessing", []bytecode.Value{bytecode.Boolean(false), bytecode.Array(names...)}
	case CatalogEventBefore:
		return "ПередЗаписью", "BeforeWrite", []bytecode.Value{bytecode.Boolean(false)}
	case CatalogEventOnWrite:
		return "ПриЗаписи", "OnWrite", []bytecode.Value{bytecode.Boolean(false)}
	case CatalogEventAfter:
		return "ПослеЗаписи", "AfterWrite", nil
	case CatalogEventBeforeDelete:
		return "ПередУдалением", "BeforeDelete", []bytecode.Value{bytecode.Boolean(false)}
	default:
		return "", "", nil
	}
}

func valueRuntimeCatalogObject(value bytecode.Value) *catalogObject {
	object, _ := value.AsRuntimeObject()
	result, _ := object.(*catalogObject)
	return result
}
