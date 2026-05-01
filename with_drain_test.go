package runnable

import (
	"context"
	"errors"
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

	t.Run("Stop falls back to cancel on drain timeout", func(t *testing.T) {
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
		require.NoError(t, err)
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
