//go:build !windows

package filelock

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func open(path string) (*os.File, error) {
	return os.Open(path)
}

// lock takes a flock lock: it belongs to the open file, not to the process,
// so a second open in the same process conflicts with the first.
func lock(file *os.File) error {
	for {
		err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, unix.EINTR):
			continue
		case errors.Is(err, unix.EWOULDBLOCK):
			return ErrLocked
		default:
			return fmt.Errorf("lock %s: %w", file.Name(), err)
		}
	}
}
