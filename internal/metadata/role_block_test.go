package metadata

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// TestEveryRightMLDoesNotKeepIsReadForStudioToRepair catches a right of the
// prototype ML does not keep that falls out of the list of rights it drops:
// a role naming it is then refused as it is read, and Studio cannot open the
// role to take the right away (ACCESS-RIGHTS, «Роли, сохранённые прежней
// версией ML»). The list it is held to is the test's own, rightsNotKept; the
// rights some right of ML stands for are written out here.
func TestEveryRightMLDoesNotKeepIsReadForStudioToRepair(t *testing.T) {
	t.Parallel()
	replaced := map[string]PermissionOperation{"ThinClient": PermissionRunApplication, "WebClient": PermissionRunApplication,
		"TechnicalSpecialistMode": PermissionDeveloperTools}
	dropped := map[string]droppedRight{}
	for _, right := range droppedRights {
		dropped[right.english] = right
	}
	for _, name := range append(slices.Clone(rightsNotKept), "ThinClient", "WebClient", "TechnicalSpecialistMode") {
		right, ok := dropped[name]
		if !ok {
			t.Errorf("%s is not among the rights ML drops: a role naming it would not open in Studio", name)
			continue
		}
		if right.instead != replaced[name] {
			t.Errorf("%s stands for %q in ML, want %q", name, right.instead, replaced[name])
		}
		role := roleFixture()
		role.Objects[0].Operations = append(role.Objects[0].Operations, right.operation)
		if err := ValidateRole("role.yaml", role, metadataConfiguration()); err != nil {
			t.Errorf("a role naming %s is not read for Studio to repair: %v", name, err)
		}
		if err := validateDroppedRights(role); err == nil || !strings.Contains(err.Error(), string(right.operation)) {
			t.Errorf("a role naming %s is run: %v", name, err)
		}
	}
	if len(dropped) != len(rightsNotKept)+len(replaced) {
		t.Errorf("ML drops %d rights, the test names %d", len(dropped), len(rightsNotKept)+len(replaced))
	}
}

// TestARoleThatStopsGivingRightsLeavesNoGrid catches WriteRole leaving the
// grid of a role that gives no right any longer: the loader would read the
// old rights back from rights.json beside the description.
func TestARoleThatStopsGivingRightsLeavesNoGrid(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	role := roleFixture()
	if err := WriteRole(root, role); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(root, "metadata", string(RoleKind), role.ID.String())
	if _, err := os.Stat(filepath.Join(folder, RoleRightsFile)); err != nil {
		t.Fatalf("a role giving rights keeps no grid: %v", err)
	}
	role.Objects = nil
	if err := WriteRole(root, role); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(folder); !os.IsNotExist(err) {
		t.Fatalf("the folder of a role giving no right is left: %v", err)
	}
}

// TestARoleGivesNoRightOnATemplateOfAnHTTPService catches a template of an
// HTTP service taken for a part a role gives rights on: the exports give
// the use of its methods and nothing on a template (partGroups), so a role
// naming one is refused as naming a part the object does not have.
func TestARoleGivesNoRightOnATemplateOfAnHTTPService(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	const template, method = "f4000000-0000-4000-8000-000000000030", "f4000000-0000-4000-8000-000000000031"
	writeHTTPService(t, root, httpServiceID, "Биллинг", `root_url: billing
templates:
  - id: `+template+`
    name: Версия
    title: {ru: Версия}
    template: /version
    methods:
      - {id: `+method+`, name: Получить, title: {ru: Получить}, method: GET, handler: ВерсияПолучить}
`)
	if err := os.WriteFile(filepath.Join(root, "metadata", string(HTTPServiceKind), "Биллинг", "МодульСервиса.bsl"),
		[]byte("Функция ВерсияПолучить(Запрос)\n\tВозврат Неопределено;\nКонецФункции\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service, err := uuid.Parse(httpServiceID)
	if err != nil {
		t.Fatal(err)
	}
	role := roleFixture()
	role.Objects = []ObjectPermission{{Object: service, Fields: []FieldPermission{{Field: method, Operations: []PermissionOperation{PermissionUse}}}}}
	if err := WriteRole(root, role); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err != nil {
		t.Fatalf("the use of a method was refused: %v", err)
	}
	role.Objects[0].Fields = append(role.Objects[0].Fields, FieldPermission{Field: template, Operations: []PermissionOperation{PermissionUse}})
	if err := WriteRole(root, role); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "field "+template+" does not belong") {
		t.Fatalf("a right on a template was taken: %v", err)
	}
}
