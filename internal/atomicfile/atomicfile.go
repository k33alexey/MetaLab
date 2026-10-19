// Package atomicfile writes a file so that it is either the old one or the
// new one, never half of each: the content goes to a new file beside it,
// synced and closed, which is then put in its place in one step. It is the
// one place that knows how a file is replaced on each system - Studio's
// sources, the configuration of a project, the settings of ML, a secret.
package atomicfile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Prepare writes a new file in directory, named by pattern as os.CreateTemp
// names one, with the permissions mode, the content write gives it, synced
// and closed, and answers its path - the file Replace then puts in place of
// another. A file that fails half-way is removed.
func Prepare(directory, pattern string, mode os.FileMode, write func(io.Writer) error) (string, error) {
	temporary, err := os.CreateTemp(directory, pattern)
	if err != nil {
		return "", fmt.Errorf("create temporary file: %w", err)
	}
	path := temporary.Name()
	failed := true
	defer func() {
		_ = temporary.Close()
		if failed {
			_ = os.Remove(path)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return "", fmt.Errorf("set temporary file permissions: %w", err)
	}
	if err := write(temporary); err != nil {
		return "", err
	}
	if err := temporary.Sync(); err != nil {
		return "", fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("close temporary file: %w", err)
	}
	failed = false
	return path, nil
}

// Write puts at path a file with the permissions mode and the content write
// gives it, through a new file beside it (Prepare, Replace): until it
// answers, the file at path is what it was.
func Write(path, pattern string, mode os.FileMode, write func(io.Writer) error) error {
	temporary, err := Prepare(filepath.Dir(path), pattern, mode, write)
	if err != nil {
		return err
	}
	defer os.Remove(temporary)
	return Replace(temporary, path)
}

// Bytes is the content of a file that is already at hand, for Prepare and
// Write.
func Bytes(content []byte) func(io.Writer) error {
	return func(writer io.Writer) error {
		if _, err := writer.Write(content); err != nil {
			return fmt.Errorf("write temporary file: %w", err)
		}
		return nil
	}
}
