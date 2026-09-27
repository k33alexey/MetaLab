package metadata

import (
	"fmt"
	"strings"
	"testing"
)

const (
	basisCatalog  = "ba510000-0000-4000-8000-000000000001"
	basisReceipt  = "ba510000-0000-4000-8000-000000000002"
	basisSale     = "ba510000-0000-4000-8000-000000000003"
	basisProcess  = "ba510000-0000-4000-8000-000000000004"
	basisRegister = "ba510000-0000-4000-8000-000000000005"
	basisColumn   = "ba510000-0000-4000-8000-000000000010"
	basisMissing  = "ba510000-0000-4000-8000-0000000000ff"
)

// basisProject writes what a new object may be made out of: a catalog, a
// document, and a register - which is none of those, because a register has no
// object to open and hand to the filling event.
func basisProject(t *testing.T) string {
	t.Helper()
	root := metadataProject(t)
	catalogNamed(t, root, basisCatalog, "Файлы")
	writeMetadata(t, root, DocumentKind, basisReceipt, `format: 1
id: `+basisReceipt+`
name: Поступление
title: {ru: Поступление}
number: {type: string, length: 11, auto: true, periodicity: year}
`)
	writeMetadata(t, root, InformationRegisterKind, basisRegister, `format: 1
id: `+basisRegister+`
name: Цены
title: {ru: Цены}
write_mode: independent
periodicity: day
dimensions:
  - {id: `+basisColumn+`, name: Файл, title: {ru: Файл}, types: [{kind: catalog, reference: `+basisCatalog+`}]}
resources:
  - {id: ba510000-0000-4000-8000-000000000011, name: Цена, title: {ru: Цена}, types: [{kind: number, precision: 15, scale: 2}]}
`)
	return root
}

// saleBasedOn writes the document under test, which is the one being made out
// of something else.
func saleBasedOn(body string) string {
	return `format: 1
id: ` + basisSale + `
name: Реализация
title: {ru: Реализация}
number: {type: string, length: 11, auto: true, periodicity: year}
` + body
}

// An object names what it may be made out of, and the list keeps the order it
// was written in and the kinds it names.
func TestObjectSaysWhatItIsMadeOutOf(t *testing.T) {
	t.Parallel()
	root := basisProject(t)
	writeMetadata(t, root, DocumentKind, basisSale, saleBasedOn(`based_on: [`+basisReceipt+`, `+basisCatalog+`]
`))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	sale, ok := catalog.DocumentDefinition("Реализация")
	if !ok || len(sale.BasedOn) != 2 {
		t.Fatalf("what the document is made out of was lost: found=%v %+v", ok, sale.BasedOn)
	}
	bases := catalog.EnteredOnBasisOf(sale.ID)
	if len(bases) != 2 ||
		bases[0].Kind != DocumentKind || bases[0].Name != "Поступление" ||
		bases[1].Kind != CatalogKind || bases[1].Name != "Файлы" {
		t.Fatalf("the bases were resolved wrong: %+v", bases)
	}

	sale.BasedOn[0] = mustUUID(t, basisCatalog)
	again, _ := catalog.DocumentDefinition("Реализация")
	if again.BasedOn[0].String() != basisReceipt {
		t.Fatal("the list of bases was handed out by reference")
	}
}

// The property belongs to every kind that has an object of its own to make -
// the five reference kinds and the three numbered ones. They reach it through
// two shared shapes, so this guards the wiring rather than each kind's code.
func TestEveryKindWithAnObjectSaysWhatItIsMadeOutOf(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]struct {
		kind    Kind
		content string
	}{
		"справочник": {CatalogKind, `code: {type: string, length: 9, auto: true}
description_length: 150
`},
		"план видов характеристик": {ChartOfCharacteristicTypesKind, `code: {type: string, length: 9, auto: true}
description_length: 100
value_type: [{kind: string, length: 100}]
`},
		"план счетов": {ChartOfAccountsKind, `code: {type: string, length: 5, auto: false}
description_length: 120
`},
		"план видов расчёта": {ChartOfCalculationTypesKind, `code: {type: string, length: 9, auto: true}
description_length: 100
`},
		"план обмена": {ExchangePlanKind, `code: {type: string, length: 36, auto: false}
description_length: 150
`},
		"документ": {DocumentKind, `number: {type: string, length: 11, auto: true, periodicity: year}
`},
		"бизнес-процесс": {BusinessProcessKind, `number: {type: string, length: 11, auto: true, periodicity: year}
`},
		"задача": {TaskKind, `number: {type: string, length: 11, auto: true, periodicity: none}
description_length: 150
addressing_attributes: []
`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := basisProject(t)
			writeMetadata(t, root, body.kind, basisSale, `format: 1
id: `+basisSale+`
name: Объект
title: {ru: Объект}
`+body.content+`based_on: [`+basisReceipt+`]
`)
			catalog, err := Load(root)
			if err != nil {
				t.Fatalf("a kind that can be made out of something refused to say so: %v", err)
			}
			made := catalog.BasisFor(mustUUID(t, basisReceipt))
			if len(made) != 1 || made[0].Kind != body.kind {
				t.Fatalf("the object was not read back as made out of the document: %+v", made)
			}
		})
	}
}

// A call is entered on the basis of a call: three documents of the
// demonstration configuration name themselves, so naming yourself is not a
// mistake to be refused but the normal way of saying "another one like this".
func TestAnObjectMayBeMadeOutOfItself(t *testing.T) {
	t.Parallel()
	root := basisProject(t)
	writeMetadata(t, root, DocumentKind, basisSale, saleBasedOn(`based_on: [`+basisSale+`, `+basisReceipt+`]
`))
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a document made out of itself was refused: %v", err)
	}
	sale, _ := catalog.DocumentDefinition("Реализация")
	bases := catalog.EnteredOnBasisOf(sale.ID)
	if len(bases) != 2 || bases[0].ID != sale.ID {
		t.Fatalf("the document lost itself as a basis: %+v", bases)
	}
	if made := catalog.BasisFor(sale.ID); len(made) != 1 || made[0].ID != sale.ID {
		t.Fatalf("the document is a basis for itself and was not read back as one: %+v", made)
	}
}

// The other list of the tab - what may be made out of this object - is the same
// links read backwards. It is not stored anywhere, and it answers across kinds.
func TestWhatIsMadeOutOfAnObjectIsReadBackwards(t *testing.T) {
	t.Parallel()
	root := basisProject(t)
	writeMetadata(t, root, DocumentKind, basisSale, saleBasedOn(`based_on: [`+basisReceipt+`]
`))
	writeMetadata(t, root, BusinessProcessKind, basisProcess, `format: 1
id: `+basisProcess+`
name: Задание
title: {ru: Задание}
number: {type: string, length: 11, auto: true, periodicity: year}
based_on: [`+basisReceipt+`]
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	made := catalog.BasisFor(mustUUID(t, basisReceipt))
	if len(made) != 2 || made[0].Kind != DocumentKind || made[1].Kind != BusinessProcessKind {
		t.Fatalf("what may be made out of the document was read wrong: %+v", made)
	}
	if made := catalog.BasisFor(mustUUID(t, basisCatalog)); len(made) != 0 {
		t.Fatalf("nothing is made out of the catalog, and the answer says %+v", made)
	}
	if bases := catalog.EnteredOnBasisOf(mustUUID(t, basisReceipt)); len(bases) != 0 {
		t.Fatalf("the document is made out of nothing, and the answer says %+v", bases)
	}
}

// Each of these describes a command that would never appear, and nobody would
// be told why: a basis that is not in the configuration, one that is not an
// object anybody can have open, the same one twice, and a list that is not a
// list of objects at all.
func TestBrokenBasisIsRefused(t *testing.T) {
	t.Parallel()
	crowd := make([]string, 0, maxBasedOnObjects+1)
	for index := range maxBasedOnObjects + 1 {
		crowd = append(crowd, fmt.Sprintf("ba510000-0000-4000-8000-%012d", index+1000))
	}
	for name, broken := range map[string]struct{ body, want string }{
		"основания нет в конфигурации": {`based_on: [` + basisMissing + `]`,
			"which is not an object of the configuration that can be a basis"},
		"основанием не может быть регистр": {`based_on: [` + basisRegister + `]`,
			"which is not an object of the configuration that can be a basis"},
		"одно основание дважды": {`based_on: [` + basisReceipt + `, ` + basisReceipt + `]`,
			"based_on[1] repeats"},
		"пустой идентификатор": {`based_on: [00000000-0000-0000-0000-000000000000]`,
			"based_on[0] must be a non-zero UUID"},
		"оснований больше, чем бывает списком": {`based_on: [` + strings.Join(crowd, ", ") + `]`,
			"must not contain more than 256 objects"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := basisProject(t)
			writeMetadata(t, root, DocumentKind, basisSale, saleBasedOn(broken.body+"\n"))
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
