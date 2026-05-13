package adapters_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xsequence/runnable"
	"github.com/0xsequence/runnable/adapters"
)

func TestRecovering_TurnsPanicIntoError(t *testing.T) {
	var captured any
	handler := func(_ context.Context, rec any, _ []byte) {
		captured = rec
	}

	work := func(ctx context.Context) error {
		panic("boom")
	}

	r := runnable.New(work, runnable.WithAdapters(adapters.Recovering(handler)))
	err := r.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
	assert.Equal(t, "boom", captured)
}

func TestRecovering_NilHandlerStillRecovers(t *testing.T) {
	work := func(ctx context.Context) error {
		panic("boom")
	}

	r := runnable.New(work, runnable.WithAdapters(adapters.Recovering(nil)))
	err := r.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestRecovering_PassesThroughOnSuccess(t *testing.T) {
	called := false
	handler := func(_ context.Context, rec any, _ []byte) {
		called = true
	}

	work := func(ctx context.Context) error { return nil }

	r := runnable.New(work, runnable.WithAdapters(adapters.Recovering(handler)))
	require.NoError(t, r.Run(context.Background()))
	assert.False(t, called, "handler must not fire when next returns normally")
}

func TestRecovering_PassesThroughError(t *testing.T) {
	work := func(ctx context.Context) error { return assert.AnError }

	r := runnable.New(work, runnable.WithAdapters(adapters.Recovering(nil)))
	err := r.Run(context.Background())
	require.ErrorIs(t, err, assert.AnError)
}
