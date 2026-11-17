package relay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRelayEndpointsEnforcesHTTPS(t *testing.T) {
	ws, httpURL, err := RelayEndpoints("https://relay.example.com")
	if err != nil {
		t.Fatalf("expected valid https URL to succeed, got %v", err)
	}
	if ws != "wss://relay.example.com/ws" {
		t.Fatalf("unexpected ws URL: %s", ws)
	}
	if httpURL != "https://relay.example.com/ws" {
		t.Fatalf("unexpected http URL: %s", httpURL)
	}

	// Trailing paths/queries are normalized to /ws.
	ws2, http2, err := RelayEndpoints("https://relay.example.com/extra/path?ignored=1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws2 != "wss://relay.example.com/ws" || http2 != "https://relay.example.com/ws" {
		t.Fatalf("unexpected normalized endpoints: ws=%s http=%s", ws2, http2)
	}

	bad := []string{
		"",
		"http://relay.example.com",
		"ws://relay.example.com",
		"wss://relay.example.com",
		"https://",
		"https://user:pass@relay.example.com",
		"not a url",
	}
	for _, raw := range bad {
		if _, _, err := RelayEndpoints(raw); err == nil {
			t.Fatalf("expected RelayEndpoints(%q) to fail", raw)
		}
	}
}

func TestToAllowedCloseCode(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{1000, 1000},
		{3001, 3001},
		{4000, 4000},
		{1011, 4011},
		{1008, 4008},
	}
	for _, tc := range cases {
		if got := toAllowedCloseCode(tc.in); got != tc.want {
			t.Fatalf("toAllowedCloseCode(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestHTTPAuthProbeStatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		want       AuthProbeResult
	}{
		{name: "401 unauthorized", statusCode: http.StatusUnauthorized, want: AuthUnauthorized},
		{name: "426 upgrade required", statusCode: http.StatusUpgradeRequired, want: AuthOK},
		{name: "429 throttled", statusCode: http.StatusTooManyRequests, want: AuthUnreachable},
		{name: "503 unavailable", statusCode: http.StatusServiceUnavailable, want: AuthUnreachable},
		{name: "200 wrong endpoint", statusCode: http.StatusOK, want: AuthNotRelay},
		{name: "404 wrong endpoint", statusCode: http.StatusNotFound, want: AuthNotRelay},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotAuth string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")
				w.WriteHeader(tc.statusCode)
			}))
			defer srv.Close()

			probe := NewHTTPAuthProbe(srv.URL, srv.Client(), 2*time.Second)
			res := probe(context.Background(), "dev-secret-tok")
			if gotAuth != "Bearer dev-secret-tok" {
				t.Fatalf("expected Bearer auth header, got %q", gotAuth)
			}
			if res != tc.want {
				t.Fatalf("probe result = %q, want %q", res, tc.want)
			}
		})
	}

	// Closed server maps to AuthUnreachable.
	deadSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	deadURL := deadSrv.URL
	deadSrv.Close()
	probe := NewHTTPAuthProbe(deadURL, deadSrv.Client(), 500*time.Millisecond)
	if got := probe(context.Background(), "tok"); got != AuthUnreachable {
		t.Fatalf("expected AuthUnreachable on closed server, got %q", got)
	}
}
