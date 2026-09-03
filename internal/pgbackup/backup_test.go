package pgbackup

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/postgresconn"
)

func TestCreateAndRestoreVerifiedArchiveWithoutPasswordExposure(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	const password = "never-in-process-arguments"
	invocations := 0
	runner := func(_ context.Context, path string, arguments, environment []string, _ io.Writer) error {
		invocations++
		combined := strings.Join(append(append([]string{path}, arguments...), environment...), " ")
		if strings.Contains(combined, password) {
			t.Fatalf("password exposed in process data: %s", combined)
		}
		if path == "pg_dump" {
			for index, argument := range arguments {
				if argument == "--file" {
					return os.WriteFile(arguments[index+1], []byte("archive-data"), 0o600)
				}
			}
		}
		return nil
	}
	tool := &Tool{dumpPath: "pg_dump", restorePath: "pg_restore", run: runner}
	connection := postgresconn.Descriptor{
		Host: "localhost", Port: 5432, Database: "sales", User: "ml", SSLMode: "disable", SecretKey: "sales.password",
	}
	archive, err := tool.Create(context.Background(), connection, password, directory, "sales.mlbackup")
	if err != nil {
		t.Fatal(err)
	}
	if archive.SizeBytes != int64(len("archive-data")) || len(archive.SHA256) != 64 {
		t.Fatalf("archive = %+v", archive)
	}
	path := filepath.Join(directory, archive.FileName)
	if err := tool.Restore(context.Background(), connection, password, path, archive.SHA256); err != nil {
		t.Fatal(err)
	}
	if invocations != 2 {
		t.Fatalf("invocations = %d, want 2", invocations)
	}
	if err := tool.Restore(context.Background(), connection, password, path, strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("checksum error = %v", err)
	}
}

func TestCreateRemovesPartialArchiveOnFailure(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	tool := &Tool{dumpPath: "pg_dump", restorePath: "pg_restore", run: func(_ context.Context, _ string, arguments, _ []string, _ io.Writer) error {
		for index, argument := range arguments {
			if argument == "--file" {
				_ = os.WriteFile(arguments[index+1], []byte("partial"), 0o600)
			}
		}
		return errors.New("forced failure")
	}}
	connection := postgresconn.Descriptor{
		Host: "localhost", Port: 5432, Database: "sales", User: "ml", SSLMode: "disable", SecretKey: "sales.password",
	}
	if _, err := tool.Create(context.Background(), connection, "secret", directory, "sales.mlbackup"); err == nil {
		t.Fatal("failed dump succeeded")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("partial files remain: %+v", entries)
	}
}
