package studio

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
)

// pngImage is a picture of one colour, w by h, as a file of PNG.
func pngImage(t *testing.T, w, h int, shade uint8) []byte {
	t.Helper()
	drawn := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			drawn.Set(x, y, color.RGBA{R: shade, G: 200, B: 40, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, drawn); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

// TestTheWindowOfAPictureWritesItsImage catches the window of a common
// picture losing or misplacing what it writes: «Добавить» makes
// ОбщаяКартинка1 with no image and both availabilities off (the help's
// defaults); «Выбрать из файла» and «Сохранить» put the image as the base
// one, 100.png, with the flag; an image changed on disk since the window
// read it refuses the save; a new name takes the folder along while
// «Очистить» leaves no image; a file of a format no picture is kept in is
// refused with nothing changed; a picture of a set is not given an image by
// the window. The project loads after each.
func TestTheWindowOfAPictureWritesItsImage(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	created, err := workspace.CreatePicture("")
	if err != nil {
		t.Fatal(err)
	}
	if created.Path != "metadata/common-pictures/ОбщаяКартинка1/object.yaml" || len(created.Images) != 0 ||
		created.Picture.AvailableForChoice || created.Picture.AvailableForAppearance || created.KindTitle != "Общая картинка" {
		t.Fatalf("the new picture is %+v at %s with %v", created.Picture, created.Path, created.Images)
	}
	star := pngImage(t, 16, 16, 250)
	edited := created.Picture
	edited.AvailableForChoice = true
	saved, err := workspace.SavePictureEditor(created.Path, PictureWindowValue{CommonPictureDefinition: edited, Image: &PictureImage{Format: "png", Content: star}}, created.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(saved.Images, []string{"100.png"}) || !saved.Picture.AvailableForChoice {
		t.Fatalf("saved: images %v, picture %+v", saved.Images, saved.Picture)
	}
	content, media, err := workspace.ReadPictureImage(saved.Path, "100.png")
	if err != nil || !bytes.Equal(content, star) || media != "image/png" {
		t.Fatalf("the image reads %d bytes of %q (%v)", len(content), media, err)
	}
	if _, err := metadata.Load(workspace.root); err != nil {
		t.Fatalf("the project with the image does not load: %v", err)
	}

	folder := filepath.Join(workspace.root, "metadata", "common-pictures", "ОбщаяКартинка1")
	if err := os.WriteFile(filepath.Join(folder, "100.png"), pngImage(t, 16, 16, 10), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SavePictureEditor(saved.Path, PictureWindowValue{CommonPictureDefinition: saved.Picture}, saved.Revision); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("an image changed on disk did not refuse the save: %v", err)
	}
	reread, err := workspace.ReadPictureEditor(saved.Path)
	if err != nil {
		t.Fatal(err)
	}
	renamed := reread.Picture
	renamed.Name = "Звезда"
	cleared, err := workspace.SavePictureEditor(reread.Path, PictureWindowValue{CommonPictureDefinition: renamed, Image: &PictureImage{}}, reread.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.Path != "metadata/common-pictures/Звезда/object.yaml" || len(cleared.Images) != 0 {
		t.Fatalf("renamed and cleared: %s with %v", cleared.Path, cleared.Images)
	}
	if _, err := os.Stat(folder); !os.IsNotExist(err) {
		t.Errorf("the old folder is still there: %v", err)
	}

	if _, err := workspace.SavePictureEditor(cleared.Path, PictureWindowValue{CommonPictureDefinition: cleared.Picture, Image: &PictureImage{Format: "exe", Content: []byte("MZ")}}, cleared.Revision); err == nil {
		t.Fatal("a file of no picture format was taken")
	}
	if again, err := workspace.ReadPictureEditor(cleared.Path); err != nil || len(again.Images) != 0 || again.Revision != cleared.Revision {
		t.Fatalf("the refused format left %v at %s (%v)", again.Images, again.Revision, err)
	}

	other := filepath.Join(workspace.root, "metadata", "common-pictures", "Звезда")
	for _, name := range []string{"100.png", "200.png"} {
		if err := os.WriteFile(filepath.Join(other, name), star, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	set, err := workspace.ReadPictureEditor(cleared.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SavePictureEditor(set.Path, PictureWindowValue{CommonPictureDefinition: set.Picture, Image: &PictureImage{}}, set.Revision); err == nil {
		t.Fatal("the window cleared a set of two images")
	}
	if _, err := metadata.Load(workspace.root); err != nil {
		t.Fatalf("the project does not load: %v", err)
	}
}

// TestARefusedImagePutsThePictureBack catches a save of the window of a
// picture that fails while setting the old images aside and leaves the
// description written, or the old image gone: the description goes back as
// it was read, the image stays, and the failure is reported.
func TestARefusedImagePutsThePictureBack(t *testing.T) {
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	created, err := workspace.CreatePicture("")
	if err != nil {
		t.Fatal(err)
	}
	old := pngImage(t, 8, 8, 100)
	saved, err := workspace.SavePictureEditor(created.Path, PictureWindowValue{CommonPictureDefinition: created.Picture, Image: &PictureImage{Format: "png", Content: old}}, created.Revision)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { renameFolder = os.Rename }()
	edited := saved.Picture
	edited.AvailableForChoice = true
	// The first step - setting the old image aside - fails, and then the
	// second - putting the new one in its place - after the old one is
	// aside already.
	for call := 1; call <= 2; call++ {
		calls := 0
		renameFolder = func(from, to string) error {
			calls++
			if calls == call {
				return errors.New("отказ диска")
			}
			return os.Rename(from, to)
		}
		_, err = workspace.SavePictureEditor(saved.Path, PictureWindowValue{CommonPictureDefinition: edited, Image: &PictureImage{Format: "png", Content: pngImage(t, 8, 8, 1)}}, saved.Revision)
		if err == nil || !strings.Contains(err.Error(), "отказ диска") {
			t.Fatalf("step %d: the refused image answered %v", call, err)
		}
		renameFolder = os.Rename
		after, err := workspace.ReadPictureEditor(saved.Path)
		if err != nil {
			t.Fatal(err)
		}
		content, _, err := workspace.ReadPictureImage(saved.Path, "100.png")
		if after.Picture.AvailableForChoice || !slices.Equal(after.Images, []string{"100.png"}) || err != nil || !bytes.Equal(content, old) || after.Revision != saved.Revision {
			t.Fatalf("step %d: after the refusal %+v, images %v, the old image kept %v (%v)", call, after.Picture, after.Images, bytes.Equal(content, old), err)
		}
	}
}

// TestAnImageOfAPictureIsReadOnlyFromItsFolder catches the reading of an
// image reaching past the folder of the picture, or reading its description
// as an image.
func TestAnImageOfAPictureIsReadOnlyFromItsFolder(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	created, err := workspace.CreatePicture("")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../../../configuration.yaml", "object.yaml", "../Справочник1.png", ".hidden.png"} {
		if _, _, err := workspace.ReadPictureImage(created.Path, name); !errors.Is(err, ErrInvalidSourcePath) {
			t.Errorf("%q answered %v, want a refusal of the path", name, err)
		}
	}
	if _, _, err := workspace.ReadPictureImage(created.Path, "100.png"); !errors.Is(err, ErrSourceNotFound) {
		t.Errorf("a missing image answered %v", err)
	}
	if _, _, err := workspace.ReadPictureImage("metadata/catalogs/Товары/object.yaml", "100.png"); !errors.Is(err, ErrInvalidSourcePath) {
		t.Errorf("an image of a catalog answered %v", err)
	}
}
