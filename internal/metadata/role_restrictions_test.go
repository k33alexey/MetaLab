package metadata

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// restrictedProject is a project with a document holding a table part and a
// constant, and the role given, read back as a catalog.
func restrictedProject(t *testing.T, role func(document, part, column, constant uuid.UUID) RoleDefinition) (*Catalog, error) {
	t.Helper()
	root := metadataProject(t)
	document, part, column, constant := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	writeMetadata(t, root, DocumentKind, document.String(), "format: 1\nid: "+document.String()+"\nname: Продажа\ntitle: {ru: Продажа}\n"+
		"number: {type: string, length: 9, periodicity: none}\n"+
		"table_parts:\n  - id: "+part.String()+"\n    name: Товары\n    title: {ru: Товары}\n    attributes:\n"+
		"      - id: "+column.String()+"\n        name: Цена\n        title: {ru: Цена}\n        types: [{kind: number, precision: 10}]\n")
	writeMetadata(t, root, ConstantKind, constant.String(), "format: 1\nid: "+constant.String()+"\nname: Режим\ntitle: {ru: Режим}\ntypes: [{kind: boolean}]\n")
	if err := WriteRole(root, role(document, part, column, constant)); err != nil {
		return nil, err
	}
	return Load(root)
}

// What a restriction names is the object's: the rights restricted are the
// ones the configurator offers on it and the role writes, among reading,
// adding, changing and deleting; the fields are those of its table, the line
// number of a table part among them. A restriction the exports never show is
// accepted and noted, and so is a field the object does not have, as the
// prototype keeps one (ACCESS-RIGHTS.md, «Ограничение по полю, которого у
// объекта нет»): the parent of a document, the line number of no table part.
//
// Defect caught: a restriction of a right the object has not, of one the role
// does not write or of one the prototype does not restrict read as a role; a
// field the prototype writes (ОС.LineNumber) refused; a restriction on a kind
// no export restricts, an empty one on the other fields, an empty template
// or a field the object does not have accepted silently; such a field
// refused, as 2.440 refused it; a field of no name accepted.
func TestRestrictionsFollowTheObject(t *testing.T) {
	t.Parallel()
	role := func(objects func(document, part, column, constant uuid.UUID) []ObjectPermission, templates []RestrictionTemplate) func(document, part, column, constant uuid.UUID) RoleDefinition {
		return func(document, part, column, constant uuid.UUID) RoleDefinition {
			return RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Кладовщик", Title: LocalizedText{"ru": "Кладовщик"},
				Objects: objects(document, part, column, constant), RestrictionTemplates: templates}
		}
	}
	read := []PermissionOperation{PermissionRead, PermissionView}
	catalog, err := restrictedProject(t, role(func(document, part, column, constant uuid.UUID) []ObjectPermission {
		return []ObjectPermission{
			{Object: document, Operations: read, Restrictions: []RightRestriction{
				{Right: PermissionRead, Fields: []string{"ref", "dataversion", part.String() + ".linenumber", column.String()}},
				{Right: PermissionRead},
				{Right: PermissionRead, Fields: []string{"parent", column.String() + ".linenumber"}, Condition: "ГДЕ ИСТИНА"}}},
			{Object: constant, Operations: []PermissionOperation{PermissionRead},
				Restrictions: []RightRestriction{{Right: PermissionRead, Condition: "ГДЕ ИСТИНА"}}},
		}
	}, []RestrictionTemplate{{Name: "Пустой"}, {Name: "Полный(Поле)", Condition: "ГДЕ #Параметр(1)"}}))
	if err != nil {
		t.Fatalf("a role of the prototype was refused: %v", err)
	}
	var noted []string
	for _, note := range catalog.Notes() {
		if note.Kind == NoteRestrictionWithoutSample {
			noted = append(noted, note.Where+": "+note.Written)
		}
	}
	slices.Sort(noted)
	want := []string{
		"roles Кладовщик restriction-templates: Пустой",
		"roles Кладовщик restrictions: constants Режим read",
		"roles Кладовщик restrictions: documents Продажа read without a condition",
	}
	if !slices.Equal(noted, want) {
		t.Fatalf("notes:\n%s\nwant:\n%s", strings.Join(noted, "\n"), strings.Join(want, "\n"))
	}
	var absent []string
	for _, note := range catalog.Notes() {
		if note.Kind == NoteRestrictionAbsentField {
			absent = append(absent, note.Where+": "+note.Written)
		}
	}
	slices.Sort(absent)
	column := catalog.Documents[0].TableParts[0].Attributes[0].ID
	if want := []string{
		"roles Кладовщик restrictions: documents Продажа read " + column.String() + ".linenumber",
		"roles Кладовщик restrictions: documents Продажа read parent",
	}; !slices.Equal(absent, want) {
		t.Fatalf("notes of fields the object does not have:\n%s\nwant:\n%s", strings.Join(absent, "\n"), strings.Join(want, "\n"))
	}
	for name, objects := range map[string]func(document, part, column, constant uuid.UUID) []ObjectPermission{
		"a right the object is not offered": func(document, part, column, constant uuid.UUID) []ObjectPermission {
			return []ObjectPermission{{Object: constant, Operations: []PermissionOperation{PermissionRead, PermissionDelete},
				Restrictions: []RightRestriction{{Right: PermissionDelete, Condition: "ГДЕ ЛОЖЬ"}}}}
		},
		"a right the role does not write": func(document, part, column, constant uuid.UUID) []ObjectPermission {
			return []ObjectPermission{{Object: document, Operations: read,
				Restrictions: []RightRestriction{{Right: PermissionUpdate, Condition: "ГДЕ ЛОЖЬ"}}}}
		},
		"a right the prototype does not restrict": func(document, part, column, constant uuid.UUID) []ObjectPermission {
			return []ObjectPermission{{Object: document, Operations: read,
				Restrictions: []RightRestriction{{Right: PermissionView, Condition: "ГДЕ ЛОЖЬ"}}}}
		},
		"a field of no name": func(document, part, column, constant uuid.UUID) []ObjectPermission {
			return []ObjectPermission{{Object: document, Operations: read,
				Restrictions: []RightRestriction{{Right: PermissionRead, Fields: []string{""}}}}}
		},
	} {
		if _, err := restrictedProject(t, role(objects, nil)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// The editor gets with its schema how each object is restricted and offers
// by it, so it must be what saving keeps: by the help only reading is
// restricted by fields - one restriction per set of fields and one on
// «Прочие поля» - and adding, changing and deleting take one each, with no
// fields. The expectation is written out here for a document, not read from
// restrictionRules, and each right the schema sends is put to the check of
// a saved role. Defect caught: the editor offering fields on a right, or two
// restrictions of it, that saving refuses, or refusing what saving keeps.
func TestRestrictionRulesOfTheSchemaAreWhatSavingKeeps(t *testing.T) {
	t.Parallel()
	schema, err := LoadPermissionSchema(compositeProject(t))
	if err != nil {
		t.Fatal(err)
	}
	index := slices.IndexFunc(schema.Objects, func(object PermissionObject) bool { return object.ID.String() == cmpDocument })
	if index < 0 {
		t.Fatal("the document is not in the schema")
	}
	document := schema.Objects[index]
	want := []PermissionOperation{PermissionRead, PermissionInsert, PermissionUpdate, PermissionDelete}
	if !slices.Equal(document.RestrictedRights, want) || !slices.Equal(document.RestrictedByFields, want[:1]) {
		t.Fatalf("a document is restricted on %v, by fields on %v; want %v and %v", document.RestrictedRights, document.RestrictedByFields, want, want[:1])
	}
	all := []PermissionOperation{PermissionRead, PermissionInsert, PermissionUpdate, PermissionDelete, PermissionView}
	for _, right := range document.RestrictedRights {
		rule := RestrictionRule{Right: right, ByFields: slices.Contains(document.RestrictedByFields, right)}
		_, err := restrictedProject(t, func(document, part, column, constant uuid.UUID) RoleDefinition {
			return RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Кладовщик", Title: LocalizedText{"ru": "Кладовщик"},
				Objects: []ObjectPermission{{Object: document, Operations: all, Restrictions: []RightRestriction{
					{Right: rule.Right, Fields: []string{"ref"}, Condition: "ГДЕ 1"},
					{Right: rule.Right, Fields: []string{column.String()}, Condition: "ГДЕ 2"},
					{Right: rule.Right, Condition: "ГДЕ 3"}}}}}
		})
		if rule.ByFields && err != nil {
			t.Errorf("%s by fields was refused: %v", rule.Right, err)
		}
		if !rule.ByFields && err == nil {
			t.Errorf("%s took fields and three restrictions", rule.Right)
		}
		_, err = restrictedProject(t, func(document, part, column, constant uuid.UUID) RoleDefinition {
			return RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Кладовщик", Title: LocalizedText{"ru": "Кладовщик"},
				Objects: []ObjectPermission{{Object: document, Operations: all, Restrictions: []RightRestriction{
					{Right: rule.Right, Condition: "ГДЕ 1"}, {Right: rule.Right, Condition: "ГДЕ 2"}}}}}
		})
		if err == nil {
			t.Errorf("%s took two restrictions on «Прочие поля»", rule.Right)
		}
	}
}

// TestTheWindowOfARoleRestrictsTheKindsTheWizardOmitsAsThePrototype holds
// what the window of a role lets restrict on the kinds the wizard does not
// offer to the prototype's (pass of 22.10.2026, the eighth, rol-*;
// ACCESS-RIGHTS.md, «Ограничение у видов, которых мастер не предлагает»):
// a constant - nothing; a sequence and a recalculation - «Чтение» alone,
// with no fields; a table of an external source - «Чтение», «Добавление»,
// «Изменение», «Удаление», reading by fields as a catalog's. The
// expectations are written out here, not read off kindUsesOf. A role loaded
// with what the window does not offer - the reading of a constant, the
// changing of a sequence - is accepted and noted, as the iron rule says.
// Defects caught: a constant offered its reading and changing to restrict;
// a sequence and a recalculation offered changing, and reading by fields;
// one list of rules for every kind (until 2.458).
func TestTheWindowOfARoleRestrictsTheKindsTheWizardOmitsAsThePrototype(t *testing.T) {
	t.Parallel()
	root := compositeProject(t)
	read := []PermissionOperation{PermissionRead}
	want := map[string][2][]PermissionOperation{
		cmpConstant:      {{}, {}},
		cmpSequence:      {read, {}},
		cmpRecalculation: {read, {}},
		goodsTable:       {{PermissionRead, PermissionInsert, PermissionUpdate, PermissionDelete}, read},
		cmpGoods:         {{PermissionRead, PermissionInsert, PermissionUpdate, PermissionDelete}, read},
	}
	schema, err := LoadPermissionSchema(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, object := range schema.Objects {
		expected, ok := want[object.ID.String()]
		if !ok {
			continue
		}
		if !slices.Equal(object.RestrictedRights, expected[0]) || !slices.Equal(object.RestrictedByFields, expected[1]) {
			t.Errorf("the window of a role restricts %s %s on %v, by fields on %v; want %v and %v", object.Kind, object.Name,
				object.RestrictedRights, object.RestrictedByFields, expected[0], expected[1])
		}
		if len(object.RestrictedByFields) == 0 && len(object.RestrictionFields)+len(object.RoleWindowFields) != 0 {
			t.Errorf("%s %s offers fields with no right restricted by them: %v", object.Kind, object.Name, object.RestrictionFields)
		}
		delete(want, object.ID.String())
	}
	if len(want) != 0 {
		t.Errorf("the schema has no %v", slices.Collect(maps.Keys(want)))
	}
	id := func(text string) uuid.UUID {
		value, err := uuid.Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	if err := WriteRole(root, RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Учетчик", Title: LocalizedText{"ru": "Учетчик"},
		Objects: []ObjectPermission{
			{Object: id(cmpConstant), Operations: read, Restrictions: []RightRestriction{{Right: PermissionRead, Condition: "ГДЕ 1"}}},
			{Object: id(cmpSequence), Operations: []PermissionOperation{PermissionUpdate},
				Restrictions: []RightRestriction{{Right: PermissionUpdate, Condition: "ГДЕ 2"}}}}}); err != nil {
		t.Fatal(err)
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a role restricting what the window does not offer was refused: %v", err)
	}
	var noted []string
	for _, note := range catalog.Notes() {
		if note.Kind == NoteRestrictionWithoutSample && strings.Contains(note.Where, "Учетчик") {
			noted = append(noted, note.Written)
		}
	}
	slices.Sort(noted)
	if expected := []string{"constants ОсновнойСклад read", "sequences Последовательность update"}; !slices.Equal(noted, expected) {
		t.Errorf("notes %v, want %v", noted, expected)
	}
}
