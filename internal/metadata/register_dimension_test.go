package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// «Использование в итогах» belongs to a dimension of an accumulation register,
// and the answer nobody wrote down is «use»: every dimension of the export says
// true, and a dimension left out of the totals by an omitted line would vanish
// from every report of that register.
func TestUseInTotalsIsUsedUnlessSaidOtherwise(t *testing.T) {
	t.Parallel()
	register := func(tail string) (AccumulationRegisterDefinition, error) {
		registerID, dimensionID, resourceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
		source := "format: 1\nid: " + registerID.String() + "\nname: Продажи\ntitle: {ru: Продажи}\nkind: turnover\n" +
			"dimensions:\n  - id: " + dimensionID.String() + "\n    name: Товар\n    title: {ru: Товар}\n" +
			"    types: [{kind: string, length: 100}]\n" + tail +
			"resources:\n  - {id: " + resourceID.String() + ", name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}\n"
		return DecodeAccumulationRegister("register.yaml", strings.NewReader(source), demoConfiguration())
	}
	silent, err := register("")
	if err != nil {
		t.Fatal(err)
	}
	if silent.Dimensions[0].UseInTotals != nil {
		t.Fatalf("an omitted line was read as a value: %+v", silent.Dimensions[0].UseInTotals)
	}
	if !silent.Dimensions[0].usesInTotals() {
		t.Fatal("a dimension nobody spoke about was left out of the totals")
	}
	refused, err := register("    use_in_totals: false\n")
	if err != nil {
		t.Fatal(err)
	}
	if refused.Dimensions[0].usesInTotals() {
		t.Fatal("a dimension said to stay out of the totals was used in them")
	}
}

// Each of the four belongs to the dimensions of one kind of register, and a
// property the owner does not have is worse than a missing one: it reads as a
// setting and changes nothing, so whoever wrote it goes looking for the effect.
func TestDimensionPropertiesBelongToTheirOwnRegister(t *testing.T) {
	t.Parallel()
	information := func(tail string) error {
		registerID, dimensionID, resourceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
		source := "format: 1\nid: " + registerID.String() + "\nname: Цены\ntitle: {ru: Цены}\nwrite_mode: independent\nperiodicity: none\n" +
			"dimensions:\n  - id: " + dimensionID.String() + "\n    name: Товар\n    title: {ru: Товар}\n" +
			"    types: [{kind: string, length: 100}]\n" + tail +
			"resources:\n  - {id: " + resourceID.String() + ", name: Цена, title: {ru: Цена}, types: [{kind: number, precision: 15, scale: 2}]}\n"
		_, err := DecodeInformationRegister("register.yaml", strings.NewReader(source), demoConfiguration())
		return err
	}
	accumulation := func(kind, tail string) error {
		registerID, dimensionID, resourceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
		source := "format: 1\nid: " + registerID.String() + "\nname: Продажи\ntitle: {ru: Продажи}\nkind: " + kind + "\n" +
			"dimensions:\n  - id: " + dimensionID.String() + "\n    name: Товар\n    title: {ru: Товар}\n" +
			"    types: [{kind: string, length: 100}]\n" + tail +
			"resources:\n  - {id: " + resourceID.String() + ", name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}\n"
		_, err := DecodeAccumulationRegister("register.yaml", strings.NewReader(source), demoConfiguration())
		return err
	}

	// «Использование в итогах» is an accumulation register's. A register of
	// balances does not use it and carries it all the same: the prototype
	// writes it on all 873 dimensions of balance registers.
	if err := accumulation("turnover", "    use_in_totals: true\n"); err != nil {
		t.Fatalf("a turnover register refused its own property: %v", err)
	}
	if err := accumulation("balance", "    use_in_totals: true\n"); err != nil {
		t.Fatalf("a balance register refused the property it carries: %v", err)
	}
	for name, err := range map[string]error{
		"использование в итогах у регистра сведений": information("    use_in_totals: true\n"),
		"ведущее у регистра накопления":              accumulation("turnover", "    master: true\n"),
		"основной отбор у регистра накопления":       accumulation("turnover", "    main_filter: true\n"),
		"режим сокращения у регистра накопления":     accumulation("turnover", "    type_reduction: deny\n"),
		"неизвестный режим сокращения":               information("    type_reduction: иногда\n"),
	} {
		if err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
	// The properties of an accounting and a calculation register belong to
	// them alone, and the dimension of the other kinds carries the words only
	// to refuse them with the reason.
	for name, testCase := range map[string]struct{ kind, lines string }{
		"базовое у регистра сведений":               {"information", "    base: true\n"},
		"связь с графиком у регистра накопления":    {"turnover", "    schedule_link: " + uuid.MustNew().String() + "\n"},
		"балансовое у регистра расчёта":             {"calculation", "    balance: true\n"},
		"признак учёта у регистра сведений":         {"information", "    accounting_flag: " + uuid.MustNew().String() + "\n"},
		"ведущее у регистра бухгалтерии":            {"accounting", "    master: true\n"},
		"использование в итогах у регистра расчёта": {"calculation", "    use_in_totals: true\n"},
		"режим сокращения у регистра бухгалтерии":   {"accounting", "    type_reduction: deny\n"},
	} {
		_, err := registerDimensionOf(testCase.kind, testCase.lines)
		if err == nil || !strings.Contains(err.Error(), "does not belong to a dimension of this register") {
			t.Fatalf("%s: error = %v", name, err)
		}
	}

	// «Запрещать незаполненные значения» belongs to a dimension of all four
	// registers, and to a resource of none: a resource holds an amount.
	fields := `dimensions:
  - {id: ` + entriesCompany + `, name: Организация, title: {ru: Организация}, types: [{kind: catalog, reference: ` + entriesCompanies + `}], deny_incomplete_values: true}
resources:
  - {id: ` + entriesSum + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}`
	if _, err := Load(entriesProject(t, true, fields)); err != nil {
		t.Fatalf("a dimension of an accounting register refused it: %v", err)
	}
	onResource := `dimensions:
  - {id: ` + entriesCompany + `, name: Организация, title: {ru: Организация}, types: [{kind: catalog, reference: ` + entriesCompanies + `}]}
resources:
  - {id: ` + entriesSum + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}], deny_incomplete_values: true}`
	// A resource of an accounting register has a structure of its own, and it
	// carries no such property: the strict reader refuses the word.
	err := entriesError(t, onResource)
	if err == nil || !strings.Contains(err.Error(), "field deny_incomplete_values not found") {
		t.Fatalf("error = %v", err)
	}
}

// registerDimensionOf reads a register of the given kind with one dimension
// whose lines are tail, and returns its dimensions. Each kind is read by its
// own decoder with the least the kind requires around the dimension.
func registerDimensionOf(kind, tail string) ([]RegisterDimension, error) {
	registerID, dimensionID, resourceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	dimension := "dimensions:\n  - id: " + dimensionID.String() + "\n    name: Товар\n    title: {ru: Товар}\n" +
		"    types: [{kind: string, length: 100}]\n" + tail
	resource := "resources:\n  - {id: " + resourceID.String() + ", name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}\n"
	head := "format: 1\nid: " + registerID.String() + "\nname: Регистр\ntitle: {ru: Регистр}\n"
	switch kind {
	case "information":
		value, err := DecodeInformationRegister("register.yaml", strings.NewReader(head+"write_mode: independent\nperiodicity: none\n"+dimension+resource), demoConfiguration())
		return value.Dimensions, err
	case "turnover", "balance":
		value, err := DecodeAccumulationRegister("register.yaml", strings.NewReader(head+"kind: "+kind+"\n"+dimension+resource), demoConfiguration())
		return value.Dimensions, err
	case "accounting":
		value, err := DecodeAccountingRegister("register.yaml", strings.NewReader(head+"chart_of_accounts: "+uuid.MustNew().String()+"\ncorrespondence: true\n"+dimension+resource), demoConfiguration())
		return value.Dimensions, err
	case "calculation":
		value, err := DecodeCalculationRegister("register.yaml", strings.NewReader(head+"chart_of_calculation_types: "+uuid.MustNew().String()+"\nperiodicity: month\naction_period: true\n"+
			"schedule: "+uuid.MustNew().String()+"\nschedule_value: "+uuid.MustNew().String()+"\nschedule_date: "+uuid.MustNew().String()+"\n"+dimension+resource), demoConfiguration())
		return value.Dimensions, err
	}
	panic("unknown kind of register " + kind)
}

// A dimension is one metadata object for all four kinds of register in the
// prototype, and it is one structure here: the common set every field has,
// ЗапрещатьНезаполненныеЗначения every dimension has, and the properties of
// its own kind. The defect on each row is the one the separate structures had
// before they were one: a kind whose dimension could not be given a tooltip or
// a quick choice, because its reader did not know the words - and a kind that
// lost its own property on the way to the shared structure.
func TestEveryRegisterDimensionCarriesTheCommonSetAndItsOwn(t *testing.T) {
	t.Parallel()
	common := "    comment: Общий набор\n    fill_checking: show-error\n    deny_incomplete_values: true\n" +
		"    presentation:\n      tooltip: {ru: Подсказка}\n    choice:\n      quick_choice: use\n"
	link := uuid.MustNew()
	for kind, own := range map[string]struct {
		lines string
		kept  func(RegisterDimension) bool
	}{
		"information": {"    master: true\n    main_filter: true\n    type_reduction: transform-values\n", func(dimension RegisterDimension) bool {
			return dimension.Master && dimension.MainFilter && dimension.TypeReduction == TypeReductionTransform
		}},
		"turnover": {"    use_in_totals: false\n", func(dimension RegisterDimension) bool {
			return dimension.UseInTotals != nil && !dimension.usesInTotals()
		}},
		"accounting": {"    balance: true\n    accounting_flag: " + link.String() + "\n", func(dimension RegisterDimension) bool {
			return dimension.Balance && dimension.AccountingFlag != nil && *dimension.AccountingFlag == link
		}},
		"calculation": {"    base: true\n    schedule_link: " + link.String() + "\n", func(dimension RegisterDimension) bool {
			return dimension.Base && dimension.ScheduleLink != nil && *dimension.ScheduleLink == link
		}},
	} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			dimensions, err := registerDimensionOf(kind, common+own.lines)
			if err != nil {
				t.Fatal(err)
			}
			dimension := dimensions[0]
			if dimension.Comment != "Общий набор" || dimension.FillChecking != ShowFillingError || !dimension.DenyIncompleteValues {
				t.Fatalf("the common set did not load: %+v", dimension)
			}
			if dimension.Presentation.ToolTip["ru"] != "Подсказка" || dimension.Choice.QuickChoice != UsageUse {
				t.Fatalf("presentation=%+v choice=%+v", dimension.Presentation, dimension.Choice)
			}
			if !own.kept(dimension) {
				t.Fatalf("the dimension lost the properties of its kind: %+v", dimension)
			}
			// A copy shares nothing with the original: a caller that changes
			// what it got must not change what the next caller gets.
			copied := cloneRegisterDimensions(dimensions)
			copied[0].Presentation.ToolTip["ru"] = "changed"
			for _, pointer := range []**uuid.UUID{&copied[0].AccountingFlag, &copied[0].ScheduleLink} {
				if *pointer != nil {
					**pointer = uuid.MustNew()
				}
			}
			if dimension.Presentation.ToolTip["ru"] != "Подсказка" || !own.kept(dimension) {
				t.Fatalf("the copy shared its presentation or a link with the original: %+v", dimension)
			}
		})
	}
}
