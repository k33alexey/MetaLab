package metadata

import (
	"fmt"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
)

// CompileModules compiles every BSL module carried by the snapshot into one
// program. ML Service calls this against PostgreSQL-stored source only - it
// never reads project files from disk.
func (snapshot RuntimeSnapshot) CompileModules() (*bytecode.Program, []syntax.Diagnostic, error) {
	if len(snapshot.Modules) == 0 {
		return nil, nil, fmt.Errorf("runtime snapshot has no BSL modules to compile")
	}
	sources := make([]compiler.ModuleSource, len(snapshot.Modules))
	for index, module := range snapshot.Modules {
		sources[index] = compiler.ModuleSource{
			Name: module.Name, Filename: module.Filename, Source: module.Source,
			PredefinedVariables: append([]string(nil), module.PredefinedVariables...),
			DefaultContext:      module.DefaultContext,
		}
	}
	program, diagnostics := compiler.CompileModules(sources)
	if len(diagnostics) != 0 {
		return nil, diagnostics, fmt.Errorf("compile BSL: %s", diagnostics[0].Error())
	}
	return program, nil, nil
}
