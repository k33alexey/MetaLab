package debugtarget

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

type clockStub struct {
	mu  sync.Mutex
	now time.Time
}

func (clock *clockStub) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *clockStub) Advance(duration time.Duration) {
	clock.mu.Lock()
	clock.now = clock.now.Add(duration)
	clock.mu.Unlock()
}

type endpointStub struct{ stops atomic.Int64 }

func (*endpointStub) Start(context.Context, StartRequest) (vm.DebugSnapshot, error) {
	return vm.DebugSnapshot{State: vm.DebugRunning}, nil
}
func (*endpointStub) Snapshot() (vm.DebugSnapshot, error) {
	return vm.DebugSnapshot{State: vm.DebugPaused}, nil
}
func (*endpointStub) Command(vm.DebugAction) (vm.DebugSnapshot, error) {
	return vm.DebugSnapshot{State: vm.DebugRunning}, nil
}
func (*endpointStub) Pause() (vm.DebugSnapshot, error) {
	return vm.DebugSnapshot{State: vm.DebugPaused}, nil
}
func (*endpointStub) SetBreakpoints([]vm.DebugBreakpoint) (vm.DebugSnapshot, error) {
	return vm.DebugSnapshot{State: vm.DebugPaused}, nil
}
func (*endpointStub) Evaluate(int, string) (vm.DebugValue, error) {
	return vm.DebugValue{Kind: "Number", Value: "42"}, nil
}
func (endpoint *endpointStub) Stop() (vm.DebugSnapshot, error) {
	endpoint.stops.Add(1)
	return vm.DebugSnapshot{State: vm.DebugStopped}, nil
}

func TestRegistryDiscoversAndExclusivelyAttachesLiveTargets(t *testing.T) {
	t.Parallel()
	clock := &clockStub{now: time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)}
	registry, err := NewRegistryWithOptions(Options{TargetTTL: 10 * time.Second, LeaseTTL: 3 * time.Second, Now: clock.Now})
	if err != nil {
		t.Fatal(err)
	}
	projectID, databaseID, sessionID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	endpoint := &endpointStub{}
	registration, descriptor, err := registry.Register(Descriptor{
		Kind: UserSession, ProjectID: projectID, DatabaseID: databaseID, SessionID: &sessionID, Name: "alexey · Chrome",
	}, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	views := registry.List(Filter{ProjectID: projectID})
	if len(views) != 1 || views[0].ID != descriptor.ID || views[0].Attached {
		t.Fatalf("targets = %+v", views)
	}
	if filtered := registry.List(Filter{ProjectID: uuid.MustNew()}); len(filtered) != 0 {
		t.Fatalf("foreign project targets = %+v", filtered)
	}
	lease, attached, err := registry.Attach(descriptor.ID, "ML Studio on Mac")
	if err != nil || !attached.Attached || attached.AttachedBy != "ML Studio on Mac" {
		t.Fatalf("Attach() = %+v, %v", attached, err)
	}
	if _, _, err := registry.Attach(descriptor.ID, "other Studio"); !errors.Is(err, ErrTargetBusy) {
		t.Fatalf("second Attach() error = %v", err)
	}
	clock.Advance(4 * time.Second)
	if _, err := registration.Heartbeat(); err != nil {
		t.Fatal(err)
	}
	if endpoint.stops.Load() != 1 {
		t.Fatalf("expired lease stops = %d", endpoint.stops.Load())
	}
	if _, err := lease.Snapshot(); !errors.Is(err, ErrLeaseExpired) {
		t.Fatalf("expired lease Snapshot() error = %v", err)
	}
	replacement, _, err := registry.Attach(descriptor.ID, "other Studio")
	if err != nil {
		t.Fatal(err)
	}
	if err := replacement.Close(); err != nil {
		t.Fatal(err)
	}
	clock.Advance(11 * time.Second)
	if targets := registry.List(Filter{}); len(targets) != 0 {
		t.Fatalf("expired targets = %+v", targets)
	}
	if endpoint.stops.Load() != 2 {
		t.Fatalf("target expiry stops = %d", endpoint.stops.Load())
	}
}

func TestStaleRegistrationCannotRemoveReconnectedTarget(t *testing.T) {
	t.Parallel()
	clock := &clockStub{now: time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)}
	registry, err := NewRegistryWithOptions(Options{TargetTTL: 2 * time.Second, LeaseTTL: time.Second, Now: clock.Now})
	if err != nil {
		t.Fatal(err)
	}
	id, projectID, databaseID, sessionID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	descriptor := Descriptor{ID: id, Kind: Client, ProjectID: projectID, DatabaseID: databaseID, SessionID: &sessionID, Name: "Browser"}
	old, _, err := registry.Register(descriptor, &endpointStub{})
	if err != nil {
		t.Fatal(err)
	}
	clock.Advance(3 * time.Second)
	registry.List(Filter{})
	current, _, err := registry.Register(descriptor, &endpointStub{})
	if err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	if targets := registry.List(Filter{}); len(targets) != 1 {
		t.Fatalf("targets after stale close = %+v", targets)
	}
	if err := current.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryAllowsOnlyOneConcurrentAttachment(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	projectID, databaseID, jobID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	_, descriptor, err := registry.Register(Descriptor{
		Kind: BackgroundJob, ProjectID: projectID, DatabaseID: databaseID, JobID: &jobID, Name: "Import",
	}, &endpointStub{})
	if err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int64
	var wait sync.WaitGroup
	for index := range 100 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			lease, _, attachErr := registry.Attach(descriptor.ID, "Studio "+time.Duration(index).String())
			if attachErr == nil {
				successes.Add(1)
				_ = lease
			} else if !errors.Is(attachErr, ErrTargetBusy) {
				t.Errorf("Attach() error = %v", attachErr)
			}
		}()
	}
	wait.Wait()
	if successes.Load() != 1 {
		t.Fatalf("successful attachments = %d", successes.Load())
	}
}

func TestVMEndpointDebugsUserSessionAndJobContext(t *testing.T) {
	t.Parallel()
	program, diagnostics := compiler.CompileSource("modules/target.bsl", `Function Run(Value)
    Result = Value + 2;
    Return Result;
EndFunction`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := NewVMEndpoint(machine.NewContext())
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	projectID, databaseID, sessionID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	registration, descriptor, err := registry.Register(Descriptor{
		Kind: UserSession, ProjectID: projectID, DatabaseID: databaseID, SessionID: &sessionID, Name: "alexey",
	}, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer registration.Close()
	lease, _, err := registry.Attach(descriptor.ID, "Studio")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	started, err := lease.Start(context.Background(), StartRequest{Routine: "Run", Arguments: []bytecode.Value{bytecode.Number(40)}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	entry := started
	for entry.State == vm.DebugRunning {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Millisecond):
			entry, err = lease.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if entry.State != vm.DebugPaused || entry.Frames[0].Location.Path != "modules/target.bsl" {
		t.Fatalf("entry = %+v", entry)
	}
	value, err := lease.Evaluate(0, "Value + 1")
	if err != nil || value.Value != "41" {
		t.Fatalf("Evaluate() = %+v, %v", value, err)
	}
	if _, err := lease.Command(vm.DebugContinue); err != nil {
		t.Fatal(err)
	}
	for {
		snapshot, snapshotErr := lease.Snapshot()
		if snapshotErr != nil {
			t.Fatal(snapshotErr)
		}
		if snapshot.State == vm.DebugCompleted {
			if snapshot.Result == nil || snapshot.Result.Value != "42" {
				t.Fatalf("result = %+v", snapshot)
			}
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Millisecond):
		}
	}
}

func TestRegistryValidatesTargetKindsAndBounds(t *testing.T) {
	t.Parallel()
	registry, err := NewRegistryWithOptions(Options{TargetTTL: time.Second, LeaseTTL: time.Second, MaxTargets: 1})
	if err != nil {
		t.Fatal(err)
	}
	projectID, databaseID := uuid.MustNew(), uuid.MustNew()
	if _, _, err := registry.Register(Descriptor{Kind: Client, ProjectID: projectID, DatabaseID: databaseID, Name: "missing session"}, &endpointStub{}); err == nil {
		t.Fatal("Register() accepted client without session")
	}
	jobID := uuid.MustNew()
	if _, _, err := registry.Register(Descriptor{Kind: ScheduledJob, ProjectID: projectID, DatabaseID: databaseID, JobID: &jobID, Name: "Schedule"}, &endpointStub{}); err != nil {
		t.Fatal(err)
	}
	otherJob := uuid.MustNew()
	if _, _, err := registry.Register(Descriptor{Kind: BackgroundJob, ProjectID: projectID, DatabaseID: databaseID, JobID: &otherJob, Name: "Other"}, &endpointStub{}); err == nil {
		t.Fatal("Register() ignored target limit")
	}
}
