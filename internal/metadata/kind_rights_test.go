package metadata

import (
	"encoding/xml"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/kinds"
)

// expectedKindRights is what the configurator writes on an object of each
// kind, by the prototype's names, from the Rights.xml of every role of the
// four exports checked, demo-base, energy, ssl and the configurator's samples
// in mdclasses (survey of 12.10.2026). Written out here, not read from
// kindRightsOf.
var expectedKindRights = func() map[Kind][]string {
	history := []string{"ReadDataHistory", "ViewDataHistory", "UpdateDataHistory", "UpdateDataHistorySettings",
		"UpdateDataHistoryVersionComment", "EditDataHistoryVersionComment", "SwitchToDataHistoryVersion"}
	missing := []string{"ReadDataHistoryOfMissingData", "UpdateDataHistoryOfMissingData"}
	reference := []string{"Read", "Insert", "Update", "Delete", "View", "InteractiveInsert", "Edit",
		"InteractiveDelete", "InteractiveSetDeletionMark", "InteractiveClearDeletionMark", "InteractiveDeleteMarked",
		"InputByString"}
	predefined := []string{"InteractiveDeletePredefinedData", "InteractiveSetDeletionMarkPredefinedData",
		"InteractiveClearDeletionMarkPredefinedData", "InteractiveDeleteMarkedPredefinedData"}
	chart := slices.Concat(reference, predefined, history, missing)
	return map[Kind][]string{
		SubsystemKind: {"View"}, SessionParameterKind: {"Get", "Set"}, CommonAttributeKind: {"View", "Edit"},
		ExchangePlanKind: slices.Concat(reference, history, missing), FilterCriterionKind: {"View"},
		CommonCommandKind: {"View"}, CommonFormKind: {"View"}, WebServiceKind: {}, HTTPServiceKind: {},
		ConstantKind: slices.Concat([]string{"Read", "Update", "View", "Edit"}, history),
		CatalogKind:  chart,
		DocumentKind: slices.Concat(reference, []string{"Posting", "UndoPosting", "InteractivePosting",
			"InteractivePostingRegular", "InteractiveUndoPosting", "InteractiveChangeOfPosted"}, history, missing),
		SequenceKind: {"Read", "Update"}, DocumentJournalKind: {"Read", "View"},
		ReportKind: {"Use", "View"}, DataProcessorKind: {"Use", "View"},
		ChartOfCharacteristicTypesKind: chart, ChartOfAccountsKind: chart, ChartOfCalculationTypesKind: chart,
		InformationRegisterKind:  slices.Concat([]string{"Read", "Update", "View", "Edit", "TotalsControl"}, history, missing),
		AccumulationRegisterKind: {"Read", "Update", "View", "Edit", "TotalsControl"},
		AccountingRegisterKind:   {"Read", "Update", "View", "Edit", "TotalsControl"},
		CalculationRegisterKind:  {"Read", "Update", "View", "Edit"},
		RecalculationKind:        {"Read", "Update"},
		BusinessProcessKind: slices.Concat(reference, []string{"InteractiveActivate", "Start", "InteractiveStart"},
			history, missing),
		TaskKind: slices.Concat(reference, []string{"InteractiveActivate", "Execute", "InteractiveExecute"},
			history, missing),
		ExternalDataSourceKind: {"Use", "Administration", "StandardAuthenticationChange",
			"SessionStandardAuthenticationChange", "SessionOSAuthenticationChange"},
		ExternalDataSourceTableKind: {"Read", "Insert", "Update", "Delete", "View", "InteractiveInsert", "Edit",
			"InteractiveDelete", "InputByString"},
	}
}()

// expectedConfigurationRights is what the same exports write on the
// configuration itself.
var expectedConfigurationRights = []string{"Administration", "DataAdministration", "UpdateDataBaseConfiguration",
	"ExclusiveMode", "ActiveUsers", "EventLog", "ThinClient", "WebClient", "MobileClient", "ThickClient",
	"ExternalConnection", "Automation", "TechnicalSpecialistMode", "SaveUserData", "InteractiveOpenExtDataProcessors",
	"InteractiveOpenExtReports", "Output", "ConfigurationExtensionsAdministration",
	"CollaborationSystemInfoBaseRegistration", "MainWindowModeNormal", "MainWindowModeWorkplace",
	"MainWindowModeEmbeddedWorkplace", "MainWindowModeFullscreenWorkplace", "MainWindowModeKiosk",
	"ExclusiveModeTerminationAtSessionStart", "AnalyticsSystemClient"}

// rightsByNames reads prototype names as rights, in the order of the help.
func rightsByNames(t *testing.T, names []string) []PermissionOperation {
	t.Helper()
	result := []PermissionOperation{}
	for _, name := range names {
		right, ok := RightByPrototypeName(name)
		if !ok {
			t.Fatalf("%s is no right of the prototype", name)
		}
		result = append(result, right)
	}
	slices.SortFunc(result, func(a, b PermissionOperation) int {
		return slices.Index(RoleRights(), a) - slices.Index(RoleRights(), b)
	})
	return result
}

// TestEveryKindOffersThePrototypesRights catches the role editor offering a
// right the configurator does not offer on an object of the kind, or missing
// one it does: a catalog without the rights on its predefined data, a
// constant with input by string, a register without the management of its
// totals, a kind a role may name with no list at all, or a kind no export
// gives rights on offered some. It also catches a right of the 69 the editor
// offers nowhere, and rights out of the order of the help.
func TestEveryKindOffersThePrototypesRights(t *testing.T) {
	t.Parallel()
	offered := map[PermissionOperation]bool{}
	for _, item := range kinds.All() {
		info, ok := kindInfoOf(item.Kind)
		if !ok {
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
	if want := rightsByNames(t, expectedConfigurationRights); !slices.Equal(configurationRights, want) {
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
// offer, reading the Rights.xml of every role under docs/. The exports lie
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
			default:
				// A part of an object, given rights in the next iteration.
				continue
			}
			for _, right := range object.Rights {
				operation, ok := RightByPrototypeName(right.Name)
				if !ok || !slices.Contains(allowed, operation) {
					t.Errorf("%s: %s on %s is not offered", path, right.Name, object.Name)
				}
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
