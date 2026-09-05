// Package clientvm manages bytecode machines loaded by a browser client.
package clientvm

import (
	"fmt"
	"sync"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
)

const (
	maxMachines               = 1024
	maxLoadedMachineBytes     = 256 << 20
	estimatedMachineExpansion = 3
)

// Registry owns the immutable machines loaded by one client runtime.
type Registry struct {
	mu       sync.RWMutex
	next     uint32
	contexts map[uint32]*vm.Context
	server   vm.ServerCaller
	cache    *vm.Cache
	weights  map[uint32]uint64
	bytes    uint64
}

// NewRegistry creates an empty client machine registry.
func NewRegistry() *Registry {
	return newRegistry(nil)
}

// NewRegistryWithServer creates a client registry with an RPC transport.
func NewRegistryWithServer(server vm.ServerCaller) *Registry {
	return newRegistry(server)
}

func newRegistry(server vm.ServerCaller) *Registry {
	return &Registry{
		next: 1, contexts: make(map[uint32]*vm.Context), server: server,
		cache: vm.NewDefaultCache(), weights: make(map[uint32]uint64),
	}
}

// Load decodes and validates a bytecode program and returns its local handle.
func (registry *Registry) Load(encoded []byte) (uint32, error) {
	if uint64(len(encoded)) > maxLoadedMachineBytes/estimatedMachineExpansion {
		return 0, fmt.Errorf("client VM bytecode exceeds loaded memory limit")
	}
	weight := uint64(len(encoded)) * estimatedMachineExpansion
	machine, err := registry.cache.LoadClient(encoded)
	if err != nil {
		return 0, fmt.Errorf("load client bytecode: %w", err)
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()
	if len(registry.contexts) >= maxMachines {
		return 0, fmt.Errorf("client VM machine limit reached")
	}
	if registry.bytes > maxLoadedMachineBytes-weight {
		return 0, fmt.Errorf("client VM loaded memory limit reached")
	}
	for {
		handle := registry.next
		registry.next++
		if registry.next == 0 {
			registry.next = 1
		}
		if handle != 0 {
			if _, exists := registry.contexts[handle]; !exists {
				registry.contexts[handle] = machine.NewClientContext(registry.server)
				registry.weights[handle] = weight
				registry.bytes += weight
				return handle, nil
			}
		}
	}
}

// Call executes a routine on a previously loaded machine.
func (registry *Registry) Call(handle uint32, name string, arguments ...bytecode.Value) (bytecode.Value, error) {
	registry.mu.RLock()
	context, ok := registry.contexts[handle]
	registry.mu.RUnlock()
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("client VM machine %d not found", handle)
	}
	return context.Call(name, arguments...)
}

// Release removes a loaded machine.
func (registry *Registry) Release(handle uint32) bool {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, ok := registry.contexts[handle]; !ok {
		return false
	}
	delete(registry.contexts, handle)
	registry.bytes -= registry.weights[handle]
	delete(registry.weights, handle)
	return true
}
