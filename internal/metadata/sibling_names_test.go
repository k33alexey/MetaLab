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

// TestEveryPartOfARegisterAndAJournalTakesItsNameByTheOneRule catches the rule
// of a sibling's name given away in a part whose names no other test reads -
// a field of an accounting and of a calculation register, a recalculation and
// its dimension, a flag of a chart of accounts, a column of a journal, a
// dimension of a sequence: the negative control of the closure of block 2б
// (25.10.2026) took siblingNames.take away there and nothing turned red. Each
// takes a name of 255 characters, refuses one of 256 and one repeated in
// another case, naming the part.
func TestEveryPartOfARegisterAndAJournalTakesItsNameByTheOneRule(t *testing.T) {
	t.Parallel()
	long := func(length int) string { return "Т" + strings.Repeat("о", length-1) }
	id := func() string { return uuid.MustNew().String() }
	field := func(name string) string {
		return "  - {id: " + id() + ", name: " + name + ", title: {ru: П}, types: [{kind: number, precision: 15, scale: 2}]}\n"
	}
	accounting := func(first, second string) string {
		return "format: 1\nid: " + id() + "\nname: Хозрасчетный\ntitle: {ru: Х}\nchart_of_accounts: " + id() + "\nresources:\n" + field(first) + field(second)
	}
	calculation := func(dimension, resource, recalculation, inner string) string {
		body := strings.Replace(calcRegisterBody, "    name: ФизическоеЛицо\n    title: {ru: Физическое лицо}\n    types: [{kind: catalog, reference: "+calcPeople+"}]\n    base: true",
			"    name: "+dimension+"\n    title: {ru: Физическое лицо}\n    types: [{kind: catalog, reference: "+calcPeople+"}]\n    base: true", 1)
		body = strings.Replace(body, "name: Результат,", "name: "+resource+",", 1)
		body = strings.Replace(body, "    name: ПерерасчетОсновныхНачислений\n", "    name: "+recalculation+"\n", 1)
		return strings.Replace(body, "        name: ФизическоеЛицо\n", "        name: "+inner+"\n", 1)
	}
	flags := func(first, second string) string {
		return chartWithMask("@@@@@", 3, "") + "accounting_flags:\n  - {id: " + id() + ", name: " + first + ", title: {ru: Ф}}\n" +
			"  - {id: " + id() + ", name: " + second + ", title: {ru: Ф}}\n"
	}
	journal := func(first, second string) string {
		column := func(name string) string {
			return "  - {id: " + id() + ", name: " + name + ", title: {ru: Г}, references: [" + aroundFirstParty + "]}\n"
		}
		return "format: 1\nid: " + aroundJournal + "\nname: Журнал\ntitle: {ru: Журнал}\ndocuments: [" + aroundFirstDoc + "]\ncolumns:\n" + column(first) + column(second)
	}
	sequence := func(first, second string) string {
		dimension := func(name string) string {
			return "  - {id: " + id() + ", name: " + name + ", title: {ru: И}, types: [{kind: catalog, reference: " + aroundGoods + "}]}\n"
		}
		return "format: 1\nid: " + aroundSequence + "\nname: Последовательность\ntitle: {ru: П}\ndocuments: [" + aroundFirstDoc + "]\ndimensions:\n" +
			dimension(first) + dimension(second)
	}
	decode := map[string]func(string) error{
		"accounting": func(body string) error {
			_, err := DecodeAccountingRegister("o.yaml", strings.NewReader(body), metadataConfiguration())
			return err
		},
		"calculation": func(body string) error {
			_, err := DecodeCalculationRegister("o.yaml", strings.NewReader(body), metadataConfiguration())
			return err
		},
		"chart": func(body string) error {
			_, err := DecodeChartOfAccounts("o.yaml", strings.NewReader(body), metadataConfiguration())
			return err
		},
		"journal": func(body string) error {
			_, err := DecodeDocumentJournal("o.yaml", strings.NewReader(body), metadataConfiguration())
			return err
		},
		"sequence": func(body string) error {
			_, err := DecodeSequence("o.yaml", strings.NewReader(body), metadataConfiguration())
			return err
		},
	}
	tooLong := " must be a valid identifier of at most 255 characters"
	for place, test := range map[string]struct {
		decoder               string
		taken, long, repeated string
		says, saysRepeated    string
	}{
		"поле регистра бухгалтерии": {"accounting", accounting("Сумма", long(255)), accounting("Сумма", long(256)), accounting("Сумма", "СУММА"),
			"resources[1].name", "resources[1].name must be unique"},
		"измерение регистра расчёта": {"calculation", calculation(long(255), "Результат", "Перерасчет", "ФизическоеЛицо"),
			calculation(long(256), "Результат", "Перерасчет", "ФизическоеЛицо"), calculation("Результат", "РЕЗУЛЬТАТ", "Перерасчет", "ФизическоеЛицо"),
			"dimensions[0].name", "resources[0].name must be unique"},
		"перерасчёт": {"calculation", calculation("ФизическоеЛицо", "Результат", long(255), "ФизическоеЛицо"),
			calculation("ФизическоеЛицо", "Результат", long(256), "ФизическоеЛицо"), "",
			"recalculations[0].name", ""},
		"измерение перерасчёта": {"calculation", calculation("ФизическоеЛицо", "Результат", "Перерасчет", long(255)),
			calculation("ФизическоеЛицо", "Результат", "Перерасчет", long(256)), "",
			"recalculations[0].dimensions[0].name", ""},
		"признак учёта плана счетов": {"chart", flags("Валютный", long(255)), flags("Валютный", long(256)), flags("Валютный", "ВАЛЮТНЫЙ"),
			"accounting_flags[1].name", "accounting_flags[1].name must be unique"},
		"графа журнала": {"journal", journal("Контрагент", long(255)), journal("Контрагент", long(256)), journal("Контрагент", "КОНТРАГЕНТ"),
			"columns[1].name", "columns[1].name must be unique"},
		"измерение последовательности": {"sequence", sequence("Склад", long(255)), sequence("Склад", long(256)), sequence("Склад", "СКЛАД"),
			"dimensions[1].name", "dimensions[1].name must be unique"},
	} {
		run := decode[test.decoder]
		if err := run(test.taken); err != nil {
			t.Errorf("%s of 255 characters refused: %v", place, err)
		}
		if err := run(test.long); err == nil || !strings.Contains(err.Error(), test.says+tooLong) {
			t.Errorf("%s of 256 characters: %v", place, err)
		}
		if test.repeated == "" {
			continue
		}
		if err := run(test.repeated); err == nil || !strings.Contains(err.Error(), test.saysRepeated) {
			t.Errorf("%s repeated in another case: %v", place, err)
		}
	}
}
