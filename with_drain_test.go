package runnable

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithDrain(t *testing.T) {
	t.Run("Stop waits for runFunc to return", func(t *testing.T) {
		started := make(chan struct{})
		runFuncErr := make(chan error, 1)

		r := New(func(ctx context.Context) error {
			close(started)
			<-Stopping(ctx)
			time.Sleep(200 * time.Millisecond)
			// Return naturally without observing ctx cancellation.
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return nil
		}, WithDrain(1*time.Second))

		go func() {
			runFuncErr <- r.Run(context.Background())
		}()

		<-started
		assert.True(t, r.IsRunning())

		start := time.Now()
		err := r.Stop(context.Background())
		elapsed := time.Since(start)
		require.NoError(t, err)
		assert.False(t, r.IsRunning())
		assert.GreaterOrEqual(t, elapsed, 200*time.Millisecond)
		assert.Less(t, elapsed, 500*time.Millisecond)

		select {
		case err := <-runFuncErr:
			require.NoError(t, err, "runFunc should return naturally, not via ctx cancellation")
		case <-time.After(time.Second):
			t.Fatal("runFunc did not return")
		}
	})

	t.Run("Stop returns ErrDrainTimedOut on fall-through", func(t *testing.T) {
		started := make(chan struct{})
		runFuncErr := make(chan error, 1)

		r := New(func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		}, WithDrain(100*time.Millisecond))

		go func() {
			runFuncErr <- r.Run(context.Background())
		}()

		<-started

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()
		err := r.Stop(stopCtx)
		require.ErrorIs(t, err, ErrDrainTimedOut)
		assert.False(t, r.IsRunning())

		select {
		case err := <-runFuncErr:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(time.Second):
			t.Fatal("runFunc did not return")
		}
	})

	t.Run("Stop returns DeadlineExceeded when runFunc stuck", func(t *testing.T) {
		started := make(chan struct{})
		release := make(chan struct{})

		r := New(func(ctx context.Context) error {
			close(started)
			<-release
			return nil
		}, WithDrain(50*time.Millisecond))

		go func() {
			_ = r.Run(context.Background())
		}()

		<-started

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer stopCancel()
		err := r.Stop(stopCtx)
		require.ErrorIs(t, err, context.DeadlineExceeded)

		// Release the runFunc so the goroutine can exit cleanly.
		close(release)
	})

	t.Run("outer ctx cancel still propagates", func(t *testing.T) {
		started := make(chan struct{})
		runFuncErr := make(chan error, 1)

		r := New(func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		}, WithDrain(1*time.Second))

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			runFuncErr <- r.Run(ctx)
		}()

		<-started
		cancel()

		select {
		case err := <-runFuncErr:
			require.True(t, errors.Is(err, context.Canceled))
		case <-time.After(time.Second):
			t.Fatal("runFunc did not exit on outer ctx cancel")
		}
		assert.False(t, r.IsRunning())
	})

	t.Run("Stop forces cancel when caller ctx expires during drain", func(t *testing.T) {
		started := make(chan struct{})
		runFuncDone := make(chan struct{})

		// runFunc respects its own ctx but not Stopping(ctx). Without
		// the independent drain timer, Stop with a caller ctx shorter
		// than drainTimeout could return ctx.Err() before r.runCancel()
		// fired, leaving the runnable alive.
		r := New(func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			close(runFuncDone)
			return ctx.Err()
		}, WithDrain(10*time.Second))

		go func() {
			_ = r.Run(context.Background())
		}()

		<-started

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer stopCancel()
		err := r.Stop(stopCtx)
		require.ErrorIs(t, err, context.DeadlineExceeded)

		select {
		case <-runFuncDone:
		case <-time.After(2 * time.Second):
			t.Fatal("runnable was not force-cancelled when caller ctx expired during drain")
		}
	})

	t.Run("concurrent Stop preserves drain semantics", func(t *testing.T) {
		started := make(chan struct{})
		drainObserved := make(chan struct{})
		var ctxCancelObserved atomic.Bool

		// runFunc must exit via Stopping(ctx). If a concurrent Stop
		// falls through to r.runCancel(), ctx.Done() fires and the
		// drain semantics are violated.
		r := New(func(ctx context.Context) error {
			close(started)
			select {
			case <-Stopping(ctx):
				close(drainObserved)
				return nil
			case <-ctx.Done():
				ctxCancelObserved.Store(true)
				return ctx.Err()
			}
		}, WithDrain(2*time.Second))

		go func() {
			_ = r.Run(context.Background())
		}()

		<-started

		const callers = 10
		var wg sync.WaitGroup
		errs := make([]error, callers)
		for i := 0; i < callers; i++ {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = r.Stop(context.Background())
			}()
		}
		wg.Wait()

		// Each Stop must return either nil (drove or waited on the
		// drain) or ErrNotRunning (Run already exited before this
		// caller grabbed the lock). No double-close panic.
		for _, err := range errs {
			if err != nil {
				require.ErrorIs(t, err, ErrNotRunning)
			}
		}

		select {
		case <-drainObserved:
		default:
			t.Fatal("runFunc never observed Stopping(ctx); drain was bypassed by concurrent Stop")
		}
		assert.False(t, ctxCancelObserved.Load(), "drain semantics violated: runCtx was hard-cancelled by a concurrent Stop")
		assert.False(t, r.IsRunning())
	})

	t.Run("secondary Stop with shorter deadline escalates runCancel", func(t *testing.T) {
		started := make(chan struct{})
		runFuncDone := make(chan struct{})

		// runFunc waits only on ctx.Done() (ignores Stopping). Without
		// escalation, Stop B's deadline expires but the runnable keeps
		// draining for the full drainTimeout (5s).
		r := New(func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			close(runFuncDone)
			return ctx.Err()
		}, WithDrain(5*time.Second))

		go func() {
			_ = r.Run(context.Background())
		}()

		<-started

		// Stop A: no deadline; primary, drives drain.
		aDone := make(chan error, 1)
		go func() {
			aDone <- r.Stop(context.Background())
		}()

		time.Sleep(20 * time.Millisecond)

		// Stop B: 100ms deadline; secondary. Must escalate so runFunc
		// exits within the caller's budget.
		bCtx, bCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer bCancel()
		start := time.Now()
		bErr := r.Stop(bCtx)
		bElapsed := time.Since(start)
		require.ErrorIs(t, bErr, context.DeadlineExceeded)
		assert.Less(t, bElapsed, 500*time.Millisecond, "Stop B should not wait beyond its own deadline")

		select {
		case <-runFuncDone:
		case <-time.After(time.Second):
			t.Fatal("runnable was not force-cancelled when secondary Stop's ctx expired")
		}

		select {
		case err := <-aDone:
			require.NoError(t, err)
		case <-time.After(time.Second):
			t.Fatal("Stop A did not return after runFunc exited")
		}
	})

	t.Run("runnable can be re-Run after a concurrent-Stop lifecycle", func(t *testing.T) {
		// Lifecycle survival smoke test: a runnable that's been
		// stopped via concurrent Stops (including one with an
		// already-cancelled ctx hitting the runCancel escalation path)
		// can be re-Run on the same instance and complete cleanly.
		//
		// This does NOT deterministically cover the runCancel-snapshot
		// race in runnable.go (where a stale Stop could in principle
		// reach r.runCancel after Run has overwritten the field). That
		// race requires pausing the secondary Stop between its lock
		// release and runCancel call while a fresh Run executes —
		// achievable only via testing/synctest or a runtime hook.
		// Both are out of scope here; the snapshot fix is verified by
		// inspection, not this test.
		r := New(func(ctx context.Context) error {
			select {
			case <-Stopping(ctx):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}, WithDrain(1*time.Second))

		go func() {
			_ = r.Run(context.Background())
		}()

		for !r.IsRunning() {
			time.Sleep(time.Millisecond)
		}

		// Primary Stop, no deadline — drives drain.
		primaryDone := make(chan error, 1)
		go func() {
			primaryDone <- r.Stop(context.Background())
		}()

		// Secondary Stop with an already-cancelled ctx — exercises
		// the ctx.Done() escalation path that calls runCancel.
		cancelledCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_ = r.Stop(cancelledCtx)

		<-primaryDone
		for r.IsRunning() {
			time.Sleep(time.Millisecond)
		}

		// Round 2 — same runnable, fresh Run. Should run undisturbed
		// until we Stop it.
		round2Done := make(chan error, 1)
		go func() {
			round2Done <- r.Run(context.Background())
		}()

		select {
		case err := <-round2Done:
			t.Fatalf("round-2 runnable exited prematurely: %v", err)
		case <-time.After(150 * time.Millisecond):
		}

		require.NoError(t, r.Stop(context.Background()))
		select {
		case err := <-round2Done:
			require.NoError(t, err)
		case <-time.After(time.Second):
			t.Fatal("round-2 runnable did not exit after Stop")
		}
	})

	t.Run("WithRetry stops retrying after Stopping fires", func(t *testing.T) {
		started := make(chan struct{}, 1)
		var attempts atomic.Int32

		// runFunc errors transiently every time. Without the
		// Stopping-aware retry guard, WithRetry keeps re-entering
		// runFunc after Stop is called.
		r := New(func(ctx context.Context) error {
			select {
			case started <- struct{}{}:
			default:
			}
			attempts.Add(1)
			<-Stopping(ctx)
			return errors.New("transient")
		}, WithDrain(2*time.Second), WithRetry(100, ResetNever))

		runDone := make(chan error, 1)
		go func() {
			runDone <- r.Run(context.Background())
		}()

		<-started

		require.NoError(t, r.Stop(context.Background()))

		select {
		case <-runDone:
		case <-time.After(time.Second):
			t.Fatal("Run did not return after Stop")
		}

		// Exactly one attempt should have run — the retry wrapper
		// must observe Stopping and abandon further attempts.
		assert.Equal(t, int32(1), attempts.Load(), "retry continued after Stop drained")
	})

	t.Run("Stopping returns nil when not configured", func(t *testing.T) {
		var observed bool

		r := New(func(ctx context.Context) error {
			ch := Stopping(ctx)
			// Selecting on nil channel blocks forever; default branch runs.
			select {
			case <-ch:
				observed = true
			default:
				observed = ch == nil
			}
			return nil
		})

		err := r.Run(context.Background())
		require.NoError(t, err)
		assert.True(t, observed, "Stopping(ctx) should be nil without WithDrain")
	})
}
