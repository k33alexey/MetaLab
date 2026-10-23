package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"slices"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/metadata"
)

// offerInNode builds the catalog model of catalog-editor.js on the source read
// from stdin and the window «Редактирование типа данных» on what the server
// offers, marks in a composite type a number, the set of catalogs and the only
// catalog, gives them to the first attribute and prints the captions of the
// rows and the attribute it ends with.
const offerInNode = `
const {readFileSync} = await import('node:fs');
const vm = await import('node:vm');
const context = vm.createContext({structuredClone, crypto});
vm.runInContext(readFileSync('ui/studio-common.js', 'utf8'), context);
vm.runInContext(readFileSync(process.argv[1], 'utf8'), context);
context.source = JSON.parse(readFileSync(0, 'utf8'));
process.stdout.write(JSON.stringify(vm.runInContext(` + "`" + `
  const model = createCatalogModel(source);
  const attribute = model.addAttribute('catalog');
  const window = typeEditorModel(model.typeOffer, attribute.types);
  window.setComposite(true);
  window.mark('number', true);
  window.mark('catalog-ref', true);
  window.fold('catalog-ref', true);
  window.mark('catalog:' + source.catalog.id, true);
  window.mark('string', false);
  model.setAttributeTypes('catalog', attribute.id, window.result().types);
  ({rows: window.visible().map(row => row.caption), catalog: model.value()});
` + "`" + `, context)));
`

// The window offers an attribute of a catalog what the prototype's offers a
// stored field, and what it builds the save accepts. Defects caught: the
// editor keeping a list of its own - it offered four kinds of the eleven, and
// an enumeration as "Перечисление" rather than ПеречислениеСсылка - and a set
// or a type naming an object the save refuses. The expectation is written out
// from the opis (docs/block-00/obshchee.md, «Состав по местам»), not read
// from the server's offer.
func TestCatalogEditorOffersWhatAStoredFieldHolds(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for this test")
	}
	workspace, err := Open(createProject(t))
	if err != nil {
		t.Fatal(err)
	}
	created, err := workspace.CreateCatalog("Товары")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(created)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, node, "--input-type=module", "-e", offerInNode, "ui/catalog-editor.js")
	command.Stdin = bytes.NewReader(payload)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, stderr.String())
	}
	var answer struct {
		Rows    []string                   `json:"rows"`
		Catalog metadata.CatalogDefinition `json:"catalog"`
	}
	if err := json.Unmarshal(output, &answer); err != nil {
		t.Fatalf("node answered %q: %v", output, err)
	}
	expected := []string{"Число", "Строка", "Дата", "Булево", "ХранилищеЗначения", "УникальныйИдентификатор",
		"СправочникСсылка", "Товары", "ДокументСсылка", "ПеречислениеСсылка", "ПланВидовХарактеристикСсылка", "ПланСчетовСсылка",
		"ПланВидовРасчетаСсылка", "БизнесПроцессСсылка", "ТочкаМаршрутаБизнесПроцессаСсылка", "ЗадачаСсылка", "ПланОбменаСсылка",
		"Характеристика", "ОпределяемыйТип", "ЛюбаяСсылка"}
	if !slices.Equal(answer.Rows, expected) {
		t.Errorf("the window shows\n%v\na stored field is offered\n%v", answer.Rows, expected)
	}
	types := answer.Catalog.Attributes[0].Types
	id := created.Catalog.ID
	built := []metadata.Type{{Kind: metadata.NumberType, Precision: 10}, {Kind: metadata.CatalogSet}, {Kind: metadata.CatalogType, Reference: &id}}
	if len(types) != len(built) || types[0] != built[0] || types[1] != built[1] || types[2].Kind != built[2].Kind ||
		types[2].Reference == nil || *types[2].Reference != id {
		t.Fatalf("the window built %+v, not %+v", types, built)
	}
	if _, err := workspace.SaveCatalogEditor(created.Path, answer.Catalog, created.Revision); err != nil {
		t.Fatalf("the save refuses what the window built: %v", err)
	}
}
