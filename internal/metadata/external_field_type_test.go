package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A field of a table of a source takes one type and no reference to an object
// of the base (opis of 15.10.2026, vneshnij-istochnik.md): the editor of its
// type has no «Составной тип данных», a composite defined type «Проверка
// конфигурации» refuses - «Поле не может иметь составной тип» - and a direct
// reference to a catalog the editor does not offer, while a defined type of
// one reference to a catalog it takes. A function of a source takes a
// composite type (funkciya-istochnika.md). The texts are written out here.
//
// Defect caught: a field of a string and a number, of a composite defined
// type, or of a reference to a catalog loaded; a defined type of one
// reference refused; the rule put into the check the function shares.
func TestAFieldOfASourceTakesOneTypeAndNoReferenceToTheBase(t *testing.T) {
	t.Parallel()
	goods, single, composite := uuid.MustNew().String(), uuid.MustNew().String(), uuid.MustNew().String()
	project := func(t *testing.T, fieldTypes string) string {
		t.Helper()
		root := metadataProject(t)
		catalogNamed(t, root, goods, "Товары")
		writeMetadata(t, root, DefinedTypeKind, single, "format: 1\nid: "+single+"\nname: Товар\ntitle: {ru: Товар}\ntypes: [{kind: catalog, reference: "+goods+"}]\n")
		writeMetadata(t, root, DefinedTypeKind, composite, "format: 1\nid: "+composite+"\nname: Адресат\ntitle: {ru: Адресат}\n"+
			"types: [{kind: catalog, reference: "+goods+"}, {kind: string, length: 10}]\n")
		writeExternalSource(t, root, warehouseSource, "Склад", "functions:\n  - {id: "+uuid.MustNew().String()+
			", name: Остаток, title: {ru: Остаток}, return_value: true, types: [{kind: string, length: 10}, {kind: number, precision: 15}], expression_in_data_source: f()}\n")
		writeExternalTable(t, root, "Склад", stockTable, "Остатки", "name_in_data_source: dbo.Stock\ndata_type: non-object\nfields:\n"+
			"  - {id: "+stockGoodsField+", name: Поле, title: {ru: Поле}, name_in_data_source: f, types: "+fieldTypes+"}\n")
		return root
	}
	for name, test := range map[string]struct{ types, want string }{
		"строка и число":         {"[{kind: string, length: 10}, {kind: number, precision: 15}]", "field Поле has a composite type: the designer refuses a field of a source of more than one type"},
		"составной определяемый": {"[{kind: defined-type, reference: " + composite + "}]", "field Поле has a composite type"},
		"ссылка на справочник":   {"[{kind: catalog, reference: " + goods + "}]", "field Поле is typed by a reference to an object of the base, catalog"},
	} {
		if message := loadRefused(t, project(t, test.types), name); !strings.Contains(message, test.want) {
			t.Errorf("%s refused for another reason: %v", name, message)
		}
	}
	for name, types := range map[string]string{
		"определяемый из одной ссылки": "[{kind: defined-type, reference: " + single + "}]",
		"строка": "[{kind: string, length: 10}]",
	} {
		if _, err := Load(project(t, types)); err != nil {
			t.Errorf("%s was refused: %v", name, err)
		}
	}
}
