package systemdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrSettingNotFound means that an ML System setting does not exist.
var ErrSettingNotFound = errors.New("ML System setting not found")

// Setting is one namespaced, revisioned platform setting.
type Setting struct {
	Key       string
	Value     json.RawMessage
	Revision  int64
	UpdatedAt time.Time
}

// SettingsRepository persists non-secret ML Service settings as JSON.
type SettingsRepository struct {
	pool *pgxpool.Pool
}

// Set creates or atomically updates a setting and increments its revision.
func (repository *SettingsRepository) Set(ctx context.Context, key string, value json.RawMessage) (Setting, error) {
	if err := validateSetting(key, value); err != nil {
		return Setting{}, err
	}
	var setting Setting
	err := repository.pool.QueryRow(ctx, `
INSERT INTO ml_system.settings(key, value)
VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE SET
    value = EXCLUDED.value,
    revision = ml_system.settings.revision + 1,
    updated_at = clock_timestamp()
RETURNING key, value, revision, updated_at`, key, string(value)).Scan(
		&setting.Key, &setting.Value, &setting.Revision, &setting.UpdatedAt,
	)
	if err != nil {
		return Setting{}, fmt.Errorf("save ML System setting %q: %w", key, err)
	}
	return setting, nil
}

// Get returns a setting by its exact key.
func (repository *SettingsRepository) Get(ctx context.Context, key string) (Setting, error) {
	if err := validateSettingKey(key); err != nil {
		return Setting{}, err
	}
	var setting Setting
	err := repository.pool.QueryRow(ctx, `
SELECT key, value, revision, updated_at
FROM ml_system.settings
WHERE key = $1`, key).Scan(&setting.Key, &setting.Value, &setting.Revision, &setting.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Setting{}, fmt.Errorf("%w: %s", ErrSettingNotFound, key)
	}
	if err != nil {
		return Setting{}, fmt.Errorf("read ML System setting %q: %w", key, err)
	}
	return setting, nil
}

// Delete removes a setting and reports whether it existed.
func (repository *SettingsRepository) Delete(ctx context.Context, key string) (bool, error) {
	if err := validateSettingKey(key); err != nil {
		return false, err
	}
	result, err := repository.pool.Exec(ctx, "DELETE FROM ml_system.settings WHERE key = $1", key)
	if err != nil {
		return false, fmt.Errorf("delete ML System setting %q: %w", key, err)
	}
	return result.RowsAffected() == 1, nil
}

func validateSetting(key string, value json.RawMessage) error {
	if err := validateSettingKey(key); err != nil {
		return err
	}
	if len(value) == 0 || !json.Valid(value) {
		return fmt.Errorf("ML System setting %q must contain valid JSON", key)
	}
	return nil
}

func validateSettingKey(key string) error {
	if key == "" || key != strings.TrimSpace(key) || len(key) > 200 {
		return fmt.Errorf("invalid ML System setting key %q", key)
	}
	for _, part := range strings.Split(key, ".") {
		if part == "" {
			return fmt.Errorf("invalid ML System setting key %q", key)
		}
		for _, symbol := range part {
			if symbol != '_' && symbol != '-' && (symbol < 'a' || symbol > 'z') && (symbol < '0' || symbol > '9') {
				return fmt.Errorf("invalid ML System setting key %q", key)
			}
		}
	}
	return nil
}
