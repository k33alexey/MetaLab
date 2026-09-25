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
		"лестница без основания":           {[]string{"200.png", "400.png"}, []string{"no image at density 100", "falls back"}},
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
