package metadata

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func commonModuleFixture() CommonModuleDefinition {
	return CommonModuleDefinition{
		Format: CurrentFormat, ID: uuid.MustNew(), Name: "ОбщегоНазначения", Title: LocalizedText{"ru": "Общего назначения"},
		Module: uuid.MustNew(), Server: true,
	}
}

func TestDecodeCommonModuleStrictAndBounded(t *testing.T) {
	t.Parallel()
	module := commonModuleFixture()
	var encoded bytes.Buffer
	if err := Encode(&encoded, module); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeCommonModule("common-module.yaml", bytes.NewReader(encoded.Bytes()), metadataConfiguration())
	if err != nil || decoded.ID != module.ID || decoded.Module != module.Module {
		t.Fatalf("decode common module = %+v, %v", decoded, err)
	}
	tests := map[string]func(*CommonModuleDefinition){
		"format":        func(m *CommonModuleDefinition) { m.Format++ },
		"zero identity": func(m *CommonModuleDefinition) { m.ID = uuid.UUID{} },
		"name":          func(m *CommonModuleDefinition) { m.Name = "Invalid Name" },
		"language":      func(m *CommonModuleDefinition) { m.Title = LocalizedText{"de": "Allgemein"} },
		"zero module":   func(m *CommonModuleDefinition) { m.Module = uuid.UUID{} },
		"no context at all": func(m *CommonModuleDefinition) {
			m.Server, m.Client = false, false
		},
		"server_call without server": func(m *CommonModuleDefinition) {
			m.Server, m.ServerCall = false, true
			m.Client = true
		},
		"privileged without server": func(m *CommonModuleDefinition) {
			m.Server, m.Privileged = false, true
			m.Client = true
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			mutated := module
			mutate(&mutated)
			if err := ValidateCommonModule("common-module.yaml", mutated, metadataConfiguration()); err == nil {
				t.Fatalf("%s: accepted invalid common module", name)
			}
		})
	}
	if _, err := DecodeCommonModule("common-module.yaml", strings.NewReader(encoded.String()+"unknown: true\n"), metadataConfiguration()); err == nil {
		t.Fatal("accepted unknown field")
	}
}

func TestCatalogCommonModuleLookupReturnsIsolatedCopies(t *testing.T) {
	t.Parallel()
	module := commonModuleFixture()
	catalog, err := NewCatalogSnapshotWithCommonModules(metadataConfiguration(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, []CommonModuleDefinition{module})
	if err != nil {
		t.Fatal(err)
	}
	byName, ok := catalog.CommonModule("ОбщегоНазначения")
	if !ok || byName.ID != module.ID {
		t.Fatalf("common module by name = %+v, %v", byName, ok)
	}
	byID, ok := catalog.CommonModuleByID(module.ID)
	if !ok || byID.Module != module.Module {
		t.Fatalf("common module by id = %+v, %v", byID, ok)
	}
	byModule, ok := catalog.CommonModuleByModuleID(module.Module)
	if !ok || byModule.ID != module.ID {
		t.Fatalf("common module by module id = %+v, %v", byModule, ok)
	}
	byID.Title["ru"] = "Изменено"
	again, _ := catalog.CommonModuleByID(module.ID)
	if again.Title["ru"] != "Общего назначения" {
		t.Fatal("common module lookup exposed mutable metadata")
	}
}

func TestCommonModuleDefaultContext(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		client, server bool
		want           syntax.ExecutionContext
	}{
		{"client only", true, false, syntax.ContextClient},
		{"server only", false, true, syntax.ContextServer},
		{"client and server", true, true, syntax.ContextClientServer},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definition := CommonModuleDefinition{Client: test.client, Server: test.server}
			if got := definition.DefaultContext(); got != test.want {
				t.Fatalf("DefaultContext() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestLoadValidatesCommonModuleSourceAndUniqueness(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	if err := os.MkdirAll(filepath.Join(root, "metadata", string(CommonModuleKind)), 0o755); err != nil {
		t.Fatal(err)
	}
	moduleSourceID := uuid.MustNew()
	if err := os.WriteFile(filepath.Join(root, "modules", moduleSourceID.String()+".bsl"),
		[]byte("Функция Значение() Экспорт\nВозврат 1;\nКонецФункции\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commonModuleID := uuid.MustNew()
	writeMetadata(t, root, CommonModuleKind, commonModuleID.String(), "format: 1\nid: "+commonModuleID.String()+
		"\nname: ОбщегоНазначения\ntitle: {ru: Общего назначения}\nserver: true\nmodule: "+moduleSourceID.String()+"\n")
	if _, err := load(root, true); err != nil {
		t.Fatalf("valid common module rejected: %v", err)
	}

	if err := os.Remove(filepath.Join(root, "modules", moduleSourceID.String()+".bsl")); err != nil {
		t.Fatal(err)
	}
	if _, err := load(root, true); err == nil || !strings.Contains(err.Error(), "missing or unsafe") {
		t.Fatalf("common module with a missing source file was accepted: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "modules", moduleSourceID.String()+".bsl"),
		[]byte("Функция Значение() Экспорт\nВозврат 1;\nКонецФункции\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	duplicateID := uuid.MustNew()
	writeMetadata(t, root, CommonModuleKind, duplicateID.String(), "format: 1\nid: "+duplicateID.String()+
		"\nname: Дубликат\ntitle: {ru: Дубликат}\nserver: true\nmodule: "+moduleSourceID.String()+"\n")
	if _, err := load(root, true); err == nil || !strings.Contains(err.Error(), "share the same module source") {
		t.Fatalf("two common modules sharing one module source were accepted: %v", err)
	}
}

// A module of any one context is saved by the designer: the external
// connection alone, or the ordinary application's client alone (checked by the
// owner, 01.10.2026). Defect caught: "client or server" refused both, and with
// them every configuration that has one. The ordinary client's flag is carried
// and runs nothing: beside the server it leaves a server module, and alone it
// gives a client module nobody calls, as the root's ordinary module is.
func TestCommonModuleOfAnyOneContextIsSaved(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		set  func(*CommonModuleDefinition)
		want syntax.ExecutionContext
	}{
		"только внешнее соединение": {func(m *CommonModuleDefinition) { m.ExternalConnection = true }, syntax.ContextServer},
		"только обычное приложение": {func(m *CommonModuleDefinition) { m.ClientOrdinaryApplication = true }, syntax.ContextClient},
		"сервер и обычное приложение": {func(m *CommonModuleDefinition) {
			m.Server, m.ClientOrdinaryApplication = true, true
		}, syntax.ContextServer},
	} {
		module := commonModuleFixture()
		module.Server = false
		test.set(&module)
		var encoded bytes.Buffer
		if err := Encode(&encoded, module); err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeCommonModule("common-module.yaml", bytes.NewReader(encoded.Bytes()), metadataConfiguration())
		if err != nil {
			t.Errorf("%s: refused: %v", name, err)
			continue
		}
		if decoded.ClientOrdinaryApplication != module.ClientOrdinaryApplication {
			t.Errorf("%s: the ordinary client's flag was lost", name)
		}
		if got := decoded.DefaultContext(); got != test.want {
			t.Errorf("%s: context = %v, want %v", name, got, test.want)
		}
	}
}
