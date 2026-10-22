package studio

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/icons"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// TestTreeShowsWhatEachKindKeeps catches the tree laid out against the
// kinds: every object stood with Формы, Команды and Макеты, a sequence's too.
// The expectation is the help's (ОбъектМетаданных: <Вид>.Формы, .Команды,
// .Макеты) and the configurator's tree.
func TestTreeShowsWhatEachKindKeeps(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	ids := map[string]uuid.UUID{}
	for _, object := range []string{"calculation-registers/Начисления", "sequences/Партии", "filter-criteria/ПоКонтрагенту", "settings-storages/Отчеты"} {
		id := uuid.MustNew()
		ids[object] = id
		writeProjectFile(t, root, "metadata/"+object+"/object.yaml", "format: 1\nid: "+id.String()+"\nname: "+lastSegment(object)+"\ntitle: {ru: "+lastSegment(object)+"}\n")
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatalf("the tree is not built: %v", err)
	}
	for object, groups := range map[string]string{
		"calculation-registers/Начисления": "forms commands templates",
		"sequences/Партии":                 "",
		"filter-criteria/ПоКонтрагенту":    "forms commands",
		"settings-storages/Отчеты":         "forms templates",
	} {
		var got string
		for _, group := range []string{"forms", "commands", "templates"} {
			if _, ok := findNodeByID(snapshot.Tree, ids[object].String()+":"+group); ok {
				got += " " + group
			}
		}
		if want := groups; got != "" && got[1:] != want || got == "" && want != "" {
			t.Errorf("%s stands with groups %q, its kind keeps %q", object, got, want)
		}
	}
}

// writeCalculationRegisters writes two calculation registers on one chart of
// calculation types: Начисления describes the recalculation Перерасчет with
// the dimension Измерение1 and no folder of its module, then Второй with its
// module in its folder; Удержания describes none.
func writeCalculationRegisters(t *testing.T, root string) (accruals, deductions uuid.UUID) {
	t.Helper()
	chart, accruals, deductions, employee := uuid.MustNew().String(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew().String()
	writeProjectFile(t, root, "metadata/charts-of-calculation-types/Виды/object.yaml", "format: 1\nid: "+chart+
		"\nname: Виды\ntitle: {ru: Виды}\ncode: {type: string, length: 9, auto: false}\ndescription_length: 100\n")
	register := func(id uuid.UUID, name, rest string) {
		writeProjectFile(t, root, "metadata/calculation-registers/"+name+"/object.yaml", "format: 1\nid: "+id.String()+"\nname: "+name+
			"\ntitle: {ru: "+name+"}\nchart_of_calculation_types: "+chart+"\nperiodicity: month\n"+
			"resources:\n  - {id: "+uuid.MustNew().String()+", name: Результат, title: {ru: Результат}, types: [{kind: number, precision: 15, scale: 2}]}\n"+rest)
	}
	register(accruals, "Начисления", "dimensions:\n  - {id: "+employee+", name: Сотрудник, title: {ru: Сотрудник}, types: [{kind: string, length: 10}]}\n"+
		"recalculations:\n  - {id: "+uuid.MustNew().String()+", name: Перерасчет, title: {ru: Перерасчет}, dimensions: [{id: "+uuid.MustNew().String()+
		", name: Измерение1, title: {ru: Измерение1}, register_dimension: "+employee+"}]}\n"+
		"  - {id: "+uuid.MustNew().String()+", name: Второй, title: {ru: Второй}}\n")
	writeProjectFile(t, root, "metadata/calculation-registers/Начисления/recalculations/Второй/МодульНабораЗаписей.bsl", "")
	register(deductions, "Удержания", "")
	return accruals, deductions
}

// TestTreeShowsTheRecalculationsTheRegisterDescribes holds the recalculations
// to the prototype's tree (block-00/pereraschet.md, snapshots derevo/05, 05b):
// «Перерасчеты» stands under every calculation register after its
// attributes and before its forms, empty where it describes none; under it
// the recalculations in the order of the description, each with its
// dimensions right under it and its record set module offered from its menu,
// whether its folder is there or not. Defects caught: the tree read the
// recalculations off the folders on disk (2.437, cases 2в-14 and 2в-15) - a recalculation
// described with no module stood nowhere, its dimensions were never shown,
// and a register without the folder had no «Перерасчеты»; the module of a
// recalculation with no folder offered and not made, or made for one the
// register does not describe.
func TestTreeShowsTheRecalculationsTheRegisterDescribes(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	accruals, deductions := writeCalculationRegisters(t, root)
	if _, err := metadata.Load(root); err != nil {
		t.Fatalf("the project does not load: %v", err)
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	captions := func(node Node) []string {
		var result []string
		for _, child := range node.Children {
			result = append(result, child.Title)
		}
		return result
	}
	register, _ := findNodeByID(snapshot.Tree, accruals.String())
	if got, want := captions(register), []string{"Измерения", "Ресурсы", "Реквизиты", "Перерасчёты", "Формы", "Команды", "Макеты"}; !slices.Equal(got, want) {
		t.Fatalf("Начисления shows %v, the prototype %v", got, want)
	}
	group := register.Children[3]
	if got := captions(group); !slices.Equal(got, []string{"Перерасчет", "Второй"}) {
		t.Fatalf("«Перерасчеты» of Начисления shows %v, want Перерасчет, then Второй", got)
	}
	if got := captions(group.Children[0]); !slices.Equal(got, []string{"Измерение1"}) || group.Children[0].Children[0].Icon != "dimension" {
		t.Errorf("Перерасчет shows %v under it, want its dimension Измерение1 drawn as a dimension", group.Children[0].Children)
	}
	empty, _ := findNodeByID(snapshot.Tree, deductions.String())
	if got := captions(empty); len(got) < 4 || got[3] != "Перерасчёты" || len(empty.Children[3].Children) != 0 {
		t.Errorf("Удержания shows %v, want an empty «Перерасчеты» after its attributes", got)
	}
	const folder = "metadata/calculation-registers/Начисления/recalculations/"
	for _, name := range []string{"Перерасчет", "Второй"} {
		module, owner, ok := findModuleCommand(snapshot.Tree, folder+name+"/МодульНабораЗаписей.bsl")
		if !ok || module.Command != "Открыть модуль набора записей" || module.Disabled || owner.Title != name {
			t.Errorf("the module of %s is offered as %+v by %q", name, module, owner.Title)
			continue
		}
		if _, err := workspace.OpenModule(module.Path); err != nil {
			t.Errorf("the module of %s does not open: %v", name, err)
		}
	}
	if _, err := metadata.Load(root); err != nil {
		t.Errorf("the project does not load once the modules are opened: %v", err)
	}
	if _, err := workspace.OpenModule(folder + "Чужой/МодульНабораЗаписей.bsl"); err == nil {
		t.Error("the module of a recalculation Начисления does not describe is made")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(folder+"Чужой"))); !os.IsNotExist(err) {
		t.Errorf("a folder is left for the recalculation Начисления does not describe: %v", err)
	}
}

func lastSegment(path string) string {
	for index := len(path) - 1; index >= 0; index-- {
		if path[index] == '/' {
			return path[index+1:]
		}
	}
	return path
}

// TestTreeDrawsEveryNodeWithItsIcon catches a node of the tree left without
// its icon, or named one the set does not draw: the tree shows an empty square
// there. The icons a node is expected to have are written out here from the
// prototype's tree (block 00: a branch and its objects share the kind's
// icon, the folder «Формы» the icon of a form, «Измерения» of a dimension),
// not read from the descriptors the code draws them by.
func TestTreeDrawsEveryNodeWithItsIcon(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	ids := map[string]uuid.UUID{}
	for _, object := range []string{"catalogs/Банки", "information-registers/Курсы", "sequences/Партии"} {
		id := uuid.MustNew()
		ids[object] = id
		writeProjectFile(t, root, "metadata/"+object+"/object.yaml", "format: 1\nid: "+id.String()+"\nname: "+lastSegment(object)+"\ntitle: {ru: "+lastSegment(object)+"}\n")
	}
	accruals, _ := writeCalculationRegisters(t, root)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	var walk func(Node)
	walk = func(node Node) {
		if !icons.Has(node.Icon) {
			t.Errorf("node %q (%s) is drawn with icon %q, which the set does not draw", node.Title, node.ID, node.Icon)
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(snapshot.Tree)
	for id, want := range map[string]string{
		"project":                      "configuration",
		"metadata/common":              "common",
		"metadata/catalogs":            "catalog",
		ids["catalogs/Банки"].String(): "catalog",
		ids["catalogs/Банки"].String() + ":forms":                   "form",
		ids["catalogs/Банки"].String() + ":commands":                "command",
		ids["catalogs/Банки"].String() + ":templates":               "template",
		ids["catalogs/Банки"].String() + ":attributes":              "attribute",
		ids["catalogs/Банки"].String() + ":table-parts":             "tabular-section",
		ids["information-registers/Курсы"].String():                 "information-register",
		ids["information-registers/Курсы"].String() + ":dimensions": "dimension",
		ids["information-registers/Курсы"].String() + ":resources":  "resource",
		accruals.String() + ":recalculations":                       "recalculation",
		ids["sequences/Партии"].String():                            "sequence",
		"metadata/document-numerators":                              "numerator",
	} {
		node, ok := findNodeByID(snapshot.Tree, id)
		if !ok {
			t.Errorf("node %s is not in the tree", id)
			continue
		}
		if node.Icon != want {
			t.Errorf("node %q (%s) is drawn with %q, the prototype draws it with %q", node.Title, id, node.Icon, want)
		}
	}
}
