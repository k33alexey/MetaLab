package vm

import (
	"context"
	"fmt"
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

type metadataRuntimeStub struct{ value bytecode.Value }

func (runtime *metadataRuntimeStub) GetConstant(_ context.Context, _ string) (bytecode.Value, error) {
	return runtime.value, nil
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
