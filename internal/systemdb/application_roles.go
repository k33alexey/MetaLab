package systemdb

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// This storage bound matches migration 009. The metadata compiler additionally
// bounds the total number of effective object, field and command permissions.
const MaxApplicationRoles = 1024

var ErrApplicationRolesChanged = errors.New("application role assignment changed; reload it before saving")
var ErrInvalidApplicationRoles = errors.New("invalid application role assignment")

type ApplicationRoleAssignment struct {
	ProjectID *uuid.UUID  `json:"projectId,omitempty"`
	RoleIDs   []uuid.UUID `json:"roleIds"`
	Revision  int64       `json:"revision"`
}

// ApplicationRoles is a trusted server read for an already authenticated user,
// not an HTTP authorization boundary. It also rechecks active App membership.
// A missing selection returns no roles, including for system administrators.
func (repository *DatabaseAccessRepository) ApplicationRoles(ctx context.Context, userID, databaseID uuid.UUID) (ApplicationRoleAssignment, error) {
	var projectText string
	var roleTexts []string
	var assignment ApplicationRoleAssignment
	err := repository.pool.QueryRow(ctx, `
SELECT COALESCE(roles.project_id::text,''), COALESCE(roles.role_ids::text[],'{}'::text[]), COALESCE(roles.revision,0)
FROM ml_system.database_access AS access
JOIN ml_system.users AS users ON users.id=access.user_id
LEFT JOIN ml_system.application_role_assignments AS roles ON roles.user_id=access.user_id AND roles.database_id=access.database_id
WHERE access.user_id=$1 AND access.database_id=$2 AND access.revoked_at IS NULL AND access.app_access
  AND users.enabled AND NOT users.must_change_password`, userID.String(), databaseID.String()).Scan(&projectText, &roleTexts, &assignment.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return ApplicationRoleAssignment{}, ErrDatabaseAccessDenied
	}
	if err != nil {
		return ApplicationRoleAssignment{}, fmt.Errorf("read application roles: %w", err)
	}
	return parseApplicationRoleAssignment(assignment, projectText, roleTexts)
}

// GetApplicationRoles is an administrator-only read, also usable while the
// target user still has a temporary password. Authority is checked in ML System.
func (repository *DatabaseAccessRepository) GetApplicationRoles(ctx context.Context, actorID, userID, databaseID uuid.UUID) (ApplicationRoleAssignment, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return ApplicationRoleAssignment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	allowed, err := canManageDatabaseAccess(ctx, tx, actorID, databaseID)
	if err != nil {
		return ApplicationRoleAssignment{}, err
	}
	if !allowed {
		return ApplicationRoleAssignment{}, ErrDatabaseAccessDenied
	}
	if err := requireRoleRecipient(ctx, tx, userID, databaseID); err != nil {
		return ApplicationRoleAssignment{}, err
	}
	return readApplicationRoles(ctx, tx, userID, databaseID)
}

// SetApplicationRoles atomically replaces a selection, records the change and
// terminates only this user's sessions in this database. The caller must first
// validate IDs against the active publication; storage cannot resolve YAML.
func (repository *DatabaseAccessRepository) SetApplicationRoles(ctx context.Context, actorID, userID, databaseID, projectID uuid.UUID, roleIDs []uuid.UUID, expectedRevision int64) (ApplicationRoleAssignment, error) {
	roles, err := canonicalApplicationRoles(projectID, roleIDs, expectedRevision)
	if err != nil {
		return ApplicationRoleAssignment{}, err
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return ApplicationRoleAssignment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockDatabasePermissions(ctx, tx, databaseID); err != nil {
		return ApplicationRoleAssignment{}, err
	}
	// Serialize with account disabling/demotion, not just database grant edits.
	var actor string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM ml_system.users WHERE id=$1 FOR SHARE`, actorID.String()).Scan(&actor); errors.Is(err, pgx.ErrNoRows) {
		return ApplicationRoleAssignment{}, ErrDatabaseAccessDenied
	} else if err != nil {
		return ApplicationRoleAssignment{}, err
	}
	allowed, err := canManageDatabaseAccess(ctx, tx, actorID, databaseID)
	if err != nil {
		return ApplicationRoleAssignment{}, err
	}
	if !allowed {
		return ApplicationRoleAssignment{}, ErrDatabaseOwnerOnly
	}
	if err := requireRoleRecipient(ctx, tx, userID, databaseID); err != nil {
		return ApplicationRoleAssignment{}, err
	}
	previous, err := readApplicationRoles(ctx, tx, userID, databaseID)
	if err != nil {
		return ApplicationRoleAssignment{}, err
	}
	if previous.Revision != expectedRevision {
		return ApplicationRoleAssignment{}, ErrApplicationRolesChanged
	}
	if previous.ProjectID != nil && *previous.ProjectID == projectID && slices.Equal(previous.RoleIDs, roles) {
		return previous, nil
	}
	texts := make([]string, len(roles))
	for index, id := range roles {
		texts[index] = id.String()
	}
	var revision int64
	err = tx.QueryRow(ctx, `
INSERT INTO ml_system.application_role_assignments(user_id,database_id,project_id,role_ids,updated_by_user_id)
VALUES($1,$2,$3,$4::uuid[],$5)
ON CONFLICT(user_id,database_id) DO UPDATE SET project_id=$3,role_ids=$4::uuid[],updated_by_user_id=$5,
 revision=ml_system.application_role_assignments.revision+1,updated_at=clock_timestamp()
RETURNING revision`, userID.String(), databaseID.String(), projectID.String(), texts, actorID.String()).Scan(&revision)
	if err != nil {
		return ApplicationRoleAssignment{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE ml_system.database_sessions SET terminated_at=clock_timestamp()
WHERE database_id=$2 AND terminated_at IS NULL AND portal_session_id IN (SELECT id FROM ml_system.portal_sessions WHERE user_id=$1)`, userID.String(), databaseID.String()); err != nil {
		return ApplicationRoleAssignment{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO ml_system.audit_events(level,event_code,database_id,user_id,message,details)
VALUES('info','application.roles_changed',$1,$2,'Application roles updated',jsonb_build_object('userId',$3::text,'projectId',$4::text,'roleIds',$5::text[],'revision',$6::bigint))`,
		databaseID.String(), actorID.String(), userID.String(), projectID.String(), texts, revision); err != nil {
		return ApplicationRoleAssignment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ApplicationRoleAssignment{}, err
	}
	return ApplicationRoleAssignment{ProjectID: &projectID, RoleIDs: roles, Revision: revision}, nil
}

func canonicalApplicationRoles(projectID uuid.UUID, ids []uuid.UUID, revision int64) ([]uuid.UUID, error) {
	if projectID.IsZero() || len(ids) > MaxApplicationRoles || revision < 0 {
		return nil, ErrInvalidApplicationRoles
	}
	result := append([]uuid.UUID{}, ids...)
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	for index, id := range result {
		if id.IsZero() || index > 0 && result[index-1] == id {
			return nil, ErrInvalidApplicationRoles
		}
	}
	return result, nil
}

func requireRoleRecipient(ctx context.Context, tx pgx.Tx, userID, databaseID uuid.UUID) error {
	var id string
	err := tx.QueryRow(ctx, `SELECT users.id::text FROM ml_system.users AS users
JOIN ml_system.database_access AS access ON access.user_id=users.id
WHERE users.id=$1 AND users.enabled AND access.database_id=$2 AND access.revoked_at IS NULL AND access.app_access
FOR SHARE OF users, access`, userID.String(), databaseID.String()).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDatabaseAccessDenied
	}
	return err
}

func readApplicationRoles(ctx context.Context, tx pgx.Tx, userID, databaseID uuid.UUID) (ApplicationRoleAssignment, error) {
	var assignment ApplicationRoleAssignment
	var projectText string
	var roleTexts []string
	err := tx.QueryRow(ctx, `SELECT project_id::text,role_ids::text[],revision FROM ml_system.application_role_assignments WHERE user_id=$1 AND database_id=$2`, userID.String(), databaseID.String()).Scan(&projectText, &roleTexts, &assignment.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return ApplicationRoleAssignment{RoleIDs: []uuid.UUID{}}, nil
	}
	if err != nil {
		return ApplicationRoleAssignment{}, err
	}
	return parseApplicationRoleAssignment(assignment, projectText, roleTexts)
}

func parseApplicationRoleAssignment(assignment ApplicationRoleAssignment, projectText string, roleTexts []string) (ApplicationRoleAssignment, error) {
	if projectText != "" {
		id, err := uuid.Parse(projectText)
		if err != nil {
			return ApplicationRoleAssignment{}, err
		}
		assignment.ProjectID = &id
	}
	assignment.RoleIDs = make([]uuid.UUID, len(roleTexts))
	for index, text := range roleTexts {
		id, err := uuid.Parse(text)
		if err != nil {
			return ApplicationRoleAssignment{}, err
		}
		assignment.RoleIDs[index] = id
	}
	return assignment, nil
}
