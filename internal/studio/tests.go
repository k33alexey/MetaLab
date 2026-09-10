package studio

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/testsuite"
)

// TestRuntimeProvider opens a fresh database-backed runtime for one test run.
type TestRuntimeProvider func(context.Context) (*metadata.Runtime, func(), error)

// SetTestRuntimeProvider connects this database-bound Studio to its Debug data.
func (workspace *Workspace) SetTestRuntimeProvider(provider TestRuntimeProvider) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	workspace.testRuntime = provider
}

func (workspace *Workspace) TestCases() ([]testsuite.Case, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	program, err := workspace.compileTestsLocked()
	if err != nil {
		return nil, err
	}
	return testsuite.Discover(program), nil
}

func (workspace *Workspace) RunTests(ctx context.Context, selection testsuite.Selection) (testsuite.Report, error) {
	workspace.mu.Lock()
	if workspace.testRunning {
		workspace.mu.Unlock()
		return testsuite.Report{}, fmt.Errorf("a test run is already in progress")
	}
	if workspace.debugDatabase.IsZero() || workspace.testRuntime == nil {
		workspace.mu.Unlock()
		return testsuite.Report{}, fmt.Errorf("tests require ML Studio connected to a running Debug database")
	}
	program, err := workspace.compileTestsLocked()
	if err != nil {
		workspace.mu.Unlock()
		return testsuite.Report{}, err
	}
	provider := workspace.testRuntime
	workspace.testRunning = true
	workspace.mu.Unlock()
	defer func() {
		workspace.mu.Lock()
		workspace.testRunning = false
		workspace.mu.Unlock()
	}()

	runtime, closeRuntime, err := provider(ctx)
	if err != nil {
		return testsuite.Report{}, err
	}
	if closeRuntime != nil {
		defer closeRuntime()
	}
	return testsuite.Run(ctx, program, runtime, selection)
}

func (workspace *Workspace) compileTestsLocked() (*bytecode.Program, error) {
	catalog, err := metadata.Load(workspace.root)
	if err != nil {
		return nil, err
	}
	descriptors := workspace.moduleDescriptors(catalog)
	sources := make([]compiler.ModuleSource, 0, 32)
	sourceBytes := 0
	for _, directory := range []string{"modules", "tests"} {
		entries, err := os.ReadDir(filepath.Join(workspace.root, directory))
		if err != nil {
			return nil, fmt.Errorf("read BSL %s: %w", directory, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || filepath.Ext(entry.Name()) != ".bsl" {
				continue
			}
			if len(sources) >= maxStudioDebugModules {
				return nil, fmt.Errorf("test runner supports at most %d BSL modules", maxStudioDebugModules)
			}
			relative := filepath.ToSlash(filepath.Join(directory, entry.Name()))
			file, err := workspace.readSource(relative)
			if err != nil {
				return nil, err
			}
			if sourceBytes > maxStudioDebugSource-len(file.Content) {
				return nil, fmt.Errorf("test BSL source exceeds %d bytes", maxStudioDebugSource)
			}
			sourceBytes += len(file.Content)
			id := strings.TrimSuffix(entry.Name(), ".bsl")
			descriptor := descriptors[id]
			if descriptor.name == "" {
				descriptor.name = "Модуль" + strings.ReplaceAll(id, "-", "")
				if directory == "tests" {
					descriptor.name = "Тест" + strings.ReplaceAll(id, "-", "")
				}
			}
			sources = append(sources, compiler.ModuleSource{
				Name: descriptor.name, Filename: relative, Source: file.Content,
				PredefinedVariables: append([]string(nil), descriptor.predefined...),
			})
		}
	}
	program, diagnostics := compiler.CompileModules(sources)
	if len(diagnostics) != 0 {
		return nil, fmt.Errorf("cannot run tests: %s", diagnostics[0].Error())
	}
	return program, nil
}

func registerTestRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("GET /api/tests", func(response http.ResponseWriter, _ *http.Request) {
		cases, err := workspace.TestCases()
		writeTestResponse(response, cases, err)
	})
	routes.HandleFunc("POST /api/tests/run", func(response http.ResponseWriter, request *http.Request) {
		var selection testsuite.Selection
		if !decodeStudioMutation(response, request, &selection) {
			return
		}
		report, err := workspace.RunTests(request.Context(), selection)
		writeTestResponse(response, report, err)
	})
}

func writeTestResponse(response http.ResponseWriter, value any, err error) {
	if err != nil {
		http.Error(response, err.Error(), http.StatusConflict)
		return
	}
	writeStudioJSON(response, value)
}
