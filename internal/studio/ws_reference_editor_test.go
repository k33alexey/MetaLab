package studio

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
