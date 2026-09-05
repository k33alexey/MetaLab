package vm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
)

const (
	resourceCheckInterval  = uint64(256)
	estimatedValueBytes    = uint64(96)
	estimatedIdentityBytes = uint64(8)
	maximumCallDepth       = uint32(4096)
)

var (
	ErrInstructionLimit  = errors.New("BSL instruction limit exceeded")
	ErrMemoryLimit       = errors.New("BSL memory limit exceeded")
	ErrExecutionTimeout  = errors.New("BSL execution time limit exceeded")
	ErrExecutionCanceled = errors.New("BSL execution canceled")
	ErrCallDepthLimit    = errors.New("maximum BSL call depth exceeded")
)

// Limits bounds one top-level BSL call, including all nested routine calls.
type Limits struct {
	MaxInstructions uint64
	MaxMemoryBytes  uint64
	MaxCallDepth    uint32
	MaxDuration     time.Duration
}

// DefaultLimits returns production-safe defaults. Service-level configuration
// may lower these values for individual jobs or sessions.
func DefaultLimits() Limits {
	return Limits{
		MaxInstructions: 10_000_000,
		MaxMemoryBytes:  64 << 20,
		MaxCallDepth:    maxCallDepth,
		MaxDuration:     30 * time.Second,
	}
}

func (limits Limits) validate() error {
	if limits.MaxInstructions == 0 {
		return fmt.Errorf("maximum instruction count must be positive")
	}
	if limits.MaxMemoryBytes == 0 {
		return fmt.Errorf("maximum VM memory must be positive")
	}
	if limits.MaxCallDepth == 0 || limits.MaxCallDepth > maximumCallDepth {
		return fmt.Errorf("maximum call depth must be between 1 and %d", maximumCallDepth)
	}
	if limits.MaxDuration <= 0 {
		return fmt.Errorf("maximum execution duration must be positive")
	}
	return nil
}

func programFitsMemory(program *bytecode.Program, limit uint64) bool {
	used := uint64(0)
	add := func(size uint64) bool {
		if size > limit-used {
			return false
		}
		used += size
		return true
	}
	if !add(uint64(len(program.Modules))*64) || !add(uint64(len(program.Functions))*256) {
		return false
	}
	for _, module := range program.Modules {
		if !add(uint64(len(module.Name)+len(module.Source))) || !add(uint64(len(module.Variables))*32) {
			return false
		}
		for _, variable := range module.Variables {
			if !add(uint64(len(variable.Name))) {
				return false
			}
		}
	}
	for _, function := range program.Functions {
		if !add(uint64(len(function.Name))) || !add(uint64(len(function.Parameters))*128) ||
			!add(uint64(len(function.Constants))*estimatedValueBytes) ||
			!add(uint64(len(function.CallSites))*40) || !add(uint64(len(function.ModuleVars))*8) ||
			!add(uint64(len(function.Exceptions))*24) || !add(uint64(len(function.Code))*64) {
			return false
		}
		for _, parameter := range function.Parameters {
			remaining := limit - used
			size, ok := parameter.Default.DynamicMemory(remaining)
			if !ok || !add(size) {
				return false
			}
		}
		for _, value := range function.Constants {
			remaining := limit - used
			size, ok := value.DynamicMemory(remaining)
			if !ok || !add(size) {
				return false
			}
		}
		for _, call := range function.CallSites {
			if !add(uint64(len(call.References)) * 8) {
				return false
			}
		}
	}
	return true
}

func moduleContextMemory(program *bytecode.Program) (uint64, bool) {
	variables := uint64(0)
	modulesWithVariables := uint64(0)
	for _, module := range program.Modules {
		if len(module.Variables) == 0 {
			continue
		}
		modulesWithVariables++
		if uint64(len(module.Variables)) > (^uint64(0)-variables)/estimatedValueBytes {
			return 0, false
		}
		variables += uint64(len(module.Variables)) * estimatedValueBytes
	}
	if modulesWithVariables == 0 {
		return 0, true
	}
	slices := uint64(len(program.Modules)+1) * 24
	if variables > ^uint64(0)-slices {
		return 0, false
	}
	return variables + slices, true
}

type executionBudget struct {
	context      context.Context
	limits       Limits
	deadline     time.Time
	instructions uint64
	nextCheck    uint64
	retained     uint64
	frames       uint64
	persistent   *uint64
}

func newExecutionBudget(ctx context.Context, limits Limits, arguments []bytecode.Value, persistent *uint64) (executionBudget, error) {
	if ctx == nil {
		return executionBudget{}, fmt.Errorf("execution context is nil")
	}
	persistentBytes := uint64(0)
	if persistent != nil {
		persistentBytes = *persistent
	}
	budget := executionBudget{
		context: ctx, limits: limits,
		nextCheck: resourceCheckInterval, retained: persistentBytes, persistent: persistent,
	}
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return executionBudget{}, fmt.Errorf("%w: %v", ErrExecutionTimeout, err)
		}
		return executionBudget{}, fmt.Errorf("%w: %v", ErrExecutionCanceled, err)
	}
	if persistentBytes > limits.MaxMemoryBytes {
		return executionBudget{}, memoryLimitError(limits.MaxMemoryBytes)
	}
	for _, argument := range arguments {
		remaining := limits.MaxMemoryBytes - budget.retained
		size, ok := argument.DynamicMemory(remaining)
		if !ok || size > remaining {
			return executionBudget{}, memoryLimitError(limits.MaxMemoryBytes)
		}
		budget.retained += size
	}
	return budget, nil
}

func (budget *executionBudget) step() error {
	if budget.instructions >= budget.limits.MaxInstructions {
		return fmt.Errorf("%w: maximum %d instructions", ErrInstructionLimit, budget.limits.MaxInstructions)
	}
	budget.instructions++
	if budget.instructions < budget.nextCheck {
		return nil
	}
	budget.nextCheck = budget.instructions + resourceCheckInterval
	if err := budget.context.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("%w: %v", ErrExecutionTimeout, err)
		}
		return fmt.Errorf("%w: %v", ErrExecutionCanceled, err)
	}
	now := time.Now()
	if budget.deadline.IsZero() {
		budget.deadline = now.Add(budget.limits.MaxDuration)
		return nil
	}
	if !now.Before(budget.deadline) {
		return fmt.Errorf("%w: maximum %s", ErrExecutionTimeout, budget.limits.MaxDuration)
	}
	return nil
}

func (budget *executionBudget) rpcContext() (context.Context, context.CancelFunc) {
	deadline := budget.deadline
	if deadline.IsZero() {
		deadline = time.Now().Add(budget.limits.MaxDuration)
		budget.deadline = deadline
	}
	return context.WithDeadline(budget.context, deadline)
}

func (budget *executionBudget) externalError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %v", ErrExecutionTimeout, err)
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%w: %v", ErrExecutionCanceled, err)
	}
	return err
}

func (budget *executionBudget) checkClock() error {
	if err := budget.context.Err(); err != nil {
		return budget.externalError(err)
	}
	if !budget.deadline.IsZero() && !time.Now().Before(budget.deadline) {
		return fmt.Errorf("%w: maximum %s", ErrExecutionTimeout, budget.limits.MaxDuration)
	}
	return nil
}

func (budget *executionBudget) enter(function *bytecode.Function, depth int) (uint64, error) {
	if uint32(depth) >= budget.limits.MaxCallDepth {
		return 0, fmt.Errorf("%w: maximum %d calls", ErrCallDepthLimit, budget.limits.MaxCallDepth)
	}
	localSlots := max(uint64(function.LocalCount), inlineLocalSize)
	stackSlots := max(uint64(function.MaxStack), inlineStackSize)
	exceptionSlots := max(uint64(len(function.Exceptions)), 8)
	frame := localSlots*(estimatedValueBytes+estimatedIdentityBytes) +
		stackSlots*estimatedValueBytes + exceptionSlots*estimatedIdentityBytes
	if frame > budget.limits.MaxMemoryBytes-budget.frames ||
		budget.retained > budget.limits.MaxMemoryBytes-budget.frames-frame {
		return 0, memoryLimitError(budget.limits.MaxMemoryBytes)
	}
	budget.frames += frame
	return frame, nil
}

func (budget *executionBudget) leave(frame uint64) { budget.frames -= frame }

func (budget *executionBudget) retain(value bytecode.Value) error {
	if value.Kind() != bytecode.StringKind && value.Kind() != bytecode.ArrayKind {
		return nil
	}
	remaining := budget.limits.MaxMemoryBytes - budget.frames - budget.retained
	size, ok := value.DynamicMemory(remaining)
	if !ok || size > remaining {
		return memoryLimitError(budget.limits.MaxMemoryBytes)
	}
	budget.retained += size
	return nil
}

func (budget *executionBudget) fit(value bytecode.Value) error {
	if value.Kind() != bytecode.StringKind && value.Kind() != bytecode.ArrayKind {
		return nil
	}
	remaining := budget.limits.MaxMemoryBytes - budget.frames - budget.retained
	size, ok := value.DynamicMemory(remaining)
	if !ok || size > remaining {
		return memoryLimitError(budget.limits.MaxMemoryBytes)
	}
	return nil
}

func (budget *executionBudget) replacePersistent(old, value bytecode.Value) error {
	if budget.persistent == nil {
		return nil
	}
	oldSize, oldOK := old.DynamicMemory(budget.limits.MaxMemoryBytes)
	newSize, newOK := value.DynamicMemory(budget.limits.MaxMemoryBytes)
	if !oldOK || !newOK || oldSize > *budget.persistent {
		return memoryLimitError(budget.limits.MaxMemoryBytes)
	}
	next := *budget.persistent - oldSize
	if newSize > budget.limits.MaxMemoryBytes-next {
		return memoryLimitError(budget.limits.MaxMemoryBytes)
	}
	*budget.persistent = next + newSize
	return nil
}

func memoryLimitError(limit uint64) error {
	return fmt.Errorf("%w: maximum %d bytes", ErrMemoryLimit, limit)
}

func isResourceFailure(err error) bool {
	return errors.Is(err, ErrInstructionLimit) || errors.Is(err, ErrMemoryLimit) ||
		errors.Is(err, ErrExecutionTimeout) || errors.Is(err, ErrExecutionCanceled) ||
		errors.Is(err, ErrCallDepthLimit)
}
