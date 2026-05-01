package runnable

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTicker(t *testing.T) {
	t.Run("fires on interval", func(t *testing.T) {
		var count atomic.Int32

		r := NewTicker(50*time.Millisecond, func(ctx context.Context) error {
			count.Add(1)
			return nil
		})

		go func() {
			_ = r.Run(context.Background())
		}()

		time.Sleep(175 * time.Millisecond)

		err := r.Stop(context.Background())
		require.NoError(t, err)

		c := count.Load()
		assert.GreaterOrEqual(t, c, int32(2))
		assert.LessOrEqual(t, c, int32(4))
	})

	t.Run("Stop with drain allows current tick to finish", func(t *testing.T) {
		tickStarted := make(chan struct{}, 1)
		var completed atomic.Int32

		r := NewTicker(20*time.Millisecond, func(ctx context.Context) error {
			select {
			case tickStarted <- struct{}{}:
			default:
			}
			time.Sleep(200 * time.Millisecond)
			completed.Add(1)
			return nil
		}, WithDrain(1*time.Second))

		go func() {
			_ = r.Run(context.Background())
		}()

		<-tickStarted

		start := time.Now()
		err := r.Stop(context.Background())
		elapsed := time.Since(start)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, completed.Load(), int32(1), "in-flight tick should complete")
		assert.Less(t, elapsed, 500*time.Millisecond)
	})

	t.Run("Stop without drain cancels in-flight tick", func(t *testing.T) {
		tickStarted := make(chan struct{}, 1)
		tickErr := make(chan error, 1)

		r := NewTicker(20*time.Millisecond, func(ctx context.Context) error {
			select {
			case tickStarted <- struct{}{}:
			default:
			}
			<-ctx.Done()
			tickErr <- ctx.Err()
			return ctx.Err()
		})

		runDone := make(chan error, 1)
		go func() {
			runDone <- r.Run(context.Background())
		}()

		<-tickStarted
		err := r.Stop(context.Background())
		require.NoError(t, err)

		select {
		case e := <-tickErr:
			require.ErrorIs(t, e, context.Canceled)
		case <-time.After(time.Second):
			t.Fatal("tick did not observe ctx cancellation")
		}

		select {
		case e := <-runDone:
			require.ErrorIs(t, e, context.Canceled)
		case <-time.After(time.Second):
			t.Fatal("Run did not return")
		}
	})

	t.Run("tick error aborts loop", func(t *testing.T) {
		sentinel := errors.New("boom")
		var count atomic.Int32

		r := NewTicker(20*time.Millisecond, func(ctx context.Context) error {
			if count.Add(1) == 2 {
				return sentinel
			}
			return nil
		})

		err := r.Run(context.Background())
		require.ErrorIs(t, err, sentinel)
		assert.Equal(t, int32(2), count.Load())
	})

	t.Run("respects outer ctx cancel", func(t *testing.T) {
		var count atomic.Int32

		r := NewTicker(20*time.Millisecond, func(ctx context.Context) error {
			count.Add(1)
			return nil
		})

		ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
		defer cancel()

		err := r.Run(ctx)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.False(t, r.IsRunning())
	})
}
