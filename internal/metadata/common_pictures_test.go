package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	commonPictureID     = "d0000000-0000-4000-8000-000000000001"
	commonPictureSecond = "d0000000-0000-4000-8000-000000000002"
)

// writeCommonPicture writes one picture of the configuration's picture
// library, without any image of its own.
func writeCommonPicture(t *testing.T, root, id, name, properties string) {
	t.Helper()
	writeMetadata(t, root, CommonPictureKind, id, `format: 1
id: `+id+`
name: `+name+`
title: {ru: `+name+`}
`+properties)
}

// writeCommonPictureImage writes one image of a picture. What density it
// stands for is what it is called: the file is the whole declaration.
func writeCommonPictureImage(t *testing.T, root, name, file string) {
	t.Helper()
	path, err := project.CommonPictureImagePath(name, file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte("образ"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A picture carries what it is offered for and the image at every step of the
// ladder it has. The two flags decide whether a user is shown the picture at
// all while the application runs, so losing them loses the picture from two
// dialogs at once.
func TestCommonPictureCarriesItsAvailabilityAndItsDensities(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeCommonPicture(t, root, commonPictureID, "Печать",
		"available_for_appearance: true\navailable_for_choice: true\ncomment: Знак печати\n")
	for _, file := range []string{"85.png", "100.png", "125.png", "150.png", "175.png", "200.png", "300.svg", "400.svg"} {
		writeCommonPictureImage(t, root, "Печать", file)
	}
	writeCommonPicture(t, root, commonPictureSecond, "Обмен", "")
	writeCommonPictureImage(t, root, "Обмен", "100.svg")

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	picture, ok := catalog.CommonPicture("Печать")
	if !ok {
		t.Fatal("the picture was lost")
	}
	if !picture.AvailableForAppearance || !picture.AvailableForChoice || picture.Comment != "Знак печати" {
		t.Fatalf("the picture came back without what it is offered for: %+v", picture)
	}
	second, err := uuid.Parse(commonPictureSecond)
	if err != nil {
		t.Fatal(err)
	}
	plain, ok := catalog.CommonPictureByID(second)
	if !ok {
		t.Fatal("a picture with one image is not found by its identifier")
	}
	if plain.AvailableForAppearance || plain.AvailableForChoice {
		t.Fatalf("a picture nobody offered came back offered: %+v", plain)
	}

	picture.Name = "Подменено"
	again, _ := catalog.CommonPicture("Печать")
	if again.Name == "Подменено" {
		t.Fatal("common picture lookup exposed mutable metadata")
	}
}

// A picture with no image at all is ordinary while no editor writes one.
func TestCommonPictureWithoutImageLoads(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeCommonPicture(t, root, commonPictureID, "Пустая", "")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := catalog.CommonPicture("Пустая"); !ok {
		t.Fatal("a picture without an image was lost")
	}
}

func TestCommonPictureFolderIsChecked(t *testing.T) {
	t.Parallel()
	for name, expected := range map[string]struct {
		files     []string
		complains []string
	}{
		"одна плотность дважды":            {[]string{"100.png", "100.svg"}, []string{"keeps both", "100.png", "100.svg"}},
		"не плотность вовсе":               {[]string{"100.png", "Печать.png"}, []string{"Печать.png", "named by the density"}},
		"плотность не с лестницы":          {[]string{"100.png", "110.png"}, []string{"110.png", "named by the density"}},
		"формат, который нечем нарисовать": {[]string{"100.png", "200.psd"}, []string{"200.psd", "named by the density"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeCommonPicture(t, root, commonPictureID, "Печать", "")
			for _, file := range expected.files {
				writeCommonPictureImage(t, root, "Печать", file)
			}
			_, err := Load(root)
			if err == nil {
				t.Fatal("the project loaded")
			}
			for _, complaint := range expected.complains {
				if !strings.Contains(err.Error(), complaint) {
					t.Fatalf("the complaint does not say %q: %v", complaint, err)
				}
			}
		})
	}
}

// The prototype saves a picture without its base density - sb keeps four drawn
// at 150 alone - and a screen asking for another density is given the nearest
// one. Defect caught: such a picture was refused, and the configuration with it.
func TestCommonPictureWithoutTheBaseDensityLoads(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeCommonPicture(t, root, commonPictureID, "Радиус", "")
	for _, file := range []string{"150.png", "400.png"} {
		writeCommonPictureImage(t, root, "Радиус", file)
	}
	if _, err := Load(root); err != nil {
		t.Fatalf("a picture without its base density was refused: %v", err)
	}
}

// A picture of the prototype is a set of variants: a file, the density and the
// interface it is drawn at, whether it is a template, its glyph size - and one
// file drawn for several variants (help, the editor of picture variants; erp
// 874 pictures this way). It is also loaded transparent with the colour of one
// pixel. Defect caught: the folder knew only files named by density, so the
// interface, the template flag, the glyph size, the shared file and the
// transparency were lost at import.
func TestCommonPictureCarriesItsVariantsAndTransparency(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeCommonPicture(t, root, commonPictureID, "Skype", `load_transparent: true
transparent_pixel: {x: 14, y: 8}
variants:
  - {file: 100.png, density: 100, glyph_width: 16, glyph_height: 16}
  - {file: 200.png, density: 200, template: true}
  - {file: Picture.png, density: 100, interface: "8.2", glyph_width: 16, glyph_height: 16}
  - {file: Picture.png, density: 100, interface: 8.2-ordinary-application}
  - {file: l.png, density: 150, interface: taxi-mobile}
`)
	// The prototype finds a file with case ignored, and keeps files no
	// variant names.
	for _, file := range []string{"100.png", "200.png", "picture.png", "l.png", "Картинка.png"} {
		writeCommonPictureImage(t, root, "Skype", file)
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a picture with variants was refused: %v", err)
	}
	picture, _ := catalog.CommonPicture("Skype")
	switch {
	case !picture.LoadTransparent || picture.TransparentPixel == nil || *picture.TransparentPixel != (PicturePixel{X: 14, Y: 8}):
		t.Fatalf("the transparency was lost: %+v", picture)
	case len(picture.Variants) != 5:
		t.Fatalf("the variants were lost: %+v", picture.Variants)
	case picture.Variants[0].GlyphWidth != 16 || picture.Variants[0].GlyphHeight != 16:
		t.Fatalf("the glyph size was lost: %+v", picture.Variants[0])
	case !picture.Variants[1].Template || picture.Variants[0].Template:
		t.Fatalf("the template flag was lost or invented: %+v", picture.Variants)
	case picture.Variants[2].Interface != Version82PictureInterface || picture.Variants[3].Interface != Version82OrdinaryInterface ||
		picture.Variants[4].Interface != TaxiMobilePictureInterface || picture.Variants[0].Interface != CurrentPictureInterface:
		t.Fatalf("the interfaces were lost: %+v", picture.Variants)
	}
	picture.Variants[0].File = "Подменено"
	picture.TransparentPixel.X = 0
	again, _ := catalog.CommonPicture("Skype")
	if again.Variants[0].File != "100.png" || again.TransparentPixel.X != 14 {
		t.Fatal("common picture lookup exposed mutable metadata")
	}

	// A variant drawing a file that is not in the folder draws nothing.
	writeCommonPicture(t, root, commonPictureID, "Skype", `variants:
  - {file: 100.png, density: 100}
  - {file: 300.png, density: 300}
`)
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "300.png, which is not in its folder") {
		t.Fatalf("a variant without its file: %v", err)
	}
	// With variants, what lies in the folder still has to be an image.
	writeCommonPicture(t, root, commonPictureID, "Skype", "variants: [{file: 100.png, density: 100}]\n")
	if err := os.WriteFile(filepath.Join(root, "metadata", string(CommonPictureKind), "Skype", "заметки.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "which is not an image") {
		t.Fatalf("a file that is not an image beside variants: %v", err)
	}
}

// Two files differing in case alone cannot both be the one a variant names,
// since the prototype finds a file with case ignored. Only a file system that
// tells them apart can hold both, so elsewhere there is nothing to test.
func TestCommonPictureFilesDifferingInCaseAreRefused(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeCommonPicture(t, root, commonPictureID, "Skype", "variants: [{file: picture.png, density: 100}]\n")
	writeCommonPictureImage(t, root, "Skype", "picture.png")
	writeCommonPictureImage(t, root, "Skype", "Picture.png")
	entries, err := os.ReadDir(filepath.Join(root, "metadata", string(CommonPictureKind), "Skype"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 3 {
		t.Skip("this file system does not tell names apart by case")
	}
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "differ in case alone") {
		t.Fatalf("two files differing in case: %v", err)
	}
}

// What a picture says about itself is checked where it is written down.
func TestBrokenCommonPicturesAreRefused(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ body, want string }{
		"пиксель без прозрачности": {"transparent_pixel: {x: 1, y: 1}", "belongs to a picture loaded transparent only"},
		"пиксель за краем":         {"load_transparent: true\ntransparent_pixel: {x: -1, y: 1}", "must be a point inside the image"},
		"плотность не с лестницы":  {"variants: [{file: 110.png, density: 110}]", "density must be one of"},
		"интерфейс не существует":  {"variants: [{file: 100.png, density: 100, interface: windows-95}]", "interface must be"},
		"глиф отрицательный":       {"variants: [{file: 100.png, density: 100, glyph_width: -1}]", "negative glyph size"},
		"файл в чужой папке":       {"variants: [{file: ../100.png, density: 100}]", "must be the name of an image file"},
		"файл не образ":            {"variants: [{file: 100.txt, density: 100}]", "must be the name of an image file"},
		"описание вместо образа":   {"variants: [{file: object.yaml, density: 100}]", "must be the name of an image file"},
		"два образа на одно место": {"variants:\n  - {file: a.png, density: 100, interface: \"8.2\"}\n  - {file: b.png, density: 100, interface: \"8.2\"}",
			"second image for the same density and interface"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeCommonPicture("object.yaml", strings.NewReader(`format: 1
id: `+commonPictureID+`
name: Печать
title: {ru: Печать}
`+broken.body+`
`), metadataConfiguration())
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
	// One file for two interfaces at one density is the ordinary case.
	if _, err := DecodeCommonPicture("object.yaml", strings.NewReader(`format: 1
id: `+commonPictureID+`
name: Печать
title: {ru: Печать}
variants:
  - {file: Picture.png, density: 100, interface: "8.2"}
  - {file: Picture.png, density: 100, interface: 8.2-ordinary-application}
`), metadataConfiguration()); err != nil {
		t.Fatalf("one file for two interfaces: %v", err)
	}
}

// A command shown with a picture that was never added is a blank place in the
// interface: the platform draws nothing, and a command shown as a picture
// alone becomes unreadable. Until pictures were objects there was nothing to
// check this against.
func TestCommandIsShownWithAPictureThatExists(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeCommonPicture(t, root, commonPictureID, "Обмен", "")
	writeCommonCommand(t, root, "Обменяться", `format: 1
id: `+commonCommandID+`
name: Обменяться
title: {ru: Обменяться}
picture: {common: `+commonPictureSecond+`}
`, true)
	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), commonPictureSecond) {
		t.Fatalf("a command shown with a picture nobody added loaded: %v", err)
	}

	writeCommonCommand(t, root, "Обменяться", `format: 1
id: `+commonCommandID+`
name: Обменяться
title: {ru: Обменяться}
picture: {common: `+commonPictureID+`}
`, true)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	command, ok := catalog.CommonCommand("Обменяться")
	if !ok || command.Picture == nil || command.Picture.Common == nil ||
		command.Picture.Common.String() != commonPictureID {
		t.Fatalf("the command lost the picture it is shown with: %+v", command)
	}

	// A picture named anywhere may be loaded transparent (commands 99 times
	// in erp). Defect caught: the flag was not in the reference and was lost.
	writeCommonCommand(t, root, "Обменяться", `format: 1
id: `+commonCommandID+`
name: Обменяться
title: {ru: Обменяться}
picture: {standard: Обмен, load_transparent: true}
`, true)
	catalog, err = Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if command, _ := catalog.CommonCommand("Обменяться"); command.Picture == nil || !command.Picture.LoadTransparent {
		t.Fatalf("the command lost loading its picture transparent: %+v", command.Picture)
	}
}

// A group of commands names a picture the same way a command does, and it is
// checked the same way.
func TestCommandGroupIsShownWithAPictureThatExists(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CommandGroupKind, commandGroupID, `format: 1
id: `+commandGroupID+`
name: Обмены
title: {ru: Обмены}
category: actions-panel
picture: {common: `+commonPictureID+`}
`)
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), commonPictureID) {
		t.Fatalf("a group shown with a picture nobody added loaded: %v", err)
	}
	writeCommonPicture(t, root, commonPictureID, "Обмен", "")
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}
}

// An object's own command names a picture too, and the check reaches it
// through the same list that keeps every object's commands in one place.
func TestObjectCommandIsShownWithAPictureThatExists(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, templateCatalog, `format: 1
id: `+templateCatalog+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true}
description_length: 150
commands:
  - {id: `+commandID+`, name: Открыть, title: {ru: Открыть}, picture: {common: `+commonPictureID+`}}
`)
	writeCommandModule(t, root, CatalogKind, "Контрагенты", "Открыть")
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), commonPictureID) {
		t.Fatalf("an object command shown with a picture nobody added loaded: %v", err)
	}
	writeCommonPicture(t, root, commonPictureID, "Обмен", "")
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}
}
