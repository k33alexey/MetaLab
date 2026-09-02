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
	if len(migrations) != 2 {
		t.Fatalf("migration count = %d, want 2", len(migrations))
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
}
