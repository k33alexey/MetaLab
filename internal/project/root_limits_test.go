package project

import (
	"fmt"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

func rootForLimits() Project {
	return Project{
		Format: CurrentFormat, ID: uuid.MustNew(), Name: "Demo",
		Title: LocalizedText{"ru": "Демо"}, DefaultLanguage: "ru",
		Languages: []Language{{ID: uuid.MustNew(), Name: "Русский", Title: LocalizedText{"ru": "Русский"}, Code: "ru"}},
	}
}

// The root takes what the configurator takes: a name of 255 characters, any
// number of languages, a comment, a vendor and a version of any length.
// Defect caught: ceilings with no source - 128 for a name, 100 languages,
// 1024, 512 and 128 for the texts - refused at transfer what the prototype
// saves.
func TestRootTakesWhatTheConfiguratorTakes(t *testing.T) {
	t.Parallel()
	value := rootForLimits()
	long := "A" + strings.Repeat("я", MaxNameLength-1)
	value.Name, value.NamePrefix = long, long
	value.Languages[0].Name = long
	for index := range 120 {
		code := fmt.Sprintf("l%d", index)
		value.Languages = append(value.Languages, Language{ID: uuid.MustNew(), Name: "Язык" + code, Title: LocalizedText{"ru": code}, Code: code})
	}
	value.Comment = strings.Repeat("к", 5000)
	value.Languages[0].Comment = strings.Repeat("к", 5000)
	value.Vendor = strings.Repeat("п", 2000)
	value.Version = strings.Repeat("1.", 300) + "1"
	if err := value.Validate(); err != nil {
		t.Fatalf("the root the configurator saves was refused: %v", err)
	}
}

// The one ceiling that stays is the name's, 255 characters, at its edge.
// Defect caught: the ceiling dropped with the others, or set one character
// off.
func TestRootNameStopsAt255(t *testing.T) {
	t.Parallel()
	over := "A" + strings.Repeat("я", MaxNameLength)
	for name, broken := range map[string]func(*Project){
		"имя":          func(value *Project) { value.Name = over },
		"имя языка":    func(value *Project) { value.Languages[0].Name = over },
		"префикс имён": func(value *Project) { value.NamePrefix = over },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			value := rootForLimits()
			broken(&value)
			if err := value.Validate(); err == nil || !strings.Contains(err.Error(), "must not exceed 255 characters") {
				t.Fatalf("refused for another reason or not at all: %v", err)
			}
		})
	}
}

// A text of the root is still one line of printable characters: the ceiling
// went, the shape stayed. Defect caught: the check went with the ceiling.
func TestRootTextsStayLines(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]func(*Project){
		"комментарий":       func(value *Project) { value.Comment = "две\nстроки" },
		"поставщик":         func(value *Project) { value.Vendor = "\a" },
		"версия":            func(value *Project) { value.Version = "1\t2" },
		"комментарий языка": func(value *Project) { value.Languages[0].Comment = "две\nстроки" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			value := rootForLimits()
			broken(&value)
			if err := value.Validate(); err == nil || !strings.Contains(err.Error(), "must be one line of printable characters") {
				t.Fatalf("refused for another reason or not at all: %v", err)
			}
		})
	}
}

// The root has no purpose (owner, 15.10.2026, METADATA-OBJECTS «Только
// управляемое приложение»): with no mobile platform, an application of the
// platform is the one value left and says nothing. A file naming one, in the
// spelling of ML or of either format of the prototype, is refused by the
// strict reading. Defect caught: the purpose taken and carried, as it was
// until 2.360.
func TestRootHasNoPurpose(t *testing.T) {
	t.Parallel()
	head := "format: 1\nid: 018f1f72-3b4c-7d6e-8f90-123456789abc\nname: Demo\ntitle: {ru: Демо}\ndefault_language: ru\n" +
		"languages:\n  - {id: 018f1f72-3b4c-7d6e-8f90-000000000001, name: Русский, title: {ru: Русский}, code: ru}\n"
	if _, err := Decode(strings.NewReader(head)); err != nil {
		t.Fatalf("a root without a purpose: %v", err)
	}
	for _, purposes := range []string{"[platform-application]", "[platform-application, mobile-platform-application]"} {
		if _, err := Decode(strings.NewReader(head + "use_purposes: " + purposes + "\n")); err == nil || !strings.Contains(err.Error(), "field use_purposes not found") {
			t.Errorf("use_purposes %s: %v", purposes, err)
		}
	}
}
