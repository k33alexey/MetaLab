package metadata

import (
	"fmt"
	"path/filepath"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// ConfigureProgramEvents connects object and record-set modules from one
// compiled application program to metadata lifecycle events.
func (runtime *Runtime) ConfigureProgramEvents(program *bytecode.Program) error {
	return runtime.ConfigureProgramEventsWithObserver(program, nil)
}

// ConfigureProgramEventsWithObserver connects event contexts to the same
// observer as the test that triggered them.
func (runtime *Runtime) ConfigureProgramEventsWithObserver(program *bytecode.Program, observer vm.InstructionObserver) error {
	if runtime == nil || runtime.catalog == nil {
		return fmt.Errorf("metadata runtime is not configured")
	}
	machine, err := vm.New(program)
	if err != nil {
		return err
	}
	moduleNames := make(map[string]string, len(program.Modules))
	for _, module := range program.Modules {
		moduleNames[filepath.ToSlash(module.Source)] = module.Name
	}
	moduleName := func(id *uuid.UUID) (string, bool) {
		if id == nil {
			return "", false
		}
		name, ok := moduleNames["modules/"+id.String()+".bsl"]
		return name, ok
	}
	subscriptionModule := func(subscription EventSubscriptionDefinition) (string, bool) {
		common, ok := runtime.catalog.CommonModuleByID(subscription.Module)
		if !ok {
			return "", false
		}
		return moduleName(&common.Module)
	}
	for _, definition := range runtime.catalog.Catalogs {
		var handlers catalogEventHandlers
		if name, ok := moduleName(definition.ObjectModule); ok {
			handler, err := NewCatalogBSLEvents(runtime, machine.NewContextWithMetadataAndObserver(runtime, observer), definition)
			if err != nil {
				return err
			}
			handler.module = name
			handlers = append(handlers, handler)
		}
		for _, subscription := range runtime.catalog.eventSubscriptionsForObject(definition.ID) {
			name, ok := subscriptionModule(subscription)
			if !ok {
				continue
			}
			handlers = append(handlers, &catalogEventSubscriptionHandler{
				runtime: runtime, context: machine.NewContextWithMetadataAndObserver(runtime, observer),
				definition: definition, subscription: subscription, module: name,
			})
		}
		if len(handlers) == 0 {
			continue
		}
		if err := runtime.SetCatalogEventHandler(definition.Name, handlers); err != nil {
			return err
		}
	}
	for _, definition := range runtime.catalog.Documents {
		var handlers documentEventHandlers
		if name, ok := moduleName(definition.ObjectModule); ok {
			handler, err := NewDocumentBSLEvents(runtime, machine.NewContextWithMetadataAndObserver(runtime, observer), definition)
			if err != nil {
				return err
			}
			handler.module = name
			handlers = append(handlers, handler)
		}
		for _, subscription := range runtime.catalog.eventSubscriptionsForObject(definition.ID) {
			name, ok := subscriptionModule(subscription)
			if !ok {
				continue
			}
			handlers = append(handlers, &documentEventSubscriptionHandler{
				runtime: runtime, context: machine.NewContextWithMetadataAndObserver(runtime, observer),
				definition: definition, subscription: subscription, module: name,
			})
		}
		if len(handlers) == 0 {
			continue
		}
		if err := runtime.SetDocumentEventHandler(definition.Name, handlers); err != nil {
			return err
		}
	}
	for _, definition := range runtime.catalog.InformationRegisters {
		var handlers informationRegisterEventHandlers
		if name, ok := moduleName(definition.RecordSetModule); ok {
			handler, err := NewInformationRegisterBSLEvents(runtime, machine.NewContextWithMetadataAndObserver(runtime, observer), definition)
			if err != nil {
				return err
			}
			handler.module = name
			handlers = append(handlers, handler)
		}
		for _, subscription := range runtime.catalog.eventSubscriptionsForObject(definition.ID) {
			name, ok := subscriptionModule(subscription)
			if !ok {
				continue
			}
			handlers = append(handlers, &informationRegisterEventSubscriptionHandler{
				runtime: runtime, context: machine.NewContextWithMetadataAndObserver(runtime, observer),
				definition: definition, subscription: subscription, module: name,
			})
		}
		if len(handlers) == 0 {
			continue
		}
		if err := runtime.SetInformationRegisterEventHandler(definition.Name, handlers); err != nil {
			return err
		}
	}
	for _, definition := range runtime.catalog.AccumulationRegisters {
		var handlers accumulationRegisterEventHandlers
		if name, ok := moduleName(definition.RecordSetModule); ok {
			handler, err := NewAccumulationRegisterBSLEvents(runtime, machine.NewContextWithMetadataAndObserver(runtime, observer), definition)
			if err != nil {
				return err
			}
			handler.module = name
			handlers = append(handlers, handler)
		}
		for _, subscription := range runtime.catalog.eventSubscriptionsForObject(definition.ID) {
			name, ok := subscriptionModule(subscription)
			if !ok {
				continue
			}
			handlers = append(handlers, &accumulationRegisterEventSubscriptionHandler{
				runtime: runtime, context: machine.NewContextWithMetadataAndObserver(runtime, observer),
				definition: definition, subscription: subscription, module: name,
			})
		}
		if len(handlers) == 0 {
			continue
		}
		if err := runtime.SetAccumulationRegisterEventHandler(definition.Name, handlers); err != nil {
			return err
		}
	}
	return nil
}
