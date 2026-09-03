//go:build desktop && (darwin || windows)

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/user"
	"time"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/platform"
	"github.com/k33alexey/MetaLab/internal/secretstore"
	"github.com/k33alexey/MetaLab/internal/studio"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func runStudio(ctx context.Context, configuration appconfig.Config, projectPath, databaseText string) error {
	databaseID, err := uuid.Parse(databaseText)
	if err != nil {
		return fmt.Errorf("invalid database identifier: %w", err)
	}
	workspace, err := studio.Open(projectPath)
	if err != nil {
		return fmt.Errorf("open ML Project: %w", err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		return fmt.Errorf("read ML Project: %w", err)
	}
	platformRuntime := platform.New(ctx, configuration, secretstore.New())
	defer platformRuntime.Close()
	lease, err := openStudioLease(ctx, platformRuntime, databaseID, snapshot.Manifest.ID)
	if err != nil {
		return err
	}
	defer func() {
		releaseContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = platformRuntime.ReleaseStudioSession(releaseContext, lease.Session.ID, lease.Token)
	}()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start ML Studio UI: %w", err)
	}
	server := &http.Server{
		Handler: studio.NewHandler(workspace), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.Serve(listener) }()

	app := application.New(application.Options{
		Name: "MetaLab Studio", Description: "ML Studio",
		Mac: application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "ML Studio — " + snapshot.Manifest.Title, Width: 1280, Height: 800,
		MinWidth: 800, MinHeight: 560, BackgroundColour: application.NewRGB(30, 31, 34),
		URL: "http://" + listener.Addr().String(),
	})
	window.Center()
	window.Show()
	heartbeatContext, stopHeartbeat := context.WithCancel(context.Background())
	heartbeatDone := make(chan error, 1)
	go monitorStudioLease(heartbeatContext, platformRuntime, lease, app, heartbeatDone)
	app.OnShutdown(func() {
		stopHeartbeat()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	})
	go func() {
		<-ctx.Done()
		app.Quit()
	}()
	if err := app.Run(); err != nil {
		stopHeartbeat()
		<-heartbeatDone
		return fmt.Errorf("run ML Studio: %w", err)
	}
	stopHeartbeat()
	heartbeatErr := <-heartbeatDone
	if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve ML Studio UI: %w", err)
	}
	if heartbeatErr != nil {
		return heartbeatErr
	}
	return nil
}

func openStudioLease(ctx context.Context, runtime *platform.Runtime, databaseID, projectID uuid.UUID) (platform.StudioLease, error) {
	sessionText, token := os.Getenv(studioSessionIDEnvironment), os.Getenv(studioSessionTokenEnvironment)
	if (sessionText == "") != (token == "") {
		return platform.StudioLease{}, fmt.Errorf("incomplete inherited ML Studio session")
	}
	if sessionText != "" {
		sessionID, err := uuid.Parse(sessionText)
		if err != nil {
			return platform.StudioLease{}, fmt.Errorf("invalid inherited ML Studio session: %w", err)
		}
		session, err := runtime.HeartbeatStudioSession(ctx, sessionID, token, int64(os.Getpid()))
		if err != nil {
			return platform.StudioLease{}, fmt.Errorf("resume ML Studio session: %w", err)
		}
		if session.DatabaseID != databaseID || session.ProjectID != projectID {
			return platform.StudioLease{}, fmt.Errorf("inherited ML Studio session does not match the database and project")
		}
		return platform.StudioLease{Token: token, Session: session}, nil
	}
	ownerName := "local-user"
	if current, err := user.Current(); err == nil && current.Username != "" {
		ownerName = current.Username
	}
	hostName := "localhost"
	if current, err := os.Hostname(); err == nil && current != "" {
		hostName = current
	}
	lease, err := runtime.AcquireStudioSession(ctx, databaseID, projectID, ownerName, hostName, int64(os.Getpid()))
	if err != nil {
		return platform.StudioLease{}, fmt.Errorf("acquire exclusive ML Studio session: %w", err)
	}
	return lease, nil
}

func monitorStudioLease(ctx context.Context, runtime *platform.Runtime, lease platform.StudioLease, app *application.App, done chan<- error) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-ctx.Done():
			done <- nil
			return
		case <-ticker.C:
			heartbeatContext, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, err := runtime.HeartbeatStudioSession(heartbeatContext, lease.Session.ID, lease.Token, int64(os.Getpid()))
			cancel()
			if err == nil {
				failures = 0
				continue
			}
			failures++
			if errors.Is(err, systemdb.ErrStudioSessionNotFound) || failures >= 2 {
				app.Quit()
				done <- fmt.Errorf("ML Studio session ended: %w", err)
				return
			}
		}
	}
}
