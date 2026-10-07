package studio

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// pictureFormSource is a form of an object whose two fields are drawn with
// a picture of their own, each lying in the folder of its element.
func pictureFormSource(t *testing.T) (*Workspace, string, ManagedFormSource) {
	t.Helper()
	workspace, relative, _ := createManagedFormSource(t)
	opened, err := workspace.ReadManagedForm(relative)
	if err != nil {
		t.Fatal(err)
	}
	group := &opened.Form.Items[0]
	group.Children[0].HeaderPicture = &metadata.PictureReference{File: "HeaderPicture.png"}
	group.Children = append(group.Children, metadata.ManagedFormElement{ID: uuid.MustNew(), Name: "Цена",
		Kind: metadata.FormElementInputField, FieldColumn: metadata.FieldColumn{FooterPicture: &metadata.PictureReference{File: "FooterPicture.png"}}})
	saved, err := workspace.SaveManagedForm(relative, opened.Form, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	writePicture(t, workspace, relative, "Наименование/HeaderPicture.png", "наименование")
	writePicture(t, workspace, relative, "Цена/FooterPicture.png", "цена")
	if _, err := metadata.Load(workspace.root); err != nil {
		t.Fatalf("the project with pictures does not load: %v", err)
	}
	return workspace, relative, saved
}

func picturesDirectory(workspace *Workspace, relative string) string {
	return filepath.Join(workspace.root, filepath.Dir(filepath.FromSlash(relative)), project.FormItemsDirectory)
}

func writePicture(t *testing.T, workspace *Workspace, relative, name, content string) {
	t.Helper()
	path := filepath.Join(picturesDirectory(workspace, relative), filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// pictures lists what the folder of the pictures of the form's elements
// holds: element/file=content, sorted.
func pictures(t *testing.T, workspace *Workspace, relative string) []string {
	t.Helper()
	var result []string
	base := picturesDirectory(workspace, relative)
	folders, err := os.ReadDir(base)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, folder := range folders {
		files, err := os.ReadDir(filepath.Join(base, folder.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			content, err := os.ReadFile(filepath.Join(base, folder.Name(), file.Name()))
			if err != nil {
				t.Fatal(err)
			}
			result = append(result, folder.Name()+"/"+file.Name()+"="+string(content))
		}
	}
	slices.Sort(result)
	return result
}

func elementNamed(items []metadata.ManagedFormElement, name string) *metadata.ManagedFormElement {
	for index := range items {
		if items[index].Name == name {
			return &items[index]
		}
		if found := elementNamed(items[index].Children, name); found != nil {
			return found
		}
	}
	return nil
}

// Renaming or removing an element in the designer takes the folder of its
// pictures along, so the project still loads: the folder is named by the
// element, and the designer writes only the description of the form.
//
// Defect caught: a renamed element leaving its pictures under the old name,
// so that the project does not load after an ordinary save; a removed
// element leaving its folder behind, refused at load as a folder of no
// element; two elements trading names getting each other's pictures; a
// change of case alone moving a folder that is found without regard to case;
// an emptied folder of pictures left behind.
func TestFormDesignerTakesThePicturesOfAnElementAlong(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		change func(form *metadata.ManagedForm)
		want   []string
	}{
		"переименование": {func(form *metadata.ManagedForm) {
			elementNamed(form.Items, "Наименование").Name = "Название"
		}, []string{"Название/HeaderPicture.png=наименование", "Цена/FooterPicture.png=цена"}},
		"удаление": {func(form *metadata.ManagedForm) {
			form.Items[0].Children = form.Items[0].Children[:1]
		}, []string{"Наименование/HeaderPicture.png=наименование"}},
		"удаление всех": {func(form *metadata.ManagedForm) {
			form.Items[0].Children = nil
		}, nil},
		"обмен именами": {func(form *metadata.ManagedForm) {
			first, second := elementNamed(form.Items, "Наименование"), elementNamed(form.Items, "Цена")
			first.Name, second.Name = "Цена", "Наименование"
		}, []string{"Наименование/FooterPicture.png=цена", "Цена/HeaderPicture.png=наименование"}},
		"новый элемент на имени удалённого": {func(form *metadata.ManagedForm) {
			form.Items[0].Children = append(form.Items[0].Children[:1], metadata.ManagedFormElement{ID: uuid.MustNew(), Name: "Цена", Kind: metadata.FormElementInputField})
		}, []string{"Наименование/HeaderPicture.png=наименование"}},
		"только регистр": {func(form *metadata.ManagedForm) {
			elementNamed(form.Items, "Наименование").Name = "НАИМЕНОВАНИЕ"
		}, []string{"Наименование/HeaderPicture.png=наименование", "Цена/FooterPicture.png=цена"}},
		"перенос в другую группу": {func(form *metadata.ManagedForm) {
			moved := form.Items[0].Children[1]
			form.Items[0].Children = form.Items[0].Children[:1]
			form.Items = append(form.Items, metadata.ManagedFormElement{ID: uuid.MustNew(), Name: "Группа", Kind: metadata.FormElementUsualGroup,
				Children: []metadata.ManagedFormElement{moved}})
		}, []string{"Наименование/HeaderPicture.png=наименование", "Цена/FooterPicture.png=цена"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			workspace, relative, saved := pictureFormSource(t)
			test.change(&saved.Form)
			if _, err := workspace.SaveManagedForm(relative, saved.Form, saved.Revision); err != nil {
				t.Fatal(err)
			}
			if found := pictures(t, workspace, relative); !slices.Equal(found, test.want) {
				t.Fatalf("pictures = %v, want %v", found, test.want)
			}
			if _, err := os.Lstat(picturesDirectory(workspace, relative)); test.want == nil && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("an emptied folder of pictures is left: %v", err)
			}
			if _, err := metadata.Load(workspace.root); err != nil {
				t.Fatalf("the project does not load after the save: %v", err)
			}
		})
	}
}

// A save the designer cannot carry through leaves the form and every folder
// of pictures as they were.
//
// Defect caught: a rename onto a folder the form does not know overwriting
// or merging it; a save refused for a changed revision moving folders all
// the same; the form left unwritten while its folders already moved, or a
// folder left set aside under a name of no element, after a failure in the
// middle.
func TestARefusedFormSaveLeavesThePicturesAsTheyWere(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		prepare func(t *testing.T, workspace *Workspace, relative string, saved *ManagedFormSource)
		want    string
	}{
		"имя занято чужой папкой": {func(t *testing.T, workspace *Workspace, relative string, saved *ManagedFormSource) {
			writePicture(t, workspace, relative, "Сумма/Picture.png", "чужая")
			elementNamed(saved.Form.Items, "Наименование").Name = "Сумма"
		}, "element Наименование is renamed to Сумма, and the pictures of the form's elements already hold a folder Сумма"},
		"ревизия устарела": {func(t *testing.T, workspace *Workspace, relative string, saved *ManagedFormSource) {
			elementNamed(saved.Form.Items, "Наименование").Name = "Название"
			saved.Revision = strings.Repeat("0", len(saved.Revision))
		}, ErrSourceChanged.Error()},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			workspace, relative, saved := pictureFormSource(t)
			before := pictures(t, workspace, relative)
			content, err := os.ReadFile(filepath.Join(workspace.root, filepath.FromSlash(relative)))
			if err != nil {
				t.Fatal(err)
			}
			test.prepare(t, workspace, relative, &saved)
			expected := pictures(t, workspace, relative)
			if _, err := workspace.SaveManagedForm(relative, saved.Form, saved.Revision); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
			if found := pictures(t, workspace, relative); !slices.Equal(found, expected) || len(before) != 2 {
				t.Fatalf("pictures = %v, want %v", found, expected)
			}
			if after, err := os.ReadFile(filepath.Join(workspace.root, filepath.FromSlash(relative))); err != nil || string(after) != string(content) {
				t.Fatalf("the form changed: %v", err)
			}
		})
	}
}

// A failure while the folders are set aside, or while the form is written,
// puts every folder back; a failure once the form is written says which
// folder is left over. Not parallel: it replaces how a folder is moved.
//
// Defect caught: a folder set aside and never put back, so that the project
// does not load and the picture sits under a name nobody knows; a save that
// half failed reported as done.
func TestAFormSaveFailingInTheMiddlePutsThePicturesBack(t *testing.T) {
	workspace, relative, saved := pictureFormSource(t)
	before := pictures(t, workspace, relative)
	form := saved.Form
	elementNamed(form.Items, "Наименование").Name = "Название"
	elementNamed(form.Items, "Цена").Name = "Стоимость"

	failOn := func(call int) {
		calls := 0
		renameFolder = func(from, to string) error {
			calls++
			if calls == call {
				return errors.New("отказ диска")
			}
			return os.Rename(from, to)
		}
	}
	defer func() { renameFolder = os.Rename }()

	// The second folder cannot be set aside: the first goes back.
	failOn(2)
	if _, err := workspace.SaveManagedForm(relative, form, saved.Revision); err == nil || !strings.Contains(err.Error(), "отказ диска") {
		t.Fatalf("set aside: err = %v", err)
	}
	if found := pictures(t, workspace, relative); !slices.Equal(found, before) {
		t.Fatalf("set aside: pictures = %v, want %v", found, before)
	}

	// The form cannot be written: both folders go back.
	if runtime.GOOS != "windows" {
		renameFolder = os.Rename
		directory := filepath.Dir(filepath.Join(workspace.root, filepath.FromSlash(relative)))
		if err := os.Chmod(directory, 0o555); err != nil {
			t.Fatal(err)
		}
		_, err := workspace.SaveManagedForm(relative, form, saved.Revision)
		if err := os.Chmod(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		if err == nil {
			t.Fatal("the form was written into a folder that cannot be written")
		}
		if found := pictures(t, workspace, relative); !slices.Equal(found, before) {
			t.Fatalf("write: pictures = %v, want %v", found, before)
		}
		if _, err := metadata.Load(workspace.root); err != nil {
			t.Fatalf("write: the project does not load: %v", err)
		}
	}

	// The form is written and the second folder cannot take its new name:
	// the save says so, and names the folder.
	failOn(4)
	_, err := workspace.SaveManagedForm(relative, form, saved.Revision)
	if err == nil || !strings.Contains(err.Error(), "the form is saved, and the pictures of element") || !strings.Contains(err.Error(), ".ml-move-") {
		t.Fatalf("finish: err = %v", err)
	}
}

// A button of a context menu or of the command bar of the form is an element
// with pictures of its own like any other: renamed, its folder goes with it;
// removed, its folder goes away.
//
// Defect caught: the buttons of menus and command bars left out of the
// elements the save looks at, so that renaming one leaves its pictures
// under the old name and the project no longer loads.
func TestTheButtonsOfMenusTakeTheirPicturesAlong(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		change func(form *metadata.ManagedForm)
		want   []string
	}{
		"кнопка меню": {func(form *metadata.ManagedForm) {
			form.Items[0].Children[0].ContextMenu.Children[0].Name = "Распечатать"
		}, []string{"Записать/Picture.png=записать", "Распечатать/Picture.png=печать"}},
		"кнопка панели формы": {func(form *metadata.ManagedForm) {
			form.AutoCommandBar.Children[0].Name = "Сохранить"
		}, []string{"Печать/Picture.png=печать", "Сохранить/Picture.png=записать"}},
		"удалённая кнопка": {func(form *metadata.ManagedForm) {
			form.AutoCommandBar.Children = nil
		}, []string{"Печать/Picture.png=печать"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			workspace, relative, _ := createManagedFormSource(t)
			opened, err := workspace.ReadManagedForm(relative)
			if err != nil {
				t.Fatal(err)
			}
			button := func(name string) metadata.ManagedFormElement {
				return metadata.ManagedFormElement{ID: uuid.MustNew(), Name: name, Kind: metadata.FormElementButton,
					Command: "Form.StandardCommand.Close", GroupProperties: metadata.GroupProperties{Picture: &metadata.PictureReference{File: "Picture.png"}}}
			}
			opened.Form.Items[0].Children[0].ContextMenu = &metadata.FormAttachedMenu{ID: uuid.MustNew(), Name: "НаименованиеКонтекстноеМеню",
				Children: []metadata.ManagedFormElement{button("Печать")}}
			opened.Form.AutoCommandBar = &metadata.FormAttachedMenu{ID: uuid.MustNew(), Name: "ФормаКоманднаяПанель",
				Children: []metadata.ManagedFormElement{button("Записать")}}
			saved, err := workspace.SaveManagedForm(relative, opened.Form, opened.Revision)
			if err != nil {
				t.Fatal(err)
			}
			writePicture(t, workspace, relative, "Печать/Picture.png", "печать")
			writePicture(t, workspace, relative, "Записать/Picture.png", "записать")
			if _, err := metadata.Load(workspace.root); err != nil {
				t.Fatalf("the project with the pictures of menus does not load: %v", err)
			}
			test.change(&saved.Form)
			if _, err := workspace.SaveManagedForm(relative, saved.Form, saved.Revision); err != nil {
				t.Fatal(err)
			}
			if found := pictures(t, workspace, relative); !slices.Equal(found, test.want) {
				t.Fatalf("pictures = %v, want %v", found, test.want)
			}
			if _, err := metadata.Load(workspace.root); err != nil {
				t.Fatalf("the project does not load after the save: %v", err)
			}
		})
	}
}
