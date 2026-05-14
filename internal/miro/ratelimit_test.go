package miro

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNilLimiterIsNoOp(t *testing.T) {
	var l *Limiter
	if err := l.Wait(context.Background()); err != nil {
		t.Errorf("nil Limiter Wait returned %v, want nil", err)
	}
}

func TestZeroRateIsNoOp(t *testing.T) {
	l := NewLimiter(0, 5)
	start := time.Now()
	for i := 0; i < 100; i++ {
		if err := l.Wait(context.Background()); err != nil {
			t.Fatalf("Wait #%d: %v", i, err)
		}
	}
	// 100 sequential calls should take microseconds, not seconds.
	if d := time.Since(start); d > 100*time.Millisecond {
		t.Errorf("zero-rate Limiter took %v for 100 calls; should be effectively instant", d)
	}
}

func TestNegativeRateIsNoOp(t *testing.T) {
	l := NewLimiter(-1, 1)
	if err := l.Wait(context.Background()); err != nil {
		t.Errorf("negative-rate Limiter Wait returned %v, want nil", err)
	}
}

func TestBurstAllowsImmediateCalls(t *testing.T) {
	// 10 rps, burst 3. The first 3 calls should not block.
	l := NewLimiter(10, 3)
	start := time.Now()
	for i := 0; i < 3; i++ {
		if err := l.Wait(context.Background()); err != nil {
			t.Fatalf("burst call #%d: %v", i, err)
		}
	}
	if d := time.Since(start); d > 30*time.Millisecond {
		t.Errorf("burst of 3 took %v; should be near-instant", d)
	}
}

func TestSteadyStateIsRateLimited(t *testing.T) {
	// 20 rps = one every 50ms, burst 1. Two calls back-to-back should
	// span at least ~50ms after the initial token is consumed.
	l := NewLimiter(20, 1)
	// Drain the initial bucket.
	if err := l.Wait(context.Background()); err != nil {
		t.Fatalf("drain: %v", err)
	}
	start := time.Now()
	if err := l.Wait(context.Background()); err != nil {
		t.Fatalf("rate-limited call: %v", err)
	}
	// Allow some slack for scheduler latency, but require at least 30ms.
	if d := time.Since(start); d < 30*time.Millisecond {
		t.Errorf("rate-limited call took %v; expected at least ~50ms", d)
	}
}

func TestContextCancellationUnblocksWait(t *testing.T) {
	// Very low rate so Wait would otherwise block a long time.
	l := NewLimiter(0.1, 1)
	// Consume the burst token.
	if err := l.Wait(context.Background()); err != nil {
		t.Fatalf("drain: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := l.Wait(ctx)
	d := time.Since(start)
	if err == nil {
		t.Fatal("Wait returned nil after context deadline expired")
	}
	if d > 200*time.Millisecond {
		t.Errorf("Wait took %v to honour 50ms deadline", d)
	}
}

func TestConcurrentWaitersAreSerialised(t *testing.T) {
	// 50 rps, burst 1. With 5 concurrent goroutines, total elapsed
	// should be > ~80ms (5 calls at 50ms intervals, accounting for the
	// initial burst token).
	l := NewLimiter(50, 1)
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = l.Wait(context.Background())
		}()
	}
	wg.Wait()
	d := time.Since(start)
	// 5 calls, 1 free (burst), 4 at 20ms each = ~80ms minimum.
	if d < 50*time.Millisecond {
		t.Errorf("5 serialised calls at 50rps took %v; expected at least ~80ms", d)
	}
}
