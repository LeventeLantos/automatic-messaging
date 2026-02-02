package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew_InvalidArgs(t *testing.T) {
	t.Parallel()

	t.Run("interval must be > 0", func(t *testing.T) {
		t.Parallel()

		s, err := New(0, func(context.Context) {})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if s != nil {
			t.Fatalf("expected nil scheduler, got %#v", s)
		}
	})

	t.Run("tickFn must not be nil", func(t *testing.T) {
		t.Parallel()

		s, err := New(100*time.Millisecond, nil)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if s != nil {
			t.Fatalf("expected nil scheduler, got %#v", s)
		}
	})
}

func TestScheduler_StartStop_Basics(t *testing.T) {
	var calls atomic.Int64

	s, err := New(10*time.Millisecond, func(context.Context) {
		calls.Add(1)
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if s.IsRunning() {
		t.Fatalf("expected scheduler not running initially")
	}

	// Start should succeed first time.
	if ok := s.Start(); !ok {
		t.Fatalf("expected Start() true on first call")
	}

	if !s.IsRunning() {
		t.Fatalf("expected scheduler running after Start()")
	}

	// Start should fail when already running.
	if ok := s.Start(); ok {
		t.Fatalf("expected Start() false when already running")
	}

	// Wait for at least one tick (there is an immediate tick on Start()).
	waitForAtLeast(t, &calls, 1, 500*time.Millisecond)

	// Stop should succeed first time.
	if ok := s.Stop(); !ok {
		t.Fatalf("expected Stop() true on first call")
	}
	if s.IsRunning() {
		t.Fatalf("expected scheduler not running after Stop()")
	}

	// Stop should fail when already stopped.
	if ok := s.Stop(); ok {
		t.Fatalf("expected Stop() false when already stopped")
	}
}

func TestScheduler_DoesNotTickAfterStop(t *testing.T) {
	var calls atomic.Int64

	s, err := New(10*time.Millisecond, func(context.Context) {
		calls.Add(1)
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if ok := s.Start(); !ok {
		t.Fatalf("expected Start() true")
	}

	// Wait for a couple ticks so we have a baseline.
	waitForAtLeast(t, &calls, 2, 750*time.Millisecond)
	beforeStop := calls.Load()

	if ok := s.Stop(); !ok {
		t.Fatalf("expected Stop() true")
	}

	// Sleep longer than interval to ensure no further ticks occur.
	time.Sleep(100 * time.Millisecond)
	afterStop := calls.Load()

	if afterStop != beforeStop {
		t.Fatalf("expected no ticks after Stop; before=%d after=%d", beforeStop, afterStop)
	}
}

func TestScheduler_ImmediateTickOnStart(t *testing.T) {
	var calls atomic.Int64

	// Use a very large interval, expectt an immediate tick on Start()
	// due to safeTick before the loop.
	s, err := New(10*time.Second, func(context.Context) {
		calls.Add(1)
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if ok := s.Start(); !ok {
		t.Fatalf("expected Start() true")
	}
	defer s.Stop()

	waitForAtLeast(t, &calls, 1, 500*time.Millisecond)
}

func TestScheduler_PanicInTickIsRecoveredAndContinues(t *testing.T) {
	var calls atomic.Int64
	var panicked atomic.Bool

	s, err := New(10*time.Millisecond, func(context.Context) {
		// First call panics, subsequent calls increment.
		if panicked.CompareAndSwap(false, true) {
			panic("boom")
		}
		calls.Add(1)
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if ok := s.Start(); !ok {
		t.Fatalf("expected Start() true")
	}
	defer s.Stop()

	// If panic is recovered properly, scheduler should keep ticking afterwards.
	// Expect at least 1 non-panicking call to increment calls.
	waitForAtLeast(t, &calls, 1, 750*time.Millisecond)
}

func TestScheduler_StartStopMultipleTimes(t *testing.T) {
	var calls atomic.Int64

	s, err := New(10*time.Millisecond, func(context.Context) {
		calls.Add(1)
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	for i := 0; i < 3; i++ {
		if ok := s.Start(); !ok {
			t.Fatalf("iteration %d: expected Start() true", i)
		}

		waitForAtLeast(t, &calls, 1, 750*time.Millisecond)

		if ok := s.Stop(); !ok {
			t.Fatalf("iteration %d: expected Stop() true", i)
		}

		// Reset counter for next iteration to make the check clearer.
		calls.Store(0)
	}
}

func TestScheduler_TickFnReceivesCancelableContext(t *testing.T) {
	// This test ensures the tick function gets a context that is cancelled on Stop().
	// We capture the ctx from a tick and then stop the scheduler, expecting ctx.Done to close.
	var capturedMu sync.Mutex
	var captured context.Context

	s, err := New(10*time.Millisecond, func(ctx context.Context) {
		capturedMu.Lock()
		if captured == nil {
			captured = ctx
		}
		capturedMu.Unlock()
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if ok := s.Start(); !ok {
		t.Fatalf("expected Start() true")
	}

	// Wait until we captured a context.
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		capturedMu.Lock()
		got := captured
		capturedMu.Unlock()

		if got != nil {
			break
		}
		if time.Now().After(deadline) {
			_ = s.Stop()
			t.Fatalf("did not capture tick context in time")
		}
		time.Sleep(5 * time.Millisecond)
	}

	if ok := s.Stop(); !ok {
		t.Fatalf("expected Stop() true")
	}

	capturedMu.Lock()
	ctx := captured
	capturedMu.Unlock()

	select {
	case <-ctx.Done():
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("expected tick context to be canceled after Stop()")
	}
}

// waitForAtLeast waits until calls >= n or fails the test after timeout.
// Uses polling to avoid test flakes across CI.
func waitForAtLeast(t *testing.T, calls *atomic.Int64, n int64, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for {
		if calls.Load() >= n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for calls >= %d (got %d)", n, calls.Load())
		}
		time.Sleep(5 * time.Millisecond)
	}
}
