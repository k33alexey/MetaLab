package metadata

import (
	"errors"
	"strings"
	"testing"
)

// unresolvedOf loads a project that refers to objects it does not have: the
// strict load must refuse it naming every such place, and the reading for
// editing must open it and list them. It returns the list.
func unresolvedOf(t *testing.T, root string) []UnresolvedReference {
	t.Helper()
	_, strictErr := Load(root)
	if !errors.Is(strictErr, ErrUnresolvedReference) {
		t.Fatalf("the strict load: %v", strictErr)
	}
	catalog, err := read(root, true, false)
	if err != nil {
		t.Fatalf("the reading for editing refused the project: %v", err)
	}
	unresolved := catalog.UnresolvedReferences()
	if len(unresolved) == 0 {
		t.Fatalf("the strict load refused %v, and the reading for editing found nothing", strictErr)
	}
	for _, item := range unresolved {
		if !strings.Contains(strictErr.Error(), item.Where+" "+item.ID.String()) {
			t.Errorf("the refusal does not name %s %s: %v", item.Where, item.ID, strictErr)
		}
	}
	return unresolved
}

// The role editor reads a project for editing: one that refers to an object
// it does not have opens there, while the strict load refuses it.
//
// Defect caught: the role editor reading strictly, so that a project a
// colleague broke by deleting an object cannot be opened where it is fixed.
func TestTheRoleEditorOpensAProjectWithAReferenceToNothing(t *testing.T) {
	t.Parallel()
	root := compositeProject(t)
	breakReference(t, root, SubsystemKind, "Склад", cmpReport)
	if _, err := Load(root); !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("the strict load: %v", err)
	}
	if _, err := LoadPermissionSchema(root); err != nil {
		t.Fatalf("the role editor refused the project: %v", err)
	}
}
