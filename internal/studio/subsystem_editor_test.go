package studio

import (
	"errors"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
)

// A subsystem is made under another and moved as the prototype makes and
// moves it (podsistema.md, «Новая подсистема», «Проверки», «Команды»): a
// child of «Подсистема1» is «Подсистема1» again, the next child
// «Подсистема2»; moved to the root, «Подсистема2» goes, «Подсистема1» - a
// name the root has - is refused and stays; a subsystem moved under its own
// child is refused; a move over a subsystem changed since it was read is
// refused - and the project loads after every refusal.
// Defects caught: a refused move written all the same; a child named among every subsystem and not its siblings;
// a move taking a name its new siblings have, or making a subsystem its own
// parent; a move over a description changed meanwhile.
func TestASubsystemIsMadeUnderAnotherAndMoved(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	root, err := workspace.CreateSubsystem("")
	if err != nil {
		t.Fatal(err)
	}
	first, err := workspace.CreateSubsystemChild(SubsystemChild{Parent: root.Subsystem.ID})
	if err != nil {
		t.Fatal(err)
	}
	second, err := workspace.CreateSubsystemChild(SubsystemChild{Parent: root.Subsystem.ID})
	if err != nil {
		t.Fatal(err)
	}
	if root.Subsystem.Name != "Подсистема1" || first.Subsystem.Name != "Подсистема1" || second.Subsystem.Name != "Подсистема2" ||
		first.Subsystem.Parent == nil || *first.Subsystem.Parent != root.Subsystem.ID || !first.Subsystem.IncludeInCommandInterface {
		t.Fatalf("made: %+v, %+v, %+v", root.Subsystem, first.Subsystem, second.Subsystem)
	}
	moved, err := workspace.MoveSubsystem(SubsystemMove{Path: second.Path, Revision: second.Revision})
	if err != nil || moved.Subsystem.Parent != nil {
		t.Fatalf("moved to the root: %+v, %v", moved.Subsystem, err)
	}
	if _, err := workspace.MoveSubsystem(SubsystemMove{Path: first.Path, Revision: first.Revision}); err == nil {
		t.Error("a name the root has was taken")
	}
	if again, _ := workspace.ReadSubsystemEditor(first.Path); again.Subsystem.Parent == nil {
		t.Error("a refused move moved the subsystem")
	}
	child := first.Subsystem.ID
	opened, _ := workspace.ReadSubsystemEditor(root.Path)
	if _, err := workspace.MoveSubsystem(SubsystemMove{Path: root.Path, Revision: opened.Revision, Parent: &child}); err == nil {
		t.Error("a subsystem was moved under its own child")
	}
	if _, err := metadata.Load(workspace.root); err != nil {
		t.Errorf("a refused move left the project broken: %v", err)
	}
	if _, err := workspace.MoveSubsystem(SubsystemMove{Path: moved.Path, Revision: second.Revision}); !errors.Is(err, ErrSourceChanged) {
		t.Errorf("a move over a changed subsystem: %v", err)
	}
}
