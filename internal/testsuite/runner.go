// Package testsuite discovers and executes BSL test procedures.
package testsuite

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
)

// Runtime is the protected metadata boundary required by the test runner.
type Runtime interface {
	vm.MetadataRuntime
	BeginTestExecution(context.Context) (context.Context, func(error) error, error)
}

type programEventRuntime interface {
	ConfigureProgramEvents(*bytecode.Program) error
}

type observedProgramEventRuntime interface {
	ConfigureProgramEventsWithObserver(*bytecode.Program, vm.InstructionObserver) error
}

type protectedRuntime interface {
	BeginTestExecutionWithOptions(context.Context, string) (context.Context, func(error) error, error)
}

type Status string

const (
	Passed Status = "passed"
	Failed Status = "failed"
)

// Case is one exported procedure from a BSL module under tests/.
type Case struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Module  string `json:"module"`
	Routine string `json:"routine"`
	Line    int    `json:"line"`
}

// Selection limits execution to one module or one routine. Empty means all.
type Selection struct {
	Path    string `json:"path,omitempty"`
	Routine string `json:"routine,omitempty"`
}

type Frame struct {
	Module  string `json:"module,omitempty"`
	Routine string `json:"routine"`
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
}

type Result struct {
	Case       Case          `json:"case"`
	Status     Status        `json:"status"`
	Duration   time.Duration `json:"duration"`
	Error      string        `json:"error,omitempty"`
	Stack      []Frame       `json:"stack,omitempty"`
	FinishedAt time.Time     `json:"finishedAt"`
}

type Report struct {
	Results    []Result      `json:"results"`
	Passed     int           `json:"passed"`
	Failed     int           `json:"failed"`
	Duration   time.Duration `json:"duration"`
	FinishedAt time.Time     `json:"finishedAt"`
	Role       string        `json:"role,omitempty"`
	Coverage   Coverage      `json:"coverage"`
}

type RunOptions struct {
	Selection Selection
	Role      string
}

// Discover returns deterministic test cases without executing module code.
func Discover(program *bytecode.Program) []Case {
	if program == nil {
		return nil
	}
	cases := make([]Case, 0)
	for index := range program.Functions {
		function := &program.Functions[index]
		if function.IsFunction || !function.Export || int(function.Module) >= len(program.Modules) {
			continue
		}
		module := program.Modules[function.Module]
		if !strings.HasPrefix(module.Source, "tests/") {
			continue
		}
		line := 1
		if len(function.Code) != 0 && function.Code[0].Span.Start.Line > 0 {
			line = function.Code[0].Span.Start.Line
		}
		cases = append(cases, Case{
			ID: module.Source + "#" + function.Name, Path: module.Source,
			Module: module.Name, Routine: function.Name, Line: line,
		})
	}
	sort.Slice(cases, func(left, right int) bool {
		if cases[left].Path != cases[right].Path {
			return cases[left].Path < cases[right].Path
		}
		if cases[left].Line != cases[right].Line {
			return cases[left].Line < cases[right].Line
		}
		return cases[left].Routine < cases[right].Routine
	})
	return cases
}

// Run executes selected tests sequentially in isolated VM contexts and
// protected transactions. Every transaction is finalized with rollback.
func Run(ctx context.Context, program *bytecode.Program, runtime Runtime, selection Selection) (Report, error) {
	return RunWithOptions(ctx, program, runtime, RunOptions{Selection: selection})
}

// RunWithOptions executes tests with a selected application role and coverage.
func RunWithOptions(ctx context.Context, program *bytecode.Program, runtime Runtime, options RunOptions) (Report, error) {
	if ctx == nil || runtime == nil {
		return Report{}, fmt.Errorf("test runner requires context and metadata runtime")
	}
	machine, err := vm.New(program)
	if err != nil {
		return Report{}, err
	}
	cases := selectCases(Discover(program), options.Selection)
	if len(cases) == 0 && (options.Selection.Path != "" || options.Selection.Routine != "") {
		return Report{}, fmt.Errorf("selected BSL test was not found")
	}
	started := time.Now()
	coverage := newCoverageCollector(program)
	report := Report{Results: make([]Result, 0, len(cases)), Role: strings.TrimSpace(options.Role)}
	for _, test := range cases {
		if err := ctx.Err(); err != nil {
			return Report{}, err
		}
		result := runCase(ctx, machine, program, runtime, test, report.Role, coverage)
		report.Results = append(report.Results, result)
		if result.Status == Passed {
			report.Passed++
		} else {
			report.Failed++
		}
	}
	report.Duration = time.Since(started)
	report.FinishedAt = time.Now().UTC()
	report.Coverage = coverage.report()
	return report, nil
}

func selectCases(cases []Case, selection Selection) []Case {
	path, routine := strings.TrimSpace(selection.Path), strings.TrimSpace(selection.Routine)
	selected := make([]Case, 0, len(cases))
	for _, test := range cases {
		if path != "" && test.Path != path || routine != "" && !strings.EqualFold(test.Routine, routine) {
			continue
		}
		selected = append(selected, test)
	}
	return selected
}

func runCase(ctx context.Context, machine *vm.Machine, program *bytecode.Program, runtime Runtime, test Case, role string, coverage *coverageCollector) Result {
	started := time.Now()
	result := Result{Case: test}
	var err error
	if events, ok := runtime.(observedProgramEventRuntime); ok {
		err = events.ConfigureProgramEventsWithObserver(program, coverage)
	} else if events, ok := runtime.(programEventRuntime); ok {
		err = events.ConfigureProgramEvents(program)
	}
	if err != nil {
		err = fmt.Errorf("configure BSL object events: %w", err)
	}
	testContext, finish := ctx, (func(error) error)(nil)
	if err == nil {
		if protected, ok := runtime.(protectedRuntime); ok {
			testContext, finish, err = protected.BeginTestExecutionWithOptions(ctx, role)
		} else {
			testContext, finish, err = runtime.BeginTestExecution(ctx)
		}
	}
	if err == nil {
		if finish == nil {
			err = fmt.Errorf("test runtime did not provide transaction cleanup")
		} else {
			_, err = machine.NewContextWithMetadataAndObserver(runtime, coverage).CallContext(testContext, test.Module+"."+test.Routine)
		}
		if finish != nil {
			cleanupErr := finish(err)
			if cleanupErr != nil {
				if err == nil {
					err = cleanupErr
				} else {
					err = errors.Join(err, cleanupErr)
				}
			}
		}
	}
	result.Duration = time.Since(started)
	result.FinishedAt = time.Now().UTC()
	if err == nil {
		result.Status = Passed
		return result
	}
	result.Status, result.Error = Failed, err.Error()
	var runtimeError *vm.RuntimeError
	if errors.As(err, &runtimeError) {
		result.Stack = make([]Frame, 0, len(runtimeError.Stack))
		for _, frame := range runtimeError.Stack {
			result.Stack = append(result.Stack, Frame{
				Module: frame.Module, Routine: frame.Function,
				Path: frame.Filename, Line: frame.Span.Start.Line, Column: frame.Span.Start.Column,
			})
		}
	}
	return result
}
