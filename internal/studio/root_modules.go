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

// rootModule is one module of the configuration root: what it is called in the
// tree, the file it lies in, the address Studio opens it by, and what it starts
// as when it is opened for the first time.
type rootModule struct {
	id    string
	title string
	file  string
	stub  string
}

// rootModules are the modules the configuration root owns. The session module
// answers where session parameters get their values; the application module is
// where the application itself starts and stops; the external connection
// module is what runs when something outside connects with nobody in front of
// it; the ordinary application module is carried from a transferred
// configuration and never runs.
//
// Each stub is the predefined routines of that module, empty. A module always
// stands in the tree whether or not its file exists, so opening one has to
// produce something to write in rather than an error about a missing file.
var rootModules = []rootModule{
	{
		id: "session-module", title: "Модуль сеанса", file: project.SessionModuleFile,
		stub: "Процедура УстановкаПараметровСеанса(ИменаПараметровСеанса)\n\nКонецПроцедуры\n",
	},
	{
		id: "application-module", title: "Модуль приложения", file: project.ApplicationModuleFile,
		stub: "Процедура ПередНачаломРаботыСистемы(Отказ)\n\nКонецПроцедуры\n\n" +
			"Процедура ПриНачалеРаботыСистемы()\n\nКонецПроцедуры\n\n" +
			"Процедура ПередЗавершениемРаботыСистемы(Отказ)\n\nКонецПроцедуры\n\n" +
			"Процедура ПриЗавершенииРаботыСистемы()\n\nКонецПроцедуры\n",
	},
	{
		id: "external-connection-module", title: "Модуль внешнего соединения",
		file: project.ExternalConnectionModuleFile,
		stub: "Процедура ПриНачалеРаботыСистемы()\n\nКонецПроцедуры\n\n" +
			"Процедура ПриЗавершенииРаботыСистемы()\n\nКонецПроцедуры\n",
	},
	{
		// Carried from a transferred configuration and never run: ML has no
		// ordinary application. It stands in the tree so that a developer sees
		// what used to happen at start-up, and it gets no stub - offering to
		// create a module nothing will ever call would be an invitation to
		// write code for nowhere.
		id: "ordinary-application-module", title: "Модуль обычного приложения",
		file: project.OrdinaryApplicationModuleFile,
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

// rootModuleNodesLocked reports the modules of the configuration root. Each
// node is always present, whether or not its file exists yet: a module of the
// root belongs to it the way its description does, and a developer looking for
// "where does the application start" must find it in the tree instead of having
// to know it can be created.
func (workspace *Workspace) rootModuleNodesLocked() []Node {
	nodes := make([]Node, 0, len(rootModules))
	for _, module := range rootModules {
		value := "создан"
		if _, err := os.Stat(filepath.Join(workspace.root, module.file)); os.IsNotExist(err) {
			value = "не создан"
		}
		nodes = append(nodes, Node{
			ID: module.id, Kind: "metadata", Title: module.title, Path: module.file,
			Properties: []Property{{Name: "Файл", Value: module.file}, {Name: "Состояние", Value: value}},
		})
	}
	return nodes
}

// rootPictureNodesLocked reports the two pictures the configuration root owns.
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
			if !entry.IsDir() {
				images = append(images, entry.Name())
			}
		}
		sort.Strings(images)
		nodes = append(nodes, Node{
			ID: "root-picture:" + picture, Kind: "metadata", Title: picture, Path: picture,
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
	routes.HandleFunc("POST /api/root-module/{id}", func(response http.ResponseWriter, request *http.Request) {
		if !validateStudioMutation(response, request) {
			return
		}
		file, err := workspace.OpenRootModule(request.PathValue("id"))
		if err != nil {
			writeSourceError(response, err)
			return
		}
		writeStudioJSON(response, file)
	})
	// The session module kept its own address: it had one before the root had
	// two modules, and an address that already works is not worth breaking.
	routes.HandleFunc("POST /api/session-module", func(response http.ResponseWriter, request *http.Request) {
		if !validateStudioMutation(response, request) {
			return
		}
		file, err := workspace.OpenSessionModule()
		if err != nil {
			writeSourceError(response, err)
			return
		}
		writeStudioJSON(response, file)
	})
}
