package metadata

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Load reads and cross-validates all currently supported application metadata.
func Load(root string) (*Catalog, error) {
	return load(root, true)
}

// Role editing must also work when an existing role has a dangling reference.
// Only the editor-schema path skips roles; runtime Load always validates them.
// A load with the roles is strict, one without them is a reading for editing.
func load(root string, includeRoles bool) (*Catalog, error) {
	return read(root, includeRoles, includeRoles)
}

// read is the load itself. A strict one refuses a project that refers to an
// object it does not have; one that is not collects such references
// (UnresolvedReferences), as the reading for editing must to open a project
// so that it can be fixed (ML-STUDIO.md).
func read(root string, includeRoles, strict bool) (*Catalog, error) {
	configuration, err := project.ValidateLayout(root)
	if err != nil {
		return nil, err
	}
	if err := validateMetadataDirectories(root); err != nil {
		return nil, err
	}
	catalog := &Catalog{Project: configuration, rolesLoaded: includeRoles}
	if err := catalog.loadRootInterface(root); err != nil {
		return nil, err
	}
	for _, item := range kindInfos {
		if item.add == nil || item.kind == RoleKind && !includeRoles {
			continue
		}
		if err := catalog.loadSources(root, item, configuration); err != nil {
			return nil, err
		}
	}
	if err := catalog.indexAndValidate(root); err != nil {
		return nil, err
	}
	if err := catalog.checkFormAttributes(); err != nil {
		return nil, err
	}
	catalog.collectRemnants()
	if strict {
		if err := catalog.refuseUnresolved(); err != nil {
			return nil, err
		}
	}
	if err := catalog.collectHelpPages(root); err != nil {
		return nil, err
	}
	return catalog, nil
}

// collectHelpPages records every page of help the project keeps - of the
// configuration, of an object, a form, a subsystem - by the language it is in.
// The folders were checked while the project was read; here only their pages
// are listed, for the note on a page in a language the configuration does not
// declare (NoteHelpInUndeclaredLanguage).
func (catalog *Catalog) collectHelpPages(root string) error {
	catalog.helpPages = nil
	for _, start := range []string{project.HelpDirectory, "metadata"} {
		base := filepath.Join(root, start)
		if _, err := os.Stat(base); err != nil {
			continue
		}
		err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || filepath.Base(filepath.Dir(path)) != project.HelpDirectory {
				return nil
			}
			code, ok := project.HelpPage(entry.Name())
			if !ok {
				return nil
			}
			relative, _ := filepath.Rel(root, path)
			catalog.helpPages = append(catalog.helpPages, helpPage{where: filepath.ToSlash(relative), code: code})
			return nil
		})
		if err != nil {
			return fmt.Errorf("list help pages: %w", err)
		}
	}
	return nil
}

// helpPage is one page of help as the project keeps it.
type helpPage struct {
	where string
	code  string
}

// NewCatalogSnapshot validates already decoded metadata, for example from a publication package.
func NewCatalogSnapshot(configuration project.Project, constants []Constant, enumerations []Enumeration, definedTypes []DefinedTypeObject, catalogs []CatalogDefinition, documents []DocumentDefinition, informationRegisters []InformationRegisterDefinition) (*Catalog, error) {
	return NewCatalogSnapshotWithAccumulationRegisters(configuration, constants, enumerations, definedTypes, catalogs, documents, informationRegisters, nil)
}

// NewCatalogSnapshotWithAccumulationRegisters is the compatibility constructor
// for metadata without project roles.
func NewCatalogSnapshotWithAccumulationRegisters(configuration project.Project, constants []Constant, enumerations []Enumeration, definedTypes []DefinedTypeObject, catalogs []CatalogDefinition, documents []DocumentDefinition, informationRegisters []InformationRegisterDefinition, accumulationRegisters []AccumulationRegisterDefinition) (*Catalog, error) {
	return NewCatalogSnapshotWithRoles(configuration, constants, enumerations, definedTypes, catalogs, documents, informationRegisters, accumulationRegisters, nil)
}

// NewCatalogSnapshotWithRoles is the compatibility constructor for metadata
// without project subsystems.
func NewCatalogSnapshotWithRoles(configuration project.Project, constants []Constant, enumerations []Enumeration, definedTypes []DefinedTypeObject, catalogs []CatalogDefinition, documents []DocumentDefinition, informationRegisters []InformationRegisterDefinition, accumulationRegisters []AccumulationRegisterDefinition, roles []RoleDefinition) (*Catalog, error) {
	return NewCatalogSnapshotWithSubsystems(configuration, constants, enumerations, definedTypes, catalogs, documents, informationRegisters, accumulationRegisters, roles, nil)
}

// NewCatalogSnapshotWithSubsystems is the compatibility constructor for
// metadata without project session parameters.
func NewCatalogSnapshotWithSubsystems(configuration project.Project, constants []Constant, enumerations []Enumeration, definedTypes []DefinedTypeObject, catalogs []CatalogDefinition, documents []DocumentDefinition, informationRegisters []InformationRegisterDefinition, accumulationRegisters []AccumulationRegisterDefinition, roles []RoleDefinition, subsystems []SubsystemDefinition) (*Catalog, error) {
	return NewCatalogSnapshotWithSessionParameters(configuration, constants, enumerations, definedTypes, catalogs, documents, informationRegisters, accumulationRegisters, roles, subsystems, nil)
}

// NewCatalogSnapshotWithSessionParameters is the compatibility constructor
// for metadata without project common attributes.
func NewCatalogSnapshotWithSessionParameters(configuration project.Project, constants []Constant, enumerations []Enumeration, definedTypes []DefinedTypeObject, catalogs []CatalogDefinition, documents []DocumentDefinition, informationRegisters []InformationRegisterDefinition, accumulationRegisters []AccumulationRegisterDefinition, roles []RoleDefinition, subsystems []SubsystemDefinition, sessionParameters []SessionParameter) (*Catalog, error) {
	return NewCatalogSnapshotWithCommonAttributes(configuration, constants, enumerations, definedTypes, catalogs, documents, informationRegisters, accumulationRegisters, roles, subsystems, sessionParameters, nil)
}

// NewCatalogSnapshotWithCommonAttributes is the compatibility constructor
// for metadata without project common modules.
func NewCatalogSnapshotWithCommonAttributes(configuration project.Project, constants []Constant, enumerations []Enumeration, definedTypes []DefinedTypeObject, catalogs []CatalogDefinition, documents []DocumentDefinition, informationRegisters []InformationRegisterDefinition, accumulationRegisters []AccumulationRegisterDefinition, roles []RoleDefinition, subsystems []SubsystemDefinition, sessionParameters []SessionParameter, commonAttributes []CommonAttributeDefinition) (*Catalog, error) {
	return NewCatalogSnapshotWithCommonModules(configuration, constants, enumerations, definedTypes, catalogs, documents, informationRegisters, accumulationRegisters, roles, subsystems, sessionParameters, commonAttributes, nil)
}

// NewCatalogSnapshotWithCommonModules is the compatibility constructor for
// metadata without project event subscriptions.
func NewCatalogSnapshotWithCommonModules(configuration project.Project, constants []Constant, enumerations []Enumeration, definedTypes []DefinedTypeObject, catalogs []CatalogDefinition, documents []DocumentDefinition, informationRegisters []InformationRegisterDefinition, accumulationRegisters []AccumulationRegisterDefinition, roles []RoleDefinition, subsystems []SubsystemDefinition, sessionParameters []SessionParameter, commonAttributes []CommonAttributeDefinition, commonModules []CommonModuleDefinition) (*Catalog, error) {
	return NewCatalogSnapshotWithEventSubscriptions(configuration, constants, enumerations, definedTypes, catalogs, documents, informationRegisters, accumulationRegisters, roles, subsystems, sessionParameters, commonAttributes, commonModules, nil)
}

// NewCatalogSnapshotWithEventSubscriptions validates decoded metadata, role
// object/field references, subsystem membership, common attribute
// propagation and event subscription references. Form command references
// require the full RuntimeSnapshot.
func NewCatalogSnapshotWithEventSubscriptions(configuration project.Project, constants []Constant, enumerations []Enumeration, definedTypes []DefinedTypeObject, catalogs []CatalogDefinition, documents []DocumentDefinition, informationRegisters []InformationRegisterDefinition, accumulationRegisters []AccumulationRegisterDefinition, roles []RoleDefinition, subsystems []SubsystemDefinition, sessionParameters []SessionParameter, commonAttributes []CommonAttributeDefinition, commonModules []CommonModuleDefinition, eventSubscriptions []EventSubscriptionDefinition) (*Catalog, error) {
	return RuntimeSnapshot{Project: configuration, Constants: constants, Enumerations: enumerations, DefinedTypes: definedTypes,
		Catalogs: catalogs, Documents: documents, InformationRegisters: informationRegisters, AccumulationRegisters: accumulationRegisters,
		Roles: roles, Subsystems: subsystems, SessionParameters: sessionParameters, CommonAttributes: commonAttributes,
		CommonModules: commonModules, EventSubscriptions: eventSubscriptions}.catalogOf(nil)
}

// catalogOf validates what the snapshot holds as a catalog. outside are the
// objects of the project the snapshot names and holds no description of
// (RuntimeSnapshot.Outside): a right a role gives on one and a register a
// document writes into are kept, not refused.
func (snapshot RuntimeSnapshot) catalogOf(outside map[uuid.UUID]Kind) (*Catalog, error) {
	configuration := snapshot.Project
	result := &Catalog{
		Project: configuration, Constants: slices.Clone(snapshot.Constants), Enumerations: slices.Clone(snapshot.Enumerations),
		DefinedTypes: slices.Clone(snapshot.DefinedTypes), Catalogs: slices.Clone(snapshot.Catalogs), Documents: slices.Clone(snapshot.Documents),
		InformationRegisters:  slices.Clone(snapshot.InformationRegisters),
		AccumulationRegisters: slices.Clone(snapshot.AccumulationRegisters),
		Roles:                 slices.Clone(snapshot.Roles),
		Subsystems:            slices.Clone(snapshot.Subsystems),
		SessionParameters:     slices.Clone(snapshot.SessionParameters),
		CommonAttributes:      slices.Clone(snapshot.CommonAttributes),
		CommonModules:         slices.Clone(snapshot.CommonModules),
		EventSubscriptions:    slices.Clone(snapshot.EventSubscriptions),
		outside:               outside,
	}
	for index := range result.Roles {
		result.Roles[index] = cloneRole(result.Roles[index])
	}
	for index := range result.Subsystems {
		result.Subsystems[index] = cloneSubsystemDefinition(result.Subsystems[index])
	}
	for index := range result.Constants {
		result.Constants[index] = cloneConstant(result.Constants[index])
	}
	for index := range result.CommonModules {
		result.CommonModules[index] = cloneCommonModuleDefinition(result.CommonModules[index])
	}
	for index := range result.EventSubscriptions {
		result.EventSubscriptions[index] = cloneEventSubscriptionDefinition(result.EventSubscriptions[index])
	}
	for index := range result.SessionParameters {
		result.SessionParameters[index] = cloneSessionParameter(result.SessionParameters[index])
	}
	for index := range result.CommonAttributes {
		result.CommonAttributes[index] = cloneCommonAttributeDefinition(result.CommonAttributes[index])
	}
	for index := range result.Enumerations {
		result.Enumerations[index] = cloneEnumeration(result.Enumerations[index])
	}
	for index := range result.DefinedTypes {
		result.DefinedTypes[index] = cloneDefinedType(result.DefinedTypes[index])
	}
	for index := range result.Catalogs {
		result.Catalogs[index] = cloneCatalogDefinition(result.Catalogs[index])
	}
	for index := range result.Documents {
		result.Documents[index] = cloneDocumentDefinition(result.Documents[index])
	}
	for index := range result.ChartsOfCharacteristicTypes {
		result.ChartsOfCharacteristicTypes[index] = cloneChartOfCharacteristicTypes(result.ChartsOfCharacteristicTypes[index])
	}
	for index := range result.ChartsOfAccounts {
		result.ChartsOfAccounts[index] = cloneChartOfAccounts(result.ChartsOfAccounts[index])
	}
	for index := range result.ChartsOfCalculationTypes {
		result.ChartsOfCalculationTypes[index] = cloneChartOfCalculationTypes(result.ChartsOfCalculationTypes[index])
	}
	for index := range result.BusinessProcesses {
		result.BusinessProcesses[index] = cloneBusinessProcess(result.BusinessProcesses[index])
	}
	for index := range result.Tasks {
		result.Tasks[index] = cloneTask(result.Tasks[index])
	}
	for index := range result.ExchangePlans {
		result.ExchangePlans[index] = cloneExchangePlan(result.ExchangePlans[index])
	}
	for index := range result.AccountingRegisters {
		result.AccountingRegisters[index] = cloneAccountingRegister(result.AccountingRegisters[index])
	}
	for index := range result.CalculationRegisters {
		result.CalculationRegisters[index] = cloneCalculationRegister(result.CalculationRegisters[index])
	}
	for index := range result.Reports {
		result.Reports[index] = cloneReport(result.Reports[index])
	}
	for index := range result.DataProcessors {
		result.DataProcessors[index] = cloneDataProcessor(result.DataProcessors[index])
	}
	for index := range result.Numerators {
		result.Numerators[index] = cloneNumerator(result.Numerators[index])
	}
	for index := range result.Sequences {
		result.Sequences[index] = cloneSequence(result.Sequences[index])
	}
	for index := range result.DocumentJournals {
		result.DocumentJournals[index] = cloneDocumentJournal(result.DocumentJournals[index])
	}
	for index := range result.InformationRegisters {
		result.InformationRegisters[index] = cloneInformationRegisterDefinition(result.InformationRegisters[index])
	}
	for index := range result.AccumulationRegisters {
		result.AccumulationRegisters[index] = cloneAccumulationRegisterDefinition(result.AccumulationRegisters[index])
	}
	if err := result.indexAndValidate(""); err != nil {
		return nil, err
	}
	return result, nil
}

// loadSources reads every description of one kind and puts it into the
// catalog. A role is read with the grid of its rights beside it.
func (catalog *Catalog) loadSources(root string, item kindInfo, configuration project.Project) error {
	walk := loadKind
	if keptInFolder(item.kind) {
		walk = loadObjectKind
	}
	return walk(root, item.kind, func(relative string, file *os.File) error {
		var rights io.Reader
		if item.kind == RoleKind {
			id, err := uuid.Parse(strings.TrimSuffix(filepath.Base(relative), ".yaml"))
			if err != nil {
				return err
			}
			grid, err := openRoleRights(root, id)
			if err != nil {
				return err
			}
			if grid != nil {
				defer grid.Close()
				rights = grid
			}
		}
		value, _, err := DecodeSource(relative, file, rights, configuration)
		if err != nil {
			return err
		}
		return item.add(catalog, root, value)
	})
}

func loadKind(root string, kind Kind, decode func(string, *os.File) error) error {
	directory := filepath.Join(root, "metadata", string(kind))
	info, err := os.Lstat(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect metadata %s: %w", kind, err)
	}
	if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("metadata path %q must be a directory without symbolic links", filepath.Join("metadata", string(kind)))
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read metadata %s: %w", kind, err)
	}
	described := map[string]bool{}
	for _, entry := range entries {
		described[entry.Name()] = !entry.IsDir()
	}
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		// A subsystem keeps beside its file a folder of the same name when it
		// has help, as the prototype keeps one beside Имя.xml. The name is the
		// identifier, not the subsystem's own: subsystems lie flat here, and
		// names repeat across branches - 41 in erp.
		// A group of commands keeps one for the same reason when its
		// picture is a file of its own, and so does a subsystem. A role keeps
		// one for the grid of its rights.
		if KeepsIdentifiedFolder(kind) && entry.IsDir() && entry.Type()&fs.ModeSymlink == 0 {
			if err := ValidateIdentifiedFolder(directory, kind, entry.Name(), described[entry.Name()+".yaml"]); err != nil {
				return err
			}
			continue
		}
		if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 || filepath.Ext(entry.Name()) != ".yaml" {
			return fmt.Errorf("unexpected metadata source %q", filepath.Join("metadata", string(kind), entry.Name()))
		}
		if _, err := uuid.Parse(strings.TrimSuffix(entry.Name(), ".yaml")); err != nil {
			return fmt.Errorf("metadata source %q must use a UUID filename: %w", entry.Name(), err)
		}
		relative := filepath.ToSlash(filepath.Join("metadata", string(kind), entry.Name()))
		file, err := os.Open(filepath.Join(directory, entry.Name()))
		if err != nil {
			return fmt.Errorf("open %s: %w", relative, err)
		}
		decodeErr := decode(relative, file)
		closeErr := file.Close()
		if decodeErr != nil {
			return fmt.Errorf("load %s: %w", relative, decodeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close %s: %w", relative, closeErr)
		}
	}
	return nil
}

// openRoleRights opens the grid of one role, or returns nil for a role that
// keeps none. A grid that is a link or a folder is refused rather than
// followed, as every other source of the project is.
func openRoleRights(root string, id uuid.UUID) (*os.File, error) {
	path := filepath.Join(root, "metadata", string(RoleKind), id.String(), RoleRightsFile)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect role %s rights: %w", id, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("role %s keeps its %s as something other than a file", id, RoleRightsFile)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open role %s rights: %w", id, err)
	}
	return file, nil
}

// KeepsIdentifiedFolder says an object of the kind may keep a folder named by
// its identifier beside its description: a subsystem and a group of commands
// for their help and picture, a role for the grid of its rights. The loader
// and Studio's tree both ask here.
func KeepsIdentifiedFolder(kind Kind) bool {
	return kind == SubsystemKind || kind == CommandGroupKind || kind == RoleKind
}

// ValidateIdentifiedFolder checks the folder a subsystem, a group of commands
// or a role keeps beside its description: named by the identifier, standing
// beside a description of that name, and holding what that kind keeps there -
// the help of a subsystem and image files, whose picture is checked once the
// description is read in validateIdentifiedPictureFiles, or the grid of a
// role's rights.
func ValidateIdentifiedFolder(directory string, kind Kind, name string, described bool) error {
	relative := filepath.ToSlash(filepath.Join("metadata", string(kind), name))
	what, short, keeps := "subsystem", "subsystem", "only its help and the file of its picture"
	switch kind {
	case CommandGroupKind:
		what, short, keeps = "group of commands", "group", "only the file of its picture"
	case RoleKind:
		what, short, keeps = "role", "role", "only its "+RoleRightsFile
	}
	if _, err := uuid.Parse(name); err != nil {
		return fmt.Errorf("unexpected metadata source %q: a %s's folder is named by its identifier", relative, what)
	}
	if !described {
		return fmt.Errorf("unexpected metadata source %q: no %s %s.yaml stands beside it", relative, what, name)
	}
	entries, err := os.ReadDir(filepath.Join(directory, name))
	if err != nil {
		return fmt.Errorf("read %s: %w", relative, err)
	}
	for _, entry := range entries {
		switch {
		case entry.Type()&fs.ModeSymlink != 0:
		case kind == RoleKind:
			if entry.Type().IsRegular() && entry.Name() == RoleRightsFile {
				continue
			}
		case kind == SubsystemKind && entry.IsDir() && entry.Name() == project.HelpDirectory:
			if err := validateHelpFolder("subsystem "+name, filepath.Join(directory, name, entry.Name())); err != nil {
				return err
			}
			continue
		case !entry.IsDir() && PictureFile(entry.Name()):
			continue
		}
		return fmt.Errorf("%s keeps %q, and a %s's folder keeps %s", relative, entry.Name(), short, keeps)
	}
	return nil
}

// validateIdentifiedPictureFiles checks, once the descriptions are read, that
// the folder of every subsystem and group of commands holds the file its
// picture draws and no other image.
func (catalog *Catalog) validateIdentifiedPictureFiles(root string) error {
	if root == "" {
		return nil
	}
	check := func(kind Kind, what string, id uuid.UUID, name string, picture *PictureReference) error {
		directory := filepath.Join(root, "metadata", string(kind), id.String())
		if err := requirePictureFile(what+" "+name, directory, picture); err != nil {
			return err
		}
		entries, err := os.ReadDir(directory)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("%s %s: %w", what, name, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() && !picture.drawsFile(entry.Name()) {
				return fmt.Errorf("%s %s keeps %q, which its picture does not draw", what, name, entry.Name())
			}
		}
		return nil
	}
	for _, item := range catalog.Subsystems {
		if err := check(SubsystemKind, "subsystem", item.ID, item.Name, item.Picture); err != nil {
			return err
		}
	}
	for _, item := range catalog.CommandGroups {
		if err := check(CommandGroupKind, "command group", item.ID, item.Name, item.Picture); err != nil {
			return err
		}
	}
	return nil
}

// requirePictureFile checks that the folder of whoever is shown with a
// picture of its own holds that picture's file: a reference to a file nobody
// keeps is a blank place in the interface. The prototype keeps the file of
// every such reference (241 references of five exports, none missing).
//
// A set of variants is a folder instead, checked as the folder of a common
// picture is: it holds images only, and every one its variants name.
func requirePictureFile(owner, directory string, picture *PictureReference) error {
	if picture.fileName() == "" {
		return nil
	}
	entries, err := os.ReadDir(directory)
	if err == nil {
		for _, entry := range entries {
			if !picture.drawsEntry(entry) {
				continue
			}
			if len(picture.Variants) == 0 {
				return nil
			}
			return validatePictureFolder(owner, filepath.Join(directory, entry.Name()), "", PictureImages{Variants: picture.Variants})
		}
	}
	if len(picture.Variants) > 0 {
		return fmt.Errorf("%s is shown with the variants of picture %s, and its folder holds no folder of them", owner, picture.File)
	}
	return fmt.Errorf("%s is shown with picture file %s, which its folder does not hold", owner, picture.File)
}

// loadObjectKind scans metadata/<kind>/ for per-object folders (named after
// the object) and decodes the fixed-name object.yaml inside each one. Unlike
// loadKind, entries are directories, not flat <uuid>.yaml files — this is how
// catalogs, documents and registers group their own description with their
// modules, forms, commands and templates, physically located inside that same
// folder (see validateObjectFileSources).
func loadObjectKind(root string, kind Kind, decode func(string, *os.File) error) error {
	directory := filepath.Join(root, "metadata", string(kind))
	info, err := os.Lstat(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect metadata %s: %w", kind, err)
	}
	if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("metadata path %q must be a directory without symbolic links", filepath.Join("metadata", string(kind)))
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read metadata %s: %w", kind, err)
	}
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		if !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("unexpected metadata source %q", filepath.Join("metadata", string(kind), entry.Name()))
		}
		if err := project.ObjectName(entry.Name()); err != nil {
			return fmt.Errorf("metadata object folder %q: %w", entry.Name(), err)
		}
		relative := filepath.ToSlash(filepath.Join("metadata", string(kind), entry.Name(), "object.yaml"))
		file, err := os.Open(filepath.Join(directory, entry.Name(), "object.yaml"))
		if err != nil {
			return fmt.Errorf("open %s: %w", relative, err)
		}
		decodeErr := decode(relative, file)
		closeErr := file.Close()
		if decodeErr != nil {
			return fmt.Errorf("load %s: %w", relative, decodeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close %s: %w", relative, closeErr)
		}
	}
	return nil
}

func (catalog *Catalog) indexAndValidate(root string) error {
	catalog.unresolved = nil
	catalog.objects = map[uuid.UUID]indexedObject{}
	for _, language := range catalog.Project.Languages {
		catalog.objects[language.ID] = indexedObject{kind: LanguageKind, name: language.Name}
	}
	catalog.formOwnerKinds = map[uuid.UUID]Kind{}
	if err := catalog.propagateCommonAttributes(); err != nil {
		return err
	}
	if err := catalog.indexCommonForms(root); err != nil {
		return err
	}
	sort.Slice(catalog.Roles, func(i, j int) bool { return catalog.Roles[i].ID.String() < catalog.Roles[j].ID.String() })
	catalog.roleByName, catalog.roleByID = make(map[string]int, len(catalog.Roles)), make(map[uuid.UUID]int, len(catalog.Roles))
	sort.Slice(catalog.Subsystems, func(i, j int) bool { return catalog.Subsystems[i].ID.String() < catalog.Subsystems[j].ID.String() })
	catalog.subsystemByName, catalog.subsystemByID = make(map[string]int, len(catalog.Subsystems)), make(map[uuid.UUID]int, len(catalog.Subsystems))
	sort.Slice(catalog.Constants, func(i, j int) bool { return catalog.Constants[i].ID.String() < catalog.Constants[j].ID.String() })
	sort.Slice(catalog.FunctionalOptions, func(i, j int) bool {
		return catalog.FunctionalOptions[i].ID.String() < catalog.FunctionalOptions[j].ID.String()
	})
	catalog.functionalOptionByName, catalog.functionalOptionByID = make(map[string]int, len(catalog.FunctionalOptions)), make(map[uuid.UUID]int, len(catalog.FunctionalOptions))
	sort.Slice(catalog.FunctionalOptionParameters, func(i, j int) bool {
		return catalog.FunctionalOptionParameters[i].ID.String() < catalog.FunctionalOptionParameters[j].ID.String()
	})
	catalog.functionalOptionParameterByName, catalog.functionalOptionParameterByID = make(map[string]int, len(catalog.FunctionalOptionParameters)), make(map[uuid.UUID]int, len(catalog.FunctionalOptionParameters))
	sort.Slice(catalog.FilterCriteria, func(i, j int) bool {
		return catalog.FilterCriteria[i].ID.String() < catalog.FilterCriteria[j].ID.String()
	})
	catalog.filterCriterionByName, catalog.filterCriterionByID = make(map[string]int, len(catalog.FilterCriteria)), make(map[uuid.UUID]int, len(catalog.FilterCriteria))
	sort.Slice(catalog.SettingsStorages, func(i, j int) bool {
		return catalog.SettingsStorages[i].ID.String() < catalog.SettingsStorages[j].ID.String()
	})
	catalog.settingsStorageByName, catalog.settingsStorageByID = make(map[string]int, len(catalog.SettingsStorages)), make(map[uuid.UUID]int, len(catalog.SettingsStorages))
	sort.Slice(catalog.ScheduledJobs, func(i, j int) bool {
		return catalog.ScheduledJobs[i].ID.String() < catalog.ScheduledJobs[j].ID.String()
	})
	catalog.scheduledJobByName, catalog.scheduledJobByID = make(map[string]int, len(catalog.ScheduledJobs)), make(map[uuid.UUID]int, len(catalog.ScheduledJobs))
	sort.Slice(catalog.CommonCommands, func(i, j int) bool {
		return catalog.CommonCommands[i].ID.String() < catalog.CommonCommands[j].ID.String()
	})
	catalog.commonCommandByName, catalog.commonCommandByID = make(map[string]int, len(catalog.CommonCommands)), make(map[uuid.UUID]int, len(catalog.CommonCommands))
	sort.Slice(catalog.CommandGroups, func(i, j int) bool {
		return catalog.CommandGroups[i].ID.String() < catalog.CommandGroups[j].ID.String()
	})
	catalog.commandGroupByName, catalog.commandGroupByID = make(map[string]int, len(catalog.CommandGroups)), make(map[uuid.UUID]int, len(catalog.CommandGroups))
	sort.Slice(catalog.CommonTemplates, func(i, j int) bool {
		return catalog.CommonTemplates[i].ID.String() < catalog.CommonTemplates[j].ID.String()
	})
	catalog.commonTemplateByName, catalog.commonTemplateByID = make(map[string]int, len(catalog.CommonTemplates)), make(map[uuid.UUID]int, len(catalog.CommonTemplates))
	catalog.xdtoPackageByName, catalog.xdtoPackageByID = make(map[string]int, len(catalog.XDTOPackages)), make(map[uuid.UUID]int, len(catalog.XDTOPackages))
	catalog.webServiceByName, catalog.webServiceByID = make(map[string]int, len(catalog.WebServices)), make(map[uuid.UUID]int, len(catalog.WebServices))
	catalog.httpServiceByName, catalog.httpServiceByID = make(map[string]int, len(catalog.HTTPServices)), make(map[uuid.UUID]int, len(catalog.HTTPServices))
	catalog.wsReferenceByName, catalog.wsReferenceByID = make(map[string]int, len(catalog.WSReferences)), make(map[uuid.UUID]int, len(catalog.WSReferences))
	catalog.webSocketClientByName, catalog.webSocketClientByID = make(map[string]int, len(catalog.WebSocketClients)), make(map[uuid.UUID]int, len(catalog.WebSocketClients))
	catalog.externalDataSourceByName, catalog.externalDataSourceByID = make(map[string]int, len(catalog.ExternalDataSources)), make(map[uuid.UUID]int, len(catalog.ExternalDataSources))
	catalog.externalTableByID = map[uuid.UUID]externalTableLocation{}
	catalog.externalCubeByID = map[uuid.UUID]externalCubeLocation{}
	catalog.externalDimensionTableByID = map[uuid.UUID]externalDimensionTableLocation{}
	sort.Slice(catalog.CommonPictures, func(i, j int) bool {
		return catalog.CommonPictures[i].ID.String() < catalog.CommonPictures[j].ID.String()
	})
	catalog.commonPictureByName, catalog.commonPictureByID = make(map[string]int, len(catalog.CommonPictures)), make(map[uuid.UUID]int, len(catalog.CommonPictures))
	sort.Slice(catalog.StyleItems, func(i, j int) bool {
		return catalog.StyleItems[i].ID.String() < catalog.StyleItems[j].ID.String()
	})
	catalog.styleItemByName, catalog.styleItemByID = make(map[string]int, len(catalog.StyleItems)), make(map[uuid.UUID]int, len(catalog.StyleItems))
	sort.Slice(catalog.Styles, func(i, j int) bool {
		return catalog.Styles[i].ID.String() < catalog.Styles[j].ID.String()
	})
	catalog.styleByName, catalog.styleByID = make(map[string]int, len(catalog.Styles)), make(map[uuid.UUID]int, len(catalog.Styles))
	sort.Slice(catalog.SessionParameters, func(i, j int) bool {
		return catalog.SessionParameters[i].ID.String() < catalog.SessionParameters[j].ID.String()
	})
	catalog.sessionParameterByName, catalog.sessionParameterByID = make(map[string]int, len(catalog.SessionParameters)), make(map[uuid.UUID]int, len(catalog.SessionParameters))
	sort.Slice(catalog.CommonAttributes, func(i, j int) bool {
		return catalog.CommonAttributes[i].ID.String() < catalog.CommonAttributes[j].ID.String()
	})
	catalog.commonAttributeByName, catalog.commonAttributeByID = make(map[string]int, len(catalog.CommonAttributes)), make(map[uuid.UUID]int, len(catalog.CommonAttributes))
	sort.Slice(catalog.CommonModules, func(i, j int) bool {
		return catalog.CommonModules[i].ID.String() < catalog.CommonModules[j].ID.String()
	})
	catalog.commonModuleByName, catalog.commonModuleByID = make(map[string]int, len(catalog.CommonModules)), make(map[uuid.UUID]int, len(catalog.CommonModules))
	catalog.commonModuleByModuleID = make(map[uuid.UUID]int, len(catalog.CommonModules))
	sort.Slice(catalog.EventSubscriptions, func(i, j int) bool {
		return catalog.EventSubscriptions[i].ID.String() < catalog.EventSubscriptions[j].ID.String()
	})
	catalog.eventSubscriptionByName, catalog.eventSubscriptionByID = make(map[string]int, len(catalog.EventSubscriptions)), make(map[uuid.UUID]int, len(catalog.EventSubscriptions))
	sort.Slice(catalog.Enumerations, func(i, j int) bool { return catalog.Enumerations[i].ID.String() < catalog.Enumerations[j].ID.String() })
	sort.Slice(catalog.DefinedTypes, func(i, j int) bool { return catalog.DefinedTypes[i].ID.String() < catalog.DefinedTypes[j].ID.String() })
	sort.Slice(catalog.Catalogs, func(i, j int) bool { return catalog.Catalogs[i].ID.String() < catalog.Catalogs[j].ID.String() })
	sort.Slice(catalog.Documents, func(i, j int) bool { return catalog.Documents[i].ID.String() < catalog.Documents[j].ID.String() })
	sort.Slice(catalog.ChartsOfCharacteristicTypes, func(i, j int) bool {
		return catalog.ChartsOfCharacteristicTypes[i].ID.String() < catalog.ChartsOfCharacteristicTypes[j].ID.String()
	})
	sort.Slice(catalog.ChartsOfAccounts, func(i, j int) bool {
		return catalog.ChartsOfAccounts[i].ID.String() < catalog.ChartsOfAccounts[j].ID.String()
	})
	sort.Slice(catalog.ChartsOfCalculationTypes, func(i, j int) bool {
		return catalog.ChartsOfCalculationTypes[i].ID.String() < catalog.ChartsOfCalculationTypes[j].ID.String()
	})
	sort.Slice(catalog.BusinessProcesses, func(i, j int) bool {
		return catalog.BusinessProcesses[i].ID.String() < catalog.BusinessProcesses[j].ID.String()
	})
	sort.Slice(catalog.Tasks, func(i, j int) bool { return catalog.Tasks[i].ID.String() < catalog.Tasks[j].ID.String() })
	sort.Slice(catalog.ExchangePlans, func(i, j int) bool {
		return catalog.ExchangePlans[i].ID.String() < catalog.ExchangePlans[j].ID.String()
	})
	sort.Slice(catalog.AccountingRegisters, func(i, j int) bool {
		return catalog.AccountingRegisters[i].ID.String() < catalog.AccountingRegisters[j].ID.String()
	})
	sort.Slice(catalog.CalculationRegisters, func(i, j int) bool {
		return catalog.CalculationRegisters[i].ID.String() < catalog.CalculationRegisters[j].ID.String()
	})
	sort.Slice(catalog.Reports, func(i, j int) bool { return catalog.Reports[i].ID.String() < catalog.Reports[j].ID.String() })
	sort.Slice(catalog.DataProcessors, func(i, j int) bool {
		return catalog.DataProcessors[i].ID.String() < catalog.DataProcessors[j].ID.String()
	})
	sort.Slice(catalog.Numerators, func(i, j int) bool {
		return catalog.Numerators[i].ID.String() < catalog.Numerators[j].ID.String()
	})
	sort.Slice(catalog.Sequences, func(i, j int) bool {
		return catalog.Sequences[i].ID.String() < catalog.Sequences[j].ID.String()
	})
	sort.Slice(catalog.DocumentJournals, func(i, j int) bool {
		return catalog.DocumentJournals[i].ID.String() < catalog.DocumentJournals[j].ID.String()
	})
	sort.Slice(catalog.InformationRegisters, func(i, j int) bool {
		return catalog.InformationRegisters[i].ID.String() < catalog.InformationRegisters[j].ID.String()
	})
	sort.Slice(catalog.AccumulationRegisters, func(i, j int) bool {
		return catalog.AccumulationRegisters[i].ID.String() < catalog.AccumulationRegisters[j].ID.String()
	})
	catalog.constantByName, catalog.constantByID = make(map[string]int, len(catalog.Constants)), make(map[uuid.UUID]int, len(catalog.Constants))
	catalog.enumerationByName, catalog.enumerationByID = make(map[string]int, len(catalog.Enumerations)), make(map[uuid.UUID]int, len(catalog.Enumerations))
	catalog.definedTypeByName, catalog.definedTypeByID = make(map[string]int, len(catalog.DefinedTypes)), make(map[uuid.UUID]int, len(catalog.DefinedTypes))
	catalog.catalogByName, catalog.catalogByID = make(map[string]int, len(catalog.Catalogs)), make(map[uuid.UUID]int, len(catalog.Catalogs))
	catalog.documentByName, catalog.documentByID = make(map[string]int, len(catalog.Documents)), make(map[uuid.UUID]int, len(catalog.Documents))
	catalog.chartOfCharacteristicTypesByName = make(map[string]int, len(catalog.ChartsOfCharacteristicTypes))
	catalog.chartOfCharacteristicTypesByID = make(map[uuid.UUID]int, len(catalog.ChartsOfCharacteristicTypes))
	catalog.chartOfAccountsByName = make(map[string]int, len(catalog.ChartsOfAccounts))
	catalog.chartOfAccountsByID = make(map[uuid.UUID]int, len(catalog.ChartsOfAccounts))
	catalog.chartOfCalculationTypesByName = make(map[string]int, len(catalog.ChartsOfCalculationTypes))
	catalog.chartOfCalculationTypesByID = make(map[uuid.UUID]int, len(catalog.ChartsOfCalculationTypes))
	catalog.businessProcessByName = make(map[string]int, len(catalog.BusinessProcesses))
	catalog.businessProcessByID = make(map[uuid.UUID]int, len(catalog.BusinessProcesses))
	catalog.taskByName = make(map[string]int, len(catalog.Tasks))
	catalog.taskByID = make(map[uuid.UUID]int, len(catalog.Tasks))
	catalog.exchangePlanByName = make(map[string]int, len(catalog.ExchangePlans))
	catalog.exchangePlanByID = make(map[uuid.UUID]int, len(catalog.ExchangePlans))
	catalog.accountingRegisterByName = make(map[string]int, len(catalog.AccountingRegisters))
	catalog.accountingRegisterByID = make(map[uuid.UUID]int, len(catalog.AccountingRegisters))
	catalog.calculationRegisterByName = make(map[string]int, len(catalog.CalculationRegisters))
	catalog.calculationRegisterByID = make(map[uuid.UUID]int, len(catalog.CalculationRegisters))
	catalog.reportByName = make(map[string]int, len(catalog.Reports))
	catalog.reportByID = make(map[uuid.UUID]int, len(catalog.Reports))
	catalog.dataProcessorByName = make(map[string]int, len(catalog.DataProcessors))
	catalog.dataProcessorByID = make(map[uuid.UUID]int, len(catalog.DataProcessors))
	catalog.numeratorByName = make(map[string]int, len(catalog.Numerators))
	catalog.numeratorByID = make(map[uuid.UUID]int, len(catalog.Numerators))
	catalog.sequenceByName = make(map[string]int, len(catalog.Sequences))
	catalog.sequenceByID = make(map[uuid.UUID]int, len(catalog.Sequences))
	catalog.documentJournalByName = make(map[string]int, len(catalog.DocumentJournals))
	catalog.documentJournalByID = make(map[uuid.UUID]int, len(catalog.DocumentJournals))
	catalog.informationRegisterByName, catalog.informationRegisterByID = make(map[string]int, len(catalog.InformationRegisters)), make(map[uuid.UUID]int, len(catalog.InformationRegisters))
	catalog.accumulationRegisterByName, catalog.accumulationRegisterByID = make(map[string]int, len(catalog.AccumulationRegisters)), make(map[uuid.UUID]int, len(catalog.AccumulationRegisters))
	allIDs := map[uuid.UUID]string{}
	catalog.identifiers = allIDs
	// unique checks an identifier is used once across the configuration, and
	// keeps what uses it.
	unique := func(id uuid.UUID, what string) error {
		if previous, exists := allIDs[id]; exists {
			return fmt.Errorf("%w: %s and %s use %s", ErrDuplicateID, previous, what, id)
		}
		allIDs[id] = what
		return nil
	}
	// order is the objects in the order they are added, for the walk of what
	// each holds once all are in.
	var order []uuid.UUID
	add := func(kind Kind, id uuid.UUID, name string, index int, names map[string]int, ids map[uuid.UUID]int) error {
		folded := strings.ToLower(name)
		if _, exists := names[folded]; exists {
			return fmt.Errorf("%w: %s %q", ErrDuplicateName, kindWord(kind), name)
		}
		if err := unique(id, kindWord(kind)+" "+name); err != nil {
			return err
		}
		names[folded] = index
		catalog.objects[id] = indexedObject{kind: kind, name: name}
		order = append(order, id)
		if ids != nil {
			ids[id] = index
		}
		return nil
	}
	// within places an object inside its owner, where the tree shows it.
	within := func(id, owner uuid.UUID, title LocalizedText) {
		object := catalog.objects[id]
		object.owner, object.title = &owner, title
		catalog.objects[id] = object
	}
	for index, item := range catalog.Roles {
		if err := ValidateRole("role "+item.Name, item, catalog.Project); err != nil {
			return err
		}
		if err := add(RoleKind, item.ID, item.Name, index, catalog.roleByName, catalog.roleByID); err != nil {
			return err
		}
	}
	// A subsystem's name is unique among its siblings, not across the
	// configuration: the prototype keeps БазоваяФункциональность under a dozen
	// parents (41/35/29 repeated names in the configurations being moved). The
	// lookup by name is by path, built once the parents are known - see
	// validateSubsystemReferences.
	siblings := map[uuid.UUID]map[string]int{}
	for index, item := range catalog.Subsystems {
		var parent uuid.UUID
		if item.Parent != nil {
			parent = *item.Parent
		}
		if siblings[parent] == nil {
			siblings[parent] = map[string]int{}
		}
		if err := add(SubsystemKind, item.ID, item.Name, index, siblings[parent], catalog.subsystemByID); err != nil {
			return err
		}
		if item.Parent != nil {
			within(item.ID, parent, item.Title)
		}
	}
	for index, item := range catalog.Constants {
		if err := add(ConstantKind, item.ID, item.Name, index, catalog.constantByName, catalog.constantByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.FunctionalOptions {
		if err := add(FunctionalOptionKind, item.ID, item.Name, index, catalog.functionalOptionByName, catalog.functionalOptionByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.ScheduledJobs {
		if err := add(ScheduledJobKind, item.ID, item.Name, index, catalog.scheduledJobByName, catalog.scheduledJobByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.CommonCommands {
		if err := add(CommonCommandKind, item.ID, item.Name, index, catalog.commonCommandByName, catalog.commonCommandByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.CommandGroups {
		if err := add(CommandGroupKind, item.ID, item.Name, index, catalog.commandGroupByName, catalog.commandGroupByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.CommonTemplates {
		if err := add(CommonTemplateKind, item.ID, item.Name, index, catalog.commonTemplateByName, catalog.commonTemplateByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.CommonPictures {
		if err := add(CommonPictureKind, item.ID, item.Name, index, catalog.commonPictureByName, catalog.commonPictureByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.XDTOPackages {
		if err := add(XDTOPackageKind, item.ID, item.Name, index, catalog.xdtoPackageByName, catalog.xdtoPackageByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.WebServices {
		if err := add(WebServiceKind, item.ID, item.Name, index, catalog.webServiceByName, catalog.webServiceByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.HTTPServices {
		if err := add(HTTPServiceKind, item.ID, item.Name, index, catalog.httpServiceByName, catalog.httpServiceByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.WSReferences {
		if err := add(WSReferenceKind, item.ID, item.Name, index, catalog.wsReferenceByName, catalog.wsReferenceByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.WebSocketClients {
		if err := add(WebSocketClientKind, item.ID, item.Name, index, catalog.webSocketClientByName, catalog.webSocketClientByID); err != nil {
			return err
		}
	}
	// A table is named within its source, and its identifier is unique across
	// the whole configuration like every other.
	externalTableNames, externalFunctionNames := map[string]int{}, map[string]int{}
	for index, item := range catalog.ExternalDataSources {
		if err := add(ExternalDataSourceKind, item.ID, item.Name, index, catalog.externalDataSourceByName, catalog.externalDataSourceByID); err != nil {
			return err
		}
		for tableIndex, table := range item.Tables {
			if err := add(ExternalDataSourceTableKind, table.ID, item.Name+"."+table.Name, tableIndex, externalTableNames, nil); err != nil {
				return err
			}
			catalog.externalTableByID[table.ID] = externalTableLocation{source: index, table: tableIndex}
			within(table.ID, item.ID, table.Title)
		}
		// A function is named within its source's collection of functions,
		// apart from its tables and cubes (help: ВнешнийИсточникДанных.Функции).
		for functionIndex, function := range item.Functions {
			if err := add(ExternalFunctionKind, function.ID, item.Name+"."+function.Name, functionIndex, externalFunctionNames, nil); err != nil {
				return err
			}
			within(function.ID, item.ID, function.Title)
		}
		// A cube is named within its source and a dimension table within its
		// cube; every identifier is unique across the configuration.
		for cubeIndex, cube := range item.Cubes {
			if err := add(ExternalCubeKind, cube.ID, item.Name+"."+cube.Name, cubeIndex, externalTableNames, nil); err != nil {
				return err
			}
			catalog.externalCubeByID[cube.ID] = externalCubeLocation{source: index, cube: cubeIndex}
			within(cube.ID, item.ID, cube.Title)
			for tableIndex, table := range cube.DimensionTables {
				if err := add(ExternalDimensionTableKind, table.ID, item.Name+"."+cube.Name+"."+table.Name, tableIndex, externalTableNames, nil); err != nil {
					return err
				}
				catalog.externalDimensionTableByID[table.ID] = externalDimensionTableLocation{source: index, cube: cubeIndex, table: tableIndex}
				within(table.ID, cube.ID, table.Title)
			}
		}
	}
	for index, item := range catalog.StyleItems {
		if err := add(StyleItemKind, item.ID, item.Name, index, catalog.styleItemByName, catalog.styleItemByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.Styles {
		if err := add(StyleKind, item.ID, item.Name, index, catalog.styleByName, catalog.styleByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.SettingsStorages {
		if err := add(SettingsStorageKind, item.ID, item.Name, index, catalog.settingsStorageByName, catalog.settingsStorageByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.FilterCriteria {
		if err := add(FilterCriterionKind, item.ID, item.Name, index, catalog.filterCriterionByName, catalog.filterCriterionByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.FunctionalOptionParameters {
		if err := add(FunctionalOptionParameterKind, item.ID, item.Name, index,
			catalog.functionalOptionParameterByName, catalog.functionalOptionParameterByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.SessionParameters {
		if err := add(SessionParameterKind, item.ID, item.Name, index, catalog.sessionParameterByName, catalog.sessionParameterByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.CommonAttributes {
		if err := add(CommonAttributeKind, item.ID, item.Name, index, catalog.commonAttributeByName, catalog.commonAttributeByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.CommonModules {
		if err := add(CommonModuleKind, item.ID, item.Name, index, catalog.commonModuleByName, catalog.commonModuleByID); err != nil {
			return err
		}
		if previous, exists := catalog.commonModuleByModuleID[item.Module]; exists {
			return fmt.Errorf("%w: common module %s and %s share the same module source %s", ErrDuplicateID, catalog.CommonModules[previous].Name, item.Name, item.Module)
		}
		catalog.commonModuleByModuleID[item.Module] = index
	}
	for index, item := range catalog.EventSubscriptions {
		if err := add(EventSubscriptionKind, item.ID, item.Name, index, catalog.eventSubscriptionByName, catalog.eventSubscriptionByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.Enumerations {
		if err := add(EnumerationKind, item.ID, item.Name, index, catalog.enumerationByName, catalog.enumerationByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.DefinedTypes {
		if err := add(DefinedTypeKind, item.ID, item.Name, index, catalog.definedTypeByName, catalog.definedTypeByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.Catalogs {
		if err := add(CatalogKind, item.ID, item.Name, index, catalog.catalogByName, catalog.catalogByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.ChartsOfCharacteristicTypes {
		if err := add(ChartOfCharacteristicTypesKind, item.ID, item.Name, index, catalog.chartOfCharacteristicTypesByName, catalog.chartOfCharacteristicTypesByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.ChartsOfAccounts {
		if err := add(ChartOfAccountsKind, item.ID, item.Name, index, catalog.chartOfAccountsByName, catalog.chartOfAccountsByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.ChartsOfCalculationTypes {
		if err := add(ChartOfCalculationTypesKind, item.ID, item.Name, index, catalog.chartOfCalculationTypesByName, catalog.chartOfCalculationTypesByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.Tasks {
		if err := add(TaskKind, item.ID, item.Name, index, catalog.taskByName, catalog.taskByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.AccountingRegisters {
		if err := add(AccountingRegisterKind, item.ID, item.Name, index, catalog.accountingRegisterByName, catalog.accountingRegisterByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.CalculationRegisters {
		if err := add(CalculationRegisterKind, item.ID, item.Name, index, catalog.calculationRegisterByName, catalog.calculationRegisterByID); err != nil {
			return err
		}
		for _, recalculation := range item.Recalculations {
			name := item.Name + "." + recalculation.Name
			if err := unique(recalculation.ID, "recalculation "+name); err != nil {
				return err
			}
			catalog.objects[recalculation.ID] = indexedObject{kind: RecalculationKind, name: name}
			order = append(order, recalculation.ID)
			within(recalculation.ID, item.ID, recalculation.Title)
		}
	}
	for _, running := range []struct {
		kind    Kind
		names   map[string]int
		ids     map[uuid.UUID]int
		objects []runningObject
	}{
		{ReportKind, catalog.reportByName, catalog.reportByID, reportsAsRunning(catalog.Reports)},
		{DataProcessorKind, catalog.dataProcessorByName, catalog.dataProcessorByID, dataProcessorsAsRunning(catalog.DataProcessors)},
	} {
		for index, item := range running.objects {
			if err := add(running.kind, item.id, item.name, index, running.names, running.ids); err != nil {
				return err
			}
		}
	}
	for index, item := range catalog.Numerators {
		if err := add(NumeratorKind, item.ID, item.Name, index, catalog.numeratorByName, catalog.numeratorByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.Sequences {
		if err := add(SequenceKind, item.ID, item.Name, index, catalog.sequenceByName, catalog.sequenceByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.DocumentJournals {
		if err := add(DocumentJournalKind, item.ID, item.Name, index, catalog.documentJournalByName, catalog.documentJournalByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.ExchangePlans {
		if err := add(ExchangePlanKind, item.ID, item.Name, index, catalog.exchangePlanByName, catalog.exchangePlanByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.BusinessProcesses {
		if err := add(BusinessProcessKind, item.ID, item.Name, index, catalog.businessProcessByName, catalog.businessProcessByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.Documents {
		if err := add(DocumentKind, item.ID, item.Name, index, catalog.documentByName, catalog.documentByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.InformationRegisters {
		if err := add(InformationRegisterKind, item.ID, item.Name, index, catalog.informationRegisterByName, catalog.informationRegisterByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.AccumulationRegisters {
		if err := add(AccumulationRegisterKind, item.ID, item.Name, index, catalog.accumulationRegisterByName, catalog.accumulationRegisterByID); err != nil {
			return err
		}
	}
	// What each object holds is walked once all objects are in: the walk reads
	// the object through its kind's index.
	for _, id := range order {
		object := catalog.objects[id]
		for _, held := range catalog.heldIdentities(object.kind, id) {
			if err := unique(held.id, kindWord(object.kind)+" "+object.name+" "+held.word+" "+held.name); err != nil {
				return err
			}
		}
	}
	// A field of a source refers to a table of that same source, which is
	// narrower than what the walk of every object asks of a reference; the
	// narrower rule answers first, and says what is wrong.
	if err := catalog.validateExternalDataSources(); err != nil {
		return err
	}
	if err := catalog.validateObjectsAsLoaded(root, order); err != nil {
		return err
	}
	for _, item := range catalog.Constants {
		if item.Default != nil {
			if _, err := catalog.NormalizeValue(item, *item.Default); err != nil {
				return fmt.Errorf("constant %s default: %w", item.Name, err)
			}
		}
	}
	for _, item := range catalog.CommonModules {
		if err := validateCommonModuleSource(root, item.Name, item.Module); err != nil {
			return err
		}
	}
	for _, item := range catalog.ChartsOfCharacteristicTypes {
		owner := "chart of characteristic types " + item.Name
		if err := catalog.validateReferences(owner+" value type", item.ValueType); err != nil {
			return err
		}
		// The catalog of additional values is where characteristics whose
		// values fit no existing type keep them; a chart pointing at a catalog
		// that is not there would lose those values silently.
		if item.AdditionalValues != nil {
			if _, ok := catalog.catalogByID[*item.AdditionalValues]; !ok {
				return fmt.Errorf("%s references unknown catalog of additional values %s", owner, item.AdditionalValues)
			}
		}
		if err := catalog.validatePredefinedCharacteristicTypes(owner, item); err != nil {
			return err
		}
	}
	for _, item := range catalog.ChartsOfAccounts {
		if err := catalog.validateChartOfAccountsAnalytics("chart of accounts "+item.Name, item); err != nil {
			return err
		}
	}
	for _, item := range catalog.ChartsOfCalculationTypes {
		if err := catalog.validateCalculationBase("chart of calculation types "+item.Name, item); err != nil {
			return err
		}
	}
	for _, item := range catalog.Tasks {
		if err := catalog.validateTaskAddressing("task "+item.Name, item); err != nil {
			return err
		}
	}
	owners := catalog.documentAttributeOwners()
	for index, item := range catalog.Documents {
		if item.Numerator == nil {
			continue
		}
		numerator, ok := catalog.numeratorByID[*item.Numerator]
		if !ok {
			return fmt.Errorf("document %s is numbered by unknown numerator %s", item.Name, item.Numerator)
		}
		// The shared numbering becomes the document's own settings here, once,
		// so that everything downstream reads one number and not two. All but
		// one: a numerator has no automatic numbering - the help gives it type,
		// length, allowed length, uniqueness and periodicity and nothing else -
		// so whether a number is given automatically stays the document's own.
		// Taking the numerator's whole number, as before, numbered no document
		// of a numerator automatically, ever.
		auto := item.Number.Auto
		catalog.Documents[index].Number = catalog.Numerators[numerator].Number
		catalog.Documents[index].Number.Auto = auto
	}
	for _, item := range catalog.Sequences {
		if err := catalog.validateSequence("sequence "+item.Name, item, owners); err != nil {
			return err
		}
	}
	for _, item := range catalog.DocumentJournals {
		if err := catalog.validateDocumentJournal("document journal "+item.Name, item, owners); err != nil {
			return err
		}
	}
	for _, item := range catalog.ExchangePlans {
		if err := catalog.validateExchangePlanRegistration("exchange plan "+item.Name, item); err != nil {
			return err
		}
	}
	for _, item := range catalog.BusinessProcesses {
		if err := catalog.validateBusinessProcessRoute("business process "+item.Name, item); err != nil {
			return err
		}
	}
	for _, item := range catalog.AccumulationRegisters {
		for _, resource := range item.Resources {
			if _, err := catalog.accumulationResourceType(resource); err != nil {
				return fmt.Errorf("accumulation register %s: %w", item.Name, err)
			}
		}
	}
	for _, item := range catalog.AccountingRegisters {
		if err := catalog.validateAccountingRegister(item); err != nil {
			return err
		}
	}
	for _, item := range catalog.CalculationRegisters {
		if err := catalog.validateCalculationRegister(root, item); err != nil {
			return err
		}
	}
	if err := catalog.validateRoleReferences(); err != nil {
		return err
	}
	if err := catalog.validateSubsystemReferences(); err != nil {
		return err
	}
	catalog.resolveCommandInterfaces(root)
	catalog.resolveRootInterface()
	if err := catalog.validateEventSubscriptionReferences(); err != nil {
		return err
	}
	if err := catalog.validateFunctionalOptions(); err != nil {
		return err
	}
	if err := catalog.validateFunctionalOptionParameters(); err != nil {
		return err
	}
	if err := catalog.validateFilterCriteria(); err != nil {
		return err
	}
	if err := catalog.validateCharacteristics(); err != nil {
		return err
	}
	if err := catalog.validateDocumentMovements(); err != nil {
		return err
	}
	if err := catalog.validateCatalogOwners(); err != nil {
		return err
	}
	if err := catalog.validateBasedOnReferences(); err != nil {
		return err
	}
	if err := catalog.validateSettingsStorageReferences(); err != nil {
		return err
	}
	if err := catalog.validateConstantFiles(root); err != nil {
		return err
	}
	if err := catalog.validateCommonCommandFiles(root); err != nil {
		return err
	}
	if err := catalog.validateIdentifiedPictureFiles(root); err != nil {
		return err
	}
	catalog.noteCommandGroupReferences()
	if err := catalog.validateCommandParameterReferences(); err != nil {
		return err
	}
	if err := catalog.validateCommonTemplateFiles(root); err != nil {
		return err
	}
	if err := catalog.validateCommonPictureFiles(root); err != nil {
		return err
	}
	if err := catalog.validateSeparationReferences(); err != nil {
		return err
	}
	if err := catalog.validateCommonPictureReferences(); err != nil {
		return err
	}
	if err := catalog.validateStyleReferences(); err != nil {
		return err
	}
	if err := catalog.validateScheduledJobReferences(); err != nil {
		return err
	}
	if err := catalog.validateConfigurationDefaults(root); err != nil {
		return err
	}
	if err := catalog.validateXDTOPackages(root); err != nil {
		return err
	}
	if err := catalog.validateWebServices(); err != nil {
		return err
	}
	if err := catalog.validateHTTPServices(); err != nil {
		return err
	}
	if err := catalog.validateWSReferenceFiles(root); err != nil {
		return err
	}
	if err := catalog.validateWebSocketClientFiles(root); err != nil {
		return err
	}
	// Both resolve through defined types or across objects, so they come
	// after everything is loaded. Defined types end: one never holds another
	// (DecodeDefinedType refuses it), and a catalog made without decoding is
	// made from one that was decoded - a runtime snapshot - so a cycle could
	// come only from a snapshot spoilt by hand, which is not guarded against.
	if err := catalog.validateInputByStringTypes(); err != nil {
		return err
	}
	return catalog.resolveFieldChoices(root != "")
}

// validateObjectsAsLoaded is the one walk of the checks every object takes
// once the whole configuration is in, whatever its kind: every type its
// fields and the fields of its table parts name, or the type of its value,
// refers to an object that is there; and its folder keeps what it declares
// and nothing else. What each kind holds comes from its composition
// (kindCompositionOf), so a new place of fields or a new kind is checked by
// being described, not by a loop of its own here. What a kind checks besides
// is its own, in indexAndValidate.
func (catalog *Catalog) validateObjectsAsLoaded(root string, order []uuid.UUID) error {
	for _, id := range order {
		object := catalog.objects[id]
		owner := kindWord(object.kind) + " " + object.name
		composition := kindCompositionOf[object.kind]
		if composition.types != nil {
			if err := catalog.validateReferences(owner, composition.types(catalog, id)); err != nil {
				return err
			}
		}
		parts, _ := catalog.partsOf(object.kind, id)
		names := make(map[string]string, len(parts))
		for _, part := range parts {
			names[part.key] = part.name
			if part.attribute == nil {
				continue
			}
			field := owner
			if part.parent != "" {
				field += " table part " + names[part.parent]
			}
			field += " " + partWord(part.group) + " " + part.name
			if err := catalog.validateReferences(field, part.attribute.Types); err != nil {
				return err
			}
		}
		if composition.files != nil {
			files := composition.files(catalog, id)
			files.root, files.id, files.directoryKind, files.kind, files.name = root, id, object.kind, kindWord(object.kind), object.name
			if err := catalog.validateObjectFileSources(files); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateChartOfAccountsAnalytics ties a chart of accounts to the chart of
// characteristic types it takes its analytics from: the chart has to exist, and
// every kind of analytics a predefined account carries has to be a predefined
// characteristic of that chart. An account pointing at analytics nobody
// declared would be a hole in the books that nothing reports.
func (catalog *Catalog) validateChartOfAccountsAnalytics(owner string, item ChartOfAccountsDefinition) error {
	if item.ExtDimensionTypes == nil {
		return nil
	}
	index, ok := catalog.chartOfCharacteristicTypesByID[*item.ExtDimensionTypes]
	if !ok {
		return fmt.Errorf("%s references unknown chart of characteristic types %s for its analytics", owner, item.ExtDimensionTypes)
	}
	characteristics := map[string]bool{}
	for _, predefined := range catalog.ChartsOfCharacteristicTypes[index].Predefined {
		characteristics[strings.ToLower(predefined.Name)] = true
	}
	for _, account := range item.Predefined {
		for _, dimension := range account.ExtDimensions {
			if !characteristics[strings.ToLower(dimension.Characteristic)] {
				return fmt.Errorf("%s predefined account %s carries analytics %s, which is not a predefined characteristic of %s",
					owner, account.Name, dimension.Characteristic, catalog.ChartsOfCharacteristicTypes[index].Name)
			}
		}
	}
	return nil
}

// validatePredefinedCharacteristicTypes resolves the value type of every
// predefined characteristic and keeps it inside the chart's. The type of an
// element narrows the chart's and never widens it: a characteristic allowed a
// type its chart does not have would hold values no field typed by the chart
// can take. Narrowing is by kind and reference only - an element may well keep
// a shorter string or fewer digits than the chart, and in the configurations
// being moved it does.
func (catalog *Catalog) validatePredefinedCharacteristicTypes(owner string, item ChartOfCharacteristicTypesDefinition) error {
	allowed, err := catalog.expandTypes(item.ValueType, nil)
	if err != nil {
		return fmt.Errorf("%s value type: %w", owner, err)
	}
	held := map[string]bool{}
	for _, one := range allowed {
		held[typeKindKey(one)] = true
	}
	for _, predefined := range item.Predefined {
		// A reference to nothing is caught here as well: what is not in the
		// configuration is not in the chart's type either.
		path := owner + " predefined " + predefined.Name + " value type"
		own, err := catalog.expandTypes(predefined.ValueType, nil)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		for _, one := range own {
			if !held[typeKindKey(one)] {
				return fmt.Errorf("%s allows %s, which the chart's value type does not", path, typeKindKey(one))
			}
		}
	}
	return nil
}

// typeKindKey tells types apart by what they are, not by their qualifiers.
func typeKindKey(item Type) string {
	if item.Reference == nil {
		return string(item.Kind)
	}
	return string(item.Kind) + ":" + item.Reference.String()
}

// validateCalculationBase resolves the charts a base is taken from and the
// base types a predefined calculation type names. A base pointing at a chart
// that is not there, or at a type nobody declared, computes a different number
// while looking transferred - which is worse than failing to transfer.
func (catalog *Catalog) validateCalculationBase(owner string, item ChartOfCalculationTypesDefinition) error {
	own := map[string]bool{}
	for _, predefined := range item.Predefined {
		own[strings.ToLower(predefined.Name)] = true
	}
	charts := map[string]map[string]bool{strings.ToLower(item.Name): own}
	for _, id := range item.BaseCharts {
		index, ok := catalog.chartOfCalculationTypesByID[id]
		if !ok {
			return fmt.Errorf("%s takes its base from unknown chart of calculation types %s", owner, id)
		}
		base := catalog.ChartsOfCalculationTypes[index]
		names := map[string]bool{}
		for _, predefined := range base.Predefined {
			names[strings.ToLower(predefined.Name)] = true
		}
		charts[strings.ToLower(base.Name)] = names
	}
	for _, predefined := range item.Predefined {
		for _, name := range predefined.Base {
			chart, kind := item.Name, name
			if chartName, typeName, found := strings.Cut(name, "."); found {
				chart, kind = chartName, typeName
			}
			names, ok := charts[strings.ToLower(chart)]
			if !ok {
				return fmt.Errorf("%s predefined %s takes its base from %s, which is not among its base charts", owner, predefined.Name, chart)
			}
			if !names[strings.ToLower(kind)] {
				return fmt.Errorf("%s predefined %s takes its base from %s, which is not a calculation type of %s", owner, predefined.Name, kind, chart)
			}
		}
	}
	return nil
}

// addressingAsAttributes treats addressing attributes as the attributes they
// are: each is a column of the task and carries types like any other.
func addressingAsAttributes(item TaskDefinition) []Attribute {
	result := make([]Attribute, 0, len(item.AddressingAttributes))
	for _, attribute := range item.AddressingAttributes {
		result = append(result, Attribute{ID: attribute.ID, Name: attribute.Name, Title: attribute.Title, Types: attribute.Types})
	}
	return result
}

// validateTaskAddressing resolves the register a task is addressed through and
// the dimensions its attributes are matched against. Addressing that points at
// a register that is not there, or at a dimension it does not have, would leave
// the task reaching nobody - and a task nobody sees is worse than no task.
func (catalog *Catalog) validateTaskAddressing(owner string, item TaskDefinition) error {
	if item.CurrentPerformer != nil {
		if _, ok := catalog.sessionParameterByID[*item.CurrentPerformer]; !ok {
			return fmt.Errorf("%s names unknown session parameter %s as the current performer", owner, item.CurrentPerformer)
		}
	}
	if item.Addressing == nil {
		return nil
	}
	index, ok := catalog.informationRegisterByID[*item.Addressing]
	if !ok {
		return fmt.Errorf("%s is addressed through unknown information register %s", owner, item.Addressing)
	}
	register := catalog.InformationRegisters[index]
	dimensions := map[uuid.UUID]bool{}
	for _, dimension := range register.Dimensions {
		dimensions[dimension.ID] = true
	}
	byID := map[uuid.UUID][]Type{}
	for _, dimension := range register.Dimensions {
		byID[dimension.ID] = dimension.Types
	}
	for _, attribute := range item.AddressingAttributes {
		if attribute.Dimension == nil {
			continue
		}
		if !dimensions[*attribute.Dimension] {
			return fmt.Errorf("%s addressing attribute %s is matched against %s, which is not a dimension of %s",
				owner, attribute.Name, attribute.Dimension, register.Name)
		}
		// A value the dimension cannot hold is a value the register will never
		// be asked about: the task would be addressed to somebody the platform
		// then fails to find, and nothing anywhere would say so.
		held, err := catalog.heldTypes(byID[*attribute.Dimension])
		if err != nil {
			return err
		}
		addressed, err := catalog.expandTypes(attribute.Types, nil)
		if err != nil {
			return err
		}
		for _, one := range addressed {
			if !held[typeKey(one)] {
				return fmt.Errorf("%s addressing attribute %s holds %s, which the dimension it is matched against in %s cannot",
					owner, attribute.Name, one.Kind, register.Name)
			}
		}
	}
	return nil
}

// validateAccountingRegister resolves the chart an entry is made against and
// the flags its fields depend on. A flag from another chart is the quiet kind
// of mistake: the field would exist, the accounts would never turn it on, and
// the column would stay empty for as long as anybody cared to look.
func (catalog *Catalog) validateAccountingRegister(item AccountingRegisterDefinition) error {
	owner := "accounting register " + item.Name
	// A register with no chart has no flags to depend on and no ext
	// dimensions: an empty chart answers both questions.
	chart := ChartOfAccountsDefinition{Name: "no chart of accounts"}
	if !item.ChartOfAccounts.IsZero() {
		index, ok := catalog.chartOfAccountsByID[item.ChartOfAccounts]
		if !ok {
			return fmt.Errorf("%s makes entries against unknown chart of accounts %s", owner, item.ChartOfAccounts)
		}
		chart = catalog.ChartsOfAccounts[index]
	}
	for _, resource := range item.Resources {
		if _, err := catalog.accumulationResourceType(resource.Attribute); err != nil {
			return fmt.Errorf("%s: %w", owner, err)
		}
	}
	flags, extFlags := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	for _, flag := range chart.AccountingFlags {
		flags[flag.ID] = true
	}
	for _, flag := range chart.ExtDimensionAccountingFlags {
		extFlags[flag.ID] = true
	}
	// A dimension and a resource both depend on a flag of the chart; only a
	// resource can depend on a flag of its ext dimensions.
	type flaggedField struct {
		field                 Attribute
		accounting, extension *uuid.UUID
	}
	fields := make([]flaggedField, 0, len(item.Dimensions)+len(item.Resources))
	for _, dimension := range item.Dimensions {
		fields = append(fields, flaggedField{dimension.Attribute, dimension.AccountingFlag, nil})
	}
	for _, resource := range item.Resources {
		fields = append(fields, flaggedField{resource.Attribute, resource.AccountingFlag, resource.ExtDimensionAccountingFlag})
	}
	for _, flagged := range fields {
		field := flagged.field
		if flagged.accounting != nil && !flags[*flagged.accounting] {
			return fmt.Errorf("%s field %s depends on %s, which is not an accounting flag of %s",
				owner, field.Name, flagged.accounting, chart.Name)
		}
		if flagged.extension != nil && !extFlags[*flagged.extension] {
			return fmt.Errorf("%s field %s depends on %s, which is not an ext dimension accounting flag of %s",
				owner, field.Name, flagged.extension, chart.Name)
		}
	}
	// The ext dimensions of an entry are as many as the chart allows, and the
	// chart was not in hand when the register was decoded: a description of
	// Субконто4 on a chart that allows three passed there and is caught here.
	if issues := validateStandardAttributes("standard_attributes", item.StandardAttributes,
		accountingDescribedFields(item, chart.MaxExtDimensionCount), catalog.Project); len(issues) > 0 {
		return fmt.Errorf("%s: %s", owner, strings.Join(issues, "; "))
	}
	// So are the links: one taking its value from Субконто4 passed the
	// decoder, which counted as many ext dimensions as the links mention, and
	// is caught here on a chart that allows three.
	if issues := validateAccountingRegisterLinks(item, chart.EffectiveMaxExtDimensionCount()); len(issues) > 0 {
		return fmt.Errorf("%s: %s", owner, strings.Join(issues, "; "))
	}
	return nil
}

// takesABase says whether a chart gathers a base at all. An unset dependency
// and one set to "none" are the same answer, and treating them differently
// would let a base period through on half the charts that have none.
func takesABase(chart ChartOfCalculationTypesDefinition) bool {
	return chart.BaseDependency != "" && chart.BaseDependency != NoBaseDependency
}

// validateCalculationRegister resolves the chart a register calculates by, the
// schedule it reads, and the two links every recalculation dimension carries.
//
// Everything checked here is a link whose absence shows up as a calculation
// that quietly produces the wrong number rather than as an error: a base
// dimension where nothing takes a base, a schedule link to a register that is
// not the schedule, a recalculation that nothing ever sets off.
func (catalog *Catalog) validateCalculationRegister(root string, item CalculationRegisterDefinition) error {
	owner := "calculation register " + item.Name
	index, ok := catalog.chartOfCalculationTypesByID[item.ChartOfCalculationTypes]
	if !ok {
		return fmt.Errorf("%s calculates by unknown chart of calculation types %s", owner, item.ChartOfCalculationTypes)
	}
	chart := catalog.ChartsOfCalculationTypes[index]
	if item.ActionPeriod && !chart.ActionPeriodUse {
		return fmt.Errorf("%s keeps a period of action, but %s has no competition over it, so nothing would ever displace anything", owner, chart.Name)
	}
	if item.BasePeriod && !takesABase(chart) {
		return fmt.Errorf("%s keeps a base period, but %s takes no base, so the period would be gathered over nothing", owner, chart.Name)
	}
	scheduleDimensions := map[uuid.UUID]bool{}
	if item.Schedule != nil {
		scheduleIndex, ok := catalog.informationRegisterByID[*item.Schedule]
		if !ok {
			return fmt.Errorf("%s reads a schedule that is not an information register: %s", owner, item.Schedule)
		}
		schedule := catalog.InformationRegisters[scheduleIndex]
		for _, dimension := range schedule.Dimensions {
			scheduleDimensions[dimension.ID] = true
		}
		resources := map[uuid.UUID]bool{}
		for _, resource := range schedule.Resources {
			resources[resource.ID] = true
		}
		if item.ScheduleValue != nil && !resources[*item.ScheduleValue] {
			return fmt.Errorf("%s takes the value of the schedule from %s, which is not a resource of %s", owner, item.ScheduleValue, schedule.Name)
		}
		if item.ScheduleDate != nil && !scheduleDimensions[*item.ScheduleDate] {
			return fmt.Errorf("%s takes the date of the schedule from %s, which is not a dimension of %s", owner, item.ScheduleDate, schedule.Name)
		}
		// The syntax assistant's own description of the schedule: a register of
		// information that is not periodic, its date a dimension of the type
		// Date and its value a resource of the type Number. Both schedules of
		// the configurations being moved are so.
		if schedule.Periodicity != InformationRegisterPeriodNone {
			return fmt.Errorf("%s reads schedule %s, which is periodic; a schedule is a register that is not", owner, schedule.Name)
		}
		for _, dimension := range schedule.Dimensions {
			if item.ScheduleDate != nil && dimension.ID == *item.ScheduleDate && !singleKind(dimension.Types, DateType) {
				return fmt.Errorf("%s takes the date of the schedule from %s, which is not of the type Date", owner, dimension.Name)
			}
		}
		for _, resource := range schedule.Resources {
			if item.ScheduleValue != nil && resource.ID == *item.ScheduleValue && !singleKind(resource.Types, NumberType) {
				return fmt.Errorf("%s takes the value of the schedule from %s, which is not of the type Number", owner, resource.Name)
			}
		}
	}
	dimensions := map[uuid.UUID]bool{}
	for _, dimension := range item.Dimensions {
		dimensions[dimension.ID] = true
		if dimension.Base && !takesABase(chart) {
			return fmt.Errorf("%s dimension %s is a base dimension, but %s takes no base", owner, dimension.Name, chart.Name)
		}
		if dimension.ScheduleLink != nil && !scheduleDimensions[*dimension.ScheduleLink] {
			return fmt.Errorf("%s dimension %s is linked to %s, which is not a dimension of the schedule", owner, dimension.Name, dimension.ScheduleLink)
		}
	}
	for _, attribute := range item.Attributes {
		if attribute.ScheduleLink != nil && !scheduleDimensions[*attribute.ScheduleLink] {
			return fmt.Errorf("%s attribute %s is linked to %s, which is not a dimension of the schedule", owner, attribute.Name, attribute.ScheduleLink)
		}
	}
	for _, resource := range item.Resources {
		if _, err := catalog.accumulationResourceType(resource); err != nil {
			return fmt.Errorf("%s: %w", owner, err)
		}
	}
	// Leading data are fields of calculation registers - this one or another.
	// The help does not say which fields, and every reference in the exports
	// is a dimension; a resource or an attribute is accepted and noted, see
	// NoteLeadingDataNotDimension. A field that belongs to nothing sets
	// nothing off.
	leading := map[uuid.UUID]bool{}
	for _, register := range catalog.CalculationRegisters {
		for _, dimension := range register.Dimensions {
			leading[dimension.ID] = true
		}
		for _, resource := range register.Resources {
			leading[resource.ID] = true
		}
		for _, attribute := range register.Attributes {
			leading[attribute.ID] = true
		}
	}
	for _, recalculation := range item.Recalculations {
		for _, dimension := range recalculation.Dimensions {
			if !dimensions[dimension.RegisterDimension] {
				return fmt.Errorf("%s recalculation %s dimension %s corresponds to %s, which is not a dimension of this register",
					owner, recalculation.Name, dimension.Name, dimension.RegisterDimension)
			}
			for _, source := range dimension.LeadingData {
				if !leading[source] {
					return fmt.Errorf("%s recalculation %s dimension %s is set off by %s, which is not a field of any calculation register",
						owner, recalculation.Name, dimension.Name, source)
				}
			}
		}
	}
	return validateRecalculationFiles(root, item)
}

// documentAttributeOwners maps every attribute of every document - its own and
// those of its table parts - to the document that owns it. Sequences and
// journals both point at attributes across several kinds of document, and
// without this they would be pointing at identifiers nobody resolves.
func (catalog *Catalog) documentAttributeOwners() map[uuid.UUID]uuid.UUID {
	owners := map[uuid.UUID]uuid.UUID{}
	for _, document := range catalog.Documents {
		for _, attribute := range document.Attributes {
			owners[attribute.ID] = document.ID
		}
		for _, part := range document.TableParts {
			for _, attribute := range part.Attributes {
				owners[attribute.ID] = document.ID
			}
		}
	}
	return owners
}

// validateSequence resolves what a sequence follows. Every pointer here is one
// that fails silently if it is wrong: a dimension mapped to an attribute of a
// document outside the sequence gets no value, and a boundary that gets no
// value is a boundary that never moves.
func (catalog *Catalog) validateSequence(owner string, item SequenceDefinition, owners map[uuid.UUID]uuid.UUID) error {
	documents := map[uuid.UUID]bool{}
	for _, id := range item.Documents {
		if _, ok := catalog.documentByID[id]; !ok {
			return fmt.Errorf("%s follows unknown document %s", owner, id)
		}
		documents[id] = true
	}
	registerDimensions := map[uuid.UUID]bool{}
	for _, id := range item.Movements {
		switch {
		case hasID(catalog.informationRegisterByID, id):
			for _, dimension := range catalog.InformationRegisters[catalog.informationRegisterByID[id]].Dimensions {
				registerDimensions[dimension.ID] = true
			}
		case hasID(catalog.accumulationRegisterByID, id):
			for _, dimension := range catalog.AccumulationRegisters[catalog.accumulationRegisterByID[id]].Dimensions {
				registerDimensions[dimension.ID] = true
			}
		case hasID(catalog.accountingRegisterByID, id):
			for _, dimension := range catalog.AccountingRegisters[catalog.accountingRegisterByID[id]].Dimensions {
				registerDimensions[dimension.ID] = true
			}
		case hasID(catalog.calculationRegisterByID, id):
			for _, dimension := range catalog.CalculationRegisters[catalog.calculationRegisterByID[id]].Dimensions {
				registerDimensions[dimension.ID] = true
			}
		default:
			return fmt.Errorf("%s watches unknown register %s", owner, id)
		}
	}
	for _, dimension := range item.Dimensions {
		if err := catalog.validateReferences(owner+" dimension "+dimension.Name, dimension.Types); err != nil {
			return err
		}
		for _, attribute := range dimension.DocumentAttributes {
			document, ok := owners[attribute]
			if !ok {
				return fmt.Errorf("%s dimension %s is taken from unknown attribute %s", owner, dimension.Name, attribute)
			}
			if !documents[document] {
				return fmt.Errorf("%s dimension %s is taken from an attribute of a document the sequence does not follow", owner, dimension.Name)
			}
		}
		for _, field := range dimension.DocumentStandardAttributes {
			if !documents[field.Document] {
				return fmt.Errorf("%s dimension %s is taken from %s of a document the sequence does not follow", owner, dimension.Name, field.Standard)
			}
		}
		for _, target := range dimension.RegisterDimensions {
			if !registerDimensions[target] {
				return fmt.Errorf("%s dimension %s is matched against %s, which is not a dimension of any register it watches", owner, dimension.Name, target)
			}
		}
	}
	return nil
}

// validateDocumentJournal resolves the documents a journal shows and the
// attributes each of its columns shows for them.
func (catalog *Catalog) validateDocumentJournal(owner string, item DocumentJournalDefinition, owners map[uuid.UUID]uuid.UUID) error {
	documents := map[uuid.UUID]bool{}
	for _, id := range item.Documents {
		if _, ok := catalog.documentByID[id]; !ok {
			return fmt.Errorf("%s shows unknown document %s", owner, id)
		}
		documents[id] = true
	}
	for _, column := range item.Columns {
		shown := map[uuid.UUID]bool{}
		for _, attribute := range column.References {
			document, ok := owners[attribute]
			if !ok {
				return fmt.Errorf("%s column %s shows unknown attribute %s", owner, column.Name, attribute)
			}
			if !documents[document] {
				return fmt.Errorf("%s column %s shows an attribute of a document the journal does not list", owner, column.Name)
			}
			// One column shows one attribute per document. Two would be two
			// answers to "what goes in this cell" for the same row.
			if shown[document] {
				return fmt.Errorf("%s column %s shows two attributes of one document", owner, column.Name)
			}
			shown[document] = true
		}
		for _, field := range column.StandardReferences {
			if !documents[field.Document] {
				return fmt.Errorf("%s column %s shows %s of a document the journal does not list", owner, column.Name, field.Standard)
			}
			if shown[field.Document] {
				return fmt.Errorf("%s column %s shows two attributes of one document", owner, column.Name)
			}
			shown[field.Document] = true
		}
	}
	return nil
}

func hasID(index map[uuid.UUID]int, id uuid.UUID) bool {
	_, ok := index[id]
	return ok
}

// validateExchangePlanRegistration resolves every entry of the content: an
// entry naming an object that is not there, or naming it under the wrong kind,
// registers nothing. Nothing would say so either - the exchange would simply
// carry less than the plan claims, and the difference shows up as missing data
// on the other side, far from here.
func (catalog *Catalog) validateExchangePlanRegistration(owner string, item ExchangePlanDefinition) error {
	for _, entry := range item.Content {
		if !catalog.hasObject(entry.Kind, entry.Object) {
			return fmt.Errorf("%s registers changes of %s %s, which the project does not have", owner, entry.Kind, entry.Object)
		}
	}
	return nil
}

// validateBusinessProcessRoute resolves the kind of task a process creates and
// the processes its nested points start.
func (catalog *Catalog) validateBusinessProcessRoute(owner string, item BusinessProcessDefinition) error {
	if item.Task != nil {
		if _, ok := catalog.taskByID[*item.Task]; !ok {
			return fmt.Errorf("%s creates tasks of unknown kind %s", owner, item.Task)
		}
	}
	var addressed map[string][]Type
	if item.Task != nil {
		task := catalog.Tasks[catalog.taskByID[*item.Task]]
		addressed = make(map[string][]Type, len(task.AddressingAttributes))
		for _, attribute := range task.AddressingAttributes {
			addressed[strings.ToLower(attribute.Name)] = attribute.Types
		}
	}
	for _, point := range item.Route.Points {
		if point.Kind == ActivityPoint && item.Task == nil {
			return fmt.Errorf("%s has an activity point %s but names no kind of task to create there", owner, point.Name)
		}
		// A point that addresses its tasks by an attribute the kind of task
		// does not have addresses nobody, and it does so silently.
		for _, value := range point.Addressing {
			types, ok := addressed[strings.ToLower(value.Attribute)]
			if !ok {
				return fmt.Errorf("%s point %s addresses tasks by %s, which is not an addressing attribute of the task it creates", owner, point.Name, value.Attribute)
			}
			if value.Value == nil {
				continue
			}
			if _, err := catalog.normalizeTypes(fmt.Sprintf("%s point %s addressing %s", owner, point.Name, value.Attribute), types, *value.Value); err != nil {
				return err
			}
		}
		if point.NestedProcess == nil {
			continue
		}
		if _, ok := catalog.businessProcessByID[*point.NestedProcess]; !ok {
			return fmt.Errorf("%s point %s starts unknown business process %s", owner, point.Name, point.NestedProcess)
		}
		// A process may nest itself: the demo base saved by the designer has
		// an assignment whose nested point starts more assignments of the same
		// kind. Each nested one is a new process, so nothing loops at load.
	}
	return nil
}

func (catalog *Catalog) validateReferences(owner string, types []Type) error {
	for _, item := range types {
		if item.Reference == nil {
			continue
		}
		found, known := catalog.typeNamesObject(item.Kind, *item.Reference)
		switch {
		case !known || found:
		case objectTypes[item.Kind]:
			return fmt.Errorf("%s is typed %s of unknown %s %s", owner, item.Kind, kindWord(typeOwners[item.Kind]), item.Reference)
		default:
			return fmt.Errorf("%s references unknown %s %s", owner, kindWord(typeOwners[item.Kind]), item.Reference)
		}
	}
	return nil
}

// validateCommonModuleSource checks that a common module's source file
// exists at the shared top-level modules/ directory — common modules are
// not owned by any single prikladnoy object, so they keep the flat layout.
func validateCommonModuleSource(root, name string, module uuid.UUID) error {
	if root == "" {
		return nil
	}
	path := filepath.Join(root, "modules", module.String()+".bsl")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("common module %s module %s is missing or unsafe", name, module)
	}
	return nil
}

// objectFiles is everything one object keeps on disk beside its description:
// its own modules, its forms, the module of each of its commands and the
// content of each of its templates. They are checked together because they
// share one folder, and a file in that folder that nothing declares is as much
// a mistake as a declaration with no file behind it.
// The module roles each kind of object may keep are the kind's own
// (kinds.Layout.Modules).

type objectFiles struct {
	root          string
	directoryKind Kind
	kind, name    string
	// id is the object's identifier, recorded with its kind for what refers
	// to the object by identifier and names its kind beside it.
	id uuid.UUID
	// modules lists the module roles this object may keep directly in its
	// folder; nil is every role its kind has. A module has no identifier and
	// is not declared anywhere: the file is the declaration, so what is
	// checked is the other direction — that a .bsl lying there plays a role
	// this kind actually has.
	modules []kinds.Module
	// formSlots is the set of form roles this kind has, taken from what the
	// object declared. Every kind hands over its own set - a register opens a
	// list and nothing else, a catalog opens five different things - so there
	// is nothing here that says which roles exist.
	formSlots []formSlot
	commands  []ObjectCommand
	templates []ObjectTemplate
	// collections are the folders of this object's own content beside its
	// forms, commands and templates and the folders of what lies inside it -
	// the pictures of the route map of a business process.
	collections []string
	// route is the route map of a business process, whose items may draw a
	// picture from a file kept in the process's folder.
	route *RouteMap
}

// validateObjectFileSources checks the per-object folder
// (metadata/<kind>/<name>/) against what the object declares, in both
// directions: a declaration with no file behind it is a mistake, and so is a
// file nothing declares.
//
// Modules are checked in one direction only, because they are declared by
// lying where they lie. МодульОбъекта.bsl is the object module of whatever
// object's folder it is in, and an object with no such file simply has no
// object module. What can still be wrong is a module in a role this kind of
// object does not have - the record set of something that keeps no records -
// and that is what is caught here.
func (catalog *Catalog) validateObjectFileSources(files objectFiles) error {
	if catalog.formOwnerKinds == nil {
		catalog.formOwnerKinds = map[uuid.UUID]Kind{}
	}
	catalog.formOwnerKinds[files.id] = files.directoryKind
	if files.root == "" {
		return nil
	}
	root, kind, name := files.root, files.kind, files.name
	// The folder comes from the project's translation of the address, so an
	// object inside another - a table of a source - is found where it lies.
	relative, err := project.ObjectDirectory(string(files.directoryKind), name)
	if err != nil {
		return fmt.Errorf("%s %s: %w", kind, name, err)
	}
	directory := filepath.Join(root, filepath.FromSlash(relative))
	modules := files.modules
	if modules == nil {
		modules = kinds.Must(files.directoryKind).Layout().Modules
	}
	collections := slices.Concat(project.SubordinateCollections(string(files.directoryKind)), project.InnerFolders(string(files.directoryKind)), files.collections)
	if err := validateObjectFolderEntries(directory, files.directoryKind, kind, name, modules, collections...); err != nil {
		return err
	}
	if err := catalog.indexObjectForms(directory, files.directoryKind, kind, name, files.formSlots); err != nil {
		return err
	}
	if err := catalog.validateCommonFormSlots(kind, name, files.formSlots); err != nil {
		return err
	}
	if err := validateObjectCommandFiles(directory, kind, name, files.commands); err != nil {
		return err
	}
	if files.route != nil {
		if err := validateRoutePictureFiles(directory, kind, name, files.route); err != nil {
			return err
		}
	}
	return validateObjectTemplateFiles(directory, kind, name, files.templates)
}

// validateRoutePictureFiles checks the folder of the pictures of a route map
// against the map, both ways: every item drawing a file of its own finds it
// in its folder, and every folder there is an item holding the one file its
// picture draws.
func validateRoutePictureFiles(directory, kind, name string, route *RouteMap) error {
	items := map[string]*PictureReference{}
	base := filepath.Join(directory, project.RouteDirectory)
	add := func(item string, look *RouteLook) error {
		var picture *PictureReference
		if look != nil {
			picture = look.Picture
		}
		items[strings.ToLower(item)] = picture
		// The folder is named by the item, whatever its case.
		return requirePictureFile(fmt.Sprintf("%s %s route item %s", kind, name, item), findFolded(base, item), picture)
	}
	for _, point := range route.Points {
		if err := add(point.Name, point.Look); err != nil {
			return err
		}
	}
	for _, decoration := range route.Decorations {
		if err := add(decoration.Name, decoration.Look); err != nil {
			return err
		}
	}
	folders, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("%s %s route: %w", kind, name, err)
	}
	for _, folder := range folders {
		picture, ok := items[strings.ToLower(folder.Name())]
		if !ok || !folder.IsDir() || folder.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s %s keeps %q among the pictures of its route, and the route has no item of that name",
				kind, name, folder.Name())
		}
		files, err := os.ReadDir(filepath.Join(base, folder.Name()))
		if err != nil {
			return fmt.Errorf("%s %s route item %s: %w", kind, name, folder.Name(), err)
		}
		for _, file := range files {
			if !file.Type().IsRegular() || !picture.drawsFile(file.Name()) {
				return fmt.Errorf("%s %s route item %s keeps %q, which its picture does not draw",
					kind, name, folder.Name(), file.Name())
			}
		}
	}
	return nil
}

// findFolded returns the entry of a folder whose name matches without regard
// to case, or the name itself when none does.
func findFolded(directory, name string) string {
	entries, _ := os.ReadDir(directory)
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), name) {
			return filepath.Join(directory, entry.Name())
		}
	}
	return filepath.Join(directory, name)
}

// indexObjectForms reads the forms folder of one object and records what it
// found, then checks that every slot names one of those forms.
//
// The folder is the list of forms; the slots only say which of them the
// platform opens by default. There are normally more forms than slots - the
// prototype's catalog of users keeps seven forms behind three slots - so a
// form no slot names is a form like any other, not an orphan, and is kept.
//
// What is recorded is the form's own identifier, because that is what a role,
// the portal and ML App refer to a form by. The name finds the file; the
// identifier travels.
func (catalog *Catalog) indexObjectForms(directory string, directoryKind Kind, kind, name string, slots []formSlot) error {
	entries, err := os.ReadDir(filepath.Join(directory, "forms"))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%s %s forms: %w", kind, name, err)
	}
	found := map[string]objectFormRef{}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s %s keeps %q among its forms, and a form is a folder",
				kind, name, entry.Name())
		}
		if project.SubordinateName(entry.Name()) != nil {
			return fmt.Errorf("%s %s keeps a form folder %q, which is not a name", kind, name, entry.Name())
		}
		id, err := readObjectFormIdentity(directory, entry.Name())
		if err != nil {
			return fmt.Errorf("%s %s form %s: %w", kind, name, entry.Name(), err)
		}
		if err := validateObjectFormFolder(directory, entry.Name()); err != nil {
			return fmt.Errorf("%s %s form %s: %w", kind, name, entry.Name(), err)
		}
		found[strings.ToLower(entry.Name())] = objectFormRef{name: entry.Name(), id: id,
			path: filepath.Join(directory, "forms", entry.Name(), project.FormMetadataFile)}
	}
	for _, slot := range slots {
		reference, problem := parseFormReference(slot.form)
		if slot.form == "" || problem != "" || reference.common || !reference.unresolved.IsZero() {
			continue
		}
		if _, ok := found[strings.ToLower(reference.name)]; !ok {
			return fmt.Errorf("%s %s names %s %s, which it does not keep", kind, name, slot.role(), slot.form)
		}
	}
	if len(found) == 0 {
		return nil
	}
	if catalog.objectForms == nil {
		catalog.objectForms = map[Kind]map[string]objectFormIndex{}
	}
	if catalog.objectForms[directoryKind] == nil {
		catalog.objectForms[directoryKind] = map[string]objectFormIndex{}
	}
	catalog.objectForms[directoryKind][strings.ToLower(name)] = objectFormIndex{object: name, kind: kind, forms: found}
	return nil
}

// indexCommonForms reads the common forms once, so that every role pointing at
// one is answered from memory. A configuration has as many roles as it has
// objects, and reading the folder per role would read it hundreds of times.
func (catalog *Catalog) indexCommonForms(root string) error {
	if root == "" {
		return nil
	}
	forms, err := ReadCommonForms(root, catalog.Project)
	if err != nil {
		return err
	}
	catalog.commonFormNames = make(map[string]bool, len(forms))
	catalog.commonFormsRead = forms
	for _, form := range forms {
		catalog.commonFormNames[strings.ToLower(form.Name)] = true
		catalog.objects[form.ID] = indexedObject{kind: CommonFormKind, name: form.Name}
	}
	return nil
}

// validateCommonFormSlots checks the roles that open a form the object does not
// own. A role pointing at a common form that is not in the configuration is a
// form that never opens, and the object it belongs to says nothing about it.
func (catalog *Catalog) validateCommonFormSlots(kind, name string, slots []formSlot) error {
	for _, slot := range slots {
		if slot.form == "" {
			continue
		}
		reference, problem := parseFormReference(slot.form)
		if problem == "" && !reference.unresolved.IsZero() {
			catalog.noteUnresolved(kind+" "+name+" "+slot.role(), reference.unresolved)
			continue
		}
		if problem != "" || !reference.common {
			continue
		}
		if !catalog.commonFormNames[strings.ToLower(reference.name)] {
			return fmt.Errorf("%s %s opens %s with common form %s, which is not in the configuration",
				kind, name, slot.role(), reference.name)
		}
	}
	return nil
}

// validateObjectFormFolder checks that a form's folder holds the form, its
// module, its help and the pictures of its elements, and nothing else. The module is not declared anywhere - the file
// lying there under the name of its role is the whole declaration - so the
// only thing that can be wrong is a file nobody can name.
func validateObjectFormFolder(directory, form string) error {
	entries, err := os.ReadDir(filepath.Join(directory, "forms", form))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() && entry.Type()&fs.ModeSymlink == 0 && entry.Name() == project.HelpDirectory {
			if err := validateHelpFolder("form "+form, filepath.Join(directory, "forms", form, entry.Name())); err != nil {
				return err
			}
			continue
		}
		// The pictures of the elements are checked against the elements,
		// once the form is read whole (checkFormAttributes).
		if entry.Name() == project.FormItemsDirectory {
			continue
		}
		if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 ||
			entry.Name() != project.FormMetadataFile && entry.Name() != project.FormModuleFile {
			return fmt.Errorf("keeps %q, and a form keeps only its description, its module, its help and the pictures of its elements", entry.Name())
		}
	}
	return nil
}

// readObjectFormIdentity reads the identifier one form keeps in its own
// description, and checks the description agrees about the form's name.
//
// Only these fields are read here, while the objects are still being read.
// The whole form is read once everything it may refer to is loaded
// (checkFormAttributes), and a form that does not read refuses the project
// there: the load is strict. Showing such a form as an error without closing
// the project is the Studio's reading for editing, not this one.
func readObjectFormIdentity(directory, form string) (uuid.UUID, error) {
	path := filepath.Join(directory, "forms", form, project.FormMetadataFile)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return uuid.UUID{}, fmt.Errorf("has no %s", project.FormMetadataFile)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return uuid.UUID{}, err
	}
	var identity struct {
		ID     uuid.UUID  `yaml:"id"`
		Name   string     `yaml:"name"`
		Module *uuid.UUID `yaml:"module"`
	}
	if err := yaml.Unmarshal(content, &identity); err != nil {
		return uuid.UUID{}, err
	}
	if identity.ID.IsZero() {
		return uuid.UUID{}, fmt.Errorf("has no identifier of its own")
	}
	// A form of an object does not name its module: the module is the file
	// beside it. Naming one would be a second place to keep in step, and the
	// two would disagree the first time either moved.
	if identity.Module != nil {
		return uuid.UUID{}, fmt.Errorf("names a module, and its module is %s beside it", project.FormModuleFile)
	}
	if err := agreesWithFolder("form", identity.Name, form); err != nil {
		return uuid.UUID{}, err
	}
	return identity.ID, nil
}

// validateObjectFolderEntries checks that the object's own folder holds only
// what an object may keep there: its description, a module in a role this kind
// of object has, and the folders of its forms, commands and templates.
func validateObjectFolderEntries(directory string, directoryKind Kind, kind, name string, modules []kinds.Module, collections ...string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("%s %s: %w", kind, name, err)
	}
	for _, entry := range entries {
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s %s keeps %q, which is a symbolic link", kind, name, entry.Name())
		}
		if entry.IsDir() {
			// A collection of the objects that lie inside this one - the
			// dimension tables of a cube - is read as objects of its own.
			if slices.Contains(collections, entry.Name()) {
				continue
			}
			if entry.Name() == project.HelpDirectory {
				if err := validateHelpFolder(kind+" "+name, filepath.Join(directory, entry.Name())); err != nil {
					return err
				}
				continue
			}
			if owned := project.ObjectSubordinateDirectories(string(directoryKind)); !slices.Contains(owned, entry.Name()) {
				if len(owned) == 0 {
					return fmt.Errorf("%s %s keeps a folder %q, and a %s keeps no forms, commands or templates",
						kind, name, entry.Name(), kind)
				}
				words := strings.Join(owned, ", ")
				if last := strings.LastIndex(words, ", "); last >= 0 {
					words = words[:last] + " or " + words[last+2:]
				}
				return fmt.Errorf("%s %s keeps a folder %q, which is not %s", kind, name, entry.Name(), words)
			}
			continue
		}
		if entry.Name() == project.ObjectMetadataFile {
			continue
		}
		if !slices.ContainsFunc(modules, func(module kinds.Module) bool { return module.File() == entry.Name() }) {
			if slices.Contains(project.ObjectModuleFiles(), entry.Name()) {
				return fmt.Errorf("%s %s keeps %q, and a %s has no module in that role",
					kind, name, entry.Name(), kind)
			}
			return fmt.Errorf("%s %s keeps %q, which is neither its description nor a module of its own",
				kind, name, entry.Name())
		}
	}
	return nil
}

// validateObjectCommandFiles checks the commands folder of one object. Every
// command the object declares keeps a folder named after it, holding the
// module that runs it - a command without a body is a place in the interface
// that answers a click with nothing, which is worse than not offering it at
// all. A folder no command declares is an orphan and is refused: its module
// would never run, and nothing would say why.
func validateObjectCommandFiles(directory, kind, name string, commands []ObjectCommand) error {
	declared := make(map[string]*PictureReference, len(commands))
	for _, command := range commands {
		info, err := os.Lstat(filepath.Join(directory, "commands", command.Name, project.CommandModuleFile))
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("%s %s command %s has no module", kind, name, command.Name)
		}
		if err := requirePictureFile(fmt.Sprintf("%s %s command %s", kind, name, command.Name),
			filepath.Join(directory, "commands", command.Name), command.Picture); err != nil {
			return err
		}
		declared[strings.ToLower(command.Name)] = command.Picture
	}
	entries, err := os.ReadDir(filepath.Join(directory, "commands"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("%s %s commands: %w", kind, name, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s %s keeps %q among its commands, and a command is a folder",
				kind, name, entry.Name())
		}
		picture, ok := declared[strings.ToLower(entry.Name())]
		if !ok {
			return fmt.Errorf("%s %s keeps a folder for command %s, which it does not declare",
				kind, name, entry.Name())
		}
		content, err := os.ReadDir(filepath.Join(directory, "commands", entry.Name()))
		if err != nil {
			return fmt.Errorf("%s %s command %s: %w", kind, name, entry.Name(), err)
		}
		for _, file := range content {
			if file.IsDir() || file.Type()&fs.ModeSymlink != 0 || file.Name() != project.CommandModuleFile && !picture.drawsFile(file.Name()) {
				return fmt.Errorf("%s %s command %s holds %q, and a command keeps only its module and the file of its picture",
					kind, name, entry.Name(), file.Name())
			}
		}
	}
	return nil
}

// validateObjectTemplateFiles checks the templates folder of one object
// against what the object declares. The content itself is not read here and
// may not exist at all - no editor writes it yet, and the reference
// configuration carries templates with no content of their own. What is
// checked is that every folder belongs to a declared template and holds only
// the files that template's kind allows, so that content written later lands
// where the platform will look for it.
func validateObjectTemplateFiles(directory, kind, name string, templates []ObjectTemplate) error {
	declared := make(map[string]TemplateKind, len(templates))
	for _, template := range templates {
		declared[strings.ToLower(template.Name)] = template.Kind
	}
	entries, err := os.ReadDir(filepath.Join(directory, "templates"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("%s %s templates: %w", kind, name, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s %s keeps %q among its templates, and a template is a folder",
				kind, name, entry.Name())
		}
		if project.SubordinateName(entry.Name()) != nil {
			return fmt.Errorf("%s %s keeps a template folder %q, which is not a name", kind, name, entry.Name())
		}
		templateKind, ok := declared[strings.ToLower(entry.Name())]
		if !ok {
			return fmt.Errorf("%s %s keeps content for template %s, which it does not declare",
				kind, name, entry.Name())
		}
		content, err := os.ReadDir(filepath.Join(directory, "templates", entry.Name()))
		if err != nil {
			return fmt.Errorf("%s %s template %s: %w", kind, name, entry.Name(), err)
		}
		for _, file := range content {
			if templateKind == HTMLTemplate && file.IsDir() && file.Name() == project.TemplateResourcesDirectory {
				if err := validateTemplateResources(fmt.Sprintf("%s %s template %s", kind, name, entry.Name()),
					filepath.Join(directory, "templates", entry.Name(), file.Name())); err != nil {
					return err
				}
				continue
			}
			if templateKind == GraphicalSchema && file.IsDir() && file.Type()&fs.ModeSymlink == 0 && file.Name() == project.SchemaItemsDirectory {
				if err := validateSchemaItems(fmt.Sprintf("%s %s template %s", kind, name, entry.Name()),
					filepath.Join(directory, "templates", entry.Name(), file.Name())); err != nil {
					return err
				}
				continue
			}
			if file.IsDir() || file.Type()&fs.ModeSymlink != 0 {
				return fmt.Errorf("%s %s template %s holds %q, which is not a file of its content",
					kind, name, entry.Name(), file.Name())
			}
			if !templateContentFileName(templateKind, file.Name()) {
				return fmt.Errorf("%s %s template %s is a %s and cannot hold %q",
					kind, name, entry.Name(), templateKind, file.Name())
			}
		}
	}
	return nil
}

// validateTemplateResources checks the folder of an HTML template's resources:
// files only, and nothing deeper. What the documents refer to is not checked
// against it - the prototype keeps documents that name images it does not
// have, and a document is not ours to rewrite.
func validateTemplateResources(owner, directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("%s: %w", owner, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 || !project.TemplateResource(entry.Name()) {
			return fmt.Errorf("%s keeps %q among its resources, which is not a file", owner, entry.Name())
		}
	}
	return nil
}

// validateSchemaItems checks the folder of the pictures of a graphical
// schema's items: a folder per item, each holding files and nothing deeper.
// Which item a folder is for is not checked against the schema - the schema's
// content is carried, not read, until it is drawn.
func validateSchemaItems(owner, directory string) error {
	items, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("%s: %w", owner, err)
	}
	for _, item := range items {
		if !item.IsDir() || item.Type()&fs.ModeSymlink != 0 || !project.TemplateResource(item.Name()) {
			return fmt.Errorf("%s keeps %q among the pictures of its items, and each item keeps a folder named by it",
				owner, item.Name())
		}
		files, err := os.ReadDir(filepath.Join(directory, item.Name()))
		if err != nil {
			return fmt.Errorf("%s item %s: %w", owner, item.Name(), err)
		}
		for _, file := range files {
			if file.IsDir() || file.Type()&fs.ModeSymlink != 0 || !project.TemplateResource(file.Name()) {
				return fmt.Errorf("%s item %s keeps %q, which is not a picture file", owner, item.Name(), file.Name())
			}
		}
	}
	return nil
}

// templateContentFileName says whether a file inside a template's folder is
// content that kind of template may have. An HTML template keeps one document
// per language, so its file is named by the language; every other kind keeps
// one file under a fixed name.
func templateContentFileName(kind TemplateKind, file string) bool {
	if kind == HTMLTemplate {
		code, found := strings.CutSuffix(file, ".html")
		return found && validLanguageCode(code)
	}
	return file == kind.contentFile()
}

// validLanguageCode accepts the shape of a language code - see
// project.LanguageCodeShape.
func validLanguageCode(code string) bool {
	return project.LanguageCodeShape(code)
}

// validateHelpFolder checks the help of one owner: pages named by their
// language, and the folder of the pictures they show, holding files and
// nothing deeper. What the pages refer to is not checked - a page is the
// prototype's text and not ours to rewrite.
func validateHelpFolder(owner, directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("%s: %w", owner, err)
	}
	for _, entry := range entries {
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s keeps %q in its help, which is a symbolic link", owner, entry.Name())
		}
		if entry.IsDir() {
			if entry.Name() != project.TemplateResourcesDirectory {
				return fmt.Errorf("%s keeps a folder %q in its help, and help keeps only its pages and the folder %s of their pictures",
					owner, entry.Name(), project.TemplateResourcesDirectory)
			}
			if err := validateTemplateResources(owner+" help", filepath.Join(directory, entry.Name())); err != nil {
				return err
			}
			continue
		}
		if _, ok := project.HelpPage(entry.Name()); !ok {
			return fmt.Errorf("%s keeps %q in its help, which is not a page named by its language, such as ru.html", owner, entry.Name())
		}
	}
	return nil
}

// refusedMetadataKinds are the kinds of the prototype MetaLab does not carry at
// all - METADATA-OBJECTS.md, «От четырёх механизмов прототипа…». A folder of
// one in a project is refused with that reason rather than as an unknown one.
var refusedMetadataKinds = map[string]string{
	"bots":                 "bots are not carried by MetaLab",
	"integration-services": "integration services are not carried by MetaLab",
}

// validateMetadataDirectories refuses anything in metadata/ that is not the
// folder of a kind the platform reads. A folder nobody reads is a set of
// objects passed over in silence - the one outcome the conformance report
// calls unacceptable - and Studio's tree refused it while saving the data did
// not.
func validateMetadataDirectories(root string) error {
	entries, err := os.ReadDir(filepath.Join(root, "metadata"))
	if err != nil {
		return fmt.Errorf("read metadata directory: %w", err)
	}
	known := map[string]bool{}
	for _, kind := range project.MetadataKinds() {
		known[kind] = true
	}
	// Every such folder is named at once: removing them one error at a time is
	// a round trip per folder for no reason.
	var issues []string
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		path := filepath.ToSlash(filepath.Join("metadata", entry.Name()))
		if reason, refused := refusedMetadataKinds[entry.Name()]; refused {
			issues = append(issues, fmt.Sprintf("%q: %s", path, reason))
			continue
		}
		if !known[entry.Name()] || !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			issues = append(issues, fmt.Sprintf("%q: not a kind of metadata object", path))
		}
	}
	if len(issues) != 0 {
		return fmt.Errorf("unexpected metadata paths: %s", strings.Join(issues, "; "))
	}
	return nil
}

// validateRecalculationFiles checks the folder of a calculation register's
// recalculations: a folder per recalculation the register describes, holding
// at most its record set module. A folder no recalculation is named by is an
// orphan - its module would never run - and is refused, as an orphaned
// command is.
func validateRecalculationFiles(root string, item CalculationRegisterDefinition) error {
	if root == "" {
		return nil
	}
	relative, err := project.ObjectDirectory(string(CalculationRegisterKind), item.Name)
	if err != nil {
		return err
	}
	directory := filepath.Join(root, filepath.FromSlash(relative), project.RecalculationsDirectory)
	entries, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("calculation register %s recalculations: %w", item.Name, err)
	}
	known := map[string]bool{}
	for _, recalculation := range item.Recalculations {
		known[strings.ToLower(recalculation.Name)] = true
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("calculation register %s keeps %q among its recalculations, and a recalculation is a folder", item.Name, entry.Name())
		}
		if !known[strings.ToLower(entry.Name())] {
			return fmt.Errorf("calculation register %s keeps a folder for recalculation %s, which it does not describe", item.Name, entry.Name())
		}
		files, err := os.ReadDir(filepath.Join(directory, entry.Name()))
		if err != nil {
			return fmt.Errorf("calculation register %s recalculation %s: %w", item.Name, entry.Name(), err)
		}
		for _, file := range files {
			if file.IsDir() || !project.KeepsModule(string(RecalculationKind), file.Name()) {
				return fmt.Errorf("calculation register %s recalculation %s keeps %q, and a recalculation keeps only its record set module", item.Name, entry.Name(), file.Name())
			}
		}
	}
	return nil
}

// singleKind reports whether a type description is exactly one type of one
// kind - what "of the type Date" says of a field.
func singleKind(types []Type, kind TypeKind) bool {
	single, ok := SingleType(types)
	return ok && single.Kind == kind
}
