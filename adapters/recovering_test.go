package adapters_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xsequence/runnable"
	"github.com/0xsequence/runnable/adapters"
)

type capturingPublisher struct {
	mu     sync.Mutex
	events []any
}

func (c *capturingPublisher) Publish(event any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
}

func (c *capturingPublisher) snapshot() []any {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]any, len(c.events))
	copy(out, c.events)
	return out
}

func TestRecovering_TurnsPanicIntoError(t *testing.T) {
	work := func(ctx context.Context) error {
		panic("boom")
	}

	r := runnable.New(work, runnable.WithAdapters(adapters.Recovering()))
	err := r.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestRecovering_PublishesPanicEvent(t *testing.T) {
	pub := &capturingPublisher{}

	work := func(ctx context.Context) error {
		panic("boom")
	}

	r := runnable.New(work,
		runnable.WithPublisher(pub),
		runnable.WithAdapters(adapters.Recovering()),
	)
	err := r.Run(context.Background())
	require.Error(t, err)

	events := pub.snapshot()
	require.Len(t, events, 1)
	ev, ok := events[0].(runnable.PanicRecoveredEvent)
	require.True(t, ok, "expected PanicRecoveredEvent, got %T", events[0])
	assert.Equal(t, "boom", ev.Recovered)
	assert.NotEmpty(t, ev.Stack)
}

func TestRecovering_NoPublishOnSuccess(t *testing.T) {
	pub := &capturingPublisher{}
	work := func(ctx context.Context) error { return nil }

	r := runnable.New(work,
		runnable.WithPublisher(pub),
		runnable.WithAdapters(adapters.Recovering()),
	)
	require.NoError(t, r.Run(context.Background()))
	assert.Empty(t, pub.snapshot())
}

func TestRecovering_PassesThroughError(t *testing.T) {
	work := func(ctx context.Context) error { return assert.AnError }

	r := runnable.New(work, runnable.WithAdapters(adapters.Recovering()))
	err := r.Run(context.Background())
	require.ErrorIs(t, err, assert.AnError)
}
