package vm

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
)

const (
	maxDebugBreakpoints     = 10_000
	maxDebugExpressionBytes = 16 << 10
	maxDebugValueBytes      = 4 << 10
	maxDebugExpressionDepth = 64
)

var (
	ErrDebugNotPaused = errors.New("BSL debug session is not paused")
	ErrDebugFinished  = errors.New("BSL debug session has finished")
)

// DebugState identifies the lifecycle state of one isolated debug execution.
type DebugState string

const (
	DebugRunning   DebugState = "running"
	DebugPaused    DebugState = "paused"
	DebugCompleted DebugState = "completed"
	DebugFailed    DebugState = "failed"
	DebugStopped   DebugState = "stopped"
)

// DebugAction controls execution after a pause.
type DebugAction string

const (
	DebugContinue DebugAction = "continue"
	DebugStepInto DebugAction = "stepInto"
	DebugStepOver DebugAction = "stepOver"
	DebugStepOut  DebugAction = "stepOut"
)

// DebugBreakpoint is a one-based source line breakpoint.
type DebugBreakpoint struct {
	Filename string `json:"path"`
	Line     int    `json:"line"`
}

// DebugLocation links a paused frame to its source range.
type DebugLocation struct {
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	EndLine   int    `json:"endLine"`
	EndColumn int    `json:"endColumn"`
}

// DebugValue is a bounded presentation of a BSL runtime value.
type DebugValue struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// DebugVariable is a named local or module value.
type DebugVariable struct {
	Name  string     `json:"name"`
	Scope string     `json:"scope"`
	Value DebugValue `json:"value"`
}

// DebugFrame is one navigable frame, ordered from the current routine outward.
type DebugFrame struct {
	ID          int             `json:"id"`
	Module      string          `json:"module"`
	Routine     string          `json:"routine"`
	Location    DebugLocation   `json:"location"`
	Variables   []DebugVariable `json:"variables"`
	Instruction int             `json:"instruction"`
}

// DebugSnapshot is an immutable view of the latest debugger state.
type DebugSnapshot struct {
	Sequence uint64       `json:"sequence"`
	State    DebugState   `json:"state"`
	Reason   string       `json:"reason,omitempty"`
	Message  string       `json:"message,omitempty"`
	Frames   []DebugFrame `json:"frames,omitempty"`
	Result   *DebugValue  `json:"result,omitempty"`
	Error    string       `json:"error,omitempty"`
}

type debugStopKey struct {
	depth       int
	instruction int
	path        string
	line        int
}

type debugEvaluationFrame struct {
	variables map[string]bytecode.Value
	env       executionEnvironment
}

// DebugSession owns one asynchronously executing VM call.
type DebugSession struct {
	mu             sync.Mutex
	state          DebugState
	sequence       uint64
	reason         string
	message        string
	frames         []DebugFrame
	evaluation     []debugEvaluationFrame
	result         *DebugValue
	err            string
	breakpoints    map[string]map[int]struct{}
	mode           DebugAction
	targetDepth    int
	targetPath     string
	targetLine     int
	lastStop       debugStopKey
	hasLastStop    bool
	entered        bool
	pauseRequested bool
	changed        chan struct{}
	resume         chan struct{}
	context        context.Context
	cancel         context.CancelFunc
}

type debugLiveFrame struct {
	function    *bytecode.Function
	locals      []bytecode.Value
	modules     [][]bytecode.Value
	env         executionEnvironment
	instruction int
	span        syntax.Span
}

type debugRuntime struct {
	session *DebugSession
	program *bytecode.Program
	frames  []debugLiveFrame
}

// StartDebug starts the same server VM used by normal execution and pauses at
// the first executable instruction. The returned session is concurrency-safe.
func (runtimeContext *Context) StartDebug(ctx context.Context, name string, breakpoints []DebugBreakpoint, arguments ...bytecode.Value) (*DebugSession, error) {
	if ctx == nil {
		return nil, fmt.Errorf("debug context is nil")
	}
	function, ok := runtimeContext.machine.lookup(name)
	if !ok {
		return nil, fmt.Errorf("routine %q not found", name)
	}
	completed, err := completeArguments(function, arguments)
	if err != nil {
		return nil, err
	}
	if !allowsSide(function.Context, runtimeContext.env.side) {
		return nil, unavailableContextError(function, runtimeContext.env.side)
	}
	debugContext, cancel := context.WithCancel(ctx)
	session := &DebugSession{
		state: DebugRunning, mode: DebugStepInto, targetDepth: 0,
		breakpoints: make(map[string]map[int]struct{}), changed: make(chan struct{}), resume: make(chan struct{}, 1),
		context: debugContext, cancel: cancel,
	}
	if err := session.SetBreakpoints(breakpoints); err != nil {
		cancel()
		return nil, err
	}
	debugger := &debugRuntime{session: session, program: runtimeContext.machine.program}
	go runtimeContext.runDebug(function, completed, debugger)
	return session, nil
}

func (runtimeContext *Context) runDebug(function *bytecode.Function, arguments []bytecode.Value, debugger *debugRuntime) {
	runtimeContext.mutex.Lock()
	defer runtimeContext.mutex.Unlock()
	ctx, finish, err := beginRuntimeExecution(debugger.session.context, runtimeContext.env.metadata)
	if err != nil {
		debugger.session.finish(bytecode.Undefined(), err)
		return
	}
	env := runtimeContext.env
	env.debug = debugger
	budget, err := newExecutionBudget(ctx, runtimeContext.machine.limits, arguments, &runtimeContext.memory)
	var result bytecode.Value
	if err == nil {
		result, err = executeWithValues(runtimeContext.machine.program, function, arguments, runtimeContext.modules, env, &budget)
		if current, valid := moduleValuesMemory(runtimeContext.machine.moduleMemory, runtimeContext.modules, runtimeContext.machine.limits.MaxMemoryBytes); valid {
			runtimeContext.memory = current
		} else {
			err = memoryLimitError(runtimeContext.machine.limits.MaxMemoryBytes)
		}
	}
	if err != nil {
		err = finalizeRuntimeError(runtimeContext.machine.program, err)
	}
	if finish != nil {
		if cleanupErr := finish(err); cleanupErr != nil {
			if err == nil {
				err = cleanupErr
			} else {
				err = errors.Join(err, cleanupErr)
			}
		}
	}
	debugger.session.finish(result, err)
}

// Snapshot returns a detached state view.
func (session *DebugSession) Snapshot() DebugSnapshot {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.snapshotLocked()
}

// Wait returns after a newer pause or terminal state is available.
func (session *DebugSession) Wait(ctx context.Context, after uint64) (DebugSnapshot, error) {
	for {
		session.mu.Lock()
		if session.sequence > after || terminalDebugState(session.state) {
			result := session.snapshotLocked()
			session.mu.Unlock()
			return result, nil
		}
		changed := session.changed
		session.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return DebugSnapshot{}, ctx.Err()
		}
	}
}

// Resume continues or steps from the current pause.
func (session *DebugSession) Resume(action DebugAction) error {
	switch action {
	case DebugContinue, DebugStepInto, DebugStepOver, DebugStepOut:
	default:
		return fmt.Errorf("unknown debug action %q", action)
	}
	session.mu.Lock()
	if session.state != DebugPaused {
		state := session.state
		session.mu.Unlock()
		if terminalDebugState(state) {
			return ErrDebugFinished
		}
		return ErrDebugNotPaused
	}
	depth := len(session.evaluation) - 1
	session.mode = action
	session.targetDepth = depth
	session.targetPath, session.targetLine = "", 0
	if action == DebugStepOut && len(session.frames) > 1 {
		session.targetPath = session.frames[1].Location.Path
		session.targetLine = session.frames[1].Location.Line
	}
	session.state = DebugRunning
	session.reason = ""
	session.message = ""
	session.notifyLocked()
	session.mu.Unlock()
	select {
	case session.resume <- struct{}{}:
		return nil
	case <-session.context.Done():
		return ErrDebugFinished
	}
}

// Pause requests a stop at the next instruction boundary.
func (session *DebugSession) Pause() error {
	session.mu.Lock()
	defer session.mu.Unlock()
	if terminalDebugState(session.state) {
		return ErrDebugFinished
	}
	if session.state == DebugRunning {
		session.pauseRequested = true
	}
	return nil
}

// Stop cancels execution, including a session waiting at a breakpoint.
func (session *DebugSession) Stop() {
	session.cancel()
}

// SetBreakpoints atomically replaces the active breakpoint collection.
func (session *DebugSession) SetBreakpoints(values []DebugBreakpoint) error {
	if len(values) > maxDebugBreakpoints {
		return fmt.Errorf("debugger supports at most %d breakpoints", maxDebugBreakpoints)
	}
	next := make(map[string]map[int]struct{})
	for _, value := range values {
		path := filepath.ToSlash(strings.TrimSpace(value.Filename))
		if path == "" || value.Line < 1 || value.Line > 10_000_000 {
			return fmt.Errorf("invalid breakpoint %q:%d", value.Filename, value.Line)
		}
		if next[path] == nil {
			next[path] = make(map[int]struct{})
		}
		next[path][value.Line] = struct{}{}
	}
	session.mu.Lock()
	session.breakpoints = next
	session.mu.Unlock()
	return nil
}

// Evaluate calculates a side-effect-free expression in a selected paused frame.
func (session *DebugSession) Evaluate(frame int, source string) (DebugValue, error) {
	if source == "" || len(source) > maxDebugExpressionBytes || !utf8.ValidString(source) {
		return DebugValue{}, fmt.Errorf("debug expression must be valid UTF-8 and at most %d bytes", maxDebugExpressionBytes)
	}
	expression, err := parseDebugExpression(source)
	if err != nil {
		return DebugValue{}, err
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.state != DebugPaused {
		return DebugValue{}, ErrDebugNotPaused
	}
	if frame < 0 || frame >= len(session.evaluation) {
		return DebugValue{}, fmt.Errorf("debug frame %d does not exist", frame)
	}
	selected := session.evaluation[frame]
	ctx, cancel := context.WithTimeout(session.context, 2*time.Second)
	defer cancel()
	value, err := evaluateDebugExpression(ctx, expression, selected.variables, selected.env, 0)
	if err != nil {
		return DebugValue{}, err
	}
	return presentDebugValue(value), nil
}

func captureDebugFrame(function *bytecode.Function, locals []bytecode.Value, modules [][]bytecode.Value, env executionEnvironment, instruction int, span syntax.Span) debugLiveFrame {
	frame := debugLiveFrame{
		function: function, locals: append([]bytecode.Value(nil), locals...),
		modules: make([][]bytecode.Value, len(modules)), env: env, instruction: instruction, span: span,
	}
	frame.env.debug = nil
	for index := range modules {
		frame.modules[index] = append([]bytecode.Value(nil), modules[index]...)
	}
	return frame
}

func (debugger *debugRuntime) before(function *bytecode.Function, locals []bytecode.Value, modules [][]bytecode.Value, env executionEnvironment, depth, instruction int, operation bytecode.Instruction) (time.Duration, error) {
	debugger.prepareFrame(function, depth, instruction, operation.Span)
	path := debugFunctionPath(debugger.program, function)
	reason := debugger.session.pauseReason(path, depth, instruction, operation.Span.Start.Line)
	if reason == "" && operation.Opcode != bytecode.OpCall {
		select {
		case <-debugger.session.context.Done():
			return 0, fmt.Errorf("%w: %v", ErrExecutionCanceled, debugger.session.context.Err())
		default:
			return 0, nil
		}
	}
	frame := captureDebugFrame(function, locals, modules, env, instruction, operation.Span)
	debugger.frames[depth] = frame
	if reason == "" {
		return 0, nil
	}
	return debugger.pause(reason, "")
}

func (debugger *debugRuntime) prepareFrame(function *bytecode.Function, depth, instruction int, span syntax.Span) {
	if len(debugger.frames) > depth+1 {
		for index := depth + 1; index < len(debugger.frames); index++ {
			debugger.frames[index] = debugLiveFrame{}
		}
		debugger.frames = debugger.frames[:depth+1]
	}
	frame := debugLiveFrame{function: function, instruction: instruction, span: span}
	if len(debugger.frames) == depth {
		debugger.frames = append(debugger.frames, frame)
	} else {
		current := &debugger.frames[depth]
		current.function, current.instruction, current.span = function, instruction, span
	}
}

func (debugger *debugRuntime) exception(frame debugLiveFrame, depth int, message string) (time.Duration, error) {
	debugger.prepareFrame(frame.function, depth, frame.instruction, frame.span)
	debugger.frames[depth] = frame
	return debugger.pause("exception", message)
}

func (debugger *debugRuntime) pause(reason, message string) (time.Duration, error) {
	started := time.Now()
	session := debugger.session
	session.mu.Lock()
	if err := session.context.Err(); err != nil {
		session.mu.Unlock()
		return 0, fmt.Errorf("%w: %v", ErrExecutionCanceled, err)
	}
	session.state, session.reason, session.message = DebugPaused, reason, message
	session.frames, session.evaluation = debugger.snapshots()
	current := debugger.frames[len(debugger.frames)-1]
	path := debugFunctionPath(debugger.program, current.function)
	session.lastStop = debugStopKey{depth: len(debugger.frames) - 1, instruction: current.instruction, path: path, line: current.span.Start.Line}
	session.hasLastStop = true
	session.sequence++
	session.notifyLocked()
	session.mu.Unlock()
	select {
	case <-session.resume:
		return time.Since(started), nil
	case <-session.context.Done():
		return time.Since(started), fmt.Errorf("%w: %v", ErrExecutionCanceled, session.context.Err())
	}
}

func (debugger *debugRuntime) snapshots() ([]DebugFrame, []debugEvaluationFrame) {
	frames := make([]DebugFrame, 0, len(debugger.frames))
	evaluation := make([]debugEvaluationFrame, 0, len(debugger.frames))
	for sourceIndex := len(debugger.frames) - 1; sourceIndex >= 0; sourceIndex-- {
		live := debugger.frames[sourceIndex]
		module, path := "", debugFunctionPath(debugger.program, live.function)
		if int(live.function.Module) < len(debugger.program.Modules) {
			module = debugger.program.Modules[live.function.Module].Name
		}
		variables := make([]DebugVariable, 0, len(live.locals))
		values := make(map[string]bytecode.Value)
		for index, name := range live.function.LocalNames {
			if name == "" || index >= len(live.locals) {
				continue
			}
			value := live.locals[index]
			variables = append(variables, DebugVariable{Name: name, Scope: "local", Value: presentDebugValue(value)})
			values[strings.ToLower(name)] = value
		}
		for moduleIndex, definition := range debugger.program.Modules {
			if moduleIndex >= len(live.modules) {
				continue
			}
			for variableIndex, item := range definition.Variables {
				if variableIndex >= len(live.modules[moduleIndex]) {
					continue
				}
				value := live.modules[moduleIndex][variableIndex]
				values[strings.ToLower(definition.Name+"."+item.Name)] = value
				if uint16(moduleIndex) == live.function.Module {
					if _, exists := values[strings.ToLower(item.Name)]; !exists {
						values[strings.ToLower(item.Name)] = value
					}
					variables = append(variables, DebugVariable{Name: item.Name, Scope: "module", Value: presentDebugValue(value)})
				}
			}
		}
		frames = append(frames, DebugFrame{
			ID: len(frames), Module: module, Routine: live.function.Name, Instruction: live.instruction,
			Location:  DebugLocation{Path: path, Line: live.span.Start.Line, Column: live.span.Start.Column, EndLine: live.span.End.Line, EndColumn: live.span.End.Column},
			Variables: variables,
		})
		evaluation = append(evaluation, debugEvaluationFrame{variables: values, env: live.env})
	}
	return frames, evaluation
}

func (session *DebugSession) pauseReason(path string, depth, instruction, line int) string {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.pauseRequested {
		session.pauseRequested = false
		return "pause"
	}
	if !session.entered {
		session.entered = true
		return "entry"
	}
	current := debugStopKey{depth: depth, instruction: instruction, path: path, line: line}
	different := !session.hasLastStop || current.depth != session.lastStop.depth || current.path != session.lastStop.path || current.line != session.lastStop.line
	if lines := session.breakpoints[path]; different && lines != nil {
		if _, exists := lines[line]; exists {
			return "breakpoint"
		}
	}
	if different {
		switch session.mode {
		case DebugStepInto:
			return "step"
		case DebugStepOver:
			if depth <= session.targetDepth {
				return "step"
			}
		case DebugStepOut:
			if depth < session.targetDepth && (path != session.targetPath || line != session.targetLine) {
				return "step"
			}
		}
	}
	return ""
}

func (session *DebugSession) finish(value bytecode.Value, err error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if errors.Is(err, ErrExecutionCanceled) || session.context.Err() != nil {
		session.state = DebugStopped
		session.reason = "stopped"
	} else if err != nil {
		session.state = DebugFailed
		session.reason = "error"
		session.err = err.Error()
	} else {
		session.state = DebugCompleted
		session.reason = "completed"
		presented := presentDebugValue(value)
		session.result = &presented
	}
	session.evaluation = nil
	session.sequence++
	session.notifyLocked()
	session.cancel()
}

func (session *DebugSession) notifyLocked() {
	close(session.changed)
	session.changed = make(chan struct{})
}

func (session *DebugSession) snapshotLocked() DebugSnapshot {
	result := DebugSnapshot{
		Sequence: session.sequence, State: session.state, Reason: session.reason, Message: session.message,
		Frames: append([]DebugFrame(nil), session.frames...), Result: session.result, Error: session.err,
	}
	for index := range result.Frames {
		result.Frames[index].Variables = append([]DebugVariable(nil), session.frames[index].Variables...)
	}
	if session.result != nil {
		value := *session.result
		result.Result = &value
	}
	return result
}

func terminalDebugState(state DebugState) bool {
	return state == DebugCompleted || state == DebugFailed || state == DebugStopped
}

func debugFunctionPath(program *bytecode.Program, function *bytecode.Function) string {
	if int(function.Module) < len(program.Modules) {
		return filepath.ToSlash(program.Modules[function.Module].Source)
	}
	return ""
}

func presentDebugValue(value bytecode.Value) DebugValue {
	text := value.String()
	if raw, ok := value.AsString(); ok {
		text = strconv.Quote(raw)
	}
	if len(text) > maxDebugValueBytes {
		end := maxDebugValueBytes
		for end > 0 && !utf8.ValidString(text[:end]) {
			end--
		}
		text = text[:end] + "…"
	}
	return DebugValue{Kind: value.Kind().String(), Value: text}
}

func parseDebugExpression(source string) (syntax.Expression, error) {
	module, diagnostics := syntax.Parse("<debug-expression>", "Функция Вычислить()\nВозврат "+source+";\nКонецФункции")
	if len(diagnostics) != 0 {
		return nil, fmt.Errorf("invalid debug expression: %s", diagnostics[0].Message)
	}
	if len(module.Routines) != 1 || len(module.Routines[0].Body) != 1 {
		return nil, fmt.Errorf("debug expression must contain one expression")
	}
	statement, ok := module.Routines[0].Body[0].(*syntax.ReturnStatement)
	if !ok || statement.Value == nil {
		return nil, fmt.Errorf("debug expression must contain one expression")
	}
	return statement.Value, nil
}

func evaluateDebugExpression(ctx context.Context, expression syntax.Expression, variables map[string]bytecode.Value, env executionEnvironment, depth int) (bytecode.Value, error) {
	if err := ctx.Err(); err != nil {
		return bytecode.Undefined(), err
	}
	if depth > maxDebugExpressionDepth {
		return bytecode.Undefined(), fmt.Errorf("debug expression nesting exceeds %d", maxDebugExpressionDepth)
	}
	evaluate := func(value syntax.Expression) (bytecode.Value, error) {
		return evaluateDebugExpression(ctx, value, variables, env, depth+1)
	}
	switch node := expression.(type) {
	case *syntax.IdentifierExpression:
		name := node.Name
		if node.Qualifier != "" {
			name = node.Qualifier + "." + node.Name
		}
		value, ok := variables[strings.ToLower(name)]
		if !ok {
			return bytecode.Undefined(), fmt.Errorf("unknown debug variable %s", name)
		}
		return value, nil
	case *syntax.NumberExpression:
		return bytecode.ParseNumber(node.Text)
	case *syntax.StringExpression:
		return bytecode.String(node.Value), nil
	case *syntax.BooleanExpression:
		return bytecode.Boolean(node.Value), nil
	case *syntax.DateExpression:
		return bytecode.ParseDate(node.Text)
	case *syntax.UndefinedExpression:
		return bytecode.Undefined(), nil
	case *syntax.NullExpression:
		return bytecode.Null(), nil
	case *syntax.GroupExpression:
		return evaluate(node.Expression)
	case *syntax.UnaryExpression:
		value, err := evaluate(node.Operand)
		if err != nil {
			return bytecode.Undefined(), err
		}
		switch node.Operator {
		case syntax.Plus:
			if value.Kind() != bytecode.NumberKind {
				return bytecode.Undefined(), fmt.Errorf("unary plus requires a number")
			}
			return value, nil
		case syntax.Minus:
			return bytecode.NegateNumber(value)
		case syntax.Not:
			boolean, ok := value.AsBoolean()
			if !ok {
				return bytecode.Undefined(), fmt.Errorf("Not requires a boolean")
			}
			return bytecode.Boolean(!boolean), nil
		}
	case *syntax.BinaryExpression:
		left, err := evaluate(node.Left)
		if err != nil {
			return bytecode.Undefined(), err
		}
		if node.Operator == syntax.And || node.Operator == syntax.Or {
			boolean, ok := left.AsBoolean()
			if !ok {
				return bytecode.Undefined(), fmt.Errorf("logical operation requires booleans")
			}
			if node.Operator == syntax.And && !boolean || node.Operator == syntax.Or && boolean {
				return left, nil
			}
		}
		right, err := evaluate(node.Right)
		if err != nil {
			return bytecode.Undefined(), err
		}
		opcode, ok := debugBinaryOpcode(node.Operator)
		if !ok {
			return bytecode.Undefined(), fmt.Errorf("unsupported debug operator %s", node.Operator)
		}
		return binary(opcode, left, right)
	case *syntax.MemberExpression:
		if identifier, ok := node.Receiver.(*syntax.IdentifierExpression); ok {
			if value, exists := variables[strings.ToLower(identifier.Name+"."+node.Name)]; exists {
				return value, nil
			}
		}
		receiver, err := evaluate(node.Receiver)
		if err != nil {
			return bytecode.Undefined(), err
		}
		if object, ok := receiver.AsRuntimeObject(); ok {
			return dispatchObjectProperty(ctx, env, object, node.Name)
		}
		return bytecode.CollectionProperty(receiver, node.Name)
	case *syntax.IndexExpression:
		receiver, err := evaluate(node.Collection)
		if err != nil {
			return bytecode.Undefined(), err
		}
		key, err := evaluate(node.Index)
		if err != nil {
			return bytecode.Undefined(), err
		}
		if object, ok := receiver.AsRuntimeObject(); ok {
			if name, isString := key.AsString(); isString {
				return dispatchObjectProperty(ctx, env, object, name)
			}
		}
		return bytecode.CollectionIndex(receiver, key)
	case *syntax.CallExpression, *syntax.NewExpression:
		return bytecode.Undefined(), fmt.Errorf("calls and constructors are not allowed in debug expressions")
	}
	return bytecode.Undefined(), fmt.Errorf("unsupported debug expression %T", expression)
}

func debugBinaryOpcode(kind syntax.Kind) (bytecode.Opcode, bool) {
	switch kind {
	case syntax.Plus:
		return bytecode.OpAdd, true
	case syntax.Minus:
		return bytecode.OpSubtract, true
	case syntax.Star:
		return bytecode.OpMultiply, true
	case syntax.Slash:
		return bytecode.OpDivide, true
	case syntax.Percent:
		return bytecode.OpModulo, true
	case syntax.Equal:
		return bytecode.OpEqual, true
	case syntax.NotEqual:
		return bytecode.OpNotEqual, true
	case syntax.Less:
		return bytecode.OpLess, true
	case syntax.LessEqual:
		return bytecode.OpLessEqual, true
	case syntax.Greater:
		return bytecode.OpGreater, true
	case syntax.GreaterEqual:
		return bytecode.OpGreaterEqual, true
	case syntax.And:
		return bytecode.OpAnd, true
	case syntax.Or:
		return bytecode.OpOr, true
	default:
		return 0, false
	}
}
