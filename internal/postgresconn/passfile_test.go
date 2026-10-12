package postgresconn

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Defect: a ":" or "\" in a field is not escaped, so libpq splits the line in
// the wrong place and sends a piece of the password, or matches another
// database. A Unix socket host is not matched by libpq unless written as localhost.
func TestPasswordLineEscapesEveryField(t *testing.T) {
	t.Parallel()

	connection := Descriptor{Host: "db:1", Port: 5432, Database: `app\db`, User: "ml:user"}
	line := passwordLine(Credential{Connection: connection, Password: `p:a\ss`})
	if line != "db\\:1:5432:app\\\\db:ml\\:user:p\\:a\\\\ss\n" {
		t.Fatalf("pgpass line = %q", line)
	}
	connection.Host = "/tmp"
	if line := passwordLine(Credential{Connection: connection, Password: "secret"}); !strings.HasPrefix(line, "localhost:") {
		t.Fatalf("Unix socket pgpass line = %q", line)
	}
}

func environmentValue(environment []string, name string) (string, int) {
	value, count := "", 0
	for _, variable := range environment {
		if after, found := strings.CutPrefix(variable, name+"="); found {
			value, count = after, count+1
		}
	}
	return value, count
}

// Defect: the password file outlives the tool run (on success or failure),
// is readable by others, lies beside the data the tool works on, or the tool
// still receives a password inherited through PGPASSWORD.
func TestPasswordFileLivesOnlyForTheActionAndOnlyForTheOwner(t *testing.T) {
	t.Setenv("PGPASSWORD", "inherited")
	t.Setenv("PGPASSFILE", "/inherited/pgpass")

	credentials := []Credential{
		{Connection: Descriptor{Host: "source", Port: 5432, Database: "main", User: "a"}, Password: "first"},
		{Connection: Descriptor{Host: "target", Port: 5433, Database: "copy", User: "b"}, Password: "second"},
	}
	for _, failure := range []error{nil, errors.New("pg_dump failed")} {
		var path string
		err := WithPasswordFile(credentials, func(environment []string) error {
			var count int
			path, count = environmentValue(environment, "PGPASSFILE")
			if count != 1 {
				t.Fatalf("PGPASSFILE given %d times", count)
			}
			if _, count := environmentValue(environment, "PGPASSWORD"); count != 0 {
				t.Fatal("inherited PGPASSWORD reaches the tool")
			}
			if timeout, _ := environmentValue(environment, "PGCONNECT_TIMEOUT"); timeout != "10" {
				t.Fatalf("PGCONNECT_TIMEOUT = %q", timeout)
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != "source:5432:main:a:first\ntarget:5433:copy:b:second\n" {
				t.Fatalf("password file = %q", content)
			}
			if !strings.HasPrefix(filepath.Base(filepath.Dir(path)), "metalab-pgpass-") {
				t.Fatalf("password file is not in its own directory: %s", path)
			}
			if runtime.GOOS != "windows" {
				for checked, want := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700} {
					info, err := os.Stat(checked)
					if err != nil {
						t.Fatal(err)
					}
					if info.Mode().Perm() != want {
						t.Fatalf("%s permissions = %v, want %v", checked, info.Mode().Perm(), want)
					}
				}
			}
			return failure
		})
		if !errors.Is(err, failure) || (failure == nil && err != nil) {
			t.Fatalf("WithPasswordFile error = %v, want %v", err, failure)
		}
		if _, statErr := os.Stat(filepath.Dir(path)); !os.IsNotExist(statErr) {
			t.Fatalf("password directory left after the action (failure %v): %v", failure, statErr)
		}
	}
}

// Defect: a tool started without a password file (a version check) still
// receives an inherited password or password file.
func TestToolEnvironmentWithoutPasswordFile(t *testing.T) {
	t.Setenv("PGPASSWORD", "inherited")
	t.Setenv("PGPASSFILE", "/inherited/pgpass")
	environment := ToolEnvironment("")
	for _, name := range []string{"PGPASSWORD", "PGPASSFILE"} {
		if _, count := environmentValue(environment, name); count != 0 {
			t.Fatalf("%s reaches the tool", name)
		}
	}
}
