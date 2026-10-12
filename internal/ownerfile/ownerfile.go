// Package ownerfile creates files and directories that only their owner can
// open: the process user, and on Windows also the system and administrators.
//
// It is the one place that knows how a secret is laid on disk — a password
// handed to PostgreSQL tools, a backup archive, a protected secret. A file is
// created already closed to others, never opened first and restricted after:
// on Windows a handle opened in between keeps its access once the rights
// change, and the content written by then is readable through it.
package ownerfile

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// temporaryAttempts bounds the search for a free random name, as os.CreateTemp does.
const temporaryAttempts = 10000

// CreateTemp creates a new owner-only file in directory (the system temporary
// directory when empty), named by pattern with its last "*" replaced by a
// random string, and opens it for reading and writing.
func CreateTemp(directory, pattern string) (*os.File, error) {
	var file *os.File
	err := temporaryName(directory, pattern, func(path string) (err error) {
		file, err = Create(path)
		return err
	})
	return file, err
}

// MkdirTemp creates a new owner-only directory in directory (the system
// temporary directory when empty) and returns its path. What is created inside
// it is owner-only as well.
func MkdirTemp(directory, pattern string) (string, error) {
	var created string
	err := temporaryName(directory, pattern, func(path string) error {
		created = path
		return Mkdir(path)
	})
	return created, err
}

func temporaryName(directory, pattern string, create func(string) error) error {
	if strings.ContainsRune(pattern, os.PathSeparator) || strings.ContainsRune(pattern, '/') {
		return fmt.Errorf("pattern %q contains a path separator", pattern)
	}
	if directory == "" {
		directory = os.TempDir()
	}
	prefix, suffix := pattern, ""
	if star := strings.LastIndex(pattern, "*"); star >= 0 {
		prefix, suffix = pattern[:star], pattern[star+1:]
	}
	for range temporaryAttempts {
		random := make([]byte, 8)
		if _, err := rand.Read(random); err != nil {
			return fmt.Errorf("choose temporary name: %w", err)
		}
		err := create(filepath.Join(directory, prefix+hex.EncodeToString(random)+suffix))
		if !errors.Is(err, os.ErrExist) {
			return err
		}
	}
	return fmt.Errorf("no free temporary name for %q in %s", pattern, directory)
}
