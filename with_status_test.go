package runnable

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithStatus(t *testing.T) {

	t.Run("with status", func(t *testing.T) {
		store := NewStatusStore()

		started := make(chan struct{})
		r := New(func(ctx context.Context) error {
			started <- struct{}{}
			time.Sleep(500 * time.Millisecond)
			return nil
		}, WithStatus("test", store))

		go func() {
			err := r.Run(context.Background())
			require.NoError(t, err)
		}()

		<-started

		s := store.Get()
		assert.Equal(t, true, s["test"].Running)

		err := r.Stop(context.Background())
		require.NoError(t, err)

		s = store.Get()
		assert.Equal(t, false, s["test"].Running)
	})

	t.Run("with status, error", func(t *testing.T) {
		store := NewStatusStore()

		r := New(func(ctx context.Context) error {
			return assert.AnError
		}, WithStatus("test", store))

		err := r.Run(context.Background())
		require.Error(t, err)

		s := store.Get()
		assert.Equal(t, false, s["test"].Running)
		assert.Equal(t, assert.AnError, s["test"].LastError)
	})

	t.Run("with status, restarts counted via RetryEvent", func(t *testing.T) {
		// adapters.Retry publishes RetryEvent; WithStatus subscribes via a
		// statusPublisher tagged with the runnable id. Restarts in the
		// resulting Status is the count of those events. We exercise the
		// publish path directly without importing adapters (cycle).
		store := NewStatusStore()
		r := New(func(ctx context.Context) error {
			p := PublisherFrom(ctx)
			require.NotNil(t, p, "WithStatus must install a Publisher on runCtx")
			p.Publish(RetryEvent{Attempt: 1, Err: assert.AnError})
			p.Publish(RetryEvent{Attempt: 2, Err: assert.AnError})
			return nil
		}, WithStatus("test", store))

		require.NoError(t, r.Run(context.Background()))

		s := store.Get()
		assert.Equal(t, 2, s["test"].Restarts)
	})
}
