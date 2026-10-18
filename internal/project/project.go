// Package project defines the source model of an ML Project.
package project

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/uuid"
	"go.yaml.in/yaml/v3"
)

const (
	// CurrentFormat is the latest supported ML Project configuration format.
	CurrentFormat = 1
	// MaxNameLength is the longest name of anything named by an identifier -
	// the configuration, a language, an object, a field: the designer takes
	// 255 characters (owner, 01.10.2026), the configurations being moved go up
	// to 96.
	MaxNameLength = 255
)

var (
	// ErrUnsupportedFormat identifies a configuration written in an unsupported format version.
	ErrUnsupportedFormat = errors.New("unsupported ML Project format")
)

// ValidationIssue points to one invalid configuration field.
type ValidationIssue struct {
	Path    string
	Message string
}

// ValidationError contains all independently detectable configuration problems.
type ValidationError struct {
	Issues            []ValidationIssue
	unsupportedFormat bool
}

func (validation *ValidationError) Error() string {
	problems := make([]string, 0, len(validation.Issues))
	for _, issue := range validation.Issues {
		problems = append(problems, issue.Path+" "+issue.Message)
	}
	return "invalid ML Project: " + strings.Join(problems, "; ")
}

// Is makes future or obsolete configuration versions programmatically distinguishable.
func (validation *ValidationError) Is(target error) bool {
	return target == ErrUnsupportedFormat && validation.unsupportedFormat
}

// Project is the configuration root, stored in configuration.yaml.
//
// It is the root itself and not a configuration standing beside it. Everything the
// configuration used to hold - the identifier, the name, the synonym, the default
// language and the list of languages - are properties of the root, and giving
// the root an identifier of its own would be two identifiers for one thing:
// they diverge, and the only question is when. The identifier written into a
// database when a project is first applied, and checked before every later
// one, is this one.
//
// The languages are read before anything else because every synonym in the
// configuration is checked against them, and the root is what declares them.
type Project struct {
	Format int       `yaml:"format" json:"format"`
	ID     uuid.UUID `yaml:"id" json:"id"`
	Name   string    `yaml:"name" json:"name"`
	// Title is the root's synonym - what a person reads where the name is an
	// identifier. It is localized like every other object's synonym: a
	// configuration whose synonym is written in two languages would otherwise
	// lose one of them without a word.
	Title           LocalizedText `yaml:"title" json:"title"`
	Comment         string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	DefaultLanguage string        `yaml:"default_language" json:"defaultLanguage"`
	Languages       []Language    `yaml:"languages" json:"languages"`
	// BriefInformation and DetailedInformation are what the application says
	// about itself: the first in a line, the second at length.
	BriefInformation    LocalizedText `yaml:"brief_information,omitempty" json:"briefInformation,omitempty"`
	DetailedInformation LocalizedText `yaml:"detailed_information,omitempty" json:"detailedInformation,omitempty"`
	// Vendor and Version are not localized: who made it and which release it
	// is are the same in every language, and translating a version number
	// would make two releases look like one.
	Vendor  string `yaml:"vendor,omitempty" json:"vendor,omitempty"`
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
	// Copyright is read by a person and is localized for the same reason the
	// synonym is.
	Copyright LocalizedText `yaml:"copyright,omitempty" json:"copyright,omitempty"`
	// The three addresses are localized because each may lead to a different
	// page per language: where to read about who made this, where to read
	// about the configuration itself, and where its updates are published.
	VendorAddress        LocalizedText `yaml:"vendor_address,omitempty" json:"vendorAddress,omitempty"`
	InformationAddress   LocalizedText `yaml:"information_address,omitempty" json:"informationAddress,omitempty"`
	UpdateCatalogAddress LocalizedText `yaml:"update_catalog_address,omitempty" json:"updateCatalogAddress,omitempty"`
	// What follows are the defaults of the root: what the application opens,
	// draws with and saves into when nothing nearer says otherwise. Each of
	// them names another object of the configuration, so each is stored by
	// identifier, the way every reference between objects is - a name would
	// break the moment the object it names is renamed.
	//
	// They are checked where the whole configuration is known, not here: this
	// file alone cannot tell whether a role exists. A default pointing at
	// nothing is a form that will not open, and a project has to be told that
	// while it is being written, not when a user clicks.
	//
	// DefaultStyle is what the application is drawn with unless something
	// nearer says otherwise.
	DefaultStyle *uuid.UUID `yaml:"default_style,omitempty" json:"defaultStyle,omitempty"`
	// DefaultRoles are the rights a user works with when the list of users is
	// empty. The order is the configuration's own and is kept as written.
	DefaultRoles []uuid.UUID `yaml:"default_roles,omitempty" json:"defaultRoles,omitempty"`
	// SubsystemsOrder and SubsystemsVisibility are the root's half of the
	// command interface: the order the sections of the application stand in,
	// and which of them a role sees. The order names top-level subsystems; one
	// left out stands after those named. Both are carried and not yet acted
	// upon - the command interface is built in block 18.
	SubsystemsOrder      []uuid.UUID           `yaml:"subsystems_order,omitempty" json:"subsystemsOrder,omitempty"`
	SubsystemsVisibility []SubsystemVisibility `yaml:"subsystems_visibility,omitempty" json:"subsystemsVisibility,omitempty"`
	// The five forms below stand in for a report, a constant or a search that
	// names no form of its own. All five are common forms: they belong to no
	// object, which is exactly why the root can hand them to every object at
	// once.
	DefaultReportForm         *uuid.UUID `yaml:"default_report_form,omitempty" json:"defaultReportForm,omitempty"`
	DefaultReportSettingsForm *uuid.UUID `yaml:"default_report_settings_form,omitempty" json:"defaultReportSettingsForm,omitempty"`
	DefaultReportVariantForm  *uuid.UUID `yaml:"default_report_variant_form,omitempty" json:"defaultReportVariantForm,omitempty"`
	DefaultConstantsForm      *uuid.UUID `yaml:"default_constants_form,omitempty" json:"defaultConstantsForm,omitempty"`
	DefaultSearchForm         *uuid.UUID `yaml:"default_search_form,omitempty" json:"defaultSearchForm,omitempty"`
	// DefaultReportAppearanceTemplate is how reports are painted when a report
	// brings no appearance of its own. It is a common template, and one of a
	// single kind: an appearance template. Left empty, reports are painted the
	// way the platform paints them.
	DefaultReportAppearanceTemplate *uuid.UUID `yaml:"default_report_appearance_template,omitempty" json:"defaultReportAppearanceTemplate,omitempty"`
	// The five storages are where what a user saved is kept. They are named
	// separately rather than as one storage, because a configuration is free
	// to keep report variants in one place and form data in another - and
	// most keep neither, letting the platform use its own.
	CommonSettingsStorage           *uuid.UUID `yaml:"common_settings_storage,omitempty" json:"commonSettingsStorage,omitempty"`
	ReportsUserSettingsStorage      *uuid.UUID `yaml:"reports_user_settings_storage,omitempty" json:"reportsUserSettingsStorage,omitempty"`
	ReportsVariantsStorage          *uuid.UUID `yaml:"reports_variants_storage,omitempty" json:"reportsVariantsStorage,omitempty"`
	DynamicListsUserSettingsStorage *uuid.UUID `yaml:"dynamic_lists_user_settings_storage,omitempty" json:"dynamicListsUserSettingsStorage,omitempty"`
	FormDataSettingsStorage         *uuid.UUID `yaml:"form_data_settings_storage,omitempty" json:"formDataSettingsStorage,omitempty"`
	// URLExternalDataStorage is the sixth storage: where the data behind a
	// navigation link is kept when the link carries more than a reference.
	URLExternalDataStorage *uuid.UUID `yaml:"url_external_data_storage,omitempty" json:"urlExternalDataStorage,omitempty"`
	// The forms below stand in the same place as the five above - a common
	// form the root hands out when a nearer one is not named - and are listed
	// apart only because each waits on something different.
	//
	// The settings form of a dynamic list is ours to open and works.
	DefaultDynamicListSettingsForm *uuid.UUID `yaml:"default_dynamic_list_settings_form,omitempty" json:"defaultDynamicListSettingsForm,omitempty"`
	// AuxiliaryConstantsForm is what opens when the main constants form is not
	// named or cannot be used. A slot beside a main one is how the prototype
	// describes every other form, and the root is no exception.
	AuxiliaryConstantsForm *uuid.UUID `yaml:"auxiliary_constants_form,omitempty" json:"auxiliaryConstantsForm,omitempty"`
	// The three data history forms are carried and never opened: data history
	// is the prototype's mechanism, transferred and not implemented, and what
	// answers the question it was switched on for is the change history of ML.
	DataHistoryChangesForm           *uuid.UUID `yaml:"data_history_changes_form,omitempty" json:"dataHistoryChangesForm,omitempty"`
	DataHistoryVersionForm           *uuid.UUID `yaml:"data_history_version_form,omitempty" json:"dataHistoryVersionForm,omitempty"`
	DataHistoryVersionDifferenceForm *uuid.UUID `yaml:"data_history_version_difference_form,omitempty" json:"dataHistoryVersionDifferenceForm,omitempty"`
	// CollaborationSystemUsersChoiceForm names a form for a mechanism ML does
	// not have at all. It is carried for the same reason as everything else in
	// this file that ML does not execute: a configuration that named it, named
	// it.
	CollaborationSystemUsersChoiceForm *uuid.UUID `yaml:"collaboration_system_users_choice_form,omitempty" json:"collaborationSystemUsersChoiceForm,omitempty"`
	// What follows are the settings of the root that are not references: how
	// data is locked, what happens to a number nobody used, in which variant
	// of the language the configuration is written, what its names begin with
	// and which dictionaries the full-text search is told about besides its
	// own.
	//
	// Each is empty by default, and empty means the platform's own behaviour
	// rather than an unanswered question: a configuration that says nothing
	// about locking is locked the way ML locks, and that is the same answer it
	// would get by saying so.
	DataLockControl      DataLockControlMode      `yaml:"data_lock_control,omitempty" json:"dataLockControl,omitempty"`
	ObjectAutonumeration ObjectAutonumerationMode `yaml:"object_autonumeration,omitempty" json:"objectAutonumeration,omitempty"`
	ScriptVariant        ScriptVariant            `yaml:"script_variant,omitempty" json:"scriptVariant,omitempty"`
	// NamePrefix is what the names of this configuration's own objects begin
	// with. A library merged into an application prefixes its names so that
	// two libraries holding a catalog of the same purpose do not collide.
	NamePrefix string `yaml:"name_prefix,omitempty" json:"namePrefix,omitempty"`
	// AdditionalFullTextSearchDictionaries name what the search is told about
	// the language besides what the platform itself knows - forms of words and
	// synonyms. A dictionary is kept in a common template or in a constant,
	// so each one says which of the two it is: an identifier alone would make
	// the reader of the file guess where to look.
	AdditionalFullTextSearchDictionaries []DictionaryReference `yaml:"additional_full_text_search_dictionaries,omitempty" json:"additionalFullTextSearchDictionaries,omitempty"`
	// What follows are the modes of the root ML does not act on: they are
	// stored as written and shown in the properties of the root - the second
	// category of the conformance report, transferred and not implemented.
	//
	// The modes that exist only for the ordinary application, and the modes
	// of earlier releases and of mechanisms ML does not have, are not carried
	// at all (owner, 14.10.2026, METADATA-OBJECTS «Только управляемое
	// приложение»): the run mode, managed forms in the ordinary application
	// and ordinary forms in the managed one, modality, both modes of
	// synchronous calls, tablespaces, the compatibility of the platform, of
	// the extensions and of the interface. A file naming one is refused by
	// the strict reading; code reading Метаданные.РежимСовместимости and the
	// like gets ML's own behaviour as a constant (METADATA-OBJECTS). Nor is
	// the purpose of the configuration - an application of the platform or
	// of the mobile platform: with no mobile platform the one value left
	// would say nothing (owner, 15.10.2026).
	MainWindowMode            MainWindowMode `yaml:"main_window_mode,omitempty" json:"mainWindowMode,omitempty"`
	BinaryDataStorage         UseMode        `yaml:"binary_data_storage,omitempty" json:"binaryDataStorage,omitempty"`
	BinaryDataBlockStorageUse UseMode        `yaml:"binary_data_block_storage_use,omitempty" json:"binaryDataBlockStorageUse,omitempty"`
	IncludeHelpInContents     bool           `yaml:"include_help_in_contents,omitempty" json:"includeHelpInContents,omitempty"`
}

// UseMode is the answer to "is this used".
type UseMode string

const (
	Used    UseMode = "use"
	NotUsed UseMode = "do-not-use"
)

// MainWindowMode is what the main window of the client application looks like.
type MainWindowMode string

const (
	NormalWindow              MainWindowMode = "normal"
	WorkplaceWindow           MainWindowMode = "workplace"
	FullscreenWorkplaceWindow MainWindowMode = "fullscreen-workplace"
	EmbeddedWorkplaceWindow   MainWindowMode = "embedded-workplace"
	KioskWindow               MainWindowMode = "kiosk"
)

// DataLockControlMode says how the configuration locks the data it reads and
// writes. Managed locking is what ML does; the other two are carried because a
// transferred configuration says which one it was written for, and answering
// "we changed it for you" without a word is not an answer.
type DataLockControlMode string

const (
	AutomaticDataLock           DataLockControlMode = "automatic"
	ManagedDataLock             DataLockControlMode = "managed"
	AutomaticAndManagedDataLock DataLockControlMode = "automatic-and-managed"
)

// ObjectAutonumerationMode says what becomes of an automatically given number
// when the object that took it was never written. Released, the number is
// handed to the next object and the sequence has no holes; kept, it is spent
// and the sequence does. Which of the two is right is the application's to
// decide: a hole in an invoice numbering is a question from an auditor, and a
// number reused after a rollback is a different document under a known number.
type ObjectAutonumerationMode string

const (
	// ReleaseAutonumber hands an unused number back to the next object.
	ReleaseAutonumber ObjectAutonumerationMode = "release"
	// KeepAutonumber spends the number whether or not it was written.
	KeepAutonumber ObjectAutonumerationMode = "keep"
)

// ScriptVariant says in which variant of the built-in language the
// configuration is written. The language has two keyword sets meaning exactly
// the same thing, and a configuration written in one of them reads as noise in
// the other.
type ScriptVariant string

const (
	RussianScript ScriptVariant = "russian"
	EnglishScript ScriptVariant = "english"
)

// DictionaryKind is where an additional full-text search dictionary is kept:
// in a common template or in a constant. A dictionary in a constant is one the
// application fills at run time; a dictionary in a template is one it ships
// with.
type DictionaryKind string

const (
	TemplateDictionary DictionaryKind = "common-templates"
	ConstantDictionary DictionaryKind = "constants"
)

// DictionaryReference names one additional dictionary of the full-text search.
type DictionaryReference struct {
	Kind   DictionaryKind `yaml:"kind" json:"kind"`
	Object uuid.UUID      `yaml:"object" json:"object"`
}

// Language defines an interface language available in an ML Project.
// Language is a configured project language. Like every other metadata object
// of the platform it has a stable UUID: the code is what translations are keyed
// by and what a user's language setting names, so it cannot also serve as the
// object's identity - renaming a code would then silently mean "another
// language" everywhere the object itself is referenced.
type Language struct {
	ID   uuid.UUID `yaml:"id" json:"id"`
	Name string    `yaml:"name" json:"name"`
	// Title is localized, like the synonym of every other object. A language
	// whose synonym is written in two languages would otherwise lose one, and
	// the list of languages is precisely the place where two of them are in
	// front of the reader at once.
	//
	// That this is checked against the very list it belongs to is not a
	// circle: the languages and their synonyms lie in one file and are read
	// together, so by the time a synonym is checked the list is whole.
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	Code    string        `yaml:"code" json:"code"`
}

// Decode reads one strict YAML document and validates it.
func Decode(reader io.Reader) (Project, error) {
	return DecodeSource("ML Project", reader)
}

// DecodeSource reads one named strict YAML document so diagnostics identify its source.
func DecodeSource(source string, reader io.Reader) (Project, error) {
	// No ceiling on the size of a source: the prototype names none, and its
	// configurations keep files of tens of megabytes (METADATA-OBJECTS.md,
	// «Пределы на состав»; owner, 03.10.2026). What a file costs to read is
	// a limit of execution, decided by the format it is kept in - the content
	// of a spreadsheet, the largest of them, is JSON for that reason.
	content, err := io.ReadAll(reader)
	if err != nil {
		return Project{}, fmt.Errorf("read %s: %w", source, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)

	var result Project
	if err := decoder.Decode(&result); err != nil {
		return Project{}, fmt.Errorf("decode %s: %w", source, err)
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return Project{}, fmt.Errorf("decode trailing YAML in %s: %w", source, err)
		}
		return Project{}, fmt.Errorf("decode %s: multiple YAML documents are not allowed", source)
	}

	// A configuration written before languages had identities is read, not refused:
	// translations are keyed by language CODE, so nothing in the data depends on
	// the identity yet. The missing ones are filled here and become permanent at
	// the next save, which is what keeps old projects working without a
	// migration nobody has written.
	if err := result.assignLanguageIdentities(); err != nil {
		return Project{}, fmt.Errorf("decode %s: %w", source, err)
	}
	if err := result.Validate(); err != nil {
		return Project{}, fmt.Errorf("validate %s: %w", source, err)
	}

	return result, nil
}

// assignLanguageIdentities gives every language that has none an identity
// DERIVED from the project and the language code, not a random one. Reading the
// same file twice has to produce the same project: publication compares two
// independently read snapshots, and a fresh identity per read would make a
// project differ from itself. The derived value is written out at the next save
// and from then on is an ordinary stored identity, free to outlive the code it
// was derived from.
func (p Project) assignLanguageIdentities() error {
	for index := range p.Languages {
		if p.Languages[index].ID.IsZero() {
			p.Languages[index].ID = uuid.Derive(p.ID, "language:"+strings.ToLower(p.Languages[index].Code))
		}
	}
	return nil
}

// Encode validates and writes a stable YAML representation.
func Encode(writer io.Writer, value Project) error {
	if err := value.Validate(); err != nil {
		return err
	}

	var content bytes.Buffer
	encoder := yaml.NewEncoder(&content)
	encoder.SetIndent(2)

	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode ML Project: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("close ML Project encoder: %w", err)
	}
	if _, err := io.Copy(writer, &content); err != nil {
		return fmt.Errorf("write ML Project: %w", err)
	}

	return nil
}

// NewLanguage creates a configured language with a fresh identity. The title
// given is the language's synonym in itself - the one translation a new
// language certainly has, because it is the language being added.
func NewLanguage(name, title, code string) (Language, error) {
	id, err := uuid.New()
	if err != nil {
		return Language{}, err
	}
	return Language{ID: id, Name: name, Title: LocalizedText{code: title}, Code: code}, nil
}

// EnsureLanguageIdentities fills in the identity of every language that has
// none, so a configuration built in code or edited in a browser - where UUIDs are
// not invented - is saved complete.
func EnsureLanguageIdentities(configuration Project) (Project, error) {
	configuration.Languages = append([]Language(nil), configuration.Languages...)
	if err := configuration.assignLanguageIdentities(); err != nil {
		return Project{}, err
	}
	return configuration, nil
}

// Validate checks the project invariants used by all readers and writers.
func (p Project) Validate() error {
	var issues []ValidationIssue
	add := func(path, message string) { issues = append(issues, ValidationIssue{Path: path, Message: message}) }

	if p.Format != CurrentFormat {
		add("format", fmt.Sprintf("must be %d", CurrentFormat))
	}
	if p.ID.IsZero() {
		add("id", "must be a non-zero UUID")
	}
	if !isIdentifier(p.Name) {
		add("name", "must start with a letter and contain only letters or digits")
	} else if utf8.RuneCountInString(p.Name) > MaxNameLength {
		add("name", fmt.Sprintf("must not exceed %d characters", MaxNameLength))
	}
	// No ceiling on the number of languages, the length of a comment, a
	// vendor or a version: neither the help nor the configurator names one
	// (METADATA-OBJECTS.md, «Пределы на состав»).
	if len(p.Languages) == 0 {
		add("languages", "must contain at least one language")
	}

	seenNames := make(map[string]struct{}, len(p.Languages))
	seenCodes := make(map[string]struct{}, len(p.Languages))
	seenIDs := make(map[uuid.UUID]struct{}, len(p.Languages))
	for index, language := range p.Languages {
		prefix := fmt.Sprintf("languages[%d]", index)
		if language.ID.IsZero() {
			add(prefix+".id", "must be a non-zero UUID")
		} else if _, exists := seenIDs[language.ID]; exists {
			add(prefix+".id", "must be unique")
		}
		seenIDs[language.ID] = struct{}{}
		if !isIdentifier(language.Name) {
			add(prefix+".name", "must start with a letter and contain only letters or digits")
		} else if utf8.RuneCountInString(language.Name) > MaxNameLength {
			add(prefix+".name", fmt.Sprintf("must not exceed %d characters", MaxNameLength))
		}
		if language.Comment != "" && !isDisplayText(language.Comment, math.MaxInt) {
			add(prefix+".comment", "must be one line of printable characters")
		}
		if !LanguageCodeShape(language.Code) {
			add(prefix+".code", "must be a language code: no spaces, «=», «;», quotes, dots or separators of a path")
		}

		normalizedName := strings.ToLower(language.Name)
		if _, exists := seenNames[normalizedName]; exists {
			add(prefix+".name", "must be unique")
		}
		seenNames[normalizedName] = struct{}{}

		normalizedCode := strings.ToLower(language.Code)
		if _, exists := seenCodes[normalizedCode]; exists {
			add(prefix+".code", "must be unique")
		}
		seenCodes[normalizedCode] = struct{}{}
	}

	if !slices.ContainsFunc(p.Languages, func(language Language) bool {
		return strings.EqualFold(language.Code, p.DefaultLanguage)
	}) {
		add("default_language", "must reference a configured language code")
	}

	// The texts of the root are checked against the languages the root itself
	// declares - which is why they are checked here and not before the list is
	// read. The synonym has to be there; the rest of them a configuration is
	// free not to fill in.
	configured := make(map[string]bool, len(p.Languages))
	for _, language := range p.Languages {
		configured[strings.ToLower(language.Code)] = true
	}
	// An address is a line: a link written over two lines is not a link.
	// Every other text is written on as many lines as it needs.
	checkText := func(path string, text LocalizedText, required bool, line bool) {
		if len(text) == 0 {
			if required {
				add(path, "must contain at least one translation")
			}
			return
		}
		for _, code := range sortedKeys(text) {
			// A translation in a language the configuration does not declare
			// - erp, acc and sb carry «en» beside ru and uk - and one with no
			// code at all are kept: the help ties a text to no list. Both
			// are notes (metadata.NoteTextInUndeclaredLanguage). Only a code
			// that cannot be a code is refused.
			if code != "" && !LanguageCodeShape(code) {
				add(path+"."+code, "is not a language code")
			}
			switch {
			// A text of spaces alone is kept as written: the prototype keeps
			// a tooltip of one space (sb, 24 times). It is a note.
			case text[code] != "" && strings.TrimSpace(text[code]) == "":
			case line && !isDisplayText(text[code], math.MaxInt):
				add(path+"."+code, "must be one line of printable characters")
			case !line && !isLocalizedText(text[code]):
				add(path+"."+code, "must say something in printable characters, line breaks allowed")
			}
		}
	}
	checkText("title", p.Title, true, false)
	// A language's own synonym is checked here, with the texts of the root and
	// against the same list: until the whole list is read there is nothing to
	// check it against.
	for index, language := range p.Languages {
		checkText(fmt.Sprintf("languages[%d].title", index), language.Title, true, false)
	}
	checkText("brief_information", p.BriefInformation, false, false)
	checkText("detailed_information", p.DetailedInformation, false, false)
	checkText("copyright", p.Copyright, false, false)
	checkText("vendor_address", p.VendorAddress, false, true)
	checkText("information_address", p.InformationAddress, false, true)
	checkText("update_catalog_address", p.UpdateCatalogAddress, false, true)
	for path, value := range map[string]string{"comment": p.Comment, "vendor": p.Vendor, "version": p.Version} {
		if value != "" && !isDisplayText(value, math.MaxInt) {
			add(path, "must be one line of printable characters")
		}
	}

	// The defaults are checked here for shape only - that a reference is a
	// reference at all, and that a role is named once. Whether the object on
	// the other end exists is a question about the whole configuration, and it
	// is answered where the whole configuration is read.
	for path, reference := range map[string]*uuid.UUID{
		"default_style": p.DefaultStyle, "default_report_form": p.DefaultReportForm,
		"default_report_settings_form": p.DefaultReportSettingsForm,
		"default_report_variant_form":  p.DefaultReportVariantForm,
		"default_constants_form":       p.DefaultConstantsForm, "default_search_form": p.DefaultSearchForm,
		"default_report_appearance_template":     p.DefaultReportAppearanceTemplate,
		"common_settings_storage":                p.CommonSettingsStorage,
		"reports_user_settings_storage":          p.ReportsUserSettingsStorage,
		"reports_variants_storage":               p.ReportsVariantsStorage,
		"dynamic_lists_user_settings_storage":    p.DynamicListsUserSettingsStorage,
		"form_data_settings_storage":             p.FormDataSettingsStorage,
		"url_external_data_storage":              p.URLExternalDataStorage,
		"default_dynamic_list_settings_form":     p.DefaultDynamicListSettingsForm,
		"auxiliary_constants_form":               p.AuxiliaryConstantsForm,
		"data_history_changes_form":              p.DataHistoryChangesForm,
		"data_history_version_form":              p.DataHistoryVersionForm,
		"data_history_version_difference_form":   p.DataHistoryVersionDifferenceForm,
		"collaboration_system_users_choice_form": p.CollaborationSystemUsersChoiceForm,
	} {
		if reference != nil && reference.IsZero() {
			add(path, "must be a non-zero UUID")
		}
	}
	for _, issue := range validateSubsystemsInterface(p.SubsystemsOrder, p.SubsystemsVisibility) {
		field, message, _ := strings.Cut(issue, " ")
		add(field, message)
	}
	seenRoles := make(map[uuid.UUID]struct{}, len(p.DefaultRoles))
	for index, role := range p.DefaultRoles {
		prefix := fmt.Sprintf("default_roles[%d]", index)
		if role.IsZero() {
			add(prefix, "must be a non-zero UUID")
			continue
		}
		// A role granted twice grants nothing more than a role granted once,
		// so a repeat is a mistake in the list rather than a stronger right.
		if _, exists := seenRoles[role]; exists {
			add(prefix, "must be unique")
		}
		seenRoles[role] = struct{}{}
	}

	// A mode is a word from a known set, and a word outside it is not a
	// stricter setting but an unanswerable question: nothing downstream could
	// decide what to do with it.
	switch p.DataLockControl {
	case "", AutomaticDataLock, ManagedDataLock, AutomaticAndManagedDataLock:
	default:
		add("data_lock_control", "must be automatic, managed or automatic-and-managed")
	}
	switch p.ObjectAutonumeration {
	case "", ReleaseAutonumber, KeepAutonumber:
	default:
		add("object_autonumeration", "must be release or keep")
	}
	switch p.ScriptVariant {
	case "", RussianScript, EnglishScript:
	default:
		add("script_variant", "must be russian or english")
	}
	// The prefix stands in front of a name, so it has to be something a name
	// may begin with: a prefix that cannot be part of an identifier produces
	// objects that cannot be named.
	if p.NamePrefix != "" {
		if !isIdentifier(p.NamePrefix) {
			add("name_prefix", "must start with a letter and contain only letters or digits")
		} else if utf8.RuneCountInString(p.NamePrefix) > MaxNameLength {
			add("name_prefix", fmt.Sprintf("must not exceed %d characters", MaxNameLength))
		}
	}
	seenDictionaries := make(map[DictionaryReference]struct{}, len(p.AdditionalFullTextSearchDictionaries))
	for index, dictionary := range p.AdditionalFullTextSearchDictionaries {
		prefix := fmt.Sprintf("additional_full_text_search_dictionaries[%d]", index)
		switch dictionary.Kind {
		case TemplateDictionary, ConstantDictionary:
		default:
			add(prefix+".kind", "must be common-templates or constants")
		}
		if dictionary.Object.IsZero() {
			add(prefix+".object", "must be a non-zero UUID")
			continue
		}
		// The same dictionary named twice is read twice and helps once.
		if _, exists := seenDictionaries[dictionary]; exists {
			add(prefix, "must be unique")
		}
		seenDictionaries[dictionary] = struct{}{}
	}

	for path, mode := range map[string]UseMode{
		"binary_data_storage":           p.BinaryDataStorage,
		"binary_data_block_storage_use": p.BinaryDataBlockStorageUse,
	} {
		switch mode {
		case "", Used, NotUsed:
		default:
			add(path, "must be use or do-not-use")
		}
	}
	switch p.MainWindowMode {
	case "", NormalWindow, WorkplaceWindow, FullscreenWorkplaceWindow, EmbeddedWorkplaceWindow, KioskWindow:
	default:
		add("main_window_mode", "must be normal, workplace, fullscreen-workplace, embedded-workplace or kiosk")
	}
	if len(issues) > 0 {
		return &ValidationError{Issues: issues, unsupportedFormat: p.Format > 0 && p.Format != CurrentFormat}
	}

	return nil
}

// isIdentifier is the prototype's rule for a name, the same one a metadata
// object follows: a letter or an underscore first, then letters, digits and
// underscores.
func isIdentifier(value string) bool {
	for index, current := range []rune(value) {
		if current == '_' || unicode.IsLetter(current) {
			continue
		}
		if index == 0 || !unicode.IsDigit(current) {
			return false
		}
	}

	return value != ""
}

// isLocalizedText is a translation of a text of the configuration: a
// synonym, the brief and the detailed information, the copyright, the
// addresses. The prototype writes them on as many lines as they need, with no
// ceiling of length; other control characters are refused.
func isLocalizedText(value string) bool {
	if !utf8.ValidString(value) || strings.TrimSpace(value) == "" {
		return false
	}
	for _, symbol := range value {
		if unicode.IsControl(symbol) && symbol != '\n' && symbol != '\r' && symbol != '\t' {
			return false
		}
	}
	return true
}

func isDisplayText(value string, maximum int) bool {
	if !utf8.ValidString(value) || strings.TrimSpace(value) == "" || utf8.RuneCountInString(value) > maximum {
		return false
	}
	for _, symbol := range value {
		if unicode.IsControl(symbol) {
			return false
		}
	}
	return true
}
