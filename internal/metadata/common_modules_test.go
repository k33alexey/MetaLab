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
		"language":      func(m *CommonModuleDefinition) { m.Title = LocalizedText{"d=e": "Allgemein"} },
		"zero module":   func(m *CommonModuleDefinition) { m.Module = uuid.UUID{} },
		"no context at all": func(m *CommonModuleDefinition) {
			m.Server, m.Client = false, false
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
// connection alone (checked by the owner, 01.10.2026). Defect caught: "client
// or server" refused it, and with it every configuration that has one; a
// module of the external connection compiled as anything but a server module.
func TestCommonModuleOfAnyOneContextIsSaved(t *testing.T) {
	t.Parallel()
	module := commonModuleFixture()
	module.Server, module.ExternalConnection = false, true
	var encoded bytes.Buffer
	if err := Encode(&encoded, module); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeCommonModule("common-module.yaml", bytes.NewReader(encoded.Bytes()), metadataConfiguration())
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if got := decoded.DefaultContext(); got != syntax.ContextServer {
		t.Errorf("context = %v, want the server's", got)
	}
}

// The flags combine as the designer lets them (opis of the common module,
// 14.10.2026): «Клиент» and «Привилегированный» clear each other; «Вызов
// сервера» stays marked when «Сервер» is cleared (four modules of lombard1),
// «Привилегированный» needs «Сервер»; a global
// module has no reuse of return values; the ordinary application's client is
// no flag of ML, and a description that writes it is not read. Every case is
// written here with what the model must say.
//
// Defect caught: a client privileged module or a global module caching its
// values taken, which the designer cannot save; the client modules of
// lombard1 with «Вызов сервера» refused on import; the ordinary client's flag
// carried again, or taken silently and dropped.
func TestCommonModuleFlagsCombineAsTheDesignerLetsThem(t *testing.T) {
	t.Parallel()
	head := "format: 1\nid: " + uuid.MustNew().String() + "\nname: Модуль\ntitle: {ru: Модуль}\nmodule: " + uuid.MustNew().String() + "\n"
	for flags, want := range map[string]string{
		"server: true\nprivileged: true\n":                                  "",
		"client: true\nserver_call: true\n":                                 "",
		"client: true\nprivileged: true\n":                                  "privileged requires server",
		"client: true\nserver: true\n":                                      "",
		"server: true\nglobal: true\nreturn_values_reuse: dont-use\n":       "",
		"server: true\nreturn_values_reuse: during-session\n":               "",
		"client: true\nserver: true\nprivileged: true\n":                    "privileged excludes client",
		"server: true\nglobal: true\nreturn_values_reuse: during-request\n": "return_values_reuse must be dont-use on a global module",
		"server: true\nglobal: true\nreturn_values_reuse: during-session\n": "return_values_reuse must be dont-use on a global module",
		"server: true\nclient_ordinary_application: true\n":                 "field client_ordinary_application not found",
		"client_ordinary_application: true\n":                               "field client_ordinary_application not found",
	} {
		_, err := DecodeCommonModule("common-module.yaml", strings.NewReader(head+flags), metadataConfiguration())
		switch {
		case want == "" && err != nil:
			t.Errorf("%q refused: %v", flags, err)
		case want != "" && (err == nil || !strings.Contains(err.Error(), want)):
			t.Errorf("%q: %v, want %q", flags, err, want)
		}
	}
}
