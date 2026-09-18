package metadata

import (
	"fmt"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
)

// WireBSLEvents connects every document, catalog, information register and
// accumulation register defined in catalog to program's compiled routines,
// so Записать/Провести/ОтменитьПроведение/УстановитьПометкуУдаления actually
// execute BSL business logic (ПередЗаписью/ОбработкаПроведения/...) instead
// of only performing their built-in Go-level effects. program is expected to
// come from RuntimeSnapshot.CompileModules, whose module names already match
// the *ModuleName functions in document_bsl_events.go/catalog_bsl_events.go/
// information_register_bsl_events.go/accumulation_register_bsl_events.go - a
// module without a matching routine is simply never invoked, so objects with
// no BSL module at all are unaffected.
func WireBSLEvents(runtime *Runtime, program *bytecode.Program, catalog *Catalog) error {
	if runtime == nil || program == nil || catalog == nil {
		return fmt.Errorf("wiring BSL events requires a runtime, a compiled program and a metadata catalog")
	}
	machine, err := vm.New(program)
	if err != nil {
		return fmt.Errorf("start BSL machine: %w", err)
	}
	eventContext := machine.NewContextWithMetadata(runtime)
	// The session module gets a BSL context of its own, never the one the
	// object events run on: the handler is called from INSIDE another running
	// routine (application code reads a session parameter mid-write), and a
	// context cannot be executing two calls at once.
	sessionModule, err := NewSessionBSLEvents(machine.NewContextWithMetadata(runtime))
	if err != nil {
		return err
	}
	runtime.SetSessionModuleHandler(sessionModule)
	for _, definition := range catalog.Documents {
		bridge, err := NewDocumentBSLEvents(runtime, eventContext, definition)
		if err != nil {
			return err
		}
		if err := runtime.SetDocumentEventHandler(definition.Name, bridge); err != nil {
			return err
		}
	}
	for _, definition := range catalog.Catalogs {
		bridge, err := NewCatalogBSLEvents(runtime, eventContext, definition)
		if err != nil {
			return err
		}
		if err := runtime.SetCatalogEventHandler(definition.Name, bridge); err != nil {
			return err
		}
	}
	for _, definition := range catalog.InformationRegisters {
		bridge, err := NewInformationRegisterBSLEvents(runtime, eventContext, definition)
		if err != nil {
			return err
		}
		if err := runtime.SetInformationRegisterEventHandler(definition.Name, bridge); err != nil {
			return err
		}
	}
	for _, definition := range catalog.AccumulationRegisters {
		bridge, err := NewAccumulationRegisterBSLEvents(runtime, eventContext, definition)
		if err != nil {
			return err
		}
		if err := runtime.SetAccumulationRegisterEventHandler(definition.Name, bridge); err != nil {
			return err
		}
	}
	return nil
}
