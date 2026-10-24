package metadata

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestDocumentRepositoryLifecycleIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := integrationDatabase(t, ctx, 0)
	projectID := uuid.MustNew()

	catalogID, documentID := uuid.MustNew(), uuid.MustNew()
	partnerID, linesID, productID, quantityID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{
		Catalogs: []CatalogDefinition{{
			ID: catalogID, Name: "Контрагенты", Code: CatalogCode{Type: StringType, Length: 9, Unique: true}, DescriptionLength: 250,
		}},
		Documents: []DocumentDefinition{{
			ID: documentID, Name: "Продажа", Posting: DocumentPosting{Allowed: true},
			Number:     DocumentNumber{Type: StringType, Length: 11, Unique: true, Periodicity: NumberPeriodYear},
			Attributes: []Attribute{{ID: partnerID, Name: "Контрагент", FillChecking: ShowFillingError, Types: []Type{{Kind: CatalogType, Reference: &catalogID}}}},
			TableParts: []TablePart{{ID: linesID, Name: "Товары", Attributes: []Attribute{
				{ID: productID, Name: "Товар", FillChecking: ShowFillingError, Types: []Type{{Kind: StringType, Length: 100}}},
				{ID: quantityID, Name: "Количество", FillChecking: ShowFillingError, Types: []Type{{Kind: NumberType, Precision: 15, Scale: 3}}},
			}}},
		}},
		catalogByName:  map[string]int{"контрагенты": 0},
		catalogByID:    map[uuid.UUID]int{catalogID: 0},
		documentByName: map[string]int{"продажа": 0},
		documentByID:   map[uuid.UUID]int{documentID: 0},
	}
	desired := migrateIntegrationSchema(t, ctx, pool, catalog, projectID)
	actual, err := schemadiff.Inspect(ctx, pool, schemadiff.ApplicationSchema)
	if err != nil {
		t.Fatal(err)
	}
	if plan, compareErr := schemadiff.Compare(desired, actual); compareErr != nil || len(plan.Changes) != 0 {
		t.Fatalf("post-migration plan=%+v error=%v", plan, compareErr)
	}

	catalogRepository, err := NewCatalogRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	partner, err := catalogRepository.New(ctx, "Контрагенты", nil)
	if err != nil {
		t.Fatal(err)
	}
	partner.Code, partner.Description = "C001", "Покупатель"
	if err := catalogRepository.Save(ctx, partner, nil); err != nil {
		t.Fatal(err)
	}

	repository, err := NewDocumentRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	fixedDate := time.Date(2026, 9, 6, 12, 30, 45, 123_400_000, time.UTC)
	repository.now = func() time.Time { return fixedDate }
	var events []DocumentEvent
	handler := DocumentEventHandlerFunc(func(_ context.Context, event DocumentEvent, record *DocumentRecord) (bool, error) {
		events = append(events, event)
		if event == DocumentEventFill {
			record.Number = "SALE-1"
			record.Attributes[partnerID] = Value{Kind: CatalogType, Data: partner.Reference.ObjectID.String(), Object: partner.Reference.CatalogID}
		}
		return false, nil
	})
	record, err := repository.New(ctx, "продажа", handler)
	if err != nil {
		t.Fatal(err)
	}
	record.TableParts[linesID] = []DocumentRow{{Values: map[uuid.UUID]Value{
		productID: {Kind: StringType, Data: "Ноутбук"}, quantityID: {Kind: NumberType, Data: "2.000"},
	}}}
	if err := repository.Save(ctx, record, handler); err != nil {
		t.Fatal(err)
	}
	page, err := repository.List(ctx, "Продажа", nil, 20)
	if err != nil || len(page.Records) != 1 || page.Records[0].Number != "SALE-1" || page.NextCursor != nil {
		t.Fatalf("document list page=%+v error=%v", page, err)
	}
	wantEvents := []DocumentEvent{DocumentEventFill, DocumentEventFillCheck, DocumentEventBefore, DocumentEventOnWrite}
	if !slices.Equal(events, wantEvents) || record.Version != 1 || !record.Date.Equal(fixedDate) {
		t.Fatalf("events=%v version=%d date=%s", events, record.Version, record.Date)
	}
	loaded, err := repository.Get(ctx, record.Reference)
	if err != nil || loaded.Number != "SALE-1" || loaded.Posted || loaded.Attributes[partnerID].Data != partner.Reference.ObjectID.String() ||
		len(loaded.TableParts[linesID]) != 1 || loaded.TableParts[linesID][0].Values[quantityID].Data != "2" {
		t.Fatalf("loaded=%+v error=%v", loaded, err)
	}
	found, ok, err := repository.FindByNumber(ctx, "Продажа", "SALE-1", &fixedDate)
	if err != nil || !ok || found != record.Reference {
		t.Fatalf("found=%+v ok=%v error=%v", found, ok, err)
	}
	if _, _, err := repository.FindByNumber(ctx, "Продажа", "SALE-1", nil); err == nil {
		t.Fatal("periodic FindByNumber accepted a missing date")
	}

	stale := cloneDocumentRecord(loaded)
	loaded.Number = "SALE-2"
	if err := repository.Save(ctx, loaded, nil); err != nil || loaded.Version != 2 {
		t.Fatalf("update version=%d error=%v", loaded.Version, err)
	}
	if err := repository.Save(ctx, stale, nil); !errors.Is(err, ErrDocumentWriteConflict) {
		t.Fatalf("stale write error=%v", err)
	}

	duplicate, err := repository.New(ctx, "Продажа", nil)
	if err != nil {
		t.Fatal(err)
	}
	duplicate.Number, duplicate.Date = "SALE-2", fixedDate
	duplicate.Attributes[partnerID] = Value{Kind: CatalogType, Data: partner.Reference.ObjectID.String(), Object: partner.Reference.CatalogID}
	if err := repository.Save(ctx, duplicate, nil); err == nil {
		t.Fatal("duplicate periodic number was accepted")
	}
	duplicate.Reference.ObjectID = uuid.MustNew()
	duplicate.Date = fixedDate.AddDate(1, 0, 0)
	if err := repository.Save(ctx, duplicate, nil); err != nil {
		t.Fatalf("same number in another period: %v", err)
	}
	dynamicPage, err := repository.ListDynamic(ctx, "Продажа", DynamicListRequest{
		Search: "SALE-2", SortField: "Date", Descending: true,
		Filters: []ListFilter{{Field: "Posted", Value: "false"}},
	})
	if err != nil || len(dynamicPage.Records) != 2 || dynamicPage.Records[0].Reference != duplicate.Reference || dynamicPage.NextCursor != nil {
		t.Fatalf("dynamic document list page=%+v error=%v", dynamicPage, err)
	}

	// A restriction of access on documents: conditions are not run yet, so the
	// role reaches no document by any way. FindByNumber matters most here - its
	// restriction lands after two existing placeholders, which is where
	// parameter numbering goes wrong if it is going to.
	role := RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Ограниченный", Title: LocalizedText{"ru": "Ограниченный"},
		Objects: []ObjectPermission{{Object: documentID, Operations: []PermissionOperation{PermissionRead},
			Restrictions: []RightRestriction{{Right: PermissionRead, Condition: "ГДЕ Ссылка = &Документ"}}}}}
	catalog.Project.ID = projectID
	catalog.Roles, catalog.roleByID = []RoleDefinition{role}, map[uuid.UUID]int{role.ID: 0}
	policy, err := CompilePermissions(catalog, []uuid.UUID{role.ID})
	if err != nil {
		t.Fatal(err)
	}
	restrictedCtx := WithPermissions(ctx, policy)
	if _, err := repository.Get(restrictedCtx, loaded.Reference); !errors.Is(err, ErrDocumentRecordNotFound) {
		t.Fatalf("restricted document must read as not found, got %v", err)
	}
	if reference, ok, err := repository.FindByNumber(restrictedCtx, "Продажа", "SALE-2", &fixedDate); err != nil || ok {
		t.Fatalf("restricted document was revealed by number lookup: %+v ok=%v error=%v", reference, ok, err)
	}
	if reference, ok, err := repository.FindByNumber(ctx, "Продажа", "SALE-2", &fixedDate); err != nil || !ok || reference != loaded.Reference {
		t.Fatalf("document must stay findable by number without permissions: %+v ok=%v error=%v", reference, ok, err)
	}
	restrictedList, err := repository.ListDynamic(restrictedCtx, "Продажа", DynamicListRequest{})
	if err != nil || len(restrictedList.Records) != 0 {
		t.Fatalf("restricted document list=%+v error=%v", restrictedList, err)
	}
	basicList, err := repository.List(restrictedCtx, "Продажа", nil, 20)
	if err != nil || len(basicList.Records) != 0 {
		t.Fatalf("restricted basic list=%+v error=%v", basicList, err)
	}

	runtime, err := NewRuntimeWithObjects(nil, catalogRepository, repository, catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	program, diagnostics := compiler.CompileSource("document.bsl", `&НаСервере
Функция Проверить()
    Документ = Документы.Продажа.СоздатьДокумент();
    Документ.Номер = "SALE-3";
    Документ.Дата = '20260907120000';
    Документ.Контрагент = Справочники.Контрагенты.НайтиПоКоду("C001");
    Строка = Документ.Товары.Добавить();
    Строка.Товар = "Монитор";
    Строка.Количество = 3;
    Документ.Записать();
    Ссылка = Документы.Продажа.НайтиПоНомеру("SALE-3", '20260907120000');
    Загружен = Ссылка.ПолучитьОбъект();
    Возврат Загружен.Номер + ":" + Загружен.Контрагент.ПолучитьОбъект().Наименование + ":" + Загружен.Товары[0].Товар;
КонецФункции`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	result, err := machine.NewContextWithMetadata(runtime).CallContext(ctx, "Проверить")
	if err != nil || result.String() != "SALE-3:Покупатель:Монитор" {
		t.Fatalf("BSL document result=%v error=%v", result, err)
	}
}
