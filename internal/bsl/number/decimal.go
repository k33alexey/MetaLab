// Package number implements bounded decimal arithmetic for BSL values.
package number

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// MaxPrecision is the maximum number of decimal digits retained by a BSL number.
const MaxPrecision = 38

const maxDecimalTextLength = MaxPrecision + 3 // optional sign, zero, decimal point and fraction

var smallPowersOfTen = [...]int64{
	1,
	10,
	100,
	1_000,
	10_000,
	100_000,
	1_000_000,
	10_000_000,
	100_000_000,
	1_000_000_000,
	10_000_000_000,
	100_000_000_000,
	1_000_000_000_000,
	10_000_000_000_000,
	100_000_000_000_000,
	1_000_000_000_000_000,
	10_000_000_000_000_000,
	100_000_000_000_000_000,
	1_000_000_000_000_000_000,
}

// Decimal is an immutable fixed-point decimal number. Values that fit in int64
// use an allocation-free representation; larger values transparently use big.Int.
type Decimal struct {
	coefficient *big.Int
	small       int64
	scale       int
}

// Parse converts a non-exponential decimal representation into a value.
func Parse(text string) (Decimal, error) {
	if text == "" {
		return Decimal{}, fmt.Errorf("empty number")
	}
	if len(text) > maxDecimalTextLength {
		return Decimal{}, fmt.Errorf("number representation is too long")
	}
	negative := false
	if text[0] == '+' || text[0] == '-' {
		negative = text[0] == '-'
		text = text[1:]
	}
	if text == "" {
		return Decimal{}, fmt.Errorf("number contains no digits")
	}
	parts := strings.Split(text, ".")
	if len(parts) > 2 || parts[0] == "" {
		return Decimal{}, fmt.Errorf("invalid decimal number")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	digits := parts[0] + fraction
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return Decimal{}, fmt.Errorf("invalid decimal digit")
		}
	}
	signedDigits := digits
	if negative {
		signedDigits = "-" + signedDigits
	}
	if coefficient, err := strconv.ParseInt(signedDigits, 10, 64); err == nil {
		return checkedSmall(coefficient, len(fraction))
	}
	coefficient := new(big.Int)
	if _, ok := coefficient.SetString(digits, 10); !ok {
		return Decimal{}, fmt.Errorf("invalid decimal number")
	}
	if negative {
		coefficient.Neg(coefficient)
	}
	return checkedBig(coefficient, len(fraction))
}

// FromFloat64 converts a finite API boundary value through its shortest decimal form.
func FromFloat64(value float64) (Decimal, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return Decimal{}, fmt.Errorf("number must be finite")
	}
	return Parse(strconv.FormatFloat(value, 'f', -1, 64))
}

// FromScaledInteger constructs coefficient * 10^-scale without losing precision.
func FromScaledInteger(coefficient int64, scale int) (Decimal, error) {
	if scale < 0 {
		return Decimal{}, fmt.Errorf("scale must not be negative")
	}
	return checkedSmall(coefficient, scale)
}

// String returns the canonical non-exponential decimal representation.
func (value Decimal) String() string {
	if value.coefficient == nil {
		return formatCoefficient(strconv.FormatInt(value.small, 10), value.scale)
	}
	return formatCoefficient(value.coefficient.String(), value.scale)
}

func formatCoefficient(coefficient string, scale int) string {
	negative := strings.HasPrefix(coefficient, "-")
	if negative {
		coefficient = coefficient[1:]
	}
	if scale > 0 {
		if len(coefficient) <= scale {
			coefficient = strings.Repeat("0", scale-len(coefficient)+1) + coefficient
		}
		point := len(coefficient) - scale
		coefficient = coefficient[:point] + "." + coefficient[point:]
	}
	if negative && coefficient != "0" {
		return "-" + coefficient
	}
	return coefficient
}

// Float64 converts a decimal for external APIs that only support binary numbers.
func (value Decimal) Float64() float64 {
	if value.coefficient == nil {
		return float64(value.small) / math.Pow10(value.scale)
	}
	result, _ := strconv.ParseFloat(value.String(), 64)
	return result
}

// Integer returns an exact machine integer when the decimal has no fractional part.
func (value Decimal) Integer() (int64, bool) {
	if value.scale != 0 {
		return 0, false
	}
	if value.coefficient == nil {
		return value.small, true
	}
	return value.coefficient.Int64(), value.coefficient.IsInt64()
}

// RoundedInteger returns value * 10^scale rounded half away from zero.
func (value Decimal) RoundedInteger(scale int) (int64, error) {
	if scale < 0 {
		return 0, fmt.Errorf("scale must not be negative")
	}
	exponent := scale - value.scale
	if value.coefficient == nil {
		if exponent >= 0 {
			if result, ok := scaleInt64(value.small, exponent); ok {
				return result, nil
			}
		} else if -exponent < len(smallPowersOfTen) {
			return roundedQuotientInt64(value.small, smallPowersOfTen[-exponent]), nil
		}
	}
	coefficient := value.coefficientValue()
	if exponent >= 0 {
		coefficient.Mul(coefficient, powerOfTen(exponent))
	} else {
		divisor := powerOfTen(-exponent)
		quotient, remainder := new(big.Int), new(big.Int)
		quotient.QuoRem(coefficient, divisor, remainder)
		doubledRemainder := new(big.Int).Lsh(new(big.Int).Abs(remainder), 1)
		if doubledRemainder.Cmp(divisor) >= 0 {
			if coefficient.Sign() < 0 {
				quotient.Sub(quotient, big.NewInt(1))
			} else {
				quotient.Add(quotient, big.NewInt(1))
			}
		}
		coefficient = quotient
	}
	if !coefficient.IsInt64() {
		return 0, fmt.Errorf("scaled number exceeds int64")
	}
	return coefficient.Int64(), nil
}

// IsZero reports whether the number equals zero.
func (value Decimal) IsZero() bool {
	if value.coefficient == nil {
		return value.small == 0
	}
	return value.coefficient.Sign() == 0
}

// Negate returns the arithmetic opposite.
func (value Decimal) Negate() Decimal {
	if value.coefficient == nil && value.small != math.MinInt64 {
		return Decimal{small: -value.small, scale: value.scale}
	}
	coefficient := value.coefficientValue()
	coefficient.Neg(coefficient)
	return normalizeBig(coefficient, value.scale)
}

// Add performs exact bounded addition.
func (value Decimal) Add(other Decimal) (Decimal, error) {
	if left, right, scale, ok := alignSmall(value, other); ok {
		if coefficient, ok := addInt64(left, right); ok {
			return checkedSmall(coefficient, scale)
		}
	}
	left, right, scale := alignBig(value, other)
	return checkedBig(new(big.Int).Add(left, right), scale)
}

// Subtract performs exact bounded subtraction.
func (value Decimal) Subtract(other Decimal) (Decimal, error) {
	if left, right, scale, ok := alignSmall(value, other); ok {
		if coefficient, ok := subtractInt64(left, right); ok {
			return checkedSmall(coefficient, scale)
		}
	}
	left, right, scale := alignBig(value, other)
	return checkedBig(new(big.Int).Sub(left, right), scale)
}

// Multiply performs exact bounded multiplication.
func (value Decimal) Multiply(other Decimal) (Decimal, error) {
	if value.coefficient == nil && other.coefficient == nil {
		if coefficient, ok := multiplyInt64(value.small, other.small); ok {
			return checkedSmall(coefficient, value.scale+other.scale)
		}
	}
	coefficient := new(big.Int).Mul(value.coefficientValue(), other.coefficientValue())
	return checkedBig(coefficient, value.scale+other.scale)
}

// Divide returns a decimal rounded to at most MaxPrecision significant places.
func (value Decimal) Divide(other Decimal) (Decimal, error) {
	if other.IsZero() {
		return Decimal{}, fmt.Errorf("division by zero")
	}
	if numerator, denominator, _, ok := alignSmall(value, other); ok {
		for scale := 0; scale <= MaxPrecision; scale++ {
			if numerator%denominator == 0 {
				return checkedSmall(numerator/denominator, scale)
			}
			var multiplied bool
			numerator, multiplied = multiplyInt64(numerator, 10)
			if !multiplied {
				break
			}
		}
	}
	numerator := value.coefficientValue()
	denominator := other.coefficientValue()
	negative := numerator.Sign()*denominator.Sign() < 0
	numerator.Abs(numerator)
	denominator.Abs(denominator)

	integerNumerator := new(big.Int).Set(numerator)
	integerDenominator := new(big.Int).Set(denominator)
	if other.scale > value.scale {
		integerNumerator.Mul(integerNumerator, powerOfTen(other.scale-value.scale))
	} else if value.scale > other.scale {
		integerDenominator.Mul(integerDenominator, powerOfTen(value.scale-other.scale))
	}
	integerPart := new(big.Int).Quo(integerNumerator, integerDenominator)
	integerDigits := 0
	if integerPart.Sign() != 0 {
		integerDigits = len(integerPart.String())
	}
	if integerDigits > MaxPrecision {
		return Decimal{}, fmt.Errorf("number exceeds %d digits", MaxPrecision)
	}
	resultScale := MaxPrecision - integerDigits
	exponent := resultScale + other.scale - value.scale
	if exponent >= 0 {
		numerator.Mul(numerator, powerOfTen(exponent))
	} else {
		denominator.Mul(denominator, powerOfTen(-exponent))
	}
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(denominator) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if negative {
		quotient.Neg(quotient)
	}
	return checkedBig(quotient, resultScale)
}

// Remainder implements a - trunc(a/b)*b for fractional operands.
func (value Decimal) Remainder(other Decimal) (Decimal, error) {
	if other.IsZero() {
		return Decimal{}, fmt.Errorf("division by zero")
	}
	if left, right, scale, ok := alignSmall(value, other); ok {
		return checkedSmall(left%right, scale)
	}
	left, right, scale := alignBig(value, other)
	return checkedBig(new(big.Int).Rem(left, right), scale)
}

// Compare returns -1, 0 or 1.
func (value Decimal) Compare(other Decimal) int {
	if left, right, _, ok := alignSmall(value, other); ok {
		if left < right {
			return -1
		}
		if left > right {
			return 1
		}
		return 0
	}
	left, right, _ := alignBig(value, other)
	return left.Cmp(right)
}

func checkedSmall(coefficient int64, scale int) (Decimal, error) {
	result := normalizeSmall(coefficient, scale)
	if err := result.validatePrecision(); err != nil {
		return Decimal{}, err
	}
	return result, nil
}

func checkedBig(coefficient *big.Int, scale int) (Decimal, error) {
	result := normalizeBig(coefficient, scale)
	if err := result.validatePrecision(); err != nil {
		return Decimal{}, err
	}
	return result, nil
}

func normalizeSmall(coefficient int64, scale int) Decimal {
	if coefficient == 0 {
		return Decimal{}
	}
	for scale > 0 && coefficient%10 == 0 {
		coefficient /= 10
		scale--
	}
	return Decimal{small: coefficient, scale: scale}
}

func normalizeBig(coefficient *big.Int, scale int) Decimal {
	if coefficient.Sign() == 0 {
		return Decimal{}
	}
	coefficient = new(big.Int).Set(coefficient)
	ten := big.NewInt(10)
	quotient, remainder := new(big.Int), new(big.Int)
	for scale > 0 {
		quotient.QuoRem(coefficient, ten, remainder)
		if remainder.Sign() != 0 {
			break
		}
		coefficient.Set(quotient)
		scale--
	}
	if coefficient.IsInt64() {
		return normalizeSmall(coefficient.Int64(), scale)
	}
	return Decimal{coefficient: coefficient, scale: scale}
}

func (value Decimal) validatePrecision() error {
	digits := 1
	if value.coefficient == nil {
		digits = decimalDigits(value.small)
	} else {
		text := value.coefficient.String()
		digits = len(text)
		if text[0] == '-' {
			digits--
		}
	}
	if value.scale > digits {
		digits = value.scale
	}
	if digits > MaxPrecision {
		return fmt.Errorf("number exceeds %d digits", MaxPrecision)
	}
	return nil
}

func decimalDigits(value int64) int {
	digits := 1
	for value <= -10 || value >= 10 {
		value /= 10
		digits++
	}
	return digits
}

func (value Decimal) coefficientValue() *big.Int {
	if value.coefficient == nil {
		return big.NewInt(value.small)
	}
	return new(big.Int).Set(value.coefficient)
}

func alignSmall(left, right Decimal) (int64, int64, int, bool) {
	if left.coefficient != nil || right.coefficient != nil {
		return 0, 0, 0, false
	}
	scale := left.scale
	if right.scale > scale {
		scale = right.scale
	}
	leftCoefficient, rightCoefficient := left.small, right.small
	var ok bool
	if scale > left.scale {
		leftCoefficient, ok = scaleInt64(leftCoefficient, scale-left.scale)
		if !ok {
			return 0, 0, 0, false
		}
	}
	if scale > right.scale {
		rightCoefficient, ok = scaleInt64(rightCoefficient, scale-right.scale)
		if !ok {
			return 0, 0, 0, false
		}
	}
	return leftCoefficient, rightCoefficient, scale, true
}

func alignBig(left, right Decimal) (*big.Int, *big.Int, int) {
	scale := left.scale
	if right.scale > scale {
		scale = right.scale
	}
	leftCoefficient := left.coefficientValue()
	rightCoefficient := right.coefficientValue()
	if scale > left.scale {
		leftCoefficient.Mul(leftCoefficient, powerOfTen(scale-left.scale))
	}
	if scale > right.scale {
		rightCoefficient.Mul(rightCoefficient, powerOfTen(scale-right.scale))
	}
	return leftCoefficient, rightCoefficient, scale
}

func scaleInt64(value int64, exponent int) (int64, bool) {
	if exponent < 0 || exponent >= len(smallPowersOfTen) {
		return 0, false
	}
	return multiplyInt64(value, smallPowersOfTen[exponent])
}

func addInt64(left, right int64) (int64, bool) {
	if right > 0 && left > math.MaxInt64-right || right < 0 && left < math.MinInt64-right {
		return 0, false
	}
	return left + right, true
}

func subtractInt64(left, right int64) (int64, bool) {
	if right < 0 && left > math.MaxInt64+right || right > 0 && left < math.MinInt64+right {
		return 0, false
	}
	return left - right, true
}

func multiplyInt64(left, right int64) (int64, bool) {
	if left == 0 || right == 0 {
		return 0, true
	}
	if left == -1 && right == math.MinInt64 || right == -1 && left == math.MinInt64 {
		return 0, false
	}
	result := left * right
	if result/right != left {
		return 0, false
	}
	return result, true
}

func roundedQuotientInt64(value, divisor int64) int64 {
	quotient, remainder := value/divisor, value%divisor
	if remainder < 0 {
		remainder = -remainder
	}
	if remainder*2 >= divisor {
		if value < 0 {
			quotient--
		} else {
			quotient++
		}
	}
	return quotient
}

func powerOfTen(exponent int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exponent)), nil)
}
