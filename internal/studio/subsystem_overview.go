package studio

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// «Все подсистемы» - the command of the branch «Подсистемы» and of the root
// (docs/block-00/podsistema.md, «Командный интерфейс, „Объекты
// подсистемы“, „Все подсистемы“», «Предсъёмка 28.10.2026»; snapshots
// series-2.603/51, 52, predsyomka-1/01-30): the tree of the subsystems in
// the command interface, the content of the one chosen and its command
// interface. It changes the content and the command interface of many
// subsystems, so it stands in the place of the open file, as «Все роли»
// does: «Сохранить» writes the subsystems changed at once.

// subsystemsAddress is where «Все подсистемы» reads every subsystem and
// writes those it changed.
const subsystemsAddress = "/api/subsystems"

// SubsystemOverview is what «Все подсистемы» reads: every subsystem of the
// project in the order of the tree - the window shows those in the command
// interface - where each is described and at what revision, the tree of the
// objects «Редактировать состав…» offers, and the name of the project, the
// root of «Выбор объекта».
type SubsystemOverview struct {
	Project         string                 `json:"project"`
	DefaultLanguage string                 `json:"defaultLanguage"`
	Subsystems      []SubsystemOverviewRow `json:"subsystems"`
	Composition     []ObjectChoiceBranch   `json:"composition"`
}

// SubsystemOverviewRow is a subsystem as «Все подсистемы» reads and writes
// it: where it is described, the revision read and the subsystem.
type SubsystemOverviewRow struct {
	Path      string                       `json:"path"`
	Revision  string                       `json:"revision"`
	Subsystem metadata.SubsystemDefinition `json:"subsystem"`
}

func (workspace *Workspace) ReadSubsystemOverview() (SubsystemOverview, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readSubsystemOverviewLocked()
}

func (workspace *Workspace) readSubsystemOverviewLocked() (SubsystemOverview, error) {
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return SubsystemOverview{}, err
	}
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		return SubsystemOverview{}, err
	}
	rows := []SubsystemOverviewRow{}
	for _, place := range subsystemPlaces(loaded, uuid.UUID{}) {
		file, err := workspace.readSource(place.path)
		if err != nil {
			return SubsystemOverview{}, err
		}
		subsystem, err := metadata.DecodeSubsystem(place.path, strings.NewReader(file.Content), configuration)
		if err != nil {
			return SubsystemOverview{}, err
		}
		rows = append(rows, SubsystemOverviewRow{Path: place.path, Revision: file.Revision, Subsystem: subsystem})
	}
	return SubsystemOverview{Project: configuration.Name, DefaultLanguage: configuration.DefaultLanguage, Subsystems: rows,
		Composition: objectChoiceTree(loaded, metadata.SubsystemCompositionKinds())}, nil
}

// SaveSubsystems writes the subsystems «Все подсистемы» changed as one save:
// each read at its revision, named and placed as it was - the window changes
// the content and the command interface, a subsystem is renamed in its own
// window and moved by «Переместить подсистему» - and all written or none
// (replaceSourcesLocked).
func (workspace *Workspace) SaveSubsystems(saves []SubsystemOverviewRow) (SubsystemOverview, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	changes, originals := map[string][]byte{}, map[string][]byte{}
	loaded, _ := metadata.Load(workspace.root)
	for _, save := range saves {
		opened, head, err := readObjectDescription(workspace, metadata.SubsystemKind, save.Path, metadata.DecodeSubsystem)
		if err != nil {
			return SubsystemOverview{}, err
		}
		if head.Revision != save.Revision {
			return SubsystemOverview{}, ErrSourceChanged
		}
		if save.Subsystem.ID != opened.ID || save.Subsystem.Name != opened.Name || !sameParent(save.Subsystem.Parent, opened.Parent) {
			return SubsystemOverview{}, fmt.Errorf("subsystem %q: all subsystems are saved with their names and places, a subsystem is renamed in its own window", opened.Name)
		}
		if _, twice := changes[save.Path]; twice {
			return SubsystemOverview{}, fmt.Errorf("subsystem %q is saved twice", opened.Name)
		}
		order := &save.Subsystem.CommandInterface.SubsystemsOrder
		*order = subsectionsKept(loaded, &opened.ID, opened.CommandInterface.SubsystemsOrder, *order)
		var encoded bytes.Buffer
		if err := metadata.Encode(&encoded, save.Subsystem); err != nil {
			return SubsystemOverview{}, err
		}
		content, err := workspace.validateYAMLSource(save.Path, encoded.Bytes())
		if err != nil {
			return SubsystemOverview{}, fmt.Errorf("subsystem %q: %w", opened.Name, err)
		}
		before, err := workspace.readSource(save.Path)
		if err != nil {
			return SubsystemOverview{}, err
		}
		changes[save.Path], originals[save.Path] = content, []byte(before.Content)
	}
	if err := workspace.replaceSourcesLocked(changes, originals); err != nil {
		return SubsystemOverview{}, err
	}
	workspace.invalidateStudioIndexesLocked()
	return workspace.readSubsystemOverviewLocked()
}

func registerSubsystemOverviewRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("GET "+subsystemsAddress, answerQuery(writeSourceError, func(*http.Request) (any, error) {
		return workspace.ReadSubsystemOverview()
	}))
	routes.HandleFunc("PUT "+subsystemsAddress, answerStudio(0, writeSourceError, func(input struct {
		Subsystems []SubsystemOverviewRow `json:"subsystems"`
	}) (any, error) {
		return workspace.SaveSubsystems(input.Subsystems)
	}))
}
