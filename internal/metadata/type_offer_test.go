package metadata

import (
	"slices"
	"testing"
)

// offeredRows names the rows of an offer as the window draws them: the name,
// and a sign of what the row is - "□" a type with a box, "▸" a folder.
func offeredRows(offer TypeOffer) []string {
	var rows []string
	for _, row := range offer.Rows {
		sign := "□"
		if row.Kind == "" && !row.Arbitrary {
			sign = "▸"
		}
		rows = append(rows, sign+row.Name.Russian)
	}
	return rows
}

// The window offers in each place what the prototype's offers there, in its
// order, a set of a kind with a box where the prototype has one and a folder
// where it has none. Defect caught: a place offering another's list - the
// command parameter offering the primitives, the value of a characteristic
// offering the sets - or the groups in the order of the tree. The
// expectations are written out from the opis (docs/block-00/obshchee.md,
// «Состав по местам», snapshots configurator-2026-10-23/tip-*), not read
// from choicePlaces.
func TestTypeOfferIsThePrototypes(t *testing.T) {
	t.Parallel()
	references := []string{"СправочникСсылка", "ДокументСсылка", "ПеречислениеСсылка", "ПланВидовХарактеристикСсылка", "ПланСчетовСсылка",
		"ПланВидовРасчетаСсылка", "БизнесПроцессСсылка", "ТочкаМаршрутаБизнесПроцессаСсылка", "ЗадачаСсылка", "ПланОбменаСсылка"}
	marked := func(sign string, names []string) []string {
		var rows []string
		for _, name := range names {
			rows = append(rows, sign+name)
		}
		return rows
	}
	expected := map[TypeChoicePlace][]string{
		StoredFieldChoice: slices.Concat(marked("□", []string{"Число", "Строка", "Дата", "Булево", "ХранилищеЗначения", "УникальныйИдентификатор"}),
			marked("□", references), []string{"▸Характеристика", "▸ОпределяемыйТип", "□ЛюбаяСсылка"}),
		CommandParameterChoice:    slices.Concat(marked("▸", references), []string{"▸ОпределяемыйТип"}),
		CharacteristicValueChoice: slices.Concat(marked("□", []string{"Число", "Строка", "Дата", "Булево"}), marked("▸", references), []string{"▸ОпределяемыйТип"}),
		FilterCriterionChoice: slices.Concat(marked("□", []string{"Произвольный", "Число", "Строка", "Дата", "Булево", "УникальныйИдентификатор"}),
			marked("□", references), []string{"▸Характеристика", "▸ОпределяемыйТип", "□ЛюбаяСсылка"}),
	}
	// An attribute of a data processor, a report and a form: one list
	// (snapshots tip-59…62, redaktor-formy 53 and 58). The document PDF of
	// the window waits for question В2 of the series 2.531.
	memory := slices.Concat(
		marked("□", []string{"Произвольный", "Число", "Строка", "Дата", "Булево", "УникальныйИдентификатор", "ДинамическийСписок",
			"ТаблицаЗначений", "ДеревоЗначений", "СписокЗначений", "ФорматированнаяСтрока", "ТабличныйДокумент",
			"ФорматированныйДокумент", "ТекстовыйДокумент", "Null"}),
		marked("□", references), []string{"▸Характеристика", "▸ОпределяемыйТип", "□ЛюбаяСсылка"},
		marked("▸", []string{"СправочникОбъект", "ДокументОбъект", "ПланВидовХарактеристикОбъект", "ПланСчетовОбъект",
			"ПланВидовРасчетаОбъект", "ОтчетОбъект", "ОбработкаОбъект", "БизнесПроцессОбъект", "ЗадачаОбъект", "ПланОбменаОбъект",
			"РегистрСведенийНаборЗаписей", "РегистрСведенийМенеджерЗаписи", "РегистрНакопленияНаборЗаписей",
			"РегистрБухгалтерииНаборЗаписей", "РегистрРасчетаНаборЗаписей"}),
		marked("□", []string{"КонстантыНабор", "ОтчетОбъект", "КомпоновщикНастроекКомпоновкиДанных", "ГрафическаяСхема",
			"ГеографическаяСхема", "Диаграмма", "ДиаграммаГанта", "Дендрограмма", "Планировщик", "ОписаниеТипов", "Картинка", "Цвет",
			"Шрифт", "СтандартнаяДатаНачала", "СтандартныйПериод"}),
		[]string{"▸Системные перечисления"})
	expected[RunningObjectChoice] = memory
	expected[FormAttributeChoice] = memory
	required := map[TypeChoicePlace]bool{StoredFieldChoice: true, CharacteristicValueChoice: true}
	for place, rows := range expected {
		offer, ok := (&Catalog{}).TypeOffer(place)
		if !ok {
			t.Errorf("%s offers nothing", place)
			continue
		}
		if got := offeredRows(offer); !slices.Equal(got, rows) {
			t.Errorf("%s offers\n%v\nthe prototype offers\n%v", place, got, rows)
		}
		if offer.Required != required[place] {
			t.Errorf("%s: a type required %v, the prototype %v", place, offer.Required, required[place])
		}
		// The folder of the enumerations of the platform holds the 24 of
		// the window, each a type with a box (TestTheSystemEnumerationsAreTheWindows
		// holds their names and order).
		if last := offer.Rows[len(offer.Rows)-1]; slices.Contains(rows, "▸Системные перечисления") &&
			(len(last.Types) != len(systemEnumerationsOfTheWindow) || slices.ContainsFunc(last.Types, func(row TypeOfferRow) bool { return row.Kind == "" })) {
			t.Errorf("%s: the folder of the enumerations of the platform holds %+v", place, last.Types)
		}
	}
	if len(choicePlaces) != len(expected) {
		t.Errorf("%d places offer types, %d are checked here", len(choicePlaces), len(expected))
	}
	if _, ok := (&Catalog{}).TypeOffer("nowhere"); ok {
		t.Error("a place nobody knows offers types")
	}
}

// Every type a place offers, built as the window builds it - a type marked
// anew with the qualifiers of newTypes, a type naming each object a row names
// - is accepted by the model where the place stands. Defect caught: the window
// offering what the save then refuses, as a set in a place whose model takes
// none or a string of no length. The objects are those of the composite
// project, which holds every kind.
func TestEveryOfferedTypeIsAccepted(t *testing.T) {
	t.Parallel()
	catalog, err := Load(compositeProject(t))
	if err != nil {
		t.Fatal(err)
	}
	for place, at := range choicePlaces {
		offer, _ := catalog.TypeOffer(place)
		named := 0
		for _, row := range offer.Rows {
			var types []Type
			if row.Kind != "" {
				types = append(types, offer.newType(row.Kind))
			}
			for _, object := range row.Objects {
				id := object.ID
				types = append(types, Type{Kind: row.ObjectKind, Reference: &id})
				named++
			}
			for _, inside := range row.Types {
				types = append(types, offer.newType(inside.Kind))
			}
			for _, item := range types {
				if issues := validateTypesIn("types", []Type{item}, at.model); len(issues) != 0 {
					t.Errorf("%s offers %+v, the model refuses it: %v", place, item, issues)
				}
			}
		}
		if named == 0 {
			t.Errorf("%s names no object of the composite project", place)
		}
	}
}

// newType is a type of the kind as the window marks it anew.
func (offer TypeOffer) newType(kind TypeKind) Type {
	if item, ok := offer.NewTypes[kind]; ok {
		return item
	}
	return Type{Kind: kind}
}
