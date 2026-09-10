package studio

import (
	"context"
	"fmt"
	"net/http"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
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
	if err := testsuite.EnsureConflictFree(ctx, workspace.root); err != nil {
		return testsuite.Report{}, err
	}
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
	return testsuite.CompileProject(workspace.root)
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
