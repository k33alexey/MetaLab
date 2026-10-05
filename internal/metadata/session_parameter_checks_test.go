package metadata

import (
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A string assigned to a session parameter becomes the value the parameter's
// types say it is: the identifier of an enumeration value becomes that value,
// an identifier becomes an identifier - and only where the parameter allows
// one. A parameter that holds strings alone keeps the same text as a string.
//
// Defect caught: the enumeration value left as text; an identifier made out
// of the text of a parameter that only holds strings.
func TestSessionParameterTakesTheKindItsTypesAllow(t *testing.T) {
	t.Parallel()
	enumeration, first := uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{
		Enumerations:    []Enumeration{{ID: enumeration, Name: "Склады", Values: []EnumerationValue{{ID: first, Name: "Первый"}}}},
		enumerationByID: map[uuid.UUID]int{enumeration: 0},
	}
	runtime := &Runtime{catalog: catalog}
	text := Type{Kind: StringType, Length: 100}
	for _, test := range []struct {
		name  string
		types []Type
		data  string
		want  TypeKind
	}{
		{"значение перечисления", []Type{{Kind: EnumerationType, Reference: &enumeration}, text}, first.String(), EnumerationType},
		{"идентификатор", []Type{{Kind: UUIDType}, text}, first.String(), UUIDType},
		{"только строка", []Type{text}, first.String(), StringType},
	} {
		value, err := runtime.valueFromBSLForSessionParameter(SessionParameter{Name: "Параметр", Types: test.types}, bytecode.String(test.data))
		if err != nil || value.Kind != test.want {
			t.Fatalf("%s: %+v, %v - want %s", test.name, value, err, test.want)
		}
	}
}
