package secrets

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"banshee/internal/store"
)

type disabledCipher struct{}

func (disabledCipher) IsAvailable() bool               { return false }
func (disabledCipher) Encrypt(string) ([]byte, error)  { return nil, ErrCipherUnavailable }
func (disabledCipher) Decrypt([]byte) (string, error)  { return "", ErrCipherUnavailable }

func TestSecretsRefusesPlaintextWhenCipherUnavailable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "secrets.sqlite")
	db, err := store.OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("OpenDatabase: %v", err)
	}
	defer db.Close()
	if _, err := store.ApplyMigrations(db, nil); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}

	s := NewStore(db, disabledCipher{})
	err = s.Save("device_token", "super-secret-value")
	if !errors.Is(err, ErrCipherUnavailable) {
		t.Fatalf("expected ErrCipherUnavailable, got %v", err)
	}

	st, err := s.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.DeviceTokenSet || st.EncryptionAvailable {
		t.Fatalf("unexpected secret status when cipher disabled: %+v", st)
	}
}

func TestSecretsRejectsUnknownNamesAndEmptyOrOversizedTokens(t *testing.T) {
	db, err := store.OpenDatabase(filepath.Join(t.TempDir(), "secrets.sqlite"))
	if err != nil {
		t.Fatalf("OpenDatabase: %v", err)
	}
	defer db.Close()
	if _, err := store.ApplyMigrations(db, nil); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}

	s := NewStore(db, NewOSCipher())
	if err := s.Save("arbitrary_key", "abc"); err == nil {
		t.Fatal("expected unknown secret name to fail")
	}
	if err := s.Save("device_token", "   "); err == nil {
		t.Fatal("expected whitespace-only token to fail")
	}
	if err := s.Save("device_token", strings.Repeat("x", maxTokenLength+1)); err == nil {
		t.Fatal("expected oversized token to fail")
	}
}

func TestSecretsDPAPIRoundTripLeavesZeroPlaintextOnDisk(t *testing.T) {
	cipher := NewOSCipher()
	if !cipher.IsAvailable() {
		t.Skip("OS DPAPI cipher not available on this host")
	}

	dbPath := filepath.Join(t.TempDir(), "dpapi.sqlite")
	db, err := store.OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("OpenDatabase: %v", err)
	}
	if _, err := store.ApplyMigrations(db, nil); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}

	const rawToken = "dev_banshee_secret_token_987654321"
	s := NewStore(db, cipher)
	if err := s.Save("device_token", "  "+rawToken+"  "); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := s.Load("device_token")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded != rawToken {
		t.Fatalf("expected trimmed %q, got %q", rawToken, loaded)
	}

	st, err := s.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.EncryptionAvailable || !st.DeviceTokenSet || st.TuringTokenSet {
		t.Fatalf("unexpected secret status: %+v", st)
	}
	_ = db.Close()

	// Verify the raw SQLite file bytes never contain the plaintext token.
	diskBytes, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if bytes.Contains(diskBytes, []byte(rawToken)) {
		t.Fatal("security failure: plaintext token found inside SQLite file bytes")
	}
}
