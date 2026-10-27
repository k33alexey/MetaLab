package studio

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Pages of the window of an object mark the objects whose content lists it:
// «Подсистемы» the subsystems it takes part in (docs/block-00/spravochnik.md,
// page 2; snapshot okno-02), «Функциональные опции» the options it is in
// (page 3; snapshot okno-03), «Обмен данными» the exchange plans that
// register its changes, each with its autoregistration (page 14; snapshot
// okno-14), «Ввод на основании» the objects entered on its basis (page 12;
// snapshot okno-12). The content is the other object's, so a mark of the page is an
// edit of that object, written with the object of the window by «Сохранить»
// and taken back with it.

// MemberPlace is an object whose content may list the object of the window,
// as its page shows it: under its parent, by its name, marked when the
// object is in its content - with the autoregistration of an exchange plan
// there. As the window writes it back it is a mark: the object and its
// autoregistration.
type MemberPlace struct {
	ID         uuid.UUID           `json:"id"`
	Parent     *uuid.UUID          `json:"parent,omitempty"`
	Name       string              `json:"name"`
	Member     bool                `json:"member,omitempty"`
	AutoRecord metadata.AutoRecord `json:"autoRecord,omitempty"`
	// path is where the object is described.
	path string
}

// memberPlace is an object of a kind whose content may list the object of
// the window, with where it is described: in a folder named after it or in
// a file named by its identifier, as its kind keeps it.
func memberPlace(kind metadata.Kind, id uuid.UUID, name string) MemberPlace {
	place := MemberPlace{ID: id, Name: name}
	if kinds.Must(kind).Layout().KeepsFolder() {
		place.path, _ = project.ObjectMetadataPath(string(kind), name)
	} else {
		place.path, _ = project.MetadataPath(string(kind), id)
	}
	return place
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
		place := memberPlace(metadata.SubsystemKind, item.ID, item.Name)
		place.Parent, place.Member = item.Owner, slices.Contains(members, object)
		places = append(places, place)
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
		place := memberPlace(metadata.FunctionalOptionKind, item.ID, item.Name)
		place.Member = slices.ContainsFunc(option.Content, wholeObject(kind, object))
		places = append(places, place)
	}
	return places
}

// exchangePlaces are the exchange plans of the project in the order of the
// tree, each marked where its content lists the object, with the
// autoregistration it is listed with.
func exchangePlaces(loaded *metadata.Catalog, object uuid.UUID) []MemberPlace {
	var places []MemberPlace
	for _, item := range loaded.KindObjects(metadata.ExchangePlanKind) {
		plan, _ := loaded.ExchangePlan(item.Name)
		place := memberPlace(metadata.ExchangePlanKind, item.ID, item.Name)
		if index := slices.IndexFunc(plan.Content, func(listed metadata.ExchangePlanContentItem) bool { return listed.Object == object }); index >= 0 {
			place.Member, place.AutoRecord = true, plan.Content[index].Auto
		}
		places = append(places, place)
	}
	return places
}

// basisPlaces are the objects that may be entered on the basis of another,
// in the order of the tree, by their full names, each marked where its list
// of bases names the object. The object itself is left out: its own list is
// the window's «Вводится на основании», written with it.
func basisPlaces(loaded *metadata.Catalog, object uuid.UUID) []MemberPlace {
	var marked []uuid.UUID
	for _, carrier := range loaded.BasisFor(object) {
		marked = append(marked, carrier.ID)
	}
	var places []MemberPlace
	for _, carrier := range loaded.BasisCarriers() {
		if carrier.ID == object {
			continue
		}
		place := memberPlace(carrier.Kind, carrier.ID, carrier.Name)
		place.Name, place.Member = carrier.FullName, slices.Contains(marked, carrier.ID)
		places = append(places, place)
	}
	return places
}

// members are the places marked.
func members(places []MemberPlace) []MemberPlace {
	result := []MemberPlace{}
	for _, place := range places {
		if place.Member {
			result = append(result, place)
		}
	}
	return result
}

// wholeObject tells the item of the content of an option that is the object
// itself.
func wholeObject(kind metadata.Kind, object uuid.UUID) func(metadata.FunctionalOptionItem) bool {
	return func(item metadata.FunctionalOptionItem) bool {
		return item.Kind == kind && item.Object == object && item.TablePart == nil && item.Element == nil
	}
}

// membership is what one page of the window marks: the objects of a kind
// whose content may list the object, the marks of those it is to be in, and
// how the description of one of them read from content takes the object in
// with its mark - last, unless it is there already - or out, when the mark
// is nil, answered to be encoded.
type membership struct {
	places  []MemberPlace
	marked  []MemberPlace
	include func(relative, content string, mark *MemberPlace, configuration project.Project) (any, error)
}

// marksOf are the marks of the objects of ids, which carry nothing but the
// object.
func marksOf(ids []uuid.UUID) []MemberPlace {
	marks := make([]MemberPlace, 0, len(ids))
	for _, id := range ids {
		marks = append(marks, MemberPlace{ID: id})
	}
	return marks
}

// subsystemMembership is the page «Подсистемы» of the object: the members
// of a subsystem list it.
func subsystemMembership(object uuid.UUID, places []MemberPlace, marked []uuid.UUID) membership {
	return membership{places: places, marked: marksOf(marked),
		include: func(relative, content string, mark *MemberPlace, configuration project.Project) (any, error) {
			subsystem, err := metadata.DecodeSubsystem(relative, strings.NewReader(content), configuration)
			subsystem.Members = listed(subsystem.Members, object, func(id uuid.UUID) bool { return id == object }, mark != nil)
			return subsystem, err
		}}
}

// optionMembership is the page «Функциональные опции» of the object of a
// kind: the content of an option lists it as an item naming the object
// alone, and the items naming its parts stay as they are.
func optionMembership(kind metadata.Kind, object uuid.UUID, places []MemberPlace, marked []uuid.UUID) membership {
	return membership{places: places, marked: marksOf(marked),
		include: func(relative, content string, mark *MemberPlace, configuration project.Project) (any, error) {
			option, err := metadata.DecodeFunctionalOption(relative, strings.NewReader(content), configuration)
			option.Content = listed(option.Content, metadata.FunctionalOptionItem{Kind: kind, Object: object}, wholeObject(kind, object), mark != nil)
			return option, err
		}}
}

// exchangeMembership is the page «Обмен данными» of the object of a kind:
// the content of an exchange plan lists it with its autoregistration.
func exchangeMembership(kind metadata.Kind, object uuid.UUID, places []MemberPlace, marked []MemberPlace) membership {
	return membership{places: places, marked: marked,
		include: func(relative, content string, mark *MemberPlace, configuration project.Project) (any, error) {
			plan, err := metadata.DecodeExchangePlan(relative, strings.NewReader(content), configuration)
			item := metadata.ExchangePlanContentItem{Kind: kind, Object: object}
			if mark != nil {
				item.Auto = mark.AutoRecord
			}
			plan.Content = listed(plan.Content, item, func(listed metadata.ExchangePlanContentItem) bool { return listed.Object == object }, mark != nil)
			return plan, err
		}}
}

// basisMembership is the list «Является основанием для» of the page «Ввод на
// основании»: the list of bases of an object of any of the kinds that keep
// one names the object.
func basisMembership(object uuid.UUID, places []MemberPlace, marked []uuid.UUID) membership {
	return membership{places: places, marked: marksOf(marked),
		include: func(relative, content string, mark *MemberPlace, configuration project.Project) (any, error) {
			description, _, err := metadata.DecodeSource(relative, strings.NewReader(content), nil, configuration)
			if err != nil {
				return nil, err
			}
			return metadata.ListBasis(description, object, mark != nil)
		}}
}

// listed is items with the one isIt tells taken out, or, when member, put
// in its place - last when it was not there.
func listed[T any](items []T, item T, isIt func(T) bool, member bool) []T {
	index := slices.IndexFunc(items, isIt)
	switch {
	case !member:
		return slices.DeleteFunc(items, isIt)
	case index >= 0:
		items[index] = item
		return items
	}
	return append(items, item)
}

// memberSave is the description of one object written by a window, and what
// to write back when what follows it is refused.
type memberSave struct {
	path, before string
	revision     string
}

// writeMembershipLocked puts the object into the content of the objects
// each page marks, with its mark, and out of the others it is in, and
// answers how to put every object written back. An object refused halfway
// puts back those written before it. Nothing is written for an object whose
// content does not change.
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
			var mark *MemberPlace
			if index := slices.IndexFunc(page.marked, func(mark MemberPlace) bool { return mark.ID == place.ID }); index >= 0 {
				mark = &page.marked[index]
			}
			if place.Member == (mark != nil) && (mark == nil || mark.AutoRecord == place.AutoRecord) {
				continue
			}
			written, err := workspace.rewriteMembersLocked(place.path, mark, page.include, configuration)
			if err == nil {
				done = append(done, memberSave{path: place.path, before: written.Content, revision: written.Revision})
				continue
			}
			if restoreErr := restore(); restoreErr != nil {
				return nil, fmt.Errorf("%s: %w; %v", place.Name, err, restoreErr)
			}
			return nil, fmt.Errorf("%s: %w", place.Name, err)
		}
	}
	return restore, nil
}

// rewriteMembersLocked takes the object into the content of the object
// described at relative with its mark, or out of it, and answers what the
// file held before, with the revision it has now.
func (workspace *Workspace) rewriteMembersLocked(relative string, mark *MemberPlace, include func(string, string, *MemberPlace, project.Project) (any, error), configuration project.Project) (SourceFile, error) {
	file, err := workspace.readSource(relative)
	if err != nil {
		return SourceFile{}, err
	}
	value, err := include(relative, file.Content, mark, configuration)
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
