package secretstore

import (
	"errors"
	"strings"
	"testing"
)

func TestStoreLifecycleAndSecretRedaction(t *testing.T) {
	t.Parallel()

	backend := &memoryBackend{values: make(map[string]string)}
	store := &Store{backend: backend}
	const secret = "do-not-print-this-password"
	if err := store.Set("postgres.system.password", secret); err != nil {
		t.Fatal(err)
	}
	read, err := store.Get("postgres.system.password")
	if err != nil || read != secret {
		t.Fatalf("secret = %q, error = %v", read, err)
	}
	if err := store.Delete("postgres.system.password"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("postgres.system.password"); !errors.Is(err, ErrNotFound) || strings.Contains(err.Error(), secret) {
		t.Fatalf("Get() error = %v", err)
	}
}

func TestStoreRejectsInvalidKeysAndEmptySecrets(t *testing.T) {
	t.Parallel()

	store := &Store{backend: &memoryBackend{values: make(map[string]string)}}
	for _, key := range []string{"", " secret", "Postgres.password"} {
		if err := store.Set(key, "secret"); err == nil {
			t.Fatalf("Set(%q) succeeded", key)
		}
	}
	if err := store.Set("postgres.password", ""); err == nil {
		t.Fatal("empty secret accepted")
	}
}

func TestStoreDoesNotExposeBackendErrorSecrets(t *testing.T) {
	t.Parallel()

	const secret = "backend accidentally returned a secret"
	store := &Store{backend: &memoryBackend{failure: errors.New(secret)}}
	err := store.Set("postgres.password", "value")
	if err == nil || strings.Contains(err.Error(), secret) || !errors.Is(err, store.backend.(*memoryBackend).failure) {
		t.Fatalf("Set() error = %v", err)
	}
}

type memoryBackend struct {
	values  map[string]string
	failure error
}

func (backend *memoryBackend) Set(service, account, secret string) error {
	if backend.failure != nil {
		return backend.failure
	}
	backend.values[service+":"+account] = secret
	return nil
}
func (backend *memoryBackend) Get(service, account string) (string, error) {
	if backend.failure != nil {
		return "", backend.failure
	}
	value, exists := backend.values[service+":"+account]
	if !exists {
		return "", ErrNotFound
	}
	return value, nil
}
func (backend *memoryBackend) Delete(service, account string) error {
	key := service + ":" + account
	if _, exists := backend.values[key]; !exists {
		return ErrNotFound
	}
	delete(backend.values, key)
	return nil
}
