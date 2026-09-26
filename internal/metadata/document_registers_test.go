package metadata

import (
	"strings"
	"testing"
)

const (
	movementGoods       = "ca000000-0000-4000-8000-000000000001"
	movementDocument    = "ca000000-0000-4000-8000-000000000002"
	movementSecondDoc   = "ca000000-0000-4000-8000-000000000003"
	movementBalance     = "ca000000-0000-4000-8000-000000000010"
	movementBalanceGood = "ca000000-0000-4000-8000-000000000011"
	movementBalanceQty  = "ca000000-0000-4000-8000-000000000012"
	movementPrices      = "ca000000-0000-4000-8000-000000000020"
	movementPricesGood  = "ca000000-0000-4000-8000-000000000021"
	movementPricesValue = "ca000000-0000-4000-8000-000000000022"
	movementRates       = "ca000000-0000-4000-8000-000000000030"
	movementRatesKey    = "ca000000-0000-4000-8000-000000000031"
	movementRatesValue  = "ca000000-0000-4000-8000-000000000032"
	movementMissing     = "ca000000-0000-4000-8000-0000000000ff"
)

// movementProject writes two documents, one accumulation register, one
// information register written by a recorder and one written independently.
func movementProject(t *testing.T, first, second string) string {
	t.Helper()
	root := metadataProject(t)
	catalogNamed(t, root, movementGoods, "Номенклатура")
	writeMetadata(t, root, AccumulationRegisterKind, movementBalance, `format: 1
id: `+movementBalance+`
name: ОстаткиТоваров
title: {ru: Остатки товаров}
kind: balance
dimensions:
  - {id: `+movementBalanceGood+`, name: Номенклатура, title: {ru: Номенклатура}, types: [{kind: catalog, reference: `+movementGoods+`}]}
resources:
  - {id: `+movementBalanceQty+`, name: Количество, title: {ru: Количество}, types: [{kind: number, precision: 15, scale: 3}]}
`)
	writeMetadata(t, root, InformationRegisterKind, movementPrices, `format: 1
id: `+movementPrices+`
name: Цены
title: {ru: Цены}
write_mode: recorder
periodicity: recorder-position
dimensions:
  - {id: `+movementPricesGood+`, name: Номенклатура, title: {ru: Номенклатура}, types: [{kind: catalog, reference: `+movementGoods+`}]}
resources:
  - {id: `+movementPricesValue+`, name: Цена, title: {ru: Цена}, types: [{kind: number, precision: 15, scale: 2}]}
`)
	writeMetadata(t, root, InformationRegisterKind, movementRates, `format: 1
id: `+movementRates+`
name: КурсыВалют
title: {ru: Курсы валют}
write_mode: independent
periodicity: day
dimensions:
  - {id: `+movementRatesKey+`, name: Валюта, title: {ru: Валюта}, types: [{kind: string, length: 3}]}
resources:
  - {id: `+movementRatesValue+`, name: Курс, title: {ru: Курс}, types: [{kind: number, precision: 15, scale: 4}]}
`)
	writeMetadata(t, root, DocumentKind, movementDocument, `format: 1
id: `+movementDocument+`
name: Поступление
title: {ru: Поступление}
number: {type: string, length: 9, auto: true, periodicity: none}
posting: true
`+first+`
`)
	writeMetadata(t, root, DocumentKind, movementSecondDoc, `format: 1
id: `+movementSecondDoc+`
name: Продажа
title: {ru: Продажа}
number: {type: string, length: 9, auto: true, periodicity: none}
posting: true
`+second+`
`)
	return root
}

// The document names its registers, and the question "who writes into this
// register" is answered by reading the documents. Both directions have to
// work, because both are asked - by the form offering a command, and by the
// record store checking a recorder.
func TestDocumentNamesTheRegistersItWritesInto(t *testing.T) {
	t.Parallel()
	root := movementProject(t,
		"movements: ["+movementBalance+", "+movementPrices+"]",
		"movements: ["+movementBalance+"]")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	document, ok := catalog.DocumentDefinition("Поступление")
	if !ok || len(document.Movements) != 2 {
		t.Fatalf("the document lost the registers it writes into: found=%v %+v", ok, document.Movements)
	}
	targets := catalog.DocumentMovements(document.ID)
	if len(targets) != 2 || targets[0].Kind != AccumulationRegisterKind || targets[1].Kind != InformationRegisterKind {
		t.Fatalf("a register was resolved to the wrong kind: %+v", targets)
	}
	if targets[0].Name != "ОстаткиТоваров" || targets[1].Name != "Цены" {
		t.Fatalf("a register was resolved to the wrong object: %+v", targets)
	}

	balance, _ := catalog.AccumulationRegisterDefinition("ОстаткиТоваров")
	recorders := catalog.RegisterRecorders(balance.ID)
	if len(recorders) != 2 {
		t.Fatalf("both documents write into the register, and it answers with %d", len(recorders))
	}
	prices, _ := catalog.InformationRegisterDefinition("Цены")
	if got := catalog.RegisterRecorders(prices.ID); len(got) != 1 || got[0] != document.ID {
		t.Fatalf("one document writes into the register, and it answers with %+v", got)
	}
	rates, _ := catalog.InformationRegisterDefinition("КурсыВалют")
	if got := catalog.RegisterRecorders(rates.ID); len(got) != 0 {
		t.Fatalf("nobody writes into the independent register, and it answers with %+v", got)
	}

	second, _ := catalog.DocumentDefinition("Продажа")
	second.Movements[0] = mustUUID(t, movementPrices)
	again, _ := catalog.DocumentDefinition("Продажа")
	if again.Movements[0].String() != movementBalance {
		t.Fatal("a document's movements were handed out by reference")
	}
}

// A register named by nobody is legal - the prototype has no rule that a
// register must be written into, and a configuration under construction breaks
// that rule all the time.
func TestARegisterNobodyWritesIntoIsAllowed(t *testing.T) {
	t.Parallel()
	root := movementProject(t, "", "")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	balance, _ := catalog.AccumulationRegisterDefinition("ОстаткиТоваров")
	if got := catalog.RegisterRecorders(balance.ID); len(got) != 0 {
		t.Fatalf("a register nobody writes into answered with %+v", got)
	}
}

func TestBrokenDocumentMovementsAreRefused(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ first, want string }{
		"регистра нет в конфигурации": {"movements: [" + movementMissing + "]",
			"which is not a register of the configuration"},
		"регистр сведений пишется независимо": {"movements: [" + movementRates + "]",
			"written independently and keeps no recorder"},
		"движение в справочник": {"movements: [" + movementGoods + "]",
			"which is not a register of the configuration"},
		"один регистр дважды": {"movements: [" + movementBalance + ", " + movementBalance + "]",
			"movements[1] repeats"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := movementProject(t, broken.first, "")
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
