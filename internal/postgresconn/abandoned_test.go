package postgresconn

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// useTemporaryDirectory points os.TempDir at a directory of the test alone.
func useTemporaryDirectory(t *testing.T) string {
	directory := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, directory)
	}
	return directory
}

const holderVariable = "ML_TEST_HOLD_PASSWORD_FILE"

// TestHoldPasswordFileHelper is the process killed in the middle of a tool
// run: it prints its password file and waits to be killed.
func TestHoldPasswordFileHelper(t *testing.T) {
	if os.Getenv(holderVariable) == "" {
		t.Skip("helper process only")
	}
	credentials := []Credential{{Connection: Descriptor{Host: "db", Port: 5432, Database: "main", User: "a"}, Password: "secret"}}
	_ = WithPasswordFile(credentials, func(environment []string) error {
		path, _ := environmentValue(environment, "PGPASSFILE")
		os.Stdout.WriteString(path + "\n")
		select {}
	})
}

// Defect: a process killed in the middle of pg_dump leaves the password as
// plain text in the temporary directory for good; or the cleanup removes the
// password file of a tool still running in another process.
func TestKilledProcessPasswordFileIsRemovedAndLivingOneKept(t *testing.T) {
	useTemporaryDirectory(t)
	command := exec.Command(os.Args[0], "-test.run=^TestHoldPasswordFileHelper$")
	command.Env = append(os.Environ(), holderVariable+"=1")
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil {
		command.Process.Kill()
		t.Fatalf("helper printed no password file: %v", err)
	}
	path := line[:len(line)-1]

	if removed, err := RemoveAbandonedPasswordFiles(); removed != 0 || err != nil {
		command.Process.Kill()
		t.Fatalf("cleanup with the tool running: removed %d, error %v", removed, err)
	}
	if _, err := os.Stat(path); err != nil {
		command.Process.Kill()
		t.Fatalf("password file of a running process removed: %v", err)
	}

	if err := command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = command.Wait()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("killed process left no password file, the test proves nothing: %v", err)
	}
	if removed, err := RemoveAbandonedPasswordFiles(); removed != 1 || err != nil {
		t.Fatalf("cleanup after the kill: removed %d, error %v", removed, err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("abandoned password directory left: %v", err)
	}
}

// Defect: the cleanup removes the password directory of a tool running in
// the same process (Manager runs a copy while Studio starts in it), because a
// lock held by the own process does not stop it.
func TestPasswordFileOfOwnRunningToolIsKept(t *testing.T) {
	useTemporaryDirectory(t)
	credentials := []Credential{{Connection: Descriptor{Host: "db", Port: 5432, Database: "main", User: "a"}, Password: "secret"}}
	err := WithPasswordFile(credentials, func(environment []string) error {
		path, _ := environmentValue(environment, "PGPASSFILE")
		if removed, err := RemoveAbandonedPasswordFiles(); removed != 0 || err != nil {
			t.Fatalf("cleanup during the run: removed %d, error %v", removed, err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("password file removed during the run: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Defect: the cleanup removes a directory its process is creating right now
// (no lock file yet, no password yet), so that process's tool run fails; or
// it follows a link someone put under the directory name and removes what
// the link points to.
func TestCleanupKeepsDirectoryWithoutLockAndLinks(t *testing.T) {
	temporary := useTemporaryDirectory(t)
	creating := filepath.Join(temporary, "metalab-pgpass-creating")
	if err := os.Mkdir(creating, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(creating, passwordLockName+".pending"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		target := t.TempDir()
		if err := os.WriteFile(filepath.Join(target, passwordLockName), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(temporary, "metalab-pgpass-link")); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if _, err := os.Stat(filepath.Join(target, passwordLockName)); err != nil {
				t.Fatalf("cleanup followed a link: %v", err)
			}
		}()
	}
	if removed, err := RemoveAbandonedPasswordFiles(); removed != 0 || err != nil {
		t.Fatalf("removed %d, error %v", removed, err)
	}
	if _, err := os.Stat(creating); err != nil {
		t.Fatalf("directory being created removed: %v", err)
	}
}
