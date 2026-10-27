package metadata

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// XDTOPackageKind holds the type descriptions a configuration exchanges data
// by. A package is a schema: what the types are called, what they are made of,
// and which namespace they belong to.
const XDTOPackageKind = kinds.XDTOPackage

// XDTOPackageContentFile is the schema itself, beside the description. The
// schema is not a property of the package but its body, so it lies in a file
// the way a template's content does - and it is XML, because that is what a
// schema of exchanged types is everywhere it is read.
const XDTOPackageContentFile = "content.xml"

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
	return decodeDescription(source, reader, configuration, func(value XDTOPackageDefinition, configuration project.Project) []string {
		return validateNamespace("namespace", value.Namespace, true)
	})
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
		if err := catalog.readXDTOContent(item, filepath.Join(directory, XDTOPackageContentFile)); err != nil {
			return err
		}
	}
	return nil
}

// readXDTOContent reads the body of a package, where it has one, and keeps
// it with the catalog. The body names the namespace of the package again,
// as the prototype's does: the two must be one (the 294 packages of the
// exports agree).
func (catalog *Catalog) readXDTOContent(item XDTOPackageDefinition, path string) error {
	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("XDTO package %s: %w", item.Name, err)
	}
	defer file.Close()
	content, err := DecodeXDTOContent(file)
	if err != nil {
		return fmt.Errorf("XDTO package %s: %w", item.Name, err)
	}
	if content.Namespace != item.Namespace {
		return fmt.Errorf("XDTO package %s: the body defines namespace %q, and the package %q", item.Name, content.Namespace, item.Namespace)
	}
	if catalog.xdtoContents == nil {
		catalog.xdtoContents = map[uuid.UUID]XDTOContent{}
	}
	catalog.xdtoContents[item.ID] = content
	return nil
}

// XDTOPackageContent returns the body of a package read with the catalog,
// by the identifier of the package; false where it has none.
func (catalog *Catalog) XDTOPackageContent(id uuid.UUID) (XDTOContent, bool) {
	content, ok := catalog.xdtoContents[id]
	if !ok {
		return XDTOContent{}, false
	}
	return content.clone(), true
}

// platformXDTONamespaces are the namespaces of the platform's own XDTO
// packages a web service may use, in the order of the window «Выбор
// пакетов XDTO» (docs/block-00/web-servis.md, «Пространства платформы»;
// snapshots series-2.584/132-136): 66, every one under http://v8.1c.ru/,
// written here without it.
var platformXDTONamespaces = []string{
	"8.1/data-composition-system/appearance-template", "8.1/data-composition-system/area-template", "8.1/data-composition-system/common",
	"8.1/data-composition-system/composition-template", "8.1/data-composition-system/core", "8.1/data-composition-system/details",
	"8.1/data-composition-system/result", "8.1/data-composition-system/schema", "8.1/data-composition-system/settings",
	"8.1/data/core", "8.1/data/enterprise", "8.1/data/enterprise/current-config",
	"8.1/data/txtedt", "8.1/data/ui", "8.1/xdto",
	"8.2/bsl", "8.2/data/bsl", "8.2/data/chart",
	"8.2/data/data-analysis", "8.2/data/formatted-document", "8.2/data/geo",
	"8.2/data/graphscheme", "8.2/data/spreadsheet", "8.2/data/test",
	"8.2/managed-application/cmi", "8.2/managed-application/core", "8.2/managed-application/dynamic-list-composition-template",
	"8.2/managed-application/dynamic-list-server", "8.2/managed-application/edbexception", "8.2/managed-application/InteractiveSecurityException",
	"8.2/managed-application/logform", "8.2/managed-application/logform/layouter", "8.2/managed-application/logformexception",
	"8.2/managed-application/modules", "8.2/managed-application/objectlocksexception", "8.2/managed-application/remoteexception",
	"8.2/managed-application/seancecontextexception", "8.2/managed-application/SecondFactorAuthException", "8.2/managed-application/user-settings",
	"8.2/roles", "8.2/virtual-resource-system", "8.3/data/entext",
	"8.3/data/ext", "8.3/data/mob", "8.3/data/pdf",
	"8.3/data/planner", "8.3/dbcopies", "8.3/debugger/debugArea",
	"8.3/debugger/debugAutoAttach", "8.3/debugger/debugBaseData", "8.3/debugger/debugBreakpoints",
	"8.3/debugger/debugCalculations", "8.3/debugger/debugDBGUICommands", "8.3/debugger/debugForegroundData",
	"8.3/debugger/debugMeasure", "8.3/debugger/debugRDBGRequestResponse", "8.3/debugger/debugRTEFilter",
	"8.3/debugger/debugRTEInfo", "8.3/ecs/core", "8.3/managed-application/logform/hippo",
	"8.3/mobile-application/app", "8.3/mobile-application/form", "8.3/stt/core",
	"8.3/TestAndRepairExchangePlanIntegrityReport", "8.3/xcf/dumpinfo", "8.3/xcf/enums",
}

// PlatformXDTONamespaces answers the namespaces of the platform's XDTO
// packages whole, in the order of the window.
func PlatformXDTONamespaces() []string {
	namespaces := make([]string, len(platformXDTONamespaces))
	for index, tail := range platformXDTONamespaces {
		namespaces[index] = "http://v8.1c.ru/" + tail
	}
	return namespaces
}

// newXDTONamespace is the namespace of a new package, the prototype's
// (paket-xdto.md, «Новый пакет»).
const newXDTONamespace = "http://www.sample-package.org"
