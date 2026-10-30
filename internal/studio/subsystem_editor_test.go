package studio

import (
	"errors"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
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

// automaticSubsections takes back the order of «Подразделы» that making
// subsystems under the subsystem described at relative wrote, for a test of
// the automatic order.
func automaticSubsections(t *testing.T, workspace *Workspace, relative string) {
	t.Helper()
	opened, err := workspace.ReadSubsystemEditor(relative)
	if err != nil {
		t.Fatal(err)
	}
	value := opened.Subsystem
	value.CommandInterface.SubsystemsOrder = nil
	if _, err := workspace.SaveSubsystemWindow(relative, SubsystemWindowValue{SubsystemDefinition: value}, opened.Revision); err != nil {
		t.Fatal(err)
	}
}

// A subsystem made under another by «Добавить ▸ Подчиненная подсистема»
// goes at the end of the order of «Подразделы» of its parent, which shows
// «(ручной порядок)» at once (podsistema.md, «Досъёмка 28.10.2026, ночь»;
// snapshot dosyomka-2639/01): the first ones in the automatic order of
// those there - the alphabet of their presentations, not the order they
// were made in - then the new one; a parent ordered by hand keeps its order
// and gets the new one last. Defects caught: the parent left of no order;
// the new one put among the others by the alphabet; the subsystems there
// written in the order they were made; a manual order replaced.
func TestASubsystemMadeUnderAnotherGoesLastInItsSubsections(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	parent, err := workspace.CreateSubsystem("Склад")
	if err != nil {
		t.Fatal(err)
	}
	order := func() []string {
		t.Helper()
		opened, err := workspace.ReadSubsystemEditor(parent.Path)
		if err != nil {
			t.Fatal(err)
		}
		names := map[uuid.UUID]string{}
		loaded, err := metadata.Load(workspace.root)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range loaded.Subsystems {
			names[item.ID] = item.Name
		}
		var result []string
		for _, id := range opened.Subsystem.CommandInterface.SubsystemsOrder {
			result = append(result, names[id])
		}
		return result
	}
	child := func(name, title string) {
		t.Helper()
		made, err := workspace.CreateSubsystemChild(SubsystemChild{Parent: parent.Subsystem.ID})
		if err != nil {
			t.Fatal(err)
		}
		value := made.Subsystem
		value.Name, value.Title = name, metadata.LocalizedText{"ru": title}
		if _, err := workspace.SaveSubsystemWindow(made.Path, SubsystemWindowValue{SubsystemDefinition: value}, made.Revision); err != nil {
			t.Fatal(err)
		}
	}
	child("Я", "Яблоки")
	if got := order(); !slices.Equal(got, []string{"Я"}) {
		t.Errorf("the first subsystem under it ordered %q", got)
	}
	child("А", "апельсины")
	if got := order(); !slices.Equal(got, []string{"Я", "А"}) {
		t.Errorf("the second subsystem under it ordered %q, want it after the first", got)
	}
	// Of no order, those there in the alphabet of their presentations -
	// «апельсины» before «Яблоки», though made after, a small letter
	// before a capital one - then the new one.
	automaticSubsections(t, workspace, parent.Path)
	child("Б", "Бананы")
	if got := order(); !slices.Equal(got, []string{"А", "Я", "Б"}) {
		t.Errorf("the subsections are ordered %q, want А, Я and the new one", got)
	}
	child("В", "Вишни")
	if got := order(); !slices.Equal(got, []string{"А", "Я", "Б", "В"}) {
		t.Errorf("a manual order became %q, want it kept and the new one last", got)
	}
}
