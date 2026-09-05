package bytecode

import (
	"fmt"
	"math"
	"strconv"
	"time"

	bslnumber "github.com/k33alexey/MetaLab/internal/bsl/number"
)

// ValueKind identifies the prototype runtime value representation.
type ValueKind uint8

const (
	UndefinedKind ValueKind = iota
	NumberKind
	StringKind
	BooleanKind
	ArrayKind
	NullKind
	DateKind
)

var valueKindNames = [...]string{
	UndefinedKind: "undefined",
	NumberKind:    "number",
	StringKind:    "string",
	BooleanKind:   "boolean",
	ArrayKind:     "array",
	NullKind:      "null",
	DateKind:      "date",
}

func (kind ValueKind) String() string {
	if int(kind) >= len(valueKindNames) || valueKindNames[kind] == "" {
		return fmt.Sprintf("value_kind(%d)", kind)
	}
	return valueKindNames[kind]
}

// Value is one immutable BSL runtime value. Later iterations add the remaining BSL types.
type Value struct {
	kind      ValueKind
	number    bslnumber.Decimal
	text      string
	boolean   bool
	elements  []Value
	dateTicks int64
}

// Undefined returns the BSL Undefined value.
func Undefined() Value { return Value{} }

// Number returns a finite number for trusted internal callers and panics if the
// value cannot be represented. External input must use NumberFromFloat64.
func Number(value float64) Value {
	result, err := NumberFromFloat64(value)
	if err != nil {
		panic(err)
	}
	return result
}

// NumberFromFloat64 validates and converts a binary-number API boundary value.
func NumberFromFloat64(value float64) (Value, error) {
	decimal, err := bslnumber.FromFloat64(value)
	if err != nil {
		return Undefined(), err
	}
	return Value{kind: NumberKind, number: decimal}, nil
}

// ParseNumber constructs an exact decimal BSL number from source or wire text.
func ParseNumber(text string) (Value, error) {
	decimal, err := bslnumber.Parse(text)
	if err != nil {
		return Undefined(), err
	}
	return Value{kind: NumberKind, number: decimal}, nil
}

// String returns a BSL string.
func String(value string) Value { return Value{kind: StringKind, text: value} }

// Boolean returns a BSL boolean value.
func Boolean(value bool) Value { return Value{kind: BooleanKind, boolean: value} }

// Array returns an immutable sequence value used by For Each. Collection APIs are added separately.
func Array(values ...Value) Value {
	return Value{kind: ArrayKind, elements: append([]Value(nil), values...)}
}

// Null returns the database NULL value, which is distinct from Undefined.
func Null() Value { return Value{kind: NullKind} }

// Date returns a date with 100-microsecond precision in the supported BSL range.
func Date(value time.Time) (Value, error) {
	value = value.UTC()
	if value.Year() < 1 || value.Year() > 3999 {
		return Undefined(), fmt.Errorf("date year %d is outside 1..3999", value.Year())
	}
	ticks := value.Unix()*10_000 + int64(value.Nanosecond()/100_000)
	return Value{kind: DateKind, dateTicks: ticks}, nil
}

// ParseDate parses an 8- or 14-digit BSL date literal.
func ParseDate(text string) (Value, error) {
	if len(text) != 8 && len(text) != 14 {
		return Undefined(), fmt.Errorf("date literal must contain 8 or 14 digits")
	}
	part := func(start, end int) (int, error) { return strconv.Atoi(text[start:end]) }
	year, err := part(0, 4)
	if err != nil {
		return Undefined(), fmt.Errorf("invalid date year")
	}
	month, monthErr := part(4, 6)
	day, dayErr := part(6, 8)
	hour, minute, second := 0, 0, 0
	if len(text) == 14 {
		var minuteErr, secondErr error
		hour, err = part(8, 10)
		minute, minuteErr = part(10, 12)
		second, secondErr = part(12, 14)
		if err != nil || minuteErr != nil || secondErr != nil {
			return Undefined(), fmt.Errorf("invalid date time")
		}
	}
	if monthErr != nil || dayErr != nil || year < 1 || year > 3999 {
		return Undefined(), fmt.Errorf("invalid date")
	}
	parsed := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC)
	if parsed.Year() != year || int(parsed.Month()) != month || parsed.Day() != day ||
		parsed.Hour() != hour || parsed.Minute() != minute || parsed.Second() != second {
		return Undefined(), fmt.Errorf("invalid calendar date")
	}
	return Date(parsed)
}

// Kind returns the value kind.
func (value Value) Kind() ValueKind { return value.kind }

// AsNumber returns the numeric payload.
func (value Value) AsNumber() (float64, bool) {
	return value.number.Float64(), value.kind == NumberKind
}

// NumberText returns the exact canonical decimal representation.
func (value Value) NumberText() (string, bool) {
	return value.number.String(), value.kind == NumberKind
}

// NumberInteger returns an exact integer payload.
func (value Value) NumberInteger() (int64, bool) {
	if value.kind != NumberKind {
		return 0, false
	}
	return value.number.Integer()
}

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

// DynamicMemory estimates heap storage retained by strings and arrays. The
// limit makes the traversal safe for untrusted, deeply nested values.
func (value Value) DynamicMemory(limit uint64) (uint64, bool) {
	return dynamicMemory(value, limit, 0)
}

func dynamicMemory(value Value, limit uint64, depth int) (uint64, bool) {
	const (
		estimatedValueSlotBytes = uint64(96)
		maxNesting              = 64
	)
	if depth > maxNesting {
		return limit, false
	}
	var size uint64
	switch value.kind {
	case StringKind:
		size = uint64(len(value.text))
	case ArrayKind:
		if uint64(len(value.elements)) > limit/estimatedValueSlotBytes {
			return limit, false
		}
		size = uint64(len(value.elements)) * estimatedValueSlotBytes
		for _, element := range value.elements {
			remaining := limit - min(size, limit)
			elementSize, ok := dynamicMemory(element, remaining, depth+1)
			if !ok || elementSize > remaining {
				return limit, false
			}
			size += elementSize
		}
	}
	return size, size <= limit
}

// AsDate returns the date payload in UTC.
func (value Value) AsDate() (time.Time, bool) {
	if value.kind != DateKind {
		return time.Time{}, false
	}
	seconds := value.dateTicks / 10_000
	remainder := value.dateTicks % 10_000
	if remainder < 0 {
		seconds--
		remainder += 10_000
	}
	return time.Unix(seconds, remainder*100_000).UTC(), true
}

// DateTicks returns 100-microsecond ticks from the Unix epoch for wire encoding.
func (value Value) DateTicks() (int64, bool) { return value.dateTicks, value.kind == DateKind }

// DateFromTicks restores a wire-encoded date after validating its range.
func DateFromTicks(ticks int64) (Value, error) {
	value := Value{kind: DateKind, dateTicks: ticks}
	date, _ := value.AsDate()
	if date.Year() < 1 || date.Year() > 3999 {
		return Undefined(), fmt.Errorf("encoded date is outside 1..3999")
	}
	return value, nil
}

// AddDateSeconds adds a number of seconds using BSL's 100-microsecond precision.
func AddDateSeconds(value Value, seconds Value) (Value, error) {
	if value.kind != DateKind || seconds.kind != NumberKind {
		return Undefined(), fmt.Errorf("invalid date arithmetic")
	}
	ticks, err := seconds.number.RoundedInteger(4)
	if err != nil {
		return Undefined(), fmt.Errorf("date arithmetic overflow: %w", err)
	}
	if (ticks > 0 && value.dateTicks > math.MaxInt64-ticks) ||
		(ticks < 0 && value.dateTicks < math.MinInt64-ticks) {
		return Undefined(), fmt.Errorf("date arithmetic overflow")
	}
	return DateFromTicks(value.dateTicks + ticks)
}

// DateDifferenceSeconds returns the exact signed interval between two dates in seconds.
func DateDifferenceSeconds(left, right Value) (Value, bool) {
	if left.kind != DateKind || right.kind != DateKind {
		return Undefined(), false
	}
	number, err := bslnumber.FromScaledInteger(left.dateTicks-right.dateTicks, 4)
	if err != nil {
		return Undefined(), false
	}
	return Value{kind: NumberKind, number: number}, true
}

func (value Value) String() string {
	switch value.kind {
	case UndefinedKind:
		return "Undefined"
	case NumberKind:
		return value.number.String()
	case StringKind:
		return value.text
	case BooleanKind:
		if value.boolean {
			return "True"
		}
		return "False"
	case ArrayKind:
		return fmt.Sprintf("Array(%d)", len(value.elements))
	case NullKind:
		return "Null"
	case DateKind:
		date, _ := value.AsDate()
		return date.Format("2006-01-02 15:04:05.0000")
	default:
		return fmt.Sprintf("value(%d)", value.kind)
	}
}

// NegateNumber returns -value.
func NegateNumber(value Value) (Value, error) {
	if value.kind != NumberKind {
		return Undefined(), fmt.Errorf("value is not a number")
	}
	return Value{kind: NumberKind, number: value.number.Negate()}, nil
}

// AddNumbers performs bounded exact decimal addition.
func AddNumbers(left, right Value) (Value, error) { return numberOperation(left, right, "add") }

// SubtractNumbers performs bounded exact decimal subtraction.
func SubtractNumbers(left, right Value) (Value, error) {
	return numberOperation(left, right, "subtract")
}

// MultiplyNumbers performs bounded exact decimal multiplication.
func MultiplyNumbers(left, right Value) (Value, error) {
	return numberOperation(left, right, "multiply")
}

// DivideNumbers performs bounded rounded decimal division.
func DivideNumbers(left, right Value) (Value, error) { return numberOperation(left, right, "divide") }

// RemainderNumbers performs bounded exact decimal remainder.
func RemainderNumbers(left, right Value) (Value, error) {
	return numberOperation(left, right, "remainder")
}

// CompareNumbers compares two numeric values.
func CompareNumbers(left, right Value) (int, bool) {
	if left.kind != NumberKind || right.kind != NumberKind {
		return 0, false
	}
	return left.number.Compare(right.number), true
}

func numberOperation(left, right Value, operation string) (Value, error) {
	if left.kind != NumberKind || right.kind != NumberKind {
		return Undefined(), fmt.Errorf("numeric operation requires two numbers")
	}
	var (
		result bslnumber.Decimal
		err    error
	)
	switch operation {
	case "add":
		result, err = left.number.Add(right.number)
	case "subtract":
		result, err = left.number.Subtract(right.number)
	case "multiply":
		result, err = left.number.Multiply(right.number)
	case "divide":
		result, err = left.number.Divide(right.number)
	case "remainder":
		result, err = left.number.Remainder(right.number)
	default:
		return Undefined(), fmt.Errorf("unknown numeric operation %q", operation)
	}
	if err != nil {
		return Undefined(), err
	}
	return Value{kind: NumberKind, number: result}, nil
}
