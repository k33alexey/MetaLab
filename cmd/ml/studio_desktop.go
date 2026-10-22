//go:build desktop && (darwin || windows)

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/host"
	"github.com/k33alexey/MetaLab/internal/platform"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/secretstore"
	"github.com/k33alexey/MetaLab/internal/studio"
	"github.com/k33alexey/MetaLab/internal/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// studioWindowTitle is what the window is called: the configuration's synonym
// in the project's own default language, and its name where no synonym was
// written down - a window titled "ML Studio — " and nothing else says less
// than the folder the project was opened from.
func studioWindowTitle(configuration project.Project) string {
	return configuration.Title.Presentation(configuration.DefaultLanguage, configuration.Name)
}

func runStudio(ctx context.Context, settings appconfig.Config, projectPath, databaseText string) error {
	databaseID, err := uuid.Parse(databaseText)
	if err != nil {
		return fmt.Errorf("invalid database identifier: %w", err)
	}
	workspace, err := studio.OpenForDatabase(projectPath, databaseID)
	if err != nil {
		return fmt.Errorf("open ML Project: %w", err)
	}
	settingsDirectory, err := studio.UserSettingsDirectory()
	if err != nil {
		return err
	}
	workspace.KeepUserSettingsIn(settingsDirectory)
	snapshot, err := workspace.Snapshot()
	if err != nil {
		return fmt.Errorf("read ML Project: %w", err)
	}
	platformRuntime := platform.New(ctx, settings, secretstore.New())
	defer platformRuntime.Close()
	lease, err := openStudioLease(ctx, platformRuntime, databaseID, snapshot.Configuration.ID)
	if err != nil {
		return err
	}
	defer host.ReleaseStudioLease(ctx, platformRuntime, lease)
	served, err := host.ServeStudio(platformRuntime, workspace, databaseID, lease)
	if err != nil {
		return err
	}

	app := application.New(application.Options{
		Name: "MetaLab Studio", Description: "ML Studio",
		Mac: application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "ML Studio — " + studioWindowTitle(snapshot.Configuration), Width: 1280, Height: 800,
		MinWidth: 800, MinHeight: 560, BackgroundColour: application.NewRGB(30, 31, 34),
		URL: served.URL,
	})
	window.Center()
	window.Show()
	heartbeatContext, stopHeartbeat := context.WithCancel(context.Background())
	heartbeatDone := make(chan error, 1)
	go func() { heartbeatDone <- host.KeepStudioLease(heartbeatContext, platformRuntime, lease, app.Quit) }()
	app.OnShutdown(func() {
		stopHeartbeat()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = served.Shutdown(shutdownContext)
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
	if err := served.Wait(); err != nil {
		return err
	}
	return heartbeatErr
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
	return platform.StudioLease{}, fmt.Errorf("open ML Studio from an authenticated ML Manager")
}
