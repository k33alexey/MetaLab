package metadata

import (
	"bytes"
	"fmt"
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

// Every role of form the prototype gives a hierarchical object has to survive
// the trip. A catalog has ten: the object, the list and the choice, the folder
// and the choice of a folder, and an auxiliary form beside each of the five.
// A role nobody carries over is a form the platform quietly generates instead
// of opening what the developer drew.
func TestHierarchicalObjectKeepsEveryRoleOfFormItHas(t *testing.T) {
	t.Parallel()
	roles := []struct{ key, form string }{
		{"object", "ФормаЭлемента"},
		{"list", "ФормаСписка"},
		{"choice", "ФормаВыбора"},
		{"folder", "ФормаГруппы"},
		{"folder_choice", "ФормаВыбораГруппы"},
		{"auxiliary_object", "ДругаяФормаЭлемента"},
		{"auxiliary_list", "ДругаяФормаСписка"},
		{"auxiliary_choice", "ДругаяФормаВыбора"},
		{"auxiliary_folder", "ДругаяФормаГруппы"},
		{"auxiliary_folder_choice", "ДругаяФормаВыбораГруппы"},
	}
	root := metadataProject(t)
	body := `format: 1
id: ` + formCatalog + `
name: Пользователи
title: {ru: Пользователи}
code: {type: string, length: 9, auto: true}
description_length: 150
hierarchy: {enabled: true, kind: folders-and-items}
forms:
`
	for _, role := range roles {
		body += "  " + role.key + ": " + role.form + "\n"
	}
	writeMetadata(t, root, CatalogKind, formCatalog, body)
	for index, role := range roles {
		writeObjectForm(t, root, CatalogKind, "Пользователи", role.form,
			"d5000000-0000-4000-8000-0000000001"+fmt.Sprintf("%02d", index))
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	object, ok := catalog.CatalogDefinition("Пользователи")
	if !ok {
		t.Fatal("the catalog was lost")
	}
	for _, role := range roles {
		if _, ok := catalog.ObjectFormID(CatalogKind, "Пользователи", role.form); !ok {
			t.Fatalf("the form of role %s was not kept", role.key)
		}
	}
	if object.Forms.Folder != "ФормаГруппы" || object.Forms.FolderChoice != "ФормаВыбораГруппы" {
		t.Fatalf("the folder roles were lost: %+v", object.Forms)
	}
	if object.Forms.AuxiliaryObject != "ДругаяФормаЭлемента" ||
		object.Forms.AuxiliaryFolderChoice != "ДругаяФормаВыбораГруппы" {
		t.Fatalf("the auxiliary roles were lost: %+v", object.Forms)
	}
}

// A role that names a form the object does not keep is reported by the name
// the role goes by, so the message says which of the ten is wrong.
func TestMissingFormIsReportedByItsRole(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, formCatalog, `format: 1
id: `+formCatalog+`
name: Пользователи
title: {ru: Пользователи}
code: {type: string, length: 9, auto: true}
description_length: 150
hierarchy: {enabled: true, kind: folders-and-items}
forms: {auxiliary_folder_choice: ДругаяФормаВыбораГруппы}
`)
	_, err := Load(root)
	if err == nil {
		t.Fatal("a role naming a form that is not there was accepted")
	}
	if !strings.Contains(err.Error(), "names auxiliary folder choice form ДругаяФормаВыбораГруппы, which it does not keep") {
		t.Fatalf("refused for another reason: %v", err)
	}
}

// The folder roles come with the folders. On an object with no hierarchy, or
// one whose hierarchy is of items alone, there is no folder to open a form of:
// the form would read as working and never open, the same way a level count
// nobody limits does.
func TestFolderFormNeedsAHierarchyWithFolders(t *testing.T) {
	t.Parallel()
	for name, hierarchy := range map[string]string{
		"без иерархии":       "",
		"иерархия элементов": "hierarchy: {enabled: true, kind: items}",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeCatalog("object.yaml", strings.NewReader(`format: 1
id: `+formCatalog+`
name: Пользователи
title: {ru: Пользователи}
code: {type: string, length: 9, auto: true}
description_length: 150
`+hierarchy+`
forms: {folder: ФормаГруппы, auxiliary_folder: ДругаяФормаГруппы}
`), metadataConfiguration())
			if err == nil {
				t.Fatalf("%s: a folder form on an object with no folders was accepted", name)
			}
			for _, want := range []string{
				"forms.folder needs a hierarchy of folders and items",
				"forms.auxiliary_folder needs a hierarchy of folders and items",
			} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("%s: %q missing from %v", name, want, err)
				}
			}
		})
	}
}

// A kind whose hierarchy has no folders at all does not have the roles either.
// Every row of a chart of accounts is an account, so there is no group form to
// name, and naming one is a mistake in the description rather than a setting
// that does nothing.
func TestChartOfAccountsHasNoFolderRoles(t *testing.T) {
	t.Parallel()
	_, err := DecodeChartOfAccounts("object.yaml", strings.NewReader(`format: 1
id: `+formCatalog+`
name: Основной
title: {ru: Основной}
code: {type: string, length: 9, auto: false}
description_length: 150
forms: {folder: ФормаГруппы}
`), metadataConfiguration())
	if err == nil || !strings.Contains(err.Error(), "folder") {
		t.Fatalf("DecodeChartOfAccounts() error = %v", err)
	}
}

// A hierarchical object writes its roles as one flat set: the five every
// object has and the four a folder brings stand side by side under forms, with
// no group of their own. That matters because Studio saves an edited catalog by
// writing the description back, and a role that reads but does not write is a
// form lost on the first save.
func TestFolderRolesSurviveWritingTheDescriptionBack(t *testing.T) {
	t.Parallel()
	before, err := DecodeCatalog("object.yaml", strings.NewReader(`format: 1
id: `+formCatalog+`
name: Пользователи
title: {ru: Пользователи}
code: {type: string, length: 9, auto: true}
description_length: 150
hierarchy: {enabled: true, kind: folders-and-items}
forms: {object: ФормаЭлемента, folder: ФормаГруппы, auxiliary_folder_choice: ДругаяФормаВыбораГруппы}
`), metadataConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	var written bytes.Buffer
	if err := Encode(&written, before); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(written.String(), "\n    folder: ФормаГруппы\n") &&
		!strings.Contains(written.String(), "\n  folder: ФормаГруппы\n") {
		t.Fatalf("the folder role was not written as a role of its own:\n%s", written.String())
	}
	after, err := DecodeCatalog("object.yaml", bytes.NewReader(written.Bytes()), metadataConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	if after.Forms != before.Forms {
		t.Fatalf("the roles did not survive the round trip: %+v want %+v", after.Forms, before.Forms)
	}
}

// A chart of characteristic types is hierarchical the same way a catalog is, so
// it has the same folder roles - and the kinds that are not keep the roles they
// do have. A document has no folders and does have an auxiliary form beside
// each of its three.
func TestFolderRolesGoWithTheKindsThatHaveFolders(t *testing.T) {
	t.Parallel()
	chart, err := DecodeChartOfCharacteristicTypes("object.yaml", strings.NewReader(`format: 1
id: `+formCatalog+`
name: Свойства
title: {ru: Свойства}
code: {type: string, length: 9, auto: true}
description_length: 150
value_type: [{kind: string, length: 50}]
hierarchy: {enabled: true, kind: folders-and-items}
forms: {folder: ФормаГруппы, auxiliary_folder: ДругаяФормаГруппы}
`), metadataConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	if chart.Forms.Folder != "ФормаГруппы" || chart.Forms.AuxiliaryFolder != "ДругаяФормаГруппы" {
		t.Fatalf("the folder roles of a chart of characteristic types were lost: %+v", chart.Forms)
	}
	document, err := DecodeDocument("object.yaml", strings.NewReader(`format: 1
id: `+formCatalog+`
name: Заказ
title: {ru: Заказ}
number: {type: string, length: 9, auto: true, periodicity: year}
forms: {auxiliary_object: ДругаяФормаДокумента, auxiliary_list: ДругаяФормаСписка}
`), metadataConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	if document.Forms.AuxiliaryObject != "ДругаяФормаДокумента" || document.Forms.AuxiliaryList != "ДругаяФормаСписка" {
		t.Fatalf("the auxiliary roles of a document were lost: %+v", document.Forms)
	}
}
