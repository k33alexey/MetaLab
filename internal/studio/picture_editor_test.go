package studio

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
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
// refused with nothing changed; «Очистить» at a set of two images takes the
// set away, as the prototype's (obshchaya-kartinka.md, «Досъёмка
// 28.10.2026»). The project loads after each.
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
	if len(set.Set) != 2 || set.Set[0] != (metadata.PictureVariant{File: "100.png", Density: 100}) || set.Set[1].Density != 200 {
		t.Fatalf("the set of two images named by density: %+v", set.Set)
	}
	if emptied, err := workspace.SavePictureEditor(set.Path, PictureWindowValue{CommonPictureDefinition: set.Picture, Image: &PictureImage{}}, set.Revision); err != nil ||
		len(emptied.Images) != 0 || emptied.Set != nil {
		t.Fatalf("«Очистить» at a set left %v, %+v (%v)", emptied.Images, emptied.Set, err)
	}
	if _, err := metadata.Load(workspace.root); err != nil {
		t.Fatalf("the project does not load: %v", err)
	}
}

// TestTheWindowOfAPictureWritesItsSetOfVariants catches the set of
// variants of a picture saved wrong (ML-STUDIO.md, «Набор вариантов»):
// «Включить набор вариантов» keeps the one image as the one variant at
// density 100; the editor's set - the kept file with a direction, a new
// image for a browser, a new image at a density whose name a kept file
// takes - writes the variants and names the new files by their key; a file
// no variant of the old set named stays; a variant naming a file the folder
// does not have, and two variants of one key, are refused with nothing
// changed; «Выключить набор вариантов» makes the image of a variant the base
// image and takes every other image of the set away. The project loads
// after each.
func TestTheWindowOfAPictureWritesItsSetOfVariants(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	created, err := workspace.CreatePicture("Звезда")
	if err != nil {
		t.Fatal(err)
	}
	small, large, third := pngImage(t, 16, 16, 250), pngImage(t, 32, 32, 20), pngImage(t, 24, 24, 90)
	single, err := workspace.SavePictureEditor(created.Path, PictureWindowValue{CommonPictureDefinition: created.Picture,
		Image: &PictureImage{Format: "png", Content: small}}, created.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if single.Set != nil {
		t.Fatalf("a picture of one image shows a set: %+v", single.Set)
	}
	loads := func(when string) {
		t.Helper()
		if _, err := metadata.Load(workspace.root); err != nil {
			t.Fatalf("%s: the project does not load: %v", when, err)
		}
	}
	enabled, err := workspace.SavePictureEditor(single.Path, PictureWindowValue{CommonPictureDefinition: single.Picture,
		Set: []PictureSetVariant{{PictureVariant: metadata.PictureVariant{File: "100.png", Density: 100}}}}, single.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if want := []metadata.PictureVariant{{File: "100.png", Density: 100}}; !reflect.DeepEqual(enabled.Picture.Variants, want) ||
		!slices.Equal(enabled.Images, []string{"100.png"}) || !reflect.DeepEqual(enabled.Set, want) {
		t.Fatalf("turned on: %+v, images %v, set %+v", enabled.Picture.Variants, enabled.Images, enabled.Set)
	}
	loads("turned on")
	folder := filepath.Join(workspace.root, "metadata", "common-pictures", "Звезда")
	// A file no variant names lies in the folder, as the prototype keeps.
	if err := os.WriteFile(filepath.Join(folder, "старый.png"), third, 0o644); err != nil {
		t.Fatal(err)
	}
	opened, err := workspace.ReadPictureEditor(enabled.Path)
	if err != nil {
		t.Fatal(err)
	}
	browser := metadata.PictureWebBrowser("microsoft-internet-explorer-microsoft-edge-html")
	edited, err := workspace.SavePictureEditor(opened.Path, PictureWindowValue{CommonPictureDefinition: opened.Picture, Set: []PictureSetVariant{
		{PictureVariant: metadata.PictureVariant{File: "100.PNG", Density: 100, Direction: "rtl", GlyphWidth: 16, GlyphHeight: 16}},
		{PictureVariant: metadata.PictureVariant{Density: 200, WebBrowser: browser}, Image: &PictureImage{Format: "png", Content: large}},
		{PictureVariant: metadata.PictureVariant{Density: 100}, Image: &PictureImage{Format: "PNG", Content: third}},
	}}, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	want := []metadata.PictureVariant{{File: "100.png", Density: 100, Direction: "rtl", GlyphWidth: 16, GlyphHeight: 16},
		{File: "200-microsoft-internet-explorer-microsoft-edge-html.png", Density: 200, WebBrowser: browser},
		{File: "100-2.png", Density: 100}}
	if !reflect.DeepEqual(edited.Picture.Variants, want) || !slices.Equal(edited.Images,
		[]string{"100-2.png", "100.png", "200-microsoft-internet-explorer-microsoft-edge-html.png", "старый.png"}) {
		t.Fatalf("edited: %+v, images %v", edited.Picture.Variants, edited.Images)
	}
	for name, content := range map[string][]byte{"100.png": small, "200-microsoft-internet-explorer-microsoft-edge-html.png": large, "100-2.png": third} {
		if read, _, err := workspace.ReadPictureImage(edited.Path, name); err != nil || !bytes.Equal(read, content) {
			t.Errorf("%s reads %d bytes (%v)", name, len(read), err)
		}
	}
	loads("edited")

	for what, set := range map[string][]PictureSetVariant{
		"a file not in the folder": {{PictureVariant: metadata.PictureVariant{File: "300.png", Density: 300}}},
		"two variants of one key": {{PictureVariant: metadata.PictureVariant{File: "100.png", Density: 100}},
			{PictureVariant: metadata.PictureVariant{Density: 100}, Image: &PictureImage{Format: "png", Content: large}}},
		"an image of no format": {{PictureVariant: metadata.PictureVariant{Density: 150}, Image: &PictureImage{Format: "exe", Content: []byte("MZ")}}},
	} {
		if _, err := workspace.SavePictureEditor(edited.Path, PictureWindowValue{CommonPictureDefinition: edited.Picture, Set: set}, edited.Revision); err == nil {
			t.Errorf("%s was taken", what)
		}
		if again, err := workspace.ReadPictureEditor(edited.Path); err != nil || again.Revision != edited.Revision {
			t.Errorf("%s changed the picture (%v)", what, err)
		}
	}

	off, err := workspace.SavePictureEditor(edited.Path, PictureWindowValue{CommonPictureDefinition: edited.Picture,
		Image: &PictureImage{File: "200-microsoft-internet-explorer-microsoft-edge-html.png"}}, edited.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if off.Picture.Variants != nil || off.Set != nil || !slices.Equal(off.Images, []string{"100.png"}) {
		t.Fatalf("turned off: %+v, images %v", off.Picture.Variants, off.Images)
	}
	if read, _, _ := workspace.ReadPictureImage(off.Path, "100.png"); !bytes.Equal(read, large) {
		t.Error("turned off: the base image is not the variant's")
	}
	loads("turned off")
	if _, err := workspace.SavePictureEditor(off.Path, PictureWindowValue{CommonPictureDefinition: off.Picture,
		Image: &PictureImage{File: "../object.yaml"}}, off.Revision); err == nil {
		t.Error("an image named outside the images of the picture was taken")
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
