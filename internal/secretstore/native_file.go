//go:build windows || linux

package secretstore

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/k33alexey/MetaLab/internal/atomicfile"
	"github.com/k33alexey/MetaLab/internal/ownerfile"
	"github.com/k33alexey/MetaLab/internal/servicestate"
)

// fileBackend keeps each secret in its own file in a machine-wide directory:
// the account that writes a secret (an administrator) is not the one that
// reads it (the service), so a store scoped to a user does not fit. What a
// file is sealed with is the operating system's part (native_windows.go,
// native_linux.go), how it replaces the previous one atomicfile's; where the
// directory lies, how it is laid and who a file is handed to —
// servicestate's.
type fileBackend struct {
	// directory replaces the machine-wide one in tests.
	directory string
}

func nativeBackend() backend { return fileBackend{} }

func (store fileBackend) Set(service, account, secret string) error {
	directory, path, err := store.secretPath(service, account)
	if err != nil {
		return err
	}
	if err := servicestate.PrepareOwnDirectory(directory); err != nil {
		return err
	}
	sealed, err := sealSecret([]byte(secret))
	if err != nil {
		return err
	}
	temporary, err := ownerfile.CreateTemp(directory, ".secret-*")
	if err != nil {
		return fmt.Errorf("create temporary secret: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.Write(sealed); err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return fmt.Errorf("write secret: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close secret: %w", closeErr)
	}
	// The file is handed over before it gets its name, so a secret under its
	// name is never one the service cannot read.
	if err := servicestate.HandOver(temporaryPath); err != nil {
		return err
	}
	if err := atomicfile.Replace(temporaryPath, path); err != nil {
		return fmt.Errorf("replace secret: %w", err)
	}
	return nil
}

func (store fileBackend) Get(service, account string) (string, error) {
	_, path, err := store.secretPath(service, account)
	if err != nil {
		return "", err
	}
	sealed, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read secret: %w", err)
	}
	if len(sealed) == 0 {
		return "", fmt.Errorf("secret file is empty")
	}
	plain, err := openSecret(sealed)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (store fileBackend) Delete(service, account string) error {
	_, path, err := store.secretPath(service, account)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	return err
}

func (store fileBackend) secretPath(service, account string) (string, string, error) {
	directory := store.directory
	if directory == "" {
		var err error
		if directory, err = servicestate.Secrets(); err != nil {
			return "", "", err
		}
	}
	digest := sha256.Sum256([]byte(service + ":" + account))
	return directory, filepath.Join(directory, hex.EncodeToString(digest[:])+secretExtension), nil
}
