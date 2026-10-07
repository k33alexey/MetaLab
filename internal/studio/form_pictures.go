package studio

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// renameFolder moves a folder of pictures; a test replaces it to fail a save
// in the middle.
var renameFolder = os.Rename

// pictureFolderMove is the folder of the pictures of one element of a form
// that a save moves: to the element's new name when it was renamed, or away
// when it was removed.
type pictureFolderMove struct {
	element string
	from    string
	to      string // empty when the element is gone
	staged  string
}

// pictureFolders is what a save of a form does to the folders of the
// pictures of its elements (project.FormItemsDirectory). The folders are
// named by the element, so renaming an element in the designer without
// moving its folder would leave a project that does not load.
//
// The folders are first set aside under names of no element, then the form
// is written, then they take their places. A failure before the form is
// written puts every folder back, so a refused save leaves the project as
// it was.
type pictureFolders struct {
	directory string
	moves     []pictureFolderMove
}

// planPictureFolders compares the form on disk with the form being saved by
// the identifiers of their elements. A folder moves only when the name of
// its element changed beyond case, since a folder is found without regard
// to case. A folder that is no element of the form on disk is left alone:
// the designer did not put it there and does not decide about it.
func planPictureFolders(formDirectory string, before, after metadata.ManagedForm) (pictureFolders, error) {
	plan := pictureFolders{directory: filepath.Join(formDirectory, project.FormItemsDirectory)}
	info, err := os.Lstat(plan.directory)
	if errors.Is(err, fs.ErrNotExist) {
		return plan, nil
	}
	if err != nil {
		return plan, err
	}
	if !info.IsDir() {
		return plan, nil
	}
	entries, err := os.ReadDir(plan.directory)
	if err != nil {
		return plan, err
	}
	previous := map[string]uuid.UUID{}
	names := map[uuid.UUID]string{}
	formElementNames(before.FormItems(), names)
	for id, name := range names {
		previous[strings.ToLower(name)] = id
	}
	current := map[uuid.UUID]string{}
	formElementNames(after.FormItems(), current)
	staying := map[string]bool{}
	for _, entry := range entries {
		id, known := previous[strings.ToLower(entry.Name())]
		if !known || !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			staying[strings.ToLower(entry.Name())] = true
			continue
		}
		name, kept := current[id]
		switch {
		case !kept:
			plan.moves = append(plan.moves, pictureFolderMove{element: names[id], from: entry.Name()})
		case !strings.EqualFold(name, names[id]):
			plan.moves = append(plan.moves, pictureFolderMove{element: names[id], from: entry.Name(), to: name})
		default:
			staying[strings.ToLower(entry.Name())] = true
		}
	}
	for _, move := range plan.moves {
		if move.to != "" && staying[strings.ToLower(move.to)] {
			return plan, fmt.Errorf("element %s is renamed to %s, and the pictures of the form's elements already hold a folder %s",
				move.element, move.to, move.to)
		}
	}
	return plan, nil
}

// formElementNames records the name of every element of a form by its
// identifier, however deep it stands.
func formElementNames(items []metadata.ManagedFormElement, into map[uuid.UUID]string) {
	for _, item := range items {
		into[item.ID] = item.Name
		formElementNames(item.Nested(), into)
	}
}

// setAside moves every folder the save moves to a name of no element. On a
// failure the folders already moved are put back.
func (plan *pictureFolders) setAside() error {
	for index := range plan.moves {
		move := &plan.moves[index]
		id, err := uuid.New()
		if err == nil {
			move.staged = ".ml-move-" + id.String()
			err = renameFolder(filepath.Join(plan.directory, move.from), filepath.Join(plan.directory, move.staged))
		}
		if err != nil {
			move.staged = ""
			plan.putBack()
			return fmt.Errorf("set aside the pictures of element %s: %w", move.element, err)
		}
	}
	return nil
}

// putBack returns the folders set aside to where they were.
func (plan *pictureFolders) putBack() {
	for index := range plan.moves {
		move := &plan.moves[index]
		if move.staged != "" {
			_ = renameFolder(filepath.Join(plan.directory, move.staged), filepath.Join(plan.directory, move.from))
			move.staged = ""
		}
	}
}

// finish gives the folders set aside their new names and removes those of
// the elements that are gone, once the form is written. The form is saved
// whatever happens here, so a failure names the folder that is left over.
func (plan *pictureFolders) finish() error {
	var failures []string
	for _, move := range plan.moves {
		staged := filepath.Join(plan.directory, move.staged)
		var err error
		if move.to == "" {
			err = os.RemoveAll(staged)
		} else {
			err = renameFolder(staged, filepath.Join(plan.directory, move.to))
		}
		if err != nil {
			failures = append(failures, fmt.Sprintf("the pictures of element %s are left in %s: %v", move.element, move.staged, err))
		}
	}
	if len(plan.moves) > 0 {
		// An emptied folder of pictures is no folder: git keeps none.
		_ = os.Remove(plan.directory)
	}
	if len(failures) > 0 {
		return fmt.Errorf("the form is saved, and %s", strings.Join(failures, "; "))
	}
	return nil
}
