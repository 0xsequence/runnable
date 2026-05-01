package runnable

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunnable(t *testing.T) {

	t.Run("runnable with timeout, finish in time", func(t *testing.T) {
		started := make(chan struct{})

		r := New(func(ctx context.Context) error {
			started <- struct{}{}
			time.Sleep(500 * time.Millisecond)
			return nil
		})

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		go func() {
			err := r.Run(ctx)
			require.NoError(t, err)
		}()

		<-started
		assert.Equal(t, true, r.IsRunning())

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer stopCancel()
		err := r.Stop(stopCtx)
		require.NoError(t, err)
		assert.Equal(t, false, r.IsRunning())
	})

	t.Run("runnable with timeout", func(t *testing.T) {
		started := make(chan struct{})

		r := New(func(ctx context.Context) error {
			started <- struct{}{}
			time.Sleep(2 * time.Second)
			<-ctx.Done()
			return ctx.Err()
		})

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		go func() {
			err := r.Run(ctx)
			require.Error(t, err, context.DeadlineExceeded)
		}()

		<-started
		assert.Equal(t, true, r.IsRunning())

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer stopCancel()
		err := r.Stop(stopCtx)
		require.NoError(t, err)
		assert.Equal(t, false, r.IsRunning())
	})

	t.Run("runnable with timeout, stop before run", func(t *testing.T) {
		r := New(func(ctx context.Context) error {
			return nil
		})

		assert.Equal(t, false, r.IsRunning())

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer stopCancel()
		err := r.Stop(stopCtx)
		require.Error(t, err, ErrNotRunning)
		assert.Equal(t, false, r.IsRunning())
	})

	t.Run("runnable, stop timeout proves no drain behavior", func(t *testing.T) {
		started := make(chan struct{})
		ctxCancelObserved := make(chan struct{})

		r := New(func(ctx context.Context) error {
			started <- struct{}{}
			<-ctx.Done()
			close(ctxCancelObserved)
			time.Sleep(2 * time.Second)
			return ctx.Err()
		})

		go func() {
			_ = r.Run(context.Background())
		}()

		<-started
		assert.Equal(t, true, r.IsRunning())

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer stopCancel()
		err := r.Stop(stopCtx)
		require.ErrorIs(t, err, context.DeadlineExceeded)

		// Without WithDrain, Stop cancels runFunc's ctx immediately.
		select {
		case <-ctxCancelObserved:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("expected runFunc's ctx to be cancelled when Stop fires without WithDrain")
		}

		assert.Equal(t, true, r.IsRunning())
	})
}
