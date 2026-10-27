package metadata

import (
	"cmp"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// noteObjectOutsideOrder notes every object of a kind whose order the root
// keeps, left out of that order: its folder was added past Studio, by hand or
// by another tool (owner, 24.10.2026, BLOCKS 2б «порядок объектов
// конфигурации хранится»).
func noteObjectOutsideOrder(catalog *Catalog, note func(where, written string)) {
	for _, kind := range orderedKinds(catalog) {
		listed := catalog.listedInOrder(kind)
		for _, id := range catalog.ObjectsOrder(kind) {
			if !listed[id] {
				note(kindWord(kind)+" "+catalog.objects[id].name, "")
			}
		}
	}
}

// ObjectsOrder answers the objects of a kind of the configuration's tree in
// the order the developer gave them: the order the root keeps for the kind
// (project.Project.ObjectsOrder), then the objects it leaves out, by name. A
// kind with no order kept is in the order of names. The catalog's own lists
// stay in the order of identifiers - what the tree, the roles window and the
// sections show is read from here.
func (catalog *Catalog) ObjectsOrder(kind Kind) []uuid.UUID {
	listed := catalog.listedInOrder(kind)
	var order, rest []uuid.UUID
	for _, id := range catalog.Project.ObjectsOrder[string(kind)] {
		if object, ok := catalog.objects[id]; ok && object.kind == kind {
			order = append(order, id)
		}
	}
	for id, object := range catalog.objects {
		if object.kind == kind && !listed[id] {
			rest = append(rest, id)
		}
	}
	slices.SortFunc(rest, func(left, right uuid.UUID) int {
		return cmp.Or(strings.Compare(strings.ToLower(catalog.objects[left].name), strings.ToLower(catalog.objects[right].name)),
			strings.Compare(left.String(), right.String()))
	})
	return append(order, rest...)
}

// listedInOrder is the objects of a kind the kept order names.
func (catalog *Catalog) listedInOrder(kind Kind) map[uuid.UUID]bool {
	list := catalog.Project.ObjectsOrder[string(kind)]
	listed := make(map[uuid.UUID]bool, len(list))
	for _, id := range list {
		if object, ok := catalog.objects[id]; ok && object.kind == kind {
			listed[id] = true
		}
	}
	return listed
}

// orderedKinds is the kinds the root keeps an order of, in a fixed order.
func orderedKinds(catalog *Catalog) []Kind {
	result := make([]Kind, 0, len(catalog.Project.ObjectsOrder))
	for kind := range catalog.Project.ObjectsOrder {
		result = append(result, Kind(kind))
	}
	slices.Sort(result)
	return result
}

// resolveObjectsOrder checks that every line of a kept order names an object
// of its kind. A line naming nothing is an object deleted past Studio: the
// strict load refuses the project, the reading for editing lists it, as every
// reference to nothing (UnresolvedReference).
//
// A catalog made from a runtime snapshot (root "") holds a part of the kinds
// only and is no project being read, so there is nothing to check it against.
func (catalog *Catalog) resolveObjectsOrder(root string) {
	if root == "" {
		return
	}
	for _, kind := range orderedKinds(catalog) {
		for _, id := range catalog.Project.ObjectsOrder[string(kind)] {
			if object, ok := catalog.objects[id]; !ok || object.kind != kind {
				catalog.noteUnresolved("the project's objects_order."+string(kind), id)
			}
		}
	}
}
