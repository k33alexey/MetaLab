package studio

import (
	"bytes"
	"fmt"
	"io"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// windowHead is what the window of an object reads beside the object: the
// head of its palette and the subsystems of the project, those holding the
// object marked (its page «Подсистемы»).
type windowHead struct {
	objectHead
	Subsystems []MemberPlace `json:"subsystems"`
}

// readObjectWindow reads the description of an object of a kind that has a
// window (readObjectDescription) with the subsystems holding it, and
// answers the project loaded for what else the window reads of it.
func readObjectWindow[T any](workspace *Workspace, kind metadata.Kind, relative string,
	decode func(string, io.Reader, project.Project) (T, error), id func(T) uuid.UUID) (T, windowHead, *metadata.Catalog, error) {
	value, head, err := readObjectDescription(workspace, kind, relative, decode)
	if err != nil {
		return value, windowHead{}, nil, err
	}
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		return value, windowHead{}, nil, err
	}
	return value, windowHead{objectHead: head, Subsystems: subsystemPlaces(loaded, id(value))}, loaded, nil
}

// objectWindowSource is what the reading of the window of an object answers: the
// object with the head of its window, and how an object of its kind is
// named - by its identifier and its name.
type objectWindowSource[T any] interface {
	windowOf() (T, windowHead)
	named(T) (uuid.UUID, string)
}

// saveObjectWindowAs writes the object a window of rows and subsystems gives
// (saveObjectWindow) under the lock and reads it back as the window reads
// it (read).
func saveObjectWindowAs[S objectWindowSource[T], T any](workspace *Workspace, relative, revision string, value T, subsystems *[]uuid.UUID, read func(string) (S, error)) (S, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	var kind S
	written, err := saveObjectWindow(workspace, relative, revision, value, subsystems, func(relative string) (T, windowHead, error) {
		opened, err := read(relative)
		object, head := opened.windowOf()
		return object, head, err
	}, kind.named)
	if err != nil {
		var none S
		return none, err
	}
	return read(written)
}

// saveObjectWindow writes an object of a window of rows and subsystems as one
// save (writeObjectWindowLocked): the object read last at revision is opened
// by read, identity names an object by its identifier and name, subsystems
// are the marks of the page «Подсистемы», none where it was not shown.
// Answers where the description lies after.
func saveObjectWindow[T any](workspace *Workspace, relative, revision string, value T, subsystems *[]uuid.UUID,
	read func(string) (T, windowHead, error), identity func(T) (uuid.UUID, string)) (string, error) {
	opened, head, err := read(relative)
	if err != nil {
		return "", err
	}
	if head.Revision != revision {
		return "", ErrSourceChanged
	}
	id, name := identity(value)
	openedID, openedName := identity(opened)
	if id != openedID {
		return "", fmt.Errorf("the identifier of an object cannot be changed")
	}
	if name != openedName {
		if err := workspace.objectNameErrorLocked(relative, "", name); err != nil {
			return "", err
		}
	}
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, value); err != nil {
		return "", err
	}
	var pages []membership
	if subsystems != nil {
		pages = append(pages, subsystemMembership(id, head.Subsystems, *subsystems))
	}
	written, _, err := workspace.writeObjectWindowLocked(relative, openedName, name, encoded.String(), revision, pages, nil)
	return written, err
}
