//go:build windows

package ownerfile

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ownerOnly is the security descriptor of every owner-only path: full access
// for the system (SY), administrators (BA) and the process user, nothing
// inherited from the parent (P). A directory passes its entries on to the
// files (OI) and directories (CI) created inside it.
func ownerOnly(directory bool) (*windows.SECURITY_DESCRIPTOR, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("read Windows process identity: %w", err)
	}
	inherit := ""
	if directory {
		inherit = "OICI"
	}
	sddl := "D:P"
	for _, trustee := range []string{"SY", "BA", user.User.Sid.String()} {
		sddl += "(A;" + inherit + ";FA;;;" + trustee + ")"
	}
	descriptor, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return nil, fmt.Errorf("build owner-only Windows ACL: %w", err)
	}
	return descriptor, nil
}

func ownerOnlyAttributes(directory bool) (*windows.SecurityAttributes, error) {
	descriptor, err := ownerOnly(directory)
	if err != nil {
		return nil, err
	}
	attributes := &windows.SecurityAttributes{SecurityDescriptor: descriptor}
	attributes.Length = uint32(unsafe.Sizeof(*attributes))
	return attributes, nil
}

// Create creates a new owner-only file at path and opens it for reading and
// writing; an existing file is an error. The rights are given at creation, so
// no other process can open the file in between.
func Create(path string) (*os.File, error) {
	attributes, err := ownerOnlyAttributes(false)
	if err != nil {
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, &os.PathError{Op: "create owner-only file", Path: path, Err: err}
	}
	handle, err := windows.CreateFile(name,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		attributes, windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "create owner-only file", Path: path, Err: err}
	}
	return os.NewFile(uintptr(handle), path), nil
}

// Mkdir creates a new owner-only directory at path; an existing one is an error.
func Mkdir(path string) error {
	attributes, err := ownerOnlyAttributes(true)
	if err != nil {
		return err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return &os.PathError{Op: "create owner-only directory", Path: path, Err: err}
	}
	if err := windows.CreateDirectory(name, attributes); err != nil {
		return &os.PathError{Op: "create owner-only directory", Path: path, Err: err}
	}
	return nil
}

// Restrict closes an existing file or directory to everyone but its owner.
func Restrict(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("restrict %s to its owner: %w", path, err)
	}
	descriptor, err := ownerOnly(info.IsDir())
	if err != nil {
		return err
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return fmt.Errorf("read owner-only Windows ACL: %w", err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil); err != nil {
		return fmt.Errorf("restrict %s to its owner: %w", path, err)
	}
	return nil
}

// IsOwnDirectory reports whether path is a directory itself, not a link or a
// junction to one, and its owner is the one this process gives to what it
// creates. A service's temporary directory (Windows\Temp) is open for other
// users to create entries in, so a name alone proves nothing.
func IsOwnDirectory(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	if info.Mode().Type() != os.ModeDir {
		return false, nil
	}
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return false, &os.PathError{Op: "read owner", Path: path, Err: err}
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		return false, &os.PathError{Op: "read owner", Path: path, Err: err}
	}
	processOwner, err := defaultOwner()
	if err != nil {
		return false, err
	}
	return owner != nil && owner.Equals(processOwner), nil
}

// defaultOwner is the owner the process token gives to new objects: the user,
// or the administrators group for an elevated administrator.
func defaultOwner() (*windows.SID, error) {
	token := windows.GetCurrentProcessToken()
	size := uint32(64)
	for {
		buffer := make([]byte, size)
		err := windows.GetTokenInformation(token, windows.TokenOwner, &buffer[0], size, &size)
		if err == nil {
			return (*struct{ Owner *windows.SID })(unsafe.Pointer(&buffer[0])).Owner.Copy()
		}
		if !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
			return nil, fmt.Errorf("read Windows process default owner: %w", err)
		}
	}
}
