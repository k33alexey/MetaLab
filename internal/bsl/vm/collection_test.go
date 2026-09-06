package vm

import (
	"errors"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
)

func TestArrayCollectionsExecuteInRussianAndEnglish(t *testing.T) {
	t.Parallel()

	sources := []string{
		`Функция Рассчитать()
Элементы = Новый Массив;
Элементы.Добавить(10);
Элементы.Добавить(20);
Элементы.Вставить(1, 15);
Элементы[0] = 5;
Итог = 0;
Для Каждого Элемент Из Элементы Цикл Итог = Итог + Элемент; КонецЦикла;
Возврат Итог + Элементы.Количество();
КонецФункции`,
		`Function Run()
Items = New Array;
Items.Add(10);
Items.Add(20);
Items.Insert(1, 15);
Items[0] = 5;
Total = 0;
For Each Item In Items Do Total = Total + Item; EndDo;
Return Total + Items.Count();
EndFunction`,
	}
	for index, source := range sources {
		program, diagnostics := compiler.CompileSource("collections.bsl", source)
		if len(diagnostics) != 0 {
			t.Fatalf("source %d diagnostics = %v", index, diagnostics)
		}
		machine, err := New(program)
		if err != nil {
			t.Fatal(err)
		}
		name := "Рассчитать"
		if index == 1 {
			name = "Run"
		}
		result, err := machine.Call(name)
		if err != nil || result.String() != "43" {
			t.Fatalf("source %d result = %v, %v", index, result, err)
		}
	}
}

func TestStructureAndMapPropertiesIndexesAndIteration(t *testing.T) {
	t.Parallel()

	machine := compileMachine(t, `Function Run()
Data = New Structure("Name, Amount", "MetaLab", 40);
Data.Amount = Data.Amount + 2;
HasName = Data.Property("Name", FoundName);
Values = New Map;
Values.Insert("entry", Data);
Total = 0;
For Each Pair In Values Do
    If Pair.Key = "entry" Then Total = Total + Pair.Value.Amount; EndIf;
EndDo;
Return "" + HasName + ":" + FoundName + ":" + Values["entry"].Name + ":" + Total;
EndFunction`)
	result, err := machine.Call("Run")
	if err != nil || result.String() != "True:MetaLab:MetaLab:42" {
		t.Fatalf("Run() = %v, %v", result, err)
	}
}

func TestValueListItemsAndDynamicConstructor(t *testing.T) {
	t.Parallel()

	machine := compileMachine(t, `Function Run()
Items = New("ValueList");
Item = Items.Add(42, "Answer");
Item.Check = True;
Found = Items.FindByValue(42);
Total = 0;
For Each Current In Items Do Total = Total + Current.Value; EndDo;
Return Found.Presentation + ":" + Found.Check + ":" + Items.UnloadValues()[0] + ":" + Total;
EndFunction`)
	result, err := machine.Call("Run")
	if err != nil || result.String() != "Answer:True:42:42" {
		t.Fatalf("Run() = %v, %v", result, err)
	}
}

func TestValueTableColumnsRowsAndTotals(t *testing.T) {
	t.Parallel()

	machine := compileMachine(t, `Function Run()
Table = New ValueTable;
Table.Columns.Add("Name");
Table.Columns.Add("Amount");
First = Table.Add();
First.Name = "A";
First["Amount"] = 20;
Second = Table.Add();
Second.Name = "B";
Second.Amount = 22;
Found = Table.Find("B", "Name");
Iterated = 0;
For Each Current In Table Do Iterated = Iterated + Current.Amount; EndDo;
Return Found.Name + ":" + Table.Total("Amount") + ":" + Table.Count() + ":" + Iterated;
EndFunction`)
	result, err := machine.Call("Run")
	if err != nil || result.String() != "B:42:2:42" {
		t.Fatalf("Run() = %v, %v", result, err)
	}
}

func TestExtendedValueListAndValueTableOperations(t *testing.T) {
	t.Parallel()

	list := compileMachine(t, `Function Run()
Items = New ValueList;
Items.Add(1, "One");
Second = Items.Add(2, "Two");
Copy = Items.Copy();
Items.Move(Second, -1);
Items.FillChecks(True);
Return "" + Items[0].Value + ":" + Items[1].Check + ":" + Copy.Count();
EndFunction`)
	if result, err := list.Call("Run"); err != nil || result.String() != "2:True:2" {
		t.Fatalf("ValueList Run() = %v, %v", result, err)
	}

	table := compileMachine(t, `Function Run()
Table = New ValueTable;
Table.Columns.Add("Name");
Table.Columns.Add("Amount");
First = Table.Add(); First.Name = "A"; First.Amount = 20;
Second = Table.Add(); Second.Name = "B"; Second.Amount = 22;
Filter = New Structure("Name", "B");
Rows = Table.FindRows(Filter);
Copy = Table.Copy(Rows, "Name, Amount");
ColumnsOnly = Table.CopyColumns("Amount");
Amounts = Table.UnloadColumn("Amount");
Amounts[0] = 1;
Amounts[1] = 2;
Table.LoadColumn(Amounts, "Amount");
Return "" + Rows.Count() + ":" + Copy.Total("Amount") + ":" + ColumnsOnly.Columns.Count() + ":" + Table.Total("Amount");
EndFunction`)
	if result, err := table.Call("Run"); err != nil || result.String() != "1:22:1:3" {
		t.Fatalf("ValueTable Run() = %v, %v", result, err)
	}
}

func TestCollectionBytecodeRoundTrip(t *testing.T) {
	t.Parallel()

	program, diagnostics := compiler.CompileSource("collections.bsl", `Function Run()
Items = New Array;
Items.Add(42);
Return Items[0];
EndFunction`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	encoded, err := bytecode.MarshalBinary(program)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := bytecode.UnmarshalBinary(encoded)
	if err != nil {
		t.Fatal(err)
	}
	machine, err := New(decoded)
	if err != nil {
		t.Fatal(err)
	}
	result, err := machine.Call("Run")
	if err != nil || result.String() != "42" {
		t.Fatalf("Run() = %v, %v", result, err)
	}
}

func TestCollectionGrowthHonorsMemoryLimitBeforeMutation(t *testing.T) {
	t.Parallel()

	limits := DefaultLimits()
	limits.MaxMemoryBytes = 16 << 10
	machine := compileMachineWithLimits(t, `Procedure Fill(Items)
For Index = 1 To 100 Do Items.Add(Index); EndDo;
EndProcedure`, limits)
	items := bytecode.Array()
	if _, err := machine.Call("Fill", items); !errors.Is(err, ErrMemoryLimit) {
		t.Fatalf("Fill() error = %v", err)
	}
	count, err := bytecode.CollectionMethod(items, "Count", nil)
	if err != nil || count.String() != "43" {
		t.Fatalf("atomic collection count = %v, %v; want 43", count, err)
	}
}
