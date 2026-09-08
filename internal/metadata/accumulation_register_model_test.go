package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestDecodeAccumulationRegister(t *testing.T) {
	t.Parallel()
	registerID, recorderID, dimensionID, resourceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	source := "format: 1\nid: " + registerID.String() + "\nname: ОстаткиТоваров\ntitle: {ru: Остатки товаров}\nkind: balance\n" +
		"dimensions:\n  - id: " + dimensionID.String() + "\n    name: Товар\n    title: {ru: Товар}\n    types: [{kind: string, length: 100}]\n" +
		"resources:\n  - id: " + resourceID.String() + "\n    name: Количество\n    title: {ru: Количество}\n    types: [{kind: number, precision: 15, scale: 3}]\n" +
		"recorders: [" + recorderID.String() + "]\n"
	manifest := project.Project{Format: 1, ID: uuid.MustNew(), Name: "Demo", Title: "Demo", DefaultLanguage: "ru", Languages: []project.Language{{Code: "ru", Name: "Русский"}}}
	value, err := DecodeAccumulationRegister("register.yaml", strings.NewReader(source), manifest)
	if err != nil || value.ID != registerID || value.Kind != AccumulationRegisterBalance || len(value.Resources) != 1 {
		t.Fatalf("value=%+v error=%v", value, err)
	}
}

func TestDecodeAccumulationRegisterRejectsNonNumericResource(t *testing.T) {
	t.Parallel()
	registerID, recorderID, resourceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	source := "format: 1\nid: " + registerID.String() + "\nname: Продажи\ntitle: {ru: Продажи}\nkind: turnover\n" +
		"resources:\n  - id: " + resourceID.String() + "\n    name: Сумма\n    title: {ru: Сумма}\n    types: [{kind: string, length: 20}]\nrecorders: [" + recorderID.String() + "]\n"
	manifest := project.Project{Format: 1, ID: uuid.MustNew(), Name: "Demo", Title: "Demo", DefaultLanguage: "ru", Languages: []project.Language{{Code: "ru", Name: "Русский"}}}
	if _, err := DecodeAccumulationRegister("register.yaml", strings.NewReader(source), manifest); err == nil || !strings.Contains(err.Error(), "exactly one number") {
		t.Fatalf("error=%v", err)
	}
}

func TestAccumulationRegisterSchemaHasMovementsAndTotals(t *testing.T) {
	t.Parallel()
	registerID, recorderID, dimensionID, resourceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{AccumulationRegisters: []AccumulationRegisterDefinition{{
		ID: registerID, Name: "Остатки", Kind: AccumulationRegisterBalance, Recorders: []uuid.UUID{recorderID},
		Dimensions: []Attribute{{ID: dimensionID, Name: "Склад", Types: []Type{{Kind: StringType, Length: 50}}}},
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
