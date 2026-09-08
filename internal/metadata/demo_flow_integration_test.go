package metadata

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestSalesAndWarehouseVerticalFlowIntegration(t *testing.T) {
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
	root := filepath.Join("..", "..", "examples", "sales-and-warehouse")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	projectID := catalog.Project.ID
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

	catalogs, err := NewCatalogRepository(pool, catalog)
	if err != nil {
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
	runtime, err := NewRuntimeWithAllRegisters(nil, catalogs, documents, nil, registers, catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, definition := range catalog.Documents {
		modulePath := filepath.Join(root, "modules", definition.ObjectModule.String()+".bsl")
		source, err := os.ReadFile(modulePath)
		if err != nil {
			t.Fatal(err)
		}
		program, diagnostics := CompileDocumentObjectModule(definition, modulePath, string(source))
		if len(diagnostics) != 0 {
			t.Fatalf("%s: %v", definition.Name, diagnostics)
		}
		machine, err := vm.New(program)
		if err != nil {
			t.Fatal(err)
		}
		handler, err := NewDocumentBSLEvents(runtime, machine.NewContextWithMetadata(runtime), definition)
		if err != nil {
			t.Fatal(err)
		}
		if err := runtime.SetDocumentEventHandler(definition.Name, handler); err != nil {
			t.Fatal(err)
		}
	}

	product, err := catalogs.New(ctx, "Товары", nil)
	if err != nil {
		t.Fatal(err)
	}
	product.Description = "Ноутбук"
	if err := catalogs.Save(ctx, product, nil); err != nil {
		t.Fatal(err)
	}
	counterparty, err := catalogs.New(ctx, "Контрагенты", nil)
	if err != nil {
		t.Fatal(err)
	}
	counterparty.Description = "Покупатель"
	if err := catalogs.Save(ctx, counterparty, nil); err != nil {
		t.Fatal(err)
	}
	productDefinition, _ := catalog.CatalogDefinition("Товары")
	counterpartyDefinition, _ := catalog.CatalogDefinition("Контрагенты")
	productValue, _ := runtime.wrapCatalogReference(productDefinition, product.Reference)
	counterpartyValue, _ := runtime.wrapCatalogReference(counterpartyDefinition, counterparty.Reference)
	date := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	dateValue, _ := bytecode.Date(date)

	post := func(documentName string, quantity, price float64) DocumentReference {
		t.Helper()
		value, err := runtime.CreateDocumentObject(ctx, documentName)
		if err != nil {
			t.Fatal(err)
		}
		object, _ := value.AsRuntimeObject()
		for name, assigned := range map[string]bytecode.Value{"Дата": dateValue, "Контрагент": counterpartyValue} {
			if err := runtime.SetObjectProperty(ctx, object, name, assigned); err != nil {
				t.Fatal(err)
			}
		}
		table, err := runtime.GetObjectProperty(ctx, object, "Товары")
		if err != nil {
			t.Fatal(err)
		}
		row, err := bytecode.CollectionMethod(table, "Добавить", nil)
		if err != nil {
			t.Fatal(err)
		}
		for name, assigned := range map[string]bytecode.Value{"Товар": productValue, "Количество": bytecode.Number(quantity), "Цена": bytecode.Number(price)} {
			if err := bytecode.SetCollectionProperty(row, name, assigned); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := runtime.CallObjectMethod(ctx, object, "Провести", nil); err != nil {
			t.Fatal(err)
		}
		return object.(*documentObject).record.Reference
	}
	receipt := post("ПоступлениеТоваров", 10, 12.5)
	sale := post("ПродажаТоваров", 3, 15)
	for _, reference := range []DocumentReference{receipt, sale} {
		record, err := documents.Get(ctx, reference)
		if err != nil || !record.Posted {
			t.Fatalf("posted document=%+v error=%v", record, err)
		}
	}

	register, _ := catalog.AccumulationRegisterDefinition("ОстаткиТоваров")
	productDimension := register.Dimensions[0].ID
	quantityResource, amountResource := register.Resources[0].ID, register.Resources[1].ID
	rows, err := registers.Balances(ctx, register.Name, date.Add(time.Hour), map[uuid.UUID]Value{
		productDimension: {Kind: CatalogType, Data: product.Reference.ObjectID.String()},
	})
	if err != nil || len(rows) != 1 || rows[0].Turnover[quantityResource].Data != "7" || rows[0].Turnover[amountResource].Data != "80" {
		t.Fatalf("demo balances=%+v error=%v", rows, err)
	}
	products, err := catalogs.List(ctx, "Товары", nil, 20)
	if err != nil || len(products.Records) != 1 || products.Records[0].Description != "Ноутбук" {
		t.Fatalf("products=%+v error=%v", products, err)
	}
	receipts, err := documents.List(ctx, "ПоступлениеТоваров", nil, 20)
	if err != nil || len(receipts.Records) != 1 || !receipts.Records[0].Posted {
		t.Fatalf("receipts=%+v error=%v", receipts, err)
	}
}
