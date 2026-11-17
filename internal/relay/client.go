package relay

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultLivenessCheck = 15 * time.Second
	defaultDegradedAfter = 45 * time.Second
	defaultDeadAfter     = 60 * time.Second
)

// ConnState describes the live connection health of the relay client.
type ConnState string

const (
	StateStandby   ConnState = "standby"
	StateOffline   ConnState = "offline"
	StateDegraded  ConnState = "degraded"
	StateConnected ConnState = "connected"
)

// Status captures the observable relay client state without exposing credentials or media URLs.
type Status struct {
	State           ConnState  `json:"state"`
	RelayURL        string     `json:"relayUrl"`
	LastContactAt   *time.Time `json:"lastContactAt"`
	LastReplayCount *int64     `json:"lastReplayCount"`
	Error           string     `json:"error"`
}

// AlertStore is the durable local storage contract required by the relay client.
type AlertStore interface {
	MaxSeq() (int64, error)
	UpsertFromRelay(seq int64, event EventPayload, missed bool) error
}

// ClientOptions configures a Relay Client instance.
type ClientOptions struct {
	RelayURL      string
	ClientVersion string
	GetToken      func(ctx context.Context) (string, error)
	Store         AlertStore
	Dial          Dialer
	ProbeAuth     AuthProbeFunc
	Backoff       *Backoff
	LivenessCheck time.Duration
	DegradedAfter time.Duration
	DeadAfter     time.Duration
	Now           func() time.Time
	Logger        *slog.Logger
}

// Client maintains an outbound authenticated WebSocket connection to the Turing relay.
type Client struct {
	opts ClientOptions

	mu        sync.Mutex
	status    Status
	listeners []func(Status)
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewClient constructs a relay Client configured with safe defaults.
func NewClient(opts ClientOptions) *Client {
	if opts.Backoff == nil {
		opts.Backoff = NewBackoff(BackoffOptions{})
	}
	if opts.LivenessCheck <= 0 {
		opts.LivenessCheck = defaultLivenessCheck
	}
	if opts.DegradedAfter <= 0 {
		opts.DegradedAfter = defaultDegradedAfter
	}
	if opts.DeadAfter <= 0 {
		opts.DeadAfter = defaultDeadAfter
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Dial == nil {
		opts.Dial = NewGorillaDialer(nil, 10*time.Second)
	}
	if opts.ClientVersion == "" {
		opts.ClientVersion = "0.1.0-dev"
	}

	return &Client{
		opts: opts,
		status: Status{
			State:    StateStandby,
			RelayURL: opts.RelayURL,
		},
	}
}

// Status returns a thread-safe copy of the current relay status.
func (c *Client) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return cloneStatus(c.status)
}

// OnStatus registers a callback invoked whenever relay status changes.
func (c *Client) OnStatus(fn func(Status)) {
	if fn == nil {
		return
	}
	c.mu.Lock()
	c.listeners = append(c.listeners, fn)
	c.mu.Unlock()
}

// Start begins the relay connection supervisor in a background goroutine.
func (c *Client) Start(parent context.Context) {
	c.Stop()

	ctx, cancel := context.WithCancel(parent)
	c.mu.Lock()
	c.cancel = cancel
	c.mu.Unlock()

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.run(ctx)
	}()
}

// Stop tears down any active WebSocket connection and stops reconnect attempts.
func (c *Client) Stop() {
	c.mu.Lock()
	cancel := c.cancel
	c.cancel = nil
	c.mu.Unlock()

	if cancel != nil {
		cancel()
		c.wg.Wait()
	}
}

func (c *Client) updateStatus(mutate func(s *Status)) {
	c.mu.Lock()
	mutate(&c.status)
	snap := cloneStatus(c.status)
	listeners := make([]func(Status), len(c.listeners))
	copy(listeners, c.listeners)
	c.mu.Unlock()

	for _, fn := range listeners {
		fn(cloneStatus(snap))
	}
}

func cloneStatus(s Status) Status {
	if s.LastContactAt != nil {
		t := *s.LastContactAt
		s.LastContactAt = &t
	}
	if s.LastReplayCount != nil {
		n := *s.LastReplayCount
		s.LastReplayCount = &n
	}
	return s
}

func (c *Client) run(ctx context.Context) {
	wsURL, httpURL, err := RelayEndpoints(c.opts.RelayURL)
	if err != nil {
		c.updateStatus(func(s *Status) {
			s.State = StateOffline
			s.Error = err.Error()
		})
		return
	}

	probe := c.opts.ProbeAuth
	if probe == nil {
		probe = NewHTTPAuthProbe(httpURL, nil, 10*time.Second)
	}

	for {
		if ctx.Err() != nil {
			c.setStoppedStatus()
			return
		}

		token, err := c.opts.GetToken(ctx)
		if ctx.Err() != nil {
			c.setStoppedStatus()
			return
		}
		if err != nil {
			c.opts.Logger.Error("relay: device token unreadable")
			c.updateStatus(func(s *Status) {
				s.State = StateOffline
				s.Error = "device token unreadable"
			})
			return
		}
		if token == "" {
			c.opts.Logger.Warn("relay: no device token set")
			c.updateStatus(func(s *Status) {
				s.State = StateOffline
				s.Error = "no device token set"
			})
			return
		}

		conn, err := c.opts.Dial(ctx, wsURL, token)
		if err != nil {
			if ctx.Err() != nil {
				c.setStoppedStatus()
				return
			}
			if errors.Is(err, ErrUnauthorized) {
				c.opts.Logger.Error("relay: device token rejected (HTTP 401)")
				c.updateStatus(func(s *Status) {
					s.State = StateOffline
					s.Error = "device token rejected (HTTP 401)"
				})
				return
			}

			switch probe(ctx, token) {
			case AuthUnauthorized:
				c.opts.Logger.Error("relay: device token rejected (HTTP 401)")
				c.updateStatus(func(s *Status) {
					s.State = StateOffline
					s.Error = "device token rejected (HTTP 401)"
				})
				return
			case AuthNotRelay:
				c.opts.Logger.Warn("relay: endpoint is not a turing relay")
				c.updateStatus(func(s *Status) {
					s.State = StateOffline
					s.Error = "endpoint is not a turing relay"
				})
			default:
				c.updateStatus(func(s *Status) {
					s.State = StateOffline
					s.Error = "relay unreachable — retrying"
				})
			}

			if !sleepWithContext(ctx, c.opts.Backoff.Next()) {
				c.setStoppedStatus()
				return
			}
			continue
		}

		rejected, reason := c.serveConn(ctx, conn)
		if ctx.Err() != nil {
			c.setStoppedStatus()
			return
		}
		if rejected {
			c.opts.Logger.Error("relay: device token rejected by server close")
			c.updateStatus(func(s *Status) {
				s.State = StateOffline
				s.Error = "device token rejected"
			})
			return
		}

		c.updateStatus(func(s *Status) {
			s.State = StateOffline
			s.Error = reason
		})

		if !sleepWithContext(ctx, c.opts.Backoff.Next()) {
			c.setStoppedStatus()
			return
		}
	}
}

func (c *Client) setStoppedStatus() {
	c.updateStatus(func(s *Status) {
		s.State = StateStandby
		s.Error = ""
	})
}

type readResult struct {
	data []byte
	err  error
}

func (c *Client) serveConn(ctx context.Context, conn Conn) (rejected bool, disconnectReason string) {
	cursor, err := c.opts.Store.MaxSeq()
	if err != nil {
		_ = conn.Close(1011, "store cursor failed")
		return false, "failed to read local alert cursor"
	}

	helloBytes, err := EncodeHello(cursor, c.opts.ClientVersion)
	if err != nil {
		_ = conn.Close(1011, "encode hello failed")
		return false, "failed to build relay hello"
	}
	if err := conn.WriteText(helloBytes); err != nil {
		_ = conn.Close(1011, "write hello failed")
		return false, "failed to send relay hello"
	}

	lastContact := c.opts.Now()
	replayDone := false
	c.updateStatus(func(s *Status) {
		s.State = StateDegraded
		s.LastContactAt = &lastContact
		s.Error = ""
	})

	reads := make(chan readResult, 16)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			data, readErr := conn.ReadMessage()
			select {
			case reads <- readResult{data: data, err: readErr}:
			case <-ctx.Done():
				return
			}
			if readErr != nil {
				return
			}
		}
	}()

	ticker := time.NewTicker(c.opts.LivenessCheck)
	defer ticker.Stop()

	closeAndDrain := func(code int, reason string) {
		_ = conn.Close(code, reason)
		<-readDone
	}

	for {
		select {
		case <-ctx.Done():
			closeAndDrain(1000, "client stopped")
			return false, ""

		case res := <-reads:
			if res.err != nil {
				<-readDone
				var closeErr *websocket.CloseError
				if errors.As(res.err, &closeErr) {
					if closeErr.Code == websocket.ClosePolicyViolation {
						return true, "device token rejected"
					}
					return false, fmt.Sprintf("connection closed (%d)", closeErr.Code)
				}
				return false, "connection closed"
			}

			now := c.opts.Now()
			lastContact = now
			c.updateStatus(func(s *Status) {
				s.LastContactAt = &now
				if replayDone && s.State == StateDegraded {
					s.State = StateConnected
				}
			})

			frame, parseErr := ParseServerFrame(res.data)
			if parseErr != nil {
				c.opts.Logger.Warn("relay: dropped malformed frame", "reason", parseErr.Error())
				continue
			}

			switch frame.Type {
			case FrameTypeEvent:
				seq := *frame.Seq
				if err := c.opts.Store.UpsertFromRelay(seq, *frame.Event, !replayDone); err != nil {
					c.opts.Logger.Error("relay: store failed; closing to force redelivery", "seq", seq)
					closeAndDrain(1011, "store failed")
					return false, "local alert store error"
				}
				ackBytes, err := EncodeAck(seq)
				if err != nil {
					closeAndDrain(1011, "encode ack failed")
					return false, "failed to encode ack"
				}
				if err := conn.WriteText(ackBytes); err != nil {
					closeAndDrain(1011, "write ack failed")
					return false, "connection closed"
				}

			case FrameTypeReplayDone:
				replayDone = true
				c.opts.Backoff.Reset()
				count := *frame.Count
				c.updateStatus(func(s *Status) {
					s.State = StateConnected
					s.LastReplayCount = &count
					s.Error = ""
				})
				c.opts.Logger.Info("relay: replay done", "count", count)

			case FrameTypePing:
				pongBytes, err := EncodePong(frame.T)
				if err != nil {
					continue
				}
				if err := conn.WriteText(pongBytes); err != nil {
					closeAndDrain(1011, "write pong failed")
					return false, "connection closed"
				}

			case FrameTypePong:
				// Liveness timestamp already refreshed above.
			}

		case <-ticker.C:
			idle := c.opts.Now().Sub(lastContact)
			if idle > c.opts.DeadAfter {
				c.opts.Logger.Warn("relay: heartbeat timeout", "idle", idle.String())
				closeAndDrain(4000, "heartbeat timeout")
				return false, "heartbeat timeout"
			}
			if idle > c.opts.DegradedAfter {
				c.updateStatus(func(s *Status) {
					if s.State == StateConnected {
						s.State = StateDegraded
					}
				})
			}
		}
	}
}

func sleepWithContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
