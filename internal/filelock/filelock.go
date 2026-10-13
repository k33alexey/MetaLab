// Package filelock holds an exclusive lock on a file that the operating
// system releases when the holder ends, however it ends: a process killed in
// the middle of its work leaves no lock behind. Two opens of one file
// conflict even within one process.
package filelock

import (
	"errors"
	"fmt"
	"os"
)

// ErrLocked reports that another open of the file holds its lock.
var ErrLocked = errors.New("file is locked")

// Lock is an exclusive lock on an open file.
type Lock struct {
	file *os.File
}

// Hold locks file without waiting. On any error the file is closed.
func Hold(file *os.File) (*Lock, error) {
	if err := lock(file); err != nil {
		file.Close()
		return nil, err
	}
	return &Lock{file: file}, nil
}

// Open opens the existing file at path and locks it without waiting:
// ErrLocked while someone else holds it, an os.ErrNotExist error when there
// is no such file.
func Open(path string) (*Lock, error) {
	file, err := open(path)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	return Hold(file)
}

// Release releases the lock and closes the file.
func (lock *Lock) Release() error {
	return lock.file.Close()
}
