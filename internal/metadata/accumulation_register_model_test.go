package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestDecodeAccumulationRegister(t *testing.T) {
	t.Parallel()
	registerID, dimensionID, resourceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	source := "format: 1\nid: " + registerID.String() + "\nname: ОстаткиТоваров\ntitle: {ru: Остатки товаров}\nkind: balance\n" +
		"dimensions:\n  - id: " + dimensionID.String() + "\n    name: Товар\n    title: {ru: Товар}\n    types: [{kind: string, length: 100}]\n" +
		"resources:\n  - id: " + resourceID.String() + "\n    name: Количество\n    title: {ru: Количество}\n    types: [{kind: number, precision: 15, scale: 3}]\n" +
		""
	configuration := project.Project{Format: 1, ID: uuid.MustNew(), Name: "Demo", Title: project.LocalizedText{"ru": "Demo"}, DefaultLanguage: "ru", Languages: []project.Language{{Code: "ru", Name: "Русский"}}}
	value, err := DecodeAccumulationRegister("register.yaml", strings.NewReader(source), configuration)
	if err != nil || value.ID != registerID || value.Kind != AccumulationRegisterBalance || len(value.Resources) != 1 {
		t.Fatalf("value=%+v error=%v", value, err)
	}
}

func TestDecodeAccumulationRegisterRejectsNonNumericResource(t *testing.T) {
	t.Parallel()
	registerID, resourceID := uuid.MustNew(), uuid.MustNew()
	source := "format: 1\nid: " + registerID.String() + "\nname: Продажи\ntitle: {ru: Продажи}\nkind: turnover\n" +
		"resources:\n  - id: " + resourceID.String() + "\n    name: Сумма\n    title: {ru: Сумма}\n    types: [{kind: string, length: 20}]\n"
	configuration := project.Project{Format: 1, ID: uuid.MustNew(), Name: "Demo", Title: project.LocalizedText{"ru": "Demo"}, DefaultLanguage: "ru", Languages: []project.Language{{Code: "ru", Name: "Русский"}}}
	if _, err := DecodeAccumulationRegister("register.yaml", strings.NewReader(source), configuration); err == nil || !strings.Contains(err.Error(), "exactly one number") {
		t.Fatalf("error=%v", err)
	}
}

func TestAccumulationRegisterSchemaHasMovementsAndTotals(t *testing.T) {
	t.Parallel()
	registerID, dimensionID, resourceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{AccumulationRegisters: []AccumulationRegisterDefinition{{
		ID: registerID, Name: "Остатки", Kind: AccumulationRegisterBalance,
		Dimensions: []RegisterDimension{{Attribute: Attribute{ID: dimensionID, Name: "Склад", Types: []Type{{Kind: StringType, Length: 50}}}}},
		Resources:  []Attribute{{ID: resourceID, Name: "Количество", Types: []Type{{Kind: NumberType, Precision: 15, Scale: 3}}}},
	}}}
	schema, err := catalog.ApplicationSchema()
	if err != nil || len(schema.Tables) != 2 {
		t.Fatalf("tables=%d error=%v", len(schema.Tables), err)
	}
	movement, _ := PhysicalAccumulationRegisterTable(registerID)
	totals, _ := PhysicalAccumulationRegisterTotalsTable(registerID)
	seen := map[string]bool{}
	for _, table := range schema.Tables {
		seen[table.Name] = true
	}
	if !seen[movement] || !seen[totals] {
		t.Fatalf("schema has tables %+v", seen)
	}
}

// The dimensions, resources and attributes of a register are one set of
// fields: a query names any of them by name and the import finds any by
// identifier, so no two may share either, whichever groups they are in. Both
// registers that keep the three groups are checked. Defects caught: a
// resource with the identifier of a dimension taken as a field of its own
// (no test held this before 2.372), and a name told apart by case alone.
func TestRegisterFieldsAreToldApart(t *testing.T) {
	t.Parallel()
	configuration := project.Project{Format: 1, ID: uuid.MustNew(), Name: "Demo", Title: project.LocalizedText{"ru": "Demo"}, DefaultLanguage: "ru", Languages: []project.Language{{Code: "ru", Name: "Русский"}}}
	field := func(group string, id uuid.UUID, name, types string) string {
		return group + ":\n  - id: " + id.String() + "\n    name: " + name + "\n    title: {ru: " + name + "}\n    types: [" + types + "]\n"
	}
	decoders := map[string]struct {
		head   string
		decode func(string) error
	}{
		"accumulation": {"kind: balance\n", func(source string) error {
			_, err := DecodeAccumulationRegister("register.yaml", strings.NewReader(source), configuration)
			return err
		}},
		"information": {"write_mode: independent\nperiodicity: none\n", func(source string) error {
			_, err := DecodeInformationRegister("register.yaml", strings.NewReader(source), configuration)
			return err
		}},
	}
	for kind, register := range decoders {
		shared := uuid.MustNew()
		for _, testCase := range []struct {
			name, resourceName string
			resourceID         uuid.UUID
			want               string
		}{
			{"one identifier", "Количество", shared, "resources.Количество UUID conflicts with dimensions.Товар"},
			{"one name in another case", "товар", uuid.MustNew(), "resources.товар conflicts with dimensions.Товар"},
		} {
			source := "format: 1\nid: " + uuid.MustNew().String() + "\nname: Остатки\ntitle: {ru: Остатки}\n" + register.head +
				field("dimensions", shared, "Товар", "{kind: string, length: 100}") +
				field("resources", testCase.resourceID, testCase.resourceName, "{kind: number, precision: 15, scale: 3}")
			if err := register.decode(source); err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Errorf("%s register, %s: error = %v, want %q", kind, testCase.name, err, testCase.want)
			}
		}
	}
}
