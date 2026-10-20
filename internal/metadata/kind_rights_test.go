package metadata

import (
	"encoding/xml"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/kinds"
)

// expectedKindRights is what the configurator writes on an object of each
// kind, by the prototype's names, from the Rights.xml of every role of the
// four exports checked, demo-base, energy, ssl and the configurator's samples
// in mdclasses (survey of 12.10.2026), in the order every one of them writes
// them, the order of the window of a role (survey of 20.10.2026). Written out
// here, not read from kindRightsOf or roleRights.
var expectedKindRights = func() map[Kind][]string {
	history := []string{"ReadDataHistory", "ReadDataHistoryOfMissingData", "UpdateDataHistory",
		"UpdateDataHistoryOfMissingData", "UpdateDataHistorySettings", "UpdateDataHistoryVersionComment",
		"ViewDataHistory", "EditDataHistoryVersionComment", "SwitchToDataHistoryVersion"}
	// A constant always has its value: no history of missing data.
	constantHistory := []string{"ReadDataHistory", "UpdateDataHistory", "UpdateDataHistorySettings",
		"UpdateDataHistoryVersionComment", "ViewDataHistory", "EditDataHistoryVersionComment",
		"SwitchToDataHistoryVersion"}
	reference := []string{"Read", "Insert", "Update", "Delete", "View", "InteractiveInsert", "Edit",
		"InteractiveDelete", "InteractiveSetDeletionMark", "InteractiveClearDeletionMark", "InteractiveDeleteMarked",
		"InputByString"}
	predefined := []string{"InteractiveDeletePredefinedData", "InteractiveSetDeletionMarkPredefinedData",
		"InteractiveClearDeletionMarkPredefinedData", "InteractiveDeleteMarkedPredefinedData"}
	chart := slices.Concat(reference, predefined, history)
	return map[Kind][]string{
		SubsystemKind: {"View"}, SessionParameterKind: {"Get", "Set"}, CommonAttributeKind: {"View", "Edit"},
		ExchangePlanKind: slices.Concat(reference, history), FilterCriterionKind: {"View"},
		CommonCommandKind: {"View"}, CommonFormKind: {"View"}, WebServiceKind: {}, HTTPServiceKind: {},
		ConstantKind: slices.Concat([]string{"Read", "Update", "View", "Edit"}, constantHistory),
		CatalogKind:  chart,
		DocumentKind: slices.Concat([]string{"Read", "Insert", "Update", "Delete", "Posting", "UndoPosting", "View",
			"InteractiveInsert", "Edit", "InteractiveDelete", "InteractiveSetDeletionMark", "InteractiveClearDeletionMark",
			"InteractiveDeleteMarked", "InteractivePosting", "InteractivePostingRegular", "InteractiveUndoPosting",
			"InteractiveChangeOfPosted", "InputByString"}, history),
		SequenceKind: {"Read", "Update"}, DocumentJournalKind: {"Read", "View"},
		ReportKind: {"Use", "View"}, DataProcessorKind: {"Use", "View"},
		ChartOfCharacteristicTypesKind: chart, ChartOfAccountsKind: chart, ChartOfCalculationTypesKind: chart,
		InformationRegisterKind:  slices.Concat([]string{"Read", "Update", "View", "Edit", "TotalsControl"}, history),
		AccumulationRegisterKind: {"Read", "Update", "View", "Edit", "TotalsControl"},
		AccountingRegisterKind:   {"Read", "Update", "View", "Edit", "TotalsControl"},
		CalculationRegisterKind:  {"Read", "Update", "View", "Edit"},
		RecalculationKind:        {"Read", "Update"},
		BusinessProcessKind: slices.Concat(reference, []string{"InteractiveActivate", "Start", "InteractiveStart"},
			history),
		TaskKind: slices.Concat(reference, []string{"InteractiveActivate", "Execute", "InteractiveExecute"},
			history),
		ExternalDataSourceKind: {"Use", "Administration", "StandardAuthenticationChange",
			"SessionStandardAuthenticationChange", "SessionOSAuthenticationChange"},
		ExternalDataSourceTableKind: {"Read", "Insert", "Update", "Delete", "View", "InteractiveInsert", "Edit",
			"InteractiveDelete", "InputByString"},
		ExternalCubeKind: {"Read", "View"}, ExternalDimensionTableKind: {"Read", "View"},
	}
}()

// expectedPartRights is what the same exports write on a part of an object,
// by the class the prototype names the part with (survey of 12.10.2026).
// Written out here, not read from partGroups.
var expectedPartRights = map[string][]string{
	"StandardAttribute": {"View", "Edit"}, "Attribute": {"View", "Edit"}, "Dimension": {"View", "Edit"},
	"Resource": {"View", "Edit"}, "AccountingFlag": {"View", "Edit"}, "ExtDimensionAccountingFlag": {"View", "Edit"},
	"AddressingAttribute": {"View", "Edit"}, "TabularSection": {"View", "Edit"},
	"StandardTabularSection": {"View", "Edit"}, "Field": {"View", "Edit"}, "Command": {"View"},
	"Operation": {"Use"}, "Method": {"Use"},
}

// TestEveryPartOffersThePrototypesRights catches a sort of part offering a
// right the configurator does not write on such a part, or missing one it
// does: a command with Edit, an attribute without it, an operation of a web
// service with View. A sort of part the exports never give rights on - the
// template of an HTTP service - offers none.
func TestEveryPartOffersThePrototypesRights(t *testing.T) {
	t.Parallel()
	for _, item := range partGroups {
		want := []PermissionOperation{}
		if names, ok := expectedPartRights[item.prototype]; ok {
			want = rightsByNames(t, names)
		}
		got := partRightsOf(item.group)
		if got == nil {
			got = []PermissionOperation{}
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s offers %v, the prototype %v", item.prototype, got, want)
		}
	}
	for class := range expectedPartRights {
		if !slices.ContainsFunc(partGroups, func(item partGroupInfo) bool { return item.prototype == class }) {
			t.Errorf("no sort of part is %s", class)
		}
	}
}

// TestEveryKindSaysWhatItHolds catches a kind of the model left out of
// kindCompositionOf: an object of it would hold nothing a role, a functional
// option or a criterion could name, and nothing would say so.
func TestEveryKindSaysWhatItHolds(t *testing.T) {
	t.Parallel()
	for _, item := range kindInfos {
		if _, ok := kindCompositionOf[item.kind]; !ok {
			t.Errorf("%s says nothing of what its objects hold", item.kind)
		}
	}
	for kind := range kindCompositionOf {
		if _, ok := kindInfoOf(kind); !ok {
			t.Errorf("%s is no kind of the model", kind)
		}
	}
}

// rightsSeenInTheWindow are the rights the window of a role offers on an
// object of a kind no export gives rights on (pass of 20.10.2026 with the
// owner, block-00/rol, «Права частей внешнего источника»: a function of a
// source, «Использование» and «Просмотр»). Written out here, not read off
// kindRightsOf.
var rightsSeenInTheWindow = map[Kind][]string{ExternalFunctionKind: {"Use", "View"}}

// expectedConfigurationRights is what the same exports write on the
// configuration itself, in their order, the window's (block-00/rol/03…06).
var expectedConfigurationRights = []string{"Administration", "DataAdministration", "UpdateDataBaseConfiguration",
	"ExclusiveMode", "ActiveUsers", "EventLog", "ThinClient", "WebClient", "MobileClient", "ThickClient",
	"ExternalConnection", "Automation", "TechnicalSpecialistMode", "CollaborationSystemInfoBaseRegistration",
	"MainWindowModeNormal", "MainWindowModeWorkplace", "MainWindowModeEmbeddedWorkplace",
	"MainWindowModeFullscreenWorkplace", "MainWindowModeKiosk", "AnalyticsSystemClient",
	"ExclusiveModeTerminationAtSessionStart", "SaveUserData", "ConfigurationExtensionsAdministration",
	"InteractiveOpenExtDataProcessors", "InteractiveOpenExtReports", "Output"}

// rightsNotKept are the rights of the prototype nothing stands for in ML: its
// thick and mobile clients (ACCESS-RIGHTS, «Запуск клиента — одно право»),
// the rights on the mechanisms ML refused (ACCESS-RIGHTS, «Права на
// механизмы, от которых ML отказался, не переносятся»), and by the owner's
// decisions of 20.10.2026 the rights of the configuration that would change
// nothing in ML, the rights of the history of data and those of
// authentication on an external source. The thin and the web client are the
// start of the application, the mode of the technical specialist the
// developer tools. The interactive opening of external data processors and
// reports is kept again: the mechanism comes back as ML's own (the owner's
// decision of 20.10.2026).
var rightsNotKept = []string{"ThickClient", "MobileClient", "ConfigurationExtensionsAdministration", "CollaborationSystemInfoBaseRegistration",
	"ExternalConnection", "Automation", "MainWindowModeNormal", "MainWindowModeWorkplace", "MainWindowModeEmbeddedWorkplace",
	"MainWindowModeFullscreenWorkplace", "MainWindowModeKiosk", "AnalyticsSystemClient",
	"ReadDataHistory", "ReadDataHistoryOfMissingData", "UpdateDataHistory", "UpdateDataHistoryOfMissingData",
	"UpdateDataHistorySettings", "UpdateDataHistoryVersionComment", "ViewDataHistory", "EditDataHistoryVersionComment",
	"SwitchToDataHistoryVersion",
	"StandardAuthenticationChange", "SessionStandardAuthenticationChange", "SessionOSAuthenticationChange"}

// mlConfigurationRights are ML's own rights of the configuration no right of
// the prototype stands for, which no export writes: the change history,
// right after the developer tools (ACCESS-RIGHTS, «Своих прав у ML три»).
var mlConfigurationRights = []PermissionOperation{"change-history"}

// rightsByNames reads prototype names as rights, in their order, each once:
// the thin and the web client are one right in ML, standing where the thin
// client does, and the rights ML does not keep are no right of it.
func rightsByNames(t *testing.T, names []string) []PermissionOperation {
	t.Helper()
	result := []PermissionOperation{}
	for _, name := range names {
		if slices.Contains(rightsNotKept, name) {
			continue
		}
		right, ok := RightByPrototypeName(name)
		if !ok {
			t.Fatalf("%s is no right of the prototype", name)
		}
		if !slices.Contains(result, right) {
			result = append(result, right)
		}
	}
	return result
}

// TestEveryKindOffersThePrototypesRights catches the role editor offering a
// right the configurator does not offer on an object of the kind, or missing
// one it does: a catalog without the rights on its predefined data, a
// constant with input by string, a register without the management of its
// totals, a kind a role may name with no list at all, or a kind no export
// gives rights on offered some. It also catches a right a role carries the editor
// offers nowhere, and rights out of the prototype's order: the history of
// data viewed second, as the help lists it, or the modes of the main window
// after the output.
func TestEveryKindOffersThePrototypesRights(t *testing.T) {
	t.Parallel()
	offered := map[PermissionOperation]bool{}
	for _, item := range kinds.All() {
		info, ok := kindInfoOf(item.Kind)
		if !ok {
			continue
		}
		// A kind the window of a role offers rights on and no export gives
		// any on: offered, and unproven, so every role naming it is noted.
		if names, seen := rightsSeenInTheWindow[item.Kind]; seen {
			if want := rightsByNames(t, names); !slices.Equal(info.uses.offered, want) || info.uses.rights != unproven {
				t.Errorf("%s offers %v (rights %v), the window %v, unproven", item.Kind, info.uses.offered, info.uses.rights, want)
			}
			for _, right := range info.uses.offered {
				offered[right] = true
			}
			continue
		}
		names, expected := expectedKindRights[item.Kind]
		if !expected {
			if info.uses.offered != nil {
				t.Errorf("%s offers %v, and no export gives rights on it", item.Kind, info.uses.offered)
			}
			if info.uses.rights == present {
				t.Errorf("%s is shown in roles and offers no rights", item.Kind)
			}
			continue
		}
		if info.uses.rights != present {
			t.Errorf("%s offers rights and is not shown in roles", item.Kind)
		}
		if want := rightsByNames(t, names); !slices.Equal(info.uses.offered, want) || info.uses.offered == nil {
			t.Errorf("%s offers %v, the prototype %v", item.Kind, info.uses.offered, want)
		}
		for _, right := range info.uses.offered {
			offered[right] = true
		}
	}
	want := rightsByNames(t, expectedConfigurationRights)
	at := slices.Index(want, "developer-tools") + 1
	if want = slices.Insert(want, at, mlConfigurationRights...); !slices.Equal(configurationRights, want) {
		t.Errorf("the configuration offers %v, the prototype %v", configurationRights, want)
	}
	for _, right := range configurationRights {
		offered[right] = true
	}
	for _, right := range RoleRights() {
		if !offered[right] {
			t.Errorf("%s is offered on nothing", right)
		}
	}
}

// TestKindRightsAgreeWithTheExports catches a pair of kind and right an
// export of the configurator writes on an object and the editor does not
// offer, and rights the editor offers in an order other than the one an
// export writes them in, reading the Rights.xml of every role under docs/. The exports lie
// outside the repository; without them the test is skipped with the reason.
// The interfaces of the ordinary application and the integration services are
// left out by the owner's decisions of 30.09 and 12.10.2026.
func TestKindRightsAgreeWithTheExports(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "docs")
	if _, err := os.Stat(filepath.Join(root, "import-examples")); err != nil {
		t.Skipf("the exports are not here (%v): they are kept out of the repository", err)
	}
	byClass := map[string]Kind{}
	for _, item := range kinds.All() {
		byClass[item.Name.English] = item.Kind
	}
	type rights struct {
		Objects []struct {
			Name   string `xml:"name"`
			Rights []struct {
				Name string `xml:"name"`
			} `xml:"right"`
		} `xml:"object"`
	}
	read := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || entry.Name() != "Rights.xml" ||
			!strings.Contains(filepath.ToSlash(path), "/Roles/") {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var role rights
		if err := xml.Unmarshal(content, &role); err != nil {
			return err
		}
		read++
		for _, object := range role.Objects {
			parts := strings.Split(object.Name, ".")
			var allowed []PermissionOperation
			switch {
			case len(parts) == 2 && parts[0] == "Configuration":
				allowed = configurationRights
			case len(parts) == 2 && (parts[0] == "Interface" || parts[0] == "IntegrationService"):
				continue
			case len(parts) == 2:
				kind, ok := byClass[parts[0]]
				if !ok {
					t.Errorf("%s: %s is no kind", path, object.Name)
					continue
				}
				allowed = usesOf(kind).offered
			case parts[0] == "Interface" || parts[0] == "IntegrationService":
				continue
			default:
				// A nested object or a part: the last class of the name decides.
				class := parts[len(parts)-2]
				if names, ok := expectedPartRights[class]; ok {
					allowed = rightsByNames(t, names)
					break
				}
				kind, ok := byClass[class]
				if !ok {
					t.Errorf("%s: %s is no kind and no part", path, object.Name)
					continue
				}
				allowed = usesOf(kind).offered
			}
			last := -1
			for _, right := range object.Rights {
				if slices.Contains(rightsNotKept, right.Name) {
					continue
				}
				operation, ok := RightByPrototypeName(right.Name)
				place := slices.Index(allowed, operation)
				if !ok || place < 0 {
					t.Errorf("%s: %s on %s is not offered", path, right.Name, object.Name)
					continue
				}
				// The thin and the web client both stand for the start of
				// the application, one place.
				if place < last {
					t.Errorf("%s: %s on %s is offered before %s", path, right.Name, object.Name, allowed[last])
				}
				last = place
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if read == 0 {
		t.Fatal("no role read")
	}
	t.Logf("roles read: %d", read)
}

// A kind whose parts are read off its description (kindComposition.described)
// keeps in every field partGroups names a list of parts of that sort: fields
// (an Attribute, or a dimension, a resource, a flag holding one), table parts,
// commands. Defects caught: a kind read off its description that keeps under
// such a name something else - the fields a criterion of filter searches by
// are uuids - and fails the load the first time an object of it is read, or
// that the composition cannot find, and so silently holds no parts at all; a
// field partGroups names that no kind has, so that its parts are read nowhere.
func TestADescribedKindKeepsPartsWhereItsPartsAreRead(t *testing.T) {
	t.Parallel()
	attribute := reflect.TypeFor[Attribute]()
	holdsField := func(element reflect.Type) bool {
		if element == attribute {
			return true
		}
		field, ok := element.FieldByName("Attribute")
		return ok && field.Type == attribute
	}
	kept := map[string]bool{}
	for kind, composition := range kindCompositionOf {
		if !composition.described {
			continue
		}
		info, _ := kindInfoOf(kind)
		if info.object == nil {
			t.Errorf("%s reads its parts off a description it cannot find", kind)
			continue
		}
		for _, sort := range partGroups {
			if sort.field == "" {
				continue
			}
			field, ok := info.described.FieldByName(sort.field)
			if !ok {
				continue
			}
			kept[sort.field] = true
			want := "a list of fields"
			okType := field.Type.Kind() == reflect.Slice
			if okType {
				switch element := field.Type.Elem(); sort.group {
				case TablePartsPart:
					want, okType = "a list of table parts", element == reflect.TypeFor[TablePart]()
				case CommandsPart:
					want, okType = "a list of commands", element == reflect.TypeFor[ObjectCommand]()
				default:
					okType = holdsField(element)
				}
			}
			if !okType {
				t.Errorf("%s keeps %s in %s, want %s", kind, field.Type, sort.field, want)
			}
		}
	}
	for _, sort := range partGroups {
		if sort.field != "" && !kept[sort.field] {
			t.Errorf("no kind keeps %s in %s", sort.group, sort.field)
		}
	}
}

// prototypeCaptions are the captions the window of a role lists the rights
// under in the configurator 8.3.27, written out from the pass of 20.10.2026
// over the configuration and an object of every kind (block-00/rol), not
// read from roleRights: «Запуск приложения», «Инструменты разработчика» and
// «История изменений» are ML's own rights; the rights ML does not keep are
// not listed.
var prototypeCaptions = map[string]string{
	"Read": "Чтение", "Insert": "Добавление", "Update": "Изменение", "Delete": "Удаление", "Posting": "Проведение",
	"UndoPosting": "Отмена проведения", "Use": "Использование", "View": "Просмотр",
	"InteractiveInsert": "Интерактивное добавление", "Edit": "Редактирование",
	"InteractiveDelete": "Интерактивное удаление", "InteractiveSetDeletionMark": "Интерактивная пометка на удаление",
	"InteractiveClearDeletionMark": "Интерактивное снятие пометки удаления",
	"InteractiveDeleteMarked":      "Интерактивное удаление помеченных", "InteractivePosting": "Интерактивное проведение",
	"InteractivePostingRegular": "Интерактивное проведение неоперативное",
	"InteractiveUndoPosting":    "Интерактивная отмена проведения", "InteractiveChangeOfPosted": "Интерактивное изменение проведенных",
	"InputByString": "Ввод по строке", "TotalsControl": "Управление итогами", "Administration": "Администрирование",
	"DataAdministration": "Администрирование данных", "UpdateDataBaseConfiguration": "Обновление конфигурации базы данных",
	"ExclusiveMode": "Монопольный режим", "ActiveUsers": "Активные пользователи", "EventLog": "Журнал регистрации",
	"RunApplication": "Запуск приложения", "DeveloperTools": "Инструменты разработчика", "ChangeHistory": "История изменений",
	"ExclusiveModeTerminationAtSessionStart": "Завершение монопольного режима при начале сеанса",
	"SaveUserData":                           "Сохранение данных пользователя",
	"InteractiveOpenExtDataProcessors":       "Интерактивное открытие внешних обработок",
	"InteractiveOpenExtReports":              "Интерактивное открытие внешних отчетов", "Output": "Вывод",
	"Get": "Получение", "Set": "Установка", "InteractiveActivate": "Интерактивная активация", "Start": "Старт",
	"InteractiveStart": "Интерактивный старт", "Execute": "Выполнение", "InteractiveExecute": "Интерактивное выполнение",
	"InteractiveDeletePredefinedData":            "Интерактивное удаление предопределенных",
	"InteractiveSetDeletionMarkPredefinedData":   "Интерактивная пометка на удаление предопределенных",
	"InteractiveClearDeletionMarkPredefinedData": "Интерактивное снятие пометки удаления предопределенных",
	"InteractiveDeleteMarkedPredefinedData":      "Интерактивное удаление помеченных предопределенных",
}

// TestRightsAreCaptionedAsThePrototype catches a right the editor lists
// under a caption of its own making - the name in words, as before 2.412,
// gave «Интерактивная пометка удаления» and «Режим основного окна обычный» -
// and a right shown with no description under the tree.
func TestRightsAreCaptionedAsThePrototype(t *testing.T) {
	t.Parallel()
	schema, err := LoadPermissionSchema(compositeProject(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Rights) != len(prototypeCaptions) {
		t.Errorf("the editor lists %d rights, the prototype's pass %d", len(schema.Rights), len(prototypeCaptions))
	}
	for _, right := range schema.Rights {
		var english string
		for _, kept := range roleRights {
			if kept.operation == right.Key {
				english = kept.english
			}
		}
		if want, ok := prototypeCaptions[english]; !ok || right.Title != want {
			t.Errorf("%s is listed as %q, the prototype %q", right.Key, right.Title, want)
		}
		if strings.TrimSpace(right.Description) == "" || right.Name == "" {
			t.Errorf("%s has no description or no name to show under the tree: %+v", right.Key, right)
		}
	}
}
