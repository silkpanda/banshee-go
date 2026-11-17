package store

import (
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *SettingsStore {
	t.Helper()
	db, err := OpenDatabase(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("OpenDatabase: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := ApplyMigrations(db, nil); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}
	return NewSettingsStore(db)
}

func TestSettingsRejectsUnknownOrSecretRefKeys(t *testing.T) {
	s := openTestDB(t)

	for _, key := range []string{"unknown_key", "device_token_secret_ref", "turing_token_secret_ref"} {
		if err := s.Update(key, "anything"); err == nil {
			t.Fatalf("expected Update(%q) to be rejected by allowlist", key)
		}
	}
}

func TestSettingsEnforcesHTTPSRelayURL(t *testing.T) {
	s := openTestDB(t)

	for _, bad := range []string{"http://relay.example.com", "ws://relay.test", "not-a-url", ""} {
		if err := s.Update("relay_url", bad); err == nil {
			t.Fatalf("expected relay_url %q to be rejected", bad)
		}
	}

	if err := s.Update("relay_url", "https://relay.example.com/"); err != nil {
		t.Fatalf("expected valid https relay_url to succeed: %v", err)
	}
	snap, err := s.GetAll()
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if snap.RelayURL != "https://relay.example.com" {
		t.Fatalf("expected trimmed https://relay.example.com, got %q", snap.RelayURL)
	}
}

func TestSettingsRoundTripsAndToleratesCorruptRows(t *testing.T) {
	s := openTestDB(t)

	if err := s.Update("sound_volume", 0.35); err != nil {
		t.Fatalf("Update sound_volume: %v", err)
	}
	if err := s.Update("event_type_filter", "people"); err != nil {
		t.Fatalf("Update event_type_filter: %v", err)
	}

	// Simulate corrupt JSON written directly to SQLite.
	if _, err := s.db.Exec(`INSERT OR REPLACE INTO settings (key, value, updated_at) VALUES ('relay_url', '{not-json', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("seed corrupt row: %v", err)
	}

	snap, err := s.GetAll()
	if err != nil {
		t.Fatalf("GetAll should fall back on corrupt rows, got error: %v", err)
	}
	if snap.RelayURL != defaultRelaySettingURL {
		t.Fatalf("expected corrupt relay_url to fall back to default %q, got %q", defaultRelaySettingURL, snap.RelayURL)
	}
	if snap.SoundVolume != 0.35 || snap.EventTypeFilter != "people" {
		t.Fatalf("unexpected snapshot values: %+v", snap)
	}
}
