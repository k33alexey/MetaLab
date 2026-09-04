package bytecode

import (
	"bytes"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
)

func TestBinaryCodecRoundTripIsDeterministic(t *testing.T) {
	t.Parallel()

	date, err := ParseDate("20260904153045")
	if err != nil {
		t.Fatal(err)
	}
	exact, err := ParseNumber("99999999999999999999999999999999999999")
	if err != nil {
		t.Fatal(err)
	}
	program := &Program{Version: Version, Modules: []Module{{
		Name: "Основной", Variables: []ModuleVariable{{Name: "Состояние", Export: true}},
	}}, Functions: []Function{{
		Name: "Расчёт", IsFunction: true, Export: true, Arity: 1, LocalCount: 2, MaxStack: 2,
		Parameters: []Parameter{{HasDefault: true, Default: Number(10)}},
		Constants:  []Value{Undefined(), Number(2.5), String("MetaLab"), Boolean(true), Null(), date, exact},
		ModuleVars: []VariableReference{{Kind: ModuleReference, Variable: 0}},
		Code: []Instruction{
			{Opcode: OpLoadLocal, Span: testSpan(1, 1)},
			{Opcode: OpConstant, Operand: 1, Span: testSpan(2, 8)},
			{Opcode: OpMultiply, Span: testSpan(2, 8)},
			{Opcode: OpReturn, Span: testSpan(2, 1)},
		},
	}}}
	encoded, err := MarshalBinary(program)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := UnmarshalBinary(encoded)
	if err != nil {
		t.Fatal(err)
	}
	reencoded, err := MarshalBinary(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, reencoded) {
		t.Fatal("binary encoding is not deterministic")
	}
	function, ok := decoded.Lookup("расчёт")
	if !ok || !function.IsFunction || !function.Export || function.Arity != 1 || function.LocalCount != 2 {
		t.Fatalf("decoded function = %+v", function)
	}
	if len(decoded.Modules) != 1 || decoded.Modules[0].Name != "Основной" || !decoded.Modules[0].Variables[0].Export {
		t.Fatalf("decoded modules = %+v", decoded.Modules)
	}
	if len(function.Parameters) != 1 || !function.Parameters[0].HasDefault || function.Parameters[0].Default.String() != "10" {
		t.Fatalf("decoded parameters = %+v", function.Parameters)
	}
	if text, ok := function.Constants[2].AsString(); !ok || text != "MetaLab" {
		t.Fatalf("decoded string = %q, %v", text, ok)
	}
	if boolean, ok := function.Constants[3].AsBoolean(); !ok || !boolean {
		t.Fatalf("decoded boolean = %v, %v", boolean, ok)
	}
	if function.Constants[4].Kind() != NullKind || function.Constants[5].String() != date.String() {
		t.Fatalf("decoded null/date = %v, %v", function.Constants[4], function.Constants[5])
	}
	if number, ok := function.Constants[6].NumberText(); !ok || number != exact.String() {
		t.Fatalf("decoded exact number = %q, %v", number, ok)
	}
}

func TestBinaryCodecRejectsMalformedData(t *testing.T) {
	t.Parallel()

	valid, err := MarshalBinary(&Program{Version: Version, Functions: []Function{{
		Name: "Test", MaxStack: 1, Constants: []Value{Undefined()},
		Code: []Instruction{{Opcode: OpConstant}, {Opcode: OpReturn}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string][]byte{
		"empty":          nil,
		"wrong header":   append([]byte("NOPE"), valid[4:]...),
		"truncated":      valid[:len(valid)-1],
		"trailing bytes": append(append([]byte(nil), valid...), 0),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, decodeErr := UnmarshalBinary(data); decodeErr == nil {
				t.Fatal("UnmarshalBinary() accepted malformed data")
			}
		})
	}
}

func TestMarshalBinaryRejectsNilAndInvalidProgram(t *testing.T) {
	t.Parallel()

	if _, err := MarshalBinary(nil); err == nil {
		t.Fatal("MarshalBinary(nil) succeeded")
	}
	if _, err := MarshalBinary(&Program{}); err == nil {
		t.Fatal("MarshalBinary() accepted an invalid program")
	}
}

func TestBinaryCodecRejectsMalformedBoolean(t *testing.T) {
	t.Parallel()

	decoder := wireDecoder{data: []byte{byte(BooleanKind), 2}}
	if _, err := decoder.readValue(); err == nil {
		t.Fatal("readValue() accepted a malformed boolean")
	}
}

func TestBinaryCodecRejectsMalformedNumber(t *testing.T) {
	t.Parallel()

	var encoded bytes.Buffer
	encoded.WriteByte(byte(NumberKind))
	if err := writeString(&encoded, "1e100"); err != nil {
		t.Fatal(err)
	}
	decoder := wireDecoder{data: encoded.Bytes()}
	if _, err := decoder.readValue(); err == nil {
		t.Fatal("readValue() accepted a malformed decimal")
	}
}

func testSpan(line, column int) syntax.Span {
	return syntax.Span{
		Start: syntax.Position{Offset: column - 1, Line: line, Column: column},
		End:   syntax.Position{Offset: column, Line: line, Column: column + 1},
	}
}

func FuzzBinaryCodecNeverAcceptsAnUnstableProgram(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("MLBC\x01\x00\x00\x00\x00\x00"))
	f.Fuzz(func(t *testing.T, data []byte) {
		program, err := UnmarshalBinary(data)
		if err != nil {
			return
		}
		encoded, err := MarshalBinary(program)
		if err != nil {
			t.Fatalf("accepted program cannot be encoded: %v", err)
		}
		if _, err := UnmarshalBinary(encoded); err != nil {
			t.Fatalf("round-trip bytecode cannot be decoded: %v", err)
		}
	})
}
