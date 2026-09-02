//go:build !windows

package appconfig

import "os"

func replaceFile(source, destination string) error { return os.Rename(source, destination) }
