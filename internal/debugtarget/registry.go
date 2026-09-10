// Package debugtarget coordinates live BSL debug targets without coupling ML
// Studio to a concrete process or transport.
package debugtarget

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	defaultTargetTTL   = 45 * time.Second
	defaultLeaseTTL    = 15 * time.Second
	defaultMaxTargets  = 10_000
	maxTargetNameRunes = 256
	maxOwnerRunes      = 256
)

var (
	ErrTargetNotFound   = errors.New("debug target not found or disconnected")
	ErrTargetRegistered = errors.New("debug target is already registered")
	ErrTargetBusy       = errors.New("debug target is attached to another Studio")
	ErrLeaseExpired     = errors.New("debug target lease expired")
	ErrNoDebugSession   = errors.New("debug target session is not started")
)

// Kind describes where BSL is executed.
type Kind string

const (
	Client        Kind = "client"
	UserSession   Kind = "userSession"
	BackgroundJob Kind = "backgroundJob"
	ScheduledJob  Kind = "scheduledJob"
)

// Descriptor identifies one live runtime available to ML Studio.
type Descriptor struct {
	ID          uuid.UUID  `json:"id"`
	Kind        Kind       `json:"kind"`
	ProjectID   uuid.UUID  `json:"projectId"`
	DatabaseID  uuid.UUID  `json:"databaseId"`
	SessionID   *uuid.UUID `json:"sessionId,omitempty"`
	JobID       *uuid.UUID `json:"jobId,omitempty"`
	Name        string     `json:"name"`
	ConnectedAt time.Time  `json:"connectedAt"`
	LastSeenAt  time.Time  `json:"lastSeenAt"`
}

// View is the Studio-facing state of a target and its exclusive attachment.
type View struct {
	Descriptor
	Attached   bool   `json:"attached"`
	AttachedBy string `json:"attachedBy,omitempty"`
}

// Filter limits target discovery to the current project/database.
type Filter struct {
	ProjectID  uuid.UUID
	DatabaseID uuid.UUID
	Kinds      []Kind
}

// StartRequest starts one routine in the selected live runtime.
type StartRequest struct {
	Routine     string
	Arguments   []bytecode.Value
	Breakpoints []vm.DebugBreakpoint
}

// Endpoint is implemented by an in-process VM or a remote debug agent.
// Methods must be concurrency-safe; Stop must return promptly.
type Endpoint interface {
	Start(context.Context, StartRequest) (vm.DebugSnapshot, error)
	Snapshot() (vm.DebugSnapshot, error)
	Command(vm.DebugAction) (vm.DebugSnapshot, error)
	Pause() (vm.DebugSnapshot, error)
	SetBreakpoints([]vm.DebugBreakpoint) (vm.DebugSnapshot, error)
	Evaluate(int, string) (vm.DebugValue, error)
	Stop() (vm.DebugSnapshot, error)
}

// Options controls bounded liveness. Now is intended for deterministic tests.
type Options struct {
	TargetTTL  time.Duration
	LeaseTTL   time.Duration
	MaxTargets int
	Now        func() time.Time
}

// Registry owns live target registrations and exclusive Studio leases.
type Registry struct {
	mu         sync.Mutex
	targets    map[uuid.UUID]*targetEntry
	targetTTL  time.Duration
	leaseTTL   time.Duration
	maxTargets int
	now        func() time.Time
}

type targetEntry struct {
	descriptor   Descriptor
	endpoint     Endpoint
	registration uuid.UUID
	lease        *leaseState
}

type leaseState struct {
	token     uuid.UUID
	owner     string
	expiresAt time.Time
}

// Registration proves ownership of one advertised runtime.
type Registration struct {
	registry *Registry
	targetID uuid.UUID
	token    uuid.UUID
}

// Lease grants one Studio exclusive control of a target.
type Lease struct {
	registry *Registry
	targetID uuid.UUID
	token    uuid.UUID
}

// NewRegistry creates a bounded in-memory target registry.
func NewRegistry() *Registry {
	registry, _ := NewRegistryWithOptions(Options{})
	return registry
}

// NewRegistryWithOptions creates a registry with explicit liveness settings.
func NewRegistryWithOptions(options Options) (*Registry, error) {
	if options.TargetTTL == 0 {
		options.TargetTTL = defaultTargetTTL
	}
	if options.LeaseTTL == 0 {
		options.LeaseTTL = defaultLeaseTTL
	}
	if options.MaxTargets == 0 {
		options.MaxTargets = defaultMaxTargets
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.TargetTTL < time.Second || options.TargetTTL > 24*time.Hour ||
		options.LeaseTTL < time.Second || options.LeaseTTL > 24*time.Hour ||
		options.MaxTargets < 1 || options.MaxTargets > 1_000_000 {
		return nil, fmt.Errorf("invalid debug target registry options")
	}
	return &Registry{
		targets: make(map[uuid.UUID]*targetEntry), targetTTL: options.TargetTTL,
		leaseTTL: options.LeaseTTL, maxTargets: options.MaxTargets, now: options.Now,
	}, nil
}

// Register advertises a live runtime. A zero target ID is generated.
func (registry *Registry) Register(descriptor Descriptor, endpoint Endpoint) (*Registration, Descriptor, error) {
	if registry == nil || endpoint == nil {
		return nil, Descriptor{}, fmt.Errorf("debug target registry and endpoint are required")
	}
	if descriptor.ID.IsZero() {
		id, err := uuid.New()
		if err != nil {
			return nil, Descriptor{}, err
		}
		descriptor.ID = id
	}
	if err := validateDescriptor(descriptor); err != nil {
		return nil, Descriptor{}, err
	}
	token, err := uuid.New()
	if err != nil {
		return nil, Descriptor{}, err
	}
	now := registry.now().UTC()
	descriptor.Name = strings.TrimSpace(descriptor.Name)
	descriptor.ConnectedAt, descriptor.LastSeenAt = now, now
	descriptor = cloneDescriptor(descriptor)

	registry.mu.Lock()
	expired := registry.pruneLocked(now)
	if _, exists := registry.targets[descriptor.ID]; exists {
		registry.mu.Unlock()
		stopEndpoints(expired)
		return nil, Descriptor{}, ErrTargetRegistered
	}
	if len(registry.targets) >= registry.maxTargets {
		registry.mu.Unlock()
		stopEndpoints(expired)
		return nil, Descriptor{}, fmt.Errorf("debug target limit %d reached", registry.maxTargets)
	}
	registry.targets[descriptor.ID] = &targetEntry{descriptor: descriptor, endpoint: endpoint, registration: token}
	registry.mu.Unlock()
	stopEndpoints(expired)
	return &Registration{registry: registry, targetID: descriptor.ID, token: token}, cloneDescriptor(descriptor), nil
}

// List returns deterministic live target views.
func (registry *Registry) List(filter Filter) []View {
	if registry == nil {
		return nil
	}
	now := registry.now().UTC()
	allowed := make(map[Kind]bool, len(filter.Kinds))
	for _, kind := range filter.Kinds {
		allowed[kind] = true
	}
	registry.mu.Lock()
	expired := registry.pruneLocked(now)
	result := make([]View, 0, len(registry.targets))
	for _, entry := range registry.targets {
		if !filter.ProjectID.IsZero() && entry.descriptor.ProjectID != filter.ProjectID ||
			!filter.DatabaseID.IsZero() && entry.descriptor.DatabaseID != filter.DatabaseID ||
			len(allowed) != 0 && !allowed[entry.descriptor.Kind] {
			continue
		}
		result = append(result, viewOf(entry))
	}
	registry.mu.Unlock()
	stopEndpoints(expired)
	sort.Slice(result, func(left, right int) bool {
		if result[left].Kind != result[right].Kind {
			return result[left].Kind < result[right].Kind
		}
		if result[left].Name != result[right].Name {
			return result[left].Name < result[right].Name
		}
		return result[left].ID.String() < result[right].ID.String()
	})
	return result
}

// Attach grants exclusive control to one Studio owner.
func (registry *Registry) Attach(targetID uuid.UUID, owner string) (*Lease, View, error) {
	owner = strings.TrimSpace(owner)
	if registry == nil || targetID.IsZero() || owner == "" || !utf8.ValidString(owner) || utf8.RuneCountInString(owner) > maxOwnerRunes {
		return nil, View{}, fmt.Errorf("valid debug target and Studio owner are required")
	}
	token, err := uuid.New()
	if err != nil {
		return nil, View{}, err
	}
	now := registry.now().UTC()
	registry.mu.Lock()
	expired := registry.pruneLocked(now)
	entry := registry.targets[targetID]
	if entry == nil {
		registry.mu.Unlock()
		stopEndpoints(expired)
		return nil, View{}, ErrTargetNotFound
	}
	if entry.lease != nil {
		registry.mu.Unlock()
		stopEndpoints(expired)
		return nil, View{}, fmt.Errorf("%w: %s", ErrTargetBusy, entry.lease.owner)
	}
	entry.lease = &leaseState{token: token, owner: owner, expiresAt: now.Add(registry.leaseTTL)}
	view := viewOf(entry)
	registry.mu.Unlock()
	stopEndpoints(expired)
	return &Lease{registry: registry, targetID: targetID, token: token}, view, nil
}

// Heartbeat keeps a runtime registration alive.
func (registration *Registration) Heartbeat() (Descriptor, error) {
	if registration == nil || registration.registry == nil {
		return Descriptor{}, ErrTargetNotFound
	}
	now := registration.registry.now().UTC()
	registration.registry.mu.Lock()
	expired := registration.registry.pruneLocked(now)
	entry := registration.registry.targets[registration.targetID]
	if entry == nil || entry.registration != registration.token {
		registration.registry.mu.Unlock()
		stopEndpoints(expired)
		return Descriptor{}, ErrTargetNotFound
	}
	entry.descriptor.LastSeenAt = now
	descriptor := cloneDescriptor(entry.descriptor)
	registration.registry.mu.Unlock()
	stopEndpoints(expired)
	return descriptor, nil
}

// Close removes the same registration without affecting a newer reconnect.
func (registration *Registration) Close() error {
	if registration == nil || registration.registry == nil {
		return nil
	}
	registration.registry.mu.Lock()
	entry := registration.registry.targets[registration.targetID]
	if entry == nil || entry.registration != registration.token {
		registration.registry.mu.Unlock()
		return nil
	}
	delete(registration.registry.targets, registration.targetID)
	registration.registry.mu.Unlock()
	_, _ = entry.endpoint.Stop()
	return nil
}

// Heartbeat keeps the exclusive Studio attachment alive.
func (lease *Lease) Heartbeat() (View, error) {
	entry, expired, err := lease.entry()
	stopEndpoints(expired)
	if err != nil {
		return View{}, err
	}
	return viewOf(entry), nil
}

// Close releases exclusive control. It does not unregister the runtime.
func (lease *Lease) Close() error {
	if lease == nil || lease.registry == nil {
		return nil
	}
	lease.registry.mu.Lock()
	entry := lease.registry.targets[lease.targetID]
	if entry != nil && entry.lease != nil && entry.lease.token == lease.token {
		entry.lease = nil
	}
	lease.registry.mu.Unlock()
	return nil
}

// Start begins one debug execution on the leased runtime.
func (lease *Lease) Start(ctx context.Context, request StartRequest) (vm.DebugSnapshot, error) {
	endpoint, err := lease.endpoint()
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	return endpoint.Start(ctx, request)
}

// Snapshot returns the current remote/local debugger state.
func (lease *Lease) Snapshot() (vm.DebugSnapshot, error) {
	endpoint, err := lease.endpoint()
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	return endpoint.Snapshot()
}

// Command resumes or steps the leased runtime.
func (lease *Lease) Command(action vm.DebugAction) (vm.DebugSnapshot, error) {
	endpoint, err := lease.endpoint()
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	return endpoint.Command(action)
}

// Pause requests a stop in the leased runtime.
func (lease *Lease) Pause() (vm.DebugSnapshot, error) {
	endpoint, err := lease.endpoint()
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	return endpoint.Pause()
}

// SetBreakpoints updates breakpoints in the leased runtime.
func (lease *Lease) SetBreakpoints(values []vm.DebugBreakpoint) (vm.DebugSnapshot, error) {
	endpoint, err := lease.endpoint()
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	return endpoint.SetBreakpoints(values)
}

// Evaluate calculates a safe expression in the leased runtime.
func (lease *Lease) Evaluate(frame int, expression string) (vm.DebugValue, error) {
	endpoint, err := lease.endpoint()
	if err != nil {
		return vm.DebugValue{}, err
	}
	return endpoint.Evaluate(frame, expression)
}

// Stop cancels the leased runtime's current debug execution.
func (lease *Lease) Stop() (vm.DebugSnapshot, error) {
	endpoint, err := lease.endpoint()
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	return endpoint.Stop()
}

func (lease *Lease) endpoint() (Endpoint, error) {
	entry, expired, err := lease.entry()
	stopEndpoints(expired)
	if err != nil {
		return nil, err
	}
	return entry.endpoint, nil
}

func (lease *Lease) entry() (*targetEntry, []Endpoint, error) {
	if lease == nil || lease.registry == nil {
		return nil, nil, ErrLeaseExpired
	}
	now := lease.registry.now().UTC()
	lease.registry.mu.Lock()
	expired := lease.registry.pruneLocked(now)
	entry := lease.registry.targets[lease.targetID]
	if entry == nil || entry.lease == nil || entry.lease.token != lease.token {
		lease.registry.mu.Unlock()
		return nil, expired, ErrLeaseExpired
	}
	entry.lease.expiresAt = now.Add(lease.registry.leaseTTL)
	copy := *entry
	copy.descriptor = cloneDescriptor(entry.descriptor)
	lease.registry.mu.Unlock()
	return &copy, expired, nil
}

func (registry *Registry) pruneLocked(now time.Time) []Endpoint {
	var stopped []Endpoint
	for id, entry := range registry.targets {
		if !entry.descriptor.LastSeenAt.Add(registry.targetTTL).After(now) {
			delete(registry.targets, id)
			stopped = append(stopped, entry.endpoint)
			continue
		}
		if entry.lease != nil && !entry.lease.expiresAt.After(now) {
			entry.lease = nil
			stopped = append(stopped, entry.endpoint)
		}
	}
	return stopped
}

func stopEndpoints(values []Endpoint) {
	for _, endpoint := range values {
		_, _ = endpoint.Stop()
	}
}

func validateDescriptor(descriptor Descriptor) error {
	name := strings.TrimSpace(descriptor.Name)
	if descriptor.ID.IsZero() || descriptor.ProjectID.IsZero() || descriptor.DatabaseID.IsZero() || name == "" ||
		!utf8.ValidString(name) || utf8.RuneCountInString(name) > maxTargetNameRunes {
		return fmt.Errorf("invalid debug target descriptor")
	}
	switch descriptor.Kind {
	case Client, UserSession:
		if descriptor.SessionID == nil || descriptor.SessionID.IsZero() || descriptor.JobID != nil {
			return fmt.Errorf("%s debug target requires a session identifier", descriptor.Kind)
		}
	case BackgroundJob, ScheduledJob:
		if descriptor.JobID == nil || descriptor.JobID.IsZero() || descriptor.SessionID != nil {
			return fmt.Errorf("%s debug target requires a job identifier", descriptor.Kind)
		}
	default:
		return fmt.Errorf("unknown debug target kind %q", descriptor.Kind)
	}
	return nil
}

func viewOf(entry *targetEntry) View {
	view := View{Descriptor: cloneDescriptor(entry.descriptor)}
	if entry.lease != nil {
		view.Attached, view.AttachedBy = true, entry.lease.owner
	}
	return view
}

func cloneDescriptor(source Descriptor) Descriptor {
	result := source
	if source.SessionID != nil {
		value := *source.SessionID
		result.SessionID = &value
	}
	if source.JobID != nil {
		value := *source.JobID
		result.JobID = &value
	}
	return result
}

// VMEndpoint adapts a persistent user/job VM context to a distributed target.
type VMEndpoint struct {
	mu      sync.Mutex
	context *vm.Context
	session *vm.DebugSession
}

// NewVMEndpoint creates a target that preserves the supplied runtime context.
func NewVMEndpoint(runtimeContext *vm.Context) (*VMEndpoint, error) {
	if runtimeContext == nil {
		return nil, fmt.Errorf("debug VM context is required")
	}
	return &VMEndpoint{context: runtimeContext}, nil
}

func (endpoint *VMEndpoint) Start(ctx context.Context, request StartRequest) (vm.DebugSnapshot, error) {
	endpoint.mu.Lock()
	defer endpoint.mu.Unlock()
	if endpoint.session != nil {
		endpoint.session.Stop()
	}
	session, err := endpoint.context.StartDebug(ctx, request.Routine, request.Breakpoints, request.Arguments...)
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	endpoint.session = session
	return session.Snapshot(), nil
}

func (endpoint *VMEndpoint) Snapshot() (vm.DebugSnapshot, error) {
	session, err := endpoint.current()
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	return session.Snapshot(), nil
}

func (endpoint *VMEndpoint) Command(action vm.DebugAction) (vm.DebugSnapshot, error) {
	session, err := endpoint.current()
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	if err := session.Resume(action); err != nil {
		return vm.DebugSnapshot{}, err
	}
	return session.Snapshot(), nil
}

func (endpoint *VMEndpoint) Pause() (vm.DebugSnapshot, error) {
	session, err := endpoint.current()
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	if err := session.Pause(); err != nil {
		return vm.DebugSnapshot{}, err
	}
	return session.Snapshot(), nil
}

func (endpoint *VMEndpoint) SetBreakpoints(values []vm.DebugBreakpoint) (vm.DebugSnapshot, error) {
	session, err := endpoint.current()
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	if err := session.SetBreakpoints(values); err != nil {
		return vm.DebugSnapshot{}, err
	}
	return session.Snapshot(), nil
}

func (endpoint *VMEndpoint) Evaluate(frame int, expression string) (vm.DebugValue, error) {
	session, err := endpoint.current()
	if err != nil {
		return vm.DebugValue{}, err
	}
	return session.Evaluate(frame, expression)
}

func (endpoint *VMEndpoint) Stop() (vm.DebugSnapshot, error) {
	session, err := endpoint.current()
	if err != nil {
		return vm.DebugSnapshot{}, err
	}
	session.Stop()
	return session.Snapshot(), nil
}

func (endpoint *VMEndpoint) current() (*vm.DebugSession, error) {
	endpoint.mu.Lock()
	defer endpoint.mu.Unlock()
	if endpoint.session == nil {
		return nil, ErrNoDebugSession
	}
	return endpoint.session, nil
}
