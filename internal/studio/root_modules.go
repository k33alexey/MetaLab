package studio

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
)

// rootModule is one module of the configuration root: the command of the
// root's menu that opens it, the file it lies in, the address Studio opens it
// by, what it starts as when it is opened for the first time.
type rootModule struct {
	id      string
	command string
	file    string
	stub    string
}

// rootModules are the modules the configuration root owns, in the order of
// the commands of its menu: application, session, external connection, at
// the head of the menu (inventory of block 00, konfiguraciya.md, snapshot
// konfiguraciya/15-menyu). The session module
// answers where session parameters get their values; the application module is
// where the application itself starts and stops; the external connection
// module is what runs when something outside connects with nobody in front of
// it. The ordinary application module is not carried (owner, 14.10.2026).
//
// Each stub is the predefined routines of that module, empty. A module is
// always offered in the root's menu whether or not its file exists, so opening
// one has to produce something to write in rather than an error about a
// missing file.
var rootModules = []rootModule{
	{
		id: "application-module", command: "Открыть модуль приложения", file: project.ApplicationModuleFile,
		stub: "Процедура ПередНачаломРаботыСистемы(Отказ)\n\nКонецПроцедуры\n\n" +
			"Процедура ПриНачалеРаботыСистемы()\n\nКонецПроцедуры\n\n" +
			"Процедура ПередЗавершениемРаботыСистемы(Отказ)\n\nКонецПроцедуры\n\n" +
			"Процедура ПриЗавершенииРаботыСистемы()\n\nКонецПроцедуры\n",
	},
	{
		id: "session-module", command: "Открыть модуль сеанса", file: project.SessionModuleFile,
		stub: "Процедура УстановкаПараметровСеанса(ИменаПараметровСеанса)\n\nКонецПроцедуры\n",
	},
	{
		id: "external-connection-module", command: "Открыть модуль внешнего соединения",
		file: project.ExternalConnectionModuleFile,
		stub: "Процедура ПриНачалеРаботыСистемы()\n\nКонецПроцедуры\n\n" +
			"Процедура ПриЗавершенииРаботыСистемы()\n\nКонецПроцедуры\n",
	},
}

func rootModuleByID(id string) (rootModule, bool) {
	for _, module := range rootModules {
		if module.id == id {
			return module, true
		}
	}
	return rootModule{}, false
}

// rootModuleCommands are the commands of the root's menu that open its
// modules. A module of the root is no row of the tree, as the prototype's is
// not: it is opened from the root's menu whether or not its file exists, and
// made the first time it is opened.
func rootModuleCommands() []NodeModule {
	commands := make([]NodeModule, 0, len(rootModules))
	for _, module := range rootModules {
		commands = append(commands, NodeModule{Command: module.command, Path: module.file})
	}
	return commands
}

// rootPictureNodesLocked reports the pictures the configuration root owns.
// Unlike a module, a picture is shown only when it is there: a module always
// exists and only its contents are a decision, while a configuration without a
// picture of the main section is an ordinary configuration, and an empty branch offering to make one
// would be an offer, not a fact.
func (workspace *Workspace) rootPictureNodesLocked() []Node {
	var nodes []Node
	for _, picture := range project.RootPictureDirectories() {
		entries, err := os.ReadDir(filepath.Join(workspace.root, picture))
		if err != nil || len(entries) == 0 {
			continue
		}
		images := make([]string, 0, len(entries))
		for _, entry := range entries {
			if !entry.IsDir() && entry.Name() != project.RootPictureDescriptionFile {
				images = append(images, entry.Name())
			}
		}
		sort.Strings(images)
		nodes = append(nodes, Node{
			ID: "root-picture:" + picture, Kind: "metadata", Icon: pictureIcon, Title: picture, Path: picture,
			Properties: []Property{{Name: "Изображения", Value: strings.Join(images, ", ")}},
		})
	}
	return nodes
}

// OpenRootModule returns one module of the configuration root by its address
// (rootModules), made the first time it is opened (OpenModule).
func (workspace *Workspace) OpenRootModule(id string) (SourceFile, error) {
	module, ok := rootModuleByID(id)
	if !ok {
		return SourceFile{}, fmt.Errorf("%q is not a module of the project root", id)
	}
	return workspace.OpenModule(module.file)
}

// OpenModule returns a module the menu of its owner opens, making it the first
// time it is opened. Creation is part of opening on purpose: the prototype's
// menu offers every module its owner has - the root's, an object's, a
// constant's, a service's - whether or not it has text, so "create it" is not
// a decision to put to the developer, only its contents are. A module of the
// root starts as its empty predefined routines, any other empty. Only a
// module some menu offers is made (moduleStubLocked); any other missing file
// stays missing.
func (workspace *Workspace) OpenModule(relative string) (SourceFile, error) {
	relative, language, err := validateEditablePath(relative)
	if err != nil {
		return SourceFile{}, err
	}
	if language != "bsl" {
		return SourceFile{}, ErrInvalidSourcePath
	}
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	if _, err := project.ValidateLayout(workspace.root); err != nil {
		return SourceFile{}, err
	}
	file, err := workspace.readSource(relative)
	if err != ErrSourceNotFound {
		return file, err
	}
	stub, ok := workspace.moduleStubLocked(relative)
	if !ok {
		return SourceFile{}, ErrSourceNotFound
	}
	target := filepath.Join(workspace.root, filepath.FromSlash(relative))
	// A recalculation its register describes keeps no folder until its
	// module is first opened: the folder is made with the module, and taken
	// back with it when the module is not written.
	var made []string
	for folder := filepath.Dir(target); ; folder = filepath.Dir(folder) {
		if _, err := os.Lstat(folder); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return SourceFile{}, fmt.Errorf("inspect %s: %w", relative, err)
		}
		made = append(made, folder)
	}
	failed := true
	defer func() {
		if failed {
			for _, folder := range made {
				_ = os.Remove(folder)
			}
		}
	}()
	for index := len(made) - 1; index >= 0; index-- {
		if err := os.Mkdir(made[index], 0o755); err != nil {
			made = made[index+1:]
			return SourceFile{}, fmt.Errorf("create %s: %w", relative, err)
		}
	}
	created, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return SourceFile{}, fmt.Errorf("create %s: %w", relative, err)
	}
	defer func() {
		_ = created.Close()
		if failed {
			_ = os.Remove(target)
		}
	}()
	if _, err := created.WriteString(stub); err != nil {
		return SourceFile{}, fmt.Errorf("write %s: %w", relative, err)
	}
	if err := created.Sync(); err != nil {
		return SourceFile{}, fmt.Errorf("sync %s: %w", relative, err)
	}
	if err := created.Close(); err != nil {
		return SourceFile{}, fmt.Errorf("close %s: %w", relative, err)
	}
	failed = false
	workspace.invalidateStudioIndexesLocked()
	return sourceFile(relative, language, []byte(stub)), nil
}

// moduleStubLocked is what a missing module starts as when its owner's menu
// opens it, and whether the menu offers it at all: a module of the root, or a
// module of an object that is there, which its kind opens from the menu
// (kinds.Module.Command) and the object keeps by its description
// (metadata.Catalog.ObjectModules) - the object module of a table of no
// objects is not made. An object of the kind described inside its owner - a
// recalculation - has no description of its own: its owner's description is
// what says it is there, as the tree shows it (objectBranches), so while the
// project does not load its module is not made. While the project does not
// load, what any other object keeps by its description is not known, and
// every module of its kind is made.
func (workspace *Workspace) moduleStubLocked(relative string) (string, bool) {
	for _, module := range rootModules {
		if module.file == relative {
			return module.stub, true
		}
	}
	kind, name, rest, ok := project.SplitObjectPath(relative)
	if !ok || len(rest) == 0 {
		return "", false
	}
	file := rest[len(rest)-1]
	module, ok := kinds.ModuleOfFile(file)
	if !ok || module.Command() == "" {
		return "", false
	}
	folder := filepath.Join(workspace.root, filepath.FromSlash(path.Dir(relative)))
	if len(rest) == 3 {
		inner, ok := project.InnerKind(kind, rest[0])
		return "", ok && project.KeepsModule(inner, file) && workspace.describesInside(kind, name, inner, rest[1])
	}
	if len(rest) != 1 || !project.KeepsModule(kind, file) {
		return "", false
	}
	description := path.Join(path.Dir(relative), project.DescriptionFile(kind))
	head, err := yamlSourceHead(filepath.Join(folder, project.DescriptionFile(kind)), description, "")
	if err != nil {
		return "", false
	}
	catalog, _ := metadata.Load(workspace.root)
	return "", slices.Contains(catalog.ObjectModules(kinds.Kind(kind), head.id), module)
}

// OpenSessionModule opens the session module, the module the root has had
// since before it had two.
func (workspace *Workspace) OpenSessionModule() (SourceFile, error) {
	return workspace.OpenRootModule("session-module")
}

func registerRootModuleRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("POST /api/module", answerMarked(writeSourceError, func(request *http.Request) (any, error) {
		return workspace.OpenModule(request.URL.Query().Get("path"))
	}))
	routes.HandleFunc("POST /api/root-module/{id}", answerMarked(writeSourceError, func(request *http.Request) (any, error) {
		return workspace.OpenRootModule(request.PathValue("id"))
	}))
	// The session module kept its own address: it had one before the root had
	// two modules, and an address that already works is not worth breaking.
	routes.HandleFunc("POST /api/session-module", answerMarked(writeSourceError, func(*http.Request) (any, error) {
		return workspace.OpenSessionModule()
	}))
}

// describesInside says the object of a kind describes an object of the inner
// kind by the name - a calculation register its recalculation -, read off the
// branches of the tree, which show what the object describes inside itself.
func (workspace *Workspace) describesInside(kind, owner, inner, name string) bool {
	catalog, err := metadata.Load(workspace.root)
	if err != nil {
		return false
	}
	head, ok := catalog.ObjectHead(metadata.Kind(kind), owner)
	if !ok {
		return false
	}
	for _, branch := range catalog.ObjectBranches(metadata.Kind(kind), head.ID) {
		if string(branch.Kind) != inner {
			continue
		}
		for _, part := range branch.Parts {
			if part.Name == name {
				return true
			}
		}
	}
	return false
}
