package metadata

import (
	"strings"
	"testing"
)

const presentationID = "40000000-0000-4000-8000-000000000071"

// The whole group, on the kind it is most obvious for. Eight properties that
// were read and dropped: a catalog called "Товар" in the singular and "Товары"
// in the plural came in with neither name, and a user saw the identifier.
func TestCatalogKeepsHowItIsNamedToAPerson(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, presentationID, `format: 1
id: `+presentationID+`
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
object_presentation: {ru: Товар}
extended_object_presentation: {ru: Товар или услуга}
list_presentation: {ru: Товары}
extended_list_presentation: {ru: Товары и услуги}
explanation: {ru: "Всё, что продаётся и покупается"}
comment: заведено при переносе
use_standard_commands: true
include_help_in_contents: true
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	definition, ok := catalog.CatalogDefinition("номенклатура")
	if !ok {
		t.Fatal("the catalog was not read")
	}
	if definition.ObjectPresentation["ru"] != "Товар" || definition.ExtendedObjectPresentation["ru"] != "Товар или услуга" ||
		definition.ListPresentation["ru"] != "Товары" || definition.ExtendedListPresentation["ru"] != "Товары и услуги" ||
		definition.Explanation["ru"] != "Всё, что продаётся и покупается" ||
		definition.Comment != "заведено при переносе" ||
		!definition.UseStandardCommands || !definition.IncludeHelpInContents {
		t.Fatalf("presentation = %+v", definition.Presentations)
	}
	// Five of the eight are localized texts, so a lookup hands out copies.
	definition.ListPresentation["ru"] = "Изменено"
	definition.Explanation["ru"] = "Изменено"
	again, _ := catalog.CatalogDefinition("Номенклатура")
	if again.ListPresentation["ru"] != "Товары" || again.Explanation["ru"] != "Всё, что продаётся и покупается" {
		t.Fatal("the catalog handed out its own presentation texts")
	}
}

// All eight kinds carry the whole set, and they reach it through two different
// shared shapes - five reference kinds through one, three numbered kinds through
// the other. A kind wired into only one of the two would look fine until someone
// filled in a name on the other.
func TestEveryObjectKindCarriesTheWholeSet(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]struct {
		kind    Kind
		id      string
		content string
	}{
		"справочник": {CatalogKind, presentationID, `code: {type: string, length: 9, auto: true}
description_length: 150
`},
		"план видов характеристик": {ChartOfCharacteristicTypesKind, presentationID, `code: {type: string, length: 9, auto: true}
description_length: 100
value_type: [{kind: string, length: 100}]
`},
		"план счетов": {ChartOfAccountsKind, presentationID, `code: {type: string, length: 5, auto: false}
description_length: 120
`},
		"план видов расчёта": {ChartOfCalculationTypesKind, presentationID, `code: {type: string, length: 9, auto: true}
description_length: 100
`},
		"план обмена": {ExchangePlanKind, presentationID, `code: {type: string, length: 36, auto: false}
description_length: 150
`},
		"документ": {DocumentKind, presentationID, `number: {type: string, length: 11, auto: true, periodicity: year}
`},
		"задача": {TaskKind, presentationID, `number: {type: string, length: 11, auto: true, periodicity: none}
description_length: 150
addressing_attributes: []
`},
		"бизнес-процесс": {BusinessProcessKind, presentationID, `number: {type: string, length: 11, auto: true, periodicity: year}
`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, body.kind, presentationID, `format: 1
id: `+presentationID+`
name: Объект
title: {ru: Объект}
`+body.content+`object_presentation: {ru: Один}
extended_object_presentation: {ru: Один подробно}
list_presentation: {ru: Многие}
extended_list_presentation: {ru: Многие подробно}
explanation: {ru: Пояснение}
comment: комментарий
use_standard_commands: true
include_help_in_contents: true
`)
			if _, err := Load(root); err != nil {
				t.Fatalf("a kind that carries the whole set refused part of it: %v", err)
			}
		})
	}
}

// The texts are read by a person, so they are localized, and a translation into
// a language the configuration does not have is a translation nobody will see.
// The check is the one every other title gets - the point is that these five
// texts get it too.
func TestPresentationTextsAreCheckedLikeAnyOtherTitle(t *testing.T) {
	t.Parallel()
	for name, field := range map[string]string{
		"представление объекта":             "object_presentation",
		"расширенное представление объекта": "extended_object_presentation",
		"представление списка":              "list_presentation",
		"расширенное представление списка":  "extended_list_presentation",
		"пояснение": "explanation",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, CatalogKind, presentationID, `format: 1
id: `+presentationID+`
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
`+field+`: {de: Ware}
`)
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), "unconfigured language") {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// Silence is the ordinary case and must stay legal: the prototype writes all
// eight properties on every object and fills a minority of them, so an empty
// presentation has to pass rather than fail the check that a title carries at
// least one translation.
func TestAnEmptyPresentationIsTheOrdinaryCase(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, presentationID, `format: 1
id: `+presentationID+`
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	definition, _ := catalog.CatalogDefinition("номенклатура")
	presentation := definition.Presentations
	if len(presentation.ObjectPresentation) != 0 || len(presentation.ExtendedObjectPresentation) != 0 ||
		len(presentation.ListPresentation) != 0 || len(presentation.ExtendedListPresentation) != 0 ||
		len(presentation.Explanation) != 0 || presentation.Comment != "" ||
		presentation.UseStandardCommands || presentation.IncludeHelpInContents {
		t.Fatalf("an unstated presentation came out as %+v", presentation)
	}
}
