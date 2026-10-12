package studio

import (
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// TestTreeShowsWhatEachKindKeeps catches the tree laid out against the
// kinds: a calculation register keeping the module of a recalculation broke
// the whole tree ("unexpected source path .../recalculations"), and every
// object stood with Формы, Команды and Макеты, a sequence's too. The
// expectation is the help's (ОбъектМетаданных: <Вид>.Формы, .Команды, .Макеты)
// and the configurator's tree, where the recalculations stand under their
// register with their modules.
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
	module, ok := findNodeByID(snapshot.Tree, recalculation+"/МодульНабораЗаписей.bsl")
	if !ok || module.Title != "Модуль набора записей" {
		t.Fatalf("the module of the recalculation stands in the tree as %+v", module)
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
