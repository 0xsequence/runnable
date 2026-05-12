package runnable_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xsequence/runnable"
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

func TestWithPublisher_InstalledInRunCtx(t *testing.T) {
	p := &capturingPublisher{}
	var observed runnable.Publisher

	r := runnable.New(func(ctx context.Context) error {
		observed = runnable.PublisherFrom(ctx)
		return nil
	}, runnable.WithPublisher(p))

	require.NoError(t, r.Run(context.Background()))
	assert.Same(t, p, observed, "PublisherFrom should return the publisher installed via WithPublisher")
}

func TestPublish_NoOpWithoutPublisher(t *testing.T) {
	r := runnable.New(func(ctx context.Context) error {
		runnable.Publish(ctx, "anything") // must not panic when no publisher set
		return nil
	})
	require.NoError(t, r.Run(context.Background()))
}

func TestWithPublisher_StacksAdditively(t *testing.T) {
	a, b := &capturingPublisher{}, &capturingPublisher{}

	r := runnable.New(func(ctx context.Context) error {
		runnable.Publish(ctx, "hello")
		return nil
	}, runnable.WithPublisher(a), runnable.WithPublisher(b))

	require.NoError(t, r.Run(context.Background()))
	assert.Equal(t, []any{"hello"}, a.snapshot())
	assert.Equal(t, []any{"hello"}, b.snapshot())
}
