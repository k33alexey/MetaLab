package metadata

import (
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestConfigureProgramEventsUsesCompiledModuleIdentity(t *testing.T) {
	t.Parallel()
	id := uuid.MustNew()
	definition := CatalogDefinition{ID: id, Name: "Товары", ObjectModule: &id}
	catalog := &Catalog{
		Catalogs:      []CatalogDefinition{definition},
		catalogByName: map[string]int{"товары": 0}, catalogByID: map[uuid.UUID]int{id: 0},
	}
	runtime := &Runtime{
		catalog: catalog, events: make(map[uuid.UUID]CatalogEventHandler),
		documentEvents: make(map[uuid.UUID]DocumentEventHandler), informationRegisterEvents: make(map[uuid.UUID]InformationRegisterEventHandler),
		accumulationRegisterEvents: make(map[uuid.UUID]AccumulationRegisterEventHandler),
	}
	moduleName := "МодульОбъектаСправочника.Товары"
	program, diagnostics := compiler.CompileModules([]compiler.ModuleSource{{
		Name: moduleName, Filename: "modules/" + id.String() + ".bsl",
		Source:              "Процедура ПередЗаписью(Отказ)\nКонецПроцедуры",
		PredefinedVariables: []string{"ЭтотОбъект", "ThisObject"},
	}})
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	if err := runtime.ConfigureProgramEvents(program); err != nil {
		t.Fatal(err)
	}
	handler, ok := runtime.catalogEventHandler(id).(*CatalogBSLEvents)
	if !ok || handler.module != moduleName {
		t.Fatalf("handler=%+v", handler)
	}
}
