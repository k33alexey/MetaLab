package main

import (
	"os"

	"github.com/k33alexey/MetaLab/internal/buildinfo"
	"github.com/k33alexey/MetaLab/internal/cli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	application := cli.New(buildinfo.Info{
		Version: version,
		Commit:  commit,
		Date:    date,
	})

	os.Exit(application.Run(os.Args[1:], os.Stdout, os.Stderr))
}
