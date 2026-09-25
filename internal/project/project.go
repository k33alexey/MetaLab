// Package project defines the source model of an ML Project.
package project

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
	// MaxYAMLDocumentBytes bounds individual source documents before parsing.
	MaxYAMLDocumentBytes = 4 << 20
)

var (
	// ErrUnsupportedFormat identifies a configuration written in an unsupported format version.
	ErrUnsupportedFormat = errors.New("unsupported ML Project format")
	// ErrYAMLDocumentTooLarge prevents unbounded memory use while parsing project sources.
	ErrYAMLDocumentTooLarge = errors.New("ML Project YAML document is too large")
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
	// DefaultInterface is carried and never resolved. It names the main menu
	// and toolbars an ordinary application is built from, and ML builds only
	// managed interfaces - there is no kind of object for it to point at.
	// Dropping it instead would be the one thing we forbade ourselves: a
	// silent loss. It is carried by name, because the only thing that could
	// give it an identifier is the branch that does not exist.
	DefaultInterface string `yaml:"default_interface,omitempty" json:"defaultInterface,omitempty"`
	// DefaultRoles are the rights a user works with when the list of users is
	// empty. The order is the configuration's own and is kept as written.
	DefaultRoles []uuid.UUID `yaml:"default_roles,omitempty" json:"defaultRoles,omitempty"`
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
	// What follows are the modes of the root. Almost all of them describe
	// behaviour ML does not have at all: the ordinary application, modal
	// windows, synchronous calls into platform extensions and add-ins,
	// tablespaces of the database, and compatibility with earlier releases of
	// the platform this one is written against.
	//
	// They are stored as written, shown in the properties of the root and act
	// on nothing - the second category of the conformance report, transferred
	// and not implemented. Dropping them is the one outcome that category
	// exists to prevent: a developer opening a transferred configuration must
	// not find that part of its properties vanished without a word, and an
	// administrator must not be left guessing why the transferred base behaves
	// unlike the original.
	DefaultRunMode                       ClientRunMode `yaml:"default_run_mode,omitempty" json:"defaultRunMode,omitempty"`
	UsePurposes                          []UsePurpose  `yaml:"use_purposes,omitempty" json:"usePurposes,omitempty"`
	UseManagedFormsInOrdinaryApplication bool          `yaml:"use_managed_forms_in_ordinary_application,omitempty" json:"useManagedFormsInOrdinaryApplication,omitempty"`
	UseOrdinaryFormsInManagedApplication bool          `yaml:"use_ordinary_forms_in_managed_application,omitempty" json:"useOrdinaryFormsInManagedApplication,omitempty"`
	ModalityUse                          UseMode       `yaml:"modality_use,omitempty" json:"modalityUse,omitempty"`
	SynchronousPlatformExtensionCallUse  UseMode       `yaml:"synchronous_platform_extension_call_use,omitempty" json:"synchronousPlatformExtensionCallUse,omitempty"`
	// SynchronousExtensionCallUse is the older mode of the same thing, left
	// behind by the platform in 8.3.8 and still written in configurations from
	// before it. Both are carried, because a configuration that holds the old
	// one holds it, and which of the two it wrote is part of what it says.
	SynchronousExtensionCallUse UseMode                    `yaml:"synchronous_extension_call_use,omitempty" json:"synchronousExtensionCallUse,omitempty"`
	InterfaceCompatibility      InterfaceCompatibilityMode `yaml:"interface_compatibility,omitempty" json:"interfaceCompatibility,omitempty"`
	MainWindowMode              MainWindowMode             `yaml:"main_window_mode,omitempty" json:"mainWindowMode,omitempty"`
	DatabaseTablespacesUse      UseMode                    `yaml:"database_tablespaces_use,omitempty" json:"databaseTablespacesUse,omitempty"`
	BinaryDataStorage           UseMode                    `yaml:"binary_data_storage,omitempty" json:"binaryDataStorage,omitempty"`
	BinaryDataBlockStorageUse   UseMode                    `yaml:"binary_data_block_storage_use,omitempty" json:"binaryDataBlockStorageUse,omitempty"`
	// The two compatibility versions are kept as versions and not as words
	// from a list. The list is the release history of another platform: we do
	// not own it, it grows without us, and a version we had not enumerated
	// would be refused at import - which is the one thing a mode carried for
	// the sake of not losing it must never do. Empty means the configuration
	// is written against no earlier release.
	CompatibilityVersion          string `yaml:"compatibility_version,omitempty" json:"compatibilityVersion,omitempty"`
	ExtensionCompatibilityVersion string `yaml:"extension_compatibility_version,omitempty" json:"extensionCompatibilityVersion,omitempty"`
	IncludeHelpInContents         bool   `yaml:"include_help_in_contents,omitempty" json:"includeHelpInContents,omitempty"`
	// What follows describes a mobile application. ML does not build one, so
	// none of it is acted upon - it is carried for the same reason the modes
	// above are: a configuration that declared it, declared it, and a property
	// that disappears at transfer is a property nobody can ask about later.
	//
	// Four of the seven are lists of words the prototype's own tooling knows:
	// which of its abilities the application uses, which permissions of the
	// operating system it asks for, which navigation links it intercepts and
	// which kinds of data it accepts through "share". They are stored as
	// written, because those lists belong to another platform and to the
	// mobile operating systems, both of which grow without us - and a word we
	// had not enumerated would be refused at transfer, which is the one thing
	// a property carried for the sake of not losing it must never do.
	UsedMobileFunctionalities []string `yaml:"used_mobile_functionalities,omitempty" json:"usedMobileFunctionalities,omitempty"`
	RequiredMobilePermissions []string `yaml:"required_mobile_permissions,omitempty" json:"requiredMobilePermissions,omitempty"`
	MobileApplicationURLs     []string `yaml:"mobile_application_urls,omitempty" json:"mobileApplicationUrls,omitempty"`
	AllowedShareRequestTypes  []string `yaml:"allowed_share_request_types,omitempty" json:"allowedShareRequestTypes,omitempty"`
	// MobileClientSignature is what the mobile client is signed with.
	MobileClientSignature string `yaml:"mobile_client_signature,omitempty" json:"mobileClientSignature,omitempty"`
	// StandaloneConfigurationContent is which objects go into the standalone
	// application - the one that works without a connection and synchronises
	// later. Each entry names an object of the configuration, so each says
	// which kind it is: there is no single list to search, and an identifier
	// alone would have to be looked for in all of them.
	StandaloneConfigurationContent []ObjectReference `yaml:"standalone_configuration_content,omitempty" json:"standaloneConfigurationContent,omitempty"`
	// StandaloneConfigurationRestrictionRoles narrow the rights a person has
	// while working without a connection: what is safe to do against a copy
	// nobody else sees is not the same as what is safe against the base.
	StandaloneConfigurationRestrictionRoles []uuid.UUID `yaml:"standalone_configuration_restriction_roles,omitempty" json:"standaloneConfigurationRestrictionRoles,omitempty"`
}

// ObjectReference names one object of the configuration by the kind it belongs
// to and its identifier. The kind is part of the reference and not a hint: an
// identifier alone would have to be searched for in every kind there is.
type ObjectReference struct {
	Kind   string    `yaml:"kind" json:"kind"`
	Object uuid.UUID `yaml:"object" json:"object"`
}

// ClientRunMode is which application the platform starts by default. ML builds
// only the managed application, so the value says what the configuration was
// written for and nothing more.
type ClientRunMode string

const (
	AutoRunMode                ClientRunMode = "auto"
	ManagedApplicationRunMode  ClientRunMode = "managed-application"
	OrdinaryApplicationRunMode ClientRunMode = "ordinary-application"
)

// UsePurpose is what the configuration is meant to run on.
type UsePurpose string

const (
	PersonalComputerPurpose UsePurpose = "personal-computer"
	MobileDevicePurpose     UsePurpose = "mobile-device"
)

// UseMode is the answer to "is this used": three-valued where the platform
// warns before it refuses, two-valued everywhere else. Which of the two shapes
// a mode has is checked per property, because a mode that cannot warn must not
// be allowed to say it warns.
type UseMode string

const (
	Used            UseMode = "use"
	NotUsed         UseMode = "do-not-use"
	UsedWithWarning UseMode = "use-with-warnings"
)

// InterfaceCompatibilityMode is which generation of the prototype's interface
// the configuration was drawn for.
type InterfaceCompatibilityMode string

const (
	Version82Interface          InterfaceCompatibilityMode = "version-8-2"
	Version82AllowTaxiInterface InterfaceCompatibilityMode = "version-8-2-allow-taxi"
	TaxiAllowVersion82Interface InterfaceCompatibilityMode = "taxi-allow-version-8-2"
	TaxiInterface               InterfaceCompatibilityMode = "taxi"
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
	ID    uuid.UUID `yaml:"id" json:"id"`
	Name  string    `yaml:"name" json:"name"`
	Title string    `yaml:"title" json:"title"`
	Code  string    `yaml:"code" json:"code"`
}

// Decode reads one strict YAML document and validates it.
func Decode(reader io.Reader) (Project, error) {
	return DecodeSource("ML Project", reader)
}

// DecodeSource reads one named strict YAML document so diagnostics identify its source.
func DecodeSource(source string, reader io.Reader) (Project, error) {
	content, err := io.ReadAll(io.LimitReader(reader, MaxYAMLDocumentBytes+1))
	if err != nil {
		return Project{}, fmt.Errorf("read %s: %w", source, err)
	}
	if len(content) > MaxYAMLDocumentBytes {
		return Project{}, fmt.Errorf("decode %s: %w (maximum %d bytes)", source, ErrYAMLDocumentTooLarge, MaxYAMLDocumentBytes)
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
	if content.Len() > MaxYAMLDocumentBytes {
		return ErrYAMLDocumentTooLarge
	}
	if _, err := io.Copy(writer, &content); err != nil {
		return fmt.Errorf("write ML Project: %w", err)
	}

	return nil
}

// NewLanguage creates a configured language with a fresh identity.
func NewLanguage(name, title, code string) (Language, error) {
	id, err := uuid.New()
	if err != nil {
		return Language{}, err
	}
	return Language{ID: id, Name: name, Title: title, Code: code}, nil
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
	} else if utf8.RuneCountInString(p.Name) > 128 {
		add("name", "must not exceed 128 characters")
	}
	if len(p.Languages) == 0 {
		add("languages", "must contain at least one language")
	} else if len(p.Languages) > 100 {
		add("languages", "must not contain more than 100 languages")
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
		} else if utf8.RuneCountInString(language.Name) > 128 {
			add(prefix+".name", "must not exceed 128 characters")
		}
		if !isDisplayText(language.Title, 512) {
			add(prefix+".title", "must contain 1 to 512 printable characters")
		}
		if !isLocaleCode(language.Code) {
			add(prefix+".code", "must be a lowercase language code with optional region")
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
	checkText := func(path string, text LocalizedText, required bool) {
		if len(text) == 0 {
			if required {
				add(path, "must contain at least one translation")
			}
			return
		}
		for _, code := range sortedKeys(text) {
			if !configured[strings.ToLower(code)] {
				add(path+"."+code, "uses an unconfigured language")
			}
			if !isDisplayText(text[code], 512) {
				add(path+"."+code, "must contain 1 to 512 printable characters")
			}
		}
	}
	checkText("title", p.Title, true)
	checkText("brief_information", p.BriefInformation, false)
	checkText("detailed_information", p.DetailedInformation, false)
	checkText("copyright", p.Copyright, false)
	checkText("vendor_address", p.VendorAddress, false)
	checkText("information_address", p.InformationAddress, false)
	checkText("update_catalog_address", p.UpdateCatalogAddress, false)
	if p.Comment != "" && !isDisplayText(p.Comment, 1024) {
		add("comment", "must contain 1 to 1024 printable characters")
	}
	if p.Vendor != "" && !isDisplayText(p.Vendor, 512) {
		add("vendor", "must contain 1 to 512 printable characters")
	}
	if p.Version != "" && !isDisplayText(p.Version, 128) {
		add("version", "must contain 1 to 128 printable characters")
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
	if p.DefaultInterface != "" && !isIdentifier(p.DefaultInterface) {
		add("default_interface", "must start with a letter and contain only letters or digits")
	} else if utf8.RuneCountInString(p.DefaultInterface) > 128 {
		add("default_interface", "must not exceed 128 characters")
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
		} else if utf8.RuneCountInString(p.NamePrefix) > 128 {
			add("name_prefix", "must not exceed 128 characters")
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

	switch p.DefaultRunMode {
	case "", AutoRunMode, ManagedApplicationRunMode, OrdinaryApplicationRunMode:
	default:
		add("default_run_mode", "must be auto, managed-application or ordinary-application")
	}
	seenPurposes := make(map[UsePurpose]struct{}, len(p.UsePurposes))
	for index, purpose := range p.UsePurposes {
		path := fmt.Sprintf("use_purposes[%d]", index)
		switch purpose {
		case PersonalComputerPurpose, MobileDevicePurpose:
		default:
			add(path, "must be personal-computer or mobile-device")
			continue
		}
		// Named twice, a purpose is still one purpose.
		if _, exists := seenPurposes[purpose]; exists {
			add(path, "must be unique")
		}
		seenPurposes[purpose] = struct{}{}
	}
	// A mode that cannot warn must not be allowed to say it warns: the
	// platform either uses tablespaces or does not, and "with warnings" there
	// would be a value nothing could act on.
	for path, mode := range map[string]UseMode{
		"modality_use": p.ModalityUse,
		"synchronous_platform_extension_call_use": p.SynchronousPlatformExtensionCallUse,
		"synchronous_extension_call_use":          p.SynchronousExtensionCallUse,
	} {
		switch mode {
		case "", Used, NotUsed, UsedWithWarning:
		default:
			add(path, "must be use, do-not-use or use-with-warnings")
		}
	}
	for path, mode := range map[string]UseMode{
		"database_tablespaces_use":      p.DatabaseTablespacesUse,
		"binary_data_storage":           p.BinaryDataStorage,
		"binary_data_block_storage_use": p.BinaryDataBlockStorageUse,
	} {
		switch mode {
		case "", Used, NotUsed:
		default:
			add(path, "must be use or do-not-use")
		}
	}
	switch p.InterfaceCompatibility {
	case "", Version82Interface, Version82AllowTaxiInterface, TaxiAllowVersion82Interface, TaxiInterface:
	default:
		add("interface_compatibility", "must be one of the four interface generations")
	}
	switch p.MainWindowMode {
	case "", NormalWindow, WorkplaceWindow, FullscreenWorkplaceWindow, EmbeddedWorkplaceWindow, KioskWindow:
	default:
		add("main_window_mode", "must be normal, workplace, fullscreen-workplace, embedded-workplace or kiosk")
	}
	for path, version := range map[string]string{
		"compatibility_version":           p.CompatibilityVersion,
		"extension_compatibility_version": p.ExtensionCompatibilityVersion,
	} {
		if version != "" && !isVersion(version) {
			add(path, "must be a version such as 8.3.21")
		}
	}

	// A list of words is checked for being a list of words: an empty entry
	// names nothing, a repeat asks for the same thing twice, and neither could
	// be acted upon by the tooling that will read this back.
	for path, list := range map[string][]string{
		"used_mobile_functionalities": p.UsedMobileFunctionalities,
		"required_mobile_permissions": p.RequiredMobilePermissions,
		"mobile_application_urls":     p.MobileApplicationURLs,
		"allowed_share_request_types": p.AllowedShareRequestTypes,
	} {
		seen := make(map[string]struct{}, len(list))
		for index, item := range list {
			where := fmt.Sprintf("%s[%d]", path, index)
			if strings.TrimSpace(item) != item || item == "" {
				add(where, "must be a word without surrounding spaces")
				continue
			}
			if utf8.RuneCountInString(item) > 256 {
				add(where, "must not exceed 256 characters")
			}
			if _, exists := seen[item]; exists {
				add(where, "must be unique")
			}
			seen[item] = struct{}{}
		}
	}
	if utf8.RuneCountInString(p.MobileClientSignature) > 4096 {
		add("mobile_client_signature", "must not exceed 4096 characters")
	}
	seenContent := make(map[ObjectReference]struct{}, len(p.StandaloneConfigurationContent))
	for index, item := range p.StandaloneConfigurationContent {
		where := fmt.Sprintf("standalone_configuration_content[%d]", index)
		if item.Kind == "" {
			add(where+".kind", "must name a kind of metadata")
		}
		if item.Object.IsZero() {
			add(where+".object", "must be a non-zero UUID")
			continue
		}
		// The same object taken into the standalone application twice is taken
		// once: the second entry adds nothing and hides a mistake in the list.
		if _, exists := seenContent[item]; exists {
			add(where, "must be unique")
		}
		seenContent[item] = struct{}{}
	}
	seenRestrictions := make(map[uuid.UUID]struct{}, len(p.StandaloneConfigurationRestrictionRoles))
	for index, role := range p.StandaloneConfigurationRestrictionRoles {
		where := fmt.Sprintf("standalone_configuration_restriction_roles[%d]", index)
		if role.IsZero() {
			add(where, "must be a non-zero UUID")
			continue
		}
		if _, exists := seenRestrictions[role]; exists {
			add(where, "must be unique")
		}
		seenRestrictions[role] = struct{}{}
	}

	if len(issues) > 0 {
		return &ValidationError{Issues: issues, unsupportedFormat: p.Format > 0 && p.Format != CurrentFormat}
	}

	return nil
}

// isVersion says whether this is a version of the shape the compatibility
// modes are written in - two or three numbers separated by dots. The set of
// versions itself is not ours to know: it is another platform's release
// history, and refusing a release we had not heard of would lose exactly what
// carrying the mode was for.
func isVersion(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) < 2 || len(parts) > 4 {
		return false
	}
	for _, part := range parts {
		if part == "" || len(part) > 4 {
			return false
		}
		for _, digit := range part {
			if !unicode.IsDigit(digit) {
				return false
			}
		}
	}
	return true
}

func isIdentifier(value string) bool {
	for index, current := range []rune(value) {
		if index == 0 && !unicode.IsLetter(current) {
			return false
		}
		if !unicode.IsLetter(current) && !unicode.IsDigit(current) {
			return false
		}
	}

	return value != ""
}

func isLocaleCode(value string) bool {
	parts := strings.Split(value, "-")
	if len(parts) < 1 || len(parts) > 2 || len(parts[0]) < 2 || len(parts[0]) > 3 {
		return false
	}
	for _, current := range parts[0] {
		if current < 'a' || current > 'z' {
			return false
		}
	}
	if len(parts) == 1 {
		return true
	}
	if len(parts[1]) != 2 {
		return false
	}
	for _, current := range parts[1] {
		if current < 'A' || current > 'Z' {
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
