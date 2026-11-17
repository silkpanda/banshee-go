package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvLoadsUnsetBansheeVarsOnly(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := `
# Comment line
BANSHEE_RELAY_URL="https://custom-relay.example.com"
BANSHEE_DEVICE_TOKEN='tok-from-dotenv'
UNRELATED_SECRET=should-not-load
EMPTY_LINE_BELOW=

`
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Setenv("BANSHEE_RELAY_URL", "")
	t.Setenv("BANSHEE_DEVICE_TOKEN", "existing-env-token")
	t.Setenv("UNRELATED_SECRET", "")

	if err := LoadDotEnvIfPresent(envPath); err != nil {
		t.Fatalf("LoadDotEnvIfPresent failed: %v", err)
	}

	if got := os.Getenv("BANSHEE_RELAY_URL"); got != "https://custom-relay.example.com" {
		t.Fatalf("expected BANSHEE_RELAY_URL from .env, got %q", got)
	}
	// Existing non-empty env var must win over .env.
	if got := os.Getenv("BANSHEE_DEVICE_TOKEN"); got != "existing-env-token" {
		t.Fatalf("expected existing env var to be preserved, got %q", got)
	}
	if got := os.Getenv("UNRELATED_SECRET"); got != "" {
		t.Fatalf("expected non-BANSHEE_ key to be ignored, got %q", got)
	}

	// Missing file is a clean no-op.
	if err := LoadDotEnvIfPresent(filepath.Join(dir, "missing.env")); err != nil {
		t.Fatalf("expected missing .env to be ignored, got %v", err)
	}
}
