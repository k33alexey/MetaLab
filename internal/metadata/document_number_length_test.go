package metadata

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/querylang"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A number of length 0 switches the number of a document off, as a code of
// length 0 does a catalog's - checked by the owner on 01.10.2026; 9 documents
// of the configurations being moved have none, and every one was refused on
// import. No column, no field on the object or a form, nothing to query,
// search or grant; input by string naming it is refused. Only a document was
// checked: a business process keeps a number of at least 1.
func TestANumberOfLengthZeroIsSwitchedOff(t *testing.T) {
	t.Parallel()
	body := func(extra string) string {
		return "format: 1\nid: " + documentID + "\nname: Акт\ntitle: {ru: Акт}\nnumber: {type: string, length: 0, periodicity: none}\n" + extra
	}
	if _, err := DecodeDocument("object.yaml", strings.NewReader(body("")), metadataConfiguration()); err != nil {
		t.Fatalf("a document without a number was refused: %v", err)
	}
	if _, err := DecodeDocument("object.yaml", strings.NewReader(body("input_by_string:\n  - {standard: Номер}\n")), metadataConfiguration()); err == nil ||
		!strings.Contains(err.Error(), "Номер is switched off by a length of 0") {
		t.Fatalf("input by a switched off number: %v", err)
	}
	if issues := validateNumberShape(DocumentNumber{Type: StringType, Periodicity: NumberPeriodNone}); len(issues) == 0 {
		t.Fatal("a number of length 0 was taken where it was not checked")
	}

	docID := uuid.MustNew()
	definition := DocumentDefinition{ID: docID, Name: "Акт", Number: DocumentNumber{Type: StringType, Periodicity: NumberPeriodNone}}
	catalog := &Catalog{Documents: []DocumentDefinition{definition},
		documentByName: map[string]int{"акт": 0}, documentByID: map[uuid.UUID]int{docID: 0}}
	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	table, _ := PhysicalDocumentTable(docID)
	for _, candidate := range schema.Tables {
		if candidate.Name != table {
			continue
		}
		for _, column := range candidate.Columns {
			if column.Name == "number" {
				t.Fatal("the table got a number column")
			}
		}
		for _, index := range candidate.Indexes {
			if strings.Contains(strings.Join(index.Keys, ","), "number,") || strings.HasSuffix(strings.Join(index.Keys, ","), ",number") {
				t.Fatalf("the table got an index by the number: %+v", index)
			}
		}
	}
	runtime := &Runtime{catalog: catalog}
	parsed, err := querylang.Parse(`ВЫБРАТЬ Д.Номер ИЗ Документ.Акт КАК Д`)
	if err != nil {
		t.Fatal(err)
	}
	compiler, err := runtime.newQueryCompiler(context.Background(), parsed, map[string]bytecode.Value{}, nil)
	if err == nil {
		_, err = compiler.compile(parsed)
	}
	if err == nil {
		t.Fatal("a query read the number of a document that has none")
	}
	form, err := catalog.DocumentForm("Акт", ObjectForm, "ru")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range form.Fields {
		if field.Name == "Number" {
			t.Fatal("the generated form shows a number")
		}
	}
	if len(form.List.SearchFields) != 0 {
		t.Fatalf("the list is searched by %v", form.List.SearchFields)
	}
	object := &documentObject{definition: definition, runtime: runtime,
		record: &DocumentRecord{Reference: DocumentReference{DocumentID: docID, ObjectID: uuid.MustNew()}}}
	if _, err := runtime.GetObjectProperty(context.Background(), object, "Номер"); err == nil {
		t.Fatal("the object of a numberless document has a number")
	}
}

// Lengthened from 0, the number is added to a table with documents in it, and
// they get the empty number; without a default that could not be done at all.
func TestANumberLengthenedFromZeroComesBackEmptyIntegration(t *testing.T) {
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
	docID := uuid.MustNew()
	catalogWith := func(length int) *Catalog {
		return &Catalog{
			Documents:      []DocumentDefinition{{ID: docID, Name: "Акт", Number: DocumentNumber{Type: StringType, Length: length, Periodicity: NumberPeriodNone}}},
			documentByName: map[string]int{"акт": 0}, documentByID: map[uuid.UUID]int{docID: 0},
		}
	}
	migrate := func(catalog *Catalog) {
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
			ProjectID: projectID, PackageSHA256: repeatCatalogHex('a', 64), GitCommit: repeatCatalogHex('b', 40), Desired: desired,
			ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true,
		}); err != nil {
			t.Fatal(err)
		}
		actual, err := schemadiff.Inspect(ctx, pool, schemadiff.ApplicationSchema)
		if err != nil {
			t.Fatal(err)
		}
		if again, err := schemadiff.Compare(desired, actual); err != nil || len(again.Changes) != 0 {
			t.Fatalf("an unchanged schema is a migration again: %+v %v", again.Changes, err)
		}
	}
	migrate(catalogWith(0))
	repository, err := NewDocumentRepository(pool, catalogWith(0))
	if err != nil {
		t.Fatal(err)
	}
	record, err := repository.New(ctx, "Акт", nil)
	if err != nil {
		t.Fatal(err)
	}
	record.Date = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if err := repository.Save(ctx, record, nil); err != nil {
		t.Fatalf("a document without a number was refused: %v", err)
	}
	migrate(catalogWith(11))
	repository, err = NewDocumentRepository(pool, catalogWith(11))
	if err != nil {
		t.Fatal(err)
	}
	read, err := repository.Get(ctx, record.Reference)
	if err != nil || read.Number != "" {
		t.Fatalf("a document that had no number reads %+v, %v", read, err)
	}
}
