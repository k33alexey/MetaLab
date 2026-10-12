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
// from stdin, gives the first attribute every type that names an object, and
// prints what the picker offers and the attribute it ends with.
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
  model.setTypeKind('catalog', attribute.id, 0, 'catalog');
  ({offered: model.typeKinds.map(([kind, label]) => ({kind, label, namesObject: model.namesObject(kind)})), catalog: model.value()});
` + "`" + `, context)));
`

// The type picker of the catalog editor offers every type a stored attribute
// may hold that names an object of metadata, under the prototype's name, and
// what it builds the save accepts. Defect caught: the picker keeping a list of
// its own - it offered four kinds of the eleven, and an enumeration as
// "Перечисление" rather than ПеречислениеСсылка. The expectation is written
// out here from the help (8.3.27), not read from the server's list.
func TestCatalogEditorOffersEveryStoredReference(t *testing.T) {
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
		Offered []struct {
			Kind        metadata.TypeKind `json:"kind"`
			Label       string            `json:"label"`
			NamesObject bool              `json:"namesObject"`
		} `json:"offered"`
		Catalog metadata.CatalogDefinition `json:"catalog"`
	}
	if err := json.Unmarshal(output, &answer); err != nil {
		t.Fatalf("node answered %q: %v", output, err)
	}
	var references []string
	for _, item := range answer.Offered {
		if item.NamesObject {
			references = append(references, item.Label)
		}
	}
	slices.Sort(references)
	expected := []string{"БизнесПроцессСсылка", "ДокументСсылка", "ЗадачаСсылка", "ОпределяемыйТип", "ПеречислениеСсылка",
		"ПланВидовРасчетаСсылка", "ПланВидовХарактеристикСсылка", "ПланОбменаСсылка", "ПланСчетовСсылка", "СправочникСсылка",
		"ТочкаМаршрутаБизнесПроцессаСсылка"}
	if !slices.Equal(references, expected) {
		t.Errorf("the picker offers %v, a stored attribute may hold %v", references, expected)
	}
	attribute := answer.Catalog.Attributes[0]
	if attribute.Types[0].Kind != metadata.CatalogType || attribute.Types[0].Reference == nil || *attribute.Types[0].Reference != created.Catalog.ID {
		t.Fatalf("a reference to a catalog points at %+v, not at the only catalog", attribute.Types)
	}
	if _, err := workspace.SaveCatalogEditor(created.Path, answer.Catalog, created.Revision); err != nil {
		t.Fatalf("the save refuses what the picker built: %v", err)
	}
}
