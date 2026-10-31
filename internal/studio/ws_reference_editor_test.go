package studio

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
)

// wsdlDescription is a description of a service of one operation in WSDL
// 1.1, written for the tests.
const wsdlDescription = `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://schemas.xmlsoap.org/wsdl/" xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/"
  xmlns:tns="http://example.org/stock" targetNamespace="http://example.org/stock" name="Stock">
  <message name="AskIn"><part name="item" type="xsd:string" xmlns:xsd="http://www.w3.org/2001/XMLSchema"/></message>
  <portType name="StockPort"><operation name="Ask"><input message="tns:AskIn"/></operation></portType>
  <service name="StockService"><port name="StockSoap" binding="tns:StockBinding"><soap:address location="http://example.org/stock"/></port></service>
</definitions>`

// TestAWSReferenceIsMadeByImport catches an import of a description of a
// service that makes what the prototype refuses, or loses what it takes
// (docs/block-00/ws-ssylka.md): a file makes WSСсылка1 with the description
// and the address file:///… of its full path; a page of the network keeps
// its address; an empty address, a missing file, a text that is not XML and
// a description of WSDL 2.0 are refused in the prototype's words and make
// nothing; «Импорт WSDL…» puts the new description and address in place,
// and a refused one leaves them; the palette does not change the address.
func TestAWSReferenceIsMadeByImport(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	files := t.TempDir()
	write := func(name, content string) string {
		t.Helper()
		file := filepath.Join(files, name)
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return file
	}
	stock := write("stock.wsdl", wsdlDescription)
	ctx := context.Background()
	made, err := workspace.ImportWSReference(ctx, WSImport{Address: stock})
	if err != nil {
		t.Fatal(err)
	}
	if made.Path != "metadata/ws-references/WSСсылка1/object.yaml" || made.Reference.LocationURL != "file://"+filepath.ToSlash(stock) || made.KindTitle != "WS-ссылка" {
		t.Fatalf("the reference made is %+v at %s", made.Reference, made.Path)
	}
	definition := filepath.Join(workspace.root, "metadata", "ws-references", "WSСсылка1", "definition.xml")
	if content, err := os.ReadFile(definition); err != nil || string(content) != wsdlDescription {
		t.Fatalf("the description kept is %q (%v)", content, err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) { _, _ = response.Write([]byte(wsdlDescription)) }))
	defer server.Close()
	fromNetwork, err := workspace.ImportWSReference(ctx, WSImport{Address: server.URL + "/stock?wsdl"})
	if err != nil {
		t.Fatal(err)
	}
	if fromNetwork.Reference.Name != "WSСсылка2" || fromNetwork.Reference.LocationURL != server.URL+"/stock?wsdl" {
		t.Errorf("the reference from the network is %+v", fromNetwork.Reference)
	}

	before, _ := os.ReadDir(filepath.Join(workspace.root, "metadata", "ws-references"))
	for _, refused := range []struct {
		address string
		want    error
	}{
		{"", errWSFileMissing},
		{filepath.Join(files, "missing.wsdl"), errWSFileMissing},
		{write("text.wsdl", "просто текст"), errWSNotWSDL},
		{write("wsdl20.wsdl", `<description xmlns="http://www.w3.org/ns/wsdl"/>`), errWSNotDefinitions},
		{write("foreign.wsdl", `<definitions xmlns="http://example.org/other"/>`), errWSNotDefinitions},
	} {
		if _, err := workspace.ImportWSReference(ctx, WSImport{Address: refused.address}); !errors.Is(err, refused.want) {
			t.Errorf("%q answered %v, want %v", refused.address, err, refused.want)
		}
	}
	if after, _ := os.ReadDir(filepath.Join(workspace.root, "metadata", "ws-references")); len(after) != len(before) {
		t.Errorf("the refused imports left %d references, want %d", len(after), len(before))
	}

	other := write("other.wsdl", strings.Replace(wsdlDescription, "StockService", "OtherService", 1))
	if _, err := workspace.ImportWSReference(ctx, WSImport{Path: made.Path, Revision: made.Revision, Address: write("bad.wsdl", "<oops")}); !errors.Is(err, errWSNotWSDL) {
		t.Errorf("a broken description answered %v", err)
	}
	if content, _ := os.ReadFile(definition); string(content) != wsdlDescription {
		t.Error("a refused import changed the description")
	}
	again, err := workspace.ImportWSReference(ctx, WSImport{Path: made.Path, Revision: made.Revision, Address: other})
	if err != nil {
		t.Fatal(err)
	}
	if content, _ := os.ReadFile(definition); !strings.Contains(string(content), "OtherService") || again.Reference.LocationURL != "file://"+filepath.ToSlash(other) {
		t.Errorf("«Импорт WSDL…» left %s", again.Reference.LocationURL)
	}
	if _, err := workspace.ImportWSReference(ctx, WSImport{Path: made.Path, Revision: made.Revision, Address: stock}); !errors.Is(err, ErrSourceChanged) {
		t.Errorf("an import over a reference read before answered %v", err)
	}

	edited := again.Reference
	edited.Comment, edited.LocationURL = "склад", "http://elsewhere"
	saved, err := workspace.SaveWSReferenceEditor(again.Path, edited, again.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Reference.Comment != "склад" || saved.Reference.LocationURL != again.Reference.LocationURL {
		t.Errorf("the palette wrote %+v", saved.Reference)
	}
	if _, err := metadata.Load(workspace.root); err != nil {
		t.Fatalf("the project with the references does not load: %v", err)
	}
}

// wsdlImporting is a description importing two schemas - one by an address
// relative to its own, one absolute - and naming the first twice.
const wsdlImporting = `<definitions xmlns="http://schemas.xmlsoap.org/wsdl/" targetNamespace="http://example.org/stock">
  <types>
    <xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.org/stock">
      <xs:import namespace="http://example.org/goods" schemaLocation="goods.xsd"/>
      <xs:import namespace="http://example.org/prices" schemaLocation="ABSOLUTE/prices.xsd"/>
    </xs:schema>
    <xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.org/more">
      <xs:import namespace="http://example.org/goods" schemaLocation="goods.xsd"/>
    </xs:schema>
  </types>
</definitions>`

// The schemas a description imports are kept as the prototype keeps them
// (ws-ssylka.md, «Выгрузки»: 5 of the 12 references of the exports keep
// «1.xsd» beside the description, named «file://1.xsd»): an import takes
// each schema by its address - relative to the description's or absolute -
// once, keeps it as «1.xsd», «2.xsd» in the order named, and names it so in
// the description; «Импорт WSDL…» of a description importing fewer takes
// the schemas no longer named away; a schema that cannot be read refuses
// the import - nothing made, nothing changed.
// Defects caught: a schema lost on import, as the model lost it before; a
// schema kept twice or under another name; the description left naming the
// address and not the file kept; a schema of the import before left over;
// a schema missing taken for a description that works.
func TestTheSchemasADescriptionImportsAreKeptBesideIt(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	served := map[string]string{"/goods.xsd": `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.org/goods"/>`,
		"/abs/prices.xsd": `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.org/prices"/>`}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/stock.wsdl":
			_, _ = response.Write([]byte(strings.ReplaceAll(wsdlImporting, "ABSOLUTE", server.URL+"/abs")))
		case "/one.wsdl":
			_, _ = response.Write([]byte(strings.Replace(strings.ReplaceAll(wsdlImporting, "ABSOLUTE", server.URL+"/abs"),
				`<xs:import namespace="http://example.org/goods" schemaLocation="goods.xsd"/>`, "", -1)))
		case "/broken.wsdl":
			_, _ = response.Write([]byte(strings.ReplaceAll(wsdlImporting, "goods.xsd", "missing.xsd")))
		default:
			content, ok := served[request.URL.Path]
			if !ok {
				http.NotFound(response, request)
				return
			}
			_, _ = response.Write([]byte(content))
		}
	}))
	defer server.Close()
	ctx := context.Background()
	made, err := workspace.ImportWSReference(ctx, WSImport{Address: server.URL + "/stock.wsdl"})
	if err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(workspace.root, "metadata", "ws-references", made.Reference.Name)
	read := func(name string) string {
		content, _ := os.ReadFile(filepath.Join(folder, name))
		return string(content)
	}
	if got := read("definition.xml"); strings.Count(got, `schemaLocation="file://1.xsd"`) != 2 || strings.Count(got, `schemaLocation="file://2.xsd"`) != 1 {
		t.Errorf("the description kept names %s", got)
	}
	if read("1.xsd") != served["/goods.xsd"] || read("2.xsd") != served["/abs/prices.xsd"] {
		t.Errorf("the schemas kept: %q, %q", read("1.xsd"), read("2.xsd"))
	}
	if _, err := metadata.Load(workspace.root); err != nil {
		t.Fatalf("the project with the schemas: %v", err)
	}
	again, err := workspace.ImportWSReference(ctx, WSImport{Path: made.Path, Revision: made.Revision, Address: server.URL + "/one.wsdl"})
	if err != nil {
		t.Fatal(err)
	}
	if read("1.xsd") != served["/abs/prices.xsd"] || read("2.xsd") != "" {
		t.Errorf("after the import of one schema: %q, %q", read("1.xsd"), read("2.xsd"))
	}
	before := read("definition.xml")
	if _, err := workspace.ImportWSReference(ctx, WSImport{Path: again.Path, Revision: again.Revision, Address: server.URL + "/broken.wsdl"}); !errors.Is(err, errWSFileMissing) {
		t.Errorf("a schema missing answered %v", err)
	}
	if read("definition.xml") != before || read("1.xsd") != served["/abs/prices.xsd"] {
		t.Error("a refused import changed the reference")
	}
	entries, _ := os.ReadDir(filepath.Join(workspace.root, "metadata", "ws-references"))
	if _, err := workspace.ImportWSReference(ctx, WSImport{Address: server.URL + "/broken.wsdl"}); !errors.Is(err, errWSFileMissing) {
		t.Errorf("a new reference of a schema missing answered %v", err)
	}
	if after, _ := os.ReadDir(filepath.Join(workspace.root, "metadata", "ws-references")); len(after) != len(entries) {
		t.Error("a refused import made a reference")
	}
}

// The schemas a schema imports are taken too, the whole chain, as the
// prototype takes them (ws-ssylka.md, «Цепочка схем, RPC, „Возможно
// пустое“»; series-2.603/wsdl/dump): a description importing a schema
// that imports another keeps «1.xsd» naming «file://2.xsd» and «2.xsd»; a
// schema is read by an address relative to the schema naming it. Where a
// description imports two and the first imports a third, the third is
// «3.xsd» - found after the description's own (not shot: question В4 of
// series 2.672) - and a schema named again, a chain going round too, is
// taken once. Defects caught: the schemas of a schema left out - the
// import failing at run time; a relative address read from the
// description's place; a schema kept twice; a chain going round fetched
// without end; the import inside a schema left naming the address.
func TestTheChainOfSchemasIsKept(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	schema := func(namespace string, imports ...string) string {
		text := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="` + namespace + `">`
		for _, location := range imports {
			text += `<xs:import namespace="urn:x" schemaLocation="` + location + `"/>`
		}
		return text + `</xs:schema>`
	}
	description := func(imports ...string) string {
		text := `<definitions xmlns="http://schemas.xmlsoap.org/wsdl/" targetNamespace="urn:svc"><types>`
		for _, location := range imports {
			text += `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:svc"><xs:import namespace="urn:x" schemaLocation="` + location + `"/></xs:schema>`
		}
		return text + `</types></definitions>`
	}
	served := map[string]string{
		"/chain.wsdl":  description("deep/a.xsd"),
		"/deep/a.xsd":  schema("urn:a", "b.xsd"),
		"/deep/b.xsd":  schema("urn:b"),
		"/branch.wsdl": description("deep/a2.xsd", "c.xsd"),
		"/deep/a2.xsd": schema("urn:a2", "b2.xsd"),
		"/deep/b2.xsd": schema("urn:b2", "a2.xsd"),
		"/c.xsd":       schema("urn:c", "deep/a2.xsd"),
		"/broken.wsdl": description("deep/a3.xsd"),
		"/deep/a3.xsd": schema("urn:a3", "missing.xsd"),
		"/deep/c3.xsd": schema("urn:c3"),
	}
	fetched := map[string]int{}
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		mu.Lock()
		fetched[request.URL.Path]++
		mu.Unlock()
		content, ok := served[request.URL.Path]
		if !ok {
			http.NotFound(response, request)
			return
		}
		_, _ = response.Write([]byte(content))
	}))
	defer server.Close()
	ctx := context.Background()
	kept := func(address string) map[string]string {
		t.Helper()
		made, err := workspace.ImportWSReference(ctx, WSImport{Address: server.URL + address})
		if err != nil {
			t.Fatalf("%s: %v", address, err)
		}
		folder := filepath.Join(workspace.root, "metadata", "ws-references", made.Reference.Name)
		entries, _ := os.ReadDir(folder)
		files := map[string]string{}
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".xsd") || entry.Name() == "definition.xml" {
				content, _ := os.ReadFile(filepath.Join(folder, entry.Name()))
				files[entry.Name()] = string(content)
			}
		}
		return files
	}
	chain := kept("/chain.wsdl")
	if len(chain) != 3 || chain["definition.xml"] != description("file://1.xsd") || chain["1.xsd"] != schema("urn:a", "file://2.xsd") || chain["2.xsd"] != schema("urn:b") {
		t.Errorf("the chain kept: %q", chain)
	}
	branch := kept("/branch.wsdl")
	if len(branch) != 4 || branch["definition.xml"] != description("file://1.xsd", "file://2.xsd") || branch["1.xsd"] != schema("urn:a2", "file://3.xsd") ||
		branch["2.xsd"] != schema("urn:c", "file://1.xsd") || branch["3.xsd"] != schema("urn:b2", "file://1.xsd") {
		t.Errorf("the branches kept: %q", branch)
	}
	if fetched["/deep/a2.xsd"] != 1 || fetched["/deep/b2.xsd"] != 1 {
		t.Errorf("a schema named again fetched %d and %d times, want once", fetched["/deep/a2.xsd"], fetched["/deep/b2.xsd"])
	}
	entries, _ := os.ReadDir(filepath.Join(workspace.root, "metadata", "ws-references"))
	if _, err := workspace.ImportWSReference(ctx, WSImport{Address: server.URL + "/broken.wsdl"}); !errors.Is(err, errWSFileMissing) {
		t.Errorf("a schema of a schema missing answered %v", err)
	}
	if after, _ := os.ReadDir(filepath.Join(workspace.root, "metadata", "ws-references")); len(after) != len(entries) {
		t.Error("a refused import made a reference")
	}
	if _, err := metadata.Load(workspace.root); err != nil {
		t.Fatalf("the project with the chains: %v", err)
	}
}
