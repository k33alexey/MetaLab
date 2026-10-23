package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A common attribute is not named as a standard attribute of an object its
// content takes, in either language, case aside: the designer refuses «Номер»
// on an attribute whose content takes a document (opis of 15.10.2026,
// obshchij-rekvizit.md). One standard attribute per kind of the content is
// written here from the help of each kind, not read from the table the model
// reads.
//
// Defect caught: «Код», «Наименование», «Ссылка», «ПометкаУдаления» on a
// catalog and the like loaded as a field beside the standard one of the same
// name; a name of another kind's standard attribute refused.
func TestACommonAttributeIsNotNamedAsAStandardAttribute(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		object uuid.UUID
		name   string
		where  string
	}{
		{contentFirst, "Код", "catalogs Товары"},
		{contentFirst, "description", "catalogs Товары"},
		{contentFirst, "Ссылка", "catalogs Товары"},
		{contentFirst, "ПОМЕТКАУДАЛЕНИЯ", "catalogs Товары"},
		{contentDocument, "Номер", "documents Продажа"},
		{contentDocument, "Posted", "documents Продажа"},
		{contentInformation, "Период", "information-registers Цены"},
		{contentAccumulation, "Регистратор", "accumulation-registers Остатки"},
		{contentProcess, "Стартован", "business-processes Согласование"},
		{contentTask, "Выполнена", "tasks ЗадачаИсполнителя"},
		{contentCharacteristics, "ТипЗначения", "charts-of-characteristic-types ВидыХарактеристик"},
		{contentAccounts, "Забалансовый", "charts-of-accounts Хозрасчетный"},
		{contentCalculationTypes, "ПериодДействияБазовый", "charts-of-calculation-types ОсновныеНачисления"},
		{contentExchange, "ЭтотУзел", "exchange-plans ОбменСФилиалами"},
		{contentAccounting, "Счет", "accounting-registers Проводки"},
		{contentCalculation, "ВидРасчета", "calculation-registers Начисления"},
	} {
		common := contentAttribute(CommonAttributeAutoUseDontUse, CommonAttributeContentItem{Metadata: test.object, Use: CommonAttributeUseUse})
		common.Name = test.name
		err := contentCatalog(common).propagateCommonAttributes()
		if err == nil || !strings.Contains(err.Error(), "common attribute "+test.name+" is named as a standard attribute of "+test.where) {
			t.Errorf("%s on %s: %v, want the refusal naming the object", test.name, test.where, err)
		}
	}
	// «Номер» is a standard attribute of a document, not of a catalog.
	common := contentAttribute(CommonAttributeAutoUseDontUse, CommonAttributeContentItem{Metadata: contentFirst, Use: CommonAttributeUseUse})
	common.Name = "Номер"
	if err := contentCatalog(common).propagateCommonAttributes(); err != nil {
		t.Fatalf("a name no standard attribute of a catalog has was refused: %v", err)
	}
}
