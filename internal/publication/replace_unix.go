//go:build !windows

package publication

import "os"

func replacePackageFile(source, destination string) error { return os.Rename(source, destination) }
