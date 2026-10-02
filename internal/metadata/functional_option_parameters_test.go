package metadata

import (
	"strings"
	"testing"
)

const (
	parameterID     = "d1000000-0000-4000-8000-000000000001"
	parameterSecond = "d1000000-0000-4000-8000-000000000002"
)

// A parameter is the axis along which the value of an option differs: turned
// on for one warehouse and off for another. It says where that axis appears -
// the object it stands for, and the dimensions of the registers that hold the
// value by it.
func TestFunctionalOptionParameterNamesTheAxisOfAnOption(t *testing.T) {
	t.Parallel()
	root := optionProject(t)
	writeMetadata(t, root, FunctionalOptionKind, optionID, `format: 1
id: `+optionID+`
name: УчётПоСкладу
title: {ru: Учёт по складу}
location: {kind: information-registers, object: `+optionRegister+`, element: `+optionResource+`}
`)
	writeMetadata(t, root, FunctionalOptionParameterKind, parameterID, `format: 1
id: `+parameterID+`
name: Склад
title: {ru: Склад}
comment: По какому складу читается значение опции
use:
  - {kind: catalogs, object: `+optionCatalog+`}
  - {kind: information-registers, object: `+optionRegister+`, element: `+optionDimension+`}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	parameter, ok := catalog.FunctionalOptionParameter("Склад")
	if !ok {
		t.Fatal("the parameter did not load")
	}
	switch {
	case parameter.Comment == "":
		t.Fatalf("the comment was lost: %+v", parameter)
	case len(parameter.Use) != 2:
		t.Fatalf("where the axis appears was lost: %+v", parameter.Use)
	case parameter.Use[0].Element != nil:
		t.Fatalf("an object the parameter stands for was given a field: %+v", parameter.Use[0])
	case parameter.Use[1].Element == nil || parameter.Use[1].Element.String() != optionDimension:
		t.Fatalf("the dimension of the register was lost: %+v", parameter.Use[1])
	}

	parameter.Use[0].Object = mustUUID(t, optionRegister)
	again, _ := catalog.FunctionalOptionParameter("Склад")
	if again.Use[0].Object.String() != optionCatalog {
		t.Fatal("a parameter was handed out by reference")
	}
}

// An option whose value depends on something it has no parameter for is
// saved by the prototype - 7, 20 and 2 of them in the three configurations
// being moved - and reading it is defined: parameters passed in part make a
// Boolean option the OR of the values they select (help,
// ПолучитьФункциональнуюОпцию). Defect caught: such an option was refused,
// and with it the whole configuration.
func TestOptionWithoutAParameterForWhatItDependsOnIsKept(t *testing.T) {
	t.Parallel()
	for name, shape := range map[string]struct{ option, parameter string }{
		"измерение регистра без параметра": {
			"location: {kind: information-registers, object: " + optionRegister + ", element: " + optionResource + "}", ""},
		"параметр стоит за другое измерение": {
			"location: {kind: information-registers, object: " + optionRegister + ", element: " + optionResource + "}",
			"use: [{kind: catalogs, object: " + optionCatalog + "}]"},
		"значение в реквизите справочника без параметра": {
			"location: {kind: catalogs, object: " + optionCatalog + ", element: " + optionAttribute + "}", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := optionProject(t)
			writeMetadata(t, root, FunctionalOptionKind, optionID, `format: 1
id: `+optionID+`
name: Опция
title: {ru: Опция}
`+shape.option+`
`)
			if shape.parameter != "" {
				writeMetadata(t, root, FunctionalOptionParameterKind, parameterID, `format: 1
id: `+parameterID+`
name: Параметр
title: {ru: Параметр}
`+shape.parameter+`
`)
			}
			if _, err := Load(root); err != nil {
				t.Fatalf("%s: refused: %v", name, err)
			}
		})
	}
}

// An option kept in a constant has one value and nothing to look it up by, so
// it needs no parameter at all.
func TestOptionInAConstantNeedsNoParameter(t *testing.T) {
	t.Parallel()
	root := optionProject(t)
	writeMetadata(t, root, FunctionalOptionKind, optionID, `format: 1
id: `+optionID+`
name: ИспользоватьСклады
title: {ru: Использовать склады}
location: {kind: constants, object: `+optionConstant+`}
`)
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}
}

// A parameter standing for something that is not there stands for nothing.
func TestBrokenFunctionalOptionParametersAreRefused(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ body, want string }{
		"объекта не существует": {"use: [{kind: catalogs, object: " + parameterSecond + "}]",
			"which is not in the configuration"},
		"поля не существует": {"use: [{kind: information-registers, object: " + optionRegister +
			", element: " + parameterSecond + "}]", "which is not a dimension of that information register"},
		// A parameter stands for a dimension (help, FunctionalOptionsParameter.Use).
		// Defect caught: a resource, which is where the value is kept, was
		// accepted as the axis it is looked up by.
		"ресурс вместо измерения": {"use: [{kind: information-registers, object: " + optionRegister +
			", element: " + optionResource + "}]", "which is not a dimension of that information register"},
		"регистра не существует": {"use: [{kind: information-registers, object: " + parameterSecond +
			", element: " + optionDimension + "}]", "which is not in the configuration"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := optionProject(t)
			writeMetadata(t, root, FunctionalOptionParameterKind, parameterID, `format: 1
id: `+parameterID+`
name: Параметр
title: {ru: Параметр}
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
	// The shape is checked before anything is resolved.
	for name, broken := range map[string]struct{ body, want string }{
		"вида объекта не существует": {"use: [{kind: слайды, object: " + optionCatalog + "}]",
			"kind must be catalogs or information-registers"},
		// Only a catalog or a dimension (help). Defect caught: any kind the
		// model knew was accepted, a document or a common command among them.
		"документ": {"use: [{kind: documents, object: " + optionCatalog + "}]",
			"kind must be catalogs or information-registers"},
		"общая команда": {"use: [{kind: common-commands, object: " + optionCatalog + "}]",
			"kind must be catalogs or information-registers"},
		"регистр накопления": {"use: [{kind: accumulation-registers, object: " + optionRegister + ", element: " + optionDimension + "}]",
			"kind must be catalogs or information-registers"},
		"реквизит справочника": {"use: [{kind: catalogs, object: " + optionCatalog + ", element: " + optionAttribute + "}]",
			"a parameter stands for a catalog as a whole"},
		"регистр без измерения": {"use: [{kind: information-registers, object: " + optionRegister + "}]",
			"a parameter stands for a dimension of the register"},
		// The help: the property has to be filled.
		"использования нет": {"comment: ни за что", "use must name at least one catalog or dimension"},
		"одно и то же дважды": {"use:\n  - {kind: catalogs, object: " + optionCatalog + "}\n  - {kind: catalogs, object: " +
			optionCatalog + "}", "is already in the use"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeFunctionalOptionParameter("parameter.yaml", strings.NewReader(`format: 1
id: `+parameterID+`
name: Параметр
title: {ru: Параметр}
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
