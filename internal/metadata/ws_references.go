package metadata

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// WSReferenceKind holds the descriptions of other systems' web services this
// configuration calls. It is the other side of a web service: a web service is
// what the configuration answers, a WS reference is what it asks somebody else.
const WSReferenceKind = kinds.WSReference

// WSReferenceDefinitionFile is the imported description of the service, beside
// the description of the object. It is the body of the reference and not a
// property of it, the way a schema is the body of an XDTO package: services,
// their endpoints, operations and parameters all arrive with it and leave with
// it, and none of them is an object of the configuration.
const WSReferenceDefinitionFile = "definition.xml"

// wsdl11Namespace is the namespace of the root "definitions" of a service
// description. The designer imports WSDL 1.1 alone: a description in WSDL
// 2.0, whose root is "description", and "definitions" in any other namespace
// it refuses - «WSDL должен содержать элемент definitions» (opis of
// 15.10.2026, ws-ssylka.md); one with no service at all it takes.
const wsdl11Namespace = "http://schemas.xmlsoap.org/wsdl/"

// WSReferenceDefinition is one description of a service somebody else runs.
type WSReferenceDefinition struct {
	Format  int           `yaml:"format" json:"format"`
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	// LocationURL is where the description was taken from when it was
	// imported. It is not required and nothing dereferences it: the reference
	// works by its description, not by this address, and the address may long
	// have stopped answering - or have been a file on somebody's disk.
	LocationURL string `yaml:"location_url,omitempty" json:"locationUrl,omitempty"`
}

// DecodeWSReference reads and validates one WS reference.
func DecodeWSReference(source string, reader io.Reader, configuration project.Project) (WSReferenceDefinition, error) {
	return decodeDescription(source, reader, configuration, func(value WSReferenceDefinition, configuration project.Project) []string {
		return validateLocationURL(value.LocationURL)
	})
}

// validateLocationURL checks the address in form only: one line with no control
// characters in it. What it says is not checked, for the reason it is not
// required.
func validateLocationURL(value string) []string {
	for _, symbol := range value {
		if unicode.IsControl(symbol) {
			return []string{"location_url must be one line without control characters"}
		}
	}
	return nil
}

func cloneWSReference(value WSReferenceDefinition) WSReferenceDefinition {
	value.Title = cloneTitle(value.Title)
	return value
}

// WSReference returns one WS reference by name, folded case.
func (catalog *Catalog) WSReference(name string) (WSReferenceDefinition, bool) {
	index, ok := catalog.wsReferenceByName[strings.ToLower(name)]
	if !ok {
		return WSReferenceDefinition{}, false
	}
	return cloneWSReference(catalog.WSReferences[index]), true
}

// WSReferenceByID returns one WS reference by identifier.
func (catalog *Catalog) WSReferenceByID(id uuid.UUID) (WSReferenceDefinition, bool) {
	index, ok := catalog.wsReferenceByID[id]
	if !ok {
		return WSReferenceDefinition{}, false
	}
	return cloneWSReference(catalog.WSReferences[index]), true
}

// validateWSReferenceFiles checks the folder of every WS reference: it holds
// the description of the object, the description of the service when there is
// one, and nothing else.
//
// The service description is not required. It was, on the reasoning that the
// prototype creates a reference only by importing a description, and an
// export of the platform itself refuted it: a reference there lies with no
// description at all. When there is one it is checked for being one: a file
// that does not read as XML is an import cut off half way, and that is better
// seen when the project is read than on the first call.
func (catalog *Catalog) validateWSReferenceFiles(root string) error {
	if root == "" {
		return nil
	}
	for _, item := range catalog.WSReferences {
		directory := filepath.Join(root, "metadata", string(WSReferenceKind), item.Name)
		entries, err := os.ReadDir(directory)
		if err != nil {
			return fmt.Errorf("WS reference %s: %w", item.Name, err)
		}
		described := false
		schemas := map[string]bool{}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 ||
				(name != project.ObjectMetadataFile && name != WSReferenceDefinitionFile && !WSReferenceSchemaFile(name)) {
				return fmt.Errorf("WS reference %s keeps %q, and a reference keeps its description, the service's and the schemas it imports",
					item.Name, name)
			}
			described = described || name == WSReferenceDefinitionFile
			if WSReferenceSchemaFile(name) {
				schemas[name] = false
			}
		}
		if !described {
			if len(schemas) > 0 {
				return fmt.Errorf("WS reference %s keeps schemas and no description of the service imports them", item.Name)
			}
			continue
		}
		if err := checkWSDefinition(filepath.Join(directory, WSReferenceDefinitionFile)); err != nil {
			return fmt.Errorf("WS reference %s: %s: %w", item.Name, WSReferenceDefinitionFile, err)
		}
		if err := checkWSReferenceSchemas(directory, schemas); err != nil {
			return fmt.Errorf("WS reference %s: %w", item.Name, err)
		}
	}
	return nil
}

// WSReferenceSchemaFile says a file of the folder of a reference is a schema
// its description imports: the prototype keeps each beside the description
// under a number - «1.xsd» - and names it in the description by
// «file://1.xsd» (5 of the 12 references of the exports, ws-ssylka.md,
// «Выгрузки»).
func WSReferenceSchemaFile(name string) bool {
	number, found := strings.CutSuffix(name, ".xsd")
	if !found || number == "" || number[0] == '0' {
		return false
	}
	for _, digit := range number {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

// wsSchemaLocation is how a description or a schema names a schema kept
// beside it: «file://<number>.xsd».
var wsSchemaLocation = regexp.MustCompile(`schemaLocation\s*=\s*["']file://([1-9][0-9]*\.xsd)["']`)

// checkWSReferenceSchemas checks the schemas kept beside a description:
// each is well-formed XML whose root is a schema of XML Schema, each is
// named by the description or by another schema - one named by nobody is
// left over from another import - and each schema named is there. schemas
// are the files of the folder, marked as they are found named.
func checkWSReferenceSchemas(directory string, schemas map[string]bool) error {
	named := func(path string) ([]string, error) {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var names []string
		for _, match := range wsSchemaLocation.FindAllStringSubmatch(string(content), -1) {
			names = append(names, match[1])
		}
		return names, nil
	}
	queue, err := named(filepath.Join(directory, WSReferenceDefinitionFile))
	if err != nil {
		return err
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		seen, kept := schemas[name]
		if !kept {
			return fmt.Errorf("the description imports %s, which is not kept beside it", name)
		}
		if seen {
			continue
		}
		schemas[name] = true
		if err := checkWSSchema(filepath.Join(directory, name)); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		more, err := named(filepath.Join(directory, name))
		if err != nil {
			return err
		}
		queue = append(queue, more...)
	}
	for name, seen := range schemas {
		if !seen {
			return fmt.Errorf("%s is kept beside the description and nothing imports it", name)
		}
	}
	return nil
}

// checkWSSchema reads a schema through to its end: well-formed XML whose
// root is the schema of XML Schema.
func checkWSSchema(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	root, err := xmlRootOf(file)
	if err != nil {
		return fmt.Errorf("the schema is not well-formed XML: %w", err)
	}
	if root == (xml.Name{}) {
		return errors.New("the schema holds no document")
	}
	if root != (xml.Name{Space: xmlSchemaNamespace, Local: "schema"}) {
		return fmt.Errorf("the root element is {%s}%s, and an imported schema is the schema of XML Schema", root.Space, root.Local)
	}
	return nil
}

// checkWSDefinition reads the service description in the file at path
// through to its end (WSDefinitionFaultOf).
func checkWSDefinition(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = WSDefinitionFaultOf(file)
	return err
}

// WSDefinitionFault is what is wrong with a service description, as the
// designer tells it apart on import (opis of 15.10.2026, ws-ssylka.md):
// not XML at all - «Ошибка разбора WSDL» - or XML whose root is not the
// definitions of WSDL 1.1 - «WSDL должен содержать элемент definitions».
type WSDefinitionFault int

const (
	WSDefinitionFits WSDefinitionFault = iota
	WSDefinitionNotXML
	WSDefinitionNotDefinitions
)

// WSDefinitionFaultOf reads a service description through to its end: it
// has to be well-formed XML whose root is the "definitions" of WSDL 1.1, and
// the error says what it is not.
//
// The description is not taken apart. What it describes - services, endpoints,
// operations - is not an object of the configuration, and a reading that
// understood half of WSDL would refuse descriptions the prototype takes. The
// whole file is read, and not only its root, because a file cut off in the
// middle has a perfectly good root.
func WSDefinitionFaultOf(reader io.Reader) (WSDefinitionFault, error) {
	// No ceiling on the size of the description: a description carries the
	// whole data model of the service, the prototype limits none, and it is
	// read token by token, never whole (owner, 03.10.2026).
	root, err := xmlRootOf(reader)
	if err != nil {
		return WSDefinitionNotXML, fmt.Errorf("the description is not well-formed XML: %w", err)
	}
	if root == (xml.Name{}) {
		return WSDefinitionNotXML, errors.New("the description holds no document")
	}
	if root.Space != wsdl11Namespace || root.Local != "definitions" {
		return WSDefinitionNotDefinitions, fmt.Errorf("the root element is {%s}%s, and a service description is the definitions of WSDL 1.1 {%s}: the designer imports no other",
			root.Space, root.Local, wsdl11Namespace)
	}
	return WSDefinitionFits, nil
}

// xmlRootOf reads a document through to its end - a file cut off in the
// middle has a perfectly good root - and answers the name of its root, none
// where it holds no element.
func xmlRootOf(reader io.Reader) (xml.Name, error) {
	decoder := xml.NewDecoder(reader)
	var root xml.Name
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return root, nil
		}
		if err != nil {
			return xml.Name{}, err
		}
		if element, ok := token.(xml.StartElement); ok && root == (xml.Name{}) {
			root = element.Name
		}
	}
}
