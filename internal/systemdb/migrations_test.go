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
	if len(migrations) != 1 {
		t.Fatalf("migration count = %d, want 1", len(migrations))
	}
	migration := migrations[0]
	if migration.version != 1 || migration.name != "settings" || len(migration.checksum) != 64 {
		t.Fatalf("migration = %+v", migration)
	}
	if !strings.Contains(migration.sql, "CREATE TABLE ml_system.settings") {
		t.Fatalf("migration SQL = %q", migration.sql)
	}
}
