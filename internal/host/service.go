// Package host runs MetaLab application modes independently of the CLI.
package host

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/prototype"
)

// RunService starts ML Service and blocks until cancellation or failure.
func RunService(ctx context.Context, configuration appconfig.Config) error {
	databaseURL := os.Getenv("ML_DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("ML_DATABASE_URL is required for service mode")
	}
	runtime, err := prototype.OpenRuntime(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer runtime.Close()

	listener, err := net.Listen("tcp", configuration.Service.Listen)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", configuration.Service.Listen, err)
	}
	server := &http.Server{
		Handler: runtime.Service.Handler(), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}
	finished := make(chan error, 1)
	go func() { finished <- server.Serve(listener) }()
	slog.Info("ML Service started", "address", "http://"+listener.Addr().String())

	select {
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shutdown ML Service: %w", err)
		}
		err := <-finished
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve ML Service: %w", err)
		}
		return nil
	case err := <-finished:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve ML Service: %w", err)
	}
}
