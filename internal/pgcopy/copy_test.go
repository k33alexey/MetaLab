package pgcopy

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/postgresconn"
)

func TestPGPassEscapesEveryField(t *testing.T) {
	t.Parallel()

	descriptor := postgresconn.Descriptor{Host: "db:1", Port: 5432, Database: `app\db`, User: "ml:user"}
	line := pgpassLine(descriptor, `p:a\ss`)
	if line != "db\\:1:5432:app\\\\db:ml\\:user:p\\:a\\\\ss\n" {
		t.Fatalf("pgpass line = %q", line)
	}
	descriptor.Host = "/tmp"
	if line := pgpassLine(descriptor, "secret"); !strings.HasPrefix(line, "localhost:") {
		t.Fatalf("Unix socket pgpass line = %q", line)
	}
}

func TestCopyNeverPlacesPasswordsInArgumentsAndChecksVersions(t *testing.T) {
	t.Parallel()

	const sourcePassword = "source-secret"
	const targetPassword = "target-secret"
	var invocations [][]string
	runner := func(_ context.Context, path string, arguments, environment []string, output io.Writer) error {
		combined := strings.Join(append(append([]string{path}, arguments...), environment...), " ")
		if strings.Contains(combined, sourcePassword) || strings.Contains(combined, targetPassword) {
			t.Fatalf("password exposed in process data: %s", combined)
		}
		invocations = append(invocations, append([]string{path}, arguments...))
		if len(arguments) == 1 && arguments[0] == "--version" {
			_, _ = io.WriteString(output, "pg_dump (PostgreSQL) 16.14\n")
		}
		return nil
	}
	copier := &Copier{dumpPath: "pg_dump", restorePath: "pg_restore", run: runner}
	source := postgresconn.Descriptor{Host: "source", Port: 5432, Database: "main", User: "source", SSLMode: "require", SecretKey: "source.password"}
	target := postgresconn.Descriptor{Host: "target", Port: 5432, Database: "debug", User: "target", SSLMode: "require", SecretKey: "target.password"}
	if err := copier.Copy(context.Background(), source, sourcePassword, 16, target, targetPassword, 16); err != nil {
		t.Fatal(err)
	}
	if len(invocations) != 4 {
		t.Fatalf("invocations = %#v", invocations)
	}
}

func TestCopyRejectsIncompatibleVersionsAndBoundsDiagnostics(t *testing.T) {
	t.Parallel()

	runner := func(_ context.Context, _ string, arguments, _ []string, output io.Writer) error {
		if len(arguments) == 1 && arguments[0] == "--version" {
			_, _ = io.WriteString(output, "pg_dump (PostgreSQL) 16.14\n")
			return nil
		}
		_, _ = io.WriteString(output, strings.Repeat("x", diagnosticLimit*2))
		return errors.New("failed")
	}
	copier := &Copier{dumpPath: "pg_dump", restorePath: "pg_restore", run: runner}
	descriptor := postgresconn.Descriptor{Host: "db", Port: 5432, Database: "app", User: "ml", SSLMode: "require", SecretKey: "app.password"}
	if err := copier.Copy(context.Background(), descriptor, "source", 17, descriptor, "target", 17); err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("version error = %v", err)
	}
	if err := copier.Copy(context.Background(), descriptor, "source", 16, descriptor, "target", 16); err == nil || len(err.Error()) > diagnosticLimit+100 {
		t.Fatalf("bounded error length = %d, error = %v", len(err.Error()), err)
	}
}
