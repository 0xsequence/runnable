package runnable

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithStats(t *testing.T) {

	t.Run("with stats", func(t *testing.T) {
		stats := NewStats()

		started := make(chan struct{})
		r := New(func(ctx context.Context) error {
			started <- struct{}{}
			time.Sleep(500 * time.Millisecond)
			return nil
		}, WithStats("test", stats))

		go func() {
			err := r.Run(context.Background())
			require.NoError(t, err)
		}()

		<-started

		s := stats.Get()
		assert.Equal(t, true, s.Runnable["test"].Running)

		err := r.Stop(context.Background())
		require.NoError(t, err)

		s = stats.Get()
		assert.Equal(t, false, s.Runnable["test"].Running)
	})

	t.Run("with stats, error", func(t *testing.T) {
		stats := NewStats()

		r := New(func(ctx context.Context) error {
			return assert.AnError
		}, WithStats("test", stats))

		err := r.Run(context.Background())
		require.Error(t, err)

		s := stats.Get()
		assert.Equal(t, false, s.Runnable["test"].Running)
		assert.Equal(t, assert.AnError, s.Runnable["test"].LastError)
	})

	t.Run("with stats, restart", func(t *testing.T) {
		stats := NewStats()

		counter := 0
		r := New(func(ctx context.Context) error {
			defer func() { counter++ }()
			if counter < 1 {
				return assert.AnError
			}
			return nil
		}, WithStats("test", stats), WithRetry(3, ResetNever))

		err := r.Run(context.Background())
		require.NoError(t, err)

		s := stats.Get()
		assert.Equal(t, false, s.Runnable["test"].Running)
		assert.Equal(t, 1, s.Runnable["test"].Restarts)
	})
}
