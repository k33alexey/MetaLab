package testsuite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func commonModuleTestProject(t *testing.T, flags string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "project")
	manifest := project.Project{
		Format: project.CurrentFormat, ID: uuid.MustNew(), Name: "CommonModuleTest", Title: "Common Module Test", DefaultLanguage: "ru",
		Languages: []project.Language{{Name: "Русский", Title: "Русский", Code: "ru"}},
	}
	if err := project.Initialize(root, manifest); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "metadata", "common-modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	moduleSourceID := uuid.MustNew()
	if err := os.WriteFile(filepath.Join(root, "modules", moduleSourceID.String()+".bsl"),
		[]byte("Функция Удвоить(Знач Число) Экспорт\nВозврат Число * 2;\nКонецФункции\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commonModuleID := uuid.MustNew()
	if err := os.WriteFile(filepath.Join(root, "metadata", "common-modules", commonModuleID.String()+".yaml"),
		[]byte("format: 1\nid: "+commonModuleID.String()+"\nname: ОбщегоНазначения\ntitle: {ru: Общего назначения}\n"+flags+"\nmodule: "+moduleSourceID.String()+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCompileProjectResolvesCommonModuleCallsByDeclaredName(t *testing.T) {
	root := commonModuleTestProject(t, "server: true")
	callerID := uuid.MustNew()
	if err := os.WriteFile(filepath.Join(root, "modules", callerID.String()+".bsl"),
		[]byte("&НаСервере\nФункция Запустить() Экспорт\nВозврат ОбщегоНазначения.Удвоить(21);\nКонецФункции\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	program, err := CompileProject(root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, module := range program.Modules {
		if module.Name == "ОбщегоНазначения" {
			found = true
		}
	}
	if !found {
		t.Fatalf("common module was not compiled under its declared name: %+v", program.Modules)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	callerName := "Модуль" + strings.ReplaceAll(callerID.String(), "-", "")
	result, err := machine.Call(callerName + ".Запустить")
	if err != nil || result.String() != "42" {
		t.Fatalf("common module call result = %v, error = %v", result, err)
	}
}

func TestCompileProjectAllowsClientCallIntoServerOnlyCommonModule(t *testing.T) {
	// A client routine calling a server-only common module routine compiles:
	// the VM already bridges this automatically ("вызов сервера"), so no
	// extra ServerCall enforcement is added on top of it.
	root := commonModuleTestProject(t, "server: true")
	callerID := uuid.MustNew()
	if err := os.WriteFile(filepath.Join(root, "modules", callerID.String()+".bsl"),
		[]byte("&НаКлиенте\nФункция Запустить() Экспорт\nВозврат ОбщегоНазначения.Удвоить(21);\nКонецФункции\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileProject(root); err != nil {
		t.Fatalf("client call into a server-only common module was unexpectedly rejected: %v", err)
	}
}

func TestCompileProjectRejectsServerCallIntoClientOnlyCommonModule(t *testing.T) {
	// The common module here declares client: true (no server), so its
	// undirected routine defaults to client-only: a server caller must be
	// rejected, proving DefaultContext actually took effect rather than
	// silently leaving the routine usable everywhere (ContextShared).
	root := commonModuleTestProject(t, "client: true")
	callerID := uuid.MustNew()
	if err := os.WriteFile(filepath.Join(root, "modules", callerID.String()+".bsl"),
		[]byte("&НаСервере\nФункция Запустить() Экспорт\nВозврат ОбщегоНазначения.Удвоить(21);\nКонецФункции\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileProject(root); err == nil {
		t.Fatal("server caller was allowed to call a client-only common module routine")
	}
}
