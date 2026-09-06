package vm

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
)

func TestMetadataContextsExecuteInRussianAndEnglish(t *testing.T) {
	t.Parallel()
	sources := []string{
		`&НаСервере
Функция Проверить()
Константы.Курс.Установить(42);
Статус = Перечисления.Статусы.Активен;
Типы = ОпределяемыеТипы.Код;
Возврат "" + Константы.Курс.Получить() + ":" + Статус + ":" + Типы[0].Тип;
КонецФункции`,
		`&AtServer
Function Run()
Constants.Rate.Set(42);
Status = Enums.Statuses.Active;
Types = DefinedTypes.Code;
Return "" + Constants.Rate.Get() + ":" + Status + ":" + Types[0].Kind;
EndFunction`,
	}
	for index, source := range sources {
		program, diagnostics := compiler.CompileSource("metadata.bsl", source)
		if len(diagnostics) != 0 {
			t.Fatalf("source %d diagnostics = %v", index, diagnostics)
		}
		machine, err := New(program)
		if err != nil {
			t.Fatal(err)
		}
		runtime := &metadataRuntimeStub{}
		name := "Проверить"
		if index == 1 {
			name = "Run"
		}
		result, err := machine.NewContextWithMetadata(runtime).Call(name)
		if err != nil || result.String() != "42:active:string" || runtime.value.String() != "42" {
			t.Fatalf("source %d result=%v stored=%v error=%v", index, result, runtime.value, err)
		}
	}
}

func TestMetadataContextRequiresRuntime(t *testing.T) {
	t.Parallel()
	program, diagnostics := compiler.CompileSource("metadata.bsl", `&AtServer
Function Run()
Return Constants.Rate.Get();
EndFunction`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := New(program)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Call("Run"); err == nil {
		t.Fatal("metadata call executed without a runtime")
	}
}

func TestCompilerRejectsMetadataOutsideServerOnlyRoutine(t *testing.T) {
	t.Parallel()
	contexts := []string{"", "&AtClient\n", "&AtClientAtServer\n"}
	for _, directive := range contexts {
		_, diagnostics := compiler.CompileSource("metadata.bsl", directive+`Function Run()
Return Enums.Statuses.Active;
EndFunction`)
		if len(diagnostics) != 1 || diagnostics[0].Code != "BSL3042" {
			t.Fatalf("directive %q diagnostics = %v", directive, diagnostics)
		}
	}
}

func TestCatalogManagerAndRuntimeObjectDispatch(t *testing.T) {
	t.Parallel()
	program, diagnostics := compiler.CompileSource("catalog.bsl", `&AtServer
Function Run()
    Item = Catalogs.Products.CreateItem();
    Item.Code = "P001";
    Item.Description = "Product";
    Item.Write();
    Ref = Catalogs.Products.FindByCode("P001");
    Return Item.Code + ":" + Ref.UUID;
EndFunction`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := New(program)
	if err != nil {
		t.Fatal(err)
	}
	runtime := &catalogRuntimeStub{}
	result, err := machine.NewContextWithMetadata(runtime).Call("Run")
	if err != nil || result.String() != "P001:record-id" || !runtime.written {
		t.Fatalf("result=%v written=%v error=%v", result, runtime.written, err)
	}
}

type metadataRuntimeStub struct{ value bytecode.Value }

func (runtime *metadataRuntimeStub) GetConstant(_ context.Context, _ string) (bytecode.Value, error) {
	return runtime.value, nil
}

type catalogRuntimeStub struct {
	metadataRuntimeStub
	written bool
}

type runtimeObjectStub struct {
	runtime    *catalogRuntimeStub
	properties map[string]bytecode.Value
}

func (*runtimeObjectStub) RuntimeTypeName() string { return "CatalogObject.Products" }
func (object *runtimeObjectStub) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*runtimeObjectStub)
	return ok && candidate == object
}
func (*runtimeObjectStub) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	return 256, limit >= 256
}

func (runtime *catalogRuntimeStub) CreateCatalogObject(context.Context, string) (bytecode.Value, error) {
	return bytecode.Object(&runtimeObjectStub{runtime: runtime, properties: map[string]bytecode.Value{}})
}
func (runtime *catalogRuntimeStub) GetCatalogObject(context.Context, string, bytecode.Value) (bytecode.Value, error) {
	return runtime.CreateCatalogObject(context.Background(), "")
}
func (runtime *catalogRuntimeStub) FindCatalogByCode(context.Context, string, bytecode.Value) (bytecode.Value, error) {
	return bytecode.Object(&runtimeObjectStub{runtime: runtime, properties: map[string]bytecode.Value{"uuid": bytecode.String("record-id")}})
}
func (runtime *catalogRuntimeStub) GetCatalogReference(context.Context, string, bytecode.Value) (bytecode.Value, error) {
	return runtime.FindCatalogByCode(context.Background(), "", bytecode.Undefined())
}
func (*catalogRuntimeStub) GetObjectProperty(_ context.Context, object bytecode.RuntimeObject, name string) (bytecode.Value, error) {
	stub := object.(*runtimeObjectStub)
	value, ok := stub.properties[strings.ToLower(name)]
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("unknown property %s", name)
	}
	return value, nil
}
func (*catalogRuntimeStub) SetObjectProperty(_ context.Context, object bytecode.RuntimeObject, name string, value bytecode.Value) error {
	object.(*runtimeObjectStub).properties[strings.ToLower(name)] = value
	return nil
}
func (runtime *catalogRuntimeStub) CallObjectMethod(_ context.Context, _ bytecode.RuntimeObject, name string, arguments []bytecode.Value) (bytecode.Value, error) {
	if !strings.EqualFold(name, "Write") || len(arguments) != 0 {
		return bytecode.Undefined(), fmt.Errorf("unknown method %s", name)
	}
	runtime.written = true
	return bytecode.Undefined(), nil
}
func (runtime *metadataRuntimeStub) SetConstant(_ context.Context, _ string, value bytecode.Value) error {
	runtime.value = value
	return nil
}
func (*metadataRuntimeStub) GetEnumerationValue(_ context.Context, _, _ string) (bytecode.Value, error) {
	return bytecode.String("active"), nil
}
func (*metadataRuntimeStub) GetDefinedType(_ context.Context, _ string) (bytecode.Value, error) {
	descriptor, err := bytecode.ConstructCollection("Structure", []bytecode.Value{
		bytecode.String("Kind,Тип"), bytecode.String("string"), bytecode.String("string"),
	})
	if err != nil {
		return bytecode.Undefined(), fmt.Errorf("create type descriptor: %w", err)
	}
	return bytecode.Array(descriptor), nil
}
