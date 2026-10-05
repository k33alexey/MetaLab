package metadata

import (
	"errors"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
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
		if !strings.Contains(strictErr.Error(), item.Where+" "+item.Text()) {
			t.Errorf("the refusal does not name %s %s: %v", item.Where, item.Text(), strictErr)
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

// The other remnants of what was deleted are errors of the project like a
// reference to nothing (owner, 05.10.2026): the type of an object the project
// no longer has, a value naming a type it no longer has - as a filling and as
// a choice parameter - and a path of a link leading to no field. Each is
// refused by the strict load, named with its place and as it is written, and
// listed by the reading for editing.
//
// Defect caught: a remnant still accepted with a note, as it was before; a
// remnant of one place collected and of another not; and one named without
// what is written, so that a developer cannot find it in the file.
func TestRemnantsOfWhatWasDeletedAreErrors(t *testing.T) {
	t.Parallel()
	const value = "466cbe70-c94c-4cdc-a0fb-f9f9084bdef2.00000000-0000-0000-0000-000000000000"
	gone := uuid.MustNew().String()
	for name, test := range map[string]struct {
		write        func(t *testing.T, root string)
		where, wrote string
	}{
		"тип удалённого объекта": {func(t *testing.T, root string) {
			noteCatalog(t, root, "commands:\n  - {id: "+uuid.MustNew().String()+", name: Подбор, title: {ru: Подбор}, parameter: [{kind: vanished-type, reference: "+gone+"}]}\n", "")
			writeCommandModule(t, root, CatalogKind, "Товары", "Подбор")
		}, "Подбор parameter type", gone},
		"значение заполнения удалённого типа": {func(t *testing.T, root string) {
			noteCatalog(t, root, "", noteField("string", "    filling: {value: {kind: unresolved-reference, data: \""+value+"\"}}\n"))
		}, " filling", value},
		"параметр выбора удалённого типа": {func(t *testing.T, root string) {
			noteCatalog(t, root, "", noteField("string", "    choice: {parameters: [{name: Отбор.Вид, values: [{kind: unresolved-reference, data: \""+value+"\"}]}]}\n"))
		}, " choice parameter Отбор.Вид", value},
		"путь связи в никуда": {func(t *testing.T, root string) {
			noteCatalog(t, root, "", noteField("string", "    choice: {parameter_links: [{name: Отбор.Владелец, source: {unresolved: \"-3\"}}]}\n"))
		}, " choice link Отбор.Владелец", "-3"},
		"связь по типу в никуда": {func(t *testing.T, root string) {
			noteCatalog(t, root, "", noteField("string", "    choice: {link_by_type: {source: {unresolved: \"-3\"}}}\n"))
		}, " link by type", "-3"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			test.write(t, root)
			unresolved := unresolvedOf(t, root)
			if len(unresolved) != 1 || !strings.HasSuffix(unresolved[0].Where, test.where) || unresolved[0].Text() != test.wrote {
				t.Fatalf("unresolved = %+v, want %q at …%q", unresolved, test.wrote, test.where)
			}
		})
	}
}
