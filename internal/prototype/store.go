package prototype

import (
	"context"
	"time"
)

// Calculation is one persisted architecture-prototype operation.
type Calculation struct {
	ID        int64     `json:"id"`
	Left      float64   `json:"left"`
	Right     float64   `json:"right"`
	Result    float64   `json:"result"`
	CreatedAt time.Time `json:"createdAt"`
}

// Stats describes persisted prototype operations.
type Stats struct {
	Count      int64   `json:"count"`
	LastResult float64 `json:"lastResult"`
}

// Store is the persistence boundary used by the prototype service.
type Store interface {
	Migrate(context.Context) error
	Save(context.Context, Calculation) (Calculation, error)
	Stats(context.Context) (Stats, error)
	Ping(context.Context) error
}
