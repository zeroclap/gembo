package ccxtutil

import (
	"context"
	"testing"
	"time"
)

func TestWatchWithContext_Success(t *testing.T) {
	ctx := context.Background()
	val, err := WatchWithContext(ctx, func() (string, error) {
		return "hello", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "hello" {
		t.Fatalf("expected 'hello', got %q", val)
	}
}

func TestWatchWithContext_Error(t *testing.T) {
	ctx := context.Background()
	_, err := WatchWithContext(ctx, func() (string, error) {
		return "", context.DeadlineExceeded
	})
	if err != context.DeadlineExceeded {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestWatchWithContext_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context before the function returns.
	cancel()

	_, err := WatchWithContext(ctx, func() (string, error) {
		// Simulate a slow CCXT call that blocks longer than the context.
		time.Sleep(100 * time.Millisecond)
		return "late", nil
	})
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if ctx.Err() == nil {
		t.Fatal("expected context to be cancelled")
	}
}

func TestWatchWithContext_DrainOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Channel to signal the CCXT goroutine completed.
	done := make(chan struct{})

	_, _ = WatchWithContext(ctx, func() (string, error) {
		defer close(done)
		// Wait a bit before the context is cancelled.
		time.Sleep(50 * time.Millisecond)
		return "result", nil
	})

	cancel()

	// The background drain goroutine should let the CCXT goroutine exit.
	select {
	case <-done:
		// OK — goroutine completed and was drained.
	case <-time.After(2 * time.Second):
		t.Fatal("CCXT goroutine was not drained after context cancel")
	}
}
