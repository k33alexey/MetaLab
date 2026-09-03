//go:build !windows

package studio

import "os"

func replaceStudioFile(source, destination string) error { return os.Rename(source, destination) }
