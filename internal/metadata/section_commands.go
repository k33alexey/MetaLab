package metadata

import (
	"reflect"
	"slices"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// SectionLine is a command the section of a subsystem shows by itself, before
// its command interface says otherwise (docs/block-00/podsistema.md,
// «Командный интерфейс…», «Досъёмка 28.10.2026»): a standard command of an
// object of its content (kinds.SectionCommand), an object's own command or a
// common command of its content in the group its property names - visible -
// and where it stands. Title is its presentation; a command to create one or
// a folder of it adds «: создать» or «: создать группу» by its Standard.
type SectionLine struct {
	InterfaceCommand
	Group   InterfaceGroup `json:"group"`
	Kind    Kind           `json:"kind"`
	Title   LocalizedText  `json:"title"`
	Visible bool           `json:"visible"`
}

// SectionLines are the commands the objects of a subsystem's content give
// its section, in the order of the content. An object whose «Использовать
// стандартные команды» is off gives no standard command; a command of its
// own placed in a group of a form is not the section's.
func (catalog *Catalog) SectionLines(members []uuid.UUID) []SectionLine {
	var lines []SectionLine
	for _, member := range members {
		kind, object, ok := catalog.objectByID(member)
		if !ok {
			continue
		}
		id := member
		field := func(name string) reflect.Value { return object.FieldByName(name) }
		standard := field("UseStandardCommands")
		if !standard.IsValid() || standard.Bool() {
			for _, command := range kinds.Must(kind).SectionCommands() {
				if !sectionCondition(command.When, object) {
					continue
				}
				line := SectionLine{InterfaceCommand: InterfaceCommand{Object: &id, Standard: StandardCommand(command.Standard)},
					Group: InterfaceGroup{Standard: InterfaceGroupKind(command.Group)}, Kind: kind, Visible: command.Visible,
					Title: headOf(object.Interface()).title}
				presentation := map[kinds.SectionCommandTitle]string{kinds.ListTitle: "ListPresentation", kinds.CreateTitle: "ObjectPresentation"}[command.Title]
				if said := field(presentation); presentation != "" && said.IsValid() && len(said.Interface().(LocalizedText)) > 0 {
					line.Title = said.Interface().(LocalizedText)
				}
				lines = append(lines, line)
			}
		}
		own := func(commandID uuid.UUID, title LocalizedText, group string, groupRef *uuid.UUID) {
			if placed, ok := catalog.sectionGroup(group, groupRef); ok {
				lines = append(lines, SectionLine{InterfaceCommand: InterfaceCommand{Command: &commandID}, Group: placed, Kind: kind, Title: title, Visible: true})
			}
		}
		if kind == CommonCommandKind {
			command := object.Interface().(CommonCommandDefinition)
			own(command.ID, command.Title, command.Group, command.GroupRef)
			continue
		}
		// A role's Commands are its rights on commands, not commands of its
		// own: only an object's own commands are the section's.
		if commands := field("Commands"); commands.IsValid() {
			objectCommands, _ := commands.Interface().([]ObjectCommand)
			for _, command := range objectCommands {
				own(command.ID, command.Title, command.Group, command.GroupRef)
			}
		}
	}
	return lines
}

// OpensSectionLine says whether a role opens a command of a section - the
// filter «Отбор по ролям» of the windows of a command interface keeps the
// commands one of its roles opens (obshchee.md, «Видимость по ролям»;
// snapshots vopr-2.636/02-06): a command to create one or a folder of it by
// «Интерактивное добавление» of its object, its list by «Просмотр». The
// prototype was seen on these two alone; every other command - an object
// opened, an object's own command, a common command, a common form - is
// taken by «Просмотр» of itself the same way until it is seen (question В5
// of series 2.639). A right a role does not write is not granted.
func (role RoleDefinition) OpensSectionLine(line SectionLine) bool {
	right := PermissionView
	if line.Standard == StandardCreate || line.Standard == StandardCreateFolder {
		right = PermissionInteractiveInsert
	}
	for _, object := range role.Objects {
		if line.Object != nil && object.Object == *line.Object || line.Command != nil && object.Object == *line.Command {
			return slices.Contains(object.Operations, right)
		}
		if line.Command == nil {
			continue
		}
		for _, field := range object.Fields {
			if field.Field == line.Command.String() {
				return slices.Contains(field.Operations, right)
			}
		}
	}
	return false
}

// sectionCondition says whether an object gives a command its kind gives
// only some objects: an independent register of information, or one a
// recorder writes; a catalog with folders.
func sectionCondition(when kinds.SectionCommandCondition, object reflect.Value) bool {
	switch when {
	case kinds.EveryObject:
		return true
	case kinds.FolderObject:
		hierarchy := object.FieldByName("Hierarchy")
		return hierarchy.IsValid() && hierarchy.Interface().(Hierarchy).HasFolders()
	}
	mode := object.FieldByName("WriteMode")
	independent := mode.IsValid() && InformationRegisterWriteMode(mode.String()) == InformationRegisterIndependent
	return independent == (when == kinds.IndependentObject)
}

// sectionGroup is the group of a section a command placed in group - a
// standard group of the platform - or groupRef - a command group of the
// project - stands in; none for a group of a form.
func (catalog *Catalog) sectionGroup(group string, groupRef *uuid.UUID) (InterfaceGroup, bool) {
	ofSection := func(category CommandCategory) bool {
		return category == NavigationPanelCategory || category == ActionsPanelCategory
	}
	if groupRef != nil {
		index, ok := catalog.commandGroupByID[*groupRef]
		return InterfaceGroup{Group: groupRef}, ok && ofSection(catalog.CommandGroups[index].Category)
	}
	return InterfaceGroup{Standard: InterfaceGroupKind(group)}, ofSection(standardCommandGroups[group])
}

// objectByID finds an object of any kind the catalog keeps a list of by its
// identifier: its kind and its description; a common form, which the load
// reads as a form and keeps no list of, by the head kept of it.
func (catalog *Catalog) objectByID(id uuid.UUID) (Kind, reflect.Value, bool) {
	for _, info := range kindInfos {
		list, ok := catalog.objectsOf(info.kind)
		if !ok {
			continue
		}
		for index := range list.Len() {
			if item := list.Index(index); headOf(item.Interface()).id == id {
				return info.kind, item, true
			}
		}
	}
	for _, form := range catalog.commonFormHeads {
		if form.ID == id {
			return CommonFormKind, reflect.ValueOf(form), true
		}
	}
	return "", reflect.Value{}, false
}
