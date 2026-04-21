package ccxtutil

import (
	"context"
	"fmt"
)

// WatchWithContext calls a blocking CCXT Watch* function in a goroutine and
// races the result against ctx.Done(). If the context is cancelled first, the
// spawned goroutine is left to complete on its own (CCXT has no cancellation
// API), but its result channel is drained by a background goroutine so it can
// exit cleanly once the underlying WebSocket call returns.
func WatchWithContext[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	type result struct {
		val T
		err error
	}
	ch := make(chan result, 1)

	go func() {
		v, err := fn()
		ch <- result{val: v, err: err}
	}()

	select {
	case res := <-ch:
		return res.val, res.err
	case <-ctx.Done():
		// Drain the channel in the background so the goroutine can exit
		// once the CCXT call returns (e.g. after api.Close()).
		go func() { <-ch }()
		var zero T
		return zero, fmt.Errorf("watch cancelled: %w", ctx.Err())
	}
}
