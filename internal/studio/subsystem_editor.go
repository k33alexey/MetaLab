package studio

import (
	"bytes"
	"errors"
	"net/http"
	"slices"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The window of a subsystem (docs/block-00/podsistema.md, «Окно»; snapshots
// podsistema/04-07): «Основные» - the name, the synonym, the comment, the
// two flags, «Командный интерфейс», the explanation, the picture -
// «Функциональные опции» - the options of the project, marked where the
// subsystem is in their content - and «Прочее» - the help and «Включать в
// содержание справки»; its palette of three tabs. «Состав» - the tree of
// the objects with a box at each and the objects held below. The window
// «Командный интерфейс», «Все подсистемы» and the menu of a subsystem are
// items of their own.

// SubsystemEditorSource is a subsystem as its window and palette read and
// write it.
type SubsystemEditorSource struct {
	windowHead
	Subsystem metadata.SubsystemDefinition `json:"subsystem"`
	// Composition is the tree of the page «Состав»: the objects of the kinds
	// a subsystem's window offers (metadata.SubsystemCompositionKinds).
	Composition []ObjectChoiceBranch `json:"composition"`
	// Pictures name the pictures the field «Картинка» shows and its window
	// «Выбор картинки» offers.
	Pictures PictureChoice `json:"pictures"`
}

// SubsystemWindowValue is what the window of a subsystem writes: the
// subsystem and, when its page «Функциональные опции» was shown, the
// options marked.
type SubsystemWindowValue struct {
	metadata.SubsystemDefinition
	FunctionalOptions *[]uuid.UUID `json:"functionalOptions,omitempty"`
}

func (source SubsystemEditorSource) windowOf() (metadata.SubsystemDefinition, windowHead) {
	return source.Subsystem, source.windowHead
}

func (SubsystemEditorSource) named(value metadata.SubsystemDefinition) (uuid.UUID, string) {
	return value.ID, value.Name
}

// subsystemShape is how the window names an object of its kind.
var subsystemShape = objectShape[metadata.SubsystemDefinition]{named: SubsystemEditorSource{}.named}

func (source SubsystemEditorSource) identity() (uuid.UUID, string) {
	return source.Subsystem.ID, source.Revision
}

// subsystemEditorAddress is where the window of a subsystem is read and
// written.
const subsystemEditorAddress = "/api/subsystem"

func (workspace *Workspace) ReadSubsystemEditor(relative string) (SubsystemEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readSubsystemEditorLocked(relative)
}

func (workspace *Workspace) readSubsystemEditorLocked(relative string) (SubsystemEditorSource, error) {
	value, head, loaded, err := readObjectWindow(workspace, metadata.SubsystemKind, relative, metadata.DecodeSubsystem, subsystemShape)
	if err != nil {
		return SubsystemEditorSource{}, err
	}
	return SubsystemEditorSource{windowHead: head, Subsystem: value, Composition: objectChoiceTree(loaded, metadata.SubsystemCompositionKinds()),
		Pictures: pictureChoice(loaded)}, nil
}

// SaveSubsystemWindow writes the subsystem and the options its window marks
// as one save (saveObjectWindow).
func (workspace *Workspace) SaveSubsystemWindow(relative string, window SubsystemWindowValue, revision string) (SubsystemEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	written, err := saveObjectWindow(workspace, relative, revision, window.SubsystemDefinition,
		windowMarks{FunctionalOptions: window.FunctionalOptions, Kind: metadata.SubsystemKind},
		func(relative string) (metadata.SubsystemDefinition, windowHead, error) {
			opened, err := workspace.readSubsystemEditorLocked(relative)
			return opened.Subsystem, opened.windowHead, err
		}, subsystemShape)
	if err != nil {
		return SubsystemEditorSource{}, err
	}
	return workspace.readSubsystemEditorLocked(written)
}

// CreateSubsystem makes the next subsystem of the root - «Подсистема1»,
// in the command interface and in the contents of the help (podsistema.md,
// «Новая подсистема») - by the one way every object is made
// (CreateObject) and opens its window.
func (workspace *Workspace) CreateSubsystem(name string) (SubsystemEditorSource, error) {
	return createObjectAs(workspace, metadata.SubsystemKind, name, nil, workspace.readSubsystemEditorLocked)
}

// DeleteSubsystem removes the file of a subsystem by the one order of steps
// of every deletion.
func (workspace *Workspace) DeleteSubsystem(relative, expectedRevision string) error {
	return deleteObjectFolder(workspace, metadata.SubsystemKind, relative, expectedRevision, workspace.readSubsystemEditorLocked)
}

// subsystemChildAddress and subsystemMoveAddress are where a subsystem is
// made under another and moved, subsystemSectionAddress where the window
// «Командный интерфейс» reads the groups of its section, and
// projectSectionsAddress where the root's reads its sections.
const (
	subsystemChildAddress   = "/api/subsystem/child"
	subsystemMoveAddress    = "/api/subsystem/move"
	subsystemSectionAddress = "/api/subsystem/section"
	projectSectionsAddress  = "/api/project/sections"
)

// SectionGroup is a group of the section of a subsystem as the window
// «Командный интерфейс» lists it (podsistema.md, «Командный интерфейс…»;
// snapshots series-2.603/42, 43): the group, its caption and its sign, the
// panel it is of - «Выбор группы команд» offers a command the groups of the
// panel it stands in by default (podsistema.md, «Досъёмка 28.10.2026, поздний
// вечер») - and the commands the content gives it (metadata.SectionLines).
type SectionGroup struct {
	Group    metadata.InterfaceGroup `json:"group"`
	Title    string                  `json:"title"`
	Icon     string                  `json:"icon"`
	Panel    SectionPanel            `json:"panel"`
	Commands []SectionCommandLine    `json:"commands"`
}

// SectionPanel is the panel of a section a group is of.
type SectionPanel string

const (
	NavigationPanel SectionPanel = "navigation"
	ActionsPanel    SectionPanel = "actions"
)

// SectionSubsystem is a subsection of a section - a subsystem under the
// subsystem in the command interface - under «Подразделы», by its synonym
// or its name, and the roles that open a command of its content or of the
// content of a subsystem under it: «Отбор по ролям» leaves out a subsection
// none of its roles opens anything of (obshchee.md, «Видимость по ролям»).
type SectionSubsystem struct {
	ID    uuid.UUID   `json:"id"`
	Title string      `json:"title"`
	Opens []uuid.UUID `json:"opens"`
}

// SectionRole is a role of the project in the order of the tree: by its
// name «Выбор объекта: Роль» lists it (snapshot series-2.603/45), by its
// synonym, or its name, its column of visibility is captioned
// (obshchee.md, «Список команд»).
type SectionRole struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Title string    `json:"title"`
	Icon  string    `json:"icon"`
}

// SubsystemSectionAnswer is what the window «Командный интерфейс» of a
// subsystem reads: the groups of the section, its subsections, the roles of
// the project and its name, the root of «Выбор объекта: Роль».
type SubsystemSectionAnswer struct {
	Groups      []SectionGroup     `json:"groups"`
	Subsections []SectionSubsystem `json:"subsections"`
	Roles       []SectionRole      `json:"roles"`
	Project     string             `json:"project"`
}

// SectionCommandLine is a command of a group, the sign of its kind and the
// roles that open it (metadata.RoleDefinition.OpensSectionLine).
type SectionCommandLine struct {
	metadata.SectionLine
	Icon  string      `json:"icon"`
	Opens []uuid.UUID `json:"opens"`
}

// sectionGroups are the groups of a section in the prototype's order:
// «Панель навигации.Важное», «.Обычное», the project's groups of the
// navigation panel in the order of the tree, «.См. также»; «Панель
// действий.Создать», «.Отчеты», «.Сервис», the project's groups of the
// actions panel. A project's group is captioned by its synonym, or its
// name.
func sectionGroups(loaded *metadata.Catalog, language string, members []uuid.UUID) []SectionGroup {
	standard := func(kind metadata.InterfaceGroupKind, title, icon string, panel SectionPanel) SectionGroup {
		return SectionGroup{Group: metadata.InterfaceGroup{Standard: kind}, Title: title, Icon: icon, Panel: panel}
	}
	project := func(category metadata.CommandCategory, panel string, of SectionPanel) []SectionGroup {
		var groups []SectionGroup
		for _, item := range loaded.CommandGroups {
			if item.Category != category {
				continue
			}
			id := item.ID
			groups = append(groups, SectionGroup{Group: metadata.InterfaceGroup{Group: &id}, Title: panel + "." + item.Title.Presentation(language, item.Name),
				Icon: "command-group", Panel: of})
		}
		return groups
	}
	// The signs of the groups are the set's own until the icons of the
	// window are drawn (the owner's eye): a group, «+», a report, a gear.
	groups := []SectionGroup{standard(metadata.NavigationPanelImportant, "Панель навигации.Важное", "command-group", NavigationPanel),
		standard(metadata.NavigationPanelOrdinary, "Панель навигации.Обычное", "command-group", NavigationPanel)}
	groups = append(groups, project(metadata.NavigationPanelCategory, "Панель навигации", NavigationPanel)...)
	groups = append(groups, standard(metadata.NavigationPanelSeeAlso, "Панель навигации.См. также", "command-group", NavigationPanel),
		standard(metadata.ActionsPanelCreate, "Панель действий.Создать", "add", ActionsPanel),
		standard(metadata.ActionsPanelReports, "Панель действий.Отчеты", "report", ActionsPanel),
		standard(metadata.ActionsPanelTools, "Панель действий.Сервис", "settings", ActionsPanel))
	groups = append(groups, project(metadata.ActionsPanelCategory, "Панель действий", ActionsPanel)...)
	for _, line := range loaded.SectionLines(members) {
		for index := range groups {
			if sameGroup(groups[index].Group, line.Group) {
				groups[index].Commands = append(groups[index].Commands, SectionCommandLine{SectionLine: line, Icon: sectionIcon(line),
					Opens: openingRoles(loaded, []metadata.SectionLine{line})})
				break
			}
		}
	}
	return groups
}

func sameGroup(one, other metadata.InterfaceGroup) bool {
	return one.Standard == other.Standard && one.Written == other.Written &&
		(one.Group == nil) == (other.Group == nil) && (one.Group == nil || *one.Group == *other.Group)
}

// sectionIcon is the sign of a command: of its kind's list, its creation,
// or the kind itself.
func sectionIcon(line metadata.SectionLine) string {
	if line.Command != nil {
		return "command"
	}
	return kinds.Must(line.Kind).Icon
}

// SubsystemSection answers the groups of the section of the subsystem whose
// content is members - the window's, saved or not - and its subsections.
func (workspace *Workspace) SubsystemSection(subsystem uuid.UUID, members []uuid.UUID) (SubsystemSectionAnswer, error) {
	return workspace.sectionAnswer(func(loaded *metadata.Catalog, language string) ([]SectionGroup, []SectionSubsystem) {
		return sectionGroups(loaded, language, members), subsections(loaded, language, &subsystem)
	})
}

// ProjectSections answers the sections of the application - the subsystems
// of the root in the command interface - the window «Командный интерфейс»
// of the root lists (konfiguraciya.md, «Окна»; snapshots konfiguraciya/16,
// 17, series-2.595/36-40): no groups of commands.
func (workspace *Workspace) ProjectSections() (SubsystemSectionAnswer, error) {
	return workspace.sectionAnswer(func(loaded *metadata.Catalog, language string) ([]SectionGroup, []SectionSubsystem) {
		return []SectionGroup{}, subsections(loaded, language, nil)
	})
}

// sectionAnswer is what a window of a command interface reads: the groups
// and the sections of (the project loaded, its main language), the roles of
// the project and its name, the root of «Выбор объекта: Роль».
func (workspace *Workspace) sectionAnswer(of func(*metadata.Catalog, string) ([]SectionGroup, []SectionSubsystem)) (SubsystemSectionAnswer, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return SubsystemSectionAnswer{}, err
	}
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		return SubsystemSectionAnswer{}, err
	}
	roles := []SectionRole{}
	for _, role := range loaded.KindObjects(metadata.RoleKind) {
		roles = append(roles, SectionRole{ID: role.ID, Name: role.Name, Title: role.Title.Presentation(configuration.DefaultLanguage, role.Name),
			Icon: kinds.Must(metadata.RoleKind).Icon})
	}
	groups, sections := of(loaded, configuration.DefaultLanguage)
	return SubsystemSectionAnswer{Groups: groups, Subsections: sections, Roles: roles, Project: configuration.Name}, nil
}

// openingRoles are the roles of the project that open one of lines at least,
// in the order of the project.
func openingRoles(loaded *metadata.Catalog, lines []metadata.SectionLine) []uuid.UUID {
	opening := []uuid.UUID{}
	for _, role := range loaded.Roles {
		if slices.ContainsFunc(lines, role.OpensSectionLine) {
			opening = append(opening, role.ID)
		}
	}
	return opening
}

// contentLines are the commands the content of a subsystem and of every
// subsystem under it gives their sections.
func contentLines(loaded *metadata.Catalog, subsystem metadata.SubsystemDefinition) []metadata.SectionLine {
	lines := loaded.SectionLines(subsystem.Members)
	for _, item := range loaded.Subsystems {
		if item.Parent != nil && *item.Parent == subsystem.ID {
			lines = append(lines, contentLines(loaded, item)...)
		}
	}
	return lines
}

// subsections are the subsystems under parent - the root where none - that
// are in the command interface, as the project reads them - the window
// orders them: a subsystem not in it is no section (help,
// Подсистема.ВключатьВКомандныйИнтерфейс), and «Все подсистемы» leaves it
// out too (podsistema.md, «Предсъёмка 28.10.2026»).
func subsections(loaded *metadata.Catalog, language string, parent *uuid.UUID) []SectionSubsystem {
	var found []SectionSubsystem
	for _, item := range loaded.Subsystems {
		if !sameParent(item.Parent, parent) || !item.IncludeInCommandInterface {
			continue
		}
		found = append(found, SectionSubsystem{ID: item.ID, Title: item.Title.Presentation(language, item.Name),
			Opens: openingRoles(loaded, contentLines(loaded, item))})
	}
	return found
}

func registerSubsystemEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("POST "+subsystemChildAddress, answerStudio(0, writeSourceError, func(input SubsystemChild) (any, error) {
		return workspace.CreateSubsystemChild(input)
	}))
	routes.HandleFunc("POST "+subsystemMoveAddress, answerStudio(0, writeSourceError, func(input SubsystemMove) (any, error) {
		return workspace.MoveSubsystem(input)
	}))
	routes.HandleFunc("POST "+subsystemSectionAddress, answerStudio(0, writeSourceError, func(input struct {
		Subsystem uuid.UUID   `json:"subsystem"`
		Members   []uuid.UUID `json:"members"`
	}) (any, error) {
		return workspace.SubsystemSection(input.Subsystem, input.Members)
	}))
	routes.HandleFunc("GET "+projectSectionsAddress, answerQuery(writeSourceError, func(*http.Request) (any, error) {
		return workspace.ProjectSections()
	}))
	registerObjectEditor(routes, subsystemEditorAddress, workspace.ReadSubsystemEditor, workspace.SaveSubsystemWindow, workspace.CreateSubsystem,
		workspace.DeleteSubsystem)
}

// SubsystemChild asks for a subsystem under Parent - «Добавить ▸
// Подчиненная подсистема» of the menu of a subsystem.
type SubsystemChild struct {
	Parent uuid.UUID `json:"parent"`
}

// SubsystemMove moves the subsystem described at Path, read at Revision,
// under Parent - none, the root of the project («Переместить подсистему»).
type SubsystemMove struct {
	Path     string     `json:"path"`
	Revision string     `json:"revision"`
	Parent   *uuid.UUID `json:"parent,omitempty"`
}

// CreateSubsystemChild makes a subsystem under parent, named the next number
// of «Подсистема» among its siblings - a child of «Подсистема1» is
// «Подсистема1» again (podsistema.md, «Новая подсистема») - with the values
// of every new subsystem, and opens its window.
func (workspace *Workspace) CreateSubsystemChild(child SubsystemChild) (SubsystemEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	parent, err := newObjectPath(metadata.SubsystemKind, child.Parent, "")
	if err != nil {
		return SubsystemEditorSource{}, err
	}
	if _, err := workspace.readSubsystemEditorLocked(parent); err != nil {
		return SubsystemEditorSource{}, err
	}
	// The node of a subsystem in the tree is named by its identifier.
	relative, err := workspace.createObjectInLocked(metadata.SubsystemKind, child.Parent.String(), "", func(fresh *metadata.Fresh) error {
		subsystem, _ := fresh.Value.(metadata.SubsystemDefinition)
		above := child.Parent
		subsystem.Parent = &above
		fresh.Value = subsystem
		return nil
	})
	if err != nil {
		return SubsystemEditorSource{}, err
	}
	return workspace.readSubsystemEditorLocked(relative)
}

// errSubsystemInsideItself refuses a subsystem moved under itself or under
// a subsystem of its own.
var errSubsystemInsideItself = errors.New("Подсистему нельзя переместить в неё саму или в подчиненную ей")

// MoveSubsystem puts a subsystem under another parent or the root: a name
// taken among the new siblings - «Имя не уникально!» - and a parent inside
// the subsystem itself are refused, and the subsystem stays where it was.
func (workspace *Workspace) MoveSubsystem(move SubsystemMove) (SubsystemEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	opened, err := workspace.readSubsystemEditorLocked(move.Path)
	if err != nil {
		return SubsystemEditorSource{}, err
	}
	if opened.Revision != move.Revision {
		return SubsystemEditorSource{}, ErrSourceChanged
	}
	// The new parent must not lie inside the subsystem, and the name must be
	// free among its new siblings - both checked before anything is written.
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		return SubsystemEditorSource{}, err
	}
	for above := move.Parent; above != nil; {
		if *above == opened.Subsystem.ID {
			return SubsystemEditorSource{}, errSubsystemInsideItself
		}
		parent, ok := loaded.SubsystemByID(*above)
		if !ok {
			return SubsystemEditorSource{}, errSubsystemInsideItself
		}
		above = parent.Parent
	}
	group := branchOf(string(metadata.SubsystemKind))
	if move.Parent != nil {
		group = move.Parent.String()
	}
	if !sameParent(opened.Subsystem.Parent, move.Parent) {
		if err := workspace.objectNameErrorLocked("", group, opened.Subsystem.Name); err != nil {
			return SubsystemEditorSource{}, err
		}
	}
	moved := opened.Subsystem
	moved.Parent = move.Parent
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, moved); err != nil {
		return SubsystemEditorSource{}, err
	}
	if _, err := workspace.saveSourceLocked(move.Path, encoded.String(), opened.Revision); err != nil {
		return SubsystemEditorSource{}, err
	}
	return workspace.readSubsystemEditorLocked(move.Path)
}

// sameParent says two parents of a subsystem are one - the root or the same
// subsystem.
func sameParent(one, other *uuid.UUID) bool {
	return one == nil && other == nil || one != nil && other != nil && *one == *other
}
