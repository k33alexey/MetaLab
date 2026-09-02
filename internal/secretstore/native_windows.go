//go:build windows

package secretstore

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

type nativeBackend struct{}

func (nativeBackend) Set(service, account, secret string) error {
	directory, path, err := windowsSecretPath(service, account)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create Windows secret directory: %w", err)
	}
	if err := restrictWindowsPath(directory); err != nil {
		return err
	}
	protected, err := protectWindowsData([]byte(secret))
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".secret-*")
	if err != nil {
		return fmt.Errorf("create temporary Windows secret: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.Write(protected); err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return fmt.Errorf("write Windows secret: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close Windows secret: %w", closeErr)
	}
	if err := restrictWindowsPath(temporaryPath); err != nil {
		return err
	}
	from, err := windows.UTF16PtrFromString(temporaryPath)
	if err != nil {
		return fmt.Errorf("encode temporary Windows secret path: %w", err)
	}
	to, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("encode Windows secret path: %w", err)
	}
	if err := windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH); err != nil {
		return fmt.Errorf("replace Windows secret: %w", err)
	}
	return nil
}

func (nativeBackend) Get(service, account string) (string, error) {
	_, path, err := windowsSecretPath(service, account)
	if err != nil {
		return "", err
	}
	protected, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read Windows secret: %w", err)
	}
	plain, err := unprotectWindowsData(protected)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (nativeBackend) Delete(service, account string) error {
	_, path, err := windowsSecretPath(service, account)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	return err
}

func windowsSecretPath(service, account string) (string, string, error) {
	programData := os.Getenv("ProgramData")
	if programData == "" {
		return "", "", fmt.Errorf("ProgramData is not configured")
	}
	directory := filepath.Join(programData, "MetaLab", "secrets")
	digest := sha256.Sum256([]byte(service + ":" + account))
	return directory, filepath.Join(directory, hex.EncodeToString(digest[:])+".dpapi"), nil
}

func protectWindowsData(plain []byte) ([]byte, error) {
	input := windows.DataBlob{Size: uint32(len(plain)), Data: &plain[0]}
	var output windows.DataBlob
	flags := uint32(windows.CRYPTPROTECT_LOCAL_MACHINE | windows.CRYPTPROTECT_UI_FORBIDDEN)
	if err := windows.CryptProtectData(&input, nil, nil, 0, nil, flags, &output); err != nil {
		return nil, fmt.Errorf("protect Windows secret: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	return append([]byte(nil), unsafe.Slice(output.Data, output.Size)...), nil
}

func unprotectWindowsData(protected []byte) ([]byte, error) {
	if len(protected) == 0 {
		return nil, fmt.Errorf("Windows secret is empty")
	}
	input := windows.DataBlob{Size: uint32(len(protected)), Data: &protected[0]}
	var output windows.DataBlob
	if err := windows.CryptUnprotectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, fmt.Errorf("unprotect Windows secret: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	return append([]byte(nil), unsafe.Slice(output.Data, output.Size)...), nil
}

func restrictWindowsPath(path string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("read Windows process identity: %w", err)
	}
	sddl := "D:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;FA;;;" + user.User.Sid.String() + ")"
	descriptor, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return fmt.Errorf("build Windows secret ACL: %w", err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return fmt.Errorf("read Windows secret ACL: %w", err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil); err != nil {
		return fmt.Errorf("restrict Windows secret path: %w", err)
	}
	return nil
}
