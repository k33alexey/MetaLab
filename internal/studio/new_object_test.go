package studio

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
)

// TestEveryKindIsMadeByOneWay catches «Добавить» that makes some kinds and not
// others, or makes one by a way of its own: a common module, a group of
// commands, a common command and an enumeration are made by the path a
// catalog, a constant and a role are, numbered in their branch, laid where
// their kind keeps a description - by identifier or in a folder named after
// the object - and the project with them loads; a common module comes with
// its text, empty. A kind Studio does not make yet is refused with nothing
// left on disk.
func TestEveryKindIsMadeByOneWay(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	// The name and where the description lies, written out from the layout of
	// the project: a folder for a catalog, a constant and an enumeration, one
	// file for the rest.
	cases := []struct {
		kind   metadata.Kind
		name   string
		folder bool
	}{
		{metadata.CatalogKind, "Справочник1", true},
		{metadata.ConstantKind, "Константа1", true},
		{metadata.RoleKind, "Роль1", false},
		{metadata.CommonModuleKind, "ОбщийМодуль1", false},
		{metadata.CommandGroupKind, "ГруппаКоманд1", false},
		{metadata.EnumerationKind, "Перечисление1", true},
	}
	for _, item := range cases {
		relative, err := workspace.CreateObject(item.kind, "")
		if err != nil {
			t.Fatalf("«Добавить» %s: %v", item.kind, err)
		}
		want := "metadata/" + string(item.kind) + "/" + item.name + "/object.yaml"
		if !item.folder {
			want = "metadata/" + string(item.kind) + "/" + path.Base(relative)
		}
		if relative != want || path.Dir(relative) == "." {
			t.Errorf("the new %s lies at %s, want %s", item.kind, relative, want)
		}
	}
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		t.Fatalf("the project with the new objects does not load: %v", err)
	}
	for _, item := range cases {
		if !slices.ContainsFunc(loaded.KindObjects(item.kind), func(object metadata.ReferenceTypeObject) bool { return object.Name == item.name }) {
			t.Errorf("the project has no %s %s", item.kind, item.name)
		}
	}
	for _, module := range loaded.CommonModules {
		if module.Name != "ОбщийМодуль1" {
			continue
		}
		text, err := project.ModulePath(module.Module)
		if err != nil {
			t.Fatal(err)
		}
		if content, err := os.ReadFile(filepath.Join(workspace.root, filepath.FromSlash(text))); err != nil || len(content) != 0 {
			t.Errorf("the text of the new common module is %q (%v), want an empty file", content, err)
		}
	}
	// The second of a kind is the next number, and stands last.
	if _, err := workspace.CreateObject(metadata.CommonModuleKind, ""); err != nil {
		t.Fatal(err)
	}
	again, err := metadata.Load(workspace.root)
	if err != nil {
		t.Fatal(err)
	}
	if modules := again.KindObjects(metadata.CommonModuleKind); modules[len(modules)-1].Name != "ОбщийМодуль2" {
		t.Errorf("the last common module is %s, want ОбщийМодуль2", modules[len(modules)-1].Name)
	}

	before, err := os.ReadDir(filepath.Join(workspace.root, "metadata"))
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []metadata.Kind{metadata.FunctionalOptionKind, metadata.CommonCommandKind, metadata.ScheduledJobKind} {
		if _, err := workspace.CreateObject(kind, ""); !errors.Is(err, ErrNoNewObject) {
			t.Errorf("%s, which Studio does not make yet, answered %v", kind, err)
		}
	}
	if after, err := os.ReadDir(filepath.Join(workspace.root, "metadata")); err != nil || len(after) != len(before) {
		t.Errorf("the refused kind left %d entries in metadata, want %d (%v)", len(after), len(before), err)
	}
}

// TestANewObjectIsGivenOnlyTheRightsItsKindHas catches «Устанавливать права
// для новых объектов» giving a role «Чтение» of an object whose kind the
// window of a role offers no such right on: a common module made after such
// a role left the role naming a right the project then refused to load.
// A constant, which has «Чтение», is given it.
func TestANewObjectIsGivenOnlyTheRightsItsKindHas(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	role, err := workspace.CreateRole("")
	if err != nil {
		t.Fatal(err)
	}
	role.Role.GrantNewObjectsByDefault = true
	if _, err := workspace.SaveRole(role.Path, role.Role, role.Revision); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []metadata.Kind{metadata.CommonModuleKind, metadata.CommandGroupKind, metadata.ConstantKind} {
		if _, err := workspace.CreateObject(kind, ""); err != nil {
			t.Fatalf("«Добавить» %s: %v", kind, err)
		}
	}
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		t.Fatalf("the project does not load: %v", err)
	}
	granted, _ := loaded.Role("Роль1")
	constants := loaded.KindObjects(metadata.ConstantKind)
	newConstant := constants[len(constants)-1].ID
	var named []string
	for _, object := range granted.Objects {
		switch object.Object {
		case loaded.Project.ID:
		case newConstant:
			named = append(named, "the new constant "+string(object.Operations[0]))
		default:
			named = append(named, object.Object.String())
		}
	}
	if want := []string{"the new constant read"}; !slices.Equal(named, want) {
		t.Errorf("the role was given %v, want %v", named, want)
	}
}
