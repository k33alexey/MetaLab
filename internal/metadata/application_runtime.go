package metadata

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// NewApplicationRuntime connects every currently supported metadata repository
// to one application database pool.
func NewApplicationRuntime(pool *pgxpool.Pool, catalog *Catalog, actor *uuid.UUID) (*Runtime, error) {
	if pool == nil || catalog == nil {
		return nil, fmt.Errorf("application runtime requires PostgreSQL and metadata catalog")
	}
	constants, err := NewConstantRepository(pool, catalog)
	if err != nil {
		return nil, err
	}
	catalogs, err := NewCatalogRepository(pool, catalog)
	if err != nil {
		return nil, err
	}
	documents, err := NewDocumentRepository(pool, catalog)
	if err != nil {
		return nil, err
	}
	informationRegisters, err := NewInformationRegisterRepository(pool, catalog)
	if err != nil {
		return nil, err
	}
	accumulationRegisters, err := NewAccumulationRegisterRepository(pool, catalog)
	if err != nil {
		return nil, err
	}
	return NewRuntimeWithAllRegisters(constants, catalogs, documents, informationRegisters, accumulationRegisters, catalog, actor)
}
