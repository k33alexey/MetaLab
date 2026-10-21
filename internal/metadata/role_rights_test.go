package metadata

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// fullRole is a role naming every state a right can be in, on an object and
// on its parts, with a restriction and a command of a form.
func fullRole() RoleDefinition {
	return RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Полная", Title: LocalizedText{"ru": "Полная"},
		IndependentChildRights: true,
		Objects: []ObjectPermission{
			{Object: uuid.MustNew(), Operations: RoleRights(), Fields: []FieldPermission{
				{Field: "extdimensiontypes.turnoversonly", Operations: []PermissionOperation{PermissionView}, Denied: []PermissionOperation{PermissionEdit}},
				{Field: uuid.MustNew().String(), Denied: []PermissionOperation{PermissionView}}}},
			{Object: uuid.MustNew(), Denied: []PermissionOperation{PermissionRead, PermissionInteractiveDeleteMarkedPredefinedData},
				Restrictions: []RightRestriction{
					{Right: PermissionRead, Fields: []string{"ref", "dataversion"}},
					{Right: PermissionRead, Condition: "#ПоЗначениям(\"Документ\", \"Чтение\")\nИЛИ ЛОЖЬ"}}},
		},
		RestrictionTemplates: []RestrictionTemplate{{Name: "ПоЗначениям(Таблица, Право)", Condition: "ГДЕ #Параметр(1) = #Параметр(2)"}},
		Commands:             []CommandPermission{{Form: uuid.MustNew(), Command: uuid.MustNew()}},
	}
}

// A role written as its two files and read back is the same role: every one
// of the forty-five rights it carries - the prototype's sixty-nine but its
// four clients, the two rights on mechanisms ML refused and the twenty-one
// the owner did not carry on 20.10.2026 (eight of the configuration, the mode
// of the technical specialist, nine of the history of data, three of
// authentication on a source), and ML's three own: the start of the
// application, the developer tools, the change history - a denial apart from a grant, a part only denied,
// a dotted key, a restriction on an object the role only denies (235 such in
// the exports), the flags and the commands.
//
// Defect caught: a state lost between the files - a denial written as
// nothing, a restriction left in the grid or dropped, a part with no grant
// cleaned away - each of which turns a role of the prototype into another.
func TestRoleSurvivesItsTwoFiles(t *testing.T) {
	t.Parallel()
	role := fullRole()
	description, grid, err := EncodeRole(role)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(description, []byte("operations: [read")) || !bytes.Contains(description, []byte("restrictions:")) {
		t.Fatalf("the description holds the grid or misses the restriction:\n%s", description)
	}
	if bytes.Contains(grid, []byte("restrictions")) || bytes.Contains(grid, []byte("condition")) || !bytes.Contains(grid, []byte(`"denied"`)) {
		t.Fatalf("the grid holds a restriction or misses a denial:\n%s", grid)
	}
	decoded, err := DecodeRole("role.yaml", bytes.NewReader(description), bytes.NewReader(grid), metadataConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	// Against the role written, not against its own second writing: a state
	// lost on the way out is lost from both writings alike.
	if !reflect.DeepEqual(decoded, role) {
		t.Fatalf("the role did not come back the same:\nwrote %+v\nread  %+v", role, decoded)
	}
	again, againGrid, err := EncodeRole(decoded)
	if err != nil || !bytes.Equal(again, description) || !bytes.Equal(againGrid, grid) {
		t.Fatalf("a second writing differs from the first (%v):\n%s\n%s", err, again, againGrid)
	}
	if !slices.Equal(decoded.Objects[0].Operations, RoleRights()) || len(RoleRights()) != 45 {
		t.Fatalf("rights carried: %d of %d", len(decoded.Objects[0].Operations), len(RoleRights()))
	}
	if !decoded.IndependentChildRights || len(decoded.Objects[1].Restrictions) != 2 || len(decoded.RestrictionTemplates) != 1 || len(decoded.Commands) != 1 {
		t.Fatalf("flags, restriction or command lost: %+v", decoded)
	}
	// One value a line: a right changed is a line of difference.
	if !bytes.Contains(grid, []byte("\n        \"interactive-posting-regular\",\n")) {
		t.Fatalf("the grid is not one value a line:\n%.400s", grid)
	}
	// A role with nothing to say about any object keeps no grid.
	empty := role
	empty.Objects, empty.Commands = nil, nil
	if _, grid, err := EncodeRole(empty); err != nil || grid != nil {
		t.Fatalf("an empty role keeps a grid: %q, %v", grid, err)
	}
}

// What the description and the grid say has to agree.
//
// Defect caught: a restriction of an object the grid never names, or two
// restrictions for one object, read as if it were a role.
func TestRoleFilesThatDisagreeAreRefused(t *testing.T) {
	t.Parallel()
	role := fullRole()
	description, grid, err := EncodeRole(role)
	if err != nil {
		t.Fatal(err)
	}
	restricted := role.Objects[1].Object.String()
	for name, broken := range map[string][2]string{
		"restriction of an object the grid does not name": {strings.Replace(string(description), restricted, uuid.MustNew().String(), 1), string(grid)},
		"two restrictions for one object":                 {strings.Replace(string(description), "restrictions:\n", "restrictions:\n  - object: "+restricted+"\n    rights:\n      - right: read\n        condition: ГДЕ ЛОЖЬ\n", 1), string(grid)},
		"restriction without a grid":                      {string(description), ""},
		"unknown field in the grid":                       {string(description), strings.Replace(string(grid), `"objects"`, `"full_access": true, "objects"`, 1)},
		"two values in the grid":                          {string(description), string(grid) + "{}"},
		"a right that is not one":                         {string(description), strings.Replace(string(grid), `"read"`, `"admin"`, 1)},
	} {
		var gridReader *bytes.Reader
		if broken[1] != "" {
			gridReader = bytes.NewReader([]byte(broken[1]))
		}
		var err error
		if gridReader == nil {
			_, err = DecodeRole("role.yaml", strings.NewReader(broken[0]), nil, metadataConfiguration())
		} else {
			_, err = DecodeRole("role.yaml", strings.NewReader(broken[0]), gridReader, metadataConfiguration())
		}
		if err == nil {
			t.Errorf("%s was read as a role", name)
		}
	}
}

// The prototype writes rights under its English names, and the help gives the
// Russian ones too; the mode of the technical specialist was the mode of all
// functions before 8.3.16, and an export in compatibility with an earlier
// version writes the old name only.
//
// Defect caught: a role of such an export losing the right, or refused.
func TestRightsAreFoundByThePrototypesNames(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]PermissionOperation{
		"InteractiveDeleteMarkedPredefinedData": PermissionInteractiveDeleteMarkedPredefinedData,
		"ИнтерактивноеУдалениеПомеченныхПредопределенныхДанных": PermissionInteractiveDeleteMarkedPredefinedData,
		// The mode of the technical specialist, under either name, is the
		// developer tools in ML (ACCESS-RIGHTS, «Своих прав у ML три»).
		// Its name before 8.3.16 is written in both languages too: the help
		// of ПравоДоступа lists РежимВсеФункции (AllFunctionsMode).
		"AllFunctionsMode":                       "developer-tools",
		"РежимВсеФункции":                        "developer-tools",
		"РежимТехническогоСпециалиста":           "developer-tools",
		"DeveloperTools":                         "developer-tools",
		"ИсторияИзменений":                       "change-history",
		"ExclusiveModeTerminationAtSessionStart": PermissionExclusiveModeTerminationAtSessionStart,
		"insert":                                 PermissionInsert,
		// The thin and the web client are the start of the application in ML
		// (ACCESS-RIGHTS, «Запуск клиента — одно право»).
		"ThinClient":       "run-application",
		"ВебКлиент":        "run-application",
		"RunApplication":   "run-application",
		"ЗапускПриложения": "run-application",
		// The interactive opening of external data processors and reports is
		// carried again (the owner's decision of 20.10.2026,
		// METADATA-OBJECTS, «Внешние отчёты и обработки — свои у ML»).
		"InteractiveOpenExtDataProcessors":    "interactive-open-ext-data-processors",
		"ИнтерактивноеОткрытиеВнешнихОтчетов": "interactive-open-ext-reports",
	} {
		if got, ok := RightByPrototypeName(name); !ok || got != want {
			t.Errorf("%s found as %q, %v", name, got, ok)
		}
	}
	// FullAccess is no right; the thick and the mobile client and the rights
	// on the mechanisms ML refused are rights of the prototype nothing stands
	// for in ML.
	for _, name := range []string{"FullAccess", "ThickClient", "ТолстыйКлиент", "MobileClient", "МобильныйКлиент",
		"ConfigurationExtensionsAdministration", "РегистрацияИнформационнойБазыСистемыВзаимодействия",
		// The rights that would change nothing in ML, of the history of data
		// and of authentication on a source (the owner's decisions of
		// 20.10.2026).
		"ExternalConnection", "Automation", "MainWindowModeKiosk", "РежимОсновногоОкнаОбычный", "AnalyticsSystemClient",
		"ReadDataHistory", "ПереходНаВерсиюИсторииДанных", "StandardAuthenticationChange", "ИзменениеАутентификацииОССеанса"} {
		if got, ok := RightByPrototypeName(name); ok {
			t.Errorf("%s found as %q", name, got)
		}
	}
}

// A role saved while ML still kept the four clients or the rights on the
// mechanisms it refused (ACCESS-RIGHTS, «Роли, сохранённые прежней версией
// ML»): it is read - Studio opens it to repair -
// and refused, naming the role and the right, by the load that saves the
// project into a database and by the application reading the configuration
// a database holds. The rights are written here, not read from droppedRights.
//
// Defects caught: such a role refused by the reading, so Studio cannot open
// it to repair; accepted by the load or by the snapshot, so a right ML does
// not keep runs; a refusal that does not say which role and which right.
func TestRolesWithClientsMLDoesNotKeepAreReadAndNotRun(t *testing.T) {
	t.Parallel()
	for _, item := range []struct {
		right  PermissionOperation
		denied bool
	}{{"thin-client", false}, {"web-client", false}, {"thick-client", false}, {"mobile-client", false}, {"web-client", true},
		{"configuration-extensions-administration", true}, {"collaboration-system-info-base-registration", false},
		{"technical-specialist-mode", false}, {"external-connection", false}, {"main-window-mode-kiosk", true},
		{"analytics-system-client", false}} {
		verb := "gives"
		if item.denied {
			verb = "denies"
		}
		want := "role Кладовщик " + verb + " the right " + string(item.right) + ", which ML does not keep"
		root := compositeProject(t)
		project, err := Load(root)
		if err != nil {
			t.Fatal(err)
		}
		role, _ := project.Role("Кладовщик")
		permission := ObjectPermission{Object: project.Project.ID, Operations: []PermissionOperation{PermissionAdministration}}
		if item.denied {
			permission.Denied = []PermissionOperation{item.right}
		} else {
			permission.Operations = append(permission.Operations, item.right)
		}
		role.Objects = append(role.Objects, permission)
		if err := WriteRole(root, role); err != nil {
			t.Fatalf("%s: the role is not written: %v", want, err)
		}
		if _, err := LoadPermissionSchema(root); err != nil {
			t.Fatalf("%s: the schema of the editor is not read: %v", want, err)
		}
		if _, err := Load(root); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("the load: %v, want %q", err, want)
		}
		if err := ValidateProjectRole(root, role); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("saving the role in Studio: %v, want %q", err, want)
		}
	}
	catalog, role, form := roleCatalogFixture(t)
	snapshot, err := NewRuntimeSnapshot(catalog, []ManagedForm{form})
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Roles[0].Objects = append(snapshot.Roles[0].Objects, ObjectPermission{Object: catalog.Project.ID, Operations: []PermissionOperation{"thin-client"}})
	if _, err := snapshot.Catalog(); err == nil || !strings.Contains(err.Error(), "role "+role.Name+" gives the right thin-client, which ML does not keep") {
		t.Errorf("the configuration a database holds: %v", err)
	}
}

// A role saved by an earlier ML with rights on commands of forms
// (ACCESS-RIGHTS, «Команды формы»): it is read - Studio opens it to take them
// away - and refused, naming the role, by the load, by saving the role and by
// the configuration a database holds; a command of a form is available by the
// roles its own use names. The pair here names a command no form declares
// and one a common form does: neither is a right a role gives.
//
// Defects caught: such a role refused by the reading, so Studio cannot open
// it to repair; accepted by the load or by the snapshot, so a right the
// prototype has no place for is kept and published; a refusal not naming the
// role.
func TestRolesWithCommandsOfFormsAreReadAndNotRun(t *testing.T) {
	t.Parallel()
	const want = "role Кладовщик gives rights on 1 commands of forms, which a role does not give"
	root := compositeProject(t)
	project, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	role, _ := project.Role("Кладовщик")
	role.Commands = []CommandPermission{{Form: uuid.MustNew(), Command: uuid.MustNew()}}
	if err := WriteRole(root, role); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPermissionSchema(root); err != nil {
		t.Fatalf("the schema of the editor is not read: %v", err)
	}
	description, err := os.ReadFile(filepath.Join(root, "metadata", "roles", role.ID.String()+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	grid, err := os.ReadFile(filepath.Join(root, "metadata", "roles", role.ID.String(), RoleRightsFile))
	if err != nil {
		t.Fatal(err)
	}
	read, err := DecodeRole("role.yaml", bytes.NewReader(description), bytes.NewReader(grid), project.Project)
	if err != nil || len(read.Commands) != 1 {
		t.Fatalf("the role is not read to repair: %v, %+v", err, read.Commands)
	}
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("the load: %v, want %q", err, want)
	}
	if err := ValidateProjectRole(root, role); err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("saving the role in Studio: %v, want %q", err, want)
	}
	role.Commands = nil
	if err := ValidateProjectRole(root, role); err != nil {
		t.Errorf("the role repaired is refused: %v", err)
	}
	catalog, reader, form := roleCatalogFixture(t)
	snapshot, err := NewRuntimeSnapshot(catalog, []ManagedForm{form})
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Roles[0].Commands = []CommandPermission{{Form: form.ID, Command: form.Commands[0].ID}}
	if _, err := snapshot.Catalog(); err == nil || !strings.Contains(err.Error(), "role "+reader.Name+" gives rights on 1 commands of forms") {
		t.Errorf("the configuration a database holds: %v", err)
	}
}

// The loader reads the grid from the role's folder and refuses a folder that
// is anything else.
//
// Defect caught: a grid silently not read - a role loading with no rights at
// all - and a folder holding what a role does not keep.
func TestLoaderReadsTheGridBesideTheRole(t *testing.T) {
	t.Parallel()
	root := compositeProject(t)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	role, ok := catalog.Role("Кладовщик")
	if !ok || len(role.Objects) != 2 || !slices.Equal(role.Objects[1].Operations, []PermissionOperation{PermissionRead, PermissionInsert}) {
		t.Fatalf("the grid was not read: %+v", role.Objects)
	}
	folder := filepath.Join(root, "metadata", "roles", cmpRole)
	for name, breakIt := range map[string]func(t *testing.T){
		"a stray file": func(t *testing.T) { writeFile(t, filepath.Join(folder, "notes.txt"), "x") },
		"a folder of no role": func(t *testing.T) {
			writeFile(t, filepath.Join(root, "metadata", "roles", uuid.MustNew().String(), RoleRightsFile), "{}")
		},
		"a grid that is a link": func(t *testing.T) {
			target := filepath.Join(t.TempDir(), RoleRightsFile)
			if err := os.Rename(filepath.Join(folder, RoleRightsFile), target); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(folder, RoleRightsFile)); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			root = compositeProject(t)
			folder = filepath.Join(root, "metadata", "roles", cmpRole)
			breakIt(t)
			if _, err := Load(root); err == nil {
				t.Fatalf("a project with %s loaded", name)
			}
		})
	}
}

// A role gives rights on whatever the prototype's roles give them on - the
// configuration itself, subsystems, common forms and commands, session
// parameters, services, a recalculation - and on the parts the prototype
// names, standard table parts among them. Rights on a kind no export gives
// them on are carried with a note; a part the object lacks and an object the
// configuration lacks are refused. The expectation is the exports' (the
// Rights.xml of every role of the four checked, demo-base and energy), written
// out here, not read from kindUsesOf.
//
// A right no export writes where it stands - on the configuration, on an
// object of its kind, on a part of its sort - granted or denied, is carried
// with the note right-without-sample, which names the role, the place and the
// right.
//
// Defect caught: a role of the prototype refused for naming a subsystem, the
// configuration or a recalculation (demo-base, energy); one naming a common
// module refused without a reason, or accepted silently; a right outside the
// exports' lists accepted silently, or one inside them noted; a right on a
// part of a cube or a dimension table, which no export gives, refused as a
// part of nothing or accepted silently; a right on a command of an
// enumeration noted twice, as the object and as the right on its part.
func TestRightsGoWhereThePrototypeGivesThem(t *testing.T) {
	t.Parallel()
	root := compositeProject(t)
	project, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	grant := func(object string, rights ...PermissionOperation) ObjectPermission {
		id, err := uuid.Parse(object)
		if err != nil {
			t.Fatal(err)
		}
		return ObjectPermission{Object: id, Operations: rights}
	}
	part := func(permission ObjectPermission, field string, rights ...PermissionOperation) ObjectPermission {
		permission.Fields = append(permission.Fields, FieldPermission{Field: field, Operations: rights})
		return permission
	}
	accepted := map[string]ObjectPermission{
		"the configuration":     grant(project.Project.ID.String(), PermissionAdministration, PermissionRunApplication),
		"a subsystem":           grant(cmpSubsystem, PermissionView),
		"a common form":         grant(cmpCommonForm, PermissionView),
		"a common command":      grant(cmpCommonCommand, PermissionView),
		"a common attribute":    grant(cmpCommonAttribute, PermissionView, PermissionEdit),
		"a session parameter":   grant(cmpSessionUser, PermissionGet, PermissionSet),
		"a filter criterion":    grant(cmpCriterion, PermissionView),
		"a web service":         grant(cmpWebService),
		"an HTTP service":       grant(cmpHTTPService),
		"a sequence":            grant(cmpSequence, PermissionRead, PermissionUpdate),
		"a journal's type":      part(grant(cmpJournal, PermissionRead), "type", PermissionView),
		"a business process":    grant(cmpProcess, PermissionStart, PermissionInteractiveStart),
		"a task":                grant(cmpTask, PermissionExecute),
		"a standard table part": part(grant(cmpAccounts, PermissionRead), "extdimensiontypes", PermissionView),
		"its field":             part(grant(cmpAccounts, PermissionRead), "extdimensiontypes.turnoversonly", PermissionEdit),
		"a competing kind":      part(grant(cmpCalculationTypes, PermissionRead), "leadingcalculationtypes.calculationtype", PermissionView),
		"a recalculation":       grant(cmpRecalculation, PermissionRead, PermissionUpdate),
	}
	noted := map[string]ObjectPermission{
		"a common module": grant(cmpCommonModule, PermissionView),
		"a numerator":     grant(cmpNumerator, PermissionRead),
		"a role":          grant(cmpRole, PermissionView),
		"a defined type":  grant(cmpDefinedType, PermissionView),
		"an enumeration":  grant(cmpEnumeration, PermissionView),
		// The window gives the viewing of an enumeration's command, no export
		// writes it; the object is noted, not the right on its part as well.
		"a command of an enumeration": part(grant(cmpEnumeration), cmpEnumCommand, PermissionView),
		// Offered by the window of a role, written by no export (block-00/rol).
		"a function of a source": grant(cmpExternalFunction, PermissionUse, PermissionView),
	}
	deny := func(permission ObjectPermission, rights ...PermissionOperation) ObjectPermission {
		permission.Denied = append(permission.Denied, rights...)
		return permission
	}
	unoffered := map[string]struct {
		permission ObjectPermission
		written    string
	}{
		"update on a report": {grant(cmpReport, PermissionView, PermissionUpdate), "reports Ведомость update"},
		"posting denied":     {deny(grant(cmpReport, PermissionUse), PermissionPosting), "reports Ведомость posting"},
		"view on the configuration": {grant(project.Project.ID.String(), PermissionRunApplication, PermissionView),
			"configuration " + project.Project.Name + " view"},
		"use on a field of a standard table part": {part(grant(cmpAccounts, PermissionRead),
			"extdimensiontypes.turnoversonly", PermissionView, PermissionUse),
			"charts-of-accounts Хозрасчетный.ВидыСубконто.ТолькоОбороты use"},
		// The window offers it (block-00/rol), no export writes it.
		"view on a dimension of a cube": {part(grant(salesCube, PermissionRead), goodsDimension, PermissionView),
			"external-data-source-cubes Склад.Продажи.Товар view"},
		"view on a field of a dimension table": {part(grant(goodsDimTable, PermissionRead), goodsDimTableCode, PermissionView),
			"external-data-source-dimension-tables Склад.Продажи.Товары.Код view"},
	}
	for name, item := range unoffered {
		role, _ := project.Role("Кладовщик")
		role.Objects = append(role.Objects, item.permission)
		if err := WriteRole(root, role); err != nil {
			t.Fatal(err)
		}
		catalog, err := Load(root)
		if err != nil {
			t.Errorf("%s was refused: %v", name, err)
			continue
		}
		want := []Note{{Kind: NoteRightWithoutSample, Where: "roles Кладовщик rights", Written: item.written}}
		if got := catalog.Notes(); !slices.Equal(got, want) {
			t.Errorf("%s: notes %+v, want %+v", name, got, want)
		}
	}
	refused := map[string]ObjectPermission{
		"a foreign part":       part(grant(cmpAccounts, PermissionRead), "leadingcalculationtypes", PermissionView),
		"a part of nothing":    part(grant(cmpSubsystem, PermissionView), "ref", PermissionView),
		"an object of nothing": grant(uuid.MustNew().String(), PermissionRead),
	}
	for name, permission := range accepted {
		role, _ := project.Role("Кладовщик")
		role.Objects = append(role.Objects, permission)
		if err := WriteRole(root, role); err != nil {
			t.Fatal(err)
		}
		catalog, err := Load(root)
		if err != nil {
			t.Errorf("rights on %s were refused: %v", name, err)
			continue
		}
		if notes := catalog.Notes(); len(notes) != 0 {
			t.Errorf("rights on %s, which the exports give, are noted: %+v", name, notes)
		}
	}
	for name, permission := range noted {
		role, _ := project.Role("Кладовщик")
		role.Objects = append(role.Objects, permission)
		if err := WriteRole(root, role); err != nil {
			t.Fatal(err)
		}
		catalog, err := Load(root)
		if err != nil {
			t.Errorf("rights on %s were refused: %v", name, err)
			continue
		}
		// The object is noted whole, not right by right as well.
		if notes := catalog.Notes(); len(notes) != 1 || notes[0].Kind != NoteUseWithoutSample ||
			notes[0].Where != "roles Кладовщик objects" {
			t.Errorf("rights on %s: notes %+v, want one use-without-sample", name, notes)
		}
	}
	for name, permission := range refused {
		role, _ := project.Role("Кладовщик")
		role.Objects = append(role.Objects, permission)
		if err := WriteRole(root, role); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(root); err == nil {
			t.Errorf("rights on %s were accepted", name)
		}
	}
}
