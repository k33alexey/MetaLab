//go:build !darwin && !linux && !windows

// Package localadmin verifies the OS boundary for destructive local CLI actions.
package localadmin

import "fmt"

// Require rejects unsupported operating systems.
func Require() error { return fmt.Errorf("local OS administrator verification is unsupported") }
