package relay

import (
	"testing"
	"time"
)

func TestBackoffBoundsAndReset(t *testing.T) {
	// Zero jitter always lands on baseMs (1s).
	bMin := NewBackoff(BackoffOptions{
		Base:   1 * time.Second,
		Factor: 2,
		Cap:    60 * time.Second,
		Rand:   func() float64 { return 0 },
	})
	for i := 0; i < 10; i++ {
		d := bMin.Next()
		if d != 1*time.Second {
			t.Fatalf("attempt %d with rand=0: got %v, want 1s", i, d)
		}
	}
	if bMin.Attempts() != 10 {
		t.Fatalf("expected 10 attempts, got %d", bMin.Attempts())
	}
	bMin.Reset()
	if bMin.Attempts() != 0 {
		t.Fatalf("expected 0 attempts after Reset, got %d", bMin.Attempts())
	}

	// Max jitter (rand -> 1) scales exponentially up to Cap (60s).
	bMax := NewBackoff(BackoffOptions{
		Base:   1 * time.Second,
		Factor: 2,
		Cap:    60 * time.Second,
		Rand:   func() float64 { return 0.999999 },
	})
	expectedCeilings := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		32 * time.Second,
		60 * time.Second,
		60 * time.Second,
	}
	for i, ceil := range expectedCeilings {
		d := bMax.Next()
		if d < 1*time.Second || d > 60*time.Second {
			t.Fatalf("attempt %d out of [1s, 60s] bounds: %v", i, d)
		}
		if d > ceil || (ceil > 1*time.Second && d < ceil-10*time.Millisecond) {
			t.Fatalf("attempt %d with rand~1: got %v, expected near %v", i, d, ceil)
		}
	}
}
