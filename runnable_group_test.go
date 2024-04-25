package runnable

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGroup(t *testing.T) {

	t.Run("group", func(t *testing.T) {
		run1started := make(chan struct{})
		run2started := make(chan struct{})

		// Create a new group
		group := NewGroup(
			New(func(ctx context.Context) error {
				run1started <- struct{}{}
				time.Sleep(500 * time.Millisecond)
				return nil
			}),
			New(func(ctx context.Context) error {
				run2started <- struct{}{}
				time.Sleep(500 * time.Millisecond)
				return nil
			}),
		)

		// Run the group
		go func() {
			err := group.Run(context.Background())
			require.NoError(t, err)
		}()

		// Wait for the first runnable to start
		<-run1started
		<-run2started

		// Stop the group
		err := group.Stop(context.Background())
		require.NoError(t, err)
	})

	t.Run("group, error", func(t *testing.T) {
		// Create a new group
		group := NewGroup(
			New(func(ctx context.Context) error {
				select {
				case <-ctx.Done():
				}
				return nil
			}),
			New(func(ctx context.Context) error {
				return assert.AnError
			}),
		)

		// Run the group
		err := group.Run(context.Background())
		require.Error(t, err)
	})

}
