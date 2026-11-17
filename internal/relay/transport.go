package relay

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// AuthProbeResult classifies an HTTP probe against the relay /ws upgrade endpoint.
type AuthProbeResult string

const (
	AuthOK           AuthProbeResult = "ok"
	AuthUnauthorized AuthProbeResult = "unauthorized"
	AuthUnreachable  AuthProbeResult = "unreachable"
	AuthNotRelay     AuthProbeResult = "not-relay"
)

// ErrUnauthorized indicates the relay rejected the device bearer token with HTTP 401.
var ErrUnauthorized = errors.New("device token rejected (HTTP 401)")

// Conn abstracts one active WebSocket connection for the relay client.
type Conn interface {
	ReadMessage() ([]byte, error)
	WriteText(data []byte) error
	Close(code int, reason string) error
}

// Dialer opens an authenticated WebSocket connection using Authorization: Bearer <token>.
type Dialer func(ctx context.Context, wsURL string, token string) (Conn, error)

// AuthProbeFunc checks token validity over HTTPS when a WebSocket handshake fails.
type AuthProbeFunc func(ctx context.Context, token string) AuthProbeResult

// RelayEndpoints validates an https:// relay URL and returns its wss:// and https:// /ws endpoints.
func RelayEndpoints(relayURL string) (wsURL string, httpURL string, err error) {
	trimmed := strings.TrimSpace(relayURL)
	if trimmed == "" {
		return "", "", errors.New("relay URL cannot be empty")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", "", fmt.Errorf("invalid relay URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return "", "", errors.New("relay URL must use https://")
	}
	if parsed.Host == "" {
		return "", "", errors.New("relay URL must include a host")
	}
	if parsed.User != nil {
		return "", "", errors.New("relay URL must not include embedded credentials")
	}
	return "wss://" + parsed.Host + "/ws", "https://" + parsed.Host + "/ws", nil
}

func toAllowedCloseCode(code int) int {
	if code == 1000 || (code >= 3000 && code <= 4999) {
		return code
	}
	return 4000 + (code % 1000)
}

type gorillaConn struct {
	ws      *websocket.Conn
	writeMu sync.Mutex
}

func (g *gorillaConn) ReadMessage() ([]byte, error) {
	for {
		msgType, data, err := g.ws.ReadMessage()
		if err != nil {
			return nil, err
		}
		// Relay sends JSON text frames; ignore non-text frames per protocol spec.
		if msgType == websocket.TextMessage {
			return data, nil
		}
	}
}

func (g *gorillaConn) WriteText(data []byte) error {
	g.writeMu.Lock()
	defer g.writeMu.Unlock()
	_ = g.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return g.ws.WriteMessage(websocket.TextMessage, data)
}

func (g *gorillaConn) Close(code int, reason string) error {
	g.writeMu.Lock()
	defer g.writeMu.Unlock()
	msg := websocket.FormatCloseMessage(toAllowedCloseCode(code), reason)
	_ = g.ws.WriteControl(websocket.CloseMessage, msg, time.Now().Add(2*time.Second))
	return g.ws.Close()
}

// NewGorillaDialer creates a production WebSocket dialer sending Authorization: Bearer <token>.
func NewGorillaDialer(tlsConfig *tls.Config, handshakeTimeout time.Duration) Dialer {
	if handshakeTimeout <= 0 {
		handshakeTimeout = 10 * time.Second
	}
	return func(ctx context.Context, wsURL string, token string) (Conn, error) {
		dialer := websocket.Dialer{
			TLSClientConfig:  tlsConfig,
			HandshakeTimeout: handshakeTimeout,
		}
		headers := http.Header{}
		headers.Set("Authorization", "Bearer "+token)

		ws, resp, err := dialer.DialContext(ctx, wsURL, headers)
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if err != nil {
			if resp != nil && resp.StatusCode == http.StatusUnauthorized {
				return nil, fmt.Errorf("%w", ErrUnauthorized)
			}
			return nil, fmt.Errorf("websocket dial failed: %w", err)
		}
		return &gorillaConn{ws: ws}, nil
	}
}

// NewHTTPAuthProbe probes the relay /ws HTTP endpoint to distinguish 401 bad tokens from network/server outages.
func NewHTTPAuthProbe(httpURL string, baseClient *http.Client, timeout time.Duration) AuthProbeFunc {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	var client http.Client
	if baseClient != nil {
		client = *baseClient
	}
	client.Timeout = timeout
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return func(ctx context.Context, token string) AuthProbeResult {
		probeCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, httpURL, nil)
		if err != nil {
			return AuthUnreachable
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			return AuthUnreachable
		}
		defer resp.Body.Close()

		switch {
		case resp.StatusCode == http.StatusUnauthorized:
			return AuthUnauthorized
		case resp.StatusCode == http.StatusUpgradeRequired:
			return AuthOK
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			return AuthUnreachable
		default:
			return AuthNotRelay
		}
	}
}
