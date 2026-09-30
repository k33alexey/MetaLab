package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The properties in this file were found by the sweep of the model against the
// syntax assistant: each is in the help and was in no structure of ours, so the
// import would have dropped it. The defect of each test is that loss.

// informationRegisterWithStorage reads an information register with a resource
// of the type value storage and a boolean attribute, and the storage lines of
// the resource as given.
func informationRegisterWithStorage(resourceTypes, storage string) error {
	switchID := uuid.MustNew()
	source := "format: 1\nid: " + uuid.MustNew().String() + "\nname: Файлы\ntitle: {ru: Файлы}\nwrite_mode: independent\nperiodicity: none\n" +
		"dimensions:\n  - {id: " + uuid.MustNew().String() + ", name: Владелец, title: {ru: Владелец}, types: [{kind: string, length: 50}]}\n" +
		"resources:\n  - id: " + uuid.MustNew().String() + "\n    name: Данные\n    title: {ru: Данные}\n    types: [" + resourceTypes + "]\n" + strings.ReplaceAll(storage, "SWITCH", switchID.String()) +
		"  - {id: " + uuid.MustNew().String() + ", name: Размер, title: {ru: Размер}, types: [{kind: number, precision: 10}]}\n" +
		"attributes:\n  - {id: " + switchID.String() + ", name: ВоВнешнемХранилище, title: {ru: Во внешнем хранилище}, types: [{kind: boolean}]}\n"
	_, err := DecodeInformationRegister("register.yaml", strings.NewReader(source), demoConfiguration())
	return err
}

// A value of the type value storage may be kept in a binary data storage of the
// base, and with the storage in use a boolean field of the same object may
// decide it row by row. Each rule is held against the setting that would be
// read by nothing: a mode that is neither value, a value that is not a value
// storage, a switch with the storage not in use, a switch that names nothing or
// a field that cannot say yes or no.
func TestBinaryDataStorageIsCarriedAndChecked(t *testing.T) {
	t.Parallel()
	storage := "{kind: value-storage}"
	for name, testCase := range map[string]struct {
		types, lines, refusal string
	}{
		"используется с булевым переключателем": {storage, "    binary_data_storage: use\n    binary_data_storage_field: SWITCH\n", ""},
		"используется без переключателя":        {storage, "    binary_data_storage: use\n", ""},
		"не используется":                       {storage, "    binary_data_storage: dont-use\n", ""},
		"неизвестный режим":                     {storage, "    binary_data_storage: иногда\n", "must be use or dont-use"},
		"не хранилище значения":                 {"{kind: string, length: 10}", "    binary_data_storage: use\n", "belongs to a field of the type value storage"},
		"переключатель без использования":       {storage, "    binary_data_storage: dont-use\n    binary_data_storage_field: SWITCH\n", "needs binary_data_storage: use"},
		"переключатель не на поле объекта":      {storage, "    binary_data_storage: use\n    binary_data_storage_field: " + uuid.MustNew().String() + "\n", "names no attribute of this object"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := informationRegisterWithStorage(testCase.types, testCase.lines)
			if testCase.refusal == "" {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), testCase.refusal) {
				t.Fatalf("error = %v, expected it to say %q", err, testCase.refusal)
			}
		})
	}
}

// The switch is a boolean attribute of the same object. A number attribute
// cannot say yes or no, and a resource is not what the configurator offers -
// it lists the attributes of the object, even for the switch of a resource.
func TestBinaryDataStorageSwitchIsABooleanAttribute(t *testing.T) {
	t.Parallel()
	register := func(switchGroup, switchType string) error {
		switchID := uuid.MustNew()
		source := "format: 1\nid: " + uuid.MustNew().String() + "\nname: Файлы\ntitle: {ru: Файлы}\nwrite_mode: independent\nperiodicity: none\n" +
			"resources:\n  - id: " + uuid.MustNew().String() + "\n    name: Данные\n    title: {ru: Данные}\n    types: [{kind: value-storage}]\n" +
			"    binary_data_storage: use\n    binary_data_storage_field: " + switchID.String() + "\n"
		switcher := "  - {id: " + switchID.String() + ", name: Переключатель, title: {ru: Переключатель}, types: [" + switchType + "]}\n"
		if switchGroup == "resources" {
			source += switcher
		} else {
			source += switchGroup + ":\n" + switcher
		}
		_, err := DecodeInformationRegister("register.yaml", strings.NewReader(source), demoConfiguration())
		return err
	}
	if err := register("attributes", "{kind: boolean}"); err != nil {
		t.Fatalf("a boolean attribute as the switch was refused: %v", err)
	}
	if err := register("attributes", "{kind: number, precision: 10}"); err == nil || !strings.Contains(err.Error(), "must name a boolean field") {
		t.Fatalf("a number attribute as the switch: %v", err)
	}
	if err := register("resources", "{kind: boolean}"); err == nil || !strings.Contains(err.Error(), "names no attribute of this object") {
		t.Fatalf("a boolean resource as the switch: %v", err)
	}
}

// A copy of a field shares nothing with the original, the switch included.
func TestBinaryDataStorageSwitchIsCopied(t *testing.T) {
	t.Parallel()
	id := uuid.MustNew()
	// The expected value is kept apart from the variable the pointer points
	// at: compared with that variable, a shared pointer would change both
	// sides at once and the check would see nothing.
	want := id
	original := Attribute{ID: uuid.MustNew(), Name: "Данные", BinaryDataStorage: UsageUse, BinaryDataStorageField: &id}
	copied := cloneAttribute(original)
	*copied.BinaryDataStorageField = uuid.MustNew()
	if *original.BinaryDataStorageField != want {
		t.Fatal("the copy of a field shared its binary data storage switch with the original")
	}
}

// A numerator carries a comment, as every object of the configuration does.
func TestNumeratorCarriesAComment(t *testing.T) {
	t.Parallel()
	value, err := DecodeNumerator("numerator.yaml", strings.NewReader("format: 1\nid: "+uuid.MustNew().String()+
		"\nname: Сквозная\ntitle: {ru: Сквозная}\ncomment: Общая нумерация счетов\nnumber: {type: string, length: 11, unique: true, periodicity: year}\n"), metadataConfiguration())
	if err != nil || value.Comment != "Общая нумерация счетов" {
		t.Fatalf("numerator = %+v, %v", value, err)
	}
}

// A settings storage carries templates of its own, and they are checked the way
// the templates of every other object are.
func TestSettingsStorageCarriesTemplates(t *testing.T) {
	t.Parallel()
	source := func(kind string) string {
		return "format: 1\nid: " + uuid.MustNew().String() + "\nname: Хранилище\ntitle: {ru: Хранилище}\n" +
			"templates:\n  - {id: " + uuid.MustNew().String() + ", name: Печать, title: {ru: Печать}, kind: " + kind + "}\n"
	}
	value, err := DecodeSettingsStorage("object.yaml", strings.NewReader(source("spreadsheet")), metadataConfiguration())
	if err != nil || len(value.Templates) != 1 || value.Templates[0].Kind != SpreadsheetTemplate {
		t.Fatalf("settings storage = %+v, %v", value, err)
	}
	copied := cloneSettingsStorage(value)
	copied.Templates[0].Title["ru"] = "changed"
	if value.Templates[0].Title["ru"] != "Печать" {
		t.Fatal("the copy of a settings storage shared its templates with the original")
	}
	if _, err := DecodeSettingsStorage("object.yaml", strings.NewReader(source("картинка")), metadataConfiguration()); err == nil {
		t.Fatal("a template of an unknown kind was accepted")
	}
}
