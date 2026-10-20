package studio

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The modules of an object are no rows of the tree: each opens from the head
// of the object's menu by the words the prototype's menu of its kind gives,
// in its order, whether or not it has text yet (inventory of block 00:
// spravochnik.md - «открыть модуль объекта, модуль менеджера»; konstanta.md -
// «Открыть модуль менеджера значения; Открыть модуль менеджера»;
// registr-svedenij.md, posledovatelnost.md, zhurnal-dokumentov.md,
// perechislenie.md, http-servis.md; obshchaya-komanda.md and komanda.md -
// «Открыть модуль команды», bold). The expectation is written out here from
// the inventory, not read from the descriptors. Defects caught: a module left
// a row of the tree; a kind's module missing from its menu, in another order
// or with other words (the constant's «менеджера значения» said as the
// tree's «Модуль значения»); a module offered only once it has a file; the
// command of a command not its own action, or another module made one.
func TestObjectsOpenTheirModulesFromTheirMenu(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	type offer struct {
		Command string
		Main    bool
	}
	plain := func(words ...string) []offer {
		result := make([]offer, 0, len(words))
		for _, word := range words {
			result = append(result, offer{Command: word})
		}
		return result
	}
	objects := map[string]struct {
		files map[string]string
		want  []offer
	}{
		"catalogs/Банки": {map[string]string{"МодульОбъекта.bsl": "Процедура ПередЗаписью(Отказ)\nКонецПроцедуры\n"},
			plain("Открыть модуль объекта", "Открыть модуль менеджера")},
		"constants/Курс":              {nil, plain("Открыть модуль менеджера значения", "Открыть модуль менеджера")},
		"information-registers/Курсы": {nil, plain("Открыть модуль набора записей", "Открыть модуль менеджера")},
		"sequences/Партии":            {nil, plain("Открыть модуль набора записей")},
		"document-journals/Журнал":    {nil, plain("Открыть модуль менеджера")},
		"enumerations/Виды":           {nil, plain("Открыть модуль менеджера")},
		"http-services/Обмен":         {map[string]string{"МодульСервиса.bsl": "\n"}, plain("Открыть модуль")},
		"common-commands/Печать": {map[string]string{"МодульКоманды.bsl": "&НаКлиенте\nПроцедура ОбработкаКоманды(П, П2)\nКонецПроцедуры\n"},
			[]offer{{Command: "Открыть модуль команды", Main: true}}},
	}
	for object, item := range objects {
		writeProjectFile(t, root, "metadata/"+object+"/object.yaml",
			"format: 1\nid: "+uuid.MustNew().String()+"\nname: "+lastSegment(object)+"\ntitle: {ru: "+lastSegment(object)+"}\n")
		for file, content := range item.files {
			writeProjectFile(t, root, "metadata/"+object+"/"+file, content)
		}
	}
	const command = "metadata/catalogs/Банки/commands/Обновить/МодульКоманды.bsl"
	writeProjectFile(t, root, command, "&НаКлиенте\nПроцедура ОбработкаКоманды(П, П2)\nКонецПроцедуры\n")
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	for object, item := range objects {
		folder := "metadata/" + object
		var node Node
		var walk func(Node)
		walk = func(current Node) {
			// The object stands above its branches, which open its
			// description too.
			if current.Path == folder+"/object.yaml" && node.ID == "" {
				node = current
			}
			for _, child := range current.Children {
				walk(child)
			}
		}
		walk(snapshot.Tree)
		var got []offer
		for _, module := range node.Modules {
			if !strings.HasPrefix(module.Path, folder+"/") || strings.Contains(strings.TrimPrefix(module.Path, folder+"/"), "/") || module.Disabled {
				t.Errorf("%s offers %+v", object, module)
			}
			got = append(got, offer{Command: module.Command, Main: module.Main})
		}
		if !reflect.DeepEqual(got, item.want) {
			t.Errorf("the menu of %s opens %+v, the prototype's opens %+v", object, got, item.want)
		}
	}
	if module, owner, ok := findModuleCommand(snapshot.Tree, command); !ok || module.Command != "Открыть модуль команды" || !module.Main || owner.Title != "Обновить" {
		t.Errorf("the command of the catalog opens its module as %+v from %q", module, owner.Title)
	}
	var walk func(Node)
	walk = func(node Node) {
		if strings.HasSuffix(node.Path, ".bsl") && node.Kind != "commands" {
			t.Errorf("module %s is a row of the tree: %+v", node.Path, node)
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(snapshot.Tree)
}

// Opening a module from the menu makes it the first time, empty, as the
// prototype's object has every module of its kind whether or not it has text;
// opening it again keeps what was written. Only a module some menu offers is
// made: one of a role the kind has not, of an object that is not there, of a
// command or a form that is not there, stays missing, and nothing is left
// behind. Defects caught: a module offered and refused as missing; opening
// that writes over a module's text; a file made where the loader then refuses
// the project.
func TestOpeningAModuleMakesItOnlyWhereTheMenuOffersIt(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	writeProjectFile(t, root, "metadata/catalogs/Банки/object.yaml",
		"format: 1\nid: "+uuid.MustNew().String()+"\nname: Банки\ntitle: {ru: Банки}\n")
	writeProjectFile(t, root, "metadata/catalogs/Банки/МодульОбъекта.bsl", "Процедура ПередЗаписью(Отказ)\nКонецПроцедуры\n")
	// A form that is there, its module not: a form opens by «Открыть форму»,
	// no menu offers its module.
	writeProjectFile(t, root, "metadata/catalogs/Банки/forms/ФормаЭлемента/form.yaml", "format: 1\n")
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	existing, err := workspace.OpenModule("metadata/catalogs/Банки/МодульОбъекта.bsl")
	if err != nil || !strings.Contains(existing.Content, "ПередЗаписью") {
		t.Fatalf("the object module opened as %+v (%v)", existing, err)
	}
	const manager = "metadata/catalogs/Банки/МодульМенеджера.bsl"
	made, err := workspace.OpenModule(manager)
	if err != nil || made.Content != "" || made.Language != "bsl" || made.Path != manager {
		t.Fatalf("the manager module opened as %+v (%v)", made, err)
	}
	if content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(manager))); err != nil || len(content) != 0 {
		t.Fatalf("the manager module was not made empty: %q %v", content, err)
	}
	written, err := workspace.SaveSource(manager, "Функция Найти() Экспорт\nКонецФункции\n", made.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := workspace.OpenModule(manager); err != nil || again.Revision != written.Revision {
		t.Fatalf("opening again replaced the module: %+v (%v)", again, err)
	}
	for _, path := range []string{
		"metadata/catalogs/Банки/МодульНабораЗаписей.bsl",
		"metadata/catalogs/Нет/МодульОбъекта.bsl",
		"metadata/catalogs/Банки/commands/Нет/МодульКоманды.bsl",
		"metadata/catalogs/Банки/forms/Нет/МодульФормы.bsl",
		"metadata/catalogs/Банки/forms/ФормаЭлемента/МодульФормы.bsl",
		"metadata/catalogs/Банки/object.yaml",
	} {
		if opened, err := workspace.OpenModule(path); err == nil {
			t.Errorf("%s opened as %+v", path, opened)
		}
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(path))); path != "metadata/catalogs/Банки/object.yaml" && !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s was made: %v", path, err)
		}
	}
}
