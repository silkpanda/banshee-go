package relay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type fakeConn struct {
	mu       sync.Mutex
	in       chan []byte
	closeErr error
	sent     []string
	closed   bool
	closeCod int
	closeMsg string
}

func newFakeConn() *fakeConn {
	return &fakeConn{in: make(chan []byte, 32)}
}

func (f *fakeConn) pushServer(msg string) {
	f.in <- []byte(msg)
}

func (f *fakeConn) failRead(err error) {
	f.mu.Lock()
	f.closeErr = err
	f.mu.Unlock()
	close(f.in)
}

func (f *fakeConn) ReadMessage() ([]byte, error) {
	msg, ok := <-f.in
	if !ok {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.closeErr != nil {
			return nil, f.closeErr
		}
		return nil, errors.New("closed")
	}
	return msg, nil
}

func (f *fakeConn) WriteText(data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return errors.New("write on closed socket")
	}
	f.sent = append(f.sent, string(data))
	return nil
}

func (f *fakeConn) Close(code int, reason string) error {
	f.mu.Lock()
	already := f.closed
	f.closed = true
	f.closeCod = code
	f.closeMsg = reason
	f.mu.Unlock()
	if !already {
		defer func() { _ = recover() }()
		close(f.in)
	}
	return nil
}

func (f *fakeConn) Sent() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]string, len(f.sent))
	copy(cp, f.sent)
	return cp
}

type fakeStore struct {
	mu      sync.Mutex
	maxSeq  int64
	upserts []struct {
		seq    int64
		missed bool
		id     string
	}
	failUpsert bool
}

func (s *fakeStore) MaxSeq() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxSeq, nil
}

func (s *fakeStore) UpsertFromRelay(seq int64, ev EventPayload, missed bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failUpsert {
		return errors.New("sqlite disk full")
	}
	s.upserts = append(s.upserts, struct {
		seq    int64
		missed bool
		id     string
	}{seq: seq, missed: missed, id: ev.EventID})
	if seq > s.maxSeq {
		s.maxSeq = seq
	}
	return nil
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool, desc string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", desc)
}

func sampleEventFrame(seq int64, id string) string {
	return `{"type":"event","seq":` + jsonNumber(seq) + `,"event":{"event_id":"` + id + `","event_type":"people","stages_received":["created"],"stage":"created","camera":{"id":1,"name":"Lobby"},"site":{"id":2,"name":"HQ"},"occurred_at":"2026-11-13T15:00:00Z","received_at":"2026-11-13T15:00:01Z","mediums":[],"metadata":null}}`
}

func jsonNumber(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestRelayClientMissingAndUnreadableToken(t *testing.T) {
	st := &fakeStore{maxSeq: 10}
	dialCalled := false

	cNoTok := NewClient(ClientOptions{
		RelayURL: "https://relay.test",
		GetToken: func(context.Context) (string, error) { return "", nil },
		Store:    st,
		Dial: func(context.Context, string, string) (Conn, error) {
			dialCalled = true
			return nil, errors.New("should not dial")
		},
	})
	cNoTok.Start(context.Background())
	waitUntil(t, time.Second, func() bool {
		return cNoTok.Status().State == StateOffline && cNoTok.Status().Error == "no device token set"
	}, "no token offline status")
	cNoTok.Stop()
	if dialCalled {
		t.Fatal("must not dial when no token is configured")
	}

	cErrTok := NewClient(ClientOptions{
		RelayURL: "https://relay.test",
		GetToken: func(context.Context) (string, error) { return "", errors.New("DPAPI failed") },
		Store:    st,
		Dial: func(context.Context, string, string) (Conn, error) {
			return nil, errors.New("should not dial")
		},
	})
	cErrTok.Start(context.Background())
	waitUntil(t, time.Second, func() bool {
		return cErrTok.Status().State == StateOffline && cErrTok.Status().Error == "device token unreadable"
	}, "unreadable token offline status")
	cErrTok.Stop()
}

func TestRelayClientReplayOrderingAndAckAfterStore(t *testing.T) {
	st := &fakeStore{maxSeq: 41}
	fc := newFakeConn()

	c := NewClient(ClientOptions{
		RelayURL:      "https://relay.test",
		ClientVersion: "0.1.0-dev",
		GetToken:      func(context.Context) (string, error) { return "dev-tok", nil },
		Store:         st,
		Dial: func(context.Context, string, string) (Conn, error) {
			return fc, nil
		},
	})
	c.Start(context.Background())
	defer c.Stop()

	waitUntil(t, time.Second, func() bool {
		return len(fc.Sent()) >= 1 && c.Status().State == StateDegraded
	}, "hello sent and degraded state")

	if fc.Sent()[0] != `{"type":"hello","cursor":41,"client_version":"0.1.0-dev"}` {
		t.Fatalf("unexpected hello frame: %s", fc.Sent()[0])
	}

	fc.pushServer(sampleEventFrame(42, "evt-42"))
	fc.pushServer(`{"type":"replay_done","count":1,"from_seq":42,"to_seq":42}`)
	fc.pushServer(sampleEventFrame(43, "evt-43"))
	fc.pushServer(`{"type":"ping","t":99}`)

	waitUntil(t, time.Second, func() bool {
		return len(fc.Sent()) >= 4 && c.Status().State == StateConnected
	}, "acks, pong, and connected state")

	sent := fc.Sent()
	if sent[1] != `{"type":"ack","seq":42}` || sent[2] != `{"type":"ack","seq":43}` || sent[3] != `{"type":"pong","t":99}` {
		t.Fatalf("unexpected sent frames: %#v", sent)
	}
	if c.Status().LastReplayCount == nil || *c.Status().LastReplayCount != 1 {
		t.Fatalf("expected LastReplayCount=1, got %+v", c.Status().LastReplayCount)
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	if len(st.upserts) != 2 || !st.upserts[0].missed || st.upserts[1].missed {
		t.Fatalf("expected replay event missed=true and live event missed=false, got %+v", st.upserts)
	}
}

func TestRelayClientStoreFailureSkipsAckAndCloses(t *testing.T) {
	st := &fakeStore{maxSeq: 5, failUpsert: true}
	fc := newFakeConn()
	var dialCount int
	var mu sync.Mutex

	c := NewClient(ClientOptions{
		RelayURL:      "https://relay.test",
		ClientVersion: "0.1.0-dev",
		GetToken:      func(context.Context) (string, error) { return "dev-tok", nil },
		Store:         st,
		Backoff:       NewBackoff(BackoffOptions{Base: 5 * time.Millisecond, Cap: 10 * time.Millisecond}),
		Dial: func(context.Context, string, string) (Conn, error) {
			mu.Lock()
			dialCount++
			n := dialCount
			mu.Unlock()
			if n == 1 {
				return fc, nil
			}
			return newFakeConn(), nil
		},
	})
	c.Start(context.Background())
	defer c.Stop()

	waitUntil(t, time.Second, func() bool { return len(fc.Sent()) >= 1 }, "hello sent")
	fc.pushServer(sampleEventFrame(6, "evt-6"))

	waitUntil(t, time.Second, func() bool {
		fc.mu.Lock()
		defer fc.mu.Unlock()
		return fc.closed
	}, "socket closed on store failure")

	for _, frame := range fc.Sent() {
		if strings.Contains(frame, `"ack"`) {
			t.Fatalf("client sent ack despite store failure: %#v", fc.Sent())
		}
	}
}

func TestRelayClientAuthProbe401StopsReconnect(t *testing.T) {
	st := &fakeStore{}
	var dials int
	var mu sync.Mutex
	secretToken := "super-secret-device-token-123"

	c := NewClient(ClientOptions{
		RelayURL: "https://relay.test",
		GetToken: func(context.Context) (string, error) { return secretToken, nil },
		Store:    st,
		Backoff:  NewBackoff(BackoffOptions{Base: 5 * time.Millisecond, Cap: 10 * time.Millisecond}),
		Dial: func(context.Context, string, string) (Conn, error) {
			mu.Lock()
			dials++
			mu.Unlock()
			return nil, errors.New("handshake failed")
		},
		ProbeAuth: func(context.Context, string) AuthProbeResult {
			return AuthUnauthorized
		},
	})
	c.Start(context.Background())
	defer c.Stop()

	waitUntil(t, time.Second, func() bool {
		return c.Status().State == StateOffline && strings.Contains(c.Status().Error, "device token rejected")
	}, "401 diagnostic offline status")

	time.Sleep(40 * time.Millisecond)
	mu.Lock()
	totalDials := dials
	mu.Unlock()
	if totalDials != 1 {
		t.Fatalf("expected 401 to halt reconnect loop after 1 dial, got %d", totalDials)
	}
	if strings.Contains(c.Status().Error, secretToken) {
		t.Fatalf("status error leaked secret token: %s", c.Status().Error)
	}
}

func TestRelayClientHeartbeatTimeoutAndBackoff(t *testing.T) {
	st := &fakeStore{}
	fc1 := newFakeConn()
	fc2 := newFakeConn()
	var dials int
	var mu sync.Mutex

	nowMu := sync.Mutex{}
	fakeNow := time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)
	advance := func(d time.Duration) {
		nowMu.Lock()
		fakeNow = fakeNow.Add(d)
		nowMu.Unlock()
	}

	c := NewClient(ClientOptions{
		RelayURL:      "https://relay.test",
		GetToken:      func(context.Context) (string, error) { return "tok", nil },
		Store:         st,
		Backoff:       NewBackoff(BackoffOptions{Base: 5 * time.Millisecond, Cap: 10 * time.Millisecond}),
		LivenessCheck: 5 * time.Millisecond,
		DegradedAfter: 45 * time.Second,
		DeadAfter:     60 * time.Second,
		Now: func() time.Time {
			nowMu.Lock()
			defer nowMu.Unlock()
			return fakeNow
		},
		Dial: func(context.Context, string, string) (Conn, error) {
			mu.Lock()
			dials++
			n := dials
			mu.Unlock()
			if n == 1 {
				return fc1, nil
			}
			return fc2, nil
		},
	})
	c.Start(context.Background())
	defer c.Stop()

	waitUntil(t, time.Second, func() bool { return len(fc1.Sent()) >= 1 }, "initial hello")
	fc1.pushServer(`{"type":"replay_done","count":0,"from_seq":0,"to_seq":0}`)
	waitUntil(t, time.Second, func() bool { return c.Status().State == StateConnected }, "connected")

	advance(46 * time.Second)
	waitUntil(t, time.Second, func() bool { return c.Status().State == StateDegraded }, "degraded after 45s idle")

	advance(20 * time.Second)
	waitUntil(t, time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return dials >= 2 && len(fc2.Sent()) >= 1
	}, "heartbeat timeout reconnect")
}

func TestRelayClientLiveMockWebSocketIntegration(t *testing.T) {
	upgrader := websocket.Upgrader{}
	var receivedAuth string
	var receivedHello string
	var receivedAck string
	doneCh := make(chan struct{})

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ws" {
			http.NotFound(w, r)
			return
		}
		receivedAuth = r.Header.Get("Authorization")
		if receivedAuth != "Bearer live-integration-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()

		_, helloBytes, err := ws.ReadMessage()
		if err != nil {
			return
		}
		receivedHello = string(helloBytes)

		_ = ws.WriteMessage(websocket.TextMessage, []byte(sampleEventFrame(77, "evt-live-77")))
		_ = ws.WriteMessage(websocket.TextMessage, []byte(`{"type":"replay_done","count":1,"from_seq":77,"to_seq":77}`))

		_, ackBytes, err := ws.ReadMessage()
		if err == nil {
			receivedAck = string(ackBytes)
			close(doneCh)
		}
		// Keep open until client stops.
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	st := &fakeStore{maxSeq: 70}
	_, httpEndpoint, err := RelayEndpoints(srv.URL)
	if err != nil {
		t.Fatalf("RelayEndpoints failed: %v", err)
	}

	tlsCfg := srv.Client().Transport.(*http.Transport).TLSClientConfig
	c := NewClient(ClientOptions{
		RelayURL:      srv.URL,
		ClientVersion: "0.1.0-test",
		GetToken:      func(context.Context) (string, error) { return "live-integration-token", nil },
		Store:         st,
		Dial:          NewGorillaDialer(tlsCfg, 5*time.Second),
		ProbeAuth:     NewHTTPAuthProbe(httpEndpoint, srv.Client(), 2*time.Second),
	})
	c.Start(context.Background())
	defer c.Stop()

	select {
	case <-doneCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for live TLS WebSocket hello -> event -> ack flow")
	}

	waitUntil(t, time.Second, func() bool { return c.Status().State == StateConnected }, "connected state")
	if receivedAuth != "Bearer live-integration-token" {
		t.Fatalf("unexpected auth header: %q", receivedAuth)
	}
	if receivedHello != `{"type":"hello","cursor":70,"client_version":"0.1.0-test"}` {
		t.Fatalf("unexpected hello: %s", receivedHello)
	}
	if receivedAck != `{"type":"ack","seq":77}` {
		t.Fatalf("unexpected ack: %s", receivedAck)
	}
}
