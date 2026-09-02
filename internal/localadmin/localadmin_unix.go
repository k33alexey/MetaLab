//go:build darwin || linux

// Package localadmin verifies the OS boundary for destructive local CLI actions.
package localadmin

import (
	"fmt"
	"os"
)

// Require returns an error unless the process is elevated by the local OS.
func Require() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("local OS administrator privileges are required; run this command with sudo")
	}
	return nil
}
