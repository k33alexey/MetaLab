package metadata

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// UnresolvedReference is a reference by identifier that nothing in the
// project carries: the object it pointed at was deleted and the reference was
// left behind. The prototype saves such a configuration and opens it - a form
// role falls back to the generated form, a command placed in the missing
// group is simply not drawn - and the configurations being moved have some.
//
// In a project it is an error (owner, 05.10.2026; METADATA-OBJECTS.md): the
// import does not carry it - it lists it in the report - and a reference left
// by hand is for the developer to fix. The strict load refuses the project
// over it, naming every such place at once; the reading for editing, which
// must open a project to let it be fixed, collects them instead, and
// UnresolvedReferences lists them as the errors they are.
type UnresolvedReference struct {
	// Where names what holds the reference, as messages about it do.
	Where string
	ID    uuid.UUID
}

// UnresolvedReferences lists the references the last reading for editing
// found pointing at nothing, in the order they were met. A strict load never
// returns a catalog that has one.
func (catalog *Catalog) UnresolvedReferences() []UnresolvedReference {
	return slices.Clone(catalog.unresolved)
}

func (catalog *Catalog) noteUnresolved(where string, id uuid.UUID) {
	catalog.unresolved = append(catalog.unresolved, UnresolvedReference{Where: where, ID: id})
}

// ErrUnresolvedReference is the refusal of a project that refers by
// identifier to an object it does not have.
var ErrUnresolvedReference = errors.New("references to objects the project does not have")

// refuseUnresolved is the strict load's answer to the references collected:
// all of them in one refusal, so that the developer fixes them in one go.
func (catalog *Catalog) refuseUnresolved() error {
	if len(catalog.unresolved) == 0 {
		return nil
	}
	places := make([]string, 0, len(catalog.unresolved))
	for _, item := range catalog.unresolved {
		places = append(places, item.Where+" "+item.ID.String())
	}
	return fmt.Errorf("%w: %s", ErrUnresolvedReference, strings.Join(places, "; "))
}
