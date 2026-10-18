package metadata

import (
	"path/filepath"
	"slices"
	"testing"
)

// Ловит перечень видов приложения, разошедшийся с тем, что приложение
// исполняет: навигация, адреса портала и поиск объекта по имени читают его
// одного (ApplicationKinds), и вид, отмеченный без исполнения, открылся бы в
// меню и упал на первом щелчке, а снятый — пропал бы из меню молча.
// Ожидание выписано здесь, а не взято из kindInfos: приложение открывает
// справочники и документы, остальное — блок 9.
func TestApplicationOpensCatalogsAndDocuments(t *testing.T) {
	if got := ApplicationKinds(); !slices.Equal(got, []Kind{CatalogKind, DocumentKind}) {
		t.Fatalf("ML App opens %v, want catalogs and documents", got)
	}
	for _, kind := range []Kind{InformationRegisterKind, AccumulationRegisterKind, ReportKind, EnumerationKind, Kind("unknown")} {
		if OpensInApplication(kind) {
			t.Errorf("ML App would open %s", kind)
		}
	}
}

// Ловит объект вида, не найденный по имени в другом регистре букв (портал
// получает имя из адреса), чужой вид, отданный за свой, и синоним, потерянный
// по дороге в навигацию.
func TestObjectHeadsNameObjectsOfOneKind(t *testing.T) {
	catalog, err := Load(filepath.Join("..", "..", "examples", "sales-and-warehouse"))
	if err != nil {
		t.Fatal(err)
	}
	goods, ok := catalog.CatalogDefinition("Товары")
	if !ok {
		t.Fatal("the example has no catalog Товары")
	}
	heads := catalog.ObjectHeads(CatalogKind)
	if len(heads) != len(catalog.Catalogs) || !slices.ContainsFunc(heads, func(head ObjectHead) bool {
		return head.ID == goods.ID && head.Name == "Товары" && head.Title["ru"] == goods.Title["ru"] && goods.Title["ru"] != ""
	}) {
		t.Fatalf("catalog heads %+v do not name Товары with its synonym", heads)
	}
	if head, ok := catalog.ObjectHead(CatalogKind, "товары"); !ok || head.ID != goods.ID {
		t.Fatalf("Товары by its name in lower case: %+v %v", head, ok)
	}
	if _, ok := catalog.ObjectHead(DocumentKind, "Товары"); ok {
		t.Fatal("a catalog was found among the documents")
	}
	if heads := catalog.ObjectHeads(Kind("unknown")); heads != nil {
		t.Fatalf("an unknown kind has heads %+v", heads)
	}
}
