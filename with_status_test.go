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

	t.Run("with status, restart", func(t *testing.T) {
		store := NewStatusStore()

		counter := 0
		r := New(func(ctx context.Context) error {
			defer func() { counter++ }()
			if counter < 1 {
				return assert.AnError
			}
			return nil
		}, WithStatus("test", store), WithRetry(3, ResetNever))

		err := r.Run(context.Background())
		require.NoError(t, err)

		s := store.Get()
		assert.Equal(t, false, s["test"].Running)
		assert.Equal(t, 1, s["test"].Restarts)
	})
}
