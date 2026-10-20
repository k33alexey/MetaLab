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
// a role carries every one of them ML keeps on whatever it is given to,
// whether or not this platform acts on it yet; the rights ML does not keep
// are droppedRights.
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
	PermissionSaveUserData                               PermissionOperation = "save-user-data"
	PermissionStandardAuthenticationChange               PermissionOperation = "standard-authentication-change"
	PermissionSessionStandardAuthenticationChange        PermissionOperation = "session-standard-authentication-change"
	PermissionSessionOSAuthenticationChange              PermissionOperation = "session-os-authentication-change"
	PermissionInteractiveDeletePredefinedData            PermissionOperation = "interactive-delete-predefined-data"
	PermissionInteractiveSetDeletionMarkPredefinedData   PermissionOperation = "interactive-set-deletion-mark-predefined-data"
	PermissionInteractiveClearDeletionMarkPredefinedData PermissionOperation = "interactive-clear-deletion-mark-predefined-data"
	PermissionInteractiveDeleteMarkedPredefinedData      PermissionOperation = "interactive-delete-marked-predefined-data"
	PermissionReadDataHistory                            PermissionOperation = "read-data-history"
	PermissionViewDataHistory                            PermissionOperation = "view-data-history"
	PermissionReadDataHistoryOfMissingData               PermissionOperation = "read-data-history-of-missing-data"
	PermissionUpdateDataHistory                          PermissionOperation = "update-data-history"
	PermissionUpdateDataHistoryOfMissingData             PermissionOperation = "update-data-history-of-missing-data"
	PermissionUpdateDataHistorySettings                  PermissionOperation = "update-data-history-settings"
	PermissionUpdateDataHistoryVersionComment            PermissionOperation = "update-data-history-version-comment"
	PermissionEditDataHistoryVersionComment              PermissionOperation = "edit-data-history-version-comment"
	PermissionSwitchToDataHistoryVersion                 PermissionOperation = "switch-to-data-history-version"
	PermissionMainWindowModeNormal                       PermissionOperation = "main-window-mode-normal"
	PermissionMainWindowModeWorkplace                    PermissionOperation = "main-window-mode-workplace"
	PermissionMainWindowModeEmbeddedWorkplace            PermissionOperation = "main-window-mode-embedded-workplace"
	PermissionMainWindowModeFullscreenWorkplace          PermissionOperation = "main-window-mode-fullscreen-workplace"
	PermissionMainWindowModeKiosk                        PermissionOperation = "main-window-mode-kiosk"
	PermissionTechnicalSpecialistMode                    PermissionOperation = "technical-specialist-mode"
	PermissionExclusiveModeTerminationAtSessionStart     PermissionOperation = "exclusive-mode-termination-at-session-start"
	PermissionAnalyticsSystemClient                      PermissionOperation = "analytics-system-client"
)

// PermissionRunApplication is the one right of starting the application
// (ACCESS-RIGHTS, «Запуск клиента — одно право»): the prototype gives one a
// client, and ML has one client, the browser. It is ML's own right, named
// by no name of the prototype, so it stands apart from theirs.
const PermissionRunApplication PermissionOperation = "run-application"

// roleRight is one right under the names the prototype knows it by.
type roleRight struct {
	operation PermissionOperation
	english   string
	russian   string
}

// keptRight is a right ML keeps, with the caption the window of a role lists
// it under and the description shown under the tree for the right chosen in
// the list. The captions are the prototype's (configurator 8.3.27, every
// kind and the configuration, 20.10.2026, block-00/rol): most are the name in
// words, but the deletion mark is put «на удаление», the predefined drop
// «данных», the modes of the main window quote theirs, and two rights of an
// external source of data say more than their names. The descriptions are
// ML's own words; the window puts the name of the right in brackets under
// them, as the prototype does.
type keptRight struct {
	roleRight
	title       string
	description string
}

// roleRights is every right a role carries - those of the help of
// ПравоДоступа (8.3.27), but for those ML does not keep (droppedRights), and
// the start of the application where the prototype lists the thin client,
// the first of the clients. The four exports use sixty-eight of the
// prototype's rights: the one they do not, ending exclusive mode at the
// start of a session, the configurator writes in its own samples of 3.25
// and 3.27.
//
// The order is the prototype's, not the help's: the window of a role lists
// the rights of an object in it (block-00/rol), and the Rights.xml of every
// role of the exports writes them so, the 2526 roles never disagreeing.
// What the help orders otherwise is the configuration's rights, from the
// administration to the output, the use of a report or a data processor,
// before its viewing, and the viewing of the history of data, after the
// update of the comment of a version. A kind takes in this order the rights
// it offers (inPrototypeOrder).
var roleRights = []keptRight{
	{roleRight{PermissionRead, "Read", "Чтение"}, `Чтение`, `Право читать данные`},
	{roleRight{PermissionInsert, "Insert", "Добавление"}, `Добавление`, `Право добавлять данные`},
	{roleRight{PermissionUpdate, "Update", "Изменение"}, `Изменение`, `Право изменять данные`},
	{roleRight{PermissionDelete, "Delete", "Удаление"}, `Удаление`, `Право удалять данные`},
	{roleRight{PermissionPosting, "Posting", "Проведение"}, `Проведение`, `Право проводить документ`},
	{roleRight{PermissionUndoPosting, "UndoPosting", "ОтменаПроведения"}, `Отмена проведения`, `Право отменять проведение документа`},
	{roleRight{PermissionUse, "Use", "Использование"}, `Использование`, `Право использовать`},
	{roleRight{PermissionView, "View", "Просмотр"}, `Просмотр`, `Право видеть в интерфейсе`},
	{roleRight{PermissionInteractiveInsert, "InteractiveInsert", "ИнтерактивноеДобавление"}, `Интерактивное добавление`, `Право добавлять в интерфейсе`},
	{roleRight{PermissionEdit, "Edit", "Редактирование"}, `Редактирование`, `Право редактировать в интерфейсе`},
	{roleRight{PermissionInteractiveDelete, "InteractiveDelete", "ИнтерактивноеУдаление"}, `Интерактивное удаление`, `Право удалять в интерфейсе`},
	{roleRight{PermissionInteractiveSetDeletionMark, "InteractiveSetDeletionMark", "ИнтерактивнаяПометкаУдаления"}, `Интерактивная пометка на удаление`, `Право помечать на удаление в интерфейсе`},
	{roleRight{PermissionInteractiveClearDeletionMark, "InteractiveClearDeletionMark", "ИнтерактивноеСнятиеПометкиУдаления"}, `Интерактивное снятие пометки удаления`, `Право снимать пометку удаления в интерфейсе`},
	{roleRight{PermissionInteractiveDeleteMarked, "InteractiveDeleteMarked", "ИнтерактивноеУдалениеПомеченных"}, `Интерактивное удаление помеченных`, `Право удалять помеченные в интерфейсе`},
	{roleRight{PermissionInteractivePosting, "InteractivePosting", "ИнтерактивноеПроведение"}, `Интерактивное проведение`, `Право проводить документ в интерфейсе`},
	{roleRight{PermissionInteractivePostingRegular, "InteractivePostingRegular", "ИнтерактивноеПроведениеНеОперативное"}, `Интерактивное проведение неоперативное`, `Право проводить документ в интерфейсе в неоперативном режиме`},
	{roleRight{PermissionInteractiveUndoPosting, "InteractiveUndoPosting", "ИнтерактивнаяОтменаПроведения"}, `Интерактивная отмена проведения`, `Право отменять проведение документа в интерфейсе`},
	{roleRight{PermissionInteractiveChangeOfPosted, "InteractiveChangeOfPosted", "ИнтерактивноеИзменениеПроведенных"}, `Интерактивное изменение проведенных`, `Право изменять проведенные документы в интерфейсе`},
	{roleRight{PermissionInputByString, "InputByString", "ВводПоСтроке"}, `Ввод по строке`, `Право выбирать значение в поле ввода по набранной строке`},
	{roleRight{PermissionTotalsControl, "TotalsControl", "УправлениеИтогами"}, `Управление итогами`, `Право управлять итогами регистра`},
	{roleRight{PermissionAdministration, "Administration", "Администрирование"}, `Администрирование`, `Право выполнять административные функции`},
	{roleRight{PermissionDataAdministration, "DataAdministration", "АдминистрированиеДанных"}, `Администрирование данных`, `Право администрировать данные`},
	{roleRight{PermissionUpdateDataBaseConfiguration, "UpdateDataBaseConfiguration", "ОбновлениеКонфигурацииБазыДанных"}, `Обновление конфигурации базы данных`, `Право обновлять конфигурацию базы данных`},
	{roleRight{PermissionExclusiveMode, "ExclusiveMode", "МонопольныйРежим"}, `Монопольный режим`, `Право устанавливать монопольный режим`},
	{roleRight{PermissionActiveUsers, "ActiveUsers", "АктивныеПользователи"}, `Активные пользователи`, `Право получать список активных пользователей`},
	{roleRight{PermissionEventLog, "EventLog", "ЖурналРегистрации"}, `Журнал регистрации`, `Право просматривать журнал регистрации`},
	{roleRight{PermissionRunApplication, "RunApplication", "ЗапускПриложения"}, `Запуск приложения`, `Право запускать приложение`},
	{roleRight{PermissionExternalConnection, "ExternalConnection", "ВнешнееСоединение"}, `Внешнее соединение`, `Право подключаться внешним соединением`},
	{roleRight{PermissionAutomation, "Automation", "Automation"}, `Automation`, `Право подключаться через Automation`},
	{roleRight{PermissionTechnicalSpecialistMode, "TechnicalSpecialistMode", "РежимТехническогоСпециалиста"}, `Режим технического специалиста`, `Право на функции технического специалиста`},
	{roleRight{PermissionMainWindowModeNormal, "MainWindowModeNormal", "РежимОсновногоОкнаОбычный"}, `Режим основного окна "Обычный"`, `Право запускать приложение с основным окном в режиме "Обычный"`},
	{roleRight{PermissionMainWindowModeWorkplace, "MainWindowModeWorkplace", "РежимОсновногоОкнаРабочееМесто"}, `Режим основного окна "Рабочее место"`, `Право запускать приложение с основным окном в режиме "Рабочее место"`},
	{roleRight{PermissionMainWindowModeEmbeddedWorkplace, "MainWindowModeEmbeddedWorkplace", "РежимОсновногоОкнаВстроенноеРабочееМесто"}, `Режим основного окна "Встроенное рабочее место"`, `Право запускать приложение с основным окном в режиме "Встроенное рабочее место"`},
	{roleRight{PermissionMainWindowModeFullscreenWorkplace, "MainWindowModeFullscreenWorkplace", "РежимОсновногоОкнаПолноэкранноеРабочееМесто"}, `Режим основного окна "Полноэкранное рабочее место"`, `Право запускать приложение с основным окном в режиме "Полноэкранное рабочее место"`},
	{roleRight{PermissionMainWindowModeKiosk, "MainWindowModeKiosk", "РежимОсновногоОкнаКиоск"}, `Режим основного окна "Киоск"`, `Право запускать приложение с основным окном в режиме "Киоск"`},
	{roleRight{PermissionAnalyticsSystemClient, "AnalyticsSystemClient", "КлиентСистемыАналитики"}, `Клиент системы аналитики`, `Право запускать клиент системы аналитики`},
	{roleRight{PermissionExclusiveModeTerminationAtSessionStart, "ExclusiveModeTerminationAtSessionStart", "ЗавершениеМонопольногоРежимаПриНачалеСеанса"}, `Завершение монопольного режима при начале сеанса`, `Право, начиная сеанс, завершить сеанс, установивший монопольный режим`},
	{roleRight{PermissionSaveUserData, "SaveUserData", "СохранениеДанныхПользователя"}, `Сохранение данных пользователя`, `Право сохранять данные пользователя`},
	{roleRight{PermissionInteractiveOpenExtDataProcessors, "InteractiveOpenExtDataProcessors", "ИнтерактивноеОткрытиеВнешнихОбработок"}, `Интерактивное открытие внешних обработок`, `Право открывать внешние обработки в интерфейсе`},
	{roleRight{PermissionInteractiveOpenExtReports, "InteractiveOpenExtReports", "ИнтерактивноеОткрытиеВнешнихОтчетов"}, `Интерактивное открытие внешних отчетов`, `Право открывать внешние отчеты в интерфейсе`},
	{roleRight{PermissionOutput, "Output", "Вывод"}, `Вывод`, `Право печатать, сохранять в файл и копировать в буфер обмена`},
	{roleRight{PermissionGet, "Get", "Получение"}, `Получение`, `Право получать значение`},
	{roleRight{PermissionSet, "Set", "Установка"}, `Установка`, `Право устанавливать значение`},
	{roleRight{PermissionInteractiveActivate, "InteractiveActivate", "ИнтерактивнаяАктивация"}, `Интерактивная активация`, `Право активировать задачу в интерфейсе`},
	{roleRight{PermissionStart, "Start", "Старт"}, `Старт`, `Право стартовать бизнес-процесс`},
	{roleRight{PermissionInteractiveStart, "InteractiveStart", "ИнтерактивныйСтарт"}, `Интерактивный старт`, `Право стартовать бизнес-процесс в интерфейсе`},
	{roleRight{PermissionExecute, "Execute", "Выполнение"}, `Выполнение`, `Право выполнять задачу`},
	{roleRight{PermissionInteractiveExecute, "InteractiveExecute", "ИнтерактивноеВыполнение"}, `Интерактивное выполнение`, `Право выполнять задачу в интерфейсе`},
	{roleRight{PermissionStandardAuthenticationChange, "StandardAuthenticationChange", "ИзменениеСтандартнойАутентификации"}, `Изменение стандартной аутентификации текущего пользователя`, `Право менять стандартную аутентификацию текущего пользователя`},
	{roleRight{PermissionSessionStandardAuthenticationChange, "SessionStandardAuthenticationChange", "ИзменениеСтандартнойАутентификацииСеанса"}, `Изменение стандартной аутентификации сеанса`, `Право менять стандартную аутентификацию сеанса`},
	{roleRight{PermissionSessionOSAuthenticationChange, "SessionOSAuthenticationChange", "ИзменениеАутентификацииОССеанса"}, `Изменение аутентификации операционной системы сеанса`, `Право менять аутентификацию операционной системы сеанса`},
	{roleRight{PermissionInteractiveDeletePredefinedData, "InteractiveDeletePredefinedData", "ИнтерактивноеУдалениеПредопределенныхДанных"}, `Интерактивное удаление предопределенных`, `Право удалять предопределенные в интерфейсе`},
	{roleRight{PermissionInteractiveSetDeletionMarkPredefinedData, "InteractiveSetDeletionMarkPredefinedData", "ИнтерактивнаяПометкаУдаленияПредопределенныхДанных"}, `Интерактивная пометка на удаление предопределенных`, `Право помечать предопределенные на удаление в интерфейсе`},
	{roleRight{PermissionInteractiveClearDeletionMarkPredefinedData, "InteractiveClearDeletionMarkPredefinedData", "ИнтерактивноеСнятиеПометкиУдаленияПредопределенныхДанных"}, `Интерактивное снятие пометки удаления предопределенных`, `Право снимать пометку удаления с предопределенных в интерфейсе`},
	{roleRight{PermissionInteractiveDeleteMarkedPredefinedData, "InteractiveDeleteMarkedPredefinedData", "ИнтерактивноеУдалениеПомеченныхПредопределенныхДанных"}, `Интерактивное удаление помеченных предопределенных`, `Право удалять помеченные предопределенные в интерфейсе`},
	{roleRight{PermissionReadDataHistory, "ReadDataHistory", "ЧтениеИсторииДанных"}, `Чтение истории данных`, `Право читать историю данных`},
	{roleRight{PermissionReadDataHistoryOfMissingData, "ReadDataHistoryOfMissingData", "ЧтениеИсторииДанныхОтсутствующихДанных"}, `Чтение истории данных отсутствующих данных`, `Право читать историю данных, которых уже нет`},
	{roleRight{PermissionUpdateDataHistory, "UpdateDataHistory", "ИзменениеИсторииДанных"}, `Изменение истории данных`, `Право изменять историю данных`},
	{roleRight{PermissionUpdateDataHistoryOfMissingData, "UpdateDataHistoryOfMissingData", "ИзменениеИсторииДанныхОтсутствующихДанных"}, `Изменение истории данных отсутствующих данных`, `Право изменять историю данных, которых уже нет`},
	{roleRight{PermissionUpdateDataHistorySettings, "UpdateDataHistorySettings", "ИзменениеНастроекИсторииДанных"}, `Изменение настроек истории данных`, `Право изменять настройки истории данных`},
	{roleRight{PermissionUpdateDataHistoryVersionComment, "UpdateDataHistoryVersionComment", "ИзменениеКомментарияВерсииИсторииДанных"}, `Изменение комментария версии истории данных`, `Право изменять комментарий версии истории данных`},
	{roleRight{PermissionViewDataHistory, "ViewDataHistory", "ПросмотрИсторииДанных"}, `Просмотр истории данных`, `Право просматривать историю данных`},
	{roleRight{PermissionEditDataHistoryVersionComment, "EditDataHistoryVersionComment", "РедактированиеКомментарияВерсииИсторииДанных"}, `Редактирование комментария версии истории данных`, `Право редактировать комментарий версии истории данных в интерфейсе`},
	{roleRight{PermissionSwitchToDataHistoryVersion, "SwitchToDataHistoryVersion", "ПереходНаВерсиюИсторииДанных"}, `Переход на версию истории данных`, `Право переходить на версию истории данных`},
}

// droppedRight is a right of the prototype ML does not keep, under the names
// the prototype knows it by, with the right that stands for it in ML - none
// when nothing does - and why.
type droppedRight struct {
	roleRight
	instead PermissionOperation
	why     string
}

// droppedRights are the rights of the prototype ML does not keep: the thin
// and the web client are «Запуск приложения» in ML, the thick and the mobile
// client ML does not have (ACCESS-RIGHTS, «Запуск клиента — одно право»),
// and nothing stands for a right on a mechanism ML refused (ACCESS-RIGHTS,
// «Права на механизмы, от которых ML отказался, не переносятся»). Taking a
// right away later is one more line here. A role
// naming one is read for Studio to repair, and refused by every load that
// runs it: the project is not saved into a database, an application whose
// configuration gives one does not open (ACCESS-RIGHTS, «Роли, сохранённые
// прежней версией ML»).
var droppedRights = []droppedRight{
	{roleRight{"thin-client", "ThinClient", "ТонкийКлиент"}, PermissionRunApplication, "the thin and the web client are the one right run-application in ML"},
	{roleRight{"web-client", "WebClient", "ВебКлиент"}, PermissionRunApplication, "the thin and the web client are the one right run-application in ML"},
	{roleRight{"thick-client", "ThickClient", "ТолстыйКлиент"}, "", "ML has no thick client"},
	{roleRight{"mobile-client", "MobileClient", "МобильныйКлиент"}, "", "ML has no mobile client"},
	{roleRight{"configuration-extensions-administration", "ConfigurationExtensionsAdministration", "АдминистрированиеРасширенийКонфигурации"}, "",
		"ML has no configuration extensions"},
	{roleRight{"collaboration-system-info-base-registration", "CollaborationSystemInfoBaseRegistration", "РегистрацияИнформационнойБазыСистемыВзаимодействия"}, "",
		"ML has no collaboration system"},
}

// droppedRightOf finds a right ML does not keep by the name a role keeps it
// under.
func droppedRightOf(operation PermissionOperation) (droppedRight, bool) {
	for _, right := range droppedRights {
		if right.operation == operation {
			return right, true
		}
	}
	return droppedRight{}, false
}

// DroppedRights answers, by the name a role keeps it under, every right ML
// does not keep with the one standing for it, "" for none: what Studio's
// «Убрать недоступные права» puts in its place.
func DroppedRights() map[PermissionOperation]PermissionOperation {
	result := make(map[PermissionOperation]PermissionOperation, len(droppedRights))
	for _, right := range droppedRights {
		result[right.operation] = right.instead
	}
	return result
}

// renamedRights are the names a right was known by before. The help says the
// mode of the technical specialist was called the mode of all functions
// before 8.3.16, and a configuration kept in compatibility with an earlier
// version writes it under the old name and never under the new one.
var renamedRights = map[string]PermissionOperation{"AllFunctionsMode": PermissionTechnicalSpecialistMode}

// knownRights answers by name what roleRights lists in order.
var knownRights = func() map[PermissionOperation]bool {
	known := make(map[PermissionOperation]bool, len(roleRights))
	for _, right := range roleRights {
		known[right.operation] = true
	}
	return known
}()

// rightPrerequisites are, by right, the rights the window of a role marks
// together with it, and the right it clears together with any of them
// (pass of 20.10.2026 with the owner, block-00/rol «Права зависят друг от
// друга», materials/configurator-2026-10-20/prava/zapis.md «Карта
// зависимостей прав», snapshots prava/20-25 and 33-41): only the rights
// right before it, the window following them through - an interactive
// right needs its plain one and the editing, the editing needs the update
// and the viewing, which need the reading. One map for every kind: a node
// takes, of what a right needs through it, the rights the node has
// (rightsRequired) - the viewing needs the reading of a catalog and the use
// of a report, a part's editing needs its viewing alone, and the
// configuration's administration needs the administration of data. A kind
// may leave a prerequisite out (kindPrerequisitesNotTakenOf). The kinds the
// pass did not open take the map by the owner's word of 20.10.2026: a kind
// with the rights of one it opened behaves as that one, a sequence and a
// recalculation need the reading for the update, a table of an external
// source the catalog's, the source itself none. The help names no such
// tie of rights at run time (ПравоДоступа, «Редактирование роли»): the
// window keeps a role whole, and the platform checks each right as given.
var rightPrerequisites = map[PermissionOperation][]PermissionOperation{
	PermissionAdministration: {PermissionDataAdministration},
	PermissionInsert:         {PermissionRead}, PermissionUpdate: {PermissionRead}, PermissionDelete: {PermissionRead},
	PermissionView:              {PermissionRead, PermissionUse},
	PermissionEdit:              {PermissionUpdate, PermissionView},
	PermissionInteractiveInsert: {PermissionInsert, PermissionEdit},
	PermissionInteractiveDelete: {PermissionDelete, PermissionEdit}, PermissionInteractiveDeleteMarked: {PermissionDelete, PermissionEdit},
	PermissionInteractiveSetDeletionMark: {PermissionEdit}, PermissionInteractiveClearDeletionMark: {PermissionEdit},
	PermissionInputByString:                              {PermissionView},
	PermissionInteractiveDeletePredefinedData:            {PermissionInteractiveDelete},
	PermissionInteractiveSetDeletionMarkPredefinedData:   {PermissionInteractiveSetDeletionMark},
	PermissionInteractiveClearDeletionMarkPredefinedData: {PermissionInteractiveClearDeletionMark},
	PermissionInteractiveDeleteMarkedPredefinedData:      {PermissionInteractiveDeleteMarked},
	PermissionPosting:                                    {PermissionUpdate}, PermissionUndoPosting: {PermissionUpdate},
	PermissionInteractivePosting:        {PermissionPosting, PermissionEdit},
	PermissionInteractivePostingRegular: {PermissionInteractivePosting},
	PermissionInteractiveUndoPosting:    {PermissionUndoPosting, PermissionEdit},
	PermissionInteractiveChangeOfPosted: {PermissionEdit},
	PermissionInteractiveActivate:       {PermissionUpdate},
	PermissionStart:                     {PermissionUpdate}, PermissionInteractiveStart: {PermissionStart},
	PermissionExecute: {PermissionUpdate}, PermissionInteractiveExecute: {PermissionExecute},
	PermissionReadDataHistory:                 {PermissionRead},
	PermissionReadDataHistoryOfMissingData:    {PermissionReadDataHistory},
	PermissionUpdateDataHistory:               {PermissionReadDataHistory},
	PermissionUpdateDataHistoryOfMissingData:  {PermissionUpdateDataHistory, PermissionReadDataHistoryOfMissingData},
	PermissionUpdateDataHistoryVersionComment: {PermissionReadDataHistory},
	PermissionViewDataHistory:                 {PermissionView, PermissionReadDataHistory},
	PermissionEditDataHistoryVersionComment:   {PermissionView, PermissionUpdateDataHistoryVersionComment},
	PermissionSwitchToDataHistoryVersion:      {PermissionView},
}

// rightsRequired is every right the window of a role marks together with
// operation on an object of the kind, "" for the configuration, in the
// prototype's order: rightPrerequisites followed through, but for what the
// kind leaves out. A node takes of them the rights it has; the window
// clears operation with any of them.
func rightsRequired(kind Kind, operation PermissionOperation) []PermissionOperation {
	notTaken := kindPrerequisitesNotTakenOf[kind]
	seen := map[PermissionOperation]bool{}
	var follow func(PermissionOperation)
	follow = func(right PermissionOperation) {
		for _, required := range rightPrerequisites[right] {
			if !seen[required] && !slices.Contains(notTaken[right], required) {
				seen[required] = true
				follow(required)
			}
		}
	}
	follow(operation)
	result := make([]PermissionOperation, 0, len(seen))
	for right := range seen {
		result = append(result, right)
	}
	return inPrototypeOrder(result)
}

// rightIndex is the place of every right in the prototype's order.
var rightIndex = func() map[PermissionOperation]int {
	result := make(map[PermissionOperation]int, len(roleRights))
	for index, right := range roleRights {
		result[right.operation] = index
	}
	return result
}()

// RoleRights lists every right a role can carry, in the prototype's order.
func RoleRights() []PermissionOperation {
	result := make([]PermissionOperation, len(roleRights))
	for index, right := range roleRights {
		result[index] = right.operation
	}
	return result
}

// RightByPrototypeName finds a right by either of the names the prototype
// writes it under, or by the one it was called before, case aside. A right
// ML does not keep is found as the one standing for it - the thin and the
// web client as the start of the application - and not at all when none
// does.
func RightByPrototypeName(name string) (PermissionOperation, bool) {
	for old, operation := range renamedRights {
		if strings.EqualFold(old, name) {
			return operation, true
		}
	}
	for _, right := range droppedRights {
		if strings.EqualFold(right.english, name) || strings.EqualFold(right.russian, name) {
			return right.instead, right.instead != ""
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
// Omitted objects and fields grant nothing. Object operations do not
// implicitly grant fields; a right on a command of an object does not grant
// the operations it runs.
//
// On disk a role is two files. The description - the name, the flags, the
// restrictions and their templates - is YAML, as everything else is; the grid
// of rights is RoleRightsFile, JSON, in the folder named by the role's
// identifier. The grid is the largest content of a configuration after the
// spreadsheets: acc keeps 20 MiB of it, read from YAML in 1.5 s and 459 MiB,
// from JSON in 0.17 s and 68 MiB (11.10.2026, all roles of the four exports).
type RoleDefinition struct {
	Format  int                `yaml:"format" json:"format"`
	ID      uuid.UUID          `yaml:"id" json:"id"`
	Name    string             `yaml:"name" json:"name"`
	Title   LocalizedText      `yaml:"title" json:"title"`
	Comment string             `yaml:"comment,omitempty" json:"comment,omitempty"`
	Objects []ObjectPermission `yaml:"-" json:"objects,omitempty"`
	// Commands are the commands of forms a role saved by an earlier ML gave.
	// A role of the prototype gives no right on a command of a form: whom it
	// is available to is the command's own use (ManagedFormCommand.Use), and
	// the role's window offers rights on the commands of an object alone
	// (help, «Редактирование роли»; no export writes one). They are read so
	// that Studio opens such a role and takes them away, and refused by every
	// load that runs the role (validateDroppedRights; ACCESS-RIGHTS, «Роли,
	// сохранённые прежней версией ML»).
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
	// RestrictionTemplates are the templates the conditions of this role's
	// restrictions take text from. They grant nothing by themselves.
	RestrictionTemplates []RestrictionTemplate `yaml:"restriction_templates,omitempty" json:"restrictionTemplates,omitempty"`
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
	// Restrictions are the restrictions of access to data on the rights the
	// role writes here. They never widen access.
	Restrictions []RightRestriction `yaml:"restrictions,omitempty" json:"restrictions,omitempty"`
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

// CommandPermission is a command of a form a role saved by an earlier ML gave,
// by the form and the command (RoleDefinition.Commands).
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
	Format                   int                   `yaml:"format"`
	ID                       uuid.UUID             `yaml:"id"`
	Name                     string                `yaml:"name"`
	Title                    LocalizedText         `yaml:"title"`
	Comment                  string                `yaml:"comment,omitempty"`
	GrantNewObjectsByDefault bool                  `yaml:"grant_new_objects_by_default,omitempty"`
	GrantNewFieldsByDefault  bool                  `yaml:"grant_new_fields_by_default,omitempty"`
	IndependentChildRights   bool                  `yaml:"independent_child_rights,omitempty"`
	RestrictionTemplates     []RestrictionTemplate `yaml:"restriction_templates,omitempty"`
	Restrictions             []roleRestriction     `yaml:"restrictions,omitempty"`
}

type roleRestriction struct {
	Object uuid.UUID          `yaml:"object"`
	Rights []RightRestriction `yaml:"rights"`
}

// roleGrid is the JSON half: every object the role says anything about, in
// the role's own order, and the commands of forms an earlier ML gave.
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
		IndependentChildRights: role.IndependentChildRights, RestrictionTemplates: role.RestrictionTemplates,
	}
	var rights roleGrid
	for _, object := range role.Objects {
		rights.Objects = append(rights.Objects, roleGridObject{Object: object.Object, Operations: object.Operations, Denied: object.Denied, Fields: object.Fields})
		if len(object.Restrictions) > 0 {
			value.Restrictions = append(value.Restrictions, roleRestriction{Object: object.Object, Rights: object.Restrictions})
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
		IndependentChildRights: value.IndependentChildRights, RestrictionTemplates: value.RestrictionTemplates,
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
		if position < 0 || restricted[restriction.Object] || len(restriction.Rights) == 0 {
			return RoleDefinition{}, fmt.Errorf("%s: restrictions[%d] must name, once and with rights, an object the role's %s names", source, index, RoleRightsFile)
		}
		restricted[restriction.Object] = true
		role.Objects[position].Restrictions = restriction.Rights
	}
	if err := ValidateRole(source, role, configuration); err != nil {
		return RoleDefinition{}, err
	}
	return role, nil
}

// ValidateRole checks structure. Catalog validation additionally checks object
// and field ownership, and refuses what ML no longer keeps.
func ValidateRole(source string, value RoleDefinition, configuration project.Project) error {
	return checkDescription(source, value, configuration, validateRole)
}

// validateRole checks what a role adds to the head every description has.
func validateRole(value RoleDefinition, configuration project.Project) []string {
	var issues []string
	if !utf8.ValidString(value.Comment) {
		issues = append(issues, "comment must be valid UTF-8")
	}
	issues = append(issues, validateRestrictionTemplates(value.RestrictionTemplates)...)
	objects := make(map[uuid.UUID]bool, len(value.Objects))
	for index, object := range value.Objects {
		prefix := fmt.Sprintf("objects[%d]", index)
		if object.Object.IsZero() || objects[object.Object] {
			issues = append(issues, prefix+".object must be a unique non-zero UUID")
		}
		objects[object.Object] = true
		issues = append(issues, validateRightStates(prefix, object.Operations, object.Denied)...)
		issues = append(issues, validateRestrictions(prefix+".restrictions", object)...)
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
			issues = append(issues, validateRightStates(fieldPrefix, field.Operations, field.Denied)...)
		}
	}
	commands := make(map[CommandPermission]bool, len(value.Commands))
	for index, command := range value.Commands {
		if command.Form.IsZero() || command.Command.IsZero() || commands[command] {
			issues = append(issues, fmt.Sprintf("commands[%d] must be a unique pair of non-zero form and command UUIDs", index))
		}
		commands[command] = true
	}
	return issues
}

// validateRightStates checks that every right named is one a role carries,
// or one ML no longer keeps, which the load that runs the role refuses
// (validateDroppedRights) and Studio reads to repair; named once, and either
// granted or denied - not both.
func validateRightStates(path string, granted, denied []PermissionOperation) []string {
	var issues []string
	seen := make(map[PermissionOperation]bool, len(granted)+len(denied))
	for _, list := range []struct {
		name   string
		rights []PermissionOperation
	}{{"operations", granted}, {"denied", denied}} {
		for _, operation := range list.rights {
			if _, dropped := droppedRightOf(operation); !knownRights[operation] && !dropped {
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
		object.Restrictions = cloneRestrictions(object.Restrictions)
	}
	value.Commands = slices.Clone(value.Commands)
	value.RestrictionTemplates = slices.Clone(value.RestrictionTemplates)
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
