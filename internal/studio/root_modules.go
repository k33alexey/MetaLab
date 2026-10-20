package studio

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
		commands = append(commands, NodeModule{Command: module.command, Path: module.file, Root: module.id})
	}
	return commands
}

// rootPictureNodesLocked reports the pictures the configuration root owns.
// Unlike a module, a picture is shown only when it is there: a module always
// exists and only its contents are a decision, while a configuration without a
// logo is an ordinary configuration, and an empty branch offering to make one
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

// OpenRootModule returns one module of the configuration root, creating it with
// its empty predefined routines the first time it is opened. Creation is part
// of opening on purpose: in the configuration there is exactly one of each and
// it always exists, so "create it" is not a decision to put to the developer -
// only its contents are.
func (workspace *Workspace) OpenRootModule(id string) (SourceFile, error) {
	module, ok := rootModuleByID(id)
	if !ok {
		return SourceFile{}, fmt.Errorf("%q is not a module of the configuration root", id)
	}
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	if _, err := project.ValidateLayout(workspace.root); err != nil {
		return SourceFile{}, err
	}
	file, err := workspace.readSource(module.file)
	if err == nil {
		return file, nil
	}
	if err != ErrSourceNotFound {
		return SourceFile{}, err
	}
	target := filepath.Join(workspace.root, module.file)
	created, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return SourceFile{}, fmt.Errorf("create %s: %w", module.file, err)
	}
	failed := true
	defer func() {
		_ = created.Close()
		if failed {
			_ = os.Remove(target)
		}
	}()
	if _, err := created.WriteString(module.stub); err != nil {
		return SourceFile{}, fmt.Errorf("write %s: %w", module.file, err)
	}
	if err := created.Sync(); err != nil {
		return SourceFile{}, fmt.Errorf("sync %s: %w", module.file, err)
	}
	if err := created.Close(); err != nil {
		return SourceFile{}, fmt.Errorf("close %s: %w", module.file, err)
	}
	failed = false
	workspace.invalidateStudioIndexesLocked()
	return sourceFile(module.file, "bsl", []byte(module.stub)), nil
}

// OpenSessionModule opens the session module, the module the root has had
// since before it had two.
func (workspace *Workspace) OpenSessionModule() (SourceFile, error) {
	return workspace.OpenRootModule("session-module")
}

func registerRootModuleRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("POST /api/root-module/{id}", answerMarked(writeSourceError, func(request *http.Request) (any, error) {
		return workspace.OpenRootModule(request.PathValue("id"))
	}))
	// The session module kept its own address: it had one before the root had
	// two modules, and an address that already works is not worth breaking.
	routes.HandleFunc("POST /api/session-module", answerMarked(writeSourceError, func(*http.Request) (any, error) {
		return workspace.OpenSessionModule()
	}))
}
