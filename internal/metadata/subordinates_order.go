package metadata

import (
	"fmt"
	"maps"
	"slices"
	"sort"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// SubordinatesOrder is the order the developer gave what an object keeps in
// folders of subordinates: the prototype keeps the forms of an object in
// its own order and shows them so (the export's ChildObjects: ФормаЭлемента,
// ФормаВыбора, ...), while a folder has none. One field on every kind keeping
// forms, embedded in its description (METADATA-OBJECTS.md, «Формат порядка в
// описании объекта»; owner, 30.10.2026). The key names the folder of the
// subordinates; the commands and the templates lie in lists of the
// description itself, which keep their order.
type SubordinatesOrder struct {
	Order ObjectOrder `yaml:"order,omitempty" json:"order,omitzero"`
}

// ObjectOrder is the identifiers of the subordinates of an object in the
// developer's order, by their folder.
type ObjectOrder struct {
	Forms []uuid.UUID `yaml:"forms,omitempty" json:"forms,omitempty"`
}

func (order SubordinatesOrder) clone() SubordinatesOrder {
	order.Order.Forms = slices.Clone(order.Order.Forms)
	return order
}

// validateFormsOrder checks the list of the order of forms for shape: an
// identifier once, none zero.
func validateFormsOrder(list []uuid.UUID) error {
	seen := make(map[uuid.UUID]bool, len(list))
	for index, id := range list {
		switch {
		case id.IsZero():
			return fmt.Errorf("order.forms[%d] must be a non-zero UUID", index)
		case seen[id]:
			return fmt.Errorf("order.forms[%d] names %s twice", index, id)
		}
		seen[id] = true
	}
	return nil
}

// orderedForms is the forms of an object in the developer's order: those the
// order names, in its order, then those it leaves out, by name. Without an
// order - the key left out - every form is in the order of names.
func (index objectFormIndex) orderedForms() []objectFormRef {
	byID := make(map[uuid.UUID]objectFormRef, len(index.forms))
	for _, form := range index.forms {
		byID[form.id] = form
	}
	result := make([]objectFormRef, 0, len(index.forms))
	listed := map[uuid.UUID]bool{}
	for _, id := range index.order {
		if form, ok := byID[id]; ok {
			result = append(result, form)
			listed[id] = true
		}
	}
	var rest []objectFormRef
	for _, form := range index.forms {
		if !listed[form.id] {
			rest = append(rest, form)
		}
	}
	sort.Slice(rest, func(left, right int) bool { return rest[left].name < rest[right].name })
	return append(result, rest...)
}

// resolveFormsOrder notes every line of the order of forms of an object
// naming no form of it: a form deleted past Studio, a reference to nothing
// (UnresolvedReference), as a line of objects_order.
func (catalog *Catalog) resolveFormsOrder(kind, name string, order []uuid.UUID, forms map[string]objectFormRef) {
	kept := make(map[uuid.UUID]bool, len(forms))
	for _, form := range forms {
		kept[form.id] = true
	}
	for _, id := range order {
		if !kept[id] {
			catalog.noteUnresolved(kind+" "+name+" order.forms", id)
		}
	}
}

// noteFormOutsideOrder notes every form of an object keeping an order of its
// forms that the order leaves out: its folder was added past Studio.
func noteFormOutsideOrder(catalog *Catalog, note func(where, written string)) {
	for _, objectKind := range slices.Sorted(maps.Keys(catalog.objectForms)) {
		objects := catalog.objectForms[objectKind]
		for _, key := range slices.Sorted(maps.Keys(objects)) {
			index := objects[key]
			if len(index.order) == 0 {
				continue
			}
			for _, form := range index.orderedForms() {
				if !slices.Contains(index.order, form.id) {
					note(kindWord(objectKind)+" "+index.object+" form "+form.name, "")
				}
			}
		}
	}
}
