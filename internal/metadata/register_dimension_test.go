package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A dimension of a register is a metadata object of its own in the prototype -
// «ОбъектМетаданных: Измерение» with 44 properties against «Реквизит» with 38 -
// and four of the extra ones had nowhere to live in our model, because a
// dimension was described by the same record as a catalog attribute. Putting
// them on that record would have given them to a catalog attribute, which does
// not have them in the prototype.
func TestInformationRegisterDimensionCarriesWhatOnlyADimensionHas(t *testing.T) {
	t.Parallel()
	registerID, dimensionID, resourceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	source := "format: 1\nid: " + registerID.String() + "\nname: Цены\ntitle: {ru: Цены}\nwrite_mode: independent\nperiodicity: none\n" +
		"dimensions:\n  - id: " + dimensionID.String() + "\n    name: Товар\n    title: {ru: Товар}\n" +
		"    types: [{kind: string, length: 100}]\n" +
		"    master: true\n    main_filter: true\n    deny_incomplete_values: true\n    type_reduction: transform-values\n" +
		"resources:\n  - {id: " + resourceID.String() + ", name: Цена, title: {ru: Цена}, types: [{kind: number, precision: 15, scale: 2}]}\n"
	value, err := DecodeInformationRegister("register.yaml", strings.NewReader(source), demoConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	dimension := value.Dimensions[0]
	if !dimension.Master || !dimension.MainFilter || !dimension.DenyIncompleteValues {
		t.Fatalf("dimension = %+v", dimension)
	}
	if dimension.TypeReduction != TypeReductionTransform {
		t.Fatalf("type reduction = %q", dimension.TypeReduction)
	}
	// And it is still a field: the type of the value and the name are where they
	// were, and nothing about the file moved.
	if dimension.Name != "Товар" || len(dimension.Types) != 1 {
		t.Fatalf("the dimension stopped being a field: %+v", dimension)
	}
}

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

	// «Использование в итогах» is an accumulation register's, and the register of
	// balances does not use it either: «для регистра накопления остатков это
	// свойство не используется».
	if err := accumulation("turnover", "    use_in_totals: true\n"); err != nil {
		t.Fatalf("a turnover register refused its own property: %v", err)
	}
	for name, err := range map[string]error{
		"использование в итогах у регистра сведений": information("    use_in_totals: true\n"),
		"использование в итогах у регистра остатков": accumulation("balance", "    use_in_totals: true\n"),
		"ведущее у регистра накопления":              accumulation("turnover", "    master: true\n"),
		"основной отбор у регистра накопления":       accumulation("turnover", "    main_filter: true\n"),
		"режим сокращения у регистра накопления":     accumulation("turnover", "    type_reduction: deny\n"),
		"неизвестный режим сокращения":               information("    type_reduction: иногда\n"),
	} {
		if err == nil {
			t.Fatalf("%s: accepted", name)
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
	err := entriesError(t, onResource)
	if err == nil || !strings.Contains(err.Error(), "deny_incomplete_values belongs to a dimension") {
		t.Fatalf("error = %v", err)
	}
}
