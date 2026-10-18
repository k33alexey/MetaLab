package appconfig

import (
	"path/filepath"
	"runtime"
	"testing"
)

// Defect: without a folder in the settings the backups go to the settings
// folder of whoever runs the process, so the service under its own account
// and Manager under an administrator keep and look for them in two places
// (ML-SERVICE.md, «Управление службой»).
func TestBackupDirectoryWithoutSettingIsTheServiceFolder(t *testing.T) {
	var want string
	switch runtime.GOOS {
	case "linux":
		want = "/var/lib/metalab/backups"
	case "windows":
		programData := t.TempDir()
		t.Setenv("ProgramData", programData)
		want = filepath.Join(programData, "MetaLab", "backups")
	case "darwin":
		home := t.TempDir()
		t.Setenv("HOME", home)
		want = filepath.Join(home, "Library", "Application Support", "MetaLab", "backups")
	default:
		t.Skipf("ML is not supported on %s", runtime.GOOS)
	}
	if got, err := Default().BackupDirectory(); err != nil || got != want {
		t.Fatalf("BackupDirectory() = %q, %v; want %q", got, err, want)
	}
}
