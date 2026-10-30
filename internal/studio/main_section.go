package studio

import (
	"net/http"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// «Командный интерфейс основного раздела» - the command of the menu of the
// root (docs/block-00/konfiguraciya.md, «Окна», «Досъёмка 27.10.2026»,
// «Предсъёмка 28.10.2026»; snapshots konfiguraciya/21, 22,
// series-2.595/50-52, 69, 70, predsyomka-1/34-38): a modal window, the
// commands available at the left, the commands of the main section at the
// right. A command is of the main section where the section places it: the
// configurations being moved write a placement and a visibility for every
// command of it and nothing for any other (erp 139 and 139, acc 136 and
// 136), so the placements are its content.

// mainSectionAddress is where the window reads the main section and writes
// it.
const mainSectionAddress = "/api/project/main-section"

// MainSectionWindow is what the window reads: the command interface of the
// main section at its revision - none for no file; the groups of a section
// with the commands placed, each in the group it is placed in
// (sectionCommandsPane); the tree «Доступные команды» and the commands of
// each of its objects by its identifier; the roles, the name and the main
// language of the project.
type MainSectionWindow struct {
	Revision         string                          `json:"revision"`
	CommandInterface metadata.CommandInterface       `json:"commandInterface"`
	Groups           []SectionGroup                  `json:"groups"`
	Available        []ObjectChoiceBranch            `json:"available"`
	Commands         map[string][]SectionCommandLine `json:"commands"`
	Roles            []SectionRole                   `json:"roles"`
	Project          string                          `json:"project"`
	DefaultLanguage  string                          `json:"defaultLanguage"`
}

// mainSectionAvailable is the tree «Доступные команды» (predsyomka-1/34-36,
// series-2.595/52): the branches of the project tree whose objects give a
// section a command - «Общие» holding its own, the prototype's three of it
// being exchange plans, common commands and common forms, then constants to
// tasks - each object by its name, and the commands each gives a section by
// their identifiers of object, the same lines the content of a subsystem
// gives it (metadata.SectionLines), of the kinds a section may hold
// (metadata.SubsystemCompositionKinds). A branch of no such object is left
// out.
func mainSectionAvailable(loaded *metadata.Catalog) ([]ObjectChoiceBranch, map[string][]SectionCommandLine) {
	commands := map[string][]SectionCommandLine{}
	tree := shapedObjectChoiceTree(loaded, metadata.SubsystemCompositionKinds(), func(_ metadata.Kind, item *ObjectChoiceItem) bool {
		for _, line := range loaded.SectionLines([]uuid.UUID{item.ID}) {
			commands[item.ID.String()] = append(commands[item.ID.String()], SectionCommandLine{SectionLine: line, Icon: sectionIcon(line),
				Opens: openingRoles(loaded, []metadata.SectionLine{line})})
		}
		return len(commands[item.ID.String()]) > 0
	})
	return withObjects(tree), commands
}

// withObjects is a tree of branches without the branches that hold no
// object, at any depth.
func withObjects(tree []ObjectChoiceBranch) []ObjectChoiceBranch {
	var kept []ObjectChoiceBranch
	for _, branch := range tree {
		branch.Children = withObjects(branch.Children)
		if len(branch.Children) > 0 || len(branch.Objects) > 0 {
			kept = append(kept, branch)
		}
	}
	return kept
}

func (workspace *Workspace) ReadMainSection() (MainSectionWindow, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readMainSectionLocked(nil)
}

// ReadMainSectionAs reads the window as if the project said said - what
// its OK gave before and «Сохранить» of the root has not written yet: the
// window opened again shows it (the owner's decision of 29.10.2026, В7 of
// series 2.646).
func (workspace *Workspace) ReadMainSectionAs(said metadata.CommandInterface) (MainSectionWindow, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readMainSectionLocked(&said)
}

// readMainSectionLocked reads the window of the main section, its command
// interface said, or where nil the project's.
func (workspace *Workspace) readMainSectionLocked(said *metadata.CommandInterface) (MainSectionWindow, error) {
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return MainSectionWindow{}, err
	}
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		return MainSectionWindow{}, err
	}
	revision, err := workspace.rootDescriptionRevisionLocked(project.MainSectionCommandInterfaceFile)
	if err != nil {
		return MainSectionWindow{}, err
	}
	if said == nil {
		said = &loaded.MainSectionCommandInterface
	}
	window := MainSectionWindow{Revision: revision, CommandInterface: *said,
		Roles: projectRoles(loaded, configuration.DefaultLanguage), Project: configuration.Name, DefaultLanguage: configuration.DefaultLanguage}
	window.Available, window.Commands = mainSectionAvailable(loaded)
	window.Groups = sectionGroups(loaded, configuration.DefaultLanguage, nil)
	// A command placed stands in the group it is placed in, its home in
	// this window: the main section has no content of its own to give it
	// another. One the project does not have, or placed in a group the
	// window has not, is kept as written and not shown.
	for _, placed := range said.CommandsPlacement {
		line, ok := availableLine(window.Commands, placed.InterfaceCommand)
		if !ok {
			continue
		}
		for index := range window.Groups {
			if sameGroup(window.Groups[index].Group, placed.Group) {
				line.Group = placed.Group
				window.Groups[index].Commands = append(window.Groups[index].Commands, line)
				break
			}
		}
	}
	return window, nil
}

// availableLine is the line of «Доступные команды» that is command.
func availableLine(commands map[string][]SectionCommandLine, command metadata.InterfaceCommand) (SectionCommandLine, bool) {
	for _, lines := range commands {
		for _, line := range lines {
			if line.InterfaceCommand.Same(command) {
				return line, true
			}
		}
	}
	return SectionCommandLine{}, false
}

// The window is read here; what its OK gives is written by «Сохранить» of
// the root with configuration.yaml (SaveProjectEditor, RootWindowsValue).
func registerMainSectionRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("GET "+mainSectionAddress, answerQuery(writeSourceError, func(*http.Request) (any, error) {
		return workspace.ReadMainSection()
	}))
	routes.HandleFunc("POST "+mainSectionAddress+"/window", answerStudio(0, writeSourceError, func(input struct {
		Value metadata.CommandInterface `json:"value"`
	}) (any, error) {
		return workspace.ReadMainSectionAs(input.Value)
	}))
}
