// Package systemservice adapts ML Service to native operating-system services.
package systemservice

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	kservice "github.com/kardianos/service"

	"github.com/k33alexey/MetaLab/internal/appconfig"
)

const stopTimeout = 10 * time.Second

// Runner executes the actual ML Service workload.
type Runner func(context.Context, appconfig.Config) error

type backend interface {
	kservice.Service
}

type factory func(kservice.Interface, *kservice.Config) (backend, error)

// Controller installs, controls and runs the native ML Service.
type Controller struct {
	runner      Runner
	factory     factory
	interactive func() bool
}

// New creates a native service controller.
func New(runner Runner) *Controller {
	return &Controller{
		runner: runner, interactive: kservice.Interactive,
		factory: func(program kservice.Interface, config *kservice.Config) (backend, error) {
			return kservice.New(program, config)
		},
	}
}

// Run enters the operating-system service lifecycle.
func (controller *Controller) Run(ctx context.Context, configuration appconfig.Config) error {
	if controller.runner == nil {
		return fmt.Errorf("ML Service runner is nil")
	}
	if controller.interactive != nil && controller.interactive() {
		return controller.runner(ctx, configuration)
	}
	program := newProgram(ctx, configuration, controller.runner)
	definition, err := serviceDefinition("")
	if err != nil {
		return err
	}
	definition.Option["RunWait"] = program.Wait
	service, err := controller.factory(program, definition)
	if err != nil {
		return fmt.Errorf("create native service: %w", err)
	}
	if err := service.Run(); err != nil {
		return fmt.Errorf("run native service: %w", err)
	}
	return program.Err()
}

// Control executes an install, lifecycle or status action.
func (controller *Controller) Control(action, configurationPath string) (string, error) {
	if action == "install" {
		if configurationPath == "" {
			return "", fmt.Errorf("service install requires an explicit configuration path")
		}
		absolute, err := filepath.Abs(configurationPath)
		if err != nil {
			return "", fmt.Errorf("resolve configuration path: %w", err)
		}
		configurationPath = absolute
		information, err := os.Stat(configurationPath)
		if err != nil {
			return "", fmt.Errorf("read service configuration %q: %w", configurationPath, err)
		}
		if !information.Mode().IsRegular() {
			return "", fmt.Errorf("service configuration %q is not a regular file", configurationPath)
		}
	}
	definition, err := serviceDefinition(configurationPath)
	if err != nil {
		return "", err
	}
	service, err := controller.factory(passiveProgram{}, definition)
	if err != nil {
		return "", fmt.Errorf("create native service: %w", err)
	}
	if action == "status" {
		status, statusErr := service.Status()
		if statusErr != nil {
			return "", fmt.Errorf("read ML Service status: %w", statusErr)
		}
		return statusName(status), nil
	}
	if !isControlAction(action) {
		return "", fmt.Errorf("unsupported service action %q", action)
	}
	if err := kservice.Control(service, action); err != nil {
		return "", err
	}
	return action + " completed", nil
}

func serviceDefinition(configurationPath string) (*kservice.Config, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve MetaLab executable: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return nil, fmt.Errorf("resolve MetaLab executable path: %w", err)
	}
	arguments := []string{"service", "run"}
	if configurationPath != "" {
		arguments = append(arguments, "--config", configurationPath)
	}
	definition := &kservice.Config{
		Name: "metalab", DisplayName: "MetaLab Service",
		Description:      "MetaLab application platform service",
		Executable:       executable,
		Arguments:        arguments,
		WorkingDirectory: filepath.Dir(executable),
		Dependencies:     []string{"After=network-online.target", "Wants=network-online.target"},
		Option: kservice.KeyValue{
			"KeepAlive": true, "RunAtLoad": true, "Restart": "on-failure",
			"DelayedAutoStart": true, "StartType": "automatic",
			"OnFailure": "restart", "OnFailureDelayDuration": "5s",
		},
	}
	configureServiceIdentity(definition)
	return definition, nil
}

func statusName(status kservice.Status) string {
	switch status {
	case kservice.StatusRunning:
		return "running"
	case kservice.StatusStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

func isControlAction(action string) bool {
	switch action {
	case "install", "uninstall", "start", "stop", "restart":
		return true
	default:
		return false
	}
}

type program struct {
	parent        context.Context
	configuration appconfig.Config
	runner        Runner

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

func newProgram(parent context.Context, configuration appconfig.Config, runner Runner) *program {
	return &program{parent: parent, configuration: configuration, runner: runner, done: make(chan struct{})}
}

func (program *program) Start(service kservice.Service) error {
	program.mu.Lock()
	if program.cancel != nil {
		program.mu.Unlock()
		return fmt.Errorf("ML Service is already started")
	}
	ctx, cancel := context.WithCancel(program.parent)
	program.cancel = cancel
	program.mu.Unlock()
	go func() {
		err := program.runner(ctx, program.configuration)
		program.mu.Lock()
		program.err = err
		program.mu.Unlock()
		close(program.done)
		if ctx.Err() == nil {
			_ = service.Stop()
		}
	}()
	return nil
}

func (program *program) Stop(kservice.Service) error {
	program.mu.Lock()
	cancel := program.cancel
	program.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-program.done:
		return program.Err()
	case <-time.After(stopTimeout):
		return fmt.Errorf("ML Service did not stop within %s", stopTimeout)
	}
}

func (program *program) Shutdown(service kservice.Service) error { return program.Stop(service) }

func (program *program) Wait() { <-program.done }

func (program *program) Err() error {
	program.mu.Lock()
	defer program.mu.Unlock()
	if errors.Is(program.err, context.Canceled) {
		return nil
	}
	return program.err
}

type passiveProgram struct{}

func (passiveProgram) Start(kservice.Service) error { return nil }
func (passiveProgram) Stop(kservice.Service) error  { return nil }
