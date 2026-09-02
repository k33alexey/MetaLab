package bytecode

import "fmt"

// ValueKind identifies the prototype runtime value representation.
type ValueKind uint8

const (
	UndefinedKind ValueKind = iota
	NumberKind
	StringKind
)

// Value is one BSL runtime value. Later iterations add the remaining BSL types.
type Value struct {
	kind   ValueKind
	number float64
	text   string
}

// Undefined returns the BSL Undefined value.
func Undefined() Value { return Value{} }

// Number returns a number used by the architecture prototype.
func Number(value float64) Value { return Value{kind: NumberKind, number: value} }

// String returns a BSL string.
func String(value string) Value { return Value{kind: StringKind, text: value} }

// Kind returns the value kind.
func (value Value) Kind() ValueKind { return value.kind }

// AsNumber returns the numeric payload.
func (value Value) AsNumber() (float64, bool) { return value.number, value.kind == NumberKind }

// AsString returns the string payload.
func (value Value) AsString() (string, bool) { return value.text, value.kind == StringKind }

func (value Value) String() string {
	switch value.kind {
	case UndefinedKind:
		return "Undefined"
	case NumberKind:
		return fmt.Sprintf("%g", value.number)
	case StringKind:
		return value.text
	default:
		return fmt.Sprintf("value(%d)", value.kind)
	}
}
