//go:build desktop && (darwin || windows)

// Command mlprototype-desktop runs the full WebView architecture prototype.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/k33alexey/MetaLab/internal/prototype"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func main() {
	databaseURL := flag.String("database-url", os.Getenv("ML_DATABASE_URL"), "PostgreSQL connection string")
	smokeTimeout := flag.Duration("smoke-timeout", 0, "close automatically after a WebView smoke test")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	runtime, err := prototype.OpenRuntime(ctx, *databaseURL)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer runtime.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	server := &http.Server{
		Handler: runtime.Service.Handler(), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.Serve(listener) }()

	app := application.New(application.Options{
		Name:        "MetaLab",
		Description: "MetaLab architecture prototype",
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	url := "http://" + listener.Addr().String()
	if *smokeTimeout > 0 {
		url += "/?smoke=1"
	}
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "MetaLab", Width: 900, Height: 640, MinWidth: 560, MinHeight: 480,
		BackgroundColour: application.NewRGB(16, 20, 29),
		URL:              url,
	})
	window.Center()
	window.Show()
	app.OnShutdown(func() {
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownContext)
	})
	if *smokeTimeout > 0 {
		time.AfterFunc(*smokeTimeout, app.Quit)
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
