//go:build windows

package ownerfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// fileAllAccess is the FA right of SDDL, FILE_ALL_ACCESS in winnt.h.
const fileAllAccess = 0x1f01ff

// assertOwnerOnly reads the DACL Windows actually keeps for path and requires
// exactly: protected from inheritance, full access for the system,
// administrators and the process user, passed on to children only by a directory.
func assertOwnerOnly(t *testing.T, path string, directory bool) {
	t.Helper()
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := descriptor.Control()
	if err != nil {
		t.Fatal(err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatalf("%s: DACL inherits from the parent: %s", path, descriptor)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatal(err)
	}
	system, _ := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	administrators, _ := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	want := []*windows.SID{system, administrators, user.User.Sid}
	if int(dacl.AceCount) != len(want) {
		t.Fatalf("%s: %d ACEs, want %d: %s", path, dacl.AceCount, len(want), descriptor)
	}
	var inherit uint8
	if directory {
		inherit = windows.OBJECT_INHERIT_ACE | windows.CONTAINER_INHERIT_ACE
	}
	for index, trustee := range want {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, uint32(index), &ace); err != nil {
			t.Fatal(err)
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Mask != fileAllAccess ||
			ace.Header.AceFlags != inherit || !sid.Equals(trustee) {
			t.Fatalf("%s: ACE %d is %s; descriptor %s", path, index, sid, descriptor)
		}
	}
}

// Defect: a secret file or directory is created with the rights of its
// parent and is closed only afterwards, or not closed at all.
func TestPathsAreOwnerOnlyFromCreation(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	file, err := Create(filepath.Join(directory, "pgpass"))
	if err != nil {
		t.Fatal(err)
	}
	assertOwnerOnly(t, file.Name(), false)
	_ = file.Close()

	private, err := MkdirTemp(directory, "metalab-*")
	if err != nil {
		t.Fatal(err)
	}
	assertOwnerOnly(t, private, true)

	// What a tool such as pg_dump creates inside receives the same rights.
	child := filepath.Join(private, "database.dump")
	if err := os.WriteFile(child, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	descriptor, err := windows.GetNamedSecurityInfo(child, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, _ := descriptor.DACL()
	if dacl.AceCount != 3 {
		t.Fatalf("file inside an owner-only directory: %s", descriptor)
	}

	temporary, err := CreateTemp(directory, ".secret-*")
	if err != nil {
		t.Fatal(err)
	}
	assertOwnerOnly(t, temporary.Name(), false)
	_ = temporary.Close()
}

// Defect: Create opens a file laid at the path beforehand.
func TestCreateRefusesAnExistingFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "pgpass")
	if err := os.WriteFile(path, []byte("planted"), 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(path); !errors.Is(err, os.ErrExist) {
		t.Fatalf("Create over an existing file = %v, want os.ErrExist", err)
	}
	if err := Mkdir(filepath.Dir(path)); !errors.Is(err, os.ErrExist) {
		t.Fatalf("Mkdir over an existing directory = %v, want os.ErrExist", err)
	}
}

// Defect: Restrict leaves an existing path open to the rights of its parent.
func TestRestrictClosesExistingPaths(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(directory, "secret")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Restrict(file); err != nil {
		t.Fatal(err)
	}
	if err := Restrict(directory); err != nil {
		t.Fatal(err)
	}
	assertOwnerOnly(t, file, false)
	assertOwnerOnly(t, directory, true)
}
