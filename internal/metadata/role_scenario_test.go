package metadata

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// TestARoleOfEveryStateGoesThroughTheLoadAsItWasWritten is the block's
// role taken through the mechanisms of its iterations at once: the rights of
// the configuration ML keeps and its own (2.352, 2.421), the rights of a
// document granted and denied, a right on a field and restrictions of three
// sorts (2.276-2.294, 2.306), a template (2.293), the command of an
// enumeration and a dimension of a cube, which no export proves (2.416,
// 2.423, 2.426), and a right ML no longer keeps (2.357). Written as Studio
// writes it, the role is loaded as it was written, with one note for the
// enumeration and one for the right on the cube; with the right ML does not
// keep it is refused, naming the role and the right. Defects caught: a state
// lost between the two files of a role, a note doubled or missing, a role
// with a right ML drops run.
func TestARoleOfEveryStateGoesThroughTheLoadAsItWasWritten(t *testing.T) {
	t.Parallel()
	root := compositeProject(t)
	project, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	id := func(text string) uuid.UUID {
		value, err := uuid.Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	role := RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Полная", Title: LocalizedText{"ru": "Полная"},
		GrantNewObjectsByDefault: true, IndependentChildRights: true,
		RestrictionTemplates: []RestrictionTemplate{{Name: "Склад(Поле)", Condition: "ГДЕ #Поле = &Склад"}},
		Objects: []ObjectPermission{
			{Object: project.Project.ID, Operations: []PermissionOperation{PermissionAdministration, PermissionDeveloperTools, PermissionChangeHistory},
				Denied: []PermissionOperation{PermissionRunApplication}},
			{Object: id(cmpDocument), Operations: []PermissionOperation{PermissionRead, PermissionUpdate, PermissionPosting},
				Denied: []PermissionOperation{PermissionInteractiveDelete},
				Fields: []FieldPermission{{Field: cmpDocumentWarehouse, Operations: []PermissionOperation{PermissionView}, Denied: []PermissionOperation{PermissionEdit}}},
				Restrictions: []RightRestriction{{Right: PermissionRead, Fields: []string{cmpDocumentWarehouse}, Condition: "ГДЕ #Склад(\"Склад\")"},
					{Right: PermissionRead, Condition: "ГДЕ ИСТИНА"}, {Right: PermissionUpdate, Condition: "ГДЕ ЛОЖЬ"}}},
			{Object: id(cmpEnumeration), Fields: []FieldPermission{{Field: cmpEnumCommand, Operations: []PermissionOperation{PermissionView}}}},
			{Object: id(salesCube), Operations: []PermissionOperation{PermissionRead},
				Fields: []FieldPermission{{Field: goodsDimension, Operations: []PermissionOperation{PermissionView}}}},
		}}
	if err := WriteRole(root, role); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("the role was refused: %v", err)
	}
	read, ok := loaded.Role("полная")
	if !ok {
		t.Fatal("the role is not found by its name in another case")
	}
	// Against the role written, not against its own second writing: a state
	// lost on the way out would be lost from both writings alike.
	if !reflect.DeepEqual(read, role) {
		t.Fatalf("the role came back otherwise:\nwrote %+v\nread  %+v", role, read)
	}
	want := []Note{
		{Kind: NoteUseWithoutSample, Where: "roles Полная objects", Written: "enumerations ВидыТоваров"},
		{Kind: NoteRightWithoutSample, Where: "roles Полная rights", Written: "external-data-source-cubes Склад.Продажи.Товар view"},
	}
	var notes []Note
	for _, note := range loaded.Notes() {
		if strings.HasPrefix(note.Where, "roles Полная") {
			notes = append(notes, note)
		}
	}
	slices.SortFunc(notes, func(a, b Note) int { return strings.Compare(string(a.Kind), string(b.Kind)) })
	slices.SortFunc(want, func(a, b Note) int { return strings.Compare(string(a.Kind), string(b.Kind)) })
	if !slices.Equal(notes, want) {
		t.Errorf("the role is noted %+v, want %+v", notes, want)
	}
	for _, dropped := range []struct {
		denied bool
		says   string
	}{{false, "role Полная gives the right thin-client"}, {true, "role Полная denies the right thin-client"}} {
		changed := cloneRole(role)
		if dropped.denied {
			changed.Objects[0].Denied = append(changed.Objects[0].Denied, "thin-client")
		} else {
			changed.Objects[0].Operations = append(changed.Objects[0].Operations, "thin-client")
		}
		if err := WriteRole(root, changed); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(root); err == nil || !strings.Contains(err.Error(), dropped.says) {
			t.Errorf("a role with a right ML does not keep: %v", err)
		}
	}
}
