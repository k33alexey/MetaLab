package metadata

import (
	"context"
	"fmt"
	"sync"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
)

// SessionModuleName is the canonical name of the project's session module
// inside a compiled program. The configuration root owns exactly one session
// module, so unlike object and common modules it is named by its role rather
// than by a UUID - see moduleNameDescriptors, which must stay in sync.
const SessionModuleName = "МодульСеанса"

const (
	setSessionParametersRU = "УстановкаПараметровСеанса"
	setSessionParametersEN = "SetSessionParameters"
)

// SessionBSLEvents invokes the single predefined procedure of the session
// module. A project without a session module, or with one that does not
// declare the procedure, simply has no handler: session parameters then keep
// whatever default they declare, exactly as before.
type SessionBSLEvents struct {
	mu      sync.Mutex
	context *vm.Context
	module  string
}

func NewSessionBSLEvents(machineContext *vm.Context) (*SessionBSLEvents, error) {
	if machineContext == nil {
		return nil, fmt.Errorf("session module events require a BSL context")
	}
	return &SessionBSLEvents{context: machineContext, module: SessionModuleName}, nil
}

// SetSessionParameters runs the handler. names lists the parameters that must
// be initialized; nil means the session-start form of the call, where the
// solution initializes everything it needs at once.
//
// The handler is free to set MORE parameters than asked: initializing a group
// that shares one read of the data is what real solutions do, and it is why
// the caller re-checks every requested name after one call instead of calling
// the handler once per name.
func (handler *SessionBSLEvents) SetSessionParameters(ctx context.Context, names []string) (bool, error) {
	if handler == nil {
		return false, nil
	}
	routine := ""
	switch {
	case handler.context.HasRoutine(handler.module, setSessionParametersRU):
		routine = setSessionParametersRU
	case handler.context.HasRoutine(handler.module, setSessionParametersEN):
		routine = setSessionParametersEN
	default:
		return false, nil
	}
	argument := bytecode.Undefined()
	if names != nil {
		values := make([]bytecode.Value, 0, len(names))
		for _, name := range names {
			values = append(values, bytecode.String(name))
		}
		argument = bytecode.Array(values...)
	}
	handler.mu.Lock()
	defer handler.mu.Unlock()
	if _, _, err := handler.context.CallContextMutable(ctx, handler.module+"."+routine, argument); err != nil {
		return false, fmt.Errorf("%s: %w", setSessionParametersRU, err)
	}
	return true, nil
}

// SetSessionModuleHandler installs the compiled session module of this project.
func (runtime *Runtime) SetSessionModuleHandler(handler *SessionBSLEvents) {
	runtime.sessionParametersMu.Lock()
	runtime.sessionModule = handler
	runtime.sessionParametersMu.Unlock()
}

// InitializeSessionParameters runs the session-start form of the handler, the
// one that receives no names and initializes whatever the solution needs up
// front. Nothing calls it on the request path today: an ML runtime is built
// per request, so running it there would report "the session is starting"
// dozens of times inside one real session. It belongs to the session lifecycle
// that arrives with the application module.
func (runtime *Runtime) InitializeSessionParameters(ctx context.Context) error {
	_, err := runtime.runSessionModule(ctx, nil)
	return err
}

// runSessionModule calls the handler unless it is already running. The guard is
// not an optimization: a real handler reads session parameters to decide what
// still needs initializing, and without it every such read would call the
// handler again, forever.
func (runtime *Runtime) runSessionModule(ctx context.Context, names []string) (bool, error) {
	runtime.sessionParametersMu.Lock()
	handler, running := runtime.sessionModule, runtime.sessionModuleRunning
	if handler == nil || running {
		runtime.sessionParametersMu.Unlock()
		return false, nil
	}
	runtime.sessionModuleRunning = true
	runtime.sessionParametersMu.Unlock()
	defer func() {
		runtime.sessionParametersMu.Lock()
		runtime.sessionModuleRunning = false
		runtime.sessionParametersMu.Unlock()
	}()
	return handler.SetSessionParameters(ctx, names)
}
