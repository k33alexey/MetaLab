package metadata

import (
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

// The editor gets the rules of restricting with its schema and offers by
// them, so they must be what saving keeps: by the help only reading is
// restricted by fields - one restriction per set of fields and one on
// «Прочие поля» - and adding, changing and deleting take one each, with no
// fields. The expectation is written out here, not read from
// restrictionRules, and each rule the schema sends is put to the check of a
// saved role. Defect caught: the editor offering fields on a right, or two
// restrictions of it, that saving refuses, or refusing what saving keeps.
func TestRestrictionRulesOfTheSchemaAreWhatSavingKeeps(t *testing.T) {
	t.Parallel()
	schema, err := LoadPermissionSchema(compositeProject(t))
	if err != nil {
		t.Fatal(err)
	}
	want := []RestrictionRule{{Right: PermissionRead, ByFields: true}, {Right: PermissionInsert}, {Right: PermissionUpdate}, {Right: PermissionDelete}}
	if !slices.Equal(schema.RestrictionRules, want) {
		t.Fatalf("rules %v, want %v", schema.RestrictionRules, want)
	}
	all := []PermissionOperation{PermissionRead, PermissionInsert, PermissionUpdate, PermissionDelete, PermissionView}
	for _, rule := range schema.RestrictionRules {
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
