//go:build desktop && (darwin || windows)

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"time"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/gitclient"
	"github.com/k33alexey/MetaLab/internal/manager"
	"github.com/k33alexey/MetaLab/internal/platform"
	"github.com/k33alexey/MetaLab/internal/secretstore"
	"github.com/k33alexey/MetaLab/internal/studio"
	"github.com/k33alexey/MetaLab/internal/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func runManager(ctx context.Context, configuration appconfig.Config) error {
	platformRuntime := platform.New(ctx, configuration, secretstore.New())
	defer platformRuntime.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start ML Manager UI: %w", err)
	}
	server := &http.Server{
		Handler: manager.NewHandlerWithPlatformAndStudio(configuration, platformRuntime, executableStudioLauncher{configurationPath: configuration.SourcePath, runtime: platformRuntime}), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.Serve(listener) }()

	app := application.New(application.Options{
		Name:        "MetaLab",
		Description: "ML Manager",
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "ML Manager", Width: 900, Height: 640, MinWidth: 560, MinHeight: 480,
		BackgroundColour: application.NewRGB(15, 20, 29),
		URL:              "http://" + listener.Addr().String(),
	})
	window.Center()
	window.Show()
	app.OnShutdown(func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	})
	go func() {
		<-ctx.Done()
		app.Quit()
	}()
	if err := app.Run(); err != nil {
		return fmt.Errorf("run ML Manager: %w", err)
	}
	if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve ML Manager UI: %w", err)
	}
	return nil
}

const (
	studioSessionIDEnvironment    = "ML_STUDIO_SESSION_ID"
	studioSessionTokenEnvironment = "ML_STUDIO_SESSION_TOKEN"
)

type executableStudioLauncher struct {
	configurationPath string
	runtime           *platform.Runtime
}

func (launcher executableStudioLauncher) CloneProject(ctx context.Context, repository, destination string) error {
	return gitclient.Clone(ctx, repository, destination)
}

func (launcher executableStudioLauncher) OpenStudio(ctx context.Context, databaseID uuid.UUID, projectPath string) error {
	workspace, err := studio.Open(projectPath)
	if err != nil {
		return fmt.Errorf("open ML Project: %w", err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		return fmt.Errorf("read ML Project: %w", err)
	}
	ownerName := "local-user"
	if current, currentErr := user.Current(); currentErr == nil && current.Username != "" {
		ownerName = current.Username
	}
	hostName := "localhost"
	if current, hostErr := os.Hostname(); hostErr == nil && current != "" {
		hostName = current
	}
	lease, err := launcher.runtime.AcquireStudioSession(ctx, databaseID, snapshot.Manifest.ID, ownerName, hostName, int64(os.Getpid()))
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		releaseStudioLaunchLease(ctx, launcher.runtime, lease)
		return fmt.Errorf("locate MetaLab executable: %w", err)
	}
	arguments := []string{"studio", "--database", databaseID.String(), "--project", projectPath}
	if launcher.configurationPath != "" {
		arguments = append(arguments, "--config", launcher.configurationPath)
	}
	command := exec.Command(executable, arguments...)
	command.Env = append(os.Environ(), studioSessionIDEnvironment+"="+lease.Session.ID.String(), studioSessionTokenEnvironment+"="+lease.Token)
	if err := command.Start(); err != nil {
		releaseStudioLaunchLease(ctx, launcher.runtime, lease)
		return fmt.Errorf("start ML Studio: %w", err)
	}
	_ = command.Process.Release()
	return nil
}

func releaseStudioLaunchLease(ctx context.Context, runtime *platform.Runtime, lease platform.StudioLease) {
	releaseContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = runtime.ReleaseStudioSession(releaseContext, lease.Session.ID, lease.Token)
}
