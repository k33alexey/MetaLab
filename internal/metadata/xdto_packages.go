package metadata

import (
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

// XDTOPackageKind holds the type descriptions a configuration exchanges data
// by. A package is a schema: what the types are called, what they are made of,
// and which namespace they belong to.
const XDTOPackageKind Kind = "xdto-packages"

// XDTOPackageContentFile is the schema itself, beside the description. The
// schema is not a property of the package but its body, so it lies in a file
// the way a template's content does - and it is XML, because that is what a
// schema of exchanged types is everywhere it is read.
const XDTOPackageContentFile = "content.xml"

// maxNamespaceLength bounds a namespace. It is a name, not a document.
const maxNamespaceLength = 512

// XDTOPackageDefinition is one package of exchanged types.
//
// The package is described by very little - what it is called and which
// namespace it defines. Everything else is the schema, and the schema lives in
// a file beside the description rather than inside it.
type XDTOPackageDefinition struct {
	Format  int           `yaml:"format" json:"format"`
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	// Namespace is what the types of this package belong to, and it is what
	// the other side of an exchange names them by. Two configurations agree on
	// a namespace and not on a package name: the name is ours, the namespace
	// is shared.
	Namespace string `yaml:"namespace" json:"namespace"`
}

// DecodeXDTOPackage reads and validates one package of exchanged types.
func DecodeXDTOPackage(source string, reader io.Reader, configuration project.Project) (XDTOPackageDefinition, error) {
	var value XDTOPackageDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return XDTOPackageDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateNamespace("namespace", value.Namespace, true)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return XDTOPackageDefinition{}, err
	}
	return value, nil
}

// validateNamespace checks a namespace for being a namespace: one word with no
// spaces in it, because it is written into every message the exchange sends and
// read by a side that is not ours.
//
// What the word says is not checked. A namespace is an address in form only -
// nothing dereferences it - and refusing one because it does not look like a
// URL we recognise would refuse a configuration that works.
func validateNamespace(path, value string, required bool) []string {
	if value == "" {
		if required {
			return []string{path + " must name the namespace the types belong to"}
		}
		return nil
	}
	if len([]rune(value)) > maxNamespaceLength {
		return []string{fmt.Sprintf("%s must not exceed %d characters", path, maxNamespaceLength)}
	}
	for _, symbol := range value {
		if unicode.IsSpace(symbol) || !unicode.IsPrint(symbol) {
			return []string{path + " must not contain spaces or control characters"}
		}
	}
	return nil
}

func cloneXDTOPackage(value XDTOPackageDefinition) XDTOPackageDefinition {
	value.Title = cloneTitle(value.Title)
	return value
}

// XDTOPackage returns one package by name, folded case.
func (catalog *Catalog) XDTOPackage(name string) (XDTOPackageDefinition, bool) {
	index, ok := catalog.xdtoPackageByName[strings.ToLower(name)]
	if !ok {
		return XDTOPackageDefinition{}, false
	}
	return cloneXDTOPackage(catalog.XDTOPackages[index]), true
}

// XDTOPackageByID returns one package by identifier - which is how a web
// service names the packages it is described by.
func (catalog *Catalog) XDTOPackageByID(id uuid.UUID) (XDTOPackageDefinition, bool) {
	index, ok := catalog.xdtoPackageByID[id]
	if !ok {
		return XDTOPackageDefinition{}, false
	}
	return cloneXDTOPackage(catalog.XDTOPackages[index]), true
}

// validateXDTOPackages checks what no single package can check about itself:
// that no two of them define the same namespace, and that each folder holds the
// schema and nothing else.
//
// Two packages on one namespace are two answers to "what is a type called X
// here", and the exchange would take whichever it read last. It is the one
// mistake of this kind that produces no error at all, only the wrong data.
func (catalog *Catalog) validateXDTOPackages(root string) error {
	namespaces := make(map[string]string, len(catalog.XDTOPackages))
	for _, item := range catalog.XDTOPackages {
		if previous, taken := namespaces[item.Namespace]; taken {
			return fmt.Errorf("XDTO packages %s and %s both define namespace %s", previous, item.Name, item.Namespace)
		}
		namespaces[item.Namespace] = item.Name
	}
	if root == "" {
		return nil
	}
	for _, item := range catalog.XDTOPackages {
		directory := filepath.Join(root, "metadata", string(XDTOPackageKind), item.Name)
		entries, err := os.ReadDir(directory)
		if err != nil {
			return fmt.Errorf("XDTO package %s: %w", item.Name, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
				return fmt.Errorf("XDTO package %s keeps %q, and a package keeps its description and its schema",
					item.Name, entry.Name())
			}
			if entry.Name() != project.ObjectMetadataFile && entry.Name() != XDTOPackageContentFile {
				return fmt.Errorf("XDTO package %s keeps %q, and a package keeps its description and its schema",
					item.Name, entry.Name())
			}
		}
	}
	return nil
}
