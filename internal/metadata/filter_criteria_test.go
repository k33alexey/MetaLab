package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

const (
	criterionID        = "d2000000-0000-4000-8000-000000000001"
	criterionContracts = "d2000000-0000-4000-8000-000000000002"
	criterionOrder     = "d2000000-0000-4000-8000-000000000003"
	criterionInvoice   = "d2000000-0000-4000-8000-000000000004"
	criterionContract  = "d2000000-0000-4000-8000-000000000010"
	criterionPart      = "d2000000-0000-4000-8000-000000000011"
	criterionPartField = "d2000000-0000-4000-8000-000000000012"
	criterionNumber    = "d2000000-0000-4000-8000-000000000013"
	criterionManager   = "d2000000-0000-4000-8000-000000000020"
	criterionListForm  = "d2000000-0000-4000-8000-000000000021"
	criterionAuxForm   = "d2000000-0000-4000-8000-000000000022"
)

// criterionProject writes a catalog of contracts and two documents that refer
// to it, one of them from a table part - the shape the reference configuration
// uses for its own criterion.
func criterionProject(t *testing.T) string {
	t.Helper()
	root := metadataProject(t)
	catalogNamed(t, root, criterionContracts, "Договоры")
	writeMetadata(t, root, DocumentKind, criterionOrder, `format: 1
id: `+criterionOrder+`
name: ЗаказПокупателя
title: {ru: Заказ покупателя}
number: {type: string, length: 9, auto: true, periodicity: none}
attributes:
  - {id: `+criterionContract+`, name: Договор, title: {ru: Договор}, types: [{kind: catalog, reference: `+criterionContracts+`}]}
  - {id: `+criterionNumber+`, name: НомерПоДанным, title: {ru: Номер по данным}, types: [{kind: string, length: 20}]}
`)
	writeMetadata(t, root, DocumentKind, criterionInvoice, `format: 1
id: `+criterionInvoice+`
name: СчетНаОплату
title: {ru: Счёт на оплату}
number: {type: string, length: 9, auto: true, periodicity: none}
table_parts:
  - id: `+criterionPart+`
    name: Основания
    title: {ru: Основания}
    attributes:
      - {id: `+criterionPartField+`, name: Договор, title: {ru: Договор}, types: [{kind: catalog, reference: `+criterionContracts+`}]}
`)
	return root
}

// A criterion is two halves: what may be searched for, and where it is
// searched. Both have to survive, and the second reaches inside a table part.
func TestFilterCriterionKeepsWhatIsSearchedAndWhere(t *testing.T) {
	t.Parallel()
	root := criterionProject(t)
	writeMetadata(t, root, FilterCriterionKind, criterionID, `format: 1
id: `+criterionID+`
name: СвязанныеДокументы
title: {ru: Связанные документы}
comment: От договора ко всем местам, где он использован
explanation: {ru: Где использован договор}
list_presentation: {ru: Связанные документы}
extended_list_presentation: {ru: Список связанных документов}
types: [{kind: catalog, reference: `+criterionContracts+`}]
use_standard_commands: true
forms: {list: ФормаСписка, auxiliary: Вспомогательная}
fields:
  - {kind: documents, object: `+criterionOrder+`, attribute: `+criterionContract+`}
  - {kind: documents, object: `+criterionInvoice+`, table_part: `+criterionPart+`, attribute: `+criterionPartField+`}
`)
	writeObjectModule(t, root, FilterCriterionKind, "СвязанныеДокументы", project.ManagerModuleFile)
	writeObjectForm(t, root, FilterCriterionKind, "СвязанныеДокументы", "ФормаСписка", criterionListForm)
	writeObjectForm(t, root, FilterCriterionKind, "СвязанныеДокументы", "Вспомогательная", criterionAuxForm)

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	criterion, ok := catalog.FilterCriterion("СвязанныеДокументы")
	if !ok {
		t.Fatal("the criterion did not load")
	}
	switch {
	case len(criterion.Types) != 1 || criterion.Types[0].Kind != CatalogType:
		t.Fatalf("what may be searched for was lost: %+v", criterion.Types)
	case len(criterion.Fields) != 2:
		t.Fatalf("where it is searched was lost: %+v", criterion.Fields)
	case criterion.Fields[1].TablePart == nil:
		t.Fatalf("a field inside a table part lost half of its address: %+v", criterion.Fields[1])
	case criterion.Forms.List == "" || criterion.Forms.Auxiliary == "":
		t.Fatalf("the forms were lost: %+v", criterion)
	case !criterion.UseStandardCommands || len(criterion.Explanation) == 0:
		t.Fatalf("the presentation settings were lost: %+v", criterion)
	}

	criterion.Fields[0].Object = mustUUID(t, criterionInvoice)
	again, _ := catalog.FilterCriterion("СвязанныеДокументы")
	if again.Fields[0].Object.String() != criterionOrder {
		t.Fatal("a criterion was handed out by reference")
	}
}

// A field that cannot hold what the criterion searches for is searched in
// vain: it never matches, and nobody is told why. The reference configuration
// keeps this exactly - every field of its criterion is typed by one of the
// criterion's own types.
func TestCriterionFieldMustBeAbleToHoldWhatIsSearched(t *testing.T) {
	t.Parallel()
	root := criterionProject(t)
	writeMetadata(t, root, FilterCriterionKind, criterionID, `format: 1
id: `+criterionID+`
name: СвязанныеДокументы
title: {ru: Связанные документы}
types: [{kind: catalog, reference: `+criterionContracts+`}]
fields:
  - {kind: documents, object: `+criterionOrder+`, attribute: `+criterionNumber+`}
`)
	_, err := Load(root)
	if err == nil {
		t.Fatal("a field that can never match was accepted")
	}
	if !strings.Contains(err.Error(), "can never match") {
		t.Fatalf("refused for another reason: %v", err)
	}
}

// What a criterion searches has to be there, and be what it says.
func TestBrokenFilterCriteriaAreRefused(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ body, want string }{
		"объекта не существует": {`types: [{kind: catalog, reference: ` + criterionContracts + `}]
fields: [{kind: documents, object: ` + criterionManager + `, attribute: ` + criterionContract + `}]`,
			"which is not in the configuration"},
		"реквизита не существует": {`types: [{kind: catalog, reference: ` + criterionContracts + `}]
fields: [{kind: documents, object: ` + criterionOrder + `, attribute: ` + criterionManager + `}]`,
			"which that object does not have"},
		"реквизит чужой табличной части": {`types: [{kind: catalog, reference: ` + criterionContracts + `}]
fields: [{kind: documents, object: ` + criterionInvoice + `, table_part: ` + criterionPart + `, attribute: ` + criterionContract + `}]`,
			"which that table part does not have"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := criterionProject(t)
			writeMetadata(t, root, FilterCriterionKind, criterionID, `format: 1
id: `+criterionID+`
name: Критерий
title: {ru: Критерий}
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
		"искать не по чему": {"types: []", "types must contain 1..32 types"},
		"одно поле дважды": {`types: [{kind: catalog, reference: ` + criterionContracts + `}]
fields:
  - {kind: documents, object: ` + criterionOrder + `, attribute: ` + criterionContract + `}
  - {kind: documents, object: ` + criterionOrder + `, attribute: ` + criterionContract + `}`,
			"is already among the fields"},
		"поле без реквизита": {`types: [{kind: catalog, reference: ` + criterionContracts + `}]
fields: [{kind: documents, object: ` + criterionOrder + `}]`, "attribute must be a non-zero UUID"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeFilterCriterion("object.yaml", strings.NewReader(`format: 1
id: `+criterionID+`
name: Критерий
title: {ru: Критерий}
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
