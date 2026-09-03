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
  ml service run [--config PATH]
  ml service install --config PATH
  ml service start|stop|restart|status|uninstall
  ml admin reset-password --login LOGIN [--config PATH]
  ml config validate [--config PATH]
  ml version
  ml help

Running ml without arguments starts ML Manager.
Environment overrides: ML_LANGUAGE, ML_SERVICE_LISTEN, ML_BACKUP_DIRECTORY, ML_DATABASE_URL, ML_SYSTEM_DATABASE_URL.
`

// Runner starts one long-running MetaLab mode.
type Runner func(context.Context, appconfig.Config) error

// ServiceControl executes one native service-manager action.
type ServiceControl func(action, configurationPath string) (string, error)

// EmergencyCredentials are displayed once after a local administrator reset.
type EmergencyCredentials struct {
	TemporaryPassword string
	RecoveryCodes     []string
}

// AdministratorReset performs an OS-local emergency reset.
type AdministratorReset func(context.Context, string, appconfig.Config) (EmergencyCredentials, error)

// Commands contains platform-specific mode implementations.
type Commands struct {
	Manager Runner
	Service Runner
	Control ServiceControl
	Reset   AdministratorReset
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
		return cli.runService(ctx, args[1:], stdout, stderr)
	case "admin":
		return cli.runAdmin(ctx, args[1:], stdout, stderr)
	case "config":
		return cli.runConfig(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func (cli CLI) runAdmin(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "reset-password" {
		fmt.Fprintln(stderr, "usage: ml admin reset-password --login LOGIN [--config PATH]")
		return 2
	}
	flags := flag.NewFlagSet("admin reset-password", flag.ContinueOnError)
	flags.SetOutput(stderr)
	login := flags.String("login", "", "administrator login")
	configurationPath := flags.String("config", "", "path to MetaLab YAML configuration")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || *login == "" {
		if err == nil {
			fmt.Fprintln(stderr, "usage: ml admin reset-password --login LOGIN [--config PATH]")
		}
		return 2
	}
	if cli.commands.Reset == nil {
		fmt.Fprintln(stderr, "local administrator reset is unavailable in this build")
		return 1
	}
	configuration, _, err := appconfig.Load(*configurationPath)
	if err != nil {
		fmt.Fprintf(stderr, "admin reset-password: %v\n", err)
		return 1
	}
	credentials, err := cli.commands.Reset(ctx, *login, configuration)
	if err != nil {
		fmt.Fprintf(stderr, "admin reset-password: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Temporary password: %s\nRecovery codes (shown once):\n", credentials.TemporaryPassword)
	for _, code := range credentials.RecoveryCodes {
		fmt.Fprintln(stdout, code)
	}
	return 0
}

func (cli CLI) runService(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "run" || len(args[0]) > 0 && args[0][0] == '-' {
		if len(args) != 0 && args[0] == "run" {
			args = args[1:]
		}
		return cli.runMode(ctx, "service run", args, cli.commands.Service, stderr)
	}
	action := args[0]
	if action == "install" {
		if !hasConfigFlag(args[1:]) {
			fmt.Fprintln(stderr, "service install requires --config PATH")
			return 2
		}
		_, path, ok := loadConfiguration("service install", args[1:], stderr)
		if !ok {
			return 2
		}
		return cli.controlService(action, path, stdout, stderr)
	}
	if len(args) != 1 {
		fmt.Fprintf(stderr, "service %s: unexpected arguments\n", action)
		return 2
	}
	return cli.controlService(action, "", stdout, stderr)
}

func (cli CLI) controlService(action, path string, stdout, stderr io.Writer) int {
	if cli.commands.Control == nil {
		fmt.Fprintln(stderr, "native service control is unavailable in this build")
		return 1
	}
	message, err := cli.commands.Control(action, path)
	if err != nil {
		fmt.Fprintf(stderr, "service %s: %v\n", action, err)
		return 1
	}
	fmt.Fprintln(stdout, message)
	return 0
}

func hasConfigFlag(args []string) bool {
	for _, argument := range args {
		if argument == "--config" || len(argument) > len("--config=") && argument[:len("--config=")] == "--config=" {
			return true
		}
	}
	return false
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
