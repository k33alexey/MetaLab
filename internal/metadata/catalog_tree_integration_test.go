package metadata

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// treeFixture is a configuration with a nesting limit and a catalog
// subordinate to the nested one, which is what the branch of a deletion mark
// has to reach.
func treeFixture(t *testing.T, ctx context.Context, levels int) (*CatalogRepository, uuid.UUID, uuid.UUID) {
	t.Helper()
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	projectID, treeID, ownedID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_, _ = pool.Exec(cleanup, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE")
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.object_sequences WHERE metadata_id = ANY($1::uuid[])",
			[]string{treeID.String(), ownedID.String()})
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
		pool.Close()
	})
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE"); err != nil {
		t.Fatal(err)
	}
	hierarchy := Hierarchy{Enabled: true, Kind: FoldersAndItemsHierarchy}
	if levels > 0 {
		hierarchy.LimitLevels, hierarchy.LevelCount = true, levels
	}
	catalog := &Catalog{
		Catalogs: []CatalogDefinition{
			{ID: treeID, Name: "Дерево", DescriptionLength: 150, Hierarchy: hierarchy,
				Code: CatalogCode{Type: StringType, Length: 9, Auto: true}},
			{ID: ownedID, Name: "Подчинённые", DescriptionLength: 150,
				Code: CatalogCode{Type: StringType, Length: 9, Auto: true},
				// Subordinate to either, because the branch starts at a folder
				// and the rows under it are items.
				Owners: []uuid.UUID{treeID}, Subordination: SubordinateToFoldersAndItem},
		},
		catalogByName: map[string]int{"дерево": 0, "подчинённые": 1},
		catalogByID:   map[uuid.UUID]int{treeID: 0, ownedID: 1},
	}
	desired, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := schemadiff.Prepare(ctx, pool, desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schemadiff.Execute(ctx, pool, schemadiff.MigrationRequest{
		ProjectID: projectID, PackageSHA256: repeatCatalogHex('1', 64), GitCommit: repeatCatalogHex('2', 40), Desired: desired,
		ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := EnsureObjectIntegrityStorage(ctx, pool); err != nil {
		t.Fatal(err)
	}
	repository, err := NewCatalogRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	return repository, treeID, ownedID
}

// newFolderUnder and newItemUnder write one row and return it.
func newFolderUnder(t *testing.T, ctx context.Context, repository *CatalogRepository, name, description string, parent uuid.UUID) *CatalogRecord {
	t.Helper()
	record, err := repository.NewFolder(ctx, name, nil)
	if err != nil {
		t.Fatal(err)
	}
	record.Description, record.Parent = description, parent
	if err := repository.Save(ctx, record, nil); err != nil {
		t.Fatalf("%s: %v", description, err)
	}
	return record
}

func newItemUnder(t *testing.T, ctx context.Context, repository *CatalogRepository, name, description string, parent uuid.UUID) *CatalogRecord {
	t.Helper()
	record, err := repository.New(ctx, name, nil)
	if err != nil {
		t.Fatal(err)
	}
	record.Description, record.Parent = description, parent
	if err := repository.Save(ctx, record, nil); err != nil {
		t.Fatalf("%s: %v", description, err)
	}
	return record
}

// A row cannot be moved under something that already stands below it. Nothing
// in the schema prevents this: a foreign key says the parent is there, not
// that it is above, and a ring makes every walk over the tree run forever.
func TestARowCannotStandUnderItsOwnBranchIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	repository, _, _ := treeFixture(t, ctx, 0)
	top := newFolderUnder(t, ctx, repository, "Дерево", "Верх", uuid.UUID{})
	middle := newFolderUnder(t, ctx, repository, "Дерево", "Середина", top.Reference.ObjectID)
	bottom := newFolderUnder(t, ctx, repository, "Дерево", "Низ", middle.Reference.ObjectID)

	moved, err := repository.Get(ctx, top.Reference)
	if err != nil {
		t.Fatal(err)
	}
	moved.Parent = bottom.Reference.ObjectID
	if err := repository.Save(ctx, moved, nil); err == nil || !strings.Contains(err.Error(), "which is below it") {
		t.Fatalf("a ring of parents was accepted: %v", err)
	}
	// Moving the same branch sideways is not a ring and stays allowed.
	sibling := newFolderUnder(t, ctx, repository, "Дерево", "Сосед", uuid.UUID{})
	moved, err = repository.Get(ctx, middle.Reference)
	if err != nil {
		t.Fatal(err)
	}
	moved.Parent = sibling.Reference.ObjectID
	if err := repository.Save(ctx, moved, nil); err != nil {
		t.Fatalf("moving a branch sideways was refused: %v", err)
	}
}

// Two moves, each correct on its own, that together close a ring. This is what
// the lock on the tree is for: without it both walks see a tree without a ring,
// both commit, and the ring appears between them.
func TestTwoMovesCannotCloseARingTogetherIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	repository, _, _ := treeFixture(t, ctx, 0)
	first := newFolderUnder(t, ctx, repository, "Дерево", "Первая", uuid.UUID{})
	second := newFolderUnder(t, ctx, repository, "Дерево", "Вторая", uuid.UUID{})

	move := func(from, to CatalogReference) error {
		record, err := repository.Get(ctx, from)
		if err != nil {
			return err
		}
		record.Parent = to.ObjectID
		return repository.Save(ctx, record, nil)
	}
	var wait sync.WaitGroup
	errors := make([]error, 2)
	wait.Add(2)
	go func() { defer wait.Done(); errors[0] = move(first.Reference, second.Reference) }()
	go func() { defer wait.Done(); errors[1] = move(second.Reference, first.Reference) }()
	wait.Wait()

	failed := 0
	for _, err := range errors {
		if err != nil {
			failed++
			if !strings.Contains(err.Error(), "which is below it") {
				t.Fatalf("refused for another reason: %v", err)
			}
		}
	}
	if failed != 1 {
		t.Fatalf("exactly one of the two moves must be refused, and %d were: %v", failed, errors)
	}
}

// A nesting limit is a limit on rows, and a row carries what is under it: a
// branch moved deeper takes its own height with it.
func TestNestingStopsAtTheLimitIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	repository, _, _ := treeFixture(t, ctx, 3)
	first := newFolderUnder(t, ctx, repository, "Дерево", "Первый", uuid.UUID{})
	second := newFolderUnder(t, ctx, repository, "Дерево", "Второй", first.Reference.ObjectID)
	third := newFolderUnder(t, ctx, repository, "Дерево", "Третий", second.Reference.ObjectID)

	// A fourth level at once.
	deeper, err := repository.New(ctx, "Дерево", nil)
	if err != nil {
		t.Fatal(err)
	}
	deeper.Description, deeper.Parent = "Четвёртый", third.Reference.ObjectID
	if err := repository.Save(ctx, deeper, nil); err == nil || !strings.Contains(err.Error(), "allows 3 levels") {
		t.Fatalf("a fourth level was accepted: %v", err)
	}

	// A branch two rows tall moved one level down: the row itself would stand
	// at the second level and its deepest row at the fourth.
	elsewhere := newFolderUnder(t, ctx, repository, "Дерево", "Сбоку", uuid.UUID{})
	moved, err := repository.Get(ctx, first.Reference)
	if err != nil {
		t.Fatal(err)
	}
	moved.Parent = elsewhere.Reference.ObjectID
	if err := repository.Save(ctx, moved, nil); err == nil || !strings.Contains(err.Error(), "allows 3 levels") {
		t.Fatalf("a branch was moved past the limit: %v", err)
	}
}

// Marking a row for deletion marks the branch under it by default, and the
// branch is two things at once: the rows below it in this catalog and the rows
// of every catalog subordinate to them.
func TestMarkingARowMarksTheBranchIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	repository, _, _ := treeFixture(t, ctx, 0)
	folder := newFolderUnder(t, ctx, repository, "Дерево", "Папка", uuid.UUID{})
	item := newItemUnder(t, ctx, repository, "Дерево", "Элемент", folder.Reference.ObjectID)
	aside := newItemUnder(t, ctx, repository, "Дерево", "В стороне", uuid.UUID{})

	owned, err := repository.New(ctx, "Подчинённые", nil)
	if err != nil {
		t.Fatal(err)
	}
	owned.Description = "Подчинённый элементу"
	owned.Owner = Value{Kind: CatalogType, Object: item.Reference.CatalogID, Data: item.Reference.ObjectID.String()}
	if err := repository.Save(ctx, owned, nil); err != nil {
		t.Fatal(err)
	}

	marked, err := repository.Get(ctx, folder.Reference)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.SetDeletionMark(ctx, marked, true, true, nil); err != nil {
		t.Fatal(err)
	}
	for name, reference := range map[string]CatalogReference{
		"папка": folder.Reference, "элемент под ней": item.Reference, "подчинённый элементу": owned.Reference,
	} {
		row, err := repository.Get(ctx, reference)
		if err != nil || !row.DeletionMark {
			t.Fatalf("%s не помечен: %+v error=%v", name, row, err)
		}
	}
	untouched, err := repository.Get(ctx, aside.Reference)
	if err != nil || untouched.DeletionMark {
		t.Fatalf("a row outside the branch was marked: %+v error=%v", untouched, err)
	}

	// Taking the mark off walks the same branch.
	cleared, err := repository.Get(ctx, folder.Reference)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.SetDeletionMark(ctx, cleared, false, true, nil); err != nil {
		t.Fatal(err)
	}
	row, err := repository.Get(ctx, owned.Reference)
	if err != nil || row.DeletionMark {
		t.Fatalf("the subordinate row kept its mark: %+v error=%v", row, err)
	}

	// And a caller that means one row says so.
	alone, err := repository.Get(ctx, folder.Reference)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.SetDeletionMark(ctx, alone, true, false, nil); err != nil {
		t.Fatal(err)
	}
	below, err := repository.Get(ctx, item.Reference)
	if err != nil || below.DeletionMark {
		t.Fatalf("the branch was marked although only one row was asked for: %+v error=%v", below, err)
	}
}
