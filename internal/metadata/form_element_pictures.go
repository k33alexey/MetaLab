package metadata

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
)

// elementPicture is one picture an element of a form is drawn with, with the
// property it stands in.
type elementPicture struct {
	name  string
	value *PictureReference
}

// pictures lists every picture the element is drawn with, whether it names
// a file of its own or not. A picture of the element's own lies in the
// folder of the element in the form's items folder, under the name its
// reference gives (project.FormItemsDirectory).
func (element ManagedFormElement) pictures() []elementPicture {
	return []elementPicture{
		{"header_picture", element.HeaderPicture},
		{"footer_picture", element.FooterPicture},
		{"choice_button_picture", element.ChoiceButtonPicture},
		{"values_picture", element.ValuesPicture},
	}
}

// ownPictures lists the pictures of the element drawn from a file of its own.
func (element ManagedFormElement) ownPictures() []elementPicture {
	var result []elementPicture
	for _, picture := range element.pictures() {
		if picture.value.fileName() != "" {
			result = append(result, picture)
		}
	}
	return result
}

// validateOwnPictureFiles checks that no two pictures of one element draw the
// same file. The prototype names the file by the property (HeaderPicture.png,
// Picture.png), so each lies apart; one file under two would change both
// pictures when either is edited.
func validateOwnPictureFiles(path string, element ManagedFormElement) []string {
	var issues []string
	own := element.ownPictures()
	for index, picture := range own {
		for _, earlier := range own[:index] {
			if strings.EqualFold(earlier.value.File, picture.value.File) {
				issues = append(issues, fmt.Sprintf("%s.%s.file is the file of %s too", path, picture.name, earlier.name))
			}
		}
	}
	return issues
}

// validateFormItemPictureFiles checks the items folder of a form against the
// form, both ways: every element drawn from a file of its own finds the file
// in its folder, and every folder there is an element holding only the files
// its pictures draw. The folder is named by the element, whatever its case,
// as the folder of an item of a route map is.
func validateFormItemPictureFiles(folder string, form ManagedForm) error {
	base := filepath.Join(folder, project.FormItemsDirectory)
	elements := map[string]ManagedFormElement{}
	stack := append([]ManagedFormElement(nil), form.Items...)
	for len(stack) > 0 {
		element := stack[len(stack)-1]
		stack = append(stack[:len(stack)-1], element.Children...)
		elements[strings.ToLower(element.Name)] = element
		for _, picture := range element.ownPictures() {
			if err := requirePictureFile("element "+element.Name+" "+picture.name, findFolded(base, element.Name), picture.value); err != nil {
				return err
			}
		}
	}
	info, err := os.Lstat(base)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("keeps %q, and the pictures of its elements are a folder", project.FormItemsDirectory)
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		element, ok := elements[strings.ToLower(entry.Name())]
		if !ok || !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("keeps %q among the pictures of its elements, and the form has no element of that name", entry.Name())
		}
		files, err := os.ReadDir(filepath.Join(base, entry.Name()))
		if err != nil {
			return fmt.Errorf("element %s: %w", element.Name, err)
		}
		own := element.ownPictures()
		for _, file := range files {
			drawn := false
			for _, picture := range own {
				drawn = drawn || picture.value.drawsFile(file.Name())
			}
			if !file.Type().IsRegular() || !drawn {
				return fmt.Errorf("element %s keeps %q, which its pictures do not draw", element.Name, file.Name())
			}
		}
	}
	return nil
}
