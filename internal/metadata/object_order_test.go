package metadata

import (
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	orderCatalogA  = "e5000000-0000-4000-8000-00000000000a"
	orderCatalogB  = "e5000000-0000-4000-8000-00000000000b"
	orderCatalogC  = "e5000000-0000-4000-8000-00000000000c"
	orderConstantA = "e5000000-0000-4000-8000-0000000000a1"
	orderConstantB = "e5000000-0000-4000-8000-0000000000b1"
)

// orderProject is a project with three catalogs, Альфа, Бета and Гамма, and
// two constants, whose identifiers run in the order of their names - so an
// order that is neither is seen to come from the root and from nothing else.
func orderProject(t *testing.T, order map[string][]uuid.UUID) string {
	t.Helper()
	root := metadataProject(t)
	for id, name := range map[string]string{orderCatalogA: "Альфа", orderCatalogB: "Бета", orderCatalogC: "Гамма"} {
		writeMetadata(t, root, CatalogKind, id, "format: 1\nid: "+id+"\nname: "+name+"\ntitle: {ru: "+name+"}\n"+
			"code: {type: string, length: 9}\ndescription_length: 25\n")
	}
	for id, name := range map[string]string{orderConstantA: "Первая", orderConstantB: "Вторая"} {
		writeMetadata(t, root, ConstantKind, id, "format: 1\nid: "+id+"\nname: "+name+"\ntitle: {ru: "+name+"}\n"+
			"types: [{kind: boolean}]\n")
	}
	if order != nil {
		saveDefaults(t, root, func(configuration *project.Project) { configuration.ObjectsOrder = order })
	}
	return root
}

func ids(t *testing.T, texts ...string) []uuid.UUID {
	t.Helper()
	result := make([]uuid.UUID, 0, len(texts))
	for _, text := range texts {
		result = append(result, mustUUID(t, text))
	}
	return result
}

// The developer's order of the objects of a kind is the prototype's own - it
// moves an object up and down, sorts, and writes its export in that order
// (lombard1: catalogs out of alphabet in nine places) - and the root keeps it
// (owner, 24.10.2026). Defects caught: the order was not read, or was replaced
// by the order of names or of identifiers.
func TestObjectsOrderIsTheOneTheRootKeeps(t *testing.T) {
	t.Parallel()
	root := orderProject(t, map[string][]uuid.UUID{"catalogs": ids(t, orderCatalogC, orderCatalogA, orderCatalogB)})
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := catalog.ObjectsOrder(CatalogKind), ids(t, orderCatalogC, orderCatalogA, orderCatalogB); !slices.Equal(got, want) {
		t.Fatalf("the order of catalogs is %v, want %v", got, want)
	}
	// A kind the root keeps no order of is in the order of names: Вторая
	// before Первая, against their identifiers.
	if got, want := catalog.ObjectsOrder(ConstantKind), ids(t, orderConstantB, orderConstantA); !slices.Equal(got, want) {
		t.Fatalf("the order of constants is %v, want %v", got, want)
	}
	for _, note := range catalog.Notes() {
		if note.Kind == NoteObjectOutsideOrder {
			t.Fatalf("an object the order names is noted: %+v", note)
		}
	}
	// The catalog's own list stays in the order of identifiers: what reads it
	// by position - the schema, the snapshot - does not move with the tree.
	if catalog.Catalogs[0].ID.String() != orderCatalogA || catalog.Catalogs[2].ID.String() != orderCatalogC {
		t.Fatalf("the list of catalogs moved: %s, %s", catalog.Catalogs[0].Name, catalog.Catalogs[2].Name)
	}
}

// An object of a kind with a kept order that the order leaves out was added
// past Studio. It stands after the listed ones, by name, and is noted.
// Defects caught: the object was lost from the order, put first, or not noted.
func TestObjectLeftOutOfTheOrderGoesLastAndIsNoted(t *testing.T) {
	t.Parallel()
	root := orderProject(t, map[string][]uuid.UUID{"catalogs": ids(t, orderCatalogC)})
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := catalog.ObjectsOrder(CatalogKind), ids(t, orderCatalogC, orderCatalogA, orderCatalogB); !slices.Equal(got, want) {
		t.Fatalf("the order of catalogs is %v, want %v", got, want)
	}
	var noted []string
	for _, note := range catalog.Notes() {
		if note.Kind == NoteObjectOutsideOrder {
			noted = append(noted, note.Where)
		}
	}
	if want := []string{"catalog Альфа", "catalog Бета"}; !slices.Equal(noted, want) {
		t.Fatalf("noted %v, want %v", noted, want)
	}
}

// A line of the order naming no object of its kind is an object deleted past
// Studio, or an object of another kind: a reference to nothing, refused by
// the strict load and listed by the reading for editing. Defect caught: the
// line was dropped silently and the order shifted under it.
func TestOrderLineNamingNothingIsAReferenceToNothing(t *testing.T) {
	t.Parallel()
	gone := "e5000000-0000-4000-8000-0000000000ff"
	for name, list := range map[string][]uuid.UUID{
		"объекта нет":         ids(t, orderCatalogB, gone),
		"объект другого вида": ids(t, orderCatalogB, orderConstantA),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := orderProject(t, map[string][]uuid.UUID{"catalogs": list})
			if !containsWhere(unresolvedOf(t, root), "the configuration's objects_order.catalogs") {
				t.Fatalf("%s: the line was not listed", name)
			}
		})
	}
}
