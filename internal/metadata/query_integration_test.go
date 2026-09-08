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
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestBasicQueryLanguageIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	projectID, catalogID, priceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	documentID, accumulationID, warehouseID, quantityID, recorderID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
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
	catalog := &Catalog{
		Catalogs: []CatalogDefinition{{
			ID: catalogID, Name: "Товары", Code: CatalogCode{Type: StringType, Length: 20, Unique: true}, DescriptionLength: 100,
			Attributes: []Attribute{{ID: priceID, Name: "Цена", Types: []Type{{Kind: NumberType, Precision: 15, Scale: 2}}}},
		}},
		Documents: []DocumentDefinition{{
			ID: documentID, Name: "Продажа", Number: DocumentNumber{Type: StringType, Length: 20},
		}},
		AccumulationRegisters: []AccumulationRegisterDefinition{{
			ID: accumulationID, Name: "ТоварыНаСкладах", Kind: AccumulationRegisterBalance,
			Dimensions: []Attribute{{ID: warehouseID, Name: "Склад", Types: []Type{{Kind: StringType, Length: 50}}}},
			Resources:  []Attribute{{ID: quantityID, Name: "Количество", Types: []Type{{Kind: NumberType, Precision: 15, Scale: 3}}}},
			Recorders:  []uuid.UUID{documentID},
		}},
		catalogByName: map[string]int{"товары": 0}, catalogByID: map[uuid.UUID]int{catalogID: 0},
		documentByName: map[string]int{"продажа": 0}, documentByID: map[uuid.UUID]int{documentID: 0},
		accumulationRegisterByName: map[string]int{"товарынаскладах": 0}, accumulationRegisterByID: map[uuid.UUID]int{accumulationID: 0},
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
		ProjectID: projectID, PackageSHA256: repeatCatalogHex('7', 64), GitCommit: repeatCatalogHex('8', 40), Desired: desired,
		ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	movementTable, _ := PhysicalAccumulationRegisterTable(accumulationID)
	warehouseColumn, _ := PhysicalAttributeColumn(warehouseID)
	quantityColumn, _ := PhysicalAttributeColumn(quantityID)
	insertMovement := "INSERT INTO " + qualifiedCatalogTable(movementTable) + " (record_id, dimension_key, period, recorder_type, recorder_ref, line_no, active, totals_split, movement_kind, " +
		pgx.Identifier{warehouseColumn}.Sanitize() + ", " + pgx.Identifier{quantityColumn}.Sanitize() + ") VALUES ($1, $2, $3, $4, $5, 1, true, 0, 1, $6, $7)"
	if _, err := pool.Exec(ctx, insertMovement, uuid.MustNew().String(), strings.Repeat("a", 64), time.Now().UTC(), documentID.String(), recorderID.String(), "Основной", "5.500"); err != nil {
		t.Fatal(err)
	}
	repository, err := NewCatalogRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		code, description, price string
		deleted                  bool
	}{{"K2", "Бета", "2", false}, {"K1", "Альфа", "10", false}, {"K3", "Удалён", "30", true}} {
		record, err := repository.New(ctx, "Товары", nil)
		if err != nil {
			t.Fatal(err)
		}
		record.Code, record.Description, record.DeletionMark = item.code, item.description, item.deleted
		record.Attributes[priceID] = Value{Kind: NumberType, Data: item.price}
		if err := repository.Save(ctx, record, nil); err != nil {
			t.Fatal(err)
		}
	}
	runtime, err := NewRuntimeWithCatalogs(nil, repository, catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	movements, err := runtime.executeQuery(ctx, `ВЫБРАТЬ Регистратор, ВидДвижения, Количество ИЗ РегистрНакопления.ТоварыНаСкладах ГДЕ ВидДвижения = &Вид`, map[string]bytecode.Value{"вид": bytecode.String("receipt")})
	if err != nil || len(movements.rows) != 1 {
		t.Fatalf("movement query rows=%d error=%v", len(movements.rows), err)
	}
	recorderObject, ok := movements.rows[0][0].AsRuntimeObject()
	recorder, valid := recorderObject.(*documentReferenceObject)
	movement, _ := movements.rows[0][1].AsString()
	quantity, _ := movements.rows[0][2].NumberText()
	if !ok || !valid || recorder.reference != (DocumentReference{DocumentID: documentID, ObjectID: recorderID}) || movement != "receipt" || quantity != "5.5" {
		t.Fatalf("decoded movement=%v, %v, %s, %s", recorderObject, valid, movement, quantity)
	}

	program, diagnostics := compiler.CompileSource("query.bsl", `&НаСервере
Функция Проверить()
    Коды = Новый Массив;
    Коды.Добавить("K1");
    Коды.Добавить("K2");
    Запрос = Новый Запрос("ВЫБРАТЬ Товары.Ссылка, Товары.Наименование КАК Имя, Товары.Цена ИЗ Справочник.Товары КАК Товары ГДЕ НЕ Товары.ПометкаУдаления И Товары.Код В (&Коды) УПОРЯДОЧИТЬ ПО Имя");
    Запрос.УстановитьПараметр("Коды", Коды);
    Результат = Запрос.Выполнить();
    Если Результат.Пустой() Тогда
        Возврат "Пусто";
    КонецЕсли;
    Выборка = Результат.Выбрать();
    Итог = "";
    Пока Выборка.Следующий() Цикл
        Если Выборка.Ссылка.Пустая() Тогда
            Возврат "Пустая ссылка";
        КонецЕсли;
		Если Выборка.Имя = "Альфа" И Выборка.Цена <> 10 Тогда
			Возврат "Неверная цена Альфа";
		КонецЕсли;
		Если Выборка.Имя = "Бета" И Выборка.Цена <> 2 Тогда
			Возврат "Неверная цена Бета";
		КонецЕсли;
		Итог = Итог + Выборка.Имя + ";";
    КонецЦикла;
    Таблица = Результат.Выгрузить();
	Если Таблица.Количество() <> 2 Тогда
		Возврат "Неверное количество";
	КонецЕсли;
	Возврат Итог + ":" + Таблица[0].Имя;
КонецФункции`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	result, err := machine.NewContextWithMetadata(runtime).CallContext(ctx, "Проверить")
	if err != nil || result.String() != "Альфа;Бета;:Альфа" {
		t.Fatalf("query BSL result=%v error=%v", result, err)
	}

	for _, text := range []string{
		`ВЫБРАТЬ Код, Цена КАК Стоимость ИЗ Справочник.Товары ГДЕ НЕ ПометкаУдаления УПОРЯДОЧИТЬ ПО Стоимость`,
		`ВЫБРАТЬ Код ИЗ Справочник.Товары ГДЕ НЕ ПометкаУдаления УПОРЯДОЧИТЬ ПО Цена`,
	} {
		ordered, err := runtime.executeQuery(ctx, text, nil)
		if err != nil || len(ordered.rows) != 2 {
			t.Fatalf("numeric order rows=%d error=%v", len(ordered.rows), err)
		}
		first, _ := ordered.rows[0][0].AsString()
		second, _ := ordered.rows[1][0].AsString()
		if first != "K2" || second != "K1" {
			t.Fatalf("numeric order=%q, %q, want K2, K1", first, second)
		}
	}

	if _, err := runtime.executeQuery(ctx, `ВЫБРАТЬ Код ИЗ Справочник.Товары ГДЕ Код = &Код`, map[string]bytecode.Value{}); err == nil || !strings.Contains(err.Error(), "не установлен параметр") {
		t.Fatalf("missing parameter error=%v", err)
	}
	injection := `K1' OR TRUE`
	empty, err := runtime.executeQuery(ctx, `ВЫБРАТЬ Код ИЗ Справочник.Товары ГДЕ Код = &Код`, map[string]bytecode.Value{"код": bytecode.String(injection)})
	if err != nil || len(empty.rows) != 0 {
		t.Fatalf("bound injection result=%+v error=%v", empty, err)
	}
	if _, found, err := repository.FindByCode(ctx, "Товары", "K1"); err != nil || !found {
		t.Fatalf("catalog after bound injection found=%v error=%v", found, err)
	}

	execution, finish, err := runtime.BeginExecution(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.BeginTransaction(execution); err != nil {
		t.Fatal(err)
	}
	uncommitted, err := repository.New(execution, "Товары", nil)
	if err != nil {
		t.Fatal(err)
	}
	uncommitted.Code, uncommitted.Description = "TX", "В транзакции"
	uncommitted.Attributes[priceID] = Value{Kind: NumberType, Data: "1"}
	if err := repository.Save(execution, uncommitted, nil); err != nil {
		t.Fatal(err)
	}
	visible, err := runtime.executeQuery(execution, `ВЫБРАТЬ Код ИЗ Справочник.Товары ГДЕ Код = &Код`, map[string]bytecode.Value{"код": bytecode.String("TX")})
	if err != nil || len(visible.rows) != 1 {
		t.Fatalf("transactional query rows=%d error=%v", len(visible.rows), err)
	}
	if err := runtime.RollbackTransaction(execution); err != nil {
		t.Fatal(err)
	}
	if err := finish(nil); err != nil {
		t.Fatal(err)
	}
	if _, found, err := repository.FindByCode(ctx, "Товары", "TX"); err != nil || found {
		t.Fatalf("rolled-back query record found=%v error=%v", found, err)
	}

	if _, err := runtime.executeQuery(ctx, `ВЫБРАТЬ Неизвестное ИЗ Справочник.Товары`, nil); err == nil {
		t.Fatal("unknown logical field was accepted")
	}
	if err := ctx.Err(); err != nil {
		t.Fatal(err)
	}
}
