//go:build desktop && (darwin || windows)

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/manager"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func runManager(ctx context.Context, configuration appconfig.Config) error {
	var systemDatabase *systemdb.Database
	if databaseURL := os.Getenv("ML_SYSTEM_DATABASE_URL"); databaseURL != "" {
		var err error
		systemDatabase, err = systemdb.Open(ctx, databaseURL)
		if err != nil {
			return fmt.Errorf("open ML System for Manager: %w", err)
		}
		defer systemDatabase.Close()
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start ML Manager UI: %w", err)
	}
	handler := manager.NewHandler(configuration)
	if systemDatabase != nil {
		handler = manager.NewHandlerWithSetup(configuration, systemDatabase.Administrators)
	}
	server := &http.Server{
		Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
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
