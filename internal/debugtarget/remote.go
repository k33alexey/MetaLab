package debugtarget

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
)

const (
	// ProtocolVersion changes when the command/reply contract becomes incompatible.
	ProtocolVersion       uint16 = 1
	maxRemoteRoundTrip           = 5 * time.Second
	maxRemoteStopWait            = time.Second
	maxWireDepth                 = 64
	maxWireItems                 = 1 << 20
	maxWireTextBytes             = 16 << 20
	maxRemoteError               = 64 << 10
	maxRemoteArguments           = 256
	maxRemoteRoutine             = 1024
	maxRemoteExpression          = 16 << 10
	maxRemoteBreakpoints         = 10_000
	maxRemoteFrames              = 256
	maxRemoteVariables           = 1 << 16
	maxRemoteSnapshotText        = 16 << 20
)

var ErrAgentDisconnected = errors.New("debug target agent disconnected")

// Operation identifies one transport-neutral debugger request.
type Operation string

const (
	StartOperation          Operation = "start"
	SnapshotOperation       Operation = "snapshot"
	CommandOperation        Operation = "command"
	PauseOperation          Operation = "pause"
	SetBreakpointsOperation Operation = "setBreakpoints"
	EvaluateOperation       Operation = "evaluate"
	StopOperation           Operation = "stop"
)

// WireValue preserves exact primitive BSL arguments across process boundaries.
type WireValue struct {
	Kind    string      `json:"kind"`
	Text    string      `json:"text,omitempty"`
	Boolean bool        `json:"boolean,omitempty"`
	Ticks   int64       `json:"ticks,omitempty"`
	Items   []WireValue `json:"items,omitempty"`
}

// AgentCommand is delivered to a browser, user-session or job debug agent.
type AgentCommand struct {
	Version     uint16               `json:"version"`
	Sequence    uint64               `json:"sequence"`
	Operation   Operation            `json:"operation"`
	Routine     string               `json:"routine,omitempty"`
	Arguments   []WireValue          `json:"arguments,omitempty"`
	Breakpoints []vm.DebugBreakpoint `json:"breakpoints,omitempty"`
	Action      vm.DebugAction       `json:"action,omitempty"`
	Frame       int                  `json:"frame,omitempty"`
	Expression  string               `json:"expression,omitempty"`
}

// AgentReply completes exactly one command.
type AgentReply struct {
	Version  uint16            `json:"version"`
	Sequence uint64            `json:"sequence"`
	Snapshot *vm.DebugSnapshot `json:"snapshot,omitempty"`
	Value    *vm.DebugValue    `json:"value,omitempty"`
	Error    string            `json:"error,omitempty"`
}

// RemoteEndpoint routes Endpoint calls to one connected agent. AgentCommand and
// AgentReply can be carried by WebSocket, local IPC or an authenticated poller.
type RemoteEndpoint struct {
	context  context.Context
	cancel   context.CancelFunc
	commands chan AgentCommand
	replies  chan AgentReply
	callMu   sync.Mutex
	agentMu  sync.Mutex
	sequence uint64
	pending  uint64
}

// Agent is the runtime-facing half of a RemoteEndpoint.
type Agent struct{ endpoint *RemoteEndpoint }

// NewRemoteEndpoint creates both sides of a bounded debug command channel.
func NewRemoteEndpoint(ctx context.Context) (*RemoteEndpoint, *Agent, error) {
	if ctx == nil {
		return nil, nil, fmt.Errorf("remote debug context is required")
	}
	remoteContext, cancel := context.WithCancel(ctx)
	endpoint := &RemoteEndpoint{
		context: remoteContext, cancel: cancel,
		commands: make(chan AgentCommand), replies: make(chan AgentReply, 1),
	}
	return endpoint, &Agent{endpoint: endpoint}, nil
}

// Next waits for the next command to execute in the remote runtime.
func (agent *Agent) Next(ctx context.Context) (AgentCommand, error) {
	if agent == nil || agent.endpoint == nil || ctx == nil {
		return AgentCommand{}, ErrAgentDisconnected
	}
	select {
	case command := <-agent.endpoint.commands:
		agent.endpoint.agentMu.Lock()
		if agent.endpoint.pending != 0 {
			agent.endpoint.agentMu.Unlock()
			return AgentCommand{}, fmt.Errorf("remote debug agent already has a pending command")
		}
		agent.endpoint.pending = command.Sequence
		agent.endpoint.agentMu.Unlock()
		return command, nil
	case <-agent.endpoint.context.Done():
		return AgentCommand{}, ErrAgentDisconnected
	case <-ctx.Done():
		return AgentCommand{}, ctx.Err()
	}
}

// Reply returns the result for the currently pending command.
func (agent *Agent) Reply(reply AgentReply) error {
	if agent == nil || agent.endpoint == nil {
		return ErrAgentDisconnected
	}
	agent.endpoint.agentMu.Lock()
	if reply.Version != ProtocolVersion || reply.Sequence == 0 || agent.endpoint.pending != reply.Sequence {
		agent.endpoint.agentMu.Unlock()
		return fmt.Errorf("unexpected remote debug reply %d", reply.Sequence)
	}
	agent.endpoint.pending = 0
	agent.endpoint.agentMu.Unlock()
	reply = normalizeAgentReply(reply)
	reply = cloneAgentReply(reply)
	select {
	case agent.endpoint.replies <- reply:
		return nil
	case <-agent.endpoint.context.Done():
		return ErrAgentDisconnected
	default:
		return fmt.Errorf("remote debug reply channel is busy")
	}
}

func normalizeAgentReply(reply AgentReply) AgentReply {
	failure := func(err error) AgentReply {
		return AgentReply{Version: ProtocolVersion, Sequence: reply.Sequence, Error: boundedRemoteText(err.Error(), maxRemoteError)}
	}
	if len(reply.Error) > maxRemoteError || !utf8.ValidString(reply.Error) {
		return failure(fmt.Errorf("remote debug agent returned an invalid error"))
	}
	if reply.Error != "" {
		reply.Snapshot, reply.Value = nil, nil
		return reply
	}
	if (reply.Snapshot == nil) == (reply.Value == nil) {
		return failure(fmt.Errorf("remote debug agent returned an invalid reply"))
	}
	if reply.Snapshot != nil {
		if err := validateSnapshot(*reply.Snapshot); err != nil {
			return failure(err)
		}
	} else if err := validateDebugValue(*reply.Value); err != nil {
		return failure(err)
	}
	return reply
}

// Close disconnects the agent and releases any waiting Studio operation.
func (agent *Agent) Close() {
	if agent != nil && agent.endpoint != nil {
		agent.endpoint.cancel()
	}
}

// Serve executes commands sequentially against a runtime endpoint until the
// agent or caller disconnects. Network transports can use ExecuteAgentCommand
// with decoded AgentCommand messages instead.
func (agent *Agent) Serve(ctx context.Context, runtime Endpoint) error {
	if agent == nil || runtime == nil || ctx == nil {
		return fmt.Errorf("remote debug agent runtime is required")
	}
	for {
		command, err := agent.Next(ctx)
		if err != nil {
			return err
		}
		if err := agent.Reply(ExecuteAgentCommand(ctx, runtime, command)); err != nil {
			return err
		}
	}
}

// ExecuteAgentCommand applies one validated wire command to a local runtime.
func ExecuteAgentCommand(ctx context.Context, runtime Endpoint, command AgentCommand) AgentReply {
	reply := AgentReply{Version: ProtocolVersion, Sequence: command.Sequence}
	if ctx == nil || runtime == nil {
		reply.Error = "invalid remote debug command"
		return reply
	}
	if err := validateAgentCommand(command); err != nil {
		reply.Error = err.Error()
		return reply
	}
	var snapshot vm.DebugSnapshot
	var value vm.DebugValue
	var err error
	switch command.Operation {
	case StartOperation:
		arguments := make([]bytecode.Value, len(command.Arguments))
		total := 0
		for index, wire := range command.Arguments {
			arguments[index], err = decodeWireValue(wire, 0, &total)
			if err != nil {
				err = fmt.Errorf("decode remote debug argument %d: %w", index+1, err)
				break
			}
		}
		if err == nil {
			snapshot, err = runtime.Start(ctx, StartRequest{
				Routine: command.Routine, Arguments: arguments,
				Breakpoints: append([]vm.DebugBreakpoint(nil), command.Breakpoints...),
			})
		}
	case SnapshotOperation:
		snapshot, err = runtime.Snapshot()
	case CommandOperation:
		snapshot, err = runtime.Command(command.Action)
	case PauseOperation:
		snapshot, err = runtime.Pause()
	case SetBreakpointsOperation:
		snapshot, err = runtime.SetBreakpoints(append([]vm.DebugBreakpoint(nil), command.Breakpoints...))
	case EvaluateOperation:
		value, err = runtime.Evaluate(command.Frame, command.Expression)
	case StopOperation:
		snapshot, err = runtime.Stop()
	default:
		err = fmt.Errorf("unknown remote debug operation %q", command.Operation)
	}
	if err != nil {
		reply.Error = boundedRemoteText(err.Error(), maxRemoteError)
		return reply
	}
	if command.Operation == EvaluateOperation {
		reply.Value = &value
	} else {
		reply.Snapshot = &snapshot
	}
	return reply
}

func validateAgentCommand(command AgentCommand) error {
	if command.Version != ProtocolVersion || command.Sequence == 0 {
		return fmt.Errorf("invalid remote debug command")
	}
	if len(command.Arguments) > maxRemoteArguments || len(command.Breakpoints) > maxRemoteBreakpoints ||
		len(command.Routine) > maxRemoteRoutine || len(command.Expression) > maxRemoteExpression ||
		!utf8.ValidString(command.Routine) || !utf8.ValidString(command.Expression) {
		return fmt.Errorf("remote debug command exceeds limits")
	}
	for _, breakpoint := range command.Breakpoints {
		if breakpoint.Line < 1 || breakpoint.Line > 10_000_000 || breakpoint.Filename == "" ||
			len(breakpoint.Filename) > maxRemoteExpression || !utf8.ValidString(breakpoint.Filename) {
			return fmt.Errorf("remote debug command contains an invalid breakpoint")
		}
	}
	switch command.Operation {
	case StartOperation:
		if command.Routine == "" {
			return fmt.Errorf("remote debug routine is required")
		}
	case SnapshotOperation, PauseOperation, StopOperation:
	case CommandOperation:
		switch command.Action {
		case vm.DebugContinue, vm.DebugStepInto, vm.DebugStepOver, vm.DebugStepOut:
		default:
			return fmt.Errorf("invalid remote debug action %q", command.Action)
		}
	case SetBreakpointsOperation:
	case EvaluateOperation:
		if command.Frame < 0 || command.Expression == "" {
			return fmt.Errorf("invalid remote debug expression")
		}
	default:
		return fmt.Errorf("unknown remote debug operation %q", command.Operation)
	}
	return nil
}

func cloneAgentReply(source AgentReply) AgentReply {
	result := source
	if source.Value != nil {
		value := *source.Value
		result.Value = &value
	}
	if source.Snapshot != nil {
		snapshot := *source.Snapshot
		snapshot.Frames = append([]vm.DebugFrame(nil), source.Snapshot.Frames...)
		for index := range snapshot.Frames {
			snapshot.Frames[index].Variables = append([]vm.DebugVariable(nil), source.Snapshot.Frames[index].Variables...)
		}
		if source.Snapshot.Result != nil {
			value := *source.Snapshot.Result
			snapshot.Result = &value
		}
		result.Snapshot = &snapshot
	}
	return result
}

func (endpoint *RemoteEndpoint) Start(ctx context.Context, request StartRequest) (vm.DebugSnapshot, error) {
	if len(request.Arguments) > maxRemoteArguments || len(request.Breakpoints) > maxRemoteBreakpoints {
		return vm.DebugSnapshot{}, fmt.Errorf("remote debug start exceeds limits")
	}
	arguments := make([]WireValue, len(request.Arguments))
	total := 0
	for index, value := range request.Arguments {
		encoded, err := encodeWireValue(value, 0, &total)
		if err != nil {
			return vm.DebugSnapshot{}, fmt.Errorf("encode remote debug argument %d: %w", index+1, err)
		}
		arguments[index] = encoded
	}
	reply, err := endpoint.exchange(ctx, AgentCommand{
		Operation: StartOperation, Routine: request.Routine,
		Arguments: arguments, Breakpoints: append([]vm.DebugBreakpoint(nil), request.Breakpoints...),
	}, maxRemoteRoundTrip)
	return snapshotReply(reply, err)
}

func (endpoint *RemoteEndpoint) Snapshot() (vm.DebugSnapshot, error) {
	reply, err := endpoint.exchange(context.Background(), AgentCommand{Operation: SnapshotOperation}, maxRemoteRoundTrip)
	return snapshotReply(reply, err)
}

func (endpoint *RemoteEndpoint) Command(action vm.DebugAction) (vm.DebugSnapshot, error) {
	reply, err := endpoint.exchange(context.Background(), AgentCommand{Operation: CommandOperation, Action: action}, maxRemoteRoundTrip)
	return snapshotReply(reply, err)
}

func (endpoint *RemoteEndpoint) Pause() (vm.DebugSnapshot, error) {
	reply, err := endpoint.exchange(context.Background(), AgentCommand{Operation: PauseOperation}, maxRemoteRoundTrip)
	return snapshotReply(reply, err)
}

func (endpoint *RemoteEndpoint) SetBreakpoints(values []vm.DebugBreakpoint) (vm.DebugSnapshot, error) {
	if len(values) > maxRemoteBreakpoints {
		return vm.DebugSnapshot{}, fmt.Errorf("remote debugger supports at most %d breakpoints", maxRemoteBreakpoints)
	}
	reply, err := endpoint.exchange(context.Background(), AgentCommand{
		Operation: SetBreakpointsOperation, Breakpoints: append([]vm.DebugBreakpoint(nil), values...),
	}, maxRemoteRoundTrip)
	return snapshotReply(reply, err)
}

func (endpoint *RemoteEndpoint) Evaluate(frame int, expression string) (vm.DebugValue, error) {
	reply, err := endpoint.exchange(context.Background(), AgentCommand{
		Operation: EvaluateOperation, Frame: frame, Expression: expression,
	}, maxRemoteRoundTrip)
	if err != nil {
		return vm.DebugValue{}, err
	}
	if reply.Value == nil || reply.Snapshot != nil {
		return vm.DebugValue{}, fmt.Errorf("remote debug agent returned an invalid evaluation reply")
	}
	if err := validateDebugValue(*reply.Value); err != nil {
		return vm.DebugValue{}, err
	}
	return *reply.Value, nil
}

func (endpoint *RemoteEndpoint) Stop() (vm.DebugSnapshot, error) {
	reply, err := endpoint.exchange(context.Background(), AgentCommand{Operation: StopOperation}, maxRemoteStopWait)
	return snapshotReply(reply, err)
}

func (endpoint *RemoteEndpoint) exchange(ctx context.Context, command AgentCommand, timeout time.Duration) (AgentReply, error) {
	if endpoint == nil || ctx == nil {
		return AgentReply{}, ErrAgentDisconnected
	}
	endpoint.callMu.Lock()
	defer endpoint.callMu.Unlock()
	if err := endpoint.context.Err(); err != nil {
		return AgentReply{}, ErrAgentDisconnected
	}
	requestContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	endpoint.sequence++
	if endpoint.sequence == 0 {
		endpoint.sequence++
	}
	command.Sequence = endpoint.sequence
	command.Version = ProtocolVersion
	if err := validateAgentCommand(command); err != nil {
		return AgentReply{}, err
	}
	select {
	case endpoint.commands <- command:
	case <-endpoint.context.Done():
		return AgentReply{}, ErrAgentDisconnected
	case <-requestContext.Done():
		return AgentReply{}, requestContext.Err()
	}
	select {
	case reply := <-endpoint.replies:
		if reply.Version != ProtocolVersion {
			return AgentReply{}, fmt.Errorf("unsupported remote debug protocol version %d", reply.Version)
		}
		if reply.Sequence != command.Sequence {
			return AgentReply{}, fmt.Errorf("remote debug reply sequence %d does not match %d", reply.Sequence, command.Sequence)
		}
		if len(reply.Error) > maxRemoteError || !utf8.ValidString(reply.Error) {
			return AgentReply{}, fmt.Errorf("remote debug agent returned an invalid error")
		}
		if reply.Error != "" {
			return AgentReply{}, errors.New(reply.Error)
		}
		return reply, nil
	case <-endpoint.context.Done():
		endpoint.clearPending(command.Sequence)
		return AgentReply{}, ErrAgentDisconnected
	case <-requestContext.Done():
		endpoint.clearPending(command.Sequence)
		return AgentReply{}, requestContext.Err()
	}
}

func (endpoint *RemoteEndpoint) clearPending(sequence uint64) {
	endpoint.agentMu.Lock()
	if endpoint.pending == sequence {
		endpoint.pending = 0
	}
	endpoint.agentMu.Unlock()
}

func snapshotReply(reply AgentReply, err error) (vm.DebugSnapshot, error) {
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	if reply.Snapshot == nil || reply.Value != nil {
		return vm.DebugSnapshot{}, fmt.Errorf("remote debug agent returned an invalid snapshot reply")
	}
	if err := validateSnapshot(*reply.Snapshot); err != nil {
		return vm.DebugSnapshot{}, err
	}
	return *reply.Snapshot, nil
}

func validateSnapshot(snapshot vm.DebugSnapshot) error {
	switch snapshot.State {
	case vm.DebugRunning, vm.DebugPaused, vm.DebugCompleted, vm.DebugFailed, vm.DebugStopped:
	default:
		return fmt.Errorf("remote debug agent returned invalid state %q", snapshot.State)
	}
	textBytes := 0
	addText := func(values ...string) bool {
		for _, value := range values {
			if !utf8.ValidString(value) || len(value) > maxRemoteSnapshotText-textBytes {
				return false
			}
			textBytes += len(value)
		}
		return true
	}
	if len(snapshot.Frames) > maxRemoteFrames || len(snapshot.Error) > maxRemoteError ||
		len(snapshot.Message) > maxRemoteError || len(snapshot.Reason) > maxRemoteError ||
		!addText(snapshot.Error, snapshot.Message, snapshot.Reason) {
		return fmt.Errorf("remote debug snapshot exceeds limits")
	}
	variableCount := 0
	for _, frame := range snapshot.Frames {
		if len(frame.Variables) > maxRemoteVariables-variableCount || frame.ID < 0 || frame.Instruction < 0 ||
			frame.Location.Line < 0 || frame.Location.Column < 0 || frame.Location.EndLine < 0 || frame.Location.EndColumn < 0 ||
			len(frame.Module) > maxRemoteExpression || len(frame.Routine) > maxRemoteRoutine || len(frame.Location.Path) > maxRemoteExpression ||
			!addText(frame.Module, frame.Routine, frame.Location.Path) {
			return fmt.Errorf("remote debug frame exceeds limits")
		}
		variableCount += len(frame.Variables)
		for _, variable := range frame.Variables {
			if len(variable.Name) > maxRemoteRoutine || len(variable.Scope) > 128 ||
				!addText(variable.Name, variable.Scope) {
				return fmt.Errorf("remote debug variable exceeds limits")
			}
			if err := validateDebugValue(variable.Value); err != nil {
				return err
			}
			if !addText(variable.Value.Kind, variable.Value.Value) {
				return fmt.Errorf("remote debug snapshot exceeds limits")
			}
		}
	}
	if snapshot.Result != nil {
		if err := validateDebugValue(*snapshot.Result); err != nil {
			return err
		}
		if !addText(snapshot.Result.Kind, snapshot.Result.Value) {
			return fmt.Errorf("remote debug snapshot exceeds limits")
		}
	}
	return nil
}

func validateDebugValue(value vm.DebugValue) error {
	if value.Kind == "" || len(value.Kind) > 128 || len(value.Value) > (4<<10)+4 || !utf8.ValidString(value.Kind) || !utf8.ValidString(value.Value) {
		return fmt.Errorf("remote debug value exceeds limits")
	}
	return nil
}

func boundedRemoteText(value string, limit int) string {
	if !utf8.ValidString(value) {
		return "remote debug error contains invalid UTF-8"
	}
	if len(value) <= limit {
		return value
	}
	end := limit
	for end > 0 && !utf8.ValidString(value[:end]) {
		end--
	}
	return value[:end]
}

// EncodeWireValue converts a transferable BSL primitive/array argument.
func EncodeWireValue(value bytecode.Value) (WireValue, error) {
	total := 0
	return encodeWireValue(value, 0, &total)
}

func encodeWireValue(value bytecode.Value, depth int, total *int) (WireValue, error) {
	if depth > maxWireDepth || *total >= maxWireItems {
		return WireValue{}, fmt.Errorf("debug argument exceeds nesting or item limits")
	}
	*total++
	switch value.Kind() {
	case bytecode.UndefinedKind:
		return WireValue{Kind: "undefined"}, nil
	case bytecode.NullKind:
		return WireValue{Kind: "null"}, nil
	case bytecode.NumberKind:
		text, _ := value.NumberText()
		return WireValue{Kind: "number", Text: text}, nil
	case bytecode.StringKind:
		text, _ := value.AsString()
		if len(text) > maxWireTextBytes || !utf8.ValidString(text) {
			return WireValue{}, fmt.Errorf("debug string argument exceeds limits")
		}
		return WireValue{Kind: "string", Text: text}, nil
	case bytecode.BooleanKind:
		boolean, _ := value.AsBoolean()
		return WireValue{Kind: "boolean", Boolean: boolean}, nil
	case bytecode.DateKind:
		ticks, _ := value.DateTicks()
		return WireValue{Kind: "date", Ticks: ticks}, nil
	case bytecode.ArrayKind:
		length, _ := value.ArrayLength()
		if length > maxWireItems-*total {
			return WireValue{}, fmt.Errorf("debug array argument exceeds item limit")
		}
		items := make([]WireValue, length)
		for index := range items {
			item, _ := value.ArrayElement(index)
			encoded, err := encodeWireValue(item, depth+1, total)
			if err != nil {
				return WireValue{}, fmt.Errorf("array element %d: %w", index, err)
			}
			items[index] = encoded
		}
		return WireValue{Kind: "array", Items: items}, nil
	default:
		return WireValue{}, fmt.Errorf("BSL value %s cannot cross the debug transport", value.Kind())
	}
}

// DecodeWireValue validates and restores a transported BSL argument.
func DecodeWireValue(value WireValue) (bytecode.Value, error) {
	total := 0
	return decodeWireValue(value, 0, &total)
}

func decodeWireValue(value WireValue, depth int, total *int) (bytecode.Value, error) {
	if depth > maxWireDepth || *total >= maxWireItems || len(value.Text) > maxWireTextBytes || !utf8.ValidString(value.Text) {
		return bytecode.Undefined(), fmt.Errorf("debug wire value exceeds limits")
	}
	*total++
	switch value.Kind {
	case "undefined":
		if value.Text != "" || value.Boolean || value.Ticks != 0 || len(value.Items) != 0 {
			return bytecode.Undefined(), fmt.Errorf("invalid undefined debug wire value")
		}
		return bytecode.Undefined(), nil
	case "null":
		if value.Text != "" || value.Boolean || value.Ticks != 0 || len(value.Items) != 0 {
			return bytecode.Undefined(), fmt.Errorf("invalid null debug wire value")
		}
		return bytecode.Null(), nil
	case "number":
		if value.Text == "" || value.Boolean || value.Ticks != 0 || len(value.Items) != 0 {
			return bytecode.Undefined(), fmt.Errorf("invalid number debug wire value")
		}
		return bytecode.ParseNumber(value.Text)
	case "string":
		if value.Boolean || value.Ticks != 0 || len(value.Items) != 0 {
			return bytecode.Undefined(), fmt.Errorf("invalid string debug wire value")
		}
		return bytecode.String(value.Text), nil
	case "boolean":
		if value.Text != "" || value.Ticks != 0 || len(value.Items) != 0 {
			return bytecode.Undefined(), fmt.Errorf("invalid boolean debug wire value")
		}
		return bytecode.Boolean(value.Boolean), nil
	case "date":
		if value.Text != "" || value.Boolean || len(value.Items) != 0 {
			return bytecode.Undefined(), fmt.Errorf("invalid date debug wire value")
		}
		return bytecode.DateFromTicks(value.Ticks)
	case "array":
		if value.Text != "" || value.Boolean || value.Ticks != 0 {
			return bytecode.Undefined(), fmt.Errorf("invalid array debug wire value")
		}
		if len(value.Items) > maxWireItems-*total {
			return bytecode.Undefined(), fmt.Errorf("debug wire array exceeds item limit")
		}
		items := make([]bytecode.Value, len(value.Items))
		for index, item := range value.Items {
			decoded, err := decodeWireValue(item, depth+1, total)
			if err != nil {
				return bytecode.Undefined(), fmt.Errorf("array element %d: %w", index, err)
			}
			items[index] = decoded
		}
		return bytecode.Array(items...), nil
	default:
		return bytecode.Undefined(), fmt.Errorf("unknown debug wire value kind %q", value.Kind)
	}
}
