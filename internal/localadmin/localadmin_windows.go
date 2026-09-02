//go:build windows

// Package localadmin verifies the OS boundary for destructive local CLI actions.
package localadmin

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// Require returns an error unless the process is elevated by the local OS.
func Require() error {
	token := windows.GetCurrentProcessToken()
	if !token.IsElevated() {
		return fmt.Errorf("local OS administrator privileges are required; run an elevated terminal")
	}
	return nil
}
