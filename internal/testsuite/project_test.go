package testsuite

import (
	"testing"

	"github.com/k33alexey/MetaLab/internal/gitclient"
)

func TestCompileProjectUsesCanonicalModuleNames(t *testing.T) {
	program, err := CompileProject("../../examples/sales-and-warehouse")
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Modules) == 0 {
		t.Fatal("demo project compiled without modules")
	}
	found := false
	for _, module := range program.Modules {
		if module.Name == "МодульОбъектаДокумента.ПоступлениеТоваров" {
			found = true
		}
	}
	if !found {
		t.Fatalf("canonical object module name was not compiled: %+v", program.Modules)
	}
}

func TestConflictPath(t *testing.T) {
	status := gitclient.Status{Entries: []gitclient.StatusEntry{{Path: "modules/a.bsl"}, {Path: "modules/b.bsl", Conflicted: true}}}
	if got := conflictPath(status); got != "modules/b.bsl" {
		t.Fatalf("conflictPath=%q", got)
	}
}
