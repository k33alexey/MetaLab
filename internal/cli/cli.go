// Package cli implements the root MetaLab command-line interface.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/buildinfo"
)

const usage = `MetaLab

Usage:
  ml manager [--config PATH]
  ml service [--config PATH]
  ml config validate [--config PATH]
  ml version
  ml help

Running ml without arguments starts ML Manager.
Environment overrides: ML_LANGUAGE, ML_SERVICE_LISTEN, ML_DATABASE_URL.
`

// Runner starts one long-running MetaLab mode.
type Runner func(context.Context, appconfig.Config) error

// Commands contains platform-specific mode implementations.
type Commands struct {
	Manager Runner
	Service Runner
}

// CLI is the root MetaLab command-line application.
type CLI struct {
	build    buildinfo.Info
	commands Commands
}

// New creates a CLI for the provided build and mode implementations.
func New(build buildinfo.Info, commands Commands) CLI {
	return CLI{build: build, commands: commands}
}

// Run executes a command and returns its process exit code.
func (cli CLI) Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		args = []string{"manager"}
	}

	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "version", "-v", "--version":
		fmt.Fprintln(stdout, cli.build.String())
		return 0
	case "manager":
		return cli.runMode(ctx, "manager", args[1:], cli.commands.Manager, stderr)
	case "service":
		return cli.runMode(ctx, "service", args[1:], cli.commands.Service, stderr)
	case "config":
		return cli.runConfig(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func (cli CLI) runMode(ctx context.Context, name string, args []string, runner Runner, stderr io.Writer) int {
	configuration, _, ok := loadConfiguration(name, args, stderr)
	if !ok {
		return 2
	}
	if runner == nil {
		fmt.Fprintf(stderr, "%s mode is unavailable in this build\n", name)
		return 1
	}
	if err := runner(ctx, configuration); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 1
	}
	return 0
}

func (cli CLI) runConfig(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "validate" {
		fmt.Fprintln(stderr, "usage: ml config validate [--config PATH]")
		return 2
	}
	_, path, ok := loadConfiguration("config validate", args[1:], stderr)
	if !ok {
		return 2
	}
	fmt.Fprintf(stdout, "configuration is valid: %s\n", path)
	return 0
}

func loadConfiguration(name string, args []string, stderr io.Writer) (appconfig.Config, string, bool) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("config", "", "path to MetaLab YAML configuration")
	if err := flags.Parse(args); err != nil {
		return appconfig.Config{}, "", false
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "%s: unexpected argument %q\n", name, flags.Arg(0))
		return appconfig.Config{}, "", false
	}
	configuration, loadedPath, err := appconfig.Load(*path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return appconfig.Config{}, loadedPath, false
	}
	return configuration, loadedPath, true
}
