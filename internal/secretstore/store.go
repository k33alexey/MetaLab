// Package secretstore keeps MetaLab secrets in the native OS credential store.
package secretstore

import (
	"errors"
	"fmt"
	"strings"
)

const serviceName = "MetaLab"

// ErrNotFound means that a named MetaLab secret does not exist.
var ErrNotFound = errors.New("MetaLab secret not found")

type backend interface {
	Set(service, account, secret string) error
	Get(service, account string) (string, error)
	Delete(service, account string) error
}

// Store provides namespaced access to the protected store of the operating system.
type Store struct {
	backend backend
}

// New creates the native operating-system secret store.
func New() *Store { return &Store{backend: nativeBackend{}} }

// Set creates or replaces a secret. Secret values are never included in errors.
func (store *Store) Set(key, secret string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	if secret == "" {
		return fmt.Errorf("secret %q is empty", key)
	}
	if err := store.backend.Set(serviceName, key, secret); err != nil {
		return backendError{operation: "store", key: key, cause: err}
	}
	return nil
}

// Get reads a secret without exposing it in error text.
func (store *Store) Get(key string) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	secret, err := store.backend.Get(serviceName, key)
	if errors.Is(err, ErrNotFound) {
		return "", fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	if err != nil {
		return "", backendError{operation: "read", key: key, cause: err}
	}
	return secret, nil
}

// Delete removes a secret. Deleting an absent secret succeeds.
func (store *Store) Delete(key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	err := store.backend.Delete(serviceName, key)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return backendError{operation: "delete", key: key, cause: err}
	}
	return nil
}

type backendError struct {
	operation string
	key       string
	cause     error
}

func (err backendError) Error() string {
	return fmt.Sprintf("%s secret %q in OS credential store failed", err.operation, err.key)
}

func (err backendError) Unwrap() error { return err.cause }

func validateKey(key string) error {
	if key == "" || key != strings.TrimSpace(key) || len(key) > 200 {
		return fmt.Errorf("invalid secret key %q", key)
	}
	for _, symbol := range key {
		if symbol != '.' && symbol != '_' && symbol != '-' && (symbol < 'a' || symbol > 'z') && (symbol < '0' || symbol > '9') {
			return fmt.Errorf("invalid secret key %q", key)
		}
	}
	return nil
}
