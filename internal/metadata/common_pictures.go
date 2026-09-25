package metadata

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// CommonPictureKind holds the images the configuration offers by name: to its
// own commands and groups, to the cells of a spreadsheet, to the conditional
// appearance a user sets up while the application runs.
const CommonPictureKind Kind = "common-pictures"

// ScreenDensity is how many pixels a screen spends on one unit of interface,
// written as a percentage of the base. The same picture is drawn from a
// different file on a dense screen, so that an icon stays an icon instead of
// being stretched into a blur.
//
// The ladder has eight steps and 100 is the base: every other step is that
// same image at another size, and a density a screen asks for and the picture
// does not have falls back to the base.
type ScreenDensity int

// screenDensities is the ladder, from the smallest step to the largest.
var screenDensities = []ScreenDensity{85, 100, 125, 150, 175, 200, 300, 400}

// BaseScreenDensity is the step every picture must have: without it there is
// nothing to fall back to, and a screen whose density the picture skips would
// have no image at all.
const BaseScreenDensity ScreenDensity = 100

// pictureFormats are the image formats a picture may be stored in. The list is
// closed on purpose: a format nothing can draw is a picture that silently does
// not appear, and that is indistinguishable from a picture nobody added.
var pictureFormats = []string{"png", "svg", "gif", "ico", "jpg", "bmp"}

// CommonPictureDefinition is one image of the configuration's picture library.
//
// The image itself is not a property but a file, the same way a template's
// content is: the folder holds one file per screen density, and the file being
// there is the whole declaration that the picture has that density.
type CommonPictureDefinition struct {
	Format  int           `yaml:"format" json:"format"`
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	// AvailableForAppearance offers this picture where a user sets up the
	// conditional appearance of a report while the application runs.
	AvailableForAppearance bool `yaml:"available_for_appearance,omitempty" json:"availableForAppearance,omitempty"`
	// AvailableForChoice offers it where a user picks a picture for a cell,
	// a drawing or a page header of a spreadsheet.
	AvailableForChoice bool `yaml:"available_for_choice,omitempty" json:"availableForChoice,omitempty"`
}

// DecodeCommonPicture reads and validates one common picture.
func DecodeCommonPicture(source string, reader io.Reader, configuration project.Project) (CommonPictureDefinition, error) {
	var value CommonPictureDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return CommonPictureDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	if err := issuesError(source, value.Format, issues); err != nil {
		return CommonPictureDefinition{}, err
	}
	return value, nil
}

func cloneCommonPicture(value CommonPictureDefinition) CommonPictureDefinition {
	value.Title = cloneTitle(value.Title)
	return value
}

// CommonPicture returns one common picture by name, folded case.
func (catalog *Catalog) CommonPicture(name string) (CommonPictureDefinition, bool) {
	index, ok := catalog.commonPictureByName[strings.ToLower(name)]
	if !ok {
		return CommonPictureDefinition{}, false
	}
	return cloneCommonPicture(catalog.CommonPictures[index]), true
}

// CommonPictureByID returns one common picture by identifier - which is how a
// command names the picture it is shown with.
func (catalog *Catalog) CommonPictureByID(id uuid.UUID) (CommonPictureDefinition, bool) {
	index, ok := catalog.commonPictureByID[id]
	if !ok {
		return CommonPictureDefinition{}, false
	}
	return cloneCommonPicture(catalog.CommonPictures[index]), true
}

// PictureDensity reads a file name inside a picture's folder as the density it
// stands for. The name is the declaration: 100.png is the base image, 200.png
// the same image for a screen of twice the density.
func PictureDensity(file string) (ScreenDensity, bool) {
	extension := filepath.Ext(file)
	if len(extension) < 2 || !slices.Contains(pictureFormats, extension[1:]) {
		return 0, false
	}
	step, err := strconv.Atoi(strings.TrimSuffix(file, extension))
	if err != nil {
		return 0, false
	}
	density := ScreenDensity(step)
	if !slices.Contains(screenDensities, density) {
		return 0, false
	}
	return density, true
}

// validateCommonPictureFiles checks the folder of every common picture: its
// own description, and the image at the densities it offers.
//
// A picture with no image at all is allowed, for the same reason a template
// with no content is: no editor writes one yet. What is not allowed is the
// same density twice - nothing would say which of the two the platform draws -
// or a ladder without its base, which leaves a screen asking for a density the
// picture skips with nothing to fall back to.
func (catalog *Catalog) validateCommonPictureFiles(root string) error {
	if root == "" {
		return nil
	}
	for _, item := range catalog.CommonPictures {
		directory := filepath.Join(root, "metadata", string(CommonPictureKind), item.Name)
		entries, err := os.ReadDir(directory)
		if err != nil {
			return fmt.Errorf("common picture %s: %w", item.Name, err)
		}
		densities := map[ScreenDensity]string{}
		for _, entry := range entries {
			if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
				return fmt.Errorf("common picture %s keeps %q, which is not one of its images",
					item.Name, entry.Name())
			}
			if entry.Name() == project.ObjectMetadataFile {
				continue
			}
			density, ok := PictureDensity(entry.Name())
			if !ok {
				return fmt.Errorf("common picture %s keeps %q, and an image is named by the density it is drawn at",
					item.Name, entry.Name())
			}
			if previous, taken := densities[density]; taken {
				return fmt.Errorf("common picture %s keeps both %s and %s for density %d",
					item.Name, previous, entry.Name(), density)
			}
			densities[density] = entry.Name()
		}
		if len(densities) > 0 && densities[BaseScreenDensity] == "" {
			return fmt.Errorf("common picture %s has no image at density %d, which every other density falls back to",
				item.Name, BaseScreenDensity)
		}
	}
	return nil
}

// validateCommonPictureReferences checks that every picture a command or a
// group of commands is shown with is a picture the configuration has.
//
// Until common pictures were objects there was nothing to check this against,
// and a command could name a picture that had never existed: the platform
// would then draw it with nothing, and a command shown as a picture alone
// would be a blank place in the interface nobody can read.
func (catalog *Catalog) validateCommonPictureReferences() error {
	shown := func(owner, name string, picture *PictureReference) error {
		if picture == nil || picture.Common == nil {
			return nil
		}
		if _, ok := catalog.commonPictureByID[*picture.Common]; !ok {
			return fmt.Errorf("%s %s is shown with unknown common picture %s", owner, name, picture.Common)
		}
		return nil
	}
	for _, item := range catalog.CommonCommands {
		if err := shown("common command", item.Name, item.Picture); err != nil {
			return err
		}
	}
	for _, item := range catalog.CommandGroups {
		if err := shown("command group", item.Name, item.Picture); err != nil {
			return err
		}
	}
	for _, owned := range catalog.everyObjectCommands() {
		for _, command := range owned.commands {
			if err := shown(owned.owner+" command", command.Name, command.Picture); err != nil {
				return err
			}
		}
	}
	return nil
}
