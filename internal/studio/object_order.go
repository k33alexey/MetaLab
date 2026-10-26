package studio

import (
	"fmt"
	"slices"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// placeInOrderLocked puts a new object of a kind last in the order the root
// keeps, as the prototype puts it last in its branch (owner, 24.10.2026). A
// kind with no order kept yet starts one: the order the tree shows now - by
// name - and the new object after it. When the project does not load, a kind
// with no order is left without one, and the new object stands by name.
//
// The answer puts the root back as it was, for a caller whose own step after
// this one is refused. The caller holds workspace.mu, and the object's files
// are already written: the order is read from the project as it now is.
func (workspace *Workspace) placeInOrderLocked(kind metadata.Kind, id uuid.UUID) (func() error, error) {
	return workspace.changeOrderLocked(kind, func(order []uuid.UUID, kept bool) ([]uuid.UUID, bool) {
		if !kept {
			loaded, err := metadata.Load(workspace.root)
			if err != nil {
				return nil, false
			}
			order = slices.DeleteFunc(loaded.ObjectsOrder(kind), func(other uuid.UUID) bool { return other == id })
		}
		return append(slices.Clone(order), id), true
	})
}

// dropFromOrderLocked takes an object of a kind out of the order the root
// keeps, before the object itself goes: a line naming nothing is a reference
// to nothing, which the strict load refuses. A kind with no order, or an order
// not naming the object, is left as it is. The answer puts the root back.
func (workspace *Workspace) dropFromOrderLocked(kind metadata.Kind, id uuid.UUID) (func() error, error) {
	return workspace.changeOrderLocked(kind, func(order []uuid.UUID, kept bool) ([]uuid.UUID, bool) {
		if !kept || !slices.Contains(order, id) {
			return nil, false
		}
		return slices.DeleteFunc(slices.Clone(order), func(other uuid.UUID) bool { return other == id }), true
	})
}

// deleteObjectLocked deletes an object of a kind as every window of Studio
// does: the object leaves the order first, as a line left naming nothing
// would refuse the project; remove takes its files away and leaves them as
// they were when it is refused; a refused remove puts the line back.
func (workspace *Workspace) deleteObjectLocked(kind metadata.Kind, id uuid.UUID, relative string, remove func() error) error {
	restoreOrder, err := workspace.dropFromOrderLocked(kind, id)
	if err != nil {
		return fmt.Errorf("delete %q: %w", relative, err)
	}
	if err := remove(); err != nil {
		if restoreErr := restoreOrder(); restoreErr != nil {
			return fmt.Errorf("delete %q: %w; restoring the order failed: %v", relative, err, restoreErr)
		}
		return fmt.Errorf("delete %q: %w", relative, err)
	}
	workspace.invalidateStudioIndexesLocked()
	return nil
}

// changeOrderLocked writes the order of one kind that change answers from the
// one kept (kept says whether there is one), and nothing when change says
// there is nothing to write. The answer writes the root back as it was.
func (workspace *Workspace) changeOrderLocked(kind metadata.Kind, change func(order []uuid.UUID, kept bool) ([]uuid.UUID, bool)) (func() error, error) {
	unchanged := func() error { return nil }
	before, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return unchanged, err
	}
	current, kept := before.ObjectsOrder[string(kind)]
	order, write := change(current, kept)
	if !write {
		return unchanged, nil
	}
	after := before
	after.ObjectsOrder = make(map[string][]uuid.UUID, len(before.ObjectsOrder)+1)
	for other, list := range before.ObjectsOrder {
		after.ObjectsOrder[other] = list
	}
	after.ObjectsOrder[string(kind)] = order
	if err := project.SaveConfiguration(workspace.root, after); err != nil {
		return unchanged, err
	}
	return func() error { return project.SaveConfiguration(workspace.root, before) }, nil
}
