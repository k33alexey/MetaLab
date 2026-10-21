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

// TestThePartsOfAKindNoExportGivesRightsOnAreOfferedNone catches the window
// of a role offering rights on the parts of a kind whose parts no export
// gives rights on (Rights.xml of every role, 13.10.2026): the commands of a
// filter criterion are carried with a note and not offered. Where the pass in
// the configurator saw the window offer them - the commands of an
// enumeration, the dimensions of a cube - it offers what it saw.
func TestThePartsOfAKindNoExportGivesRightsOnAreOfferedNone(t *testing.T) {
	t.Parallel()
	for _, item := range []struct {
		kind  Kind
		group PartGroup
		want  []PermissionOperation
	}{
		{FilterCriterionKind, CommandsPart, nil},
		{ExternalDimensionTableKind, FieldsPart, nil},
		{EnumerationKind, CommandsPart, []PermissionOperation{PermissionView}},
		{ExternalCubeKind, DimensionsPart, []PermissionOperation{PermissionView}},
		{CatalogKind, CommandsPart, []PermissionOperation{PermissionView}},
	} {
		if got := offeredPartRights(item.kind, item.group); !slices.Equal(got, item.want) {
			t.Errorf("%s offers %v on its %s, want %v", item.kind, got, item.group, item.want)
		}
	}
}
