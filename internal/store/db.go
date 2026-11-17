package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Migration represents a single ordered, transactional schema step.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// DefaultMigrations defines Banshee's local SQLite schema, aligned with turing-api §7.3.
var DefaultMigrations = []Migration{
	{
		Version: 1,
		Name:    "0001-settings",
		SQL: `CREATE TABLE settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
	},
	{
		Version: 2,
		Name:    "0002-secrets",
		SQL: `CREATE TABLE secrets (
			name TEXT PRIMARY KEY,
			blob BLOB NOT NULL,
			rotated_at TEXT NOT NULL
		);`,
	},
	{
		Version: 3,
		Name:    "0003-alerts",
		SQL: `CREATE TABLE alerts (
			event_id TEXT PRIMARY KEY,
			seq INTEGER NOT NULL,
			event_type TEXT NOT NULL CHECK (event_type IN ('people', 'vehicle')),
			stages_received TEXT NOT NULL,
			camera_id INTEGER NOT NULL,
			camera_name TEXT NOT NULL,
			site_id INTEGER NOT NULL,
			site_name TEXT NOT NULL,
			occurred_at TEXT NOT NULL,
			received_at TEXT NOT NULL,
			snapshot_url TEXT,
			clip_url TEXT,
			metadata_summary TEXT,
			status TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'acknowledged', 'dismissed', 'snoozed')),
			acknowledged_at TEXT,
			dismissed_at TEXT,
			snooze_until TEXT,
			sound_played INTEGER NOT NULL DEFAULT 0,
			missed INTEGER NOT NULL DEFAULT 0
		);
		CREATE INDEX alerts_seq ON alerts (seq);`,
	},
}

const metaTableSQL = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY CHECK (version > 0),
	name TEXT NOT NULL,
	applied_at TEXT NOT NULL
);`

// DefaultDBPath returns %APPDATA%\banshee\banshee.sqlite on Windows (or user config dir elsewhere).
func DefaultDBPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "banshee", "banshee.sqlite"), nil
}

// OpenDatabase opens (creating parent folders if needed) a SQLite database file.
func OpenDatabase(dbPath string) (*sql.DB, error) {
	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON; PRAGMA busy_timeout = 5000;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configure sqlite pragmas: %w", err)
	}
	return db, nil
}

// ApplyMigrations applies pending migrations in ascending version order inside one transaction each.
func ApplyMigrations(db *sql.DB, extra []Migration) (int, error) {
	if _, err := db.Exec(metaTableSQL); err != nil {
		return 0, fmt.Errorf("create schema_migrations table: %w", err)
	}

	current, err := currentVersion(db)
	if err != nil {
		return 0, err
	}

	all := make([]Migration, 0, len(DefaultMigrations)+len(extra))
	all = append(all, DefaultMigrations...)
	all = append(all, extra...)

	applied := 0
	for _, m := range all {
		if m.Version <= current {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return applied, fmt.Errorf("begin migration %d (%s): %w", m.Version, m.Name, err)
		}
		if _, err := tx.Exec(m.SQL); err != nil {
			_ = tx.Rollback()
			return applied, fmt.Errorf("migration %d (%s) failed and rolled back: %w", m.Version, m.Name, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
			m.Version,
			m.Name,
			time.Now().UTC().Format(time.RFC3339Nano),
		); err != nil {
			_ = tx.Rollback()
			return applied, fmt.Errorf("record migration %d (%s): %w", m.Version, m.Name, err)
		}
		if err := tx.Commit(); err != nil {
			_ = tx.Rollback()
			return applied, fmt.Errorf("commit migration %d (%s): %w", m.Version, m.Name, err)
		}
		current = m.Version
		applied++
	}
	return applied, nil
}

func currentVersion(db *sql.DB) (int, error) {
	var v sql.NullInt64
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&v); err != nil {
		return 0, fmt.Errorf("read current schema version: %w", err)
	}
	if !v.Valid {
		return 0, nil
	}
	return int(v.Int64), nil
}
