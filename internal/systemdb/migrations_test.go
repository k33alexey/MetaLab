package systemdb

import (
	"strings"
	"testing"
)

func TestEmbeddedMigrationsAreOrderedAndChecksummed(t *testing.T) {
	t.Parallel()

	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 11 {
		t.Fatalf("migration count = %d, want 11", len(migrations))
	}
	migration := migrations[0]
	if migration.version != 1 || migration.name != "settings" || len(migration.checksum) != 64 {
		t.Fatalf("migration = %+v", migration)
	}
	if !strings.Contains(migration.sql, "CREATE TABLE ml_system.settings") {
		t.Fatalf("migration SQL = %q", migration.sql)
	}
	if migrations[1].version != 2 || migrations[1].name != "users" || !strings.Contains(migrations[1].sql, "CREATE TABLE ml_system.users") {
		t.Fatalf("second migration = %+v", migrations[1])
	}
	if migrations[2].version != 3 || migrations[2].name != "databases" || !strings.Contains(migrations[2].sql, "CREATE TABLE ml_system.databases") {
		t.Fatalf("third migration = %+v", migrations[2])
	}
	if migrations[3].version != 4 || migrations[3].name != "database_modes" || !strings.Contains(migrations[3].sql, "ADD COLUMN mode") {
		t.Fatalf("fourth migration = %+v", migrations[3])
	}
	if migrations[4].version != 5 || migrations[4].name != "operations" || !strings.Contains(migrations[4].sql, "CREATE TABLE ml_system.portal_sessions") {
		t.Fatalf("fifth migration = %+v", migrations[4])
	}
	if migrations[5].version != 6 || migrations[5].name != "studio_sessions" || !strings.Contains(migrations[5].sql, "CREATE TABLE ml_system.studio_sessions") {
		t.Fatalf("sixth migration = %+v", migrations[5])
	}
	if migrations[6].version != 7 || migrations[6].name != "database_access" || !strings.Contains(migrations[6].sql, "CREATE TABLE ml_system.database_access") {
		t.Fatalf("seventh migration = %+v", migrations[6])
	}
	if migrations[7].version != 8 || migrations[7].name != "manager_access" {
		t.Fatalf("eighth migration = %+v", migrations[7])
	}
	if migrations[8].version != 9 || migrations[8].name != "application_roles" {
		t.Fatalf("ninth migration = %+v", migrations[8])
	}
	if migrations[9].version != 10 || migrations[9].name != "database_session_user_scope" || !strings.Contains(migrations[9].sql, "database_sessions_one_active_idx") {
		t.Fatalf("tenth migration = %+v", migrations[9])
	}
	if migrations[10].version != 11 || migrations[10].name != "single_portal_session" || !strings.Contains(migrations[10].sql, "portal_sessions_one_active_idx") {
		t.Fatalf("eleventh migration = %+v", migrations[10])
	}
}
