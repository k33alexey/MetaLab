package host

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/k33alexey/MetaLab/internal/platform"
	"github.com/k33alexey/MetaLab/internal/publication"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/studio"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// StudioServer is ML Studio's interface for one database on a local port: the
// workspace saves data and reads saved names through the platform, and every
// request is checked against the Studio lease. The window of the desktop build
// and the test stand both open URL; neither knows how the two are wired.
type StudioServer struct {
	// URL is the one-time launch address: it is good for one open within a
	// minute, as studio.NewAuthorizedHandler issues it.
	URL    string
	server *http.Server
	served chan error
}

// ServeStudio binds workspace to databaseID and starts serving it.
func ServeStudio(runtime *platform.Runtime, workspace *studio.Workspace, databaseID uuid.UUID, lease platform.StudioLease) (*StudioServer, error) {
	workspace.SetSavedNamesProvider(func(namesContext context.Context) map[string]string {
		return runtime.ApplicationPhysicalNames(namesContext, databaseID)
	})
	workspace.SetSaveDataProvider(func(saveContext context.Context, root string, consent schemadiff.MigrationConsent) (publication.SavedState, schemadiff.MigrationRecord, error) {
		return runtime.SaveApplicationData(saveContext, databaseID, root, consent)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start ML Studio UI: %w", err)
	}
	handler, launchPath, err := studio.NewAuthorizedHandler(workspace, listener.Addr().String(), func(requestContext context.Context) error {
		return runtime.AuthorizeStudioLease(requestContext, lease)
	})
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	served := &StudioServer{
		URL: "http://" + listener.Addr().String() + launchPath,
		server: &http.Server{
			Handler: handler, ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
		},
		served: make(chan error, 1),
	}
	go func() { served.served <- served.server.Serve(listener) }()
	return served, nil
}

// Shutdown stops serving and waits for the requests in flight.
func (served *StudioServer) Shutdown(ctx context.Context) error {
	return served.server.Shutdown(ctx)
}

// Wait returns how serving ended once it has; a shutdown is not a failure.
func (served *StudioServer) Wait() error {
	if err := <-served.served; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve ML Studio UI: %w", err)
	}
	return nil
}

// KeepStudioLease renews lease every ten seconds until ctx ends. When the
// lease is gone - terminated from ML Manager - or two renewals in a row fail,
// it calls end and returns why; when ctx ends first, it returns nil.
func KeepStudioLease(ctx context.Context, runtime *platform.Runtime, lease platform.StudioLease, end func()) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return nil
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
				end()
				return fmt.Errorf("ML Studio session ended: %w", err)
			}
		}
	}
}

// ReleaseStudioLease gives the lease back, even when ctx is already cancelled.
func ReleaseStudioLease(ctx context.Context, runtime *platform.Runtime, lease platform.StudioLease) {
	releaseContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = runtime.ReleaseStudioSession(releaseContext, lease.Session.ID, lease.Token)
}
