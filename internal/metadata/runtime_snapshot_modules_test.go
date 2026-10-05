package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProjectModulesFromDemoProject(t *testing.T) {
	t.Parallel()
	root := demoProjectRoot(t)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	modules, err := LoadProjectModules(root, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 2 {
		t.Fatalf("expected 2 document object modules in the demo project, got %d: %+v", len(modules), modules)
	}
	byName := make(map[string]RuntimeModule, len(modules))
	for _, module := range modules {
		byName[module.Name] = module
	}
	receipt, ok := byName["МодульОбъектаДокумента.ПоступлениеТоваров"]
	if !ok {
		t.Fatalf("missing receipt document module, got names: %v", moduleNames(modules))
	}
	if !strings.Contains(receipt.Source, "ВидДвиженияНакопления.Приход") {
		t.Fatalf("receipt module source missing expected posting logic: %s", receipt.Source)
	}
	sale, ok := byName["МодульОбъектаДокумента.ПродажаТоваров"]
	if !ok {
		t.Fatalf("missing sale document module, got names: %v", moduleNames(modules))
	}
	if !strings.Contains(sale.Source, "ВидДвиженияНакопления.Расход") {
		t.Fatalf("sale module source missing expected posting logic: %s", sale.Source)
	}
	for _, module := range modules {
		if len(module.PredefinedVariables) != 4 {
			t.Fatalf("document object module %s should predefine ЭтотОбъект/ThisObject/Движения/Movements, got %v", module.Name, module.PredefinedVariables)
		}
	}
	// Building the snapshot and compiling it must succeed end to end.
	snapshot, err := RuntimeSnapshot{Format: CurrentFormat}.WithModules(modules)
	if err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, err := snapshot.CompileModules(); err != nil {
		t.Fatalf("compile demo project BSL: %v (%v)", err, diagnostics)
	}
}

func moduleNames(modules []RuntimeModule) []string {
	names := make([]string, len(modules))
	for index, module := range modules {
		names[index] = module.Name
	}
	return names
}

// demoProjectRoot locates examples/sales-and-warehouse relative to this
// package, walking up from the working directory the same way `go test`
// runs it (package directory), independent of the caller's cwd.
func demoProjectRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "examples", "sales-and-warehouse"))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// A project publishes whatever number and size of modules it has: erp has
// 10 962 modules and 425 MB of them, against the 10 000 modules and 16 MB in
// all that publication used to take. Ten thousand and one modules, one of them
// of 17 MB, must all be read.
func TestProjectModulesHaveNoCeilingOnCountOrSize(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	directory := filepath.Join(root, "modules")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	const count = 10_001
	large := strings.Repeat("// Строка комментария модуля.\n", 600_000)
	for index := range count {
		content := "Процедура П()\nКонецПроцедуры\n"
		if index == 0 {
			content = large + content
		}
		if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("m%05d.bsl", index)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	modules, err := LoadProjectModules(root, catalog)
	if err != nil {
		t.Fatalf("%d modules with %d bytes in one were refused: %v", count, len(large), err)
	}
	read := 0
	for _, module := range modules {
		if strings.HasPrefix(module.Filename, "modules/") {
			read++
		}
	}
	if read != count {
		t.Fatalf("%d modules read, want %d", read, count)
	}
}
