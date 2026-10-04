package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// putFile writes a file at a path below the project, making its folders.
func putFile(t *testing.T, root string, steps ...string) {
	t.Helper()
	path := filepath.Join(append([]string{root}, steps...)...)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("образ"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// pictureFileOwner is one place a reference to a picture of its own stands,
// written with the given picture, with its folder and the steps to it.
type pictureFileOwner struct {
	write  func(t *testing.T, root, picture string)
	folder []string
	// read returns the reference as loaded.
	read func(catalog *Catalog) *PictureReference
}

func pictureFileOwners() map[string]pictureFileOwner {
	subsystem, group := uuid.MustNew().String(), uuid.MustNew().String()
	catalogID := uuid.MustNew().String()
	return map[string]pictureFileOwner{
		"команда объекта": {
			write: func(t *testing.T, root, picture string) {
				writeMetadata(t, root, CatalogKind, catalogID, "format: 1\nid: "+catalogID+"\nname: Товары\ntitle: {ru: Товары}\n"+
					"code: {type: string, length: 9}\ndescription_length: 150\n"+
					"commands:\n  - {id: "+uuid.MustNew().String()+", name: Подбор, title: {ru: Подбор}, picture: "+picture+"}\n")
				writeCommandModule(t, root, CatalogKind, "Товары", "Подбор")
			},
			folder: []string{"metadata", string(CatalogKind), "Товары", "commands", "Подбор"},
			read: func(catalog *Catalog) *PictureReference {
				return catalog.Catalogs[0].Commands[0].Picture
			},
		},
		"общая команда": {
			write: func(t *testing.T, root, picture string) {
				writeCommonCommand(t, root, "Обменяться", "format: 1\nid: "+commonCommandID+"\nname: Обменяться\ntitle: {ru: Обменяться}\npicture: "+picture+"\n", true)
			},
			folder: []string{"metadata", string(CommonCommandKind), "Обменяться"},
			read: func(catalog *Catalog) *PictureReference {
				command, _ := catalog.CommonCommand("Обменяться")
				return command.Picture
			},
		},
		"подсистема": {
			write: func(t *testing.T, root, picture string) {
				writeMetadata(t, root, SubsystemKind, subsystem, "format: 1\nid: "+subsystem+"\nname: Продажи\ntitle: {ru: Продажи}\npicture: "+picture+"\n")
			},
			folder: []string{"metadata", string(SubsystemKind), subsystem},
			read:   func(catalog *Catalog) *PictureReference { return catalog.Subsystems[0].Picture },
		},
		"группа команд": {
			write: func(t *testing.T, root, picture string) {
				writeMetadata(t, root, CommandGroupKind, group, "format: 1\nid: "+group+"\nname: Обмены\ntitle: {ru: Обмены}\ncategory: actions-panel\npicture: "+picture+"\n")
			},
			folder: []string{"metadata", string(CommandGroupKind), group},
			read:   func(catalog *Catalog) *PictureReference { return catalog.CommandGroups[0].Picture },
		},
		"точка карты маршрута": {
			write: func(t *testing.T, root, picture string) {
				routeProcessInto(t, root, "\n  points:\n    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start, look: {picture: "+picture+"}}\n"+
					"    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}\n"+
					"  transitions:\n    - {from: Старт, to: Завершение}\n")
			},
			folder: []string{"metadata", string(BusinessProcessKind), "Задание", "route", "Старт"},
			read: func(catalog *Catalog) *PictureReference {
				return catalog.BusinessProcesses[0].Route.Points[0].Look.Picture
			},
		},
		"фигура карты маршрута": {
			write: func(t *testing.T, root, picture string) {
				routeProcessInto(t, root, "\n  points:\n    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start}\n"+
					"    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}\n"+
					"  transitions:\n    - {from: Старт, to: Завершение}\n"+
					"  decorations:\n    - {name: Пояснение, shape: Block, location: {top: 1, left: 1, bottom: 20, right: 20}, look: {picture: "+picture+"}}\n")
			},
			folder: []string{"metadata", string(BusinessProcessKind), "Задание", "route", "Пояснение"},
			read: func(catalog *Catalog) *PictureReference {
				return catalog.BusinessProcesses[0].Route.Decorations[0].Look.Picture
			},
		},
	}
}

// routeProcessInto writes the process of routeOnlyProcess into a project
// already made.
func routeProcessInto(t *testing.T, root, route string) {
	t.Helper()
	writeMetadata(t, root, TaskKind, taskID, "format: 1\nid: "+taskID+"\nname: ЗадачаИсполнителя\ntitle: {ru: Задача исполнителя}\n"+
		"number: {type: string, length: 14, auto: true, unique: true, periodicity: none}\ndescription_length: 150\n")
	writeMetadata(t, root, BusinessProcessKind, businessProcessID, "format: 1\nid: "+businessProcessID+"\nname: Задание\ntitle: {ru: Задание}\n"+
		"number: {type: string, length: 11, auto: true, unique: true, periodicity: none}\ntask: "+taskID+"\nroute:"+route)
}

// A picture drawn from a file of its own lies in the folder of whoever is
// shown with it, under the name the reference gives - as the prototype keeps
// it beside its owner and names it in the reference. Defects caught: the
// reference had no place for a file, and decoding refused it; the folder of
// a command, a common command, a subsystem, a group or an item of a route map
// refused the file; the name was compared by case where the prototype does
// not compare it.
func TestPictureDrawnFromAFileLiesInTheFolderOfItsOwner(t *testing.T) {
	t.Parallel()
	for owner, place := range pictureFileOwners() {
		t.Run(owner, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			place.write(t, root, "{file: Picture.png, load_transparent: true, transparent_pixel: {x: 1, y: 1}}")
			putFile(t, root, append(place.folder, "picture.png")...)
			catalog, err := Load(root)
			if err != nil {
				t.Fatalf("a picture drawn from its file was refused: %v", err)
			}
			if picture := place.read(catalog); picture == nil || picture.File != "Picture.png" || !picture.Names() {
				t.Fatalf("the file of the picture was lost: %+v", picture)
			}
		})
	}
}

// The file is required, and nothing else is kept in its place. Defects
// caught: a reference to a file nobody keeps loaded - a blank place in the
// interface; another image beside the one drawn was carried silently, so two
// files claimed to be the picture.
func TestPictureFileIsRequiredAndAlone(t *testing.T) {
	t.Parallel()
	for owner, place := range pictureFileOwners() {
		t.Run(owner, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			place.write(t, root, "{file: Picture.png}")
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), "is shown with picture file Picture.png, which its folder does not hold") {
				t.Fatalf("a picture without its file: refused for another reason or not at all: %v", err)
			}

			putFile(t, root, append(place.folder, "Picture.png")...)
			putFile(t, root, append(place.folder, "Other.png")...)
			_, err = Load(root)
			if err == nil || !strings.Contains(err.Error(), `"Other.png"`) {
				t.Fatalf("a second image beside the picture: refused for another reason or not at all: %v", err)
			}
		})
	}
}

// Without a file of its own the owner's folder keeps no image at all: an
// image nobody draws is refused where it lies. Defect caught: the folder took
// any image whether or not the picture named one.
func TestImageNobodyDrawsIsRefused(t *testing.T) {
	t.Parallel()
	for owner, place := range pictureFileOwners() {
		t.Run(owner, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			place.write(t, root, "{standard: Обмен}")
			putFile(t, root, append(place.folder, "Picture.png")...)
			if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "Picture.png") {
				t.Fatalf("an image nobody draws: refused for another reason or not at all: %v", err)
			}
		})
	}
}

// The reference names an image file and one source only. Defects caught: a
// file beside a standard picture was taken, and so was a name that is not an
// image or leaves the folder.
func TestPictureFileReferenceIsChecked(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ picture, refusal string }{
		"файл и стандартная": {"{file: Picture.png, standard: Обмен}", "names more than one of a standard picture, a common picture and a file"},
		"файл и общая":       {"{file: Picture.png, common: " + commonPictureID + "}", "names more than one of a standard picture, a common picture and a file"},
		"не картинка":        {"{file: Module.bsl}", "picture.file must be the name of an image file"},
		"путь":               {"{file: ../Picture.png}", "picture.file must be the name of an image file"},
		"скрытый файл":       {"{file: .png}", "picture.file must be the name of an image file"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			id := uuid.MustNew().String()
			_, err := DecodeSubsystem("subsystem.yaml", strings.NewReader("format: 1\nid: "+id+"\nname: Продажи\ntitle: {ru: Продажи}\npicture: "+test.picture+"\n"),
				metadataConfiguration())
			if err == nil || !strings.Contains(err.Error(), test.refusal) {
				t.Fatalf("refused for another reason or not at all: %v", err)
			}
		})
	}
}

// The folders of a subsystem and of a group keep what each may: the help is
// a subsystem's, and neither keeps anything but images beside it. Defects
// caught: a group took a folder of help it cannot have; a folder took a file
// that is not an image.
func TestFolderOfASubsystemOrGroupKeepsOnlyWhatItMay(t *testing.T) {
	t.Parallel()
	owners := pictureFileOwners()
	for name, test := range map[string]struct {
		owner, file, refusal string
	}{
		"справка у группы":              {"группа команд", "help/ru.html", "a group's folder keeps only the file of its picture"},
		"текст у группы":                {"группа команд", "заметки.txt", "a group's folder keeps only the file of its picture"},
		"текст у подсистемы":            {"подсистема", "заметки.txt", "a subsystem's folder keeps only its help and the file of its picture"},
		"папка у подсистемы":            {"подсистема", "картинки/Picture.png", "a subsystem's folder keeps only its help and the file of its picture"},
		"папка без элемента":            {"точка карты маршрута", "../Финиш/Picture.png", "the route has no item of that name"},
		"файл прямо в маршруте":         {"точка карты маршрута", "../Picture.png", "the route has no item of that name"},
		"картинка у элемента без файла": {"точка карты маршрута", "../Завершение/Picture.png", "route item Завершение keeps \"Picture.png\", which its picture does not draw"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			place := owners[test.owner]
			root := metadataProject(t)
			place.write(t, root, "{file: Picture.png}")
			putFile(t, root, append(place.folder, "Picture.png")...)
			putFile(t, root, filepath.Clean(filepath.Join(append(place.folder, test.file)...)))
			if _, err := Load(root); err == nil || !strings.Contains(err.Error(), test.refusal) {
				t.Fatalf("refused for another reason or not at all: %v", err)
			}
		})
	}
}
