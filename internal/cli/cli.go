// Package cli implements the root MetaLab command-line interface.
package cli

import (
	"fmt"
	"io"

	"github.com/k33alexey/MetaLab/internal/buildinfo"
)

const usage = `MetaLab

Usage:
  ml help
  ml version
`

// CLI is the root MetaLab command-line application.
type CLI struct {
	build buildinfo.Info
}

// New creates a CLI for the provided build.
func New(build buildinfo.Info) CLI {
	return CLI{build: build}
}

// Run executes a command and returns its process exit code.
func (c CLI) Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, usage)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "version", "-v", "--version":
		fmt.Fprintln(stdout, c.build.String())
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
