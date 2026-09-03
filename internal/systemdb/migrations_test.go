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
	if len(migrations) != 6 {
		t.Fatalf("migration count = %d, want 6", len(migrations))
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
}
