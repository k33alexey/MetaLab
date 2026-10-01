package metadata

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"testing"
	"time"
)

// TestMain checks that the package leaves no goroutine behind. A goroutine a
// test started and never stopped - a pool's background worker whose pool was
// never closed, a reader on a channel nobody closes - passes every assertion
// and outlives the test, and in the service it outlives the request: memory
// and connections that grow with every call and are found only in production.
//
// Connections need no check of their own here: closing a pool waits until
// every connection it lent is back, so a connection that is never returned
// hangs the test that owns the pool instead of passing it.
func TestMain(m *testing.M) {
	before := runtime.NumGoroutine()
	code := m.Run()
	if code == 0 {
		if leaked := goroutinesLeftBehind(before); leaked != "" {
			fmt.Fprintf(os.Stderr, "the tests of the package left goroutines behind (%d before):\n%s\n", before, leaked)
			code = 1
		}
	}
	os.Exit(code)
}

// goroutinesLeftBehind waits for the goroutines that finish on their own -
// a closed pool's workers take a moment to notice - and describes the ones that
// are still there.
func goroutinesLeftBehind(before int) string {
	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > before {
		if time.Now().After(deadline) {
			var dump strings.Builder
			_ = pprof.Lookup("goroutine").WriteTo(&dump, 1)
			return fmt.Sprintf("%d now\n%s", runtime.NumGoroutine(), dump.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	return ""
}
