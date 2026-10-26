package studio

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// ErrNoNewObject is a kind Studio does not make yet: its «Добавить» comes
// with its window or palette (owner, 26.10.2026).
var ErrNoNewObject = fmt.Errorf("an object of this kind is not made by Studio yet")

// CreateObject makes the next object of a kind - «<Вид>N» unless a name is
// given - with the prototype's values of a new one (metadata.NewObject), and
// answers where its description lies. It is the one way every object is
// made; the window of a kind reads the new object as it reads any.
func (workspace *Workspace) CreateObject(kind metadata.Kind, name string) (string, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.createObjectLocked(kind, name, nil)
}

// createObjectAs makes an object of a kind - shaped by a constructor where
// one is given (createObjectLocked) - and reads it by the reading of its
// window, under the one lock.
func createObjectAs[Window any](workspace *Workspace, kind metadata.Kind, name string, shape func(*metadata.Fresh) error, read func(string) (Window, error)) (Window, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	relative, err := workspace.createObjectLocked(kind, name, shape)
	if err != nil {
		var none Window
		return none, err
	}
	return read(relative)
}

// createObjectLocked writes the files of a new object - its description and,
// of a role, its grid of rights - where its kind keeps them, puts the object
// last among the objects of its kind, and gives it to the roles that are
// given every new object. A refusal on the way takes the object away again:
// no file, no folder, no line of the order and no right of a role is left of
// it. shape, when given, makes of the kind's new object what a constructor
// asked for - another kind of template, its content - before it is checked.
func (workspace *Workspace) createObjectLocked(kind metadata.Kind, name string, shape func(*metadata.Fresh) error) (string, error) {
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return "", err
	}
	id, err := uuid.New()
	if err != nil {
		return "", err
	}
	// «Добавить» names nothing: the object is the kind's next number.
	if name == "" {
		if name, err = workspace.newObjectNameLocked(string(kind)); err != nil {
			return "", err
		}
	}
	if err := workspace.objectNameErrorLocked("", branchOf(string(kind)), name); err != nil {
		return "", err
	}
	fresh, ok := metadata.NewObject(kind, id, name, configuration)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrNoNewObject, kind)
	}
	if shape != nil {
		if err := shape(&fresh); err != nil {
			return "", err
		}
	}
	description, rights, err := metadata.EncodeObject(fresh.Value)
	if err != nil {
		return "", err
	}
	relative, err := newObjectPath(kind, id, name)
	if err != nil {
		return "", err
	}
	// The new object is read back as the load reads it: what Studio makes,
	// the project accepts.
	if _, _, err := metadata.DecodeSource(relative, bytes.NewReader(description), bytes.NewReader(rights), configuration); err != nil {
		return "", err
	}
	// The folder of the kind: a link there is refused rather than followed
	// (ValidateLayout checked the metadata root itself).
	directory := filepath.Join(workspace.root, "metadata", string(kind))
	if err := os.Mkdir(directory, 0o755); err != nil && !os.IsExist(err) {
		return "", err
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", ErrInvalidSourcePath
	}
	descriptionPath := filepath.Join(workspace.root, filepath.FromSlash(relative))
	var undo []func()
	takeAway := func() {
		for index := len(undo) - 1; index >= 0; index-- {
			undo[index]()
		}
		workspace.invalidateStudioIndexesLocked()
	}
	// An object of a kind kept in a folder named after it gets the folder.
	if objectDirectory := filepath.Dir(descriptionPath); objectDirectory != directory {
		if err := os.Mkdir(objectDirectory, 0o755); err != nil {
			return "", err
		}
		undo = append(undo, func() { _ = os.Remove(objectDirectory) })
	}
	// A role's grid goes first, as a save writes it (replaceRoleFilesLocked),
	// and goes with the description when anything after it is refused.
	if rights != nil {
		gridPath := workspace.roleGridPath(relative)
		undo = append(undo, func() { _ = writeRoleGrid(gridPath, nil) })
		if err := writeRoleGrid(gridPath, rights); err != nil {
			takeAway()
			return "", fmt.Errorf("create %s: %w", kind, err)
		}
	}
	// The description, then the files the object comes with; none of them
	// may be there already.
	takeFiles, err := workspace.writeNewFilesLocked(append([]newFile{{relative, description}}, freshFiles(fresh.Files)...))
	if err != nil {
		takeAway()
		return "", fmt.Errorf("create %s: %w", kind, err)
	}
	undo = append(undo, takeFiles)
	workspace.invalidateStudioIndexesLocked()
	// The new object stands last among the objects of its kind; an order
	// that cannot be written takes the object away again, as a refused
	// grant does.
	restoreOrder, err := workspace.placeInOrderLocked(kind, id)
	if err != nil {
		takeAway()
		return "", fmt.Errorf("place new %s in the order: %w", kind, err)
	}
	// A refused grant takes the new object away again: the roles are left as
	// they were, the order is put back, and an object no role was told of is
	// not left behind.
	if _, err := workspace.applyRoleAutoGrantsLocked(func(role *metadata.RoleDefinition) bool {
		if !role.GrantNewObjectsByDefault {
			return false
		}
		return role.GrantObjectReadByDefault(kind, id)
	}); err != nil {
		restoreErr := restoreOrder()
		takeAway()
		if restoreErr != nil {
			return "", fmt.Errorf("grant default access to new %s: %w; restoring the order failed: %v", kind, err, restoreErr)
		}
		return "", fmt.Errorf("grant default access to new %s: %w", kind, err)
	}
	return relative, nil
}

// newFile is a file of a new object: where it lies and what it holds.
type newFile struct {
	relative string
	content  []byte
}

// freshFiles are the files a new object comes with.
func freshFiles(fresh []metadata.FreshFile) []newFile {
	files := make([]newFile, len(fresh))
	for index, file := range fresh {
		files[index] = newFile{relative: file.Path, content: file.Content}
	}
	return files
}

// writeNewFilesLocked writes files that must not be there yet, each with the
// folders it needs, and answers how to take them away again - the files
// and the folders made for them; a refusal on the way takes away what was
// written.
func (workspace *Workspace) writeNewFilesLocked(files []newFile) (func(), error) {
	var undo []func()
	takeAway := func() {
		for index := len(undo) - 1; index >= 0; index-- {
			undo[index]()
		}
		workspace.invalidateStudioIndexesLocked()
	}
	for _, file := range files {
		written := filepath.Join(workspace.root, filepath.FromSlash(file.relative))
		// The folders the file needs, from the outermost missing one down.
		var made []string
		for directory := filepath.Dir(written); ; directory = filepath.Dir(directory) {
			if _, err := os.Lstat(directory); err == nil {
				break
			}
			made = append(made, directory)
		}
		for index := len(made) - 1; index >= 0; index-- {
			if err := os.Mkdir(made[index], 0o755); err != nil {
				takeAway()
				return func() {}, err
			}
			directory := made[index]
			undo = append(undo, func() { _ = os.Remove(directory) })
		}
		if err := writeNewFile(written, file.content); err != nil {
			takeAway()
			return func() {}, err
		}
		undo = append(undo, func() { _ = os.Remove(written) })
	}
	workspace.invalidateStudioIndexesLocked()
	return takeAway, nil
}

// writeNewFile writes a file that must not be there yet, synced to disk;
// one written halfway is taken away.
func writeNewFile(written string, content []byte) error {
	file, err := os.OpenFile(written, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(content)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	if closeErr := file.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		_ = os.Remove(written)
	}
	return writeErr
}

// newObjectPath is where the description of a new object lies: in a folder
// named after it for a kind kept so, by its identifier for the rest.
func newObjectPath(kind metadata.Kind, id uuid.UUID, name string) (string, error) {
	if kinds.Must(kind).Layout().KeepsFolder() {
		return project.ObjectMetadataPath(string(kind), name)
	}
	return project.MetadataPath(string(kind), id)
}
