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
	if err := loadObjectKind(root, EnumerationKind, func(source string, file *os.File, folder string) error {
		value, err := DecodeEnumeration(source, file, manifest)
		if err == nil && value.Name != folder {
			err = fmt.Errorf("object %s lies in folder %s", value.Name, folder)
		}
		if err == nil {
			catalog.Enumerations = append(catalog.Enumerations, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadKind(root, ScheduledJobKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeScheduledJob(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match filename UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.ScheduledJobs = append(catalog.ScheduledJobs, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadKind(root, FunctionalOptionKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeFunctionalOption(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match filename UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.FunctionalOptions = append(catalog.FunctionalOptions, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadKind(root, FunctionalOptionParameterKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeFunctionalOptionParameter(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match filename UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.FunctionalOptionParameters = append(catalog.FunctionalOptionParameters, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, SettingsStorageKind, func(source string, file *os.File, folder string) error {
		value, err := DecodeSettingsStorage(source, file, manifest)
		if err == nil && value.Name != folder {
			err = fmt.Errorf("object %s lies in folder %s", value.Name, folder)
		}
		if err == nil {
			catalog.SettingsStorages = append(catalog.SettingsStorages, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, FilterCriterionKind, func(source string, file *os.File, folder string) error {
		value, err := DecodeFilterCriterion(source, file, manifest)
		if err == nil && value.Name != folder {
			err = fmt.Errorf("object %s lies in folder %s", value.Name, folder)
		}
		if err == nil {
			catalog.FilterCriteria = append(catalog.FilterCriteria, value)
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
	if err := loadObjectKind(root, CatalogKind, func(source string, file *os.File, folder string) error {
		value, err := DecodeCatalog(source, file, manifest)
		if err == nil && value.Name != folder {
			err = fmt.Errorf("object %s lies in folder %s", value.Name, folder)
		}
		if err == nil {
			catalog.Catalogs = append(catalog.Catalogs, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, ChartOfCharacteristicTypesKind, func(source string, file *os.File, folder string) error {
		value, err := DecodeChartOfCharacteristicTypes(source, file, manifest)
		if err == nil && value.Name != folder {
			err = fmt.Errorf("object %s lies in folder %s", value.Name, folder)
		}
		if err == nil {
			catalog.ChartsOfCharacteristicTypes = append(catalog.ChartsOfCharacteristicTypes, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, ChartOfAccountsKind, func(source string, file *os.File, folder string) error {
		value, err := DecodeChartOfAccounts(source, file, manifest)
		if err == nil && value.Name != folder {
			err = fmt.Errorf("object %s lies in folder %s", value.Name, folder)
		}
		if err == nil {
			catalog.ChartsOfAccounts = append(catalog.ChartsOfAccounts, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, ChartOfCalculationTypesKind, func(source string, file *os.File, folder string) error {
		value, err := DecodeChartOfCalculationTypes(source, file, manifest)
		if err == nil && value.Name != folder {
			err = fmt.Errorf("object %s lies in folder %s", value.Name, folder)
		}
		if err == nil {
			catalog.ChartsOfCalculationTypes = append(catalog.ChartsOfCalculationTypes, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, TaskKind, func(source string, file *os.File, folder string) error {
		value, err := DecodeTask(source, file, manifest)
		if err == nil && value.Name != folder {
			err = fmt.Errorf("object %s lies in folder %s", value.Name, folder)
		}
		if err == nil {
			catalog.Tasks = append(catalog.Tasks, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, AccountingRegisterKind, func(source string, file *os.File, folder string) error {
		value, err := DecodeAccountingRegister(source, file, manifest)
		if err == nil && value.Name != folder {
			err = fmt.Errorf("object %s lies in folder %s", value.Name, folder)
		}
		if err == nil {
			catalog.AccountingRegisters = append(catalog.AccountingRegisters, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, CalculationRegisterKind, func(source string, file *os.File, folder string) error {
		value, err := DecodeCalculationRegister(source, file, manifest)
		if err == nil && value.Name != folder {
			err = fmt.Errorf("object %s lies in folder %s", value.Name, folder)
		}
		if err == nil {
			catalog.CalculationRegisters = append(catalog.CalculationRegisters, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, ReportKind, func(source string, file *os.File, folder string) error {
		value, err := DecodeReport(source, file, manifest)
		if err == nil && value.Name != folder {
			err = fmt.Errorf("object %s lies in folder %s", value.Name, folder)
		}
		if err == nil {
			catalog.Reports = append(catalog.Reports, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, DataProcessorKind, func(source string, file *os.File, folder string) error {
		value, err := DecodeDataProcessor(source, file, manifest)
		if err == nil && value.Name != folder {
			err = fmt.Errorf("object %s lies in folder %s", value.Name, folder)
		}
		if err == nil {
			catalog.DataProcessors = append(catalog.DataProcessors, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadKind(root, NumeratorKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeNumerator(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match directory UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.Numerators = append(catalog.Numerators, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadKind(root, SequenceKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeSequence(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match directory UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.Sequences = append(catalog.Sequences, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, DocumentJournalKind, func(source string, file *os.File, folder string) error {
		value, err := DecodeDocumentJournal(source, file, manifest)
		if err == nil && value.Name != folder {
			err = fmt.Errorf("object %s lies in folder %s", value.Name, folder)
		}
		if err == nil {
			catalog.DocumentJournals = append(catalog.DocumentJournals, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, ExchangePlanKind, func(source string, file *os.File, folder string) error {
		value, err := DecodeExchangePlan(source, file, manifest)
		if err == nil && value.Name != folder {
			err = fmt.Errorf("object %s lies in folder %s", value.Name, folder)
		}
		if err == nil {
			catalog.ExchangePlans = append(catalog.ExchangePlans, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, BusinessProcessKind, func(source string, file *os.File, folder string) error {
		value, err := DecodeBusinessProcess(source, file, manifest)
		if err == nil && value.Name != folder {
			err = fmt.Errorf("object %s lies in folder %s", value.Name, folder)
		}
		if err == nil {
			catalog.BusinessProcesses = append(catalog.BusinessProcesses, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, DocumentKind, func(source string, file *os.File, folder string) error {
		value, err := DecodeDocument(source, file, manifest)
		if err == nil && value.Name != folder {
			err = fmt.Errorf("object %s lies in folder %s", value.Name, folder)
		}
		if err == nil {
			catalog.Documents = append(catalog.Documents, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, InformationRegisterKind, func(source string, file *os.File, folder string) error {
		value, err := DecodeInformationRegister(source, file, manifest)
		if err == nil && value.Name != folder {
			err = fmt.Errorf("object %s lies in folder %s", value.Name, folder)
		}
		if err == nil {
			catalog.InformationRegisters = append(catalog.InformationRegisters, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadObjectKind(root, AccumulationRegisterKind, func(source string, file *os.File, folder string) error {
		value, err := DecodeAccumulationRegister(source, file, manifest)
		if err == nil && value.Name != folder {
			err = fmt.Errorf("object %s lies in folder %s", value.Name, folder)
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
func loadObjectKind(root string, kind Kind, decode func(string, *os.File, string) error) error {
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
		if err := project.ObjectName(entry.Name()); err != nil {
			return fmt.Errorf("metadata object folder %q: %w", entry.Name(), err)
		}
		relative := filepath.ToSlash(filepath.Join("metadata", string(kind), entry.Name(), "object.yaml"))
		file, err := os.Open(filepath.Join(directory, entry.Name(), "object.yaml"))
		if err != nil {
			return fmt.Errorf("open %s: %w", relative, err)
		}
		decodeErr := decode(relative, file, entry.Name())
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
	for index, item := range catalog.FunctionalOptions {
		if err := add("functional option", item.ID, item.Name, index, catalog.functionalOptionByName, catalog.functionalOptionByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.ScheduledJobs {
		if err := add("scheduled job", item.ID, item.Name, index, catalog.scheduledJobByName, catalog.scheduledJobByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.SettingsStorages {
		if err := add("settings storage", item.ID, item.Name, index, catalog.settingsStorageByName, catalog.settingsStorageByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.FilterCriteria {
		if err := add("filter criterion", item.ID, item.Name, index, catalog.filterCriterionByName, catalog.filterCriterionByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.FunctionalOptionParameters {
		if err := add("functional option parameter", item.ID, item.Name, index,
			catalog.functionalOptionParameterByName, catalog.functionalOptionParameterByID); err != nil {
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
	for index, item := range catalog.Tasks {
		if err := add("task", item.ID, item.Name, index, catalog.taskByName, catalog.taskByID); err != nil {
			return err
		}
		for _, attribute := range append(slices.Clone(item.Attributes), addressingAsAttributes(item)...) {
			if _, common := catalog.commonAttributeByID[attribute.ID]; common {
				continue
			}
			if previous, ok := allIDs[attribute.ID]; ok {
				return fmt.Errorf("%w: %s and task attribute %s.%s use %s", ErrDuplicateID, previous, item.Name, attribute.Name, attribute.ID)
			}
			allIDs[attribute.ID] = "task attribute " + item.Name + "." + attribute.Name
		}
		for _, part := range item.TableParts {
			if previous, ok := allIDs[part.ID]; ok {
				return fmt.Errorf("%w: %s and task table part %s.%s use %s", ErrDuplicateID, previous, item.Name, part.Name, part.ID)
			}
			allIDs[part.ID] = "task table part " + item.Name + "." + part.Name
		}
	}
	for index, item := range catalog.AccountingRegisters {
		if err := add("accounting register", item.ID, item.Name, index, catalog.accountingRegisterByName, catalog.accountingRegisterByID); err != nil {
			return err
		}
		for _, field := range append(slices.Clone(item.Dimensions), item.Resources...) {
			if previous, ok := allIDs[field.ID]; ok {
				return fmt.Errorf("%w: %s and accounting register field %s.%s use %s", ErrDuplicateID, previous, item.Name, field.Name, field.ID)
			}
			allIDs[field.ID] = "accounting register field " + item.Name + "." + field.Name
		}
		for _, attribute := range item.Attributes {
			if _, common := catalog.commonAttributeByID[attribute.ID]; common {
				continue
			}
			if previous, ok := allIDs[attribute.ID]; ok {
				return fmt.Errorf("%w: %s and accounting register attribute %s.%s use %s", ErrDuplicateID, previous, item.Name, attribute.Name, attribute.ID)
			}
			allIDs[attribute.ID] = "accounting register attribute " + item.Name + "." + attribute.Name
		}
	}
	for index, item := range catalog.CalculationRegisters {
		if err := add("calculation register", item.ID, item.Name, index, catalog.calculationRegisterByName, catalog.calculationRegisterByID); err != nil {
			return err
		}
		for _, field := range append(calculationRegisterFields(item), item.Attributes...) {
			if _, common := catalog.commonAttributeByID[field.ID]; common {
				continue
			}
			if previous, ok := allIDs[field.ID]; ok {
				return fmt.Errorf("%w: %s and calculation register field %s.%s use %s", ErrDuplicateID, previous, item.Name, field.Name, field.ID)
			}
			allIDs[field.ID] = "calculation register field " + item.Name + "." + field.Name
		}
		for _, recalculation := range item.Recalculations {
			if previous, ok := allIDs[recalculation.ID]; ok {
				return fmt.Errorf("%w: %s and recalculation %s.%s use %s", ErrDuplicateID, previous, item.Name, recalculation.Name, recalculation.ID)
			}
			allIDs[recalculation.ID] = "recalculation " + item.Name + "." + recalculation.Name
			for _, dimension := range recalculation.Dimensions {
				if previous, ok := allIDs[dimension.ID]; ok {
					return fmt.Errorf("%w: %s and recalculation dimension %s.%s.%s use %s",
						ErrDuplicateID, previous, item.Name, recalculation.Name, dimension.Name, dimension.ID)
				}
				allIDs[dimension.ID] = "recalculation dimension " + item.Name + "." + recalculation.Name + "." + dimension.Name
			}
		}
	}
	for _, running := range []struct {
		kind    string
		names   map[string]int
		ids     map[uuid.UUID]int
		objects []runningObject
	}{
		{"report", catalog.reportByName, catalog.reportByID, reportsAsRunning(catalog.Reports)},
		{"data processor", catalog.dataProcessorByName, catalog.dataProcessorByID, dataProcessorsAsRunning(catalog.DataProcessors)},
	} {
		for index, item := range running.objects {
			if err := add(running.kind, item.id, item.name, index, running.names, running.ids); err != nil {
				return err
			}
			for _, attribute := range item.attributes {
				if previous, ok := allIDs[attribute.ID]; ok {
					return fmt.Errorf("%w: %s and %s attribute %s.%s use %s", ErrDuplicateID, previous, running.kind, item.name, attribute.Name, attribute.ID)
				}
				allIDs[attribute.ID] = running.kind + " attribute " + item.name + "." + attribute.Name
			}
			for _, part := range item.parts {
				if previous, ok := allIDs[part.ID]; ok {
					return fmt.Errorf("%w: %s and %s table part %s.%s use %s", ErrDuplicateID, previous, running.kind, item.name, part.Name, part.ID)
				}
				allIDs[part.ID] = running.kind + " table part " + item.name + "." + part.Name
			}
		}
	}
	for index, item := range catalog.Numerators {
		if err := add("numerator", item.ID, item.Name, index, catalog.numeratorByName, catalog.numeratorByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.Sequences {
		if err := add("sequence", item.ID, item.Name, index, catalog.sequenceByName, catalog.sequenceByID); err != nil {
			return err
		}
		for _, dimension := range item.Dimensions {
			if previous, ok := allIDs[dimension.ID]; ok {
				return fmt.Errorf("%w: %s and sequence dimension %s.%s use %s", ErrDuplicateID, previous, item.Name, dimension.Name, dimension.ID)
			}
			allIDs[dimension.ID] = "sequence dimension " + item.Name + "." + dimension.Name
		}
	}
	for index, item := range catalog.DocumentJournals {
		if err := add("document journal", item.ID, item.Name, index, catalog.documentJournalByName, catalog.documentJournalByID); err != nil {
			return err
		}
		for _, column := range item.Columns {
			if previous, ok := allIDs[column.ID]; ok {
				return fmt.Errorf("%w: %s and journal column %s.%s use %s", ErrDuplicateID, previous, item.Name, column.Name, column.ID)
			}
			allIDs[column.ID] = "journal column " + item.Name + "." + column.Name
		}
	}
	for index, item := range catalog.ExchangePlans {
		if err := add("exchange plan", item.ID, item.Name, index, catalog.exchangePlanByName, catalog.exchangePlanByID); err != nil {
			return err
		}
		for _, attribute := range item.Attributes {
			if _, common := catalog.commonAttributeByID[attribute.ID]; common {
				continue
			}
			if previous, ok := allIDs[attribute.ID]; ok {
				return fmt.Errorf("%w: %s and exchange plan attribute %s.%s use %s", ErrDuplicateID, previous, item.Name, attribute.Name, attribute.ID)
			}
			allIDs[attribute.ID] = "exchange plan attribute " + item.Name + "." + attribute.Name
		}
		for _, part := range item.TableParts {
			if previous, ok := allIDs[part.ID]; ok {
				return fmt.Errorf("%w: %s and exchange plan table part %s.%s use %s", ErrDuplicateID, previous, item.Name, part.Name, part.ID)
			}
			allIDs[part.ID] = "exchange plan table part " + item.Name + "." + part.Name
		}
	}
	for index, item := range catalog.BusinessProcesses {
		if err := add("business process", item.ID, item.Name, index, catalog.businessProcessByName, catalog.businessProcessByID); err != nil {
			return err
		}
		for _, point := range item.Route.Points {
			if previous, ok := allIDs[point.ID]; ok {
				return fmt.Errorf("%w: %s and route point %s.%s use %s", ErrDuplicateID, previous, item.Name, point.Name, point.ID)
			}
			allIDs[point.ID] = "route point " + item.Name + "." + point.Name
		}
		for _, attribute := range item.Attributes {
			if _, common := catalog.commonAttributeByID[attribute.ID]; common {
				continue
			}
			if previous, ok := allIDs[attribute.ID]; ok {
				return fmt.Errorf("%w: %s and business process attribute %s.%s use %s", ErrDuplicateID, previous, item.Name, attribute.Name, attribute.ID)
			}
			allIDs[attribute.ID] = "business process attribute " + item.Name + "." + attribute.Name
		}
		for _, part := range item.TableParts {
			if previous, ok := allIDs[part.ID]; ok {
				return fmt.Errorf("%w: %s and business process table part %s.%s use %s", ErrDuplicateID, previous, item.Name, part.Name, part.ID)
			}
			allIDs[part.ID] = "business process table part " + item.Name + "." + part.Name
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
		if err := validateObjectFileSources(objectFiles{root: root, directoryKind: CatalogKind, kind: "catalog", name: item.Name, modules: objectKindModules, forms: item.Forms, commands: item.Commands, templates: item.Templates}); err != nil {
			return err
		}
	}
	for _, item := range catalog.SettingsStorages {
		if err := validateObjectFileSources(objectFiles{root: root, directoryKind: SettingsStorageKind,
			kind: "settings storage", name: item.Name, modules: managerKindModules,
			extraForms: []namedSource{
				{"save form", item.Forms.Save}, {"load form", item.Forms.Load},
				{"auxiliary save form", item.Forms.AuxiliarySave}, {"auxiliary load form", item.Forms.AuxiliaryLoad},
			}}); err != nil {
			return err
		}
	}
	for _, item := range catalog.FilterCriteria {
		if err := validateObjectFileSources(objectFiles{root: root, directoryKind: FilterCriterionKind,
			kind: "filter criterion", name: item.Name, modules: managerKindModules,
			extraForms: []namedSource{{"list form", item.Forms.List}, {"auxiliary form", item.Forms.Auxiliary}},
			commands:   item.Commands}); err != nil {
			return err
		}
	}
	for _, item := range catalog.Enumerations {
		if err := validateObjectFileSources(objectFiles{root: root, directoryKind: EnumerationKind,
			kind: "enumeration", name: item.Name, modules: managerKindModules,
			extraForms: []namedSource{
				{"list form", item.Forms.List}, {"choice form", item.Forms.Choice},
				{"auxiliary list form", item.Forms.AuxiliaryList},
				{"auxiliary choice form", item.Forms.AuxiliaryChoice},
			},
			commands: item.Commands, templates: item.Templates}); err != nil {
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
		if err := validateObjectFileSources(objectFiles{root: root, directoryKind: ChartOfCharacteristicTypesKind, kind: "chart of characteristic types", name: item.Name, modules: objectKindModules, forms: item.Forms, commands: item.Commands, templates: item.Templates}); err != nil {
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
		if err := validateObjectFileSources(objectFiles{root: root, directoryKind: ChartOfAccountsKind, kind: "chart of accounts", name: item.Name, modules: objectKindModules, forms: item.Forms, commands: item.Commands, templates: item.Templates}); err != nil {
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
		if err := validateObjectFileSources(objectFiles{root: root, directoryKind: ChartOfCalculationTypesKind, kind: "chart of calculation types", name: item.Name, modules: objectKindModules, forms: item.Forms, commands: item.Commands, templates: item.Templates}); err != nil {
			return err
		}
	}
	for _, item := range catalog.Tasks {
		owner := "task " + item.Name
		if err := catalog.validateTaskAddressing(owner, item); err != nil {
			return err
		}
		for _, attribute := range append(slices.Clone(item.Attributes), addressingAsAttributes(item)...) {
			if err := catalog.validateReferences(owner+" attribute "+attribute.Name, attribute.Types); err != nil {
				return err
			}
		}
		if err := validateObjectFileSources(objectFiles{root: root, directoryKind: TaskKind, kind: "task", name: item.Name, modules: objectKindModules, forms: item.Forms, commands: item.Commands, templates: item.Templates}); err != nil {
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
		// so that everything downstream reads one number and not two.
		catalog.Documents[index].Number = catalog.Numerators[numerator].Number
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
		if err := validateObjectFileSources(objectFiles{root: root, directoryKind: DocumentJournalKind, kind: "document journal", name: item.Name, modules: managerKindModules, forms: item.Forms, commands: item.Commands, templates: item.Templates}); err != nil {
			return err
		}
	}
	for _, item := range catalog.ExchangePlans {
		owner := "exchange plan " + item.Name
		if err := catalog.validateExchangePlanRegistration(owner, item); err != nil {
			return err
		}
		for _, attribute := range item.Attributes {
			if err := catalog.validateReferences(owner+" attribute "+attribute.Name, attribute.Types); err != nil {
				return err
			}
		}
		if err := validateObjectFileSources(objectFiles{root: root, directoryKind: ExchangePlanKind, kind: "exchange plan", name: item.Name, modules: objectKindModules, forms: item.Forms, commands: item.Commands, templates: item.Templates}); err != nil {
			return err
		}
	}
	for _, item := range catalog.BusinessProcesses {
		owner := "business process " + item.Name
		if err := catalog.validateBusinessProcessRoute(owner, item); err != nil {
			return err
		}
		for _, attribute := range item.Attributes {
			if err := catalog.validateReferences(owner+" attribute "+attribute.Name, attribute.Types); err != nil {
				return err
			}
		}
		if err := validateObjectFileSources(objectFiles{root: root, directoryKind: BusinessProcessKind, kind: "business process", name: item.Name, modules: objectKindModules, forms: item.Forms, commands: item.Commands, templates: item.Templates}); err != nil {
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
		if err := validateObjectFileSources(objectFiles{root: root, directoryKind: DocumentKind, kind: "document", name: item.Name, modules: objectKindModules, forms: item.Forms, commands: item.Commands, templates: item.Templates}); err != nil {
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
		if err := validateObjectFileSources(objectFiles{root: root, directoryKind: InformationRegisterKind, kind: "information register", name: item.Name, modules: recordSetKindModules, forms: item.Forms, commands: item.Commands, templates: item.Templates}); err != nil {
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
		if err := validateObjectFileSources(objectFiles{root: root, directoryKind: AccumulationRegisterKind, kind: "accumulation register", name: item.Name, modules: recordSetKindModules, forms: item.Forms, commands: item.Commands, templates: item.Templates}); err != nil {
			return err
		}
	}
	for _, item := range catalog.AccountingRegisters {
		if err := catalog.validateAccountingRegister(root, item); err != nil {
			return err
		}
	}
	for _, item := range catalog.CalculationRegisters {
		if err := catalog.validateCalculationRegister(root, item); err != nil {
			return err
		}
	}
	for _, item := range catalog.Reports {
		owner := "report " + item.Name
		for _, attribute := range append(slices.Clone(item.Attributes), tablePartAttributes(item.TableParts)...) {
			if err := catalog.validateReferences(owner+" attribute "+attribute.Name, attribute.Types); err != nil {
				return err
			}
		}
		if err := validateObjectFileSources(objectFiles{root: root, directoryKind: ReportKind, kind: "report", name: item.Name, modules: objectKindModules, forms: ObjectForms{}, commands: item.Commands, templates: item.Templates}); err != nil {
			return err
		}
	}
	for _, item := range catalog.DataProcessors {
		owner := "data processor " + item.Name
		for _, attribute := range append(slices.Clone(item.Attributes), tablePartAttributes(item.TableParts)...) {
			if err := catalog.validateReferences(owner+" attribute "+attribute.Name, attribute.Types); err != nil {
				return err
			}
		}
		if err := validateObjectFileSources(objectFiles{root: root, directoryKind: DataProcessorKind, kind: "data processor", name: item.Name, modules: objectKindModules, forms: item.Forms, commands: item.Commands, templates: item.Templates}); err != nil {
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
	if err := catalog.validateFunctionalOptions(); err != nil {
		return err
	}
	if err := catalog.validateFunctionalOptionParameters(); err != nil {
		return err
	}
	if err := catalog.validateFilterCriteria(); err != nil {
		return err
	}
	if err := catalog.validateSettingsStorageReferences(); err != nil {
		return err
	}
	if err := catalog.validateScheduledJobReferences(); err != nil {
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
		allowed, err := catalog.expandTypes(byID[*attribute.Dimension], nil)
		if err != nil {
			return err
		}
		held := map[string]bool{}
		for _, one := range allowed {
			held[typeKey(one)] = true
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
func (catalog *Catalog) validateAccountingRegister(root string, item AccountingRegisterDefinition) error {
	owner := "accounting register " + item.Name
	index, ok := catalog.chartOfAccountsByID[item.ChartOfAccounts]
	if !ok {
		return fmt.Errorf("%s makes entries against unknown chart of accounts %s", owner, item.ChartOfAccounts)
	}
	chart := catalog.ChartsOfAccounts[index]
	flags, extFlags := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	for _, flag := range chart.AccountingFlags {
		flags[flag.ID] = true
	}
	for _, flag := range chart.ExtDimensionAccountingFlags {
		extFlags[flag.ID] = true
	}
	for _, field := range append(slices.Clone(item.Dimensions), item.Resources...) {
		if err := catalog.validateReferences(owner+" field "+field.Name, field.Types); err != nil {
			return err
		}
		if field.AccountingFlag != nil && !flags[*field.AccountingFlag] {
			return fmt.Errorf("%s field %s depends on %s, which is not an accounting flag of %s",
				owner, field.Name, field.AccountingFlag, chart.Name)
		}
		if field.ExtDimensionAccountingFlag != nil && !extFlags[*field.ExtDimensionAccountingFlag] {
			return fmt.Errorf("%s field %s depends on %s, which is not an ext dimension accounting flag of %s",
				owner, field.Name, field.ExtDimensionAccountingFlag, chart.Name)
		}
	}
	for _, attribute := range item.Attributes {
		if err := catalog.validateReferences(owner+" attribute "+attribute.Name, attribute.Types); err != nil {
			return err
		}
	}
	for _, recorder := range item.Recorders {
		if _, ok := catalog.documentByID[recorder]; !ok {
			return fmt.Errorf("%s references unknown recorder document %s", owner, recorder)
		}
	}
	return validateObjectFileSources(objectFiles{root: root, directoryKind: AccountingRegisterKind, kind: "accounting register", name: item.Name, modules: recordSetKindModules, forms: item.Forms, commands: item.Commands, templates: item.Templates})
}

// takesABase says whether a chart gathers a base at all. An unset dependency
// and one set to "none" are the same answer, and treating them differently
// would let a base period through on half the charts that have none.
func takesABase(chart ChartOfCalculationTypesDefinition) bool {
	return chart.BaseDependency != "" && chart.BaseDependency != NoBaseDependency
}

// tablePartAttributes is every attribute inside the parts, flattened.
func tablePartAttributes(parts []TablePart) []Attribute {
	var result []Attribute
	for _, part := range parts {
		result = append(result, part.Attributes...)
	}
	return result
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
	}
	dimensions := map[uuid.UUID]bool{}
	for _, dimension := range item.Dimensions {
		dimensions[dimension.ID] = true
		if err := catalog.validateReferences(owner+" dimension "+dimension.Name, dimension.Types); err != nil {
			return err
		}
		if dimension.Base && !takesABase(chart) {
			return fmt.Errorf("%s dimension %s is a base dimension, but %s takes no base", owner, dimension.Name, chart.Name)
		}
		if dimension.ScheduleLink != nil && !scheduleDimensions[*dimension.ScheduleLink] {
			return fmt.Errorf("%s dimension %s is linked to %s, which is not a dimension of the schedule", owner, dimension.Name, dimension.ScheduleLink)
		}
	}
	for _, field := range append(slices.Clone(item.Resources), item.Attributes...) {
		if err := catalog.validateReferences(owner+" field "+field.Name, field.Types); err != nil {
			return err
		}
	}
	for _, recorder := range item.Recorders {
		if _, ok := catalog.documentByID[recorder]; !ok {
			return fmt.Errorf("%s references unknown recorder document %s", owner, recorder)
		}
	}
	// Leading data are dimensions of calculation registers - this one or
	// another. A leading dimension that belongs to nothing sets nothing off.
	leading := map[uuid.UUID]bool{}
	for _, register := range catalog.CalculationRegisters {
		for _, dimension := range register.Dimensions {
			leading[dimension.ID] = true
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
					return fmt.Errorf("%s recalculation %s dimension %s is set off by %s, which is not a dimension of any calculation register",
						owner, recalculation.Name, dimension.Name, source)
				}
			}
		}
	}
	return validateObjectFileSources(objectFiles{root: root, directoryKind: CalculationRegisterKind, kind: "calculation register", name: item.Name, modules: recordSetKindModules, forms: item.Forms, commands: item.Commands, templates: item.Templates})
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
		found := false
		switch entry.Kind {
		case ConstantKind:
			_, found = catalog.constantByID[entry.Object]
		case CatalogKind:
			_, found = catalog.catalogByID[entry.Object]
		case DocumentKind:
			_, found = catalog.documentByID[entry.Object]
		case ChartOfCharacteristicTypesKind:
			_, found = catalog.chartOfCharacteristicTypesByID[entry.Object]
		case ChartOfAccountsKind:
			_, found = catalog.chartOfAccountsByID[entry.Object]
		case ChartOfCalculationTypesKind:
			_, found = catalog.chartOfCalculationTypesByID[entry.Object]
		case BusinessProcessKind:
			_, found = catalog.businessProcessByID[entry.Object]
		case TaskKind:
			_, found = catalog.taskByID[entry.Object]
		case InformationRegisterKind:
			_, found = catalog.informationRegisterByID[entry.Object]
		case AccumulationRegisterKind:
			_, found = catalog.accumulationRegisterByID[entry.Object]
		}
		if !found {
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
		if *point.NestedProcess == item.ID {
			return fmt.Errorf("%s point %s starts the process it belongs to", owner, point.Name)
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
		case BusinessProcessType, RoutePointType:
			if _, ok := catalog.businessProcessByID[*item.Reference]; !ok {
				return fmt.Errorf("%s references unknown business process %s", owner, item.Reference)
			}
		case TaskType:
			if _, ok := catalog.taskByID[*item.Reference]; !ok {
				return fmt.Errorf("%s references unknown task %s", owner, item.Reference)
			}
		case ExchangePlanType:
			if _, ok := catalog.exchangePlanByID[*item.Reference]; !ok {
				return fmt.Errorf("%s references unknown exchange plan %s", owner, item.Reference)
			}
		case CharacteristicSet:
			if _, ok := catalog.chartOfCharacteristicTypesByID[*item.Reference]; !ok {
				return fmt.Errorf("%s is typed by the characteristics of unknown chart %s", owner, item.Reference)
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

// objectFiles is everything one object keeps on disk beside its description:
// its own modules, its forms, the module of each of its commands and the
// content of each of its templates. They are checked together because they
// share one folder, and a file in that folder that nothing declares is as much
// a mistake as a declaration with no file behind it.
// The module roles each kind of object may keep. A reference object keeps an
// object module, a register keeps the module of a record set, and anything
// nobody writes to - a document journal, an enumeration, a criterion, a
// settings storage - keeps only a manager, because there is no object to have
// a module of.
var (
	objectKindModules    = []string{project.ObjectModuleFile, project.ManagerModuleFile}
	recordSetKindModules = []string{project.RecordSetModuleFile, project.ManagerModuleFile}
	managerKindModules   = []string{project.ManagerModuleFile}
)

type objectFiles struct {
	root          string
	directoryKind Kind
	kind, name    string
	// modules lists the module roles this kind of object may keep directly in
	// its folder. A module has no identifier and is not declared anywhere: the
	// file is the declaration, so what is checked is the other direction —
	// that a .bsl lying there plays a role this kind actually has.
	modules []string
	forms   ObjectForms
	// extraForms carries the forms of a kind whose set is not the usual three.
	// An enumeration is the case that made it necessary: it has no form of a
	// single value, and it has an auxiliary form beside each of the two it has.
	extraForms []namedSource
	commands   []ObjectCommand
	templates  []ObjectTemplate
}

// namedSource is one file an object declares, with the name it is called by in
// a message about it.
type namedSource struct {
	role string
	id   *uuid.UUID
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
func validateObjectFileSources(files objectFiles) error {
	if files.root == "" {
		return nil
	}
	root, kind, name := files.root, files.kind, files.name
	forms := files.forms
	directory := filepath.Join(root, "metadata", string(files.directoryKind), name)
	type source struct {
		role, path string
		id         *uuid.UUID
	}
	build := func(role string, sourceID *uuid.UUID) source {
		if sourceID == nil {
			return source{role: role}
		}
		return source{role: role, id: sourceID, path: filepath.Join(directory, "forms", sourceID.String()+".yaml")}
	}
	sources := []source{
		build("object form", forms.Object),
		build("list form", forms.List),
		build("choice form", forms.Choice),
	}
	for _, form := range files.extraForms {
		sources = append(sources, build(form.role, form.id))
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
	if err := validateObjectFolderEntries(directory, kind, name, files.modules); err != nil {
		return err
	}
	if err := validateObjectCommandFiles(directory, kind, name, files.commands); err != nil {
		return err
	}
	return validateObjectTemplateFiles(directory, kind, name, files.templates)
}

// validateObjectFolderEntries checks that the object's own folder holds only
// what an object may keep there: its description, a module in a role this kind
// of object has, and the folders of its forms, commands and templates.
func validateObjectFolderEntries(directory, kind, name string, modules []string) error {
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
			if !slices.Contains(project.ObjectSubordinateDirectories(), entry.Name()) {
				return fmt.Errorf("%s %s keeps a folder %q, which is not forms, commands or templates",
					kind, name, entry.Name())
			}
			continue
		}
		if entry.Name() == project.ObjectMetadataFile {
			continue
		}
		if !slices.Contains(modules, entry.Name()) {
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
	declared := make(map[string]bool, len(commands))
	for _, command := range commands {
		info, err := os.Lstat(filepath.Join(directory, "commands", command.Name, project.CommandModuleFile))
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("%s %s command %s has no module", kind, name, command.Name)
		}
		declared[strings.ToLower(command.Name)] = true
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
		if !declared[strings.ToLower(entry.Name())] {
			return fmt.Errorf("%s %s keeps a folder for command %s, which it does not declare",
				kind, name, entry.Name())
		}
		content, err := os.ReadDir(filepath.Join(directory, "commands", entry.Name()))
		if err != nil {
			return fmt.Errorf("%s %s command %s: %w", kind, name, entry.Name(), err)
		}
		for _, file := range content {
			if file.IsDir() || file.Type()&fs.ModeSymlink != 0 || file.Name() != project.CommandModuleFile {
				return fmt.Errorf("%s %s command %s holds %q, and a command keeps only its module",
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
		declared[template.ID.String()] = template.Kind
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
		templateKind, ok := declared[entry.Name()]
		if !ok {
			return fmt.Errorf("%s %s keeps content for template %s, which it does not declare",
				kind, name, entry.Name())
		}
		content, err := os.ReadDir(filepath.Join(directory, "templates", entry.Name()))
		if err != nil {
			return fmt.Errorf("%s %s template %s: %w", kind, name, entry.Name(), err)
		}
		for _, file := range content {
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

// validLanguageCode accepts the shape of a language code, not the list of
// them: which languages a configuration has is decided by the configuration,
// and languages become an object of their own later in this block.
func validLanguageCode(code string) bool {
	if len(code) < 1 || len(code) > 8 {
		return false
	}
	for _, symbol := range code {
		switch {
		case symbol >= 'a' && symbol <= 'z', symbol >= '0' && symbol <= '9', symbol == '-':
		default:
			return false
		}
	}
	return true
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
