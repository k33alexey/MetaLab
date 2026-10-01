package metadata

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The tree a catalog's rows stand in, once the rows are actually in it.
//
// Three things become checkable the moment the parent is written, and none of
// them is a property to carry: a ring of parents, a nesting deeper than the
// object allows, and marking a whole branch for deletion at once. The first
// two are refusals, the third is what the prototype does by default - see
// SetDeletionMark.
//
// A ring is not a curiosity. A row whose parent chain loops makes every walk
// over the tree run forever: opening the list, reading the level, marking a
// branch. Nothing else in the schema prevents it - a foreign key says the
// parent is there, not that it is above.

// maxTreeWalk stops a walk that a ring already in the data would otherwise run
// forever. Nesting is limited to 32 levels where it is limited at all, so this
// is a guard against broken data rather than a limit on anybody's tree.
const maxTreeWalk = 10_000

// treeWrite is what one write has to know about the tree before it takes any
// lock. It exists because the answer decides which locks are taken, and locks
// have to be taken in one order.
type treeWrite struct {
	// reparents says this write moves a row that is already there under a
	// different parent. Only such a write can close a ring: a row being
	// inserted is pointed at by nobody yet, and a row moved to the top of the
	// tree loses ancestors rather than gaining them.
	reparents bool
	// storedParent is what the row's parent was when the question was asked,
	// before the row was locked. Checked again under the lock.
	storedParent uuid.UUID
	// known says storedParent was read at all.
	known bool
}

// prepareTreeWrite decides whether this write can close a ring, and takes the
// lock that stops two of them closing one together.
//
// The lock is on the catalog, not on the row, and it is taken before the row
// lock rather than after. Both of those are the point:
//
//   - two writes, each moving a row under the other, are each correct on their
//     own. A lock on the row being written is held by each of them over its own
//     row, so neither waits for the other, both walk a tree without a ring, and
//     the ring appears when the second commits. The walk has to happen under a
//     lock that covers the whole tree it walks.
//   - taking it before the row lock keeps one order for both locks. Taking it
//     after would let one transaction hold the tree and want a row while
//     another holds that row and wants the tree, which PostgreSQL resolves by
//     killing one of them.
//
// What it does not do is lock the catalog for every write. An insert never
// needs it, and neither does a save that leaves the parent where it was, so
// loading a configuration and editing rows in bulk are untouched; only moving
// a row under a new parent waits for another move in the same catalog.
//
// The parent is read here without any lock, so it may be stale by the time the
// row lock is taken. checkTreePlacement reads it again under the lock and
// refuses the write as a conflict if it moved, which is the same answer the
// version check would give a moment later.
func (repository *CatalogRepository) prepareTreeWrite(ctx context.Context, transaction pgx.Tx, definition CatalogDefinition, record *CatalogRecord) (treeWrite, error) {
	if !catalogHasParent(definition) || record.Version == 0 {
		return treeWrite{}, nil
	}
	stored, found, err := repository.storedRowParent(ctx, transaction, definition, record.Reference.ObjectID)
	if err != nil {
		return treeWrite{}, fmt.Errorf("catalog %s: %w", definition.Name, err)
	}
	if !found {
		return treeWrite{}, nil
	}
	result := treeWrite{storedParent: stored, known: true, reparents: stored != record.Parent && !record.Parent.IsZero()}
	if !result.reparents {
		return result, nil
	}
	if err := lockCatalogTree(ctx, transaction, definition.ID); err != nil {
		return treeWrite{}, fmt.Errorf("catalog %s: %w", definition.Name, err)
	}
	return result, nil
}

// checkTreePlacement is the part of checkPlacement that walks the tree: the
// ring, the depth, and the confirmation that the row has not been moved by
// somebody else since prepareTreeWrite looked.
func (repository *CatalogRepository) checkTreePlacement(ctx context.Context, transaction pgx.Tx, definition CatalogDefinition, record *CatalogRecord, tree treeWrite, storedParent uuid.UUID, stored bool) error {
	if tree.known && stored && tree.storedParent != storedParent {
		// Somebody moved this row between the question and the lock. The tree
		// lock was decided on the old answer, so this write is not the one to
		// go on: it is refused as the conflict it is, and the caller reads and
		// tries again.
		return ErrCatalogWriteConflict
	}
	// A row at the top has nothing above it and no ring to close, but it is
	// still counted: with one level a folder at the top is already too deep.
	depth := 0
	if !record.Parent.IsZero() {
		var meetsRow bool
		var err error
		depth, meetsRow, err = repository.ancestorChain(ctx, transaction, definition, record.Parent, record.Reference.ObjectID)
		if err != nil {
			return fmt.Errorf("catalog %s parent: %w", definition.Name, err)
		}
		if meetsRow {
			return fmt.Errorf("catalog %s row %s cannot stand under %s, which is below it", definition.Name, record.Reference.ObjectID, record.Parent)
		}
	}
	if !definition.Hierarchy.LimitLevels {
		return nil
	}
	// A row carries what is under it. Moving a branch deeper moves all of it,
	// so the levels the branch takes count towards the limit as much as the
	// depth of the new place does. A folder takes one more level than it
	// stands at, for what it holds: the level of items is a level, and with
	// two of them a folder at the top holds items and a folder in a folder is
	// refused - the prototype, checked on the platform 01.10.2026.
	folders := catalogHasFolders(definition)
	levels := 1
	if folders && record.IsFolder {
		levels = 2
	}
	if record.Version != 0 {
		below, err := repository.branchLevels(ctx, transaction, definition, record.Reference.ObjectID, folders)
		if err != nil {
			return fmt.Errorf("catalog %s: %w", definition.Name, err)
		}
		levels = max(levels, below)
	}
	if depth+levels > definition.Hierarchy.LevelCount {
		if folders {
			return fmt.Errorf("catalog %s allows %d levels of nesting counting the level of items, and this branch would take %d",
				definition.Name, definition.Hierarchy.LevelCount, depth+levels)
		}
		return fmt.Errorf("catalog %s allows %d levels of nesting, and this row would stand at %d",
			definition.Name, definition.Hierarchy.LevelCount, depth+levels)
	}
	return nil
}

// storedRowParent reads one row's parent. It is a question about the row as it
// is in the database, which is not the same as the record being written.
func (repository *CatalogRepository) storedRowParent(ctx context.Context, transaction pgx.Tx, definition CatalogDefinition, rowID uuid.UUID) (uuid.UUID, bool, error) {
	table, err := PhysicalCatalogTable(definition.ID)
	if err != nil {
		return uuid.UUID{}, false, err
	}
	var parent *string
	err = transaction.QueryRow(ctx, "SELECT parent::text FROM "+qualifiedCatalogTable(table)+" WHERE ref = $1", rowID.String()).Scan(&parent)
	if err == pgx.ErrNoRows {
		return uuid.UUID{}, false, nil
	}
	if err != nil {
		return uuid.UUID{}, false, err
	}
	if parent == nil {
		return uuid.UUID{}, true, nil
	}
	id, err := uuid.Parse(*parent)
	return id, true, err
}

// ancestorChain walks up from one row: how many rows stand from the top of the
// tree down to it, and whether the row being written is among them. Meeting it
// is the ring.
func (repository *CatalogRepository) ancestorChain(ctx context.Context, transaction pgx.Tx, definition CatalogDefinition, from, target uuid.UUID) (int, bool, error) {
	table, err := PhysicalCatalogTable(definition.ID)
	if err != nil {
		return 0, false, err
	}
	qualified := qualifiedCatalogTable(table)
	statement := `WITH RECURSIVE chain(ref, parent, depth) AS (
	SELECT ref, parent, 1 FROM ` + qualified + ` WHERE ref = $1
	UNION ALL
	SELECT item.ref, item.parent, chain.depth + 1 FROM ` + qualified + ` AS item
		JOIN chain ON item.ref = chain.parent WHERE chain.depth < $3
)
SELECT coalesce(max(depth), 0), coalesce(bool_or(ref = $2), false) FROM chain`
	var depth int
	var meets bool
	if err := transaction.QueryRow(ctx, statement, from.String(), target.String(), maxTreeWalk).Scan(&depth, &meets); err != nil {
		return 0, false, err
	}
	return depth, meets, nil
}

// branchLevels is how many levels the branch from one row takes, the row's
// own level being the first: the deepest row counts, and a folder counts one
// more, for the level of what it holds.
func (repository *CatalogRepository) branchLevels(ctx context.Context, transaction pgx.Tx, definition CatalogDefinition, root uuid.UUID, folders bool) (int, error) {
	table, err := PhysicalCatalogTable(definition.ID)
	if err != nil {
		return 0, err
	}
	qualified := qualifiedCatalogTable(table)
	// Only a hierarchy of folders and items has the folder column.
	rootRoom, itemRoom := "0", "0"
	if folders {
		rootRoom, itemRoom = "is_folder::integer", "item.is_folder::integer"
	}
	statement := `WITH RECURSIVE below(ref, depth, room) AS (
	SELECT ref, 1, ` + rootRoom + ` FROM ` + qualified + ` WHERE ref = $1
	UNION ALL
	SELECT item.ref, below.depth + 1, ` + itemRoom + ` FROM ` + qualified + ` AS item
		JOIN below ON item.parent = below.ref WHERE below.depth <= $2
)
SELECT coalesce(max(depth + room), 0) FROM below`
	var levels int
	if err := transaction.QueryRow(ctx, statement, root.String(), maxTreeWalk).Scan(&levels); err != nil {
		return 0, err
	}
	return levels, nil
}

// SetDeletionMark marks a row for deletion, or takes the mark off, and by
// default does the same to everything below it.
//
// "Below" is two things at once, and the prototype says both in one sentence:
// the rows under this one in the same catalog, and the rows of every catalog
// subordinate to it - theirs too, all the way down. A partner is marked, and
// with it the contracts of that partner and the files of those contracts.
//
// The default is to include them. That is the prototype's default and it is
// the surprising half of this method: marking one row usually marks a branch,
// and a caller that means one row says so.
//
// Every affected row is written the way any row is written, through the same
// events, rights and version check. It is slower than one UPDATE over the
// branch and it is the only version that keeps a promise the rest of the
// platform makes - that nothing is written behind the object module's back.
// The branch reaches rows of other catalogs, and a row is written with the
// events of the catalog it belongs to, not with those of the one the caller
// named. So the caller hands over a way to find a handler per catalog rather
// than one handler: the runtime keeps exactly such a registry, and a caller
// with no modules passes nothing.
type catalogHandlers func(catalogID uuid.UUID) CatalogEventHandler

func (handlers catalogHandlers) forCatalog(catalogID uuid.UUID) CatalogEventHandler {
	if handlers == nil {
		return nil
	}
	return handlers(catalogID)
}

func (repository *CatalogRepository) SetDeletionMark(ctx context.Context, record *CatalogRecord, mark, includeSubordinate bool, handlers catalogHandlers) error {
	if record == nil {
		return fmt.Errorf("catalog record is required")
	}
	definition, ok := repository.catalog.CatalogByID(record.Reference.CatalogID)
	if !ok {
		return fmt.Errorf("unknown catalog %s", record.Reference.CatalogID)
	}
	own := handlers.forCatalog(definition.ID)
	if !includeSubordinate {
		record.DeletionMark = mark
		return repository.Save(ctx, record, own)
	}
	return runDataTransaction(ctx, repository.pool, nil, func(transactionContext context.Context, transaction pgx.Tx) error {
		below, err := repository.rowsBelow(transactionContext, transaction, definition.ID, record.Reference.ObjectID)
		if err != nil {
			return err
		}
		for _, reference := range below {
			subordinate, err := repository.Get(transactionContext, reference)
			if err != nil {
				return err
			}
			if subordinate.DeletionMark == mark {
				continue
			}
			subordinate.DeletionMark = mark
			if err := repository.Save(transactionContext, subordinate, handlers.forCatalog(reference.CatalogID)); err != nil {
				return err
			}
		}
		record.DeletionMark = mark
		return repository.Save(transactionContext, record, own)
	})
}

// rowsBelow collects everything that follows one row: its children in the same
// catalog and the rows of every catalog subordinate to it, and then the same
// question about each of those.
//
// The rows come back in a settled order - by catalog as the configuration
// lists them, by identifier within one - so that two runs of the same mark
// write the same rows in the same order, and a deadlock between two of them is
// not a matter of luck.
//
// Subordination between catalogs may be a ring where the tree may not: a
// catalog owned by another that is owned by the first is nothing the
// configuration forbids. The visited set is what keeps this walk finite.
func (repository *CatalogRepository) rowsBelow(ctx context.Context, transaction pgx.Tx, catalogID, rowID uuid.UUID) ([]CatalogReference, error) {
	visited := map[CatalogReference]bool{{CatalogID: catalogID, ObjectID: rowID}: true}
	queue := []CatalogReference{{CatalogID: catalogID, ObjectID: rowID}}
	var result []CatalogReference
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		found, err := repository.rowsUnder(ctx, transaction, current)
		if err != nil {
			return nil, err
		}
		for _, reference := range found {
			if visited[reference] {
				continue
			}
			visited[reference] = true
			result = append(result, reference)
			queue = append(queue, reference)
		}
	}
	sort.Slice(result, func(first, second int) bool {
		if result[first].CatalogID != result[second].CatalogID {
			return result[first].CatalogID.String() < result[second].CatalogID.String()
		}
		return result[first].ObjectID.String() < result[second].ObjectID.String()
	})
	return result, nil
}

// rowsUnder is one step of that walk: the children of one row and the rows
// belonging to it.
func (repository *CatalogRepository) rowsUnder(ctx context.Context, transaction pgx.Tx, reference CatalogReference) ([]CatalogReference, error) {
	definition, ok := repository.catalog.CatalogByID(reference.CatalogID)
	if !ok {
		return nil, fmt.Errorf("unknown catalog %s", reference.CatalogID)
	}
	var result []CatalogReference
	if catalogHasParent(definition) {
		children, err := repository.rowsMatching(ctx, transaction, definition.ID, "parent = $1", reference.ObjectID.String())
		if err != nil {
			return nil, fmt.Errorf("catalog %s children: %w", definition.Name, err)
		}
		result = append(result, children...)
	}
	for _, subordinate := range repository.catalog.Catalogs {
		owns := false
		for _, owner := range subordinate.Owners {
			if owner == reference.CatalogID {
				owns = true
			}
		}
		if !owns {
			continue
		}
		predicate, arguments := "owner = $1", []any{reference.ObjectID.String()}
		if len(subordinate.Owners) > 1 {
			predicate = "owner_type = $1 AND owner_ref = $2"
			arguments = []any{reference.CatalogID.String(), reference.ObjectID.String()}
		}
		owned, err := repository.rowsMatching(ctx, transaction, subordinate.ID, predicate, arguments...)
		if err != nil {
			return nil, fmt.Errorf("catalog %s subordinate rows: %w", subordinate.Name, err)
		}
		result = append(result, owned...)
	}
	return result, nil
}

func (repository *CatalogRepository) rowsMatching(ctx context.Context, transaction pgx.Tx, catalogID uuid.UUID, predicate string, arguments ...any) ([]CatalogReference, error) {
	table, err := PhysicalCatalogTable(catalogID)
	if err != nil {
		return nil, err
	}
	rows, err := transaction.Query(ctx, "SELECT ref::text FROM "+qualifiedCatalogTable(table)+" WHERE "+predicate+" ORDER BY ref", arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []CatalogReference
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return nil, err
		}
		id, err := uuid.Parse(text)
		if err != nil {
			return nil, err
		}
		result = append(result, CatalogReference{CatalogID: catalogID, ObjectID: id})
	}
	return result, rows.Err()
}
