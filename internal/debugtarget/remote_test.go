package debugtarget

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
)

func TestRemoteEndpointRoutesAllDebuggerOperations(t *testing.T) {
	t.Parallel()
	endpoint, agent, err := NewRemoteEndpoint(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()
	agentResult := make(chan error, 1)
	go func() {
		operations := []Operation{
			StartOperation, SnapshotOperation, CommandOperation, PauseOperation,
			SetBreakpointsOperation, EvaluateOperation, StopOperation,
		}
		for _, operation := range operations {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			command, nextErr := agent.Next(ctx)
			cancel()
			if nextErr != nil {
				agentResult <- nextErr
				return
			}
			if command.Operation != operation {
				agentResult <- errors.New("unexpected operation " + string(command.Operation))
				return
			}
			reply := AgentReply{Version: ProtocolVersion, Sequence: command.Sequence}
			if operation == EvaluateOperation {
				reply.Value = &vm.DebugValue{Kind: "Number", Value: "42"}
			} else {
				state := vm.DebugPaused
				if operation == CommandOperation {
					if command.Action != vm.DebugStepOver {
						agentResult <- errors.New("unexpected debug action")
						return
					}
					state = vm.DebugRunning
				}
				if operation == StopOperation {
					state = vm.DebugStopped
				}
				reply.Snapshot = &vm.DebugSnapshot{Sequence: command.Sequence, State: state}
			}
			if operation == StartOperation {
				if command.Routine != "Рассчитать" || len(command.Arguments) != 1 || command.Arguments[0].Kind != "number" || command.Arguments[0].Text != "40" {
					agentResult <- errors.New("unexpected start payload")
					return
				}
			}
			if operation == SetBreakpointsOperation && (len(command.Breakpoints) != 1 || command.Breakpoints[0].Line != 3) {
				agentResult <- errors.New("unexpected breakpoints")
				return
			}
			if operation == EvaluateOperation && (command.Frame != 1 || command.Expression != "Сумма + 1") {
				agentResult <- errors.New("unexpected evaluation payload")
				return
			}
			if replyErr := agent.Reply(reply); replyErr != nil {
				agentResult <- replyErr
				return
			}
		}
		agentResult <- nil
	}()

	if snapshot, err := endpoint.Start(context.Background(), StartRequest{Routine: "Рассчитать", Arguments: []bytecode.Value{bytecode.Number(40)}}); err != nil || snapshot.State != vm.DebugPaused {
		t.Fatalf("Start() = %+v, %v", snapshot, err)
	}
	if _, err := endpoint.Snapshot(); err != nil {
		t.Fatal(err)
	}
	if snapshot, err := endpoint.Command(vm.DebugStepOver); err != nil || snapshot.State != vm.DebugRunning {
		t.Fatalf("Command() = %+v, %v", snapshot, err)
	}
	if _, err := endpoint.Pause(); err != nil {
		t.Fatal(err)
	}
	if _, err := endpoint.SetBreakpoints([]vm.DebugBreakpoint{{Filename: "modules/main.bsl", Line: 3}}); err != nil {
		t.Fatal(err)
	}
	if value, err := endpoint.Evaluate(1, "Сумма + 1"); err != nil || value.Value != "42" {
		t.Fatalf("Evaluate() = %+v, %v", value, err)
	}
	if snapshot, err := endpoint.Stop(); err != nil || snapshot.State != vm.DebugStopped {
		t.Fatalf("Stop() = %+v, %v", snapshot, err)
	}
	if err := <-agentResult; err != nil {
		t.Fatal(err)
	}
}

func TestRemoteEndpointCancellationAndDisconnect(t *testing.T) {
	t.Parallel()
	endpoint, agent, err := NewRemoteEndpoint(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, startErr := endpoint.Start(ctx, StartRequest{Routine: "Run"})
		result <- startErr
	}()
	command, err := agent.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("Start() error = %v", err)
	}
	if err := agent.Reply(AgentReply{Version: ProtocolVersion, Sequence: command.Sequence, Snapshot: &vm.DebugSnapshot{State: vm.DebugPaused}}); err == nil {
		t.Fatal("Reply() accepted a response after cancellation")
	}
	agent.Close()
	started := time.Now()
	if _, err := endpoint.Snapshot(); !errors.Is(err, ErrAgentDisconnected) {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if time.Since(started) > 100*time.Millisecond {
		t.Fatal("disconnected endpoint did not fail promptly")
	}
}

func TestRemoteAgentServesVMEndpoint(t *testing.T) {
	t.Parallel()
	program, diagnostics := compiler.CompileSource("modules/remote.bsl", "Функция Run(Value)\nResult = Value + 2;\nВозврат Result;\nКонецФункции")
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewVMEndpoint(machine.NewContext())
	if err != nil {
		t.Fatal(err)
	}
	endpoint, agent, err := NewRemoteEndpoint(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	served := make(chan error, 1)
	go func() { served <- agent.Serve(context.Background(), runtime) }()
	started, err := endpoint.Start(context.Background(), StartRequest{Routine: "Run", Arguments: []bytecode.Value{bytecode.Number(40)}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	snapshot := started
	for snapshot.State == vm.DebugRunning {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Millisecond):
			snapshot, err = endpoint.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if snapshot.State != vm.DebugPaused {
		t.Fatalf("entry = %+v", snapshot)
	}
	value, err := endpoint.Evaluate(0, "Value + 1")
	if err != nil || value.Value != "41" {
		t.Fatalf("Evaluate() = %+v, %v", value, err)
	}
	if _, err := endpoint.Command(vm.DebugContinue); err != nil {
		t.Fatal(err)
	}
	for snapshot.State != vm.DebugCompleted {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Millisecond):
			snapshot, err = endpoint.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if snapshot.Result == nil || snapshot.Result.Value != "42" {
		t.Fatalf("completed = %+v", snapshot)
	}
	agent.Close()
	if err := <-served; !errors.Is(err, ErrAgentDisconnected) {
		t.Fatalf("Serve() error = %v", err)
	}
}

func TestRemoteEndpointSerializesConcurrentCommands(t *testing.T) {
	t.Parallel()
	endpoint, agent, err := NewRemoteEndpoint(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()
	const calls = 100
	agentResult := make(chan error, 1)
	go func() {
		var previous uint64
		for range calls {
			command, nextErr := agent.Next(context.Background())
			if nextErr != nil {
				agentResult <- nextErr
				return
			}
			if command.Operation != SnapshotOperation || command.Sequence <= previous {
				agentResult <- errors.New("commands are not serialized")
				return
			}
			previous = command.Sequence
			if replyErr := agent.Reply(AgentReply{Version: ProtocolVersion, Sequence: command.Sequence, Snapshot: &vm.DebugSnapshot{State: vm.DebugPaused}}); replyErr != nil {
				agentResult <- replyErr
				return
			}
		}
		agentResult <- nil
	}()
	var wait sync.WaitGroup
	for range calls {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, snapshotErr := endpoint.Snapshot(); snapshotErr != nil {
				t.Error(snapshotErr)
			}
		}()
	}
	wait.Wait()
	if err := <-agentResult; err != nil {
		t.Fatal(err)
	}
}

func TestWireValuePreservesExactPrimitiveArguments(t *testing.T) {
	t.Parallel()
	number, err := bytecode.ParseNumber("999999999999999999999999999999.125")
	if err != nil {
		t.Fatal(err)
	}
	date, err := bytecode.ParseDate("20260910153045")
	if err != nil {
		t.Fatal(err)
	}
	value := bytecode.Array(number, bytecode.String("данные"), bytecode.Boolean(false), bytecode.Null(), bytecode.Undefined(), date)
	encoded, err := EncodeWireValue(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeWireValue(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.String() != value.String() {
		t.Fatalf("wire round trip = %s, want %s", decoded.String(), value.String())
	}
	structure, err := bytecode.ConstructCollection("Structure", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EncodeWireValue(structure); err == nil || !strings.Contains(err.Error(), "cannot cross") {
		t.Fatalf("structure EncodeWireValue() error = %v", err)
	}
	if _, err := DecodeWireValue(WireValue{Kind: "undefined", Text: "ambiguous"}); err == nil {
		t.Fatal("DecodeWireValue() accepted ambiguous payload")
	}
}

func TestRemoteEndpointRejectsMalformedAgentReply(t *testing.T) {
	t.Parallel()
	endpoint, agent, err := NewRemoteEndpoint(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()
	go func() {
		command, _ := agent.Next(context.Background())
		_ = agent.Reply(AgentReply{Version: ProtocolVersion, Sequence: command.Sequence, Snapshot: &vm.DebugSnapshot{State: "unknown"}})
	}()
	if _, err := endpoint.Snapshot(); err == nil || !strings.Contains(err.Error(), "invalid state") {
		t.Fatalf("Snapshot() error = %v", err)
	}
}

func TestRemoteEndpointRejectsOversizedAgentReply(t *testing.T) {
	t.Parallel()
	endpoint, agent, err := NewRemoteEndpoint(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()
	go func() {
		command, _ := agent.Next(context.Background())
		_ = agent.Reply(AgentReply{Version: ProtocolVersion, Sequence: command.Sequence, Snapshot: &vm.DebugSnapshot{
			State: vm.DebugPaused, Reason: strings.Repeat("x", maxRemoteError+1),
		}})
	}()
	if _, err := endpoint.Snapshot(); err == nil || !strings.Contains(err.Error(), "exceeds limits") {
		t.Fatalf("Snapshot() error = %v", err)
	}
}

func TestRemoteEndpointRejectsInvalidBreakpointBeforeTransport(t *testing.T) {
	t.Parallel()
	endpoint, agent, err := NewRemoteEndpoint(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()
	if _, err := endpoint.SetBreakpoints([]vm.DebugBreakpoint{{Filename: "modules/main.bsl", Line: 0}}); err == nil || !strings.Contains(err.Error(), "invalid breakpoint") {
		t.Fatalf("SetBreakpoints() error = %v", err)
	}
}

func TestAgentReplyIsDetachedFromRuntimeMemory(t *testing.T) {
	t.Parallel()
	endpoint, agent, err := NewRemoteEndpoint(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()
	result := make(chan vm.DebugSnapshot, 1)
	failure := make(chan error, 1)
	go func() {
		snapshot, snapshotErr := endpoint.Snapshot()
		if snapshotErr != nil {
			failure <- snapshotErr
			return
		}
		result <- snapshot
	}()
	command, err := agent.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	snapshot := &vm.DebugSnapshot{State: vm.DebugPaused, Frames: []vm.DebugFrame{{
		Routine: "Run", Variables: []vm.DebugVariable{{Name: "Value", Value: vm.DebugValue{Kind: "Number", Value: "42"}}},
	}}}
	if err := agent.Reply(AgentReply{Version: ProtocolVersion, Sequence: command.Sequence, Snapshot: snapshot}); err != nil {
		t.Fatal(err)
	}
	snapshot.Frames[0].Routine = "Changed"
	snapshot.Frames[0].Variables[0].Value.Value = "0"
	select {
	case err := <-failure:
		t.Fatal(err)
	case received := <-result:
		if received.Frames[0].Routine != "Run" || received.Frames[0].Variables[0].Value.Value != "42" {
			t.Fatalf("detached snapshot = %+v", received)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Snapshot() did not finish")
	}
}

func TestExecuteAgentCommandRejectsInvalidProtocolAndBounds(t *testing.T) {
	t.Parallel()
	runtime := &endpointStub{}
	reply := ExecuteAgentCommand(context.Background(), runtime, AgentCommand{Sequence: 1, Operation: SnapshotOperation})
	if !strings.Contains(reply.Error, "invalid remote debug command") {
		t.Fatalf("version error = %q", reply.Error)
	}
	reply = ExecuteAgentCommand(context.Background(), runtime, AgentCommand{
		Version: ProtocolVersion, Sequence: 2, Operation: EvaluateOperation, Frame: -1, Expression: "1",
	})
	if !strings.Contains(reply.Error, "invalid remote debug expression") {
		t.Fatalf("expression error = %q", reply.Error)
	}
}
