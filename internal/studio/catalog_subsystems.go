package studio

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The page «Подсистемы» of the window of an object marks the subsystems it
// takes part in (docs/block-00/spravochnik.md, page 2; snapshot okno-02).
// The membership is the subsystem's - its content lists the object - so a
// mark of the page is an edit of the subsystem, written with the object by
// «Сохранить» and taken back with it.

// SubsystemPlace is a subsystem as the page shows it: under its parent, by
// its name, marked when the object is in its content.
type SubsystemPlace struct {
	ID     uuid.UUID  `json:"id"`
	Parent *uuid.UUID `json:"parent,omitempty"`
	Name   string     `json:"name"`
	Member bool       `json:"member,omitempty"`
}

// subsystemPlaces are the subsystems of the project in the order of the
// tree, each marked where the object is in its content.
func subsystemPlaces(loaded *metadata.Catalog, object uuid.UUID) []SubsystemPlace {
	filter := loaded.SubsystemFilter()
	var places []SubsystemPlace
	for _, item := range filter.Objects {
		members, isSubsystem := filter.Subsystems[item.ID]
		if !isSubsystem {
			continue
		}
		places = append(places, SubsystemPlace{ID: item.ID, Parent: item.Owner, Name: item.Name, Member: slices.Contains(members, object)})
	}
	return places
}

// subsystemSave is the content of one subsystem written by a window, and
// what to write back when what follows it is refused.
type subsystemSave struct {
	path, before string
	revision     string
}

// writeMembershipLocked puts the object into the content of the subsystems
// marked and out of the others it is in - a new member last - and answers
// how to put every subsystem written back. A subsystem refused halfway puts
// back those written before it. Nothing is written for a subsystem whose
// content does not change.
func (workspace *Workspace) writeMembershipLocked(object uuid.UUID, marked []uuid.UUID, places []SubsystemPlace) (func() error, error) {
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return nil, err
	}
	var done []subsystemSave
	restore := func() error {
		var failed []error
		for index := len(done) - 1; index >= 0; index-- {
			if _, err := workspace.saveSourceLocked(done[index].path, done[index].before, done[index].revision); err != nil {
				failed = append(failed, fmt.Errorf("restoring subsystem %q failed: %w", done[index].path, err))
			}
		}
		return errors.Join(failed...)
	}
	for _, place := range places {
		if place.Member == slices.Contains(marked, place.ID) {
			continue
		}
		relative, err := project.MetadataPath(string(metadata.SubsystemKind), place.ID)
		if err == nil {
			var written SourceFile
			if written, err = workspace.rewriteMembersLocked(relative, object, !place.Member, configuration); err == nil {
				done = append(done, subsystemSave{path: relative, before: written.Content, revision: written.Revision})
				continue
			}
		}
		if restoreErr := restore(); restoreErr != nil {
			return nil, fmt.Errorf("subsystem %s: %w; %v", place.Name, err, restoreErr)
		}
		return nil, fmt.Errorf("subsystem %s: %w", place.Name, err)
	}
	return restore, nil
}

// rewriteMembersLocked adds the object to the content of the subsystem
// described at relative, or takes it out, and answers what the file held
// before, with the revision it has now.
func (workspace *Workspace) rewriteMembersLocked(relative string, object uuid.UUID, member bool, configuration project.Project) (SourceFile, error) {
	file, err := workspace.readSource(relative)
	if err != nil {
		return SourceFile{}, err
	}
	subsystem, err := metadata.DecodeSubsystem(relative, strings.NewReader(file.Content), configuration)
	if err != nil {
		return SourceFile{}, err
	}
	subsystem.Members = slices.DeleteFunc(subsystem.Members, func(id uuid.UUID) bool { return id == object })
	if member {
		subsystem.Members = append(subsystem.Members, object)
	}
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, subsystem); err != nil {
		return SourceFile{}, err
	}
	saved, err := workspace.saveSourceLocked(relative, encoded.String(), file.Revision)
	if err != nil {
		return SourceFile{}, err
	}
	return SourceFile{Content: file.Content, Revision: saved.Revision}, nil
}
