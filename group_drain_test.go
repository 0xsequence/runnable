package runnable_test

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

func TestNewGroup_DrainEnabledChild(t *testing.T) {
	// Load-bearing test: a Draining-wrapped child of a group must
	// drain when the group is stopped. In v0.1 this was silently
	// broken — the child observed groupCtx.Done() and exited
	// without ever seeing its drain signal. The adapter design
	// fixes this by construction.
	started := make(chan struct{})
	drainObserved := make(chan struct{})
	var ctxCancelObserved atomic.Bool

	drainingChild := runnable.New(adapters.Draining(1*time.Second, func(ctx context.Context) error {
		close(started)
		select {
		case <-adapters.Stopping(ctx):
			close(drainObserved)
			return nil
		case <-ctx.Done():
			ctxCancelObserved.Store(true)
			return ctx.Err()
		}
	}))

	plainChild := runnable.New(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	group := runnable.NewGroup(drainingChild, plainChild)
	go func() { _ = group.Run(context.Background()) }()

	<-started
	require.NoError(t, group.Stop(context.Background()))

	select {
	case <-drainObserved:
	default:
		t.Fatal("draining child never observed Stopping; group did not propagate drain")
	}
	assert.False(t, ctxCancelObserved.Load(), "draining child saw ctx.Done() instead of Stopping")
}
