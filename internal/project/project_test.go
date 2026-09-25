package project

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

const validYAML = `format: 1
id: 018f1f72-3b4c-7d6e-8f90-123456789abc
name: SalesDemo
title:
  ru: Продажи и склад
default_language: ru
languages:
  - id: 018f1f72-3b4c-7d6e-8f90-000000000001
    name: Русский
    title: Русский
    code: ru
  - id: 018f1f72-3b4c-7d6e-8f90-000000000002
    name: Українська
    title: Українська
    code: uk
  - id: 018f1f72-3b4c-7d6e-8f90-000000000003
    name: English
    title: English
    code: en
`

// legacyYAML is a manifest written before languages had identities. Reading it
// must keep working - the identity is the platform's own bookkeeping, and the
// data it addresses (translations) is keyed by code, not by it.
const legacyYAML = `format: 1
id: 018f1f72-3b4c-7d6e-8f90-123456789abc
name: SalesDemo
title:
  ru: Продажи и склад
default_language: ru
languages:
  - name: Русский
    title: Русский
    code: ru
`

func TestDecodeValidProject(t *testing.T) {
	t.Parallel()

	value, err := Decode(strings.NewReader(validYAML))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if value.Name != "SalesDemo" {
		t.Fatalf("Project.Name = %q, want SalesDemo", value.Name)
	}
	if value.ID.String() != "018f1f72-3b4c-7d6e-8f90-123456789abc" {
		t.Fatalf("Project.ID = %s", value.ID)
	}
	if len(value.Languages) != 3 {
		t.Fatalf("len(Project.Languages) = %d, want 3", len(value.Languages))
	}
}

func TestEncodeIsStable(t *testing.T) {
	t.Parallel()

	value, err := Decode(strings.NewReader(validYAML))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	var first bytes.Buffer
	if err := Encode(&first, value); err != nil {
		t.Fatalf("first Encode() error = %v", err)
	}
	firstYAML := first.String()

	restored, err := Decode(strings.NewReader(firstYAML))
	if err != nil {
		t.Fatalf("second Decode() error = %v", err)
	}

	var second bytes.Buffer
	if err := Encode(&second, restored); err != nil {
		t.Fatalf("second Encode() error = %v", err)
	}

	if firstYAML != second.String() {
		t.Fatalf("serialization is not stable:\nfirst:\n%s\nsecond:\n%s", firstYAML, second.String())
	}
	if firstYAML != validYAML {
		t.Fatalf("Encode() output differs from canonical YAML:\ngot:\n%s\nwant:\n%s", firstYAML, validYAML)
	}
}

func TestDecodeRejectsUnknownField(t *testing.T) {
	t.Parallel()

	input := validYAML + "unexpected: value\n"
	if _, err := Decode(strings.NewReader(input)); err == nil {
		t.Fatal("Decode() returned no error for unknown field")
	}
}

func TestDecodeRejectsMultipleDocuments(t *testing.T) {
	t.Parallel()

	input := validYAML + "---\n" + validYAML
	if _, err := Decode(strings.NewReader(input)); err == nil {
		t.Fatal("Decode() returned no error for multiple documents")
	}
}

func TestDecodeReportsNamedSourceAndLine(t *testing.T) {
	t.Parallel()

	_, err := DecodeSource("metadata/catalogs/example.yaml", strings.NewReader("format: 1\nunknown: true\n"))
	if err == nil || !strings.Contains(err.Error(), "metadata/catalogs/example.yaml") || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("DecodeSource() error = %v", err)
	}
}

func TestDecodeRejectsOversizedDocument(t *testing.T) {
	t.Parallel()

	reader := io.LimitReader(strings.NewReader(strings.Repeat("x", MaxYAMLDocumentBytes+1)), MaxYAMLDocumentBytes+1)
	if _, err := Decode(reader); !errors.Is(err, ErrYAMLDocumentTooLarge) {
		t.Fatalf("Decode() error = %v, want ErrYAMLDocumentTooLarge", err)
	}
}

func TestUnsupportedFormatIsMatchableAndStructured(t *testing.T) {
	t.Parallel()

	value, err := Decode(strings.NewReader(strings.Replace(validYAML, "format: 1", "format: 2", 1)))
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("Decode() value=%+v error=%v, want ErrUnsupportedFormat", value, err)
	}
	var validation *ValidationError
	if !errors.As(err, &validation) || len(validation.Issues) != 1 || validation.Issues[0].Path != "format" {
		t.Fatalf("validation error = %#v", validation)
	}
}

func TestValidationDiagnosticIncludesNamedSource(t *testing.T) {
	t.Parallel()

	input := strings.Replace(validYAML, "  ru: Продажи и склад", "  ru: ''", 1)
	_, err := DecodeSource("configuration.yaml", strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "validate configuration.yaml") || !strings.Contains(err.Error(), "title") {
		t.Fatalf("DecodeSource() error = %v", err)
	}
}

func TestValidateReportsAllProblems(t *testing.T) {
	t.Parallel()

	value := Project{
		Format:          2,
		Name:            "1 invalid",
		DefaultLanguage: "de",
		Languages: []Language{
			{Name: "Русский", Title: "", Code: "RU"},
			{Name: "русский", Title: "Українська", Code: "RU"},
		},
	}

	err := value.Validate()
	if err == nil {
		t.Fatal("Validate() returned no error")
	}

	for _, expected := range []string{
		"format must be 1",
		"id must be a non-zero UUID",
		"name must start with a letter",
		"title must contain 1 to 512 printable characters",
		"languages[0].title must contain 1 to 512 printable characters",
		"languages[0].code",
		"languages[1].name must be unique",
		"languages[1].code must be unique",
		"default_language",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("Validate() error = %q, want substring %q", err, expected)
		}
	}
}

func TestValidateRejectsUnboundedOrControlText(t *testing.T) {
	t.Parallel()

	value := Project{
		Format: CurrentFormat, ID: uuid.MustNew(), Name: "A" + strings.Repeat("b", 128),
		Title: LocalizedText{"ru": "Unsafe\nTitle"}, DefaultLanguage: "ru",
		Languages: []Language{{ID: uuid.MustNew(), Name: "Русский", Title: "Русский", Code: "ru"}},
	}
	err := value.Validate()
	if err == nil || !strings.Contains(err.Error(), "name must not exceed 128") || !strings.Contains(err.Error(), "title.ru must contain") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateAcceptsUnicodeIdentifiersAndRegion(t *testing.T) {
	t.Parallel()

	value := Project{
		Format:          CurrentFormat,
		ID:              uuid.MustNew(),
		Name:            "Торгівля2",
		Title:           LocalizedText{"uk-UA": "Торгівля"},
		DefaultLanguage: "uk-UA",
		Languages: []Language{
			{ID: uuid.MustNew(), Name: "Українська", Title: "Українська", Code: "uk-UA"},
		},
	}

	if err := value.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestEncodeRejectsInvalidProject(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	if err := Encode(&output, Project{}); err == nil {
		t.Fatal("Encode() returned no error for invalid project")
	}
}

func TestEncodeReportsWriterError(t *testing.T) {
	t.Parallel()

	value, err := Decode(strings.NewReader(validYAML))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if err := Encode(failingWriter{}, value); err == nil {
		t.Fatal("Encode() returned no writer error")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

// Old manifests gain identities when they are read, and keep them from the
// moment they are written back: nothing has to be migrated by hand, and the
// project stops being the one metadata object without a stable identity.
func TestLanguagesWithoutIdentitiesGainThemOnDecode(t *testing.T) {
	t.Parallel()
	value, err := Decode(strings.NewReader(legacyYAML))
	if err != nil {
		t.Fatal(err)
	}
	if len(value.Languages) != 1 || value.Languages[0].ID.IsZero() {
		t.Fatalf("decoded languages = %+v", value.Languages)
	}
	var encoded bytes.Buffer
	if err := Encode(&encoded, value); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(encoded.String(), "id: "+value.Languages[0].ID.String()) {
		t.Fatalf("written manifest does not carry the identity:\n%s", encoded.String())
	}
	restored, err := Decode(strings.NewReader(encoded.String()))
	if err != nil {
		t.Fatal(err)
	}
	if restored.Languages[0].ID != value.Languages[0].ID {
		t.Fatalf("identity changed on the next read: %s -> %s", value.Languages[0].ID, restored.Languages[0].ID)
	}
}

// Reading the same file twice must produce the same project. Publication
// compares two independently read snapshots of one project, so an identity
// invented per read would make a project differ from itself and every save
// would look like a change.
func TestLanguageIdentitiesAreDerivedNotInvented(t *testing.T) {
	t.Parallel()
	first, err := Decode(strings.NewReader(legacyYAML))
	if err != nil {
		t.Fatal(err)
	}
	second, err := Decode(strings.NewReader(legacyYAML))
	if err != nil {
		t.Fatal(err)
	}
	if first.Languages[0].ID != second.Languages[0].ID {
		t.Fatalf("two reads produced different identities: %s vs %s", first.Languages[0].ID, second.Languages[0].ID)
	}
	// Derived from the project and the code, so another project does not get
	// the same identity for its own Russian.
	other := strings.Replace(legacyYAML, "018f1f72-3b4c-7d6e-8f90-123456789abc", "018f1f72-3b4c-7d6e-8f90-cba987654321", 1)
	elsewhere, err := Decode(strings.NewReader(other))
	if err != nil {
		t.Fatal(err)
	}
	if elsewhere.Languages[0].ID == first.Languages[0].ID {
		t.Fatal("two projects derived the same identity for their own language")
	}
}

func TestValidateRejectsDuplicateLanguageIdentities(t *testing.T) {
	t.Parallel()
	shared := uuid.MustNew()
	value := Project{
		Format: CurrentFormat, ID: uuid.MustNew(), Name: "Demo", Title: LocalizedText{"ru": "Demo"}, DefaultLanguage: "ru",
		Languages: []Language{
			{ID: shared, Name: "Русский", Title: "Русский", Code: "ru"},
			{ID: shared, Name: "English", Title: "English", Code: "en"},
		},
	}
	if err := value.Validate(); err == nil {
		t.Fatal("two languages with one identity were accepted")
	}
}

// The root is the configuration itself, so it says what the configuration is,
// who made it and where to read about it. None of it was in the model, so a
// real configuration lost all of it at once.
func TestRootCarriesWhatTheConfigurationSaysAboutItself(t *testing.T) {
	t.Parallel()

	const source = `format: 1
id: 018f1f72-3b4c-7d6e-8f90-123456789abc
name: SalesDemo
title:
  ru: Продажи и склад
  en: Sales and warehouse
comment: Для целей тестирования
default_language: ru
languages:
  - id: 018f1f72-3b4c-7d6e-8f90-000000000001
    name: Русский
    title: Русский
    code: ru
  - id: 018f1f72-3b4c-7d6e-8f90-000000000003
    name: English
    title: English
    code: en
brief_information:
  ru: Торговля со склада
detailed_information:
  ru: Закупки, продажи и остатки на складах
vendor: MetaLab
version: 1.0.0.1
copyright:
  ru: © MetaLab, 2026
vendor_address:
  ru: https://example.org/vendor
information_address:
  ru: https://example.org/configuration
update_catalog_address:
  ru: https://example.org/updates
`
	value, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	switch {
	case value.Title["ru"] != "Продажи и склад" || value.Title["en"] != "Sales and warehouse":
		t.Fatalf("the synonym came back as %+v", value.Title)
	case value.Comment != "Для целей тестирования":
		t.Fatalf("comment = %q", value.Comment)
	case value.BriefInformation["ru"] != "Торговля со склада":
		t.Fatalf("brief information = %+v", value.BriefInformation)
	case value.DetailedInformation["ru"] != "Закупки, продажи и остатки на складах":
		t.Fatalf("detailed information = %+v", value.DetailedInformation)
	case value.Vendor != "MetaLab" || value.Version != "1.0.0.1":
		t.Fatalf("vendor = %q, version = %q", value.Vendor, value.Version)
	case value.Copyright["ru"] != "© MetaLab, 2026":
		t.Fatalf("copyright = %+v", value.Copyright)
	case value.VendorAddress["ru"] != "https://example.org/vendor":
		t.Fatalf("vendor address = %+v", value.VendorAddress)
	case value.InformationAddress["ru"] != "https://example.org/configuration":
		t.Fatalf("information address = %+v", value.InformationAddress)
	case value.UpdateCatalogAddress["ru"] != "https://example.org/updates":
		t.Fatalf("update catalog address = %+v", value.UpdateCatalogAddress)
	}

	// Writing it back and reading it again gives the same project: a property
	// that survives one round trip but not two is a property that is lost the
	// moment anyone edits the file.
	var written bytes.Buffer
	if err := Encode(&written, value); err != nil {
		t.Fatal(err)
	}
	again, err := Decode(bytes.NewReader(written.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(value, again) {
		t.Fatalf("the root did not survive a round trip:\n%+v\n%+v", value, again)
	}
}

// Every text of the root is checked against the languages the root itself
// declares. A synonym in a language the project does not have would be a
// translation nobody can ever read.
func TestRootTextsAreCheckedAgainstTheProjectLanguages(t *testing.T) {
	t.Parallel()

	base := func() Project {
		return Project{
			Format: CurrentFormat, ID: uuid.MustNew(), Name: "Demo",
			Title: LocalizedText{"ru": "Демо"}, DefaultLanguage: "ru",
			Languages: []Language{{ID: uuid.MustNew(), Name: "Русский", Title: "Русский", Code: "ru"}},
		}
	}
	if err := base().Validate(); err != nil {
		t.Fatalf("a correct root was refused: %v", err)
	}
	for name, broken := range map[string]func(value *Project){
		"синонима нет вовсе":              func(value *Project) { value.Title = nil },
		"синоним на языке, которого нет":  func(value *Project) { value.Title = LocalizedText{"de": "Demo"} },
		"авторские права на чужом языке":  func(value *Project) { value.Copyright = LocalizedText{"de": "©"} },
		"адрес с управляющим символом":    func(value *Project) { value.InformationAddress = LocalizedText{"ru": "http://a\nb"} },
		"краткая информация пуста":        func(value *Project) { value.BriefInformation = LocalizedText{"ru": "  "} },
		"версия с управляющим символом":   func(value *Project) { value.Version = "1.0\n0" },
		"комментарий с управляющим сим-м": func(value *Project) { value.Comment = "первая\nвторая" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			value := base()
			broken(&value)
			if err := value.Validate(); err == nil {
				t.Fatal("the root was accepted")
			}
		})
	}

	// What a configuration chose not to fill in is not an error: only the
	// synonym has to be there.
	value := base()
	if err := value.Validate(); err != nil {
		t.Fatalf("a root with only a synonym was refused: %v", err)
	}
}
