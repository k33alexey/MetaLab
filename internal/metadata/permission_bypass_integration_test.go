package metadata

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// TestApplicationRoleBypassAttemptsIntegration exercises negative paths not
// covered by TestApplicationRoleEnforcementIntegration: posting versus plain
// write, table-part writes gated separately from the object itself, both
// register kinds written directly (not only through a document), and a query
// join across two sources where only one is readable.
func TestApplicationRoleBypassAttemptsIntegration(t *testing.T) {
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

	documentID, productAttrID, linesPartID, quantityAttrID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	accumulationID, accumulationProductID, accumulationQuantityID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	informationID, priceProductID, priceValueID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	fullRoleID, noPostRoleID, noLinesRoleID, noAccumulationRoleID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()

	document := DocumentDefinition{
		ID: documentID, Name: "Поступление", Posting: true,
		Number: DocumentNumber{Type: StringType, Length: 10, Unique: true, Periodicity: NumberPeriodNone},
		Attributes: []Attribute{
			{ID: productAttrID, Name: "Товар", Required: true, Types: []Type{{Kind: StringType, Length: 50}}},
		},
		TableParts: []TablePart{{ID: linesPartID, Name: "Строки", Attributes: []Attribute{
			{ID: quantityAttrID, Name: "Количество", Types: []Type{{Kind: NumberType, Precision: 15, Scale: 3}}},
		}}},
	}
	accumulation := AccumulationRegisterDefinition{
		ID: accumulationID, Name: "Остатки", Kind: AccumulationRegisterBalance, Recorders: []uuid.UUID{documentID},
		Dimensions: []Attribute{{ID: accumulationProductID, Name: "Товар", Required: true, Types: []Type{{Kind: StringType, Length: 50}}}},
		Resources:  []Attribute{{ID: accumulationQuantityID, Name: "Количество", Required: true, Types: []Type{{Kind: NumberType, Precision: 15, Scale: 3}}}},
	}
	information := InformationRegisterDefinition{
		ID: informationID, Name: "ПоследняяЦена", WriteMode: InformationRegisterIndependent, Periodicity: InformationRegisterPeriodNone,
		Dimensions: []Attribute{{ID: priceProductID, Name: "Товар", Required: true, Types: []Type{{Kind: StringType, Length: 50}}}},
		Resources:  []Attribute{{ID: priceValueID, Name: "Цена", Required: true, Types: []Type{{Kind: NumberType, Precision: 15, Scale: 2}}}},
	}
	documentFields := []FieldPermission{
		{Field: "number", Operations: []PermissionOperation{PermissionRead, PermissionUpdate}},
		{Field: "date", Operations: []PermissionOperation{PermissionRead, PermissionUpdate}},
		{Field: productAttrID.String(), Operations: []PermissionOperation{PermissionRead, PermissionUpdate}},
		{Field: linesPartID.String(), Operations: []PermissionOperation{PermissionRead, PermissionUpdate}},
		{Field: quantityAttrID.String(), Operations: []PermissionOperation{PermissionRead, PermissionUpdate}},
	}
	documentFieldsWithoutLines := []FieldPermission{
		{Field: "number", Operations: []PermissionOperation{PermissionRead, PermissionUpdate}},
		{Field: "date", Operations: []PermissionOperation{PermissionRead, PermissionUpdate}},
		{Field: productAttrID.String(), Operations: []PermissionOperation{PermissionRead, PermissionUpdate}},
		// Visible, but not writable: the table part itself carries no Update bit.
		{Field: linesPartID.String(), Operations: []PermissionOperation{PermissionRead}},
	}
	registerFields := []ObjectPermission{
		{Object: accumulationID, Operations: []PermissionOperation{PermissionRead, PermissionUpdate}, Fields: []FieldPermission{
			{Field: "period", Operations: []PermissionOperation{PermissionRead, PermissionUpdate}},
			{Field: "recorder", Operations: []PermissionOperation{PermissionRead, PermissionUpdate}},
			{Field: "active", Operations: []PermissionOperation{PermissionRead, PermissionUpdate}},
			{Field: "movementkind", Operations: []PermissionOperation{PermissionRead, PermissionUpdate}},
			{Field: accumulationProductID.String(), Operations: []PermissionOperation{PermissionRead, PermissionUpdate}},
			{Field: accumulationQuantityID.String(), Operations: []PermissionOperation{PermissionRead, PermissionUpdate}},
		}},
		{Object: informationID, Operations: []PermissionOperation{PermissionRead, PermissionUpdate}, Fields: []FieldPermission{
			{Field: priceProductID.String(), Operations: []PermissionOperation{PermissionRead, PermissionUpdate}},
			{Field: priceValueID.String(), Operations: []PermissionOperation{PermissionRead, PermissionUpdate}},
		}},
	}
	fullRole := RoleDefinition{
		ID: fullRoleID, Name: "Полные", Title: LocalizedText{"ru": "Полные права"},
		Objects: append([]ObjectPermission{{
			Object: documentID, Operations: []PermissionOperation{PermissionRead, PermissionCreate, PermissionUpdate, PermissionPost, PermissionUndoPosting},
			Fields: documentFields,
		}}, registerFields...),
	}
	noPostRole := RoleDefinition{
		ID: noPostRoleID, Name: "БезПроведения", Title: LocalizedText{"ru": "Без проведения"},
		Objects: []ObjectPermission{{
			Object: documentID, Operations: []PermissionOperation{PermissionRead, PermissionCreate, PermissionUpdate}, Fields: documentFields,
		}},
	}
	noLinesRole := RoleDefinition{
		ID: noLinesRoleID, Name: "БезСтрок", Title: LocalizedText{"ru": "Без строк"},
		Objects: []ObjectPermission{{
			Object: documentID, Operations: []PermissionOperation{PermissionRead, PermissionCreate, PermissionUpdate}, Fields: documentFieldsWithoutLines,
		}},
	}
	noAccumulationRole := RoleDefinition{
		ID: noAccumulationRoleID, Name: "БезРегистров", Title: LocalizedText{"ru": "Без регистров"},
		Objects: []ObjectPermission{{Object: documentID, Operations: []PermissionOperation{PermissionRead}, Fields: []FieldPermission{
			{Field: "number", Operations: []PermissionOperation{PermissionRead}},
		}}},
	}
	catalog := &Catalog{
		Project:               project.Project{ID: projectID},
		Documents:             []DocumentDefinition{document},
		AccumulationRegisters: []AccumulationRegisterDefinition{accumulation},
		InformationRegisters:  []InformationRegisterDefinition{information},
		Roles:                 []RoleDefinition{fullRole, noPostRole, noLinesRole, noAccumulationRole},
		documentByName:        map[string]int{"поступление": 0}, documentByID: map[uuid.UUID]int{documentID: 0},
		accumulationRegisterByName: map[string]int{"остатки": 0}, accumulationRegisterByID: map[uuid.UUID]int{accumulationID: 0},
		informationRegisterByName:  map[string]int{"последняяцена": 0}, informationRegisterByID: map[uuid.UUID]int{informationID: 0},
		roleByName: map[string]int{"полные": 0, "безпроведения": 1, "безстрок": 2, "безрегистров": 3},
		roleByID:   map[uuid.UUID]int{fullRoleID: 0, noPostRoleID: 1, noLinesRoleID: 2, noAccumulationRoleID: 3},
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
		ProjectID: projectID, PackageSHA256: repeatCatalogHex('c', 64), GitCommit: repeatCatalogHex('d', 40), Desired: desired,
		ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}

	documents, err := NewDocumentRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	accumulations, err := NewAccumulationRegisterRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	informations, err := NewInformationRegisterRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewRuntimeWithAllRegisters(nil, nil, documents, informations, accumulations, catalog, nil)
	if err != nil {
		t.Fatal(err)
	}

	fullPolicy, err := CompilePermissions(catalog, []uuid.UUID{fullRoleID})
	if err != nil {
		t.Fatal(err)
	}
	noPostPolicy, err := CompilePermissions(catalog, []uuid.UUID{noPostRoleID})
	if err != nil {
		t.Fatal(err)
	}
	noLinesPolicy, err := CompilePermissions(catalog, []uuid.UUID{noLinesRoleID})
	if err != nil {
		t.Fatal(err)
	}
	noAccumulationPolicy, err := CompilePermissions(catalog, []uuid.UUID{noAccumulationRoleID})
	if err != nil {
		t.Fatal(err)
	}

	newDate := bytecode.Value{}
	newDate, err = bytecode.Date(time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}

	createDraftDocument := func(policyCtx context.Context, number string) bytecode.RuntimeObject {
		t.Helper()
		value, err := runtime.CreateDocumentObject(policyCtx, "Поступление")
		if err != nil {
			t.Fatalf("create document: %v", err)
		}
		object, _ := value.AsRuntimeObject()
		for name, assigned := range map[string]bytecode.Value{"Номер": bytecode.String(number), "Дата": newDate, "Товар": bytecode.String("Монитор")} {
			if err := runtime.SetObjectProperty(policyCtx, object, name, assigned); err != nil {
				t.Fatalf("set %s: %v", name, err)
			}
		}
		return object
	}

	// Table part write is gated separately from the document's own Update grant:
	// a role missing the table-part field must be denied even though it can
	// otherwise create and update the document.
	noLinesCtx := WithPermissions(ctx, noLinesPolicy)
	noLinesObject := createDraftDocument(noLinesCtx, "NO-LINES1")
	table, err := runtime.GetObjectProperty(noLinesCtx, noLinesObject, "Строки")
	if err != nil {
		t.Fatalf("read table part: %v", err)
	}
	row, err := bytecode.CollectionMethod(table, "Добавить", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := bytecode.SetCollectionProperty(row, "Количество", bytecode.Number(3)); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.CallObjectMethod(noLinesCtx, noLinesObject, "Записать", []bytecode.Value{bytecode.String("write")}); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("table part write without its field grant: %v", err)
	}

	// Post versus plain write are distinct object operations: Update alone
	// must not authorize posting.
	noPostCtx := WithPermissions(ctx, noPostPolicy)
	noPostObject := createDraftDocument(noPostCtx, "NO-POST01")
	if _, err := runtime.CallObjectMethod(noPostCtx, noPostObject, "Записать", []bytecode.Value{bytecode.String("write")}); err != nil {
		t.Fatalf("plain write should be allowed without Post rights: %v", err)
	}
	if _, err := runtime.CallObjectMethod(noPostCtx, noPostObject, "Провести", nil); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("posting without Post rights: %v", err)
	}
	if _, err := runtime.CallObjectMethod(noPostCtx, noPostObject, "ОтменитьПроведение", nil); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("undo-posting without UndoPosting rights: %v", err)
	}

	// A role with document rights only (no register rights) can create and
	// write a document, but cannot touch either register kind directly.
	noAccumulationCtx := WithPermissions(ctx, noAccumulationPolicy)
	if _, err := runtime.CreateAccumulationRegisterRecordSet(noAccumulationCtx, "Остатки"); err != nil {
		// Construction itself is in-memory and unchecked; only the DB touch is gated.
		t.Fatalf("in-memory record set construction should not require rights: %v", err)
	}
	accumulationSetValue, err := runtime.CreateAccumulationRegisterRecordSet(noAccumulationCtx, "Остатки")
	if err != nil {
		t.Fatal(err)
	}
	accumulationSet, _ := accumulationSetValue.AsRuntimeObject()
	if _, _, err := runtime.callAccumulationRegisterMethod(noAccumulationCtx, accumulationSet, "Прочитать", nil); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("read accumulation register without rights: %v", err)
	}
	if _, _, err := runtime.callAccumulationRegisterMethod(noAccumulationCtx, accumulationSet, "Записать", nil); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("write accumulation register without rights: %v", err)
	}
	informationSetValue, err := runtime.CreateInformationRegisterRecordSet(noAccumulationCtx, "ПоследняяЦена")
	if err != nil {
		t.Fatal(err)
	}
	informationSet, _ := informationSetValue.AsRuntimeObject()
	if _, _, err := runtime.callInformationRegisterMethod(noAccumulationCtx, informationSet, "Прочитать", nil); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("read information register without rights: %v", err)
	}
	if _, _, err := runtime.callInformationRegisterMethod(noAccumulationCtx, informationSet, "Записать", nil); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("write information register without rights: %v", err)
	}

	// Full role: same operations succeed end to end, including register writes.
	fullCtx := WithPermissions(ctx, fullPolicy)
	fullObject := createDraftDocument(fullCtx, "FULL00001")
	fullTable, err := runtime.GetObjectProperty(fullCtx, fullObject, "Строки")
	if err != nil {
		t.Fatal(err)
	}
	fullRow, err := bytecode.CollectionMethod(fullTable, "Добавить", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := bytecode.SetCollectionProperty(fullRow, "Количество", bytecode.Number(2)); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.CallObjectMethod(fullCtx, fullObject, "Провести", []bytecode.Value{bytecode.String("real-time")}); err != nil {
		t.Fatalf("posting with full rights: %v", err)
	}
	accumulationFullValue, err := runtime.CreateAccumulationRegisterRecordSet(fullCtx, "Остатки")
	if err != nil {
		t.Fatal(err)
	}
	accumulationFullSet, _ := accumulationFullValue.AsRuntimeObject()
	recorderValue, err := runtime.wrapDocumentReference(document, fullObject.(*documentObject).record.Reference)
	if err != nil {
		t.Fatal(err)
	}
	filterValue, _, err := runtime.getAccumulationRegisterProperty(fullCtx, accumulationFullSet, "Отбор")
	if err != nil {
		t.Fatal(err)
	}
	filterObject, _ := filterValue.AsRuntimeObject()
	recorderFilterValue, _, err := runtime.getAccumulationRegisterProperty(fullCtx, filterObject, "Регистратор")
	if err != nil {
		t.Fatal(err)
	}
	recorderFilterObject, _ := recorderFilterValue.AsRuntimeObject()
	if _, _, err := runtime.callAccumulationRegisterMethod(fullCtx, recorderFilterObject, "Установить", []bytecode.Value{recorderValue}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runtime.callAccumulationRegisterMethod(fullCtx, accumulationFullSet, "Прочитать", nil); err != nil {
		t.Fatalf("read accumulation register with full rights: %v", err)
	}

	// Query join across two sources: missing Read on the joined register
	// denies the whole query, not just the columns from that source.
	joinQuery := "ВЫБРАТЬ Поступление.Ссылка ИЗ Документ.Поступление КАК Поступление " +
		"ЛЕВОЕ СОЕДИНЕНИЕ РегистрНакопления.Остатки КАК Остатки ПО Остатки.Товар = Поступление.Товар"
	if _, err := runtime.executeQuery(noAccumulationCtx, joinQuery, nil); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("join query without register rights: %v", err)
	}
	if _, err := runtime.executeQuery(fullCtx, joinQuery, nil); err != nil {
		t.Fatalf("join query with full rights: %v", err)
	}
}
