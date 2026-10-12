package metadata

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const RoleKind = kinds.Role

// PermissionOperation is one access right, named after the prototype's own
// name for it in kebab case: Insert is insert, InteractivePosting is
// interactive-posting. The help of ПравоДоступа lists sixty-nine of them, and
// a role carries every one of them on whatever it is given to, whether or
// not this platform acts on it yet.
type PermissionOperation string

const (
	PermissionRead                                       PermissionOperation = "read"
	PermissionInsert                                     PermissionOperation = "insert"
	PermissionUpdate                                     PermissionOperation = "update"
	PermissionDelete                                     PermissionOperation = "delete"
	PermissionPosting                                    PermissionOperation = "posting"
	PermissionUndoPosting                                PermissionOperation = "undo-posting"
	PermissionView                                       PermissionOperation = "view"
	PermissionInteractiveInsert                          PermissionOperation = "interactive-insert"
	PermissionEdit                                       PermissionOperation = "edit"
	PermissionInteractiveDelete                          PermissionOperation = "interactive-delete"
	PermissionInteractiveSetDeletionMark                 PermissionOperation = "interactive-set-deletion-mark"
	PermissionInteractiveClearDeletionMark               PermissionOperation = "interactive-clear-deletion-mark"
	PermissionInteractiveDeleteMarked                    PermissionOperation = "interactive-delete-marked"
	PermissionInteractivePosting                         PermissionOperation = "interactive-posting"
	PermissionInteractivePostingRegular                  PermissionOperation = "interactive-posting-regular"
	PermissionInteractiveUndoPosting                     PermissionOperation = "interactive-undo-posting"
	PermissionInteractiveChangeOfPosted                  PermissionOperation = "interactive-change-of-posted"
	PermissionInputByString                              PermissionOperation = "input-by-string"
	PermissionTotalsControl                              PermissionOperation = "totals-control"
	PermissionUse                                        PermissionOperation = "use"
	PermissionAdministration                             PermissionOperation = "administration"
	PermissionDataAdministration                         PermissionOperation = "data-administration"
	PermissionExclusiveMode                              PermissionOperation = "exclusive-mode"
	PermissionActiveUsers                                PermissionOperation = "active-users"
	PermissionEventLog                                   PermissionOperation = "event-log"
	PermissionExternalConnection                         PermissionOperation = "external-connection"
	PermissionAutomation                                 PermissionOperation = "automation"
	PermissionInteractiveOpenExtDataProcessors           PermissionOperation = "interactive-open-ext-data-processors"
	PermissionInteractiveOpenExtReports                  PermissionOperation = "interactive-open-ext-reports"
	PermissionGet                                        PermissionOperation = "get"
	PermissionSet                                        PermissionOperation = "set"
	PermissionInteractiveActivate                        PermissionOperation = "interactive-activate"
	PermissionStart                                      PermissionOperation = "start"
	PermissionInteractiveStart                           PermissionOperation = "interactive-start"
	PermissionExecute                                    PermissionOperation = "execute"
	PermissionInteractiveExecute                         PermissionOperation = "interactive-execute"
	PermissionOutput                                     PermissionOperation = "output"
	PermissionUpdateDataBaseConfiguration                PermissionOperation = "update-database-configuration"
	PermissionThinClient                                 PermissionOperation = "thin-client"
	PermissionWebClient                                  PermissionOperation = "web-client"
	PermissionThickClient                                PermissionOperation = "thick-client"
	PermissionSaveUserData                               PermissionOperation = "save-user-data"
	PermissionStandardAuthenticationChange               PermissionOperation = "standard-authentication-change"
	PermissionSessionStandardAuthenticationChange        PermissionOperation = "session-standard-authentication-change"
	PermissionSessionOSAuthenticationChange              PermissionOperation = "session-os-authentication-change"
	PermissionInteractiveDeletePredefinedData            PermissionOperation = "interactive-delete-predefined-data"
	PermissionInteractiveSetDeletionMarkPredefinedData   PermissionOperation = "interactive-set-deletion-mark-predefined-data"
	PermissionInteractiveClearDeletionMarkPredefinedData PermissionOperation = "interactive-clear-deletion-mark-predefined-data"
	PermissionInteractiveDeleteMarkedPredefinedData      PermissionOperation = "interactive-delete-marked-predefined-data"
	PermissionConfigurationExtensionsAdministration      PermissionOperation = "configuration-extensions-administration"
	PermissionReadDataHistory                            PermissionOperation = "read-data-history"
	PermissionViewDataHistory                            PermissionOperation = "view-data-history"
	PermissionReadDataHistoryOfMissingData               PermissionOperation = "read-data-history-of-missing-data"
	PermissionUpdateDataHistory                          PermissionOperation = "update-data-history"
	PermissionUpdateDataHistoryOfMissingData             PermissionOperation = "update-data-history-of-missing-data"
	PermissionUpdateDataHistorySettings                  PermissionOperation = "update-data-history-settings"
	PermissionUpdateDataHistoryVersionComment            PermissionOperation = "update-data-history-version-comment"
	PermissionEditDataHistoryVersionComment              PermissionOperation = "edit-data-history-version-comment"
	PermissionSwitchToDataHistoryVersion                 PermissionOperation = "switch-to-data-history-version"
	PermissionCollaborationSystemInfoBaseRegistration    PermissionOperation = "collaboration-system-info-base-registration"
	PermissionMobileClient                               PermissionOperation = "mobile-client"
	PermissionMainWindowModeNormal                       PermissionOperation = "main-window-mode-normal"
	PermissionMainWindowModeWorkplace                    PermissionOperation = "main-window-mode-workplace"
	PermissionMainWindowModeEmbeddedWorkplace            PermissionOperation = "main-window-mode-embedded-workplace"
	PermissionMainWindowModeFullscreenWorkplace          PermissionOperation = "main-window-mode-fullscreen-workplace"
	PermissionMainWindowModeKiosk                        PermissionOperation = "main-window-mode-kiosk"
	PermissionTechnicalSpecialistMode                    PermissionOperation = "technical-specialist-mode"
	PermissionExclusiveModeTerminationAtSessionStart     PermissionOperation = "exclusive-mode-termination-at-session-start"
	PermissionAnalyticsSystemClient                      PermissionOperation = "analytics-system-client"
)

// roleRight is one right under the names the prototype knows it by.
type roleRight struct {
	operation PermissionOperation
	english   string
	russian   string
}

// roleRights is every right of the prototype, in the order of the help of
// ПравоДоступа (8.3.27). The four exports use sixty-eight of them: the one
// they do not, ending exclusive mode at the start of a session, the
// configurator writes in its own samples of 3.25 and 3.27.
var roleRights = []roleRight{
	{PermissionRead, "Read", "Чтение"},
	{PermissionInsert, "Insert", "Добавление"},
	{PermissionUpdate, "Update", "Изменение"},
	{PermissionDelete, "Delete", "Удаление"},
	{PermissionPosting, "Posting", "Проведение"},
	{PermissionUndoPosting, "UndoPosting", "ОтменаПроведения"},
	{PermissionView, "View", "Просмотр"},
	{PermissionInteractiveInsert, "InteractiveInsert", "ИнтерактивноеДобавление"},
	{PermissionEdit, "Edit", "Редактирование"},
	{PermissionInteractiveDelete, "InteractiveDelete", "ИнтерактивноеУдаление"},
	{PermissionInteractiveSetDeletionMark, "InteractiveSetDeletionMark", "ИнтерактивнаяПометкаУдаления"},
	{PermissionInteractiveClearDeletionMark, "InteractiveClearDeletionMark", "ИнтерактивноеСнятиеПометкиУдаления"},
	{PermissionInteractiveDeleteMarked, "InteractiveDeleteMarked", "ИнтерактивноеУдалениеПомеченных"},
	{PermissionInteractivePosting, "InteractivePosting", "ИнтерактивноеПроведение"},
	{PermissionInteractivePostingRegular, "InteractivePostingRegular", "ИнтерактивноеПроведениеНеОперативное"},
	{PermissionInteractiveUndoPosting, "InteractiveUndoPosting", "ИнтерактивнаяОтменаПроведения"},
	{PermissionInteractiveChangeOfPosted, "InteractiveChangeOfPosted", "ИнтерактивноеИзменениеПроведенных"},
	{PermissionInputByString, "InputByString", "ВводПоСтроке"},
	{PermissionTotalsControl, "TotalsControl", "УправлениеИтогами"},
	{PermissionUse, "Use", "Использование"},
	{PermissionAdministration, "Administration", "Администрирование"},
	{PermissionDataAdministration, "DataAdministration", "АдминистрированиеДанных"},
	{PermissionExclusiveMode, "ExclusiveMode", "МонопольныйРежим"},
	{PermissionActiveUsers, "ActiveUsers", "АктивныеПользователи"},
	{PermissionEventLog, "EventLog", "ЖурналРегистрации"},
	{PermissionExternalConnection, "ExternalConnection", "ВнешнееСоединение"},
	{PermissionAutomation, "Automation", "Automation"},
	{PermissionInteractiveOpenExtDataProcessors, "InteractiveOpenExtDataProcessors", "ИнтерактивноеОткрытиеВнешнихОбработок"},
	{PermissionInteractiveOpenExtReports, "InteractiveOpenExtReports", "ИнтерактивноеОткрытиеВнешнихОтчетов"},
	{PermissionGet, "Get", "Получение"},
	{PermissionSet, "Set", "Установка"},
	{PermissionInteractiveActivate, "InteractiveActivate", "ИнтерактивнаяАктивация"},
	{PermissionStart, "Start", "Старт"},
	{PermissionInteractiveStart, "InteractiveStart", "ИнтерактивныйСтарт"},
	{PermissionExecute, "Execute", "Выполнение"},
	{PermissionInteractiveExecute, "InteractiveExecute", "ИнтерактивноеВыполнение"},
	{PermissionOutput, "Output", "Вывод"},
	{PermissionUpdateDataBaseConfiguration, "UpdateDataBaseConfiguration", "ОбновлениеКонфигурацииБазыДанных"},
	{PermissionThinClient, "ThinClient", "ТонкийКлиент"},
	{PermissionWebClient, "WebClient", "ВебКлиент"},
	{PermissionThickClient, "ThickClient", "ТолстыйКлиент"},
	{PermissionSaveUserData, "SaveUserData", "СохранениеДанныхПользователя"},
	{PermissionStandardAuthenticationChange, "StandardAuthenticationChange", "ИзменениеСтандартнойАутентификации"},
	{PermissionSessionStandardAuthenticationChange, "SessionStandardAuthenticationChange", "ИзменениеСтандартнойАутентификацииСеанса"},
	{PermissionSessionOSAuthenticationChange, "SessionOSAuthenticationChange", "ИзменениеАутентификацииОССеанса"},
	{PermissionInteractiveDeletePredefinedData, "InteractiveDeletePredefinedData", "ИнтерактивноеУдалениеПредопределенныхДанных"},
	{PermissionInteractiveSetDeletionMarkPredefinedData, "InteractiveSetDeletionMarkPredefinedData", "ИнтерактивнаяПометкаУдаленияПредопределенныхДанных"},
	{PermissionInteractiveClearDeletionMarkPredefinedData, "InteractiveClearDeletionMarkPredefinedData", "ИнтерактивноеСнятиеПометкиУдаленияПредопределенныхДанных"},
	{PermissionInteractiveDeleteMarkedPredefinedData, "InteractiveDeleteMarkedPredefinedData", "ИнтерактивноеУдалениеПомеченныхПредопределенныхДанных"},
	{PermissionConfigurationExtensionsAdministration, "ConfigurationExtensionsAdministration", "АдминистрированиеРасширенийКонфигурации"},
	{PermissionReadDataHistory, "ReadDataHistory", "ЧтениеИсторииДанных"},
	{PermissionViewDataHistory, "ViewDataHistory", "ПросмотрИсторииДанных"},
	{PermissionReadDataHistoryOfMissingData, "ReadDataHistoryOfMissingData", "ЧтениеИсторииДанныхОтсутствующихДанных"},
	{PermissionUpdateDataHistory, "UpdateDataHistory", "ИзменениеИсторииДанных"},
	{PermissionUpdateDataHistoryOfMissingData, "UpdateDataHistoryOfMissingData", "ИзменениеИсторииДанныхОтсутствующихДанных"},
	{PermissionUpdateDataHistorySettings, "UpdateDataHistorySettings", "ИзменениеНастроекИсторииДанных"},
	{PermissionUpdateDataHistoryVersionComment, "UpdateDataHistoryVersionComment", "ИзменениеКомментарияВерсииИсторииДанных"},
	{PermissionEditDataHistoryVersionComment, "EditDataHistoryVersionComment", "РедактированиеКомментарияВерсииИсторииДанных"},
	{PermissionSwitchToDataHistoryVersion, "SwitchToDataHistoryVersion", "ПереходНаВерсиюИсторииДанных"},
	{PermissionCollaborationSystemInfoBaseRegistration, "CollaborationSystemInfoBaseRegistration", "РегистрацияИнформационнойБазыСистемыВзаимодействия"},
	{PermissionMobileClient, "MobileClient", "МобильныйКлиент"},
	{PermissionMainWindowModeNormal, "MainWindowModeNormal", "РежимОсновногоОкнаОбычный"},
	{PermissionMainWindowModeWorkplace, "MainWindowModeWorkplace", "РежимОсновногоОкнаРабочееМесто"},
	{PermissionMainWindowModeEmbeddedWorkplace, "MainWindowModeEmbeddedWorkplace", "РежимОсновногоОкнаВстроенноеРабочееМесто"},
	{PermissionMainWindowModeFullscreenWorkplace, "MainWindowModeFullscreenWorkplace", "РежимОсновногоОкнаПолноэкранноеРабочееМесто"},
	{PermissionMainWindowModeKiosk, "MainWindowModeKiosk", "РежимОсновногоОкнаКиоск"},
	{PermissionTechnicalSpecialistMode, "TechnicalSpecialistMode", "РежимТехническогоСпециалиста"},
	{PermissionExclusiveModeTerminationAtSessionStart, "ExclusiveModeTerminationAtSessionStart", "ЗавершениеМонопольногоРежимаПриНачалеСеанса"},
	{PermissionAnalyticsSystemClient, "AnalyticsSystemClient", "КлиентСистемыАналитики"},
}

// renamedRights are the names a right was known by before. The help says the
// mode of the technical specialist was called the mode of all functions
// before 8.3.16, and a configuration kept in compatibility with an earlier
// version writes it under the old name and never under the new one.
var renamedRights = map[string]PermissionOperation{"AllFunctionsMode": PermissionTechnicalSpecialistMode}

// knownRights and executedRights answer by name what roleRights and
// objectOperations list in order.
var knownRights, executedRights = func() (map[PermissionOperation]bool, map[PermissionOperation]bool) {
	known := make(map[PermissionOperation]bool, len(roleRights))
	for _, right := range roleRights {
		known[right.operation] = true
	}
	executed := make(map[PermissionOperation]bool, len(objectOperations))
	for _, operation := range objectOperations {
		executed[operation] = true
	}
	return known, executed
}()

// rightIndex is the place of every right in the order of the help.
var rightIndex = func() map[PermissionOperation]int {
	result := make(map[PermissionOperation]int, len(roleRights))
	for index, right := range roleRights {
		result[right.operation] = index
	}
	return result
}()

// RoleRights lists every right a role can carry, in the order of the help.
func RoleRights() []PermissionOperation {
	result := make([]PermissionOperation, len(roleRights))
	for index, right := range roleRights {
		result[index] = right.operation
	}
	return result
}

// RightByPrototypeName finds a right by either of the names the prototype
// writes it under, or by the one it was called before, case aside.
func RightByPrototypeName(name string) (PermissionOperation, bool) {
	for old, operation := range renamedRights {
		if strings.EqualFold(old, name) {
			return operation, true
		}
	}
	for _, right := range roleRights {
		if strings.EqualFold(right.english, name) || strings.EqualFold(right.russian, name) {
			return right.operation, true
		}
	}
	return "", false
}

// objectOperations are the rights this platform acts on, in a stable order:
// it decides how they are shown and in what order they are compiled, and a
// set would make both depend on map iteration. Every other right is carried
// and not followed - following them is block 10's.
var objectOperations = []PermissionOperation{
	PermissionRead, PermissionView, PermissionInsert, PermissionUpdate,
	PermissionDelete, PermissionPosting, PermissionUndoPosting, PermissionTotalsControl,
}

// RoleDefinition contains additive grants, not system/Manager privileges.
// Omitted objects, fields and commands grant nothing. Object operations do not
// implicitly grant fields; command access does not grant underlying operations.
//
// On disk a role is two files. The description - the name, the flags, the
// restrictions and their templates - is YAML, as everything else is; the grid
// of rights is RoleRightsFile, JSON, in the folder named by the role's
// identifier. The grid is the largest content of a configuration after the
// spreadsheets: acc keeps 20 MiB of it, read from YAML in 1.5 s and 459 MiB,
// from JSON in 0.17 s and 68 MiB (11.10.2026, all roles of the four exports).
type RoleDefinition struct {
	Format   int                 `yaml:"format" json:"format"`
	ID       uuid.UUID           `yaml:"id" json:"id"`
	Name     string              `yaml:"name" json:"name"`
	Title    LocalizedText       `yaml:"title" json:"title"`
	Comment  string              `yaml:"comment,omitempty" json:"comment,omitempty"`
	Objects  []ObjectPermission  `yaml:"-" json:"objects,omitempty"`
	Commands []CommandPermission `yaml:"-" json:"commands,omitempty"`
	// GrantNewObjectsByDefault: a newly created metadata object automatically
	// gets PermissionRead in this role, instead of staying inaccessible until
	// a developer configures it by hand.
	GrantNewObjectsByDefault bool `yaml:"grant_new_objects_by_default,omitempty" json:"grantNewObjectsByDefault,omitempty"`
	// GrantNewFieldsByDefault: the same, but for a new attribute/table part
	// added to an object this role already has some access to - it never
	// grants a field on an object the role has no ObjectPermission entry
	// for at all.
	GrantNewFieldsByDefault bool `yaml:"grant_new_fields_by_default,omitempty" json:"grantNewFieldsByDefault,omitempty"`
	// IndependentChildRights is the prototype's "independent rights of child
	// objects": left off, a right on an attribute, a table part, an attribute
	// of a table part or a command holds only where the role also has that
	// right on the object owning it, checked before the roles are added up;
	// set, the child's own setting alone decides (help, the role editor). Off
	// is the prototype's default; erp sets it in 2 roles of 1211. Carried and
	// not yet followed - the check of rights is block 10's.
	IndependentChildRights bool `yaml:"independent_child_rights,omitempty" json:"independentChildRights,omitempty"`
	// PolicyTemplates are rules named once and reused by this role's own
	// restrictions. They grant nothing by themselves.
	PolicyTemplates []PolicyTemplate `yaml:"policy_templates,omitempty" json:"policyTemplates,omitempty"`
}

// ObjectPermission is what a role says about one object: the configuration
// itself, by the project's identifier, or an object of it.
//
// A right is in one of three states, as in the prototype: granted, denied in
// so many words, or not mentioned. The last two are not the same. With the
// role's rights set for attributes by default, an attribute the role says
// nothing about takes the right of its object, and one denied keeps it denied
// (the four exports write 466 thousand denials against 150 thousand grants).
//
// Which rights an object can use does not refuse a role: the help gives no
// list by kind, and a role of the prototype keeps rights its object has no use
// for - posting on a document that is never posted, managing the totals of an
// information register. They are carried; whether they act is for the check
// of rights to say. A right no export writes on such an object is carried
// with the note right-without-sample.
type ObjectPermission struct {
	Object     uuid.UUID             `yaml:"object" json:"object"`
	Operations []PermissionOperation `yaml:"operations,omitempty" json:"operations,omitempty"`
	Denied     []PermissionOperation `yaml:"denied,omitempty" json:"denied,omitempty"`
	Fields     []FieldPermission     `yaml:"fields,omitempty" json:"fields,omitempty"`
	// Policies narrow which rows the granted operations reach. They never widen
	// access: an object with no operation grant stays unreachable regardless.
	Policies []AccessPolicy `yaml:"policies,omitempty" json:"policies,omitempty"`
}

// FieldPermission is what a role says about one part of an object - its
// attribute, standard attribute, table part, dimension, resource, accounting
// flag, addressing attribute, command, operation of a web service or method of
// an HTTP service. Field is the part's identifier, or for a standard one its
// prototype name in lower case: recordtype, and for a field of a standard
// table part the part's name before it, extdimensiontypes.turnoversonly.
// Names and translated titles are never security identifiers. A table part
// grant does not grant its columns.
type FieldPermission struct {
	Field      string                `yaml:"field" json:"field"`
	Operations []PermissionOperation `yaml:"operations,omitempty" json:"operations,omitempty"`
	Denied     []PermissionOperation `yaml:"denied,omitempty" json:"denied,omitempty"`
}

// Commands are scoped to the form UUID, since command IDs are unique per form.
// Standard autogenerated commands are controlled by their object operations;
// these grants address commands explicitly declared in managed form YAML.
type CommandPermission struct {
	Form    uuid.UUID `yaml:"form" json:"form"`
	Command uuid.UUID `yaml:"command" json:"command"`
}

// RoleRightsFile is the name the grid of a role's rights is kept under, in the
// folder named by the role's identifier beside its description.
const RoleRightsFile = "rights.json"

// roleDescription is the YAML half of a role: everything but the grid, with
// the restrictions keyed by the object they narrow.
type roleDescription struct {
	Format                   int               `yaml:"format"`
	ID                       uuid.UUID         `yaml:"id"`
	Name                     string            `yaml:"name"`
	Title                    LocalizedText     `yaml:"title"`
	Comment                  string            `yaml:"comment,omitempty"`
	GrantNewObjectsByDefault bool              `yaml:"grant_new_objects_by_default,omitempty"`
	GrantNewFieldsByDefault  bool              `yaml:"grant_new_fields_by_default,omitempty"`
	IndependentChildRights   bool              `yaml:"independent_child_rights,omitempty"`
	PolicyTemplates          []PolicyTemplate  `yaml:"policy_templates,omitempty"`
	Restrictions             []roleRestriction `yaml:"restrictions,omitempty"`
}

type roleRestriction struct {
	Object   uuid.UUID      `yaml:"object"`
	Policies []AccessPolicy `yaml:"policies"`
}

// roleGrid is the JSON half: every object the role says anything about, in
// the role's own order, and the commands of forms.
type roleGrid struct {
	Objects  []roleGridObject    `json:"objects,omitempty"`
	Commands []CommandPermission `json:"commands,omitempty"`
}

type roleGridObject struct {
	Object     uuid.UUID             `json:"object"`
	Operations []PermissionOperation `json:"operations,omitempty"`
	Denied     []PermissionOperation `json:"denied,omitempty"`
	Fields     []FieldPermission     `json:"fields,omitempty"`
}

// EncodeRole writes a role as its two files. The grid is nil for a role that
// says nothing about any object or command: such a role keeps no folder.
func EncodeRole(role RoleDefinition) (description []byte, grid []byte, err error) {
	value := roleDescription{
		Format: role.Format, ID: role.ID, Name: role.Name, Title: role.Title, Comment: role.Comment,
		GrantNewObjectsByDefault: role.GrantNewObjectsByDefault, GrantNewFieldsByDefault: role.GrantNewFieldsByDefault,
		IndependentChildRights: role.IndependentChildRights, PolicyTemplates: role.PolicyTemplates,
	}
	var rights roleGrid
	for _, object := range role.Objects {
		rights.Objects = append(rights.Objects, roleGridObject{Object: object.Object, Operations: object.Operations, Denied: object.Denied, Fields: object.Fields})
		if len(object.Policies) > 0 {
			value.Restrictions = append(value.Restrictions, roleRestriction{Object: object.Object, Policies: object.Policies})
		}
	}
	rights.Commands = role.Commands
	var content bytes.Buffer
	if err := Encode(&content, value); err != nil {
		return nil, nil, err
	}
	if len(rights.Objects) == 0 && len(rights.Commands) == 0 {
		return content.Bytes(), nil, nil
	}
	// One value to a line, two spaces deep, fields in the order of the
	// struct: a right changed is a line or two of difference.
	encoded, err := json.MarshalIndent(rights, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("encode role rights: %w", err)
	}
	return content.Bytes(), append(encoded, '\n'), nil
}

// WriteRole writes a role into a project as its two files, removing the grid
// of a role that no longer keeps one. It writes and nothing more: Studio,
// which must not lose an edit made meanwhile, saves through its own revisions.
func WriteRole(root string, role RoleDefinition) error {
	description, grid, err := EncodeRole(role)
	if err != nil {
		return err
	}
	directory := filepath.Join(root, "metadata", string(RoleKind))
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create roles folder: %w", err)
	}
	if err := os.WriteFile(filepath.Join(directory, role.ID.String()+".yaml"), description, 0o644); err != nil {
		return fmt.Errorf("write role %s: %w", role.Name, err)
	}
	folder := filepath.Join(directory, role.ID.String())
	if grid == nil {
		if err := os.RemoveAll(folder); err != nil {
			return fmt.Errorf("remove rights of role %s: %w", role.Name, err)
		}
		return nil
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return fmt.Errorf("create folder of role %s: %w", role.Name, err)
	}
	if err := os.WriteFile(filepath.Join(folder, RoleRightsFile), grid, 0o644); err != nil {
		return fmt.Errorf("write rights of role %s: %w", role.Name, err)
	}
	return nil
}

// DecodeRole reads a role from its description and its grid. A nil grid is a
// role without one, which says nothing about any object.
func DecodeRole(source string, description io.Reader, grid io.Reader, configuration project.Project) (RoleDefinition, error) {
	var value roleDescription
	if err := decodeStrict(source, description, &value); err != nil {
		return RoleDefinition{}, err
	}
	role := RoleDefinition{
		Format: value.Format, ID: value.ID, Name: value.Name, Title: value.Title, Comment: value.Comment,
		GrantNewObjectsByDefault: value.GrantNewObjectsByDefault, GrantNewFieldsByDefault: value.GrantNewFieldsByDefault,
		IndependentChildRights: value.IndependentChildRights, PolicyTemplates: value.PolicyTemplates,
	}
	if grid != nil {
		var rights roleGrid
		if err := decodeStrictJSON(source+" "+RoleRightsFile, grid, &rights); err != nil {
			return RoleDefinition{}, err
		}
		for _, object := range rights.Objects {
			role.Objects = append(role.Objects, ObjectPermission{Object: object.Object, Operations: object.Operations, Denied: object.Denied, Fields: object.Fields})
		}
		role.Commands = rights.Commands
	}
	// A restriction narrows a right the grid gives, so its object is one the
	// grid names; one naming an object the grid does not is a restriction of
	// nothing, and two for one object are one restriction split in two.
	restricted := make(map[uuid.UUID]bool, len(value.Restrictions))
	for index, restriction := range value.Restrictions {
		position := slices.IndexFunc(role.Objects, func(object ObjectPermission) bool { return object.Object == restriction.Object })
		if position < 0 || restricted[restriction.Object] || len(restriction.Policies) == 0 {
			return RoleDefinition{}, fmt.Errorf("%s: restrictions[%d] must name, once and with policies, an object the role's %s names", source, index, RoleRightsFile)
		}
		restricted[restriction.Object] = true
		role.Objects[position].Policies = restriction.Policies
	}
	if err := ValidateRole(source, role, configuration); err != nil {
		return RoleDefinition{}, err
	}
	return role, nil
}

// ValidateRole checks structure. Catalog validation additionally checks object
// and field ownership; publication validates every referenced form command.
func ValidateRole(source string, value RoleDefinition, configuration project.Project) error {
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	if !utf8.ValidString(value.Comment) {
		issues = append(issues, "comment must be valid UTF-8")
	}
	issues = append(issues, validatePolicyTemplates(value.PolicyTemplates)...)
	// Name to declared parameter count: a restriction must supply one field per
	// template parameter, so the arity travels with the name.
	templates := make(map[string]int, len(value.PolicyTemplates))
	for _, template := range value.PolicyTemplates {
		templates[template.Name] = len(template.Parameters)
	}
	objects := make(map[uuid.UUID]bool, len(value.Objects))
	for index, object := range value.Objects {
		prefix := fmt.Sprintf("objects[%d]", index)
		if object.Object.IsZero() || objects[object.Object] {
			issues = append(issues, prefix+".object must be a unique non-zero UUID")
		}
		objects[object.Object] = true
		issues = append(issues, validateRightStates(prefix, object.Operations, object.Denied, knownRights)...)
		issues = append(issues, validatePolicies(prefix+".policies", object.Policies, templates)...)
		fields := make(map[string]bool, len(object.Fields))
		for fieldIndex, field := range object.Fields {
			fieldPrefix := fmt.Sprintf("%s.fields[%d]", prefix, fieldIndex)
			if !validPermissionField(field.Field) {
				issues = append(issues, fieldPrefix+".field must be a canonical UUID or a lowercase standard field key")
			}
			if fields[field.Field] {
				issues = append(issues, fieldPrefix+".field must be unique within the object")
			}
			fields[field.Field] = true
			// A part the role says nothing about is simply not listed.
			if len(field.Operations) == 0 && len(field.Denied) == 0 {
				issues = append(issues, fieldPrefix+" must grant or deny at least one right")
			}
			issues = append(issues, validateRightStates(fieldPrefix, field.Operations, field.Denied, knownRights)...)
		}
	}
	commands := make(map[CommandPermission]bool, len(value.Commands))
	for index, command := range value.Commands {
		if command.Form.IsZero() || command.Command.IsZero() || commands[command] {
			issues = append(issues, fmt.Sprintf("commands[%d] must be a unique pair of non-zero form and command UUIDs", index))
		}
		commands[command] = true
	}
	return issuesError(source, value.Format, issues)
}

// validateRightStates checks that every right named is one the list may
// hold, named once, and either granted or denied - not both. A role's grid
// holds any of the prototype's rights; a restriction, only one this platform
// acts on.
func validateRightStates(path string, granted, denied []PermissionOperation, allowed map[PermissionOperation]bool) []string {
	var issues []string
	seen := make(map[PermissionOperation]bool, len(granted)+len(denied))
	for _, list := range []struct {
		name   string
		rights []PermissionOperation
	}{{"operations", granted}, {"denied", denied}} {
		for _, operation := range list.rights {
			if !allowed[operation] {
				issues = append(issues, fmt.Sprintf("%s.%s names %q, which is not a right it may hold", path, list.name, operation))
			} else if seen[operation] {
				issues = append(issues, fmt.Sprintf("%s.%s names %q a second time", path, list.name, operation))
			}
			seen[operation] = true
		}
	}
	return issues
}

func validPermissionField(field string) bool {
	if id, err := uuid.Parse(field); err == nil {
		return !id.IsZero() && id.String() == field
	}
	// No ceiling on the length: whether the key is one the object has is
	// decided against the object itself, and a key it does not have is
	// refused there however long it is. One dot at most: a field of a
	// standard table part is named after the part.
	part, field, nested := strings.Cut(field, ".")
	return standardKeyShape(part) && (!nested || standardKeyShape(field))
}

// standardKeyShape is a prototype name in lower case: letters, then digits
// where the name numbers its fields - extdimension1 is the first subconto of
// an accounting register (acc grants rights on it in its roles).
func standardKeyShape(key string) bool {
	if key == "" || key[0] < 'a' || key[0] > 'z' {
		return false
	}
	for _, character := range key {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

// decodeStrictJSON reads one JSON value and refuses any field the target does
// not have, and anything after the value.
func decodeStrictJSON(source string, reader io.Reader, target any) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", source, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode %s: more than one JSON value", source)
	}
	return nil
}

func (catalog *Catalog) Role(name string) (RoleDefinition, bool) {
	index, ok := catalog.roleByName[strings.ToLower(name)]
	if !ok {
		return RoleDefinition{}, false
	}
	return cloneRole(catalog.Roles[index]), true
}

func (catalog *Catalog) RoleByID(id uuid.UUID) (RoleDefinition, bool) {
	index, ok := catalog.roleByID[id]
	if !ok {
		return RoleDefinition{}, false
	}
	return cloneRole(catalog.Roles[index]), true
}

func cloneRole(value RoleDefinition) RoleDefinition {
	value.Title = cloneTitle(value.Title)
	value.Objects = slices.Clone(value.Objects)
	for index := range value.Objects {
		object := &value.Objects[index]
		object.Operations = slices.Clone(object.Operations)
		object.Denied = slices.Clone(object.Denied)
		object.Fields = slices.Clone(object.Fields)
		for field := range object.Fields {
			object.Fields[field].Operations = slices.Clone(object.Fields[field].Operations)
			object.Fields[field].Denied = slices.Clone(object.Fields[field].Denied)
		}
		object.Policies = clonePolicies(object.Policies)
	}
	value.Commands = slices.Clone(value.Commands)
	value.PolicyTemplates = clonePolicyTemplates(value.PolicyTemplates)
	return value
}

// GrantObjectReadByDefault adds PermissionRead for objectID if the role does
// not already grant any operation on it - used when GrantNewObjectsByDefault
// is set, right after a new metadata object is created. Reports whether it
// changed the role.
func (role *RoleDefinition) GrantObjectReadByDefault(objectID uuid.UUID) bool {
	for index := range role.Objects {
		if role.Objects[index].Object == objectID {
			if slices.Contains(role.Objects[index].Operations, PermissionRead) || slices.Contains(role.Objects[index].Denied, PermissionRead) {
				return false
			}
			role.Objects[index].Operations = append(role.Objects[index].Operations, PermissionRead)
			return true
		}
	}
	role.Objects = append(role.Objects, ObjectPermission{Object: objectID, Operations: []PermissionOperation{PermissionRead}})
	return true
}

// GrantFieldReadByDefault adds PermissionView for fieldKey on objectID, but
// only when the role already has an ObjectPermission entry for objectID -
// used when GrantNewFieldsByDefault is set, right after a new attribute or
// table part is added to an object. A role with no access to the object at
// all does not get access to one of its fields in isolation. Reports whether
// it changed the role.
func (role *RoleDefinition) GrantFieldReadByDefault(objectID uuid.UUID, fieldKey string) bool {
	for index := range role.Objects {
		if role.Objects[index].Object != objectID {
			continue
		}
		for fieldIndex := range role.Objects[index].Fields {
			if role.Objects[index].Fields[fieldIndex].Field == fieldKey {
				// A part the role already denies stays denied: the default is
				// for what nobody has decided, not over a decision.
				field := role.Objects[index].Fields[fieldIndex]
				if slices.Contains(field.Operations, PermissionView) || slices.Contains(field.Denied, PermissionView) {
					return false
				}
				role.Objects[index].Fields[fieldIndex].Operations = append(role.Objects[index].Fields[fieldIndex].Operations, PermissionView)
				return true
			}
		}
		role.Objects[index].Fields = append(role.Objects[index].Fields, FieldPermission{Field: fieldKey, Operations: []PermissionOperation{PermissionView}})
		return true
	}
	return false
}

// ObjectOperations lists every right an object permission can carry, in the
// order they are shown and compiled.
func ObjectOperations() []PermissionOperation { return slices.Clone(objectOperations) }
