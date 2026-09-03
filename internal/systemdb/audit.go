package systemdb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// AuditEvent is one structured, non-secret operational event.
type AuditEvent struct {
	ID         int64          `json:"id"`
	OccurredAt time.Time      `json:"occurredAt"`
	Level      string         `json:"level"`
	Code       string         `json:"code"`
	DatabaseID *uuid.UUID     `json:"databaseId,omitempty"`
	UserID     *uuid.UUID     `json:"userId,omitempty"`
	SessionID  *uuid.UUID     `json:"sessionId,omitempty"`
	Message    string         `json:"message"`
	Details    map[string]any `json:"details"`
}

// AuditRepository persists the platform registration journal.
type AuditRepository struct{ pool *pgxpool.Pool }

// Write appends one event. Details must already be free of secrets.
func (repository *AuditRepository) Write(ctx context.Context, event AuditEvent) (AuditEvent, error) {
	if event.Level != "info" && event.Level != "warning" && event.Level != "error" {
		return AuditEvent{}, fmt.Errorf("invalid audit level %q", event.Level)
	}
	event.Code, event.Message = strings.TrimSpace(event.Code), strings.TrimSpace(event.Message)
	if event.Code == "" || event.Message == "" || len(event.Code) > 128 || len(event.Message) > 4000 {
		return AuditEvent{}, fmt.Errorf("invalid audit event")
	}
	if event.Details == nil {
		event.Details = map[string]any{}
	}
	details, err := json.Marshal(event.Details)
	if err != nil || len(details) > 32<<10 {
		return AuditEvent{}, fmt.Errorf("invalid audit event details")
	}
	err = repository.pool.QueryRow(ctx, `
INSERT INTO ml_system.audit_events(level, event_code, database_id, user_id, session_id, message, details)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, occurred_at`, event.Level, event.Code, optionalAuditUUID(event.DatabaseID),
		optionalAuditUUID(event.UserID), optionalAuditUUID(event.SessionID), event.Message, details,
	).Scan(&event.ID, &event.OccurredAt)
	if err != nil {
		return AuditEvent{}, fmt.Errorf("write audit event: %w", err)
	}
	return event, nil
}

// List returns the newest events, optionally restricted to one database.
func (repository *AuditRepository) List(ctx context.Context, databaseID *uuid.UUID, limit int) ([]AuditEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var database any
	if databaseID != nil {
		database = databaseID.String()
	}
	rows, err := repository.pool.Query(ctx, `
SELECT id, occurred_at, level, event_code,
       COALESCE(database_id::text, ''), COALESCE(user_id::text, ''), COALESCE(session_id::text, ''),
       message, details
FROM ml_system.audit_events
WHERE $1::uuid IS NULL OR database_id = $1
ORDER BY occurred_at DESC, id DESC LIMIT $2`, database, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()
	items := make([]AuditEvent, 0)
	for rows.Next() {
		var item AuditEvent
		var databaseText, userText, sessionText string
		var details []byte
		if err := rows.Scan(&item.ID, &item.OccurredAt, &item.Level, &item.Code,
			&databaseText, &userText, &sessionText, &item.Message, &details); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		if err := parseOptionalAuditUUID(databaseText, &item.DatabaseID); err != nil {
			return nil, err
		}
		if err := parseOptionalAuditUUID(userText, &item.UserID); err != nil {
			return nil, err
		}
		if err := parseOptionalAuditUUID(sessionText, &item.SessionID); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(details, &item.Details); err != nil {
			return nil, fmt.Errorf("decode audit details: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit events: %w", err)
	}
	return items, nil
}

func optionalAuditUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return id.String()
}

func parseOptionalAuditUUID(value string, target **uuid.UUID) error {
	if value == "" {
		return nil
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return fmt.Errorf("parse audit identifier: %w", err)
	}
	*target = &id
	return nil
}
