package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

const (
	formCatalog = "d5000000-0000-4000-8000-000000000001"
	formItem    = "d5000000-0000-4000-8000-000000000010"
	formList    = "d5000000-0000-4000-8000-000000000011"
	formPick    = "d5000000-0000-4000-8000-000000000012"
)

// A form is found by its name, and the slots of an object name one of the
// forms the object keeps. The folder is the list of forms; a slot only says
// which of them the platform opens by default.
//
// Two things follow, and the prototype's own catalog of users shows both: it
// keeps seven forms behind three slots, and one of those forms - ФормаСписка -
// stands in two slots at once, as the main list and as the main choice.
func TestObjectKeepsMoreFormsThanSlotsAndMayShareOne(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, formCatalog, `format: 1
id: `+formCatalog+`
name: Пользователи
title: {ru: Пользователи}
code: {type: string, length: 9, auto: true}
description_length: 150
forms:
  object: ФормаЭлемента
  list: ФормаСписка
  choice: ФормаСписка
`)
	writeObjectForm(t, root, CatalogKind, "Пользователи", "ФормаЭлемента", formItem)
	writeObjectForm(t, root, CatalogKind, "Пользователи", "ФормаСписка", formList)
	// A form no slot names: an ordinary form of the object, opened by a
	// command rather than by the platform. It must be kept, not called an
	// orphan.
	writeObjectForm(t, root, CatalogKind, "Пользователи", "ВводПароля", formPick)

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	object, err := catalog.CatalogForm("Пользователи", ObjectForm, "ru")
	if err != nil || object.Generated || object.SourceID == nil || object.SourceID.String() != formItem {
		t.Fatalf("the object form was lost: %+v error=%v", object, err)
	}
	list, err := catalog.CatalogForm("Пользователи", ListForm, "ru")
	if err != nil || list.SourceID == nil || list.SourceID.String() != formList {
		t.Fatalf("the list form was lost: %+v error=%v", list, err)
	}
	choice, err := catalog.CatalogForm("Пользователи", ChoiceForm, "ru")
	if err != nil || choice.SourceID == nil || *choice.SourceID != *list.SourceID {
		t.Fatalf("one form in two slots did not survive: %+v error=%v", choice, err)
	}
	if id, ok := catalog.ObjectFormID(CatalogKind, "Пользователи", "ВводПароля"); !ok || id.String() != formPick {
		t.Fatalf("a form no slot names was not kept: %s %v", id, ok)
	}
}

// A published application has no folders: it runs from a snapshot. What the
// folders said therefore has to travel inside it, or every form an object
// declared silently turns back into the one the platform generates - and
// nothing anywhere says a form went missing.
func TestPublishedSnapshotStillKnowsAnObjectsForms(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, formCatalog, `format: 1
id: `+formCatalog+`
name: Пользователи
title: {ru: Пользователи}
code: {type: string, length: 9, auto: true}
description_length: 150
forms: {object: ФормаЭлемента}
`)
	writeObjectForm(t, root, CatalogKind, "Пользователи", "ФормаЭлемента", formItem)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	form := ManagedForm{
		Format: CurrentFormat, ID: mustUUID(t, formItem), Name: "ФормаЭлемента",
		Title: LocalizedText{"ru": "Форма элемента"}, Kind: ObjectForm,
	}
	snapshot, err := NewRuntimeSnapshot(catalog, []ManagedForm{form})
	if err != nil {
		t.Fatal(err)
	}
	running, err := snapshot.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := running.CatalogForm("Пользователи", ObjectForm, "ru")
	if err != nil || descriptor.Generated || descriptor.SourceID == nil || descriptor.SourceID.String() != formItem {
		t.Fatalf("the published application lost the form: %+v error=%v", descriptor, err)
	}
	if _, ok := snapshot.Form(*descriptor.SourceID); !ok {
		t.Fatal("the form the descriptor points at is not in the snapshot")
	}
}

// What can be wrong about a form is where it lies and what it calls itself.
func TestBrokenObjectFormsAreRefused(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		slot  string
		write func(t *testing.T, root string)
		want  string
	}{
		"слот называет форму, которой нет": {
			slot: "forms: {object: ФормаЭлемента}", want: "which it does not keep",
		},
		"форма называет себя иначе, чем папка": {
			slot: "forms: {object: ФормаЭлемента}",
			write: func(t *testing.T, root string) {
				writeObjectForm(t, root, CatalogKind, "Пользователи", "ФормаЭлемента", formItem)
				path := filepath.Join(root, "metadata", string(CatalogKind), "Пользователи", "forms", "ФормаЭлемента", project.FormMetadataFile)
				body := "format: 1\nid: " + formItem + "\nname: ДругаяФорма\ntitle: {ru: x}\nkind: object\n"
				if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "lies in a folder called",
		},
		"в папке формы нет описания": {
			write: func(t *testing.T, root string) {
				directory := filepath.Join(root, "metadata", string(CatalogKind), "Пользователи", "forms", "ФормаЭлемента")
				if err := os.MkdirAll(directory, 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: "has no " + project.FormMetadataFile,
		},
		"форма без идентификатора": {
			write: func(t *testing.T, root string) {
				directory := filepath.Join(root, "metadata", string(CatalogKind), "Пользователи", "forms", "ФормаЭлемента")
				if err := os.MkdirAll(directory, 0o755); err != nil {
					t.Fatal(err)
				}
				body := "format: 1\nname: ФормаЭлемента\n"
				if err := os.WriteFile(filepath.Join(directory, project.FormMetadataFile), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "has no identifier of its own",
		},
		"вместо папки формы файл": {
			write: func(t *testing.T, root string) {
				directory := filepath.Join(root, "metadata", string(CatalogKind), "Пользователи", "forms")
				if err := os.MkdirAll(directory, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, formItem+".yaml"), []byte("format: 1\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "a form is a folder",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, CatalogKind, formCatalog, `format: 1
id: `+formCatalog+`
name: Пользователи
title: {ru: Пользователи}
code: {type: string, length: 9, auto: true}
description_length: 150
`+test.slot+`
`)
			if test.write != nil {
				test.write(t, root)
			}
			_, err := Load(root)
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

// A slot carries a name, and a name is an identifier. Anything else would be
// a folder nobody can create.
func TestFormSlotMustCarryAName(t *testing.T) {
	t.Parallel()
	_, err := DecodeCatalog("object.yaml", strings.NewReader(`format: 1
id: `+formCatalog+`
name: Пользователи
title: {ru: Пользователи}
code: {type: string, length: 9, auto: true}
description_length: 150
forms: {list: "Форма списка"}
`), metadataConfiguration())
	if err == nil || !strings.Contains(err.Error(), "forms.list must be the name of a form") {
		t.Fatalf("DecodeCatalog() error = %v", err)
	}
}
