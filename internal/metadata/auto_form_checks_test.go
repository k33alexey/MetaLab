package metadata

import (
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The standard form of an object edits its code, description, number and
// date; the standard list shows them read-only. A field left untitled is shown
// by its name, a language the form has no words for gets the English ones, and
// a title written only in the configuration's default language is found by
// it.
func TestStandardFormEditsWhatTheObjectFormEditsAndNamesItsFields(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	goods := uuid.MustNew().String()
	writeMetadata(t, root, CatalogKind, goods, "format: 1\nid: "+goods+"\nname: Товары\ntitle: {ru: Товары}\n"+
		"code: {type: string, length: 9, auto: true}\ndescription_length: 150\n"+
		"attributes:\n  - {id: "+uuid.MustNew().String()+", name: Артикул, title: {}, types: [{kind: string, length: 20}]}\n"+
		"  - {id: "+uuid.MustNew().String()+", name: Вес, title: {ru: Вес товара, en: Weight}, types: [{kind: number, precision: 10, scale: 3}]}\n"+
		"table_parts:\n  - {id: "+uuid.MustNew().String()+", name: Состав, title: {}, attributes: [{id: "+uuid.MustNew().String()+", name: Часть, title: {ru: Часть}, types: [{kind: boolean}]}]}\n"+
		"  - {id: "+uuid.MustNew().String()+", name: Упаковки, title: {ru: Упаковки товара}, attributes: [{id: "+uuid.MustNew().String()+", name: Упаковка, title: {ru: Упаковка}, types: [{kind: boolean}]}]}\n")
	order := uuid.MustNew().String()
	writeMetadata(t, root, DocumentKind, order, "format: 1\nid: "+order+"\nname: Заказ\ntitle: {ru: Заказ}\n"+
		"number: {type: string, length: 9, auto: true, periodicity: none}\nposting: {allowed: true}\n")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	readOnly := func(form FormDescriptor) map[string]bool {
		result := map[string]bool{}
		for _, field := range form.Fields {
			result[field.Name] = field.ReadOnly
		}
		return result
	}
	for _, test := range []struct {
		kind FormKind
		want bool
	}{{ObjectForm, false}, {ListForm, true}, {ChoiceForm, true}} {
		goodsForm, err := catalog.CatalogForm("Товары", test.kind, "ru")
		if err != nil {
			t.Fatal(err)
		}
		orderForm, err := catalog.DocumentForm("Заказ", test.kind, "ru")
		if err != nil {
			t.Fatal(err)
		}
		fields := readOnly(goodsForm)
		for name, value := range readOnly(orderForm) {
			fields[name] = value
		}
		for _, name := range []string{"Code", "Description", "Number", "Date"} {
			if value, ok := fields[name]; !ok || value != test.want {
				t.Fatalf("%s form: %s read-only = %v (present %v), want %v", test.kind, name, value, ok, test.want)
			}
		}
	}

	form, err := catalog.CatalogForm("Товары", ObjectForm, "ru")
	if err != nil {
		t.Fatal(err)
	}
	titles := map[string]string{}
	for _, field := range form.Fields {
		titles[field.Name] = field.Title
	}
	if titles["Артикул"] != "Артикул" || titles["Вес"] != "Вес товара" {
		t.Fatalf("field titles = %v", titles)
	}
	if len(form.TableParts) != 2 || form.TableParts[0].Title != "Состав" || form.TableParts[1].Title != "Упаковки товара" {
		t.Fatalf("table parts = %+v", form.TableParts)
	}
	// The title is written in ru, the configuration's default, and in en; a
	// form asked for in uk finds the default, not the first by the alphabet.
	if form, err := catalog.CatalogForm("Товары", ObjectForm, "uk"); err != nil || fieldTitle(form, "Вес") != "Вес товара" {
		t.Fatalf("uk title = %q, %v", fieldTitle(form, "Вес"), err)
	}
	if form, err := catalog.CatalogForm("Товары", ObjectForm, "de"); err != nil || fieldTitle(form, "Code") != "Code" || fieldTitle(form, "Description") != "Description" {
		t.Fatalf("de titles = %q, %q, %v", fieldTitle(form, "Code"), fieldTitle(form, "Description"), err)
	}
}

func fieldTitle(form FormDescriptor, name string) string {
	for _, field := range form.Fields {
		if field.Name == name {
			return field.Title
		}
	}
	return ""
}

// A document that keeps no records has nothing to show under "Document
// movements", so its standard form does not offer the command; one that
// keeps records does.
func TestTheMovementsCommandIsOfferedOnlyByADocumentThatKeepsRecords(t *testing.T) {
	t.Parallel()
	root := entriesProject(t, false, entriesStandardFields)
	other := uuid.MustNew().String()
	writeMetadata(t, root, DocumentKind, other, "format: 1\nid: "+other+"\nname: Заметка\ntitle: {ru: Заметка}\n"+
		"number: {type: string, length: 9, auto: true, periodicity: none}\nposting: {allowed: true}\n")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	offers := func(name string) bool {
		form, err := catalog.DocumentForm(name, ObjectForm, "ru")
		if err != nil {
			t.Fatal(err)
		}
		return slices.ContainsFunc(form.Commands, func(command FormCommand) bool { return command.Name == "Movements" })
	}
	if !offers("Операция") {
		t.Fatal("a document that keeps records does not offer its movements")
	}
	if offers("Заметка") {
		t.Fatal("a document that keeps no records offers its movements")
	}
}

// The forms of a published snapshot are listed by kind, then object, then
// name: the snapshot is compared byte for byte with the one applied, and the
// index they come from is a map.
func TestObjectFormsAreListedInOneOrder(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	for _, name := range []string{"Склады", "Банки"} {
		id := uuid.MustNew().String()
		writeMetadata(t, root, CatalogKind, id, "format: 1\nid: "+id+"\nname: "+name+"\ntitle: {ru: "+name+"}\n"+
			"code: {type: string, length: 9, auto: true}\ndescription_length: 150\n")
		for _, form := range []string{"ФормаСписка", "ФормаВыбора", "ФормаПодбора"} {
			writeObjectForm(t, root, CatalogKind, name, form, uuid.MustNew().String())
		}
	}
	id := uuid.MustNew().String()
	writeMetadata(t, root, DocumentKind, id, "format: 1\nid: "+id+"\nname: Заказ\ntitle: {ru: Заказ}\n"+
		"number: {type: string, length: 9, auto: true, periodicity: none}\n")
	for _, form := range []string{"ФормаСписка", "ФормаВыбора"} {
		writeObjectForm(t, root, DocumentKind, "Заказ", form, uuid.MustNew().String())
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	forms := catalog.ObjectForms()
	if len(forms) != 8 {
		t.Fatalf("forms = %+v", forms)
	}
	if !slices.IsSortedFunc(forms, func(left, right RuntimeObjectForm) int {
		for _, pair := range [][2]string{{string(left.ObjectKind), string(right.ObjectKind)}, {left.Object, right.Object}, {left.Name, right.Name}} {
			if pair[0] != pair[1] {
				if pair[0] < pair[1] {
					return -1
				}
				return 1
			}
		}
		return 0
	}) {
		t.Fatalf("forms are not in kind, object, name order: %+v", forms)
	}
}
