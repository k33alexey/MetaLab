//go:build desktop && (darwin || windows)

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/studio"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func runStudio(ctx context.Context, _ appconfig.Config, projectPath string) error {
	workspace, err := studio.Open(projectPath)
	if err != nil {
		return fmt.Errorf("open ML Project: %w", err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		return fmt.Errorf("read ML Project: %w", err)
	}
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
		return fmt.Errorf("run ML Studio: %w", err)
	}
	if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve ML Studio UI: %w", err)
	}
	return nil
}
