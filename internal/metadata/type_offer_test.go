package metadata

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
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
	// (snapshots tip-59…62, redaktor-formy 53 and 58).
	memory := slices.Concat(
		marked("□", []string{"Произвольный", "Число", "Строка", "Дата", "Булево", "УникальныйИдентификатор", "ДинамическийСписок",
			"ТаблицаЗначений", "ДеревоЗначений", "СписокЗначений", "ФорматированнаяСтрока", "ТабличныйДокумент",
			"ФорматированныйДокумент", "ТекстовыйДокумент", "ДокументPDF", "Null"}),
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
	// A session parameter (snapshots parametr-seansa/04, 05).
	expected[SessionParameterChoice] = slices.Concat(
		marked("□", []string{"Число", "Строка", "Дата", "Булево", "ХранилищеЗначения", "УникальныйИдентификатор", "ДвоичныеДанные", "Null"}),
		marked("□", references), []string{"▸Характеристика", "▸ОпределяемыйТип", "□ЛюбаяСсылка"},
		marked("□", []string{"ОписаниеТипов", "ФиксированныйМассив", "ФиксированноеСоответствие", "ФиксированнаяСтруктура"}),
		[]string{"▸Системные перечисления"})
	// A defined type (opredelyaemyj-tip.md, «Досъёмка 24.10.2026», snapshots
	// series-2.531/01-08): references in folders without a box, the objects,
	// the record sets and the managers, one folder of the key and one of the
	// manager of the value of a constant (the prototype's pair of folders per
	// constant is not carried), the filter criteria, the values, the
	// enumerations, and last the manager of a settings storage with a box.
	expected[DefinedTypeChoice] = slices.Concat(
		marked("□", []string{"Произвольный", "Число", "Строка", "Дата", "Булево", "ХранилищеЗначения", "УникальныйИдентификатор",
			"ДинамическийСписок", "ТаблицаЗначений", "ДеревоЗначений", "СписокЗначений", "ФорматированнаяСтрока", "ДвоичныеДанные",
			"ТабличныйДокумент", "ФорматированныйДокумент", "ТекстовыйДокумент", "ДокументPDF", "Null"}),
		marked("▸", references),
		marked("▸", []string{"СправочникОбъект", "ДокументОбъект", "ПланВидовХарактеристикОбъект", "ПланСчетовОбъект",
			"ПланВидовРасчетаОбъект", "ОтчетОбъект", "ОбработкаОбъект", "БизнесПроцессОбъект", "ЗадачаОбъект", "ПланОбменаОбъект",
			"РегистрСведенийНаборЗаписей", "РегистрНакопленияНаборЗаписей", "РегистрБухгалтерииНаборЗаписей",
			"РегистрРасчетаНаборЗаписей", "СправочникМенеджер", "ДокументМенеджер", "ПеречислениеМенеджер",
			"ПланВидовХарактеристикМенеджер", "ПланСчетовМенеджер", "ПланВидовРасчетаМенеджер", "ОтчетМенеджер", "ОбработкаМенеджер",
			"БизнесПроцессМенеджер", "ЗадачаМенеджер", "ПланОбменаМенеджер", "ЖурналДокументовМенеджер", "РегистрСведенийМенеджер",
			"РегистрНакопленияМенеджер", "РегистрБухгалтерииМенеджер", "РегистрРасчетаМенеджер", "КонстантаКлючЗначения",
			"КонстантаМенеджерЗначения", "КритерийОтбораМенеджер"}),
		marked("□", []string{"КомпоновщикНастроекКомпоновкиДанных", "ГрафическаяСхема", "ГеографическаяСхема", "Диаграмма",
			"ДиаграммаГанта", "Дендрограмма", "ОписаниеТипов", "Картинка", "Цвет", "Шрифт", "СтандартнаяДатаНачала",
			"СтандартныйПериод", "Линия", "Рамка", "ФиксированныйМассив", "ФиксированноеСоответствие", "ФиксированнаяСтруктура",
			"ОбластьПодписиДиаграммы", "ШкалаДиаграммы", "ОсьДиаграммы", "ОписаниеПалитрыЦветовДиаграммы",
			"ИнформационнаяЛинияДиаграммы", "ИнформационныйИнтервалДиаграммы"}),
		[]string{"▸Системные перечисления", "□ХранилищеНастроекМенеджер"})
	// The source of a subscription (podpiska-na-sobytie.md, «Досъёмка
	// 24.10.2026», snapshots series-2.531/09, 10): the defined types, then a
	// row with a box of every kind it listens to.
	expected[SubscriptionSourceChoice] = slices.Concat([]string{"▸ОпределяемыйТип"},
		marked("□", []string{"СправочникОбъект", "ДокументОбъект", "ПланВидовХарактеристикОбъект", "ПланСчетовОбъект",
			"ПланВидовРасчетаОбъект", "БизнесПроцессОбъект", "ЗадачаОбъект", "ПланОбменаОбъект", "РегистрСведенийНаборЗаписей",
			"РегистрНакопленияНаборЗаписей", "РегистрБухгалтерииНаборЗаписей", "РегистрРасчетаНаборЗаписей",
			"ПерерасчетНаборЗаписей", "СправочникМенеджер", "ДокументМенеджер", "ПеречислениеМенеджер",
			"ПланВидовХарактеристикМенеджер", "ПланСчетовМенеджер", "ПланВидовРасчетаМенеджер", "ОтчетМенеджер", "ОбработкаМенеджер",
			"БизнесПроцессМенеджер", "ЗадачаМенеджер", "ПланОбменаМенеджер", "ЖурналДокументовМенеджер", "РегистрСведенийМенеджер",
			"РегистрНакопленияМенеджер", "РегистрБухгалтерииМенеджер", "РегистрРасчетаМенеджер", "КонстантаМенеджерЗначения",
			"КритерийОтбораМенеджер", "ПоследовательностьНаборЗаписей", "ХранилищеНастроекМенеджер"}))
	// A field of a table of a source and a function of one, in a project with
	// no source (snapshot series-2.531/16): the simple types and binary data,
	// the defined types, and no folder of the references, there being none.
	external := slices.Concat(marked("□", []string{"Число", "Строка", "Дата", "Булево", "УникальныйИдентификатор", "ДвоичныеДанные"}),
		[]string{"▸ОпределяемыйТип"})
	expected[ExternalFieldChoice] = external
	expected[ExternalFunctionChoice] = external
	// A dimension of a cube (snapshots 18, 20): the defined types and the
	// dimension tables, here none.
	expected[CubeDimensionChoice] = []string{"▸ОпределяемыйТип"}
	// A resource of a register (snapshots 11-15): the number and the defined
	// types.
	expected[NumericResourceChoice] = []string{"□Число", "▸ОпределяемыйТип"}
	// The folder of the enumerations of the platform by place (opis,
	// «Системные перечисления по местам»): 24 in an attribute of a data
	// processor, a form and a defined type, the three of accounting in a
	// session parameter.
	enumerations := map[TypeChoicePlace]int{RunningObjectChoice: 24, FormAttributeChoice: 24, DefinedTypeChoice: 24, SessionParameterChoice: 3}
	required := map[TypeChoicePlace]bool{StoredFieldChoice: true, CharacteristicValueChoice: true, SessionParameterChoice: true,
		ExternalFieldChoice: true, CubeDimensionChoice: true, NumericResourceChoice: true}
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
		// Only the field of a source and a dimension of a cube have no
		// «Составной тип данных» (snapshots 16-18, 20, 22; a function has it,
		// 19, 21).
		if offer.Single != (place == ExternalFieldChoice || place == CubeDimensionChoice) {
			t.Errorf("%s: without a composite %v", place, offer.Single)
		}
		// The folder holds as many enumerations as the window there, the
		// first of them first, each a type with a box
		// (TestTheSystemEnumerationsAreTheWindows holds their names and
		// order).
		var inside []TypeOfferRow
		if at := slices.Index(rows, "▸Системные перечисления"); at >= 0 {
			inside = offer.Rows[at].Types
		}
		if len(inside) != enumerations[place] || len(inside) > 0 && inside[0].Kind != "accumulation-record-kind" ||
			slices.ContainsFunc(inside, func(row TypeOfferRow) bool { return row.Kind == "" }) {
			t.Errorf("%s: the folder of the enumerations of the platform holds %+v", place, inside)
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
				issues := validateTypesIn("types", []Type{item}, at.model)
				if len(at.sourceReferences) > 0 {
					issues = validateExternalFieldTypes("types", []Type{item}, at.model)
				}
				if at.model == placeNumericResource {
					issues = validateNumericResource("resource", Attribute{Types: []Type{item}})
				}
				if at.model == placeSubscriptionSource {
					issues = validateEventSubscription(EventSubscriptionDefinition{Source: []Type{item}, Event: "before-write",
						Module: uuid.MustNew(), Procedure: "Обработчик"}, catalog.Project)
				}
				if len(issues) != 0 {
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

// The window of a defined type holds the constants in the folders of the key
// and of the manager of a value, the filter criteria in theirs and the
// settings storages under the row of their manager, whose box is the kind as
// a whole; the folders of the managers the editor takes only by themselves
// are marked so, and the others not. The model takes the key of a value of a
// constant in a defined type only, beside another type too, and the manager
// of any settings storage there only, with no note. Expectations from the
// opis (opredelyaemyj-tip.md, «Не входят в составной тип», «Досъёмка
// 24.10.2026» and «Досъёмка 25.10.2026»), written here.
//
// Defect caught: the constants or the storages missing from their rows; the
// box of the whole kind refused on saving, or noted though the designer
// saves it, or taken in a form; the key of a value taken where no window
// offers it, or refused beside a string; a manager of a catalog put beside a string without the
// window asking.
func TestTheWindowOfADefinedTypeHoldsConstantsCriteriaAndStorages(t *testing.T) {
	t.Parallel()
	catalog, err := Load(compositeProject(t))
	if err != nil {
		t.Fatal(err)
	}
	offer, _ := catalog.TypeOffer(DefinedTypeChoice)
	byName := map[string]TypeOfferRow{}
	for _, row := range offer.Rows {
		byName[row.Name.Russian] = row
	}
	for name, kind := range map[string]Kind{"КонстантаКлючЗначения": ConstantKind, "КонстантаМенеджерЗначения": ConstantKind,
		"КритерийОтбораМенеджер": FilterCriterionKind, "ХранилищеНастроекМенеджер": SettingsStorageKind} {
		row := byName[name]
		if want := catalog.referenceTypeObjects(kind); len(want) == 0 || len(row.Objects) != len(want) {
			t.Errorf("%s holds %d objects, the project has %d", name, len(row.Objects), len(want))
		}
	}
	if row := byName["ХранилищеНастроекМенеджер"]; row.Kind != "settings-storage-manager" || row.ObjectKind != "settings-storage-manager" {
		t.Errorf("the row of the manager of a settings storage: %+v", row)
	}
	for name, alone := range map[string]bool{"СправочникМенеджер": true, "КритерийОтбораМенеджер": true, "ЖурналДокументовМенеджер": true,
		"ПланСчетовМенеджер": false, "КонстантаМенеджерЗначения": false, "КонстантаКлючЗначения": false, "СправочникОбъект": false} {
		if byName[name].Alone != alone {
			t.Errorf("the folder %s marked alone %v, want %v", name, byName[name].Alone, alone)
		}
	}

	key := Type{Kind: "constant-value-key", Reference: refID(uuid.MustNew().String())}
	if issues := validateTypesIn("types", []Type{{Kind: StringType, Length: 10}, key}, placeDefinedType); len(issues) != 0 {
		t.Errorf("the key of a value of a constant beside a string in a defined type: %v", issues)
	}
	for _, place := range []typePlace{placeFormAttribute, placeRunningObject, placeSessionParameter, placeStored, placeCommandParameter} {
		if issues := validateTypesIn("types", []Type{key}, place); len(issues) == 0 {
			t.Errorf("the key of a value of a constant taken %s", placeName(place))
		}
	}
	anyStorage := Type{Kind: "settings-storage-manager"}
	if issues := validateTypesIn("types", []Type{{Kind: StringType, Length: 10}, anyStorage}, placeDefinedType); len(issues) != 0 {
		t.Errorf("the manager of any settings storage in a defined type: %v", issues)
	}
	for _, place := range []typePlace{placeFormAttribute, placeRunningObject} {
		if issues := validateTypesIn("types", []Type{anyStorage}, place); len(issues) != 1 || !strings.HasSuffix(issues[0], ".reference is required") {
			t.Errorf("the manager of any settings storage %s: %v", placeName(place), issues)
		}
	}
	root := metadataProject(t)
	id := uuid.MustNew().String()
	writeMetadata(t, root, DefinedTypeKind, id, "format: 1\nid: "+id+"\nname: Хранилища\ntitle: {ru: Хранилища}\ntypes: [{kind: settings-storage-manager}]\n")
	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("a defined type of the manager of any settings storage: %v", err)
	}
	// The designer saves it and its check is silent (opis of 25.10.2026):
	// no note.
	if notes := loaded.Notes(); slices.ContainsFunc(notes, func(note Note) bool {
		return strings.HasPrefix(note.Where, "defined-types Хранилища")
	}) {
		t.Errorf("the manager of any settings storage is noted: %+v", notes)
	}
}

// Every row of the window of the source of a subscription past the defined
// types is a box of the kind as a whole holding the objects of the kind - a
// recalculation, read inside its register, by its own name, as the help names
// its record set (ПерерасчетНаборЗаписей.<Имя перерасчета>) - and the kind as
// a whole and each object are what a subscription takes (opis of 24.10.2026,
// podpiska-na-sobytie.md, «Досъёмка»).
//
// Two recalculations of one name in two registers are told apart by their
// register through a dot, and one alone with its name keeps it (decision of
// the owner of 25.10.2026, podpiska-na-sobytie.md).
//
// Defect caught: a row without its box, so the kind as a whole cannot be
// chosen; the objects of another kind under a row; a recalculation missing,
// or shown under its register's name; two of one name shown as two rows
// alike, or every recalculation named with its register; a kind offered that
// the subscription then refuses.
func TestTheWindowOfASubscriptionSourceGivesEveryKindABoxAndItsObjects(t *testing.T) {
	t.Parallel()
	root := compositeProject(t)
	deductions := uuid.MustNew().String()
	writeMetadata(t, root, CalculationRegisterKind, deductions, "format: 1\nid: "+deductions+"\nname: Удержания\ntitle: {ru: Удержания}\n"+
		"chart_of_calculation_types: "+cmpCalculationTypes+"\nperiodicity: month\n"+
		"resources:\n  - {id: "+uuid.MustNew().String()+", name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}\n"+
		"recalculations:\n  - {id: "+uuid.MustNew().String()+", name: Перерасчет, title: {ru: Перерасчёт}}\n"+
		"  - {id: "+uuid.MustNew().String()+", name: Особый, title: {ru: Особый}}\n")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	offer, _ := catalog.TypeOffer(SubscriptionSourceChoice)
	if len(offer.Rows) == 0 || offer.Rows[0].ObjectKind != DefinedType || offer.Rows[0].Kind != "" {
		t.Fatalf("the first row is not the folder of the defined types: %+v", offer.Rows)
	}
	for _, row := range offer.Rows[1:] {
		if row.Kind == "" || row.ObjectKind != row.Kind {
			t.Errorf("%s: a box %q over objects of %q", row.Name.Russian, row.Kind, row.ObjectKind)
		}
		if !listenable(row.Kind) {
			t.Errorf("%s offered, and a subscription cannot listen to it", row.Kind)
		}
	}
	byName := map[string][]string{}
	for _, row := range offer.Rows {
		for _, object := range row.Objects {
			byName[row.Name.Russian] = append(byName[row.Name.Russian], object.Name)
		}
	}
	for row, want := range map[string][]string{
		"ПерерасчетНаборЗаписей":         {"Начисления.Перерасчет", "Удержания.Перерасчет", "Особый"},
		"ПоследовательностьНаборЗаписей": names(catalog.Sequences),
		"СправочникОбъект":               names(catalog.Catalogs), "ХранилищеНастроекМенеджер": names(catalog.SettingsStorages),
		"КонстантаМенеджерЗначения": names(catalog.Constants),
	} {
		// In the order of the tree, which the composite project does not
		// write; the order is TestTheWindowsListObjectsInTheOrderOfTheTree's.
		if len(want) == 0 || !slices.Equal(slices.Sorted(slices.Values(byName[row])), slices.Sorted(slices.Values(want))) {
			t.Errorf("%s holds %v, want %v", row, byName[row], want)
		}
	}
}

// names are the names of the objects of a list, in its order.
func names[T any](list []T) []string {
	var result []string
	for _, item := range list {
		result = append(result, reflect.ValueOf(item).FieldByName("Name").String())
	}
	return result
}

// The window of a field and of a function of a source offers the object
// tables of every source as «Источник.Таблица» and the dimension tables of
// every cube of every source as «Источник.Куб.Таблица», that of a dimension of
// a cube the dimension tables alone, and the model takes a field, a function
// and a dimension of a cube of one source typed by a dimension table of another -
// «Проверка конфигурации» takes such a reference (opis of 24.10.2026,
// vneshnij-istochnik.md, snapshots series-2.531/22-25) - with the note of a
// reference to another source, and the value of a function of binary data with
// its qualifiers, as the window offers it (snapshots 19, 21).
//
// Defect caught: a table that holds records offered as a reference; another
// source's tables missing; an object named without its source, so two tables
// of one name in two sources read alike; a reference the window offers
// refused on loading, or taken without the note; binary data refused in a
// function.
func TestTheWindowOfASourceOffersTheTablesOfEverySource(t *testing.T) {
	t.Parallel()
	root := compositeProject(t)
	dimension := "[{kind: external-data-source-dimension-table, reference: " + goodsDimTable + "}]"
	writeExternalSource(t, root, otherSource, "Бухгалтерия", "functions:\n"+
		"  - {id: "+uuid.MustNew().String()+", name: Остаток, title: {ru: Остаток}, return_value: true, types: "+dimension+"}\n"+
		"  - {id: "+uuid.MustNew().String()+", name: Файл, title: {ru: Файл}, return_value: true, types: [{kind: binary-data, length: 16, fixed_length: true}]}\n")
	writeExternalTable(t, root, "Бухгалтерия", foreignTable, "Счета", "name_in_data_source: accounts\ndata_type: object\nkey_fields: [Код]\nfields:\n"+
		"  - {id: "+foreignCodeField+", name: Код, title: {ru: Код}, types: [{kind: string, length: 10}], name_in_data_source: code}\n"+
		"  - {id: "+uuid.MustNew().String()+", name: Товар, title: {ru: Товар}, types: "+dimension+", name_in_data_source: goods}\n")
	writeFile(t, filepath.Join(root, "metadata", string(ExternalDataSourceKind), "Бухгалтерия", ExternalDataSourceCubesDirectory, "Обороты", "object.yaml"),
		"format: 1\nid: "+uuid.MustNew().String()+"\nname: Обороты\ntitle: {ru: Обороты}\nname_in_data_source: Turnovers\ndimensions:\n"+
			"  - {id: "+uuid.MustNew().String()+", name: Товар, title: {ru: Товар}, types: "+dimension+"}\n")
	writeExternalTable(t, root, "Бухгалтерия", uuid.MustNew().String(), "Проводки", "name_in_data_source: entries\ndata_type: non-object\nkey_fields: [Номер]\nfields:\n"+
		"  - {id: "+uuid.MustNew().String()+", name: Номер, title: {ru: Номер}, types: [{kind: string, length: 10}], name_in_data_source: number}\n")
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a field and a function typed by a dimension table of another source: %v", err)
	}
	var noted []string
	for _, note := range catalog.Notes() {
		if note.Kind == NoteForeignSourceReference {
			noted = append(noted, note.Where)
		}
	}
	if want := []string{"external-data-sources Бухгалтерия cubes Обороты dimensions Товар", "external-data-sources Бухгалтерия functions Остаток",
		"external-data-sources Бухгалтерия tables Счета fields Товар"}; !slices.Equal(slices.Sorted(slices.Values(noted)), want) {
		t.Errorf("noted %v, want %v", noted, want)
	}
	for _, place := range []TypeChoicePlace{ExternalFieldChoice, ExternalFunctionChoice} {
		offer, _ := catalog.TypeOffer(place)
		folders := map[string][]string{}
		for _, row := range offer.Rows {
			for _, object := range row.Objects {
				folders[row.Name.Russian] = append(folders[row.Name.Russian], object.Name)
			}
		}
		tables := slices.Sorted(slices.Values(folders["ВнешнийИсточникДанныхТаблицаСсылка"]))
		if want := []string{"Бухгалтерия.Счета", "Склад.Товары"}; !slices.Equal(tables, want) {
			t.Errorf("%s offers the tables %v, want %v", place, tables, want)
		}
		if got, want := folders["ВнешнийИсточникДанныхКубТаблицаИзмеренияСсылка"], []string{"Склад.Продажи.Товары"}; !slices.Equal(got, want) {
			t.Errorf("%s offers the dimension tables %v, want %v", place, got, want)
		}
	}
	// A dimension of a cube: the dimension tables alone (snapshots 18, 20).
	offer, _ := catalog.TypeOffer(CubeDimensionChoice)
	var rows []string
	for _, row := range offer.Rows {
		rows = append(rows, row.Name.Russian)
	}
	if want := []string{"ОпределяемыйТип", "ВнешнийИсточникДанныхКубТаблицаИзмеренияСсылка"}; !slices.Equal(rows, want) {
		t.Errorf("a dimension of a cube offers %v, want %v", rows, want)
	}
}

// The window of a resource of a register marks the folder of the defined
// types as standing alone - beside the number or another defined type the
// prototype's asks «Будут исключены остальные типы данных» (opis of
// 24.10.2026, resurs.md, snapshots series-2.531/12, 13) - and the number not;
// no other place marks the defined types so. What it offers the check of a
// resource takes, one type at a time, and refuses two (TestEveryOfferedType-
// IsAccepted holds the one).
//
// Defect caught: a resource the window lets have a number and a defined type,
// which the save refuses; the mark leaking to the defined types of a stored
// field, where they stand beside any type.
func TestTheWindowOfAResourceTakesADefinedTypeAlone(t *testing.T) {
	t.Parallel()
	catalog, err := Load(compositeProject(t))
	if err != nil {
		t.Fatal(err)
	}
	for place, want := range map[TypeChoicePlace]bool{NumericResourceChoice: true, StoredFieldChoice: false, DefinedTypeChoice: false} {
		offer, _ := catalog.TypeOffer(place)
		for _, row := range offer.Rows {
			if row.ObjectKind == DefinedType && row.Alone != want {
				t.Errorf("%s marks the defined types alone %v, want %v", place, row.Alone, want)
			}
			if row.Kind == NumberType && row.Alone {
				t.Errorf("%s marks the number alone", place)
			}
		}
	}
	defined := catalog.referenceTypeObjects(DefinedTypeKind)[0].ID
	both := Attribute{Types: []Type{{Kind: NumberType, Precision: 10}, {Kind: DefinedType, Reference: &defined}}}
	if issues := validateNumericResource("resource", both); len(issues) == 0 {
		t.Error("a resource of a number and a defined type was taken")
	}
}
