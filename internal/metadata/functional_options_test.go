package metadata

import (
	"strings"
	"testing"
)

const (
	optionID        = "d0000000-0000-4000-8000-000000000001"
	optionConstant  = "d0000000-0000-4000-8000-000000000002"
	optionCatalog   = "d0000000-0000-4000-8000-000000000003"
	optionRegister  = "d0000000-0000-4000-8000-000000000004"
	optionAttribute = "d0000000-0000-4000-8000-000000000010"
	optionPart      = "d0000000-0000-4000-8000-000000000011"
	optionPartField = "d0000000-0000-4000-8000-000000000012"
	optionCommand   = "d0000000-0000-4000-8000-000000000013"
	optionCommandMd = "d0000000-0000-4000-8000-000000000014"
	optionDimension = "d0000000-0000-4000-8000-000000000015"
	optionResource  = "d0000000-0000-4000-8000-000000000016"
	optionSecond    = "d0000000-0000-4000-8000-000000000020"
	optionParameter = "d0000000-0000-4000-8000-000000000021"
)

// optionProject writes an object rich enough to be switched off piece by
// piece, plus the two places an option keeps its value.
func optionProject(t *testing.T) string {
	t.Helper()
	root := metadataProject(t)
	writeMetadata(t, root, ConstantKind, optionConstant, `format: 1
id: `+optionConstant+`
name: ИспользоватьСклады
title: {ru: Использовать склады}
types: [{kind: boolean}]
`)
	writeMetadata(t, root, CatalogKind, optionCatalog, `format: 1
id: `+optionCatalog+`
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
attributes:
  - {id: `+optionAttribute+`, name: Склад, title: {ru: Склад}, types: [{kind: string, length: 50}]}
table_parts:
  - id: `+optionPart+`
    name: Остатки
    title: {ru: Остатки}
    attributes:
      - {id: `+optionPartField+`, name: Количество, title: {ru: Количество}, types: [{kind: number, precision: 15, scale: 3}]}
commands:
  - id: `+optionCommand+`
    name: ПоказатьОстатки
    title: {ru: Показать остатки}
    module: `+optionCommandMd+`
`)
	writeCommandModule(t, root, CatalogKind, "Номенклатура", optionCommandMd)
	writeMetadata(t, root, InformationRegisterKind, optionRegister, `format: 1
id: `+optionRegister+`
name: НастройкиСкладов
title: {ru: Настройки складов}
write_mode: independent
periodicity: none
dimensions:
  - {id: `+optionDimension+`, name: Склад, title: {ru: Склад}, types: [{kind: catalog, reference: `+optionCatalog+`}]}
resources:
  - {id: `+optionResource+`, name: Использовать, title: {ru: Использовать}, types: [{kind: boolean}]}
`)
	return root
}

// A functional option switches a part of the application off, and the user's
// answer lives in the application's data rather than in the configuration.
// Both halves have to survive: where the value is kept, and what it switches.
func TestFunctionalOptionKeepsWhereItsValueLivesAndWhatItSwitches(t *testing.T) {
	t.Parallel()
	root := optionProject(t)
	writeMetadata(t, root, FunctionalOptionKind, optionID, `format: 1
id: `+optionID+`
name: ИспользоватьСклады
title: {ru: Использовать склады}
comment: Управляет видимостью складского учёта
location: {kind: constants, object: `+optionConstant+`}
privileged_get_mode: true
content:
  - {kind: catalogs, object: `+optionCatalog+`}
  - {kind: catalogs, object: `+optionCatalog+`, element: `+optionAttribute+`}
  - {kind: catalogs, object: `+optionCatalog+`, table_part: `+optionPart+`}
  - {kind: catalogs, object: `+optionCatalog+`, table_part: `+optionPart+`, element: `+optionPartField+`}
  - {kind: catalogs, object: `+optionCatalog+`, element: `+optionCommand+`}
  - {kind: information-registers, object: `+optionRegister+`, element: `+optionResource+`}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	option, ok := catalog.FunctionalOption("ИспользоватьСклады")
	if !ok {
		t.Fatal("the functional option did not load")
	}
	switch {
	case option.Location.Kind != ConstantKind || option.Location.Object.String() != optionConstant:
		t.Fatalf("where the value lives was lost: %+v", option.Location)
	case option.Location.Element != nil:
		t.Fatalf("a constant was given a field inside it: %+v", option.Location)
	case !option.PrivilegedGetMode:
		t.Fatalf("reading the value past the user's rights was lost: %+v", option)
	case len(option.Content) != 6:
		t.Fatalf("what the option switches was lost: %+v", option.Content)
	case option.Content[3].TablePart == nil || option.Content[3].Element == nil:
		t.Fatalf("an attribute of a table part lost half of its address: %+v", option.Content[3])
	}

	// The catalog hands out copies of an option too.
	option.Content[0].Object = mustUUID(t, optionRegister)
	again, _ := catalog.FunctionalOption("ИспользоватьСклады")
	if again.Content[0].Object.String() != optionCatalog {
		t.Fatal("a functional option was handed out by reference")
	}
}

// The value of an option lives in a constant, in an attribute of an object or
// in a resource of an information register. The third is how one application
// answers differently for different warehouses - the dimensions of the
// register say what the answer depends on.
func TestFunctionalOptionValueLivesInAFieldOfARegister(t *testing.T) {
	t.Parallel()
	root := optionProject(t)
	writeMetadata(t, root, FunctionalOptionKind, optionID, `format: 1
id: `+optionID+`
name: УчётПоСкладу
title: {ru: Учёт по складу}
location: {kind: information-registers, object: `+optionRegister+`, element: `+optionResource+`}
content:
  - {kind: catalogs, object: `+optionCatalog+`, element: `+optionAttribute+`}
`)
	// The value differs per warehouse, so something has to say which one:
	// a parameter standing for that dimension of the register.
	writeMetadata(t, root, FunctionalOptionParameterKind, optionParameter, `format: 1
id: `+optionParameter+`
name: Склад
title: {ru: Склад}
use:
  - {kind: information-registers, object: `+optionRegister+`, element: `+optionDimension+`}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	option, ok := catalog.FunctionalOption("УчётПоСкладу")
	if !ok {
		t.Fatal("the functional option did not load")
	}
	if option.Location.Element == nil || option.Location.Element.String() != optionResource {
		t.Fatalf("the field holding the value was lost: %+v", option.Location)
	}
}

// An option pointing at something that is not there switches nothing and says
// so nowhere: it simply never turns anything off, and the developer finds out
// long afterwards.
func TestFunctionalOptionsPointingNowhereAreRefused(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ body, want string }{
		"значение лежит в несуществующем объекте": {`location: {kind: constants, object: ` + optionSecond + `}`,
			"which is not in the configuration"},
		"значение лежит в несуществующем поле": {`location: {kind: information-registers, object: ` +
			optionRegister + `, element: ` + optionSecond + `}`, "which that information-registers does not have"},
		"переключается несуществующий объект": {`location: {kind: constants, object: ` + optionConstant + `}
content: [{kind: catalogs, object: ` + optionSecond + `}]`, "which is not in the configuration"},
		"переключается несуществующий реквизит": {`location: {kind: constants, object: ` + optionConstant + `}
content: [{kind: catalogs, object: ` + optionCatalog + `, element: ` + optionSecond + `}]`,
			"which that object does not have"},
		"переключается реквизит чужой табличной части": {`location: {kind: constants, object: ` + optionConstant + `}
content: [{kind: catalogs, object: ` + optionCatalog + `, table_part: ` + optionPart + `, element: ` + optionAttribute + `}]`,
			"which that table part does not have"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := optionProject(t)
			writeMetadata(t, root, FunctionalOptionKind, optionID, `format: 1
id: `+optionID+`
name: Опция
title: {ru: Опция}
`+broken.body+`
`)
			_, err := Load(root)
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

// What is checked in the option itself is the shape of what it says, before
// anything is resolved against the configuration.
func TestBrokenFunctionalOptionsAreRefused(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ body, want string }{
		"место хранения не названо": {"location: {object: " + optionConstant + "}", "location.kind is required"},
		"константе указано поле внутри неё": {"location: {kind: constants, object: " + optionConstant +
			", element: " + optionAttribute + "}", "a constant holds the value itself"},
		"регистру поле не указано": {"location: {kind: information-registers, object: " + optionRegister + "}",
			"location.element is required"},
		"значение негде хранить": {"location: {kind: reports, object: " + optionCatalog + "}",
			"cannot hold the value of a functional option"},
		"вида объекта не существует": {"location: {kind: constants, object: " + optionConstant + "}\n" +
			"content: [{kind: слайды, object: " + optionCatalog + "}]", "is not a kind of metadata object"},
		"одно и то же дважды": {"location: {kind: constants, object: " + optionConstant + "}\n" +
			"content:\n  - {kind: catalogs, object: " + optionCatalog + "}\n  - {kind: catalogs, object: " +
			optionCatalog + "}", "is already in the content"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeFunctionalOption("option.yaml", strings.NewReader(`format: 1
id: `+optionID+`
name: Опция
title: {ru: Опция}
`+broken.body+`
`), metadataManifest())
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}
