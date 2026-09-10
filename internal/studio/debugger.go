package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/metadata"
)

const (
	maxStudioDebugArguments = 256
	maxStudioDebugModules   = 10_000
	maxStudioDebugSource    = 256 << 20
	maxStudioDebugWait      = 3 * time.Second
)

type studioDebugStart struct {
	Path        string               `json:"path"`
	Content     string               `json:"content"`
	Routine     string               `json:"routine"`
	Arguments   []json.RawMessage    `json:"arguments,omitempty"`
	Breakpoints []vm.DebugBreakpoint `json:"breakpoints,omitempty"`
}

func (workspace *Workspace) StartDebug(input studioDebugStart) (vm.DebugSnapshot, error) {
	relative, language, err := validateEditablePath(input.Path)
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	if language != "bsl" || !strings.HasPrefix(relative, "modules/") && !strings.HasPrefix(relative, "tests/") {
		return vm.DebugSnapshot{}, fmt.Errorf("debug source must be a BSL module")
	}
	if err := validateEditorSource(input.Content); err != nil {
		return vm.DebugSnapshot{}, err
	}
	if len(input.Arguments) > maxStudioDebugArguments {
		return vm.DebugSnapshot{}, fmt.Errorf("debug routine accepts at most %d supplied arguments", maxStudioDebugArguments)
	}
	arguments := make([]bytecode.Value, len(input.Arguments))
	for index, raw := range input.Arguments {
		arguments[index], err = decodeDebugArgument(raw, 0)
		if err != nil {
			return vm.DebugSnapshot{}, fmt.Errorf("debug argument %d: %w", index+1, err)
		}
	}

	workspace.mu.Lock()
	program, moduleName, err := workspace.compileDebugProgramLocked(relative, input.Content)
	if err != nil {
		workspace.mu.Unlock()
		return vm.DebugSnapshot{}, err
	}
	function, _, found := program.LookupInModule(debugModuleIndex(program, relative), strings.TrimSpace(input.Routine))
	if !found {
		workspace.mu.Unlock()
		return vm.DebugSnapshot{}, fmt.Errorf("routine %q was not found in %s", input.Routine, relative)
	}
	machine, err := vm.New(program)
	if err != nil {
		workspace.mu.Unlock()
		return vm.DebugSnapshot{}, err
	}
	if workspace.debugSession != nil {
		workspace.debugSession.Stop()
	}
	session, err := machine.NewContext().StartDebug(context.Background(), moduleName+"."+function.Name, input.Breakpoints, arguments...)
	if err == nil {
		workspace.debugSession = session
	}
	workspace.mu.Unlock()
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	wait, cancel := context.WithTimeout(context.Background(), maxStudioDebugWait)
	defer cancel()
	snapshot, err := session.Wait(wait, 0)
	if err != nil {
		session.Stop()
		return vm.DebugSnapshot{}, err
	}
	return snapshot, nil
}

func (workspace *Workspace) DebugSnapshot() (vm.DebugSnapshot, error) {
	session, err := workspace.currentDebugSession()
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	return session.Snapshot(), nil
}

func (workspace *Workspace) DebugCommand(action vm.DebugAction) (vm.DebugSnapshot, error) {
	session, err := workspace.currentDebugSession()
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	if err := session.Resume(action); err != nil {
		return vm.DebugSnapshot{}, err
	}
	return session.Snapshot(), nil
}

func (workspace *Workspace) SetDebugBreakpoints(values []vm.DebugBreakpoint) (vm.DebugSnapshot, error) {
	session, err := workspace.currentDebugSession()
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	if err := session.SetBreakpoints(values); err != nil {
		return vm.DebugSnapshot{}, err
	}
	return session.Snapshot(), nil
}

func (workspace *Workspace) PauseDebug() (vm.DebugSnapshot, error) {
	session, err := workspace.currentDebugSession()
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	if err := session.Pause(); err != nil {
		return vm.DebugSnapshot{}, err
	}
	return session.Snapshot(), nil
}

func (workspace *Workspace) StopDebug() (vm.DebugSnapshot, error) {
	session, err := workspace.currentDebugSession()
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	session.Stop()
	wait, cancel := context.WithTimeout(context.Background(), maxStudioDebugWait)
	defer cancel()
	return session.Wait(wait, session.Snapshot().Sequence)
}

func (workspace *Workspace) EvaluateDebug(frame int, expression string) (vm.DebugValue, error) {
	session, err := workspace.currentDebugSession()
	if err != nil {
		return vm.DebugValue{}, err
	}
	return session.Evaluate(frame, expression)
}

func (workspace *Workspace) currentDebugSession() (*vm.DebugSession, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	if workspace.debugSession == nil {
		return nil, fmt.Errorf("debug session is not started")
	}
	return workspace.debugSession, nil
}

func (workspace *Workspace) compileDebugProgramLocked(currentPath, currentContent string) (*bytecode.Program, string, error) {
	catalog, _ := metadata.Load(workspace.root)
	descriptors := workspace.moduleDescriptors(catalog)
	sources := make([]compiler.ModuleSource, 0, 32)
	currentFound, currentModule, sourceBytes := false, "", 0
	appendDirectory := func(directory string, onlyCurrent bool) error {
		entries, err := os.ReadDir(filepath.Join(workspace.root, directory))
		if err != nil {
			return fmt.Errorf("read BSL %s: %w", directory, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || filepath.Ext(entry.Name()) != ".bsl" {
				continue
			}
			relative := filepath.ToSlash(filepath.Join(directory, entry.Name()))
			if onlyCurrent && relative != currentPath {
				continue
			}
			if len(sources) >= maxStudioDebugModules {
				return fmt.Errorf("debugger supports at most %d BSL modules", maxStudioDebugModules)
			}
			content := currentContent
			if relative != currentPath {
				file, readErr := workspace.readSource(relative)
				if readErr != nil {
					return readErr
				}
				content = file.Content
			} else {
				currentFound = true
			}
			if sourceBytes > maxStudioDebugSource-len(content) {
				return fmt.Errorf("debugger BSL source exceeds %d bytes", maxStudioDebugSource)
			}
			sourceBytes += len(content)
			id := strings.TrimSuffix(entry.Name(), ".bsl")
			descriptor := descriptors[id]
			if descriptor.name == "" {
				descriptor.name = "Модуль" + strings.ReplaceAll(id, "-", "")
				if directory == "tests" {
					descriptor.name = "Тест" + strings.ReplaceAll(id, "-", "")
				}
			}
			if relative == currentPath {
				currentModule = descriptor.name
			}
			sources = append(sources, compiler.ModuleSource{
				Name: descriptor.name, Filename: relative, Source: content,
				PredefinedVariables: append([]string(nil), descriptor.predefined...),
			})
		}
		return nil
	}
	if err := appendDirectory("modules", false); err != nil {
		return nil, "", err
	}
	if strings.HasPrefix(currentPath, "tests/") {
		if err := appendDirectory("tests", true); err != nil {
			return nil, "", err
		}
	}
	if !currentFound {
		return nil, "", ErrSourceNotFound
	}
	program, diagnostics := compiler.CompileModules(sources)
	if len(diagnostics) != 0 {
		return nil, "", fmt.Errorf("cannot start debugger: %s", diagnostics[0].Error())
	}
	return program, currentModule, nil
}

func debugModuleIndex(program *bytecode.Program, source string) uint16 {
	for index := range program.Modules {
		if program.Modules[index].Source == source {
			return uint16(index)
		}
	}
	return ^uint16(0)
}

func decodeDebugArgument(raw json.RawMessage, depth int) (bytecode.Value, error) {
	if depth > 64 {
		return bytecode.Undefined(), fmt.Errorf("JSON value nesting is too deep")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var source any
	if err := decoder.Decode(&source); err != nil {
		return bytecode.Undefined(), fmt.Errorf("invalid JSON value")
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return bytecode.Undefined(), fmt.Errorf("invalid JSON value")
	}
	switch value := source.(type) {
	case nil:
		return bytecode.Null(), nil
	case bool:
		return bytecode.Boolean(value), nil
	case string:
		return bytecode.String(value), nil
	case json.Number:
		return bytecode.ParseNumber(value.String())
	case []any:
		if len(value) > 100_000 {
			return bytecode.Undefined(), fmt.Errorf("JSON array is too large")
		}
		items := make([]bytecode.Value, len(value))
		for index, item := range value {
			encoded, err := json.Marshal(item)
			if err != nil {
				return bytecode.Undefined(), err
			}
			items[index], err = decodeDebugArgument(encoded, depth+1)
			if err != nil {
				return bytecode.Undefined(), err
			}
		}
		return bytecode.Array(items...), nil
	default:
		return bytecode.Undefined(), fmt.Errorf("only JSON primitives and arrays are supported")
	}
}

func registerDebugRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("POST /api/debug/start", func(response http.ResponseWriter, request *http.Request) {
		var input studioDebugStart
		if !decodeDebugRequest(response, request, &input, 2*MaxEditableFileBytes+(1<<20)) {
			return
		}
		snapshot, err := workspace.StartDebug(input)
		writeDebugResponse(response, snapshot, err)
	})
	routes.HandleFunc("GET /api/debug/state", func(response http.ResponseWriter, _ *http.Request) {
		snapshot, err := workspace.DebugSnapshot()
		writeDebugResponse(response, snapshot, err)
	})
	routes.HandleFunc("POST /api/debug/command", func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Action vm.DebugAction `json:"action"`
		}
		if !decodeStudioMutation(response, request, &input) {
			return
		}
		snapshot, err := workspace.DebugCommand(input.Action)
		writeDebugResponse(response, snapshot, err)
	})
	routes.HandleFunc("POST /api/debug/pause", func(response http.ResponseWriter, request *http.Request) {
		if !validateStudioMutation(response, request) {
			return
		}
		snapshot, err := workspace.PauseDebug()
		writeDebugResponse(response, snapshot, err)
	})
	routes.HandleFunc("PUT /api/debug/breakpoints", func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Breakpoints []vm.DebugBreakpoint `json:"breakpoints"`
		}
		if !decodeStudioMutation(response, request, &input) {
			return
		}
		snapshot, err := workspace.SetDebugBreakpoints(input.Breakpoints)
		writeDebugResponse(response, snapshot, err)
	})
	routes.HandleFunc("POST /api/debug/evaluate", func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Frame      int    `json:"frame"`
			Expression string `json:"expression"`
		}
		if !decodeStudioMutation(response, request, &input) {
			return
		}
		value, err := workspace.EvaluateDebug(input.Frame, input.Expression)
		writeDebugResponse(response, value, err)
	})
	routes.HandleFunc("DELETE /api/debug/session", func(response http.ResponseWriter, request *http.Request) {
		if !validateStudioMutation(response, request) {
			return
		}
		snapshot, err := workspace.StopDebug()
		writeDebugResponse(response, snapshot, err)
	})
}

func decodeDebugRequest(response http.ResponseWriter, request *http.Request, value any, limit int64) bool {
	if !validateStudioMutation(response, request) {
		return false
	}
	if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
		http.Error(response, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, limit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		http.Error(response, "Invalid request", http.StatusBadRequest)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(response, "Invalid request", http.StatusBadRequest)
		return false
	}
	return true
}

func writeDebugResponse(response http.ResponseWriter, value any, err error) {
	if err == nil {
		writeStudioJSON(response, value)
		return
	}
	status := http.StatusBadRequest
	if strings.Contains(err.Error(), "not started") {
		status = http.StatusNotFound
	} else if errors.Is(err, vm.ErrDebugNotPaused) || errors.Is(err, vm.ErrDebugFinished) {
		status = http.StatusConflict
	}
	http.Error(response, err.Error(), status)
}
