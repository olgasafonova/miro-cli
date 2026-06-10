package miro

import (
	"context"
	"sync"
)

// FanOut applies fn to every element of items using at most max
// concurrent workers, returning the results in input order: out[i] is
// fn's result for items[i].
//
// max <= 1 runs items sequentially in the calling goroutine — identical
// in behavior and allocation profile to a plain range loop, so the
// no-concurrency path costs nothing. A max larger than len(items) is
// clamped to len(items).
//
// FanOut never rate-limits on its own. When fn issues requests through a
// *Client, that client's shared token bucket (see Limiter) paces them no
// matter how many workers call concurrently. Raising max therefore only
// improves wall-clock when per-call latency — not the limiter — is the
// bottleneck, e.g. when the caller has raised the rate ceiling via
// --rate-limit. At the conservative default rate a higher max is
// harmless but speeds nothing up.
//
// FanOut does not cancel in-flight work on context cancellation; fn is
// expected to observe ctx itself. The bulk verbs check ctx.Err() at the
// top of fn and record a per-item error, which preserves their
// sequential cancellation semantics under fan-out.
//
// Results are written at disjoint indices, so no lock guards out; the
// wg.Wait() below is the happens-before edge to the caller's read.
func FanOut[T any, R any](ctx context.Context, items []T, max int, fn func(ctx context.Context, i int, item T) R) []R {
	if len(items) == 0 {
		return make([]R, 0)
	}
	if max < 1 {
		max = 1
	}
	if max > len(items) {
		max = len(items)
	}
	if max == 1 {
		return fanOutSequential(ctx, items, fn)
	}
	return fanOutParallel(ctx, items, max, fn)
}

// fanOutSequential runs fn over items in the calling goroutine. This is
// the max<=1 path: a plain range loop with no channel or goroutine
// overhead, byte-for-byte the behaviour of the pre-fan-out code.
func fanOutSequential[T any, R any](ctx context.Context, items []T, fn func(ctx context.Context, i int, item T) R) []R {
	out := make([]R, len(items))
	for i, it := range items {
		out[i] = fn(ctx, i, it)
	}
	return out
}

// fanOutParallel dispatches items across workers goroutines over an
// unbuffered index channel, each writing its result to out at the item's
// own index. workers is assumed already clamped to [2, len(items)].
func fanOutParallel[T any, R any](ctx context.Context, items []T, workers int, fn func(ctx context.Context, i int, item T) R) []R {
	out := make([]R, len(items))
	idx := make(chan int)
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := range idx {
				out[i] = fn(ctx, i, items[i])
			}
		}()
	}
	for i := range items {
		idx <- i
	}
	close(idx)
	wg.Wait()
	return out
}
