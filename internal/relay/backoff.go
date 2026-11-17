package relay

import (
	"math"
	"math/rand/v2"
	"sync"
	"time"
)

// BackoffOptions configures jittered exponential reconnect delays.
type BackoffOptions struct {
	Base   time.Duration
	Factor float64
	Cap    time.Duration
	Rand   func() float64
}

// Backoff computes bounded jittered exponential delays in [Base, Cap].
type Backoff struct {
	mu      sync.Mutex
	base    time.Duration
	factor  float64
	cap     time.Duration
	randFn  func() float64
	attempt int
}

// NewBackoff creates a Backoff instance using defaults of 1s base, 2x factor, and 60s cap.
func NewBackoff(opts BackoffOptions) *Backoff {
	base := opts.Base
	if base <= 0 {
		base = 1 * time.Second
	}
	factor := opts.Factor
	if factor <= 0 {
		factor = 2
	}
	maxDelay := opts.Cap
	if maxDelay < base {
		maxDelay = 60 * time.Second
		if maxDelay < base {
			maxDelay = base
		}
	}
	randFn := opts.Rand
	if randFn == nil {
		randFn = rand.Float64
	}
	return &Backoff{
		base:   base,
		factor: factor,
		cap:    maxDelay,
		randFn: randFn,
	}
}

// Next returns the next jittered delay within [base, cap] and increments the attempt counter.
func (b *Backoff) Next() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	scaled := float64(b.base) * math.Pow(b.factor, float64(b.attempt))
	ceiling := time.Duration(math.Min(float64(b.cap), scaled))
	if ceiling < b.base {
		ceiling = b.base
	}

	r := b.randFn()
	if r < 0 {
		r = 0
	} else if r >= 1 {
		r = math.Nextafter(1.0, 0.0)
	}

	delay := b.base + time.Duration(r*float64(ceiling-b.base))
	b.attempt++
	return delay
}

// Reset clears the attempt counter after a healthy replay_done handshake.
func (b *Backoff) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.attempt = 0
}

// Attempts returns how many reconnect delays have been issued since the last Reset.
func (b *Backoff) Attempts() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.attempt
}
