package metadata

import (
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
// only with a base, Код only with a length). ЭтотОбъект keeps every field: the help of СправочникОбъект
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
// name; the schema of rights gives the window of a restriction the fields of
// the table alone and the list «Все ограничения доступа» the name of the
// field the object has no longer. Defects caught: the load refusing such a
// role (2.440); the note naming the key, not the field; the field offered in
// the window of the restriction, or left unnamed for the list.
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
	absent := keys(schema.Objects[index].AbsentFields)
	for key, name := range map[string]string{"parent": "Родитель", "isfolder": "ЭтоГруппа", "owner": "Владелец"} {
		if absent[key] != name {
			t.Errorf("the list cannot name %s of the catalog: %v", name, absent)
		}
	}
	if absent["code"] != "" || absent["ref"] != "" {
		t.Errorf("a field the catalog has is among those it has not: %v", absent)
	}
}
