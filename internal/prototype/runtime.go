package prototype

import (
	"context"
	"fmt"
)

// Runtime owns the PostgreSQL pool and HTTP service of the prototype.
type Runtime struct {
	Store   *PostgresStore
	Service *Service
}

// OpenRuntime connects PostgreSQL, applies the prototype schema and creates the service.
func OpenRuntime(ctx context.Context, databaseURL string) (*Runtime, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("PostgreSQL connection string is empty")
	}
	store, err := OpenPostgresStore(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := store.Migrate(ctx); err != nil {
		store.Close()
		return nil, err
	}
	service, err := NewService(store)
	if err != nil {
		store.Close()
		return nil, err
	}
	return &Runtime{Store: store, Service: service}, nil
}

// Close releases runtime resources.
func (runtime *Runtime) Close() { runtime.Store.Close() }
