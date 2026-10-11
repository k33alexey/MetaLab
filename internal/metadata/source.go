package metadata

import (
	"fmt"
	"io"
	"path"
	"reflect"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// sourceKind is how the descriptions of one kind are read: the decoder, and
// where a decoded description goes in the catalog. The loader and Studio's
// save both read a description through it (DecodeSource), so what one accepts
// the other does too.
type sourceKind struct {
	kind Kind
	// described is the type the decoder returns; a test requires an ID and a
	// Name of it, which is what the description is checked against.
	described reflect.Type
	decode    func(source string, reader, rights io.Reader, configuration project.Project) (any, error)
	// add puts a decoded description into the catalog. A kind read only inside
	// another object - a table or a cube of an external data source - has
	// none: its owner collects it.
	add func(catalog *Catalog, root string, value any) error
}

// sourceKinds lists every kind the model reads from the project, in the order
// the load reads them. It is filled in init: an external data source's add
// reads its tables through DecodeSource, which looks the kinds up here.
var sourceKinds []sourceKind

func init() {
	sourceKinds = []sourceKind{
		{kind: RoleKind, described: reflect.TypeFor[RoleDefinition](),
			decode: func(source string, reader, rights io.Reader, configuration project.Project) (any, error) {
				return DecodeRole(source, reader, rights, configuration)
			},
			add: collect(func(catalog *Catalog) *[]RoleDefinition { return &catalog.Roles })},
		describedAs(SubsystemKind, DecodeSubsystem, func(catalog *Catalog) *[]SubsystemDefinition { return &catalog.Subsystems }),
		describedAs(ConstantKind, DecodeConstant, func(catalog *Catalog) *[]Constant { return &catalog.Constants }),
		describedAs(SessionParameterKind, DecodeSessionParameter, func(catalog *Catalog) *[]SessionParameter { return &catalog.SessionParameters }),
		describedAs(CommonAttributeKind, DecodeCommonAttribute, func(catalog *Catalog) *[]CommonAttributeDefinition { return &catalog.CommonAttributes }),
		describedAs(CommonModuleKind, DecodeCommonModule, func(catalog *Catalog) *[]CommonModuleDefinition { return &catalog.CommonModules }),
		describedAs(EventSubscriptionKind, DecodeEventSubscription, func(catalog *Catalog) *[]EventSubscriptionDefinition { return &catalog.EventSubscriptions }),
		describedAs(EnumerationKind, DecodeEnumeration, func(catalog *Catalog) *[]Enumeration { return &catalog.Enumerations }),
		describedAs(ScheduledJobKind, DecodeScheduledJob, func(catalog *Catalog) *[]ScheduledJobDefinition { return &catalog.ScheduledJobs }),
		describedAs(CommandGroupKind, DecodeCommandGroup, func(catalog *Catalog) *[]CommandGroupDefinition { return &catalog.CommandGroups }),
		describedAs(CommonCommandKind, DecodeCommonCommand, func(catalog *Catalog) *[]CommonCommandDefinition { return &catalog.CommonCommands }),
		describedAs(CommonTemplateKind, DecodeCommonTemplate, func(catalog *Catalog) *[]CommonTemplateDefinition { return &catalog.CommonTemplates }),
		describedAs(XDTOPackageKind, DecodeXDTOPackage, func(catalog *Catalog) *[]XDTOPackageDefinition { return &catalog.XDTOPackages }),
		describedAs(WebServiceKind, DecodeWebService, func(catalog *Catalog) *[]WebServiceDefinition { return &catalog.WebServices }),
		describedAs(WSReferenceKind, DecodeWSReference, func(catalog *Catalog) *[]WSReferenceDefinition { return &catalog.WSReferences }),
		describedAs(WebSocketClientKind, DecodeWebSocketClient, func(catalog *Catalog) *[]WebSocketClientDefinition { return &catalog.WebSocketClients }),
		{kind: ExternalDataSourceKind, described: reflect.TypeFor[ExternalDataSourceDefinition](),
			decode: decodeWith(DecodeExternalDataSource), add: addExternalDataSource},
		describedAs(ExternalDataSourceTableKind, DecodeExternalTable, nil),
		describedAs(ExternalCubeKind, DecodeExternalCube, nil),
		describedAs(ExternalDimensionTableKind, DecodeExternalDimensionTable, nil),
		describedAs(HTTPServiceKind, DecodeHTTPService, func(catalog *Catalog) *[]HTTPServiceDefinition { return &catalog.HTTPServices }),
		describedAs(CommonPictureKind, DecodeCommonPicture, func(catalog *Catalog) *[]CommonPictureDefinition { return &catalog.CommonPictures }),
		describedAs(StyleItemKind, DecodeStyleItem, func(catalog *Catalog) *[]StyleItemDefinition { return &catalog.StyleItems }),
		describedAs(StyleKind, DecodeStyle, func(catalog *Catalog) *[]StyleDefinition { return &catalog.Styles }),
		describedAs(FunctionalOptionKind, DecodeFunctionalOption, func(catalog *Catalog) *[]FunctionalOptionDefinition { return &catalog.FunctionalOptions }),
		describedAs(FunctionalOptionParameterKind, DecodeFunctionalOptionParameter, func(catalog *Catalog) *[]FunctionalOptionParameterDefinition {
			return &catalog.FunctionalOptionParameters
		}),
		describedAs(SettingsStorageKind, DecodeSettingsStorage, func(catalog *Catalog) *[]SettingsStorageDefinition { return &catalog.SettingsStorages }),
		describedAs(FilterCriterionKind, DecodeFilterCriterion, func(catalog *Catalog) *[]FilterCriterionDefinition { return &catalog.FilterCriteria }),
		describedAs(DefinedTypeKind, DecodeDefinedType, func(catalog *Catalog) *[]DefinedTypeObject { return &catalog.DefinedTypes }),
		describedAs(CatalogKind, DecodeCatalog, func(catalog *Catalog) *[]CatalogDefinition { return &catalog.Catalogs }),
		describedAs(ChartOfCharacteristicTypesKind, DecodeChartOfCharacteristicTypes, func(catalog *Catalog) *[]ChartOfCharacteristicTypesDefinition {
			return &catalog.ChartsOfCharacteristicTypes
		}),
		describedAs(ChartOfAccountsKind, DecodeChartOfAccounts, func(catalog *Catalog) *[]ChartOfAccountsDefinition { return &catalog.ChartsOfAccounts }),
		describedAs(ChartOfCalculationTypesKind, DecodeChartOfCalculationTypes, func(catalog *Catalog) *[]ChartOfCalculationTypesDefinition {
			return &catalog.ChartsOfCalculationTypes
		}),
		describedAs(TaskKind, DecodeTask, func(catalog *Catalog) *[]TaskDefinition { return &catalog.Tasks }),
		describedAs(AccountingRegisterKind, DecodeAccountingRegister, func(catalog *Catalog) *[]AccountingRegisterDefinition { return &catalog.AccountingRegisters }),
		describedAs(CalculationRegisterKind, DecodeCalculationRegister, func(catalog *Catalog) *[]CalculationRegisterDefinition { return &catalog.CalculationRegisters }),
		describedAs(ReportKind, DecodeReport, func(catalog *Catalog) *[]ReportDefinition { return &catalog.Reports }),
		describedAs(DataProcessorKind, DecodeDataProcessor, func(catalog *Catalog) *[]DataProcessorDefinition { return &catalog.DataProcessors }),
		describedAs(NumeratorKind, DecodeNumerator, func(catalog *Catalog) *[]NumeratorDefinition { return &catalog.Numerators }),
		describedAs(SequenceKind, DecodeSequence, func(catalog *Catalog) *[]SequenceDefinition { return &catalog.Sequences }),
		describedAs(DocumentJournalKind, DecodeDocumentJournal, func(catalog *Catalog) *[]DocumentJournalDefinition { return &catalog.DocumentJournals }),
		describedAs(ExchangePlanKind, DecodeExchangePlan, func(catalog *Catalog) *[]ExchangePlanDefinition { return &catalog.ExchangePlans }),
		describedAs(BusinessProcessKind, DecodeBusinessProcess, func(catalog *Catalog) *[]BusinessProcessDefinition { return &catalog.BusinessProcesses }),
		describedAs(DocumentKind, DecodeDocument, func(catalog *Catalog) *[]DocumentDefinition { return &catalog.Documents }),
		describedAs(InformationRegisterKind, DecodeInformationRegister, func(catalog *Catalog) *[]InformationRegisterDefinition { return &catalog.InformationRegisters }),
		describedAs(AccumulationRegisterKind, DecodeAccumulationRegister, func(catalog *Catalog) *[]AccumulationRegisterDefinition { return &catalog.AccumulationRegisters }),
	}
}

// describedAs is the reading of a kind whose description is one file and goes
// into one list of the catalog; list is nil for a kind its owner collects.
func describedAs[T any](kind Kind, decode func(string, io.Reader, project.Project) (T, error), list func(*Catalog) *[]T) sourceKind {
	item := sourceKind{kind: kind, described: reflect.TypeFor[T](), decode: decodeWith(decode)}
	if list != nil {
		item.add = collect(list)
	}
	return item
}

func decodeWith[T any](decode func(string, io.Reader, project.Project) (T, error)) func(string, io.Reader, io.Reader, project.Project) (any, error) {
	return func(source string, reader, _ io.Reader, configuration project.Project) (any, error) {
		return decode(source, reader, configuration)
	}
}

func collect[T any](list func(*Catalog) *[]T) func(*Catalog, string, any) error {
	return func(catalog *Catalog, _ string, value any) error {
		items := list(catalog)
		*items = append(*items, value.(T))
		return nil
	}
}

// sourceKindOf finds how a kind is read.
func sourceKindOf(kind Kind) (sourceKind, bool) {
	index := slices.IndexFunc(sourceKinds, func(item sourceKind) bool { return item.kind == kind })
	if index < 0 {
		return sourceKind{}, false
	}
	return sourceKinds[index], true
}

// keptInFolder reports whether an object of a kind keeps a folder named after
// it rather than one file named by its identifier.
func keptInFolder(kind Kind) bool {
	_, named := project.NamedFolderFiles(string(kind))
	return named || project.IsObjectFolderKind(string(kind))
}

// DecodeSource reads one description of the project - of an object, of an
// object inside another, of a form - the way the load reads it: by the kind
// its path names, and checked against where it lies. A flat object is found
// by the identifier its file is named by; an object that keeps a folder, and a
// form, by the name of that folder. An object that disagreed would be read as
// one thing and found as another, so that is what each has to agree with.
//
// rights is the grid of a role's rights, nil when the role keeps none; no
// other kind has a second file. described is false for a path that is not a
// description the model reads in this way - the configuration, a schema, a
// file of content - and then nothing is read.
func DecodeSource(relative string, content, rights io.Reader, configuration project.Project) (value any, described bool, err error) {
	if _, ok := project.FormFolder(relative); ok {
		form, err := DecodeManagedForm(relative, content, configuration)
		if err == nil {
			err = FormAgreesWithPath(form, relative)
		}
		if err != nil {
			return nil, true, err
		}
		return form, true, nil
	}
	item, check, ok := sourceAt(relative)
	if !ok {
		return nil, false, nil
	}
	if check == nil {
		return nil, true, fmt.Errorf("unexpected metadata source %q: an object of this kind keeps a folder named after it", relative)
	}
	value, err = item.decode(relative, content, rights, configuration)
	if err == nil {
		err = check(sourceIdentity(value))
	}
	if err != nil {
		return nil, true, err
	}
	return value, true, nil
}

// sourceAt reads a path as the description of an object of a kind, and says
// what the description has to agree with. check is nil for a file named by an
// identifier where the kind keeps a folder.
func sourceAt(relative string) (sourceKind, func(uuid.UUID, string) error, bool) {
	if kind, name, rest, ok := project.SplitObjectPath(relative); ok {
		item, known := sourceKindOf(Kind(kind))
		if !known || len(rest) != 1 || rest[0] != project.ObjectMetadataFile {
			return sourceKind{}, nil, false
		}
		// The folder is named after the object's own name, which for an
		// object inside another is what follows the owner's.
		folder := name[strings.LastIndexByte(name, '.')+1:]
		return item, func(_ uuid.UUID, name string) error { return agreesWithFolder("object", name, folder) }, true
	}
	parts := strings.Split(relative, "/")
	if len(parts) != 3 || parts[0] != "metadata" || path.Ext(parts[2]) != ".yaml" {
		return sourceKind{}, nil, false
	}
	item, known := sourceKindOf(Kind(parts[1]))
	if !known {
		return sourceKind{}, nil, false
	}
	if keptInFolder(item.kind) {
		return item, nil, true
	}
	return item, func(id uuid.UUID, _ string) error {
		filename, err := uuid.Parse(strings.TrimSuffix(parts[2], ".yaml"))
		if err != nil {
			return fmt.Errorf("metadata source %q must use a UUID filename: %w", parts[2], err)
		}
		if id != filename {
			return fmt.Errorf("metadata UUID %s does not match filename UUID %s", id, filename)
		}
		return nil
	}, true
}

// FormAgreesWithPath checks a form against where it lies: an object's form
// and a common form alike lie in a folder named after the form, so the name is
// what has to agree. The identifier inside is free - it is what roles, the
// portal and ML App refer to the form by, not what finds the file.
func FormAgreesWithPath(form ManagedForm, relative string) error {
	folder, ok := project.FormFolder(relative)
	if !ok {
		return fmt.Errorf("unexpected form source %q", relative)
	}
	return agreesWithFolder("form", form.Name, folder)
}

// agreesWithFolder checks that what lies in a folder named after it is called
// what the folder is. The folder is made from the name, so they agree exactly:
// on a file system that ignores case, a folder differing in case alone would
// hold an object whose name nobody wrote.
func agreesWithFolder(what, name, folder string) error {
	if name != folder {
		return fmt.Errorf("%s %s lies in a folder called %s", what, name, folder)
	}
	return nil
}

// sourceIdentity is what a description says it is. Every type sourceKinds
// decodes has both fields (TestEverySourceKindIsChecked).
func sourceIdentity(value any) (uuid.UUID, string) {
	item := reflect.ValueOf(value)
	return item.FieldByName("ID").Interface().(uuid.UUID), item.FieldByName("Name").String()
}
