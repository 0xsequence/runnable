package runnable_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xsequence/runnable"
	"github.com/0xsequence/runnable/adapters"
)

// Lives in runnable_test (not in the adapters package) because it
// crosses both: WithStatus is core, Retry is in adapters.
func TestStatusRestartsCountedFromRetryAdapter(t *testing.T) {
	store := runnable.NewStatusStore()
	var count atomic.Int32

	r := runnable.New(func(ctx context.Context) error {
		c := count.Add(1)
		if c < 3 {
			return assert.AnError
		}
		return nil
	},
		runnable.WithStatus("recon", store),
		runnable.WithAdapters(adapters.Retry(5, adapters.ResetNever)),
	)

	require.NoError(t, r.Run(context.Background()))

	s := store.Get()
	// Two failed attempts before success → two RetryEvents → Restarts = 2.
	assert.Equal(t, 2, s["recon"].Restarts)
	assert.Equal(t, false, s["recon"].Running)
}
