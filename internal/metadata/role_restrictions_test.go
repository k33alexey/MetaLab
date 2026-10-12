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
// accepted and noted.
//
// Defect caught: a restriction of a right the object has not, of one the role
// does not write or of one the prototype does not restrict read as a role; a
// field the prototype writes (ОС.LineNumber) refused; a restriction on a kind
// no export restricts, an empty one on the other fields or an empty template
// accepted silently.
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
				{Right: PermissionRead}}},
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
		"a field of no table part": func(document, part, column, constant uuid.UUID) []ObjectPermission {
			return []ObjectPermission{{Object: document, Operations: read,
				Restrictions: []RightRestriction{{Right: PermissionRead, Fields: []string{column.String() + ".linenumber"}}}}}
		},
	} {
		if _, err := restrictedProject(t, role(objects, nil)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}
