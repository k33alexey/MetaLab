package metadata

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// WSReferenceKind holds the descriptions of other systems' web services this
// configuration calls. It is the other side of a web service: a web service is
// what the configuration answers, a WS reference is what it asks somebody else.
const WSReferenceKind Kind = "ws-references"

// WSReferenceDefinitionFile is the imported description of the service, beside
// the description of the object. It is the body of the reference and not a
// property of it, the way a schema is the body of an XDTO package: services,
// their endpoints, operations and parameters all arrive with it and leave with
// it, and none of them is an object of the configuration.
const WSReferenceDefinitionFile = "definition.xml"

const (
	// maxLocationURLLength bounds the address a description was taken from.
	// It is an address, not a document.
	maxLocationURLLength = 1024
	// maxWSDefinitionBytes bounds the imported description. A description
	// carries the whole data model of the service, and those of large systems
	// run to megabytes, so the bound is generous; it exists so that a file put
	// in the wrong place is refused rather than read into memory whole.
	maxWSDefinitionBytes = 32 << 20
)

// The two namespaces a service description may be written in: WSDL 1.1, whose
// root is "definitions", and WSDL 2.0, whose root is "description".
const (
	wsdl11Namespace = "http://schemas.xmlsoap.org/wsdl/"
	wsdl20Namespace = "http://www.w3.org/ns/wsdl"
)

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
	var value WSReferenceDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return WSReferenceDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateLocationURL(value.LocationURL)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return WSReferenceDefinition{}, err
	}
	return value, nil
}

// validateLocationURL checks the address in form only: one line with no control
// characters in it. What it says is not checked, for the reason it is not
// required.
func validateLocationURL(value string) []string {
	if len([]rune(value)) > maxLocationURLLength {
		return []string{fmt.Sprintf("location_url must not exceed %d characters", maxLocationURLLength)}
	}
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
// the description of the object and the description of the service, both of
// them, and nothing else.
//
// The service description is required, unlike the schema of an XDTO package.
// A reference without one is a client with nothing to call, and the prototype
// creates a reference only by importing a description. It is also checked for
// being a description: a file that does not read as XML is an import cut off
// half way, and that is better seen when the project is read than on the first
// call.
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
		for _, entry := range entries {
			if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 ||
				(entry.Name() != project.ObjectMetadataFile && entry.Name() != WSReferenceDefinitionFile) {
				return fmt.Errorf("WS reference %s keeps %q, and a reference keeps its description and the service's",
					item.Name, entry.Name())
			}
			described = described || entry.Name() == WSReferenceDefinitionFile
		}
		if !described {
			return fmt.Errorf("WS reference %s has no %s: a reference is the service description it was imported with",
				item.Name, WSReferenceDefinitionFile)
		}
		if err := checkWSDefinition(filepath.Join(directory, WSReferenceDefinitionFile)); err != nil {
			return fmt.Errorf("WS reference %s: %s: %w", item.Name, WSReferenceDefinitionFile, err)
		}
	}
	return nil
}

// checkWSDefinition reads a service description through to its end: it has to
// be well-formed XML whose root is a WSDL document of either version.
//
// The description is not taken apart. What it describes - services, endpoints,
// operations - is not an object of the configuration, and a reading that
// understood half of WSDL would refuse descriptions the prototype takes. The
// whole file is read, and not only its root, because a file cut off in the
// middle has a perfectly good root.
func checkWSDefinition(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() > maxWSDefinitionBytes {
		return fmt.Errorf("the description exceeds %d MiB", maxWSDefinitionBytes>>20)
	}
	decoder := xml.NewDecoder(io.LimitReader(file, maxWSDefinitionBytes+1))
	rooted := false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("the description is not well-formed XML: %w", err)
		}
		element, ok := token.(xml.StartElement)
		if !ok || rooted {
			continue
		}
		rooted = true
		switch {
		case element.Name.Space == wsdl11Namespace && element.Name.Local == "definitions":
		case element.Name.Space == wsdl20Namespace && element.Name.Local == "description":
		default:
			return fmt.Errorf("the root element is {%s}%s, and a service description is WSDL 1.1 definitions or WSDL 2.0 description",
				element.Name.Space, element.Name.Local)
		}
	}
	if !rooted {
		return errors.New("the description holds no document")
	}
	return nil
}
