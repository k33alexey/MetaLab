package metadata

import (
	"fmt"
	"io"
	"path"
	"reflect"
	"slices"
	"strings"
	"unicode"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// kindInfo is what the model knows of one kind beside its names (those are
// the kinds package's): how its descriptions are read and where a decoded one
// goes in the catalog, how an object of it is found by its identifier, and
// which types of a type description name an object of it. The loader and
// Studio's save both read a description through it (DecodeSource), so what
// one accepts the other does too. A recalculation is read inside its
// calculation register and has no reading of its own here, only the rest.
type kindInfo struct {
	kind Kind
	// described is the type the decoder returns; a test requires an ID and a
	// Name of it, which is what the description is checked against.
	described reflect.Type
	decode    func(source string, reader, rights io.Reader, configuration project.Project) (any, error)
	// add puts a decoded description into the catalog. A kind read only inside
	// another object - a table or a cube of an external data source - has
	// none: its owner collects it.
	add func(catalog *Catalog, root string, value any) error
	// objects is the list of the catalog the kind's objects are in, nil for a
	// kind its owner collects.
	objects func(catalog *Catalog) reflect.Value
	// has says an object of the kind with the identifier is in the catalog.
	has func(catalog *Catalog, id uuid.UUID) bool
	// references are the types of a reference to an object of the kind, in
	// the order and number the kinds package names them, each with the set
	// that holds all of them; none for a kind whose objects have no
	// reference.
	references []kindReference
	// objectTypes are the types of what an object of the kind is in memory
	// and never in a column: the object, the record set, the manager.
	objectTypes []TypeKind
	// namedBy are the other elements of a type description that name an
	// object of the kind through Type.Reference: a defined type, the
	// characteristics of a chart.
	namedBy []TypeKind
	// uses says what else may name an object of the kind (kindUsesOf).
	uses kindUses
}

// LanguageKind names a language of the configuration where something may name
// one: it is described inside the configuration, not in a folder of its own.
const LanguageKind = kinds.Language

// presence says whether one use of an object of a kind is the prototype's.
type presence int

const (
	// unsaid is a kind that says nothing of the use; a test refuses it, so a
	// new kind is not left out of a use by omission.
	unsaid presence = iota
	// absent is a use the prototype has no place for, by the help.
	absent
	// present is a use an export of the configurator writes.
	present
	// unproven is a use nothing shows the configurator either writing or
	// refusing. It is accepted, as everything without a reason to refuse it
	// is (CONFORMANCE.md, «Принято, но не исполняется»), and every place of
	// it is a note (NoteUseWithoutSample).
	unproven
)

// accepted says the model takes the use.
func (use presence) accepted() bool { return use == present || use == unproven }

// kindUses is what may name an object of a kind beside its references.
type kindUses struct {
	// rights is a role giving rights on the object.
	rights presence
	// offered are the rights the configurator offers on an object of the
	// kind, in the order of the help (kindRightsOf); nil for a kind no
	// export gives rights on.
	offered []PermissionOperation
	// registration is an exchange plan registering the object's changes.
	registration presence
	// option is a functional option switching the object.
	option presence
	// restriction is a role restricting access to the object's data (help,
	// «Ограничение доступа к данным»): absent where the configurator offers
	// none of the rights restricted, which are the rights to data an object
	// keeps in the infobase; present where the exports write one.
	restriction presence
}

// usesOf is what may name an object of the kind; an unknown kind has no use.
func usesOf(kind Kind) kindUses {
	item, _ := kindInfoOf(kind)
	return item.uses
}

// kindUsesOf is the uses of every kind, kept beside kindInfos rather than in
// each line of it, as three columns read better than fifty chained calls.
//
// present is what the exports of the configurator write: the four checked,
// demo-base and energy (the Rights.xml of every role, the Content.xml of
// every exchange plan, the Content of every functional option).
//
// registration is by the help besides: a kind's data has ОбменДанными where
// its changes can be registered (СправочникОбъект, ДокументОбъект,
// ПоследовательностьНаборЗаписей, ПерерасчетНаборЗаписей,
// КонстантаМенеджерЗначения, ПланОбменаОбъект... .ОбменДанными), and a kind
// with none has no data for ЗарегистрироватьИзменения to register; no export
// puts an exchange plan into the content of a plan.
//
// Neither the help nor an export says which kinds a role or a functional
// option may name beyond those the exports show, so the rest are unproven,
// not absent.
var kindUsesOf = map[Kind]kindUses{
	RoleKind:                    {rights: unproven, registration: absent, option: unproven, restriction: absent},
	SubsystemKind:               {rights: present, registration: absent, option: present, restriction: absent},
	ConstantKind:                {rights: present, registration: present, option: present, restriction: unproven},
	SessionParameterKind:        {rights: present, registration: absent, option: unproven, restriction: absent},
	CommonAttributeKind:         {rights: present, registration: absent, option: present, restriction: absent},
	CommonModuleKind:            {rights: unproven, registration: absent, option: unproven, restriction: absent},
	EventSubscriptionKind:       {rights: unproven, registration: absent, option: unproven, restriction: absent},
	EnumerationKind:             {rights: unproven, registration: absent, option: present, restriction: absent},
	ScheduledJobKind:            {rights: unproven, registration: absent, option: unproven, restriction: absent},
	CommandGroupKind:            {rights: unproven, registration: absent, option: unproven, restriction: absent},
	CommonCommandKind:           {rights: present, registration: absent, option: present, restriction: absent},
	CommonFormKind:              {rights: present, registration: absent, option: present, restriction: absent},
	CommonTemplateKind:          {rights: unproven, registration: absent, option: unproven, restriction: absent},
	XDTOPackageKind:             {rights: unproven, registration: absent, option: unproven, restriction: absent},
	WebServiceKind:              {rights: present, registration: absent, option: unproven, restriction: absent},
	WSReferenceKind:             {rights: unproven, registration: absent, option: unproven, restriction: absent},
	WebSocketClientKind:         {rights: unproven, registration: absent, option: unproven, restriction: absent},
	ExternalDataSourceKind:      {rights: present, registration: absent, option: unproven, restriction: absent},
	ExternalDataSourceTableKind: {rights: present, registration: absent, option: unproven, restriction: unproven},
	ExternalCubeKind:            {rights: present, registration: absent, option: unproven, restriction: unproven},
	ExternalDimensionTableKind:  {rights: present, registration: absent, option: unproven, restriction: unproven},
	// A function keeps no data to register or restrict (help: ОбъектМетаданных:
	// Функция has no ОбменДанными, and its value is computed by the source);
	// ПравоДоступа takes it, and no export gives rights on one.
	ExternalFunctionKind:           {rights: unproven, registration: absent, option: unproven, restriction: absent},
	HTTPServiceKind:                {rights: present, registration: absent, option: unproven, restriction: absent},
	CommonPictureKind:              {rights: unproven, registration: absent, option: unproven, restriction: absent},
	StyleItemKind:                  {rights: unproven, registration: absent, option: unproven, restriction: absent},
	StyleKind:                      {rights: unproven, registration: absent, option: unproven, restriction: absent},
	FunctionalOptionKind:           {rights: unproven, registration: absent, option: unproven, restriction: absent},
	FunctionalOptionParameterKind:  {rights: unproven, registration: absent, option: unproven, restriction: absent},
	SettingsStorageKind:            {rights: unproven, registration: absent, option: unproven, restriction: absent},
	FilterCriterionKind:            {rights: present, registration: absent, option: present, restriction: absent},
	DefinedTypeKind:                {rights: unproven, registration: absent, option: unproven, restriction: absent},
	LanguageKind:                   {rights: unproven, registration: absent, option: unproven, restriction: absent},
	CatalogKind:                    {rights: present, registration: present, option: present, restriction: present},
	ChartOfCharacteristicTypesKind: {rights: present, registration: present, option: present, restriction: present},
	ChartOfAccountsKind:            {rights: present, registration: present, option: present, restriction: unproven},
	ChartOfCalculationTypesKind:    {rights: present, registration: present, option: present, restriction: unproven},
	TaskKind:                       {rights: present, registration: present, option: present, restriction: present},
	AccountingRegisterKind:         {rights: present, registration: present, option: present, restriction: present},
	CalculationRegisterKind:        {rights: present, registration: present, option: present, restriction: present},
	ReportKind:                     {rights: present, registration: absent, option: present, restriction: absent},
	DataProcessorKind:              {rights: present, registration: absent, option: present, restriction: absent},
	NumeratorKind:                  {rights: unproven, registration: absent, option: unproven, restriction: absent},
	SequenceKind:                   {rights: present, registration: present, option: unproven, restriction: unproven},
	DocumentJournalKind:            {rights: present, registration: absent, option: present, restriction: present},
	ExchangePlanKind:               {rights: present, registration: unproven, option: present, restriction: unproven},
	BusinessProcessKind:            {rights: present, registration: present, option: present, restriction: present},
	DocumentKind:                   {rights: present, registration: present, option: present, restriction: present},
	InformationRegisterKind:        {rights: present, registration: present, option: present, restriction: present},
	AccumulationRegisterKind:       {rights: present, registration: present, option: present, restriction: present},
	RecalculationKind:              {rights: present, registration: present, option: unproven, restriction: unproven},
}

// The rights of the history of data: nine on a kind whose data keeps
// versions that may be missing, seven on a constant, which always has its
// value.
var (
	historyRights = []PermissionOperation{PermissionReadDataHistory, PermissionViewDataHistory,
		PermissionUpdateDataHistory, PermissionUpdateDataHistorySettings, PermissionUpdateDataHistoryVersionComment,
		PermissionEditDataHistoryVersionComment, PermissionSwitchToDataHistoryVersion}
	missingHistoryRights = []PermissionOperation{PermissionReadDataHistoryOfMissingData, PermissionUpdateDataHistoryOfMissingData}
	// referenceRights are the rights on the data of an object with a
	// reference: a catalog, a node of an exchange plan, a process, a task.
	referenceRights = []PermissionOperation{PermissionRead, PermissionInsert, PermissionUpdate, PermissionDelete,
		PermissionView, PermissionInteractiveInsert, PermissionEdit, PermissionInteractiveDelete,
		PermissionInteractiveSetDeletionMark, PermissionInteractiveClearDeletionMark, PermissionInteractiveDeleteMarked,
		PermissionInputByString}
	predefinedRights = []PermissionOperation{PermissionInteractiveDeletePredefinedData,
		PermissionInteractiveSetDeletionMarkPredefinedData, PermissionInteractiveClearDeletionMarkPredefinedData,
		PermissionInteractiveDeleteMarkedPredefinedData}
	// registerRights are the rights on a register's records.
	registerRights = []PermissionOperation{PermissionRead, PermissionUpdate, PermissionView, PermissionEdit}
)

// kindRightsOf is, for every kind whose objects a role gives rights on (the
// kinds kindUsesOf marks present), the rights the configurator offers on such
// an object. The help lists the rights and not the kinds they go with
// (ПравоДоступа), so the lists are the exports': every right the Rights.xml of
// the roles of the four exports checked, demo-base, energy, ssl and the
// configurator's samples in mdclasses write on an object of the kind. They
// agree to the right: whether an object keeps history or predefined data, or
// is posted at all, does not change its list.
//
// A web and an HTTP service take their rights on their operations and
// methods, not on themselves: their lists are empty. The configuration
// itself has configurationRights.
var kindRightsOf = map[Kind][]PermissionOperation{
	SubsystemKind:        {PermissionView},
	SessionParameterKind: {PermissionGet, PermissionSet},
	CommonAttributeKind:  {PermissionView, PermissionEdit},
	ExchangePlanKind:     slices.Concat(referenceRights, historyRights, missingHistoryRights),
	FilterCriterionKind:  {PermissionView},
	CommonCommandKind:    {PermissionView},
	CommonFormKind:       {PermissionView},
	WebServiceKind:       {},
	HTTPServiceKind:      {},
	ConstantKind:         slices.Concat(registerRights, historyRights),
	CatalogKind:          slices.Concat(referenceRights, predefinedRights, historyRights, missingHistoryRights),
	DocumentKind: slices.Concat(referenceRights, []PermissionOperation{PermissionPosting, PermissionUndoPosting,
		PermissionInteractivePosting, PermissionInteractivePostingRegular, PermissionInteractiveUndoPosting,
		PermissionInteractiveChangeOfPosted}, historyRights, missingHistoryRights),
	SequenceKind:                   {PermissionRead, PermissionUpdate},
	DocumentJournalKind:            {PermissionRead, PermissionView},
	ReportKind:                     {PermissionUse, PermissionView},
	DataProcessorKind:              {PermissionUse, PermissionView},
	ChartOfCharacteristicTypesKind: slices.Concat(referenceRights, predefinedRights, historyRights, missingHistoryRights),
	ChartOfAccountsKind:            slices.Concat(referenceRights, predefinedRights, historyRights, missingHistoryRights),
	ChartOfCalculationTypesKind:    slices.Concat(referenceRights, predefinedRights, historyRights, missingHistoryRights),
	InformationRegisterKind: slices.Concat(registerRights, []PermissionOperation{PermissionTotalsControl},
		historyRights, missingHistoryRights),
	AccumulationRegisterKind: slices.Concat(registerRights, []PermissionOperation{PermissionTotalsControl}),
	AccountingRegisterKind:   slices.Concat(registerRights, []PermissionOperation{PermissionTotalsControl}),
	CalculationRegisterKind:  registerRights,
	RecalculationKind:        {PermissionRead, PermissionUpdate},
	BusinessProcessKind: slices.Concat(referenceRights, []PermissionOperation{PermissionInteractiveActivate,
		PermissionStart, PermissionInteractiveStart}, historyRights, missingHistoryRights),
	TaskKind: slices.Concat(referenceRights, []PermissionOperation{PermissionInteractiveActivate,
		PermissionExecute, PermissionInteractiveExecute}, historyRights, missingHistoryRights),
	ExternalDataSourceKind: {PermissionUse, PermissionAdministration, PermissionStandardAuthenticationChange,
		PermissionSessionStandardAuthenticationChange, PermissionSessionOSAuthenticationChange},
	ExternalDataSourceTableKind: {PermissionRead, PermissionInsert, PermissionUpdate, PermissionDelete, PermissionView,
		PermissionInteractiveInsert, PermissionEdit, PermissionInteractiveDelete, PermissionInputByString},
	// A cube and a table of its dimension are read and shown, nothing more:
	// the configurator's samples in mdclasses write these two on both.
	ExternalCubeKind:           {PermissionRead, PermissionView},
	ExternalDimensionTableKind: {PermissionRead, PermissionView},
}

// configurationRights are the rights the configurator offers on the
// configuration itself, from the same exports: everything a user may do with
// the infobase as a whole, the clients and the modes of the main window. The
// mode of the technical specialist is written under its old name too
// (renamedRights); ending exclusive mode at the start of a session only the
// configurator's samples write.
var configurationRights = inHelpOrder([]PermissionOperation{
	PermissionAdministration, PermissionDataAdministration, PermissionUpdateDataBaseConfiguration,
	PermissionExclusiveMode, PermissionActiveUsers, PermissionEventLog, PermissionThinClient, PermissionWebClient,
	PermissionMobileClient, PermissionThickClient, PermissionExternalConnection, PermissionAutomation,
	PermissionTechnicalSpecialistMode, PermissionSaveUserData, PermissionInteractiveOpenExtDataProcessors,
	PermissionInteractiveOpenExtReports, PermissionOutput, PermissionConfigurationExtensionsAdministration,
	PermissionCollaborationSystemInfoBaseRegistration, PermissionMainWindowModeNormal,
	PermissionMainWindowModeWorkplace, PermissionMainWindowModeEmbeddedWorkplace,
	PermissionMainWindowModeFullscreenWorkplace, PermissionMainWindowModeKiosk,
	PermissionExclusiveModeTerminationAtSessionStart, PermissionAnalyticsSystemClient,
})

// inHelpOrder puts rights in the order of the help, the order the editor
// shows them in.
func inHelpOrder(rights []PermissionOperation) []PermissionOperation {
	result := slices.Clone(rights)
	slices.SortFunc(result, func(a, b PermissionOperation) int { return rightIndex[a] - rightIndex[b] })
	return result
}

// kindReference is one type of a reference to an object of a kind, the set
// that holds every such type ("" for a reference no set holds), and how a
// column keeps it.
type kindReference struct {
	kind, set TypeKind
	stored    referenceStorage
}

// referenceStorage is how a column of the infobase keeps a reference.
type referenceStorage int

const (
	// storageUnsaid is a reference that says nothing of its storage; a test
	// refuses it, so a new reference is not stored by omission.
	storageUnsaid referenceStorage = iota
	// tableRow is the identifier of a row of the object's own table, which
	// the column refers to by a foreign key.
	tableRow
	// listedValue is the identifier of a value the configuration lists: the
	// values of an enumeration have no table to refer to.
	listedValue
	// byName is a name: a point of a route is part of the map the
	// configuration draws, not a row of any table.
	byName
	// notStored is a reference the infobase never keeps: a record of an
	// external source lives in another database.
	notStored
)

// referenceStorages says, for every type of a reference, how a column keeps
// it; read off kindInfos in init.
var referenceStorages = map[TypeKind]referenceStorage{}

// storedAs is how a column keeps a reference of the type, storageUnsaid for
// a type that is no reference to an object of metadata.
func storedAs(kind TypeKind) referenceStorage { return referenceStorages[kind] }

// keptAsUUID says a column keeps a value of the type as uuid: an identifier
// itself, or a reference to a row or to a listed value.
func keptAsUUID(kind TypeKind) bool {
	return kind == UUIDType || storedAs(kind) == tableRow || storedAs(kind) == listedValue
}

// keptAsText says a value of the type is carried by its Data as written: a
// string, an identifier, and every reference the infobase keeps.
func keptAsText(kind TypeKind) bool {
	return kind == StringType || keptAsUUID(kind) || storedAs(kind) == byName
}

// referenceTo is the reference to an object of a kind itself, "" for a kind
// whose objects have none.
func referenceTo(kind Kind) TypeKind {
	item, _ := kindInfoOf(kind)
	if len(item.references) == 0 {
		return ""
	}
	return item.references[0].kind
}

// typeOwners says, for every type that names an object of metadata through
// Type.Reference, the kind of that object; objectTypes is the part of it
// that lives in memory only. Both are read off kindInfos in init.
var (
	typeOwners  = map[TypeKind]Kind{}
	objectTypes = map[TypeKind]bool{}
)

// kindWord is what a message calls an object of a kind: its English name in
// words, chart of accounts.
func kindWord(kind Kind) string { return identifierWords(kinds.Must(kind).Name.English) }

// identifierWords writes an identifier as words in lower case. Capitals
// written together are one word, as the name writes them: XDTO package, HTTP
// service, изменение аутентификации ОС сеанса.
func identifierWords(identifier string) string {
	name := []rune(identifier)
	var out strings.Builder
	for index, symbol := range name {
		if index > 0 && unicode.IsUpper(symbol) && (unicode.IsLower(name[index-1]) || index+1 < len(name) && unicode.IsLower(name[index+1])) {
			out.WriteByte(' ')
		}
		out.WriteRune(symbol)
	}
	word := strings.Fields(out.String())
	for index, part := range word {
		if strings.ToUpper(part) != part {
			word[index] = strings.ToLower(part)
		}
	}
	return strings.Join(word, " ")
}

// hasObject says an object of the kind with the identifier is in the
// catalog.
func (catalog *Catalog) hasObject(kind Kind, id uuid.UUID) bool {
	item, ok := kindInfoOf(kind)
	return ok && item.has != nil && item.has(catalog, id)
}

// kindOfObject finds the kind of an object of the catalog by its identifier,
// whatever the kind: identifiers are unique across a configuration.
func (catalog *Catalog) kindOfObject(id uuid.UUID) (Kind, bool) {
	object, ok := catalog.objects[id]
	return object.kind, ok
}

// typeNamesObject says the object a type names through Type.Reference is in
// the catalog. known is false for a type that names no object of metadata.
func (catalog *Catalog) typeNamesObject(kind TypeKind, id uuid.UUID) (found, known bool) {
	owner, known := typeOwners[kind]
	if !known {
		return false, false
	}
	return catalog.hasObject(owner, id), true
}

// kindInfos lists every kind the model reads from the project, in the order
// the load reads them. It is filled in init: an external data source's add
// reads its tables through DecodeSource, which looks the kinds up here.
var kindInfos []kindInfo

func init() {
	kindInfos = []kindInfo{
		{kind: RoleKind, described: reflect.TypeFor[RoleDefinition](),
			decode: func(source string, reader, rights io.Reader, configuration project.Project) (any, error) {
				return DecodeRole(source, reader, rights, configuration)
			},
			add:     collect(func(catalog *Catalog) *[]RoleDefinition { return &catalog.Roles }),
			objects: func(catalog *Catalog) reflect.Value { return reflect.ValueOf(&catalog.Roles).Elem() },
			has:     indexed(func(c *Catalog) map[uuid.UUID]int { return c.roleByID })},
		describedAs(SubsystemKind, DecodeSubsystem, func(catalog *Catalog) *[]SubsystemDefinition { return &catalog.Subsystems }, func(c *Catalog) map[uuid.UUID]int { return c.subsystemByID }),
		describedAs(ConstantKind, DecodeConstant, func(catalog *Catalog) *[]Constant { return &catalog.Constants }, func(c *Catalog) map[uuid.UUID]int { return c.constantByID }).
			holds(ConstantValueManagerType),
		describedAs(SessionParameterKind, DecodeSessionParameter, func(catalog *Catalog) *[]SessionParameter { return &catalog.SessionParameters }, func(c *Catalog) map[uuid.UUID]int { return c.sessionParameterByID }),
		describedAs(CommonAttributeKind, DecodeCommonAttribute, func(catalog *Catalog) *[]CommonAttributeDefinition { return &catalog.CommonAttributes }, func(c *Catalog) map[uuid.UUID]int { return c.commonAttributeByID }),
		describedAs(CommonModuleKind, DecodeCommonModule, func(catalog *Catalog) *[]CommonModuleDefinition { return &catalog.CommonModules }, func(c *Catalog) map[uuid.UUID]int { return c.commonModuleByID }),
		describedAs(EventSubscriptionKind, DecodeEventSubscription, func(catalog *Catalog) *[]EventSubscriptionDefinition { return &catalog.EventSubscriptions }, func(c *Catalog) map[uuid.UUID]int { return c.eventSubscriptionByID }),
		describedAs(EnumerationKind, DecodeEnumeration, func(catalog *Catalog) *[]Enumeration { return &catalog.Enumerations }, func(c *Catalog) map[uuid.UUID]int { return c.enumerationByID }).
			refers(kindReference{EnumerationType, EnumerationSet, listedValue}).
			holds(EnumerationManagerType),
		describedAs(ScheduledJobKind, DecodeScheduledJob, func(catalog *Catalog) *[]ScheduledJobDefinition { return &catalog.ScheduledJobs }, func(c *Catalog) map[uuid.UUID]int { return c.scheduledJobByID }),
		describedAs(CommandGroupKind, DecodeCommandGroup, func(catalog *Catalog) *[]CommandGroupDefinition { return &catalog.CommandGroups }, func(c *Catalog) map[uuid.UUID]int { return c.commandGroupByID }),
		describedAs(CommonCommandKind, DecodeCommonCommand, func(catalog *Catalog) *[]CommonCommandDefinition { return &catalog.CommonCommands }, func(c *Catalog) map[uuid.UUID]int { return c.commonCommandByID }),
		describedAs(CommonTemplateKind, DecodeCommonTemplate, func(catalog *Catalog) *[]CommonTemplateDefinition { return &catalog.CommonTemplates }, func(c *Catalog) map[uuid.UUID]int { return c.commonTemplateByID }),
		describedAs(XDTOPackageKind, DecodeXDTOPackage, func(catalog *Catalog) *[]XDTOPackageDefinition { return &catalog.XDTOPackages }, func(c *Catalog) map[uuid.UUID]int { return c.xdtoPackageByID }),
		describedAs(WebServiceKind, DecodeWebService, func(catalog *Catalog) *[]WebServiceDefinition { return &catalog.WebServices }, func(c *Catalog) map[uuid.UUID]int { return c.webServiceByID }),
		describedAs(WSReferenceKind, DecodeWSReference, func(catalog *Catalog) *[]WSReferenceDefinition { return &catalog.WSReferences }, func(c *Catalog) map[uuid.UUID]int { return c.wsReferenceByID }),
		describedAs(WebSocketClientKind, DecodeWebSocketClient, func(catalog *Catalog) *[]WebSocketClientDefinition { return &catalog.WebSocketClients }, func(c *Catalog) map[uuid.UUID]int { return c.webSocketClientByID }),
		{kind: ExternalDataSourceKind, described: reflect.TypeFor[ExternalDataSourceDefinition](),
			decode: decodeWith(DecodeExternalDataSource), add: addExternalDataSource,
			objects: func(catalog *Catalog) reflect.Value { return reflect.ValueOf(&catalog.ExternalDataSources).Elem() },
			has:     indexed(func(c *Catalog) map[uuid.UUID]int { return c.externalDataSourceByID })},
		// A table, a cube and a dimension table are found among the objects of
		// every source.
		describedAs(ExternalDataSourceTableKind, DecodeExternalTable, nil, nil).
			found(func(c *Catalog, id uuid.UUID) bool { _, ok := c.externalTableByID[id]; return ok }).
			refers(kindReference{ExternalTableType, "", notStored}),
		describedAs(ExternalCubeKind, DecodeExternalCube, nil, nil).
			found(func(c *Catalog, id uuid.UUID) bool { return c.objects[id].kind == ExternalCubeKind }),
		describedAs(ExternalDimensionTableKind, DecodeExternalDimensionTable, nil, nil).
			found(func(c *Catalog, id uuid.UUID) bool { _, ok := c.externalDimensionTableByID[id]; return ok }).
			refers(kindReference{ExternalDimensionTableType, "", notStored}),
		// A function is read inside the description of its source: no
		// reading of its own, only what may name it.
		{kind: ExternalFunctionKind, has: func(c *Catalog, id uuid.UUID) bool { return c.objects[id].kind == ExternalFunctionKind }},
		describedAs(HTTPServiceKind, DecodeHTTPService, func(catalog *Catalog) *[]HTTPServiceDefinition { return &catalog.HTTPServices }, func(c *Catalog) map[uuid.UUID]int { return c.httpServiceByID }),
		describedAs(CommonPictureKind, DecodeCommonPicture, func(catalog *Catalog) *[]CommonPictureDefinition { return &catalog.CommonPictures }, func(c *Catalog) map[uuid.UUID]int { return c.commonPictureByID }),
		describedAs(StyleItemKind, DecodeStyleItem, func(catalog *Catalog) *[]StyleItemDefinition { return &catalog.StyleItems }, func(c *Catalog) map[uuid.UUID]int { return c.styleItemByID }),
		describedAs(StyleKind, DecodeStyle, func(catalog *Catalog) *[]StyleDefinition { return &catalog.Styles }, func(c *Catalog) map[uuid.UUID]int { return c.styleByID }),
		describedAs(FunctionalOptionKind, DecodeFunctionalOption, func(catalog *Catalog) *[]FunctionalOptionDefinition { return &catalog.FunctionalOptions }, func(c *Catalog) map[uuid.UUID]int { return c.functionalOptionByID }),
		describedAs(FunctionalOptionParameterKind, DecodeFunctionalOptionParameter, func(catalog *Catalog) *[]FunctionalOptionParameterDefinition {
			return &catalog.FunctionalOptionParameters
		}, func(c *Catalog) map[uuid.UUID]int { return c.functionalOptionParameterByID }),
		describedAs(SettingsStorageKind, DecodeSettingsStorage, func(catalog *Catalog) *[]SettingsStorageDefinition { return &catalog.SettingsStorages }, func(c *Catalog) map[uuid.UUID]int { return c.settingsStorageByID }),
		describedAs(FilterCriterionKind, DecodeFilterCriterion, func(catalog *Catalog) *[]FilterCriterionDefinition { return &catalog.FilterCriteria }, func(c *Catalog) map[uuid.UUID]int { return c.filterCriterionByID }),
		describedAs(DefinedTypeKind, DecodeDefinedType, func(catalog *Catalog) *[]DefinedTypeObject { return &catalog.DefinedTypes }, func(c *Catalog) map[uuid.UUID]int { return c.definedTypeByID }).
			names(DefinedType),
		describedAs(CatalogKind, DecodeCatalog, func(catalog *Catalog) *[]CatalogDefinition { return &catalog.Catalogs }, func(c *Catalog) map[uuid.UUID]int { return c.catalogByID }).
			refers(kindReference{CatalogType, CatalogSet, tableRow}).
			holds(CatalogObjectType, CatalogManagerType),
		describedAs(ChartOfCharacteristicTypesKind, DecodeChartOfCharacteristicTypes, func(catalog *Catalog) *[]ChartOfCharacteristicTypesDefinition {
			return &catalog.ChartsOfCharacteristicTypes
		}, func(c *Catalog) map[uuid.UUID]int { return c.chartOfCharacteristicTypesByID }).
			refers(kindReference{CharacteristicTypesType, CharacteristicTypesSet, tableRow}).
			holds(CharacteristicTypesObjectType, CharacteristicTypesManagerType).
			names(CharacteristicSet),
		describedAs(ChartOfAccountsKind, DecodeChartOfAccounts, func(catalog *Catalog) *[]ChartOfAccountsDefinition { return &catalog.ChartsOfAccounts }, func(c *Catalog) map[uuid.UUID]int { return c.chartOfAccountsByID }).
			refers(kindReference{AccountType, AccountSet, tableRow}).
			holds(AccountsObjectType, AccountsManagerType),
		describedAs(ChartOfCalculationTypesKind, DecodeChartOfCalculationTypes, func(catalog *Catalog) *[]ChartOfCalculationTypesDefinition {
			return &catalog.ChartsOfCalculationTypes
		}, func(c *Catalog) map[uuid.UUID]int { return c.chartOfCalculationTypesByID }).
			refers(kindReference{CalculationTypeType, CalculationTypeSet, tableRow}).
			holds(CalculationTypesObjectType, CalculationTypesManagerType),
		describedAs(TaskKind, DecodeTask, func(catalog *Catalog) *[]TaskDefinition { return &catalog.Tasks }, func(c *Catalog) map[uuid.UUID]int { return c.taskByID }).
			refers(kindReference{TaskType, TaskSet, tableRow}).
			holds(TaskObjectType, TaskManagerType),
		describedAs(AccountingRegisterKind, DecodeAccountingRegister, func(catalog *Catalog) *[]AccountingRegisterDefinition { return &catalog.AccountingRegisters }, func(c *Catalog) map[uuid.UUID]int { return c.accountingRegisterByID }).
			holds(AccountingRegisterRecordSetType, AccountingRegisterManagerType),
		describedAs(CalculationRegisterKind, DecodeCalculationRegister, func(catalog *Catalog) *[]CalculationRegisterDefinition { return &catalog.CalculationRegisters }, func(c *Catalog) map[uuid.UUID]int { return c.calculationRegisterByID }).
			holds(CalculationRegisterRecordSetType, CalculationRegisterManagerType),
		describedAs(ReportKind, DecodeReport, func(catalog *Catalog) *[]ReportDefinition { return &catalog.Reports }, func(c *Catalog) map[uuid.UUID]int { return c.reportByID }).
			holds(ReportObjectType),
		describedAs(DataProcessorKind, DecodeDataProcessor, func(catalog *Catalog) *[]DataProcessorDefinition { return &catalog.DataProcessors }, func(c *Catalog) map[uuid.UUID]int { return c.dataProcessorByID }).
			holds(DataProcessorObjectType),
		describedAs(NumeratorKind, DecodeNumerator, func(catalog *Catalog) *[]NumeratorDefinition { return &catalog.Numerators }, func(c *Catalog) map[uuid.UUID]int { return c.numeratorByID }),
		describedAs(SequenceKind, DecodeSequence, func(catalog *Catalog) *[]SequenceDefinition { return &catalog.Sequences }, func(c *Catalog) map[uuid.UUID]int { return c.sequenceByID }).
			holds(SequenceRecordSetType),
		describedAs(DocumentJournalKind, DecodeDocumentJournal, func(catalog *Catalog) *[]DocumentJournalDefinition { return &catalog.DocumentJournals }, func(c *Catalog) map[uuid.UUID]int { return c.documentJournalByID }).
			holds(DocumentJournalManagerType),
		describedAs(ExchangePlanKind, DecodeExchangePlan, func(catalog *Catalog) *[]ExchangePlanDefinition { return &catalog.ExchangePlans }, func(c *Catalog) map[uuid.UUID]int { return c.exchangePlanByID }).
			refers(kindReference{ExchangePlanType, ExchangePlanSet, tableRow}).
			holds(ExchangePlanObjectType, ExchangePlanManagerType),
		describedAs(BusinessProcessKind, DecodeBusinessProcess, func(catalog *Catalog) *[]BusinessProcessDefinition { return &catalog.BusinessProcesses }, func(c *Catalog) map[uuid.UUID]int { return c.businessProcessByID }).
			refers(kindReference{BusinessProcessType, BusinessProcessSet, tableRow}, kindReference{RoutePointType, RoutePointSet, byName}).
			holds(BusinessProcessObjectType, BusinessProcessManagerType),
		describedAs(DocumentKind, DecodeDocument, func(catalog *Catalog) *[]DocumentDefinition { return &catalog.Documents }, func(c *Catalog) map[uuid.UUID]int { return c.documentByID }).
			refers(kindReference{DocumentType, DocumentSet, tableRow}).
			holds(DocumentObjectType, DocumentManagerType),
		describedAs(InformationRegisterKind, DecodeInformationRegister, func(catalog *Catalog) *[]InformationRegisterDefinition { return &catalog.InformationRegisters }, func(c *Catalog) map[uuid.UUID]int { return c.informationRegisterByID }).
			holds(InformationRegisterRecordSetType, InformationRegisterRecordManagerType, InformationRegisterManagerType),
		describedAs(AccumulationRegisterKind, DecodeAccumulationRegister, func(catalog *Catalog) *[]AccumulationRegisterDefinition { return &catalog.AccumulationRegisters }, func(c *Catalog) map[uuid.UUID]int { return c.accumulationRegisterByID }).
			holds(AccumulationRegisterRecordSetType, AccumulationRegisterManagerType),
		// Read inside its calculation register: no reading, no list, and no
		// index of its own.
		{kind: RecalculationKind, has: (*Catalog).hasRecalculation, objectTypes: []TypeKind{RecalculationRecordSetType}},
		// A common form is read from its folder as a form is, and a language
		// inside the description of the configuration: neither has a reading
		// here, only what may name it.
		{kind: CommonFormKind, has: func(c *Catalog, id uuid.UUID) bool { return c.objects[id].kind == CommonFormKind }},
		{kind: LanguageKind, has: func(c *Catalog, id uuid.UUID) bool {
			return slices.ContainsFunc(c.Project.Languages, func(language project.Language) bool { return language.ID == id })
		}},
	}
	for index := range kindInfos {
		kindInfos[index].uses = kindUsesOf[kindInfos[index].kind]
		if offered, ok := kindRightsOf[kindInfos[index].kind]; ok {
			kindInfos[index].uses.offered = inHelpOrder(offered)
		}
	}
	for _, item := range kindInfos {
		owned := slices.Concat(item.objectTypes, item.namedBy)
		for _, reference := range item.references {
			owned = append(owned, reference.kind)
			referenceStorages[reference.kind] = reference.stored
			if reference.set != "" {
				referenceSets[reference.set] = true
			}
		}
		for _, kind := range owned {
			typeOwners[kind] = item.kind
		}
		for _, kind := range item.objectTypes {
			objectTypes[kind] = true
		}
	}
}

// describedAs is the reading of a kind whose description is one file and goes
// into one list of the catalog; list is nil for a kind its owner collects.
func describedAs[T any](kind Kind, decode func(string, io.Reader, project.Project) (T, error), list func(*Catalog) *[]T, ids func(*Catalog) map[uuid.UUID]int) kindInfo {
	item := kindInfo{kind: kind, described: reflect.TypeFor[T](), decode: decodeWith(decode)}
	if list != nil {
		item.add = collect(list)
		item.objects = func(catalog *Catalog) reflect.Value { return reflect.ValueOf(list(catalog)).Elem() }
	}
	if ids != nil {
		item.has = indexed(ids)
	}
	return item
}

// indexed finds an object by the index of identifiers of its kind.
func indexed(ids func(*Catalog) map[uuid.UUID]int) func(*Catalog, uuid.UUID) bool {
	return func(catalog *Catalog, id uuid.UUID) bool { _, ok := ids(catalog)[id]; return ok }
}

// found, refers, holds and names fill in the rest of a kind.
func (item kindInfo) found(has func(*Catalog, uuid.UUID) bool) kindInfo {
	item.has = has
	return item
}

func (item kindInfo) refers(references ...kindReference) kindInfo {
	item.references = references
	return item
}

func (item kindInfo) holds(types ...TypeKind) kindInfo {
	item.objectTypes = types
	return item
}

func (item kindInfo) names(types ...TypeKind) kindInfo {
	item.namedBy = types
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

// kindInfoOf finds how a kind is read.
func kindInfoOf(kind Kind) (kindInfo, bool) {
	index := slices.IndexFunc(kindInfos, func(item kindInfo) bool { return item.kind == kind })
	if index < 0 {
		return kindInfo{}, false
	}
	return kindInfos[index], true
}

// objectsOf is the list of the catalog the objects of a kind are in; false is
// a kind the catalog keeps no list of.
func (catalog *Catalog) objectsOf(kind Kind) (reflect.Value, bool) {
	item, ok := kindInfoOf(kind)
	if !ok || item.objects == nil {
		return reflect.Value{}, false
	}
	return item.objects(catalog), true
}

// keptInFolder reports whether an object of a kind keeps a folder named after
// it rather than one file named by its identifier.
func keptInFolder(kind Kind) bool {
	return kinds.Must(kind).Layout().KeepsFolder()
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
func sourceAt(relative string) (kindInfo, func(uuid.UUID, string) error, bool) {
	if kind, name, rest, ok := project.SplitObjectPath(relative); ok {
		item, known := kindInfoOf(Kind(kind))
		if !known || item.decode == nil || len(rest) != 1 || rest[0] != project.ObjectMetadataFile {
			return kindInfo{}, nil, false
		}
		// The folder is named after the object's own name, which for an
		// object inside another is what follows the owner's.
		folder := name[strings.LastIndexByte(name, '.')+1:]
		return item, func(_ uuid.UUID, name string) error { return agreesWithFolder("object", name, folder) }, true
	}
	parts := strings.Split(relative, "/")
	if len(parts) != 3 || parts[0] != "metadata" || path.Ext(parts[2]) != ".yaml" {
		return kindInfo{}, nil, false
	}
	item, known := kindInfoOf(Kind(parts[1]))
	if !known || item.decode == nil {
		return kindInfo{}, nil, false
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

// sourceIdentity is what a description says it is. Every type kindInfos
// decodes has both fields (TestEverySourceKindIsChecked).
func sourceIdentity(value any) (uuid.UUID, string) {
	item := reflect.ValueOf(value)
	return item.FieldByName("ID").Interface().(uuid.UUID), item.FieldByName("Name").String()
}

// ReferenceTypeChoice is one type a type editor offers for a stored field
// that names an object of metadata - a reference, or a defined type - with
// every object of the configuration it may name.
type ReferenceTypeChoice struct {
	Kind TypeKind `json:"kind"`
	// Name is the prototype's name of the type: СправочникСсылка.
	Name    kinds.Words           `json:"name"`
	Objects []ReferenceTypeObject `json:"objects"`
}

// ReferenceTypeObject is one object a reference type may name.
type ReferenceTypeObject struct {
	ID    uuid.UUID     `json:"id"`
	Name  string        `json:"name"`
	Title LocalizedText `json:"title"`
}

// ReferenceTypeChoices is every type of a stored field that names an object
// of metadata, kind by kind as kindInfos lists them: the references, a
// reference to an external source aside (storedNowhere), and defined types.
func (catalog *Catalog) ReferenceTypeChoices() []ReferenceTypeChoice {
	var result []ReferenceTypeChoice
	for _, item := range kindInfos {
		names := kinds.Must(item.kind).References()
		for index, reference := range item.references {
			if !storedNowhere(reference.kind) {
				result = append(result, ReferenceTypeChoice{Kind: reference.kind, Name: names[index], Objects: catalog.referenceTypeObjects(item.kind)})
			}
		}
		if slices.Contains(item.namedBy, DefinedType) {
			result = append(result, ReferenceTypeChoice{Kind: DefinedType, Name: kinds.Must(item.kind).Name, Objects: catalog.referenceTypeObjects(item.kind)})
		}
	}
	return result
}

func (catalog *Catalog) referenceTypeObjects(kind Kind) []ReferenceTypeObject {
	list, _ := catalog.objectsOf(kind)
	result := make([]ReferenceTypeObject, 0, list.Len())
	for index := range list.Len() {
		object := list.Index(index)
		result = append(result, ReferenceTypeObject{
			ID: object.FieldByName("ID").Interface().(uuid.UUID), Name: object.FieldByName("Name").String(),
			Title: object.FieldByName("Title").Interface().(LocalizedText),
		})
	}
	return result
}
