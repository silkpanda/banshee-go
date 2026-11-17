package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"banshee/internal/relay"
	"banshee/internal/secrets"
	"banshee/internal/store"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	db, err := store.OpenDatabase(filepath.Join(t.TempDir(), "app.sqlite"))
	if err != nil {
		t.Fatalf("OpenDatabase: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := store.ApplyMigrations(db, nil); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}
	app := NewAppWithDeps(db, secrets.NewOSCipher())
	t.Cleanup(func() { app.stopRelay() })
	return app
}

func TestGetShellStatusDefaults(t *testing.T) {
	t.Setenv("BANSHEE_RELAY_URL", "")
	app := NewApp()

	status := app.GetShellStatus()
	if status.AppName != "Banshee" {
		t.Fatalf("expected AppName Banshee, got %q", status.AppName)
	}
	if status.RelayURL != defaultRelayURL {
		t.Fatalf("expected default relay URL %q, got %q", defaultRelayURL, status.RelayURL)
	}
	if status.RelayState != "standby" {
		t.Fatalf("expected standby state, got %q", status.RelayState)
	}
}

func TestGetShellStatusUsesEnvRelayURL(t *testing.T) {
	customURL := "https://relay.example.test"
	t.Setenv("BANSHEE_RELAY_URL", customURL)
	app := NewApp()

	status := app.GetShellStatus()
	if status.RelayURL != customURL {
		t.Fatalf("expected custom relay URL %q, got %q", customURL, status.RelayURL)
	}
}

func TestGreet(t *testing.T) {
	app := NewApp()
	got := app.Greet("Locke")
	if !strings.Contains(got, "Locke") {
		t.Fatalf("expected greeting to include name, got %q", got)
	}
}

func TestAppSettingsAndSecretBindingsNeverExposePlaintext(t *testing.T) {
	app := newTestApp(t)

	if _, err := app.UpdateSetting("relay_url", "http://insecure.example.com"); err == nil {
		t.Fatal("expected http:// relay_url to be rejected")
	}

	snap, err := app.UpdateSetting("relay_url", "https://custom-relay.example.com")
	if err != nil {
		t.Fatalf("UpdateSetting failed: %v", err)
	}
	if snap.RelayURL != "https://custom-relay.example.com" {
		t.Fatalf("expected updated relayUrl, got %q", snap.RelayURL)
	}

	cipher := secrets.NewOSCipher()
	if !cipher.IsAvailable() {
		t.Skip("OS DPAPI cipher unavailable on this host")
	}

	const token = "secret_device_token_never_expose"
	st, err := app.SaveSecret("device_token", token)
	if err != nil {
		t.Fatalf("SaveSecret failed: %v", err)
	}
	if !st.DeviceTokenSet {
		t.Fatal("expected DeviceTokenSet true")
	}

	// Verify serialized status & settings JSON never include plaintext secret material.
	rawStatus, _ := json.Marshal(st)
	rawSettings, _ := json.Marshal(snap)
	if strings.Contains(string(rawStatus), token) || strings.Contains(string(rawSettings), token) {
		t.Fatal("security failure: secret plaintext leaked in status or settings response")
	}

	stAfterClear, err := app.ClearSecret("device_token")
	if err != nil {
		t.Fatalf("ClearSecret failed: %v", err)
	}
	if stAfterClear.DeviceTokenSet {
		t.Fatal("expected DeviceTokenSet false after clear")
	}
}

type fakeTraySink struct {
	mu     sync.Mutex
	labels []string
}

func (f *fakeTraySink) SetRelayStatus(label string) {
	f.mu.Lock()
	f.labels = append(f.labels, label)
	f.mu.Unlock()
}

func (f *fakeTraySink) last() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.labels) == 0 {
		return ""
	}
	return f.labels[len(f.labels)-1]
}

type memCipher struct{}

func (memCipher) IsAvailable() bool { return true }
func (memCipher) Encrypt(plaintext string) ([]byte, error) {
	return []byte("enc:" + plaintext), nil
}
func (memCipher) Decrypt(blob []byte) (string, error) {
	return strings.TrimPrefix(string(blob), "enc:"), nil
}

func TestAppRelayWiringLiveStatusTrayAndSecretRedaction(t *testing.T) {
	t.Setenv("BANSHEE_DEVICE_TOKEN", "")
	upgrader := websocket.Upgrader{}
	const validToken = "banshee-live-test-token-xyz"

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ws" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+validToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()

		_, _, err = ws.ReadMessage()
		if err != nil {
			return
		}
		_ = ws.WriteMessage(websocket.TextMessage, []byte(`{"type":"event","seq":12,"event":{"event_id":"evt-app-12","event_type":"vehicle","stages_received":["created","processed"],"stage":"processed","camera":{"id":7,"name":"North Gate"},"site":{"id":1,"name":"HQ"},"occurred_at":"2026-11-13T15:10:00Z","received_at":"2026-11-13T15:10:01Z","mediums":[{"name":"snapshot","files":[{"url":"https://media.example.com/secret.jpg"}]}],"metadata":[{"license_plate_text":"ABC123"}]}}`))
		_ = ws.WriteMessage(websocket.TextMessage, []byte(`{"type":"replay_done","count":1,"from_seq":12,"to_seq":12}`))
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	db, err := store.OpenDatabase(filepath.Join(t.TempDir(), "app-relay.sqlite"))
	if err != nil {
		t.Fatalf("OpenDatabase: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := store.ApplyMigrations(db, nil); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}

	app := NewAppWithDeps(db, memCipher{})
	t.Cleanup(func() { app.stopRelay() })
	tlsCfg := srv.Client().Transport.(*http.Transport).TLSClientConfig
	_, httpEndpoint, _ := relay.RelayEndpoints(srv.URL)
	app.dialer = relay.NewGorillaDialer(tlsCfg, 3*time.Second)
	app.probeAuth = relay.NewHTTPAuthProbe(httpEndpoint, srv.Client(), 2*time.Second)

	traySink := &fakeTraySink{}
	app.setTraySink(traySink)

	if _, err := app.UpdateSetting("relay_url", srv.URL); err != nil {
		t.Fatalf("UpdateSetting failed: %v", err)
	}

	// Start with an invalid token -> HTTP 401 diagnostic.
	if _, err := app.SaveSecret("device_token", "wrong-token"); err != nil {
		t.Fatalf("SaveSecret(wrong) failed: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		st := app.GetRelayStatus()
		if st.State == "offline" && strings.Contains(st.Error, "401") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	badSnap := app.GetRelayStatus()
	if badSnap.State != "offline" || !strings.Contains(badSnap.Error, "401") {
		t.Fatalf("expected 401 offline diagnostic, got %+v", badSnap)
	}

	// Save valid token -> App restarts relay automatically and reaches connected.
	if _, err := app.SaveSecret("device_token", validToken); err != nil {
		t.Fatalf("SaveSecret(valid) failed: %v", err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		st := app.GetRelayStatus()
		if st.State == "connected" && st.MaxSeq == 12 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	goodSnap := app.GetRelayStatus()
	if goodSnap.State != "connected" || goodSnap.MaxSeq != 12 || goodSnap.AlertCount != 1 || goodSnap.MissedCount != 1 {
		t.Fatalf("unexpected connected snapshot: %+v", goodSnap)
	}
	if goodSnap.TokenSource != "dpapi" {
		t.Fatalf("expected tokenSource dpapi, got %q", goodSnap.TokenSource)
	}
	if traySink.last() != "Connected" {
		t.Fatalf("expected tray label Connected, got %q", traySink.last())
	}
	if shell := app.GetShellStatus(); shell.RelayState != "connected" {
		t.Fatalf("expected GetShellStatus RelayState connected, got %q", shell.RelayState)
	}

	// Serialized status must never contain the token or media URL.
	raw, _ := json.Marshal(goodSnap)
	if strings.Contains(string(raw), validToken) || strings.Contains(string(raw), "secret.jpg") {
		t.Fatalf("security violation: sensitive data leaked in RelayStatusSnapshot: %s", raw)
	}

	// Clearing token restarts relay into no-token offline state.
	if _, err := app.ClearSecret("device_token"); err != nil {
		t.Fatalf("ClearSecret failed: %v", err)
	}
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		st := app.GetRelayStatus()
		if st.State == "offline" && st.Error == "no device token set" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if st := app.GetRelayStatus(); st.Error != "no device token set" {
		t.Fatalf("expected no device token set after clear, got %+v", st)
	}
	_ = context.Background()
}
