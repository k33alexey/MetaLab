package metadata

import (
	"encoding/base64"
	"encoding/json"
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
//
// A reference carries Object as well: which metadata object the referenced
// item belongs to. Without it a reference is only half a reference. While a
// column held one declared kind of object, the missing half was supplied by
// the column itself and guarded by a foreign key; a set has neither, and a
// composite column of two catalogs cannot say which of them an item came from.
// The cost of leaving it out was not a refusal but a wrong answer: a value was
// resolved against the first type of the description that matched by kind.
type Value struct {
	Kind   TypeKind  `json:"kind" yaml:"kind"`
	Data   string    `json:"data" yaml:"data"`
	Object uuid.UUID `json:"object,omitempty" yaml:"object,omitempty"`
}

// MarshalJSON keeps a value without an object spelled the way it always was.
// An array field is never omitted by the encoder, and a stored value is not
// the place to carry a zero identifier on every string and number.
func (value Value) MarshalJSON() ([]byte, error) {
	if value.Object.IsZero() {
		return json.Marshal(struct {
			Kind TypeKind `json:"kind"`
			Data string   `json:"data"`
		}{value.Kind, value.Data})
	}
	type stored Value
	return json.Marshal(stored(value))
}

// NormalizeValue validates a value against a constant and canonicalizes it.
func (catalog *Catalog) NormalizeValue(constant Constant, value Value) (Value, error) {
	return catalog.normalizeTypes("constant "+constant.Name, constant.Types, value)
}

// NormalizeSessionParameterValue validates a value against a session parameter and canonicalizes it.
func (catalog *Catalog) NormalizeSessionParameterValue(parameter SessionParameter, value Value) (Value, error) {
	return catalog.normalizeTypes("session parameter "+parameter.Name, parameter.Types, value)
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

// checkReferenceObject is the check the requirements call "by the registry,
// not by a list": the value must name a metadata object, that object must be
// the one this element of the description allows, and it must still be in the
// configuration at the moment of the write.
//
// A value that names no object is refused rather than guessed at. Guessing is
// what used to happen, and it returned an item of one catalog wearing the name
// of another.
func (catalog *Catalog) checkReferenceObject(value Value, allowed Type) string {
	if allowed.Reference == nil {
		return "the allowed type does not say which object it points at"
	}
	if value.Object.IsZero() {
		return "a reference must say which object it points at"
	}
	if value.Object != *allowed.Reference {
		return "reference to " + value.Object.String() + " is not allowed here"
	}
	if !catalog.hasObject(allowed.Kind, value.Object) {
		return "object " + value.Object.String() + " is not in the configuration"
	}
	return ""
}

// hasObject answers the registry.
func (catalog *Catalog) hasObject(kind TypeKind, id uuid.UUID) bool {
	var found bool
	switch kind {
	case EnumerationType:
		_, found = catalog.enumerationByID[id]
	case CatalogType:
		_, found = catalog.catalogByID[id]
	case DocumentType:
		_, found = catalog.documentByID[id]
	case CharacteristicTypesType:
		_, found = catalog.chartOfCharacteristicTypesByID[id]
	case AccountType:
		_, found = catalog.chartOfAccountsByID[id]
	case CalculationTypeType:
		_, found = catalog.chartOfCalculationTypesByID[id]
	case BusinessProcessType, RoutePointType:
		_, found = catalog.businessProcessByID[id]
	case TaskType:
		_, found = catalog.taskByID[id]
	case ExchangePlanType:
		_, found = catalog.exchangePlanByID[id]
	}
	return found
}

func (catalog *Catalog) allowsObjectReference(types []Type, kind TypeKind, objectID uuid.UUID) (bool, error) {
	resolved, err := catalog.expandTypes(types, nil)
	if err != nil {
		return false, err
	}
	for _, item := range resolved {
		if item.Kind == kind && item.Reference != nil && *item.Reference == objectID {
			return true, nil
		}
	}
	return false, nil
}

// setMembers is what a set contains right now, read off the configuration and
// never written down anywhere. The list is recomputed on every use, and that
// is the point: an object added today is in the set today.
func (catalog *Catalog) setMembers(kind TypeKind) []Type {
	var result []Type
	add := func(want TypeKind, kindOf TypeKind, ids []uuid.UUID) {
		if kind != AnyReferenceSet && kind != want {
			return
		}
		for index := range ids {
			id := ids[index]
			result = append(result, Type{Kind: kindOf, Reference: &id})
		}
	}
	ids := func(count int, at func(int) uuid.UUID) []uuid.UUID {
		list := make([]uuid.UUID, count)
		for index := range list {
			list[index] = at(index)
		}
		return list
	}
	add(CatalogSet, CatalogType, ids(len(catalog.Catalogs), func(i int) uuid.UUID { return catalog.Catalogs[i].ID }))
	add(DocumentSet, DocumentType, ids(len(catalog.Documents), func(i int) uuid.UUID { return catalog.Documents[i].ID }))
	add(EnumerationSet, EnumerationType, ids(len(catalog.Enumerations), func(i int) uuid.UUID { return catalog.Enumerations[i].ID }))
	add(CharacteristicTypesSet, CharacteristicTypesType, ids(len(catalog.ChartsOfCharacteristicTypes), func(i int) uuid.UUID { return catalog.ChartsOfCharacteristicTypes[i].ID }))
	add(AccountSet, AccountType, ids(len(catalog.ChartsOfAccounts), func(i int) uuid.UUID { return catalog.ChartsOfAccounts[i].ID }))
	add(CalculationTypeSet, CalculationTypeType, ids(len(catalog.ChartsOfCalculationTypes), func(i int) uuid.UUID { return catalog.ChartsOfCalculationTypes[i].ID }))
	add(BusinessProcessSet, BusinessProcessType, ids(len(catalog.BusinessProcesses), func(i int) uuid.UUID { return catalog.BusinessProcesses[i].ID }))
	// A route point belongs to the process that drew it, so the set of route
	// points is one member per business process, not one member in total.
	add(RoutePointSet, RoutePointType, ids(len(catalog.BusinessProcesses), func(i int) uuid.UUID { return catalog.BusinessProcesses[i].ID }))
	add(TaskSet, TaskType, ids(len(catalog.Tasks), func(i int) uuid.UUID { return catalog.Tasks[i].ID }))
	add(ExchangePlanSet, ExchangePlanType, ids(len(catalog.ExchangePlans), func(i int) uuid.UUID { return catalog.ExchangePlans[i].ID }))
	return result
}

// typesOpenToConfiguration says whether a type description lets the
// configuration widen it later. It is asked before storage is chosen, and it
// is asked instead of counting what the set contains today: a set holding one
// object right now is still a set, and giving it a column of that object's own
// would mean rebuilding the table the day a second object appears.
func (catalog *Catalog) typesOpenToConfiguration(types []Type, stack map[uuid.UUID]bool) (bool, error) {
	if stack == nil {
		stack = make(map[uuid.UUID]bool)
	}
	for _, item := range types {
		if referenceSets[item.Kind] || item.Kind == CharacteristicSet {
			return true, nil
		}
		if item.Kind != DefinedType {
			continue
		}
		if item.Reference == nil || stack[*item.Reference] {
			return false, fmt.Errorf("invalid defined type expansion")
		}
		index, ok := catalog.definedTypeByID[*item.Reference]
		if !ok {
			return false, fmt.Errorf("unknown defined type %s", item.Reference)
		}
		stack[*item.Reference] = true
		open, err := catalog.typesOpenToConfiguration(catalog.DefinedTypes[index].Types, stack)
		delete(stack, *item.Reference)
		if err != nil || open {
			return open, err
		}
	}
	return false, nil
}

// typeKey tells two types apart. Everything that makes them behave differently
// belongs in it, qualifiers included: a fixed-length string and a variable one
// are not the same type, and collapsing them would quietly drop one of them.
func typeKey(item Type) string {
	key := fmt.Sprintf("%s:%d:%d:%d:%t:%t:%s", item.Kind, item.Length, item.Precision, item.Scale,
		item.FixedLength, item.NonNegative, item.DateParts)
	if item.Reference != nil {
		key += ":" + item.Reference.String()
	}
	return key
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
			if referenceSets[item.Kind] {
				if err := appendTypes(catalog.setMembers(item.Kind)); err != nil {
					return err
				}
				continue
			}
			if item.Kind == CharacteristicSet {
				// What a characteristic may hold is decided by its chart, and
				// a chart is free to allow characteristics of itself. Walking
				// that without a guard would not end.
				if item.Reference == nil || stack[*item.Reference] {
					return fmt.Errorf("invalid characteristic type expansion")
				}
				index, ok := catalog.chartOfCharacteristicTypesByID[*item.Reference]
				if !ok {
					return fmt.Errorf("unknown chart of characteristic types %s", item.Reference)
				}
				stack[*item.Reference] = true
				if err := appendTypes(catalog.ChartsOfCharacteristicTypes[index].ValueType); err != nil {
					return err
				}
				delete(stack, *item.Reference)
				continue
			}
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
			key := typeKey(item)
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
		if allowed.NonNegative && strings.HasPrefix(canonical, "-") {
			return Value{}, false, "number must not be negative"
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
		// The date-parts qualifier is not decoration: an attribute that is
		// about a day must not carry a time that nobody entered and every
		// comparison then trips over.
		moment := parsed.UTC()
		switch allowed.DateParts {
		case DateOnlyParts:
			moment = time.Date(moment.Year(), moment.Month(), moment.Day(), 0, 0, 0, 0, time.UTC)
		case TimeOnlyParts:
			moment = time.Date(1, time.January, 1, moment.Hour(), moment.Minute(), moment.Second(), moment.Nanosecond(), time.UTC)
		}
		return Value{Kind: DateType, Data: moment.Format(time.RFC3339Nano)}, true, ""
	case ValueStorageType:
		if _, err := base64.StdEncoding.DecodeString(value.Data); err != nil {
			return Value{}, false, "value storage must be base64"
		}
		return Value{Kind: ValueStorageType, Data: value.Data}, true, ""
	case ObjectUUIDType:
		id, err := uuid.Parse(value.Data)
		if err != nil {
			return Value{}, false, "invalid UUID reference"
		}
		return Value{Kind: ObjectUUIDType, Data: id.String()}, true, ""
	case CatalogType, DocumentType, CharacteristicTypesType, AccountType, CalculationTypeType, BusinessProcessType, TaskType, ExchangePlanType:
		id, err := uuid.Parse(value.Data)
		if err != nil {
			return Value{}, false, "invalid UUID reference"
		}
		if id.IsZero() {
			return Value{}, false, "object reference cannot be empty"
		}
		if reason := catalog.checkReferenceObject(value, allowed); reason != "" {
			return Value{}, false, reason
		}
		return Value{Kind: allowed.Kind, Data: id.String(), Object: *allowed.Reference}, true, ""
	case RoutePointType:
		// A point is carried by name, and the name means something only inside
		// the process that drew the map: a value naming a point no map has
		// would be stored and then match nothing for the rest of its life.
		// The empty name is the point a task has not reached yet.
		if allowed.Reference == nil {
			return Value{}, false, "route point without the business process it belongs to"
		}
		index, ok := catalog.businessProcessByID[*allowed.Reference]
		if !ok {
			return Value{}, false, "unknown business process"
		}
		if reason := catalog.checkReferenceObject(value, allowed); reason != "" {
			return Value{}, false, reason
		}
		if value.Data == "" {
			return Value{Kind: RoutePointType, Object: *allowed.Reference}, true, ""
		}
		for _, point := range catalog.BusinessProcesses[index].Route.Points {
			if strings.EqualFold(point.Name, value.Data) {
				return Value{Kind: RoutePointType, Data: point.Name, Object: *allowed.Reference}, true, ""
			}
		}
		return Value{}, false, "value does not name a point of the route"
	case EnumerationType:
		id, err := uuid.Parse(value.Data)
		if err != nil || allowed.Reference == nil {
			return Value{}, false, "invalid enumeration value UUID"
		}
		index, ok := catalog.enumerationByID[*allowed.Reference]
		if !ok {
			return Value{}, false, "unknown enumeration"
		}
		if reason := catalog.checkReferenceObject(value, allowed); reason != "" {
			return Value{}, false, reason
		}
		for _, item := range catalog.Enumerations[index].Values {
			if item.ID == id {
				return Value{Kind: EnumerationType, Data: id.String(), Object: *allowed.Reference}, true, ""
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
