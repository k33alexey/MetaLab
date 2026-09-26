package metadata

import (
	"fmt"

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

// maxCatalogOwners is where a list of owners stops being a list. The prototype
// puts no number on it; two is the most the demonstration configuration uses.
const maxCatalogOwners = 32

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

// ownerKindTypes are the kinds whose elements may own a catalog's rows, each
// with the type of a reference to one. They are the kinds built like a catalog
// - a code, a description, elements of their own to belong to. The
// demonstration configuration uses two of the four; the other two are allowed
// because refusing them would refuse a configuration we have no evidence
// against, and carrying one we should not is harmless by comparison.
var ownerKindTypes = map[Kind]TypeKind{
	CatalogKind:                    CatalogType,
	ChartOfCharacteristicTypesKind: CharacteristicTypesType,
	ChartOfAccountsKind:            AccountType,
	ChartOfCalculationTypesKind:    CalculationTypeType,
}

// ownerObject finds one owner by identifier among the kinds allowed to own.
// The second result is false when no such object is there to own anything.
func (catalog *Catalog) ownerObject(id uuid.UUID) (Kind, bool) {
	for _, kind := range []Kind{CatalogKind, ChartOfCharacteristicTypesKind, ChartOfAccountsKind, ChartOfCalculationTypesKind} {
		if catalog.hasObject(ownerKindTypes[kind], id) {
			return kind, true
		}
	}
	return "", false
}

// ownerHierarchy is the hierarchy of the owning object, and whether that
// object has one at all. Only a catalog and a chart of characteristic types
// keep one; an account and a calculation type have no folders to be subordinate
// to.
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
func (catalog *Catalog) ownerName(kind Kind, id uuid.UUID) string {
	switch kind {
	case CatalogKind:
		return catalog.Catalogs[catalog.catalogByID[id]].Name
	case ChartOfCharacteristicTypesKind:
		return catalog.ChartsOfCharacteristicTypes[catalog.chartOfCharacteristicTypesByID[id]].Name
	case ChartOfAccountsKind:
		return catalog.ChartsOfAccounts[catalog.chartOfAccountsByID[id]].Name
	case ChartOfCalculationTypesKind:
		return catalog.ChartsOfCalculationTypes[catalog.chartOfCalculationTypesByID[id]].Name
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
		result = append(result, referenceType(ownerKindTypes[kind], id))
	}
	return result
}

// validateCatalogSubordination checks the shape of what one catalog says about
// its owners, without the configuration: whether the owners are named at all,
// and whether the settings that depend on having them are set without them.
func validateCatalogSubordination(owners []uuid.UUID, subordination SubordinationKind, code CatalogCode) []string {
	var issues []string
	if len(owners) > maxCatalogOwners {
		return []string{fmt.Sprintf("owners must not contain more than %d objects", maxCatalogOwners)}
	}
	issues = append(issues, validateUniqueIDs("owners", owners)...)
	if !validSubordinationKind(subordination) {
		issues = append(issues, "subordination must be to-items, to-folders or to-folders-and-items")
	}
	// Both of these say something about an owner, and without owners there is
	// nothing for them to say it about.
	if len(owners) == 0 {
		if subordination != "" {
			issues = append(issues, "subordination needs owners: there is nothing for this catalog to be subordinate to")
		}
		if code.Series == WithinOwnerSeries {
			issues = append(issues, "code.series within-owner-subordination needs owners")
		}
	}
	return issues
}

// validateCatalogOwners resolves the owners of every catalog. An owner that is
// not in the configuration owns nothing, and a row pointing at it points
// nowhere; subordination to folders where the owner has none is a restriction
// on rows that cannot exist.
func (catalog *Catalog) validateCatalogOwners() error {
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
					definition.Name, kind, catalog.ownerName(kind, id))
			}
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
