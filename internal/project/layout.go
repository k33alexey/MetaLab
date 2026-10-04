package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	// ConfigurationFile is the description of the configuration root, and with
	// it of the whole ML Project: the identifier, the name, the synonym, the
	// languages and everything else the root says about itself.
	//
	// It is not a configuration standing beside the root - there is no such thing.
	// The root is the configuration, so its description lies at the top of the
	// project beside the branches of its kinds, which is where the prototype's
	// own export puts it too.
	ConfigurationFile = "configuration.yaml"
	// SessionModuleFile and ApplicationModuleFile are the two modules of the
	// configuration root. They lie at the project root beside its description,
	// named after the role each plays - the same way every other object names
	// its modules, and for the same reason: the root owns exactly one of each,
	// so there is nothing for a UUID to tell apart, and the file lying under
	// the name of its role is the whole declaration.
	//
	// Both are optional. A project without a session module has no handler for
	// session parameters; one without an application module has nothing to run
	// as the application starts and stops.
	SessionModuleFile     = "МодульСеанса.bsl"
	ApplicationModuleFile = "МодульПриложения.bsl"
	// ExternalConnectionModuleFile holds what runs when something outside
	// connects to the base without a person in front of it.
	ExternalConnectionModuleFile = "МодульВнешнегоСоединения.bsl"
	// OrdinaryApplicationModuleFile is carried and never run: ML has no
	// ordinary application. It is not a precaution - in a real configuration
	// this is a file of several thousand characters, and dropping it would
	// lose the only record of what used to happen at start-up. Kept beside the
	// others so that a developer sees what was there, and named by its role
	// like every module of the root.
	OrdinaryApplicationModuleFile = "МодульОбычногоПриложения.bsl"
	// LogoDirectory and SplashDirectory are the two pictures the configuration
	// root owns: the logo it is shown by and the splash it opens with. They
	// are the root's own files and not references to common pictures - a
	// reference would make transferring a configuration with a splash create a
	// common picture that was never in it, and the composition of the
	// transferred configuration would then differ from the original.
	//
	// Each is a folder named after its role holding one file per screen
	// density, the same way every picture of the platform is stored, or the
	// files its variants name, described in RootPictureDescriptionFile.
	//
	// MainSectionPictureDirectory is the third, the picture of the main
	// section (help, MainSectionPicture, since 8.3.3). All three
	// configurations being moved have one, and the model had no place for it.
	LogoDirectory               = "Логотип"
	SplashDirectory             = "Заставка"
	MainSectionPictureDirectory = "КартинкаОсновногоРаздела"
	// RootPictureDescriptionFile says how the files of a picture of the root
	// are drawn - its variants and its transparency. Left out, the files are
	// named by density and drawn as they are.
	RootPictureDescriptionFile = "picture.yaml"
	// HomePageFile, ClientInterfaceFile and MainSectionCommandInterfaceFile
	// describe what the application shows before anything is opened: the
	// forms of the home page, the panels of the main window and the commands
	// of the main section. ParentConfigurationsFile is the vendor's support
	// settings, carried as the prototype wrote them. The configurations being
	// moved keep each of them - see the metadata package.
	HomePageFile                    = "НачальнаяСтраница.yaml"
	ClientInterfaceFile             = "ИнтерфейсКлиентскогоПриложения.yaml"
	MainSectionCommandInterfaceFile = "КомандныйИнтерфейсОсновногоРаздела.yaml"
	ParentConfigurationsFile        = "РодительскиеКонфигурации.bin"
	keepFile                        = ".gitkeep"

	// ObjectMetadataFile is the description of the object owning a folder.
	ObjectMetadataFile = "object.yaml"
	// ObjectModuleFile, ManagerModuleFile and RecordSetModuleFile are the
	// modules an object keeps in its own folder, named after the role each
	// one plays. Which of them a kind may have is decided by the kind: a
	// register keeps a record set where a catalog keeps an object.
	ObjectModuleFile    = "МодульОбъекта.bsl"
	ManagerModuleFile   = "МодульМенеджера.bsl"
	RecordSetModuleFile = "МодульНабораЗаписей.bsl"
	// RecalculationsDirectory is where a calculation register keeps the folders
	// of its recalculations, each holding the recalculation's record set
	// module. A recalculation is described inside the register; only its
	// module needs a place on disk.
	RecalculationsDirectory = "recalculations"
	// ServiceModuleFile is the module of one service, inside that service's
	// own folder. A service owns exactly one module - the procedures its
	// operations name - so there is nothing for an identifier to tell apart.
	ServiceModuleFile = "МодульСервиса.bsl"
	// ClientModuleFile is the module of one WebSocket client, inside that
	// client's own folder: the handlers of its connection.
	ClientModuleFile = "МодульКлиента.bsl"
	// CommandModuleFile is the module of one command, inside that command's
	// own folder.
	CommandModuleFile = "МодульКоманды.bsl"
	// FormMetadataFile is the description of one managed form, inside the
	// folder named after that form.
	FormMetadataFile = "form.yaml"
	// FormModuleFile is the module of one form, beside the form's own
	// description in that same folder.
	FormModuleFile = "МодульФормы.bsl"
	// FormBodyFile is the body of an ordinary form, beside its description,
	// carried as the prototype wrote it (Form.bin, with the module inside: 157,
	// 160, 0, 0 and 1547 ordinary forms in the exports). ML builds no ordinary
	// form, so the body is not read; its module is taken out of it into
	// FormModuleFile, where it is carried and not compiled.
	FormBodyFile = "form.bin"
	// ValueModuleFile is the module of a constant's value: the one the
	// platform calls when the value is checked and written.
	ValueModuleFile = "МодульЗначения.bsl"
)

var (
	// ErrProjectExists prevents initialization over any existing file or directory.
	ErrProjectExists = errors.New("ML Project path already exists")
	// ErrInvalidLayout indicates a missing, unsafe, or malformed project path.
	ErrInvalidLayout = errors.New("invalid ML Project layout")
	// ErrProjectIdentityChanged prevents accidental replacement with another project configuration.
	ErrProjectIdentityChanged = errors.New("ML Project identity cannot be changed")

	rootDirectories = []string{"metadata", "modules"}
	// objectModuleFiles lists every module role an object may keep directly
	// in its own folder. Commands are not here - a command module lives in
	// the folder of its command.
	objectModuleFiles = []string{ObjectModuleFile, ManagerModuleFile, RecordSetModuleFile}
	// moduleRoleFiles is every role a module may play inside the folder of
	// whatever owns it, across all kinds - the three an object keeps plus the
	// one a constant keeps around its value.
	moduleRoleFiles = []string{ObjectModuleFile, ManagerModuleFile, RecordSetModuleFile, ValueModuleFile}
	// objectSubordinateDirectories lists the folders an object may keep
	// beside its own description and modules.
	objectSubordinateDirectories = []string{"forms", "commands", "templates"}
	// namedFolderKinds lists the kinds whose object keeps a folder named after
	// it holding a fixed set of files and nothing else - no forms, commands or
	// templates of its own. Each entry says which files that folder may hold.
	//
	// They keep a folder for the same reason an object does: they own files.
	// What they do not own is subordinate entities, so they are kept apart
	// from objectFolderKinds rather than given machinery they have no use for.
	namedFolderKinds = map[string][]string{
		"common-forms":    {FormMetadataFile, FormModuleFile, FormBodyFile},
		"common-commands": {ObjectMetadataFile, CommandModuleFile},
		"constants":       {ObjectMetadataFile, ValueModuleFile, ManagerModuleFile},
		// A common template and a common picture keep a description and
		// nothing else that is source: what else lies in their folder is
		// content - a spreadsheet, an archive, an image - and content is
		// named by the kind of template or by the density of the image, not
		// by a role. It is checked where the metadata is read, and left out
		// here so that binary content stays out of code search and out of the
		// module index.
		"common-templates": {ObjectMetadataFile},
		"common-pictures":  {ObjectMetadataFile},
		// A package of exchanged types keeps its description and its schema.
		// The schema is XML - that is what a schema of exchanged types is
		// everywhere it is read - and it is not source in our sense, so it is
		// left out of the list the same way a picture's images are.
		"xdto-packages": {ObjectMetadataFile},
		// A service keeps its description and one module: the procedures its
		// operations name lie there, and nowhere else.
		"web-services":  {ObjectMetadataFile, ServiceModuleFile},
		"http-services": {ObjectMetadataFile, ServiceModuleFile},
		// A WS reference keeps its description and the service description
		// it was imported with. The latter is WSDL and not source in our
		// sense, so it is left out the way an XDTO package's schema is.
		"ws-references": {ObjectMetadataFile},
		// A WebSocket client keeps its description and one module: the
		// handlers of the connection it opens.
		"websocket-clients": {ObjectMetadataFile, ClientModuleFile},
		// An external data source keeps its description here and its tables
		// one level down, in a folder of tables - see the metadata package.
		// Only the description is listed: the tables are subordinate objects
		// with folders of their own, and the walkers of object folders do not
		// know the second level yet.
		"external-data-sources": {ObjectMetadataFile},
	}
	// objectFolderKinds lists metadata kinds whose objects group their own
	// description, modules, forms, commands and templates under one folder
	// named after the object, instead of scattering them across the flat
	// modules/ and forms/ roots.
	// Виды, у которых есть собственные модули и формы, хранят объект папкой:
	// описание лежит рядом со своим кодом и формами, а не в общей куче.
	objectFolderKinds = []string{"catalogs", "documents", "enumerations", "information-registers", "accumulation-registers",
		"charts-of-characteristic-types", "charts-of-accounts", "charts-of-calculation-types", "business-processes", "tasks",
		"filter-criteria", "settings-storages",
		"exchange-plans", "document-journals", "accounting-registers", "calculation-registers",
		"reports", "data-processors", "sequences"}
	// Порядок — тот, в котором виды показываются в дереве конфигурации, а не
	// алфавитный: сначала группа «Общие», затем объекты верхнего уровня.
	// Нумераторы и последовательности стоят рядом с документами, потому что
	// в дереве они ветви внутри «Документов».
	metadataKinds = []string{
		"subsystems",
		"common-modules",
		"session-parameters",
		"roles",
		"common-attributes",
		"exchange-plans",
		"filter-criteria",
		"event-subscriptions",
		"scheduled-jobs",
		"functional-options",
		"functional-options-parameters",
		"defined-types",
		"settings-storages",
		"common-commands",
		"command-groups",
		"common-forms",
		"common-templates",
		"common-pictures",
		"xdto-packages",
		"web-services",
		"http-services",
		"ws-references",
		"websocket-clients",
		"style-items",
		"styles",
		"languages",
		"constants",
		"catalogs",
		"documents",
		"document-numerators",
		"sequences",
		"document-journals",
		"enumerations",
		"reports",
		"data-processors",
		"charts-of-characteristic-types",
		"charts-of-accounts",
		"charts-of-calculation-types",
		"information-registers",
		"accumulation-registers",
		"accounting-registers",
		"calculation-registers",
		"business-processes",
		"tasks",
		"external-data-sources",
	}
)

// rootModuleFiles is every module the configuration root may keep, in the
// order the tree shows them.
var rootModuleFiles = []string{SessionModuleFile, ApplicationModuleFile,
	ExternalConnectionModuleFile, OrdinaryApplicationModuleFile}

// rootPictureDirectories is every picture the configuration root may keep, in
// the order the tree shows them.
var rootPictureDirectories = []string{LogoDirectory, SplashDirectory, MainSectionPictureDirectory}

// rootDescriptionFiles is every file the configuration root keeps beside its
// description and its modules, in the order the tree shows them.
var rootDescriptionFiles = []string{HomePageFile, ClientInterfaceFile, MainSectionCommandInterfaceFile, ParentConfigurationsFile}

// RootDescriptionFiles returns the files of the root that are neither its
// description nor a module nor a picture.
func RootDescriptionFiles() []string { return slices.Clone(rootDescriptionFiles) }

// RootPictureDirectories returns the pictures of the configuration root.
func RootPictureDirectories() []string { return slices.Clone(rootPictureDirectories) }

// IsRootPictureDirectory reports whether a project-root folder is one of the
// root's own pictures.
func IsRootPictureDirectory(name string) bool { return slices.Contains(rootPictureDirectories, name) }

// RootModuleFiles returns the modules of the configuration root.
func RootModuleFiles() []string { return slices.Clone(rootModuleFiles) }

// IsRootModuleFile reports whether a project-root file is one of the root's
// own modules.
func IsRootModuleFile(name string) bool { return slices.Contains(rootModuleFiles, name) }

// RootDirectories returns the canonical Git-tracked source directories.
func RootDirectories() []string { return slices.Clone(rootDirectories) }

// MetadataKinds returns the supported physical metadata directory names.
func MetadataKinds() []string { return slices.Clone(metadataKinds) }

// ObjectFolderKinds returns the metadata kinds whose objects use a
// per-object folder layout instead of a single flat YAML file.
func ObjectFolderKinds() []string { return slices.Clone(objectFolderKinds) }

// NamedFolderFiles returns the files one object of a named-folder kind may
// keep in its folder, and whether the kind keeps such a folder at all.
func NamedFolderFiles(kind string) ([]string, bool) {
	files, ok := namedFolderKinds[kind]
	return slices.Clone(files), ok
}

// NamedFolderKinds returns every kind whose object keeps a folder named after
// it holding a fixed set of files.
func NamedFolderKinds() []string {
	result := make([]string, 0, len(namedFolderKinds))
	for kind := range namedFolderKinds {
		result = append(result, kind)
	}
	slices.Sort(result)
	return result
}

// ObjectModuleFiles returns every module role name an object may keep
// directly in its own folder.
func ObjectModuleFiles() []string { return slices.Clone(objectModuleFiles) }

// ObjectSubordinateDirectories returns the folders an object may keep beside
// its own description and modules.
func ObjectSubordinateDirectories() []string { return slices.Clone(objectSubordinateDirectories) }

// Initialize atomically creates a new canonical ML Project at a previously unused path.
func Initialize(root string, configuration Project) error {
	// A configuration built in code names its languages but does not invent their
	// identities; that is this layer's job, here and in SaveConfiguration.
	configuration, err := EnsureLanguageIdentities(configuration)
	if err != nil {
		return err
	}
	if err := configuration.Validate(); err != nil {
		return err
	}
	root, err = cleanRoot(root)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(root); err == nil {
		return ErrProjectExists
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect ML Project target: %w", err)
	}

	parent := filepath.Dir(root)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create ML Project parent: %w", err)
	}
	staging, err := os.MkdirTemp(parent, "."+filepath.Base(root)+"-*")
	if err != nil {
		return fmt.Errorf("create ML Project staging directory: %w", err)
	}
	defer os.RemoveAll(staging)
	if err := os.Chmod(staging, 0o755); err != nil {
		return fmt.Errorf("set ML Project directory permissions: %w", err)
	}

	configurationPath := filepath.Join(staging, ConfigurationFile)
	file, err := os.OpenFile(configurationPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create ML Project configuration: %w", err)
	}
	if err := Encode(file, configuration); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync ML Project configuration: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close ML Project configuration: %w", err)
	}
	for _, directory := range rootDirectories {
		path := filepath.Join(staging, directory)
		if err := os.Mkdir(path, 0o755); err != nil {
			return fmt.Errorf("create ML Project directory %q: %w", directory, err)
		}
		if err := os.WriteFile(filepath.Join(path, keepFile), nil, 0o644); err != nil {
			return fmt.Errorf("create Git placeholder for %q: %w", directory, err)
		}
	}
	if err := os.Rename(staging, root); err != nil {
		if _, statErr := os.Lstat(root); statErr == nil {
			return ErrProjectExists
		}
		return fmt.Errorf("publish ML Project directory: %w", err)
	}
	return nil
}

// ValidateLayout verifies the safe root structure and the current configuration.
func ValidateLayout(root string) (Project, error) {
	root, err := cleanRoot(root)
	if err != nil {
		return Project{}, err
	}
	if err := requirePath(root, true); err != nil {
		return Project{}, err
	}
	configurationPath := filepath.Join(root, ConfigurationFile)
	if err := requirePath(configurationPath, false); err != nil {
		return Project{}, err
	}
	for _, directory := range rootDirectories {
		if err := requirePath(filepath.Join(root, directory), true); err != nil {
			return Project{}, err
		}
	}
	configuration, err := readConfiguration(configurationPath)
	if err != nil {
		return Project{}, fmt.Errorf("%w: %v", ErrInvalidLayout, err)
	}
	return configuration, nil
}

// SaveConfiguration atomically writes a validated configuration without allowing its stable UUID to change.
func SaveConfiguration(root string, configuration Project) error {
	root, err := cleanRoot(root)
	if err != nil {
		return err
	}
	current, err := ValidateLayout(root)
	if err != nil {
		return err
	}
	// A language that is already in the project keeps the identity it has, even
	// when the caller hands back a configuration built without one: saving the same
	// configuration twice has to produce the same file, and a language does not
	// become a different object because someone rebuilt the struct.
	configuration = carryLanguageIdentities(configuration, current)
	configuration, err = EnsureLanguageIdentities(configuration)
	if err != nil {
		return err
	}
	if err := configuration.Validate(); err != nil {
		return err
	}
	if current.ID != configuration.ID {
		return ErrProjectIdentityChanged
	}
	temporary, err := os.CreateTemp(root, ".configuration-*.yaml")
	if err != nil {
		return fmt.Errorf("create temporary ML Project configuration: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set temporary configuration permissions: %w", err)
	}
	if err := Encode(temporary, configuration); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync ML Project configuration: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close ML Project configuration: %w", err)
	}
	if err := replaceProjectFile(temporaryPath, filepath.Join(root, ConfigurationFile)); err != nil {
		return fmt.Errorf("replace ML Project configuration: %w", err)
	}
	return nil
}

// MetadataPath returns the canonical relative YAML path for a metadata object.
func MetadataPath(kind string, id uuid.UUID) (string, error) {
	if !slices.Contains(metadataKinds, kind) {
		return "", fmt.Errorf("unknown metadata kind %q", kind)
	}
	return sourcePath(path.Join("metadata", kind), id, ".yaml")
}

// ModulePath returns the canonical relative BSL path for a module.
func ModulePath(id uuid.UUID) (string, error) { return sourcePath("modules", id, ".bsl") }

// FormPath returns the canonical relative YAML path for a managed form.
// ReportPath returns the canonical relative YAML path for a data composition schema.
// TestPath returns the canonical relative BSL path for a test module.
// AssetPath returns a stable relative resource path while preserving its format extension.
func sourcePath(directory string, id uuid.UUID, extension string) (string, error) {
	if id.IsZero() {
		return "", fmt.Errorf("source UUID must not be zero")
	}
	return path.Join(directory, id.String()+extension), nil
}

// ObjectName reports whether a name may be a folder on disk. Every metadata
// name already has to be an identifier - letters, digits and underscore - so
// nothing has to be escaped, and names are already unique within a kind with
// case folded, so two folders cannot collide on a file system that ignores
// case.
func ObjectName(name string) error { return folderName("object", name) }

// SubordinateName reports whether the name of something an object owns - a
// command, a form, a template - may be a folder inside that object's folder.
// The rule is the object's own, and for the same reason: such a name is an
// identifier too, and unique among its siblings with case folded.
func SubordinateName(name string) error { return folderName("name", name) }

const maxFolderNameBytes = 255

func folderName(what, name string) error {
	if name == "" {
		return fmt.Errorf("%s must not be empty", what)
	}
	// The file system's limit, not ours: a path component is at most 255
	// bytes on ext4 and APFS alike. It is counted in bytes on purpose - a
	// Cyrillic letter takes two - so the model's 255 characters do not all fit,
	// and a name that does not is refused here rather than failing to save.
	// The configurations being moved stay far below it: the longest name is 96
	// characters, 192 bytes.
	if len(name) > maxFolderNameBytes {
		return fmt.Errorf("%s must not be longer than %d bytes", what, maxFolderNameBytes)
	}
	for index, symbol := range name {
		switch {
		case symbol == '_':
		case unicode.IsLetter(symbol):
		case unicode.IsDigit(symbol) && index > 0:
		default:
			return fmt.Errorf("%s %q is not an identifier", what, name)
		}
	}
	return nil
}

// ObjectDirectory returns the folder one object keeps, named by the object
// itself - for a kind that groups subordinate entities there (ObjectFolderKinds)
// or one that only keeps files (NamedFolderKinds).
//
// The name rather than the identifier, because this is what a developer reads.
// The prototype's own export is laid out the same way - names in the paths,
// identifiers inside the files - and a developer coming from it opens the
// repository and sees a tree they already know. Renaming an object renames the
// folder and changes nothing in the database: a table is named by the
// identifier, which the rename does not touch.
func ObjectDirectory(kind, name string) (string, error) {
	if subordinate, ok := subordinateObjectKinds[kind]; ok {
		// The owner may itself be subordinate, so only what follows the last
		// dot is the object's own name.
		index := strings.LastIndexByte(name, '.')
		if index < 0 {
			return "", fmt.Errorf("%s %q must be named as <owner>.<name>", kind, name)
		}
		owner, own := name[:index], name[index+1:]
		if err := ObjectName(own); err != nil {
			return "", err
		}
		ownerDirectory, err := ObjectDirectory(subordinate.owner, owner)
		if err != nil {
			return "", err
		}
		return path.Join(ownerDirectory, subordinate.collection, own), nil
	}
	if _, keepsFolder := namedFolderKinds[kind]; !keepsFolder && !slices.Contains(objectFolderKinds, kind) {
		return "", fmt.Errorf("kind %q does not use a per-object folder", kind)
	}
	if err := ObjectName(name); err != nil {
		return "", err
	}
	return path.Join("metadata", kind, name), nil
}

// subordinateKind says where the objects of a subordinate kind lie: in a
// collection folder inside an object of the owner kind.
type subordinateKind struct {
	owner      string
	collection string
}

// subordinateObjectKinds are the kinds whose objects lie inside an object of
// another kind rather than directly under metadata/. The prototype's export
// lays out a subordinate object that is an object of development in its own
// right the same way - a folder in its owner's collection, beside the owner's
// description - and a table of an external data source is one: it has its own
// modules, forms, commands and templates, exactly as a catalog does.
//
// Such an object is addressed like any other, by a kind and a name, and the
// name carries the owner: Склад.Товары is the table Товары of the source
// Склад. The address is what forms, modules and the published snapshot know
// an object by, so none of them needed a new field; only the translation of
// an address into a folder, here, knows the folder is deeper.
var subordinateObjectKinds = map[string]subordinateKind{
	"external-data-source-tables": {owner: "external-data-sources", collection: "tables"},
	"external-data-source-cubes":  {owner: "external-data-sources", collection: "cubes"},
	// A table of a cube's dimension lies inside the cube, two levels below
	// the source: Склад.Продажи.Товары is the dimension table Товары of the
	// cube Продажи of the source Склад.
	"external-data-source-dimension-tables": {owner: "external-data-source-cubes", collection: "dimension-tables"},
}

// SubordinateObjectKinds returns the kinds whose objects lie inside another
// object, sorted.
func SubordinateObjectKinds() []string {
	result := make([]string, 0, len(subordinateObjectKinds))
	for kind := range subordinateObjectKinds {
		result = append(result, kind)
	}
	slices.Sort(result)
	return result
}

// IsSubordinateObjectKind reports whether a kind's objects lie inside another
// object.
func IsSubordinateObjectKind(kind string) bool {
	_, ok := subordinateObjectKinds[kind]
	return ok
}

// SubordinateCollections returns the collection folders an object of a kind
// keeps for subordinate objects, sorted: "tables" for an external data source.
func SubordinateCollections(kind string) []string {
	var result []string
	for _, subordinate := range subordinateObjectKinds {
		if subordinate.owner == kind {
			result = append(result, subordinate.collection)
		}
	}
	slices.Sort(result)
	return result
}

// SubordinateObjectKind returns the kind of the objects an object of the owner
// kind keeps in one of its collections: external-data-source-tables for the
// tables of an external data source.
func SubordinateObjectKind(owner, collection string) (string, bool) {
	for kind, subordinate := range subordinateObjectKinds {
		if subordinate.owner == owner && subordinate.collection == collection {
			return kind, true
		}
	}
	return "", false
}

// IsObjectFolderKind reports whether an object of a kind keeps an object
// folder - its description, its modules and its forms, commands and templates
// - wherever that folder lies.
func IsObjectFolderKind(kind string) bool {
	return slices.Contains(objectFolderKinds, kind) || IsSubordinateObjectKind(kind)
}

// SplitObjectPath reads a path of the project as the object it belongs to and
// what lies below that object's folder. It is the one place that knows how
// deep an object's folder is, so that nothing reading paths has to count
// segments: metadata/catalogs/Товары/forms/Ф/form.yaml is the catalog Товары
// and forms/Ф/form.yaml below it; metadata/external-data-sources/Склад/tables/
// Товары/МодульОбъекта.bsl is the table Склад.Товары and МодульОбъекта.bsl.
//
// The path is walked as deep as it goes: a collection of subordinate objects
// followed by an object's name moves the address into that object. A path
// that stops at a collection folder belongs to the owner, with the collection
// as what lies below. ok is false for anything that is not under an object's
// folder at all.
func SplitObjectPath(relative string) (kind, name string, rest []string, ok bool) {
	parts := strings.Split(relative, "/")
	if len(parts) < 3 || parts[0] != "metadata" || ObjectName(parts[2]) != nil {
		return "", "", nil, false
	}
	kind, name, rest = parts[1], parts[2], parts[3:]
	if _, keepsFolder := namedFolderKinds[kind]; !keepsFolder && !slices.Contains(objectFolderKinds, kind) {
		return "", "", nil, false
	}
	for len(rest) >= 2 {
		moved := false
		for subordinateName, subordinate := range subordinateObjectKinds {
			if subordinate.owner == kind && rest[0] == subordinate.collection && ObjectName(rest[1]) == nil {
				kind, name, rest = subordinateName, name+"."+rest[1], rest[2:]
				moved = true
				break
			}
		}
		if !moved {
			break
		}
	}
	return kind, name, rest, true
}

// ObjectMetadataPath returns the fixed-name description file inside an
// object's own folder.
func ObjectMetadataPath(kind, name string) (string, error) {
	directory, err := ObjectDirectory(kind, name)
	if err != nil {
		return "", err
	}
	return path.Join(directory, ObjectMetadataFile), nil
}

// ObjectModulePath returns one of an object's own BSL modules, named by the
// role it plays rather than by an identifier of its own.
//
// A module has no identity apart from that role: an object has at most one
// object module, and calling the file after the role is what the prototype's
// own export does. It also means the module needs no declaration - the file
// is the declaration, and there is no second place to keep in step with it.
func ObjectModulePath(kind, name, roleFile string) (string, error) {
	directory, err := ObjectDirectory(kind, name)
	if err != nil {
		return "", err
	}
	if !slices.Contains(moduleRoleFiles, roleFile) {
		return "", fmt.Errorf("%q is not a module role of an object", roleFile)
	}
	return path.Join(directory, roleFile), nil
}

// ObjectCommandDirectory returns the folder of one command of an object.
// A command keeps a folder rather than a bare file for the same reason a
// template does: its module is one of the files it may own, not the whole of
// it, and an object has several commands whose modules would otherwise all
// want the same name.
func ObjectCommandDirectory(kind, name, command string) (string, error) {
	directory, err := ObjectDirectory(kind, name)
	if err != nil {
		return "", err
	}
	if err := SubordinateName(command); err != nil {
		return "", fmt.Errorf("command %w", err)
	}
	return path.Join(directory, "commands", command), nil
}

// ObjectCommandModulePath returns the module that runs one command.
func ObjectCommandModulePath(kind, name, command string) (string, error) {
	directory, err := ObjectCommandDirectory(kind, name, command)
	if err != nil {
		return "", err
	}
	return path.Join(directory, CommandModuleFile), nil
}

// ObjectFormDirectory returns the folder of one of an object's own managed
// forms, named after the form.
//
// A form keeps a folder rather than a bare file because it owns more than its
// description: the module that runs it lies beside it under a name of its own.
func ObjectFormDirectory(kind, name, form string) (string, error) {
	directory, err := ObjectDirectory(kind, name)
	if err != nil {
		return "", err
	}
	if err := SubordinateName(form); err != nil {
		return "", fmt.Errorf("form %w", err)
	}
	return path.Join(directory, "forms", form), nil
}

// ObjectFormPath returns the description of one of an object's own managed
// forms, found by the form's name rather than by an identifier.
//
// The form keeps an identifier of its own inside that description - roles, the
// portal and ML App refer to a form by it - but it is not what finds the file.
// The prototype's export works the same way: a form carries both a uuid and a
// name, and the name is what the folder is called.
func ObjectFormPath(kind, name, form string) (string, error) {
	directory, err := ObjectFormDirectory(kind, name, form)
	if err != nil {
		return "", err
	}
	return path.Join(directory, FormMetadataFile), nil
}

// ObjectFormModulePath returns the module of one of an object's own managed
// forms, beside the form itself.
//
// Like every other module, it carries no identifier: the file lying under the
// name of its role, in the folder of what owns it, is the whole declaration.
func ObjectFormModulePath(kind, name, form string) (string, error) {
	directory, err := ObjectFormDirectory(kind, name, form)
	if err != nil {
		return "", err
	}
	return path.Join(directory, FormModuleFile), nil
}

// CommonFormDirectory returns the folder of one common form - a form that
// belongs to no object - named after the form.
//
// A common form keeps a folder for the same reason an object's form does: its
// module lies beside it, under the name of its role, and a bare file has
// nowhere to put one.
func CommonFormDirectory(name string) (string, error) {
	if err := ObjectName(name); err != nil {
		return "", fmt.Errorf("common form %w", err)
	}
	return path.Join("metadata", "common-forms", name), nil
}

// CommonFormPath returns the description of one common form.
func CommonFormPath(name string) (string, error) {
	directory, err := CommonFormDirectory(name)
	if err != nil {
		return "", err
	}
	return path.Join(directory, FormMetadataFile), nil
}

// CommonFormModulePath returns the module of one common form.
func CommonFormModulePath(name string) (string, error) {
	directory, err := CommonFormDirectory(name)
	if err != nil {
		return "", err
	}
	return path.Join(directory, FormModuleFile), nil
}

// CommonTemplateContentPath returns one file of a common template's content.
//
// A common template's own folder is the template folder: nothing owns the
// template, so the content lies beside its description rather than one level
// down under an object's templates/. The file is still named after the kind of
// template, exactly as an object's template names it.
func CommonTemplateContentPath(template, file string) (string, error) {
	directory, err := ObjectDirectory("common-templates", template)
	if err != nil {
		return "", err
	}
	if err := plainFileName("template content", file); err != nil {
		return "", err
	}
	return path.Join(directory, file), nil
}

// CommonPictureImagePath returns one image of a common picture. Which density
// the image stands for is what it is called, so the caller passes the name and
// the metadata layer, which knows the ladder, decides whether it is one.
func CommonPictureImagePath(picture, file string) (string, error) {
	directory, err := ObjectDirectory("common-pictures", picture)
	if err != nil {
		return "", err
	}
	if err := plainFileName("picture image", file); err != nil {
		return "", err
	}
	return path.Join(directory, file), nil
}

// plainFileName rejects anything that would leave the folder it is meant to
// lie in.
func plainFileName(what, file string) error {
	if file == "" || file == "." || file == ".." || strings.ContainsAny(file, `/\`) {
		return fmt.Errorf("%s must be a plain name", what)
	}
	return nil
}

// TemplateResourcesDirectory is the folder an HTML template keeps its
// resources in - the images its documents show - beside the documents
// themselves. It is one folder for all the languages, holding files and
// nothing deeper, as the prototype keeps it (13 templates of the three
// configurations being moved, common ones and those of data processors).
const TemplateResourcesDirectory = "_files"

// RouteDirectory is the folder a business process keeps the pictures of the
// items of its route map in, those drawn from a file of their own: a folder
// per item, named by it - the items of a map are named apart - holding its
// picture, as the prototype keeps the pictures of a drawing's items
// (Items/<item>/Picture.png).
const RouteDirectory = "route"

// RoutePicturePath reports whether the steps below a business process's
// folder are a place the pictures of its route map may take. Whether the
// item exists and draws that file is checked where the metadata is read.
func RoutePicturePath(steps []string, directory bool, pictureFile func(string) bool) bool {
	if len(steps) == 0 || steps[0] != RouteDirectory {
		return false
	}
	switch len(steps) {
	case 1:
		return directory
	case 2:
		return directory && SubordinateName(steps[1]) == nil
	case 3:
		return !directory && SubordinateName(steps[1]) == nil && pictureFile(steps[2])
	}
	return false
}

// SchemaItemsDirectory is the folder a graphical schema keeps the pictures of
// its items in: a folder per item, named by the item, holding the item's
// picture under the name the schema gives it, and nothing deeper - as the
// prototype keeps them beside the schema (Items/<item>/Picture.png, erp 2).
// Our model of the schema's content has no place for an image, and an image
// is not text to be written into one.
const SchemaItemsDirectory = "items"

// SchemaItemPath reports whether the steps below a template's folder are a
// place the pictures of a graphical schema's items may take: the folder of
// the items, the folder of one item, or a file in it.
func SchemaItemPath(steps []string, directory bool) bool {
	if len(steps) == 0 || steps[0] != SchemaItemsDirectory {
		return false
	}
	switch len(steps) {
	case 1:
		return directory
	case 2:
		return directory && TemplateResource(steps[1])
	case 3:
		return !directory && TemplateResource(steps[1]) && TemplateResource(steps[2])
	}
	return false
}

// HelpDirectory is the folder the help of an object, of a form of it or of a
// common form lies in, beside the owner's description: one page per language,
// named by the language - ru.html - and the pictures the pages show in one
// folder for all the languages, as the prototype keeps it (Ext/Help, 2640,
// 1248 and 911 helps in the configurations being moved). Which languages the
// help has is the pages it holds: the list the prototype writes beside them
// names the same pages in every one of the 5470 helps of five exports.
const HelpDirectory = "help"

// HelpPage reports whether a name is a page of help, and which language it is
// in. The language is not checked against the configuration's: the prototype
// keeps texts in a language the configuration does not declare (erp, acc, sb: en).
func HelpPage(file string) (string, bool) {
	code, found := strings.CutSuffix(file, ".html")
	if !found || !LanguageCodeShape(code) {
		return "", false
	}
	return code, true
}

// HelpPath reports whether the steps below an owner's folder are a place the
// owner's help may take: the help folder itself, a page in it, the folder of
// its pictures or a picture there.
func HelpPath(steps []string, directory bool) bool {
	if len(steps) == 0 || steps[0] != HelpDirectory {
		return false
	}
	switch len(steps) {
	case 1:
		return directory
	case 2:
		if directory {
			return steps[1] == TemplateResourcesDirectory
		}
		_, ok := HelpPage(steps[1])
		return ok
	case 3:
		return !directory && steps[1] == TemplateResourcesDirectory && TemplateResource(steps[2])
	}
	return false
}

// LanguageCodeShape accepts the shape of a language code, not the list of
// them: which languages a configuration has is decided by the configuration.
//
// The help says only «a string, for example "en"», and the configurations
// saved by the configurator hold ru1, ja and sq (mdclasses). What a code must
// not hold comes from where it is used: НСтр writes «code = 'text'; ...», so a
// code holds no «=», «;» or quote, and a page of help is named «code.html», so
// it holds no separator of a path and no dot. Nothing else is refused, and no
// ceiling on the length is set: the help names none (03.10.2026).
func LanguageCodeShape(code string) bool {
	if code == "" {
		return false
	}
	for _, symbol := range code {
		if unicode.IsSpace(symbol) || unicode.IsControl(symbol) || strings.ContainsRune("=;'\"/\\.", symbol) {
			return false
		}
	}
	return true
}

// TemplateResource reports whether a name can be a file among the resources
// of an HTML template. The prototype keeps them under the names they were
// saved with - spaces, commas and brackets among them - so only what cannot
// be a file of that folder is refused.
func TemplateResource(file string) bool {
	return plainFileName("template resource", file) == nil && !strings.HasPrefix(file, ".")
}

// ObjectTemplateDirectory returns the folder holding the content of one of an
// object's own templates, named after the template.
//
// A template keeps a folder rather than a single file because its content is
// not always one file: an HTML template holds one per language, and a template
// may legitimately hold none at all while its editor has not been written yet.
func ObjectTemplateDirectory(kind, name, template string) (string, error) {
	directory, err := ObjectDirectory(kind, name)
	if err != nil {
		return "", err
	}
	if err := SubordinateName(template); err != nil {
		return "", fmt.Errorf("template %w", err)
	}
	return path.Join(directory, "templates", template), nil
}

// ObjectTemplateContentPath returns one file of a template's content. The file
// is still named after the kind of template: what changed is the folder it
// lies in, not what a template's content is called inside it.
func ObjectTemplateContentPath(kind, name, template, file string) (string, error) {
	directory, err := ObjectTemplateDirectory(kind, name, template)
	if err != nil {
		return "", err
	}
	if err := plainFileName("template content", file); err != nil {
		return "", err
	}
	return path.Join(directory, file), nil
}

// ObjectFolderSourcePaths walks every per-object folder (ObjectFolderKinds)
// and returns the source files it holds: the object's own description, its
// module(s), its managed forms and the module of each of its commands —
// sorted canonical relative paths.
//
// Template content is deliberately left out. Everything that calls this reads
// what it gets as source text, and a template holds a spreadsheet, an archive
// or a component - listing it would put binary content into code search and
// into the module index. Publication walks the tree itself and does see it. Missing kind directories are treated as empty, matching
// loadKind's tolerance for an ML Project that has not used a kind yet.
func ObjectFolderSourcePaths(root string) ([]string, error) {
	var paths []string
	// A kind that keeps only files keeps them in a folder all the same, and
	// its modules are read from there like any other.
	for _, kind := range NamedFolderKinds() {
		directory := filepath.Join(root, "metadata", kind)
		entries, err := os.ReadDir(directory)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read metadata %s: %w", kind, err)
		}
		named, _ := NamedFolderFiles(kind)
		for _, entry := range entries {
			if !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
				continue
			}
			files, err := os.ReadDir(filepath.Join(directory, entry.Name()))
			if err != nil {
				return nil, fmt.Errorf("read metadata %s %s: %w", kind, entry.Name(), err)
			}
			for _, file := range files {
				if file.IsDir() || file.Type()&fs.ModeSymlink != 0 || !slices.Contains(named, file.Name()) {
					continue
				}
				paths = append(paths, path.Join("metadata", kind, entry.Name(), file.Name()))
			}
		}
	}
	for _, kind := range objectFolderKinds {
		directory := filepath.Join(root, "metadata", kind)
		objectEntries, err := os.ReadDir(directory)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read metadata %s: %w", kind, err)
		}
		for _, objectEntry := range objectEntries {
			if !objectEntry.IsDir() || objectEntry.Type()&fs.ModeSymlink != 0 {
				continue
			}
			nested, err := objectFolderSources(root, path.Join("metadata", kind, objectEntry.Name()))
			if err != nil {
				return nil, err
			}
			paths = append(paths, nested...)
		}
	}
	// An object that lies inside another is walked the same way, from its
	// own folder: a table of a source keeps its modules and forms exactly as
	// a catalog does, one level deeper.
	for _, kind := range SubordinateObjectKinds() {
		objects, err := subordinateObjectDirectories(root, kind)
		if err != nil {
			return nil, err
		}
		for _, directory := range objects {
			nested, err := objectFolderSources(root, directory)
			if err != nil {
				return nil, err
			}
			paths = append(paths, nested...)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// subordinateObjectDirectories lists the folders of every object of a
// subordinate kind, as project-relative paths: every folder in the collection
// of every object of the owner kind, however deep the owner itself lies.
func subordinateObjectDirectories(root, kind string) ([]string, error) {
	subordinate := subordinateObjectKinds[kind]
	var owners []string
	if IsSubordinateObjectKind(subordinate.owner) {
		nested, err := subordinateObjectDirectories(root, subordinate.owner)
		if err != nil {
			return nil, err
		}
		owners = nested
	} else {
		entries, err := os.ReadDir(filepath.Join(root, "metadata", subordinate.owner))
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("read metadata %s: %w", subordinate.owner, err)
		}
		for _, entry := range entries {
			if entry.IsDir() && entry.Type()&fs.ModeSymlink == 0 {
				owners = append(owners, path.Join("metadata", subordinate.owner, entry.Name()))
			}
		}
	}
	var result []string
	for _, owner := range owners {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(owner), subordinate.collection))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read %s/%s: %w", owner, subordinate.collection, err)
		}
		for _, entry := range entries {
			if entry.IsDir() && entry.Type()&fs.ModeSymlink == 0 {
				result = append(result, path.Join(owner, subordinate.collection, entry.Name()))
			}
		}
	}
	return result, nil
}

// objectFolderSources lists the source files of one object folder, given as a
// project-relative path: the files directly inside it, and the files of its
// forms and commands. Templates are left out - their content is not source -
// and so are collections of subordinate objects, which are walked as objects
// of their own.
func objectFolderSources(root, relative string) ([]string, error) {
	directory := filepath.Join(root, filepath.FromSlash(relative))
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", relative, err)
	}
	var paths []string
	for _, entry := range entries {
		if entry.Type()&fs.ModeSymlink != 0 {
			continue
		}
		if !entry.IsDir() {
			paths = append(paths, path.Join(relative, entry.Name()))
			continue
		}
		// A form is a folder holding its description, a command a folder
		// holding its module, a recalculation a folder holding its record set
		// module.
		if entry.Name() != "forms" && entry.Name() != "commands" && entry.Name() != RecalculationsDirectory {
			continue
		}
		items, err := os.ReadDir(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s/%s: %w", relative, entry.Name(), err)
		}
		for _, item := range items {
			if !item.IsDir() || item.Type()&fs.ModeSymlink != 0 {
				continue
			}
			files, err := os.ReadDir(filepath.Join(directory, entry.Name(), item.Name()))
			if err != nil {
				return nil, fmt.Errorf("read %s/%s/%s: %w", relative, entry.Name(), item.Name(), err)
			}
			for _, file := range files {
				if file.IsDir() || file.Type()&fs.ModeSymlink != 0 {
					continue
				}
				paths = append(paths, path.Join(relative, entry.Name(), item.Name(), file.Name()))
			}
		}
	}
	return paths, nil
}

func cleanRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("%w: project path is empty", ErrInvalidLayout)
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("%w: resolve project path: %v", ErrInvalidLayout, err)
	}
	return filepath.Clean(absolute), nil
}

func requirePath(path string, directory bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("%w: inspect %q: %v", ErrInvalidLayout, path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %q must not be a symbolic link", ErrInvalidLayout, path)
	}
	if directory && !info.IsDir() {
		return fmt.Errorf("%w: %q must be a directory", ErrInvalidLayout, path)
	}
	if !directory && !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %q must be a regular file", ErrInvalidLayout, path)
	}
	return nil
}

func readConfiguration(path string) (Project, error) {
	file, err := os.Open(path)
	if err != nil {
		return Project{}, fmt.Errorf("open configuration %q: %w", path, err)
	}
	defer file.Close()
	return DecodeSource(path, file)
}

// carryLanguageIdentities copies identities from the configuration on disk onto a
// configuration that lacks them, matching by language code - the only thing the two
// have in common when the caller built the value in code.
func carryLanguageIdentities(configuration, current Project) Project {
	known := make(map[string]uuid.UUID, len(current.Languages))
	for _, language := range current.Languages {
		known[strings.ToLower(language.Code)] = language.ID
	}
	configuration.Languages = append([]Language(nil), configuration.Languages...)
	for index := range configuration.Languages {
		if !configuration.Languages[index].ID.IsZero() {
			continue
		}
		if id, ok := known[strings.ToLower(configuration.Languages[index].Code)]; ok {
			configuration.Languages[index].ID = id
		}
	}
	return configuration
}
