package metadata

import (
	"fmt"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A catalog may belong to another object: a contract belongs to a partner, a
// version of a file to the file. The prototype calls the other object the
// owner, and the subordinate catalog names its owners itself - one, or several
// of different kinds. Sixteen of the hundred and eleven catalogs of the
// demonstration configuration are subordinate, one of them to two owners at
// once.
//
// Without this the subordinate catalog still loads, still stores its rows and
// still opens - and every row of it belongs to nobody. That is the third
// category of the conformance report, not the second: the data is there and it
// means something else.

// SubordinationKind says what an owner may be when the owning object has
// folders: an item of it, a folder of it, or either. With several owners the
// same answer applies to all of them - the help says so outright.
type SubordinationKind string

const (
	SubordinateToItems          SubordinationKind = "to-items"
	SubordinateToFolders        SubordinationKind = "to-folders"
	SubordinateToFoldersAndItem SubordinationKind = "to-folders-and-items"
)

func validSubordinationKind(kind SubordinationKind) bool {
	switch kind {
	case "", SubordinateToItems, SubordinateToFolders, SubordinateToFoldersAndItem:
		return true
	default:
		return false
	}
}

// reachesFolders says whether this subordination lets a folder own a row.
func (kind SubordinationKind) reachesFolders() bool {
	return kind == SubordinateToFolders || kind == SubordinateToFoldersAndItem
}

// catalogOwnerKinds are the kinds whose elements may own a catalog's rows, in
// the order a message walks them, so that two loads of one configuration word
// a refusal the same way. The demonstration configuration uses
// two of them - a catalog and a chart of characteristic types - and the syntax
// assistant names all five: the type of СправочникОбъект.Владелец is a
// reference to a catalog, a chart of characteristic types, a chart of
// accounts, a chart of calculation types or an exchange plan. That property is
// what an owner is read back as, so it is the list.
//
// The exchange plan was missing here until the three fields reached the data.
// Four kinds had been allowed on the reasoning that refusing a kind we have no
// evidence against is worse than carrying one; the evidence turned out to
// exist, one property along, and it named a fifth.
var catalogOwnerKinds = []Kind{CatalogKind, ChartOfCharacteristicTypesKind, ChartOfAccountsKind, ChartOfCalculationTypesKind, ExchangePlanKind}

// CatalogOwnerKinds are the kinds whose objects may own a catalog, the five
// the window choosing the owners offers (docs/block-00/spravochnik.md,
// snapshot vladelcy-pyat-vidov).
func CatalogOwnerKinds() []Kind {
	return slices.Clone(catalogOwnerKinds)
}

// KindObjects are the objects of a kind of the project, in the order of the
// tree, as a window choosing among them lists them. A kind the catalog keeps
// no list of - a common form, known only by its folder - is read off the
// index of every object, by name before the order of the tree.
func (catalog *Catalog) KindObjects(kind Kind) []ReferenceTypeObject {
	if _, listed := catalog.objectsOf(kind); listed {
		return catalog.referenceTypeObjects(kind)
	}
	var result []ReferenceTypeObject
	for id, object := range catalog.objects {
		if object.kind == kind && object.owner == nil {
			result = append(result, ReferenceTypeObject{ID: id, Name: object.name, Title: object.title})
		}
	}
	slices.SortFunc(result, func(a, b ReferenceTypeObject) int { return strings.Compare(a.Name, b.Name) })
	return catalog.inTreeOrder(kind, result)
}

// ownerObject finds one owner by identifier among the kinds allowed to own.
// The second result is false when no such object is there to own anything.
func (catalog *Catalog) ownerObject(id uuid.UUID) (Kind, bool) {
	for _, kind := range catalogOwnerKinds {
		if catalog.hasObject(kind, id) {
			return kind, true
		}
	}
	return "", false
}

// ownerHierarchy is the hierarchy of the owning object, and whether that
// object has one at all. Only a catalog and a chart of characteristic types
// keep one; an account, a calculation type and a node of an exchange plan have
// no folders to be subordinate to.
func (catalog *Catalog) ownerHierarchy(kind Kind, id uuid.UUID) Hierarchy {
	switch kind {
	case CatalogKind:
		return catalog.Catalogs[catalog.catalogByID[id]].Hierarchy
	case ChartOfCharacteristicTypesKind:
		return catalog.ChartsOfCharacteristicTypes[catalog.chartOfCharacteristicTypesByID[id]].Hierarchy
	}
	return Hierarchy{}
}

// ownerName is what a message about an owner calls it.
func (catalog *Catalog) ownerName(id uuid.UUID) string {
	if object, ok := catalog.objects[id]; ok {
		return object.name
	}
	return id.String()
}

// OwnerTypes is what a reference to the owner of one row may be: one element
// of the type description per owner. It is what the standard attribute Owner
// holds, and it is read off the owners rather than written down anywhere.
func (catalog *Catalog) OwnerTypes(definition CatalogDefinition) []Type {
	result := make([]Type, 0, len(definition.Owners))
	for _, id := range definition.Owners {
		kind, ok := catalog.ownerObject(id)
		if !ok {
			continue
		}
		result = append(result, referenceType(referenceTo(kind), id))
	}
	return result
}

// validateCatalogSubordination checks the shape of what one catalog says about
// its owners, without the configuration: that no owner is named twice and that
// the subordination is one the platform knows.
func validateCatalogSubordination(owners []uuid.UUID, subordination SubordinationKind) []string {
	var issues []string
	issues = append(issues, validateUniqueIDs("owners", owners)...)
	if !validSubordinationKind(subordination) {
		issues = append(issues, "subordination must be to-items, to-folders or to-folders-and-items")
	}
	// Subordination and a code series within the owner say something about an
	// owner, and without owners there is nothing for them to say it about -
	// yet the prototype writes subordination on every catalog, owned or not -
	// 992 catalogs of the configurations being moved have it with no owner -
	// and keeps the series when the owners are taken away, on two. They are
	// carried and mean nothing until there is an owner.
	return issues
}

// validateCatalogOwners resolves the owners of every catalog. An owner that is
// not in the configuration owns nothing, and a row pointing at it points
// nowhere; subordination to folders where the owner has none is a restriction
// on rows that cannot exist.
func (catalog *Catalog) validateCatalogOwners() error {
	if err := catalog.validateOwnersAsTheDesignerDoes(); err != nil {
		return err
	}
	for _, definition := range catalog.Catalogs {
		for index, id := range definition.Owners {
			kind, ok := catalog.ownerObject(id)
			if !ok {
				return fmt.Errorf("catalog %s belongs to %s (owners[%d]), which is not an object that can own a catalog",
					definition.Name, id, index)
			}
			if !definition.Subordination.reachesFolders() {
				continue
			}
			hierarchy := catalog.ownerHierarchy(kind, id)
			if !hierarchy.Enabled || hierarchy.Kind != FoldersAndItemsHierarchy {
				return fmt.Errorf("catalog %s is subordinate to folders of %s %s, which has none",
					definition.Name, kind, catalog.ownerName(id))
			}
		}
	}
	return nil
}

// validateOwnersAsTheDesignerDoes refuses the three owners the designer of
// 8.3.27 does not let a developer name (opis of the catalog, 14.10.2026,
// docs/block-00/spravochnik.md; METADATA-OBJECTS.md, «Владелец — не сам
// справочник и не кольцо»): an owner of a catalog that keeps predefined items,
// a catalog owning itself, and a ring of owners of any length. A ring would
// also never end the deletion mark of a branch, which follows subordinate
// catalogs down. The rule is ownerFault's, the one the window of a catalog
// asks too (OwnersFault).
func (catalog *Catalog) validateOwnersAsTheDesignerDoes() error {
	ownersOf := catalog.catalogOwners(CatalogDefinition{})
	for _, definition := range catalog.Catalogs {
		fault, index := ownerFault(definition, ownersOf)
		switch fault {
		case OwnerOfPredefined:
			return fmt.Errorf("catalog %s keeps predefined items and cannot belong to %s (owners[%d])",
				definition.Name, catalog.ownerName(definition.Owners[index]), index)
		case OwnerItself:
			return fmt.Errorf("catalog %s cannot be its own owner (owners[%d])", definition.Name, index)
		case OwnerInRing:
			ring := []string{definition.Name}
			for _, id := range ownerChain(definition.Owners[index], definition.ID, ownersOf) {
				ring = append(ring, catalog.ownerName(id))
			}
			return fmt.Errorf("catalogs own each other in a ring: %s", strings.Join(append(ring, definition.Name), " belongs to "))
		}
	}
	return nil
}

// OwnerFault is which of the designer's three refusals an owner of a catalog
// meets (opis of the catalog, 14.10.2026, docs/block-00/spravochnik.md,
// «Проверки»), or none.
type OwnerFault int

const (
	OwnerFits OwnerFault = iota
	// OwnerOfPredefined is any owner of a catalog that keeps predefined items.
	OwnerOfPredefined
	// OwnerItself is the catalog named as its own owner.
	OwnerItself
	// OwnerInRing is an owner subordinate to the catalog already, directly or
	// through a chain of catalogs: naming it closes a ring.
	OwnerInRing
)

// OwnersFault is what the designer says of the owners of a catalog as its
// window holds them, against the owners every other catalog of the project
// has: the refusal and the name of the owner it falls on, the first in the
// order of the owners.
func (catalog *Catalog) OwnersFault(definition CatalogDefinition) (OwnerFault, string) {
	fault, index := ownerFault(definition, catalog.catalogOwners(definition))
	if fault == OwnerFits {
		return fault, ""
	}
	if definition.Owners[index] == definition.ID {
		return fault, definition.Name
	}
	return fault, catalog.ownerName(definition.Owners[index])
}

// catalogOwners answers the owners of a catalog of the project by its
// identifier, those of edited as the window holds them; none for an object
// that is not a catalog, whose owners never lead back to one.
func (catalog *Catalog) catalogOwners(edited CatalogDefinition) func(uuid.UUID) []uuid.UUID {
	return func(id uuid.UUID) []uuid.UUID {
		if id == edited.ID {
			return edited.Owners
		}
		if index, ok := catalog.catalogByID[id]; ok {
			return catalog.Catalogs[index].Owners
		}
		return nil
	}
}

// ownerFault is the first of the designer's refusals among the owners of one
// catalog, in their order, and the position of the owner it falls on; -1
// where none falls. ownersOf answers the owners of another catalog.
func ownerFault(definition CatalogDefinition, ownersOf func(uuid.UUID) []uuid.UUID) (OwnerFault, int) {
	for index, id := range definition.Owners {
		switch {
		case len(definition.Predefined) > 0:
			return OwnerOfPredefined, index
		case id == definition.ID:
			return OwnerItself, index
		case ownerChain(id, definition.ID, ownersOf) != nil:
			return OwnerInRing, index
		}
	}
	return OwnerFits, -1
}

// ownerChain is the way from the catalog from up its owners to the catalog
// to - from, its owner, that one's owner, the last one owned by to - or nil
// where to owns none of them at any remove.
func ownerChain(from, to uuid.UUID, ownersOf func(uuid.UUID) []uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	var walk func(id uuid.UUID) []uuid.UUID
	walk = func(id uuid.UUID) []uuid.UUID {
		if seen[id] {
			return nil
		}
		seen[id] = true
		for _, owner := range ownersOf(id) {
			if owner == to {
				return []uuid.UUID{id}
			}
			if rest := walk(owner); rest != nil {
				return append([]uuid.UUID{id}, rest...)
			}
		}
		return nil
	}
	return walk(from)
}

// ownerColumns gives a subordinate catalog's table the column its owner lives
// in. One owner is a reference to a known object, so it is a plain column with
// a key into that object's table; several owners are a composite reference and
// are stored the way every composite reference is stored - the identity of the
// object beside the identity of the row, and no key, because a key can only
// point at one table.
func (catalog *Catalog) ownerColumns(table *schemadiff.Table, definition CatalogDefinition) error {
	if len(definition.Owners) == 0 {
		return nil
	}
	if len(definition.Owners) == 1 {
		owner := definition.Owners[0]
		target, err := schemadiff.TableName(owner)
		if err != nil {
			return err
		}
		table.Columns = append(table.Columns, schemadiff.Column{Name: "owner", Type: "uuid", Nullable: true})
		table.Constraints = append(table.Constraints, schemadiff.Constraint{
			Name: physicalObjectName("fo", definition.ID), Type: "foreign_key",
			Definition: "FOREIGN KEY (owner) REFERENCES " + schemadiff.ApplicationSchema + "." + target + "(ref) DEFERRABLE INITIALLY DEFERRED",
		})
		table.Indexes = append(table.Indexes, schemadiff.Index{Name: physicalObjectName("io", definition.ID), Method: "btree", Keys: []string{"owner"}})
		return nil
	}
	table.Columns = append(table.Columns,
		schemadiff.Column{Name: "owner_type", Type: "uuid", Nullable: true},
		schemadiff.Column{Name: "owner_ref", Type: "uuid", Nullable: true},
	)
	table.Indexes = append(table.Indexes, schemadiff.Index{
		Name: physicalObjectName("io", definition.ID), Method: "btree", Keys: []string{"owner_type", "owner_ref"},
	})
	return nil
}
