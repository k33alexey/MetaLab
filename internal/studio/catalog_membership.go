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

// Pages of the window of an object mark the objects whose content lists it:
// «Подсистемы» the subsystems it takes part in (docs/block-00/spravochnik.md,
// page 2; snapshot okno-02), «Функциональные опции» the options it is in
// (page 3; snapshot okno-03). The content is the other object's, so a mark
// of the page is an edit of that object, written with the object of the
// window by «Сохранить» and taken back with it.

// MemberPlace is an object whose content may list the object of the window,
// as its page shows it: under its parent, by its name, marked when the
// object is in its content.
type MemberPlace struct {
	ID     uuid.UUID  `json:"id"`
	Parent *uuid.UUID `json:"parent,omitempty"`
	Name   string     `json:"name"`
	Member bool       `json:"member,omitempty"`
}

// subsystemPlaces are the subsystems of the project in the order of the
// tree, each marked where the object is in its content.
func subsystemPlaces(loaded *metadata.Catalog, object uuid.UUID) []MemberPlace {
	filter := loaded.SubsystemFilter()
	var places []MemberPlace
	for _, item := range filter.Objects {
		members, isSubsystem := filter.Subsystems[item.ID]
		if !isSubsystem {
			continue
		}
		places = append(places, MemberPlace{ID: item.ID, Parent: item.Owner, Name: item.Name, Member: slices.Contains(members, object)})
	}
	return places
}

// optionPlaces are the functional options of the project in the order of the
// tree, each marked where the object itself - not a part of it - is in its
// content: a part has a page of its own (obshchee.md, «Дополнительно»).
func optionPlaces(loaded *metadata.Catalog, kind metadata.Kind, object uuid.UUID) []MemberPlace {
	var places []MemberPlace
	for _, item := range loaded.KindObjects(metadata.FunctionalOptionKind) {
		option, _ := loaded.FunctionalOption(item.Name)
		places = append(places, MemberPlace{ID: item.ID, Name: item.Name, Member: slices.ContainsFunc(option.Content, wholeObject(kind, object))})
	}
	return places
}

// wholeObject tells the item of the content of an option that is the object
// itself.
func wholeObject(kind metadata.Kind, object uuid.UUID) func(metadata.FunctionalOptionItem) bool {
	return func(item metadata.FunctionalOptionItem) bool {
		return item.Kind == kind && item.Object == object && item.TablePart == nil && item.Element == nil
	}
}

// membership is what one page of the window marks: the objects of a kind
// whose content may list the object, those of them marked, and how the
// description of one of them read from content takes the object in - last
// - or out, answered to be encoded.
type membership struct {
	kind    metadata.Kind
	places  []MemberPlace
	marked  []uuid.UUID
	include func(relative, content string, member bool, configuration project.Project) (any, error)
}

// subsystemMembership is the page «Подсистемы» of the object: the members
// of a subsystem list it.
func subsystemMembership(object uuid.UUID, places []MemberPlace, marked []uuid.UUID) membership {
	return membership{kind: metadata.SubsystemKind, places: places, marked: marked,
		include: func(relative, content string, member bool, configuration project.Project) (any, error) {
			subsystem, err := metadata.DecodeSubsystem(relative, strings.NewReader(content), configuration)
			subsystem.Members = listed(subsystem.Members, object, func(id uuid.UUID) bool { return id == object }, member)
			return subsystem, err
		}}
}

// optionMembership is the page «Функциональные опции» of the object of a
// kind: the content of an option lists it as an item naming the object
// alone, and the items naming its parts stay as they are.
func optionMembership(kind metadata.Kind, object uuid.UUID, places []MemberPlace, marked []uuid.UUID) membership {
	return membership{kind: metadata.FunctionalOptionKind, places: places, marked: marked,
		include: func(relative, content string, member bool, configuration project.Project) (any, error) {
			option, err := metadata.DecodeFunctionalOption(relative, strings.NewReader(content), configuration)
			option.Content = listed(option.Content, metadata.FunctionalOptionItem{Kind: kind, Object: object}, wholeObject(kind, object), member)
			return option, err
		}}
}

// listed is items with the one isIt tells taken out, and item put last when
// member.
func listed[T any](items []T, item T, isIt func(T) bool, member bool) []T {
	items = slices.DeleteFunc(items, isIt)
	if member {
		items = append(items, item)
	}
	return items
}

// memberSave is the description of one object written by a window, and what
// to write back when what follows it is refused.
type memberSave struct {
	path, before string
	revision     string
}

// writeMembershipLocked puts the object into the content of the objects
// each page marks and out of the others it is in, and answers how to put
// every object written back. An object refused halfway puts back those
// written before it. Nothing is written for an object whose content does not
// change.
func (workspace *Workspace) writeMembershipLocked(pages []membership) (func() error, error) {
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return nil, err
	}
	var done []memberSave
	restore := func() error {
		var failed []error
		for index := len(done) - 1; index >= 0; index-- {
			if _, err := workspace.saveSourceLocked(done[index].path, done[index].before, done[index].revision); err != nil {
				failed = append(failed, fmt.Errorf("restoring %q failed: %w", done[index].path, err))
			}
		}
		return errors.Join(failed...)
	}
	for _, page := range pages {
		for _, place := range page.places {
			if place.Member == slices.Contains(page.marked, place.ID) {
				continue
			}
			relative, err := project.MetadataPath(string(page.kind), place.ID)
			if err == nil {
				var written SourceFile
				if written, err = workspace.rewriteMembersLocked(relative, !place.Member, page.include, configuration); err == nil {
					done = append(done, memberSave{path: relative, before: written.Content, revision: written.Revision})
					continue
				}
			}
			if restoreErr := restore(); restoreErr != nil {
				return nil, fmt.Errorf("%s %s: %w; %v", page.kind, place.Name, err, restoreErr)
			}
			return nil, fmt.Errorf("%s %s: %w", page.kind, place.Name, err)
		}
	}
	return restore, nil
}

// rewriteMembersLocked takes the object into the content of the object
// described at relative, or out of it, and answers what the file held
// before, with the revision it has now.
func (workspace *Workspace) rewriteMembersLocked(relative string, member bool, include func(string, string, bool, project.Project) (any, error), configuration project.Project) (SourceFile, error) {
	file, err := workspace.readSource(relative)
	if err != nil {
		return SourceFile{}, err
	}
	value, err := include(relative, file.Content, member, configuration)
	if err != nil {
		return SourceFile{}, err
	}
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, value); err != nil {
		return SourceFile{}, err
	}
	saved, err := workspace.saveSourceLocked(relative, encoded.String(), file.Revision)
	if err != nil {
		return SourceFile{}, err
	}
	return SourceFile{Content: file.Content, Revision: saved.Revision}, nil
}

// markMembers shows the places marked as the window wrote them.
func markMembers(places []MemberPlace, marked []uuid.UUID) {
	for index := range places {
		places[index].Member = slices.Contains(marked, places[index].ID)
	}
}
