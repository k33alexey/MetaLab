package systemservice

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	kservice "github.com/kardianos/service"

	"github.com/k33alexey/MetaLab/internal/appconfig"
)

func TestControllerBuildsAbsoluteInstallDefinition(t *testing.T) {
	t.Parallel()

	var captured *kservice.Config
	native := &fakeBackend{}
	controller := &Controller{factory: func(_ kservice.Interface, configuration *kservice.Config) (backend, error) {
		captured = configuration
		return native, nil
	}}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	message, err := controller.Control("install", path)
	if err != nil {
		t.Fatal(err)
	}
	if message != "install completed" || native.installs != 1 {
		t.Fatalf("message=%q installs=%d", message, native.installs)
	}
	if len(captured.Arguments) != 4 || captured.Arguments[0] != "service" || captured.Arguments[1] != "run" || !filepath.IsAbs(captured.Arguments[3]) {
		t.Fatalf("arguments = %#v", captured.Arguments)
	}
	if captured.Name != "metalab" || captured.Option["StartType"] != "automatic" {
		t.Fatalf("definition = %+v", captured)
	}
	if runtime.GOOS != "windows" && captured.Option["UserService"] != true {
		t.Fatalf("service must retain the installing user's protected-store identity: %+v", captured.Option)
	}
}

func TestControllerControlsAndReportsStatus(t *testing.T) {
	t.Parallel()

	native := &fakeBackend{status: kservice.StatusRunning}
	controller := &Controller{factory: func(kservice.Interface, *kservice.Config) (backend, error) { return native, nil }}
	status, err := controller.Control("status", "")
	if err != nil || status != "running" {
		t.Fatalf("status=%q error=%v", status, err)
	}
	for _, action := range []string{"start", "stop", "restart", "uninstall"} {
		if _, err := controller.Control(action, ""); err != nil {
			t.Fatalf("%s: %v", action, err)
		}
	}
	if native.starts != 1 || native.stops != 1 || native.restarts != 1 || native.uninstalls != 1 {
		t.Fatalf("backend = %+v", native)
	}
}

func TestControllerRejectsInvalidActions(t *testing.T) {
	t.Parallel()

	controller := &Controller{factory: func(kservice.Interface, *kservice.Config) (backend, error) { return &fakeBackend{}, nil }}
	if _, err := controller.Control("install", ""); err == nil {
		t.Fatal("install without config succeeded")
	}
	if _, err := controller.Control("invalid", ""); err == nil {
		t.Fatal("invalid action succeeded")
	}
}

func TestControllerRunsForegroundDirectlyWhenInteractive(t *testing.T) {
	t.Parallel()

	called := false
	controller := &Controller{
		interactive: func() bool { return true },
		runner: func(context.Context, appconfig.Config) error {
			called = true
			return nil
		},
	}
	if err := controller.Run(context.Background(), appconfig.Default()); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("interactive runner was not called")
	}
}

func TestProgramStopsRunnerAndWaitsForCompletion(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	finished := make(chan struct{})
	program := newProgram(context.Background(), appconfig.Default(), func(ctx context.Context, _ appconfig.Config) error {
		close(started)
		<-ctx.Done()
		close(finished)
		return ctx.Err()
	})
	service := &fakeBackend{program: program}
	if err := program.Start(service); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := program.Stop(service); err != nil {
		t.Fatal(err)
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("runner did not finish")
	}
	if err := program.Err(); err != nil {
		t.Fatalf("Err() = %v", err)
	}
}

func TestProgramReportsFailureAndRequestsNativeStop(t *testing.T) {
	t.Parallel()

	want := errors.New("startup failed")
	program := newProgram(context.Background(), appconfig.Default(), func(context.Context, appconfig.Config) error {
		return want
	})
	service := &fakeBackend{program: program}
	if err := program.Start(service); err != nil {
		t.Fatal(err)
	}
	program.Wait()
	deadline := time.Now().Add(time.Second)
	for service.stopCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if service.stopCount() != 1 || !strings.Contains(program.Err().Error(), want.Error()) {
		t.Fatalf("stops=%d error=%v", service.stopCount(), program.Err())
	}
}

type fakeBackend struct {
	mu         sync.Mutex
	program    kservice.Interface
	status     kservice.Status
	starts     int
	stops      int
	restarts   int
	installs   int
	uninstalls int
}

func (backend *fakeBackend) Run() error {
	if backend.program == nil {
		return nil
	}
	if err := backend.program.Start(backend); err != nil {
		return err
	}
	return backend.program.Stop(backend)
}

func (backend *fakeBackend) Start() error {
	backend.mu.Lock()
	backend.starts++
	backend.mu.Unlock()
	return nil
}
func (backend *fakeBackend) Restart() error {
	backend.mu.Lock()
	backend.restarts++
	backend.mu.Unlock()
	return nil
}
func (backend *fakeBackend) Install() error {
	backend.mu.Lock()
	backend.installs++
	backend.mu.Unlock()
	return nil
}
func (backend *fakeBackend) Uninstall() error {
	backend.mu.Lock()
	backend.uninstalls++
	backend.mu.Unlock()
	return nil
}
func (backend *fakeBackend) Stop() error {
	backend.mu.Lock()
	backend.stops++
	program := backend.program
	backend.mu.Unlock()
	if program != nil {
		return program.Stop(backend)
	}
	return nil
}
func (backend *fakeBackend) Status() (kservice.Status, error) { return backend.status, nil }
func (backend *fakeBackend) Logger(chan<- error) (kservice.Logger, error) {
	return kservice.ConsoleLogger, nil
}
func (backend *fakeBackend) SystemLogger(chan<- error) (kservice.Logger, error) {
	return kservice.ConsoleLogger, nil
}
func (backend *fakeBackend) String() string   { return "MetaLab Service" }
func (backend *fakeBackend) Platform() string { return "fake" }
func (backend *fakeBackend) stopCount() int {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	return backend.stops
}
