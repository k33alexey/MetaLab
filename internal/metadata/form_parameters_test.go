package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/k33alexey/MetaLab/internal/project"
)

// A form keeps its parameters through YAML and the Studio: the name, the
// type - a primitive, a reference, a set, a type of a vanished object, or
// none, which is arbitrary - and whether the parameter is a key.
//
// Defect caught: the 13943 parameters of the exports lost, so that a form
// opened with a filter or a key opens empty; a parameter with no type (5950)
// refused or read as a string; a key parameter (1127) read as an ordinary
// one, so that the same form opens twice instead of being activated; the
// type of a vanished object refused, so that the form carrying it is not
// moved.
func TestAFormKeepsItsParameters(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	source := formAttrHead + "parameters:\n" +
		"  - {name: Ключ, types: [{kind: catalog, reference: c0de0000-0000-4000-8000-000000990001}], key: true}\n" +
		"  - {name: Отбор}\n" +
		"  - {name: Владелец, types: [{kind: any-ref}]}\n" +
		"  - {name: Период, types: [{kind: standard-period}]}\n" +
		"  - {name: Текст, types: [{kind: string, length: 100}, {kind: vanished-type, reference: c0de0000-0000-4000-8000-000000990002}]}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check := func(source string, parameters []FormParameter) {
		t.Helper()
		switch {
		case len(parameters) != 5:
			t.Fatalf("%s: %+v", source, parameters)
		case parameters[0].Name != "Ключ" || !parameters[0].Key || len(parameters[0].Types) != 1 || parameters[0].Types[0].Kind != CatalogType:
			t.Fatalf("%s: the key parameter: %+v", source, parameters[0])
		case parameters[1].Key || len(parameters[1].Types) != 0:
			t.Fatalf("%s: a parameter of any type: %+v", source, parameters[1])
		case parameters[2].Types[0].Kind != AnyReferenceSet || parameters[3].Types[0].Kind != StandardPeriodType:
			t.Fatalf("%s: a set and a value type: %+v %+v", source, parameters[2], parameters[3])
		case len(parameters[4].Types) != 2 || parameters[4].Types[1].Kind != VanishedType:
			t.Fatalf("%s: a vanished type: %+v", source, parameters[4])
		}
	}
	check("read", form.Parameters)
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check("written back", again.Parameters)
	carried, err := json.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	var received ManagedForm
	if err := json.Unmarshal(carried, &received); err != nil {
		t.Fatal(err)
	}
	if err := ValidateManagedForm("studio", received, configuration); err != nil {
		t.Fatal(err)
	}
	check("carried through the Studio", received.Parameters)
	if !reflect.DeepEqual(received.Parameters, form.Parameters) || !reflect.DeepEqual(again.Parameters, form.Parameters) {
		t.Fatal("the parameters changed on the way")
	}
}

// What is wrong in a parameter is refused, naming it.
//
// Defect caught: two parameters of one name differing in case, which code
// cannot tell apart; a name that is no identifier; a type the place does not
// allow or with qualifiers of another kind; no name at all.
func TestAFormParameterRefusesWhatIsWrong(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for name, test := range map[string]struct{ parameters, want string }{
		"повтор имени":    {"  - {name: Ключ}\n  - {name: КЛЮЧ}\n", "parameters[1].name must be unique within the parameters"},
		"имя с пробелом":  {"  - {name: Ключ объекта}\n", "parameters[0].name must be a valid identifier of at most 255 characters"},
		"без имени":       {"  - {types: [{kind: boolean}]}\n", "parameters[0].name must be a valid identifier"},
		"квалификаторы":   {"  - {name: Сумма, types: [{kind: string, precision: 3}]}\n", "parameters[0].types"},
		"вид неизвестный": {"  - {name: Сумма, types: [{kind: money}]}\n", "parameters[0].types"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrHead+"parameters:\n"+test.parameters), configuration)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// The types of the parameters of a form are resolved with the project, as
// the types of its attributes are: a type naming an object the project does
// not have is refused, naming the form and the parameter.
//
// Defect caught: a parameter of a catalog that was deleted loading clean, so
// that opening the form with it fails only when it runs.
func TestTheTypesOfFormParametersAreResolved(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		reference string
		gone      bool
	}{
		"справочник есть": {reference: cmpGoods},
		"справочника нет": {reference: refGone, gone: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, string(content)+"parameters:\n  - {name: Товар, types: [{kind: catalog, reference: "+test.reference+"}], key: true}\n")
			if test.gone {
				want := "catalog Номенклатура form ФормаЭлемента parameter Товар references unknown catalog " + refGone
				if _, err := Load(root); err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("err = %v, want %q", err, want)
				}
				return
			}
			if _, err := Load(root); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// A common form resolves the types of its parameters too.
//
// Defect caught: the parameters of the common forms, read apart from the
// forms of objects, left out of the resolution, so that a common form
// opened with a deleted catalog loads clean.
func TestTheTypesOfCommonFormParametersAreResolved(t *testing.T) {
	t.Parallel()
	root := formReferencesProject(t)
	writeCommonForm(t, root, "Подбор", "format: 1\nid: c0de0000-0000-4000-8000-000000990097\nname: Подбор\ntitle: {ru: Подбор}\nkind: common\n"+
		"parameters:\n  - {name: Товар, types: [{kind: catalog, reference: "+refGone+"}]}\n")
	want := "common form Подбор parameter Товар references unknown catalog " + refGone
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %v, want %q", err, want)
	}
}
