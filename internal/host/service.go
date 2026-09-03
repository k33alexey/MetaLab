// Package host runs MetaLab application modes independently of the CLI.
package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/platform"
	"github.com/k33alexey/MetaLab/internal/portal"
	"github.com/k33alexey/MetaLab/internal/prototype"
	"github.com/k33alexey/MetaLab/internal/secretstore"
)

// RunService starts ML Service and blocks until cancellation or failure.
func RunService(ctx context.Context, configuration appconfig.Config) error {
	handler, closeRuntime, err := buildHandler(ctx, configuration, secretstore.New())
	if err != nil {
		slog.Error("ML Service started in degraded mode", "error", err)
	}
	defer closeRuntime()

	listener, err := net.Listen("tcp", configuration.Service.Listen)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", configuration.Service.Listen, err)
	}
	server := &http.Server{
		Handler: handler, ReadHeaderTimeout: 5 * time.Second,
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

func buildHandler(ctx context.Context, configuration appconfig.Config, secrets platform.Secrets) (http.Handler, func(), error) {
	if configuration.SystemDatabase != nil || os.Getenv("ML_SYSTEM_DATABASE_URL") != "" {
		runtime := platform.New(ctx, configuration, secrets)
		state := runtime.State()
		if !state.Connected {
			runtime.Close()
			if state.Error == "" {
				return degradedHandler(), func() {}, fmt.Errorf("ML System PostgreSQL is unavailable")
			}
			return degradedHandler(), func() {}, errors.New(state.Error)
		}
		return portal.NewHandler(runtime), runtime.Close, nil
	}
	databaseURL := os.Getenv("ML_DATABASE_URL")
	if databaseURL == "" {
		return degradedHandler(), func() {}, nil
	}
	runtime, err := prototype.OpenRuntime(ctx, databaseURL)
	if err != nil {
		return nil, nil, err
	}
	return runtime.Service.Handler(), runtime.Close, nil
}

func degradedHandler() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("GET /api/health", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(response).Encode(map[string]string{"status": "degraded", "database": "unconfigured"})
	})
	return routes
}
