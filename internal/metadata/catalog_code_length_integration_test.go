package metadata

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A code of length 0 is switched off, and the table has no column for it; the
// prototype shows none in its storage structure and loses every code on a
// round trip 7 → 0 → 7, and gives the rows it already has an empty code when
// a code is lengthened from 0 - all checked by the owner on 01.10.2026.
//
// The defects this catches are three. A column NOT NULL without a default,
// which cannot be added to a table that already has rows: lengthening the code
// from 0 then fails «Сохранить данные». A default spelled otherwise than
// PostgreSQL spells it back, which makes every unchanged schema a migration.
// And shortening to 0 passing as harmless when it throws the codes away.
func TestACodeOfLengthZeroHasNoColumnAndComesBackEmptyIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := integrationDatabase(t, ctx, 0)
	projectID := uuid.MustNew()
	forgetIntegrationProject(t, pool, projectID)

	goodsID, fixedID, numberedID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	catalogWith := func(goodsCode CatalogCode) *Catalog {
		return &Catalog{
			Catalogs: []CatalogDefinition{
				{ID: goodsID, Name: "Товары", Code: goodsCode, DescriptionLength: 100},
				// The two other spellings of an empty code, so that an unchanged
				// schema is seen to compare equal for each.
				{ID: fixedID, Name: "Склады", Code: CatalogCode{Type: StringType, Length: 5, FixedLength: true}, DescriptionLength: 100},
				{ID: numberedID, Name: "Цеха", Code: CatalogCode{Type: NumberType, Length: 5}, DescriptionLength: 100},
			},
			catalogByName: map[string]int{"товары": 0, "склады": 1, "цеха": 2},
			catalogByID:   map[uuid.UUID]int{goodsID: 0, fixedID: 1, numberedID: 2},
		}
	}
	migrateWith := func(catalog *Catalog, consent schemadiff.MigrationConsent) (schemadiff.Plan, error) {
		t.Helper()
		desired, err := catalog.ApplicationSchema()
		if err != nil {
			t.Fatal(err)
		}
		prepared, err := schemadiff.Prepare(ctx, pool, desired)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := schemadiff.Execute(ctx, pool, schemadiff.MigrationRequest{
			ProjectID: projectID, PackageSHA256: strings.Repeat("c", 64), GitCommit: strings.Repeat("d", 40), Desired: desired,
			ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true, Consent: consent,
		}); err != nil {
			return prepared.Plan, err
		}
		actual, err := schemadiff.Inspect(ctx, pool, schemadiff.ApplicationSchema)
		if err != nil {
			t.Fatal(err)
		}
		if again, err := schemadiff.Compare(desired, actual); err != nil || len(again.Changes) != 0 {
			t.Fatalf("an unchanged schema is a migration again: %+v %v", again.Changes, err)
		}
		return prepared.Plan, nil
	}
	migrate := func(catalog *Catalog) schemadiff.Plan {
		t.Helper()
		plan, err := migrateWith(catalog, schemadiff.MigrationConsent{})
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}
	hasCodeColumn := func() bool {
		t.Helper()
		table, _ := PhysicalCatalogTable(goodsID)
		var found bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 AND column_name = 'code')`,
			schemadiff.ApplicationSchema, table).Scan(&found); err != nil {
			t.Fatal(err)
		}
		return found
	}

	// No code: no column, and an element is written and read with the empty
	// code.
	without := catalogWith(CatalogCode{Type: StringType})
	migrate(without)
	if hasCodeColumn() {
		t.Fatal("a code of length 0 got a column")
	}
	repository, err := NewCatalogRepository(pool, without)
	if err != nil {
		t.Fatal(err)
	}
	var written []CatalogReference
	for _, description := range []string{"Первый", "Второй"} {
		record, err := repository.New(ctx, "Товары", nil)
		if err != nil {
			t.Fatal(err)
		}
		record.Description = description
		if err := repository.Save(ctx, record, nil); err != nil {
			t.Fatalf("an element without a code was refused: %v", err)
		}
		written = append(written, record.Reference)
	}
	read, err := repository.Get(ctx, written[0])
	if err != nil || read.Code != "" || read.Description != "Первый" {
		t.Fatalf("read back = %+v, %v", read, err)
	}
	read.Code = "007"
	if err := repository.Save(ctx, read, nil); err == nil || !strings.Contains(err.Error(), "has no code") {
		t.Fatalf("a code was written into a catalog that has none: %v", err)
	}
	if _, _, err := repository.FindByCode(ctx, "Товары", "007"); err == nil || !strings.Contains(err.Error(), "has no code") {
		t.Fatalf("a catalog without a code was searched by one: %v", err)
	}

	// Lengthened to 7: the column is added to a table with rows, harmlessly,
	// and the rows it already has get the empty code.
	lengthened := migrate(catalogWith(CatalogCode{Type: StringType, Length: 7}))
	if lengthened.DestructiveCount != 0 {
		t.Fatalf("adding the code was taken for a destructive change: %+v", lengthened.Changes)
	}
	if !hasCodeColumn() {
		t.Fatal("lengthening the code did not give it a column")
	}
	repository, err = NewCatalogRepository(pool, catalogWith(CatalogCode{Type: StringType, Length: 7}))
	if err != nil {
		t.Fatal(err)
	}
	for _, reference := range written {
		record, err := repository.Get(ctx, reference)
		if err != nil || record.Code != "" {
			t.Fatalf("an element that had no code reads %+v, %v", record, err)
		}
	}

	// Shortened back to 0: the codes are thrown away, and that is not done
	// without the operator agreeing to lose them.
	if _, err := migrateWith(catalogWith(CatalogCode{Type: StringType}), schemadiff.MigrationConsent{}); err == nil {
		t.Fatal("the codes were dropped without consent")
	}
	if !hasCodeColumn() {
		t.Fatal("a refused migration dropped the codes anyway")
	}
	shortened, err := migrateWith(catalogWith(CatalogCode{Type: StringType}), schemadiff.MigrationConsent{ObjectLoss: true})
	if err != nil {
		t.Fatal(err)
	}
	if shortened.ObjectLossCount == 0 {
		t.Fatalf("dropping the codes passed as harmless: %+v", shortened.Changes)
	}
	if hasCodeColumn() {
		t.Fatal("shortening the code to 0 left its column")
	}
}

// The description is switched off and brought back the way the code is: no
// column at 0, and the empty description for the rows a table already has
// when it is lengthened.
func TestADescriptionOfLengthZeroHasNoColumnAndComesBackEmptyIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := integrationDatabase(t, ctx, 0)
	projectID := uuid.MustNew()
	goodsID := uuid.MustNew()
	catalogWith := func(length int) *Catalog {
		return &Catalog{
			Catalogs:      []CatalogDefinition{{ID: goodsID, Name: "Товары", Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: length}},
			catalogByName: map[string]int{"товары": 0}, catalogByID: map[uuid.UUID]int{goodsID: 0},
		}
	}
	migrate := func(catalog *Catalog) {
		t.Helper()
		migrateIntegrationSchema(t, ctx, pool, catalog, projectID)
	}
	migrate(catalogWith(0))
	repository, err := NewCatalogRepository(pool, catalogWith(0))
	if err != nil {
		t.Fatal(err)
	}
	record, err := repository.New(ctx, "Товары", nil)
	if err != nil {
		t.Fatal(err)
	}
	record.Code = "001"
	if err := repository.Save(ctx, record, nil); err != nil {
		t.Fatalf("an element without a description was refused: %v", err)
	}
	migrate(catalogWith(50))
	repository, err = NewCatalogRepository(pool, catalogWith(50))
	if err != nil {
		t.Fatal(err)
	}
	read, err := repository.Get(ctx, record.Reference)
	if err != nil || read.Description != "" || read.Code != "001" {
		t.Fatalf("an element that had no description reads %+v, %v", read, err)
	}
}
