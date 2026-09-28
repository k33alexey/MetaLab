package metadata

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The whole point of the iteration, checked against a real database: the mode
// lives in the base, a write follows it, and switching it changes no number.
//
// Switching is cheap precisely because reading sums the rows across splits.
// Rows written under one mode are read beside rows written under the other,
// and a rebuild folds them together - «при пересчете итогов накопленные
// отдельные записи сворачиваются».
func TestTotalsModeIsFollowedAndSwitchingChangesNoNumber(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	projectID := uuid.MustNew()
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, _ = pool.Exec(cleanup, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE")
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
		pool.Close()
	})
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE"); err != nil {
		t.Fatal(err)
	}
	if err := EnsureRegisterTotalsStorage(ctx, pool); err != nil {
		t.Fatal(err)
	}

	documentID, registerID, productID, amountID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{
		Documents: []DocumentDefinition{{
			ID: documentID, Name: "Продажа", Number: DocumentNumber{Type: StringType, Length: 20, Unique: true, Periodicity: NumberPeriodYear},
			Movements: []uuid.UUID{registerID},
		}},
		AccumulationRegisters: []AccumulationRegisterDefinition{{
			ID: registerID, Name: "Продажи", Kind: AccumulationRegisterTurnover, AllowTotalsSplitting: true,
			Dimensions: []RegisterDimension{{Attribute: Attribute{ID: productID, Name: "Товар", Types: []Type{{Kind: StringType, Length: 100}}}}},
			Resources:  []Attribute{{ID: amountID, Name: "Сумма", Types: []Type{{Kind: NumberType, Precision: 15, Scale: 2}}}},
		}},
		documentByName: map[string]int{"продажа": 0}, documentByID: map[uuid.UUID]int{documentID: 0},
		accumulationRegisterByName: map[string]int{"продажи": 0}, accumulationRegisterByID: map[uuid.UUID]int{registerID: 0},
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
		ProjectID: projectID, PackageSHA256: repeatCatalogHex('c', 64), GitCommit: repeatCatalogHex('d', 40),
		Desired: desired, ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}

	documents, err := NewDocumentRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	registers, err := NewAccumulationRegisterRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	month := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	sell := func(number, amount string) DocumentReference {
		document, createErr := documents.New(ctx, "Продажа", nil)
		if createErr != nil {
			t.Fatal(createErr)
		}
		document.Number, document.Date = number, month
		if saveErr := documents.Save(ctx, document, nil); saveErr != nil {
			t.Fatal(saveErr)
		}
		set, setErr := registers.NewRecordSet("Продажи")
		if setErr != nil {
			t.Fatal(setErr)
		}
		set.Filter.Recorder = &document.Reference
		row, addErr := set.Add()
		if addErr != nil {
			t.Fatal(addErr)
		}
		row.Period, row.Recorder, row.Active = month, document.Reference, true
		row.Dimensions[productID] = Value{Kind: StringType, Data: "A"}
		row.Resources[amountID] = Value{Kind: NumberType, Data: amount}
		if writeErr := registers.Write(ctx, set, true); writeErr != nil {
			t.Fatal(writeErr)
		}
		return document.Reference
	}
	totalsTable, _ := PhysicalAccumulationRegisterTotalsTable(registerID)
	splits := func() []int16 {
		rows, queryErr := pool.Query(ctx, "SELECT totals_split FROM "+qualifiedCatalogTable(totalsTable)+" ORDER BY totals_split")
		if queryErr != nil {
			t.Fatal(queryErr)
		}
		defer rows.Close()
		var found []int16
		for rows.Next() {
			var split int16
			if scanErr := rows.Scan(&split); scanErr != nil {
				t.Fatal(scanErr)
			}
			found = append(found, split)
		}
		return found
	}
	turnover := func() string {
		result, queryErr := registers.Turnovers(ctx, "Продажи", month, month, map[uuid.UUID]Value{productID: {Kind: StringType, Data: "A"}})
		if queryErr != nil {
			t.Fatal(queryErr)
		}
		if len(result) != 1 {
			t.Fatalf("turnover rows = %d", len(result))
		}
		return result[0].Turnover[amountID].Data
	}

	// Two documents written one after the other collide with nobody, so the
	// mode being on costs nothing: both changes go into the same single row.
	// Rows multiply under concurrency and only under it - see the test below.
	sell("S-1", "100")
	sell("S-2", "200")
	if turnover() != "300" {
		t.Fatalf("turnover with the mode on = %q", turnover())
	}
	if rows := splits(); len(rows) != 1 || rows[0] != 0 {
		t.Fatalf("writes that met nobody still multiplied the totals: %v", rows)
	}

	// Switching the mode off changes no number and makes no row.
	if err := SetTotalsSplitting(ctx, pool, registerID, true, false); err != nil {
		t.Fatal(err)
	}
	sell("S-3", "50")
	if turnover() != "350" {
		t.Fatalf("turnover across both modes = %q", turnover())
	}
	if rows := splits(); len(rows) != 1 {
		t.Fatalf("a write with the mode off made a second row: %v", rows)
	}

	// A rebuild folds the rows of both modes into one per combination, and the
	// number does not move.
	if err := registers.RebuildTotals(ctx, "Продажи"); err != nil {
		t.Fatal(err)
	}
	if folded := splits(); len(folded) != 1 || folded[0] != 0 {
		t.Fatalf("the rebuild did not fold the rows: %v", folded)
	}
	if turnover() != "350" {
		t.Fatalf("turnover after the rebuild = %q", turnover())
	}

	// Turning it back on is the same switch in reverse, and the number stays.
	if err := SetTotalsSplitting(ctx, pool, registerID, true, true); err != nil {
		t.Fatal(err)
	}
	sell("S-4", "10")
	if turnover() != "360" {
		t.Fatalf("turnover after switching back on = %q", turnover())
	}
}
