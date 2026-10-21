package pgtool

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/postgresconn"
)

type invocation struct {
	path        string
	arguments   []string
	environment []string
}

func recordingTools(versions map[string]string, invocations *[]invocation) Tools {
	return New("pg_dump", "pg_restore", func(_ context.Context, path string, arguments, environment []string, output io.Writer) error {
		if len(arguments) == 1 && arguments[0] == "--version" {
			_, _ = io.WriteString(output, versions[path])
			return nil
		}
		*invocations = append(*invocations, invocation{path: path, arguments: arguments, environment: environment})
		return nil
	})
}

func sslModes(environment []string) []string {
	var modes []string
	for _, variable := range environment {
		if mode, found := strings.CutPrefix(variable, "PGSSLMODE="); found {
			modes = append(modes, mode)
		}
	}
	return modes
}

// Defect: the utilities connect with libpq's default ssl mode ("prefer",
// which falls back to an unencrypted connection) instead of the connection's
// own, or one utility gets the other connection's mode.
func TestEachRunConnectsWithItsOwnSSLMode(t *testing.T) {
	t.Setenv("PGSSLMODE", "allow")
	var invocations []invocation
	tools := recordingTools(nil, &invocations)
	source := postgresconn.Descriptor{Host: "source", Port: 5433, Database: "main", User: "reader", SSLMode: "verify-full", SecretKey: "s"}
	target := postgresconn.Descriptor{Host: "target", Port: 5434, Database: "debug", User: "writer", SSLMode: "disable", SecretKey: "t"}
	environment := postgresconn.ToolEnvironment("/protected/pgpass")
	if err := tools.Dump(context.Background(), source, "/work/database.dump", environment); err != nil {
		t.Fatal(err)
	}
	if err := tools.Restore(context.Background(), target, "/work/database.dump", IntoEmpty, environment); err != nil {
		t.Fatal(err)
	}
	if len(invocations) != 2 {
		t.Fatalf("invocations = %#v", invocations)
	}
	for index, want := range []struct {
		path, mode string
		connection []string
	}{
		{"pg_dump", "verify-full", []string{"--host", "source", "--port", "5433", "--username", "reader", "--dbname", "main"}},
		{"pg_restore", "disable", []string{"--host", "target", "--port", "5434", "--username", "writer", "--dbname", "debug"}},
	} {
		got := invocations[index]
		if got.path != want.path {
			t.Fatalf("run %d path = %s, want %s", index, got.path, want.path)
		}
		if modes := sslModes(got.environment); !slices.Equal(modes, []string{want.mode}) {
			t.Fatalf("%s PGSSLMODE = %v, want only %s", got.path, modes, want.mode)
		}
		if !strings.Contains(strings.Join(got.arguments, " "), strings.Join(want.connection, " ")) {
			t.Fatalf("%s arguments = %v, want connection %v", got.path, got.arguments, want.connection)
		}
	}
	if modes := sslModes(environment); len(modes) != 0 {
		t.Fatalf("shared environment carries PGSSLMODE %v", modes)
	}
}

// Defect: a restore into a fresh copy drops objects, or a restore of a backup
// over its database fails on the objects already there.
func TestRestoreModeDecidesWhetherExistingObjectsAreDropped(t *testing.T) {
	t.Parallel()
	var invocations []invocation
	tools := recordingTools(nil, &invocations)
	connection := postgresconn.Descriptor{Host: "db", Port: 5432, Database: "app", User: "ml", SSLMode: "require", SecretKey: "k"}
	for _, mode := range []RestoreMode{IntoEmpty, ReplaceExisting} {
		if err := tools.Restore(context.Background(), connection, "/work/archive", mode, nil); err != nil {
			t.Fatal(err)
		}
	}
	for index, wantClean := range []bool{false, true} {
		arguments := invocations[index].arguments
		if slices.Contains(arguments, "--clean") != wantClean || slices.Contains(arguments, "--if-exists") != wantClean {
			t.Fatalf("restore %d arguments = %v, clean wanted %v", index, arguments, wantClean)
		}
		if !slices.Contains(arguments, "--single-transaction") || !slices.Contains(arguments, "--exit-on-error") || arguments[len(arguments)-1] != "/work/archive" {
			t.Fatalf("restore %d arguments = %v", index, arguments)
		}
	}
}

// Defect: pg_dump of one major version writes an archive that pg_restore of
// another then reads, or an unreadable version passes as zero.
func TestMajorRefusesMismatchedOrUnreadableVersions(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		versions map[string]string
		want     int
		failure  string
	}{
		{"same", map[string]string{"pg_dump": "pg_dump (PostgreSQL) 17.2\n", "pg_restore": "pg_restore (PostgreSQL) 17.6\n"}, 17, ""},
		{"mismatch", map[string]string{"pg_dump": "pg_dump (PostgreSQL) 17.2\n", "pg_restore": "pg_restore (PostgreSQL) 16.14\n"}, 0, "differs"},
		{"unreadable", map[string]string{"pg_dump": "pg_dump 17\n", "pg_restore": "pg_restore (PostgreSQL) 17.6\n"}, 0, "cannot parse pg_dump"},
	} {
		major, err := recordingTools(test.versions, new([]invocation)).Major(context.Background())
		if test.failure == "" && (err != nil || major != test.want) {
			t.Fatalf("%s: major = %d, %v; want %d", test.name, major, err, test.want)
		}
		if test.failure != "" && (err == nil || !strings.Contains(err.Error(), test.failure)) {
			t.Fatalf("%s: error = %v, want %q", test.name, err, test.failure)
		}
	}
}

// TestAFailedRunSaysWhatTheUtilitySaid catches the refusal of pg_dump or
// pg_restore losing the utility's own words - the only place that says why -
// and a utility talking past the limit failing for it: the words beyond the
// limit are dropped, the write itself is taken whole, as io.Writer must
// answer, so the utility is not stopped by a short write.
func TestAFailedRunSaysWhatTheUtilitySaid(t *testing.T) {
	t.Parallel()
	failure := errors.New("exit status 1")
	var written int
	var writeErr error
	tools := New("pg_dump", "pg_restore", func(_ context.Context, _ string, _, _ []string, output io.Writer) error {
		written, writeErr = output.Write([]byte("pg_dump: error: permission denied\n" + strings.Repeat("x", DiagnosticLimit)))
		return failure
	})
	err := tools.Dump(context.Background(), postgresconn.Descriptor{Host: "localhost", Port: 5432, Database: "base", User: "user", SSLMode: "disable"}, "archive", nil)
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("the refusal does not say what the utility said: %v", err)
	}
	if want := len("pg_dump: error: permission denied\n") + DiagnosticLimit; written != want || writeErr != nil {
		t.Fatalf("a long write was answered %d, %v, want %d, nil", written, writeErr, want)
	}
	if len(err.Error()) > DiagnosticLimit+len(failure.Error())+2 {
		t.Fatalf("the refusal holds %d bytes, past the limit of %d", len(err.Error()), DiagnosticLimit)
	}
}
