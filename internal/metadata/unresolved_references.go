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
	// Written is the reference as the prototype wrote it, when it is not an
	// identifier: a value "<type id>.<item id>", a path "-3".
	Written string
}

// Text is the reference as it is named in a refusal and a list.
func (item UnresolvedReference) Text() string {
	if item.Written != "" {
		return item.Written
	}
	return item.ID.String()
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

// collectRemnants adds to the unresolved references the other remnants of
// what was deleted (owner, 05.10.2026): the type of an object the project
// no longer has, a value naming a type it no longer has, and a path of a link
// that leads to no field of the object. They are found in the loaded model,
// forms included, so this runs once everything is read.
func (catalog *Catalog) collectRemnants() {
	eachTypeList(catalog, func(where string, types []Type) {
		for _, item := range types {
			if item.Kind == VanishedType && item.Reference != nil {
				catalog.unresolved = append(catalog.unresolved, UnresolvedReference{Where: where + " type", ID: *item.Reference})
			}
		}
	})
	written := func(where, text string) {
		catalog.unresolved = append(catalog.unresolved, UnresolvedReference{Where: where, Written: text})
	}
	eachNoteHolder(catalog, func(holder noteHolder) {
		if holder.filling != nil && holder.filling.Value != nil && holder.filling.Value.Kind == UnresolvedReferenceValue {
			written(holder.where+" filling", holder.filling.Value.Data)
		}
		if holder.choice == nil {
			return
		}
		for _, parameter := range holder.choice.Parameters {
			for _, value := range parameter.Values {
				if value.Kind == UnresolvedReferenceValue {
					written(holder.where+" choice parameter "+parameter.Name, value.Data)
				}
			}
		}
		for _, link := range holder.choice.ParameterLinks {
			if link.Source.Unresolved != "" {
				written(holder.where+" choice link "+link.Name, link.Source.Unresolved)
			}
		}
		if link := holder.choice.LinkByType; link != nil && link.Source.Unresolved != "" {
			written(holder.where+" link by type", link.Source.Unresolved)
		}
	})
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
		places = append(places, item.Where+" "+item.Text())
	}
	return fmt.Errorf("%w: %s", ErrUnresolvedReference, strings.Join(places, "; "))
}
