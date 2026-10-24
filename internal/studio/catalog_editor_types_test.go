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
	output := runInNode(t, node, offerInNode, payload, "ui/catalog-editor.js")
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

// foldersInNode opens the window «Редактирование типа данных» on the offer read
// from stdin, in a composite type: unfolds the folders of the objects of
// catalogs and of the enumerations of the platform, marks a dynamic list, the
// object of the catalog named by argv[1] and the vertical alignment, and
// prints the rows inside the two folders, the description built and how the
// palette shows it; then opens the window anew on the vertical alignment
// alone and prints which folder is open and which row is current.
const foldersInNode = `
const {readFileSync} = await import('node:fs');
const vm = await import('node:vm');
const context = vm.createContext({structuredClone, crypto});
vm.runInContext(readFileSync('ui/studio-common.js', 'utf8'), context);
context.offer = JSON.parse(readFileSync(0, 'utf8'));
context.catalog = process.argv[1];
process.stdout.write(JSON.stringify(vm.runInContext(` + "`" + `
  const window = typeEditorModel(offer, []);
  const folder = caption => window.visible().find(row => row.caption === caption).key;
  window.setComposite(true);
  window.fold(folder('СправочникОбъект'), true);
  window.fold(folder('Системные перечисления'), true);
  window.mark('dynamic-list', true);
  window.mark('catalog-object:' + catalog, true);
  window.mark('vertical-align', true);
  const inside = window.visible().filter(row => row.depth === 1).map(row => row.caption);
  const types = window.result().types;
  const again = typeEditorModel(offer, [{kind: 'vertical-align'}]);
  ({inside, types, shown: typePresentation(offer, types),
    open: again.visible().filter(row => row.open).map(row => row.caption), current: again.current});
` + "`" + `, context)));
`

// The window offers an attribute of a form folders of two sorts - the objects
// of a kind, and the enumerations of the platform, types naming no object -
// and builds from both what the palette then shows by name. Defects caught:
// a folder of types drawn empty or its rows not marked, so the 24
// enumerations the prototype offers cannot be chosen; a type inside a folder
// shown by its kind rather than its name; a window opened on an enumeration
// leaving its folder closed. The expectation is written out from the opis
// (obshchee.md, «Системные перечисления по местам»), not read from the offer.
func TestTheTypeWindowOffersFoldersOfObjectsAndOfTypes(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for this test")
	}
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	created, err := workspace.CreateCatalog("Товары")
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := metadata.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	offer, _ := catalog.TypeOffer(metadata.FormAttributeChoice)
	payload, err := json.Marshal(offer)
	if err != nil {
		t.Fatal(err)
	}
	id := created.Catalog.ID
	output := runInNode(t, node, foldersInNode, payload, id.String())
	var answer struct {
		Inside  []string        `json:"inside"`
		Types   []metadata.Type `json:"types"`
		Shown   string          `json:"shown"`
		Open    []string        `json:"open"`
		Current string          `json:"current"`
	}
	if err := json.Unmarshal(output, &answer); err != nil {
		t.Fatalf("node answered %q: %v", output, err)
	}
	if len(answer.Inside) != 25 || answer.Inside[0] != "Товары" || answer.Inside[1] != "ВидДвиженияНакопления" ||
		answer.Inside[5] != "ВертикальноеПоложение" || answer.Inside[24] != "ТипУпорядочиванияШаблоновПоследовательностейАнализаДанных" {
		t.Errorf("inside the two folders the window shows %v", answer.Inside)
	}
	if len(answer.Types) != 3 || answer.Types[0].Kind != metadata.DynamicListType || answer.Types[1].Kind != metadata.CatalogObjectType ||
		answer.Types[1].Reference == nil || *answer.Types[1].Reference != id || answer.Types[2].Kind != metadata.VerticalAlignType {
		t.Errorf("the window built %+v", answer.Types)
	}
	if answer.Shown != "ДинамическийСписок, СправочникОбъект.Товары, ВертикальноеПоложение" {
		t.Errorf("the palette shows %q", answer.Shown)
	}
	if len(answer.Open) != 1 || answer.Open[0] != "Системные перечисления" || answer.Current != "vertical-align" {
		t.Errorf("opened on an enumeration, the window has open %v and current %q", answer.Open, answer.Current)
	}
}

// binaryInNode opens the window on the offer read from stdin with nothing
// marked: asks for the answer of «ОК» first, then unfolds the folder of the
// enumerations of the platform, marks binary data, gives it a length beyond
// the ceiling, then one of 16, fixed, and prints what each step answered.
const binaryInNode = `
const {readFileSync} = await import('node:fs');
const vm = await import('node:vm');
const context = vm.createContext({structuredClone, crypto});
vm.runInContext(readFileSync('ui/studio-common.js', 'utf8'), context);
context.offer = JSON.parse(readFileSync(0, 'utf8'));
process.stdout.write(JSON.stringify(vm.runInContext(` + "`" + `
  const window = typeEditorModel(offer, []);
  const empty = window.result().refusal;
  window.fold(window.visible().find(row => row.caption === 'Системные перечисления').key, true);
  const inside = window.visible().filter(row => row.depth === 1).map(row => row.caption);
  window.mark('binary-data', true);
  const fresh = structuredClone(window.qualifiers());
  const beyond = window.setQualifier('length', 4294967296);
  const ceiling = window.setQualifier('length', 4294967295);
  window.setQualifier('length', 16);
  window.setQualifier('fixedLength', true);
  ({empty, inside, fresh, beyond, ceiling, types: window.result().types});
` + "`" + `, context)));
`

// The window of the type of a session parameter requires a type, offers the
// three enumerations of accounting, and gives binary data a length, fixed or
// variable, under the model's ceiling - unlimited when marked anew (opis,
// parametr-seansa.md; obshchee.md, «Окно «Редактирование типа данных»»;
// snapshot tip-85). Defects caught: binary data offered without its
// qualifiers or a length past four bytes taken, which the model refuses; a
// session parameter left with no type; the 24 enumerations offered where the
// prototype offers three.
func TestTheTypeWindowOfASessionParameterGivesBinaryDataItsQualifiers(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for this test")
	}
	catalog, err := metadata.Load(createProject(t))
	if err != nil {
		t.Fatal(err)
	}
	offer, _ := catalog.TypeOffer(metadata.SessionParameterChoice)
	payload, err := json.Marshal(offer)
	if err != nil {
		t.Fatal(err)
	}
	output := runInNode(t, node, binaryInNode, payload)
	var answer struct {
		Empty  string   `json:"empty"`
		Inside []string `json:"inside"`
		Fresh  struct {
			Kind    string        `json:"kind"`
			Type    metadata.Type `json:"type"`
			Enabled bool          `json:"enabled"`
		} `json:"fresh"`
		Beyond  bool            `json:"beyond"`
		Ceiling bool            `json:"ceiling"`
		Types   []metadata.Type `json:"types"`
	}
	if err := json.Unmarshal(output, &answer); err != nil {
		t.Fatalf("node answered %q: %v", output, err)
	}
	if answer.Empty != "Необходимо выбрать тип значения." {
		t.Errorf("with nothing marked «ОК» answers %q", answer.Empty)
	}
	if !slices.Equal(answer.Inside, []string{"ВидДвиженияНакопления", "ВидСчета", "ВидДвиженияБухгалтерии"}) {
		t.Errorf("the folder of the enumerations of the platform holds %v", answer.Inside)
	}
	if answer.Fresh.Kind != "binary-data" || answer.Fresh.Type.Length != 0 || !answer.Fresh.Enabled {
		t.Errorf("binary data marked anew shows %+v", answer.Fresh)
	}
	if answer.Beyond || !answer.Ceiling {
		t.Errorf("a length of 4294967296 taken %v, of 4294967295 %v", answer.Beyond, answer.Ceiling)
	}
	if len(answer.Types) != 1 || answer.Types[0] != (metadata.Type{Kind: metadata.BinaryDataType, Length: 16, FixedLength: true}) {
		t.Errorf("the window built %+v", answer.Types)
	}
}

// runInNode runs a script of a module in Node.js from the package's folder,
// with input on stdin and the arguments given, and answers what it printed.
func runInNode(t *testing.T, node, script string, input []byte, arguments ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, node, append([]string{"--input-type=module", "-e", script}, arguments...)...)
	command.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, stderr.String())
	}
	return output
}

// singleInNode opens the window on the offer read from stdin with nothing
// marked, asks for «Составной тип данных», marks a number and then a string,
// and prints whether the window is composite and what it builds; and whether
// a window opened on a number and a string is composite.
const singleInNode = `
const {readFileSync} = await import('node:fs');
const vm = await import('node:vm');
const context = vm.createContext({structuredClone, crypto});
vm.runInContext(readFileSync('ui/studio-common.js', 'utf8'), context);
context.offer = JSON.parse(readFileSync(0, 'utf8'));
process.stdout.write(JSON.stringify(vm.runInContext(` + "`" + `
  const window = typeEditorModel(offer, []);
  window.setComposite(true);
  window.mark('number', true);
  window.mark('string', true);
  const opened = typeEditorModel(offer, [{kind: 'number', precision: 10}, {kind: 'string', length: 10}]);
  ({composite: window.composite, types: window.result().types.map(type => type.kind), opened: opened.composite});
` + "`" + `, context)));
`

// The window of a field of an external source has no «Составной тип
// данных»: asked for one it stays single, and a second mark takes the first
// away (opis of 24.10.2026, vneshnij-istochnik.md, snapshots
// series-2.531/16, 17, 22); the window of a function of a source has it
// (snapshots 19, 21). Defect caught: a field the window lets have two types,
// which the save then refuses («has a composite type»); a function that
// cannot be given a composite one.
func TestTheTypeWindowOfAFieldOfASourceIsSingle(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for this test")
	}
	for place, want := range map[metadata.TypeChoicePlace][]metadata.TypeKind{
		metadata.ExternalFieldChoice:    {metadata.StringType},
		metadata.ExternalFunctionChoice: {metadata.NumberType, metadata.StringType},
	} {
		offer, _ := (&metadata.Catalog{}).TypeOffer(place)
		payload, err := json.Marshal(offer)
		if err != nil {
			t.Fatal(err)
		}
		var answer struct {
			Composite bool                `json:"composite"`
			Types     []metadata.TypeKind `json:"types"`
			Opened    bool                `json:"opened"`
		}
		output := runInNode(t, node, singleInNode, payload)
		if err := json.Unmarshal(output, &answer); err != nil {
			t.Fatalf("node answered %q: %v", output, err)
		}
		if answer.Composite != (len(want) > 1) || answer.Opened != (len(want) > 1) || !slices.Equal(answer.Types, want) {
			t.Errorf("%s: composite %v, opened on two types composite %v, built %v, want %v", place, answer.Composite, answer.Opened, answer.Types, want)
		}
	}
}
