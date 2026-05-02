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

	t.Run("retry budget is per-Run-cycle", func(t *testing.T) {
		// Each Run cycle gets a fresh retry budget — lastTime must
		// not leak from the previous cycle. With a 100ms resetAfter
		// and a 50ms gap between cycles, a leaked lastTime would
		// cause cycle 2 to immediately reset i=0 inside the loop;
		// per-cycle scoping makes both cycles behave identically.
		runs := 0
		attempts := 0

		r := New(func(ctx context.Context) error {
			attempts++
			runs++
			if runs <= 2 {
				return assert.AnError
			}
			return nil
		}, WithRetry(3, 100*time.Millisecond))

		// Cycle 1: 2 fails + 1 success = 3 attempts, exhausts budget
		// just in time.
		require.NoError(t, r.Run(context.Background()))
		require.Equal(t, 3, attempts)

		runs = 0
		time.Sleep(50 * time.Millisecond)

		// Cycle 2: same shape, must succeed identically.
		require.NoError(t, r.Run(context.Background()))
		require.Equal(t, 6, attempts)
	})
}
