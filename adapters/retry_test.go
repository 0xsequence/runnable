package adapters_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xsequence/runnable"
	"github.com/0xsequence/runnable/adapters"
)

func TestRetry_SucceedsOnSecondAttempt(t *testing.T) {
	var count atomic.Int32
	work := func(ctx context.Context) error {
		if count.Add(1) < 2 {
			return assert.AnError
		}
		return nil
	}

	r := runnable.New(work, runnable.WithAdapters(adapters.Retry(3, adapters.ResetNever)))
	require.NoError(t, r.Run(context.Background()))
	assert.Equal(t, int32(2), count.Load())
}

func TestRetry_GivesUpAfterMaxAttempts(t *testing.T) {
	var count atomic.Int32
	work := func(ctx context.Context) error {
		count.Add(1)
		return assert.AnError
	}

	r := runnable.New(work, runnable.WithAdapters(adapters.Retry(3, adapters.ResetNever)))
	err := r.Run(context.Background())
	require.ErrorIs(t, err, assert.AnError)
	assert.Equal(t, int32(3), count.Load())
}

func TestRetry_ResetsBudgetAfterQuietPeriod(t *testing.T) {
	var count atomic.Int32
	work := func(ctx context.Context) error {
		c := count.Add(1)
		if c < 5 {
			time.Sleep(200 * time.Millisecond)
			return assert.AnError
		}
		return nil
	}

	r := runnable.New(work, runnable.WithAdapters(adapters.Retry(3, 100*time.Millisecond)))
	require.NoError(t, r.Run(context.Background()))
	assert.Equal(t, int32(5), count.Load())
}

func TestRetry_DoesNotRetryContextErrors(t *testing.T) {
	var count atomic.Int32
	work := func(ctx context.Context) error {
		count.Add(1)
		return context.Canceled
	}

	r := runnable.New(work, runnable.WithAdapters(adapters.Retry(3, adapters.ResetNever)))
	err := r.Run(context.Background())
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int32(1), count.Load())
}
