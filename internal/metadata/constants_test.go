package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
)

const (
	constantObject = "d9000000-0000-4000-8000-000000000001"
	constantForm   = "d9000000-0000-4000-8000-000000000002"
)

// writeConstantModule writes one of a constant's modules beside its
// description, under the name of the role it plays.
func writeConstantModule(t *testing.T, root, name, role string) {
	t.Helper()
	path := filepath.Join(root, "metadata", string(ConstantKind), name, role)
	if err := os.WriteFile(path, []byte("// модуль\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A constant is shown and edited like anything else, and everything that
// serves that was missing from the model: it carried six fields and no
// modules at all, while the reference configuration keeps a value module on
// 73 constants of 230 and a manager module on 8 more.
func TestConstantKeepsWhatItIsShownAndWrittenBy(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeCommonForm(t, root, "ФормаРежима", `format: 1
id: `+constantForm+`
name: ФормаРежима
title: {ru: Форма режима}
kind: common
`)
	writeMetadata(t, root, ConstantKind, constantObject, `format: 1
id: `+constantObject+`
name: Режим
title: {ru: Режим}
comment: Как считать себестоимость
explanation: {ru: Влияет на расчёт себестоимости}
extended_presentation: {ru: Режим расчёта себестоимости}
types: [{kind: boolean}]
default_form: `+constantForm+`
use_standard_commands: true
data_history: use
data_lock: automatic
`)
	writeConstantModule(t, root, "Режим", project.ValueModuleFile)
	writeConstantModule(t, root, "Режим", project.ManagerModuleFile)

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	constant, ok := catalog.Constant("Режим")
	if !ok {
		t.Fatal("the constant did not load")
	}
	switch {
	case constant.Comment == "":
		t.Fatalf("the comment was lost: %+v", constant)
	case len(constant.Explanation) == 0 || len(constant.ExtendedPresentation) == 0:
		t.Fatalf("the presentations were lost: %+v", constant)
	case constant.DefaultForm == nil || constant.DefaultForm.String() != constantForm:
		t.Fatalf("which form opens the value was lost: %+v", constant)
	case !constant.UseStandardCommands:
		t.Fatalf("whether the platform offers its own commands was lost: %+v", constant)
	case constant.DataHistory != DataHistoryUse:
		t.Fatalf("whether the history of the value is kept was lost: %+v", constant)
	case constant.DataLock != project.AutomaticDataLock:
		t.Fatalf("how the value is locked was lost: %+v", constant)
	}

	// Both modules are read and compiled under names that say whose they are.
	modules, err := LoadProjectModules(root, catalog)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, module := range modules {
		names[module.Filename] = module.Name
	}
	for role, want := range map[string]string{
		project.ValueModuleFile:   "МодульЗначенияКонстанты.Режим",
		project.ManagerModuleFile: "МодульМенеджераКонстанты.Режим",
	} {
		path := "metadata/constants/Режим/" + role
		if names[path] != want {
			t.Fatalf("%s was compiled as %q, not as %q", path, names[path], want)
		}
	}

	// The catalog hands out copies here too.
	constant.Explanation["ru"] = "Подменено"
	again, _ := catalog.Constant("Режим")
	if again.Explanation["ru"] == "Подменено" {
		t.Fatal("a constant was handed out by reference")
	}
}

// A constant with no modules is the ordinary case: most constants are values
// nobody needed to check or watch.
func TestConstantWithoutModulesIsFine(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, ConstantKind, constantObject, `format: 1
id: `+constantObject+`
name: Режим
title: {ru: Режим}
types: [{kind: boolean}]
`)
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}
}

// The folder of a constant holds its description and the two modules it may
// keep, and nothing else.
func TestBrokenConstantFoldersAreRefused(t *testing.T) {
	t.Parallel()
	const body = `format: 1
id: ` + constantObject + `
name: Режим
title: {ru: Режим}
types: [{kind: boolean}]
`
	for name, test := range map[string]struct {
		write func(t *testing.T, root string)
		want  string
	}{
		"чужой модуль": {func(t *testing.T, root string) {
			writeMetadata(t, root, ConstantKind, constantObject, body)
			writeConstantModule(t, root, "Режим", project.ObjectModuleFile)
		}, "it keeps only object.yaml"},
		"посторонний файл": {func(t *testing.T, root string) {
			writeMetadata(t, root, ConstantKind, constantObject, body)
			writeConstantModule(t, root, "Режим", "заметки.txt")
		}, "it keeps only object.yaml"},
		"форма, которой нет": {func(t *testing.T, root string) {
			writeMetadata(t, root, ConstantKind, constantObject, body+"default_form: "+constantForm+"\n")
		}, "unknown common form"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			test.write(t, root)
			_, err := Load(root)
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

// What is checked about the new properties is what would make them mean
// something other than what they say.
func TestBrokenConstantPropertiesAreRefused(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ body, want string }{
		"истории данных не существует": {"data_history: иногда", "data_history must be use or dont-use"},
		"блокировки не существует":     {"data_lock: ручная", "data_lock must be automatic, managed or automatic-and-managed"},
		"форма нулевая":                {"default_form: 00000000-0000-0000-0000-000000000000", "default_form must be a non-zero UUID"},
		"пояснение не на языке": {"explanation: {\"d=e\": Modus}",
			"explanation.d=e is not a language code"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeConstant("object.yaml", strings.NewReader(`format: 1
id: `+constantObject+`
name: Режим
title: {ru: Режим}
types: [{kind: boolean}]
`+broken.body+`
`), metadataConfiguration())
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

// A constant in the composition of a separator whose separated data is used
// «независимо и совместно» is kept per data area, and the help gives its
// value manager the field that says which (КонстантаМенеджерЗначения.<Имя>.
// <Имя общего реквизита>; the same condition in КонстантаКлючЗначения and the
// tables Константа.<Имя> and Константа.<Имя>.Изменения). A separator used
// independently alone gives none. The expectation is the help's, written
// here. Defects caught: the field not given at all, as it was until 2.309;
// given by a separator used independently; not offered by ЭтотОбъект in the
// value module; written into the constant's file.
func TestAConstantTakesTheFieldOfASeparatorUsedIndependentlyAndSimultaneously(t *testing.T) {
	t.Parallel()
	const (
		value, use        = "da000000-0000-4000-8000-000000000001", "da000000-0000-4000-8000-000000000002"
		joint, separately = "da000000-0000-4000-8000-000000000003", "da000000-0000-4000-8000-000000000004"
	)
	root := metadataProject(t)
	writeMetadata(t, root, ConstantKind, constantObject, `format: 1
id: `+constantObject+`
name: ДатаСтарта
title: {ru: Дата старта}
types: [{kind: date}]
`)
	writeConstantModule(t, root, "ДатаСтарта", project.ValueModuleFile)
	writeMetadata(t, root, SessionParameterKind, value, `format: 1
id: `+value+`
name: ОбластьДанныхЗначение
title: {ru: Область данных значение}
types: [{kind: number, precision: 7}]
`)
	writeMetadata(t, root, SessionParameterKind, use, `format: 1
id: `+use+`
name: ОбластьДанныхИспользование
title: {ru: Область данных использование}
types: [{kind: boolean}]
`)
	separator := func(id, name, level string) string {
		return `format: 1
id: ` + id + `
name: ` + name + `
title: {ru: ` + name + `}
types: [{kind: number, precision: 7}]
auto_use: dont-use
content:
  - {metadata: ` + constantObject + `, use: use}
data_separation: separate
separated_data_use: ` + level + `
data_separation_value: ` + value + `
data_separation_use: ` + use + `
`
	}
	writeMetadata(t, root, CommonAttributeKind, joint, separator(joint, "ОбластьВспомогательная", "independently-and-simultaneously"))
	writeMetadata(t, root, CommonAttributeKind, separately, separator(separately, "ОбластьОсновная", "independently"))

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	constant, _ := catalog.Constant("ДатаСтарта")
	if len(constant.CommonAttributeFields) != 1 || constant.CommonAttributeFields[0].Name != "ОбластьВспомогательная" {
		t.Fatalf("the constant's fields are %+v, want the joint separator alone", constant.CommonAttributeFields)
	}
	members := map[string]bool{}
	for _, member := range catalog.ThisObjectMembers()[project.ModuleAddress{Kind: string(ConstantKind), Object: "ДатаСтарта", Module: kinds.ValueModule}] {
		members[member.Name.Russian] = true
	}
	if !members["ОбластьВспомогательная"] || members["ОбластьОсновная"] || !members["Значение"] {
		t.Fatalf("ЭтотОбъект of the value module offers %v", members)
	}
	written, err := os.ReadFile(filepath.Join(root, "metadata", string(ConstantKind), "ДатаСтарта", project.ObjectMetadataFile))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(written), "Область") {
		t.Fatalf("the field reached the constant's file:\n%s", written)
	}
}
