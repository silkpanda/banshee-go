package secrets

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const maxTokenLength = 4096

// ErrCipherUnavailable is returned whenever OS encryption is unavailable; secrets never fall back to plaintext.
var ErrCipherUnavailable = errors.New("OS secret encryption is unavailable — refusing to store plaintext")

// Cipher abstracts OS-backed encryption (Windows DPAPI in production, injectable in tests).
type Cipher interface {
	IsAvailable() bool
	Encrypt(plaintext string) ([]byte, error)
	Decrypt(blob []byte) (string, error)
}

// SecretStatus reports only whether secrets are present and encryption is active — never token values.
type SecretStatus struct {
	EncryptionAvailable bool `json:"encryptionAvailable"`
	TuringTokenSet      bool `json:"turingTokenSet"`
	DeviceTokenSet      bool `json:"deviceTokenSet"`
}

// Store persists encrypted tokens in SQLite using an allowlisted secret name set.
type Store struct {
	db     *sql.DB
	cipher Cipher
}

// NewStore builds a SecretStore backed by SQLite and the given OS cipher.
func NewStore(db *sql.DB, cipher Cipher) *Store {
	return &Store{db: db, cipher: cipher}
}

func refSettingKey(name string) (string, error) {
	switch name {
	case "turing_token":
		return "turing_token_secret_ref", nil
	case "device_token":
		return "device_token_secret_ref", nil
	default:
		return "", fmt.Errorf("unknown secret name %q: refusal by allowlist", name)
	}
}

// Save encrypts and stores an allowlisted token; refuses empty, oversized, or unencrypted writes.
func (s *Store) Save(name, plaintext string) error {
	refKey, err := refSettingKey(name)
	if err != nil {
		return err
	}
	trimmed := strings.TrimSpace(plaintext)
	if trimmed == "" {
		return errors.New("token cannot be empty")
	}
	if len(trimmed) > maxTokenLength {
		return fmt.Errorf("token exceeds %d characters", maxTokenLength)
	}
	if s.cipher == nil || !s.cipher.IsAvailable() {
		return ErrCipherUnavailable
	}

	blob, err := s.cipher.Encrypt(trimmed)
	if err != nil {
		return fmt.Errorf("encrypt secret %q: %w", name, err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin secret save: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO secrets (name, blob, rotated_at) VALUES (?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET blob = excluded.blob, rotated_at = excluded.rotated_at`,
		name,
		blob,
		now,
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("store secret %q: %w", name, err)
	}

	refVal, _ := json.Marshal("secrets:" + name)
	if _, err := tx.Exec(
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		refKey,
		string(refVal),
		now,
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("store secret ref %q: %w", refKey, err)
	}
	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("commit secret save: %w", err)
	}
	return nil
}

// Load decrypts an allowlisted secret for backend use only.
func (s *Store) Load(name string) (string, error) {
	if _, err := refSettingKey(name); err != nil {
		return "", err
	}
	if s.cipher == nil || !s.cipher.IsAvailable() {
		return "", ErrCipherUnavailable
	}
	var blob []byte
	err := s.db.QueryRow(`SELECT blob FROM secrets WHERE name = ?`, name).Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("secret not set: %s", name)
	}
	if err != nil {
		return "", fmt.Errorf("load secret %q: %w", name, err)
	}
	plain, err := s.cipher.Decrypt(blob)
	if err != nil {
		return "", fmt.Errorf("decrypt secret %q: %w", name, err)
	}
	return plain, nil
}

// Clear removes a stored secret and its settings reference.
func (s *Store) Clear(name string) error {
	refKey, err := refSettingKey(name)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin secret clear: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM secrets WHERE name = ?`, name); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete secret %q: %w", name, err)
	}
	if _, err := tx.Exec(`DELETE FROM settings WHERE key = ?`, refKey); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete secret ref %q: %w", refKey, err)
	}
	return tx.Commit()
}

// Status reports encryption availability and token presence booleans without returning plaintext.
func (s *Store) Status() (SecretStatus, error) {
	st := SecretStatus{
		EncryptionAvailable: s.cipher != nil && s.cipher.IsAvailable(),
	}
	rows, err := s.db.Query(`SELECT name FROM secrets`)
	if err != nil {
		return st, fmt.Errorf("query secret status: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return st, err
		}
		if name == "turing_token" {
			st.TuringTokenSet = true
		}
		if name == "device_token" {
			st.DeviceTokenSet = true
		}
	}
	return st, rows.Err()
}
