package miro

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"
)

func doubler(_ context.Context, _ int, v int) int { return v * 2 }

// wantDoubled asserts out[i] == items[i]*2 for every index. Centralising
// the per-element check keeps the individual tests free of multi-branch
// conditionals.
func wantDoubled(t *testing.T, label string, items, out []int) {
	t.Helper()
	if len(out) != len(items) {
		t.Fatalf("%s: len(out)=%d, want %d", label, len(out), len(items))
	}
	for i := range items {
		if out[i] != items[i]*2 {
			t.Fatalf("%s: out[%d]=%d, want %d", label, i, out[i], items[i]*2)
		}
	}
}

func iota64() []int {
	items := make([]int, 64)
	for i := range items {
		items[i] = i
	}
	return items
}

// TestFanOutPreservesOrder is the load-bearing guarantee: bulk-delete
// and bulk-update emit an order-sensitive Results[] envelope, so a pool
// that reordered completions would corrupt the output. Run it across the
// sequential, bounded, and clamped paths.
func TestFanOutPreservesOrder(t *testing.T) {
	items := iota64()
	for _, max := range []int{0, 1, 4, 8, 1000} {
		out := FanOut(context.Background(), items, max, doubler)
		wantDoubled(t, label(max), items, out)
	}
}

func TestFanOutEmpty(t *testing.T) {
	out := FanOut(context.Background(), []int{}, 8, doubler)
	if len(out) != 0 {
		t.Fatalf("len(out)=%d, want 0", len(out))
	}
}

// TestFanOutZeroAndNegativeMaxAreSequential pins the contract that a
// zero-value Globals.Concurrency (0) and any negative value run the
// sequential path rather than spinning up zero workers or panicking.
func TestFanOutZeroAndNegativeMaxAreSequential(t *testing.T) {
	items := []int{1, 2, 3}
	for _, max := range []int{0, -1, -100} {
		out := FanOut(context.Background(), items, max, doubler)
		wantDoubled(t, label(max), items, out)
	}
}

// TestFanOutBoundsConcurrency confirms no more than max workers run at
// once. A mutex-guarded counter records the peak in-flight count; the
// sleep forces real overlap so the peak is meaningful.
func TestFanOutBoundsConcurrency(t *testing.T) {
	const limit = 4
	items := iota64()

	var mu sync.Mutex
	inflight, peak := 0, 0
	out := FanOut(context.Background(), items, limit, func(_ context.Context, _ int, v int) int {
		mu.Lock()
		inflight++
		if inflight > peak {
			peak = inflight
		}
		mu.Unlock()
		time.Sleep(2 * time.Millisecond)
		mu.Lock()
		inflight--
		mu.Unlock()
		return v * 2
	})

	if peak > limit {
		t.Fatalf("observed %d concurrent workers, want <= %d", peak, limit)
	}
	if peak < 2 {
		t.Fatalf("observed peak %d; expected real overlap (>=2) at limit %d", peak, limit)
	}
	wantDoubled(t, "bounded", items, out)
}

// TestFanOutPassesIndex guards the positional-context contract that
// bulk-update relies on for its "patches[%d]" error messages.
func TestFanOutPassesIndex(t *testing.T) {
	items := []string{"a", "b", "c"}
	out := FanOut(context.Background(), items, 2, func(_ context.Context, i int, _ string) int {
		return i
	})
	for i := range items {
		if out[i] != i {
			t.Fatalf("out[%d]=%d, want %d", i, out[i], i)
		}
	}
}

func label(max int) string {
	return "max=" + strconv.Itoa(max)
}
