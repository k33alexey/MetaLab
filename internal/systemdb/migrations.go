package systemdb

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

const migrationLockID int64 = 0x4d4c53595354454d // "MLSYSTEM"

//go:embed migrations/*.sql
var migrationFiles embed.FS

type migration struct {
	version  int64
	name     string
	checksum string
	sql      string
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded ML System migrations: %w", err)
	}
	migrations := make([]migration, 0, len(entries))
	versions := make(map[int64]struct{}, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		parts := strings.SplitN(strings.TrimSuffix(entry.Name(), ".sql"), "_", 2)
		if len(parts) != 2 || parts[1] == "" {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		version, parseErr := strconv.ParseInt(parts[0], 10, 64)
		if parseErr != nil || version <= 0 {
			return nil, fmt.Errorf("invalid migration version in %q", entry.Name())
		}
		if _, exists := versions[version]; exists {
			return nil, fmt.Errorf("duplicate migration version %d", version)
		}
		content, readErr := migrationFiles.ReadFile("migrations/" + entry.Name())
		if readErr != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), readErr)
		}
		digest := sha256.Sum256(content)
		migrations = append(migrations, migration{
			version: version, name: parts[1], checksum: hex.EncodeToString(digest[:]), sql: string(content),
		})
		versions[version] = struct{}{}
	}
	sort.Slice(migrations, func(left, right int) bool { return migrations[left].version < migrations[right].version })
	return migrations, nil
}

func migrate(ctx context.Context, connection *pgx.Conn) error {
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	return applyMigrations(ctx, connection, migrations)
}

func applyMigrations(ctx context.Context, connection *pgx.Conn, migrations []migration) error {
	transaction, err := connection.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin ML System migration: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	if _, err = transaction.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("lock ML System migrations: %w", err)
	}
	if _, err = transaction.Exec(ctx, `
CREATE SCHEMA IF NOT EXISTS ml_system;
CREATE TABLE IF NOT EXISTS ml_system.schema_migrations (
    version BIGINT PRIMARY KEY,
    name TEXT NOT NULL,
    checksum TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
)`); err != nil {
		return fmt.Errorf("bootstrap ML System migrations: %w", err)
	}

	applied, err := appliedMigrations(ctx, transaction)
	if err != nil {
		return err
	}
	known := make(map[int64]migration, len(migrations))
	for _, item := range migrations {
		known[item.version] = item
		if checksum, exists := applied[item.version]; exists {
			if checksum != item.checksum {
				return fmt.Errorf("ML System migration %d checksum differs from the applied migration", item.version)
			}
			continue
		}
		if _, err = transaction.Exec(ctx, item.sql); err != nil {
			return fmt.Errorf("apply ML System migration %03d_%s: %w", item.version, item.name, err)
		}
		if _, err = transaction.Exec(ctx,
			"INSERT INTO ml_system.schema_migrations(version, name, checksum) VALUES ($1, $2, $3)",
			item.version, item.name, item.checksum,
		); err != nil {
			return fmt.Errorf("record ML System migration %d: %w", item.version, err)
		}
	}
	for version := range applied {
		if _, exists := known[version]; !exists {
			return fmt.Errorf("ML System database has unsupported migration %d", version)
		}
	}
	if err = transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit ML System migrations: %w", err)
	}
	return nil
}

func appliedMigrations(ctx context.Context, transaction pgx.Tx) (map[int64]string, error) {
	rows, err := transaction.Query(ctx, "SELECT version, checksum FROM ml_system.schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("read ML System migrations: %w", err)
	}
	defer rows.Close()
	applied := make(map[int64]string)
	for rows.Next() {
		var version int64
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, fmt.Errorf("scan ML System migration: %w", err)
		}
		applied[version] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ML System migrations: %w", err)
	}
	return applied, nil
}
