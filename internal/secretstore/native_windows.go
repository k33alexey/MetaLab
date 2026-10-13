//go:build windows

package secretstore

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/k33alexey/MetaLab/internal/ownerfile"
)

// A secret is sealed with the DPAPI machine key, which any process of the
// machine can open; the rights on the file and the directory keep it from
// other users (PLATFORM-MODEL.md, «Секреты в Windows»).
const secretExtension = ".dpapi"

func secretDirectory() (string, error) {
	programData := os.Getenv("ProgramData")
	if programData == "" {
		return "", fmt.Errorf("ProgramData is not configured")
	}
	return filepath.Join(programData, "MetaLab", "secrets"), nil
}

func prepareSecretDirectory(directory string) error {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create Windows secret directory: %w", err)
	}
	if err := ownerfile.Restrict(directory); err != nil {
		return fmt.Errorf("restrict Windows secret directory: %w", err)
	}
	return nil
}

// handOverSecret has nothing to do: the system, the administrators and the
// writing account already have access to a file ownerfile creates.
func handOverSecret(string) error { return nil }

func replaceSecret(temporaryPath, path string) error {
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

func sealSecret(plain []byte) ([]byte, error) {
	input := windows.DataBlob{Size: uint32(len(plain)), Data: &plain[0]}
	var output windows.DataBlob
	flags := uint32(windows.CRYPTPROTECT_LOCAL_MACHINE | windows.CRYPTPROTECT_UI_FORBIDDEN)
	if err := windows.CryptProtectData(&input, nil, nil, 0, nil, flags, &output); err != nil {
		return nil, fmt.Errorf("protect Windows secret: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	return append([]byte(nil), unsafe.Slice(output.Data, output.Size)...), nil
}

func openSecret(protected []byte) ([]byte, error) {
	input := windows.DataBlob{Size: uint32(len(protected)), Data: &protected[0]}
	var output windows.DataBlob
	if err := windows.CryptUnprotectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, fmt.Errorf("unprotect Windows secret: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	return append([]byte(nil), unsafe.Slice(output.Data, output.Size)...), nil
}
