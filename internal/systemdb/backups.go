package systemdb

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

var ErrBackupNotFound = errors.New("backup not found")

// Backup describes a verified local PostgreSQL archive.
type Backup struct {
	ID         uuid.UUID `json:"id"`
	DatabaseID uuid.UUID `json:"databaseId"`
	FileName   string    `json:"fileName"`
	SizeBytes  int64     `json:"sizeBytes"`
	SHA256     string    `json:"sha256"`
	CreatedAt  time.Time `json:"createdAt"`
}

// BackupRepository stores the catalog; archive bytes remain in the configured local directory.
type BackupRepository struct{ pool *pgxpool.Pool }

// Add registers a successfully completed and hashed archive.
func (repository *BackupRepository) Add(ctx context.Context, backup Backup) (Backup, error) {
	checksum, checksumErr := hex.DecodeString(backup.SHA256)
	if backup.ID.IsZero() || backup.DatabaseID.IsZero() || filepath.Base(backup.FileName) != backup.FileName ||
		strings.TrimSpace(backup.FileName) == "" || backup.SizeBytes < 0 || len(backup.SHA256) != 64 {
		return Backup{}, fmt.Errorf("invalid backup metadata")
	}
	if checksumErr != nil || len(checksum) != 32 {
		return Backup{}, fmt.Errorf("invalid backup checksum")
	}
	err := repository.pool.QueryRow(ctx, `
INSERT INTO ml_system.backups(id, database_id, file_name, size_bytes, sha256)
VALUES ($1, $2, $3, $4, $5)
RETURNING created_at`, backup.ID.String(), backup.DatabaseID.String(), backup.FileName,
		backup.SizeBytes, backup.SHA256).Scan(&backup.CreatedAt)
	if err != nil {
		return Backup{}, fmt.Errorf("register backup: %w", err)
	}
	return backup, nil
}

// Get returns one catalog item.
func (repository *BackupRepository) Get(ctx context.Context, id uuid.UUID) (Backup, error) {
	return scanBackup(repository.pool.QueryRow(ctx, `
SELECT id::text, database_id::text, file_name, size_bytes, sha256, created_at
FROM ml_system.backups WHERE id = $1`, id.String()))
}

// List returns archives of one database newest first.
func (repository *BackupRepository) List(ctx context.Context, databaseID uuid.UUID) ([]Backup, error) {
	rows, err := repository.pool.Query(ctx, `
SELECT id::text, database_id::text, file_name, size_bytes, sha256, created_at
FROM ml_system.backups WHERE database_id = $1 ORDER BY created_at DESC`, databaseID.String())
	if err != nil {
		return nil, fmt.Errorf("list backups: %w", err)
	}
	defer rows.Close()
	items := make([]Backup, 0)
	for rows.Next() {
		item, scanErr := scanBackup(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// Delete removes one catalog entry and returns it for coordinated file cleanup.
func (repository *BackupRepository) Delete(ctx context.Context, id uuid.UUID) (Backup, error) {
	backup, err := scanBackup(repository.pool.QueryRow(ctx, `
DELETE FROM ml_system.backups WHERE id = $1
RETURNING id::text, database_id::text, file_name, size_bytes, sha256, created_at`, id.String()))
	return backup, err
}

func scanBackup(row rowScanner) (Backup, error) {
	var backup Backup
	var id, databaseID string
	if err := row.Scan(&id, &databaseID, &backup.FileName, &backup.SizeBytes, &backup.SHA256, &backup.CreatedAt); errors.Is(err, pgx.ErrNoRows) {
		return Backup{}, ErrBackupNotFound
	} else if err != nil {
		return Backup{}, fmt.Errorf("scan backup: %w", err)
	}
	var err error
	backup.ID, err = uuid.Parse(id)
	if err == nil {
		backup.DatabaseID, err = uuid.Parse(databaseID)
	}
	if err != nil {
		return Backup{}, fmt.Errorf("parse backup identifier: %w", err)
	}
	return backup, nil
}
