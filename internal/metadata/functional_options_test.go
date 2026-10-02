package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
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
`)
	writeCommandModule(t, root, CatalogKind, "Номенклатура", "ПоказатьОстатки")
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

// The value of an option lives in a constant, in an attribute of a catalog or
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
			optionRegister + `, element: ` + optionSecond + `}`, "which is not a resource of that information register"},
		// A dimension is what the value is looked up by, not where it is kept
		// (help, FunctionalOption.Location: a resource of the register).
		"значение лежит в измерении регистра": {`location: {kind: information-registers, object: ` +
			optionRegister + `, element: ` + optionDimension + `}`, "which is not a resource of that information register"},
		"значение лежит в реквизите табличной части": {`location: {kind: catalogs, object: ` +
			optionCatalog + `, element: ` + optionPartField + `}`, "which is not an attribute of that catalog"},
		"значение лежит в несуществующем справочнике": {`location: {kind: catalogs, object: ` +
			optionSecond + `, element: ` + optionAttribute + `}`, "which is not in the configuration"},
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
		// Only a constant, a catalog and an information register hold a value
		// (help, FunctionalOption.Location). Defect caught: a document, a chart
		// and a task were accepted as places the platform never reads from.
		"значение в реквизите документа": {"location: {kind: documents, object: " + optionCatalog + ", element: " + optionAttribute + "}",
			"cannot hold the value of a functional option"},
		"значение в плане видов характеристик": {"location: {kind: charts-of-characteristic-types, object: " + optionCatalog + ", element: " + optionAttribute + "}",
			"cannot hold the value of a functional option"},
		"значение в задаче": {"location: {kind: tasks, object: " + optionCatalog + ", element: " + optionAttribute + "}",
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

// The content of an option is any object of the metadata tree and its parts
// (help, the Content tab of a functional option). Every shape below is one the
// three configurations being moved switch. Defect caught: a common command
// (234/74/62), a common form (25/9/10), a filter criterion (3/1/1), a part of
// an accounting or calculation register and an accounting flag were not kinds
// an option could name, and the option was refused.
func TestFunctionalOptionSwitchesEveryKindThePrototypeDoes(t *testing.T) {
	t.Parallel()
	id := func() uuid.UUID { return uuid.MustNew() }
	command, form, criterion, criterionCommand := id(), id(), id(), id()
	accounting, accountingDimension, accountingResource, accountingAttribute, accountingCommand := id(), id(), id(), id(), id()
	calculation, calculationDimension, calculationResource, calculationAttribute := id(), id(), id(), id()
	chart, flag, subcontoFlag := id(), id(), id()
	parent, child := id(), id()
	constant := id()
	field := func(id uuid.UUID) Attribute { return Attribute{ID: id} }
	catalog := &Catalog{
		constantByID:           map[uuid.UUID]int{constant: 0},
		commonCommandByID:      map[uuid.UUID]int{command: 0},
		objectKindByID:         map[uuid.UUID]string{form: commonFormObjectKind},
		filterCriterionByID:    map[uuid.UUID]int{criterion: 0},
		FilterCriteria:         []FilterCriterionDefinition{{ID: criterion, Commands: []ObjectCommand{{ID: criterionCommand}}}},
		accountingRegisterByID: map[uuid.UUID]int{accounting: 0},
		AccountingRegisters: []AccountingRegisterDefinition{{ID: accounting,
			Dimensions: []RegisterDimension{{Attribute: field(accountingDimension)}},
			Resources:  []AccountingRegisterResource{{Attribute: field(accountingResource)}},
			Attributes: []Attribute{field(accountingAttribute)}, Commands: []ObjectCommand{{ID: accountingCommand}}}},
		calculationRegisterByID: map[uuid.UUID]int{calculation: 0},
		CalculationRegisters: []CalculationRegisterDefinition{{ID: calculation,
			Dimensions: []RegisterDimension{{Attribute: field(calculationDimension)}},
			Resources:  []Attribute{field(calculationResource)},
			Attributes: []CalculationRegisterAttribute{{Attribute: field(calculationAttribute)}}}},
		chartOfAccountsByID: map[uuid.UUID]int{chart: 0},
		ChartsOfAccounts: []ChartOfAccountsDefinition{{ID: chart,
			AccountingFlags: []AccountingFlag{{Attribute: field(flag)}}, ExtDimensionAccountingFlags: []AccountingFlag{{Attribute: field(subcontoFlag)}}}},
		subsystemByID: map[uuid.UUID]int{parent: 0, child: 1},
		Subsystems:    []SubsystemDefinition{{ID: parent}, {ID: child, Parent: &parent}},
	}
	part := func(kind Kind, object, element uuid.UUID) FunctionalOptionItem {
		return FunctionalOptionItem{Kind: kind, Object: object, Element: &element}
	}
	content := []FunctionalOptionItem{
		{Kind: CommonCommandKind, Object: command},
		{Kind: CommonFormKind, Object: form},
		{Kind: FilterCriterionKind, Object: criterion},
		part(FilterCriterionKind, criterion, criterionCommand),
		{Kind: AccountingRegisterKind, Object: accounting},
		part(AccountingRegisterKind, accounting, accountingDimension),
		part(AccountingRegisterKind, accounting, accountingResource),
		part(AccountingRegisterKind, accounting, accountingAttribute),
		part(AccountingRegisterKind, accounting, accountingCommand),
		{Kind: CalculationRegisterKind, Object: calculation},
		part(CalculationRegisterKind, calculation, calculationDimension),
		part(CalculationRegisterKind, calculation, calculationResource),
		part(CalculationRegisterKind, calculation, calculationAttribute),
		part(ChartOfAccountsKind, chart, flag),
		part(ChartOfAccountsKind, chart, subcontoFlag),
		// A subsystem inside a subsystem is switched by itself.
		{Kind: SubsystemKind, Object: child},
	}
	for _, item := range content {
		if !knownMetadataKind(item.Kind) {
			t.Fatalf("%s is not a kind an option may name", item.Kind)
		}
	}
	location := FunctionalOptionLocation{Kind: ConstantKind, Object: constant}
	catalog.FunctionalOptions = []FunctionalOptionDefinition{{Name: "Опция", Location: location, Content: content}}
	if err := catalog.validateFunctionalOptions(); err != nil {
		t.Fatalf("the prototype's content was refused: %v", err)
	}
	// Each of them still has to be there: a part the object does not have and
	// an object the configuration does not have are refused, for every kind.
	for index, item := range content {
		for name, broken := range map[string]FunctionalOptionItem{
			"object": {Kind: item.Kind, Object: id(), Element: item.Element},
			"part":   part(item.Kind, item.Object, id()),
		} {
			catalog.FunctionalOptions[0].Content = []FunctionalOptionItem{broken}
			if err := catalog.validateFunctionalOptions(); err == nil {
				t.Fatalf("content[%d] %s with a missing %s was accepted", index, item.Kind, name)
			}
		}
	}
	// A flag is a part of the chart that only an option points at: it is not
	// an attribute anything else may read.
	if elements, _ := catalog.objectElementsOf(ChartOfAccountsKind, chart); elements.has(flag) {
		t.Fatal("an accounting flag became an attribute of the chart")
	}
}
