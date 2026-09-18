package vm

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
)

// maxPanicStack bounds what is kept from a Go stack trace. Enough to see where
// the crash came from, not so much that one incident fills the log.
const maxPanicStack = 8 << 10

// Where the crash happened inside BSL is answered by Stack, not by a BSL frame
// chain: annotating every VM frame with its routine was measured at 3-4x the
// cost of the whole interpreter (a deferred recover in executeAdvanced defeats
// open-coded defers in the hottest function there is), and an incident report
// that costs every ordinary call is the wrong trade.
//
// PanicError reports that executing application code crashed the Go runtime
// instead of raising a BSL exception. It is not an ordinary BSL error: nothing
// in the language can produce it, and it always means a defect in the platform
// or in a host object it called.
//
// The platform turns it into an error rather than letting it unwind because of
// where BSL runs: inside an HTTP handler net/http would catch it and drop one
// connection, but in a background goroutine - a scheduled job, the debugger -
// an unrecovered panic takes down the whole ML Service process, with every
// session of every database on it. A defect in one application's code must not
// be able to do that.
type PanicError struct {
	// Routine is the BSL routine that was executing, as the caller named it.
	Routine string
	// Value is what was passed to panic(), preserved for the incident report.
	Value any
	// Stack is a bounded Go stack trace taken at the moment of the panic.
	Stack string
}

func (failure *PanicError) Error() string {
	return fmt.Sprintf("BSL execution of %q crashed: %v", failure.Routine, failure.Value)
}

// recoverExecution converts a panic raised anywhere under one BSL call into a
// PanicError. It never swallows it silently: the incident is logged where it
// happened, with the stack, and the error is returned to the caller as well -
// an incident nobody can see afterwards is the same as one that was ignored.
func recoverExecution(routine string, result *bytecode.Value, resultErr *error) {
	recovered := recover()
	if recovered == nil {
		return
	}
	stack := string(debug.Stack())
	if len(stack) > maxPanicStack {
		stack = stack[:maxPanicStack] + "\n… (стек усечён)"
	}
	failure := &PanicError{Routine: routine, Value: recovered, Stack: stack}
	slog.Error("BSL execution panicked",
		"routine", routine, "panic", fmt.Sprint(recovered), "stack", strings.TrimSpace(stack))
	if result != nil {
		*result = bytecode.Undefined()
	}
	if resultErr != nil {
		*resultErr = failure
	}
}
