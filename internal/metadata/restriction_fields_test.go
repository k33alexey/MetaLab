package metadata

import (
	"maps"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// TestARestrictionNamesWhatTheTableOfAQueryHolds catches a command taken for
// a field a restriction of reading may name: the help gives the fields of the
// table a query reads («Ограничение доступа к данным»), and a command keeps no
// value in it. The fields of the catalog are written out here: its attribute,
// its table part with its line number, its version of data.
func TestARestrictionNamesWhatTheTableOfAQueryHolds(t *testing.T) {
	t.Parallel()
	catalog, definition := partedCatalog(t)
	attribute, table, command := definition.Attributes[0].ID, definition.TableParts[0].ID, definition.Commands[0].ID
	var keys []string
	for _, field := range catalog.restrictionFieldsOf(CatalogKind, definition.ID) {
		keys = append(keys, field.key)
	}
	for _, want := range []string{"dataversion", attribute.String(), table.String(), table.String() + ".linenumber"} {
		if !slices.Contains(keys, want) {
			t.Errorf("a restriction of the catalog cannot name %s: %v", want, keys)
		}
	}
	if slices.Contains(keys, command.String()) {
		t.Errorf("a restriction of the catalog may name its command: %v", keys)
	}
}

// partedCatalog is a catalog of every sort of part a module or a
// restriction reads: an attribute, a table part with its own attribute, and
// a command.
func partedCatalog(t *testing.T) (*Catalog, CatalogDefinition) {
	t.Helper()
	definition := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Товары", Title: LocalizedText{"ru": "Товары"},
		Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 150,
		Attributes: []Attribute{{ID: uuid.MustNew(), Name: "Цена", Title: LocalizedText{"ru": "Цена"}, Types: []Type{{Kind: NumberType, Precision: 12, Scale: 2}}}},
		TableParts: []TablePart{{ID: uuid.MustNew(), Name: "Детали", Title: LocalizedText{"ru": "Детали"},
			Attributes: []Attribute{{ID: uuid.MustNew(), Name: "Текст", Title: LocalizedText{"ru": "Текст"}, Types: []Type{{Kind: StringType, Length: 10}}}}}},
		Commands: []ObjectCommand{{ID: uuid.MustNew(), Name: "Подбор", Title: LocalizedText{"ru": "Подбор"}}}}
	catalog, err := NewCatalogSnapshotWithRoles(metadataConfiguration(), nil, nil, nil, []CatalogDefinition{definition}, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return catalog, definition
}

// TestThisObjectHasTheFieldsOfItsObjectAlone catches the members of
// ЭтотОбъект of a module of an object taking the fields of its table parts:
// the help gives СправочникОбъект its attributes and its table parts, and a
// field of a table part is a member of its row (СправочникТабличнаяЧастьСтрока),
// not of the object.
func TestThisObjectHasTheFieldsOfItsObjectAlone(t *testing.T) {
	t.Parallel()
	catalog, definition := partedCatalog(t)
	var names []string
	for _, member := range catalog.ThisObjectMembers()[project.ModuleAddress{Kind: string(CatalogKind), Object: definition.Name, Module: kinds.ObjectModule}] {
		names = append(names, member.Name.Russian)
	}
	for _, want := range []string{"Цена", "Детали", "Наименование"} {
		if !slices.Contains(names, want) {
			t.Errorf("ЭтотОбъект of the catalog has no %s: %v", want, names)
		}
	}
	if slices.Contains(names, "Текст") {
		t.Errorf("ЭтотОбъект of the catalog has a field of its table part: %v", names)
	}
}

// TestNoFieldIsOfferedWhereNothingIsRestricted catches the schema of rights
// offering the fields of a restriction on an object no restriction may be put
// on - a data processor keeps no data in the infobase (help, «Ограничение
// доступа к данным») -, which the window of a restriction would then show
// with no right to put it on.
func TestNoFieldIsOfferedWhereNothingIsRestricted(t *testing.T) {
	t.Parallel()
	schema, err := LoadPermissionSchema(compositeProject(t))
	if err != nil {
		t.Fatal(err)
	}
	var processor bool
	for _, object := range schema.Objects {
		if len(object.RestrictedRights) == 0 && len(object.RestrictionFields) != 0 {
			t.Errorf("%s offers fields to restrict and no right: %+v", object.Name, object.RestrictionFields)
		}
		processor = processor || object.ID.String() == cmpProcessor
	}
	if !processor {
		t.Fatal("the data processor, which has an attribute, is not in the schema")
	}
}

// TestThePartsOfAKindNoExportGivesRightsOnAreOfferedAsTheWindowShows
// catches the window of a role offering on the parts of a kind whose parts
// no export gives rights on (Rights.xml of every role, 13.10.2026) other
// rights than the window of the prototype: the commands of an enumeration,
// a filter criterion, a cube and a table of a dimension are viewed, the
// dimensions and resources of a cube viewed, the fields of a table of a
// dimension viewed and edited (passes of 20 and 21.10.2026, prava/16-18,
// 44, 56-63). Where none was offered (2.434) the window gave the folder no
// right at all.
func TestThePartsOfAKindNoExportGivesRightsOnAreOfferedAsTheWindowShows(t *testing.T) {
	t.Parallel()
	view, edit := PermissionView, PermissionEdit
	for _, item := range []struct {
		kind  Kind
		group PartGroup
		want  []PermissionOperation
	}{
		{FilterCriterionKind, CommandsPart, []PermissionOperation{view}},
		{ExternalDimensionTableKind, FieldsPart, []PermissionOperation{view, edit}},
		{ExternalDimensionTableKind, CommandsPart, []PermissionOperation{view}},
		{ExternalCubeKind, CommandsPart, []PermissionOperation{view}},
		{EnumerationKind, CommandsPart, []PermissionOperation{view}},
		{ExternalCubeKind, DimensionsPart, []PermissionOperation{view}},
		{ExternalCubeKind, ResourcesPart, []PermissionOperation{view}},
		{CatalogKind, CommandsPart, []PermissionOperation{view}},
	} {
		if got := offeredPartRights(item.kind, item.group); !slices.Equal(got, item.want) {
			t.Errorf("%s offers %v on its %s, want %v", item.kind, got, item.group, item.want)
		}
	}
}

// TestTheFieldsOfARestrictionAreThoseOfTheTableOfItsObject holds the fields
// the window of a restriction lists for a catalog, a chart of characteristic
// types and a chart of accounts to the prototype's (passes of 22.10.2026 on
// ERP: Валюты and Бригады, then НоменклатураПоставщиков,
// ПодразделенияОрганизаций, КатегорииЦелей, НомераГТД,
// КаналыРекламныхВоздействий, АналитикиСтатейБюджетов, СтатьиДоходов and
// Хозрасчетный, block-00/vse-ogranicheniya-dostupa.md; ACCESS-RIGHTS.md,
// «Состав и порядок полей ограничения») and to the help of the tables of a
// query (Родитель - only of a multi-level object, Владелец - only of a
// subordinate catalog; the accounting flags of the ext dimensions are fields
// of ВидыСубконто). The lists are written out here, from the inventory, not
// read off the sets of standard attributes. Defects caught: every catalog
// offered Родитель, ЭтоГруппа, Владелец and Код whatever it was (2.437, case
// 2в-23); the version of the data first and the predefined fields before
// the attributes; Владелец after ЭтоГруппа, ЭтоГруппа with a hierarchy of
// items, Наименование of length 0 (2.440); a chart of characteristic types
// and a chart of accounts offering every standard field in the order of the
// standard attributes, the flags of the ext dimensions among the account's
// own (2.440); the fields of ВидыСубконто in the order the prototype
// describes them and its predefined one named Предопределенный (2.443; the
// fourth pass, pola-16, pola-17: НомерСтроки first, then the fields of the
// line as the window lists them), and so on a chart with no accounting flags
// of the ext dimensions (2.445 sorted only where there were flags); a chart
// of calculation types offering every standard field and part whatever its
// settings, in the order of the standard attributes, the lists after the
// attributes (2.444; the fourth pass, pola-19…24: ПериодДействияБазовый and
// ВытесняющиеВидыРасчета only with the period of action, БазовыеВидыРасчета
// only with a base, Код only with a length). «Список полей» of the window of a
// role lists the same fields in its own order (the fourth and the fifth
// passes, pola-18, pola-25…31), written out here from the inventory as well:
// the defect caught is the window of a role offering the fields in the
// wizard's order (2.446), and a field one window offers and the other not.
// ЭтотОбъект keeps every field: the help of СправочникОбъект
// makes none of them hang on the catalog.
func TestTheFieldsOfARestrictionAreThoseOfTheTableOfItsObject(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	attribute := func(name string) string {
		return "  - {id: " + uuid.MustNew().String() + ", name: " + name + ", title: {ru: " + name + "}, types: [{kind: string, length: 10}]}\n"
	}
	tablePart := func(name string, fields ...string) string {
		result := "table_parts:\n  - id: " + uuid.MustNew().String() + "\n    name: " + name + "\n    title: {ru: " + name + "}\n    attributes:\n"
		for _, field := range fields {
			result += "  " + attribute(field)
		}
		return result
	}
	type object struct {
		kind Kind
		id   uuid.UUID
		name string
		want []string
	}
	var objects []object
	write := func(kind Kind, name, settings string, want ...string) uuid.UUID {
		id := uuid.MustNew()
		writeMetadata(t, root, kind, id.String(), "format: 1\nid: "+id.String()+"\nname: "+name+"\ntitle: {ru: "+name+"}\n"+settings)
		objects = append(objects, object{kind, id, name, want})
		return id
	}
	head := []string{"Ссылка", "ВерсияДанных", "ПометкаУдаления"}
	tail := []string{"Предопределенный", "ИмяПредопределенныхДанных"}
	list := func(names ...string) []string { return slices.Concat(head, names, tail) }
	currencies := write(CatalogKind, "Валюты", "code: {type: string, length: 3}\ndescription_length: 50\nattributes:\n"+attribute("Наценка"),
		list("Код", "Наименование", "Наценка")...)
	write(CatalogKind, "Бригады", "code: {type: string, length: 0}\ndescription_length: 50\nhierarchy: {enabled: true, kind: folders-and-items}\n"+
		"attributes:\n"+attribute("Мастер")+tablePart("Состав", "Сотрудник"),
		list("Родитель", "ЭтоГруппа", "Наименование", "Мастер", "Состав", "НомерСтроки", "Сотрудник")...)
	write(CatalogKind, "Счета", "code: {type: string, length: 9}\ndescription_length: 50\nowners: ["+currencies.String()+"]\n",
		list("Владелец", "Код", "Наименование")...)
	write(CatalogKind, "НоменклатураПоставщиков", "code: {type: string, length: 9}\ndescription_length: 50\nowners: ["+currencies.String()+"]\n"+
		"hierarchy: {enabled: true, kind: folders-and-items}\nattributes:\n"+attribute("Артикул"),
		list("Владелец", "Родитель", "ЭтоГруппа", "Код", "Наименование", "Артикул")...)
	write(CatalogKind, "ПодразделенияОрганизаций", "code: {type: string, length: 9}\ndescription_length: 50\nowners: ["+currencies.String()+"]\n"+
		"hierarchy: {enabled: true, kind: items}\n"+tablePart("Руководители", "Сотрудник"),
		list("Владелец", "Родитель", "Код", "Наименование", "Руководители", "НомерСтроки", "Сотрудник")...)
	write(CatalogKind, "КатегорииЦелей", "code: {type: string, length: 0}\ndescription_length: 50\nhierarchy: {enabled: true, kind: items}\n"+
		"attributes:\n"+attribute("Описание"),
		list("Родитель", "Наименование", "Описание")...)
	write(CatalogKind, "НомераГТД", "code: {type: string, length: 30}\ndescription_length: 0\nattributes:\n"+attribute("Страна"),
		list("Код", "Страна")...)
	valueType := "value_type: [{kind: boolean}]\n"
	write(ChartOfCharacteristicTypesKind, "КаналыРекламныхВоздействий", "code: {type: string, length: 0}\ndescription_length: 50\n"+valueType,
		list("Наименование", "ТипЗначения")...)
	write(ChartOfCharacteristicTypesKind, "АналитикиСтатейБюджетов", "code: {type: string, length: 9}\ndescription_length: 50\n"+valueType+
		"attributes:\n"+attribute("Тип"),
		list("Код", "Наименование", "ТипЗначения", "Тип")...)
	write(ChartOfCharacteristicTypesKind, "СтатьиДоходов", "code: {type: string, length: 9}\ndescription_length: 50\n"+valueType+
		"hierarchy: {enabled: true, kind: folders-and-items}\nattributes:\n"+attribute("Статья")+tablePart("Счета", "Счет", "Аналитика"),
		list("Родитель", "ЭтоГруппа", "Код", "Наименование", "ТипЗначения", "Статья", "Счета", "НомерСтроки", "Счет", "Аналитика")...)
	extDimensionKinds := write(ChartOfCharacteristicTypesKind, "ВидыСубконтоХозрасчетные", "code: {type: string, length: 9}\ndescription_length: 50\n"+valueType,
		list("Код", "Наименование", "ТипЗначения")...)
	write(ChartOfAccountsKind, "Хозрасчетный", "code: {type: string, length: 9}\ndescription_length: 120\nmax_ext_dimension_count: 3\n"+
		"ext_dimension_types: "+extDimensionKinds.String()+"\nattributes:\n"+attribute("КодБыстрогоВыбора")+
		"accounting_flags:\n  - {id: "+uuid.MustNew().String()+", name: Валютный, title: {ru: Валютный}}\n"+
		"ext_dimension_accounting_flags:\n  - {id: "+uuid.MustNew().String()+", name: Суммовой, title: {ru: Суммовой}}\n",
		list("Родитель", "Код", "Наименование", "Порядок", "Вид", "Забалансовый", "КодБыстрогоВыбора", "Валютный",
			"ВидыСубконто", "НомерСтроки", "ВидСубконто", "Предопределенное", "ТолькоОбороты", "Суммовой")...)
	write(ChartOfAccountsKind, "Международный", "code: {type: string, length: 9}\ndescription_length: 120\nmax_ext_dimension_count: 3\n"+
		"ext_dimension_types: "+extDimensionKinds.String()+"\n",
		list("Родитель", "Код", "Наименование", "Порядок", "Вид", "Забалансовый",
			"ВидыСубконто", "НомерСтроки", "ВидСубконто", "Предопределенное", "ТолькоОбороты")...)
	competition := func(part string) []string {
		return []string{part, "НомерСтроки", "ВидРасчета", "Предопределенный"}
	}
	write(ChartOfCalculationTypesKind, "Начисления", "code: {type: string, length: 5}\ndescription_length: 100\n"+
		"action_period_use: true\nbase_dependency: by-action-period\nattributes:\n"+attribute("Категория")+tablePart("Показатели", "Показатель", "Значение"),
		list(slices.Concat([]string{"Код", "Наименование", "ПериодДействияБазовый"}, competition("БазовыеВидыРасчета"),
			competition("ВедущиеВидыРасчета"), competition("ВытесняющиеВидыРасчета"),
			[]string{"Категория", "Показатели", "НомерСтроки", "Показатель", "Значение"})...)...)
	write(ChartOfCalculationTypesKind, "Удержания", "code: {type: string, length: 0}\ndescription_length: 100\n"+
		"attributes:\n"+attribute("Категория"),
		list(slices.Concat([]string{"Наименование"}, competition("ВедущиеВидыРасчета"), []string{"Категория"})...)...)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	names := func(kind Kind, id uuid.UUID) []string {
		var result []string
		for _, field := range catalog.restrictionFieldsOf(kind, id) {
			result = append(result, field.name)
		}
		return result
	}
	for _, check := range objects {
		if got := names(check.kind, check.id); !slices.Equal(got, check.want) {
			t.Errorf("a restriction of %s %s names\n%v, want\n%v", check.kind, check.name, got, check.want)
		}
	}
	roleHead := []string{"Ссылка", "ВерсияДанных"}
	roleTail := []string{"ПометкаУдаления", "Предопределенный", "ИмяПредопределенныхДанных"}
	roleWindow := map[string][]string{
		"Валюты": slices.Concat(roleHead, []string{"Код", "Наименование"}, roleTail, []string{"Наценка"}),
		"Бригады": slices.Concat(roleHead, []string{"Наименование", "Родитель", "ЭтоГруппа"}, roleTail,
			[]string{"Мастер", "Состав", "НомерСтроки", "Сотрудник"}),
		"НоменклатураПоставщиков": slices.Concat(roleHead, []string{"Код", "Наименование", "Владелец", "Родитель", "ЭтоГруппа"}, roleTail,
			[]string{"Артикул"}),
		"КатегорииЦелей": slices.Concat(roleHead, []string{"Наименование", "Родитель"}, roleTail, []string{"Описание"}),
		"СтатьиДоходов": slices.Concat(roleHead, []string{"Код", "Наименование", "ТипЗначения", "Родитель", "ЭтоГруппа"}, roleTail,
			[]string{"Статья", "Счета", "НомерСтроки", "Счет", "Аналитика"}),
		"Хозрасчетный": slices.Concat(roleHead, []string{"Код", "Наименование", "Порядок", "Родитель", "Вид", "Забалансовый"}, roleTail,
			[]string{"КодБыстрогоВыбора", "Валютный", "ВидыСубконто", "НомерСтроки", "ВидСубконто", "ТолькоОбороты", "Предопределенное", "Суммовой"}),
		"Начисления": slices.Concat(roleHead, []string{"Код", "Наименование"}, roleTail, []string{"ПериодДействияБазовый"},
			competition("БазовыеВидыРасчета"), competition("ВедущиеВидыРасчета"), competition("ВытесняющиеВидыРасчета"),
			[]string{"Категория", "Показатели", "НомерСтроки", "Показатель", "Значение"}),
	}
	schema, err := LoadPermissionSchema(root)
	if err != nil {
		t.Fatal(err)
	}
	keys := func(fields []PermissionField) []string {
		var result []string
		for _, field := range fields {
			result = append(result, field.Key)
		}
		slices.Sort(result)
		return result
	}
	for _, object := range schema.Objects {
		if want, ok := roleWindow[object.Name]; ok {
			var got []string
			for _, field := range object.RoleWindowFields {
				got = append(got, field.Name)
			}
			if !slices.Equal(got, want) {
				t.Errorf("the window of a role offers %s\n%v, want\n%v", object.Name, got, want)
			}
			delete(roleWindow, object.Name)
		}
		if !slices.Equal(keys(object.RoleWindowFields), keys(object.RestrictionFields)) {
			t.Errorf("the two windows offer %s different fields:\n%v\n%v", object.Name, keys(object.RoleWindowFields), keys(object.RestrictionFields))
		}
	}
	if len(roleWindow) != 0 {
		t.Errorf("the schema has no %v", slices.Collect(maps.Keys(roleWindow)))
	}
	var members []string
	for address, list := range catalog.ThisObjectMembers() {
		if address.Object == "Валюты" && address.Module == kinds.ObjectModule {
			for _, member := range list {
				members = append(members, member.Name.Russian)
			}
		}
	}
	for _, want := range []string{"Родитель", "ЭтоГруппа", "Владелец", "Код"} {
		if !slices.Contains(members, want) {
			t.Errorf("ЭтотОбъект of Валюты has no %s: %v", want, members)
		}
	}
}

// TestARestrictionKeepsAFieldTheObjectHasNoLonger holds the decision on a
// restriction naming a field a setting of the object took away (owner,
// 22.10.2026, question В4; ACCESS-RIGHTS.md, «Ограничение по полю, которого
// у объекта нет»): the prototype keeps it and says nothing, so the load
// accepts it and notes the role, the object, the right and the field by its
// name; the schema of rights gives the windows the fields of the table
// alone, as the prototype's show no other (absent-20…27). Defects caught:
// the load refusing such a role (2.440); the note naming the key, not the
// field; the field offered in the windows; a field the catalog has taken
// for one it has not.
func TestARestrictionKeepsAFieldTheObjectHasNoLonger(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	id := uuid.MustNew()
	writeMetadata(t, root, CatalogKind, id.String(), "format: 1\nid: "+id.String()+"\nname: КатегорииЦелей\ntitle: {ru: Категории целей}\n"+
		"code: {type: string, length: 9}\ndescription_length: 50\n")
	role := RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Аналитик", Title: LocalizedText{"ru": "Аналитик"},
		Objects: []ObjectPermission{{Object: id, Operations: []PermissionOperation{PermissionRead},
			Restrictions: []RightRestriction{{Right: PermissionRead, Fields: []string{"parent"}, Condition: "ГДЕ Родитель = &Р"}}}}}
	if err := WriteRole(root, role); err != nil {
		t.Fatal(err)
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a role keeping a field the catalog has no longer was refused: %v", err)
	}
	var noted []string
	for _, note := range catalog.Notes() {
		noted = append(noted, string(note.Kind)+" "+note.Where+": "+note.Written)
	}
	if want := []string{"restriction-absent-field roles Аналитик restrictions: catalogs КатегорииЦелей read Родитель"}; !slices.Equal(noted, want) {
		t.Errorf("notes %v, want %v", noted, want)
	}
	schema, err := LoadPermissionSchema(root)
	if err != nil {
		t.Fatal(err)
	}
	index := slices.IndexFunc(schema.Objects, func(object PermissionObject) bool { return object.ID == id })
	if index < 0 {
		t.Fatal("the catalog is not in the schema")
	}
	keys := func(fields []PermissionField) map[string]string {
		result := map[string]string{}
		for _, field := range fields {
			result[field.Key] = field.Name
		}
		return result
	}
	if shown := keys(schema.Objects[index].RestrictionFields); shown["parent"] != "" {
		t.Errorf("the window of a restriction of a catalog with no hierarchy offers Родитель: %v", shown)
	}
	absent := map[string]string{}
	for _, field := range catalog.absentRestrictionFieldsOf(CatalogKind, id) {
		absent[field.key] = field.name
	}
	for key, name := range map[string]string{"parent": "Родитель", "isfolder": "ЭтоГруппа", "owner": "Владелец"} {
		if absent[key] != name {
			t.Errorf("a note cannot name %s of the catalog: %v", name, absent)
		}
	}
	if absent["code"] != "" || absent["ref"] != "" {
		t.Errorf("a field the catalog has is among those it has not: %v", absent)
	}
}

// TestTheFieldsOfARestrictionOfTheOtherKindsAreThePrototypes holds the
// fields both windows list for a restriction of the other kinds the wizard
// offers to the prototype's (passes of 22.10.2026, the seventh and the
// eighth, pola-33…57; ACCESS-RIGHTS.md, «Поля ограничения прочих видов»).
// The lists are written out here from the inventory, not read off the
// orders the code keeps. Defects caught: the version of the data before the
// reference (every kind keeping it, until 2.457); the window of a role
// listing them in the wizard's order; a journal offering Тип and none of its
// columns; a register of information offering the line number and the
// activity whatever it is, the period where it is not periodic and the
// recorder where it is independent; the registers of accumulation,
// accounting and calculation offering their fields at all; Номер of a
// document, a business process or a task with a number of no length, and
// Наименование of a task with a description of none (until 2.462; the
// ninth pass, pola-60…62), and Номер left out of a document of a numerator,
// which has the numerator's number. A restriction of
// such a register by a field, which the exports never write, is loaded and
// noted, as the iron rule of CONFORMANCE.md says.
func TestTheFieldsOfARestrictionOfTheOtherKindsAreThePrototypes(t *testing.T) {
	t.Parallel()
	root := taskWithAddressing(t, `addressing_attributes:
  - id: `+addressRoleAttribute+`
    name: РольИсполнителя
    title: {ru: Роль исполнителя}
    types: [{kind: catalog, reference: `+addressRoleCatalogID+`}]
    dimension: `+addressRoleDimension+`
table_parts:
  - id: `+uuid.MustNew().String()+`
    name: Этапы
    title: {ru: Этапы}
    attributes:
      - {id: `+uuid.MustNew().String()+`, name: Этап, title: {ru: Этап}, types: [{kind: string, length: 10}]}
`)
	field := func(name string) string {
		return "  - {id: " + uuid.MustNew().String() + ", name: " + name + ", title: {ru: " + name + "}, types: [{kind: string, length: 10}]}\n"
	}
	write := func(kind Kind, name, settings string) uuid.UUID {
		id := uuid.MustNew()
		writeMetadata(t, root, kind, id.String(), "format: 1\nid: "+id.String()+"\nname: "+name+"\ntitle: {ru: "+name+"}\n"+settings)
		return id
	}
	number := "number: {type: string, length: 11, auto: true, periodicity: none}\n"
	documentAttribute := uuid.MustNew().String()
	document := write(DocumentKind, "АвансовыйОтчет", number+"attributes:\n  - {id: "+documentAttribute+", name: Подотчетник, title: {ru: Подотчетник}, types: [{kind: string, length: 10}]}\n"+
		"table_parts:\n  - id: "+uuid.MustNew().String()+"\n    name: Расходы\n    title: {ru: Расходы}\n    attributes:\n    "+field("Сумма"))
	write(DocumentJournalKind, "Взаиморасчеты", "documents: ["+document.String()+"]\n"+
		"columns:\n  - {id: "+uuid.MustNew().String()+", name: Сотрудник, title: {ru: Сотрудник}, references: ["+documentAttribute+"]}\n")
	write(ExchangePlanKind, "Полный", "code: {type: string, length: 9}\ndescription_length: 100\nattributes:\n"+field("Адрес"))
	write(BusinessProcessKind, "Задание", number+"task: "+addressTaskID+"\nattributes:\n"+field("Описание"))
	// A number and a description of no length (pola-60…62).
	unnumbered := "number: {type: string, length: 0, periodicity: none}\n"
	write(DocumentKind, "РегламентированныйОтчет", unnumbered)
	write(BusinessProcessKind, "Поручение", unnumbered+"task: "+addressTaskID+"\n")
	write(TaskKind, "Напоминание", unnumbered+"description_length: 0\n")
	// A document of a numerator has the numerator's number, its own unwritten.
	numerator := write(NumeratorKind, "СквознаяНумерация", "number: {type: string, length: 11, periodicity: none}\n")
	write(DocumentKind, "ПоНумератору", "numerator: "+numerator.String()+"\n")
	write(InformationRegisterKind, "КурсыВалют", "write_mode: independent\nperiodicity: day\ndimensions:\n"+field("Валюта")+
		"resources:\n"+field("Курс")+"attributes:\n"+field("Источник"))
	write(InformationRegisterKind, "Настройки", "write_mode: independent\nperiodicity: none\ndimensions:\n"+field("Ключ")+"resources:\n"+field("Значение"))
	write(InformationRegisterKind, "СтатусыДокументов", "write_mode: recorder\nperiodicity: day\ndimensions:\n"+field("Документ")+"resources:\n"+field("Статус"))
	write(InformationRegisterKind, "БлокировкиЯчеек", "write_mode: recorder\nperiodicity: none\ndimensions:\n"+field("Ячейка")+"resources:\n"+field("Причина"))
	dimension := uuid.MustNew().String()
	register := write(AccumulationRegisterKind, "ОстаткиТоваров", "kind: balance\ndimensions:\n  - {id: "+dimension+", name: Склад, title: {ru: Склад}, types: [{kind: string, length: 10}]}\n"+
		"resources:\n  - {id: "+uuid.MustNew().String()+", name: Количество, title: {ru: Количество}, types: [{kind: number, precision: 15, scale: 3}]}\n")
	if err := WriteRole(root, RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Кладовщик", Title: LocalizedText{"ru": "Кладовщик"},
		Objects: []ObjectPermission{{Object: register, Operations: []PermissionOperation{PermissionRead},
			Restrictions: []RightRestriction{{Right: PermissionRead, Fields: []string{dimension, "period"}, Condition: "ГДЕ Склад = &С"}}}}}); err != nil {
		t.Fatal(err)
	}
	wizard := map[string][]string{
		"АвансовыйОтчет":          {"Ссылка", "ВерсияДанных", "ПометкаУдаления", "Номер", "Дата", "Проведен", "Подотчетник", "Расходы", "НомерСтроки", "Сумма"},
		"Взаиморасчеты":           {"Ссылка", "Дата", "ПометкаУдаления", "Номер", "Проведен", "Сотрудник"},
		"Полный":                  {"Ссылка", "ВерсияДанных", "ПометкаУдаления", "ЭтотУзел", "Код", "Наименование", "НомерОтправленного", "НомерПринятого", "Адрес"},
		"Задание":                 {"Ссылка", "ВерсияДанных", "ПометкаУдаления", "Номер", "Дата", "Завершен", "ВедущаяЗадача", "Стартован", "Описание"},
		"ЗадачаИсполнителя":       {"Ссылка", "ВерсияДанных", "ПометкаУдаления", "Номер", "Дата", "БизнесПроцесс", "ТочкаМаршрута", "Наименование", "Выполнена", "Предмет", "РольИсполнителя", "Этапы", "НомерСтроки", "Этап"},
		"КурсыВалют":              {"Период", "Валюта", "Курс", "Источник"},
		"РегламентированныйОтчет": {"Ссылка", "ВерсияДанных", "ПометкаУдаления", "Дата", "Проведен"},
		"Поручение":               {"Ссылка", "ВерсияДанных", "ПометкаУдаления", "Дата", "Завершен", "ВедущаяЗадача", "Стартован"},
		"Напоминание":             {"Ссылка", "ВерсияДанных", "ПометкаУдаления", "Дата", "БизнесПроцесс", "ТочкаМаршрута", "Выполнена"},
		"ПоНумератору":            {"Ссылка", "ВерсияДанных", "ПометкаУдаления", "Номер", "Дата", "Проведен"},
		"Настройки":               {"Ключ", "Значение"},
		"СтатусыДокументов":       {"Период", "Регистратор", "Документ", "Статус"},
		"БлокировкиЯчеек":         {"Регистратор", "Ячейка", "Причина"},
		"ИсполнителиЗадач":        {"РольИсполнителя", "Исполнитель"},
		"ОстаткиТоваров":          nil,
	}
	role := map[string][]string{
		"АвансовыйОтчет":          {"Ссылка", "ВерсияДанных", "Номер", "Дата", "Проведен", "ПометкаУдаления", "Подотчетник", "Расходы", "НомерСтроки", "Сумма"},
		"Взаиморасчеты":           {"Ссылка", "Дата", "Номер", "Проведен", "ПометкаУдаления", "Сотрудник"},
		"Полный":                  {"Ссылка", "ВерсияДанных", "ЭтотУзел", "Код", "Наименование", "ПометкаУдаления", "НомерОтправленного", "НомерПринятого", "Адрес"},
		"Задание":                 {"Ссылка", "ВерсияДанных", "Номер", "Дата", "ВедущаяЗадача", "ПометкаУдаления", "Стартован", "Завершен", "Описание"},
		"ЗадачаИсполнителя":       {"Ссылка", "ВерсияДанных", "Номер", "Наименование", "Дата", "ПометкаУдаления", "БизнесПроцесс", "ТочкаМаршрута", "Выполнена", "Предмет", "РольИсполнителя", "Этапы", "НомерСтроки", "Этап"},
		"КурсыВалют":              wizard["КурсыВалют"],
		"РегламентированныйОтчет": {"Ссылка", "ВерсияДанных", "Дата", "Проведен", "ПометкаУдаления"},
		"Поручение":               {"Ссылка", "ВерсияДанных", "Дата", "ВедущаяЗадача", "ПометкаУдаления", "Стартован", "Завершен"},
		"Напоминание":             {"Ссылка", "ВерсияДанных", "Дата", "ПометкаУдаления", "БизнесПроцесс", "ТочкаМаршрута", "Выполнена"},
		"ПоНумератору":            {"Ссылка", "ВерсияДанных", "Номер", "Дата", "Проведен", "ПометкаУдаления"},
		"Настройки":               wizard["Настройки"],
		"СтатусыДокументов":       wizard["СтатусыДокументов"],
		"БлокировкиЯчеек":         wizard["БлокировкиЯчеек"],
		"ИсполнителиЗадач":        wizard["ИсполнителиЗадач"],
		"ОстаткиТоваров":          nil,
	}
	schema, err := LoadPermissionSchema(root)
	if err != nil {
		t.Fatal(err)
	}
	names := func(fields []PermissionField) []string {
		var result []string
		for _, field := range fields {
			result = append(result, field.Name)
		}
		return result
	}
	for _, object := range schema.Objects {
		want, ok := wizard[object.Name]
		if !ok {
			continue
		}
		if got := names(object.RestrictionFields); !slices.Equal(got, want) {
			t.Errorf("the wizard offers %s\n%v, want\n%v", object.Name, got, want)
		}
		if got := names(object.RoleWindowFields); !slices.Equal(got, role[object.Name]) {
			t.Errorf("the window of a role offers %s\n%v, want\n%v", object.Name, got, role[object.Name])
		}
		delete(wizard, object.Name)
	}
	if len(wizard) != 0 {
		t.Errorf("the schema has no %v", slices.Collect(maps.Keys(wizard)))
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a restriction of a register of accumulation by its fields was refused: %v", err)
	}
	var noted []string
	for _, note := range catalog.Notes() {
		if note.Kind == NoteRestrictionAbsentField {
			noted = append(noted, note.Where+": "+note.Written)
		}
	}
	want := []string{"roles Кладовщик restrictions: accumulation-registers ОстаткиТоваров read Склад",
		"roles Кладовщик restrictions: accumulation-registers ОстаткиТоваров read Период"}
	if !slices.Equal(noted, want) {
		t.Errorf("notes %v, want %v", noted, want)
	}
	composite, err := LoadPermissionSchema(compositeProject(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, object := range composite.Objects {
		switch object.Kind {
		case AccumulationRegisterKind, AccountingRegisterKind, CalculationRegisterKind:
			if len(object.RestrictedRights) == 0 || len(object.RestrictionFields)+len(object.RoleWindowFields) != 0 {
				t.Errorf("%s %s is restricted on %v by the fields %v and %v", object.Kind, object.Name, object.RestrictedRights,
					names(object.RestrictionFields), names(object.RoleWindowFields))
			}
		}
	}
}
