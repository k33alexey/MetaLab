package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/buildinfo"
)

func TestRunSimpleCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "help", args: []string{"help"}, wantStdout: "Usage:"},
		{name: "version", args: []string{"version"}, wantStdout: "MetaLab dev"},
		{name: "unknown", args: []string{"unknown"}, wantCode: 2, wantStderr: `unknown command "unknown"`},
		{name: "invalid config command", args: []string{"config"}, wantCode: 2, wantStderr: "ml config validate"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			code, stdout, stderr := run(t, Commands{}, test.args...)
			if code != test.wantCode || !strings.Contains(stdout, test.wantStdout) || !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
}

func TestNoArgumentsStartsManagerWithSharedConfiguration(t *testing.T) {
	t.Setenv("ML_LANGUAGE", "uk")
	called := false
	commands := Commands{Manager: func(_ context.Context, configuration appconfig.Config) error {
		called = true
		if configuration.Language != "uk" {
			t.Fatalf("configuration = %+v", configuration)
		}
		return nil
	}}
	code, _, stderr := run(t, commands)
	if code != 0 || !called {
		t.Fatalf("code=%d called=%v stderr=%q", code, called, stderr)
	}
}

func TestServiceUsesExplicitConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfiguration(t, path)
	called := false
	commands := Commands{Service: func(_ context.Context, configuration appconfig.Config) error {
		called = true
		if configuration.Service.Listen != "127.0.0.1:9200" {
			t.Fatalf("configuration = %+v", configuration)
		}
		return nil
	}}
	code, _, stderr := run(t, commands, "service", "--config", path)
	if code != 0 || !called {
		t.Fatalf("code=%d called=%v stderr=%q", code, called, stderr)
	}
}

func TestStudioUsesProjectAndSharedConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfiguration(t, path)
	var gotProject string
	commands := Commands{Studio: func(_ context.Context, configuration appconfig.Config, projectPath string) error {
		gotProject = projectPath
		if configuration.Service.Listen != "127.0.0.1:9200" {
			t.Fatalf("configuration = %+v", configuration)
		}
		return nil
	}}
	code, _, stderr := run(t, commands, "studio", "--project", "/projects/sales", "--config", path)
	if code != 0 || gotProject != "/projects/sales" {
		t.Fatalf("code=%d project=%q stderr=%q", code, gotProject, stderr)
	}
}

func TestStudioRequiresProjectAndDesktopRunner(t *testing.T) {
	t.Parallel()

	code, _, stderr := run(t, Commands{}, "studio")
	if code != 2 || !strings.Contains(stderr, "--project") {
		t.Fatalf("missing project: code=%d stderr=%q", code, stderr)
	}
	code, _, stderr = run(t, Commands{}, "studio", "--project", "/projects/sales")
	if code != 1 || !strings.Contains(stderr, "unavailable") {
		t.Fatalf("missing runner: code=%d stderr=%q", code, stderr)
	}
}

func TestRunReportsModeAndArgumentErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		commands Commands
		args     []string
		message  string
		code     int
	}{
		{name: "unavailable", args: []string{"manager"}, message: "unavailable", code: 1},
		{name: "runner", commands: Commands{Service: func(context.Context, appconfig.Config) error { return errors.New("failed") }}, args: []string{"service"}, message: "service run: failed", code: 1},
		{name: "argument", args: []string{"manager", "extra"}, message: "unexpected argument", code: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, _, stderr := run(t, test.commands, test.args...)
			if code != test.code || !strings.Contains(stderr, test.message) {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfiguration(t, path)
	code, stdout, stderr := run(t, Commands{}, "config", "validate", "--config", path)
	if code != 0 || !strings.Contains(stdout, path) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestServiceNativeActions(t *testing.T) {
	t.Parallel()

	var gotAction, gotPath string
	commands := Commands{Control: func(action, path string) (string, error) {
		gotAction, gotPath = action, path
		return "running", nil
	}}
	code, stdout, stderr := run(t, commands, "service", "status")
	if code != 0 || stdout != "running\n" || gotAction != "status" || gotPath != "" {
		t.Fatalf("code=%d stdout=%q stderr=%q action=%q path=%q", code, stdout, stderr, gotAction, gotPath)
	}
}

func TestServiceInstallRequiresAndValidatesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfiguration(t, path)
	var installedPath string
	commands := Commands{Control: func(action, configurationPath string) (string, error) {
		if action != "install" {
			t.Fatalf("action = %q", action)
		}
		installedPath = configurationPath
		return "install completed", nil
	}}
	code, _, stderr := run(t, commands, "service", "install", "--config", path)
	if code != 0 || installedPath != path {
		t.Fatalf("code=%d path=%q stderr=%q", code, installedPath, stderr)
	}
	code, _, stderr = run(t, commands, "service", "install")
	if code != 2 || !strings.Contains(stderr, "requires --config") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestServiceControlReportsErrors(t *testing.T) {
	t.Parallel()

	commands := Commands{Control: func(string, string) (string, error) { return "", errors.New("denied") }}
	code, _, stderr := run(t, commands, "service", "start")
	if code != 1 || !strings.Contains(stderr, "denied") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	code, _, stderr = run(t, Commands{}, "service", "stop", "extra")
	if code != 2 || !strings.Contains(stderr, "unexpected arguments") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestAdministratorEmergencyReset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfiguration(t, path)
	var gotLogin string
	commands := Commands{Reset: func(_ context.Context, login string, _ appconfig.Config) (EmergencyCredentials, error) {
		gotLogin = login
		return EmergencyCredentials{TemporaryPassword: "temporary", RecoveryCodes: []string{"ONE", "TWO"}}, nil
	}}
	code, stdout, stderr := run(t, commands, "admin", "reset-password", "--login", "admin", "--config", path)
	if code != 0 || gotLogin != "admin" || !strings.Contains(stdout, "temporary") || !strings.Contains(stdout, "ONE") {
		t.Fatalf("code=%d login=%q stdout=%q stderr=%q", code, gotLogin, stdout, stderr)
	}
}

func TestAdministratorEmergencyResetErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfiguration(t, path)
	code, _, stderr := run(t, Commands{}, "admin", "reset-password")
	if code != 2 || !strings.Contains(stderr, "--login") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	commands := Commands{Reset: func(context.Context, string, appconfig.Config) (EmergencyCredentials, error) {
		return EmergencyCredentials{}, errors.New("denied")
	}}
	code, _, stderr = run(t, commands, "admin", "reset-password", "--login", "admin", "--config", path)
	if code != 1 || !strings.Contains(stderr, "denied") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func run(t *testing.T, commands Commands, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	application := New(buildinfo.Info{Version: "dev", Commit: "none", Date: "unknown"}, commands)
	code := application.Run(context.Background(), args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func writeConfiguration(t *testing.T, path string) {
	t.Helper()
	content := []byte("version: 1\nlanguage: ru\nservice:\n  listen: 127.0.0.1:9200\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}
