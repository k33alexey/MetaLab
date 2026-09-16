package platform

import (
	"context"
	"fmt"

	"github.com/k33alexey/MetaLab/internal/publication"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// SaveApplicationData runs "Сохранить данные" (096) for one registered
// database: migrates PostgreSQL schema and refreshes the stored metadata/BSL
// snapshot directly from the live project directory - the package-free
// replacement for the retired BuildFile+Activate flow (021/024). Works for
// both Primary and Debug databases; Primary additionally requires the
// project directory to have no uncommitted Git changes (ErrDirtyPrimary).
func (runtime *Runtime) SaveApplicationData(ctx context.Context, databaseID uuid.UUID, root string, allowDestructive bool) (publication.SavedState, schemadiff.MigrationRecord, error) {
	pool, registered, err := runtime.openApplicationPool(ctx, databaseID)
	if err != nil {
		return publication.SavedState{}, schemadiff.MigrationRecord{}, err
	}
	defer pool.Close()
	mode := publication.ActivationPrimary
	if registered.Mode == systemdb.DatabaseDebug {
		mode = publication.ActivationDebug
	} else if registered.Mode != systemdb.DatabasePrimary {
		return publication.SavedState{}, schemadiff.MigrationRecord{}, fmt.Errorf("unsupported database mode %q", registered.Mode)
	}
	return publication.SaveData(ctx, pool, publication.SaveDataRequest{
		Root: root, Mode: mode, Confirmed: true, AllowDestructive: allowDestructive,
	})
}
