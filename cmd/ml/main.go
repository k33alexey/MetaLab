package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/k33alexey/MetaLab/internal/buildinfo"
	"github.com/k33alexey/MetaLab/internal/cli"
	"github.com/k33alexey/MetaLab/internal/host"
	"github.com/k33alexey/MetaLab/internal/systemservice"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	nativeService := systemservice.New(host.RunService)
	application := cli.New(buildinfo.Info{
		Version: version,
		Commit:  commit,
		Date:    date,
	}, cli.Commands{
		Manager: runManager,
		Studio:  runStudio,
		Service: nativeService.Run,
		Control: nativeService.Control,
		Reset:   resetAdministrator,
	})
	os.Exit(application.Run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
