// Command mlprototype runs the headless service used by architecture iteration 006.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/k33alexey/MetaLab/internal/prototype"
)

func main() {
	os.Exit(run())
}

func run() int {
	flags := flag.NewFlagSet("mlprototype", flag.ContinueOnError)
	listen := flags.String("listen", "127.0.0.1:8090", "HTTP listen address")
	databaseURL := flags.String("database-url", os.Getenv("ML_DATABASE_URL"), "PostgreSQL connection string")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runtime, err := prototype.OpenRuntime(ctx, *databaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer runtime.Close()

	server := &http.Server{
		Addr: *listen, Handler: runtime.Service.Handler(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}
	finished := make(chan error, 1)
	go func() {
		slog.Info("MetaLab architecture prototype started", "address", "http://"+*listen)
		finished <- server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case err := <-finished:
		if !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
}
