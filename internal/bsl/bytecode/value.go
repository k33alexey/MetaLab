package bytecode

import "fmt"

// ValueKind identifies the prototype runtime value representation.
type ValueKind uint8

const (
	UndefinedKind ValueKind = iota
	NumberKind
	StringKind
	BooleanKind
	ArrayKind
)

// Value is one immutable BSL runtime value. Later iterations add the remaining BSL types.
type Value struct {
	kind     ValueKind
	number   float64
	text     string
	boolean  bool
	elements []Value
}

// Undefined returns the BSL Undefined value.
func Undefined() Value { return Value{} }

// Number returns a number used by the architecture prototype.
func Number(value float64) Value { return Value{kind: NumberKind, number: value} }

// String returns a BSL string.
func String(value string) Value { return Value{kind: StringKind, text: value} }

// Boolean returns a BSL boolean value.
func Boolean(value bool) Value { return Value{kind: BooleanKind, boolean: value} }

// Array returns an immutable sequence value used by For Each. Collection APIs are added separately.
func Array(values ...Value) Value {
	return Value{kind: ArrayKind, elements: append([]Value(nil), values...)}
}

// Kind returns the value kind.
func (value Value) Kind() ValueKind { return value.kind }

// AsNumber returns the numeric payload.
func (value Value) AsNumber() (float64, bool) { return value.number, value.kind == NumberKind }

// AsString returns the string payload.
func (value Value) AsString() (string, bool) { return value.text, value.kind == StringKind }

// AsBoolean returns the boolean payload.
func (value Value) AsBoolean() (bool, bool) { return value.boolean, value.kind == BooleanKind }

// ArrayLength returns the sequence length.
func (value Value) ArrayLength() (int, bool) { return len(value.elements), value.kind == ArrayKind }

// ArrayElement returns one sequence element without exposing mutable storage.
func (value Value) ArrayElement(index int) (Value, bool) {
	if value.kind != ArrayKind || index < 0 || index >= len(value.elements) {
		return Undefined(), false
	}
	return value.elements[index], true
}

func (value Value) String() string {
	switch value.kind {
	case UndefinedKind:
		return "Undefined"
	case NumberKind:
		return fmt.Sprintf("%g", value.number)
	case StringKind:
		return value.text
	case BooleanKind:
		if value.boolean {
			return "True"
		}
		return "False"
	case ArrayKind:
		return fmt.Sprintf("Array(%d)", len(value.elements))
	default:
		return fmt.Sprintf("value(%d)", value.kind)
	}
}
