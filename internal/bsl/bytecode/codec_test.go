package bytecode

import (
	"bytes"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
)

func TestBinaryCodecRoundTripIsDeterministic(t *testing.T) {
	t.Parallel()

	program := &Program{Version: Version, Functions: []Function{{
		Name: "Расчёт", IsFunction: true, Export: true, Arity: 1, MaxStack: 2,
		Constants: []Value{Undefined(), Number(2.5), String("MetaLab")},
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
	if !ok || !function.IsFunction || !function.Export || function.Arity != 1 {
		t.Fatalf("decoded function = %+v", function)
	}
	if text, ok := function.Constants[2].AsString(); !ok || text != "MetaLab" {
		t.Fatalf("decoded string = %q, %v", text, ok)
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
