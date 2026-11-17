package relaysecurity

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"banshee/internal/relay"
	"banshee/internal/store"
)

func TestProbeP201RelayURLRequiresHTTPS(t *testing.T) {
	disallowed := []string{
		"http://127.0.0.1:8080",
		"ws://127.0.0.1:8080",
		"https://tok:secret@127.0.0.1:8080",
	}
	for _, raw := range disallowed {
		ws, _, err := relay.RelayEndpoints(raw)
		if err == nil {
			t.Logf("EXPLOITED P2-01: insecure relay URL %q produced %q", raw, ws)
			t.FailNow()
		}
	}
	t.Log("HELD P2-01: non-HTTPS and credentialed relay URLs rejected before dial")
}

func TestProbeP202BearerTokenHeaderOnlyNeverQueryString(t *testing.T) {
	const secret = "probe-bearer-token-98765"
	upgrader := websocket.Upgrader{}
	var seenQuery string
	var seenAuth string

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenQuery = r.URL.RawQuery
		seenAuth = r.Header.Get("Authorization")
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		_ = ws.Close()
	}))
	defer srv.Close()

	wsURL, _, err := relay.RelayEndpoints(srv.URL)
	if err != nil {
		t.Fatalf("RelayEndpoints: %v", err)
	}
	tlsCfg := srv.Client().Transport.(*http.Transport).TLSClientConfig
	dial := relay.NewGorillaDialer(tlsCfg, 2*time.Second)

	conn, err := dial(context.Background(), wsURL, secret)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	_ = conn.Close(1000, "probe done")

	if strings.Contains(seenQuery, secret) || seenQuery != "" {
		t.Logf("EXPLOITED P2-02: token or query leaked into WS URL query %q", seenQuery)
		t.FailNow()
	}
	if seenAuth != "Bearer "+secret {
		t.Logf("EXPLOITED P2-02: Authorization header missing Bearer token, got %q", seenAuth)
		t.FailNow()
	}
	t.Log("HELD P2-02: device token transmitted exclusively in Authorization Bearer header")
}

type probeAlertAdapter struct {
	s *store.AlertsStore
}

func (a probeAlertAdapter) MaxSeq() (int64, error) { return a.s.MaxSeq() }
func (a probeAlertAdapter) UpsertFromRelay(seq int64, ev relay.EventPayload, missed bool) error {
	_, err := a.s.UpsertFromRelay(seq, store.AlertUpsert{
		EventID:        ev.EventID,
		EventType:      ev.EventType,
		StagesReceived: ev.StagesReceived,
		CameraID:       *ev.Camera.ID,
		CameraName:     *ev.Camera.Name,
		SiteID:         *ev.Site.ID,
		SiteName:       *ev.Site.Name,
		OccurredAt:     *ev.OccurredAt,
		ReceivedAt:     *ev.ReceivedAt,
	}, missed)
	return err
}

func TestProbeP203ZeroTokenAndMediaLeakage(t *testing.T) {
	const secret = "probe-secret-token-xyzzy"
	const mediaURL = "https://media.local.test/private-snapshot-991.jpg"
	const plate = "PLATE-XYZ-991"

	upgrader := websocket.Upgrader{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		_, _, _ = ws.ReadMessage()
		_ = ws.WriteMessage(websocket.TextMessage, []byte(`{"type":"event","seq":1,"event":{"event_id":"evt-probe-1","event_type":"vehicle","stages_received":["processed"],"stage":"processed","camera":{"id":1,"name":"Gate"},"site":{"id":1,"name":"Yard"},"occurred_at":"2026-11-13T15:00:00Z","received_at":"2026-11-13T15:00:01Z","mediums":[{"name":"snapshot","files":[{"url":"`+mediaURL+`"}]}],"metadata":[{"license_plate_text":"`+plate+`"}]}}`))
		_ = ws.WriteMessage(websocket.TextMessage, []byte(`{"type":"replay_done","count":1,"from_seq":1,"to_seq":1}`))
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	dbPath := filepath.Join(t.TempDir(), "probe.sqlite")
	db, err := store.OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("OpenDatabase: %v", err)
	}
	if _, err := store.ApplyMigrations(db, nil); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}
	alerts := store.NewAlertsStore(db)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	tlsCfg := srv.Client().Transport.(*http.Transport).TLSClientConfig
	_, httpEndpoint, _ := relay.RelayEndpoints(srv.URL)

	c := relay.NewClient(relay.ClientOptions{
		RelayURL:  srv.URL,
		GetToken:  func(context.Context) (string, error) { return secret, nil },
		Store:     probeAlertAdapter{s: alerts},
		Dial:      relay.NewGorillaDialer(tlsCfg, 2*time.Second),
		ProbeAuth: relay.NewHTTPAuthProbe(httpEndpoint, srv.Client(), 2*time.Second),
		Logger:    logger,
	})
	c.Start(context.Background())

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.Status().State == relay.StateConnected {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	c.Stop()
	_ = db.Close()

	if strings.Contains(logBuf.String(), secret) || strings.Contains(logBuf.String(), mediaURL) || strings.Contains(logBuf.String(), plate) {
		t.Logf("EXPLOITED P2-03: sensitive token or media URL appeared in relay logs")
		t.FailNow()
	}
	rawDB, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read sqlite file: %v", err)
	}
	for _, needle := range []string{secret, mediaURL, plate} {
		if bytes.Contains(rawDB, []byte(needle)) {
			t.Logf("EXPLOITED P2-03: sensitive string %q found on disk in SQLite file", needle)
			t.FailNow()
		}
	}
	t.Log("HELD P2-03: zero token, media URL, or plate PII leaked to logs or SQLite disk")
}
