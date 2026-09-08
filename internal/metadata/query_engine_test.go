package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/querylang"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestBasicQueryCompilationUsesLogicalMetadataAndBoundValues(t *testing.T) {
	catalogID := uuid.MustNew()
	priceID := uuid.MustNew()
	catalog := &Catalog{
		Catalogs: []CatalogDefinition{{
			ID: catalogID, Name: "Товары", Code: CatalogCode{Type: StringType, Length: 20}, DescriptionLength: 100,
			Attributes: []Attribute{{ID: priceID, Name: "Цена", Types: []Type{{Kind: NumberType, Precision: 15, Scale: 2}}}},
		}},
		catalogByName: map[string]int{"товары": 0}, catalogByID: map[uuid.UUID]int{catalogID: 0},
	}
	runtime := &Runtime{catalog: catalog}
	parsed, err := querylang.Parse(`ВЫБРАТЬ Т.Ссылка, Т.Наименование КАК Имя, Т.Цена
ИЗ Справочник.Товары КАК Т
ГДЕ НЕ Т.ПометкаУдаления И Т.Код = &Код И Т.Цена >= &Цена
УПОРЯДОЧИТЬ ПО Имя`)
	if err != nil {
		t.Fatal(err)
	}
	source, err := runtime.resolveQuerySource(parsed.Source)
	if err != nil {
		t.Fatal(err)
	}
	compiler := queryCompiler{
		runtime: runtime, source: source,
		parameters: map[string]bytecode.Value{"код": bytecode.String(`K1' OR TRUE`), "цена": bytecode.Number(10)},
		aliases:    map[string]int{},
	}
	statement, err := compiler.compile(parsed)
	if err != nil {
		t.Fatal(err)
	}
	physical, _ := PhysicalCatalogTable(catalogID)
	if !strings.Contains(statement, physical) || strings.Contains(statement, "Справочник.Товары") || strings.Contains(statement, "OR TRUE") {
		t.Fatalf("unsafe or incorrect SQL: %s", statement)
	}
	if len(compiler.arguments) != 2 || compiler.arguments[0] != `K1' OR TRUE` || compiler.arguments[1] != "10" {
		t.Fatalf("arguments=%#v", compiler.arguments)
	}
	if len(compiler.outputs) != 3 || compiler.outputs[1].name != "Имя" {
		t.Fatalf("outputs=%+v", compiler.outputs)
	}
}

func TestBasicQuerySourcesExposeSupportedMetadataFields(t *testing.T) {
	catalogID, documentID, informationID, accumulationID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	dimensionID, resourceID := uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{
		Catalogs:              []CatalogDefinition{{ID: catalogID, Name: "Товары", Code: CatalogCode{Type: StringType, Length: 20}, DescriptionLength: 100}},
		Documents:             []DocumentDefinition{{ID: documentID, Name: "Продажа", Number: DocumentNumber{Type: StringType, Length: 20}}},
		InformationRegisters:  []InformationRegisterDefinition{{ID: informationID, Name: "Цены", Periodicity: InformationRegisterPeriodDay, Dimensions: []Attribute{{ID: dimensionID, Name: "Товар", Types: []Type{referenceType(CatalogType, catalogID)}}}}},
		AccumulationRegisters: []AccumulationRegisterDefinition{{ID: accumulationID, Name: "Остатки", Kind: AccumulationRegisterBalance, Dimensions: []Attribute{{ID: dimensionID, Name: "Товар", Types: []Type{referenceType(CatalogType, catalogID)}}}, Resources: []Attribute{{ID: resourceID, Name: "Количество", Types: []Type{{Kind: NumberType, Precision: 15, Scale: 3}}}}}},
		catalogByName:         map[string]int{"товары": 0}, catalogByID: map[uuid.UUID]int{catalogID: 0},
		documentByName: map[string]int{"продажа": 0}, documentByID: map[uuid.UUID]int{documentID: 0},
		informationRegisterByName: map[string]int{"цены": 0}, informationRegisterByID: map[uuid.UUID]int{informationID: 0},
		accumulationRegisterByName: map[string]int{"остатки": 0}, accumulationRegisterByID: map[uuid.UUID]int{accumulationID: 0},
	}
	runtime := &Runtime{catalog: catalog}
	for _, test := range []struct {
		path   []string
		fields []string
	}{
		{[]string{"Справочник", "Товары"}, []string{"Ссылка", "Код", "Наименование"}},
		{[]string{"Документ", "Продажа"}, []string{"Ссылка", "Номер", "Дата", "Проведен"}},
		{[]string{"РегистрСведений", "Цены"}, []string{"ИдентификаторЗаписи", "Период", "Товар"}},
		{[]string{"РегистрНакопления", "Остатки"}, []string{"ИдентификаторЗаписи", "Период", "Регистратор", "ВидДвижения", "Количество"}},
	} {
		source, err := runtime.resolveQuerySource(querylang.Source{Path: test.path, Position: querylang.Position{Line: 1, Column: 1}})
		if err != nil {
			t.Fatalf("source %v: %v", test.path, err)
		}
		for _, field := range test.fields {
			if _, ok := source.byName[strings.ToLower(field)]; !ok {
				t.Errorf("source %v is missing %s", test.path, field)
			}
		}
	}
}

func TestQueryRuntimeObjectValidation(t *testing.T) {
	runtime := &Runtime{}
	value, err := runtime.constructQuery([]bytecode.Value{bytecode.String("ВЫБРАТЬ 1 ИЗ Справочник.Товары")})
	if err != nil {
		t.Fatal(err)
	}
	object, _ := value.AsRuntimeObject()
	if handled, err := runtime.setQueryProperty(object, "Текст", bytecode.String("")); !handled || err == nil {
		t.Fatalf("empty text handled=%v error=%v", handled, err)
	}
	if _, handled, err := runtime.callQueryMethod(nil, object, "УстановитьПараметр", []bytecode.Value{bytecode.String("bad-name"), bytecode.String("x")}); !handled || err == nil {
		t.Fatalf("invalid parameter handled=%v error=%v", handled, err)
	}
}

func TestQueryResultMemoryLimitIncludesRowOverhead(t *testing.T) {
	result := &queryResultObject{rows: [][]bytecode.Value{{bytecode.Undefined()}}}
	if memory, ok := result.RuntimeDynamicMemory(256); ok || memory != 256 {
		t.Fatalf("RuntimeDynamicMemory() = %d, %v; want 256, false", memory, ok)
	}
}
