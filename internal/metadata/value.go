package metadata

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	bslnumber "github.com/k33alexey/MetaLab/internal/bsl/number"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Value is the canonical JSON representation persisted for one constant.
// Data remains text so decimals and stable UUIDs never lose precision.
type Value struct {
	Kind TypeKind `json:"kind" yaml:"kind"`
	Data string   `json:"data" yaml:"data"`
}

// NormalizeValue validates a value against a constant and canonicalizes it.
func (catalog *Catalog) NormalizeValue(constant Constant, value Value) (Value, error) {
	return catalog.normalizeTypes("constant "+constant.Name, constant.Types, value)
}

func (catalog *Catalog) normalizeTypes(owner string, types []Type, value Value) (Value, error) {
	types, err := catalog.expandTypes(types, nil)
	if err != nil {
		return Value{}, err
	}
	var reasons []string
	for _, allowed := range types {
		if allowed.Kind != value.Kind {
			continue
		}
		normalized, valid, reason := catalog.normalizeAs(value, allowed)
		if valid {
			return normalized, nil
		}
		if reason != "" {
			reasons = append(reasons, reason)
		}
	}
	if len(reasons) == 0 {
		return Value{}, fmt.Errorf("%s does not allow value kind %s", owner, value.Kind)
	}
	sort.Strings(reasons)
	return Value{}, fmt.Errorf("%s value is invalid: %s", owner, strings.Join(reasons, "; "))
}

func (catalog *Catalog) allowsCatalogReference(types []Type, catalogID uuid.UUID) (bool, error) {
	resolved, err := catalog.expandTypes(types, nil)
	if err != nil {
		return false, err
	}
	for _, item := range resolved {
		if item.Kind == CatalogType && item.Reference != nil && *item.Reference == catalogID {
			return true, nil
		}
	}
	return false, nil
}

func (catalog *Catalog) expandTypes(types []Type, stack map[uuid.UUID]bool) ([]Type, error) {
	if stack == nil {
		stack = make(map[uuid.UUID]bool)
	}
	result := make([]Type, 0, len(types))
	seen := make(map[string]bool)
	var appendTypes func([]Type) error
	appendTypes = func(items []Type) error {
		for _, item := range items {
			if item.Kind == DefinedType {
				if item.Reference == nil || stack[*item.Reference] {
					return fmt.Errorf("invalid defined type expansion")
				}
				index, ok := catalog.definedTypeByID[*item.Reference]
				if !ok {
					return fmt.Errorf("unknown defined type %s", item.Reference)
				}
				stack[*item.Reference] = true
				if err := appendTypes(catalog.DefinedTypes[index].Types); err != nil {
					return err
				}
				delete(stack, *item.Reference)
				continue
			}
			key := fmt.Sprintf("%s:%d:%d:%d", item.Kind, item.Length, item.Precision, item.Scale)
			if item.Reference != nil {
				key += ":" + item.Reference.String()
			}
			if !seen[key] {
				seen[key] = true
				result = append(result, item)
			}
		}
		return nil
	}
	if err := appendTypes(types); err != nil {
		return nil, err
	}
	return result, nil
}

func (catalog *Catalog) normalizeAs(value Value, allowed Type) (Value, bool, string) {
	switch allowed.Kind {
	case StringType:
		if !utf8.ValidString(value.Data) {
			return Value{}, false, "string is not valid UTF-8"
		}
		if allowed.Length != 0 && utf8.RuneCountInString(value.Data) > allowed.Length {
			return Value{}, false, fmt.Sprintf("string exceeds %d characters", allowed.Length)
		}
		return value, true, ""
	case NumberType:
		number, err := bslnumber.Parse(value.Data)
		if err != nil {
			return Value{}, false, "invalid decimal number"
		}
		canonical := number.String()
		precision, scale := decimalSize(canonical)
		if precision > allowed.Precision || scale > allowed.Scale {
			return Value{}, false, fmt.Sprintf("number exceeds precision %d scale %d", allowed.Precision, allowed.Scale)
		}
		return Value{Kind: NumberType, Data: canonical}, true, ""
	case BooleanType:
		if value.Data != "true" && value.Data != "false" {
			return Value{}, false, "boolean must be true or false"
		}
		return value, true, ""
	case DateType:
		parsed, err := time.Parse(time.RFC3339Nano, value.Data)
		if err != nil || parsed.Year() < 1 || parsed.Year() > 3999 {
			return Value{}, false, "date must be RFC3339 within years 1..3999"
		}
		return Value{Kind: DateType, Data: parsed.UTC().Format(time.RFC3339Nano)}, true, ""
	case UUIDType, CatalogType:
		id, err := uuid.Parse(value.Data)
		if err != nil {
			return Value{}, false, "invalid UUID reference"
		}
		if allowed.Kind == CatalogType && id.IsZero() {
			return Value{}, false, "catalog reference cannot be empty"
		}
		return Value{Kind: allowed.Kind, Data: id.String()}, true, ""
	case EnumerationType:
		id, err := uuid.Parse(value.Data)
		if err != nil || allowed.Reference == nil {
			return Value{}, false, "invalid enumeration value UUID"
		}
		index, ok := catalog.enumerationByID[*allowed.Reference]
		if !ok {
			return Value{}, false, "unknown enumeration"
		}
		for _, item := range catalog.Enumerations[index].Values {
			if item.ID == id {
				return Value{Kind: EnumerationType, Data: id.String()}, true, ""
			}
		}
		return Value{}, false, "value does not belong to the enumeration"
	default:
		return Value{}, false, "unsupported value kind"
	}
}

func decimalSize(value string) (precision, scale int) {
	value = strings.TrimPrefix(value, "-")
	parts := strings.SplitN(value, ".", 2)
	integer := strings.TrimLeft(parts[0], "0")
	if integer == "" {
		integer = "0"
	}
	precision = len(integer)
	if len(parts) == 2 {
		scale = len(parts[1])
		precision += scale
	}
	return precision, scale
}
