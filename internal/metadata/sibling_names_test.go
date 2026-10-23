package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// TestSiblingNamesAreThePrototypes catches the rule of a name among its
// siblings given away in one of its parts: a name that is no identifier, one
// longer than 255 characters (METADATA-OBJECTS «Пределы на состав»: 255 for
// the name of an object, an attribute and everything named by an
// identifier), a repeated one told apart by case, a refusal that does not say
// which siblings. The rule is written out here, not read from the code.
func TestSiblingNamesAreThePrototypes(t *testing.T) {
	t.Parallel()
	long := "Т" + strings.Repeat("о", 254)
	names := namesWithin(" within the columns")
	for _, test := range []struct {
		name, says string
	}{
		{"Товары", ""},
		{"_100", ""},
		{long, ""},
		{long + "о", "x[3].name must be a valid identifier of at most 255 characters"},
		{"1Товары", "x[4].name must be a valid identifier of at most 255 characters"},
		{"", "x[5].name must be a valid identifier of at most 255 characters"},
		{"ТОВАРЫ", "x[6].name must be unique within the columns"},
	} {
		prefix := "x[" + string(rune('0'+len(names.taken))) + "]"
		issues := names.take(prefix, test.name)
		if got := strings.Join(issues, "; "); got != test.says {
			t.Errorf("%q: %q, want %q", test.name, got, test.says)
		}
	}
}

// TestNamesLongerThanTheCeilingAreRefusedEverywhere catches a place whose
// name of a sibling took any length, as the values of an enumeration, the
// attributes and the table parts of an object did before the eighteen copies
// of the rule became one: each refuses a name of 256 characters and takes one
// of 255.
func TestNamesLongerThanTheCeilingAreRefusedEverywhere(t *testing.T) {
	t.Parallel()
	name := func(length int) string { return "Т" + strings.Repeat("о", length-1) }
	catalog := func(attribute, part string) string {
		return "format: 1\nid: " + catalogID + "\nname: Товары\ntitle: {ru: Товары}\ncode: {type: string, length: 9}\ndescription_length: 25\n" +
			"attributes:\n  - {id: " + uuid.MustNew().String() + ", name: " + attribute + ", title: {ru: А}, types: [{kind: boolean}]}\n" +
			"table_parts:\n  - id: " + uuid.MustNew().String() + "\n    name: " + part + "\n    title: {ru: Ч}\n    attributes:\n" +
			"      - {id: " + uuid.MustNew().String() + ", name: Количество, title: {ru: К}, types: [{kind: boolean}]}\n"
	}
	enumeration := func(value string) string {
		return "format: 1\nid: " + uuid.MustNew().String() + "\nname: Статусы\ntitle: {ru: Статусы}\nvalues: [{id: " +
			uuid.MustNew().String() + ", name: " + value + ", title: {ru: З}}]\n"
	}
	decodeCatalog := func(body string) error {
		_, err := DecodeCatalog("object.yaml", strings.NewReader(body), metadataConfiguration())
		return err
	}
	decodeEnumeration := func(body string) error {
		_, err := DecodeEnumeration("object.yaml", strings.NewReader(body), metadataConfiguration())
		return err
	}
	for place, test := range map[string]struct {
		decode      func(string) error
		taken, long string
		says        string
	}{
		"значение перечисления": {decodeEnumeration, enumeration(name(255)), enumeration(name(256)), "values[0].name"},
		"реквизит справочника":  {decodeCatalog, catalog(name(255), "Строки"), catalog(name(256), "Строки"), "attributes[0].name"},
		"табличная часть":       {decodeCatalog, catalog("Цвет", name(255)), catalog("Цвет", name(256)), "table_parts[0].name"},
	} {
		if err := test.decode(test.taken); err != nil {
			t.Errorf("%s of 255 characters: refused: %v", place, err)
		}
		if err := test.decode(test.long); err == nil || !strings.Contains(err.Error(), test.says+" must be a valid identifier of at most 255 characters") {
			t.Errorf("%s of 256 characters: %v", place, err)
		}
	}
}
