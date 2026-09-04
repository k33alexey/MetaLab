package number

import "testing"

func TestParseCanonicalizesExactDecimals(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"0": "0", "+42.": "42", "-0.00": "0", "001.2300": "1.23",
		"0.000000001": "0.000000001", "99999999999999999999999999999999999999": "99999999999999999999999999999999999999",
	}
	for source, want := range tests {
		value, err := Parse(source)
		if err != nil {
			t.Fatalf("Parse(%q): %v", source, err)
		}
		if got := value.String(); got != want {
			t.Fatalf("Parse(%q) = %q, want %q", source, got, want)
		}
	}
}

func TestDecimalArithmeticIsExactAndBounded(t *testing.T) {
	t.Parallel()

	left := mustParse(t, "0.1")
	right := mustParse(t, "0.2")
	sum, err := left.Add(right)
	if err != nil || sum.String() != "0.3" {
		t.Fatalf("0.1 + 0.2 = %s, %v", sum, err)
	}
	difference, err := mustParse(t, "1000.25").Subtract(mustParse(t, "0.25"))
	if err != nil || difference.String() != "1000" {
		t.Fatalf("subtraction = %s, %v", difference, err)
	}
	product, err := mustParse(t, "12.5").Multiply(mustParse(t, "0.08"))
	if err != nil || product.String() != "1" {
		t.Fatalf("multiplication = %s, %v", product, err)
	}
	quotient, err := mustParse(t, "1").Divide(mustParse(t, "3"))
	if err != nil || quotient.String() != "0.33333333333333333333333333333333333333" {
		t.Fatalf("division = %s, %v", quotient, err)
	}
	remainder, err := mustParse(t, "5.5").Remainder(mustParse(t, "2"))
	if err != nil || remainder.String() != "1.5" {
		t.Fatalf("remainder = %s, %v", remainder, err)
	}
	if sum.String() != "0.3" || left.String() != "0.1" || right.String() != "0.2" {
		t.Fatal("an arithmetic operation mutated an operand")
	}
}

func TestDecimalRejectsInvalidAndOverflowingValues(t *testing.T) {
	t.Parallel()

	for _, source := range []string{"", ".1", "1.2.3", "1e3", "123456789012345678901234567890123456789", "0000000000000000000000000000000000000000001"} {
		if _, err := Parse(source); err == nil {
			t.Fatalf("Parse(%q) succeeded", source)
		}
	}
	if _, err := mustParse(t, "99999999999999999999999999999999999999").Add(mustParse(t, "1")); err == nil {
		t.Fatal("overflowing addition succeeded")
	}
	if _, err := mustParse(t, "1").Divide(mustParse(t, "0")); err == nil {
		t.Fatal("division by zero succeeded")
	}
}

func TestDecimalComparisonIntegerAndFloatBoundary(t *testing.T) {
	t.Parallel()

	if mustParse(t, "1.20").Compare(mustParse(t, "1.2")) != 0 || mustParse(t, "-2").Compare(mustParse(t, "-1")) >= 0 {
		t.Fatal("decimal comparison is incorrect")
	}
	integer, ok := mustParse(t, "42").Integer()
	if !ok || integer != 42 {
		t.Fatalf("Integer() = %d, %v", integer, ok)
	}
	if _, ok := mustParse(t, "42.5").Integer(); ok {
		t.Fatal("Integer() accepted a fractional number")
	}
	value, err := FromFloat64(42.5)
	if err != nil || value.String() != "42.5" || value.Float64() != 42.5 {
		t.Fatalf("FromFloat64() = %s, %v", value, err)
	}
}

func TestDecimalSmallAndLargeRepresentationsHaveIdenticalSemantics(t *testing.T) {
	t.Parallel()

	large, err := mustParse(t, "9223372036854775807").Add(mustParse(t, "1"))
	if err != nil || large.String() != "9223372036854775808" {
		t.Fatalf("large addition = %s, %v", large, err)
	}
	minimum := mustParse(t, "-9223372036854775808").Negate()
	if minimum.String() != "9223372036854775808" {
		t.Fatalf("negated minimum = %s", minimum)
	}
	quotient, err := mustParse(t, "0.1").Divide(mustParse(t, "2"))
	if err != nil || quotient.String() != "0.05" {
		t.Fatalf("exact division = %s, %v", quotient, err)
	}
	for source, want := range map[string]int64{"1.23454": 12345, "1.23455": 12346, "-1.23455": -12346} {
		got, roundErr := mustParse(t, source).RoundedInteger(4)
		if roundErr != nil || got != want {
			t.Fatalf("RoundedInteger(%s) = %d, %v; want %d", source, got, roundErr, want)
		}
	}
}

func mustParse(t testing.TB, source string) Decimal {
	t.Helper()
	value, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func BenchmarkDecimalAdd(b *testing.B) {
	left := mustParse(b, "123456789.1234")
	right := mustParse(b, "987654321.8766")
	b.ReportAllocs()
	for b.Loop() {
		if _, err := left.Add(right); err != nil {
			b.Fatal(err)
		}
	}
}

func FuzzDecimalArithmeticNeverPanics(f *testing.F) {
	f.Add("0.1", "0.2")
	f.Add("9223372036854775807", "-0.0001")
	f.Add("99999999999999999999999999999999999999", "3")
	f.Fuzz(func(t *testing.T, leftText, rightText string) {
		left, leftErr := Parse(leftText)
		right, rightErr := Parse(rightText)
		if leftErr != nil || rightErr != nil {
			return
		}
		if reparsed, err := Parse(left.String()); err != nil || reparsed.Compare(left) != 0 {
			t.Fatalf("canonical left value cannot be parsed: %v", err)
		}
		if left.Compare(right) != -right.Compare(left) {
			t.Fatal("comparison is not antisymmetric")
		}
		_, _ = left.Add(right)
		_, _ = left.Subtract(right)
		_, _ = left.Multiply(right)
		_, _ = left.Divide(right)
		_, _ = left.Remainder(right)
	})
}
