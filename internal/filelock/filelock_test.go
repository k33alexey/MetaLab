package filelock

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestALockHeldElsewhereIsRefusedAndItsFileClosed catches a lock taken twice
// - two cleanups each sure the folder is theirs - and a refused Hold leaving
// its file open: every refused try would keep a descriptor until the
// process ends.
func TestALockHeldElsewhereIsRefusedAndItsFileClosed(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "lock")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	held, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Hold(file); !errors.Is(err, ErrLocked) {
		t.Fatalf("a lock held elsewhere was taken again: %v", err)
	}
	if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("the refused lock left its file open: %v", err)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	again, err := Open(path)
	if err != nil {
		t.Fatalf("a released lock cannot be taken: %v", err)
	}
	_ = again.Release()
}
