package metadata

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Load reads and cross-validates all currently supported application metadata.
func Load(root string) (*Catalog, error) {
	return load(root, true)
}

// Role editing must also work when an existing role has a dangling reference.
// Only the editor-schema path skips roles; runtime Load always validates them.
func load(root string, includeRoles bool) (*Catalog, error) {
	manifest, err := project.ValidateLayout(root)
	if err != nil {
		return nil, err
	}
	catalog := &Catalog{Project: manifest}
	if includeRoles {
		if err := loadKind(root, RoleKind, func(source string, file *os.File, id uuid.UUID) error {
			value, err := DecodeRole(source, file, manifest)
			if err == nil && value.ID != id {
				err = fmt.Errorf("metadata UUID %s does not match filename UUID %s", value.ID, id)
			}
			if err == nil {
				catalog.Roles = append(catalog.Roles, value)
			}
			return err
		}); err != nil {
			return nil, err
		}
	}
	if err := loadKind(root, SubsystemKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeSubsystem(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match filename UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.Subsystems = append(catalog.Subsystems, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadKind(root, ConstantKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeConstant(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match filename UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.Constants = append(catalog.Constants, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadKind(root, SessionParameterKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeSessionParameter(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match filename UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.SessionParameters = append(catalog.SessionParameters, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadKind(root, CommonAttributeKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeCommonAttribute(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match filename UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.CommonAttributes = append(catalog.CommonAttributes, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadKind(root, CommonModuleKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeCommonModule(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match filename UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.CommonModules = append(catalog.CommonModules, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadKind(root, EventSubscriptionKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeEventSubscription(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match filename UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.EventSubscriptions = append(catalog.EventSubscriptions, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadKind(root, EnumerationKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeEnumeration(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match filename UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.Enumerations = append(catalog.Enumerations, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadKind(root, DefinedTypeKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeDefinedType(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match filename UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.DefinedTypes = append(catalog.DefinedTypes, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, CatalogKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeCatalog(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match directory UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.Catalogs = append(catalog.Catalogs, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, ChartOfCharacteristicTypesKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeChartOfCharacteristicTypes(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match directory UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.ChartsOfCharacteristicTypes = append(catalog.ChartsOfCharacteristicTypes, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, ChartOfAccountsKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeChartOfAccounts(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match directory UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.ChartsOfAccounts = append(catalog.ChartsOfAccounts, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, ChartOfCalculationTypesKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeChartOfCalculationTypes(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match directory UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.ChartsOfCalculationTypes = append(catalog.ChartsOfCalculationTypes, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, DocumentKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeDocument(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match directory UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.Documents = append(catalog.Documents, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, InformationRegisterKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeInformationRegister(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match directory UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.InformationRegisters = append(catalog.InformationRegisters, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, AccumulationRegisterKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeAccumulationRegister(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match directory UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.AccumulationRegisters = append(catalog.AccumulationRegisters, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := catalog.indexAndValidate(root); err != nil {
		return nil, err
	}
	return catalog, nil
}

// NewCatalogSnapshot validates already decoded metadata, for example from a publication package.
func NewCatalogSnapshot(manifest project.Project, constants []Constant, enumerations []Enumeration, definedTypes []DefinedTypeObject, catalogs []CatalogDefinition, documents []DocumentDefinition, informationRegisters []InformationRegisterDefinition) (*Catalog, error) {
	return NewCatalogSnapshotWithAccumulationRegisters(manifest, constants, enumerations, definedTypes, catalogs, documents, informationRegisters, nil)
}

// NewCatalogSnapshotWithAccumulationRegisters is the compatibility constructor
// for metadata without project roles.
func NewCatalogSnapshotWithAccumulationRegisters(manifest project.Project, constants []Constant, enumerations []Enumeration, definedTypes []DefinedTypeObject, catalogs []CatalogDefinition, documents []DocumentDefinition, informationRegisters []InformationRegisterDefinition, accumulationRegisters []AccumulationRegisterDefinition) (*Catalog, error) {
	return NewCatalogSnapshotWithRoles(manifest, constants, enumerations, definedTypes, catalogs, documents, informationRegisters, accumulationRegisters, nil)
}

// NewCatalogSnapshotWithRoles is the compatibility constructor for metadata
// without project subsystems.
func NewCatalogSnapshotWithRoles(manifest project.Project, constants []Constant, enumerations []Enumeration, definedTypes []DefinedTypeObject, catalogs []CatalogDefinition, documents []DocumentDefinition, informationRegisters []InformationRegisterDefinition, accumulationRegisters []AccumulationRegisterDefinition, roles []RoleDefinition) (*Catalog, error) {
	return NewCatalogSnapshotWithSubsystems(manifest, constants, enumerations, definedTypes, catalogs, documents, informationRegisters, accumulationRegisters, roles, nil)
}

// NewCatalogSnapshotWithSubsystems is the compatibility constructor for
// metadata without project session parameters.
func NewCatalogSnapshotWithSubsystems(manifest project.Project, constants []Constant, enumerations []Enumeration, definedTypes []DefinedTypeObject, catalogs []CatalogDefinition, documents []DocumentDefinition, informationRegisters []InformationRegisterDefinition, accumulationRegisters []AccumulationRegisterDefinition, roles []RoleDefinition, subsystems []SubsystemDefinition) (*Catalog, error) {
	return NewCatalogSnapshotWithSessionParameters(manifest, constants, enumerations, definedTypes, catalogs, documents, informationRegisters, accumulationRegisters, roles, subsystems, nil)
}

// NewCatalogSnapshotWithSessionParameters is the compatibility constructor
// for metadata without project common attributes.
func NewCatalogSnapshotWithSessionParameters(manifest project.Project, constants []Constant, enumerations []Enumeration, definedTypes []DefinedTypeObject, catalogs []CatalogDefinition, documents []DocumentDefinition, informationRegisters []InformationRegisterDefinition, accumulationRegisters []AccumulationRegisterDefinition, roles []RoleDefinition, subsystems []SubsystemDefinition, sessionParameters []SessionParameter) (*Catalog, error) {
	return NewCatalogSnapshotWithCommonAttributes(manifest, constants, enumerations, definedTypes, catalogs, documents, informationRegisters, accumulationRegisters, roles, subsystems, sessionParameters, nil)
}

// NewCatalogSnapshotWithCommonAttributes is the compatibility constructor
// for metadata without project common modules.
func NewCatalogSnapshotWithCommonAttributes(manifest project.Project, constants []Constant, enumerations []Enumeration, definedTypes []DefinedTypeObject, catalogs []CatalogDefinition, documents []DocumentDefinition, informationRegisters []InformationRegisterDefinition, accumulationRegisters []AccumulationRegisterDefinition, roles []RoleDefinition, subsystems []SubsystemDefinition, sessionParameters []SessionParameter, commonAttributes []CommonAttributeDefinition) (*Catalog, error) {
	return NewCatalogSnapshotWithCommonModules(manifest, constants, enumerations, definedTypes, catalogs, documents, informationRegisters, accumulationRegisters, roles, subsystems, sessionParameters, commonAttributes, nil)
}

// NewCatalogSnapshotWithCommonModules is the compatibility constructor for
// metadata without project event subscriptions.
func NewCatalogSnapshotWithCommonModules(manifest project.Project, constants []Constant, enumerations []Enumeration, definedTypes []DefinedTypeObject, catalogs []CatalogDefinition, documents []DocumentDefinition, informationRegisters []InformationRegisterDefinition, accumulationRegisters []AccumulationRegisterDefinition, roles []RoleDefinition, subsystems []SubsystemDefinition, sessionParameters []SessionParameter, commonAttributes []CommonAttributeDefinition, commonModules []CommonModuleDefinition) (*Catalog, error) {
	return NewCatalogSnapshotWithEventSubscriptions(manifest, constants, enumerations, definedTypes, catalogs, documents, informationRegisters, accumulationRegisters, roles, subsystems, sessionParameters, commonAttributes, commonModules, nil)
}

// NewCatalogSnapshotWithEventSubscriptions validates decoded metadata, role
// object/field references, subsystem membership, common attribute
// propagation and event subscription references. Form command references
// require the full RuntimeSnapshot.
func NewCatalogSnapshotWithEventSubscriptions(manifest project.Project, constants []Constant, enumerations []Enumeration, definedTypes []DefinedTypeObject, catalogs []CatalogDefinition, documents []DocumentDefinition, informationRegisters []InformationRegisterDefinition, accumulationRegisters []AccumulationRegisterDefinition, roles []RoleDefinition, subsystems []SubsystemDefinition, sessionParameters []SessionParameter, commonAttributes []CommonAttributeDefinition, commonModules []CommonModuleDefinition, eventSubscriptions []EventSubscriptionDefinition) (*Catalog, error) {
	result := &Catalog{
		Project: manifest, Constants: slices.Clone(constants), Enumerations: slices.Clone(enumerations),
		DefinedTypes: slices.Clone(definedTypes), Catalogs: slices.Clone(catalogs), Documents: slices.Clone(documents),
		InformationRegisters:  slices.Clone(informationRegisters),
		AccumulationRegisters: slices.Clone(accumulationRegisters),
		Roles:                 slices.Clone(roles),
		Subsystems:            slices.Clone(subsystems),
		SessionParameters:     slices.Clone(sessionParameters),
		CommonAttributes:      slices.Clone(commonAttributes),
		CommonModules:         slices.Clone(commonModules),
		EventSubscriptions:    slices.Clone(eventSubscriptions),
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

func loadKind(root string, kind Kind, decode func(string, *os.File, uuid.UUID) error) error {
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
	count := 0
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		count++
		if count > maxObjectsPerKind {
			return fmt.Errorf("metadata %s exceeds %d objects", kind, maxObjectsPerKind)
		}
		if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 || filepath.Ext(entry.Name()) != ".yaml" {
			return fmt.Errorf("unexpected metadata source %q", filepath.Join("metadata", string(kind), entry.Name()))
		}
		id, err := uuid.Parse(strings.TrimSuffix(entry.Name(), ".yaml"))
		if err != nil {
			return fmt.Errorf("metadata source %q must use a UUID filename: %w", entry.Name(), err)
		}
		relative := filepath.ToSlash(filepath.Join("metadata", string(kind), entry.Name()))
		file, err := os.Open(filepath.Join(directory, entry.Name()))
		if err != nil {
			return fmt.Errorf("open %s: %w", relative, err)
		}
		decodeErr := decode(relative, file, id)
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

// loadObjectKind scans metadata/<kind>/ for per-object folders (named by the
// object's own UUID) and decodes the fixed-name object.yaml inside each one.
// Unlike loadKind, entries are directories, not flat <uuid>.yaml files — this
// is how catalogs, documents and registers group their own description with
// their module(s) and managed forms, physically located inside that same
// folder (see validateObjectFileSources).
func loadObjectKind(root string, kind Kind, decode func(string, *os.File, uuid.UUID) error) error {
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
	count := 0
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		count++
		if count > maxObjectsPerKind {
			return fmt.Errorf("metadata %s exceeds %d objects", kind, maxObjectsPerKind)
		}
		if !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("unexpected metadata source %q", filepath.Join("metadata", string(kind), entry.Name()))
		}
		id, err := uuid.Parse(entry.Name())
		if err != nil {
			return fmt.Errorf("metadata object %q must use a UUID directory name: %w", entry.Name(), err)
		}
		relative := filepath.ToSlash(filepath.Join("metadata", string(kind), entry.Name(), "object.yaml"))
		file, err := os.Open(filepath.Join(directory, entry.Name(), "object.yaml"))
		if err != nil {
			return fmt.Errorf("open %s: %w", relative, err)
		}
		decodeErr := decode(relative, file, id)
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
	if err := catalog.propagateCommonAttributes(); err != nil {
		return err
	}
	sort.Slice(catalog.Roles, func(i, j int) bool { return catalog.Roles[i].ID.String() < catalog.Roles[j].ID.String() })
	catalog.roleByName, catalog.roleByID = make(map[string]int, len(catalog.Roles)), make(map[uuid.UUID]int, len(catalog.Roles))
	sort.Slice(catalog.Subsystems, func(i, j int) bool { return catalog.Subsystems[i].ID.String() < catalog.Subsystems[j].ID.String() })
	catalog.subsystemByName, catalog.subsystemByID = make(map[string]int, len(catalog.Subsystems)), make(map[uuid.UUID]int, len(catalog.Subsystems))
	sort.Slice(catalog.Constants, func(i, j int) bool { return catalog.Constants[i].ID.String() < catalog.Constants[j].ID.String() })
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
	catalog.informationRegisterByName, catalog.informationRegisterByID = make(map[string]int, len(catalog.InformationRegisters)), make(map[uuid.UUID]int, len(catalog.InformationRegisters))
	catalog.accumulationRegisterByName, catalog.accumulationRegisterByID = make(map[string]int, len(catalog.AccumulationRegisters)), make(map[uuid.UUID]int, len(catalog.AccumulationRegisters))
	allIDs := map[uuid.UUID]string{}
	add := func(kind string, id uuid.UUID, name string, index int, names map[string]int, ids map[uuid.UUID]int) error {
		folded := strings.ToLower(name)
		if _, exists := names[folded]; exists {
			return fmt.Errorf("%w: %s %q", ErrDuplicateName, kind, name)
		}
		if previous, exists := allIDs[id]; exists {
			return fmt.Errorf("%w: %s and %s use %s", ErrDuplicateID, previous, kind+" "+name, id)
		}
		names[folded], allIDs[id] = index, kind+" "+name
		if ids != nil {
			ids[id] = index
		}
		return nil
	}
	for index, item := range catalog.Roles {
		if err := ValidateRole("role "+item.Name, item, catalog.Project); err != nil {
			return err
		}
		if err := add("role", item.ID, item.Name, index, catalog.roleByName, catalog.roleByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.Subsystems {
		if err := add("subsystem", item.ID, item.Name, index, catalog.subsystemByName, catalog.subsystemByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.Constants {
		if err := add("constant", item.ID, item.Name, index, catalog.constantByName, catalog.constantByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.SessionParameters {
		if err := add("session parameter", item.ID, item.Name, index, catalog.sessionParameterByName, catalog.sessionParameterByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.CommonAttributes {
		if err := add("common attribute", item.ID, item.Name, index, catalog.commonAttributeByName, catalog.commonAttributeByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.CommonModules {
		if err := add("common module", item.ID, item.Name, index, catalog.commonModuleByName, catalog.commonModuleByID); err != nil {
			return err
		}
		if previous, exists := catalog.commonModuleByModuleID[item.Module]; exists {
			return fmt.Errorf("%w: common module %s and %s share the same module source %s", ErrDuplicateID, catalog.CommonModules[previous].Name, item.Name, item.Module)
		}
		catalog.commonModuleByModuleID[item.Module] = index
	}
	for index, item := range catalog.EventSubscriptions {
		if err := add("event subscription", item.ID, item.Name, index, catalog.eventSubscriptionByName, catalog.eventSubscriptionByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.Enumerations {
		if err := add("enumeration", item.ID, item.Name, index, catalog.enumerationByName, catalog.enumerationByID); err != nil {
			return err
		}
		for _, value := range item.Values {
			if previous, ok := allIDs[value.ID]; ok {
				return fmt.Errorf("%w: %s and enumeration value %s.%s use %s", ErrDuplicateID, previous, item.Name, value.Name, value.ID)
			}
			allIDs[value.ID] = "enumeration value " + item.Name + "." + value.Name
		}
	}
	for index, item := range catalog.DefinedTypes {
		if err := add("defined type", item.ID, item.Name, index, catalog.definedTypeByName, catalog.definedTypeByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.Catalogs {
		if err := add("catalog", item.ID, item.Name, index, catalog.catalogByName, catalog.catalogByID); err != nil {
			return err
		}
		for _, predefined := range item.Predefined {
			if previous, ok := allIDs[predefined.ID]; ok {
				return fmt.Errorf("%w: %s and predefined catalog item %s.%s use %s", ErrDuplicateID, previous, item.Name, predefined.Name, predefined.ID)
			}
			allIDs[predefined.ID] = "predefined catalog item " + item.Name + "." + predefined.Name
		}
		for _, attribute := range item.Attributes {
			if _, common := catalog.commonAttributeByID[attribute.ID]; common {
				continue
			}
			if previous, ok := allIDs[attribute.ID]; ok {
				return fmt.Errorf("%w: %s and catalog attribute %s.%s use %s", ErrDuplicateID, previous, item.Name, attribute.Name, attribute.ID)
			}
			allIDs[attribute.ID] = "catalog attribute " + item.Name + "." + attribute.Name
		}
		for _, part := range item.TableParts {
			if previous, ok := allIDs[part.ID]; ok {
				return fmt.Errorf("%w: %s and catalog table part %s.%s use %s", ErrDuplicateID, previous, item.Name, part.Name, part.ID)
			}
			allIDs[part.ID] = "catalog table part " + item.Name + "." + part.Name
			for _, attribute := range part.Attributes {
				if previous, ok := allIDs[attribute.ID]; ok {
					return fmt.Errorf("%w: %s and catalog table part attribute %s.%s.%s use %s", ErrDuplicateID, previous, item.Name, part.Name, attribute.Name, attribute.ID)
				}
				allIDs[attribute.ID] = "catalog table part attribute " + item.Name + "." + part.Name + "." + attribute.Name
			}
		}
	}
	for index, item := range catalog.ChartsOfCharacteristicTypes {
		if err := add("chart of characteristic types", item.ID, item.Name, index, catalog.chartOfCharacteristicTypesByName, catalog.chartOfCharacteristicTypesByID); err != nil {
			return err
		}
		for _, predefined := range item.Predefined {
			if previous, ok := allIDs[predefined.ID]; ok {
				return fmt.Errorf("%w: %s and predefined characteristic %s.%s use %s", ErrDuplicateID, previous, item.Name, predefined.Name, predefined.ID)
			}
			allIDs[predefined.ID] = "predefined characteristic " + item.Name + "." + predefined.Name
		}
		for _, attribute := range item.Attributes {
			if _, common := catalog.commonAttributeByID[attribute.ID]; common {
				continue
			}
			if previous, ok := allIDs[attribute.ID]; ok {
				return fmt.Errorf("%w: %s and chart of characteristic types attribute %s.%s use %s", ErrDuplicateID, previous, item.Name, attribute.Name, attribute.ID)
			}
			allIDs[attribute.ID] = "chart of characteristic types attribute " + item.Name + "." + attribute.Name
		}
		for _, part := range item.TableParts {
			if previous, ok := allIDs[part.ID]; ok {
				return fmt.Errorf("%w: %s and chart of characteristic types table part %s.%s use %s", ErrDuplicateID, previous, item.Name, part.Name, part.ID)
			}
			allIDs[part.ID] = "chart of characteristic types table part " + item.Name + "." + part.Name
			for _, attribute := range part.Attributes {
				if previous, ok := allIDs[attribute.ID]; ok {
					return fmt.Errorf("%w: %s and chart of characteristic types table part attribute %s.%s.%s use %s", ErrDuplicateID, previous, item.Name, part.Name, attribute.Name, attribute.ID)
				}
				allIDs[attribute.ID] = "chart of characteristic types table part attribute " + item.Name + "." + part.Name + "." + attribute.Name
			}
		}
	}
	for index, item := range catalog.ChartsOfAccounts {
		if err := add("chart of accounts", item.ID, item.Name, index, catalog.chartOfAccountsByName, catalog.chartOfAccountsByID); err != nil {
			return err
		}
		for _, account := range item.Predefined {
			if previous, ok := allIDs[account.ID]; ok {
				return fmt.Errorf("%w: %s and predefined account %s.%s use %s", ErrDuplicateID, previous, item.Name, account.Name, account.ID)
			}
			allIDs[account.ID] = "predefined account " + item.Name + "." + account.Name
		}
		// A flag is a column of its own on every account, so its identity has
		// to be as unique as an attribute's.
		for _, flag := range append(slices.Clone(item.AccountingFlags), item.ExtDimensionAccountingFlags...) {
			if previous, ok := allIDs[flag.ID]; ok {
				return fmt.Errorf("%w: %s and accounting flag %s.%s use %s", ErrDuplicateID, previous, item.Name, flag.Name, flag.ID)
			}
			allIDs[flag.ID] = "accounting flag " + item.Name + "." + flag.Name
		}
		for _, attribute := range item.Attributes {
			if _, common := catalog.commonAttributeByID[attribute.ID]; common {
				continue
			}
			if previous, ok := allIDs[attribute.ID]; ok {
				return fmt.Errorf("%w: %s and chart of accounts attribute %s.%s use %s", ErrDuplicateID, previous, item.Name, attribute.Name, attribute.ID)
			}
			allIDs[attribute.ID] = "chart of accounts attribute " + item.Name + "." + attribute.Name
		}
		for _, part := range item.TableParts {
			if previous, ok := allIDs[part.ID]; ok {
				return fmt.Errorf("%w: %s and chart of accounts table part %s.%s use %s", ErrDuplicateID, previous, item.Name, part.Name, part.ID)
			}
			allIDs[part.ID] = "chart of accounts table part " + item.Name + "." + part.Name
			for _, attribute := range part.Attributes {
				if previous, ok := allIDs[attribute.ID]; ok {
					return fmt.Errorf("%w: %s and chart of accounts table part attribute %s.%s.%s use %s", ErrDuplicateID, previous, item.Name, part.Name, attribute.Name, attribute.ID)
				}
				allIDs[attribute.ID] = "chart of accounts table part attribute " + item.Name + "." + part.Name + "." + attribute.Name
			}
		}
	}
	for index, item := range catalog.ChartsOfCalculationTypes {
		if err := add("chart of calculation types", item.ID, item.Name, index, catalog.chartOfCalculationTypesByName, catalog.chartOfCalculationTypesByID); err != nil {
			return err
		}
		for _, predefined := range item.Predefined {
			if previous, ok := allIDs[predefined.ID]; ok {
				return fmt.Errorf("%w: %s and predefined calculation type %s.%s use %s", ErrDuplicateID, previous, item.Name, predefined.Name, predefined.ID)
			}
			allIDs[predefined.ID] = "predefined calculation type " + item.Name + "." + predefined.Name
		}
		for _, attribute := range item.Attributes {
			if _, common := catalog.commonAttributeByID[attribute.ID]; common {
				continue
			}
			if previous, ok := allIDs[attribute.ID]; ok {
				return fmt.Errorf("%w: %s and chart of calculation types attribute %s.%s use %s", ErrDuplicateID, previous, item.Name, attribute.Name, attribute.ID)
			}
			allIDs[attribute.ID] = "chart of calculation types attribute " + item.Name + "." + attribute.Name
		}
		for _, part := range item.TableParts {
			if previous, ok := allIDs[part.ID]; ok {
				return fmt.Errorf("%w: %s and chart of calculation types table part %s.%s use %s", ErrDuplicateID, previous, item.Name, part.Name, part.ID)
			}
			allIDs[part.ID] = "chart of calculation types table part " + item.Name + "." + part.Name
			for _, attribute := range part.Attributes {
				if previous, ok := allIDs[attribute.ID]; ok {
					return fmt.Errorf("%w: %s and chart of calculation types table part attribute %s.%s.%s use %s", ErrDuplicateID, previous, item.Name, part.Name, attribute.Name, attribute.ID)
				}
				allIDs[attribute.ID] = "chart of calculation types table part attribute " + item.Name + "." + part.Name + "." + attribute.Name
			}
		}
	}
	for index, item := range catalog.Documents {
		if err := add("document", item.ID, item.Name, index, catalog.documentByName, catalog.documentByID); err != nil {
			return err
		}
		for _, attribute := range item.Attributes {
			if _, common := catalog.commonAttributeByID[attribute.ID]; common {
				continue
			}
			if previous, ok := allIDs[attribute.ID]; ok {
				return fmt.Errorf("%w: %s and document attribute %s.%s use %s", ErrDuplicateID, previous, item.Name, attribute.Name, attribute.ID)
			}
			allIDs[attribute.ID] = "document attribute " + item.Name + "." + attribute.Name
		}
		for _, part := range item.TableParts {
			if previous, ok := allIDs[part.ID]; ok {
				return fmt.Errorf("%w: %s and document table part %s.%s use %s", ErrDuplicateID, previous, item.Name, part.Name, part.ID)
			}
			allIDs[part.ID] = "document table part " + item.Name + "." + part.Name
			for _, attribute := range part.Attributes {
				if previous, ok := allIDs[attribute.ID]; ok {
					return fmt.Errorf("%w: %s and document table part attribute %s.%s.%s use %s", ErrDuplicateID, previous, item.Name, part.Name, attribute.Name, attribute.ID)
				}
				allIDs[attribute.ID] = "document table part attribute " + item.Name + "." + part.Name + "." + attribute.Name
			}
		}
	}
	for index, item := range catalog.InformationRegisters {
		if err := add("information register", item.ID, item.Name, index, catalog.informationRegisterByName, catalog.informationRegisterByID); err != nil {
			return err
		}
		for _, field := range informationRegisterFields(item) {
			if _, common := catalog.commonAttributeByID[field.ID]; common {
				continue
			}
			if previous, ok := allIDs[field.ID]; ok {
				return fmt.Errorf("%w: %s and information register field %s.%s use %s", ErrDuplicateID, previous, item.Name, field.Name, field.ID)
			}
			allIDs[field.ID] = "information register field " + item.Name + "." + field.Name
		}
	}
	for index, item := range catalog.AccumulationRegisters {
		if err := add("accumulation register", item.ID, item.Name, index, catalog.accumulationRegisterByName, catalog.accumulationRegisterByID); err != nil {
			return err
		}
		for _, field := range accumulationRegisterFields(item) {
			if _, common := catalog.commonAttributeByID[field.ID]; common {
				continue
			}
			if previous, ok := allIDs[field.ID]; ok {
				return fmt.Errorf("%w: %s and accumulation register field %s.%s use %s", ErrDuplicateID, previous, item.Name, field.Name, field.ID)
			}
			allIDs[field.ID] = "accumulation register field " + item.Name + "." + field.Name
		}
	}
	for _, item := range catalog.Constants {
		if err := catalog.validateReferences("constant "+item.Name, item.Types); err != nil {
			return err
		}
		if item.Default != nil {
			if _, err := catalog.NormalizeValue(item, *item.Default); err != nil {
				return fmt.Errorf("constant %s default: %w", item.Name, err)
			}
		}
	}
	for _, item := range catalog.SessionParameters {
		if err := catalog.validateReferences("session parameter "+item.Name, item.Types); err != nil {
			return err
		}
		if item.Default != nil {
			if _, err := catalog.NormalizeSessionParameterValue(item, *item.Default); err != nil {
				return fmt.Errorf("session parameter %s default: %w", item.Name, err)
			}
		}
	}
	for _, item := range catalog.CommonAttributes {
		if err := catalog.validateReferences("common attribute "+item.Name, item.Types); err != nil {
			return err
		}
	}
	for _, item := range catalog.CommonModules {
		if err := validateCommonModuleSource(root, item.Name, item.Module); err != nil {
			return err
		}
	}
	for _, item := range catalog.DefinedTypes {
		if err := catalog.validateReferences("defined type "+item.Name, item.Types); err != nil {
			return err
		}
	}
	for _, item := range catalog.Catalogs {
		for _, attribute := range item.Attributes {
			if err := catalog.validateReferences("catalog "+item.Name+" attribute "+attribute.Name, attribute.Types); err != nil {
				return err
			}
		}
		for _, part := range item.TableParts {
			for _, attribute := range part.Attributes {
				if err := catalog.validateReferences("catalog "+item.Name+" table part "+part.Name+" attribute "+attribute.Name, attribute.Types); err != nil {
					return err
				}
			}
		}
		for _, predefined := range item.Predefined {
			values := make(map[uuid.UUID]Value, len(predefined.Attributes))
			for name, value := range predefined.Attributes {
				attribute, ok := findCatalogAttribute(item.Attributes, name)
				if !ok {
					return fmt.Errorf("predefined catalog item %s.%s has unknown attribute %s", item.Name, predefined.Name, name)
				}
				values[attribute.ID] = value
			}
			if _, err := catalog.normalizeAttributes(item.Name+"."+predefined.Name, item.Attributes, values); err != nil {
				return fmt.Errorf("predefined catalog item %s.%s: %w", item.Name, predefined.Name, err)
			}
		}
		if err := validateObjectFileSources(root, CatalogKind, item.ID, "catalog", item.Name, item.ObjectModule, item.ManagerModule, item.Forms); err != nil {
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
		for _, attribute := range item.Attributes {
			if err := catalog.validateReferences(owner+" attribute "+attribute.Name, attribute.Types); err != nil {
				return err
			}
		}
		for _, part := range item.TableParts {
			for _, attribute := range part.Attributes {
				if err := catalog.validateReferences(owner+" table part "+part.Name+" attribute "+attribute.Name, attribute.Types); err != nil {
					return err
				}
			}
		}
		for _, predefined := range item.Predefined {
			values := make(map[uuid.UUID]Value, len(predefined.Attributes))
			for name, value := range predefined.Attributes {
				attribute, ok := findCatalogAttribute(item.Attributes, name)
				if !ok {
					return fmt.Errorf("predefined characteristic %s.%s has unknown attribute %s", item.Name, predefined.Name, name)
				}
				values[attribute.ID] = value
			}
			if _, err := catalog.normalizeAttributes(item.Name+"."+predefined.Name, item.Attributes, values); err != nil {
				return fmt.Errorf("predefined characteristic %s.%s: %w", item.Name, predefined.Name, err)
			}
		}
		if err := validateObjectFileSources(root, ChartOfCharacteristicTypesKind, item.ID, "chart of characteristic types", item.Name, item.ObjectModule, item.ManagerModule, item.Forms); err != nil {
			return err
		}
	}
	for _, item := range catalog.ChartsOfAccounts {
		owner := "chart of accounts " + item.Name
		if err := catalog.validateChartOfAccountsAnalytics(owner, item); err != nil {
			return err
		}
		for _, attribute := range item.Attributes {
			if err := catalog.validateReferences(owner+" attribute "+attribute.Name, attribute.Types); err != nil {
				return err
			}
		}
		for _, part := range item.TableParts {
			for _, attribute := range part.Attributes {
				if err := catalog.validateReferences(owner+" table part "+part.Name+" attribute "+attribute.Name, attribute.Types); err != nil {
					return err
				}
			}
		}
		if err := validateObjectFileSources(root, ChartOfAccountsKind, item.ID, "chart of accounts", item.Name, item.ObjectModule, item.ManagerModule, item.Forms); err != nil {
			return err
		}
	}
	for _, item := range catalog.ChartsOfCalculationTypes {
		owner := "chart of calculation types " + item.Name
		if err := catalog.validateCalculationBase(owner, item); err != nil {
			return err
		}
		for _, attribute := range item.Attributes {
			if err := catalog.validateReferences(owner+" attribute "+attribute.Name, attribute.Types); err != nil {
				return err
			}
		}
		for _, part := range item.TableParts {
			for _, attribute := range part.Attributes {
				if err := catalog.validateReferences(owner+" table part "+part.Name+" attribute "+attribute.Name, attribute.Types); err != nil {
					return err
				}
			}
		}
		if err := validateObjectFileSources(root, ChartOfCalculationTypesKind, item.ID, "chart of calculation types", item.Name, item.ObjectModule, item.ManagerModule, item.Forms); err != nil {
			return err
		}
	}
	for _, item := range catalog.Documents {
		for _, attribute := range item.Attributes {
			if err := catalog.validateReferences("document "+item.Name+" attribute "+attribute.Name, attribute.Types); err != nil {
				return err
			}
		}
		for _, part := range item.TableParts {
			for _, attribute := range part.Attributes {
				if err := catalog.validateReferences("document "+item.Name+" table part "+part.Name+" attribute "+attribute.Name, attribute.Types); err != nil {
					return err
				}
			}
		}
		if err := validateObjectFileSources(root, DocumentKind, item.ID, "document", item.Name, item.ObjectModule, item.ManagerModule, item.Forms); err != nil {
			return err
		}
	}
	for _, item := range catalog.InformationRegisters {
		for _, field := range informationRegisterFields(item) {
			if err := catalog.validateReferences("information register "+item.Name+" field "+field.Name, field.Types); err != nil {
				return err
			}
		}
		for _, recorder := range item.Recorders {
			if _, ok := catalog.documentByID[recorder]; !ok {
				return fmt.Errorf("information register %s references unknown recorder document %s", item.Name, recorder)
			}
		}
		if err := validateObjectFileSources(root, InformationRegisterKind, item.ID, "information register", item.Name, item.RecordSetModule, item.ManagerModule, item.Forms); err != nil {
			return err
		}
	}
	for _, item := range catalog.AccumulationRegisters {
		for _, field := range accumulationRegisterFields(item) {
			if err := catalog.validateReferences("accumulation register "+item.Name+" field "+field.Name, field.Types); err != nil {
				return err
			}
		}
		for _, resource := range item.Resources {
			if _, err := catalog.accumulationResourceType(resource); err != nil {
				return fmt.Errorf("accumulation register %s: %w", item.Name, err)
			}
		}
		for _, recorder := range item.Recorders {
			if _, ok := catalog.documentByID[recorder]; !ok {
				return fmt.Errorf("accumulation register %s references unknown recorder document %s", item.Name, recorder)
			}
		}
		if err := validateObjectFileSources(root, AccumulationRegisterKind, item.ID, "accumulation register", item.Name, item.RecordSetModule, item.ManagerModule, item.Forms); err != nil {
			return err
		}
	}
	if err := catalog.validateRoleReferences(root); err != nil {
		return err
	}
	if err := catalog.validateSubsystemReferences(); err != nil {
		return err
	}
	if err := catalog.validateEventSubscriptionReferences(); err != nil {
		return err
	}
	return catalog.validateDefinedTypeCycles()
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

func (catalog *Catalog) validateReferences(owner string, types []Type) error {
	for _, item := range types {
		if item.Reference == nil {
			continue
		}
		switch item.Kind {
		case EnumerationType:
			if _, ok := catalog.enumerationByID[*item.Reference]; !ok {
				return fmt.Errorf("%s references unknown enumeration %s", owner, item.Reference)
			}
		case DefinedType:
			if _, ok := catalog.definedTypeByID[*item.Reference]; !ok {
				return fmt.Errorf("%s references unknown defined type %s", owner, item.Reference)
			}
		case CatalogType:
			if _, ok := catalog.catalogByID[*item.Reference]; !ok {
				return fmt.Errorf("%s references unknown catalog %s", owner, item.Reference)
			}
		case DocumentType:
			if _, ok := catalog.documentByID[*item.Reference]; !ok {
				return fmt.Errorf("%s references unknown document %s", owner, item.Reference)
			}
		case CharacteristicTypesType:
			if _, ok := catalog.chartOfCharacteristicTypesByID[*item.Reference]; !ok {
				return fmt.Errorf("%s references unknown chart of characteristic types %s", owner, item.Reference)
			}
		case AccountType:
			if _, ok := catalog.chartOfAccountsByID[*item.Reference]; !ok {
				return fmt.Errorf("%s references unknown chart of accounts %s", owner, item.Reference)
			}
		case CalculationTypeType:
			if _, ok := catalog.chartOfCalculationTypesByID[*item.Reference]; !ok {
				return fmt.Errorf("%s references unknown chart of calculation types %s", owner, item.Reference)
			}
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

// validateObjectFileSources checks that a catalog/document/register's own
// module(s) and managed forms exist inside its per-object folder
// (metadata/<kind>/<id>/), physically grouped with its own description.
func validateObjectFileSources(root string, directoryKind Kind, id uuid.UUID, kind, name string, objectModule, managerModule *uuid.UUID, forms ObjectForms) error {
	if root == "" {
		return nil
	}
	directory := filepath.Join(root, "metadata", string(directoryKind), id.String())
	type source struct {
		role, path string
		id         *uuid.UUID
	}
	build := func(role string, sourceID *uuid.UUID, isForm bool) source {
		if sourceID == nil {
			return source{role: role}
		}
		if isForm {
			return source{role: role, id: sourceID, path: filepath.Join(directory, "forms", sourceID.String()+".yaml")}
		}
		return source{role: role, id: sourceID, path: filepath.Join(directory, sourceID.String()+".bsl")}
	}
	sources := []source{
		build("object module", objectModule, false),
		build("manager module", managerModule, false),
		build("object form", forms.Object, true),
		build("list form", forms.List, true),
		build("choice form", forms.Choice, true),
	}
	for _, source := range sources {
		if source.id == nil {
			continue
		}
		info, err := os.Lstat(source.path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s %s %s %s is missing or unsafe", kind, name, source.role, source.id)
		}
	}
	return nil
}

func (catalog *Catalog) validateDefinedTypeCycles() error {
	state := make(map[uuid.UUID]uint8, len(catalog.DefinedTypes))
	var visit func(uuid.UUID) error
	visit = func(id uuid.UUID) error {
		if state[id] == 1 {
			return fmt.Errorf("defined type reference cycle contains %s", id)
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		item := catalog.DefinedTypes[catalog.definedTypeByID[id]]
		for _, allowed := range item.Types {
			if allowed.Kind == DefinedType && allowed.Reference != nil {
				if err := visit(*allowed.Reference); err != nil {
					return err
				}
			}
		}
		state[id] = 2
		return nil
	}
	for _, item := range catalog.DefinedTypes {
		if err := visit(item.ID); err != nil {
			return err
		}
	}
	return nil
}
