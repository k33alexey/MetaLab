package studio

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/k33alexey/MetaLab/internal/atomicfile"
	"github.com/k33alexey/MetaLab/internal/icons"
	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The window of a common picture (docs/block-00/obshchaya-kartinka.md,
// «Окно»): the name, the synonym, the comment, the subsystems, the two
// flags of availability, and the picture with «Выбрать из файла»,
// «Очистить» and «Сохранить в файл» beside it and its type below. The set of
// variants, its editor and the transparency are items of their own; their
// buttons stand unavailable until then. There is no editor of pictures
// (the owner, 15.10.2026, ML-STUDIO.md, «Окно общей картинки»).

// PictureEditorSource is a common picture as its window reads and writes it.
type PictureEditorSource struct {
	Path            string                           `json:"path"`
	Revision        string                           `json:"revision"`
	Picture         metadata.CommonPictureDefinition `json:"picture"`
	Languages       []project.Language               `json:"languages"`
	DefaultLanguage string                           `json:"defaultLanguage"`
	KindTitle       string                           `json:"kindTitle"`
	// Palette is the tabs of the palette of a picture and their rows, the
	// kind's (kinds.Palette).
	Palette []kinds.PaletteTab `json:"palette"`
	// Images are the files of the picture in its folder, by name; one
	// without variants is the picture the window shows, more or variants
	// make a set.
	Images []string `json:"images"`
	// Subsystems are the subsystems of the project, those holding the
	// picture marked: the field «Подсистемы» lists them.
	Subsystems []MemberPlace `json:"subsystems"`
	// description is the revision of the description alone, which its save
	// checks; Revision holds the images too.
	description string
}

// PictureWindowValue is what the window of a picture writes: the picture
// and, when «Выбрать из файла» or «Очистить» was pressed, its new image.
type PictureWindowValue struct {
	metadata.CommonPictureDefinition
	Image *PictureImage `json:"image,omitempty"`
}

// PictureImage is the image a window gives a picture: a file of a format a
// picture is kept in, by the extension of its name, with its content; no
// content clears the picture.
type PictureImage struct {
	Format  string `json:"format,omitempty"`
	Content []byte `json:"content,omitempty"`
}

// pictureEditorAddress is where the window of a picture is read and
// written, and pictureImageAddress where it reads an image of it.
const (
	pictureEditorAddress = "/api/picture"
	pictureImageAddress  = "/api/picture/image"
)

func (workspace *Workspace) ReadPictureEditor(relative string) (PictureEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readPictureEditorLocked(relative)
}

func (workspace *Workspace) readPictureEditorLocked(relative string) (PictureEditorSource, error) {
	if err := validateObjectEditorPath(relative, metadata.CommonPictureKind); err != nil {
		return PictureEditorSource{}, err
	}
	file, err := workspace.readSource(relative)
	if err != nil {
		return PictureEditorSource{}, err
	}
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return PictureEditorSource{}, err
	}
	value, err := metadata.DecodeCommonPicture(relative, strings.NewReader(file.Content), configuration)
	if err != nil {
		return PictureEditorSource{}, err
	}
	images, digest, err := workspace.pictureImagesLocked(relative)
	if err != nil {
		return PictureEditorSource{}, err
	}
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		return PictureEditorSource{}, err
	}
	revision := sha256.New()
	revision.Write([]byte(file.Revision))
	revision.Write(digest)
	return PictureEditorSource{
		Path: relative, Revision: hex.EncodeToString(revision.Sum(nil)), description: file.Revision, Picture: value, Images: images,
		Languages: editorLanguages(configuration), DefaultLanguage: configuration.DefaultLanguage,
		KindTitle: kinds.Must(metadata.CommonPictureKind).Title, Palette: kinds.Must(metadata.CommonPictureKind).Palette(),
		Subsystems: subsystemPlaces(loaded, value.ID),
	}, nil
}

// pictureImagesLocked lists the images in the folder of the picture
// described at relative, by name, and a digest of their names and content:
// an image changed outside the window changes the revision of the window.
func (workspace *Workspace) pictureImagesLocked(relative string) ([]string, []byte, error) {
	directory := filepath.Join(workspace.root, filepath.FromSlash(path.Dir(relative)))
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, nil, err
	}
	images := []string{}
	digest := sha256.New()
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 || !metadata.PictureFile(entry.Name()) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, nil, err
		}
		images = append(images, entry.Name())
		digest.Write([]byte(entry.Name()))
		digest.Write([]byte{0})
		contentDigest := sha256.Sum256(content)
		digest.Write(contentDigest[:])
	}
	return images, digest.Sum(nil), nil
}

// ReadPictureImage answers an image of the picture described at relative,
// by its name in the folder, and its media type.
func (workspace *Workspace) ReadPictureImage(relative, name string) ([]byte, string, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	if err := validateObjectEditorPath(relative, metadata.CommonPictureKind); err != nil {
		return nil, "", err
	}
	if !metadata.PictureFile(name) {
		return nil, "", ErrInvalidSourcePath
	}
	file := filepath.Join(workspace.root, filepath.FromSlash(path.Dir(relative)), name)
	info, err := os.Lstat(file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, "", ErrSourceNotFound
	}
	if err != nil || !info.Mode().IsRegular() {
		return nil, "", ErrInvalidSourcePath
	}
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, "", err
	}
	return content, mime.TypeByExtension(strings.ToLower(filepath.Ext(name))), nil
}

// SavePictureEditor writes the picture and its new image, if the window
// gave one, as one save: a refusal halfway puts back the images, the
// description and the folder. «Выбрать из файла» makes the image the one
// image of the picture, its base drawn at density 100 (metadata.
// PictureDensity); «Очистить» leaves the picture with none, which the model
// accepts. A picture keeping a set of variants is not given an image here:
// the set is changed by its own editor.
func (workspace *Workspace) SavePictureEditor(relative string, window PictureWindowValue, revision string) (PictureEditorSource, error) {
	value := window.CommonPictureDefinition
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	opened, err := workspace.readPictureEditorLocked(relative)
	if err != nil {
		return PictureEditorSource{}, err
	}
	if opened.Revision != revision {
		return PictureEditorSource{}, ErrSourceChanged
	}
	if value.ID != opened.Picture.ID {
		return PictureEditorSource{}, fmt.Errorf("picture UUID cannot be changed")
	}
	if value.Name != opened.Picture.Name {
		if err := workspace.objectNameErrorLocked(relative, "", value.Name); err != nil {
			return PictureEditorSource{}, err
		}
	}
	// The window changes what it shows, not the drawing of the files:
	// variants and transparency stay as the description has them.
	value.PictureImages = opened.Picture.PictureImages
	if window.Image != nil {
		if len(opened.Picture.Variants) > 0 || len(opened.Images) > 1 {
			return PictureEditorSource{}, fmt.Errorf("the picture keeps a set of variants: its images are changed by the editor of the set")
		}
		if len(window.Image.Content) > 0 && !metadata.PictureFile(baseImage(window.Image.Format)) {
			return PictureEditorSource{}, fmt.Errorf("an image of a picture is one of %s", strings.Join(metadata.PictureFormats(), ", "))
		}
	}
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, value); err != nil {
		return PictureEditorSource{}, err
	}
	written, moveBack, err := workspace.followNameLocked(relative, opened.Picture.Name, value.Name)
	if err != nil {
		return PictureEditorSource{}, err
	}
	original, err := workspace.readSource(written)
	if err != nil {
		return PictureEditorSource{}, errors.Join(err, moveBack())
	}
	if _, err := workspace.saveSourceLocked(written, encoded.String(), opened.description); err != nil {
		return PictureEditorSource{}, errors.Join(err, moveBack())
	}
	if window.Image != nil {
		if err := workspace.replacePictureImageLocked(written, opened.Images, *window.Image); err != nil {
			// The description goes back as it was read, then the folder.
			restored := atomicfile.Write(filepath.Join(workspace.root, filepath.FromSlash(written)), ".ml-write-*", 0o644, atomicfile.Bytes([]byte(original.Content)))
			workspace.invalidateStudioIndexesLocked()
			return PictureEditorSource{}, errors.Join(err, restored, moveBack())
		}
	}
	return workspace.readPictureEditorLocked(written)
}

// baseImage is the name of the base image of a picture in a format: the
// image drawn at density 100.
func baseImage(format string) string { return "100." + strings.ToLower(format) }

// replacePictureImageLocked puts image in place of the images of the
// picture described at relative: the new one is written beside them first,
// they are set aside, and it takes its name; a refusal on the way brings
// them back. No content leaves the picture with no image.
func (workspace *Workspace) replacePictureImageLocked(relative string, images []string, image PictureImage) error {
	directory := filepath.Join(workspace.root, filepath.FromSlash(path.Dir(relative)))
	defer workspace.invalidateStudioIndexesLocked()
	var prepared string
	if len(image.Content) > 0 {
		var err error
		if prepared, err = atomicfile.Prepare(directory, ".ml-write-*", 0o644, atomicfile.Bytes(image.Content)); err != nil {
			return err
		}
		defer os.Remove(prepared)
	}
	type aside struct{ name, temporary string }
	var set []aside
	putBack := func() error {
		var failed error
		for index := len(set) - 1; index >= 0; index-- {
			if err := renameFolder(set[index].temporary, filepath.Join(directory, set[index].name)); err != nil && failed == nil {
				failed = err
			}
		}
		return failed
	}
	for _, name := range images {
		temporary := filepath.Join(directory, ".ml-old-"+name)
		if err := renameFolder(filepath.Join(directory, name), temporary); err != nil {
			return errors.Join(err, putBack())
		}
		set = append(set, aside{name, temporary})
	}
	// The name of the new image is free now: the old ones are aside.
	if prepared != "" {
		if err := renameFolder(prepared, filepath.Join(directory, baseImage(image.Format))); err != nil {
			return errors.Join(err, putBack())
		}
	}
	for _, item := range set {
		_ = os.Remove(item.temporary)
	}
	return nil
}

// PictureLibraryItem is a common picture as «Библиотека картинок
// конфигурации» lists it (docs/block-00/obshchaya-kartinka.md, «Досъёмка
// 27.10.2026»): where it is described, its name, the image it shows - none
// for a picture of none - and whether it keeps a set of variants, which
// the library neither shows nor clears.
type PictureLibraryItem struct {
	ID    uuid.UUID `json:"id"`
	Path  string    `json:"path"`
	Name  string    `json:"name"`
	Image string    `json:"image,omitempty"`
	Set   bool      `json:"set,omitempty"`
}

// PictureLibrary lists the common pictures of the project in the order of
// the tree.
func (workspace *Workspace) PictureLibrary() ([]PictureLibraryItem, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		return nil, err
	}
	items := []PictureLibraryItem{}
	for _, object := range loaded.KindObjects(metadata.CommonPictureKind) {
		relative, err := project.ObjectMetadataPath(string(metadata.CommonPictureKind), object.Name)
		if err != nil {
			return nil, err
		}
		images, _, err := workspace.pictureImagesLocked(relative)
		if err != nil {
			return nil, err
		}
		picture, _ := loaded.CommonPicture(object.Name)
		item := PictureLibraryItem{ID: object.ID, Path: relative, Name: object.Name, Set: len(picture.Variants) > 0 || len(images) > 1}
		if len(images) == 1 && !item.Set {
			item.Image = images[0]
		}
		items = append(items, item)
	}
	return items, nil
}

// PictureChoice is what a field of a picture and the window «Выбор
// картинки» name pictures by (docs/block-00/standartnye-kartinki.md): the
// common pictures of the project in the order of the tree, by identifier
// and name, and the standard pictures of the platform in the order of the
// help - the name a reference writes, the Russian name the window shows,
// the icon drawing it, and whether the window hides it. Their images the
// window reads by PictureLibrary when it opens.
type PictureChoice struct {
	Common   []PicturePlace         `json:"common"`
	Standard []StandardPicturePlace `json:"standard"`
}

// PicturePlace is a common picture by its identifier and name.
type PicturePlace struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// StandardPicturePlace is a picture of the platform's library
// (icons.StandardPicture).
type StandardPicturePlace struct {
	Name    string `json:"name"`
	Russian string `json:"russian"`
	Icon    string `json:"icon,omitempty"`
	Hidden  bool   `json:"hidden,omitempty"`
}

func pictureChoice(loaded *metadata.Catalog) PictureChoice {
	choice := PictureChoice{Common: []PicturePlace{}}
	for _, object := range loaded.KindObjects(metadata.CommonPictureKind) {
		choice.Common = append(choice.Common, PicturePlace{ID: object.ID, Name: object.Name})
	}
	for _, picture := range icons.StandardPictures() {
		choice.Standard = append(choice.Standard, StandardPicturePlace{Name: picture.Name, Russian: picture.Russian, Icon: picture.Icon, Hidden: picture.Hidden})
	}
	return choice
}

// ClearPicture takes the image of a picture away at once - «Очистить» of
// the library, with no question (obshchaya-kartinka.md, «Досъёмка
// 27.10.2026») - by the save of its window, at the revision read now.
func (workspace *Workspace) ClearPicture(relative string) (PictureEditorSource, error) {
	opened, err := workspace.ReadPictureEditor(relative)
	if err != nil {
		return PictureEditorSource{}, err
	}
	return workspace.SavePictureEditor(relative, PictureWindowValue{CommonPictureDefinition: opened.Picture, Image: &PictureImage{}}, opened.Revision)
}

// CreatePicture makes the next common picture by the one way every object
// is made (CreateObject) and opens its window.
func (workspace *Workspace) CreatePicture(name string) (PictureEditorSource, error) {
	return createObjectAs(workspace, metadata.CommonPictureKind, name, nil, workspace.readPictureEditorLocked)
}

// DeletePicture removes a common picture's folder - its description and
// images - by the one order of steps of every deletion (deleteObjectLocked).
func (workspace *Workspace) DeletePicture(relative, expectedRevision string) error {
	return deleteObjectFolder(workspace, metadata.CommonPictureKind, relative, expectedRevision, workspace.readPictureEditorLocked)
}

func registerPictureEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("GET "+pictureImageAddress, func(response http.ResponseWriter, request *http.Request) {
		content, media, err := workspace.ReadPictureImage(request.URL.Query().Get("path"), request.URL.Query().Get("name"))
		if err != nil {
			writeSourceError(response, err)
			return
		}
		if media != "" {
			response.Header().Set("Content-Type", media)
		}
		response.Header().Set("Cache-Control", "no-store")
		_, _ = response.Write(content)
	})
	routes.HandleFunc("GET "+pictureEditorAddress+"/library", answerQuery(writeSourceError, func(*http.Request) (any, error) {
		return workspace.PictureLibrary()
	}))
	routes.HandleFunc("POST "+pictureEditorAddress+"/clear", answerStudio(0, writeSourceError, func(input struct {
		Path string `json:"path"`
	}) (any, error) {
		return workspace.ClearPicture(input.Path)
	}))
	registerObjectEditor(routes, pictureEditorAddress, workspace.ReadPictureEditor, workspace.SavePictureEditor, workspace.CreatePicture, workspace.DeletePicture)
}
