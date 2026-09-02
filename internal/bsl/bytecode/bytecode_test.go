package bytecode

import (
	"strings"
	"testing"
)

func TestProgramValidationAndLookup(t *testing.T) {
	t.Parallel()

	program := Program{Version: Version, Functions: []Function{{
		Name: "Сумма", Arity: 2, MaxStack: 2,
		Code: []Instruction{
			{Opcode: OpLoadLocal, Operand: 0},
			{Opcode: OpLoadLocal, Operand: 1},
			{Opcode: OpAdd},
			{Opcode: OpReturn},
		},
	}}}
	if err := program.Validate(); err != nil {
		t.Fatal(err)
	}
	if function, ok := program.Lookup("сУмМа"); !ok || function.Name != "Сумма" {
		t.Fatalf("Lookup() = %+v, %v", function, ok)
	}
}

func TestProgramRejectsStackUnderflow(t *testing.T) {
	t.Parallel()

	program := Program{Version: Version, Functions: []Function{{
		Name: "Broken", MaxStack: 0,
		Code: []Instruction{{Opcode: OpAdd}, {Opcode: OpReturn}},
	}}}
	if err := program.Validate(); err == nil {
		t.Fatal("Validate() accepted malformed bytecode")
	}
}

func TestProgramRejectsMalformedBytecode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		program Program
		message string
	}{
		{name: "version", program: Program{Version: Version + 1}, message: "unsupported bytecode version"},
		{name: "empty name", program: programWith(Function{Code: returningConstant(), MaxStack: 1}), message: "empty name"},
		{name: "duplicate name", program: Program{Version: Version, Functions: []Function{
			{Name: "Test", Code: returningConstant(), Constants: []Value{Undefined()}, MaxStack: 1},
			{Name: "tEsT", Code: returningConstant(), Constants: []Value{Undefined()}, MaxStack: 1},
		}}, message: "duplicate function"},
		{name: "missing return", program: programWith(Function{Name: "Test", Code: []Instruction{{Opcode: OpConstant}}, Constants: []Value{Undefined()}, MaxStack: 1}), message: "end with return"},
		{name: "constant", program: programWith(Function{Name: "Test", Code: returningConstant(), MaxStack: 1}), message: "references constant"},
		{name: "local", program: programWith(Function{Name: "Test", Code: []Instruction{{Opcode: OpLoadLocal}, {Opcode: OpReturn}}, MaxStack: 1}), message: "references local"},
		{name: "unknown opcode", program: programWith(Function{Name: "Test", Code: []Instruction{{Opcode: Opcode(255)}, {Opcode: OpReturn}}, MaxStack: 1}), message: "unknown opcode"},
		{name: "wrong max stack", program: programWith(Function{Name: "Test", Code: returningConstant(), Constants: []Value{Undefined()}, MaxStack: 2}), message: "max stack"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.program.Validate()
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Validate() error = %v, want substring %q", err, test.message)
			}
		})
	}
}

func TestValueKindsAndFormatting(t *testing.T) {
	t.Parallel()

	if value := Undefined(); value.Kind() != UndefinedKind || value.String() != "Undefined" {
		t.Fatalf("Undefined() = %#v", value)
	}
	if value := Number(42.5); value.Kind() != NumberKind || value.String() != "42.5" {
		t.Fatalf("Number() = %#v", value)
	}
	if value := String("MetaLab"); value.Kind() != StringKind || value.String() != "MetaLab" {
		t.Fatalf("String() = %#v", value)
	}
	if value := (Value{kind: ValueKind(255)}).String(); value != "value(255)" {
		t.Fatalf("unknown Value.String() = %q", value)
	}
}

func TestOpcodeStringHandlesUnknownValue(t *testing.T) {
	t.Parallel()

	if got := Opcode(255).String(); got != "opcode(255)" {
		t.Fatalf("Opcode.String() = %q", got)
	}
}

func programWith(function Function) Program {
	return Program{Version: Version, Functions: []Function{function}}
}

func returningConstant() []Instruction {
	return []Instruction{{Opcode: OpConstant}, {Opcode: OpReturn}}
}
