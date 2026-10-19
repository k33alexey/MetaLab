//go:build windows

package secretstore

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// A secret is sealed with the DPAPI machine key, which any process of the
// machine can open; the rights on the file and the directory keep it from
// other users (PLATFORM-MODEL.md, «Секреты в Windows»).
const secretExtension = ".dpapi"

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
