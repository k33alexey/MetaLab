package studio

import (
	"testing"

	"github.com/k33alexey/MetaLab/internal/icons"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// TestTreeShowsWhatEachKindKeeps catches the tree laid out against the
// kinds: a calculation register keeping the module of a recalculation broke
// the whole tree ("unexpected source path .../recalculations"), and every
// object stood with Формы, Команды and Макеты, a sequence's too. The
// expectation is the help's (ОбъектМетаданных: <Вид>.Формы, .Команды, .Макеты)
// and the configurator's tree, where the recalculations stand under their
// register, each opening its module from its menu (pereraschet.md).
func TestTreeShowsWhatEachKindKeeps(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	ids := map[string]uuid.UUID{}
	for _, object := range []string{"calculation-registers/Начисления", "sequences/Партии", "filter-criteria/ПоКонтрагенту", "settings-storages/Отчеты"} {
		id := uuid.MustNew()
		ids[object] = id
		writeProjectFile(t, root, "metadata/"+object+"/object.yaml", "format: 1\nid: "+id.String()+"\nname: "+lastSegment(object)+"\ntitle: {ru: "+lastSegment(object)+"}\n")
	}
	const recalculation = "metadata/calculation-registers/Начисления/recalculations/Перерасчет"
	writeProjectFile(t, root, recalculation+"/МодульНабораЗаписей.bsl", "")
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatalf("the tree is not built: %v", err)
	}
	module, owner, ok := findModuleCommand(snapshot.Tree, recalculation+"/МодульНабораЗаписей.bsl")
	if !ok || module.Command != "Открыть модуль набора записей" || owner.ID != recalculation {
		t.Fatalf("the module of the recalculation is offered as %+v by %s", module, owner.ID)
	}
	if _, err := workspace.ReadSource(module.Path); err != nil {
		t.Fatalf("the module of the recalculation does not open: %v", err)
	}
	if group, ok := findNodeByID(snapshot.Tree, "metadata/calculation-registers/Начисления/recalculations"); !ok || group.Title != "Перерасчёты" {
		t.Fatalf("the recalculations stand as %+v", group)
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
	for _, object := range []string{"catalogs/Банки", "information-registers/Курсы", "calculation-registers/Начисления", "sequences/Партии"} {
		id := uuid.MustNew()
		ids[object] = id
		writeProjectFile(t, root, "metadata/"+object+"/object.yaml", "format: 1\nid: "+id.String()+"\nname: "+lastSegment(object)+"\ntitle: {ru: "+lastSegment(object)+"}\n")
	}
	writeProjectFile(t, root, "metadata/calculation-registers/Начисления/recalculations/Перерасчет/МодульНабораЗаписей.bsl", "")
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
		ids["catalogs/Банки"].String() + ":forms":                             "form",
		ids["catalogs/Банки"].String() + ":commands":                          "command",
		ids["catalogs/Банки"].String() + ":templates":                         "template",
		ids["catalogs/Банки"].String() + ":attributes":                        "attribute",
		ids["catalogs/Банки"].String() + ":table-parts":                       "tabular-section",
		ids["information-registers/Курсы"].String():                           "information-register",
		ids["information-registers/Курсы"].String() + ":dimensions":           "dimension",
		ids["information-registers/Курсы"].String() + ":resources":            "resource",
		"metadata/calculation-registers/Начисления/recalculations":            "recalculation",
		"metadata/calculation-registers/Начисления/recalculations/Перерасчет": "recalculation",
		ids["sequences/Партии"].String():                                      "sequence",
		"metadata/document-numerators":                                        "numerator",
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
