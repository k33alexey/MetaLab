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
// number of languages, a comment, a vendor and a version of any length, a
// mobile client signature of 113 KB as sb keeps it. Defect caught: ceilings
// with no source - 128 for a name, 100 languages, 1024, 512 and 128 for the
// texts, 4096 for the signature - refused at transfer what the prototype
// saves.
func TestRootTakesWhatTheConfiguratorTakes(t *testing.T) {
	t.Parallel()
	value := rootForLimits()
	long := "A" + strings.Repeat("я", MaxNameLength-1)
	value.Name, value.NamePrefix, value.DefaultInterface = long, long, long
	value.Languages[0].Name = long
	for index := range 120 {
		code := fmt.Sprintf("l%d", index)
		value.Languages = append(value.Languages, Language{ID: uuid.MustNew(), Name: "Язык" + code, Title: LocalizedText{"ru": code}, Code: code})
	}
	value.Comment = strings.Repeat("к", 5000)
	value.Languages[0].Comment = strings.Repeat("к", 5000)
	value.Vendor = strings.Repeat("п", 2000)
	value.Version = strings.Repeat("1.", 300) + "1"
	value.MobileClientSignature = strings.Repeat("п", 113143)
	value.UsedMobileFunctionalities = []MobileAnswer{{Name: strings.Repeat("в", 1000), Use: true}}
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
		"имя":                func(value *Project) { value.Name = over },
		"имя языка":          func(value *Project) { value.Languages[0].Name = over },
		"префикс имён":       func(value *Project) { value.NamePrefix = over },
		"основной интерфейс": func(value *Project) { value.DefaultInterface = over },
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

// The purpose of the root is the purpose of a form: one type in the
// prototype, one list here. Defect caught: the root spoke the words of the
// format of 8.3.10, the form those of 8.3.27, and nothing tied the two.
func TestRootPurposeIsTheFormsList(t *testing.T) {
	t.Parallel()
	value := rootForLimits()
	value.UsePurposes = []UsePurpose{PlatformApplicationPurpose, MobilePlatformApplicationPurpose}
	if err := value.Validate(); err != nil {
		t.Fatal(err)
	}
	value.UsePurposes = []UsePurpose{"personal-computer"}
	if err := value.Validate(); err == nil || !strings.Contains(err.Error(), "must be platform-application or mobile-platform-application") {
		t.Fatalf("a word of the old list: refused for another reason or not at all: %v", err)
	}
}
