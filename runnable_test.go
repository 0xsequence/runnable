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

			select {
			case <-ctx.Done():
				return ctx.Err()
			}
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

	t.Run("runnable, stop timeout", func(t *testing.T) {
		started := make(chan struct{})

		r := New(func(ctx context.Context) error {
			started <- struct{}{}
			time.Sleep(2 * time.Second)
			return nil
		})

		go func() {
			err := r.Run(context.Background())
			require.NoError(t, err)
		}()

		<-started
		assert.Equal(t, true, r.IsRunning())

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer stopCancel()
		err := r.Stop(stopCtx)
		require.Error(t, err, context.DeadlineExceeded)
		assert.Equal(t, true, r.IsRunning())
	})

}
