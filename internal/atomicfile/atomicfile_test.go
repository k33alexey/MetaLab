package atomicfile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// A file written through Write is the new one whole, with the permissions
// asked, and nothing of the writing stays beside it. Defects caught: the
// content written over the old file in place, so that a crash leaves half of
// each; the permissions of the temporary name kept (CreateTemp gives 0600)
// where the file is to be read by others; a temporary file left behind.
func TestWritePutsTheNewFileWhole(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "settings.yaml")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, ".settings-*", 0o644, Bytes([]byte("new"))); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "new" {
		t.Fatalf("the file holds %q (%v), want the new content", content, err)
	}
	if info, err := os.Stat(path); err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0o644 {
		t.Fatalf("the file stands with %v (%v), want 0644", info.Mode().Perm(), err)
	}
	if entries, _ := os.ReadDir(directory); len(entries) != 1 {
		t.Fatalf("the directory holds %d entries, want the file alone", len(entries))
	}
}

// A write that fails half-way leaves the old file as it was and no new file
// beside it. Defects caught: the old file truncated or replaced by what was
// written before the failure; the half-written temporary file left in the
// directory, where a project would read it as one of its files.
func TestAFailedWriteLeavesTheOldFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "configuration.yaml")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("encode failed")
	err := Write(path, ".configuration-*", 0o644, func(writer io.Writer) error {
		_, _ = writer.Write([]byte("ne"))
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("the write answered %v, want the failure of the content", err)
	}
	content, _ := os.ReadFile(path)
	if string(content) != "old" {
		t.Fatalf("the file holds %q after a failed write, want the old content", content)
	}
	if entries, _ := os.ReadDir(directory); len(entries) != 1 {
		t.Fatalf("the directory holds %d entries after a failed write, want the old file alone", len(entries))
	}
}

// Replace puts a prepared file in place of one that is there, and refuses
// when the prepared file is gone, leaving the other as it was. Defects caught:
// a replacement that refuses an existing destination (the plain rename of
// Windows); one that reports success having put nothing in place.
func TestReplaceTakesThePlaceOfTheFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "rights.yaml")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	prepared, err := Prepare(directory, ".rights-*", 0o644, Bytes([]byte("new")))
	if err != nil {
		t.Fatal(err)
	}
	if err := Replace(prepared, path); err != nil {
		t.Fatal(err)
	}
	if content, _ := os.ReadFile(path); string(content) != "new" {
		t.Fatalf("the file holds %q, want the prepared content", content)
	}
	if err := Replace(prepared, path); err == nil {
		t.Fatal("a replacement by a file that is gone succeeded")
	}
	if content, _ := os.ReadFile(path); string(content) != "new" {
		t.Fatalf("a failed replacement changed the file to %q", content)
	}
}
