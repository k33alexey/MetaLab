package metadata

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/uuid"
	"go.yaml.in/yaml/v3"
)

const (
	formOrderCatalog = "e6000000-0000-4000-8000-000000000001"
	formOrderA       = "e6000000-0000-4000-8000-0000000000a1"
	formOrderB       = "e6000000-0000-4000-8000-0000000000b1"
	formOrderC       = "e6000000-0000-4000-8000-0000000000c1"
)

// formOrderProject is a project with a catalog Товары keeping three forms,
// Альфа, Бета and Гамма, whose identifiers run in the order of their names,
// and order written in its description as given ("" - no key).
func formOrderProject(t *testing.T, order string) string {
	t.Helper()
	root := metadataProject(t)
	body := "format: 1\nid: " + formOrderCatalog + "\nname: Товары\ntitle: {ru: Товары}\ncode: {type: string, length: 9}\ndescription_length: 25\n"
	if order != "" {
		body += "order: {forms: [" + order + "]}\n"
	}
	writeMetadata(t, root, CatalogKind, formOrderCatalog, body)
	for id, name := range map[string]string{formOrderA: "Альфа", formOrderB: "Бета", formOrderC: "Гамма"} {
		writeObjectForm(t, root, CatalogKind, "Товары", name, id)
	}
	return root
}

// The forms of an object are in the developer's order, which its description
// keeps (METADATA-OBJECTS.md, «Формат порядка в описании объекта»): the forms
// the order names, in its order, then the others by name, each noted; without
// the key - by name, nothing noted. Defects caught: the order not read, the
// forms it leaves out lost or put first, a form the order names noted, the
// order of names left behind where there is no order.
func TestTheFormsOfAnObjectAreInTheOrderItsDescriptionKeeps(t *testing.T) {
	t.Parallel()
	for _, item := range []struct {
		name, order string
		want, noted []string
	}{
		{"порядок назван целиком", formOrderC + ", " + formOrderA + ", " + formOrderB, []string{"Гамма", "Альфа", "Бета"}, nil},
		{"порядок назвал не все", formOrderC + ", " + formOrderA, []string{"Гамма", "Альфа", "Бета"}, []string{"catalog Товары form Бета"}},
		{"порядок назвал одну", formOrderB, []string{"Бета", "Альфа", "Гамма"}, []string{"catalog Товары form Альфа", "catalog Товары form Гамма"}},
		{"порядка нет", "", []string{"Альфа", "Бета", "Гамма"}, nil},
	} {
		t.Run(item.name, func(t *testing.T) {
			t.Parallel()
			catalog, err := Load(formOrderProject(t, item.order))
			if err != nil {
				t.Fatal(err)
			}
			if got := catalog.ObjectFormNames(CatalogKind, "Товары"); !slices.Equal(got, item.want) {
				t.Errorf("the forms are %v, want %v", got, item.want)
			}
			var noted []string
			for _, note := range catalog.Notes() {
				if note.Kind == NoteFormOutsideOrder {
					noted = append(noted, note.Where)
				}
			}
			if !slices.Equal(noted, item.noted) {
				t.Errorf("noted %v, want %v", noted, item.noted)
			}
		})
	}
}

// A line of the order of forms naming no form of the object is a form
// deleted past Studio: a reference to nothing, refused by the strict load and
// listed by the reading for editing; a zero identifier or one named twice is
// no order at all and is refused. Defects caught: the line dropped silently,
// a form of another object taken for one of this, a list naming a form twice
// read as written.
func TestTheOrderOfFormsRefusesWhatNamesNoForm(t *testing.T) {
	t.Parallel()
	t.Run("формы нет", func(t *testing.T) {
		t.Parallel()
		if !containsWhere(unresolvedOf(t, formOrderProject(t, formOrderA+", e6000000-0000-4000-8000-0000000000ff")), "catalog Товары order.forms") {
			t.Fatal("the line naming no form was not listed")
		}
	})
	for name, order := range map[string]string{
		"нулевой": formOrderA + ", 00000000-0000-0000-0000-000000000000",
		"дважды":  formOrderA + ", " + formOrderB + ", " + formOrderA,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(formOrderProject(t, order))
			if err == nil || !strings.Contains(err.Error(), "order.forms[") {
				t.Fatalf("%s: the order was taken: %v", name, err)
			}
		})
	}
}

// Every kind keeping forms carries the order of its forms in its
// description, and no other: the field is one embedded structure
// (SubordinatesOrder), written under the key order and left out when empty,
// in YAML and in JSON, and its copy shares nothing. Read off the kinds'
// layouts, so a kind gaining forms is held here without a word. Defects
// caught: a kind with forms whose order would be lost on reading, the key
// written empty into every description, a copy sharing the list.
func TestEveryKindKeepingFormsCarriesTheirOrder(t *testing.T) {
	t.Parallel()
	for _, item := range kindInfos {
		descriptor, ok := kinds.Of(item.kind)
		if !ok {
			continue
		}
		if item.described == nil {
			if descriptor.Layout().Forms {
				t.Errorf("%s keeps forms and no description to carry their order", item.kind)
			}
			continue
		}
		described := item.described
		if described.Kind() == reflect.Pointer {
			described = described.Elem()
		}
		field, has := described.FieldByName("SubordinatesOrder")
		if keeps := descriptor.Layout().Forms; keeps != (has && field.Anonymous) {
			t.Errorf("%s keeps forms %v, carries their order %v", item.kind, keeps, has)
		}
	}
	empty, err := yaml.Marshal(CatalogDefinition{ID: uuid.MustNew(), Name: "Товары"})
	if err != nil {
		t.Fatal(err)
	}
	emptyJSON, err := json.Marshal(DocumentDefinition{ID: uuid.MustNew(), Name: "Заказ"})
	if err != nil || !strings.Contains(string(emptyJSON), "Заказ") {
		t.Fatalf("a document in JSON: %s (%v)", emptyJSON, err)
	}
	if strings.Contains(string(empty), "order") || strings.Contains(string(emptyJSON), `"order"`) {
		t.Errorf("an empty order is written: %s / %s", empty, emptyJSON)
	}
	id := uuid.MustParse(formOrderA)
	value := CatalogDefinition{ID: uuid.MustNew(), Name: "Товары", SubordinatesOrder: SubordinatesOrder{Order: ObjectOrder{Forms: []uuid.UUID{id}}}}
	written, err := yaml.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "order:\n    forms:\n        - "+formOrderA) {
		t.Errorf("the order is written as %s", written)
	}
	var read CatalogDefinition
	if err := yaml.Unmarshal(written, &read); err != nil || !slices.Equal(read.Order.Forms, value.Order.Forms) {
		t.Errorf("the order read back is %v (%v)", read.Order.Forms, err)
	}
	copied := cloneCatalogDefinition(value)
	copied.Order.Forms[0] = uuid.MustNew()
	if value.Order.Forms[0] != id {
		t.Error("the copy of a description shares the order of its forms")
	}
}
