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
    title:
      ru: Русский
    code: ru
  - id: 018f1f72-3b4c-7d6e-8f90-000000000002
    name: Українська
    title:
      uk: Українська
    code: uk
  - id: 018f1f72-3b4c-7d6e-8f90-000000000003
    name: English
    title:
      en: English
    code: en
`

// legacyYAML is a configuration written before languages had identities. Reading it
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
    title:
      ru: Русский
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
			{Name: "Русский", Title: LocalizedText{"RU": ""}, Code: "RU"},
			{Name: "русский", Title: LocalizedText{"RU": "Українська"}, Code: "RU"},
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
		"title must contain at least one translation",
		"languages[0].title.RU must contain 1 to 512 printable characters",
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
		Languages: []Language{{ID: uuid.MustNew(), Name: "Русский", Title: LocalizedText{"ru": "Русский"}, Code: "ru"}},
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
			{ID: uuid.MustNew(), Name: "Українська", Title: LocalizedText{"uk-UA": "Українська"}, Code: "uk-UA"},
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
		t.Fatalf("written configuration does not carry the identity:\n%s", encoded.String())
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
			{ID: shared, Name: "Русский", Title: LocalizedText{"ru": "Русский"}, Code: "ru"},
			{ID: shared, Name: "English", Title: LocalizedText{"en": "English"}, Code: "en"},
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
    title:
      ru: Русский
    code: ru
  - id: 018f1f72-3b4c-7d6e-8f90-000000000003
    name: English
    title:
      en: English
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
			Languages: []Language{{ID: uuid.MustNew(), Name: "Русский", Title: LocalizedText{"ru": "Русский"}, Code: "ru"}},
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

// The defaults of the root survive being written and read back, each one
// still naming what it named. They are the file's own half of the answer:
// whether the object on the other end exists is checked where the whole
// configuration is known.
func TestRootDefaultsSurviveTheFile(t *testing.T) {
	t.Parallel()

	const source = `format: 1
id: 018f1f72-3b4c-7d6e-8f90-123456789abc
name: SalesDemo
title:
  ru: Продажи и склад
default_language: ru
languages:
  - id: 018f1f72-3b4c-7d6e-8f90-000000000001
    name: Русский
    title:
      ru: Русский
    code: ru
default_style: 018f1f72-3b4c-7d6e-8f90-000000000010
default_interface: ОсновнойИнтерфейс
default_roles:
  - 018f1f72-3b4c-7d6e-8f90-000000000020
  - 018f1f72-3b4c-7d6e-8f90-000000000021
default_report_form: 018f1f72-3b4c-7d6e-8f90-000000000030
default_report_settings_form: 018f1f72-3b4c-7d6e-8f90-000000000031
default_report_variant_form: 018f1f72-3b4c-7d6e-8f90-000000000032
default_constants_form: 018f1f72-3b4c-7d6e-8f90-000000000033
default_search_form: 018f1f72-3b4c-7d6e-8f90-000000000034
default_report_appearance_template: 018f1f72-3b4c-7d6e-8f90-000000000040
common_settings_storage: 018f1f72-3b4c-7d6e-8f90-000000000050
reports_user_settings_storage: 018f1f72-3b4c-7d6e-8f90-000000000051
reports_variants_storage: 018f1f72-3b4c-7d6e-8f90-000000000052
dynamic_lists_user_settings_storage: 018f1f72-3b4c-7d6e-8f90-000000000053
form_data_settings_storage: 018f1f72-3b4c-7d6e-8f90-000000000054
`
	value, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	switch {
	case value.DefaultStyle == nil || value.DefaultStyle.String() != "018f1f72-3b4c-7d6e-8f90-000000000010":
		t.Fatalf("the default style came back as %v", value.DefaultStyle)
	case value.DefaultInterface != "ОсновнойИнтерфейс":
		t.Fatalf("the default interface came back as %q", value.DefaultInterface)
	case len(value.DefaultRoles) != 2 ||
		value.DefaultRoles[0].String() != "018f1f72-3b4c-7d6e-8f90-000000000020":
		t.Fatalf("the default roles came back as %v", value.DefaultRoles)
	case value.DefaultReportForm == nil || value.DefaultReportSettingsForm == nil ||
		value.DefaultReportVariantForm == nil || value.DefaultConstantsForm == nil || value.DefaultSearchForm == nil:
		t.Fatalf("one of the five default forms was lost: %+v", value)
	case value.DefaultReportAppearanceTemplate == nil:
		t.Fatal("the report appearance template was lost")
	case value.CommonSettingsStorage == nil || value.ReportsUserSettingsStorage == nil ||
		value.ReportsVariantsStorage == nil || value.DynamicListsUserSettingsStorage == nil ||
		value.FormDataSettingsStorage == nil:
		t.Fatalf("one of the five storages was lost: %+v", value)
	}

	var written bytes.Buffer
	if err := Encode(&written, value); err != nil {
		t.Fatal(err)
	}
	again, err := Decode(bytes.NewReader(written.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(value, again) {
		t.Fatalf("the defaults did not survive a round trip:\n%+v\n%+v", value, again)
	}
}

// A reference has to be a reference, and a role is granted once: the rest of
// the checking belongs to the catalog, but a project that cannot say what it
// points at is broken on its own terms.
func TestRootDefaultsAreCheckedForShape(t *testing.T) {
	t.Parallel()

	base := func() Project {
		return Project{
			Format: CurrentFormat, ID: uuid.MustNew(), Name: "Demo",
			Title: LocalizedText{"ru": "Демо"}, DefaultLanguage: "ru",
			Languages: []Language{{ID: uuid.MustNew(), Name: "Русский", Title: LocalizedText{"ru": "Русский"}, Code: "ru"}},
		}
	}
	role := uuid.MustNew()
	empty := uuid.UUID{}
	for name, broken := range map[string]func(value *Project){
		"пустая ссылка на стиль":       func(value *Project) { value.DefaultStyle = &empty },
		"пустая ссылка на форму":       func(value *Project) { value.DefaultReportForm = &empty },
		"пустая ссылка на хранилище":   func(value *Project) { value.FormDataSettingsStorage = &empty },
		"пустая ссылка на макет":       func(value *Project) { value.DefaultReportAppearanceTemplate = &empty },
		"пустая роль":                  func(value *Project) { value.DefaultRoles = []uuid.UUID{empty} },
		"одна роль дважды":             func(value *Project) { value.DefaultRoles = []uuid.UUID{role, role} },
		"интерфейс не идентификатор":   func(value *Project) { value.DefaultInterface = "Основной интерфейс" },
		"интерфейс с переводом строки": func(value *Project) { value.DefaultInterface = "Основной\nИнтерфейс" },
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

	// A root that fills in none of them is ordinary: every default is the
	// configuration's to leave to the platform.
	if err := base().Validate(); err != nil {
		t.Fatalf("a root without defaults was refused: %v", err)
	}
}

// The settings of the root that name no other object survive the file, and an
// empty one means the platform's own behaviour rather than a question left
// unanswered.
func TestRootSettingsSurviveTheFile(t *testing.T) {
	t.Parallel()

	const source = `format: 1
id: 018f1f72-3b4c-7d6e-8f90-123456789abc
name: SalesDemo
title:
  ru: Продажи и склад
default_language: ru
languages:
  - id: 018f1f72-3b4c-7d6e-8f90-000000000001
    name: Русский
    title:
      ru: Русский
    code: ru
data_lock_control: managed
object_autonumeration: keep
script_variant: russian
name_prefix: бсп
additional_full_text_search_dictionaries:
  - {kind: common-templates, object: 018f1f72-3b4c-7d6e-8f90-000000000070}
  - {kind: constants, object: 018f1f72-3b4c-7d6e-8f90-000000000071}
`
	value, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	switch {
	case value.DataLockControl != ManagedDataLock:
		t.Fatalf("the data lock mode came back as %q", value.DataLockControl)
	case value.ObjectAutonumeration != KeepAutonumber:
		t.Fatalf("the autonumeration mode came back as %q", value.ObjectAutonumeration)
	case value.ScriptVariant != RussianScript:
		t.Fatalf("the script variant came back as %q", value.ScriptVariant)
	case value.NamePrefix != "бсп":
		t.Fatalf("the name prefix came back as %q", value.NamePrefix)
	case len(value.AdditionalFullTextSearchDictionaries) != 2 ||
		value.AdditionalFullTextSearchDictionaries[0].Kind != TemplateDictionary ||
		value.AdditionalFullTextSearchDictionaries[1].Kind != ConstantDictionary:
		t.Fatalf("the dictionaries came back as %+v", value.AdditionalFullTextSearchDictionaries)
	}

	var written bytes.Buffer
	if err := Encode(&written, value); err != nil {
		t.Fatal(err)
	}
	again, err := Decode(bytes.NewReader(written.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(value, again) {
		t.Fatalf("the settings did not survive a round trip:\n%+v\n%+v", value, again)
	}
}

// A mode is a word from a known set: a word outside it is not a stricter
// setting but a question nothing downstream could answer.
func TestRootSettingsAreCheckedForShape(t *testing.T) {
	t.Parallel()

	base := func() Project {
		return Project{
			Format: CurrentFormat, ID: uuid.MustNew(), Name: "Demo",
			Title: LocalizedText{"ru": "Демо"}, DefaultLanguage: "ru",
			Languages: []Language{{ID: uuid.MustNew(), Name: "Русский", Title: LocalizedText{"ru": "Русский"}, Code: "ru"}},
		}
	}
	dictionary := DictionaryReference{Kind: ConstantDictionary, Object: uuid.MustNew()}
	for name, broken := range map[string]func(value *Project){
		"блокировка не из набора":    func(value *Project) { value.DataLockControl = "strict" },
		"автонумерация не из набора": func(value *Project) { value.ObjectAutonumeration = "sometimes" },
		"вариант языка не из набора": func(value *Project) { value.ScriptVariant = "esperanto" },
		"префикс с пробелом":         func(value *Project) { value.NamePrefix = "бсп " },
		"словарь неизвестного вида": func(value *Project) {
			value.AdditionalFullTextSearchDictionaries = []DictionaryReference{{Kind: "catalogs", Object: uuid.MustNew()}}
		},
		"словарь без объекта": func(value *Project) {
			value.AdditionalFullTextSearchDictionaries = []DictionaryReference{{Kind: ConstantDictionary}}
		},
		"один словарь дважды": func(value *Project) {
			value.AdditionalFullTextSearchDictionaries = []DictionaryReference{dictionary, dictionary}
		},
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

	// The same object named once as a template and once as a constant is two
	// dictionaries, not one named twice.
	twice := base()
	object := uuid.MustNew()
	twice.AdditionalFullTextSearchDictionaries = []DictionaryReference{
		{Kind: TemplateDictionary, Object: object}, {Kind: ConstantDictionary, Object: object},
	}
	if err := twice.Validate(); err != nil {
		t.Fatalf("two dictionaries of different kinds were refused: %v", err)
	}
	if err := base().Validate(); err != nil {
		t.Fatalf("a root without settings was refused: %v", err)
	}
}

// The modes of the root survive the file. They act on nothing in ML, which is
// precisely why the only thing that can go wrong with them is losing them.
func TestRootModesSurviveTheFile(t *testing.T) {
	t.Parallel()

	const source = `format: 1
id: 018f1f72-3b4c-7d6e-8f90-123456789abc
name: SalesDemo
title:
  ru: Продажи и склад
default_language: ru
languages:
  - id: 018f1f72-3b4c-7d6e-8f90-000000000001
    name: Русский
    title:
      ru: Русский
    code: ru
default_run_mode: managed-application
use_purposes: [personal-computer, mobile-device]
use_managed_forms_in_ordinary_application: true
modality_use: use-with-warnings
synchronous_platform_extension_call_use: use
synchronous_extension_call_use: do-not-use
interface_compatibility: taxi-allow-version-8-2
main_window_mode: normal
database_tablespaces_use: do-not-use
binary_data_storage: use
binary_data_block_storage_use: do-not-use
compatibility_version: 8.3.21
extension_compatibility_version: 8.3.27
include_help_in_contents: true
`
	value, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	switch {
	case value.DefaultRunMode != ManagedApplicationRunMode:
		t.Fatalf("the run mode came back as %q", value.DefaultRunMode)
	case len(value.UsePurposes) != 2 || value.UsePurposes[0] != PersonalComputerPurpose:
		t.Fatalf("the purposes came back as %+v", value.UsePurposes)
	case !value.UseManagedFormsInOrdinaryApplication || value.UseOrdinaryFormsInManagedApplication:
		t.Fatalf("the form flags came back as %+v", value)
	case value.ModalityUse != UsedWithWarning || value.SynchronousPlatformExtensionCallUse != Used ||
		value.SynchronousExtensionCallUse != NotUsed:
		t.Fatalf("a use mode was lost: %+v", value)
	case value.InterfaceCompatibility != TaxiAllowVersion82Interface || value.MainWindowMode != NormalWindow:
		t.Fatalf("an interface mode was lost: %+v", value)
	case value.DatabaseTablespacesUse != NotUsed || value.BinaryDataStorage != Used ||
		value.BinaryDataBlockStorageUse != NotUsed:
		t.Fatalf("a storage mode was lost: %+v", value)
	case value.CompatibilityVersion != "8.3.21" || value.ExtensionCompatibilityVersion != "8.3.27":
		t.Fatalf("a compatibility version was lost: %+v", value)
	case !value.IncludeHelpInContents:
		t.Fatal("the help flag was lost")
	}

	var written bytes.Buffer
	if err := Encode(&written, value); err != nil {
		t.Fatal(err)
	}
	again, err := Decode(bytes.NewReader(written.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(value, again) {
		t.Fatalf("the modes did not survive a round trip:\n%+v\n%+v", value, again)
	}
}

// A mode is checked for being a mode at all. The compatibility version is the
// one that is not checked against a list: the list is another platform's
// release history, and refusing a release we had not heard of would lose
// exactly what carrying the mode was for.
func TestRootModesAreCheckedForShape(t *testing.T) {
	t.Parallel()

	base := func() Project {
		return Project{
			Format: CurrentFormat, ID: uuid.MustNew(), Name: "Demo",
			Title: LocalizedText{"ru": "Демо"}, DefaultLanguage: "ru",
			Languages: []Language{{ID: uuid.MustNew(), Name: "Русский", Title: LocalizedText{"ru": "Русский"}, Code: "ru"}},
		}
	}
	for name, broken := range map[string]func(value *Project){
		"режим запуска не из набора": func(value *Project) { value.DefaultRunMode = "web" },
		"назначение не из набора":    func(value *Project) { value.UsePurposes = []UsePurpose{"watch"} },
		"назначение дважды": func(value *Project) {
			value.UsePurposes = []UsePurpose{PersonalComputerPurpose, PersonalComputerPurpose}
		},
		"модальность не из набора":              func(value *Project) { value.ModalityUse = "maybe" },
		"предупреждение там, где его нет":       func(value *Project) { value.DatabaseTablespacesUse = UsedWithWarning },
		"двоичные данные с предупреждением":     func(value *Project) { value.BinaryDataStorage = UsedWithWarning },
		"совместимость интерфейса не из набора": func(value *Project) { value.InterfaceCompatibility = "версия 7.7" },
		"окно не из набора":                     func(value *Project) { value.MainWindowMode = "widget" },
		"версия не версия":                      func(value *Project) { value.CompatibilityVersion = "восьмая" },
		"версия из одного числа":                func(value *Project) { value.ExtensionCompatibilityVersion = "8" },
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

	// A release we have never heard of is accepted, because the release
	// history is not ours to enumerate.
	future := base()
	future.CompatibilityVersion = "8.4.99"
	if err := future.Validate(); err != nil {
		t.Fatalf("an unknown release was refused: %v", err)
	}
}

// What the root says about a mobile application survives the file. ML builds
// no mobile application, so losing it is the only thing that can go wrong.
func TestRootMobileApplicationSurvivesTheFile(t *testing.T) {
	t.Parallel()

	const source = `format: 1
id: 018f1f72-3b4c-7d6e-8f90-123456789abc
name: SalesDemo
title:
  ru: Продажи и склад
default_language: ru
languages:
  - id: 018f1f72-3b4c-7d6e-8f90-000000000001
    name: Русский
    title:
      ru: Русский
    code: ru
used_mobile_functionalities: [Звонки, Геолокация]
required_mobile_permissions: [Камера]
mobile_application_urls: [e1cib/navigationpoint/sales]
allowed_share_request_types: [image/png, application/pdf]
mobile_client_signature: подпись
standalone_configuration_content:
  - {kind: catalogs, object: 018f1f72-3b4c-7d6e-8f90-000000000080}
  - {kind: documents, object: 018f1f72-3b4c-7d6e-8f90-000000000081}
standalone_configuration_restriction_roles:
  - 018f1f72-3b4c-7d6e-8f90-000000000082
`
	value, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	switch {
	case len(value.UsedMobileFunctionalities) != 2 || value.UsedMobileFunctionalities[0] != "Звонки":
		t.Fatalf("the functionalities came back as %+v", value.UsedMobileFunctionalities)
	case len(value.RequiredMobilePermissions) != 1 || len(value.MobileApplicationURLs) != 1 ||
		len(value.AllowedShareRequestTypes) != 2:
		t.Fatalf("a list was lost: %+v", value)
	case value.MobileClientSignature != "подпись":
		t.Fatalf("the signature came back as %q", value.MobileClientSignature)
	case len(value.StandaloneConfigurationContent) != 2 ||
		value.StandaloneConfigurationContent[0].Kind != "catalogs":
		t.Fatalf("the standalone content came back as %+v", value.StandaloneConfigurationContent)
	case len(value.StandaloneConfigurationRestrictionRoles) != 1:
		t.Fatalf("the restriction roles came back as %+v", value.StandaloneConfigurationRestrictionRoles)
	}

	var written bytes.Buffer
	if err := Encode(&written, value); err != nil {
		t.Fatal(err)
	}
	again, err := Decode(bytes.NewReader(written.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(value, again) {
		t.Fatalf("the mobile application did not survive a round trip:\n%+v\n%+v", value, again)
	}
}

// A word we have never heard of is kept: the lists belong to another platform
// and to the mobile operating systems, and both grow without us. What is
// refused is a list entry that names nothing at all.
func TestRootMobileApplicationIsCheckedForShape(t *testing.T) {
	t.Parallel()

	base := func() Project {
		return Project{
			Format: CurrentFormat, ID: uuid.MustNew(), Name: "Demo",
			Title: LocalizedText{"ru": "Демо"}, DefaultLanguage: "ru",
			Languages: []Language{{ID: uuid.MustNew(), Name: "Русский", Title: LocalizedText{"ru": "Русский"}, Code: "ru"}},
		}
	}
	object := ObjectReference{Kind: "catalogs", Object: uuid.MustNew()}
	role := uuid.MustNew()
	for name, broken := range map[string]func(value *Project){
		"пустая возможность":     func(value *Project) { value.UsedMobileFunctionalities = []string{""} },
		"разрешение с пробелами": func(value *Project) { value.RequiredMobilePermissions = []string{" Камера"} },
		"одна ссылка дважды":     func(value *Project) { value.MobileApplicationURLs = []string{"a", "a"} },
		"состав без вида": func(value *Project) {
			value.StandaloneConfigurationContent = []ObjectReference{{Object: uuid.MustNew()}}
		},
		"состав без объекта":      func(value *Project) { value.StandaloneConfigurationContent = []ObjectReference{{Kind: "catalogs"}} },
		"один объект дважды":      func(value *Project) { value.StandaloneConfigurationContent = []ObjectReference{object, object} },
		"роль ограничения дважды": func(value *Project) { value.StandaloneConfigurationRestrictionRoles = []uuid.UUID{role, role} },
		"пустая роль ограничения": func(value *Project) { value.StandaloneConfigurationRestrictionRoles = []uuid.UUID{{}} },
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

	// A possibility nobody enumerated for us is not a mistake.
	unknown := base()
	unknown.UsedMobileFunctionalities = []string{"ВозможностьКоторойМыНеЗнаем"}
	unknown.AllowedShareRequestTypes = []string{"application/x-невиданное"}
	if err := unknown.Validate(); err != nil {
		t.Fatalf("an unknown word was refused: %v", err)
	}
}

// A language is an object of the configuration, and its synonym is localized
// like every other: the list of languages is exactly the place where a reader
// sees two of them at once.
func TestLanguageCarriesALocalizedSynonymAndAComment(t *testing.T) {
	t.Parallel()

	const source = `format: 1
id: 018f1f72-3b4c-7d6e-8f90-123456789abc
name: SalesDemo
title:
  ru: Продажи и склад
default_language: ru
languages:
  - id: 018f1f72-3b4c-7d6e-8f90-000000000001
    name: Русский
    title:
      ru: Русский
      en: Russian
    comment: Язык, на котором ведётся учёт
    code: ru
  - id: 018f1f72-3b4c-7d6e-8f90-000000000002
    name: English
    title:
      ru: Английский
      en: English
    code: en
`
	value, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	switch {
	case len(value.Languages) != 2:
		t.Fatalf("a language was lost: %+v", value.Languages)
	case value.Languages[0].Title["en"] != "Russian" || value.Languages[0].Title["ru"] != "Русский":
		t.Fatalf("the synonym of a language came back as %+v", value.Languages[0].Title)
	case value.Languages[0].Comment != "Язык, на котором ведётся учёт":
		t.Fatalf("the comment of a language came back as %q", value.Languages[0].Comment)
	case value.Languages[1].Comment != "":
		t.Fatalf("a comment appeared from nowhere: %q", value.Languages[1].Comment)
	}

	var written bytes.Buffer
	if err := Encode(&written, value); err != nil {
		t.Fatal(err)
	}
	again, err := Decode(bytes.NewReader(written.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(value, again) {
		t.Fatalf("the languages did not survive a round trip:\n%+v\n%+v", value.Languages, again.Languages)
	}
}

// The synonym of a language is checked against the very list it belongs to.
// That is not a circle: the languages and their synonyms lie in one file and
// are read together, so by the time a synonym is checked the list is whole.
func TestLanguageSynonymIsCheckedAgainstTheLanguages(t *testing.T) {
	t.Parallel()

	base := func() Project {
		return Project{
			Format: CurrentFormat, ID: uuid.MustNew(), Name: "Demo",
			Title: LocalizedText{"ru": "Демо"}, DefaultLanguage: "ru",
			Languages: []Language{
				{ID: uuid.MustNew(), Name: "Русский", Title: LocalizedText{"ru": "Русский"}, Code: "ru"},
			},
		}
	}
	unknown := base()
	unknown.Languages[0].Title = LocalizedText{"de": "Russisch"}
	if err := unknown.Validate(); err == nil {
		t.Fatal("a synonym written in a language nobody configured was accepted")
	}

	empty := base()
	empty.Languages[0].Title = nil
	if err := empty.Validate(); err == nil {
		t.Fatal("a language without a synonym was accepted")
	}

	noisy := base()
	noisy.Languages[0].Comment = "строка\nс переводом"
	if err := noisy.Validate(); err == nil {
		t.Fatal("a comment with a line break was accepted")
	}

	// Two languages naming each other in both is ordinary, and the point of
	// making the synonym localized at all.
	both := base()
	both.Languages = []Language{
		{ID: uuid.MustNew(), Name: "Русский", Title: LocalizedText{"ru": "Русский", "en": "Russian"}, Code: "ru"},
		{ID: uuid.MustNew(), Name: "English", Title: LocalizedText{"ru": "Английский", "en": "English"}, Code: "en"},
	}
	if err := both.Validate(); err != nil {
		t.Fatalf("two languages naming each other were refused: %v", err)
	}
}
