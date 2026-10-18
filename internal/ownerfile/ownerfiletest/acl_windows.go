//go:build windows

// Package ownerfiletest reads back the rights Windows keeps for a path made
// owner-only, for the tests of every package that lays one.
package ownerfiletest

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// fileAllAccess is the FA right of SDDL, FILE_ALL_ACCESS in winnt.h.
const fileAllAccess = 0x1f01ff

// AssertOwnerOnly reads the DACL Windows actually keeps for path and requires
// exactly: protected from inheritance, full access for the system,
// administrators and the process user, passed on to children only by a directory.
func AssertOwnerOnly(t testing.TB, path string, directory bool) {
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
