package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const defaultRelaySettingURL = "https://relay.example.com"

// SettingsSnapshot holds the typed, non-secret settings exposed to the UI.
type SettingsSnapshot struct {
	RelayURL        string  `json:"relayUrl"`
	SoundVolume     float64 `json:"soundVolume"`
	SoundMuted      bool    `json:"soundMuted"`
	EventTypeFilter string  `json:"eventTypeFilter"`
	AutoStart       bool    `json:"autoStart"`
}

// SettingsStore manages validated, allowlisted settings in SQLite.
type SettingsStore struct {
	db *sql.DB
}

// NewSettingsStore wraps an already-migrated SQLite database.
func NewSettingsStore(db *sql.DB) *SettingsStore {
	return &SettingsStore{db: db}
}

func defaultSnapshot() SettingsSnapshot {
	return SettingsSnapshot{
		RelayURL:        defaultRelaySettingURL,
		SoundVolume:     0.75,
		SoundMuted:      false,
		EventTypeFilter: "all",
		AutoStart:       false,
	}
}

// GetAll returns every setting, falling back to safe defaults if unset or corrupt.
func (s *SettingsStore) GetAll() (SettingsSnapshot, error) {
	snap := defaultSnapshot()

	rows, err := s.db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return snap, fmt.Errorf("read settings: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			return snap, fmt.Errorf("scan setting row: %w", err)
		}
		applyRow(&snap, key, raw)
	}
	if err := rows.Err(); err != nil {
		return snap, fmt.Errorf("iterate settings: %w", err)
	}
	return snap, nil
}

func applyRow(snap *SettingsSnapshot, key, raw string) {
	switch key {
	case "relay_url":
		var v string
		if err := json.Unmarshal([]byte(raw), &v); err == nil {
			if cleaned, err := ValidateRelayURL(v); err == nil {
				snap.RelayURL = cleaned
			}
		}
	case "sound_volume":
		var v float64
		if err := json.Unmarshal([]byte(raw), &v); err == nil && v >= 0 && v <= 1 {
			snap.SoundVolume = v
		}
	case "sound_muted":
		var v bool
		if err := json.Unmarshal([]byte(raw), &v); err == nil {
			snap.SoundMuted = v
		}
	case "event_type_filter":
		var v string
		if err := json.Unmarshal([]byte(raw), &v); err == nil && isValidEventFilter(v) {
			snap.EventTypeFilter = v
		}
	case "auto_start":
		var v bool
		if err := json.Unmarshal([]byte(raw), &v); err == nil {
			snap.AutoStart = v
		}
	}
}

// ValidateRelayURL enforces strict https:// URLs with a valid host.
func ValidateRelayURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("relay URL cannot be empty")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid relay URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return "", errors.New("relay URL must use https://")
	}
	if parsed.Host == "" {
		return "", errors.New("relay URL must include a host")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func isValidEventFilter(v string) bool {
	return v == "all" || v == "people" || v == "vehicle"
}

// Update validates and persists a single allowlisted setting key.
func (s *SettingsStore) Update(key string, value any) error {
	var validated any
	switch key {
	case "relay_url":
		str, ok := value.(string)
		if !ok {
			return errors.New("relay_url must be a string")
		}
		cleaned, err := ValidateRelayURL(str)
		if err != nil {
			return err
		}
		validated = cleaned
	case "sound_volume":
		num, ok := toFloat64(value)
		if !ok || num < 0 || num > 1 {
			return errors.New("sound_volume must be between 0 and 1")
		}
		validated = num
	case "sound_muted":
		b, ok := value.(bool)
		if !ok {
			return errors.New("sound_muted must be a boolean")
		}
		validated = b
	case "event_type_filter":
		str, ok := value.(string)
		if !ok || !isValidEventFilter(str) {
			return errors.New("event_type_filter must be all, people, or vehicle")
		}
		validated = str
	case "auto_start":
		b, ok := value.(bool)
		if !ok {
			return errors.New("auto_start must be a boolean")
		}
		validated = b
	default:
		return fmt.Errorf("unknown or protected setting key %q: refusal by allowlist", key)
	}

	encoded, err := json.Marshal(validated)
	if err != nil {
		return fmt.Errorf("encode setting %q: %w", key, err)
	}

	_, err = s.db.Exec(
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key,
		string(encoded),
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("save setting %q: %w", key, err)
	}
	return nil
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}
