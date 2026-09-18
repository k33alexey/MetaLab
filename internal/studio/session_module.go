package studio

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/k33alexey/MetaLab/internal/project"
)

// sessionModuleStub is what a session module starts as: the one predefined
// procedure, empty. The configuration root always has a session module in the
// tree, so opening it must produce something to write in rather than an error
// about a missing file.
const sessionModuleStub = "Процедура УстановкаПараметровСеанса(ИменаПараметровСеанса)\n\nКонецПроцедуры\n"

// sessionModuleNodeLocked reports the session module of this project. The node
// is always present, whether or not the file exists yet: the session module
// belongs to the configuration root the way the manifest does, and a developer
// looking for "where do session parameters get their values" must find it in
// the tree instead of having to know it can be created.
func (workspace *Workspace) sessionModuleNodeLocked() Node {
	value := "создан"
	if _, err := os.Stat(filepath.Join(workspace.root, project.SessionModuleFile)); os.IsNotExist(err) {
		value = "не создан"
	}
	return Node{
		ID: "session-module", Kind: "metadata", Title: "Модуль сеанса", Path: project.SessionModuleFile,
		Properties: []Property{{Name: "Файл", Value: project.SessionModuleFile}, {Name: "Состояние", Value: value}},
	}
}

// OpenSessionModule returns the project's session module, creating it with its
// empty predefined procedure the first time it is opened. Creation is part of
// opening on purpose: in the configuration there is exactly one session module
// and it always exists, so "create it" is not a decision to put to the
// developer - only its contents are.
func (workspace *Workspace) OpenSessionModule() (SourceFile, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	if _, err := project.ValidateLayout(workspace.root); err != nil {
		return SourceFile{}, err
	}
	file, err := workspace.readSource(project.SessionModuleFile)
	if err == nil {
		return file, nil
	}
	if err != ErrSourceNotFound {
		return SourceFile{}, err
	}
	target := filepath.Join(workspace.root, project.SessionModuleFile)
	created, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return SourceFile{}, fmt.Errorf("create session module: %w", err)
	}
	failed := true
	defer func() {
		_ = created.Close()
		if failed {
			_ = os.Remove(target)
		}
	}()
	if _, err := created.WriteString(sessionModuleStub); err != nil {
		return SourceFile{}, fmt.Errorf("write session module: %w", err)
	}
	if err := created.Sync(); err != nil {
		return SourceFile{}, fmt.Errorf("sync session module: %w", err)
	}
	if err := created.Close(); err != nil {
		return SourceFile{}, fmt.Errorf("close session module: %w", err)
	}
	failed = false
	workspace.invalidateStudioIndexesLocked()
	return sourceFile(project.SessionModuleFile, "bsl", []byte(sessionModuleStub)), nil
}

func registerSessionModuleRoutes(routes *http.ServeMux, workspace *Workspace) {
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
