package studio

import "testing"

// The main section offers the commands of every object a section may hold,
// in a project with roles: a role keeps its rights on commands in a field
// named as an object's own commands, and was once read as one. Defect
// caught: the window not opening - the server panicking - on a project with
// a role.
func TestTheMainSectionReadsAProjectWithRoles(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	addRestrictedRole(t, workspace, root)
	window, err := workspace.ReadMainSection()
	if err != nil {
		t.Fatal(err)
	}
	if len(window.Roles) != 1 || len(window.Available) == 0 {
		t.Errorf("the main section read %d roles and %d branches", len(window.Roles), len(window.Available))
	}
}
