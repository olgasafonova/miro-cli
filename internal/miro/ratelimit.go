package miro

import (
	"context"
	"sync"
	"time"
)

// DefaultRateLimit is the requests-per-second budget when none is
// configured. Miro's published Tier-1 limit on the v2 REST API is 100
// requests per minute per organization (≈1.67 req/s). Pacing slightly
// below that gives bursty scripts headroom without paging an operator
// for a 429 they can't see coming.
const DefaultRateLimit = 1.5

// DefaultRateBurst is the largest momentary spike the bucket allows.
// Lets short scripts complete a small batch quickly while still
// long-run averaging at DefaultRateLimit.
const DefaultRateBurst = 5

// Limiter is a simple token-bucket rate limiter. One bucket fills at
// rate tokens/second, capped at burst. Wait blocks until a token is
// available or ctx is cancelled.
//
// Zero-value Limiter is never-rate-limit (Wait returns immediately).
// Construct a configured limiter with NewLimiter.
type Limiter struct {
	rate  float64 // tokens added per second
	burst float64 // max tokens in the bucket

	mu     sync.Mutex
	tokens float64
	last   time.Time
}

// NewLimiter returns a Limiter that produces rate tokens per second
// (averaged) and tolerates a burst of up to burst tokens. rate <= 0
// disables limiting (Wait always returns nil immediately). burst < 1
// is bumped to 1 — sub-one bursts make Wait spin forever on the first
// call.
func NewLimiter(rate float64, burst int) *Limiter {
	if rate <= 0 {
		return &Limiter{}
	}
	b := float64(burst)
	if b < 1 {
		b = 1
	}
	return &Limiter{
		rate:   rate,
		burst:  b,
		tokens: b, // start full so the first burst doesn't block
		last:   time.Now(),
	}
}

// Wait blocks until one token is available, then consumes it. Returns
// ctx.Err() if the context is cancelled before a token frees up.
//
// A zero-value Limiter or one constructed with rate<=0 is a no-op:
// Wait returns nil immediately and does not honour context cancellation
// (callers in that path have nothing to wait for).
func (l *Limiter) Wait(ctx context.Context) error {
	if l == nil || l.rate == 0 {
		return nil
	}
	for {
		l.mu.Lock()
		l.refill(time.Now())
		if l.tokens >= 1 {
			l.tokens--
			l.mu.Unlock()
			return nil
		}
		// Compute exactly how long until the next token. Sleeping in
		// fixed quanta would either oversleep (slow) or busy-loop
		// (wasteful). nanoseconds = (1 - tokens) / rate * 1e9.
		need := 1 - l.tokens
		wait := time.Duration(need / l.rate * float64(time.Second))
		l.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
			// loop, retry
		}
	}
}

// refill is called with the lock held. It advances `tokens` by the
// elapsed time since `last`, capped at burst.
func (l *Limiter) refill(now time.Time) {
	elapsed := now.Sub(l.last).Seconds()
	if elapsed <= 0 {
		return
	}
	l.tokens += elapsed * l.rate
	if l.tokens > l.burst {
		l.tokens = l.burst
	}
	l.last = now
}
