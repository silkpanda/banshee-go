package store

import (
	"path/filepath"
	"testing"
)

func TestApplyMigrationsCreatesSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "banshee.sqlite")
	db, err := OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}
	defer db.Close()

	applied, err := ApplyMigrations(db, nil)
	if err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}
	if applied != len(DefaultMigrations) {
		t.Fatalf("expected %d migrations applied, got %d", len(DefaultMigrations), applied)
	}

	for _, table := range []string{"settings", "secrets", "alerts", "schema_migrations"} {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Fatalf("expected table %q to exist: %v", table, err)
		}
	}
}

func TestApplyMigrationsIdempotentOnReopen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "banshee.sqlite")
	db, err := OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}
	if _, err := ApplyMigrations(db, nil); err != nil {
		t.Fatalf("initial ApplyMigrations failed: %v", err)
	}
	_ = db.Close()

	db2, err := OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("reopen OpenDatabase failed: %v", err)
	}
	defer db2.Close()

	applied, err := ApplyMigrations(db2, nil)
	if err != nil {
		t.Fatalf("second ApplyMigrations failed: %v", err)
	}
	if applied != 0 {
		t.Fatalf("expected 0 migrations on second run, got %d", applied)
	}
}

func TestApplyMigrationsRollsBackOnError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "banshee.sqlite")
	db, err := OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}
	defer db.Close()

	if _, err := ApplyMigrations(db, nil); err != nil {
		t.Fatalf("baseline ApplyMigrations failed: %v", err)
	}

	bad := []Migration{
		{
			Version: 99,
			Name:    "0099-broken",
			SQL:     `CREATE TABLE partial_should_rollback (id INTEGER); THIS IS INVALID SQL;`,
		},
	}

	if _, err := ApplyMigrations(db, bad); err == nil {
		t.Fatal("expected broken migration to fail, got nil")
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='partial_should_rollback'`).Scan(&count); err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected partial_should_rollback table to be rolled back, found %d", count)
	}
}
