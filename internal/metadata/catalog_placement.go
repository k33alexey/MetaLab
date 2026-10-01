package metadata

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Where a row of a catalog sits: under which folder, whether it is a folder
// itself, and whom it belongs to. Three fields the platform gives every
// catalog - Родитель, ЭтоГруппа, Владелец - and one path for all three, which
// is why they are done together.
//
// The columns were already there: the hierarchy has been described and the
// table marked up since the schema was built, and a subordinate catalog has
// had its owner column since owners were modelled. Nothing wrote into them.
// A row was stored with its code, its description and its attributes, and the
// tree it belonged to was lost on the way in - not refused, not reported,
// simply not written. That is the third category of the conformance report,
// the one that blocks a block from closing.
//
// Which of the three a catalog has is the object's answer, not the kind's:
// the parent exists where a hierarchy does, the folder flag where the
// hierarchy has folders, the owner where owners are declared. The
// descriptions of all three are another matter and belong to the kind - the
// demonstration configuration writes the standard-attribute description of
// Родитель, ЭтоГруппа and Владелец on all hundred and eleven catalogs, flat
// and unowned ones included.

// hasParent, hasFolders and hasOwner say which of the three fields this
// catalog stores. They read the definition rather than the table, because the
// table is built from the definition and a question answered twice is a
// question that can be answered two ways.
func catalogHasParent(definition CatalogDefinition) bool { return definition.Hierarchy.Enabled }

// catalogHasCode says the catalog's code is there at all: a length of 0
// switches it off.
func catalogHasCode(definition CatalogDefinition) bool { return definition.Code.Length > 0 }

// catalogHasDescription says the same of the description.
func catalogHasDescription(definition CatalogDefinition) bool {
	return definition.DescriptionLength > 0
}

func catalogHasFolders(definition CatalogDefinition) bool {
	return definition.Hierarchy.Enabled && definition.Hierarchy.Kind == FoldersAndItemsHierarchy
}

func catalogHasOwner(definition CatalogDefinition) bool { return len(definition.Owners) > 0 }

// placementColumns are the columns and values the three fields contribute to a
// write, in the order the write wants them.
//
// The owner is one column when there is one kind it can be and two when there
// are several, exactly as the table was built - see ownerColumns. A reference
// that may point at one of five objects cannot be a key into one table, so the
// identity of the object is stored beside the identity of the row.
func (repository *CatalogRepository) placementColumns(definition CatalogDefinition, record *CatalogRecord) ([]string, []any, error) {
	var columns []string
	var arguments []any
	if catalogHasParent(definition) {
		columns = append(columns, "parent")
		if record.Parent.IsZero() {
			arguments = append(arguments, nil)
		} else {
			arguments = append(arguments, record.Parent.String())
		}
	}
	if catalogHasFolders(definition) {
		columns = append(columns, "is_folder")
		arguments = append(arguments, record.IsFolder)
	}
	if !catalogHasOwner(definition) {
		return columns, arguments, nil
	}
	if len(definition.Owners) == 1 {
		columns = append(columns, "owner")
		if record.Owner.Data == "" {
			arguments = append(arguments, nil)
			return columns, arguments, nil
		}
		arguments = append(arguments, record.Owner.Data)
		return columns, arguments, nil
	}
	columns = append(columns, "owner_type", "owner_ref")
	if record.Owner.Data == "" {
		return columns, append(arguments, nil, nil), nil
	}
	return columns, append(arguments, record.Owner.Object.String(), record.Owner.Data), nil
}

// decodePlacement reads the three fields back off the row.
func (repository *CatalogRepository) decodePlacement(definition CatalogDefinition, fields map[string]json.RawMessage, record *CatalogRecord) error {
	if catalogHasParent(definition) {
		if raw := fields["parent"]; len(raw) != 0 && string(raw) != "null" {
			parent, err := decodeJSONUUID(raw)
			if err != nil {
				return fmt.Errorf("decode catalog %s parent: %w", definition.Name, err)
			}
			record.Parent = parent
		}
	}
	if catalogHasFolders(definition) {
		if err := json.Unmarshal(fields["is_folder"], &record.IsFolder); err != nil {
			return fmt.Errorf("decode catalog %s folder flag: %w", definition.Name, err)
		}
	}
	if !catalogHasOwner(definition) {
		return nil
	}
	if len(definition.Owners) == 1 {
		raw := fields["owner"]
		if len(raw) == 0 || string(raw) == "null" {
			return nil
		}
		id, err := decodeJSONUUID(raw)
		if err != nil {
			return fmt.Errorf("decode catalog %s owner: %w", definition.Name, err)
		}
		kind, ok := repository.catalog.ownerObject(definition.Owners[0])
		if !ok {
			return fmt.Errorf("catalog %s belongs to %s, which is not in the configuration", definition.Name, definition.Owners[0])
		}
		record.Owner = Value{Kind: ownerKindTypes[kind], Object: definition.Owners[0], Data: id.String()}
		return nil
	}
	rawType, rawRef := fields["owner_type"], fields["owner_ref"]
	if len(rawRef) == 0 || string(rawRef) == "null" {
		return nil
	}
	objectID, err := decodeJSONUUID(rawType)
	if err != nil {
		return fmt.Errorf("decode catalog %s owner object: %w", definition.Name, err)
	}
	rowID, err := decodeJSONUUID(rawRef)
	if err != nil {
		return fmt.Errorf("decode catalog %s owner: %w", definition.Name, err)
	}
	kind, ok := repository.catalog.ownerObject(objectID)
	if !ok {
		return fmt.Errorf("catalog %s row belongs to %s, which is not an object of the configuration", definition.Name, objectID)
	}
	record.Owner = Value{Kind: ownerKindTypes[kind], Object: objectID, Data: rowID.String()}
	return nil
}

// normalizePlacement is what can be answered about the three fields without
// the database: whether the catalog has the field at all, and whether the
// owner is one of the objects this catalog was declared to belong to.
//
// A field the catalog does not have is refused rather than dropped. Dropping
// it would let a flat catalog be written with a parent, report success, and
// hand back a row whose parent is gone.
func (repository *CatalogRepository) normalizePlacement(definition CatalogDefinition, record *CatalogRecord) error {
	if !catalogHasParent(definition) && !record.Parent.IsZero() {
		return fmt.Errorf("catalog %s is not hierarchical and its rows have no parent", definition.Name)
	}
	if !catalogHasFolders(definition) && record.IsFolder {
		return fmt.Errorf("catalog %s has no folders", definition.Name)
	}
	if record.Parent == record.Reference.ObjectID && !record.Parent.IsZero() {
		return fmt.Errorf("catalog %s row cannot be its own parent", definition.Name)
	}
	if !catalogHasOwner(definition) {
		if record.Owner != (Value{}) {
			return fmt.Errorf("catalog %s is not subordinate and its rows have no owner", definition.Name)
		}
		return nil
	}
	if record.Owner == (Value{}) {
		return nil
	}
	owner, err := repository.catalog.normalizeTypes("catalog "+definition.Name+" owner",
		repository.catalog.OwnerTypes(definition), record.Owner)
	if err != nil {
		return err
	}
	record.Owner = owner
	return nil
}

// checkPlacement is what only the database can answer, asked inside the
// transaction that writes the row and after its lock is taken.
//
// Three questions, and each of them is a row that would otherwise be written
// and mean something else:
//
//   - a parent that is not a folder. In a hierarchy of folders and items only
//     a folder holds children; an item given children is a tree nobody can
//     draw. The foreign key sees that the parent exists, not what it is.
//   - an owner whose row is not what this catalog is subordinate to. The
//     setting says items, folders or either, and it has meant nothing until
//     now because no row had an owner to check.
//   - a folder that stops being one. ЭтоГруппа is read-only in the prototype,
//     and there is no method that turns an item into a folder: a folder is
//     made as a folder. Without the check the flag would be quietly rewritten
//     by a save of a record read as an item.
func (repository *CatalogRepository) checkPlacement(ctx context.Context, transaction pgx.Tx, definition CatalogDefinition, record *CatalogRecord, tree treeWrite) error {
	if !record.Parent.IsZero() {
		folder, found, err := repository.rowIsFolder(ctx, transaction, definition.ID, catalogHasFolders(definition), record.Parent)
		if err != nil {
			return fmt.Errorf("catalog %s parent: %w", definition.Name, err)
		}
		if !found {
			return fmt.Errorf("catalog %s parent %s is not a row of this catalog", definition.Name, record.Parent)
		}
		if catalogHasFolders(definition) && !folder {
			return fmt.Errorf("catalog %s parent %s is an item, and only a folder holds rows", definition.Name, record.Parent)
		}
	}
	if err := repository.checkOwnerRow(ctx, transaction, definition, record); err != nil {
		return err
	}
	storedParent, stored := uuid.UUID{}, false
	if record.Version != 0 && catalogHasParent(definition) {
		var err error
		storedParent, stored, err = repository.storedRowParent(ctx, transaction, definition, record.Reference.ObjectID)
		if err != nil {
			return fmt.Errorf("catalog %s: %w", definition.Name, err)
		}
	}
	if catalogHasParent(definition) {
		if err := repository.checkTreePlacement(ctx, transaction, definition, record, tree, storedParent, stored); err != nil {
			return err
		}
	}
	if record.Version == 0 || !catalogHasFolders(definition) {
		return nil
	}
	folder, found, err := repository.rowIsFolder(ctx, transaction, definition.ID, true, record.Reference.ObjectID)
	if err != nil {
		return fmt.Errorf("catalog %s: %w", definition.Name, err)
	}
	if found && folder != record.IsFolder {
		return fmt.Errorf("catalog %s cannot turn a folder into an item or back: whether a row is a folder is decided when it is made", definition.Name)
	}
	return nil
}

// checkOwnerRow asks whether the owning row is there and whether it is the
// kind of row this catalog may be subordinate to.
func (repository *CatalogRepository) checkOwnerRow(ctx context.Context, transaction pgx.Tx, definition CatalogDefinition, record *CatalogRecord) error {
	if record.Owner == (Value{}) {
		return nil
	}
	rowID, err := uuid.Parse(record.Owner.Data)
	if err != nil {
		return fmt.Errorf("catalog %s owner: %w", definition.Name, err)
	}
	kind, ok := repository.catalog.ownerObject(record.Owner.Object)
	if !ok {
		return fmt.Errorf("catalog %s owner %s is not an object of the configuration", definition.Name, record.Owner.Object)
	}
	hierarchy := repository.catalog.ownerHierarchy(kind, record.Owner.Object)
	hasFolders := hierarchy.Enabled && hierarchy.Kind == FoldersAndItemsHierarchy
	folder, found, err := repository.rowIsFolder(ctx, transaction, record.Owner.Object, hasFolders, rowID)
	if err != nil {
		return fmt.Errorf("catalog %s owner: %w", definition.Name, err)
	}
	if !found {
		return fmt.Errorf("catalog %s belongs to %s %s, which has no row %s",
			definition.Name, kind, repository.catalog.ownerName(kind, record.Owner.Object), rowID)
	}
	switch {
	case folder && definition.Subordination == SubordinateToItems:
		return fmt.Errorf("catalog %s is subordinate to items, and %s is a folder", definition.Name, rowID)
	case !folder && definition.Subordination == SubordinateToFolders:
		return fmt.Errorf("catalog %s is subordinate to folders, and %s is an item", definition.Name, rowID)
	}
	return nil
}

// rowIsFolder reads one row of an object's table: whether it is there, and
// whether it is a folder. An object with no folders answers "not a folder"
// without asking for a column that is not in the table.
func (repository *CatalogRepository) rowIsFolder(ctx context.Context, transaction pgx.Tx, objectID uuid.UUID, hasFolders bool, rowID uuid.UUID) (bool, bool, error) {
	table, err := PhysicalCatalogTable(objectID)
	if err != nil {
		return false, false, err
	}
	selected := "false"
	if hasFolders {
		selected = "is_folder"
	}
	var folder bool
	err = transaction.QueryRow(ctx, "SELECT "+selected+" FROM "+qualifiedCatalogTable(table)+" WHERE ref = $1", rowID.String()).Scan(&folder)
	if err == pgx.ErrNoRows {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return folder, true, nil
}

// wrapObjectReference hands a reference to application code whatever kind of
// object it points at.
//
// Only a catalog and a document have a reference object of their own so far,
// because only they have a store; a reference to an element of a chart or to a
// node of an exchange plan is handed over as the value it is, the same way an
// attribute holding one already is. An owner may be any of the five, so this
// is where that difference is answered once instead of at each of its callers.
func (runtime *Runtime) wrapObjectReference(kind Kind, metadataID, objectID uuid.UUID) (bytecode.Value, error) {
	switch kind {
	case CatalogKind:
		definition, ok := runtime.catalog.CatalogByID(metadataID)
		if !ok {
			return bytecode.Undefined(), fmt.Errorf("unknown catalog %s", metadataID)
		}
		return runtime.wrapCatalogReference(definition, CatalogReference{CatalogID: metadataID, ObjectID: objectID})
	case DocumentKind:
		definition, ok := runtime.catalog.DocumentByID(metadataID)
		if !ok {
			return bytecode.Undefined(), fmt.Errorf("unknown document %s", metadataID)
		}
		return runtime.wrapDocumentReference(definition, DocumentReference{DocumentID: metadataID, ObjectID: objectID})
	}
	valueType, ok := characteristicKindTypes[kind]
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("%s has no reference to hand over", kind)
	}
	return valueToBSL(Value{Kind: valueType, Object: metadataID, Data: objectID.String()})
}

// objectOfKinds finds one metadata object by identifier among the kinds a
// composite reference is allowed to name. It answers the kind, because the
// identifier alone does not say which table the row is in.
func (catalog *Catalog) objectOfKinds(kinds []Kind, id uuid.UUID) (Kind, bool) {
	for _, kind := range kinds {
		valueType, ok := characteristicKindTypes[kind]
		if !ok {
			continue
		}
		if catalog.hasObject(valueType, id) {
			return kind, true
		}
	}
	return "", false
}
