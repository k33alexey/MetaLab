package buildinfo

import "fmt"

// Info describes a MetaLab build.
type Info struct {
	Version string
	Commit  string
	Date    string
}

// String returns a human-readable build description.
func (i Info) String() string {
	return fmt.Sprintf("MetaLab %s (commit: %s, built: %s)", i.Version, i.Commit, i.Date)
}
