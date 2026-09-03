//go:build !windows

package project

import "os"

func replaceProjectFile(source, destination string) error { return os.Rename(source, destination) }
