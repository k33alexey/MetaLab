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
	for _, definition := range runtime.catalog.Catalogs {
		name, ok := moduleName(definition.ObjectModule)
		if !ok {
			continue
		}
		handler, err := NewCatalogBSLEvents(runtime, machine.NewContextWithMetadata(runtime), definition)
		if err != nil {
			return err
		}
		handler.module = name
		if err := runtime.SetCatalogEventHandler(definition.Name, handler); err != nil {
			return err
		}
	}
	for _, definition := range runtime.catalog.Documents {
		name, ok := moduleName(definition.ObjectModule)
		if !ok {
			continue
		}
		handler, err := NewDocumentBSLEvents(runtime, machine.NewContextWithMetadata(runtime), definition)
		if err != nil {
			return err
		}
		handler.module = name
		if err := runtime.SetDocumentEventHandler(definition.Name, handler); err != nil {
			return err
		}
	}
	for _, definition := range runtime.catalog.InformationRegisters {
		name, ok := moduleName(definition.RecordSetModule)
		if !ok {
			continue
		}
		handler, err := NewInformationRegisterBSLEvents(runtime, machine.NewContextWithMetadata(runtime), definition)
		if err != nil {
			return err
		}
		handler.module = name
		if err := runtime.SetInformationRegisterEventHandler(definition.Name, handler); err != nil {
			return err
		}
	}
	for _, definition := range runtime.catalog.AccumulationRegisters {
		name, ok := moduleName(definition.RecordSetModule)
		if !ok {
			continue
		}
		handler, err := NewAccumulationRegisterBSLEvents(runtime, machine.NewContextWithMetadata(runtime), definition)
		if err != nil {
			return err
		}
		handler.module = name
		if err := runtime.SetAccumulationRegisterEventHandler(definition.Name, handler); err != nil {
			return err
		}
	}
	return nil
}
