package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	firstWSReference  = "f6000000-0000-4000-8000-000000000001"
	secondWSReference = "f6000000-0000-4000-8000-000000000002"
)

// wsdl11 is the smallest description of a service in WSDL 1.1: enough to be one,
// and nothing that a test of the reference itself would need to read.
const wsdl11 = `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://schemas.xmlsoap.org/wsdl/" name="Склад" targetNamespace="http://example.org/stock">
  <service name="Склад"/>
</definitions>
`

// writeWSReference writes one reference: its description, and the service
// description beside it when the test gives one.
func writeWSReference(t *testing.T, root, id, name, body, definition string) {
	t.Helper()
	writeMetadata(t, root, WSReferenceKind, id, `format: 1
id: `+id+`
name: `+name+`
title: {ru: `+name+`}
`+body)
	if definition == "" {
		return
	}
	path := filepath.Join(root, "metadata", string(WSReferenceKind), name, WSReferenceDefinitionFile)
	if err := os.WriteFile(path, []byte(definition), 0o644); err != nil {
		t.Fatal(err)
	}
}

// loadRefused loads a project that must be refused and returns the error text.
func loadRefused(t *testing.T, root, what string) string {
	t.Helper()
	_, err := Load(root)
	if err == nil {
		t.Fatal(what + " was accepted")
	}
	return err.Error()
}

// A reference is the address it was imported from and the description it was
// imported with. Catches a kind that is still read by identity alone - the
// address would be refused as an unknown property - and a reference that is
// read but not found by the name or identifier code and other objects use.
func TestWSReferenceCarriesItsAddressAndIsFoundByNameAndIdentifier(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeWSReference(t, root, firstWSReference, "Склад", "comment: Остатки у поставщика\nlocation_url: https://stock.example.org/ws/stock?wsdl\n", wsdl11)
	writeWSReference(t, root, secondWSReference, "Банк",
		"", `<description xmlns="http://www.w3.org/ns/wsdl" targetNamespace="http://example.org/bank"/>`)

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.WSReferences) != 2 {
		t.Fatalf("a reference was lost: %+v", catalog.WSReferences)
	}
	item, ok := catalog.WSReference("склад")
	if !ok || item.LocationURL != "https://stock.example.org/ws/stock?wsdl" || item.Comment != "Остатки у поставщика" {
		t.Fatalf("the reference is not found by name or lost its properties: %+v", item)
	}
	if found, ok := catalog.WSReferenceByID(item.ID); !ok || found.Name != "Склад" {
		t.Fatalf("the reference is not found by identifier: %+v", found)
	}
	// The address is not required: a description taken from a file has none
	// worth keeping, and the reference works without it.
	if bank, ok := catalog.WSReference("Банк"); !ok || bank.LocationURL != "" {
		t.Fatalf("a reference without an address came back as %+v", bank)
	}
	if len(catalog.OutlinedObjectsOf(WSReferenceKind)) != 0 {
		t.Fatal("a WS reference is still read as an object known by identity alone")
	}
}

// A reference without its description is a client with nothing to call, and
// the prototype creates one only by importing a description. Catches a check
// that walks the folder and finds nothing wrong in a folder that holds too
// little.
func TestWSReferenceWithoutADescriptionIsRefused(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeWSReference(t, root, firstWSReference, "Склад", "", "")
	message := loadRefused(t, root, "a reference without a description")
	if !strings.Contains(message, "has no "+WSReferenceDefinitionFile) {
		t.Fatalf("the error does not say what is missing: %v", message)
	}
}

// The description is checked for being one, and read through to its end.
// Catches a check that stops at the root - a file cut off half way has a
// perfectly good root - and one that accepts any XML under the right file name.
func TestWSReferenceDescriptionMustBeWholeWSDL(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		definition string
		says       string
	}{
		"cut off half way": {wsdl11[:len(wsdl11)-30], "not well-formed XML"},
		"not XML at all":   {"Склад: остатки", "holds no document"},
		"XML but not WSDL": {`<schema xmlns="http://www.w3.org/2001/XMLSchema"/>`, "root element is {http://www.w3.org/2001/XMLSchema}schema"},
		// The right local name in no namespace is not WSDL either: the
		// namespace is what says which language the document is in.
		"definitions without a namespace": {`<definitions name="Склад"/>`, "root element is {}definitions"},
		"only a declaration":              {`<?xml version="1.0"?>`, "holds no document"},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeWSReference(t, root, firstWSReference, "Склад", "", test.definition)
			message := loadRefused(t, root, "a broken description")
			if !strings.Contains(message, test.says) || !strings.Contains(message, "WS reference Склад") {
				t.Fatalf("the error does not say what is wrong and where: %v", message)
			}
		})
	}
}

// The folder holds the two descriptions and nothing else. Catches a file put
// beside them - a second description under another name, say - which nothing
// would ever read and which would look like the one in use.
func TestWSReferenceFolderHoldsNothingElse(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeWSReference(t, root, firstWSReference, "Склад", "", wsdl11)
	stray := filepath.Join(root, "metadata", string(WSReferenceKind), "Склад", "definition-old.xml")
	if err := os.WriteFile(stray, []byte(wsdl11), 0o644); err != nil {
		t.Fatal(err)
	}
	message := loadRefused(t, root, "a stray file beside a reference")
	if !strings.Contains(message, `"definition-old.xml"`) {
		t.Fatalf("the error does not name the file: %v", message)
	}
}

// The address is checked in form and not in sense. Catches both directions: a
// line break, which would split the property when written back, is refused; an
// address that is not a URL at all - a path on somebody's disk - is accepted,
// because nothing dereferences it.
func TestWSReferenceAddressIsCheckedInFormOnly(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeWSReference(t, root, firstWSReference, "Склад", `location_url: "C:\\Обмен\\stock.wsdl"`+"\n", wsdl11)
	if _, err := Load(root); err != nil {
		t.Fatalf("an address that is a file path was refused: %v", err)
	}

	root = metadataProject(t)
	writeWSReference(t, root, firstWSReference, "Склад", `location_url: "https://stock.example.org/\nws"`+"\n", wsdl11)
	if message := loadRefused(t, root, "an address with a line break"); !strings.Contains(message, "location_url must be one line") {
		t.Fatalf("the error does not name the property: %v", message)
	}

	root = metadataProject(t)
	writeWSReference(t, root, firstWSReference, "Склад", "location_url: https://"+strings.Repeat("a", maxLocationURLLength)+"\n", wsdl11)
	if message := loadRefused(t, root, "an address longer than the bound"); !strings.Contains(message, "location_url must not exceed") {
		t.Fatalf("the error does not name the bound: %v", message)
	}
}

// Reading is strict for this kind as for every other: a property we do not
// describe stops the load with its name. Catches a decoder that was given the
// structure but not the strictness.
func TestAnUnknownPropertyOfAWSReferenceIsReportedNotIgnored(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeWSReference(t, root, firstWSReference, "Склад", "endpoint: СкладSoap\n", wsdl11)
	if message := loadRefused(t, root, "a property nobody described"); !strings.Contains(message, "endpoint") {
		t.Fatalf("the error does not name the property: %v", message)
	}
}

// A reference lies in a folder named after it. Catches a loader that takes the
// name from the description and never looks at the folder - two references
// could then claim one folder, and the one Studio opens would not be the one
// that runs.
func TestWSReferenceLiesInAFolderOfItsName(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeWSReference(t, root, firstWSReference, "Склад", "", wsdl11)
	from := filepath.Join(root, "metadata", string(WSReferenceKind), "Склад")
	if err := os.Rename(from, filepath.Join(filepath.Dir(from), "Остатки")); err != nil {
		t.Fatal(err)
	}
	if message := loadRefused(t, root, "a reference in a folder of another name"); !strings.Contains(message, "lies in a folder called Остатки") {
		t.Fatalf("the error does not say what is wrong: %v", message)
	}
}
