package bytecode

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestProgramValidationAndLookup(t *testing.T) {
	t.Parallel()

	program := Program{Version: Version, Functions: []Function{{
		Name: "Сумма", Arity: 2, LocalCount: 2, MaxStack: 2,
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
		{name: "locals smaller than arity", program: programWith(Function{Name: "Test", Arity: 1, Code: returningConstant(), Constants: []Value{Undefined()}, MaxStack: 1}), message: "local count"},
		{name: "jump", program: programWith(Function{Name: "Test", Code: []Instruction{{Opcode: OpJump, Operand: 2}, {Opcode: OpReturn}}}), message: "jumps outside code"},
		{name: "unknown opcode", program: programWith(Function{Name: "Test", Code: []Instruction{{Opcode: Opcode(255)}, {Opcode: OpReturn}}, MaxStack: 1}), message: "unknown opcode"},
		{name: "wrong max stack", program: programWith(Function{Name: "Test", Code: returningConstant(), Constants: []Value{Undefined()}, MaxStack: 2}), message: "max stack"},
		{name: "exception range", program: programWith(Function{Name: "Test", Code: returningConstant(), Constants: []Value{Undefined()}, MaxStack: 1, Exceptions: []ExceptionHandler{{Start: 1, End: 0, Target: 1}}}), message: "protected range"},
		{name: "exception target", program: programWith(Function{Name: "Test", Code: returningConstant(), Constants: []Value{Undefined()}, MaxStack: 1, Exceptions: []ExceptionHandler{{Start: 0, End: 1, Target: 2}}}), message: "invalid target"},
		{name: "reraise handler", program: programWith(Function{Name: "Test", Code: []Instruction{{Opcode: OpReraise}, {Opcode: OpConstant}, {Opcode: OpReturn}}, Constants: []Value{Undefined()}, MaxStack: 1}), message: "references exception handler"},
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

	if NumberKind.String() != "number" || ValueKind(255).String() != "value_kind(255)" {
		t.Fatal("ValueKind.String() returned an unstable name")
	}
	if value := Undefined(); value.Kind() != UndefinedKind || value.String() != "Undefined" {
		t.Fatalf("Undefined() = %#v", value)
	}
	if value := Number(42.5); value.Kind() != NumberKind || value.String() != "42.5" {
		t.Fatalf("Number() = %#v", value)
	}
	if _, err := NumberFromFloat64(math.Inf(1)); err == nil {
		t.Fatal("NumberFromFloat64() accepted infinity")
	}
	if value := String("MetaLab"); value.Kind() != StringKind || value.String() != "MetaLab" {
		t.Fatalf("String() = %#v", value)
	}
	if value := Boolean(true); value.Kind() != BooleanKind || value.String() != "True" {
		t.Fatalf("Boolean(true) = %#v", value)
	} else if boolean, ok := value.AsBoolean(); !ok || !boolean {
		t.Fatalf("AsBoolean() = %v, %v", boolean, ok)
	}
	array := Array(Number(1), String("two"))
	if length, ok := array.ArrayLength(); !ok || length != 2 || array.String() != "Array(2)" {
		t.Fatalf("Array() = %#v", array)
	}
	if element, ok := array.ArrayElement(1); !ok || element.String() != "two" {
		t.Fatalf("ArrayElement(1) = %v, %v", element, ok)
	}
	if _, ok := array.ArrayElement(2); ok {
		t.Fatal("ArrayElement() accepted an out-of-range index")
	}
	if value := Null(); value.Kind() != NullKind || value.String() != "Null" {
		t.Fatalf("Null() = %#v", value)
	}
	date, err := ParseDate("20240229010203")
	if err != nil || date.Kind() != DateKind || date.String() != "2024-02-29 01:02:03.0000" {
		t.Fatalf("ParseDate() = %v, %v", date, err)
	}
	if decoded, ok := date.AsDate(); !ok || !decoded.Equal(time.Date(2024, 2, 29, 1, 2, 3, 0, time.UTC)) {
		t.Fatalf("AsDate() = %v, %v", decoded, ok)
	}
	fraction, err := ParseNumber("0.00005")
	if err != nil {
		t.Fatal(err)
	}
	shifted, err := AddDateSeconds(date, fraction)
	if err != nil {
		t.Fatal(err)
	}
	if decoded, ok := shifted.AsDate(); !ok || decoded.Nanosecond() != 100_000 {
		t.Fatalf("fractional date arithmetic = %v, %v", decoded, ok)
	}
	for _, invalid := range []string{"", "20230229", "20240101240000", "40000101"} {
		if _, err := ParseDate(invalid); err == nil {
			t.Fatalf("ParseDate(%q) succeeded", invalid)
		}
	}
	if value := (Value{kind: ValueKind(255)}).String(); value != "value(255)" {
		t.Fatalf("unknown Value.String() = %q", value)
	}
}

func TestProgramValidatesConditionalControlFlow(t *testing.T) {
	t.Parallel()

	program := Program{Version: Version, Functions: []Function{{
		Name: "Choose", MaxStack: 1,
		Constants: []Value{Boolean(true), Number(1), Number(2)},
		Code: []Instruction{
			{Opcode: OpConstant},
			{Opcode: OpJumpIfFalse, Operand: 4},
			{Opcode: OpConstant, Operand: 1},
			{Opcode: OpReturn},
			{Opcode: OpConstant, Operand: 2},
			{Opcode: OpReturn},
		},
	}}}
	if err := program.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestProgramRejectsConflictingBranchStackDepths(t *testing.T) {
	t.Parallel()

	program := Program{Version: Version, Functions: []Function{{
		Name: "Broken", MaxStack: 2,
		Constants: []Value{Boolean(true), Number(1)},
		Code: []Instruction{
			{Opcode: OpConstant},
			{Opcode: OpJumpIfFalse, Operand: 4},
			{Opcode: OpConstant, Operand: 1},
			{Opcode: OpJump, Operand: 5},
			{Opcode: OpJump, Operand: 5},
			{Opcode: OpConstant, Operand: 1},
			{Opcode: OpReturn},
		},
	}}}
	if err := program.Validate(); err == nil || !strings.Contains(err.Error(), "conflicting stack depths") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestOpcodeStringHandlesUnknownValue(t *testing.T) {
	t.Parallel()

	for opcode := OpConstant; opcode <= OpErrorDescription; opcode++ {
		if strings.HasPrefix(opcode.String(), "opcode(") {
			t.Fatalf("opcode %d has no stable name", opcode)
		}
	}
	if got := Opcode(255).String(); got != "opcode(255)" {
		t.Fatalf("Opcode.String() = %q", got)
	}
}

func TestProgramRejectsCrossModulePrivateCall(t *testing.T) {
	t.Parallel()

	program := Program{Version: Version, Modules: []Module{{Name: "Private"}, {Name: "Caller"}}, Functions: []Function{
		{Name: "Hidden", Module: 0, MaxStack: 1, Constants: []Value{Undefined()}, Code: returningConstant()},
		{Name: "Run", Module: 1, MaxStack: 1, CallSites: []CallSite{{Target: 0}}, Code: []Instruction{{Opcode: OpCall}, {Opcode: OpReturn}}},
	}}
	if err := program.Validate(); err == nil || !strings.Contains(err.Error(), "non-exported routine") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestProgramRejectsCrossModulePrivateVariable(t *testing.T) {
	t.Parallel()

	program := Program{Version: Version, Modules: []Module{
		{Name: "Private", Variables: []ModuleVariable{{Name: "Hidden"}}},
		{Name: "Caller"},
	}, Functions: []Function{{
		Name: "Run", Module: 1, MaxStack: 1,
		ModuleVars: []VariableReference{{Kind: ModuleReference, Module: 0}},
		Code:       []Instruction{{Opcode: OpLoadModule}, {Opcode: OpReturn}},
	}}}
	if err := program.Validate(); err == nil || !strings.Contains(err.Error(), "non-exported variable") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func programWith(function Function) Program {
	return Program{Version: Version, Functions: []Function{function}}
}

func returningConstant() []Instruction {
	return []Instruction{{Opcode: OpConstant}, {Opcode: OpReturn}}
}
