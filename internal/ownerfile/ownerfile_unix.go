//go:build !windows

package ownerfile

import (
	"fmt"
	"os"
	"syscall"
)

const (
	filePermissions      os.FileMode = 0o600
	directoryPermissions os.FileMode = 0o700
)

// Create creates a new owner-only file at path and opens it for reading and
// writing; an existing file is an error.
func Create(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, filePermissions)
	if err != nil {
		return nil, fmt.Errorf("create owner-only file: %w", err)
	}
	return file, nil
}

// Mkdir creates a new owner-only directory at path; an existing one is an error.
func Mkdir(path string) error {
	if err := os.Mkdir(path, directoryPermissions); err != nil {
		return fmt.Errorf("create owner-only directory: %w", err)
	}
	return nil
}

// Restrict closes an existing file or directory to everyone but its owner.
func Restrict(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("restrict %s to its owner: %w", path, err)
	}
	permissions := filePermissions
	if info.IsDir() {
		permissions = directoryPermissions
	}
	if err := os.Chmod(path, permissions); err != nil {
		return fmt.Errorf("restrict %s to its owner: %w", path, err)
	}
	return nil
}

// IsOwnDirectory reports whether path is a directory itself, not a link to
// one, and belongs to the process user. In a temporary directory shared by
// all users (/tmp) anyone may create an entry with a name we look for.
func IsOwnDirectory(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	status, ok := info.Sys().(*syscall.Stat_t)
	return info.Mode().Type() == os.ModeDir && ok && int(status.Uid) == os.Getuid(), nil
}
