//go:build windows || linux

package secretstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Defect: a replaced secret keeps its old value, a deleted one is still read,
// or a written secret leaves its temporary file behind.
func TestFileSecretsAreReplacedAndDeleted(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "secrets")
	store := &Store{backend: fileBackend{directory: directory}}
	for _, value := range []string{"first password", "second password"} {
		if err := store.Set("database.one.password", value); err != nil {
			t.Fatal(err)
		}
		read, err := store.Get("database.one.password")
		if err != nil || read != value {
			t.Fatalf("Get() = %q, %v; want %q", read, err, value)
		}
	}
	if err := store.Set("database.two.password", "other"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("directory holds %d entries, want one file per secret", len(entries))
	}
	if err := store.Delete("database.one.password"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("database.one.password"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() after delete error = %v", err)
	}
	if read, err := store.Get("database.two.password"); err != nil || read != "other" {
		t.Fatalf("other secret = %q, %v", read, err)
	}
}

// Defect: an empty file left by a failed write is read as an empty password.
func TestEmptySecretFileIsAnError(t *testing.T) {
	backend := fileBackend{directory: t.TempDir()}
	_, path, err := backend.secretPath(serviceName, "database.one.password")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (&Store{backend: backend}).Get("database.one.password"); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want a read failure", err)
	}
}
