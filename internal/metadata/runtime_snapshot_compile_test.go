package metadata

import "testing"

func TestRuntimeSnapshotCompileModules(t *testing.T) {
	t.Parallel()
	snapshot := RuntimeSnapshot{Format: CurrentFormat}
	var err error
	snapshot, err = snapshot.WithModules([]RuntimeModule{{
		Name: "ОбщийМодуль", Filename: "modules/common.bsl", Source: "&НаСервере\nФункция Ответ() Экспорт\n\tВозврат 42;\nКонецФункции",
	}})
	if err != nil {
		t.Fatal(err)
	}
	program, diagnostics, err := snapshot.CompileModules()
	if err != nil || len(diagnostics) != 0 || program == nil {
		t.Fatalf("expected a compiled program, got program=%v diagnostics=%v err=%v", program, diagnostics, err)
	}
}

func TestRuntimeSnapshotCompileModulesReportsDiagnostics(t *testing.T) {
	t.Parallel()
	snapshot := RuntimeSnapshot{Format: CurrentFormat}
	var err error
	snapshot, err = snapshot.WithModules([]RuntimeModule{{
		Name: "ОбщийМодуль", Filename: "modules/common.bsl", Source: "Функция Сломано(",
	}})
	if err != nil {
		t.Fatal(err)
	}
	_, diagnostics, err := snapshot.CompileModules()
	if err == nil || len(diagnostics) == 0 {
		t.Fatalf("expected compile diagnostics for invalid BSL, got err=%v diagnostics=%v", err, diagnostics)
	}
}

func TestRuntimeSnapshotCompileModulesRequiresModules(t *testing.T) {
	t.Parallel()
	snapshot := RuntimeSnapshot{Format: CurrentFormat}
	if _, _, err := snapshot.CompileModules(); err == nil {
		t.Fatal("expected an error when the snapshot has no modules")
	}
}
