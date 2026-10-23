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
// catalogs down.
func (catalog *Catalog) validateOwnersAsTheDesignerDoes() error {
	for _, definition := range catalog.Catalogs {
		for index, id := range definition.Owners {
			if len(definition.Predefined) > 0 {
				return fmt.Errorf("catalog %s keeps predefined items and cannot belong to %s (owners[%d])",
					definition.Name, catalog.ownerName(id), index)
			}
			if id == definition.ID {
				return fmt.Errorf("catalog %s cannot be its own owner (owners[%d])", definition.Name, index)
			}
		}
	}
	if ring := catalog.ownerRing(); len(ring) > 0 {
		return fmt.Errorf("catalogs own each other in a ring: %s", strings.Join(ring, " belongs to "))
	}
	return nil
}

// ownerRing finds a ring of catalogs owning one another and names it from
// the catalog it starts at round to that catalog again, or returns nil. The
// catalogs are walked in their order and the owners in theirs, so one
// configuration is always worded alike.
func (catalog *Catalog) ownerRing() []string {
	const (
		unseen = iota
		onPath
		done
	)
	state := make(map[uuid.UUID]int, len(catalog.Catalogs))
	var path []uuid.UUID
	var ring []string
	var walk func(id uuid.UUID) bool
	walk = func(id uuid.UUID) bool {
		state[id] = onPath
		path = append(path, id)
		for _, owner := range catalog.Catalogs[catalog.catalogByID[id]].Owners {
			if _, isCatalog := catalog.catalogByID[owner]; !isCatalog || owner == id {
				continue
			}
			switch state[owner] {
			case onPath:
				start := slices.Index(path, owner)
				for _, member := range append(path[start:], owner) {
					ring = append(ring, catalog.Catalogs[catalog.catalogByID[member]].Name)
				}
				return true
			case unseen:
				if walk(owner) {
					return true
				}
			}
		}
		path = path[:len(path)-1]
		state[id] = done
		return false
	}
	for _, definition := range catalog.Catalogs {
		if state[definition.ID] == unseen && walk(definition.ID) {
			return ring
		}
	}
	return nil
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
