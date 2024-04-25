package runnable

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithRetry(t *testing.T) {

	t.Run("with retry", func(t *testing.T) {
		counter := 0

		r := New(func(ctx context.Context) error {
			defer func() { counter++ }()
			if counter < 1 {
				return assert.AnError
			}

			time.Sleep(500 * time.Millisecond)
			return nil
		}, WithRetry(3, ResetNever))

		err := r.Run(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 2, counter)
	})

	t.Run("with retry, error", func(t *testing.T) {
		counter := 0

		r := New(func(ctx context.Context) error {
			defer func() { counter++ }()
			return assert.AnError
		}, WithRetry(3, ResetNever))

		err := r.Run(context.Background())
		require.Error(t, err)
		assert.Equal(t, 3, counter)
	})

	t.Run("with retry, reset", func(t *testing.T) {
		counter := 0

		r := New(func(ctx context.Context) error {
			defer func() { counter++ }()
			if counter < 5 {
				time.Sleep(200 * time.Millisecond)
				return assert.AnError
			}
			return nil
		}, WithRetry(3, 100*time.Millisecond))

		err := r.Run(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 6, counter)
	})
}
