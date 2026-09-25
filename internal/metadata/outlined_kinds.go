package metadata

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The five kinds the specification describes by the branch alone: what such an
// object is made of is not written down anywhere yet, because no configuration
// we have seen carries one.
const (
	BotKind                Kind = "bots"
	WSReferenceKind        Kind = "ws-references"
	WebSocketClientKind    Kind = "websocket-clients"
	IntegrationServiceKind Kind = "integration-services"
	ExternalDataSourceKind Kind = "external-data-sources"
)

// outlinedKinds is that list, in the order the tree shows them.
var outlinedKinds = []Kind{BotKind, WSReferenceKind, WebSocketClientKind, IntegrationServiceKind, ExternalDataSourceKind}

// OutlinedKinds returns the kinds carried by identity alone.
func OutlinedKinds() []Kind { return append([]Kind(nil), outlinedKinds...) }

// OutlinedObject is one object of such a kind: everything a configuration
// object has regardless of what kind it is, and nothing else.
//
// Until today an object of these kinds was not read at all - the branch stood
// in the tree and the file in it was passed over in silence, which is the one
// outcome the conformance report calls unacceptable. Now it is read, named and
// counted; what it is made of waits for a configuration that actually carries
// one.
//
// Reading is strict, as everywhere else, and that is the point: a bot with
// properties we have never seen stops the import with the name of the property
// in the message. Loud and wrong beats quiet and lost - the first is a line in
// a report, the second is found months later by somebody wondering where an
// object went.
type OutlinedObject struct {
	Kind    Kind          `yaml:"-" json:"kind"`
	Format  int           `yaml:"format" json:"format"`
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
}

// DecodeOutlinedObject reads and validates one object of an outlined kind.
func DecodeOutlinedObject(source string, reader io.Reader, configuration project.Project) (OutlinedObject, error) {
	var value OutlinedObject
	if err := decodeStrict(source, reader, &value); err != nil {
		return OutlinedObject{}, err
	}
	if err := issuesError(source, value.Format,
		validateBase(value.Format, value.ID, value.Name, value.Title, configuration)); err != nil {
		return OutlinedObject{}, err
	}
	return value, nil
}

func cloneOutlinedObject(value OutlinedObject) OutlinedObject {
	value.Title = cloneTitle(value.Title)
	return value
}

// OutlinedObjectsOf returns every object of one outlined kind.
func (catalog *Catalog) OutlinedObjectsOf(kind Kind) []OutlinedObject {
	var result []OutlinedObject
	for _, item := range catalog.OutlinedObjects {
		if item.Kind == kind {
			result = append(result, cloneOutlinedObject(item))
		}
	}
	return result
}

// loadOutlinedKinds reads the objects of the five kinds carried by identity
// alone. They share a loader because they share everything that is known about
// them: a kind that grows a description of its own leaves this list.
func (catalog *Catalog) loadOutlinedKinds(root string, configuration project.Project) error {
	for _, kind := range outlinedKinds {
		if err := loadKind(root, kind, func(source string, file *os.File, id uuid.UUID) error {
			value, err := DecodeOutlinedObject(source, file, configuration)
			if err != nil {
				return err
			}
			if value.ID != id {
				return fmt.Errorf("%s %s is stored as %s", kind, value.ID, id)
			}
			value.Kind = kind
			catalog.OutlinedObjects = append(catalog.OutlinedObjects, value)
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// validateOutlinedObjects checks what can be checked without knowing what these
// objects are: that two of one kind do not share a name.
//
// Identifiers are checked by the loader, because the file is named by the
// identifier and two files cannot share a name.
func (catalog *Catalog) validateOutlinedObjects() error {
	seen := map[string]string{}
	for _, item := range catalog.OutlinedObjects {
		key := string(item.Kind) + ":" + strings.ToLower(item.Name)
		if previous, taken := seen[key]; taken {
			return fmt.Errorf("%s %s and %s share a name", item.Kind, previous, item.Name)
		}
		seen[key] = item.Name
	}
	return nil
}
