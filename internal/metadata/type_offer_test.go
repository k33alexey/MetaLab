package metadata

import (
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
	// The folder of the enumerations of the platform by place (opis,
	// «Системные перечисления по местам»): 24 in an attribute of a data
	// processor, a form and a defined type, the three of accounting in a
	// session parameter.
	enumerations := map[TypeChoicePlace]int{RunningObjectChoice: 24, FormAttributeChoice: 24, DefinedTypeChoice: 24, SessionParameterChoice: 3}
	required := map[TypeChoicePlace]bool{StoredFieldChoice: true, CharacteristicValueChoice: true, SessionParameterChoice: true}
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

// The window of a defined type holds the constants in the folders of the key
// and of the manager of a value, the filter criteria in theirs and the
// settings storages under the row of their manager, whose box is the kind as
// a whole; the folders of the managers the editor takes only by themselves
// are marked so, and the others not. The model takes the key of a value of a
// constant in a defined type only, beside another type too, and the manager
// of any settings storage there only, with a note. Expectations from the opis
// (opredelyaemyj-tip.md, «Не входят в составной тип» and «Досъёмка
// 24.10.2026»), written here.
//
// Defect caught: the constants or the storages missing from their rows; the
// box of the whole kind refused on saving, or taken without a note, or taken
// in a form; the key of a value taken where no window offers it, or refused
// beside a string; a manager of a catalog put beside a string without the
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
	if notes := loaded.Notes(); !slices.ContainsFunc(notes, func(note Note) bool {
		return note.Kind == NoteAnySettingsStorageManager && strings.HasPrefix(note.Where, "defined-types Хранилища")
	}) {
		t.Errorf("the manager of any settings storage is not noted: %+v", notes)
	}
}
