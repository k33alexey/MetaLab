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
	// Owner is the object whose own command the line is; none for a
	// standard command and a common command.
	Owner *uuid.UUID `json:"owner,omitempty"`
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
				presentation := map[kinds.SectionCommandTitle]string{kinds.ListTitle: "ListPresentation", kinds.CreateTitle: "ObjectPresentation",
					kinds.FolderTitle: "ListPresentation"}[command.Title]
				if said := field(presentation); presentation != "" && said.IsValid() && len(said.Interface().(LocalizedText)) > 0 {
					line.Title = said.Interface().(LocalizedText)
				}
				lines = append(lines, line)
			}
		}
		own := func(commandID uuid.UUID, title LocalizedText, group string, groupRef *uuid.UUID, owner *uuid.UUID) {
			if placed, ok := catalog.sectionGroup(group, groupRef); ok {
				lines = append(lines, SectionLine{InterfaceCommand: InterfaceCommand{Command: &commandID}, Group: placed, Kind: kind, Title: title, Visible: true, Owner: owner})
			}
		}
		if kind == CommonCommandKind {
			command := object.Interface().(CommonCommandDefinition)
			own(command.ID, command.Title, command.Group, command.GroupRef, nil)
			continue
		}
		// A role's Commands are its rights on commands, not commands of its
		// own: only an object's own commands are the section's.
		if commands := field("Commands"); commands.IsValid() {
			objectCommands, _ := commands.Interface().([]ObjectCommand)
			for _, command := range objectCommands {
				own(command.ID, command.Title, command.Group, command.GroupRef, &id)
			}
		}
	}
	return lines
}

// OpensSectionLine says whether a role opens a command of a section - the
// filter «Отбор по ролям» of the windows of a command interface keeps the
// commands one of its roles opens (obshchee.md, «Видимость по ролям»;
// snapshots vopr-2.636/02-06, dosyomka-2639/06-11): a command to create one
// or a folder of it by «Интерактивное добавление» of its object, its list by
// «Просмотр»; a constant, a common form, a common command by «Просмотр» of
// itself; an object's own command by «Просмотр» of the command and of its
// object both. A report and a data processor were not seen apart from these
// (question В5 of series 2.639) and go by «Просмотр» of themselves. A right a
// role does not write is not granted.
func (role RoleDefinition) OpensSectionLine(line SectionLine) bool {
	right := PermissionView
	if line.Standard == StandardCreate || line.Standard == StandardCreateFolder {
		right = PermissionInteractiveInsert
	}
	if line.Owner != nil && line.Command != nil {
		object := role.PermissionOf(*line.Owner)
		if object == nil || !slices.Contains(object.Operations, PermissionView) {
			return false
		}
		at := slices.IndexFunc(object.Fields, func(field FieldPermission) bool { return field.Field == line.Command.String() })
		return at >= 0 && slices.Contains(object.Fields[at].Operations, PermissionView)
	}
	target := line.Object
	if target == nil {
		target = line.Command
	}
	object := role.PermissionOf(*target)
	return object != nil && slices.Contains(object.Operations, right)
}

// PermissionOf is what the role writes about an object, nil for nothing:
// its entry in the role's list, which a change through it changes.
func (role RoleDefinition) PermissionOf(object uuid.UUID) *ObjectPermission {
	at := slices.IndexFunc(role.Objects, func(entry ObjectPermission) bool { return entry.Object == object })
	if at < 0 {
		return nil
	}
	return &role.Objects[at]
}

// sectionCondition says whether an object gives a command its kind gives
// only some objects: an independent register of information, or one a
// recorder writes; a catalog or a chart of characteristic types with
// folders.
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
