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

// TestTheFieldsOfARestrictionOfACatalogAreThoseOfItsTable holds the fields
// the window of a restriction lists for a catalog to the prototype's (pass of
// 22.10.2026 on ERP's Валюты and Бригады, block-00/vse-ogranicheniya-
// dostupa.md; ACCESS-RIGHTS.md, «Состав и порядок полей ограничения») and to
// the help of the table of a query (Родитель - only of a multi-level
// catalog, Владелец - only of a subordinate one); written out here, not read
// off the sets of standard attributes. Defects caught (2.437, case 2в-23):
// every catalog offered Родитель, ЭтоГруппа, Владелец and Код whatever it
// was; the version of the data stood first and the predefined fields before
// the attributes. ЭтотОбъект keeps every field: the help of СправочникОбъект
// makes none of them hang on the catalog.
func TestTheFieldsOfARestrictionOfACatalogAreThoseOfItsTable(t *testing.T) {
	t.Parallel()
	attribute := func(name string) Attribute {
		return Attribute{ID: uuid.MustNew(), Name: name, Title: LocalizedText{"ru": name}, Types: []Type{{Kind: StringType, Length: 10}}}
	}
	currencies := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Валюты", Title: LocalizedText{"ru": "Валюты"},
		Code: CatalogCode{Type: StringType, Length: 3}, DescriptionLength: 50, Attributes: []Attribute{attribute("Наценка")}}
	brigades := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Бригады", Title: LocalizedText{"ru": "Бригады"},
		Code: CatalogCode{Type: StringType}, DescriptionLength: 50, Hierarchy: Hierarchy{Enabled: true, Kind: FoldersAndItemsHierarchy},
		Attributes: []Attribute{attribute("Мастер")},
		TableParts: []TablePart{{ID: uuid.MustNew(), Name: "Состав", Title: LocalizedText{"ru": "Состав"}, Attributes: []Attribute{attribute("Сотрудник")}}}}
	accounts := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Счета", Title: LocalizedText{"ru": "Счета"},
		Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 50, Owners: []uuid.UUID{currencies.ID}}
	catalog, err := NewCatalogSnapshotWithRoles(metadataConfiguration(), nil, nil, nil, []CatalogDefinition{currencies, brigades, accounts}, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The fields of a table part stand under it, in an order the pass did
	// not look at: they are named after it, sorted.
	names := func(definition CatalogDefinition) []string {
		var result, inside []string
		for _, field := range catalog.restrictionFieldsOf(CatalogKind, definition.ID) {
			if field.parent != "" {
				inside = append(inside, field.name)
				continue
			}
			if len(inside) > 0 {
				slices.Sort(inside)
				result, inside = append(result, inside...), nil
			}
			result = append(result, field.name)
		}
		return result
	}
	for _, check := range []struct {
		definition CatalogDefinition
		want       []string
	}{
		{currencies, []string{"Ссылка", "ВерсияДанных", "ПометкаУдаления", "Код", "Наименование", "Наценка", "Предопределенный", "ИмяПредопределенныхДанных"}},
		{brigades, []string{"Ссылка", "ВерсияДанных", "ПометкаУдаления", "Родитель", "ЭтоГруппа", "Наименование", "Мастер", "Состав", "НомерСтроки",
			"Сотрудник", "Предопределенный", "ИмяПредопределенныхДанных"}},
		{accounts, []string{"Ссылка", "ВерсияДанных", "ПометкаУдаления", "Владелец", "Код", "Наименование", "Предопределенный", "ИмяПредопределенныхДанных"}},
	} {
		if got := names(check.definition); !slices.Equal(got, check.want) {
			t.Errorf("a restriction of %s names %v, want %v", check.definition.Name, got, check.want)
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
